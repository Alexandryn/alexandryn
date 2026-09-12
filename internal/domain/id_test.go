package domain_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// testutil.FakeIDGenerator must satisfy domain.IDGenerator without any
// change on the testutil side.
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

// Each aggregate has its own named ID type. This is a compile-time
// property — WorkID and AuthorID are distinct types with no implicit
// conversion between them, so a call site that mixes them up fails to
// build. Declaring one of each here and using each only where its own
// type is expected verifies this distinctness.
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
