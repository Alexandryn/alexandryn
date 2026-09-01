package extract_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

func TestExtractedMetadata_Validation(t *testing.T) {
	isbn := "9780306406157"
	lang := "en"
	pub := "Acme Publishing"
	desc := "A great book"
	cover := []byte{0xFF, 0xD8, 0xFF, 0xE0} // fake jpeg header

	meta, err := extract.NewExtractedMetadata("Valid Title", []string{"Author One", "Author Two"}, &isbn, &lang, &pub, &desc, cover, extract.FormatEPUB)
	if err != nil {
		t.Fatalf("NewExtractedMetadata failed: %v", err)
	}

	if meta.Title != "Valid Title" {
		t.Errorf("Title = %q, want %q", meta.Title, "Valid Title")
	}
	if len(meta.Authors) != 2 || meta.Authors[0] != "Author One" {
		t.Errorf("Authors = %+v, unexpected", meta.Authors)
	}
	if meta.ISBN == nil || *meta.ISBN != "9780306406157" {
		t.Errorf("ISBN = %v, want 9780306406157", meta.ISBN)
	}
	if meta.Format != extract.FormatEPUB {
		t.Errorf("Format = %v, want epub", meta.Format)
	}
}

func TestExtractedMetadata_WhitespaceTitleIsErrNoTitle(t *testing.T) {
	cases := []string{"", "   ", "\t\n  \r\n"}
	for _, title := range cases {
		_, err := extract.NewExtractedMetadata(title, nil, nil, nil, nil, nil, nil, extract.FormatEPUB)
		if err != extract.ErrNoTitle {
			t.Errorf("title %q err = %v, want ErrNoTitle", title, err)
		}
	}
}

func TestExtractedMetadata_FieldSanitization(t *testing.T) {
	// Invalid ISBN is dropped (set to nil), not a whole-extraction failure
	invalidISBN := "12345-not-isbn"
	// Oversized string is truncated or sanitized per domain rules
	overlongPub := strings.Repeat("A", 400)
	meta, err := extract.NewExtractedMetadata("Clean Title", []string{"  Author Spaced  "}, &invalidISBN, nil, &overlongPub, nil, nil, extract.FormatPDF)
	if err != nil {
		t.Fatalf("NewExtractedMetadata: %v", err)
	}
	if meta.ISBN != nil {
		t.Errorf("expected invalid ISBN to be dropped, got %v", *meta.ISBN)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "Author Spaced" {
		t.Errorf("expected trimmed author, got %v", meta.Authors)
	}
	if meta.Publisher != nil && len(*meta.Publisher) > 255 {
		t.Errorf("expected publisher to be bounded, length = %d", len(*meta.Publisher))
	}
}

func TestExtractedMetadata_CoverSizeCap(t *testing.T) {
	// Cover > 10 MiB is dropped
	hugeCover := make([]byte, 11*1024*1024)
	meta, err := extract.NewExtractedMetadata("Book With Huge Cover", nil, nil, nil, nil, nil, hugeCover, extract.FormatCBZ)
	if err != nil {
		t.Fatalf("NewExtractedMetadata: %v", err)
	}
	if meta.CoverBytes != nil {
		t.Error("expected cover > 10 MiB to be dropped")
	}
}
