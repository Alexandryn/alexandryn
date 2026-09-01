//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestImportCandidateRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	pool := schemaTestPool(t)
	repo := postgres.NewImportCandidateRepository(pool)

	mustExecPool(t, pool, `INSERT INTO sources (id, label, kind, can_list, can_search, can_download)
		VALUES ('src-cand-1', 'Source 1', 'local-folder', true, true, true) ON CONFLICT DO NOTHING`)

	ref, err := domain.NewFileReference("ref-book-1", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference: %v", err)
	}

	jobID := "job-101"
	now := time.Now().UTC().Truncate(time.Microsecond)
	cand := postgres.ImportCandidateRecord{
		ID:            "cand-101",
		SourceID:      "src-cand-1",
		FileReference: ref,
		Status:        postgres.ImportCandidateStatusQueued,
		JobID:         &jobID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 1. Create
	if err := repo.Create(ctx, cand); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 2. Get
	fetched, err := repo.Get(ctx, "cand-101")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.ID != "cand-101" || fetched.Status != postgres.ImportCandidateStatusQueued || fetched.FileReference.ReferenceID != "ref-book-1" {
		t.Fatalf("unexpected fetched candidate: %+v", fetched)
	}

	// 3. ExistsBySourceAndFileRefID
	exists, err := repo.ExistsBySourceAndFileRefID(ctx, "src-cand-1", "ref-book-1")
	if err != nil {
		t.Fatalf("ExistsBySourceAndFileRefID: %v", err)
	}
	if !exists {
		t.Fatal("expected candidate to exist")
	}

	existsOther, err := repo.ExistsBySourceAndFileRefID(ctx, "src-cand-1", "ref-nonexistent")
	if err != nil {
		t.Fatalf("ExistsBySourceAndFileRefID nonexistent: %v", err)
	}
	if existsOther {
		t.Fatal("expected non-existent candidate to not exist")
	}

	// 4. UpdateExtractedAndMatches
	metaJSON := []byte(`{"title":"Extracted Title","authors":["Author A"]}`)
	matchJSON := []byte(`[{"type":"open_library_work","confidence":"high","title":"Extracted Title"}]`)
	updateTime := now.Add(time.Minute)
	if err := repo.UpdateExtractedAndMatches(ctx, "cand-101", postgres.ImportCandidateStatusPending, metaJSON, matchJSON, updateTime); err != nil {
		t.Fatalf("UpdateExtractedAndMatches: %v", err)
	}

	fetched, err = repo.Get(ctx, "cand-101")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if fetched.Status != postgres.ImportCandidateStatusPending {
		t.Fatalf("status = %q, want pending", fetched.Status)
	}
	var parsedMeta map[string]any
	if err := json.Unmarshal(fetched.ExtractedMetadata, &parsedMeta); err != nil {
		t.Fatalf("unmarshal extracted metadata: %v", err)
	}
	if parsedMeta["title"] != "Extracted Title" {
		t.Fatalf("parsed title = %v, want Extracted Title", parsedMeta["title"])
	}

	// 5. UpdateStatus to confirmed
	confirmedTime := updateTime.Add(time.Minute)
	if err := repo.UpdateStatus(ctx, "cand-101", postgres.ImportCandidateStatusConfirmed, confirmedTime); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	fetched, err = repo.Get(ctx, "cand-101")
	if err != nil {
		t.Fatalf("Get after confirm: %v", err)
	}
	if fetched.Status != postgres.ImportCandidateStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", fetched.Status)
	}

	// 6. List with filter
	list, err := repo.List(ctx, strPtr("src-cand-1"), strPtr(postgres.ImportCandidateStatusConfirmed))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != "cand-101" {
		t.Fatalf("List returned %+v, want 1 item", list)
	}

	// 7. UpdateFailed
	failTime := confirmedTime.Add(time.Minute)
	if err := repo.UpdateFailed(ctx, "cand-101", "corrupt file", failTime); err != nil {
		t.Fatalf("UpdateFailed: %v", err)
	}
	fetched, err = repo.Get(ctx, "cand-101")
	if err != nil {
		t.Fatalf("Get after fail: %v", err)
	}
	if fetched.Status != postgres.ImportCandidateStatusFailed || fetched.LastError == nil || *fetched.LastError != "corrupt file" {
		t.Fatalf("unexpected failed state: %+v", fetched)
	}

	// 8. Get NotFound
	_, err = repo.Get(ctx, "nonexistent-id")
	if err == nil || domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("Get nonexistent error = %v, want NotFound", err)
	}
}

func strPtr(s string) *string {
	return &s
}
