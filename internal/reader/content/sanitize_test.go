package content_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"html"
	"io"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

// Verifies that no <script> or event-handler attribute survives HTML sanitisation.
func TestSanitizeHTML_StripsScriptsAndEventHandlers(t *testing.T) {
	in := []byte(`<p onclick="steal()">hi</p><script>evil()</script><img src="a.png" onerror="x()">`)
	out, rep := content.SanitizeHTML(in)
	s := string(out)

	if strings.Contains(s, "<script") || strings.Contains(s, "evil()") {
		t.Fatalf("script survived: %q", s)
	}
	if strings.Contains(s, "onclick") || strings.Contains(s, "onerror") {
		t.Fatalf("event handler survived: %q", s)
	}
	if !strings.Contains(s, "hi") {
		t.Fatalf("legitimate text was lost: %q", s)
	}
	if rep.ScriptsStripped == 0 {
		t.Fatalf("report did not record a stripped script: %+v", rep)
	}
}

// Verifies that no absolute http(s) href/src survives — external-resource
// network privacy is enforced at the content layer.
func TestSanitizeHTML_StripsAbsoluteExternalURLs(t *testing.T) {
	in := []byte(`<img src="https://tracker.example/pixel.gif"><a href="http://evil.example">x</a><img src="figures/plate.png">`)
	out, rep := content.SanitizeHTML(in)
	s := string(out)

	if strings.Contains(s, "tracker.example") || strings.Contains(s, "evil.example") {
		t.Fatalf("absolute external URL survived: %q", s)
	}
	if !strings.Contains(s, "figures/plate.png") {
		t.Fatalf("relative resource reference should survive: %q", s)
	}
	if rep.ExternalRefsStripped == 0 {
		t.Fatalf("report did not record a stripped external ref: %+v", rep)
	}
}

// Verifies that javascript: URIs in href/src are rejected.
func TestSanitizeHTML_StripsJavascriptURI(t *testing.T) {
	in := []byte(`<a href="javascript:alert(1)">x</a>`)
	out, _ := content.SanitizeHTML(in)
	if strings.Contains(string(out), "javascript:") {
		t.Fatalf("javascript: URI survived: %q", out)
	}
}

// Verifies that inline <svg> is stripped wholesale to eliminate script-execution surface.
func TestSanitizeHTML_StripsInlineSVG(t *testing.T) {
	in := []byte(`<p>before</p><svg onload="x()"><script>y()</script><circle/></svg><p>after</p>`)
	out, rep := content.SanitizeHTML(in)
	s := string(out)

	if strings.Contains(s, "<svg") || strings.Contains(s, "<circle") || strings.Contains(s, "onload") {
		t.Fatalf("inline svg survived: %q", s)
	}
	if !strings.Contains(s, "before") || !strings.Contains(s, "after") {
		t.Fatalf("surrounding content lost: %q", s)
	}
	if rep.SVGStripped == 0 {
		t.Fatalf("report did not record a stripped svg: %+v", rep)
	}
}

// Verifies that style attributes on any element are stripped entirely.
func TestSanitizeHTML_StripsStyleAttributes(t *testing.T) {
	in := []byte(`<p style="background:url(https://x.example/p.gif)">text</p>`)
	out, rep := content.SanitizeHTML(in)
	s := string(out)

	if strings.Contains(s, "style=") || strings.Contains(s, "x.example") {
		t.Fatalf("style attribute survived: %q", s)
	}
	if rep.StyleAttrsStripped == 0 {
		t.Fatalf("report did not record a stripped style attr: %+v", rep)
	}
}

// Verifies that <iframe>, <object>, <embed>, and non-stylesheet <link> elements are stripped.
func TestSanitizeHTML_StripsExternalFetchElements(t *testing.T) {
	in := []byte(`<iframe src="http://evil"></iframe><object data="x"></object><embed src="y"><link rel="stylesheet" href="http://z">`)
	out, _ := content.SanitizeHTML(in)
	s := string(out)
	for _, frag := range []string{"<iframe", "<object", "<embed"} {
		if strings.Contains(s, frag) {
			t.Fatalf("%s survived: %q", frag, s)
		}
	}
	// A stylesheet <link> keeps only a relative/data: href; an http(s)
	// one is stripped, leaving no external fetch.
	if strings.Contains(s, "http://z") {
		t.Fatalf("external stylesheet href survived: %q", s)
	}
}

// Verifies that <style> block text is routed through CSS sanitisation.
func TestSanitizeHTML_RoutesStyleBlockThroughCSS(t *testing.T) {
	in := []byte("<style>body{color:black} .x{background:url(https://tracker.example/p.gif)}</style><p>t</p>")
	out, _ := content.SanitizeHTML(in)
	s := string(out)

	if strings.Contains(s, "tracker.example") {
		t.Fatalf("external url in <style> survived: %q", s)
	}
	if !strings.Contains(s, "color:black") {
		t.Fatalf("legitimate CSS in <style> was lost: %q", s)
	}
}

// Verifies that external url() targets in standalone CSS are stripped while relative targets are retained.
func TestSanitizeCSS_StripsExternalURLsKeepsRelative(t *testing.T) {
	in := []byte(`.a{background:url("https://x.example/bg.png")} .b{background:url(fonts/f.woff2)} @import url(http://evil.example/x.css);`)
	out, rep := content.SanitizeCSS(in)
	s := string(out)

	if strings.Contains(s, "x.example") || strings.Contains(s, "evil.example") {
		t.Fatalf("external url survived CSS sanitisation: %q", s)
	}
	if !strings.Contains(s, "fonts/f.woff2") {
		t.Fatalf("relative url should survive: %q", s)
	}
	if rep.ExternalRefsStripped < 2 {
		t.Fatalf("report undercounted external refs: %+v", rep)
	}
}

// Verifies that javascript: URLs in CSS url() are rejected.
func TestSanitizeCSS_StripsJavascriptURL(t *testing.T) {
	in := []byte(`.a{background:url(javascript:alert(1))}`)
	out, _ := content.SanitizeCSS(in)
	if strings.Contains(string(out), "javascript:") {
		t.Fatalf("javascript: url survived CSS sanitisation: %q", out)
	}
}

// Verifies that data: URIs in CSS are allowed through unchanged.
func TestSanitizeCSS_KeepsDataURI(t *testing.T) {
	in := []byte(`.a{background:url("data:image/png;base64,AAAA")}`)
	out, _ := content.SanitizeCSS(in)
	if !strings.Contains(string(out), "data:image/png;base64,AAAA") {
		t.Fatalf("data: URI was stripped: %q", out)
	}
}

// Verifies that data: URIs on <a href> are stripped to prevent phishing/UI-redress,
// while data: images on <img src> continue to be permitted.
func TestSanitizeHTML_StripsDataURIOnAnchor(t *testing.T) {
	in := []byte(`<a href="data:text/html,<script>alert(1)</script>">link</a><img src="data:image/png;base64,AAAA">`)
	out, _ := content.SanitizeHTML(in)
	s := string(out)

	if strings.Contains(s, "data:text/html") {
		t.Fatalf("data: URI on <a> survived sanitisation: %q", s)
	}
	if !strings.Contains(s, `src="data:image/png;base64,AAAA"`) {
		t.Fatalf("data: URI on <img> should have been preserved: %q", s)
	}
}

// The content endpoint serves sanitised chapters as application/xhtml+xml,
// so the output must stay a well-formed XML document whose root is in the
// XHTML namespace: without the namespace the browser renders <p>/<h2> as
// unstyled generic XML (one run-on block of text), and a <style> block
// outside the root is a second root element — an XML parse error.
func TestSanitizeHTML_StaysWellFormedXHTML(t *testing.T) {
	in := []byte(`<?xml version='1.0' encoding='utf-8'?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en">
<head><meta charset="utf-8"/><title>Ch</title><style>p{text-indent:1em}</style></head>
<body><h2><a id="c2"/>CHAPTER II.<br/>The Pool</h2><p epub:type="z3998:fiction">“Curiouser”&#160;said Alice</p></body>
</html>`)
	out, _ := content.SanitizeHTML(in)

	dec := xml.NewDecoder(bytes.NewReader(out))
	var root *xml.StartElement
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("sanitised output is not well-formed XML: %v\n%s", err, out)
		}
		if se, ok := tok.(xml.StartElement); ok && root == nil {
			root = &se
		}
	}
	if root == nil || root.Name.Local != "html" || root.Name.Space != "http://www.w3.org/1999/xhtml" {
		t.Fatalf("root = %+v, want html in the XHTML namespace\n%s", root, out)
	}
	if !strings.Contains(string(out), "text-indent:1em") {
		t.Fatalf("sanitised <style> was lost: %s", out)
	}
}

// A book cannot choose its own root namespace: whatever xmlns it declares
// is replaced by the XHTML one.
func TestSanitizeHTML_ForcesXHTMLNamespace(t *testing.T) {
	out, _ := content.SanitizeHTML([]byte(`<html xmlns="http://www.w3.org/2000/svg"><body><p>t</p></body></html>`))
	s := string(out)
	if strings.Contains(s, "2000/svg") || strings.Count(s, "xmlns=") != 1 || !strings.Contains(s, `<html xmlns="http://www.w3.org/1999/xhtml"`) {
		t.Fatalf("root namespace not forced to XHTML: %s", s)
	}
}

// EPUB cover pages (Project Gutenberg, Calibre) wrap the cover in an
// inline <svg> holding one <image>. Inline SVG is stripped wholesale, which
// left every such cover page blank; a lone <image> becomes a plain <img>.
func TestSanitizeHTML_SVGWrappedImageBecomesImg(t *testing.T) {
	cases := map[string]string{
		"xlink:href": `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 800 1104"><image width="800" height="1104" xlink:href="images/cover.jpg"/></svg>`,
		"href":       `<svg viewBox="0 0 1 1"><image href='images/cover.jpg'></image></svg>`,
	}
	for name, svg := range cases {
		t.Run(name, func(t *testing.T) {
			out, rep := content.SanitizeHTML([]byte(`<html><body><div class="cover">` + svg + `</div></body></html>`))
			s := string(out)
			if !strings.Contains(s, `<img src="images/cover.jpg" alt="" class="alx-svg-cover"/>`) {
				t.Fatalf("cover image not kept as <img>: %s", s)
			}
			if strings.Contains(strings.ToLower(s), "<svg") || strings.Contains(s, "<image") {
				t.Fatalf("svg survived: %s", s)
			}
			if rep.SVGStripped != 0 {
				t.Errorf("SVGStripped = %d, want 0 for a converted cover", rep.SVGStripped)
			}
		})
	}
}

// The conversion never widens what an <img> may load: the policy's
// relative-or-data src check still applies, and an SVG with anything
// besides one image is stripped as before.
func TestSanitizeHTML_SVGImageConversionStaysStrict(t *testing.T) {
	cases := map[string]string{
		"external href":   `<svg><image xlink:href="https://tracker.example/p.gif"/></svg>`,
		"protocol-rel":    `<svg><image href="//tracker.example/p.gif"/></svg>`,
		"two images":      `<svg><image href="a.jpg"/><image href="b.jpg"/></svg>`,
		"image plus text": `<svg><image href="a.jpg"/><text>hi</text></svg>`,
		"quote breakout":  `<svg><image href='a.jpg" onerror="x()'/></svg>`,
	}
	for name, svg := range cases {
		t.Run(name, func(t *testing.T) {
			out, _ := content.SanitizeHTML([]byte(`<html><body>` + svg + `</body></html>`))
			s := string(out)
			if strings.Contains(s, "<img") || strings.Contains(s, "tracker") || strings.Contains(s, "onerror") {
				t.Fatalf("unsafe or ambiguous svg converted: %s", s)
			}
		})
	}
}

// bluemonday matches href/src against the untrimmed value but writes the
// trimmed one, and browsers also drop leading C0 controls, tabs, and
// newlines from a URL — so " //host", "\t//host", "\n//host" or
// "\x01//host" passed the relative-only check and came out as a
// protocol-relative reference to another host.
func TestSanitizeHTML_LeadingWhitespaceCannotSmuggleProtocolRelative(t *testing.T) {
	for _, lead := range []string{" ", "  ", "\t", "\n", "&#10;", "&#9;", "\x01", "\x1f"} {
		for _, doc := range []string{
			`<a href="` + lead + `//evil.example/">x</a>`,
			`<img src="` + lead + `//evil.example/x.png"/>`,
			`<link rel="stylesheet" href="` + lead + `//evil.example/x.css"/>`,
			`<a href="` + lead + `/\evil.example/">x</a>`,
			`<img src="/` + lead + `/evil.example/x.png"/>`,
		} {
			out, _ := content.SanitizeHTML([]byte(`<html><body>` + doc + `</body></html>`))
			if strings.Contains(string(out), "evil.example") {
				t.Errorf("%q survived as %s", doc, out)
			}
		}
	}
}

// Real-world cover markup the conversion must handle: Calibre's
// multi-line form, Sigil's ../Images path, uppercase tags, a data: cover,
// and accessibility-polished covers whose <svg> also carries a <title>,
// <desc>, or comment — the <title> becoming the image's alt text.
func TestSanitizeHTML_SVGCoverVariants(t *testing.T) {
	cases := []struct{ name, svg, want string }{
		{"calibre multi-line", `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
    version="1.1" width="100%" height="100%" viewBox="0 0 1200 1800" preserveAspectRatio="none">
    <image width="1200" height="1800"
        xlink:href="cover.jpeg"/>
</svg>`, `<img src="cover.jpeg" alt="" class="alx-svg-cover"/>`},
		{"sigil path", `<svg><image xlink:href="../Images/cover.jpg"/></svg>`, `<img src="../Images/cover.jpg" alt="" class="alx-svg-cover"/>`},
		{"uppercase", `<SVG><IMAGE XLINK:HREF="c.jpg"/></SVG>`, `<img src="c.jpg" alt="" class="alx-svg-cover"/>`},
		{"data cover", `<svg><image href="data:image/png;base64,iVBORw0KGgo="/></svg>`, `<img src="data:image/png;base64,iVBORw0KGgo=" alt="" class="alx-svg-cover"/>`},
		{"title as alt", `<svg><title>Cover: Alice &amp; the Queen</title><image xlink:href="c.jpg"/></svg>`, `<img src="c.jpg" alt="Cover: Alice &amp; the Queen" class="alx-svg-cover"/>`},
		{"desc and comment", `<svg><!-- cover --><desc>The front cover</desc><image xlink:href="c.jpg"/></svg>`, `<img src="c.jpg" alt="" class="alx-svg-cover"/>`},
		{"href beats xlink:href", `<svg><image xlink:href="old.jpg" href="new.jpg"/></svg>`, `<img src="new.jpg" alt="" class="alx-svg-cover"/>`},
		{"href inside another attribute ignored", `<svg><image title=' href="evil.jpg"' xlink:href="c.jpg"/></svg>`, `<img src="c.jpg" alt="" class="alx-svg-cover"/>`},
		{"ampersand in path", `<svg><image href="a&amp;b.jpg"/></svg>`, `<img src="a&amp;b.jpg" alt="" class="alx-svg-cover"/>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _ := content.SanitizeHTML([]byte(`<html><body>` + c.svg + `</body></html>`))
			if !strings.Contains(string(out), c.want) {
				t.Fatalf("want %s in\n%s", c.want, out)
			}
		})
	}
}

// A converted cover is not "stripped SVG"; one that is dropped still is.
func TestSanitizeHTML_SVGStrippedCountsOnlyDropped(t *testing.T) {
	_, rep := content.SanitizeHTML([]byte(`<html><body><svg><image href="c.jpg"/></svg><svg><circle r="1"/></svg></body></html>`))
	if rep.SVGStripped != 1 {
		t.Fatalf("SVGStripped = %d, want 1", rep.SVGStripped)
	}
}

// Cover markup the regex-based conversion missed, and the context errors
// it made: found by review, each confirmed against the old code.
func TestSanitizeHTML_SVGCoverContext(t *testing.T) {
	const cover = `class="alx-svg-cover"/>`
	cases := []struct{ name, in, want, notWant string }{
		{"self-closing svg does not swallow the chapter",
			`<svg/><p>keep</p><svg><image href="c.jpg"/></svg>`, `<img src="c.jpg" alt="" ` + cover, ""},
		{"self-closing svg keeps following text",
			`<svg/><p>keep</p><svg><image href="c.jpg"/></svg>`, `<p>keep</p>`, ""},
		{"svg in an attribute value is text, not markup",
			`<p title="<svg><image href='x class=evil id=pwn '/></svg>">t</p>`, `<p title=`, `id="pwn"`},
		{"prefixed svg:svg cover",
			`<svg:svg xmlns:svg="http://www.w3.org/2000/svg"><svg:image xlink:href="cover.jpg"/></svg:svg>`, `<img src="cover.jpg" alt="" ` + cover, ""},
		{"cover wrapped in a group",
			`<svg viewBox="0 0 1 1"><g><image href="cover.jpg"/></g></svg>`, `<img src="cover.jpg" alt="" ` + cover, ""},
		{"metadata sibling",
			`<svg><metadata><rdf:RDF><x>y</x></rdf:RDF></metadata><image href="cover.jpg"/></svg>`, `<img src="cover.jpg" alt="" ` + cover, ""},
		{"space in file name",
			`<svg><image href="my cover.jpg"/></svg>`, `<img src="my%20cover.jpg" alt="" ` + cover, ""},
		{"apostrophe in file name",
			`<svg><image href="Alice's cover.jpg"/></svg>`, `<img src="Alice&#39;s%20cover.jpg" alt="" ` + cover, ""},
		{"CDATA title keeps its text",
			`<svg><title><![CDATA[Alice]]></title><image href="c.jpg"/></svg>`, `alt="Alice"`, ""},
		{"unclosed svg does not swallow the chapter",
			`<svg><image href="c.jpg"/><p>keep</p>`, `keep`, "<img"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _ := content.SanitizeHTML([]byte(`<html><body>` + c.in + `</body></html>`))
			s := string(out)
			if !strings.Contains(s, c.want) {
				t.Errorf("want %s in\n%s", c.want, s)
			}
			if c.notWant != "" && strings.Contains(s, c.notWant) {
				t.Errorf("unwanted %s in\n%s", c.notWant, s)
			}
			assertWellFormedXHTML(t, out)
		})
	}
}

// Markup inside a <style> block never reaches the HTML policy, so it must
// come back as CSS text, not as live elements.
func TestSanitizeHTML_StyleBlockCannotInjectMarkup(t *testing.T) {
	in := `<html><head><style>style{display:block} <a href="https://evil.example/" style="position:fixed;inset:0">Continue</a> p{color:red}</style></head><body><p>t</p></body></html>`
	out, _ := content.SanitizeHTML([]byte(in))
	assertWellFormedXHTML(t, out)
	dec := xml.NewDecoder(bytes.NewReader(out))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "a" {
			t.Fatalf("an <a> element survived from inside <style>:\n%s", out)
		}
	}
	if !strings.Contains(string(out), "p{color:red}") {
		t.Fatalf("legitimate CSS lost: %s", out)
	}
}

// XHTML books wrap <style> text in CDATA; the markers are dropped, not
// escaped into the CSS.
func TestSanitizeHTML_StyleCDATAUnwrapped(t *testing.T) {
	out, _ := content.SanitizeHTML([]byte(`<html><head><style>/*<![CDATA[*/p{color:red}/*]]>*/</style></head><body/></html>`))
	if strings.Contains(string(out), "CDATA") || !strings.Contains(string(out), "p{color:red}") {
		t.Fatalf("CDATA not unwrapped: %s", out)
	}
}

// foliate-js finds an EPUB 3 book's table of contents by
// nav[epub:type~=toc] in the epub namespace; without epub:type and the
// prefix declaration the contents list is empty.
func TestSanitizeHTML_KeepsEPUBType(t *testing.T) {
	in := `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc" id="toc"><ol><li><a href="c1.xhtml">One</a></li></ol></nav></body></html>`
	out, _ := content.SanitizeHTML([]byte(in))
	assertWellFormedXHTML(t, out)
	dec := xml.NewDecoder(bytes.NewReader(out))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "nav" {
			for _, a := range se.Attr {
				if a.Name.Space == "http://www.idpf.org/2007/ops" && a.Name.Local == "type" && a.Value == "toc" {
					return
				}
			}
			t.Fatalf("nav lost epub:type in the epub namespace: %+v\n%s", se.Attr, out)
		}
	}
	t.Fatalf("nav element missing:\n%s", out)
}

// bluemonday trims Unicode whitespace too, so a leading U+00A0, U+3000,
// or U+0085 must not smuggle a protocol-relative reference either.
func TestSanitizeHTML_UnicodeWhitespaceCannotSmuggleProtocolRelative(t *testing.T) {
	for _, lead := range []string{"\u00a0", "\u3000", "\u0085", "\u2000", "\u2028", "\u202f", "\x7f"} {
		for _, doc := range []string{
			`<a href="` + lead + `//evil.example/">x</a>`,
			`<img src="` + lead + `//evil.example/x.png"/>`,
			`<a href="/` + lead + `/evil.example/">x</a>`,
			`<svg><image href="` + lead + `//evil.example/c.jpg"/></svg>`,
		} {
			out, _ := content.SanitizeHTML([]byte(`<html><body>` + doc + `</body></html>`))
			if strings.Contains(string(out), "//evil.example") {
				t.Errorf("%q survived as %s", doc, out)
			}
		}
	}
}

func assertWellFormedXHTML(t *testing.T, out []byte) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(out))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("not well-formed XML: %v\n%s", err, out)
		}
	}
}

// A chapter with no <html> root still has to be a namespaced XHTML
// document: epub:type needs its prefix bound, and a bare fragment is not
// rendered as XHTML at all.
func TestSanitizeHTML_RootlessFragmentBecomesDocument(t *testing.T) {
	out, _ := content.SanitizeHTML([]byte(`<style>p{color:red}</style><p epub:type="footnote">frag</p><p>two</p>`))
	assertWellFormedXHTML(t, out)
	s := string(out)
	if !strings.HasPrefix(s, `<html xmlns="http://www.w3.org/1999/xhtml"`) || !strings.Contains(s, "frag") || !strings.Contains(s, "p{color:red}") {
		t.Fatalf("fragment not wrapped as an XHTML document: %s", s)
	}
}

// An unclosed <svg> drops only its own tags; what follows it — text and
// <style> blocks alike — is processed as if the svg were not there.
func TestSanitizeHTML_UnclosedSVGKeepsLaterStyle(t *testing.T) {
	out, rep := content.SanitizeHTML([]byte(`<html><head></head><body><svg>unclosed<style>p{color:red}</style><p>after</p></body></html>`))
	s := string(out)
	if !strings.Contains(s, "after") || !strings.Contains(s, "p{color:red}") {
		t.Fatalf("content after an unclosed svg lost: %s", s)
	}
	if rep.SVGStripped != 1 {
		t.Errorf("SVGStripped = %d, want 1", rep.SVGStripped)
	}
}

// The content CSP has no 'unsafe-inline', so the one sanitised <style>
// block applies only if the response names its hash; SanitizeHTML
// reports it.
func TestSanitizeHTML_ReportsStyleHash(t *testing.T) {
	out, rep := content.SanitizeHTML([]byte(`<html><head><style>h1{text-align:center}</style></head><body/></html>`))
	start := strings.Index(string(out), "<style>") + len("<style>")
	end := strings.Index(string(out), "</style>")
	text := html.UnescapeString(string(out[start:end]))
	sum := sha256.Sum256([]byte(text))
	if want := "sha256-" + base64.StdEncoding.EncodeToString(sum[:]); rep.StyleHash != want {
		t.Fatalf("StyleHash = %q, want %q (hash of the style element's text)", rep.StyleHash, want)
	}
	if _, rep := content.SanitizeHTML([]byte(`<p>no style</p>`)); rep.StyleHash != "" {
		t.Fatalf("StyleHash = %q for a chapter with no style", rep.StyleHash)
	}
}
