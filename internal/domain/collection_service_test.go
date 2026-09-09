package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

const testLib = domain.LibraryID("lib-test")

func TestCollectionService_Create(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	collections := newFakeCollectionRepository()
	ids := testutil.NewFakeIDGenerator("collection-1")
	svc := domain.NewCollectionService(collections, ids)

	c, event, err := svc.Create(ctx, testLib, "Want to Read", now)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Name() != "Want to Read" {
		t.Fatalf("Name() = %q, want %q", c.Name(), "Want to Read")
	}
	if event.AggregateID() != "collection-1" {
		t.Fatalf("event.AggregateID() = %v, want collection-1", event.AggregateID())
	}
	if len(c.Members()) != 0 {
		t.Fatalf("Members() = %v, want empty (FR-8, legal at creation)", c.Members())
	}
}

func TestCollectionService_Create_InvalidNameRejected(t *testing.T) {
	ctx := context.Background()
	collections := newFakeCollectionRepository()
	ids := testutil.NewFakeIDGenerator("collection-1")
	svc := domain.NewCollectionService(collections, ids)

	_, _, err := svc.Create(ctx, testLib, "", time.Now())
	if err == nil {
		t.Fatal("Create(\"\") = nil error, want an error")
	}
}

// The "want to read" case: a Work can be a Collection member with zero
// LibraryEntrys referencing any of its Editions — this test only proves
// the Collection side (membership never checks or requires ownership);
// LibraryService's own tests already prove LibraryEntry independence.
func TestCollectionService_AddMember_NoOwnershipRequired(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	c, err := domain.NewCollection("collection-1", "Want to Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	collections := newFakeCollectionRepository(c)
	svc := domain.NewCollectionService(collections, testutil.NewFakeIDGenerator())

	event, err := svc.AddMember(ctx, testLib, "collection-1", "work-never-owned", now)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if event.AggregateID() != "collection-1" {
		t.Fatalf("event.AggregateID() = %v, want collection-1", event.AggregateID())
	}

	reloaded, _ := collections.FindByID(ctx, testLib, "collection-1")
	if len(reloaded.Members()) != 1 || reloaded.Members()[0].WorkID != "work-never-owned" {
		t.Fatalf("Members() = %v, want [work-never-owned]", reloaded.Members())
	}
}

func TestCollectionService_RemoveMember(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	c, err := domain.NewCollection("collection-1", "Want to Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	c.AddMember("work-1", now)
	collections := newFakeCollectionRepository(c)
	svc := domain.NewCollectionService(collections, testutil.NewFakeIDGenerator())

	if _, err := svc.RemoveMember(ctx, testLib, "collection-1", "work-1", now); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	reloaded, _ := collections.FindByID(ctx, testLib, "collection-1")
	if len(reloaded.Members()) != 0 {
		t.Fatalf("Members() after removal = %v, want empty", reloaded.Members())
	}
}

// FR-10: deleting a Collection removes its membership records only — it
// MUST NOT cascade to Work, Edition, or any LibraryEntry. Trivially true
// here (Delete only ever touches the CollectionRepository), asserted
// directly rather than left implicit.
func TestCollectionService_Delete(t *testing.T) {
	ctx := context.Background()
	c, err := domain.NewCollection("collection-1", "Want to Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	c.AddMember("work-1", time.Now())
	collections := newFakeCollectionRepository(c)
	svc := domain.NewCollectionService(collections, testutil.NewFakeIDGenerator())

	if err := svc.Delete(ctx, testLib, "collection-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = collections.FindByID(ctx, testLib, "collection-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatal("Collection still exists after Delete")
	}
}
