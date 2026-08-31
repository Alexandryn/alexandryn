package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestCursor_AddedAtRoundTrip(t *testing.T) {
	addedAt := time.Date(2026, 8, 31, 15, 30, 0, 123456789, time.UTC)
	workID := domain.WorkID("work-123")

	encoded := domain.EncodeAddedAtCursor(addedAt, workID)
	if encoded == "" {
		t.Fatal("expected non-empty encoded cursor")
	}

	gotTime, gotID, err := domain.DecodeAddedAtCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeAddedAtCursor: %v", err)
	}

	if !gotTime.Equal(addedAt) {
		t.Errorf("got time %v, want %v", gotTime, addedAt)
	}
	if gotID != workID {
		t.Errorf("got workID %q, want %q", gotID, workID)
	}
}

func TestCursor_TitleRoundTrip(t *testing.T) {
	title := "The Hobbit: Or There and Back Again"
	workID := domain.WorkID("work-456")

	encoded := domain.EncodeTitleCursor(title, workID)
	if encoded == "" {
		t.Fatal("expected non-empty encoded cursor")
	}

	gotTitle, gotID, err := domain.DecodeTitleCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeTitleCursor: %v", err)
	}

	if gotTitle != title {
		t.Errorf("got title %q, want %q", gotTitle, title)
	}
	if gotID != workID {
		t.Errorf("got workID %q, want %q", gotID, workID)
	}
}

func TestCursor_MalformedCursorRejection(t *testing.T) {
	malformed := []string{
		"not-base64-!@#$",
		"",
		"YWJj",          // "abc" without separator
		"YWJjOmRlZjpoaQ==", // too many parts
	}

	for _, m := range malformed {
		if _, _, err := domain.DecodeAddedAtCursor(m); err == nil {
			t.Errorf("DecodeAddedAtCursor(%q) = nil error, want invalid cursor error", m)
		}
		if _, _, err := domain.DecodeTitleCursor(m); err == nil {
			t.Errorf("DecodeTitleCursor(%q) = nil error, want invalid cursor error", m)
		}
	}
}
