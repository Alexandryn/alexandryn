package opds

import (
	"encoding/json"
	"strings"
)

// OPDS 2.0 is JSON built on the Readium Web Publication Manifest. Go's
// encoding/json enforces its own nesting-depth limit and we cap the body
// at 5 MiB upstream, so a JSON bomb cannot expand here; maxEntriesPerPage
// bounds a single oversized feed.

type opds2Link struct {
	Rel       any    `json:"rel"` // string or []string
	Href      string `json:"href"`
	Type      string `json:"type"`
	Templated bool   `json:"templated"`
}

type opds2Contributor struct {
	Name string
}

// UnmarshalJSON accepts a contributor as a bare string, an object with a
// "name", or an array of either (Readium allows all three).
func (c *opds2Contributor) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		c.Name = s
		return nil
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &obj); err == nil {
		c.Name = obj.Name
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err == nil && len(arr) > 0 {
		var first opds2Contributor
		if err := first.UnmarshalJSON(arr[0]); err == nil {
			c.Name = first.Name
			return nil
		}
	}
	return nil // a malformed contributor degrades to no author, not a parse failure
}

type opds2Pub struct {
	Metadata struct {
		Title  string           `json:"title"`
		Author opds2Contributor `json:"author"`
	} `json:"metadata"`
	Links  []opds2Link `json:"links"`
	Images []opds2Link `json:"images"`
}

type opds2Feed struct {
	Metadata struct {
		Title string `json:"title"`
	} `json:"metadata"`
	Links        []opds2Link `json:"links"`
	Publications []opds2Pub  `json:"publications"`
	Navigation   []opds2Link `json:"navigation"`
}

func parseOPDS2(body []byte) (parsedFeed, error) {
	var f opds2Feed
	if err := json.Unmarshal(body, &f); err != nil {
		return parsedFeed{}, errMalformedFeed
	}
	// A valid OPDS 2.0 feed has a self link and at least one of
	// navigation / publications / groups.
	if !hasRel(f.Links, "self") {
		return parsedFeed{}, errMalformedFeed
	}

	var pf parsedFeed
	for _, l := range f.Links {
		if linkHasRel(l, "next") && pf.nextHref == "" {
			pf.nextHref = l.Href
		}
		if linkHasRel(l, "search") && pf.searchHref == "" {
			pf.searchHref = l.Href // OPDS 2.0: a templated query URL directly
		}
	}

	for i, p := range f.Publications {
		if i >= maxEntriesPerPage {
			return parsedFeed{}, errMalformedFeed
		}
		pf.entries = append(pf.entries, pubToRaw(p))
	}
	return pf, nil
}

func pubToRaw(p opds2Pub) rawEntry {
	r := rawEntry{
		title:  strings.TrimSpace(p.Metadata.Title),
		author: strings.TrimSpace(p.Metadata.Author.Name),
	}
	for _, l := range p.Links {
		if isAcquisitionRel(l.Rel) && r.acqHref == "" {
			r.acqHref = l.Href
			r.acqType = l.Type
		}
	}
	if len(p.Images) > 0 {
		r.coverHref = p.Images[0].Href
	}
	// OPDS 2.0 publications have no stable per-entry id field we rely on;
	// dedup falls back to the acquisition href (feed.go).
	return r
}

func isAcquisitionRel(rel any) bool {
	for _, r := range relStrings(rel) {
		if r == "http://opds-spec.org/acquisition" ||
			strings.HasPrefix(r, "http://opds-spec.org/acquisition/") {
			return true
		}
	}
	return false
}

func linkHasRel(l opds2Link, want string) bool {
	for _, r := range relStrings(l.Rel) {
		if r == want || strings.EqualFold(r, want) {
			return true
		}
	}
	return false
}

func hasRel(links []opds2Link, want string) bool {
	for _, l := range links {
		if linkHasRel(l, want) {
			return true
		}
	}
	return false
}

func relStrings(rel any) []string {
	switch v := rel.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
