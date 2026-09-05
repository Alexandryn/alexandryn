package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

type BootstrapCapabilities struct {
	Sources  bool `json:"sources"`
	Import   bool `json:"import"`
	Settings bool `json:"settings"`
	System   bool `json:"system"`
	Network  bool `json:"network"`
}

type BootstrapResponse struct {
	Capabilities BootstrapCapabilities `json:"capabilities"`
}

// LazyBootstrapHandler serves GET /api/bootstrap based on the authenticated user's role
// and the active library's allowReaderUploads setting.
func LazyBootstrapHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			if api, ok := ref.GetAuthAPI(); ok && api.Signer != nil {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token := strings.TrimPrefix(authHeader, "Bearer ")
					if claims, err := api.Signer.VerifyAccessToken(token, time.Now()); err == nil && claims != nil {
						user = &AuthenticatedUser{
							UserID:    domain.UserID(claims.Subject),
							Role:      claims.Role,
							Libraries: claims.Libraries,
						}
					}
				}
			}
		}

		isAdmin := user != nil && user.Role == domain.RoleAdmin

		allowUpload := false
		if isAdmin {
			allowUpload = true
		} else if user != nil && user.Role == domain.RoleReader {
			if api, ok := ref.GetAuthAPI(); ok && api.Libraries != nil {
				activeLibID := ActiveLibraryFromContext(r.Context())
				if activeLibID == "" || activeLibID == domain.DefaultLibraryID {
					if h := r.Header.Get("X-Library-Id"); h != "" {
						activeLibID = domain.LibraryID(h)
					}
				}
				if activeLibID != "" {
					if lib, err := api.Libraries.FindByID(r.Context(), activeLibID); err == nil && lib != nil {
						allowUpload = lib.AllowReaderUploads()
					}
				}
			}
		}

		resp := BootstrapResponse{
			Capabilities: BootstrapCapabilities{
				Sources:  isAdmin,
				Import:   allowUpload,
				Settings: isAdmin,
				System:   isAdmin,
				Network:  isAdmin,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
