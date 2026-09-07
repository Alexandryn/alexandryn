package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func TestActivityEndpoints_RoleGating(t *testing.T) {
	adminUser := &transporthttp.AuthenticatedUser{UserID: "admin-1", Role: domain.RoleAdmin}
	readerUser := &transporthttp.AuthenticatedUser{UserID: "reader-1", Role: domain.RoleReader}

	tests := []struct {
		name       string
		method     string
		path       string
		handler    http.Handler
		wantStatus int
	}{
		{
			name:       "events reader 403",
			method:     http.MethodGet,
			path:       "/api/v1/activity/events",
			handler:    transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityEventsHandler(nil)),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "pause-all reader 403",
			method:     http.MethodPost,
			path:       "/api/v1/activity/pause-all",
			handler:    transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityPauseAllHandler(nil)),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "cancel reader 403",
			method:     http.MethodPost,
			path:       "/api/v1/activity/jobs/j1/cancel",
			handler:    transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityJobCancelHandler(nil)),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "retry reader 403",
			method:     http.MethodPost,
			path:       "/api/v1/activity/jobs/j1/retry",
			handler:    transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityJobRetryHandler(nil)),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "clear reader 403",
			method:     http.MethodPost,
			path:       "/api/v1/activity/jobs/clear-completed",
			handler:    transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityClearCompletedHandler(nil, nil)),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req = req.WithContext(transporthttp.WithUser(req.Context(), readerUser))
			rec := httptest.NewRecorder()
			tc.handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}

	// Verify unauthenticated requests receive 401
	t.Run("unauthenticated receives 401", func(t *testing.T) {
		h := transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityEventsHandler(nil))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/activity/events", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unauth status = %d, want 401", rec.Code)
		}
	})

	_ = adminUser
}

func TestActivityPauseAll_Success(t *testing.T) {
	h := transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityPauseAllHandler(nil))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activity/pause-all", nil)
	req = req.WithContext(transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
		UserID: "admin-1", Role: domain.RoleAdmin,
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Paused bool `json:"paused"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Paused {
		t.Errorf("paused = false, want true")
	}
}

func TestActivityHandlers_NilDependencyReturns503(t *testing.T) {
	adminCtx := transporthttp.WithUser(context.Background(), &transporthttp.AuthenticatedUser{
		UserID: "admin-1", Role: domain.RoleAdmin,
	})

	t.Run("events nil store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/activity/events", nil).WithContext(adminCtx)
		rec := httptest.NewRecorder()
		transporthttp.ActivityEventsHandler(nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rec.Code)
		}
	})

	t.Run("cancel nil queue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/activity/jobs/j1/cancel", nil).WithContext(adminCtx)
		req.SetPathValue("id", "j1")
		rec := httptest.NewRecorder()
		transporthttp.ActivityJobCancelHandler(nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rec.Code)
		}
	})

	t.Run("retry nil queue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/activity/jobs/j1/retry", nil).WithContext(adminCtx)
		req.SetPathValue("id", "j1")
		rec := httptest.NewRecorder()
		transporthttp.ActivityJobRetryHandler(nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rec.Code)
		}
	})

	t.Run("clear nil queue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/activity/jobs/clear-completed", nil).WithContext(adminCtx)
		rec := httptest.NewRecorder()
		transporthttp.ActivityClearCompletedHandler(nil, nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rec.Code)
		}
	})
}
