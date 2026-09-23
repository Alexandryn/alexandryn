// Package content resolves, sanitises, and serves individual entries
// from inside an owned Edition's EPUB archive. Every byte served as
// HTML or CSS passes an allowlist sanitiser before returning to the client.
package content

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// SanitizeReport counts what a pass removed, for observability logging:
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

	// A relative-only reference that does not begin with "//" and rejects data: schemes.
	// Used on <a href> so that books cannot embed data:text/html anchors to prevent UI-redress.
	reRelativeOnly = regexp.MustCompile(`(?is)^(?:(?:[^/:]|/[^/])[^:]*|#\S*)$`)
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
	// A stylesheet link whose href is relative or data: only — it fetches
	// the (also-sanitised) CSS from this system's own origin, so it does not
	// trigger external network requests. An http(s) or //host href
	// is rejected by reRelativeOrData below, same as an <img src>.
	"link",
}

// htmlPolicy builds the sanitisation policy. A fresh
// allowlist policy — not UGCPolicy, whose AllowStandardURLs permanently
// whitelists http/https and cannot be reset — so href/src accept only
// relative paths and data: URIs. <style> is not in the allowlist;
// SanitizeHTML lifts its CSS out, runs it through SanitizeCSS, and
// re-injects one sanitised <style> block, so bluemonday never has to
// reason about raw CSS text.
func htmlPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	p.AllowElements(htmlContentElements...)

	p.AllowAttrs("id", "class", "lang", "dir", "title").Globally()
	// A relative path only on <a> (data: href on <a> is disallowed).
	// Rejects protocol-relative //host references and data: URIs.
	p.AllowAttrs("href").Matching(reRelativeOnly).OnElements("a")
	p.AllowAttrs("src").Matching(reRelativeOrData).OnElements("img")
	p.AllowAttrs("alt", "width", "height").OnElements("img")
	// <link rel="stylesheet" href="relative.css"> only — no preconnect,
	// prefetch, dns-prefetch, modulepreload, or any other rel value, and
	// the href must be relative/data: (reRelativeOrData).
	p.AllowAttrs("rel").Matching(regexp.MustCompile(`(?i)^stylesheet$`)).OnElements("link")
	p.AllowAttrs("type").OnElements("link")
	p.AllowAttrs("href").Matching(reRelativeOrData).OnElements("link")
	p.AllowAttrs("colspan", "rowspan", "scope").OnElements("td", "th")
	p.AllowAttrs("span").OnElements("col", "colgroup")

	p.AllowRelativeURLs(true)
	p.RequireParseableURLs(true)
	p.AllowURLSchemes("data")
	p.AddSpaceWhenStrippingTag(true)

	return p
}

// SanitizeHTML strips scripts, event handlers, inline <svg>, style
// attributes, external-fetch elements, and any non-relative, non-data:
// href/src attributes; routes <style> block text through SanitizeCSS.
func SanitizeHTML(raw []byte) ([]byte, SanitizeReport) {
	var rep SanitizeReport
	rep.ScriptsStripped = len(reScriptTag.FindAll(raw, -1)) + len(reEventAttr.FindAll(raw, -1))
	rep.StyleAttrsStripped = len(reStyleAttr.FindAll(raw, -1))
	rep.ExternalRefsStripped = len(reExternalHREF.FindAll(raw, -1))

	// Lift every <style> block's CSS out and run it through SanitizeCSS,
	// then drop inline <svg> wholesale, before the HTML policy runs.
	var css strings.Builder
	for _, m := range reStyleBlock.FindAllSubmatch(raw, -1) {
		safe, cssRep := SanitizeCSS(m[1])
		rep.ExternalRefsStripped += cssRep.ExternalRefsStripped
		css.Write(safe)
		css.WriteByte('\n')
	}
	stripped := reStyleBlock.ReplaceAll(raw, nil)
	stripped = svgImagesToImg(stripped)
	rep.SVGStripped = len(reSVGOpen.FindAll(stripped, -1))
	stripped = stripSVG(stripped)

	cleaned := htmlPolicy().SanitizeBytes(stripped)
	cleaned = restoreXHTMLRoot(cleaned)

	if css.Len() > 0 {
		cleaned = insertStyle(cleaned, []byte("<style>"+css.String()+"</style>\n"))
	}
	return cleaned, rep
}

const xhtmlNamespace = "http://www.w3.org/1999/xhtml"

var (
	reHTMLOpen = regexp.MustCompile(`(?i)<html\b`)
	reHeadOpen = regexp.MustCompile(`(?i)<head\b[^>]*>`)
	reRootOpen = regexp.MustCompile(`(?i)<html\b[^>]*>`)
)

// restoreXHTMLRoot puts the XHTML namespace back on the root <html>. The
// policy strips every xmlns attribute (a book must not pick its own root
// namespace), but the endpoint serves chapters as application/xhtml+xml,
// where a root outside the XHTML namespace makes every element generic
// XML — no block layout, no heading or paragraph styling.
func restoreXHTMLRoot(doc []byte) []byte {
	loc := reHTMLOpen.FindIndex(doc)
	if loc == nil {
		return doc
	}
	out := make([]byte, 0, len(doc)+len(xhtmlNamespace)+9)
	out = append(out, doc[:loc[1]]...)
	out = append(out, ` xmlns="`+xhtmlNamespace+`"`...)
	return append(out, doc[loc[1]:]...)
}

// insertStyle places the sanitised <style> block inside the document —
// in <head>, else just inside <html> — since anything outside the root
// element is an XML parse error. A fragment with no root gets it
// prepended.
func insertStyle(doc, style []byte) []byte {
	loc := reHeadOpen.FindIndex(doc)
	if loc == nil {
		loc = reRootOpen.FindIndex(doc)
	}
	if loc == nil {
		return append(style, doc...)
	}
	out := make([]byte, 0, len(doc)+len(style))
	out = append(out, doc[:loc[1]]...)
	out = append(out, style...)
	return append(out, doc[loc[1]:]...)
}

var reSVGBlock = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
var reSVGSelfClose = regexp.MustCompile(`(?is)<svg[^>]*/>`)

var (
	reSVGParts    = regexp.MustCompile(`(?is)^<svg[^>]*>(.*)</svg>$`)
	reSVGImage    = regexp.MustCompile(`(?is)<image\b[^>]*>(?:\s*</image>)?`)
	reSVGImageRef = regexp.MustCompile(`(?is)\s(?:xlink:)?href\s*=\s*(?:"([^"<>]*)"|'([^'"<>]*)')`)
)

// svgImagesToImg rewrites an inline <svg> whose only content is a single
// <image> into a plain <img> of the same reference, before stripSVG drops
// every remaining <svg>. EPUB cover pages (Project Gutenberg, Calibre)
// wrap the cover this way, so stripping the SVG left them blank. The <img>
// is produced only for a relative or data: reference (the policy's own
// img src rule, which still runs on it); an SVG holding anything else, or
// pointing anywhere else, is left for stripSVG.
func svgImagesToImg(raw []byte) []byte {
	return reSVGBlock.ReplaceAllFunc(raw, func(block []byte) []byte {
		parts := reSVGParts.FindSubmatch(block)
		if parts == nil {
			return block
		}
		images := reSVGImage.FindAllIndex(parts[1], -1)
		if len(images) != 1 {
			return block
		}
		rest := append(append([]byte{}, parts[1][:images[0][0]]...), parts[1][images[0][1]:]...)
		if len(bytes.TrimSpace(rest)) != 0 {
			return block
		}
		ref := reSVGImageRef.FindSubmatch(parts[1][images[0][0]:images[0][1]])
		if ref == nil {
			return block
		}
		src := ref[1]
		if len(src) == 0 {
			src = ref[2]
		}
		if !reRelativeOrData.Match(src) {
			return block
		}
		return []byte(`<img src="` + string(src) + `" alt=""/>`)
	})
}

func stripSVG(raw []byte) []byte {
	out := reSVGBlock.ReplaceAll(raw, nil)
	out = reSVGSelfClose.ReplaceAll(out, nil)
	return out
}

// SanitizeCSS strips any url()/@import target whose URL has a scheme
// other than data:. A scheme-less (relative) reference is left
// unchanged. This is a bounded token scan rather than a full CSS parser.
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

// schemeDisallowed checks if a URL has a scheme prefix other than data:.
// A bare relative reference (no scheme) is allowed.
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
