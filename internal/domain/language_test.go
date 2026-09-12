package domain_test

import "testing"

import "github.com/Alexandryn/alexandryn/internal/domain"

// Language is a constrained value type, validated at construction (BCP-47 shape).
func TestNewLanguage(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{"valid simple tag", "en", false},
		{"valid region tag", "pt-BR", false},
		{"empty rejected", "", true},
		{"malformed tag rejected", "not a tag", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewLanguage(tt.tag)
			if tt.wantErr && err == nil {
				t.Fatalf("NewLanguage(%q) = nil error, want an error", tt.tag)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NewLanguage(%q) = %v, want nil", tt.tag, err)
			}
		})
	}
}

func TestLanguage_StringRoundTrips(t *testing.T) {
	lang, err := domain.NewLanguage("en-US")
	if err != nil {
		t.Fatalf("NewLanguage: %v", err)
	}
	if lang.String() != "en-US" {
		t.Fatalf("String() = %q, want %q", lang.String(), "en-US")
	}
}

// TestLanguage_ComparedByValue verifies value comparison for Language.
func TestLanguage_ComparedByValue(t *testing.T) {
	a, _ := domain.NewLanguage("en")
	b, _ := domain.NewLanguage("en")
	c, _ := domain.NewLanguage("fr")

	if a != b {
		t.Fatal("two Languages built from the same tag should be equal")
	}
	if a == c {
		t.Fatal("two Languages built from different tags should not be equal")
	}
}
