package domain_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// domain-bibliographic.md FR-8: an Edition cannot be constructed without
// a parent Work reference — literally unrepresentable via Go's type
// system, not a nil-check convention. There is nothing to assert at
// runtime about this (unlike a nil check, which a test could exercise) —
// the proof is that NewEdition's signature requires a WorkID as a
// positional argument with no overload omitting it, so no call site
// compiles without supplying one. Every test below supplies a WorkID for
// exactly this reason, not by omission.
func TestNewEdition_RequiresParentWork(t *testing.T) {
	lang, _ := domain.NewLanguage("en")
	e, err := domain.NewEdition(domain.EditionID("edition-1"), domain.WorkID("work-1"), lang, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition: %v", err)
	}
	if e.WorkID() != domain.WorkID("work-1") {
		t.Fatalf("WorkID() = %v, want work-1", e.WorkID())
	}
}

func TestNewEdition_ISBNOptionalButValidatedWhenPresent(t *testing.T) {
	lang, _ := domain.NewLanguage("en")

	t.Run("nil ISBN is legal", func(t *testing.T) {
		e, err := domain.NewEdition(domain.EditionID("edition-1"), domain.WorkID("work-1"), lang, nil, "", nil, nil)
		if err != nil {
			t.Fatalf("NewEdition with nil ISBN: %v", err)
		}
		if e.ISBN() != nil {
			t.Fatalf("ISBN() = %v, want nil", e.ISBN())
		}
	})
	t.Run("valid ISBN accepted", func(t *testing.T) {
		isbn := "9780306406157"
		e, err := domain.NewEdition(domain.EditionID("edition-1"), domain.WorkID("work-1"), lang, &isbn, "", nil, nil)
		if err != nil {
			t.Fatalf("NewEdition with valid ISBN: %v", err)
		}
		if e.ISBN() == nil || *e.ISBN() != isbn {
			t.Fatalf("ISBN() = %v, want %q", e.ISBN(), isbn)
		}
	})
	t.Run("invalid ISBN rejected", func(t *testing.T) {
		isbn := "not-an-isbn"
		_, err := domain.NewEdition(domain.EditionID("edition-1"), domain.WorkID("work-1"), lang, &isbn, "", nil, nil)
		if err == nil {
			t.Fatal("NewEdition with invalid ISBN = nil error, want an error")
		}
	})
}

// FR-10: Edition has its own Language field, independent of
// Work.OriginalLanguage — this is the field that actually varies per
// translation.
//
// The translation fixture: same Work, two Editions, different
// Edition.Language, distinguished from the reissue fixture: same Work,
// same language, different year/publisher (both required, named
// explicitly by domain-bibliographic.md's own Test strategy section, not
// generic placeholders).
func TestEdition_TranslationVsReissue(t *testing.T) {
	work := domain.WorkID("work-1")
	en, _ := domain.NewLanguage("en")
	fr, _ := domain.NewLanguage("fr")

	original, err := domain.NewEdition(domain.EditionID("edition-1"), work, en, nil, "Ace Books", intPtr(1969), nil)
	if err != nil {
		t.Fatalf("NewEdition (original): %v", err)
	}

	t.Run("translation: same Work, different Edition.Language", func(t *testing.T) {
		translation, err := domain.NewEdition(domain.EditionID("edition-2"), work, fr, nil, "Robert Laffont", intPtr(1971), nil)
		if err != nil {
			t.Fatalf("NewEdition (translation): %v", err)
		}
		if translation.WorkID() != original.WorkID() {
			t.Fatal("translation must belong to the same Work as the original")
		}
		if translation.Language() == original.Language() {
			t.Fatal("translation must have a different Language than the original")
		}
	})

	t.Run("reissue: same Work, same language, different year/publisher", func(t *testing.T) {
		reissue, err := domain.NewEdition(domain.EditionID("edition-3"), work, en, nil, "Ace Reissue", intPtr(1987), nil)
		if err != nil {
			t.Fatalf("NewEdition (reissue): %v", err)
		}
		if reissue.WorkID() != original.WorkID() {
			t.Fatal("reissue must belong to the same Work as the original")
		}
		if reissue.Language() != original.Language() {
			t.Fatal("reissue must have the same Language as the original — that's what distinguishes it from a translation")
		}
		if reissue.PublicationYear() == nil || original.PublicationYear() == nil ||
			*reissue.PublicationYear() == *original.PublicationYear() {
			t.Fatal("reissue must differ from the original by year (or publisher)")
		}
	})
}

func intPtr(i int) *int { return &i }
