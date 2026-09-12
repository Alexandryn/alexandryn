package domain_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Subject is a constrained value type: bounded, printable text with
// control characters rejected.
func TestNewSubject(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid subject", "Science Fiction", false},
		{"empty rejected", "", true},
		{"whitespace-only rejected", "   ", true},
		{"control character rejected", "Fantasy\x00", true},
		{"over-length rejected", strings.Repeat("a", 101), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewSubject(tt.value)
			if tt.wantErr && err == nil {
				t.Fatalf("NewSubject(%q) = nil error, want an error", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NewSubject(%q) = %v, want nil", tt.value, err)
			}
		})
	}
}

func TestSubject_StringRoundTrips(t *testing.T) {
	s, err := domain.NewSubject("Science Fiction")
	if err != nil {
		t.Fatalf("NewSubject: %v", err)
	}
	if s.String() != "Science Fiction" {
		t.Fatalf("String() = %q, want %q", s.String(), "Science Fiction")
	}
}

func TestSubject_ComparedByValue(t *testing.T) {
	a, _ := domain.NewSubject("Horror")
	b, _ := domain.NewSubject("Horror")
	c, _ := domain.NewSubject("Comedy")

	if a != b {
		t.Fatal("two Subjects built from the same value should be equal")
	}
	if a == c {
		t.Fatal("two Subjects built from different values should not be equal")
	}
}
