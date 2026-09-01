package sources

import (
	"net/url"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// maxPathLen is the resolved-path length ceiling
// (backend-source-adapter.md FR-3, amended for review 0049 finding 6).
// Linux PATH_MAX is 4096; this is a shape check, not the OS's own
// enforcement.
const maxPathLen = 4096

const maxBaseURLLen = 2048

// ValidateLocalFolderPath checks a local-folder config.basePath's
// *shape* at create/update time (FR-3): a non-empty, absolute path, no
// control characters, within the length ceiling. Windows UNC paths
// (\\server\share\...) are accepted as absolute. Whether the path
// currently exists or is readable is a health-check concern, not a shape
// violation — an unmounted drive may be back later.
func ValidateLocalFolderPath(basePath string) error {
	if strings.TrimSpace(basePath) == "" {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.basePath must not be empty"}
	}
	if len(basePath) > maxPathLen {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.basePath is too long"}
	}
	for _, r := range basePath {
		if r == 0x00 || (r < 0x20 && r != '\t') {
			return &domain.Error{Category: domain.InvalidInput, Message: "config.basePath contains a control character"}
		}
	}
	if !isAbsolutePath(basePath) {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.basePath must be an absolute path"}
	}
	return nil
}

// isAbsolutePath accepts a POSIX absolute path (leading /), a Windows
// drive-letter root (C:\ or C:/), or a Windows UNC path (\\host\share).
// It does not use filepath.IsAbs, which is GOOS-specific — a source
// configured on one platform is validated the same way everywhere.
func isAbsolutePath(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	if strings.HasPrefix(p, `\\`) { // UNC
		return true
	}
	if len(p) >= 3 && isASCIILetter(p[0]) && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	return false
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// ValidateOPDSBaseURL checks an opds config.baseUrl at create/update
// time (FR-4): a well-formed absolute http or https URL with a host.
func ValidateOPDSBaseURL(baseURL string) error {
	if strings.TrimSpace(baseURL) == "" {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.baseUrl must not be empty"}
	}
	if len(baseURL) > maxBaseURLLen {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.baseUrl is too long"}
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.baseUrl is not a valid URL"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.baseUrl must be an http or https URL"}
	}
	if u.Host == "" {
		return &domain.Error{Category: domain.InvalidInput, Message: "config.baseUrl must include a host"}
	}
	return nil
}

// IsHTTPS reports whether baseURL uses the https scheme. Used to surface
// the plain-HTTP Basic Auth warning (FR-4, frontend-source-management.md
// FR-5) — this is the backend's own view of the same fact.
func IsHTTPS(baseURL string) bool {
	u, err := url.Parse(baseURL)
	return err == nil && u.Scheme == "https"
}
