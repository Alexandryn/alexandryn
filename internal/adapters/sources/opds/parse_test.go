package opds

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

const testBase = "https://opds.example.org/opds"

func testCodec() *sources.CursorCodec {
	return sources.NewCursorCodec(bytes.Repeat([]byte{9}, 32))
}

func TestParseAtom_AcquisitionFeed(t *testing.T) {
	pf, err := parseAtom(fixture(t, "opds12_acquisition.xml"))
	if err != nil {
		t.Fatalf("parseAtom: %v", err)
	}
	if pf.nextHref != "/opds/recent?page=2" {
		t.Fatalf("nextHref = %q", pf.nextHref)
	}
	if pf.searchHref != "/opds/opensearch.xml" || !pf.searchIsDescriptionDoc {
		t.Fatalf("search = %q, descDoc=%v", pf.searchHref, pf.searchIsDescriptionDoc)
	}
	if len(pf.entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(pf.entries))
	}

	page := normalise(pf, testBase, testCodec(), "src-1")
	if len(page.Items) != 3 {
		t.Fatalf("candidates = %d, want 3", len(page.Items))
	}
	first := page.Items[0]
	if first.Title != "The Left Hand of Darkness" || first.Author == nil || *first.Author != "Ursula K. Le Guin" {
		t.Fatalf("first candidate wrong: %+v", first)
	}
	if first.FileReference.Format != "EPUB" || first.FileReference.ReferenceID != "/download/book-1.epub" {
		t.Fatalf("first file ref wrong: %+v", first.FileReference)
	}
	if first.CoverURL == nil || *first.CoverURL != "https://opds.example.org/covers/book-1.jpg" {
		t.Fatalf("first cover wrong: %v", first.CoverURL)
	}
	// Second entry uses an open-access acquisition rel and a PDF type.
	if page.Items[1].FileReference.Format != "PDF" {
		t.Fatalf("second format = %q, want PDF", page.Items[1].FileReference.Format)
	}
	// Third entry's cover is off-origin — must be dropped to nil.
	if page.Items[2].CoverURL != nil {
		t.Fatalf("off-origin cover leaked: %v", *page.Items[2].CoverURL)
	}
	// next cursor present and same-origin.
	if page.NextCursor == nil {
		t.Fatal("expected a next cursor")
	}
	c, err := testCodec().Decode(*page.NextCursor, "src-1")
	if err != nil {
		t.Fatalf("decode next cursor: %v", err)
	}
	if c.Position != "https://opds.example.org/opds/recent?page=2" {
		t.Fatalf("cursor position = %q", c.Position)
	}
}

func TestParseAtom_NavigationFeed(t *testing.T) {
	pf, err := parseAtom(fixture(t, "opds12_navigation.xml"))
	if err != nil {
		t.Fatalf("parseAtom: %v", err)
	}
	// A navigation entry has no acquisition link — it normalises to zero
	// candidates this phase (we surface publications, not sub-feeds).
	page := normalise(pf, testBase, testCodec(), "s")
	if len(page.Items) != 0 {
		t.Fatalf("navigation feed produced %d candidates, want 0", len(page.Items))
	}
}

func TestParseAtom_DeduplicatesByEntryID(t *testing.T) {
	pf, err := parseAtom(fixture(t, "opds12_dupe.xml"))
	if err != nil {
		t.Fatalf("parseAtom: %v", err)
	}
	page := normalise(pf, testBase, testCodec(), "s")
	if len(page.Items) != 1 {
		t.Fatalf("candidates = %d, want 1 (deduped)", len(page.Items))
	}
}

func TestParseAtom_BillionLaughsIsRejectedNotExpanded(t *testing.T) {
	done := make(chan struct{})
	var out parsedFeed
	var err error
	go func() {
		out, err = parseAtom(fixture(t, "opds12_billion_laughs.xml"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("parseAtom did not return within 5s on a billion-laughs payload")
	}
	if err == nil {
		t.Fatalf("billion-laughs payload parsed without error: %d entries", len(out.entries))
	}
}

func TestParseAtom_ExternalEntityNotResolved(t *testing.T) {
	_, err := parseAtom(fixture(t, "opds12_xxe_external.xml"))
	if err == nil {
		t.Fatal("external-entity payload parsed without error — entity may have been resolved")
	}
}

func TestParseAtom_RejectsNonFeed(t *testing.T) {
	for _, in := range []string{
		`<html><body>not a feed</body></html>`,
		`<feed`,
		``,
		`<?xml version="1.0"?><other/>`,
	} {
		if _, err := parseAtom([]byte(in)); err == nil {
			t.Errorf("parseAtom(%q) = nil error, want rejection", in)
		}
	}
}

func TestParseAtom_DepthLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom"><link rel="self" href="/x"/>`)
	for i := 0; i < 200; i++ {
		b.WriteString("<x>")
	}
	for i := 0; i < 200; i++ {
		b.WriteString("</x>")
	}
	b.WriteString(`</feed>`)
	if _, err := parseAtom([]byte(b.String())); err == nil {
		t.Fatal("deeply nested feed accepted, want depth-limit rejection")
	}
}

func TestParseOPDS2_Feed(t *testing.T) {
	pf, err := parseOPDS2(fixture(t, "opds20_feed.json"))
	if err != nil {
		t.Fatalf("parseOPDS2: %v", err)
	}
	if pf.nextHref != "/2.0/feed?page=2" {
		t.Fatalf("nextHref = %q", pf.nextHref)
	}
	if pf.searchHref != "/2.0/search?q={searchTerms}" || pf.searchIsDescriptionDoc {
		t.Fatalf("search = %q descDoc=%v", pf.searchHref, pf.searchIsDescriptionDoc)
	}

	const base20 = "https://opds.example.org/2.0/feed"
	page := normalise(pf, base20, testCodec(), "s")
	if len(page.Items) != 2 {
		t.Fatalf("candidates = %d, want 2", len(page.Items))
	}
	if page.Items[0].Title != "Kindred" || page.Items[0].Author == nil || *page.Items[0].Author != "Octavia E. Butler" {
		t.Fatalf("first = %+v", page.Items[0])
	}
	if page.Items[0].FileReference.Format != "EPUB" {
		t.Fatalf("first format = %q", page.Items[0].FileReference.Format)
	}
	if page.Items[0].CoverURL == nil || *page.Items[0].CoverURL != "https://opds.example.org/2.0/covers/kindred.jpg" {
		t.Fatalf("first cover = %v", page.Items[0].CoverURL)
	}
	// Second: author given as an array of objects.
	if page.Items[1].Author == nil || *page.Items[1].Author != "Octavia E. Butler" {
		t.Fatalf("second author = %v", page.Items[1].Author)
	}
}

func TestParseOPDS2_DeduplicatesByHref(t *testing.T) {
	pf, err := parseOPDS2(fixture(t, "opds20_dupe.json"))
	if err != nil {
		t.Fatalf("parseOPDS2: %v", err)
	}
	page := normalise(pf, "https://opds.example.org/2.0/dupe", testCodec(), "s")
	if len(page.Items) != 1 {
		t.Fatalf("candidates = %d, want 1 (deduped by href)", len(page.Items))
	}
}

func TestParseOPDS2_NavigationOnly(t *testing.T) {
	pf, err := parseOPDS2(fixture(t, "opds20_navigation.json"))
	if err != nil {
		t.Fatalf("parseOPDS2: %v", err)
	}
	page := normalise(pf, "https://opds.example.org/2.0/", testCodec(), "s")
	if len(page.Items) != 0 {
		t.Fatalf("navigation-only feed produced %d candidates", len(page.Items))
	}
}

func TestParseOPDS2_RejectsNonFeed(t *testing.T) {
	for _, in := range []string{`{}`, `{"publications":[]}`, `not json`, `[]`} {
		if _, err := parseOPDS2([]byte(in)); err == nil {
			t.Errorf("parseOPDS2(%q) = nil, want rejection (no self link)", in)
		}
	}
}

func TestParseOpenSearchTemplate(t *testing.T) {
	got := parseOpenSearchTemplate(fixture(t, "opds12_opensearch.xml"))
	if got != "/opds/search?q={searchTerms}" {
		t.Fatalf("template = %q, want the opds-typed one", got)
	}
	if parseOpenSearchTemplate([]byte(`<OpenSearchDescription/>`)) != "" {
		t.Fatal("empty description should yield no template")
	}
}

func TestExpandSearchTemplate(t *testing.T) {
	cases := map[string]string{
		"/s?q={searchTerms}": "/s?q=hello+world",
		"/s{?query}":         "/s?query=hello+world",
		"/s?type=book":       "/s?type=book&q=hello+world",
		"/s":                 "/s?q=hello+world",
	}
	for tmpl, want := range cases {
		if got := expandSearchTemplate(tmpl, "hello world"); got != want {
			t.Errorf("expand(%q) = %q, want %q", tmpl, got, want)
		}
	}
}

func TestNormalise_EntryWithoutFormatIsDropped(t *testing.T) {
	pf := parsedFeed{entries: []rawEntry{
		{id: "1", title: "No format", acqHref: "/d/x"},                              // no type, no ext
		{id: "2", title: "Has ext", acqHref: "/d/y.epub"},                           // ext-derived
		{id: "3", title: "", acqHref: "/d/z.epub", acqType: "application/epub+zip"}, // empty title
	}}
	page := normalise(pf, testBase, testCodec(), "s")
	if len(page.Items) != 1 || page.Items[0].Title != "Has ext" {
		t.Fatalf("normalise kept %d items: %+v", len(page.Items), page.Items)
	}
}
