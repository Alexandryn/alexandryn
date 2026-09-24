package content_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

func TestValidateResourcePath(t *testing.T) {
	bad := []string{"", "/etc/passwd", "../secret", "a/../../b", "OEBPS/../../x", `a\b`, "./a"}
	for _, p := range bad {
		if err := content.ValidateResourcePath(p); err == nil {
			t.Fatalf("path %q should be rejected", p)
		} else if domain.CategoryOf(err) != domain.InvalidInput {
			t.Fatalf("path %q: category = %v, want InvalidInput", p, domain.CategoryOf(err))
		}
	}
	good := []string{"OEBPS/chapter1.xhtml", "images/plate.png", "styles/main.css", "a/b/c.xhtml"}
	for _, p := range good {
		if err := content.ValidateResourcePath(p); err != nil {
			t.Fatalf("path %q should be accepted: %v", p, err)
		}
	}
}

func TestClassify(t *testing.T) {
	htmlBytes := []byte("<!DOCTYPE html><html><body><p>hi</p></body></html>")
	if k, ct, err := content.Classify("OEBPS/c1.xhtml", htmlBytes); err != nil || k != content.KindHTML {
		t.Fatalf("xhtml: kind=%v ct=%q err=%v", k, ct, err)
	}
	if k, _, err := content.Classify("styles/main.css", []byte("body{color:red}")); err != nil || k != content.KindCSS {
		t.Fatalf("css: kind=%v err=%v", k, err)
	}
	pngBytes := []byte("\x89PNG\r\n\x1a\n0000000000000000")
	if k, ct, err := content.Classify("images/a.png", pngBytes); err != nil || k != content.KindBinary || ct != "image/png" {
		t.Fatalf("png: kind=%v ct=%q err=%v", k, ct, err)
	}
	// Standalone SVG is refused, not sanitised-and-served.
	if _, _, err := content.Classify("images/cover.svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>")); domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("svg should be InvalidInput, got %v", err)
	}
	// An embedded executable is refused.
	if _, _, err := content.Classify("x.bin", []byte("MZ\x90\x00\x03\x00\x00\x00")); domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("executable should be InvalidInput, got %v", err)
	}
	// A woff2 font serves as binary.
	if k, ct, err := content.Classify("fonts/f.woff2", []byte("wOF2\x00\x01\x00\x00")); err != nil || k != content.KindBinary || ct != "font/woff2" {
		t.Fatalf("woff2: kind=%v ct=%q err=%v", k, ct, err)
	}
	// The three EPUB structural XML files foliate-js fetches by path to
	// locate the spine — container.xml, the OPF package document, and
	// (EPUB2) the NCX table of contents — must all be servable, or no
	// real book can ever open. Regression coverage for the bug where
	// every one of these was refused with "this resource type cannot be
	// served", discovered by opening a real EPUB against a real server.
	xmlCases := []string{
		"META-INF/container.xml",
		"OEBPS/content.opf",
		"OEBPS/toc.ncx",
	}
	xmlBytes := []byte(`<?xml version="1.0" encoding="UTF-8"?><root/>`)
	for _, name := range xmlCases {
		k, ct, err := content.Classify(name, xmlBytes)
		if err != nil || k != content.KindXML || ct != "application/xml; charset=utf-8" {
			t.Fatalf("%s: kind=%v ct=%q err=%v", name, k, ct, err)
		}
	}
}

// Real structural files, with and without a prolog, and one in a legacy
// single-byte encoding: all served as XML, with no forced charset so the
// file's own declaration governs.
func TestClassify_StructuralXML(t *testing.T) {
	cases := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Alice</dc:title></metadata></package>`,
		"OEBPS/toc.ncx":          "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><ncx xmlns=\"http://www.daisy.org/z3986/2005/ncx/\"><docTitle><text>Caf\xe9</text></docTitle></ncx>",
	}
	for name, body := range cases {
		k, ct, err := content.Classify(name, []byte(body))
		if err != nil || k != content.KindXML || ct != "application/xml; charset=utf-8" {
			t.Errorf("%s: kind=%v ct=%q err=%v", name, k, ct, err)
		}
	}
}

// XML is served unsanitised, so it must never carry anything a browser
// renders as a live document: an SVG, an XHTML chapter, XHTML/SVG/MathML
// elements nested under another root, or an XSLT stylesheet.
func TestClassify_XMLNeverServesRenderableMarkup(t *testing.T) {
	refused := map[string]string{
		"images/cover.svg with prolog": `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"svg root in .xml":             `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><a href="https://evil.example/"/></svg>`,
		"nested xhtml":                 `<?xml version="1.0"?><root><a xmlns="http://www.w3.org/1999/xhtml" href="https://evil.example/">x</a></root>`,
		"nested svg via prefix":        `<?xml version="1.0"?><root xmlns:s="http://www.w3.org/2000/svg"><s:svg/></root>`,
		"mathml":                       `<?xml version="1.0"?><math xmlns="http://www.w3.org/1998/Math/MathML"/>`,
		"xslt stylesheet":              `<?xml version="1.0"?><?xml-stylesheet type="text/xsl" href="evil.xsl"?><root/>`,
		"xslt element":                 `<?xml version="1.0"?><xsl:stylesheet xmlns:xsl="http://www.w3.org/1999/XSL/Transform" version="1.0"/>`,
		"malformed":                    `<?xml version="1.0"?><root><unclosed></root>`,
	}
	for name, body := range refused {
		entry := "OEBPS/x.xml"
		if strings.HasPrefix(name, "images/") {
			entry = "images/cover.svg"
		}
		if k, ct, err := content.Classify(entry, []byte(body)); domain.CategoryOf(err) != domain.InvalidInput {
			t.Errorf("%s: served as kind=%v ct=%q, want InvalidInput", name, k, ct)
		}
	}
}

// An XHTML chapter with a .xml name is a content document: it goes
// through the HTML sanitiser, never out raw.
func TestClassify_XHTMLRootedXMLIsSanitised(t *testing.T) {
	body := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>x</p></body></html>`
	if k, ct, err := content.Classify("OEBPS/ch1.xml", []byte(body)); err != nil || k != content.KindHTML || ct != "application/xhtml+xml; charset=utf-8" {
		t.Fatalf("kind=%v ct=%q err=%v, want KindHTML", k, ct, err)
	}
}

// The extension alone never makes a binary servable as XML.
func TestClassify_BinaryWithXMLExtensionRefused(t *testing.T) {
	for _, name := range []string{"OEBPS/payload.opf", "META-INF/x.xml", "OEBPS/toc.ncx"} {
		for _, body := range [][]byte{[]byte("MZ\x90\x00\x03\x00\x00\x00"), []byte("%PDF-1.7\n"), []byte("PK\x03\x04\x14\x00")} {
			if k, ct, err := content.Classify(name, body); domain.CategoryOf(err) != domain.InvalidInput {
				t.Errorf("%s %q: served as kind=%v ct=%q", name, body[:2], k, ct)
			}
		}
	}
}

// A DTD internal subset can give elements a default xmlns that
// encoding/xml never sees but a browser applies, smuggling SVG or XHTML
// past the namespace check. XML with an internal subset is refused; an
// external DOCTYPE (EPUB 2 NCX/OPF) is fine.
func TestClassify_XMLInternalSubsetRefused(t *testing.T) {
	smuggled := `<!DOCTYPE r [<!ATTLIST svg xmlns CDATA #FIXED "http://www.w3.org/2000/svg">]><r><svg><a href="x"><rect width="100" height="100"/></a></svg></r>`
	if k, ct, err := content.Classify("OEBPS/x.xml", []byte(smuggled)); domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("internal subset served as kind=%v ct=%q", k, ct)
	}
	ncx := `<?xml version="1.0"?><!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd"><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"/>`
	if k, _, err := content.Classify("OEBPS/toc.ncx", []byte(ncx)); err != nil || k != content.KindXML {
		t.Fatalf("external-DOCTYPE NCX: kind=%v err=%v", k, err)
	}
}

// EPUB allows UTF-16 and legacy encodings for its XML. foliate-js reads
// them with fetch().text(), which always decodes UTF-8, so the endpoint
// serves them transcoded to UTF-8 and labelled so.
func TestNormalizeXML_TranscodesToUTF8(t *testing.T) {
	utf16le := []byte{0xFF, 0xFE}
	for _, r := range `<?xml version="1.0" encoding="UTF-16"?><package xmlns="http://www.idpf.org/2007/opf"><title>Café</title></package>` {
		utf16le = append(utf16le, byte(r), byte(r>>8))
	}
	latin1 := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><ncx xmlns=\"http://www.daisy.org/z3986/2005/ncx/\"><text>Caf\xe9</text></ncx>")
	for name, in := range map[string][]byte{"OEBPS/content.opf": utf16le, "OEBPS/toc.ncx": latin1} {
		k, ct, err := content.Classify(name, in)
		if err != nil || k != content.KindXML || ct != "application/xml; charset=utf-8" {
			t.Fatalf("%s: kind=%v ct=%q err=%v", name, k, ct, err)
		}
		out, err := content.NormalizeXML(in)
		if err != nil || !strings.Contains(string(out), "Café") {
			t.Fatalf("%s: not transcoded to UTF-8: err=%v %q", name, err, out)
		}
	}
}
