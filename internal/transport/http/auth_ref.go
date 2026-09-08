package http

import (
	"log/slog"
	"sync/atomic"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

type authRefs struct {
	authAPI atomic.Pointer[AuthAPI]
}

type AuthAPI struct {
	Users              domain.UserRepository
	Credentials        domain.CredentialRepository
	RefreshTokens      domain.RefreshTokenRepository
	MFA                domain.MFARepository
	PasswordResets     domain.PasswordResetRepository
	Libraries          domain.LibraryRepository
	LibraryMemberships domain.LibraryMembershipRepository
	LibraryInvitations domain.LibraryInvitationRepository
	Hasher             auth.PasswordHasher
	Signer             auth.TokenSigner
	TOTPEngine         *auth.TOTPEngine
	Limiter            *auth.IPRateLimiter
	MFAUserLimiter     *auth.IPRateLimiter
	MasterKey          []byte
	IDs                domain.IDGenerator
	PairedDevices      domain.PairedDeviceRepository
	NetworkSettings    domain.NetworkSettingsRepository
	EnrolmentGrantJTIs domain.EnrolmentGrantJTIRepository
	EnrolmentSigner    *auth.EnrolmentGrantSigner
	Logger             *slog.Logger
}

func (r *PoolRef) SetAuthAPI(a AuthAPI) {
	r.auth.authAPI.Store(&a)
}

func (r *PoolRef) GetAuthAPI() (AuthAPI, bool) {
	a := r.auth.authAPI.Load()
	if a == nil {
		return AuthAPI{}, false
	}
	return *a, true
}
