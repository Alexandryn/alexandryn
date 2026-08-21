package domain_test

import "testing"

import "github.com/Alexandryn/alexandryn/internal/domain"

// domain-reading.md FR-3: Bookmark and Highlight MUST attach to Edition
// (not Work) — EditionID is a required positional argument.
func TestNewBookmark(t *testing.T) {
	b := domain.NewBookmark("bookmark-1", "edition-1", "loc-42", "Great line")
	if b.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", b.EditionID())
	}
	if b.Position() != "loc-42" {
		t.Fatalf("Position() = %q, want %q", b.Position(), "loc-42")
	}
	if b.Label() != "Great line" {
		t.Fatalf("Label() = %q, want %q", b.Label(), "Great line")
	}
}

func TestNewBookmark_LabelOptional(t *testing.T) {
	b := domain.NewBookmark("bookmark-1", "edition-1", "loc-42", "")
	if b.Label() != "" {
		t.Fatalf("Label() = %q, want empty", b.Label())
	}
}

// FR-4: a Highlight MUST record a start and end position (both
// Edition-scoped) and MAY carry a note and a category/color.
func TestNewHighlight(t *testing.T) {
	h := domain.NewHighlight("highlight-1", "edition-1", "loc-10", "loc-20", "Interesting", "yellow")
	if h.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", h.EditionID())
	}
	if h.StartPosition() != "loc-10" || h.EndPosition() != "loc-20" {
		t.Fatalf("StartPosition/EndPosition = %q/%q, want loc-10/loc-20", h.StartPosition(), h.EndPosition())
	}
	if h.Note() != "Interesting" || h.Category() != "yellow" {
		t.Fatalf("Note/Category = %q/%q, want Interesting/yellow", h.Note(), h.Category())
	}
}

func TestNewHighlight_NoteAndCategoryOptional(t *testing.T) {
	h := domain.NewHighlight("highlight-1", "edition-1", "loc-10", "loc-20", "", "")
	if h.Note() != "" || h.Category() != "" {
		t.Fatalf("Note/Category = %q/%q, want both empty", h.Note(), h.Category())
	}
}
