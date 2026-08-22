//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestCollectionRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-2', 'Other')")
	repo := postgres.NewCollectionRepository(pool)

	c, err := domain.NewCollection("collection-1", "To Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	added1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	added2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	c.AddMember("work-1", added1)
	c.AddMember("work-2", added2)

	if err := repo.Save(ctx, c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "collection-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name() != "To Read" {
		t.Fatalf("Name() = %q, want %q", got.Name(), "To Read")
	}
	if len(got.Members()) != 2 {
		t.Fatalf("Members() = %v, want 2 entries", got.Members())
	}
	var foundWork1 bool
	for _, m := range got.Members() {
		if m.WorkID == "work-1" {
			foundWork1 = true
			if !m.AddedAt.Equal(added1) {
				t.Fatalf("work-1 AddedAt = %v, want %v", m.AddedAt, added1)
			}
		}
	}
	if !foundWork1 {
		t.Fatalf("Members() = %v, missing work-1", got.Members())
	}
}

func TestCollectionRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewCollectionRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestCollectionRepository_Save_ReplacesMembersOnUpdate(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-2', 'Other')")
	repo := postgres.NewCollectionRepository(pool)

	c, err := domain.NewCollection("collection-1", "To Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	c.AddMember("work-1", time.Now())
	if err := repo.Save(ctx, c); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	c.RemoveMember("work-1")
	c.AddMember("work-2", time.Now())
	if err := repo.Save(ctx, c); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "collection-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(got.Members()) != 1 || got.Members()[0].WorkID != "work-2" {
		t.Fatalf("Members() = %v, want exactly [work-2], not accumulated with work-1", got.Members())
	}
}

func TestCollectionRepository_Delete(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewCollectionRepository(pool)

	c, err := domain.NewCollection("collection-1", "To Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	if err := repo.Save(ctx, c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, "collection-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.FindByID(ctx, "collection-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}
}
