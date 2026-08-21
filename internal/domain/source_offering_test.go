package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func mustNewFileReference(t *testing.T, referenceID, format string) domain.FileReference {
	t.Helper()
	ref, err := domain.NewFileReference(referenceID, format, nil)
	if err != nil {
		t.Fatalf("NewFileReference: %v", err)
	}
	return ref
}

// domain-source.md FR-2: a SourceOffering is uniquely identified by
// Source + Edition + Format — the same source offering the same edition
// in two formats is two rows, not one; re-observing the same format
// updates that row's timestamp rather than creating a new one. This is a
// repository-level upsert concern (a composite unique key), not a graph
// invariant needing a domain service (ADR 0020's single-value carve-out
// applies: the key itself is decidable from the two offerings' own
// values, no other record needs reading) — proven here as the type's own
// UniquenessKey, which a repository implementation keys its upsert on.
func TestSourceOffering_UniquenessKey(t *testing.T) {
	now := time.Now()
	epubRef := mustNewFileReference(t, "ref-epub", "epub")
	pdfRef := mustNewFileReference(t, "ref-pdf", "pdf")

	epub := domain.NewSourceOffering("offering-1", "source-1", "edition-1", epubRef, now)
	pdf := domain.NewSourceOffering("offering-2", "source-1", "edition-1", pdfRef, now)

	if epub.UniquenessKey() == pdf.UniquenessKey() {
		t.Fatal("same Source+Edition, different Format: UniquenessKey() must differ — two rows, not one")
	}

	reobserved := domain.NewSourceOffering("offering-1", "source-1", "edition-1", epubRef, now.Add(time.Hour))
	if epub.UniquenessKey() != reobserved.UniquenessKey() {
		t.Fatal("same Source+Edition+Format, different observed-at: UniquenessKey() must match — one row, re-observed")
	}
}

func TestNewSourceOffering(t *testing.T) {
	now := time.Now()
	ref := mustNewFileReference(t, "ref-1", "epub")
	o := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref, now)

	if o.SourceID() != "source-1" {
		t.Fatalf("SourceID() = %v, want source-1", o.SourceID())
	}
	if o.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", o.EditionID())
	}
	if !o.ObservedAt().Equal(now) {
		t.Fatalf("ObservedAt() = %v, want %v", o.ObservedAt(), now)
	}
}
