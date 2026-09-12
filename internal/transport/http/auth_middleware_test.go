package http_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type dummyTokenSigner struct {
	claims *auth.Claims
	err    error
}

func (d *dummyTokenSigner) Sign(claims auth.Claims) (string, error) {
	return "signed-token", nil
}

func (d *dummyTokenSigner) Verify(tokenString string, now time.Time) (*auth.Claims, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.claims, nil
}

func (d *dummyTokenSigner) VerifyAccessToken(tokenString string, now time.Time) (*auth.Claims, error) {
	c, err := d.Verify(tokenString, now)
	if err != nil {
		return nil, err
	}
	if c.Type != auth.TokenTypeAccess {
		return nil, errors.New("not an access token")
	}
	return c, nil
}

func (d *dummyTokenSigner) SignMFATicket(userID domain.UserID, expiresAt time.Time) (string, error) {
	return "mfa-ticket", nil
}

func (d *dummyTokenSigner) VerifyMFATicket(ticketString string, now time.Time) (domain.UserID, string, error) {
	return "u-1", "jti-1", nil
}

func TestAuthMiddleware(t *testing.T) {
	now := time.Now()
	claims := &auth.Claims{
		Subject:   "u-1",
		Username:  "alex",
		Role:      domain.RoleAdmin,
		Libraries: []domain.LibraryID{domain.DefaultLibraryID},
		Type:      auth.TokenTypeAccess,
		ExpiresAt: now.Add(time.Hour).Unix(),
	}

	signer := &dummyTokenSigner{claims: claims}
	authMW := transporthttp.AuthMiddleware(signer)

	nextCalled := false
	handler := authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if r.URL.Path != "/healthz" {
			u := transporthttp.UserFromContext(r.Context())
			if u == nil || u.UserID != "u-1" {
				t.Errorf("expected user u-1 in context, got %+v", u)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("public endpoint bypasses auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		rec := httptest.NewRecorder()
		nextCalled = false

		handler.ServeHTTP(rec, req)
		if !nextCalled {
			t.Error("expected handler to be called for /healthz")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("protected endpoint without token fails 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/library", nil)
		rec := httptest.NewRecorder()
		nextCalled = false

		handler.ServeHTTP(rec, req)
		if nextCalled {
			t.Error("expected next handler NOT to be called")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("protected endpoint with valid token succeeds", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/library", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		nextCalled = false

		handler.ServeHTTP(rec, req)
		if !nextCalled {
			t.Error("expected handler to be called")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("X-Library-Id in the token's claims is accepted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/library", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("X-Library-Id", string(domain.DefaultLibraryID))
		rec := httptest.NewRecorder()
		nextCalled = false

		handler.ServeHTTP(rec, req)
		if !nextCalled || rec.Code != http.StatusOK {
			t.Errorf("expected 200 with next called, got %d nextCalled=%v", rec.Code, nextCalled)
		}
	})

	t.Run("X-Library-Id not in the token's claims is rejected 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/library", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("X-Library-Id", "lib-the-user-does-not-belong-to")
		rec := httptest.NewRecorder()
		nextCalled = false

		handler.ServeHTTP(rec, req)
		if nextCalled {
			t.Error("expected next handler NOT to be called for a foreign library")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})
}

func TestAuthMiddleware_RejectsNonAccessTokenTypes(t *testing.T) {
	now := time.Now()
	// A signature-valid token minted as an MFA ticket must not authenticate the access path.
	signer := &dummyTokenSigner{claims: &auth.Claims{
		Subject:   "u-1",
		Role:      domain.RoleAdmin,
		Libraries: []domain.LibraryID{domain.DefaultLibraryID},
		ExpiresAt: now.Add(time.Hour).Unix(),
		Type:      auth.TokenTypeMFATicket,
	}}
	nextCalled := false
	handler := transporthttp.AuthMiddleware(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/library", nil)
	req.Header.Set("Authorization", "Bearer an-mfa-ticket")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if nextCalled {
		t.Error("expected an MFA ticket to be rejected on the access path")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole(t *testing.T) {
	adminUser := &transporthttp.AuthenticatedUser{
		UserID: "u-1",
		Role:   domain.RoleAdmin,
	}
	readerUser := &transporthttp.AuthenticatedUser{
		UserID: "u-2",
		Role:   domain.RoleReader,
	}

	guard := transporthttp.RequireRole(domain.RoleAdmin)
	target := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("admin user passes admin guard", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/sources", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser))
		rec := httptest.NewRecorder()

		target.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("reader user fails admin guard with 403", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/sources", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), readerUser))
		rec := httptest.NewRecorder()

		target.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", rec.Code)
		}
	})
}

func TestAuthMiddleware_EmptyLibraryClaimsRejectsProtectedRoutes(t *testing.T) {
	now := time.Now()
	signer := &dummyTokenSigner{claims: &auth.Claims{
		Subject:   "u-1",
		Role:      domain.RoleReader,
		Libraries: []domain.LibraryID{}, // No library claims
		ExpiresAt: now.Add(time.Hour).Unix(),
		Type:      auth.TokenTypeAccess,
	}}

	nextCalled := false
	handler := transporthttp.AuthMiddleware(signer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("protected route fails 403 when user has zero libraries (#246)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/library", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		nextCalled = false

		handler.ServeHTTP(rec, req)
		if nextCalled {
			t.Error("expected handler not to be called")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("library listing route succeeds when user has zero libraries (#246)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/libraries", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		nextCalled = false

		handler.ServeHTTP(rec, req)
		if !nextCalled {
			t.Error("expected handler to be called for /api/v1/libraries")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}
