//go:build integration

package observability_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/observability"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

func TestEventStore_RecordAndListEvents_LibraryScoped(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	store := observability.NewEventStore(pool, func() time.Time { return now })

	libA := "00000000-0000-0000-0000-000000000001"
	libB := "00000000-0000-0000-0000-000000000002"

	// Insert into libraries so FK passes
	_, _ = pool.Exec(ctx, `INSERT INTO libraries (id, name) VALUES ($1, 'Lib B') ON CONFLICT (id) DO NOTHING`, libB)

	// Record event for Lib A
	err := store.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind:     observability.EventJobEnqueued,
		LibraryID:     &libA,
		Payload:       map[string]any{"task": "scan"},
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("RecordEvent Lib A: %v", err)
	}

	// Record event for Lib B
	err = store.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind:     observability.EventJobCompleted,
		LibraryID:     &libB,
		Payload:       map[string]any{"task": "download"},
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("RecordEvent Lib B: %v", err)
	}

	// Record host-level event (no library)
	err = store.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind:     "host.reboot",
		LibraryID:     nil,
		Payload:       map[string]any{"reason": "upgrade"},
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("RecordEvent Host: %v", err)
	}

	// Query for Lib A: should see Lib A event + host event, but NOT Lib B event
	eventsA, err := store.ListEvents(ctx, &libA, 50)
	if err != nil {
		t.Fatalf("ListEvents Lib A: %v", err)
	}
	if len(eventsA) != 2 {
		t.Fatalf("ListEvents Lib A returned %d events, want 2", len(eventsA))
	}
	for _, ev := range eventsA {
		if ev.LibraryID != nil && *ev.LibraryID == libB {
			t.Errorf("Lib A list contained Lib B event: %+v", ev)
		}
	}

	// Query host events only (nil libraryID)
	hostEvents, err := store.ListEvents(ctx, nil, 50)
	if err != nil {
		t.Fatalf("ListEvents host: %v", err)
	}
	if len(hostEvents) != 1 || hostEvents[0].EventKind != "host.reboot" {
		t.Fatalf("hostEvents = %+v, want 1 host.reboot event", hostEvents)
	}
}

func TestEventStore_PurgeExpired(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	store := observability.NewEventStore(pool, func() time.Time { return now })

	libID := "00000000-0000-0000-0000-000000000001"

	// 1. Insert an event that was supposed to purge 1 day ago
	oldTime := now.Add(-31 * 24 * time.Hour)
	oldStore := observability.NewEventStore(pool, func() time.Time { return oldTime })
	err := oldStore.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind:     "old.event",
		LibraryID:     &libID,
		RetentionDays: 30, // purged at now - 1 day
	})
	if err != nil {
		t.Fatalf("Record old event: %v", err)
	}

	// 2. Insert a fresh event with 30 days retention from now
	err = store.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind:     "fresh.event",
		LibraryID:     &libID,
		RetentionDays: 30, // purged at now + 30 days
	})
	if err != nil {
		t.Fatalf("Record fresh event: %v", err)
	}

	// Run purge
	purged, err := store.PurgeExpired(ctx, now)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged %d rows, want 1", purged)
	}

	// Verify only fresh event remains
	events, err := store.ListEvents(ctx, &libID, 50)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].EventKind != "fresh.event" {
		t.Fatalf("events = %+v, want only fresh.event", events)
	}
}

func TestReaper_RunSweep(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	store := observability.NewEventStore(pool, func() time.Time { return now })

	spy := testutil.NewSpyHandler()
	logger := slog.New(spy)
	reaper := observability.NewReaper(store, time.Hour, logger)

	// Run sweep against empty database
	purged, err := reaper.RunSweep(ctx, now)
	if err != nil {
		t.Fatalf("RunSweep: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged = %d, want 0", purged)
	}
}
