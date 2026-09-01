package opds

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxXMLDepth bounds element nesting while scanning an Atom feed. Go's
// encoding/xml never resolves a DTD or external entity (so Billion
// Laughs / XXE cannot expand), but a deeply-nested document could still
// drive DecodeElement into deep recursion — this token-level guard
// rejects it before that happens.
const maxXMLDepth = 40

// errMalformedFeed is returned for any input that is not a parseable
// Atom OPDS feed.
var errMalformedFeed = errors.New("opds: response is not a parseable feed")

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomEntry struct {
	ID      string       `xml:"id"`
	Title   string       `xml:"title"`
	Authors []atomAuthor `xml:"author"`
	Links   []atomLink   `xml:"link"`
}

// parseAtom parses an OPDS 1.2 (Atom) feed. It walks the document with a
// depth-limited token loop, decoding each <entry> as a bounded subtree
// and collecting feed-level <link> elements, stopping at
// maxEntriesPerPage.
func parseAtom(body []byte) (parsedFeed, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = true
	dec.Entity = nil // only the five predefined XML entities; no DTD entities
	dec.CharsetReader = safeCharsetReader

	var pf parsedFeed
	var depth int
	sawFeed := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return parsedFeed{}, errMalformedFeed
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxXMLDepth {
				return parsedFeed{}, errMalformedFeed
			}
			switch t.Name.Local {
			case "feed":
				sawFeed = true
			case "entry":
				if len(pf.entries) >= maxEntriesPerPage {
					return parsedFeed{}, errMalformedFeed
				}
				var e atomEntry
				if err := dec.DecodeElement(&e, &t); err != nil {
					return parsedFeed{}, errMalformedFeed
				}
				depth-- // DecodeElement consumed the matching EndElement
				pf.entries = append(pf.entries, atomEntryToRaw(e))
			case "link":
				if depth <= 2 { // feed-level link, not an entry link
					applyFeedLink(&pf, attrLink(t))
				}
			}
		case xml.EndElement:
			depth--
			if depth < 0 {
				return parsedFeed{}, errMalformedFeed
			}
		}
	}

	if !sawFeed {
		return parsedFeed{}, errMalformedFeed
	}
	return pf, nil
}

func attrLink(se xml.StartElement) atomLink {
	var l atomLink
	for _, a := range se.Attr {
		switch a.Name.Local {
		case "rel":
			l.Rel = a.Value
		case "href":
			l.Href = a.Value
		case "type":
			l.Type = a.Value
		}
	}
	return l
}

func applyFeedLink(pf *parsedFeed, l atomLink) {
	switch {
	case strings.EqualFold(l.Rel, "next"):
		if pf.nextHref == "" {
			pf.nextHref = l.Href
		}
	case strings.EqualFold(l.Rel, "search"):
		if pf.searchHref == "" {
			pf.searchHref = l.Href
			// OPDS 1.2's search link points at an OpenSearch description
			// document (type application/opensearchdescription+xml).
			pf.searchIsDescriptionDoc = true
		}
	}
}

func atomEntryToRaw(e atomEntry) rawEntry {
	r := rawEntry{
		id:    strings.TrimSpace(e.ID),
		title: strings.TrimSpace(e.Title),
	}
	if len(e.Authors) > 0 {
		r.author = strings.TrimSpace(e.Authors[0].Name)
	}
	for _, l := range e.Links {
		switch {
		case strings.HasPrefix(l.Rel, "http://opds-spec.org/acquisition"):
			if r.acqHref == "" {
				r.acqHref = l.Href
				r.acqType = l.Type
			}
		case l.Rel == "http://opds-spec.org/image" || l.Rel == "http://opds-spec.org/cover":
			if r.coverHref == "" {
				r.coverHref = l.Href
			}
		case l.Rel == "http://opds-spec.org/image/thumbnail" && r.coverHref == "":
			r.coverHref = l.Href
		}
	}
	return r
}

// safeCharsetReader permits only UTF-8 and US-ASCII, the encodings an
// OPDS feed is realistically served in. Anything else is rejected rather
// than decoded, keeping this adapter off the path of a charset-specific
// decoding bug.
func safeCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	default:
		return nil, fmt.Errorf("opds: unsupported charset %q", charset)
	}
}

// looksLikeXML is a cheap pre-check so parseFeed can pick a parser.
func looksLikeXML(body []byte) bool {
	trimmed := bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	trimmed = bytes.TrimLeft(trimmed, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte("<"))
}
