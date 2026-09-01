package sources

import (
	"net/url"
	"strings"
)

// SameOrigin reports whether candidate resolves to the same origin
// (scheme + host + port) as configured (backend-source-adapter.md
// FR-11). Every URL that originated from a source's own response
// content — a browse cursor's wrapped continuation link, a discovered
// search link — is checked with this before it is ever fetched. A value
// resolving to a different origin, or one that is not an absolute http
// or https URL, returns false.
//
// The comparison is on the URLs' effective origin, with the default
// port for the scheme filled in, so http://h and http://h:80 match and
// https://h and https://h:443 match. Host comparison is
// case-insensitive.
func SameOrigin(configured, candidate string) bool {
	c, err := parseAbsoluteHTTP(configured)
	if err != nil {
		return false
	}
	o, err := parseAbsoluteHTTP(candidate)
	if err != nil {
		return false
	}
	return c.scheme == o.scheme &&
		strings.EqualFold(c.host, o.host) &&
		c.port == o.port
}

type originParts struct {
	scheme string
	host   string
	port   string
}

func parseAbsoluteHTTP(raw string) (originParts, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return originParts{}, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return originParts{}, errNotHTTP
	}
	host := u.Hostname()
	if host == "" {
		return originParts{}, errNoHost
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return originParts{scheme: u.Scheme, host: host, port: port}, nil
}

var (
	errNotHTTP = urlError("not an http or https URL")
	errNoHost  = urlError("no host")
)

type urlError string

func (e urlError) Error() string { return string(e) }
