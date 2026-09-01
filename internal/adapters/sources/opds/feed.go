package opds

import (
	"net/url"
	"path"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

// maxEntriesPerPage bounds how many entries this adapter will normalise
// from one upstream response, on top of the 5 MiB body cap — a defence
// against a source returning a single enormous feed page (FR-6, the
// roadmap risk table's "memory exhaustion via unbounded feed pages").
const maxEntriesPerPage = 2000

const maxCandidateTitleLen = 512

// parsedFeed is the version-agnostic shape both the OPDS 1.2 (Atom) and
// OPDS 2.0 (JSON) parsers produce. No raw XML element or JSON field name
// exists past this struct (FR-9, constitution §3).
type parsedFeed struct {
	entries    []rawEntry
	nextHref   string // rel="next", unresolved
	searchHref string // rel="search", unresolved
	// searchIsDescriptionDoc is true for OPDS 1.2, where the search link
	// points at an OpenSearch description document rather than a
	// templated query URL.
	searchIsDescriptionDoc bool
}

type rawEntry struct {
	id        string
	title     string
	author    string
	acqHref   string
	acqType   string // upstream media type
	coverHref string
}

// mediaTypeFormat maps an OPDS acquisition-link media type to this
// project's format label. An unrecognised type falls back to the href's
// file extension; an entry with neither is dropped (a FileReference
// needs a non-empty format).
var mediaTypeFormat = map[string]string{
	"application/epub+zip":           "EPUB",
	"application/pdf":                "PDF",
	"application/x-mobipocket-ebook": "MOBI",
	"application/vnd.amazon.ebook":   "AZW3",
	"application/vnd.comicbook+zip":  "CBZ",
	"application/vnd.comicbook-rar":  "CBR",
	"application/x-cbr":              "CBR",
	"application/x-cbz":              "CBZ",
	"text/fb2+xml":                   "FB2",
	"application/x-fictionbook+xml":  "FB2",
	"image/vnd.djvu":                 "DJVU",
}

var extFormat = map[string]string{
	".epub": "EPUB", ".pdf": "PDF", ".mobi": "MOBI", ".azw3": "AZW3",
	".cbz": "CBZ", ".cbr": "CBR", ".fb2": "FB2", ".djvu": "DJVU",
}

// normalise turns a parsedFeed into the wire candidates, resolving and
// origin-checking every source-supplied URL against baseURL (FR-9,
// FR-11). Duplicate entries within the page (same id) are dropped,
// later occurrences first (FR-9, mirroring the metadata adapter).
func normalise(pf parsedFeed, baseURL string, codec *sources.CursorCodec, sourceID string) sources.CandidatePage {
	seen := make(map[string]bool, len(pf.entries))
	items := make([]sources.SourceCandidate, 0, len(pf.entries))

	for _, e := range pf.entries {
		key := e.id
		if key == "" {
			key = e.acqHref
		}
		if key != "" && seen[key] {
			continue
		}
		if key != "" {
			seen[key] = true
		}

		cand, ok := candidateFromEntry(e, baseURL)
		if !ok {
			continue
		}
		items = append(items, cand)
	}

	page := sources.CandidatePage{Items: items}
	if next := resolveSameOrigin(baseURL, pf.nextHref); next != "" {
		tok := codec.Encode(sources.Cursor{SourceID: sourceID, Kind: sources.KindOPDS, Position: next})
		page.NextCursor = &tok
	}
	return page
}

func candidateFromEntry(e rawEntry, baseURL string) (sources.SourceCandidate, bool) {
	title := strings.TrimSpace(e.title)
	if err := domain.ValidateBoundedText("title", title, maxCandidateTitleLen); err != nil {
		return sources.SourceCandidate{}, false
	}
	if e.acqHref == "" {
		return sources.SourceCandidate{}, false
	}

	format := formatFor(e.acqType, e.acqHref)
	if format == "" {
		return sources.SourceCandidate{}, false
	}

	// The acquisition href is stored opaquely as the FileReference id.
	// It is never fetched here; Resolve (phase 10) re-validates it
	// same-origin before use.
	ref, err := domain.NewFileReference(e.acqHref, format, nil)
	if err != nil {
		return sources.SourceCandidate{}, false
	}

	cand := sources.SourceCandidate{Title: title, FileReference: ref}

	if author := strings.TrimSpace(e.author); author != "" {
		if domain.ValidateBoundedText("author", author, maxCandidateTitleLen) == nil {
			cand.Author = &author
		}
	}
	// A cover URL is emitted only when it resolves same-origin with the
	// configured catalog (FR-11's spirit): a self-hosted catalog serves
	// its own covers; an off-origin cover URL would make a LAN client's
	// browser beacon an attacker-chosen host on render.
	if cover := resolveSameOrigin(baseURL, e.coverHref); cover != "" {
		cand.CoverURL = &cover
	}
	return cand, true
}

func formatFor(mediaType, href string) string {
	mt := strings.ToLower(strings.TrimSpace(mediaType))
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	if f, ok := mediaTypeFormat[mt]; ok {
		return f
	}
	if u, err := url.Parse(href); err == nil {
		if f, ok := extFormat[strings.ToLower(path.Ext(u.Path))]; ok {
			return f
		}
	}
	return ""
}

// resolveSameOrigin resolves href against baseURL and returns the
// absolute URL only when it is same-origin (FR-11). A relative href
// always resolves within the base's origin; an absolute off-origin href
// is rejected (returns "").
func resolveSameOrigin(baseURL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref).String()
	if !sources.SameOrigin(baseURL, resolved) {
		return ""
	}
	return resolved
}
