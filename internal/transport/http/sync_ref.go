package http

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	readerapi "github.com/Alexandryn/alexandryn/internal/reader/api"
)

type syncRefs struct {
	syncAPI atomic.Pointer[SyncAPI]
}

// SyncAPI bundles dependencies for device management and reading sync endpoints (Phase 14).
type SyncAPI struct {
	Devices        domain.PairedDeviceRepository
	Progress       domain.ReadingProgressRepository
	LibraryEntries LibraryEntryChecker
	Editions       readerapi.EditionLookup
	SyncStore      SyncStore
	Transactor     readerapi.Transactor
	IDs            readerapi.IDs
	Now            func() time.Time
}

// SetSyncAPI stores the active SyncAPI.
func (r *PoolRef) SetSyncAPI(a SyncAPI) {
	r.sync.syncAPI.Store(&a)
}

// GetSyncAPI returns the active SyncAPI if initialized.
func (r *PoolRef) GetSyncAPI() (SyncAPI, bool) {
	a := r.sync.syncAPI.Load()
	if a == nil {
		return SyncAPI{}, false
	}
	return *a, true
}

// LazySyncMiddleware wraps SyncMiddleware with lazy dependency resolution.
func LazySyncMiddleware(ref *PoolRef) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			api, ok := ref.GetSyncAPI()
			if !ok {
				WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
				return
			}
			SyncMiddleware(api.Devices, api.Now)(next).ServeHTTP(w, r)
		})
	}
}

// LazyListDevicesHandler creates a handler for GET /api/v1/devices.
func LazyListDevicesHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetSyncAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		ListDevicesHandler(api.Devices).ServeHTTP(w, r)
	})
}

// LazyRevokeDeviceHandler creates a handler for DELETE /api/v1/devices/{id}.
func LazyRevokeDeviceHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetSyncAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		RevokeDeviceHandler(api.Devices, api.Now).ServeHTTP(w, r)
	})
}

// LazySyncReadingHandler creates a handler for GET /api/v1/sync/reading.
func LazySyncReadingHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetSyncAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		SyncReadingHandler(api.SyncStore, api.Devices, api.Now).ServeHTTP(w, r)
	})
}

// LazySyncProgressHandler creates a handler for POST /api/v1/sync/progress.
func LazySyncProgressHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetSyncAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		SyncProgressHandler(
			api.Progress,
			api.LibraryEntries,
			api.Editions,
			api.SyncStore,
			api.Devices,
			api.Transactor,
			api.IDs,
			api.Now,
		).ServeHTTP(w, r)
	})
}
