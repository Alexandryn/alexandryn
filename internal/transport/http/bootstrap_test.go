package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// dummyLibraryRepo looks up by the requested id (a NotFound test double that
// ignored id and always returned the same library would hide any bug where
// a caller queries the wrong library — see
// TestLazyBootstrapHandler_Reader_CrossLibraryHeaderIgnored).
type dummyLibraryRepo struct {
	libs map[domain.LibraryID]*domain.Library
	err  error
}

func newDummyLibraryRepo(libs ...*domain.Library) *dummyLibraryRepo {
	m := make(map[domain.LibraryID]*domain.Library, len(libs))
	for _, l := range libs {
		m[l.ID()] = l
	}
	return &dummyLibraryRepo{libs: m}
}

func (d *dummyLibraryRepo) FindByID(ctx context.Context, id domain.LibraryID) (*domain.Library, error) {
	if d.err != nil {
		return nil, d.err
	}
	if l, ok := d.libs[id]; ok {
		return l, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "library not found"}
}

func (d *dummyLibraryRepo) FindAll(ctx context.Context) ([]*domain.Library, error) {
	var out []*domain.Library
	for _, l := range d.libs {
		out = append(out, l)
	}
	return out, nil
}

func (d *dummyLibraryRepo) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.Library, error) {
	return d.FindAll(ctx)
}

func (d *dummyLibraryRepo) Save(ctx context.Context, l *domain.Library) error {
	d.libs[l.ID()] = l
	return nil
}

func (d *dummyLibraryRepo) Delete(ctx context.Context, id domain.LibraryID) error {
	delete(d.libs, id)
	return nil
}

func TestLazyBootstrapHandler_Anonymous(t *testing.T) {
	poolRef := &transporthttp.PoolRef{}
	handler := transporthttp.LazyBootstrapHandler(poolRef)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp transporthttp.BootstrapResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Capabilities.Sources || resp.Capabilities.Import || resp.Capabilities.Settings || resp.Capabilities.System || resp.Capabilities.Network {
		t.Fatalf("expected all capabilities false for anonymous, got %+v", resp.Capabilities)
	}
}

func TestLazyBootstrapHandler_Admin(t *testing.T) {
	poolRef := &transporthttp.PoolRef{}
	claims := &auth.Claims{
		Subject:   "admin-1",
		Role:      domain.RoleAdmin,
		Libraries: []domain.LibraryID{domain.DefaultLibraryID},
		Type:      auth.TokenTypeAccess,
	}
	signer := &dummyTokenSigner{claims: claims}
	poolRef.SetAuthAPI(transporthttp.AuthAPI{
		Signer: signer,
	})

	handler := transporthttp.LazyBootstrapHandler(poolRef)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp transporthttp.BootstrapResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Capabilities.Sources || !resp.Capabilities.Import || !resp.Capabilities.Settings || !resp.Capabilities.System || !resp.Capabilities.Network {
		t.Fatalf("expected all capabilities true for admin, got %+v", resp.Capabilities)
	}
}

func TestLazyBootstrapHandler_Reader(t *testing.T) {
	libUploadOff, err := domain.NewLibrary("lib-1", "Library 1", "", false, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	libUploadOn, err := domain.NewLibrary("lib-2", "Library 2", "", true, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}

	t.Run("reader with upload off", func(t *testing.T) {
		poolRef := &transporthttp.PoolRef{}
		claims := &auth.Claims{
			Subject:   "reader-1",
			Role:      domain.RoleReader,
			Libraries: []domain.LibraryID{"lib-1"},
			Type:      auth.TokenTypeAccess,
		}
		poolRef.SetAuthAPI(transporthttp.AuthAPI{
			Signer:    &dummyTokenSigner{claims: claims},
			Libraries: newDummyLibraryRepo(libUploadOff),
		})

		handler := transporthttp.LazyBootstrapHandler(poolRef)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("X-Library-Id", "lib-1")
		handler.ServeHTTP(rec, req)

		var resp transporthttp.BootstrapResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp.Capabilities.Sources || resp.Capabilities.Import || resp.Capabilities.Settings || resp.Capabilities.System || resp.Capabilities.Network {
			t.Fatalf("expected all false, got %+v", resp.Capabilities)
		}
	})

	t.Run("reader with upload on", func(t *testing.T) {
		poolRef := &transporthttp.PoolRef{}
		claims := &auth.Claims{
			Subject:   "reader-1",
			Role:      domain.RoleReader,
			Libraries: []domain.LibraryID{"lib-2"},
			Type:      auth.TokenTypeAccess,
		}
		poolRef.SetAuthAPI(transporthttp.AuthAPI{
			Signer:    &dummyTokenSigner{claims: claims},
			Libraries: newDummyLibraryRepo(libUploadOn),
		})

		handler := transporthttp.LazyBootstrapHandler(poolRef)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("X-Library-Id", "lib-2")
		handler.ServeHTTP(rec, req)

		var resp transporthttp.BootstrapResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if !resp.Capabilities.Import {
			t.Fatal("expected import=true for reader with upload enabled")
		}
		if resp.Capabilities.Sources || resp.Capabilities.Settings || resp.Capabilities.System || resp.Capabilities.Network {
			t.Fatalf("expected host-only capabilities false for reader, got %+v", resp.Capabilities)
		}
	})
}

// TestLazyBootstrapHandler_Reader_CrossLibraryHeaderIgnored reproduces the
// bug where GET /api/bootstrap trusted a raw X-Library-Id header with no
// membership check. This route is on IsPublicPath (it must work
// pre-authentication for anonymous capability probing), so AuthMiddleware's
// own libraryInClaims check never runs for it — the handler has to do its
// own. A reader who is only a member of lib-1 (uploads off) must not be
// able to see lib-2's (uploads on) capability by naming it in the header.
func TestLazyBootstrapHandler_Reader_CrossLibraryHeaderIgnored(t *testing.T) {
	libUploadOff, err := domain.NewLibrary("lib-1", "Library 1", "", false, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	libUploadOn, err := domain.NewLibrary("lib-2", "Library 2", "", true, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}

	poolRef := &transporthttp.PoolRef{}
	claims := &auth.Claims{
		Subject:   "reader-1",
		Role:      domain.RoleReader,
		Libraries: []domain.LibraryID{"lib-1"}, // NOT a member of lib-2
		Type:      auth.TokenTypeAccess,
	}
	poolRef.SetAuthAPI(transporthttp.AuthAPI{
		Signer:    &dummyTokenSigner{claims: claims},
		Libraries: newDummyLibraryRepo(libUploadOff, libUploadOn),
	})

	handler := transporthttp.LazyBootstrapHandler(poolRef)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Library-Id", "lib-2") // library the reader is not a member of

	handler.ServeHTTP(rec, req)

	var resp transporthttp.BootstrapResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Capabilities.Import {
		t.Fatal("expected import=false: lib-2's AllowReaderUploads must not be disclosed to a non-member via an unchecked X-Library-Id header")
	}
}

func TestLazyRequireIngestPermission(t *testing.T) {
	libUploadOff, err := domain.NewLibrary("lib-1", "Library 1", "", false, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	libUploadOn, err := domain.NewLibrary("lib-2", "Library 2", "", true, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}

	target := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("unauthenticated fails 401", func(t *testing.T) {
		poolRef := &transporthttp.PoolRef{}
		guard := transporthttp.LazyRequireIngestPermission(poolRef)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/discover", nil)
		guard(target).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("admin passes without checking library", func(t *testing.T) {
		poolRef := &transporthttp.PoolRef{}
		guard := transporthttp.LazyRequireIngestPermission(poolRef)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/discover", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
			UserID: "admin-1",
			Role:   domain.RoleAdmin,
		}))
		guard(target).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("reader with upload off fails 403", func(t *testing.T) {
		poolRef := &transporthttp.PoolRef{}
		poolRef.SetAuthAPI(transporthttp.AuthAPI{
			Libraries: newDummyLibraryRepo(libUploadOff),
		})
		guard := transporthttp.LazyRequireIngestPermission(poolRef)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/discover", nil)
		ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
			UserID: "reader-1",
			Role:   domain.RoleReader,
		})
		ctx = transporthttp.WithActiveLibrary(ctx, "lib-1")
		req = req.WithContext(ctx)
		guard(target).ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("reader with upload on passes 200", func(t *testing.T) {
		poolRef := &transporthttp.PoolRef{}
		poolRef.SetAuthAPI(transporthttp.AuthAPI{
			Libraries: newDummyLibraryRepo(libUploadOn),
		})
		guard := transporthttp.LazyRequireIngestPermission(poolRef)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/discover", nil)
		ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
			UserID: "reader-1",
			Role:   domain.RoleReader,
		})
		ctx = transporthttp.WithActiveLibrary(ctx, "lib-2")
		req = req.WithContext(ctx)
		guard(target).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}
