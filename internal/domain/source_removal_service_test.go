package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// domain-source.md FR-6: removing a Source MUST remove every
// SourceOffering referencing it, and MUST NOT touch domain-library.md's
// LibraryEntry records — the spec's own required fixture: a Source with
// offerings for an Edition that also has a LibraryEntry. Offerings
// removed, LibraryEntry untouched. Proven structurally here: the
// LibraryEntryRepository fake is never even passed to
// SourceRemovalService, so there is nothing for it to touch.
func TestSourceRemovalService_Remove(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	caps := domain.SourceCapabilities{CanDownload: true}
	source, err := domain.NewSource("source-1", "My Source", caps, "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	ref := mustNewFileReference(t, "ref-1", "epub")
	offering1 := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref, now)
	offering2 := domain.NewSourceOffering("offering-2", "source-1", "edition-2", ref, now)

	sources := newFakeSourceRepository(source)
	offerings := newFakeSourceOfferingRepository(offering1, offering2)

	// An Edition that also has a LibraryEntry — untouched by this
	// service, which never even holds a reference to this repository.
	entries := newFakeLibraryEntryRepository(domain.NewLibraryEntry("entry-1", "edition-1", now))

	svc := domain.NewSourceRemovalService(sources, offerings)
	sourceEvent, offeringEvents, err := svc.Remove(ctx, "source-1", now)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if sourceEvent.AggregateID() != "source-1" {
		t.Fatalf("sourceEvent.AggregateID() = %v, want source-1", sourceEvent.AggregateID())
	}
	if len(offeringEvents) != 2 {
		t.Fatalf("got %d offering-removed events, want 2", len(offeringEvents))
	}

	remaining, err := offerings.FindBySource(ctx, "source-1")
	if err != nil {
		t.Fatalf("FindBySource after Remove: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("offerings remaining after Remove = %v, want none", remaining)
	}

	_, err = sources.FindByID(ctx, "source-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatal("Source still exists after Remove")
	}

	// The LibraryEntry for edition-1 is completely unaffected.
	if _, err := entries.FindByEdition(ctx, "edition-1"); err != nil {
		t.Fatalf("LibraryEntry was affected by SourceRemovalService.Remove: %v", err)
	}
}

func TestSourceRemovalService_Remove_NoOfferingsIsFine(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	caps := domain.SourceCapabilities{CanDownload: true}
	source, err := domain.NewSource("source-1", "My Source", caps, "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	sources := newFakeSourceRepository(source)
	offerings := newFakeSourceOfferingRepository()

	svc := domain.NewSourceRemovalService(sources, offerings)
	_, offeringEvents, err := svc.Remove(ctx, "source-1", now)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(offeringEvents) != 0 {
		t.Fatalf("got %d offering-removed events, want 0", len(offeringEvents))
	}
}
