package sources_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
)

func TestValidateLocalFolderPath(t *testing.T) {
	valid := []string{
		"/srv/books",
		"/home/jane/Books/EPUB",
		`C:\Users\jane\Books`,
		`C:/Users/jane/Books`,
		`\\nas\share\books`, // UNC
		"/mnt/does-not-exist-yet",
	}
	for _, p := range valid {
		if err := sources.ValidateLocalFolderPath(p); err != nil {
			t.Errorf("ValidateLocalFolderPath(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"books",              // relative
		"./books",            // relative
		"../../etc",          // relative traversal
		"~/books",            // not absolute
		"/srv/\x00books",     // NUL
		"/srv/books\nx",      // control char
		"/" + strings.Repeat("a", 5000), // too long
	}
	for _, p := range invalid {
		if err := sources.ValidateLocalFolderPath(p); err == nil {
			t.Errorf("ValidateLocalFolderPath(%q) = nil, want error", p)
		}
	}
}

func TestValidateOPDSBaseURL(t *testing.T) {
	valid := []string{
		"https://opds.example.org/catalog",
		"http://192.168.1.10:8080/opds", // LAN OPDS is a real, accepted case
		"https://books.example.org",
	}
	for _, u := range valid {
		if err := sources.ValidateOPDSBaseURL(u); err != nil {
			t.Errorf("ValidateOPDSBaseURL(%q) = %v, want nil", u, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"ftp://opds.example.org",
		"file:///etc/passwd",
		"opds.example.org/catalog", // no scheme
		"https://",                 // no host
		"://nohost",
		"https://" + strings.Repeat("a", 3000) + ".example.org",
	}
	for _, u := range invalid {
		if err := sources.ValidateOPDSBaseURL(u); err == nil {
			t.Errorf("ValidateOPDSBaseURL(%q) = nil, want error", u)
		}
	}
}

func TestIsHTTPS(t *testing.T) {
	if !sources.IsHTTPS("https://h.example/x") {
		t.Error("https URL should be IsHTTPS")
	}
	if sources.IsHTTPS("http://h.example/x") {
		t.Error("http URL should not be IsHTTPS")
	}
}
