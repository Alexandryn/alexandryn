package content_test

import (
	"bytes"
	"encoding/xml"
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
			if !strings.Contains(s, `<img src="images/cover.jpg" alt=""/>`) {
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
