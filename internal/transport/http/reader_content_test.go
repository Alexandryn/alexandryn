package http_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func zipWith(t *testing.T, files map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zr
}

func contentServer(t *testing.T, files map[string]string) http.Handler {
	t.Helper()
	poolRef := &transporthttp.PoolRef{}
	cache := content.NewCache(func(ctx context.Context, editionID domain.EditionID) (*zip.Reader, func(), error) {
		if editionID != "edition-owned" {
			return nil, nil, &domain.Error{Category: domain.NotFound, Message: "edition not found"}
		}
		return zipWith(t, files), func() {}, nil
	})
	poolRef.SetReaderContentCache(cache)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/library/editions/{editionId}/reader/content/{path...}", transporthttp.ReaderContentHandler(poolRef, nil))
	return transporthttp.Chain(mux, transporthttp.Recovery(nil, func() string { return "test" }))
}

func TestReaderContent_ServesSanitisedHTMLWithCSP(t *testing.T) {
	srv := contentServer(t, map[string]string{
		"OEBPS/c1.xhtml": `<html><body><p>Chapter <script>evil()</script>text</p><img src="https://tracker.example/p.gif"></body></html>`,
	})

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/library/editions/edition-owned/reader/content/OEBPS/c1.xhtml", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rr.Code, rr.Body.String())
	}
	if csp := rr.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'none'") {
		t.Fatalf("CSP header missing or wrong: %q", csp)
	}
	body := rr.Body.String()
	if strings.Contains(body, "evil()") || strings.Contains(body, "tracker.example") {
		t.Fatalf("hostile content served: %q", body)
	}
	if !strings.Contains(body, "Chapter") {
		t.Fatalf("legitimate content missing: %q", body)
	}
}

func TestReaderContent_UnownedEditionIs404(t *testing.T) {
	srv := contentServer(t, map[string]string{"OEBPS/c1.xhtml": "<p>x</p>"})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/library/editions/edition-not-mine/reader/content/OEBPS/c1.xhtml", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestReaderContent_PathTraversalNeverServesContent(t *testing.T) {
	srv := contentServer(t, map[string]string{"OEBPS/c1.xhtml": "<p>x</p>", "secret": "SENSITIVE"})

	// ServeMux normalises a literal ../ (307 before the handler); a
	// dot-segment that reaches the handler is rejected by
	// ValidateResourcePath. Neither serves content.
	for _, p := range []string{
		"/api/v1/library/editions/edition-owned/reader/content/../secret",
		"/api/v1/library/editions/edition-owned/reader/content/OEBPS/./c1.xhtml",
	} {
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, p, nil))
		if rr.Code == http.StatusOK {
			t.Fatalf("%s served content (status 200): %q", p, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), "SENSITIVE") {
			t.Fatalf("%s leaked the secret entry", p)
		}
	}
}

// A dot-segment path that survives mux normalisation is rejected by the
// handler's own FR-1 validation, before any entry lookup.
func TestReaderContent_DotSegmentIs400(t *testing.T) {
	h := transporthttp.ReaderContentHandler(readyContentPoolRef(t), nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.SetPathValue("editionId", "edition-owned")
	req.SetPathValue("path", "OEBPS/../secret")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func readyContentPoolRef(t *testing.T) *transporthttp.PoolRef {
	t.Helper()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetReaderContentCache(content.NewCache(func(ctx context.Context, editionID domain.EditionID) (*zip.Reader, func(), error) {
		return zipWith(t, map[string]string{"OEBPS/c1.xhtml": "<p>x</p>"}), func() {}, nil
	}))
	return poolRef
}

func TestReaderContent_MissingEntryIs404(t *testing.T) {
	srv := contentServer(t, map[string]string{"OEBPS/c1.xhtml": "<p>x</p>"})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/library/editions/edition-owned/reader/content/OEBPS/missing.xhtml", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestReaderContent_StandaloneSVGIs400(t *testing.T) {
	srv := contentServer(t, map[string]string{"OEBPS/cover.svg": `<svg xmlns="http://www.w3.org/2000/svg"><script>x()</script></svg>`})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/library/editions/edition-owned/reader/content/OEBPS/cover.svg", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestReaderContent_CSSSanitised(t *testing.T) {
	srv := contentServer(t, map[string]string{
		"styles/main.css": `body{color:black} .x{background:url(https://tracker.example/p.gif)}`,
	})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/library/editions/edition-owned/reader/content/styles/main.css", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("content-type = %q", ct)
	}
	if strings.Contains(rr.Body.String(), "tracker.example") {
		t.Fatalf("external url survived: %q", rr.Body.String())
	}
}
