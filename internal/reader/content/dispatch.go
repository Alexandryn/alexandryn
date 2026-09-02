package content

import (
	"archive/zip"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

// Kind is how a resolved entry must be served (backend-reader-content.md FR-5).
type Kind int

const (
	KindHTML   Kind = iota // route through SanitizeHTML
	KindCSS                // route through SanitizeCSS
	KindBinary             // image/font — serve as-is, still size-capped
)

// ValidateResourcePath rejects a client-supplied *path before it is ever
// used to look up a zip entry (FR-1): no "..", no absolute segment, no
// empty path. The path comes from the client, so this runs before the
// entry-table lookup, not only after.
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

// FindEntry looks p up in an already-open archive's entry table (FR-4).
// Returns a *domain.Error with category NotFound when no entry matches.
func FindEntry(zr *zip.Reader, p string) (*zip.File, error) {
	for _, f := range zr.File {
		if f.Name == p {
			return f, nil
		}
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "no such resource in this book"}
}

// ReadEntry decompresses a single entry under the same streaming
// byte-count cap phase 10's extractor uses (FR-4: 200 MiB, reused —
// applied here per served entry, a stricter reapplication).
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

// Classify decides how an entry is served, by its sniffed content —
// never the path's extension alone (FR-5). HTML/XHTML -> KindHTML;
// text/css -> KindCSS; a non-SVG image or font -> KindBinary served with
// that MIME type. Standalone image/svg+xml, and any unrecognised or
// unexpected type (an embedded executable), are refused with
// InvalidInput.
func Classify(entryName string, data []byte) (Kind, string, error) {
	sniff := http.DetectContentType(data)
	base := strings.ToLower(strings.TrimSpace(strings.SplitN(sniff, ";", 2)[0]))

	// DetectContentType does not recognise CSS or XHTML; fall back to the
	// entry extension only for those two text formats, and only after
	// confirming the sniff is a text type (not a binary masquerading).
	ext := strings.ToLower(path.Ext(entryName))

	switch {
	case base == "text/html" || ext == ".xhtml" || ext == ".html" || ext == ".htm":
		return KindHTML, "application/xhtml+xml; charset=utf-8", nil
	case ext == ".css" && strings.HasPrefix(base, "text/"):
		return KindCSS, "text/css; charset=utf-8", nil
	case base == "image/svg+xml" || ext == ".svg":
		return 0, "", &domain.Error{Category: domain.InvalidInput, Message: "SVG resources are not served"}
	case strings.HasPrefix(base, "image/"):
		return KindBinary, base, nil
	case isFontType(base, ext):
		return KindBinary, fontMIME(ext, base), nil
	default:
		return 0, "", &domain.Error{Category: domain.InvalidInput, Message: "this resource type cannot be served"}
	}
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
