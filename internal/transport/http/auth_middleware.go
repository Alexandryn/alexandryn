package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

type userContextKey struct{}
type activeLibraryContextKey struct{}

type AuthenticatedUser struct {
	UserID    domain.UserID
	Username  string
	Role      domain.Role
	Libraries []domain.LibraryID
}

func WithUser(ctx context.Context, u *AuthenticatedUser) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

func UserFromContext(ctx context.Context) *AuthenticatedUser {
	u, _ := ctx.Value(userContextKey{}).(*AuthenticatedUser)
	return u
}

func WithActiveLibrary(ctx context.Context, libID domain.LibraryID) context.Context {
	return context.WithValue(ctx, activeLibraryContextKey{}, libID)
}

func ActiveLibraryFromContext(ctx context.Context) domain.LibraryID {
	libID, ok := ctx.Value(activeLibraryContextKey{}).(domain.LibraryID)
	if !ok || libID == "" {
		return domain.DefaultLibraryID
	}
	return libID
}

// writeForbidden writes a 403 with the shared error-body shape. The domain
// error taxonomy has no Forbidden category — 403 is an authorization outcome
// the transport layer owns, so it is written directly rather than mapped from
// a domain error.
func writeForbidden(w http.ResponseWriter, message, corrID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(errorBody{
		Code:          "Forbidden",
		Message:       message,
		CorrelationID: corrID,
	})
}

// libraryInClaims reports whether libID is one of the libraries the token
// grants access to.
func libraryInClaims(libID domain.LibraryID, claimed []domain.LibraryID) bool {
	for _, c := range claimed {
		if c == libID {
			return true
		}
	}
	return false
}

// IsPublicPath checks if a request path does not require authentication.
func IsPublicPath(path string) bool {
	if path == "/healthz" || path == "/readyz" {
		return true
	}
	publicAuthPaths := []string{
		"/api/v1/auth/setup/status",
		"/api/v1/auth/setup",
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/auth/logout",
		"/api/v1/auth/password-reset/request",
		"/api/v1/auth/password-reset/confirm",
		"/api/v1/auth/mfa/totp/verify",
		"/api/v1/network/pair/verify",
	}
	for _, p := range publicAuthPaths {
		if path == p {
			return true
		}
	}
	// Non-API routes are frontend static routes
	if !strings.HasPrefix(path, "/api/v1/") {
		return true
	}
	return false
}

// AuthMiddleware validates JWT Bearer tokens on all protected routes.
func AuthMiddleware(signer auth.TokenSigner) Middleware {
	return AuthMiddlewareWithReaderGrants(signer, nil)
}

// AuthMiddlewareWithReaderGrants is AuthMiddleware that additionally
// accepts a reader-content grant cookie — and only on a GET/HEAD of the
// reader content route, only for the edition the grant names, and only
// when the request carries no Authorization header. A nil grants signer
// disables the cookie path entirely.
func AuthMiddlewareWithReaderGrants(signer auth.TokenSigner, grants *auth.ReaderContentGrantSigner) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corrID := CorrelationIDFromContext(r.Context())

			if IsPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" && grants != nil {
				if ctx, ok := readerContentGrantContext(r, grants); ok {
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				WriteError(w, domain.Unauthorized, "missing or invalid authorization token", corrID)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			// VerifyAccessToken asserts the token type — a signature-valid
			// MFA ticket or pairing enrolment grant is rejected here.
			claims, err := signer.VerifyAccessToken(tokenString, time.Now())
			if err != nil {
				WriteError(w, domain.Unauthorized, "invalid or expired token", corrID)
				return
			}

			user := &AuthenticatedUser{
				UserID:    claims.Subject,
				Username:  claims.Username,
				Role:      claims.Role,
				Libraries: claims.Libraries,
			}

			// Active library resolution from the X-Library-Id header. A
			// header naming a library the token does not grant is rejected
			// (403) — it is not silently used, and it does not fall back to
			// a default.
			// If claims.Libraries is empty, reject protected routes with 403
			// rather than defaulting to DefaultLibraryID without a membership
			// check. Listing libraries (GET /api/v1/libraries)
			// is permitted so the user can discover available memberships.
			activeLibID := domain.LibraryID(r.Header.Get("X-Library-Id"))
			if activeLibID == "" {
				if len(claims.Libraries) > 0 {
					activeLibID = claims.Libraries[0]
				} else if r.URL.Path == "/api/v1/libraries" && r.Method == http.MethodGet {
					activeLibID = ""
				} else {
					writeForbidden(w, "you are not a member of that library", corrID)
					return
				}
			} else if !libraryInClaims(activeLibID, claims.Libraries) {
				writeForbidden(w, "you are not a member of that library", corrID)
				return
			}

			ctx := WithUser(r.Context(), user)
			ctx = WithActiveLibrary(ctx, activeLibID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// readerContentGrantContext authenticates a reader-content request by
// its grant cookie: the sandboxed reader <iframe> — and every image and
// stylesheet its chapter loads by relative URL — is a plain browser fetch
// that cannot carry an Authorization header. The grant is honoured only on
// GET/HEAD of /api/v1/library/editions/{editionId}/reader/content/…, only
// when its edition matches the path's, and it authenticates a user with
// exactly the one library the grant names and no role. Any mismatch
// returns false and the caller falls through to the Bearer check (a 401).
func readerContentGrantContext(r *http.Request, grants *auth.ReaderContentGrantSigner) (context.Context, bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return nil, false
	}
	editionID, ok := readerContentEditionFromPath(r.URL.Path)
	if !ok {
		return nil, false
	}
	cookie, err := r.Cookie(ReaderContentCookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	claims, err := grants.Verify(cookie.Value, time.Now())
	if err != nil || claims.EditionID != editionID {
		return nil, false
	}
	ctx := WithUser(r.Context(), &AuthenticatedUser{
		UserID:    claims.Subject,
		Libraries: []domain.LibraryID{claims.LibraryID},
	})
	return WithActiveLibrary(ctx, claims.LibraryID), true
}

// readerContentEditionFromPath extracts {editionId} from a
// /api/v1/library/editions/{editionId}/reader/content/{path...} request
// path, reporting false for any other path.
func readerContentEditionFromPath(path string) (domain.EditionID, bool) {
	rest, ok := strings.CutPrefix(path, readerEditionsPrefix)
	if !ok {
		return "", false
	}
	id, tail, ok := strings.Cut(rest, "/")
	if !ok || id == "" || !strings.HasPrefix(tail, "reader/content/") {
		return "", false
	}
	return domain.EditionID(id), true
}

// RequireRole ensures the authenticated user has at least one of the allowed roles.
func RequireRole(roles ...domain.Role) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corrID := CorrelationIDFromContext(r.Context())
			user := UserFromContext(r.Context())
			if user == nil {
				WriteError(w, domain.Unauthorized, "unauthorized", corrID)
				return
			}

			for _, allowed := range roles {
				if user.Role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeForbidden(w, "insufficient permissions for this resource", corrID)
		})
	}
}

// RequireIngestPermission ensures the user has admin role or the active library allows reader uploads.
func RequireIngestPermission(libRepo domain.LibraryRepository) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corrID := CorrelationIDFromContext(r.Context())
			user := UserFromContext(r.Context())
			if user == nil {
				WriteError(w, domain.Unauthorized, "unauthorized", corrID)
				return
			}

			if user.Role == domain.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			activeLibID := ActiveLibraryFromContext(r.Context())
			lib, err := libRepo.FindByID(r.Context(), activeLibID)
			if err != nil || !lib.AllowReaderUploads() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(errorBody{
					Code:          "Forbidden",
					Message:       "reader uploads are disabled for this library",
					CorrelationID: corrID,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
