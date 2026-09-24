package http_test

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

const ownedContentPath = "/api/v1/library/editions/edition-owned/reader/content/OEBPS/c1.xhtml"

// readerSessionServer wires the session and content routes behind
// LazyAuthMiddleware reading the AuthAPI off the PoolRef — the production
// path — so a regression in that wiring fails here too: a Bearer token
// is accepted anywhere, the grant cookie only where the middleware allows.
func readerSessionServer(t *testing.T, grants *auth.ReaderContentGrantSigner, now func() time.Time) http.Handler {
	t.Helper()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetReaderContentCache(content.NewCache(func(ctx context.Context, editionID domain.EditionID) (*zip.Reader, func(), error) {
		return zipWith(t, map[string]string{
			"OEBPS/c1.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Alice</p></body></html>`,
			"OEBPS/style.css":   `p { color: black; }`,
			"OEBPS/img/fig.png": "\x89PNG\r\n\x1a\n",
		}), func() {}, nil
	}))
	poolRef.SetReadingAPI(transporthttp.ReadingAPI{
		LibraryEntries: &memLibraryEntries{inLib: map[domain.EditionID]domain.LibraryID{
			"edition-owned": domain.DefaultLibraryID,
			"edition-other": domain.DefaultLibraryID,
		}},
	})
	poolRef.SetAuthAPI(transporthttp.AuthAPI{
		Signer: &dummyTokenSigner{claims: &auth.Claims{
			Subject:   "u-1",
			Libraries: []domain.LibraryID{domain.DefaultLibraryID},
			Type:      auth.TokenTypeAccess,
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}},
		ReaderGrants: grants,
	})

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/library/editions/{editionId}/reader/content/{path...}", transporthttp.ReaderContentHandler(poolRef, nil))
	mux.Handle("POST /api/v1/library/editions/{editionId}/reader/session", transporthttp.ReaderSessionHandler(poolRef, now))
	mux.Handle("GET /api/v1/reading/preferences", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	return transporthttp.Chain(mux,
		transporthttp.Recovery(nil, func() string { return "test" }),
		transporthttp.LazyAuthMiddleware(poolRef),
	)
}

func issueGrantCookie(t *testing.T, srv http.Handler, editionID string) *http.Cookie {
	t.Helper()
	return issueGrantCookieWith(t, srv, editionID, nil)
}

func issueGrantCookieWith(t *testing.T, srv http.Handler, editionID string, prep func(*http.Request)) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/editions/"+editionID+"/reader/session", nil)
	req.Header.Set("Authorization", "Bearer access")
	if prep != nil {
		prep(req)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("session status = %d, body %s", rr.Code, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("want exactly one cookie, got %d", len(cookies))
	}
	return cookies[0]
}

func getWithCookie(srv http.Handler, method, path string, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if c != nil {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestReaderSession_SetsScopedCookie(t *testing.T) {
	grants := auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn")
	srv := readerSessionServer(t, grants, time.Now)

	c := issueGrantCookie(t, srv, "edition-owned")
	if c.Name != transporthttp.ReaderContentCookieName {
		t.Errorf("cookie name = %q", c.Name)
	}
	if c.Path != "/api/v1/library/editions/edition-owned/reader/content/" {
		t.Errorf("cookie path = %q, want the edition's content route only", c.Path)
	}
	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", c.SameSite)
	}
	if c.MaxAge != int(auth.ReaderContentGrantTTL/time.Second) {
		t.Errorf("MaxAge = %d", c.MaxAge)
	}
	claims, err := grants.Verify(c.Value, time.Now())
	if err != nil {
		t.Fatalf("cookie value is not a valid grant: %v", err)
	}
	if claims.Subject != "u-1" || claims.LibraryID != domain.DefaultLibraryID || claims.EditionID != "edition-owned" {
		t.Errorf("grant claims = %+v", claims)
	}
}

func TestReaderSession_RequiresBearer(t *testing.T) {
	srv := readerSessionServer(t, auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn"), time.Now)
	c := issueGrantCookie(t, srv, "edition-owned")

	// A grant cookie cannot mint its own successor.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/editions/edition-owned/reader/session", nil)
	req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("session via cookie status = %d, want 401", rr.Code)
	}
}

func TestReaderSession_EditionNotInLibrary(t *testing.T) {
	srv := readerSessionServer(t, auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn"), time.Now)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/editions/edition-not-mine/reader/session", nil)
	req.Header.Set("Authorization", "Bearer access")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if len(rr.Result().Cookies()) != 0 {
		t.Fatal("a grant cookie was set for an edition outside the library")
	}
}

func TestReaderContent_GrantCookieAuthenticates(t *testing.T) {
	srv := readerSessionServer(t, auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn"), time.Now)
	c := issueGrantCookie(t, srv, "edition-owned")

	// The chapter document and the subresources it loads by relative URL.
	for _, p := range []string{
		ownedContentPath,
		"/api/v1/library/editions/edition-owned/reader/content/OEBPS/style.css",
		"/api/v1/library/editions/edition-owned/reader/content/OEBPS/img/fig.png",
	} {
		if rr := getWithCookie(srv, http.MethodGet, p, c); rr.Code != http.StatusOK {
			t.Errorf("GET %s with grant cookie = %d, body %s", p, rr.Code, rr.Body.String())
		}
	}
	if rr := getWithCookie(srv, http.MethodGet, ownedContentPath, c); !strings.Contains(rr.Body.String(), "Alice") {
		t.Errorf("chapter body missing: %s", rr.Body.String())
	}
}

func TestReaderContent_GrantCookieRejections(t *testing.T) {
	grants := auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn")
	srv := readerSessionServer(t, grants, time.Now)
	c := issueGrantCookie(t, srv, "edition-owned")

	t.Run("no credential", func(t *testing.T) {
		if rr := getWithCookie(srv, http.MethodGet, ownedContentPath, nil); rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("other edition", func(t *testing.T) {
		rr := getWithCookie(srv, http.MethodGet, "/api/v1/library/editions/edition-other/reader/content/OEBPS/c1.xhtml", c)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("other route", func(t *testing.T) {
		if rr := getWithCookie(srv, http.MethodGet, "/api/v1/reading/preferences", c); rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("non-GET method", func(t *testing.T) {
		if rr := getWithCookie(srv, http.MethodPost, ownedContentPath, c); rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("expired", func(t *testing.T) {
		past := func() time.Time { return time.Now().Add(-auth.ReaderContentGrantTTL - time.Minute) }
		old := issueGrantCookie(t, readerSessionServer(t, grants, past), "edition-owned")
		if rr := getWithCookie(srv, http.MethodGet, ownedContentPath, old); rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("forged under another key", func(t *testing.T) {
		forged, _, err := auth.NewReaderContentGrantSigner([]byte("other"), "alexandryn").Sign("u-1", domain.DefaultLibraryID, "edition-owned", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		rr := getWithCookie(srv, http.MethodGet, ownedContentPath, &http.Cookie{Name: transporthttp.ReaderContentCookieName, Value: forged})
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("grants disabled", func(t *testing.T) {
		off := readerSessionServer(t, nil, time.Now)
		if rr := getWithCookie(off, http.MethodGet, ownedContentPath, c); rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})
}

func TestReaderSession_SecureCookie(t *testing.T) {
	srv := readerSessionServer(t, auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn"), time.Now)
	fromProxy := func(r *http.Request) {
		r.RemoteAddr = "10.0.0.5:4000"
		r.Header.Set("X-Forwarded-Proto", "https")
	}

	t.Run("plain http", func(t *testing.T) {
		if c := issueGrantCookie(t, srv, "edition-owned"); c.Secure {
			t.Fatal("Secure set on a plain-http request")
		}
	})

	t.Run("X-Forwarded-Proto from an untrusted peer is ignored", func(t *testing.T) {
		transporthttp.SetTrustedProxyCIDRs(nil)
		if c := issueGrantCookieWith(t, srv, "edition-owned", fromProxy); c.Secure {
			t.Fatal("Secure trusted a client-supplied X-Forwarded-Proto")
		}
	})

	t.Run("chained proxies: first entry is the client-facing hop", func(t *testing.T) {
		transporthttp.SetTrustedProxyCIDRs([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
		t.Cleanup(func() { transporthttp.SetTrustedProxyCIDRs(nil) })
		chained := func(r *http.Request) {
			r.RemoteAddr = "10.0.0.5:4000"
			r.Header.Set("X-Forwarded-Proto", "https, http")
		}
		if c := issueGrantCookieWith(t, srv, "edition-owned", chained); !c.Secure {
			t.Fatal("Secure not set for X-Forwarded-Proto: https, http")
		}
	})

	t.Run("TLS terminated by a trusted proxy", func(t *testing.T) {
		transporthttp.SetTrustedProxyCIDRs([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
		t.Cleanup(func() { transporthttp.SetTrustedProxyCIDRs(nil) })
		if c := issueGrantCookieWith(t, srv, "edition-owned", fromProxy); !c.Secure {
			t.Fatal("Secure not set behind a trusted TLS-terminating proxy")
		}
	})
}

// An encoded slash makes the middleware (decoded path) and the mux
// ({editionId} from the escaped path) see different editions; the content
// handler refuses such an edition ID outright.
func TestReaderContent_EncodedSlashEditionRefused(t *testing.T) {
	srv := readerSessionServer(t, auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn"), time.Now)
	c := issueGrantCookie(t, srv, "edition-owned")
	rr := getWithCookie(srv, http.MethodGet, "/api/v1/library/editions/edition-owned%2Freader%2Fcontent%2Fx/reader/content/OEBPS/c1.xhtml", c)
	if rr.Code == http.StatusOK {
		t.Fatalf("encoded-slash edition served: %d", rr.Code)
	}
}
