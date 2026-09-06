package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	readerapi "github.com/Alexandryn/alexandryn/internal/reader/api"
)

type deviceContextKey struct{}

// WithDevice attaches a validated PairedDevice to the request context.
func WithDevice(ctx context.Context, d *domain.PairedDevice) context.Context {
	return context.WithValue(ctx, deviceContextKey{}, d)
}

// DeviceFromContext retrieves the validated PairedDevice from the context if present.
func DeviceFromContext(ctx context.Context) *domain.PairedDevice {
	d, _ := ctx.Value(deviceContextKey{}).(*domain.PairedDevice)
	return d
}

// SyncMiddleware validates that the requesting device is active and touches last_seen_at (FR-9).
// For /api/v1/sync/* routes, a valid device identifier is required.
// For /api/v1/devices/* routes, a device identifier is optional (allowing host web admins),
// but if provided, it must be valid and active.
func SyncMiddleware(deviceRepo domain.PairedDeviceRepository, now func() time.Time) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corrID := CorrelationIDFromContext(r.Context())
			user := UserFromContext(r.Context())
			if user == nil {
				WriteError(w, domain.Unauthorized, "unauthorized", corrID)
				return
			}

			devIDStr := strings.TrimSpace(r.Header.Get("X-Device-Id"))
			isSyncRoute := strings.HasPrefix(r.URL.Path, "/api/v1/sync/")

			if devIDStr == "" {
				if isSyncRoute {
					WriteError(w, domain.InvalidInput, "missing device identifier", corrID)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			devID := domain.DeviceID(devIDStr)
			dev, err := deviceRepo.FindByID(r.Context(), devID)
			if err != nil || dev.Owner() != user.UserID {
				WriteError(w, domain.Unauthorized, "unauthorized device", corrID)
				return
			}

			if dev.RevokedAt() != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(errorBody{
					Code:          "Unauthorized",
					Message:       "device revoked",
					CorrelationID: corrID,
				})
				return
			}

			currentTime := time.Now()
			if now != nil {
				currentTime = now()
			}

			if err := dev.Touch(currentTime); err == nil {
				_ = deviceRepo.Save(r.Context(), dev)
			}

			ctx := WithDevice(r.Context(), dev)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type DeviceResponse struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	DeviceClass  string  `json:"deviceClass"`
	EnrolledVia  string  `json:"enrolledVia"`
	CreatedAt    string  `json:"createdAt"`
	LastSeenAt   string  `json:"lastSeenAt"`
	LastSyncedAt *string `json:"lastSyncedAt"`
	RevokedAt    *string `json:"revokedAt"`
}

type ListDevicesResponse struct {
	Devices []DeviceResponse `json:"devices"`
}

// ListDevicesHandler returns the authenticated user's paired devices (FR-1).
// Scoped to the authenticated user's ID via FindByOwner.
func ListDevicesHandler(deviceRepo domain.PairedDeviceRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		devices, err := deviceRepo.FindByOwner(r.Context(), user.UserID)
		if err != nil {
			WriteError(w, domain.Internal, "failed to list devices", corrID)
			return
		}

		resp := ListDevicesResponse{
			Devices: make([]DeviceResponse, 0, len(devices)),
		}

		for _, d := range devices {
			item := DeviceResponse{
				ID:          string(d.ID()),
				Label:       d.Label(),
				DeviceClass: string(d.DeviceClass()),
				EnrolledVia: string(d.EnrolledVia()),
				CreatedAt:   d.CreatedAt().UTC().Format(time.RFC3339Nano),
				LastSeenAt:  d.LastSeenAt().UTC().Format(time.RFC3339Nano),
			}
			if t := d.LastSyncedAt(); t != nil {
				formatted := t.UTC().Format(time.RFC3339Nano)
				item.LastSyncedAt = &formatted
			}
			if t := d.RevokedAt(); t != nil {
				formatted := t.UTC().Format(time.RFC3339Nano)
				item.RevokedAt = &formatted
			}
			resp.Devices = append(resp.Devices, item)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// RevokeDeviceHandler revokes a device owned by the authenticated user (FR-2).
// Returns 404 on nonexistent device or cross-user attempt (no oracle).
// Returns 409 if device is already revoked.
func RevokeDeviceHandler(deviceRepo domain.PairedDeviceRepository, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		id := domain.DeviceID(r.PathValue("id"))
		if string(id) == "" {
			WriteError(w, domain.InvalidInput, "missing device id", corrID)
			return
		}

		dev, err := deviceRepo.FindByID(r.Context(), id)
		if err != nil || dev.Owner() != user.UserID {
			WriteError(w, domain.NotFound, "paired device not found", corrID)
			return
		}

		if dev.RevokedAt() != nil {
			WriteError(w, domain.Conflict, "paired device is already revoked", corrID)
			return
		}

		currentTime := time.Now()
		if now != nil {
			currentTime = now()
		}

		if err := deviceRepo.Revoke(r.Context(), id, currentTime); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), corrID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

type SyncProgressItem = postgres.SyncProgressItem
type SyncBookmarkItem = postgres.SyncBookmarkItem
type SyncHighlightItem = postgres.SyncHighlightItem
type ReadingSyncData = postgres.ReadingSyncData

type SyncStore interface {
	GetReadingSyncData(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, since int64) (*postgres.ReadingSyncData, error)
	GetProgressSyncSequence(ctx context.Context, progressID domain.ReadingProgressID) (int64, error)
}

// SyncReadingHandler handles GET /api/v1/sync/reading?since=<cursor> (FR-6).
// Returns incremental delta of reading data for active library and advances requesting device's cursor.
func SyncReadingHandler(store SyncStore, devRepo domain.PairedDeviceRepository, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}
		activeLibID := ActiveLibraryFromContext(r.Context())
		dev := DeviceFromContext(r.Context())
		if dev == nil {
			WriteError(w, domain.Unauthorized, "unauthorized device", corrID)
			return
		}

		var since int64
		sinceStr := r.URL.Query().Get("since")
		if sinceStr != "" {
			parsed, err := strconv.ParseInt(sinceStr, 10, 64)
			if err != nil || parsed < 0 {
				WriteError(w, domain.InvalidInput, "invalid since parameter", corrID)
				return
			}
			since = parsed
		}

		data, err := store.GetReadingSyncData(r.Context(), user.UserID, activeLibID, since)
		if err != nil {
			WriteError(w, domain.Internal, "failed to fetch sync data", corrID)
			return
		}

		if data.Progress == nil {
			data.Progress = []SyncProgressItem{}
		}
		if data.Bookmarks == nil {
			data.Bookmarks = []SyncBookmarkItem{}
		}
		if data.Highlights == nil {
			data.Highlights = []SyncHighlightItem{}
		}

		currentTime := time.Now()
		if now != nil {
			currentTime = now()
		}

		if data.Cursor > dev.SyncCursor() {
			_ = devRepo.AdvanceCursor(r.Context(), dev.ID(), data.Cursor, currentTime)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
	})
}

type SyncProgressInput struct {
	WorkID          string               `json:"workId"`
	Percentage      float64              `json:"percentage"`
	ObservedEpoch   int64                `json:"observedEpoch"`
	PrecisePosition *SyncPrecisePosition `json:"precisePosition,omitempty"`
	DeviceID        string               `json:"deviceId"`
	ReportedAt      *time.Time           `json:"reportedAt,omitempty"`
}

type SyncPrecisePosition struct {
	EditionID string `json:"editionId"`
	Value     string `json:"value"`
	CFI       string `json:"cfi"`
}

type LibraryEntryChecker interface {
	WorkInLibrary(ctx context.Context, workID domain.WorkID, libID domain.LibraryID) (bool, error)
}

// SyncProgressHandler handles POST /api/v1/sync/progress (FR-7).
// Runs ReconcileProgress under row lock, updates canonical progress if Advanced,
// and advances the device's sync cursor in the same transaction.
func SyncProgressHandler(
	progressRepo domain.ReadingProgressRepository,
	libEntries LibraryEntryChecker,
	syncStore SyncStore,
	devRepo domain.PairedDeviceRepository,
	tx readerapi.Transactor,
	ids readerapi.IDs,
	now func() time.Time,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}
		activeLibID := ActiveLibraryFromContext(r.Context())
		dev := DeviceFromContext(r.Context())
		if dev == nil {
			WriteError(w, domain.Unauthorized, "unauthorized device", corrID)
			return
		}

		// Cap body size at 64 KiB
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

		var req SyncProgressInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be valid JSON", corrID)
			return
		}

		workID := domain.WorkID(req.WorkID)
		if strings.TrimSpace(string(workID)) == "" {
			WriteError(w, domain.InvalidInput, "workId is required", corrID)
			return
		}

		if err := readerapi.ValidatePercentage(req.Percentage); err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		if err := readerapi.ValidateObservedEpoch(req.ObservedEpoch); err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		var pos *domain.PrecisePosition
		if req.PrecisePosition != nil {
			cfi := req.PrecisePosition.Value
			if cfi == "" {
				cfi = req.PrecisePosition.CFI
			}
			if err := readerapi.ValidateCFI(cfi); err != nil {
				writeDomainError(w, err, corrID)
				return
			}
			pos = &domain.PrecisePosition{
				EditionID: domain.EditionID(req.PrecisePosition.EditionID),
				Value:     cfi,
			}
		}

		// Work ownership / library membership check: returns 404 on mismatch (no oracle)
		inLib, err := libEntries.WorkInLibrary(r.Context(), workID, activeLibID)
		if err != nil || !inLib {
			WriteError(w, domain.NotFound, "work not found in your library", corrID)
			return
		}

		// DeviceID check: if specified, must match authenticated device
		if req.DeviceID != "" && domain.DeviceID(req.DeviceID) != dev.ID() {
			WriteError(w, domain.Unauthorized, "deviceId does not match authenticated device", corrID)
			return
		}

		pct, _ := domain.NewPercentage(req.Percentage)
		currentTime := time.Now()
		if now != nil {
			currentTime = now()
		}
		repTime := currentTime
		if req.ReportedAt != nil {
			repTime = *req.ReportedAt
		}

		report := domain.ProgressReport{
			WorkID:          workID,
			Percentage:      pct,
			ObservedEpoch:   req.ObservedEpoch,
			PrecisePosition: pos,
			DeviceID:        dev.ID(),
			ReportedAt:      repTime.UTC(),
		}

		var res *domain.ReadingProgress
		var outcome domain.ReconcileOutcome
		var newCursor int64 = dev.SyncCursor()

		txErr := tx.InTx(r.Context(), func(txCtx context.Context) error {
			current, err := progressRepo.FindByWorkAndUserForUpdate(txCtx, user.UserID, activeLibID, workID)
			if err != nil && domain.CategoryOf(err) == domain.NotFound {
				first := domain.FirstProgress(domain.ReadingProgressID(ids.NewID()), report)
				if serr := progressRepo.SaveForUser(txCtx, user.UserID, activeLibID, first); serr != nil {
					return serr
				}
				res, outcome = first, domain.ReconcileAdvanced
			} else if err != nil {
				return err
			} else {
				// Clamp ObservedEpoch if above stored Epoch
				if report.ObservedEpoch > current.Epoch() {
					report.ObservedEpoch = current.Epoch()
				}
				reconcileRes := domain.ReconcileProgress(current, report)
				res, outcome = reconcileRes.Progress, reconcileRes.Outcome
				if outcome == domain.ReconcileAdvanced {
					if serr := progressRepo.SaveForUser(txCtx, user.UserID, activeLibID, res); serr != nil {
						return serr
					}
				}
			}

			// Determine sequence for cursor advance
			seq, err := syncStore.GetProgressSyncSequence(txCtx, res.ID())
			if err == nil && seq > dev.SyncCursor() {
				if aerr := devRepo.AdvanceCursor(txCtx, dev.ID(), seq, currentTime); aerr == nil {
					newCursor = seq
				}
			}
			return nil
		})

		if txErr != nil {
			writeDomainError(w, txErr, corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"outcome":  string(outcome),
			"progress": progressToWire(res),
			"cursor":   newCursor,
		})
	})
}

