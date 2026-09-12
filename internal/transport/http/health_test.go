package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type countingPinger struct{ calls int }

func (p *countingPinger) Ping(context.Context) error { p.calls++; return nil }

type fakePinger struct{ err error }

func (p *fakePinger) Ping(context.Context) error { return p.err }

// /healthz takes the same pool-reference parameter /readyz reads,
// but must never touch it — proven with a spy whose call count is
// asserted zero after a request, not merely that the response is 200.
func TestHealthz_NeverTouchesThePoolReference(t *testing.T) {
	spy := &countingPinger{}
	ref := &transporthttp.PoolRef{}
	ref.Set(spy)

	handler := transporthttp.Healthz(ref)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if spy.calls != 0 {
		t.Fatalf("Ping was called %d times, want 0 — /healthz must never touch the database dependency", spy.calls)
	}
}

func TestHealthz_ReturnsOKWithNoPoolReferenceSetAtAll(t *testing.T) {
	ref := &transporthttp.PoolRef{} // never Set — the true pre-startup state
	handler := transporthttp.Healthz(ref)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — alive is independent of ready", rec.Code)
	}
}

// /readyz: three states, table-driven, distinguished by body text.
func TestReadyz_ThreeStates(t *testing.T) {
	t.Run("reference unset", func(t *testing.T) {
		ref := &transporthttp.PoolRef{}
		handler := transporthttp.Readyz(ref)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "not yet started") {
			t.Fatalf("body doesn't name \"not yet started\": %s", rec.Body.String())
		}
	})

	t.Run("reference set, liveness query succeeds", func(t *testing.T) {
		ref := &transporthttp.PoolRef{}
		ref.Set(&fakePinger{})
		handler := transporthttp.Readyz(ref)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("reference set, liveness query fails", func(t *testing.T) {
		ref := &transporthttp.PoolRef{}
		ref.Set(&fakePinger{err: errors.New("connection reset")})
		handler := transporthttp.Readyz(ref)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "lost the connection") {
			t.Fatalf("body doesn't name \"lost the connection\": %s", rec.Body.String())
		}
	})
}

func TestReadyz_ContentTypeAndSharedErrorShape(t *testing.T) {
	ref := &transporthttp.PoolRef{}
	handler := transporthttp.Readyz(ref)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Code          string `json:"code"`
		Message       string `json:"message"`
		CorrelationID string `json:"correlationId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body isn't valid JSON: %v\n%s", err, rec.Body.String())
	}
	if body.Code != "Unavailable" {
		t.Fatalf("code = %q, want Unavailable", body.Code)
	}
}

// DSN-redaction regression guard: a liveness-query failure whose
// .Error() string contains a synthetic, obviously-fake DSN-shaped
// substring must never surface it — the response body is always the
// fixed, generic "lost the connection" message.
const fakeReadyzDSNMarker = "postgres://readyz-marker:s3cr3t@host/db"

func TestReadyz_ConnectionFailureNeverLeaksTheDSN(t *testing.T) {
	ref := &transporthttp.PoolRef{}
	ref.Set(&fakePinger{err: errors.New("dial tcp " + fakeReadyzDSNMarker + ": connection refused")})
	handler := transporthttp.Readyz(ref)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if strings.Contains(rec.Body.String(), fakeReadyzDSNMarker) {
		t.Fatalf("response body leaked the DSN: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "lost the connection") {
		t.Fatalf("body doesn't name \"lost the connection\": %s", rec.Body.String())
	}
}

func TestPoolRef_GetOnAnUnsetReferenceReturnsFalse(t *testing.T) {
	ref := &transporthttp.PoolRef{}
	if _, ok := ref.Get(); ok {
		t.Fatal("Get() on an unset PoolRef returned ok=true, want false")
	}
}

func TestPoolRef_SetThenGetReturnsTheStoredValue(t *testing.T) {
	ref := &transporthttp.PoolRef{}
	p := &fakePinger{}
	ref.Set(p)

	got, ok := ref.Get()
	if !ok {
		t.Fatal("Get() after Set() returned ok=false")
	}
	if got != p {
		t.Fatal("Get() didn't return the same value passed to Set()")
	}
}
