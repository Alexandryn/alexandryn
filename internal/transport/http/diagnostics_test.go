package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/observability"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func TestDiagnosticsHandler_AdminAccess_ReturnsSnapshot(t *testing.T) {
	reg := observability.NewRegistry()
	reg.ObserveRequest("/api/v1/library", 20*time.Millisecond)
	startTime := time.Now().Add(-100 * time.Second)

	handler := transporthttp.DiagnosticsHandler(reg, startTime, "abc1234", "2026-09-07T00:00:00Z")
	secured := transporthttp.RequireRole(domain.RoleAdmin)(handler)

	// 1. Unauthenticated request: no user in context -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics", nil)
	recUnauth := httptest.NewRecorder()
	secured.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("unauth status = %d, want 401", recUnauth.Code)
	}

	// 2. Reader request -> 403
	reqReader := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics", nil)
	reqReader = reqReader.WithContext(transporthttp.WithUser(reqReader.Context(), &transporthttp.AuthenticatedUser{
		UserID: "u1", Role: domain.RoleReader,
	}))
	recReader := httptest.NewRecorder()
	secured.ServeHTTP(recReader, reqReader)
	if recReader.Code != http.StatusForbidden {
		t.Errorf("reader status = %d, want 403", recReader.Code)
	}

	// 3. Admin request -> 200 with diagnostics payload
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics", nil)
	reqAdmin = reqAdmin.WithContext(transporthttp.WithUser(reqAdmin.Context(), &transporthttp.AuthenticatedUser{
		UserID: "admin-1", Role: domain.RoleAdmin,
	}))
	recAdmin := httptest.NewRecorder()
	secured.ServeHTTP(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusOK {
		t.Fatalf("admin status = %d, want 200", recAdmin.Code)
	}

	var resp struct {
		UptimeSeconds int64                          `json:"uptime_seconds"`
		Version       map[string]string              `json:"version"`
		Runtime       map[string]any                 `json:"runtime"`
		Metrics       observability.MetricsSnapshot `json:"metrics"`
	}
	if err := json.Unmarshal(recAdmin.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal diagnostics: %v", err)
	}

	if resp.UptimeSeconds < 99 {
		t.Errorf("uptime_seconds = %d, want >= 99", resp.UptimeSeconds)
	}
	if resp.Version["commit"] != "abc1234" {
		t.Errorf("version.commit = %q, want abc1234", resp.Version["commit"])
	}
	if resp.Runtime["goroutines"] == nil || resp.Runtime["goroutines"].(float64) < 1 {
		t.Errorf("runtime.goroutines = %v", resp.Runtime["goroutines"])
	}
	if _, ok := resp.Metrics.Latencies["/api/v1/library"]; !ok {
		t.Errorf("metrics.latencies missing /api/v1/library: %+v", resp.Metrics.Latencies)
	}
}
