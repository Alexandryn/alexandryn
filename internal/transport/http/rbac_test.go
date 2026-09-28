package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func TestRBAC_AdminEndpoints_RejectUnauthenticatedAndReaders(t *testing.T) {
	adminOnly := transporthttp.RequireRole(domain.RoleAdmin)
	dummyOKHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	guarded := adminOnly(dummyOKHandler)

	tests := []struct {
		name       string
		method     string
		target     string
		user       *transporthttp.AuthenticatedUser
		wantStatus int
	}{
		{
			name:       "Unauthenticated request to admin route gets 401",
			method:     http.MethodGet,
			target:     "/api/v1/network/settings",
			user:       nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:   "Reader user to admin route gets 403",
			method: http.MethodGet,
			target: "/api/v1/network/settings",
			user: &transporthttp.AuthenticatedUser{
				UserID: "reader-1",
				Role:   domain.RoleReader,
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "Admin user to admin route succeeds with 200",
			method: http.MethodGet,
			target: "/api/v1/network/settings",
			user: &transporthttp.AuthenticatedUser{
				UserID: "admin-1",
				Role:   domain.RoleAdmin,
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)
			if tt.user != nil {
				req = req.WithContext(transporthttp.WithUser(req.Context(), tt.user))
			}
			rec := httptest.NewRecorder()
			guarded.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

// mockMemNetworkSettings implements domain.NetworkSettingsRepository for RBAC test
type mockMemNetworkSettings struct {
	settings *domain.NetworkSettings
}

func (m *mockMemNetworkSettings) Get(ctx context.Context) (*domain.NetworkSettings, error) {
	if m.settings == nil {
		return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	return m.settings, nil
}

func (m *mockMemNetworkSettings) Upsert(ctx context.Context, s *domain.NetworkSettings) error {
	m.settings = s
	return nil
}

func TestRBAC_NetworkSettings_HandlerEnforcesAdmin(t *testing.T) {
	repo := &mockMemNetworkSettings{
		settings: &domain.NetworkSettings{
			HostName:           "alexandryn.local",
			RememberDeviceDays: 30,
			UpdatedAt:          time.Now(),
		},
	}
	getHandler := transporthttp.GetNetworkSettingsHandler(repo, time.Now)
	patchHandler := transporthttp.UpdateNetworkSettingsHandler(repo, time.Now, nil)

	// GET unauthenticated -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/network/settings", nil)
	recUnauth := httptest.NewRecorder()
	getHandler.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("GET unauth status = %d, want 401", recUnauth.Code)
	}

	// GET reader -> 403
	reqReader := httptest.NewRequest(http.MethodGet, "/api/v1/network/settings", nil)
	reqReader = reqReader.WithContext(transporthttp.WithUser(reqReader.Context(), &transporthttp.AuthenticatedUser{
		UserID: "reader-1",
		Role:   domain.RoleReader,
	}))
	recReader := httptest.NewRecorder()
	getHandler.ServeHTTP(recReader, reqReader)
	if recReader.Code != http.StatusForbidden {
		t.Errorf("GET reader status = %d, want 403", recReader.Code)
	}

	// GET admin -> 200
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/network/settings", nil)
	reqAdmin = reqAdmin.WithContext(transporthttp.WithUser(reqAdmin.Context(), &transporthttp.AuthenticatedUser{
		UserID: "admin-1",
		Role:   domain.RoleAdmin,
	}))
	recAdmin := httptest.NewRecorder()
	getHandler.ServeHTTP(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusOK {
		t.Errorf("GET admin status = %d, want 200", recAdmin.Code)
	}

	// PATCH reader -> 403
	patchBody := bytes.NewReader([]byte(`{"hostName":"custom.local"}`))
	reqPatchReader := httptest.NewRequest(http.MethodPatch, "/api/v1/network/settings", patchBody)
	reqPatchReader.Header.Set("Content-Type", "application/json")
	reqPatchReader = reqPatchReader.WithContext(transporthttp.WithUser(reqPatchReader.Context(), &transporthttp.AuthenticatedUser{
		UserID: "reader-1",
		Role:   domain.RoleReader,
	}))
	recPatchReader := httptest.NewRecorder()
	patchHandler.ServeHTTP(recPatchReader, reqPatchReader)
	if recPatchReader.Code != http.StatusForbidden {
		t.Errorf("PATCH reader status = %d, want 403", recPatchReader.Code)
	}
}

func TestRBAC_Libraries_CreateAndUpdate_RequireAdmin(t *testing.T) {
	createHandler := transporthttp.CreateLibraryHandler(nil, nil, nil)
	updateHandler := transporthttp.UpdateLibraryHandler(nil)
	deleteHandler := transporthttp.DeleteLibraryHandler(nil)

	readerCtx := transporthttp.WithUser(context.Background(), &transporthttp.AuthenticatedUser{
		UserID: "reader-1",
		Role:   domain.RoleReader,
	})

	// Create by reader -> 403
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/libraries", bytes.NewReader([]byte(`{"name":"My Lib"}`))).WithContext(readerCtx)
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	createHandler.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusForbidden {
		t.Errorf("create lib by reader status = %d, want 403", recCreate.Code)
	}

	// Update by reader -> 403
	reqUpdate := httptest.NewRequest(http.MethodPatch, "/api/v1/libraries/lib-1", bytes.NewReader([]byte(`{"name":"New Name"}`))).WithContext(readerCtx)
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate.SetPathValue("id", "lib-1")
	recUpdate := httptest.NewRecorder()
	updateHandler.ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusForbidden {
		t.Errorf("update lib by reader status = %d, want 403", recUpdate.Code)
	}

	// Delete by reader -> 403
	reqDelete := httptest.NewRequest(http.MethodDelete, "/api/v1/libraries/lib-1", nil).WithContext(readerCtx)
	reqDelete.SetPathValue("id", "lib-1")
	recDelete := httptest.NewRecorder()
	deleteHandler.ServeHTTP(recDelete, reqDelete)
	if recDelete.Code != http.StatusForbidden {
		t.Errorf("delete lib by reader status = %d, want 403", recDelete.Code)
	}
}

func TestRBAC_DeviceEndpoints_AllowAuthenticatedReaders(t *testing.T) {
	// Devices endpoints are user-scoped: readers are allowed to list and revoke their own devices.
	memDevices := newMemPairedDevs()
	listHandler := transporthttp.ListDevicesHandler(memDevices)

	// Unauthenticated -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	recUnauth := httptest.NewRecorder()
	listHandler.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("devices unauth status = %d, want 401", recUnauth.Code)
	}

	// Reader -> 200 OK (empty list)
	reqReader := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqReader = reqReader.WithContext(transporthttp.WithUser(reqReader.Context(), &transporthttp.AuthenticatedUser{
		UserID: "reader-user-1",
		Role:   domain.RoleReader,
	}))
	recReader := httptest.NewRecorder()
	listHandler.ServeHTTP(recReader, reqReader)
	if recReader.Code != http.StatusOK {
		t.Fatalf("devices reader status = %d, want 200; body = %s", recReader.Code, recReader.Body.String())
	}

	var resp struct {
		Devices []any `json:"devices"`
	}
	if err := json.NewDecoder(recReader.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Devices) != 0 {
		t.Errorf("expected 0 devices for fresh reader, got %d", len(resp.Devices))
	}
}
