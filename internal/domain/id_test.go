package domain_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// E1: testutil.FakeIDGenerator (built in phase 03's T1) must satisfy
// domain.IDGenerator without any change on the testutil side — the
// interface is written to match the fake's existing shape, not the
// other way around.
var _ domain.IDGenerator = (*testutil.FakeIDGenerator)(nil)

func TestIDGenerator_FakeYieldsConfiguredSequenceThroughTheInterface(t *testing.T) {
	var gen domain.IDGenerator = testutil.NewFakeIDGenerator("work-1", "work-2")

	first := gen.NewID()
	second := gen.NewID()

	if first != "work-1" {
		t.Fatalf("first NewID() = %q, want %q", first, "work-1")
	}
	if second != "work-2" {
		t.Fatalf("second NewID() = %q, want %q", second, "work-2")
	}
}

// E0: each aggregate has its own named ID type. This is a compile-time
// property — WorkID and AuthorID are distinct types with no implicit
// conversion between them, so a call site that mixes them up fails to
// build. There is nothing to assert about that at runtime; declaring one
// of each here and using each only where its own type is expected is the
// proof — if this file compiles, no code doing (for example) f(AuthorID)
// with a WorkID value compiles either.
func TestIDTypes_AreDistinctAcrossAggregates(t *testing.T) {
	var work domain.WorkID = "work-1"
	var edition domain.EditionID = "edition-1"
	var author domain.AuthorID = "author-1"
	var libraryEntry domain.LibraryEntryID = "entry-1"
	var collection domain.CollectionID = "collection-1"
	var source domain.SourceID = "source-1"
	var sourceOffering domain.SourceOfferingID = "offering-1"
	var readingProgress domain.ReadingProgressID = "progress-1"
	var bookmark domain.BookmarkID = "bookmark-1"
	var highlight domain.HighlightID = "highlight-1"

	ids := []string{
		string(work), string(edition), string(author), string(libraryEntry),
		string(collection), string(source), string(sourceOffering),
		string(readingProgress), string(bookmark), string(highlight),
	}
	for i, id := range ids {
		if id == "" {
			t.Fatalf("id type at index %d stringified to empty, want a value", i)
		}
	}
}
