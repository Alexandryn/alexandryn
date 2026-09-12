//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestReadingProgressRepository_SaveAndFindByWork_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	repo := postgres.NewReadingProgressRepository(pool)

	pct, err := domain.NewPercentage(0.5)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	p := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", observedAt)

	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByWork(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByWork: %v", err)
	}
	if got.ID() != "progress-1" {
		t.Fatalf("ID() = %v, want progress-1", got.ID())
	}
	if got.Percentage() != pct {
		t.Fatalf("Percentage() = %v, want %v", got.Percentage(), pct)
	}
	if got.DeviceID() != "device-1" {
		t.Fatalf("DeviceID() = %v, want device-1", got.DeviceID())
	}
	if !got.ObservedAt().Equal(observedAt) {
		t.Fatalf("ObservedAt() = %v, want %v", got.ObservedAt(), observedAt)
	}
	if got.PrecisePosition() != nil {
		t.Fatalf("PrecisePosition() = %v, want nil", got.PrecisePosition())
	}
}

func TestReadingProgressRepository_SaveAndFindByWork_WithPrecisePosition(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewReadingProgressRepository(pool)

	pct, err := domain.NewPercentage(0.75)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	p := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", time.Now().UTC().Truncate(time.Microsecond))

	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	editionRepo := postgres.NewEditionRepository(pool)
	svc := domain.NewReadingProgressService(editionRepo)
	if err := svc.AttachPrecisePosition(ctx, p, domain.PrecisePosition{EditionID: "edition-1", Value: "loc-42"}); err != nil {
		t.Fatalf("AttachPrecisePosition: %v", err)
	}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save after attach: %v", err)
	}

	got, err := repo.FindByWork(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByWork: %v", err)
	}
	if got.PrecisePosition() == nil {
		t.Fatal("PrecisePosition() = nil, want a value")
	}
	if got.PrecisePosition().EditionID != "edition-1" || got.PrecisePosition().Value != "loc-42" {
		t.Fatalf("PrecisePosition() = %+v, want {edition-1 loc-42}", got.PrecisePosition())
	}
}

func TestReadingProgressRepository_FindByWork_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewReadingProgressRepository(pool)

	_, err := repo.FindByWork(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestReadingProgressRepository_Save_UpdatesInPlace(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	repo := postgres.NewReadingProgressRepository(pool)

	pct1, err := domain.NewPercentage(0.1)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	p1 := domain.NewReadingProgress("progress-1", "work-1", pct1, "device-1", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, p1); err != nil {
		t.Fatalf("Save p1: %v", err)
	}

	pct2, err := domain.NewPercentage(0.9)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	p2 := domain.NewReadingProgress("progress-1", "work-1", pct2, "device-2", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, p2); err != nil {
		t.Fatalf("Save p2: %v", err)
	}

	got, err := repo.FindByWork(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByWork: %v", err)
	}
	if got.Percentage() != pct2 {
		t.Fatalf("Percentage() = %v, want %v", got.Percentage(), pct2)
	}
	if got.DeviceID() != "device-2" {
		t.Fatalf("DeviceID() = %v, want device-2", got.DeviceID())
	}
}

// At most one ReadingProgress per Work. Save is
// upsert-by-id, so this proves the real UNIQUE constraint on work_id
// itself catches a *different* id colliding on the same WorkID — not
// just the id-conflict path ON CONFLICT (id) already handles.
func TestReadingProgressRepository_Save_RejectsSecondIDForSameWork(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	repo := postgres.NewReadingProgressRepository(pool)

	pct, err := domain.NewPercentage(0.5)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	first := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	second := domain.NewReadingProgress("progress-2", "work-1", pct, "device-2", time.Now().UTC().Truncate(time.Microsecond))
	err = repo.Save(ctx, second)
	if domain.CategoryOf(err) != domain.Conflict {
		t.Fatalf("category = %v, want Conflict (singleton-per-Work violated)", domain.CategoryOf(err))
	}

	got, err := repo.FindByWork(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByWork after rejected Save: %v", err)
	}
	if got.ID() != "progress-1" {
		t.Fatalf("ID() = %v, want progress-1 (rejected Save must not have overwritten)", got.ID())
	}
}

// Reuses R4's own SQL-injection proof mechanism against
// ReadingProgressRepository's own INSERT.
func TestReadingProgressRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewReadingProgressRepository(pool)

	hostileDeviceID := `O'Brien'; DROP TABLE reading_progress; --`
	pct, err := domain.NewPercentage(0.5)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	p := domain.NewReadingProgress("progress-injection", "work-1", pct, domain.DeviceID(hostileDeviceID), time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save with hostile device id: %v", err)
	}

	got, err := repo.FindByWork(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByWork: %v", err)
	}
	if string(got.DeviceID()) != hostileDeviceID {
		t.Fatalf("DeviceID() did not round-trip byte-for-byte: got %q, want %q", got.DeviceID(), hostileDeviceID)
	}

	var tableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'reading_progress')",
	).Scan(&tableExists); err != nil {
		t.Fatalf("checking reading_progress table existence: %v", err)
	}
	if !tableExists {
		t.Fatal("reading_progress table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO reading_progress")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO reading_progress query — tracer wiring is broken")
	}
	for _, q := range insertQueries {
		if !strings.Contains(q.SQL, "$1") {
			t.Fatalf("traced SQL has no placeholder: %q", q.SQL)
		}
		if strings.Contains(q.SQL, hostileDeviceID) {
			t.Fatalf("hostile value interpolated directly into SQL text: %q", q.SQL)
		}
		found := false
		for _, arg := range q.Args {
			if s, ok := arg.(string); ok && s == hostileDeviceID {
				found = true
			}
		}
		if !found {
			t.Fatalf("hostile value not present in the query's separate argument list: %v", q.Args)
		}
	}
}
