package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func TestOriginValidation(t *testing.T) {
	allowed := []string{"http://192.168.1.24:8474", "http://alexandryn.local:8474", "https://books.example.com"}
	mw := transporthttp.OriginValidation(allowed)

	mk := func(origin, referer string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/network/pair/verify", strings.NewReader(`{"code":"ABCD-2345"}`))
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if referer != "" {
			r.Header.Set("Referer", referer)
		}
		return r
	}

	// A multi-address LAN deployment: both the interface IP and the
	// .local name must pass.
	for _, o := range []string{"http://192.168.1.24:8474", "http://alexandryn.local:8474"} {
		if rec := serve(mw, mk(o, "")); rec.Code != http.StatusOK {
			t.Errorf("allowed origin %q got %d", o, rec.Code)
		}
	}

	// A foreign origin -> 403.
	if rec := serve(mw, mk("https://evil.example", "")); rec.Code != http.StatusForbidden {
		t.Errorf("foreign origin got %d, want 403", rec.Code)
	}

	// No Origin header -> allowed through (curl / native client).
	if rec := serve(mw, mk("", "")); rec.Code != http.StatusOK {
		t.Errorf("absent Origin got %d, want 200", rec.Code)
	}

	// No Origin, but a present-and-foreign Referer on a body request -> 403.
	if rec := serve(mw, mk("", "https://evil.example/page")); rec.Code != http.StatusForbidden {
		t.Errorf("foreign Referer got %d, want 403", rec.Code)
	}

	// No Origin, matching Referer -> allowed.
	if rec := serve(mw, mk("", "http://192.168.1.24:8474/connect")); rec.Code != http.StatusOK {
		t.Errorf("matching Referer got %d, want 200", rec.Code)
	}

	// No Origin, matching Referer but with a mixed-case host -> still
	// allowed. `allowed` is documented as already lower-cased (the
	// canonical form CORS/config.parseOriginList produce); a mixed-case
	// Referer host from a real client must not be spuriously rejected.
	if rec := serve(mw, mk("", "http://Alexandryn.Local:8474/connect")); rec.Code != http.StatusOK {
		t.Errorf("mixed-case-host matching Referer got %d, want 200", rec.Code)
	}
}
