package domain_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// A Work with zero Editions and zero external references is a valid,
// constructible state.
func TestNewWork_ZeroEditionsAndZeroExternalReferencesIsLegal(t *testing.T) {
	w, err := domain.NewWork(domain.WorkID("work-1"), "The Left Hand of Darkness", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork with no editions, no external refs: %v", err)
	}
	if len(w.ExternalReferences()) != 0 {
		t.Fatalf("ExternalReferences() = %v, want empty", w.ExternalReferences())
	}
}

func TestNewWork_TitleValidation(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"valid title", "The Left Hand of Darkness", false},
		{"empty title rejected", "", true},
		{"whitespace-only title rejected", "   ", true},
		{"over-length title rejected", strings.Repeat("a", 501), true},
		{"control character rejected", "Evil\x00Title", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewWork(domain.WorkID("work-1"), tt.title, "", nil, nil, nil, nil)
			if tt.wantErr && err == nil {
				t.Fatalf("NewWork(title=%q) = nil error, want an error", tt.title)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NewWork(title=%q) = %v, want nil", tt.title, err)
			}
		})
	}
}

// Subtitle is optional — an empty subtitle is not validated as if it
// were a required field (unlike title), but a present-and-invalid one
// still fails.
func TestNewWork_SubtitleValidation(t *testing.T) {
	t.Run("empty subtitle is legal (optional field)", func(t *testing.T) {
		_, err := domain.NewWork(domain.WorkID("work-1"), "Title", "", nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("NewWork with empty subtitle: %v", err)
		}
	})
	t.Run("present but over-length subtitle rejected", func(t *testing.T) {
		_, err := domain.NewWork(domain.WorkID("work-1"), "Title", strings.Repeat("a", 501), nil, nil, nil, nil)
		if err == nil {
			t.Fatal("NewWork with over-length subtitle = nil error, want an error")
		}
	})
	t.Run("present but control-character subtitle rejected", func(t *testing.T) {
		_, err := domain.NewWork(domain.WorkID("work-1"), "Title", "Evil\x00Subtitle", nil, nil, nil, nil)
		if err == nil {
			t.Fatal("NewWork with control-character subtitle = nil error, want an error")
		}
	})
}

func TestNewWork_ZeroAuthorsIsLegal(t *testing.T) {
	w, err := domain.NewWork(domain.WorkID("work-1"), "Anonymous Work", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork with zero authors: %v", err)
	}
	if len(w.Authors()) != 0 {
		t.Fatalf("Authors() = %v, want empty", w.Authors())
	}
}

func TestNewWork_OriginalLanguageOptional(t *testing.T) {
	w, err := domain.NewWork(domain.WorkID("work-1"), "Title", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork with no original language: %v", err)
	}
	if w.OriginalLanguage() != nil {
		t.Fatalf("OriginalLanguage() = %v, want nil", w.OriginalLanguage())
	}

	lang, _ := domain.NewLanguage("en")
	w2, err := domain.NewWork(domain.WorkID("work-2"), "Title", "", nil, nil, &lang, nil)
	if err != nil {
		t.Fatalf("NewWork with original language: %v", err)
	}
	if w2.OriginalLanguage() == nil || w2.OriginalLanguage().String() != "en" {
		t.Fatalf("OriginalLanguage() = %v, want en", w2.OriginalLanguage())
	}
}

// MergedInto and Contains are empty/nil at construction — both are
// domain-service operations, never constructor arguments.
func TestNewWork_MergeAndContainmentStateEmptyAtConstruction(t *testing.T) {
	w, err := domain.NewWork(domain.WorkID("work-1"), "Title", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}
	if w.MergedInto() != nil {
		t.Fatalf("MergedInto() = %v, want nil", w.MergedInto())
	}
	if len(w.Contains()) != 0 {
		t.Fatalf("Contains() = %v, want empty", w.Contains())
	}
}

func TestWork_IDAccessor(t *testing.T) {
	w, err := domain.NewWork(domain.WorkID("work-1"), "Title", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}
	if w.ID() != domain.WorkID("work-1") {
		t.Fatalf("ID() = %v, want work-1", w.ID())
	}
}
