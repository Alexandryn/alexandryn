// Package content resolves, sanitises, and serves individual entries
// from inside an owned Edition's EPUB archive. Every byte served as
// HTML or CSS passes an allowlist sanitiser before returning to the client.
package content

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	xhtml "golang.org/x/net/html"
)

// SanitizeReport counts what a pass removed, for observability logging:
// categories only, never the offending content or the surrounding
// document.
type SanitizeReport struct {
	ScriptsStripped      int
	ExternalRefsStripped int
	SVGStripped          int
	StyleAttrsStripped   int

	// StyleHash is the CSP source ("sha256-…") for the document's one
	// sanitised <style> element, empty when there is none. Not a stripping
	// count: the content CSP has no 'unsafe-inline', so the response must
	// name this hash for the book's own inline CSS to apply.
	StyleHash string
}

func (r SanitizeReport) StrippedSomething() bool {
	return r.ScriptsStripped+r.ExternalRefsStripped+r.SVGStripped+r.StyleAttrsStripped > 0
}

// refLead and refSecond are the character classes a relative reference's
// first character, and the character after a leading "/", must fall in:
// no "/", ":" (first only), "\\", C0 control, space, DEL, U+0085, or any
// Unicode space separator — everything strings.TrimSpace or a browser's
// URL parser would silently drop.
const (
	refLead   = `[^/:\\\x00-\x20\x7f\x{85}\p{Z}]`
	refSecond = `[^/\\\x00-\x20\x7f\x{85}\p{Z}]`
)

var (
	// Pre-pass detectors — bluemonday removes disallowed constructs
	// silently, so the counts come from scanning the input, not diffing
	// the output. The policy itself is what actually enforces removal.
	reScriptTag    = regexp.MustCompile(`(?is)<script[\s>]`)
	reEventAttr    = regexp.MustCompile(`(?is)\son[a-z]+\s*=`)
	reStyleAttr    = regexp.MustCompile(`(?is)\sstyle\s*=`)
	reExternalHREF = regexp.MustCompile(`(?is)\b(?:href|src)\s*=\s*["']?\s*(?:[a-z][a-z0-9+.-]*:)?//`)

	// CSS: a url(...) token or an @import target. RE2 has no
	// backreferences, so quotes are matched as optional single chars and
	// trimmed from the captured value afterwards.
	reCSSURL    = regexp.MustCompile(`(?is)url\(\s*['"]?([^'")]*?)['"]?\s*\)`)
	reCSSImport = regexp.MustCompile(`(?is)@import\s+(?:url\(\s*['"]?([^'")]*?)['"]?\s*\)|['"]([^'"]*)['"])\s*;?`)
	reScheme    = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

	// A value bluemonday may keep on href/src: a data: URI, or a
	// relative reference that does not begin with "//" (protocol-relative).
	// Neither of the first two characters may be whitespace (ASCII or
	// Unicode), a control character, or a backslash: bluemonday tests the
	// untrimmed value but writes the strings.TrimSpace'd one, browsers
	// drop leading C0 controls and any tab/newline from a URL, and treat
	// "\" as "/" — so " //host", "\u00a0//host", "\x01//host",
	// "/\t/host", and "/\host" would all become "//host".
	reRelativeOrData = regexp.MustCompile(`(?is)^(?:data:\S+|(?:` + refLead + `|/` + refSecond + `)[^:]*|#\S*)$`)

	// A relative-only reference that does not begin with "//" and rejects data: schemes.
	// Used on <a href> so that books cannot embed data:text/html anchors to prevent UI-redress.
	// Same leading-character rule as reRelativeOrData.
	reRelativeOnly = regexp.MustCompile(`(?is)^(?:(?:` + refLead + `|/` + refSecond + `)[^:]*|#\S*)$`)
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
	// EPUB 3 structural semantics: foliate-js finds the table of contents,
	// page list, and landmarks by nav[epub:type~=toc] and friends. Tokens
	// only; the epub prefix is declared on the root by finishXHTML.
	p.AllowAttrs("epub:type").Matching(regexp.MustCompile(`^[A-Za-z0-9:_ \-]*$`)).Globally()
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

// SanitizeHTML strips scripts, event handlers, inline <svg> (keeping an
// SVG-wrapped cover image as a plain <img>), style attributes,
// external-fetch elements, and any non-relative, non-data: href/src
// attributes; routes <style> block text through SanitizeCSS. The result is
// a well-formed XHTML document for the application/xhtml+xml response.
func SanitizeHTML(raw []byte) ([]byte, SanitizeReport) {
	var rep SanitizeReport
	rep.ScriptsStripped = len(reScriptTag.FindAll(raw, -1)) + len(reEventAttr.FindAll(raw, -1))
	rep.StyleAttrsStripped = len(reStyleAttr.FindAll(raw, -1))
	rep.ExternalRefsStripped = len(reExternalHREF.FindAll(raw, -1))

	pre := prepass(raw)
	rep.SVGStripped = pre.svgStripped

	var style []byte
	if len(pre.css) > 0 {
		safe, cssRep := SanitizeCSS(pre.css)
		rep.ExternalRefsStripped += cssRep.ExternalRefsStripped
		// The XML parser normalises line ends in text, so the hash is taken
		// over the text exactly as the browser will see it.
		text := strings.ReplaceAll(strings.ReplaceAll(string(safe), "\r\n", "\n"), "\r", "\n")
		// Escaped as text: the lifted CSS never reaches the HTML policy,
		// so any markup smuggled inside a <style> must not come back as
		// markup. The XML parser decodes the entities back into the CSS.
		style = []byte("<style>" + html.EscapeString(text) + "</style>\n")
		sum := sha256.Sum256([]byte(text))
		rep.StyleHash = "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	}

	return finishXHTML(htmlPolicy().SanitizeBytes(pre.out), style), rep
}

const (
	xhtmlNamespace = "http://www.w3.org/1999/xhtml"
	epubNamespace  = "http://www.idpf.org/2007/ops"

	// svgCoverClass marks an <img> converted from an SVG-wrapped cover, so
	// the reader can fit it to the page the way the SVG's viewBox did.
	svgCoverClass = "alx-svg-cover"
)

var (
	reRootOpen = regexp.MustCompile(`(?i)<html\b[^>]*>`)
	reHeadOpen = regexp.MustCompile(`(?i)<head\b[^>]*>`)
)

// finishXHTML makes the policy's output a document the browser renders
// as XHTML. The policy strips every xmlns attribute (a book must not pick
// its own namespaces), so the root gets the fixed XHTML namespace and the
// epub prefix back; and the sanitised <style> goes inside <head>, else
// inside <html>, since anything outside the root is an XML parse error.
// A fragment with no root is wrapped in one, since a bare fragment is
// neither a single-rooted document nor in the XHTML namespace. The policy
// escapes every "<" in text, so each match below is a real tag.
func finishXHTML(doc, style []byte) []byte {
	ns := ` xmlns="` + xhtmlNamespace + `" xmlns:epub="` + epubNamespace + `"`
	root := reRootOpen.FindIndex(doc)
	if root == nil {
		out := make([]byte, 0, len(doc)+len(ns)+len(style)+48)
		out = append(out, "<html"+ns+"><head>"...)
		out = append(out, style...)
		out = append(out, "</head><body>"...)
		out = append(out, doc...)
		return append(out, "</body></html>"...)
	}
	at := root[1]
	if head := reHeadOpen.FindIndex(doc[root[1]:]); head != nil {
		at = root[1] + head[1]
	}
	out := make([]byte, 0, len(doc)+len(ns)+len(style))
	out = append(out, doc[:root[0]+len("<html")]...)
	out = append(out, ns...)
	out = append(out, doc[root[0]+len("<html"):at]...)
	out = append(out, style...)
	return append(out, doc[at:]...)
}

type prepassResult struct {
	out         []byte
	css         []byte
	svgStripped int
}

// prepass runs before the HTML policy, over the same tokenizer the policy
// itself uses, so both agree on what is a tag, an attribute value, or
// raw text. It lifts every <style> block's text out (for SanitizeCSS)
// and removes every inline <svg>, except that an SVG which is only a
// wrapper around one image — the EPUB cover-page idiom — becomes a plain
// <img> (see svgCover). Everything else is copied through untouched.
func prepass(raw []byte) prepassResult {
	res := prepassFrom(raw, true)
	res.css = stripCDATAMarkers(res.css)
	return res
}

// prepassFrom is prepass's loop. With captureSVG false (the replay of an
// unclosed <svg>'s contents) <svg> tags are dropped where they stand
// rather than buffered, so a replay never replays again.
func prepassFrom(raw []byte, captureSVG bool) prepassResult {
	var res prepassResult
	out := make([]byte, 0, len(raw))
	z := xhtml.NewTokenizer(bytes.NewReader(raw))

	var svg *svgCover // non-nil while inside an <svg>
	var svgRaw []byte // raw bytes after the open <svg>, for an unclosed one
	inStyle := false

	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			break
		}
		// Copy the raw token out first, optimistically into the output:
		// TagName lowercases the buffer in place and consumes the tag,
		// after which only TagAttr can still read its attributes. A token
		// that is not passed through is taken back off the end.
		mark := len(out)
		out = append(out, z.Raw()...)
		name, hasAttr := z.TagName()
		tag := svgLocal(string(name))

		if svg != nil {
			svgRaw = append(svgRaw, out[mark:]...)
			tok := out[mark:]
			out = out[:mark]
			if svg.feed(tt, tag, hasAttr, z, tok) {
				if img := svg.img(); img != nil {
					out = append(out, img...)
				} else {
					res.svgStripped++
				}
				svg, svgRaw = nil, nil
			}
			continue
		}

		switch {
		case inStyle:
			if tt == xhtml.EndTagToken && string(name) == "style" {
				inStyle = false
				res.css = append(res.css, '\n')
			} else if tt == xhtml.TextToken {
				res.css = append(res.css, out[mark:]...)
			}
			out = out[:mark]
		case string(name) == "style" && tt == xhtml.StartTagToken:
			inStyle = true
			out = out[:mark]
		case string(name) == "style" && (tt == xhtml.SelfClosingTagToken || tt == xhtml.EndTagToken):
			out = out[:mark]
		case tag == "svg" && (tt == xhtml.SelfClosingTagToken || tt == xhtml.EndTagToken || !captureSVG):
			if tt != xhtml.EndTagToken {
				res.svgStripped++
			}
			out = out[:mark]
		case tag == "svg" && tt == xhtml.StartTagToken:
			svg = &svgCover{depth: 1}
			out = out[:mark]
		}
	}
	if svg != nil {
		// An <svg> never closed: drop only its open tag and process what
		// followed as if it were not there — text and <style> alike —
		// rather than swallow the rest of the chapter.
		res.svgStripped++
		rest := prepassFrom(svgRaw, false)
		out = append(out, rest.out...)
		res.css = append(res.css, rest.css...)
		res.svgStripped += rest.svgStripped
	}
	res.out = out
	return res
}

// svgLocal drops an "svg:" prefix, so namespace-prefixed SVG
// (<svg:svg>, <svg:image>, common in older Adobe-exported EPUBs) is read
// the same as the unprefixed form.
func svgLocal(name string) string {
	return strings.TrimPrefix(name, "svg:")
}

// svgCover accumulates one <svg> element and decides whether it is a
// cover wrapper: exactly one <image>, optionally inside <g> groups, beside
// optional <title>, <desc>, <metadata>, comments, and whitespace. Any
// other element, nested <svg>, or text makes it ordinary SVG, which is
// dropped.
type svgCover struct {
	depth    int    // open <svg> elements
	skip     string // inside <desc>/<metadata>/<title>: its tag name
	skipN    int    // nesting of skip
	images   int
	attrs    []xhtml.Attribute
	title    strings.Builder
	notCover bool
}

// feed consumes one token inside the <svg> and reports whether the
// outermost </svg> has just closed.
func (c *svgCover) feed(tt xhtml.TokenType, tag string, hasAttr bool, z *xhtml.Tokenizer, raw []byte) bool {
	if c.skip != "" {
		switch {
		case tt == xhtml.StartTagToken && tag == c.skip:
			c.skipN++
		case tt == xhtml.EndTagToken && tag == c.skip:
			c.skipN--
			if c.skipN == 0 {
				c.skip = ""
			}
		case tt == xhtml.TextToken && c.skip == "title" && c.skipN == 1:
			c.title.Write(z.Text())
		}
		return false
	}
	switch tt {
	case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
		switch tag {
		case "svg":
			c.notCover = true
			if tt == xhtml.StartTagToken {
				c.depth++
			}
		case "image":
			c.images++
			for c.images == 1 && hasAttr {
				var k, v []byte
				k, v, hasAttr = z.TagAttr()
				c.attrs = append(c.attrs, xhtml.Attribute{Key: string(k), Val: string(v)})
			}
		case "g":
		case "title", "desc", "metadata":
			if tt == xhtml.StartTagToken {
				c.skip, c.skipN = tag, 1
			}
		default:
			c.notCover = true
		}
	case xhtml.EndTagToken:
		if tag == "svg" {
			c.depth--
			return c.depth == 0
		}
	case xhtml.TextToken:
		if len(bytes.TrimSpace(raw)) != 0 {
			c.notCover = true
		}
	}
	return false
}

// img returns the <img> replacing a cover wrapper, or nil when the SVG
// is not one or its reference is not a relative or data: one (the
// policy's own img src rule, which still runs on the result). Spaces are
// percent-encoded, as a browser would; the SVG's <title> becomes the alt.
func (c *svgCover) img() []byte {
	if c.notCover || c.images != 1 {
		return nil
	}
	src, ok := svgImageRef(c.attrs)
	if !ok || strings.ContainsAny(src, `"<>`) {
		return nil
	}
	src = strings.ReplaceAll(src, " ", "%20")
	if !reRelativeOrData.MatchString(src) {
		return nil
	}
	alt := strings.Join(strings.Fields(string(stripCDATAMarkers([]byte(c.title.String())))), " ")
	return []byte(`<img src="` + html.EscapeString(src) + `" alt="` + html.EscapeString(alt) +
		`" class="` + svgCoverClass + `"/>`)
}

// svgImageRef picks an <image>'s reference. SVG 2's href wins over the
// legacy xlink:href, as it does in browsers.
func svgImageRef(attrs []xhtml.Attribute) (string, bool) {
	var xlinkHref string
	var haveXlink bool
	for _, a := range attrs {
		switch {
		case a.Key == "href":
			return a.Val, true
		case a.Key == "xlink:href" && !haveXlink:
			xlinkHref, haveXlink = a.Val, true
		}
	}
	return xlinkHref, haveXlink
}

// stripCDATAMarkers removes "<![CDATA[" and "]]>" wrappers, which XHTML
// books put around <style> text and SVG titles and which carry no
// content of their own.
func stripCDATAMarkers(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("<![CDATA["), nil)
	return bytes.ReplaceAll(b, []byte("]]>"), nil)
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
