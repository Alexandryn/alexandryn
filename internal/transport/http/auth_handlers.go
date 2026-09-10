package http

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

type authConfig struct {
	grantSigner  *auth.EnrolmentGrantSigner
	devRepo      domain.PairedDeviceRepository
	jtiRepo      domain.EnrolmentGrantJTIRepository
	settingsRepo domain.NetworkSettingsRepository
	logger       *slog.Logger
}

type LoginOption func(*authConfig)
type RefreshOption = LoginOption

// WithEnrolmentGrant configures the login handler to process device enrolment grants (T4.7, FR-9).
func WithEnrolmentGrant(
	grantSigner *auth.EnrolmentGrantSigner,
	devRepo domain.PairedDeviceRepository,
	jtiRepo domain.EnrolmentGrantJTIRepository,
	logger *slog.Logger,
) LoginOption {
	return func(c *authConfig) {
		c.grantSigner = grantSigner
		c.devRepo = devRepo
		c.jtiRepo = jtiRepo
		c.logger = logger
	}
}

// WithNetworkSettings configures network settings (such as rememberDeviceDays) for token expiry (T4.7, FR-3/FR-4).
func WithNetworkSettings(settingsRepo domain.NetworkSettingsRepository) LoginOption {
	return func(c *authConfig) {
		c.settingsRepo = settingsRepo
	}
}

// applyEnrolmentGrant associates a device with a user via an enrolment
// grant (T4.7, FR-9). Called only once a login is actually completing —
// LoginHandler calls it after the TOTP-MFA gate (not before: a password-only
// step that only returns mfaRequired must not durably assign device
// ownership or spend the grant's JTI), and TOTPVerifyHandler calls it after
// a successful code/recovery-code check for the MFA-enabled path. Fail-safe:
// any error here is logged and ignored without failing the login.
func applyEnrolmentGrant(ctx context.Context, cfg authConfig, grant string, userID domain.UserID, now time.Time, corrID string) {
	if grant == "" || cfg.grantSigner == nil || cfg.devRepo == nil || cfg.jtiRepo == nil {
		return
	}
	claims, err := cfg.grantSigner.Verify(grant, now)
	if err != nil {
		if cfg.logger != nil {
			cfg.logger.Info("invalid enrolment grant ignored", "correlationId", corrID, "error", err.Error())
		}
		return
	}
	// Atomic single-use claim before any durable work: a concurrent
	// second login with the same grant loses the claim and stops here,
	// rather than both passing an Exists check and racing (#250).
	claimed, err := cfg.jtiRepo.Claim(ctx, claims.JTI, now)
	if err != nil || !claimed {
		if cfg.logger != nil {
			cfg.logger.Info("replayed or unclaimable enrolment grant ignored", "correlationId", corrID, "jti", claims.JTI)
		}
		return
	}
	if err := cfg.devRepo.AssignOwnerByPairingSession(ctx, claims.SessionID, userID); err != nil {
		if cfg.logger != nil {
			cfg.logger.Info("failed to assign device owner after claiming grant, ignoring", "correlationId", corrID, "error", err.Error())
		}
		return
	}
}

type UserSummaryWire struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type AuthResponseWire struct {
	User         *UserSummaryWire `json:"user,omitempty"`
	AccessToken  string           `json:"accessToken,omitempty"`
	RefreshToken string           `json:"refreshToken,omitempty"`
	MFARequired  bool             `json:"mfaRequired,omitempty"`
	MFATicket    string           `json:"mfaTicket,omitempty"`
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

func generateRandomToken(bytesLen int) (raw string, hash string, err error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

// SetupStatusHandler reports whether initial admin setup has been completed (FR-1).
func SetupStatusHandler(userRepo domain.UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		count, err := userRepo.CountUsers(r.Context())
		if err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to check setup status", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"isSetup": count > 0})
	})
}

// SetupHandler handles initial admin registration wizard (FR-1).
func SetupHandler(
	userRepo domain.UserRepository,
	credRepo domain.CredentialRepository,
	libRepo domain.LibraryRepository,
	memRepo domain.LibraryMembershipRepository,
	rtRepo domain.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		count, err := userRepo.CountUsers(r.Context())
		if err != nil {
			WriteError(w, domain.CategoryOf(err), "setup check failed", corrID)
			return
		}
		if count > 0 {
			WriteError(w, domain.Conflict, "system already initialized", corrID)
			return
		}

		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request payload", corrID)
			return
		}

		if len(req.Password) < 8 || len(req.Password) > 128 {
			WriteError(w, domain.InvalidInput, "password must be between 8 and 128 characters", corrID)
			return
		}

		now := time.Now()
		userID := domain.UserID(idGen.NewID())
		user, err := domain.NewUser(userID, req.Username, req.Email, domain.RoleAdmin, now, now)
		if err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		pwdHash, err := hasher.HashPassword(req.Password)
		if err != nil {
			WriteError(w, domain.Internal, "failed to hash password", corrID)
			return
		}
		creds, err := domain.NewUserCredentials(userID, pwdHash, now)
		if err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		if err := userRepo.Save(r.Context(), user); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to create user", corrID)
			return
		}
		if err := credRepo.Save(r.Context(), creds); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to save credentials", corrID)
			return
		}

		// Ensure default library membership
		memID := domain.LibraryMembershipID(idGen.NewID())
		mem, _ := domain.NewLibraryMembership(memID, domain.DefaultLibraryID, userID, domain.RoleAdmin, now)
		_ = memRepo.Save(r.Context(), mem)

		// Create Refresh Token
		rawRT, hashRT, _ := generateRandomToken(32)
		rtID := domain.RefreshTokenID(idGen.NewID())
		rt, _ := domain.NewRefreshToken(rtID, userID, hashRT, now.Add(30*24*time.Hour), now)
		_ = rtRepo.Save(r.Context(), rt)

		// Create Access Token
		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   userID,
			Type:      auth.TokenTypeAccess,
			Username:  user.Username(),
			Role:      domain.RoleAdmin,
			Libraries: []domain.LibraryID{domain.DefaultLibraryID},
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			User: &UserSummaryWire{
				ID:       string(user.ID()),
				Username: user.Username(),
				Email:    user.Email(),
				Role:     string(user.Role()),
			},
			AccessToken:  accessToken,
			RefreshToken: rawRT,
		})
	})
}

// LoginHandler authenticates users with Argon2id and issues tokens (FR-3).
func LoginHandler(
	userRepo domain.UserRepository,
	credRepo domain.CredentialRepository,
	mfaRepo domain.MFARepository,
	memRepo domain.LibraryMembershipRepository,
	rtRepo domain.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
	limiter *auth.IPRateLimiter,
	opts ...LoginOption,
) http.Handler {
	var cfg authConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "too many login attempts, please try again later",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			EmailOrUsername string `json:"emailOrUsername"`
			Password        string `json:"password"`
			EnrolmentGrant  string `json:"enrolmentGrant,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request", corrID)
			return
		}

		// Lookup by email or username
		user, err := userRepo.FindByEmail(r.Context(), req.EmailOrUsername)
		if err != nil {
			user, err = userRepo.FindByUsername(r.Context(), req.EmailOrUsername)
			if err != nil {
				// No such account: still run the KDF so the response
				// time does not reveal whether the account exists (#187).
				_, _ = hasher.VerifyPassword(req.Password, auth.DummyPasswordHash())
				WriteError(w, domain.Unauthorized, "invalid credentials", corrID)
				return
			}
		}

		creds, err := credRepo.FindByUserID(r.Context(), user.ID())
		if err != nil {
			_, _ = hasher.VerifyPassword(req.Password, auth.DummyPasswordHash())
			WriteError(w, domain.Unauthorized, "invalid credentials", corrID)
			return
		}

		match, err := hasher.VerifyPassword(req.Password, creds.PasswordHash())
		if err != nil || !match {
			WriteError(w, domain.Unauthorized, "invalid credentials", corrID)
			return
		}

		now := time.Now()

		// Check TOTP MFA before doing anything the enrolment grant would
		// durably commit: a password-only request against an MFA-enabled
		// account must not spend the grant or assign device ownership,
		// since login has not actually completed (no tokens are issued
		// below — only mfaRequired: true). TOTPVerifyHandler processes the
		// grant instead, once the second factor is verified.
		totp, err := mfaRepo.FindByUserID(r.Context(), user.ID())
		if err == nil && totp.IsEnabled() {
			mfaTicket, err := signer.SignMFATicket(user.ID(), now.Add(5*time.Minute))
			if err != nil {
				WriteError(w, domain.Internal, "failed to issue MFA ticket", corrID)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(AuthResponseWire{
				MFARequired: true,
				MFATicket:   mfaTicket,
			})
			return
		}

		applyEnrolmentGrant(r.Context(), cfg, req.EnrolmentGrant, user.ID(), now, corrID)

		// Resolve library memberships for JWT claims
		mems, _ := memRepo.FindByUser(r.Context(), user.ID())
		var libIDs []domain.LibraryID
		for _, m := range mems {
			libIDs = append(libIDs, m.LibraryID())
		}
		if len(libIDs) == 0 {
			libIDs = []domain.LibraryID{domain.DefaultLibraryID}
		}

		rtTTL := 30 * 24 * time.Hour
		if cfg.settingsRepo != nil {
			if settings, err := cfg.settingsRepo.Get(r.Context()); err == nil && settings != nil && settings.RememberDeviceDays >= 1 && settings.RememberDeviceDays <= 90 {
				rtTTL = time.Duration(settings.RememberDeviceDays) * 24 * time.Hour
			}
		}

		rawRT, hashRT, _ := generateRandomToken(32)
		rtID := domain.RefreshTokenID(idGen.NewID())
		rt, _ := domain.NewRefreshToken(rtID, user.ID(), hashRT, now.Add(rtTTL), now)
		_ = rtRepo.Save(r.Context(), rt)

		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   user.ID(),
			Type:      auth.TokenTypeAccess,
			Username:  user.Username(),
			Role:      user.Role(),
			Libraries: libIDs,
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			User: &UserSummaryWire{
				ID:       string(user.ID()),
				Username: user.Username(),
				Email:    user.Email(),
				Role:     string(user.Role()),
			},
			AccessToken:  accessToken,
			RefreshToken: rawRT,
		})
	})
}

// RefreshHandler rotates refresh tokens and issues fresh access tokens (FR-4).
func RefreshHandler(
	rtRepo domain.RefreshTokenRepository,
	userRepo domain.UserRepository,
	memRepo domain.LibraryMembershipRepository,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
	limiter *auth.IPRateLimiter,
	opts ...RefreshOption,
) http.Handler {
	var cfg authConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "rate limit exceeded",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			WriteError(w, domain.Unauthorized, "invalid refresh token", corrID)
			return
		}

		h := sha256.Sum256([]byte(req.RefreshToken))
		hash := hex.EncodeToString(h[:])

		now := time.Now()
		rt, err := rtRepo.FindByTokenHash(r.Context(), hash)
		if err != nil || !rt.IsActive(now) {
			WriteError(w, domain.Unauthorized, "invalid or expired refresh token", corrID)
			return
		}

		user, err := userRepo.FindByID(r.Context(), rt.UserID())
		if err != nil {
			WriteError(w, domain.Unauthorized, "user no longer exists", corrID)
			return
		}

		// Rotate: revoke old token, insert new token
		rt.Revoke(now)
		_ = rtRepo.Save(r.Context(), rt)

		rtTTL := 30 * 24 * time.Hour
		if cfg.settingsRepo != nil {
			if settings, err := cfg.settingsRepo.Get(r.Context()); err == nil && settings != nil && settings.RememberDeviceDays >= 1 && settings.RememberDeviceDays <= 90 {
				rtTTL = time.Duration(settings.RememberDeviceDays) * 24 * time.Hour
			}
		}

		rawNewRT, hashNewRT, _ := generateRandomToken(32)
		newRTID := domain.RefreshTokenID(idGen.NewID())
		newRT, _ := domain.NewRefreshToken(newRTID, user.ID(), hashNewRT, now.Add(rtTTL), now)
		_ = rtRepo.Save(r.Context(), newRT)

		mems, _ := memRepo.FindByUser(r.Context(), user.ID())
		var libIDs []domain.LibraryID
		for _, m := range mems {
			libIDs = append(libIDs, m.LibraryID())
		}
		if len(libIDs) == 0 {
			libIDs = []domain.LibraryID{domain.DefaultLibraryID}
		}

		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   user.ID(),
			Type:      auth.TokenTypeAccess,
			Username:  user.Username(),
			Role:      user.Role(),
			Libraries: libIDs,
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			AccessToken:  accessToken,
			RefreshToken: rawNewRT,
		})
	})
}

// LogoutHandler revokes the active refresh token (FR-5).
func LogoutHandler(rtRepo domain.RefreshTokenRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.RefreshToken != "" {
			h := sha256.Sum256([]byte(req.RefreshToken))
			hash := hex.EncodeToString(h[:])
			if rt, err := rtRepo.FindByTokenHash(r.Context(), hash); err == nil {
				rt.Revoke(time.Now())
				_ = rtRepo.Save(r.Context(), rt)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// PasswordResetRequestHandler generates a reset token without revealing account presence (FR-6).
func PasswordResetRequestHandler(
	userRepo domain.UserRepository,
	prRepo domain.PasswordResetRepository,
	idGen domain.IDGenerator,
	limiter *auth.IPRateLimiter,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "rate limit exceeded",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Email != "" {
			if user, err := userRepo.FindByEmail(r.Context(), req.Email); err == nil {
				_, hash, _ := generateRandomToken(32)
				now := time.Now()
				prt, _ := domain.NewPasswordResetToken(
					domain.PasswordResetID(idGen.NewID()),
					user.ID(),
					hash,
					now.Add(1*time.Hour),
					now,
				)
				_ = prRepo.Save(r.Context(), prt)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "If the account exists, a reset link has been dispatched",
		})
	})
}

// PasswordResetConfirmHandler updates password and revokes existing sessions (FR-6).
func PasswordResetConfirmHandler(
	userRepo domain.UserRepository,
	credRepo domain.CredentialRepository,
	prRepo domain.PasswordResetRepository,
	rtRepo domain.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	limiter *auth.IPRateLimiter,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "rate limit exceeded",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			Token       string `json:"token"`
			NewPassword string `json:"newPassword"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
			WriteError(w, domain.InvalidInput, "invalid request", corrID)
			return
		}

		if len(req.NewPassword) < 8 || len(req.NewPassword) > 128 {
			WriteError(w, domain.InvalidInput, "password must be between 8 and 128 characters", corrID)
			return
		}

		h := sha256.Sum256([]byte(req.Token))
		hash := hex.EncodeToString(h[:])

		now := time.Now()
		prt, err := prRepo.FindByTokenHash(r.Context(), hash)
		if err != nil || !prt.IsValid(now) {
			WriteError(w, domain.Unauthorized, "invalid or expired reset token", corrID)
			return
		}

		newHash, err := hasher.HashPassword(req.NewPassword)
		if err != nil {
			WriteError(w, domain.Internal, "failed to hash password", corrID)
			return
		}

		creds, _ := domain.NewUserCredentials(prt.UserID(), newHash, now)
		if err := credRepo.Save(r.Context(), creds); err != nil {
			WriteError(w, domain.Internal, "failed to save credentials", corrID)
			return
		}

		prt.MarkUsed(now)
		_ = prRepo.Save(r.Context(), prt)
		_ = rtRepo.RevokeAllForUser(r.Context(), prt.UserID(), now)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})
}

// TOTPSetupHandler starts 2FA enrollment (ADR 0027). Generating a fresh
// secret overwrites any existing enrolment, so it is a step-up action:
// the account password is required even though the access token already
// authenticates the request (audit 0016 #107).
func TOTPSetupHandler(mfaRepo domain.MFARepository, credRepo domain.CredentialRepository, hasher auth.PasswordHasher, totpEngine *auth.TOTPEngine, masterKey []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
			WriteError(w, domain.InvalidInput, "password is required to change 2FA settings", corrID)
			return
		}
		creds, err := credRepo.FindByUserID(r.Context(), user.UserID)
		if err != nil {
			WriteError(w, domain.Unauthorized, "credentials not found", corrID)
			return
		}
		match, err := hasher.VerifyPassword(req.Password, creds.PasswordHash())
		if err != nil || !match {
			WriteError(w, domain.Unauthorized, "invalid password", corrID)
			return
		}

		secret, err := totpEngine.GenerateSecret()
		if err != nil {
			WriteError(w, domain.Internal, "failed to generate TOTP secret", corrID)
			return
		}

		codes, hashes, err := totpEngine.GenerateRecoveryCodes(8)
		if err != nil {
			WriteError(w, domain.Internal, "failed to generate recovery codes", corrID)
			return
		}

		encryptedSecret, err := auth.EncryptSecret([]byte(secret), masterKey)
		if err != nil {
			WriteError(w, domain.Internal, "failed to encrypt secret", corrID)
			return
		}

		now := time.Now()
		settings, err := domain.NewTOTPSettings(user.UserID, encryptedSecret, hashes, false, nil, now)
		if err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		if err := mfaRepo.Save(r.Context(), settings); err != nil {
			WriteError(w, domain.Internal, "failed to save MFA settings", corrID)
			return
		}

		keyURI := totpEngine.KeyURI(user.Username, secret)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"secret":        secret,
			"keyUri":        keyURI,
			"recoveryCodes": codes,
		})
	})
}

// TOTPConfirmHandler confirms 2FA enrollment with first code (ADR 0027).
func TOTPConfirmHandler(mfaRepo domain.MFARepository, totpEngine *auth.TOTPEngine, masterKey []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
			WriteError(w, domain.InvalidInput, "missing code", corrID)
			return
		}

		settings, err := mfaRepo.FindByUserID(r.Context(), user.UserID)
		if err != nil {
			WriteError(w, domain.NotFound, "MFA setup not initiated", corrID)
			return
		}

		secretBytes, err := auth.DecryptSecret(settings.EncryptedSecret(), masterKey)
		if err != nil {
			WriteError(w, domain.Internal, "failed to decrypt secret", corrID)
			return
		}

		now := time.Now()
		valid, err := totpEngine.ValidateCode(string(secretBytes), req.Code, now)
		if err != nil || !valid {
			WriteError(w, domain.InvalidInput, "invalid TOTP verification code", corrID)
			return
		}

		settings.Confirm(now)
		if err := mfaRepo.Save(r.Context(), settings); err != nil {
			WriteError(w, domain.Internal, "failed to activate MFA", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"enabled": true})
	})
}

// writeTooManyRequests writes the shared 429 body used by every
// brute-force-throttled auth endpoint.
func writeTooManyRequests(w http.ResponseWriter, corrID, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(errorBody{
		Code:          "TooManyRequests",
		Message:       message,
		CorrelationID: corrID,
	})
}

// TOTPVerifyHandler verifies MFA during login (ADR 0027). ipLimiter
// throttles per source address (parity with LoginHandler); userLimiter
// throttles per user id so an attacker who has a password cannot grind
// the ~10^6 TOTP space by rotating addresses (audit 0016 #89). Both are
// nil-safe.
func TOTPVerifyHandler(
	mfaRepo domain.MFARepository,
	userRepo domain.UserRepository,
	rtRepo domain.RefreshTokenRepository,
	memRepo domain.LibraryMembershipRepository,
	totpEngine *auth.TOTPEngine,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
	masterKey []byte,
	ipLimiter *auth.IPRateLimiter,
	userLimiter *auth.IPRateLimiter,
	ticketJTIs domain.MFATicketJTIRepository,
	opts ...LoginOption,
) http.Handler {
	var cfg authConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if ipLimiter != nil && !ipLimiter.Allow(clientIP(r)) {
			writeTooManyRequests(w, corrID, "too many MFA attempts, please try again later")
			return
		}

		var req struct {
			MFATicket      string `json:"mfaTicket"`
			Code           string `json:"code,omitempty"`
			RecoveryCode   string `json:"recoveryCode,omitempty"`
			EnrolmentGrant string `json:"enrolmentGrant,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MFATicket == "" {
			WriteError(w, domain.InvalidInput, "missing MFA ticket", corrID)
			return
		}

		now := time.Now()
		userID, ticketJTI, err := signer.VerifyMFATicket(req.MFATicket, now)
		if err != nil {
			WriteError(w, domain.Unauthorized, "invalid or expired MFA ticket", corrID)
			return
		}

		// Single-use: claim the ticket's JTI on presentation so a
		// captured ticket cannot be replayed within its TTL (#189). A
		// spent or unclaimable ticket is rejected; the client must
		// re-authenticate to get a fresh one.
		if ticketJTIs != nil && ticketJTI != "" {
			claimed, err := ticketJTIs.Claim(r.Context(), ticketJTI, now)
			if err != nil {
				WriteError(w, domain.Unavailable, "could not verify MFA ticket, try again", corrID)
				return
			}
			if !claimed {
				WriteError(w, domain.Unauthorized, "this MFA ticket has already been used, sign in again", corrID)
				return
			}
		}

		if userLimiter != nil && !userLimiter.Allow(string(userID)) {
			writeTooManyRequests(w, corrID, "too many MFA attempts for this account, please try again later")
			return
		}

		settings, err := mfaRepo.FindByUserID(r.Context(), userID)
		if err != nil || !settings.IsEnabled() {
			WriteError(w, domain.Unauthorized, "MFA not enabled for user", corrID)
			return
		}

		user, err := userRepo.FindByID(r.Context(), userID)
		if err != nil {
			WriteError(w, domain.Unauthorized, "user not found", corrID)
			return
		}

		verified := false

		if req.Code != "" {
			secretBytes, err := auth.DecryptSecret(settings.EncryptedSecret(), masterKey)
			if err == nil {
				valid, _ := totpEngine.ValidateCode(string(secretBytes), req.Code, now)
				verified = valid
			}
		} else if req.RecoveryCode != "" {
			matchIdx := totpEngine.VerifyRecoveryCode(req.RecoveryCode, settings.RecoveryCodeHashes())
			if matchIdx != -1 {
				verified = true
				// Invalidate the used recovery code
				hashes := settings.RecoveryCodeHashes()
				newHashes := append(hashes[:matchIdx], hashes[matchIdx+1:]...)
				newSettings, _ := domain.NewTOTPSettings(userID, settings.EncryptedSecret(), newHashes, true, settings.ConfirmedAt(), settings.CreatedAt())
				_ = mfaRepo.Save(r.Context(), newSettings)
			}
		}

		if !verified {
			WriteError(w, domain.Unauthorized, "invalid MFA code or recovery code", corrID)
			return
		}

		applyEnrolmentGrant(r.Context(), cfg, req.EnrolmentGrant, user.ID(), now, corrID)

		mems, _ := memRepo.FindByUser(r.Context(), user.ID())
		var libIDs []domain.LibraryID
		for _, m := range mems {
			libIDs = append(libIDs, m.LibraryID())
		}
		if len(libIDs) == 0 {
			libIDs = []domain.LibraryID{domain.DefaultLibraryID}
		}

		rawRT, hashRT, _ := generateRandomToken(32)
		rtID := domain.RefreshTokenID(idGen.NewID())
		rt, _ := domain.NewRefreshToken(rtID, user.ID(), hashRT, now.Add(30*24*time.Hour), now)
		_ = rtRepo.Save(r.Context(), rt)

		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   user.ID(),
			Type:      auth.TokenTypeAccess,
			Username:  user.Username(),
			Role:      user.Role(),
			Libraries: libIDs,
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			User: &UserSummaryWire{
				ID:       string(user.ID()),
				Username: user.Username(),
				Email:    user.Email(),
				Role:     string(user.Role()),
			},
			AccessToken:  accessToken,
			RefreshToken: rawRT,
		})
	})
}

// TOTPDisableHandler disables 2FA upon password confirmation.
func TOTPDisableHandler(mfaRepo domain.MFARepository, credRepo domain.CredentialRepository, hasher auth.PasswordHasher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
			WriteError(w, domain.InvalidInput, "password required to disable 2FA", corrID)
			return
		}

		creds, err := credRepo.FindByUserID(r.Context(), user.UserID)
		if err != nil {
			WriteError(w, domain.Unauthorized, "credentials not found", corrID)
			return
		}

		match, err := hasher.VerifyPassword(req.Password, creds.PasswordHash())
		if err != nil || !match {
			WriteError(w, domain.Unauthorized, "invalid password", corrID)
			return
		}

		if err := mfaRepo.Delete(r.Context(), user.UserID); err != nil {
			WriteError(w, domain.Internal, "failed to delete MFA settings", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"disabled": true})
	})
}
