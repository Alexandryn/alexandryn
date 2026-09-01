package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type fakeImportCandidateRepo struct {
	candidates map[string]postgres.ImportCandidateRecord
}

func (r *fakeImportCandidateRepo) Create(ctx context.Context, rec postgres.ImportCandidateRecord) error {
	r.candidates[rec.ID] = rec
	return nil
}

func (r *fakeImportCandidateRepo) Get(ctx context.Context, id string) (postgres.ImportCandidateRecord, error) {
	c, ok := r.candidates[id]
	if !ok {
		return postgres.ImportCandidateRecord{}, &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	return c, nil
}

func (r *fakeImportCandidateRepo) List(ctx context.Context, sourceID *string, status *string) ([]postgres.ImportCandidateRecord, error) {
	var list []postgres.ImportCandidateRecord
	for _, c := range r.candidates {
		if sourceID != nil && *sourceID != "" && c.SourceID != *sourceID {
			continue
		}
		if status != nil && *status != "" && c.Status != *status {
			continue
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *fakeImportCandidateRepo) UpdateStatus(ctx context.Context, id string, status string, now time.Time) error {
	c, ok := r.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	c.Status = status
	c.UpdatedAt = now
	r.candidates[id] = c
	return nil
}

func (r *fakeImportCandidateRepo) UpdateExtractedAndMatches(ctx context.Context, id string, status string, extracted []byte, matches []byte, now time.Time) error {
	c, ok := r.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	c.Status = status
	c.ExtractedMetadata = extracted
	c.MatchCandidates = matches
	c.UpdatedAt = now
	r.candidates[id] = c
	return nil
}

func (r *fakeImportCandidateRepo) UpdateFailed(ctx context.Context, id string, lastError string, now time.Time) error {
	c, ok := r.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	c.Status = postgres.ImportCandidateStatusFailed
	c.LastError = &lastError
	c.UpdatedAt = now
	r.candidates[id] = c
	return nil
}

func (r *fakeImportCandidateRepo) ExistsBySourceAndFileRefID(ctx context.Context, sourceID string, fileRefID string) (bool, error) {
	for _, c := range r.candidates {
		if c.SourceID == sourceID && c.FileReference.ReferenceID == fileRefID {
			return true, nil
		}
	}
	return false, nil
}

type fakeDiscoveryRunner struct {
	res importer.DiscoverResult
	err error
}

func (d *fakeDiscoveryRunner) Discover(ctx context.Context, sourceID string, now time.Time) (importer.DiscoverResult, error) {
	if d.err != nil {
		return importer.DiscoverResult{}, d.err
	}
	return d.res, nil
}

func TestImportDiscoverHandler(t *testing.T) {
	runner := &fakeDiscoveryRunner{
		res: importer.DiscoverResult{
			DiscoveredCount: 3,
			SkippedCount:    1,
			JobIDs:          []string{"job-1", "job-2", "job-3"},
		},
	}

	h := transporthttp.ImportDiscoverHandler(runner)

	body := []byte(`{"sourceId":"src-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/discover", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 Accepted, body = %s", w.Code, w.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if res["discoveredCount"] != float64(3) || res["skippedCount"] != float64(1) {
		t.Fatalf("response = %+v, unexpected", res)
	}
}

func TestImportListCandidatesHandler(t *testing.T) {
	ref, _ := domain.NewFileReference("ref-1", "epub", nil)
	now := time.Now().UTC().Truncate(time.Microsecond)
	repo := &fakeImportCandidateRepo{
		candidates: map[string]postgres.ImportCandidateRecord{
			"cand-1": {
				ID:                "cand-1",
				SourceID:          "src-1",
				FileReference:     ref,
				Status:            postgres.ImportCandidateStatusPending,
				ExtractedMetadata: []byte(`{"title":"Dune","authors":["Frank Herbert"]}`),
				MatchCandidates:   []byte(`[{"type":"open_library_work","confidence":"high","title":"Dune"}]`),
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		},
	}

	h := transporthttp.ImportCandidatesListHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/import/candidates?sourceId=src-1&status=pending", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	var res struct {
		Candidates []transporthttp.ImportCandidateWireDTO `json:"candidates"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal candidates: %v", err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].ID != "cand-1" || res.Candidates[0].Status != "pending" {
		t.Fatalf("candidates = %+v, unexpected", res.Candidates)
	}
}

func TestImportRejectHandler(t *testing.T) {
	ref, _ := domain.NewFileReference("ref-1", "epub", nil)
	now := time.Now().UTC().Truncate(time.Microsecond)
	repo := &fakeImportCandidateRepo{
		candidates: map[string]postgres.ImportCandidateRecord{
			"cand-1": {
				ID:            "cand-1",
				SourceID:      "src-1",
				FileReference: ref,
				Status:        postgres.ImportCandidateStatusPending,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	}

	svc := importer.NewService(nil, nil, nil, nil, nil, repo, nil, nil, nil)
	h := transporthttp.ImportCandidateRejectHandler(svc, repo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/candidates/cand-1/reject", nil)
	req.SetPathValue("id", "cand-1")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	c := repo.candidates["cand-1"]
	if c.Status != postgres.ImportCandidateStatusRejected {
		t.Fatalf("status = %q, want rejected", c.Status)
	}
}
