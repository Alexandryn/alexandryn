package domain_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// domain-source.md FR-1: a Source MUST have an internal identifier and a
// user-assigned label, and declares capabilities from a fixed, small set
// rather than the domain inferring what a source can do from its type.
func TestNewSource(t *testing.T) {
	caps := domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true}

	t.Run("valid source", func(t *testing.T) {
		s, err := domain.NewSource(domain.SourceID("source-1"), "My OPDS Catalog", caps, "opds")
		if err != nil {
			t.Fatalf("NewSource: %v", err)
		}
		if s.Label() != "My OPDS Catalog" {
			t.Fatalf("Label() = %q, want %q", s.Label(), "My OPDS Catalog")
		}
		if s.Capabilities() != caps {
			t.Fatalf("Capabilities() = %+v, want %+v", s.Capabilities(), caps)
		}
		if s.Kind() != "opds" {
			t.Fatalf("Kind() = %q, want %q", s.Kind(), "opds")
		}
	})

	t.Run("empty label rejected", func(t *testing.T) {
		_, err := domain.NewSource(domain.SourceID("source-1"), "", caps, "")
		if err == nil {
			t.Fatal("NewSource with empty label = nil error, want an error")
		}
	})

	t.Run("empty Kind is legal — a display-only, optional tag", func(t *testing.T) {
		s, err := domain.NewSource(domain.SourceID("source-1"), "My Source", caps, "")
		if err != nil {
			t.Fatalf("NewSource with empty Kind: %v", err)
		}
		if s.Kind() != "" {
			t.Fatalf("Kind() = %q, want empty", s.Kind())
		}
	})
}
