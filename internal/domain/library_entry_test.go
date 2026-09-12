package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// A LibraryEntry references exactly one Edition (not a Work directly) and
// records when it was added.
func TestNewLibraryEntry(t *testing.T) {
	now := time.Now()
	entry := domain.NewLibraryEntry(domain.LibraryEntryID("entry-1"), domain.EditionID("edition-1"), now)

	if entry.ID() != domain.LibraryEntryID("entry-1") {
		t.Fatalf("ID() = %v, want entry-1", entry.ID())
	}
	if entry.EditionID() != domain.EditionID("edition-1") {
		t.Fatalf("EditionID() = %v, want edition-1", entry.EditionID())
	}
	if !entry.AddedAt().Equal(now) {
		t.Fatalf("AddedAt() = %v, want %v", entry.AddedAt(), now)
	}
}
