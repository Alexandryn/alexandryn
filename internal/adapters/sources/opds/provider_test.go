package opds

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

// opdsServer serves fixture files by path, with an optional per-path
// override for status codes and dynamic responses.
type opdsServer struct {
	*httptest.Server
	routes map[string]http.HandlerFunc
}

func newOPDSServer(t *testing.T) *opdsServer {
	t.Helper()
	s := &opdsServer{routes: map[string]http.HandlerFunc{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := s.routes[r.URL.Path]; ok {
			h(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *opdsServer) serveFixture(path, name, contentType string) {
	s.routes[path] = func(w http.ResponseWriter, _ *http.Request) {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(b)
	}
}

func newProvider(t *testing.T, base, searchTemplate string) *Provider {
	t.Helper()
	return New(Config{
		SourceID:       "src-opds",
		BaseURL:        base,
		SearchTemplate: searchTemplate,
		Semaphore:      sources.NewSemaphore(50),
		Codec:          testCodec(),
	})
}

func TestProvider_Probe_ReachableWithSearch_OPDS12(t *testing.T) {
	srv := newOPDSServer(t)
	srv.serveFixture("/opds", "opds12_acquisition.xml", "application/atom+xml")
	srv.serveFixture("/opds/opensearch.xml", "opds12_opensearch.xml", "application/opensearchdescription+xml")
	base := srv.URL + "/opds"

	res := newProvider(t, base, "").Probe(context.Background())
	if res.Status != sources.HealthReachable {
		t.Fatalf("status = %q (%q), want reachable", res.Status, res.Detail)
	}
	if !res.Capabilities.CanList || !res.Capabilities.CanDownload {
		t.Fatalf("caps = %+v", res.Capabilities)
	}
	if !res.Capabilities.CanSearch {
		t.Fatal("CanSearch = false, want true (feed advertises OpenSearch)")
	}
	wantSearch := srv.URL + "/opds/search?q={searchTerms}"
	if res.SearchLinkURL != wantSearch {
		t.Fatalf("SearchLinkURL = %q, want %q", res.SearchLinkURL, wantSearch)
	}
}

func TestProvider_Probe_OffOriginSearchLinkDiscarded(t *testing.T) {
	srv := newOPDSServer(t)
	srv.serveFixture("/opds", "opds12_acquisition.xml", "application/atom+xml")
	srv.serveFixture("/opds/opensearch.xml", "opds12_opensearch_offorigin.xml", "application/opensearchdescription+xml")
	base := srv.URL + "/opds"

	res := newProvider(t, base, "").Probe(context.Background())
	if res.Status != sources.HealthReachable {
		t.Fatalf("status = %q", res.Status)
	}
	if res.Capabilities.CanSearch || res.SearchLinkURL != "" {
		t.Fatalf("off-origin search template was kept: canSearch=%v url=%q", res.Capabilities.CanSearch, res.SearchLinkURL)
	}
}

func TestProvider_Probe_OPDS2DirectSearchLink(t *testing.T) {
	srv := newOPDSServer(t)
	// Serve the 2.0 feed but rewrite its self link to this server's origin
	// so the search link resolves same-origin.
	srv.routes["/2.0/feed"] = func(w http.ResponseWriter, _ *http.Request) {
		b, _ := os.ReadFile(filepath.Join("testdata", "opds20_feed.json"))
		b = bytes.ReplaceAll(b, []byte("https://opds.example.org"), []byte(srv.URL))
		w.Header().Set("Content-Type", "application/opds+json")
		_, _ = w.Write(b)
	}
	base := srv.URL + "/2.0/feed"

	res := newProvider(t, base, "").Probe(context.Background())
	if !res.Capabilities.CanSearch {
		t.Fatalf("CanSearch = false, want true; detail=%q", res.Detail)
	}
	if !strings.Contains(res.SearchLinkURL, "{searchTerms}") {
		t.Fatalf("SearchLinkURL = %q, want a templated query URL", res.SearchLinkURL)
	}
}

func TestProvider_Probe_Unreachable(t *testing.T) {
	srv := newOPDSServer(t)
	srv.routes["/opds"] = func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }
	res := newProvider(t, srv.URL+"/opds", "").Probe(context.Background())
	if res.Status != sources.HealthUnreachable || res.Detail != sources.DetailHTTP5xx {
		t.Fatalf("res = %+v, want unreachable/http-5xx", res)
	}
}

func TestProvider_Probe_Unparseable(t *testing.T) {
	srv := newOPDSServer(t)
	srv.routes["/opds"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not a feed</html>"))
	}
	res := newProvider(t, srv.URL+"/opds", "").Probe(context.Background())
	if res.Detail != sources.DetailUnparseable {
		t.Fatalf("detail = %q, want unparseable-response", res.Detail)
	}
}

func TestProvider_List_FetchesBaseThenPaginates(t *testing.T) {
	srv := newOPDSServer(t)
	srv.serveFixture("/opds", "opds12_acquisition.xml", "application/atom+xml")
	srv.routes["/opds/recent"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"><id>x</id><title>p2</title><link rel="self" href="/opds/recent?page=2"/>
			<entry><title>Page Two Book</title><id>p2-1</id>
			<link rel="http://opds-spec.org/acquisition" type="application/epub+zip" href="/d/p2.epub"/></entry></feed>`))
	}
	base := srv.URL + "/opds"
	p := newProvider(t, base, "")

	page1, err := p.List(context.Background(), "", 20)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1.Items) != 3 || page1.NextCursor == nil {
		t.Fatalf("page1: %d items, cursor=%v", len(page1.Items), page1.NextCursor)
	}

	page2, err := p.List(context.Background(), *page1.NextCursor, 20)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2.Items) != 1 || page2.Items[0].Title != "Page Two Book" {
		t.Fatalf("page2 wrong: %+v", page2.Items)
	}
}

func TestProvider_List_RejectsOffOriginCursorBeforeFetch(t *testing.T) {
	srv := newOPDSServer(t)
	srv.serveFixture("/opds", "opds12_acquisition.xml", "application/atom+xml")
	base := srv.URL + "/opds"
	p := newProvider(t, base, "")

	// Forge a well-signed cursor (same codec) pointing off-origin.
	evil := testCodec().Encode(sources.Cursor{
		SourceID: "src-opds", Kind: sources.KindOPDS, Position: "https://evil.example/opds?page=2",
	})
	_, err := p.List(context.Background(), evil, 20)
	if domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("off-origin cursor: err = %v, want InvalidInput", err)
	}
}

func TestProvider_Search_ConflictWithoutTemplate(t *testing.T) {
	p := newProvider(t, "https://opds.example.org/opds", "")
	_, err := p.Search(context.Background(), "dune", "", 20)
	if domain.CategoryOf(err) != domain.Conflict {
		t.Fatalf("Search without template: err = %v, want Conflict", err)
	}
}

func TestProvider_Search_RunsQuery(t *testing.T) {
	srv := newOPDSServer(t)
	var gotQuery string
	srv.routes["/opds/search"] = func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"><id>s</id><title>results</title><link rel="self" href="/opds/search"/>
			<entry><title>Dune</title><id>d1</id>
			<link rel="http://opds-spec.org/acquisition" type="application/epub+zip" href="/d/dune.epub"/></entry></feed>`))
	}
	base := srv.URL + "/opds"
	template := srv.URL + "/opds/search?q={searchTerms}"
	p := newProvider(t, base, template)

	page, err := p.Search(context.Background(), "dune messiah", "", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotQuery != "dune messiah" {
		t.Fatalf("upstream q = %q, want %q", gotQuery, "dune messiah")
	}
	if len(page.Items) != 1 || page.Items[0].Title != "Dune" {
		t.Fatalf("results wrong: %+v", page.Items)
	}
}

func TestProvider_Search_RejectsOffOriginTemplate(t *testing.T) {
	// A stored template pointing off-origin (should never happen — probe
	// discards these — but defence in depth).
	p := newProvider(t, "https://opds.example.org/opds", "https://evil.example/search?q={searchTerms}")
	_, err := p.Search(context.Background(), "x", "", 20)
	if domain.CategoryOf(err) != domain.Unavailable {
		t.Fatalf("off-origin template: err = %v, want Unavailable", err)
	}
}
