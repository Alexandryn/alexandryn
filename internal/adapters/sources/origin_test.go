package sources_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
)

func TestSameOrigin(t *testing.T) {
	const base = "https://opds.example.org/catalog"

	same := []string{
		"https://opds.example.org/catalog?page=2",
		"https://opds.example.org/search?q=x",
		"https://OPDS.EXAMPLE.ORG/catalog", // case-insensitive host
		"https://opds.example.org:443/catalog",
	}
	for _, c := range same {
		if !sources.SameOrigin(base, c) {
			t.Errorf("SameOrigin(%q) = false, want true", c)
		}
	}

	different := []string{
		"http://opds.example.org/catalog",       // scheme
		"https://evil.example.org/catalog",       // host
		"https://opds.example.org.evil.com/x",    // suffix trick
		"https://opds.example.org:8443/catalog",  // port
		"https://169.254.169.254/latest/meta",    // cloud metadata
		"http://127.0.0.1/catalog",               // loopback
		"http://10.0.0.5/catalog",                // private
		"file:///etc/passwd",                     // non-http scheme
		"ftp://opds.example.org/catalog",         // non-http scheme
		"//opds.example.org/catalog",             // scheme-relative
		"/catalog?page=2",                        // path-only
		"",                                       // empty
		"https://opds.example.org@evil.com/x",    // userinfo host confusion
	}
	for _, c := range different {
		if sources.SameOrigin(base, c) {
			t.Errorf("SameOrigin(%q) = true, want false", c)
		}
	}
}

func TestSameOrigin_HTTPDefaultPort(t *testing.T) {
	if !sources.SameOrigin("http://h.example/x", "http://h.example:80/y") {
		t.Error("http default port 80 should match explicit :80")
	}
}

func TestSameOrigin_RejectsMalformedConfigured(t *testing.T) {
	if sources.SameOrigin("not a url", "https://h.example/x") {
		t.Error("a malformed configured origin must never match")
	}
}
