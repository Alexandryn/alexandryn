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

type dummyLibraryRepo struct {
	lib *domain.Library
	err error
}

func (d *dummyLibraryRepo) FindByID(ctx context.Context, id domain.LibraryID) (*domain.Library, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.lib, nil
}

func (d *dummyLibraryRepo) FindAll(ctx context.Context) ([]*domain.Library, error) {
	return []*domain.Library{d.lib}, nil
}

func (d *dummyLibraryRepo) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.Library, error) {
	return []*domain.Library{d.lib}, nil
}

func (d *dummyLibraryRepo) Save(ctx context.Context, l *domain.Library) error {
	d.lib = l
	return nil
}

func (d *dummyLibraryRepo) Delete(ctx context.Context, id domain.LibraryID) error {
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
		}
		poolRef.SetAuthAPI(transporthttp.AuthAPI{
			Signer:    &dummyTokenSigner{claims: claims},
			Libraries: &dummyLibraryRepo{lib: libUploadOff},
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
		}
		poolRef.SetAuthAPI(transporthttp.AuthAPI{
			Signer:    &dummyTokenSigner{claims: claims},
			Libraries: &dummyLibraryRepo{lib: libUploadOn},
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
			Libraries: &dummyLibraryRepo{lib: libUploadOff},
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
			Libraries: &dummyLibraryRepo{lib: libUploadOn},
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
