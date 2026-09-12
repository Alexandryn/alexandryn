package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

var markTime = time.Date(2026, 8, 28, 20, 0, 0, 0, time.UTC)

// Bookmark and Highlight attach to Edition (not Work) — EditionID is a
// required positional argument.
func TestNewBookmark(t *testing.T) {
	b := domain.NewBookmark("bookmark-1", "edition-1", "loc-42", "Great line", markTime)
	if b.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", b.EditionID())
	}
	if b.Position() != "loc-42" {
		t.Fatalf("Position() = %q, want %q", b.Position(), "loc-42")
	}
	if b.Label() != "Great line" {
		t.Fatalf("Label() = %q, want %q", b.Label(), "Great line")
	}
	// CreatedAt is recorded at construction.
	if !b.CreatedAt().Equal(markTime) {
		t.Fatalf("CreatedAt() = %v, want %v", b.CreatedAt(), markTime)
	}
}

func TestNewBookmark_LabelOptional(t *testing.T) {
	b := domain.NewBookmark("bookmark-1", "edition-1", "loc-42", "", markTime)
	if b.Label() != "" {
		t.Fatalf("Label() = %q, want empty", b.Label())
	}
}

// A Highlight records a start and end position (both Edition-scoped)
// and optional note and category/color.
func TestNewHighlight(t *testing.T) {
	h := domain.NewHighlight("highlight-1", "edition-1", "loc-10", "loc-20", "Interesting", "yellow", markTime)
	if h.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", h.EditionID())
	}
	if h.StartPosition() != "loc-10" || h.EndPosition() != "loc-20" {
		t.Fatalf("StartPosition/EndPosition = %q/%q, want loc-10/loc-20", h.StartPosition(), h.EndPosition())
	}
	if h.Note() != "Interesting" || h.Category() != "yellow" {
		t.Fatalf("Note/Category = %q/%q, want Interesting/yellow", h.Note(), h.Category())
	}
	if !h.CreatedAt().Equal(markTime) {
		t.Fatalf("CreatedAt() = %v, want %v", h.CreatedAt(), markTime)
	}
}

func TestNewHighlight_NoteAndCategoryOptional(t *testing.T) {
	h := domain.NewHighlight("highlight-1", "edition-1", "loc-10", "loc-20", "", "", markTime)
	if h.Note() != "" || h.Category() != "" {
		t.Fatalf("Note/Category = %q/%q, want both empty", h.Note(), h.Category())
	}
}
