package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Wire types for network API

type InitiatePairingRequestWire struct {
	Secret string `json:"secret,omitempty"`
}

type InitiatePairingResponseWire struct {
	PairingID string    `json:"pairingId"`
	Code      string    `json:"code"`
	Payload   string    `json:"payload"`
	Address   string    `json:"address"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// maxNetworkRequestBodyBytes enforces the strict 4 KiB body cap on network endpoints.
const maxNetworkRequestBodyBytes = 4096

// checkJSONContentType validates that requests with a body declare application/json.
func checkJSONContentType(r *http.Request) bool {
	if r.ContentLength == 0 {
		return true
	}
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(strings.ToLower(ct), "application/json")
}

// InitiatePairingHandler creates a new 5-minute PairingSession.
// Requires admin role and constant-time match of DEVICE_PAIRING_SECRET if configured.
func InitiatePairingHandler(
	sessionRepo domain.PairingSessionRepository,
	codeGen func() (domain.PairingCode, error),
	pairingSecret string,
	serverAddress string,
	scheme string,
	ids domain.IDGenerator,
	now func() time.Time,
	logger *slog.Logger,
) http.Handler {
	if now == nil {
		now = time.Now
	}
	if scheme == "" {
		scheme = "http"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}
		if user.Role != domain.RoleAdmin {
			writeForbidden(w, "insufficient permissions for this resource", corrID)
			return
		}

		if !checkJSONContentType(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "UnsupportedMediaType",
				Message:       "Content-Type must be application/json",
				CorrelationID: corrID,
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxNetworkRequestBodyBytes)
		var req InitiatePairingRequestWire
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			WriteError(w, domain.InvalidInput, "malformed request body or body too large", corrID)
			return
		}

		if pairingSecret != "" {
			if subtle.ConstantTimeCompare([]byte(req.Secret), []byte(pairingSecret)) != 1 {
				writeForbidden(w, "invalid pairing secret", corrID)
				return
			}
		}

		code, err := codeGen()
		if err != nil {
			WriteError(w, domain.Internal, "failed to generate pairing code", corrID)
			return
		}

		currentTime := now().UTC()
		sessionID := domain.PairingSessionID(ids.NewID())
		session, err := domain.NewPairingSession(sessionID, user.UserID, code, 5*time.Minute, currentTime)
		if err != nil {
			WriteError(w, domain.Internal, "failed to create pairing session", corrID)
			return
		}

		ip := clientIP(r)
		if err := sessionRepo.SaveWithInitiatorIP(r.Context(), session, ip); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to save pairing session", corrID)
			return
		}

		if logger != nil {
			logger.Info("pairing session initiated",
				"correlationId", corrID,
				"pairingId", string(sessionID),
				"initiatedBy", string(user.UserID),
			)
		}

		payload := fmt.Sprintf("%s://%s/connect?c=%s", scheme, serverAddress, code.Display())
		resp := InitiatePairingResponseWire{
			PairingID: string(sessionID),
			Code:      code.Display(),
			Payload:   payload,
			Address:   serverAddress,
			ExpiresAt: session.ExpiresAt(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// PairingVerifier abstracts atomic verification and provisional device enrollment.
type PairingVerifier interface {
	VerifyAndConsume(
		ctx context.Context,
		code domain.PairingCode,
		deviceID domain.DeviceID,
		label string,
		deviceClass domain.DeviceClass,
		now time.Time,
	) (*domain.PairingSession, error)
}

type VerifyPairingRequestWire struct {
	Code  string `json:"code"`
	Label string `json:"label,omitempty"`
}

type VerifyPairingResponseWire struct {
	EnrolmentGrant string `json:"enrolmentGrant"`
	Address        string `json:"address"`
	HostName       string `json:"hostName"`
}

const genericPairingNotFoundMsg = "pairing code not recognised"

func parseDeviceClass(userAgent string) domain.DeviceClass {
	ua := strings.ToLower(userAgent)
	if strings.Contains(ua, "tv") || strings.Contains(ua, "smart-tv") || strings.Contains(ua, "crkey") || strings.Contains(ua, "googletv") || strings.Contains(ua, "appletv") {
		return domain.DeviceClassTV
	}
	if strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet") || (strings.Contains(ua, "android") && !strings.Contains(ua, "mobile")) {
		return domain.DeviceClassTablet
	}
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "iphone") || strings.Contains(ua, "android") {
		return domain.DeviceClassPhone
	}
	if strings.Contains(ua, "windows") || strings.Contains(ua, "macintosh") || strings.Contains(ua, "linux") || strings.Contains(ua, "x11") {
		return domain.DeviceClassDesktop
	}
	return domain.DeviceClassUnknown
}

// VerifyPairingHandler verifies a pairing code, marks the session consumed,
// inserts a provisional PairedDevice, and issues an enrolment grant.
// Unauthenticated, Origin-checked, rate-limited.
func VerifyPairingHandler(
	verifier PairingVerifier,
	grantSigner *auth.EnrolmentGrantSigner,
	serverAddress string,
	hostName string,
	ids domain.IDGenerator,
	now func() time.Time,
	logger *slog.Logger,
) http.Handler {
	if now == nil {
		now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if !checkJSONContentType(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "UnsupportedMediaType",
				Message:       "Content-Type must be application/json",
				CorrelationID: corrID,
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxNetworkRequestBodyBytes)
		var req VerifyPairingRequestWire
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request body or body too large", corrID)
			return
		}

		code, err := domain.NewPairingCode(req.Code)
		if err != nil {
			WriteError(w, domain.InvalidInput, "malformed pairing code: must be 8 Crockford base32 characters", corrID)
			return
		}

		currentTime := now().UTC()
		devID := domain.DeviceID(ids.NewID())
		deviceClass := parseDeviceClass(r.Header.Get("User-Agent"))
		label := strings.TrimSpace(req.Label)
		if label == "" {
			label = string(deviceClass)
			if label == "" {
				label = "Device"
			}
		}

		session, err := verifier.VerifyAndConsume(r.Context(), code, devID, label, deviceClass, currentTime)
		if err != nil {
			// Wrong / expired / never-existed -> byte-identical generic 404
			WriteError(w, domain.NotFound, genericPairingNotFoundMsg, corrID)
			return
		}

		grant, err := grantSigner.Sign(session.ID(), currentTime)
		if err != nil {
			WriteError(w, domain.Internal, "failed to issue enrolment grant", corrID)
			return
		}

		if logger != nil {
			logger.Info("pairing code verified and device enrolled provisionally",
				"correlationId", corrID,
				"pairingId", string(session.ID()),
				"deviceId", string(devID),
				"deviceClass", string(deviceClass),
			)
		}

		resp := VerifyPairingResponseWire{
			EnrolmentGrant: grant,
			Address:        serverAddress,
			HostName:       hostName,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

type PairingQRResponseWire struct {
	Payload   string    `json:"payload"`
	Address   string    `json:"address"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
	State     string    `json:"state"`
}

// PairingQRHandler returns the pairing payload, address, code, and state for
// the initiating admin only. Cross-admin attempts return 404.
func PairingQRHandler(
	sessionRepo domain.PairingSessionRepository,
	serverAddress string,
	scheme string,
	logger *slog.Logger,
) http.Handler {
	if scheme == "" {
		scheme = "http"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}
		if user.Role != domain.RoleAdmin {
			writeForbidden(w, "insufficient permissions for this resource", corrID)
			return
		}

		sessionID := domain.PairingSessionID(r.PathValue("id"))
		if sessionID == "" {
			WriteError(w, domain.NotFound, "pairing session not found", corrID)
			return
		}

		session, err := sessionRepo.FindByID(r.Context(), sessionID)
		if err != nil {
			WriteError(w, domain.NotFound, "pairing session not found", corrID)
			return
		}

		// Security requirement: cross-admin enumeration defence -> 404, not 403
		if session.InitiatedBy() != user.UserID {
			WriteError(w, domain.NotFound, "pairing session not found", corrID)
			return
		}

		resp := PairingQRResponseWire{
			Address:   serverAddress,
			ExpiresAt: session.ExpiresAt(),
			State:     string(session.State()),
		}

		// Terminal states return empty code and payload
		if session.State() == domain.PairingConsumed || session.State() == domain.PairingExpired {
			resp.Code = ""
			resp.Payload = ""
		} else {
			resp.Code = session.Code().Display()
			resp.Payload = fmt.Sprintf("%s://%s/connect?c=%s", scheme, serverAddress, resp.Code)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

type NetworkInfo struct {
	Reachability string
	TLSMode      string
	BindAddress  string
	ACMEDomain   string
	HostName     string
	Addresses    []NetworkAddressWire
}

type NetworkAddressWire struct {
	Scope string `json:"scope"`
	URL   string `json:"url"`
}

type NetworkStatusReaderWire struct {
	Reachability string `json:"reachability"`
	TLSMode      string `json:"tlsMode"`
	AuthRequired bool   `json:"authRequired"`
	Address      string `json:"address"`
}

type NetworkStatusAdminWire struct {
	Reachability string               `json:"reachability"`
	TLSMode      string               `json:"tlsMode"`
	AuthRequired bool                 `json:"authRequired"`
	Address      string               `json:"address"`
	Addresses    []NetworkAddressWire `json:"addresses"`
	HostName     string               `json:"hostName"`
	ACMEDomain   string               `json:"acmeDomain,omitempty"`
}

// NetworkStatusHandler returns reachability, TLS mode, and address info scoped by role.
// Reader gets minimal status; Admin gets full interface list, hostName, and ACME domain.
// Neither response contains secrets, filesystem paths, or unscrubbed config.
func NetworkStatusHandler(
	infoProvider func() NetworkInfo,
	logger *slog.Logger,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		info := infoProvider()

		// Determine address: match client Host against known server addresses,
		// never reflect an unverified client Host header directly.
		matchedAddress := ""
		scheme := "http"
		if info.TLSMode != "none" {
			scheme = "https"
		}
		for _, addr := range info.Addresses {
			if u, err := url.Parse(addr.URL); err == nil && u.Host == r.Host {
				matchedAddress = addr.URL
				break
			}
		}
		if matchedAddress == "" {
			if len(info.Addresses) > 0 {
				matchedAddress = info.Addresses[0].URL
			} else {
				matchedAddress = fmt.Sprintf("%s://%s", scheme, info.BindAddress)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if user.Role == domain.RoleAdmin {
			adminResp := NetworkStatusAdminWire{
				Reachability: info.Reachability,
				TLSMode:      info.TLSMode,
				AuthRequired: true,
				Address:      matchedAddress,
				Addresses:    info.Addresses,
				HostName:     info.HostName,
			}
			if info.TLSMode == "acme" {
				adminResp.ACMEDomain = info.ACMEDomain
			}
			_ = json.NewEncoder(w).Encode(adminResp)
			return
		}

		readerResp := NetworkStatusReaderWire{
			Reachability: info.Reachability,
			TLSMode:      info.TLSMode,
			AuthRequired: true,
			Address:      matchedAddress,
		}
		_ = json.NewEncoder(w).Encode(readerResp)
	})
}

var dnsLabelRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func isValidHostName(host string) bool {
	label := strings.TrimSuffix(host, ".local")
	if len(label) < 1 || len(label) > 63 {
		return false
	}
	return dnsLabelRegex.MatchString(label)
}

type NetworkSettingsResponseWire struct {
	HostName           string    `json:"hostName"`
	RememberDeviceDays int       `json:"rememberDeviceDays"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// defaultNetworkSettings is the row the update handler synthesises when
// none is saved yet; GetNetworkSettingsHandler returns the same shape so
// the settings form has real values to pre-fill.
func defaultNetworkSettings(now func() time.Time) *domain.NetworkSettings {
	return &domain.NetworkSettings{
		HostName:           "alexandryn.local",
		RememberDeviceDays: 30,
		UpdatedAt:          now().UTC(),
	}
}

// GetNetworkSettingsHandler serves GET /api/v1/network/settings.
// Admin only. Returns the saved runtime-safe settings, or the defaults
// when none has been saved.
func GetNetworkSettingsHandler(settingsRepo domain.NetworkSettingsRepository, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}
		if user.Role != domain.RoleAdmin {
			writeForbidden(w, "insufficient permissions for this resource", corrID)
			return
		}

		settings, err := settingsRepo.Get(r.Context())
		if err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				settings = defaultNetworkSettings(now)
			} else {
				WriteError(w, domain.CategoryOf(err), "failed to read network settings", corrID)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NetworkSettingsResponseWire{
			HostName:           settings.HostName,
			RememberDeviceDays: settings.RememberDeviceDays,
			UpdatedAt:          settings.UpdatedAt,
		})
	})
}

// UpdateNetworkSettingsHandler updates runtime-safe network settings.
// Admin only. Strictly allows hostName and rememberDeviceDays; any other key
// (or restart-only keys) returns 400 InvalidInput.
func UpdateNetworkSettingsHandler(
	settingsRepo domain.NetworkSettingsRepository,
	now func() time.Time,
	logger *slog.Logger,
) http.Handler {
	if now == nil {
		now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}
		if user.Role != domain.RoleAdmin {
			writeForbidden(w, "insufficient permissions for this resource", corrID)
			return
		}

		if !checkJSONContentType(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "UnsupportedMediaType",
				Message:       "Content-Type must be application/json",
				CorrelationID: corrID,
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxNetworkRequestBodyBytes)
		var rawMap map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&rawMap); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request body or body too large", corrID)
			return
		}

		// Enforce strict allowlist and give helpful error messages on forbidden/restart-only keys
		for key := range rawMap {
			switch key {
			case "hostName", "rememberDeviceDays":
				// allowed
			case "bindAddress":
				WriteError(w, domain.InvalidInput, "changes to the bind address require editing the config file and restarting", corrID)
				return
			case "tlsCertFile", "tlsKeyFile", "tlsMode":
				WriteError(w, domain.InvalidInput, "changes to TLS configuration require editing the config file and restarting", corrID)
				return
			case "acmeDomain", "acmeEmail":
				WriteError(w, domain.InvalidInput, "changes to ACME domain require editing the config file and restarting", corrID)
				return
			case "authRequired":
				WriteError(w, domain.InvalidInput, "authentication cannot be disabled", corrID)
				return
			default:
				WriteError(w, domain.InvalidInput, fmt.Sprintf("unrecognised settings key %q", key), corrID)
				return
			}
		}

		existing, err := settingsRepo.Get(r.Context())
		if err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				existing = defaultNetworkSettings(now)
			} else {
				WriteError(w, domain.CategoryOf(err), "failed to read network settings", corrID)
				return
			}
		}

		if rawVal, ok := rawMap["hostName"]; ok {
			var h string
			if err := json.Unmarshal(rawVal, &h); err != nil || !isValidHostName(h) {
				WriteError(w, domain.InvalidInput, "invalid hostName: must be 1..63 lowercase alphanumeric characters or hyphens with optional .local suffix", corrID)
				return
			}
			existing.HostName = h
		}

		if rawVal, ok := rawMap["rememberDeviceDays"]; ok {
			var days int
			if err := json.Unmarshal(rawVal, &days); err != nil || days < 1 || days > 90 {
				WriteError(w, domain.InvalidInput, "rememberDeviceDays must be an integer between 1 and 90", corrID)
				return
			}
			existing.RememberDeviceDays = days
		}

		existing.UpdatedAt = now().UTC()
		if err := settingsRepo.Upsert(r.Context(), existing); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to save network settings", corrID)
			return
		}

		if logger != nil {
			logger.Info("network settings updated",
				"correlationId", corrID,
				"hostName", existing.HostName,
				"rememberDeviceDays", existing.RememberDeviceDays,
			)
		}

		resp := NetworkSettingsResponseWire{
			HostName:           existing.HostName,
			RememberDeviceDays: existing.RememberDeviceDays,
			UpdatedAt:          existing.UpdatedAt,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// DeletePairingHandler deletes or revokes a pairing session/device.
// Admin only, initiator only (returns 404 for another admin's session).
// Non-terminal session -> expired.
// Consumed session -> revokes the PairedDevice produced.
func DeletePairingHandler(
	sessionRepo domain.PairingSessionRepository,
	deviceRepo domain.PairedDeviceRepository,
	now func() time.Time,
	logger *slog.Logger,
) http.Handler {
	if now == nil {
		now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}
		if user.Role != domain.RoleAdmin {
			writeForbidden(w, "insufficient permissions for this resource", corrID)
			return
		}

		sessionID := domain.PairingSessionID(r.PathValue("id"))
		if sessionID == "" {
			WriteError(w, domain.NotFound, "pairing session not found", corrID)
			return
		}

		session, err := sessionRepo.FindByID(r.Context(), sessionID)
		if err != nil {
			WriteError(w, domain.NotFound, "pairing session not found", corrID)
			return
		}

		// Cross-admin enumeration defence: 404
		if session.InitiatedBy() != user.UserID {
			WriteError(w, domain.NotFound, "pairing session not found", corrID)
			return
		}

		currentTime := now().UTC()

		if session.State() == domain.PairingConsumed {
			// Revoke by session ID directly: the device may still be
			// provisional (owner_id NULL, not yet claimed by login), which
			// FindByPairingSessionID can't represent as a domain.PairedDevice.
			_ = deviceRepo.RevokeByPairingSessionID(r.Context(), sessionID, currentTime)
		} else if session.State() == domain.PairingPending || session.State() == domain.PairingVerified {
			session.ExpireAt(session.ExpiresAt())
			_ = sessionRepo.Save(r.Context(), session)
		}

		if logger != nil {
			logger.Info("pairing session revoked/deleted",
				"correlationId", corrID,
				"pairingId", string(sessionID),
				"state", string(session.State()),
			)
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
