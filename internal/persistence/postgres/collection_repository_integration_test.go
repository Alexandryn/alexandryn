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

	if err := repo.Save(ctx, domain.DefaultLibraryID, c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, domain.DefaultLibraryID, "collection-1")
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

	_, err := repo.FindByID(context.Background(), domain.DefaultLibraryID, "nonexistent")
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
	if err := repo.Save(ctx, domain.DefaultLibraryID, c); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	c.RemoveMember("work-1")
	c.AddMember("work-2", time.Now())
	if err := repo.Save(ctx, domain.DefaultLibraryID, c); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := repo.FindByID(ctx, domain.DefaultLibraryID, "collection-1")
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
	if err := repo.Save(ctx, domain.DefaultLibraryID, c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, domain.DefaultLibraryID, "collection-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.FindByID(ctx, domain.DefaultLibraryID, "collection-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}

	// Delete non-existent returns NotFound
	if err := repo.Delete(ctx, domain.DefaultLibraryID, "nonexistent"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("delete nonexistent = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestCollectionRepository_FindAll(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewCollectionRepository(pool)

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('w1', 'Work 1')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('w2', 'Work 2')")

	c1, _ := domain.NewCollection("c1", "Z-Collection")
	c1.AddMember("w1", time.Now())
	c1.AddMember("w2", time.Now())
	_ = repo.Save(ctx, domain.DefaultLibraryID, c1)

	c2, _ := domain.NewCollection("c2", "A-Collection")
	c2.AddMember("w1", time.Now())
	_ = repo.Save(ctx, domain.DefaultLibraryID, c2)

	c3, _ := domain.NewCollection("c3", "Empty-Collection")
	_ = repo.Save(ctx, domain.DefaultLibraryID, c3)

	all, err := repo.FindAll(ctx, domain.DefaultLibraryID)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("len(all) = %d, want 3", len(all))
	}

	// Alphabetical order: A-Collection, Empty-Collection, Z-Collection
	if all[0].Name != "A-Collection" || all[0].WorkCount != 1 {
		t.Errorf("all[0] = %+v, want A-Collection with count 1", all[0])
	}
	if all[1].Name != "Empty-Collection" || all[1].WorkCount != 0 {
		t.Errorf("all[1] = %+v, want Empty-Collection with count 0", all[1])
	}
	if all[2].Name != "Z-Collection" || all[2].WorkCount != 2 {
		t.Errorf("all[2] = %+v, want Z-Collection with count 2", all[2])
	}
}

func TestCollectionRepository_FindDetail(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewCollectionRepository(pool)

	mustExecPool(t, pool, "INSERT INTO works (id, title, subtitle) VALUES ('w1', 'Dune', 'Chronicles')")
	mustExecPool(t, pool, "INSERT INTO authors (id, name) VALUES ('a1', 'Frank Herbert')")
	mustExecPool(t, pool, "INSERT INTO work_authors (work_id, author_id) VALUES ('w1', 'a1')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('e1', 'w1', 'en', 'Chilton')")
	mustExecPool(t, pool, "INSERT INTO library_entries (id, edition_id, added_at) VALUES ('le1', 'e1', '2026-01-01T00:00:00Z')")


	c, _ := domain.NewCollection("c1", "Sci-Fi Favorites")
	addedAt := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
	c.AddMember("w1", addedAt)
	_ = repo.Save(ctx, domain.DefaultLibraryID, c)

	detail, err := repo.FindDetail(ctx, domain.DefaultLibraryID, "c1")
	if err != nil {
		t.Fatalf("FindDetail: %v", err)
	}

	if detail.ID != "c1" || detail.Name != "Sci-Fi Favorites" {
		t.Fatalf("detail = %+v, want ID c1, name Sci-Fi Favorites", detail)
	}
	if len(detail.Works) != 1 {
		t.Fatalf("len(detail.Works) = %d, want 1", len(detail.Works))
	}
	w := detail.Works[0]
	if w.ID != "w1" || w.Title != "Dune" || w.Subtitle != "Chronicles" {
		t.Errorf("w = %+v, want Dune / Chronicles", w)
	}
	if len(w.Authors) != 1 || w.Authors[0] != "Frank Herbert" {
		t.Errorf("w.Authors = %v, want [Frank Herbert]", w.Authors)
	}
	if !w.IsOwned {
		t.Errorf("w.IsOwned = false, want true")
	}
	if w.AddedAt == nil || !w.AddedAt.Equal(addedAt) {
		t.Errorf("w.AddedAt = %v, want %v", w.AddedAt, addedAt)
	}

	// Non-existent collection returns NotFound
	_, err = repo.FindDetail(ctx, domain.DefaultLibraryID, "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("FindDetail(nonexistent) category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestCollectionRepository_AddMember_Idempotent(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewCollectionRepository(pool)

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('w1', 'Work 1')")
	c, _ := domain.NewCollection("c1", "My Collection")
	_ = repo.Save(ctx, domain.DefaultLibraryID, c)

	t1 := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	if err := repo.AddMember(ctx, domain.DefaultLibraryID, "c1", "w1", t1); err != nil {
		t.Fatalf("AddMember first: %v", err)
	}

	// Second add with different timestamp: should be no-op, preserving original t1
	t2 := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	if err := repo.AddMember(ctx, domain.DefaultLibraryID, "c1", "w1", t2); err != nil {
		t.Fatalf("AddMember second: %v", err)
	}

	detail, err := repo.FindDetail(ctx, domain.DefaultLibraryID, "c1")
	if err != nil {
		t.Fatalf("FindDetail: %v", err)
	}
	if len(detail.Works) != 1 {
		t.Fatalf("len(Works) = %d, want 1", len(detail.Works))
	}
	if detail.Works[0].AddedAt == nil || !detail.Works[0].AddedAt.Equal(t1) {
		t.Fatalf("AddedAt = %v, want original %v preserved", detail.Works[0].AddedAt, t1)
	}

	// Add to non-existent collection -> NotFound
	if err := repo.AddMember(ctx, domain.DefaultLibraryID, "nonexistent", "w1", t1); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}

	// Add non-existent work -> NotFound
	if err := repo.AddMember(ctx, domain.DefaultLibraryID, "c1", "nonexistent-work", t1); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestCollectionRepository_RemoveMember(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewCollectionRepository(pool)

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('w1', 'Work 1')")
	c, _ := domain.NewCollection("c1", "My Collection")
	c.AddMember("w1", time.Now())
	_ = repo.Save(ctx, domain.DefaultLibraryID, c)

	if err := repo.RemoveMember(ctx, domain.DefaultLibraryID, "c1", "w1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	// Removing non-member returns NotFound
	if err := repo.RemoveMember(ctx, domain.DefaultLibraryID, "c1", "w1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}

	// Removing from non-existent collection returns NotFound
	if err := repo.RemoveMember(ctx, domain.DefaultLibraryID, "nonexistent", "w1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestCollectionRepository_Rename(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewCollectionRepository(pool)

	c, _ := domain.NewCollection("c1", "Old Name")
	_ = repo.Save(ctx, domain.DefaultLibraryID, c)

	if err := repo.Rename(ctx, domain.DefaultLibraryID, "c1", "New Name"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	got, err := repo.FindByID(ctx, domain.DefaultLibraryID, "c1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name() != "New Name" {
		t.Fatalf("Name = %q, want %q", got.Name(), "New Name")
	}

	// Rename with empty name returns InvalidInput
	if err := repo.Rename(ctx, domain.DefaultLibraryID, "c1", ""); domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("Rename empty category = %v, want InvalidInput", domain.CategoryOf(err))
	}

	// Rename non-existent returns NotFound
	if err := repo.Rename(ctx, domain.DefaultLibraryID, "nonexistent", "Valid Name"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("Rename nonexistent category = %v, want NotFound", domain.CategoryOf(err))
	}
}


// TestCollectionRepository_LibraryScopeIsStrict is the #87 close-gate: a
// collection created in library A is invisible and immutable to every
// method scoped to library B, and a cross-library id reads back as
// NotFound with no existence oracle.
func TestCollectionRepository_LibraryScopeIsStrict(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO libraries (id, name, description, allow_reader_uploads, created_at, updated_at) VALUES ('lib-b', 'B', '', false, now(), now()) ON CONFLICT DO NOTHING")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('w1', 'T')")
	repo := postgres.NewCollectionRepository(pool)

	const libA = domain.DefaultLibraryID
	const libB = domain.LibraryID("lib-b")

	c, _ := domain.NewCollection("c-a", "A's collection")
	if err := repo.Save(ctx, libA, c); err != nil {
		t.Fatalf("Save in library A: %v", err)
	}

	if _, err := repo.FindByID(ctx, libB, "c-a"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("FindByID from library B: category = %v, want NotFound", domain.CategoryOf(err))
	}
	if _, err := repo.FindDetail(ctx, libB, "c-a"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("FindDetail from library B: category = %v, want NotFound", domain.CategoryOf(err))
	}
	if all, err := repo.FindAll(ctx, libB); err != nil || len(all) != 0 {
		t.Fatalf("FindAll library B: err=%v count=%d, want 0", err, len(all))
	}
	if err := repo.Rename(ctx, libB, "c-a", "hijacked"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("Rename from library B: category = %v, want NotFound", domain.CategoryOf(err))
	}
	if err := repo.AddMember(ctx, libB, "c-a", "w1", time.Now().UTC()); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("AddMember from library B: category = %v, want NotFound", domain.CategoryOf(err))
	}
	if err := repo.RemoveMember(ctx, libB, "c-a", "w1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("RemoveMember from library B: category = %v, want NotFound", domain.CategoryOf(err))
	}
	if err := repo.Delete(ctx, libB, "c-a"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("Delete from library B: category = %v, want NotFound", domain.CategoryOf(err))
	}

	// The collection still exists, untouched, in library A.
	got, err := repo.FindByID(ctx, libA, "c-a")
	if err != nil || got.Name() != "A's collection" {
		t.Fatalf("collection in library A after B's attempts: err=%v name=%q", err, safeName(got))
	}

	// A Save that names a foreign library cannot move an existing
	// collection out of library A.
	moved, _ := domain.NewCollection("c-a", "moved")
	if err := repo.Save(ctx, libB, moved); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("Save c-a into library B: category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func safeName(c *domain.Collection) string {
	if c == nil {
		return "<nil>"
	}
	return c.Name()
}
