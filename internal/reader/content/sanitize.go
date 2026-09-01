// Package content resolves, sanitises, and serves individual entries
// from inside an owned Edition's EPUB (backend-reader-content.md). It is
// the second file-content boundary this project has built and the first
// that hands parsed content back to a caller, so every byte served as
// HTML/CSS passes an allowlist sanitiser first (ADR 0024).
package content

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// SanitizeReport counts what a pass removed, for the info-level
// observability signal (backend-reader-content.md Observability):
// categories only, never the offending content or the surrounding
// document.
type SanitizeReport struct {
	ScriptsStripped      int
	ExternalRefsStripped int
	SVGStripped          int
	StyleAttrsStripped   int
}

func (r SanitizeReport) StrippedSomething() bool {
	return r.ScriptsStripped+r.ExternalRefsStripped+r.SVGStripped+r.StyleAttrsStripped > 0
}

var (
	// Pre-pass detectors — bluemonday removes disallowed constructs
	// silently, so the counts come from scanning the input, not diffing
	// the output. The policy itself is what actually enforces removal.
	reScriptTag    = regexp.MustCompile(`(?is)<script[\s>]`)
	reEventAttr    = regexp.MustCompile(`(?is)\son[a-z]+\s*=`)
	reSVGOpen      = regexp.MustCompile(`(?is)<svg[\s>]`)
	reStyleAttr    = regexp.MustCompile(`(?is)\sstyle\s*=`)
	reStyleBlock   = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)
	reExternalHREF = regexp.MustCompile(`(?is)\b(?:href|src)\s*=\s*["']?\s*(?:[a-z][a-z0-9+.-]*:)?//`)

	// CSS: a url(...) token or an @import target. RE2 has no
	// backreferences, so quotes are matched as optional single chars and
	// trimmed from the captured value afterwards.
	reCSSURL    = regexp.MustCompile(`(?is)url\(\s*['"]?([^'")]*?)['"]?\s*\)`)
	reCSSImport = regexp.MustCompile(`(?is)@import\s+(?:url\(\s*['"]?([^'")]*?)['"]?\s*\)|['"]([^'"]*)['"])\s*;?`)
	reScheme    = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

	// A value bluemonday may keep on href/src: a data: URI, or a
	// relative reference that does not begin with "//" (protocol-relative).
	reRelativeOrData = regexp.MustCompile(`(?is)^(?:data:\S+|(?:[^/:]|/[^/])[^:]*|#\S*)$`)
)

// htmlContentElements is the allowlist for EPUB XHTML content — the
// structural and inline elements a reflowable book actually uses.
// Everything else (script, svg, iframe, object, embed, link, meta,
// form, ...) is absent, so bluemonday drops it.
var htmlContentElements = []string{
	"html", "head", "title", "body",
	"p", "div", "span", "br", "hr", "pre", "blockquote",
	"h1", "h2", "h3", "h4", "h5", "h6",
	"ul", "ol", "li", "dl", "dt", "dd",
	"a", "img", "figure", "figcaption",
	"em", "strong", "i", "b", "u", "s", "small", "sub", "sup",
	"code", "kbd", "samp", "var", "mark", "cite", "q", "abbr", "time", "wbr",
	"section", "article", "aside", "nav", "header", "footer", "main", "address",
	"table", "thead", "tbody", "tfoot", "tr", "td", "th", "caption", "colgroup", "col",
	"ruby", "rt", "rp", "bdi", "bdo", "del", "ins",
}

// htmlPolicy builds the sanitisation policy (ADR 0024). A fresh
// allowlist policy — not UGCPolicy, whose AllowStandardURLs permanently
// whitelists http/https and cannot be reset — so href/src accept only
// relative paths and data: URIs (FR-6). <style> is not in the allowlist;
// SanitizeHTML lifts its CSS out, runs it through SanitizeCSS, and
// re-injects one sanitised <style> block, so bluemonday never has to
// reason about raw CSS text.
func htmlPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	p.AllowElements(htmlContentElements...)

	p.AllowAttrs("id", "class", "lang", "dir", "title").Globally()
	// A relative path or a data: URI only — reject a protocol-relative
	// //host reference, which AllowRelativeURLs would otherwise treat as
	// relative (FR-6).
	p.AllowAttrs("href").Matching(reRelativeOrData).OnElements("a")
	p.AllowAttrs("src").Matching(reRelativeOrData).OnElements("img")
	p.AllowAttrs("alt", "width", "height").OnElements("img")
	p.AllowAttrs("colspan", "rowspan", "scope").OnElements("td", "th")
	p.AllowAttrs("span").OnElements("col", "colgroup")

	p.AllowRelativeURLs(true)
	p.RequireParseableURLs(true)
	p.AllowURLSchemes("data")
	p.AddSpaceWhenStrippingTag(true)

	return p
}

// SanitizeHTML applies FR-6: strips scripts, event handlers, inline
// <svg>, style attributes, external-fetch elements, and any non-relative
// non-data: href/src; routes <style> block text through SanitizeCSS.
func SanitizeHTML(raw []byte) ([]byte, SanitizeReport) {
	var rep SanitizeReport
	rep.ScriptsStripped = len(reScriptTag.FindAll(raw, -1)) + len(reEventAttr.FindAll(raw, -1))
	rep.SVGStripped = len(reSVGOpen.FindAll(raw, -1))
	rep.StyleAttrsStripped = len(reStyleAttr.FindAll(raw, -1))
	rep.ExternalRefsStripped = len(reExternalHREF.FindAll(raw, -1))

	// Lift every <style> block's CSS out and run it through SanitizeCSS
	// (FR-6: it is CSS, it gets CSS's rule), then drop inline <svg>
	// wholesale, before the HTML policy runs.
	var css strings.Builder
	for _, m := range reStyleBlock.FindAllSubmatch(raw, -1) {
		safe, cssRep := SanitizeCSS(m[1])
		rep.ExternalRefsStripped += cssRep.ExternalRefsStripped
		css.Write(safe)
		css.WriteByte('\n')
	}
	stripped := reStyleBlock.ReplaceAll(raw, nil)
	stripped = stripSVG(stripped)

	cleaned := htmlPolicy().SanitizeBytes(stripped)

	if css.Len() > 0 {
		cleaned = append([]byte("<style>"+css.String()+"</style>\n"), cleaned...)
	}
	return cleaned, rep
}

var reSVGBlock = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
var reSVGSelfClose = regexp.MustCompile(`(?is)<svg[^>]*/>`)

func stripSVG(raw []byte) []byte {
	out := reSVGBlock.ReplaceAll(raw, nil)
	out = reSVGSelfClose.ReplaceAll(out, nil)
	return out
}

// SanitizeCSS applies FR-7: strips any url()/@import target whose URL has
// a scheme other than data:. A scheme-less (relative) reference is left
// unchanged. This is a bounded token scan, not a full CSS parser (ADR
// 0024).
func SanitizeCSS(raw []byte) ([]byte, SanitizeReport) {
	var rep SanitizeReport

	out := reCSSImport.ReplaceAllFunc(raw, func(stmt []byte) []byte {
		m := reCSSImport.FindSubmatch(stmt)
		url := string(bytes.TrimSpace(m[1]))
		if url == "" {
			url = string(bytes.TrimSpace(m[2]))
		}
		if schemeDisallowed(url) {
			rep.ExternalRefsStripped++
			return nil
		}
		return stmt
	})

	out = reCSSURL.ReplaceAllFunc(out, func(tok []byte) []byte {
		m := reCSSURL.FindSubmatch(tok)
		url := string(bytes.TrimSpace(m[1]))
		if schemeDisallowed(url) {
			rep.ExternalRefsStripped++
			return []byte("url()")
		}
		return tok
	})

	return out, rep
}

// schemeDisallowed is FR-7's exact rule: "has a scheme: prefix at all,
// and that scheme isn't data:". A bare relative reference (no scheme) is
// allowed.
func schemeDisallowed(url string) bool {
	url = strings.TrimSpace(url)
	if url == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(url), "data:") {
		return false
	}
	if strings.HasPrefix(url, "//") {
		return true // protocol-relative — resolves to an absolute origin
	}
	return reScheme.MatchString(url)
}
