//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestMetadataCacheRepository_SaveAndGetWork_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	fixedTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	currentTime := fixedTime
	nowFunc := func() time.Time { return currentTime }

	repo := postgres.NewMetadataCacheRepositoryWithClock(pool, nowFunc)

	authorKey1 := "OL21594A"
	authorKey2 := "OL99999A"
	coverURL := "/api/v1/discover/covers/8256301"

	detail := &openlibrary.DiscoverWorkDetail{
		Work: openlibrary.NormalisedWork{
			Title:       "Middlemarch",
			Subtitle:    "A Study of Provincial Life",
			Description: "A masterpiece.",
			Subjects:    []string{"Fiction", "Classic"},
			Authors: []openlibrary.NormalisedAuthor{
				{OpenLibraryAuthorKey: &authorKey1, Name: "George Eliot"},
				{OpenLibraryAuthorKey: &authorKey2, Name: "Second Author"},
			},
			CoverURL: &coverURL,
		},
		Editions: []openlibrary.NormalisedEdition{
			{
				Title:                 "Middlemarch Edition 1",
				Publisher:             "Penguin",
				PublishDate:           "2003",
				Language:              "eng",
				OpenLibraryEditionKey: "OL7353617M",
				CoverURL:              &coverURL,
			},
		},
	}

	// 1. Initial Save
	if err := repo.SaveWork(ctx, "OL82563W", detail); err != nil {
		t.Fatalf("SaveWork: %v", err)
	}

	// 2. Fetch fresh cache hit
	cached, hit, err := repo.GetWork(ctx, "OL82563W")
	if err != nil {
		t.Fatalf("GetWork: %v", err)
	}
	if !hit || cached == nil {
		t.Fatal("expected cache hit, got miss")
	}

	if cached.Work.Title != "Middlemarch" || cached.Work.Subtitle != "A Study of Provincial Life" {
		t.Errorf("unexpected work title/subtitle: %s / %s", cached.Work.Title, cached.Work.Subtitle)
	}
	if len(cached.Work.Authors) != 2 || cached.Work.Authors[0].Name != "George Eliot" || cached.Work.Authors[1].Name != "Second Author" {
		t.Errorf("unexpected work authors: %+v", cached.Work.Authors)
	}
	if len(cached.Editions) != 1 || cached.Editions[0].OpenLibraryEditionKey != "OL7353617M" {
		t.Errorf("unexpected editions: %+v", cached.Editions)
	}

	// 3. Test 30-day staleness window
	// Fast-forward clock by 31 days
	currentTime = fixedTime.Add(31 * 24 * time.Hour)
	staleCached, hit, err := repo.GetWork(ctx, "OL82563W")
	if err != nil {
		t.Fatalf("GetWork after 31 days: %v", err)
	}
	if hit || staleCached != nil {
		t.Errorf("expected cache miss due to staleness after 31 days, got hit=%v", hit)
	}
}

func TestMetadataCacheRepository_SaveAndGetAuthor(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	fixedTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	currentTime := fixedTime
	nowFunc := func() time.Time { return currentTime }

	repo := postgres.NewMetadataCacheRepositoryWithClock(pool, nowFunc)

	authorKey := "OL21594A"
	author := &openlibrary.NormalisedAuthor{
		OpenLibraryAuthorKey: &authorKey,
		Name:                 "George Eliot",
	}

	if err := repo.SaveAuthor(ctx, author); err != nil {
		t.Fatalf("SaveAuthor: %v", err)
	}

	cached, hit, err := repo.GetAuthor(ctx, "OL21594A")
	if err != nil {
		t.Fatalf("GetAuthor: %v", err)
	}
	if !hit || cached == nil {
		t.Fatal("expected author cache hit, got miss")
	}
	if cached.Name != "George Eliot" {
		t.Errorf("expected George Eliot, got %s", cached.Name)
	}

	// Stale after 31 days
	currentTime = fixedTime.Add(31 * 24 * time.Hour)
	_, hit, err = repo.GetAuthor(ctx, "OL21594A")
	if err != nil {
		t.Fatalf("GetAuthor: %v", err)
	}
	if hit {
		t.Error("expected author cache miss after 31 days")
	}
}

func TestMetadataCacheRepository_SaveWork_NilSubjectsAndEditionPruning(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	repo := postgres.NewMetadataCacheRepository(pool)

	// 1. Initial save with 2 editions and nil subjects
	initialDetail := &openlibrary.DiscoverWorkDetail{
		Work: openlibrary.NormalisedWork{
			Title:    "Title Without Subjects",
			Subjects: nil, // Must not violate NOT NULL constraint in SQL
		},
		Editions: []openlibrary.NormalisedEdition{
			{Title: "Old Edition 1", OpenLibraryEditionKey: "OL101M"},
			{Title: "Old Edition 2", OpenLibraryEditionKey: "OL102M"},
		},
	}

	if err := repo.SaveWork(ctx, "OL999W", initialDetail); err != nil {
		t.Fatalf("SaveWork with nil subjects: %v", err)
	}

	cached, hit, err := repo.GetWork(ctx, "OL999W")
	if err != nil || !hit {
		t.Fatalf("GetWork failed: hit=%v, err=%v", hit, err)
	}
	if len(cached.Editions) != 2 {
		t.Fatalf("expected 2 editions initially, got %d", len(cached.Editions))
	}
	if cached.Work.Subjects == nil {
		t.Fatal("expected non-nil subjects slice from query")
	}

	// 2. Refresh work with only 1 edition -> previous editions pruned
	refreshedDetail := &openlibrary.DiscoverWorkDetail{
		Work: openlibrary.NormalisedWork{
			Title: "Refreshed Title",
		},
		Editions: []openlibrary.NormalisedEdition{
			{Title: "New Single Edition", OpenLibraryEditionKey: "OL201M"},
		},
	}

	if err := repo.SaveWork(ctx, "OL999W", refreshedDetail); err != nil {
		t.Fatalf("SaveWork refresh: %v", err)
	}

	cachedAfterRefresh, hit, err := repo.GetWork(ctx, "OL999W")
	if err != nil || !hit {
		t.Fatalf("GetWork after refresh failed: hit=%v, err=%v", hit, err)
	}
	if len(cachedAfterRefresh.Editions) != 1 {
		t.Fatalf("expected exactly 1 edition after refresh, got %d", len(cachedAfterRefresh.Editions))
	}
	if cachedAfterRefresh.Editions[0].OpenLibraryEditionKey != "OL201M" {
		t.Errorf("expected edition OL201M, got %s", cachedAfterRefresh.Editions[0].OpenLibraryEditionKey)
	}
}
