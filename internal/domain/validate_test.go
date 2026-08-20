package domain_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// FR-5/FR-6 (domain-bibliographic.md): every free-form string field routes
// through this one bounded-text validator — length bound, control-character
// rejection — plus review 0049 finding 5's amendment: empty or
// whitespace-only fails the same as over-length, not a distinct "absent"
// case.
func TestValidateBoundedText(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		maxLen  int
		wantErr bool
	}{
		{"valid short string", "title", "The Hobbit", 100, false},
		{"valid at exactly maxLen", "title", strings.Repeat("a", 100), 100, false},
		{"empty string rejected", "title", "", 100, true},
		{"whitespace-only rejected", "title", "   \t  ", 100, true},
		{"single space over otherwise-empty rejected", "title", " ", 100, true},
		{"exceeds maxLen by one rejected", "title", strings.Repeat("a", 101), 100, true},
		{"null byte rejected", "title", "Evil\x00Title", 100, true},
		{"terminal escape sequence rejected", "title", "\x1b[31mRed\x1b[0m", 100, true},
		{"newline rejected", "summary", "line one\nline two", 200, true},
		{"unicode content within bound accepted", "title", "百年孤独", 100, false},
		{"leading/trailing whitespace with real content accepted", "title", "  The Hobbit  ", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateBoundedText(tt.field, tt.value, tt.maxLen)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateBoundedText(%q, %q, %d) = nil, want an error", tt.field, tt.value, tt.maxLen)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateBoundedText(%q, %q, %d) = %v, want nil", tt.field, tt.value, tt.maxLen, err)
			}
			if err != nil && domain.CategoryOf(err) != domain.InvalidInput {
				t.Fatalf("CategoryOf(err) = %v, want InvalidInput", domain.CategoryOf(err))
			}
		})
	}
}

// FR-5 (domain-bibliographic.md): Language is a BCP-47 tag, not arbitrary
// text. Structural shape check, not full IANA subtag-registry validation
// (an explicit, narrower scope — see validate.go's own doc comment).
func TestValidateLanguageTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{"simple language", "en", false},
		{"language-region", "en-US", false},
		{"language-script-region", "zh-Hans-CN", false},
		{"language-region lowercase region", "pt-br", false},
		{"empty rejected", "", true},
		{"single character rejected", "e", true},
		{"starts with digit rejected", "1en", true},
		{"primary subtag over 8 chars rejected", "abcdefghi", true},
		{"contains a space rejected", "en US", true},
		{"contains control character rejected", "en\x00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateLanguageTag(tt.tag)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateLanguageTag(%q) = nil, want an error", tt.tag)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateLanguageTag(%q) = %v, want nil", tt.tag, err)
			}
			if err != nil && domain.CategoryOf(err) != domain.InvalidInput {
				t.Fatalf("CategoryOf(err) = %v, want InvalidInput", domain.CategoryOf(err))
			}
		})
	}
}

// FR-2 (domain-bibliographic.md): ISBN-10/13 shape-validated. Checksum
// validation, not just length/format — resolves that spec's own Open
// questions item ("ISBN-10 vs. ISBN-13 checksum validation, or just
// format/length... a phase 03/07 implementation detail once there's real
// data to validate against") in favor of the stronger check: it's a
// well-defined, cheap algorithm with no external dependency needed.
func TestValidateISBN(t *testing.T) {
	tests := []struct {
		name    string
		isbn    string
		wantErr bool
	}{
		{"valid ISBN-10", "0306406152", false},
		{"valid ISBN-10 with hyphens", "0-306-40615-2", false},
		{"valid ISBN-10 with X check digit", "080442957X", false},
		{"valid ISBN-13", "9780306406157", false},
		{"valid ISBN-13 with hyphens", "978-0-306-40615-7", false},
		{"ISBN-10 bad checksum rejected", "0306406153", true},
		{"ISBN-13 bad checksum rejected", "9780306406158", true},
		{"wrong length rejected", "12345", true},
		{"ISBN-10 with letter in wrong position rejected", "03064X6152", true},
		{"ISBN-13 with non-digit rejected", "97803064061X7", true},
		{"empty rejected", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateISBN(tt.isbn)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateISBN(%q) = nil, want an error", tt.isbn)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateISBN(%q) = %v, want nil", tt.isbn, err)
			}
			if err != nil && domain.CategoryOf(err) != domain.InvalidInput {
				t.Fatalf("CategoryOf(err) = %v, want InvalidInput", domain.CategoryOf(err))
			}
		})
	}
}
