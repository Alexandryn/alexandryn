//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestReadingPreferencesRepository_SaveAndFindByDevice_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewReadingPreferencesRepository(pool)

	p := domain.NewReadingPreferences("device-1")
	p.Set("font", "serif")
	p.Set("theme", "dark")

	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByDevice(ctx, "device-1")
	if err != nil {
		t.Fatalf("FindByDevice: %v", err)
	}
	if got.DeviceID() != "device-1" {
		t.Fatalf("DeviceID() = %v, want device-1", got.DeviceID())
	}
	if got.Settings()["font"] != "serif" || got.Settings()["theme"] != "dark" {
		t.Fatalf("Settings() = %v, want {font:serif theme:dark}", got.Settings())
	}
}

func TestReadingPreferencesRepository_SaveAndFindByDevice_EmptySettings(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewReadingPreferencesRepository(pool)

	p := domain.NewReadingPreferences("device-1")
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByDevice(ctx, "device-1")
	if err != nil {
		t.Fatalf("FindByDevice: %v", err)
	}
	if len(got.Settings()) != 0 {
		t.Fatalf("Settings() = %v, want empty", got.Settings())
	}
}

func TestReadingPreferencesRepository_FindByDevice_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewReadingPreferencesRepository(pool)

	_, err := repo.FindByDevice(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestReadingPreferencesRepository_Save_UpsertsByDevice(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewReadingPreferencesRepository(pool)

	p1 := domain.NewReadingPreferences("device-1")
	p1.Set("font", "serif")
	if err := repo.Save(ctx, p1); err != nil {
		t.Fatalf("Save p1: %v", err)
	}

	p2 := domain.NewReadingPreferences("device-1")
	p2.Set("font", "sans")
	p2.Set("spacing", "wide")
	if err := repo.Save(ctx, p2); err != nil {
		t.Fatalf("Save p2: %v", err)
	}

	got, err := repo.FindByDevice(ctx, "device-1")
	if err != nil {
		t.Fatalf("FindByDevice: %v", err)
	}
	if got.Settings()["font"] != "sans" || got.Settings()["spacing"] != "wide" {
		t.Fatalf("Settings() = %v, want {font:sans spacing:wide}", got.Settings())
	}
}

// Reuses R4's own SQL-injection proof mechanism against
// ReadingPreferencesRepository's own INSERT.
func TestReadingPreferencesRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	repo := postgres.NewReadingPreferencesRepository(pool)

	hostileDeviceID := `O'Brien'; DROP TABLE reading_preferences; --`
	p := domain.NewReadingPreferences(domain.DeviceID(hostileDeviceID))
	p.Set("font", "serif")
	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("Save with hostile device id: %v", err)
	}

	got, err := repo.FindByDevice(ctx, domain.DeviceID(hostileDeviceID))
	if err != nil {
		t.Fatalf("FindByDevice: %v", err)
	}
	if string(got.DeviceID()) != hostileDeviceID {
		t.Fatalf("DeviceID() did not round-trip byte-for-byte: got %q, want %q", got.DeviceID(), hostileDeviceID)
	}

	var tableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'reading_preferences')",
	).Scan(&tableExists); err != nil {
		t.Fatalf("checking reading_preferences table existence: %v", err)
	}
	if !tableExists {
		t.Fatal("reading_preferences table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO reading_preferences")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO reading_preferences query — tracer wiring is broken")
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
