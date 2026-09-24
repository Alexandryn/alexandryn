package content

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

// Kind represents how a resolved archive entry must be served.
type Kind int

const (
	KindHTML   Kind = iota // route through SanitizeHTML
	KindCSS                // route through SanitizeCSS
	KindBinary             // image/font — serve as-is, still size-capped
	KindXML                // structural EPUB XML — served as-is, never renderable markup
)

// ValidateResourcePath rejects an untrusted resource path before it is
// used to look up a zip entry: no "..", no absolute segment, and no
// empty path. This validation runs prior to archive entry lookup.
func ValidateResourcePath(p string) error {
	if p == "" {
		return &domain.Error{Category: domain.InvalidInput, Message: "resource path is empty"}
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return &domain.Error{Category: domain.InvalidInput, Message: "resource path must be relative"}
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return &domain.Error{Category: domain.InvalidInput, Message: "resource path must not traverse upward"}
		}
	}
	if path.Clean(p) != p {
		return &domain.Error{Category: domain.InvalidInput, Message: "resource path is not in canonical form"}
	}
	return nil
}

// FindEntry looks up path p in an open archive's entry table.
// Returns a *domain.Error with category NotFound when no entry matches.
func FindEntry(zr *zip.Reader, p string) (*zip.File, error) {
	for _, f := range zr.File {
		if f.Name == p {
			return f, nil
		}
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "no such resource in this book"}
}

// ReadEntry decompresses a single entry under the maximum decompressed
// byte-count cap (200 MiB) applied per served entry.
func ReadEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "this resource could not be read from the book"}
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, extract.MaxDecompressedReadBytes+1))
	if err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "this resource could not be read from the book"}
	}
	if int64(len(data)) > extract.MaxDecompressedReadBytes {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "this resource is too large to serve"}
	}
	return data, nil
}

// Classify decides how an entry is served based on its sniffed content
// rather than path extension alone. HTML/XHTML -> KindHTML;
// text/css -> KindCSS; standalone image/svg+xml is refused; XML text
// (the container, package, and NCX files foliate-js parses to locate the
// spine) -> KindXML, but only after classifyXML proves it holds nothing a
// browser would render as a live document; non-SVG images or fonts ->
// KindBinary served with that MIME type. Anything unrecognised or
// unexpected (such as executables) is refused with InvalidInput.
func Classify(entryName string, data []byte) (Kind, string, error) {
	sniff := http.DetectContentType(data)
	base := strings.ToLower(strings.TrimSpace(strings.SplitN(sniff, ";", 2)[0]))

	// DetectContentType does not recognise CSS or XHTML; fall back to the
	// entry extension only for those two text formats, and only after
	// confirming the sniff is a text type (not a binary masquerading).
	ext := strings.ToLower(path.Ext(entryName))

	switch {
	case base == "text/html" || ext == ".xhtml" || ext == ".html" || ext == ".htm":
		return KindHTML, htmlContentType, nil
	case ext == ".css" && strings.HasPrefix(base, "text/"):
		return KindCSS, "text/css; charset=utf-8", nil
	case base == "image/svg+xml" || ext == ".svg":
		return 0, "", errSVGRefused
	// Checked after the SVG and HTML cases: DetectContentType reports any
	// file opening with "<?xml" — most SVGs and many XHTML chapters — as
	// text/xml.
	case base == "text/xml" || base == "application/xml" ||
		((ext == ".xml" || ext == ".opf" || ext == ".ncx") && strings.HasPrefix(base, "text/")):
		return classifyXML(data)
	case strings.HasPrefix(base, "image/"):
		return KindBinary, base, nil
	case isFontType(base, ext):
		return KindBinary, fontMIME(ext, base), nil
	default:
		return 0, "", errUnservable
	}
}

const htmlContentType = "application/xhtml+xml; charset=utf-8"

var (
	errSVGRefused = &domain.Error{Category: domain.InvalidInput, Message: "SVG resources are not served"}
	errUnservable = &domain.Error{Category: domain.InvalidInput, Message: "this resource type cannot be served"}
)

// Namespaces whose elements a browser renders as a live document (links,
// forms, overlays, transforms) when the XML is opened directly or framed.
const (
	nsXHTML  = "http://www.w3.org/1999/xhtml"
	nsSVG    = "http://www.w3.org/2000/svg"
	nsMathML = "http://www.w3.org/1998/Math/MathML"
	nsXSLT   = "http://www.w3.org/1999/XSL/Transform"
)

// classifyXML decides how an XML entry is served. It is served raw, so it
// must be structural data only: a document whose root is XHTML is a
// content document and goes through the HTML sanitiser instead; any SVG,
// MathML, XSLT, or nested XHTML element, an xml-stylesheet instruction, a
// DTD internal subset (whose attribute defaults, e.g. a #FIXED xmlns, a
// browser applies but encoding/xml never sees), or XML that does not
// parse is refused. It is served transcoded to UTF-8 (NormalizeXML), and
// labelled so.
func classifyXML(data []byte) (Kind, string, error) {
	norm, err := NormalizeXML(data)
	if err != nil {
		return 0, "", err
	}
	dec := xml.NewDecoder(bytes.NewReader(norm))
	// Already UTF-8: a leftover encoding="…" declaration must not decode it again.
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	root := true
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, "", errUnservable
		}
		switch t := tok.(type) {
		case xml.Directive:
			if bytes.ContainsRune(t, '[') {
				return 0, "", errUnservable
			}
		case xml.ProcInst:
			if t.Target == "xml-stylesheet" {
				return 0, "", errUnservable
			}
		case xml.StartElement:
			switch t.Name.Space {
			case nsXHTML:
				if root {
					return KindHTML, htmlContentType, nil
				}
				return 0, "", errUnservable
			case nsSVG:
				return 0, "", errSVGRefused
			case nsMathML, nsXSLT:
				return 0, "", errUnservable
			}
			root = false
		}
	}
	if root {
		return 0, "", errUnservable
	}
	return KindXML, "application/xml; charset=utf-8", nil
}

var reXMLEncoding = regexp.MustCompile(`^\s*<\?xml[^>]*?\sencoding\s*=\s*["']([A-Za-z0-9._:-]+)["']`)

// NormalizeXML returns an XML entry transcoded to UTF-8. EPUB allows
// UTF-16 (with a BOM) and legacy encodings for its XML, but foliate-js
// reads these files with fetch().text(), which always decodes UTF-8, so
// they are served as UTF-8 whatever their declaration says.
func NormalizeXML(data []byte) ([]byte, error) {
	var r io.Reader
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		r = transform.NewReader(bytes.NewReader(data), unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder())
	default:
		m := reXMLEncoding.FindSubmatch(data)
		if m == nil || strings.EqualFold(string(m[1]), "utf-8") {
			return data, nil
		}
		cr, err := charset.NewReaderLabel(string(m[1]), bytes.NewReader(data))
		if err != nil {
			return nil, errUnservable
		}
		r = cr
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, errUnservable
	}
	return out, nil
}

func isFontType(base, ext string) bool {
	if strings.HasPrefix(base, "font/") || base == "application/font-woff" || base == "application/vnd.ms-fontobject" {
		return true
	}
	switch ext {
	case ".woff", ".woff2", ".ttf", ".otf":
		// DetectContentType reports these as application/octet-stream;
		// the extension is the only signal, and a font file has no
		// script-execution surface to sanitise.
		return base == "application/octet-stream" || strings.HasPrefix(base, "font/")
	}
	return false
}

func fontMIME(ext, base string) string {
	switch ext {
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	}
	return base
}
