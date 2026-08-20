package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// FR-1/FR-2 (domain-events.md): every event in the 23-type catalog
// implements exactly one of PublicEvent/SensitiveEvent, fixed at the
// type. Re-counted directly against domain-events.md's Domain model
// section before writing this table (13 public + 10 sensitive), not
// trusted from a cached figure.
func TestEventCatalog_EachTypeImplementsExactlyOneMarker(t *testing.T) {
	now := time.Now()

	publicEvents := []domain.Event{
		domain.NewWorkCreated("work-1", now),
		domain.NewEditionCreated("edition-1", now),
		domain.NewAuthorCreated("author-1", now),
		domain.NewWorkMerged("work-1", now),
		domain.NewWorkMergeUndone("work-1", now),
		domain.NewAuthorMerged("author-1", now),
		domain.NewAuthorMergeUndone("author-1", now),
		domain.NewWorkContainsAdded("work-1", now),
		domain.NewWorkContainsRemoved("work-1", now),
		domain.NewSourceCreated("source-1", now),
		domain.NewSourceRemoved("source-1", now),
		domain.NewSourceOfferingObserved("offering-1", now),
		domain.NewSourceOfferingRemoved("offering-1", now),
	}
	if len(publicEvents) != 13 {
		t.Fatalf("got %d public events listed, want 13 (domain-events.md's own catalog count)", len(publicEvents))
	}
	for _, e := range publicEvents {
		if _, ok := e.(domain.PublicEvent); !ok {
			t.Errorf("%s does not implement PublicEvent", e.Type())
		}
		if _, ok := e.(domain.SensitiveEvent); ok {
			t.Errorf("%s implements SensitiveEvent, want PublicEvent only", e.Type())
		}
	}

	sensitiveEvents := []domain.Event{
		domain.NewWorkImported("work-1", now),
		domain.NewEditionImported("edition-1", now),
		domain.NewLibraryEntryAdded("entry-1", now),
		domain.NewLibraryEntryRemoved("entry-1", now),
		domain.NewCollectionCreated("collection-1", now),
		domain.NewCollectionMemberAdded("collection-1", now),
		domain.NewCollectionMemberRemoved("collection-1", now),
		domain.NewReadingProgressUpdated("progress-1", now),
		domain.NewBookmarkCreated("bookmark-1", now),
		domain.NewHighlightCreated("highlight-1", now),
	}
	if len(sensitiveEvents) != 10 {
		t.Fatalf("got %d sensitive events listed, want 10 (domain-events.md's own catalog count)", len(sensitiveEvents))
	}
	for _, e := range sensitiveEvents {
		if _, ok := e.(domain.SensitiveEvent); !ok {
			t.Errorf("%s does not implement SensitiveEvent", e.Type())
		}
		if _, ok := e.(domain.PublicEvent); ok {
			t.Errorf("%s implements PublicEvent, want SensitiveEvent only", e.Type())
		}
	}
}

func TestEvent_CarriesAggregateIDAndOccurredAt(t *testing.T) {
	now := time.Now()
	e := domain.NewWorkCreated("work-42", now)

	if e.AggregateID() != "work-42" {
		t.Fatalf("AggregateID() = %q, want %q", e.AggregateID(), "work-42")
	}
	if !e.OccurredAt().Equal(now) {
		t.Fatalf("OccurredAt() = %v, want %v", e.OccurredAt(), now)
	}
	if e.Type() != "WorkCreated" {
		t.Fatalf("Type() = %q, want %q", e.Type(), "WorkCreated")
	}
}

// FR-4: a Sensitive event may be consumed by a sink declared over Event
// (both classes) — the Activity-feed exemption actually works, not just
// FR-3's restriction.
func TestEvent_BothClassesFlowThroughAWideSink(t *testing.T) {
	var received []domain.Event
	sink := func(e domain.Event) { received = append(received, e) }

	sink(domain.NewWorkCreated("work-1", time.Now()))
	sink(domain.NewReadingProgressUpdated("progress-1", time.Now()))

	if len(received) != 2 {
		t.Fatalf("got %d events through the wide sink, want 2", len(received))
	}
}
