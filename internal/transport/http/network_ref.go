package http

import (
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

type networkRefs struct {
	networkAPI atomic.Pointer[NetworkAPI]
}

// NetworkAPI bundles dependencies for network and pairing endpoints (Phase 13).
type NetworkAPI struct {
	PairingSessions    domain.PairingSessionRepository
	PairedDevices      domain.PairedDeviceRepository
	NetworkSettings    domain.NetworkSettingsRepository
	EnrolmentGrantJTIs domain.EnrolmentGrantJTIRepository
	Verifier           PairingVerifier
	GrantSigner        *auth.EnrolmentGrantSigner
	CodeGen            func() (domain.PairingCode, error)
	PairingSecret      string
	ServerAddress      string
	HostName           string
	Scheme             string
	IDs                domain.IDGenerator
	Now                func() time.Time
	Logger             *slog.Logger
	InfoProvider       func() NetworkInfo
}

// SetNetworkAPI stores the active NetworkAPI.
func (r *PoolRef) SetNetworkAPI(a NetworkAPI) {
	r.network.networkAPI.Store(&a)
}

// GetNetworkAPI returns the active NetworkAPI if initialized.
func (r *PoolRef) GetNetworkAPI() (NetworkAPI, bool) {
	a := r.network.networkAPI.Load()
	if a == nil {
		return NetworkAPI{}, false
	}
	return *a, true
}

// LazyInitiatePairingHandler creates a handler for POST /api/v1/network/pair/initiate.
func LazyInitiatePairingHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetNetworkAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		InitiatePairingHandler(
			api.PairingSessions,
			api.CodeGen,
			api.PairingSecret,
			api.ServerAddress,
			api.Scheme,
			api.IDs,
			api.Now,
			api.Logger,
		).ServeHTTP(w, r)
	})
}

// LazyVerifyPairingHandler creates a handler for POST /api/v1/network/pair/verify.
func LazyVerifyPairingHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetNetworkAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		VerifyPairingHandler(
			api.Verifier,
			api.GrantSigner,
			api.ServerAddress,
			api.HostName,
			api.IDs,
			api.Now,
			api.Logger,
		).ServeHTTP(w, r)
	})
}

// LazyPairingQRHandler creates a handler for GET /api/v1/network/pair/{id}/qr.
func LazyPairingQRHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetNetworkAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		PairingQRHandler(
			api.PairingSessions,
			api.ServerAddress,
			api.Scheme,
			api.Logger,
		).ServeHTTP(w, r)
	})
}

// LazyNetworkStatusHandler creates a handler for GET /api/v1/network/status.
// Unlike pairing endpoints, status returns an answer even if database persistence is not ready.
func LazyNetworkStatusHandler(ref *PoolRef, fallbackProvider func() NetworkInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetNetworkAPI()
		provider := fallbackProvider
		var logger *slog.Logger
		if ok {
			if api.InfoProvider != nil {
				provider = api.InfoProvider
			}
			logger = api.Logger
		}
		NetworkStatusHandler(provider, logger).ServeHTTP(w, r)
	})
}

// LazyUpdateNetworkSettingsHandler creates a handler for PATCH /api/v1/network/settings.
func LazyUpdateNetworkSettingsHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetNetworkAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		UpdateNetworkSettingsHandler(
			api.NetworkSettings,
			api.Now,
			api.Logger,
		).ServeHTTP(w, r)
	})
}

// LazyGetNetworkSettingsHandler creates a handler for GET /api/v1/network/settings.
func LazyGetNetworkSettingsHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetNetworkAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		GetNetworkSettingsHandler(api.NetworkSettings, api.Now).ServeHTTP(w, r)
	})
}

// LazyDeletePairingHandler creates a handler for DELETE /api/v1/network/pair/{id}.
func LazyDeletePairingHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetNetworkAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		DeletePairingHandler(
			api.PairingSessions,
			api.PairedDevices,
			api.Now,
			api.Logger,
		).ServeHTTP(w, r)
	})
}
