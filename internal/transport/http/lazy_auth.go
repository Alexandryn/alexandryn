package http

import (
	"net/http"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func LazySetupStatusHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		SetupStatusHandler(api.Users).ServeHTTP(w, r)
	})
}

func LazySetupHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		SetupHandler(api.Users, api.Credentials, api.Libraries, api.LibraryMemberships, api.RefreshTokens, api.Hasher, api.Signer, api.IDs).ServeHTTP(w, r)
	})
}

func LazyLoginHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		LoginHandler(api.Users, api.Credentials, api.MFA, api.LibraryMemberships, api.RefreshTokens, api.Hasher, api.Signer, api.IDs, api.Limiter).ServeHTTP(w, r)
	})
}

func LazyRefreshHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		RefreshHandler(api.RefreshTokens, api.Users, api.LibraryMemberships, api.Signer, api.IDs, api.Limiter).ServeHTTP(w, r)
	})
}

func LazyLogoutHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		LogoutHandler(api.RefreshTokens).ServeHTTP(w, r)
	})
}

func LazyPasswordResetRequestHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		PasswordResetRequestHandler(api.Users, api.PasswordResets, api.IDs, api.Limiter).ServeHTTP(w, r)
	})
}

func LazyPasswordResetConfirmHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		PasswordResetConfirmHandler(api.Users, api.Credentials, api.PasswordResets, api.RefreshTokens, api.Hasher, api.Limiter).ServeHTTP(w, r)
	})
}

func LazyTOTPSetupHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		TOTPSetupHandler(api.MFA, api.TOTPEngine, api.MasterKey).ServeHTTP(w, r)
	})
}

func LazyTOTPConfirmHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		TOTPConfirmHandler(api.MFA, api.TOTPEngine, api.MasterKey).ServeHTTP(w, r)
	})
}

func LazyTOTPVerifyHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		TOTPVerifyHandler(api.MFA, api.Users, api.RefreshTokens, api.LibraryMemberships, api.TOTPEngine, api.Signer, api.IDs, api.MasterKey).ServeHTTP(w, r)
	})
}

func LazyTOTPDisableHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		TOTPDisableHandler(api.MFA, api.Credentials, api.Hasher).ServeHTTP(w, r)
	})
}

func LazyListLibrariesHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		ListLibrariesHandler(api.Libraries, api.LibraryMemberships).ServeHTTP(w, r)
	})
}

func LazyCreateLibraryHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		CreateLibraryHandler(api.Libraries, api.LibraryMemberships, api.IDs).ServeHTTP(w, r)
	})
}

func LazyGetLibraryHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		GetLibraryHandler(api.Libraries).ServeHTTP(w, r)
	})
}

func LazyUpdateLibraryHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		UpdateLibraryHandler(api.Libraries).ServeHTTP(w, r)
	})
}

func LazyDeleteLibraryHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		DeleteLibraryHandler(api.Libraries).ServeHTTP(w, r)
	})
}

func LazyListMembersHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		ListMembersHandler(api.LibraryMemberships, api.Users).ServeHTTP(w, r)
	})
}

func LazyCreateInvitationHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		CreateInvitationHandler(api.LibraryInvitations, api.IDs).ServeHTTP(w, r)
	})
}

func LazyAcceptInvitationHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api, ok := ref.GetAuthAPI()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		AcceptInvitationHandler(api.LibraryInvitations, api.LibraryMemberships, api.IDs).ServeHTTP(w, r)
	})
}

func LazyAuthMiddleware(ref *PoolRef) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			api, ok := ref.GetAuthAPI()
			if !ok {
				WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
				return
			}

			AuthMiddleware(api.Signer)(next).ServeHTTP(w, r)
		})
	}
}

