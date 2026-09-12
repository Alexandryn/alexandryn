package domain_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Tests Author identity pattern: internal ID primary, optional external
// references. An Author may have zero external references.
func TestNewAuthor(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		wantErr bool
	}{
		{"valid name", "Ursula K. Le Guin", false},
		{"empty name rejected", "", true},
		{"over-length name rejected", strings.Repeat("a", 201), true},
		{"control character rejected", "Evil\x00Name", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewAuthor(domain.AuthorID("author-1"), tt.author, nil)
			if tt.wantErr && err == nil {
				t.Fatalf("NewAuthor(%q) = nil error, want an error", tt.author)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NewAuthor(%q) = %v, want nil", tt.author, err)
			}
		})
	}
}

func TestNewAuthor_ZeroExternalReferencesIsLegal(t *testing.T) {
	a, err := domain.NewAuthor(domain.AuthorID("author-1"), "Anonymous", nil)
	if err != nil {
		t.Fatalf("NewAuthor with nil external references: %v", err)
	}
	if len(a.ExternalReferences()) != 0 {
		t.Fatalf("ExternalReferences() = %v, want empty", a.ExternalReferences())
	}
}

func TestNewAuthor_WithExternalReference(t *testing.T) {
	refs := []domain.ExternalReference{{Source: "openlibrary", ID: "OL123A"}}
	a, err := domain.NewAuthor(domain.AuthorID("author-1"), "Ursula K. Le Guin", refs)
	if err != nil {
		t.Fatalf("NewAuthor: %v", err)
	}
	if len(a.ExternalReferences()) != 1 || a.ExternalReferences()[0].ID != "OL123A" {
		t.Fatalf("ExternalReferences() = %v, want one openlibrary reference", a.ExternalReferences())
	}
}

// MergedInto is nil at construction; recording a merge is handled by
// AuthorMergeService.
func TestNewAuthor_MergedIntoIsNilAtConstruction(t *testing.T) {
	a, err := domain.NewAuthor(domain.AuthorID("author-1"), "Ursula K. Le Guin", nil)
	if err != nil {
		t.Fatalf("NewAuthor: %v", err)
	}
	if a.MergedInto() != nil {
		t.Fatalf("MergedInto() = %v, want nil", a.MergedInto())
	}
}

func TestAuthor_IDAccessor(t *testing.T) {
	a, err := domain.NewAuthor(domain.AuthorID("author-1"), "Ursula K. Le Guin", nil)
	if err != nil {
		t.Fatalf("NewAuthor: %v", err)
	}
	if a.ID() != domain.AuthorID("author-1") {
		t.Fatalf("ID() = %v, want author-1", a.ID())
	}
}
