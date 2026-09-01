package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// T24-D2 (tasks/plan-t24-repositories.md): four interfaces gained a
// method no phase 02 service needed but real persistence does
// (EditionRepository/SourceRepository/SourceOfferingRepository's Save,
// SourceOfferingRepository's FindByID), and four aggregates gained a
// repository interface for the first time
// (ReadingProgress/Bookmark/Highlight/ReadingPreferences). Compiling
// against the fakes proves interface satisfaction; these prove the new
// methods actually round-trip, not just type-check.

func TestEditionRepository_Save(t *testing.T) {
	ctx := context.Background()
	editions := newFakeEditionRepository()
	edition := mustNewEdition(t, "edition-1", "work-1")

	if err := editions.Save(ctx, edition); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := editions.FindByID(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if reloaded.WorkID() != "work-1" {
		t.Fatalf("reloaded.WorkID() = %v, want work-1", reloaded.WorkID())
	}
}

func TestSourceRepository_Save(t *testing.T) {
	ctx := context.Background()
	sources := newFakeSourceRepository()
	caps := domain.SourceCapabilities{CanDownload: true}
	source, err := domain.NewSource("source-1", "My Source", caps, "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	if err := sources.Save(ctx, source); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := sources.FindByID(ctx, "source-1"); err != nil {
		t.Fatalf("FindByID after Save: %v", err)
	}
}

func TestSourceOfferingRepository_SaveAndFindByID(t *testing.T) {
	ctx := context.Background()
	offerings := newFakeSourceOfferingRepository()
	ref := mustNewFileReference(t, "ref-1", "epub")
	offering := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref, time.Now())

	if err := offerings.Save(ctx, offering); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := offerings.FindByID(ctx, "offering-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if reloaded.SourceID() != "source-1" {
		t.Fatalf("reloaded.SourceID() = %v, want source-1", reloaded.SourceID())
	}
}

// ReadingProgressRepository.FindByWork's NotFound-then-Save-then-found
// round trip is the shape FR-1's singleton lookup actually needs.
func TestReadingProgressRepository_FindByWorkAndSave(t *testing.T) {
	ctx := context.Background()
	repo := newFakeReadingProgressRepository()

	if _, err := repo.FindByWork(ctx, "work-1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("FindByWork before any Save: CategoryOf(err) = %v, want NotFound", domain.CategoryOf(err))
	}

	pct, _ := domain.NewPercentage(0.5)
	progress := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", time.Now())
	if err := repo.Save(ctx, progress); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := repo.FindByWork(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByWork after Save: %v", err)
	}
	if reloaded.Percentage() != pct {
		t.Fatalf("reloaded.Percentage() = %v, want %v", reloaded.Percentage(), pct)
	}
}

func TestBookmarkRepository_SaveFindByIDAndFindByEdition(t *testing.T) {
	ctx := context.Background()
	repo := newFakeBookmarkRepository()
	b1 := domain.NewBookmark("bookmark-1", "edition-1", "loc-1", "", time.Now())
	b2 := domain.NewBookmark("bookmark-2", "edition-1", "loc-2", "", time.Now())
	b3 := domain.NewBookmark("bookmark-3", "edition-OTHER", "loc-3", "", time.Now())

	for _, b := range []*domain.Bookmark{b1, b2, b3} {
		if err := repo.Save(ctx, b); err != nil {
			t.Fatalf("Save(%v): %v", b.ID(), err)
		}
	}

	if _, err := repo.FindByID(ctx, "bookmark-1"); err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	byEdition, err := repo.FindByEdition(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByEdition: %v", err)
	}
	if len(byEdition) != 2 {
		t.Fatalf("FindByEdition(edition-1) returned %d bookmarks, want 2 (edition-OTHER's must not leak in)", len(byEdition))
	}

	if err := repo.Delete(ctx, "bookmark-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.FindByID(ctx, "bookmark-1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatal("bookmark still findable after Delete")
	}
}

func TestHighlightRepository_SaveFindByIDAndFindByEdition(t *testing.T) {
	ctx := context.Background()
	repo := newFakeHighlightRepository()
	h := domain.NewHighlight("highlight-1", "edition-1", "loc-1", "loc-2", "", "", time.Now())

	if err := repo.Save(ctx, h); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := repo.FindByID(ctx, "highlight-1"); err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	byEdition, err := repo.FindByEdition(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByEdition: %v", err)
	}
	if len(byEdition) != 1 {
		t.Fatalf("FindByEdition returned %d highlights, want 1", len(byEdition))
	}
	if err := repo.Delete(ctx, "highlight-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestReadingPreferencesRepository_FindByDeviceAndSave(t *testing.T) {
	ctx := context.Background()
	repo := newFakeReadingPreferencesRepository()

	if _, err := repo.FindByDevice(ctx, "device-1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("FindByDevice before any Save: CategoryOf(err) = %v, want NotFound", domain.CategoryOf(err))
	}

	prefs := domain.NewReadingPreferences("device-1")
	prefs.Set("theme", "dark")
	if err := repo.Save(ctx, prefs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := repo.FindByDevice(ctx, "device-1")
	if err != nil {
		t.Fatalf("FindByDevice after Save: %v", err)
	}
	if reloaded.Settings()["theme"] != "dark" {
		t.Fatalf("reloaded.Settings()[\"theme\"] = %q, want dark", reloaded.Settings()["theme"])
	}
}
