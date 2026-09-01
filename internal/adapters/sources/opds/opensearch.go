package opds

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// openSearchDescription is the subset of an OpenSearch description
// document (linked with rel="search" from an OPDS 1.2 feed) this adapter
// needs: the query URL templates.
type openSearchDescription struct {
	URLs []struct {
		Type     string `xml:"type,attr"`
		Template string `xml:"template,attr"`
	} `xml:"Url"`
}

// parseOpenSearchTemplate extracts the best query-URL template from an
// OpenSearch description document: an OPDS/Atom-typed Url containing the
// {searchTerms} macro, falling back to any Url with the macro. Returns
// "" when the document names no usable template.
func parseOpenSearchTemplate(body []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = true
	dec.Entity = nil
	dec.CharsetReader = safeCharsetReader

	var d openSearchDescription
	if err := dec.Decode(&d); err != nil {
		return ""
	}

	var fallback string
	for _, u := range d.URLs {
		if !strings.Contains(u.Template, "{searchTerms}") {
			continue
		}
		t := strings.ToLower(u.Type)
		if strings.Contains(t, "opds") || strings.Contains(t, "atom") {
			return u.Template
		}
		if fallback == "" {
			fallback = u.Template
		}
	}
	return fallback
}
