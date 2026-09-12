package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Removing a Source removes every SourceOffering referencing it, and
// does not touch LibraryEntry records. The cascade applies as a single
// atomic unit composed through the Transactor.
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
	tx := &fakeTransactor{}

	// An Edition that also has a LibraryEntry — untouched by this
	// service, which never even holds a reference to this repository.
	entries := newFakeLibraryEntryRepository(domain.NewLibraryEntry("entry-1", "edition-1", now))

	svc := domain.NewSourceRemovalService(sources, offerings, tx)
	sourceEvent, offeringEvents, err := svc.Remove(ctx, "source-1", now)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if tx.callCount() != 1 {
		t.Fatalf("Transactor.InTx called %d times, want exactly 1 — the whole cascade must be one atomic unit", tx.callCount())
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

	svc := domain.NewSourceRemovalService(sources, offerings, &fakeTransactor{})
	_, offeringEvents, err := svc.Remove(ctx, "source-1", now)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(offeringEvents) != 0 {
		t.Fatalf("got %d offering-removed events, want 0", len(offeringEvents))
	}
}

// If the transaction itself fails to begin (distinct from the operation's
// own logic failing), Remove must surface that error and must not have
// deleted the Source — the whole point of composing through InTx.
func TestSourceRemovalService_Remove_TransactionBeginFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	caps := domain.SourceCapabilities{CanDownload: true}
	source, err := domain.NewSource("source-1", "My Source", caps, "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	sources := newFakeSourceRepository(source)
	offerings := newFakeSourceOfferingRepository()

	svc := domain.NewSourceRemovalService(sources, offerings, failingTransactor{})
	_, _, err = svc.Remove(ctx, "source-1", now)
	if err == nil {
		t.Fatal("Remove with a failing Transactor = nil error, want an error")
	}

	if _, err := sources.FindByID(ctx, "source-1"); err != nil {
		t.Fatalf("Source was deleted despite the transaction never beginning: %v", err)
	}
}
