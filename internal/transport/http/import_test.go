package http_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	// candidateLibrary optionally records which library each candidate
	// belongs to; GetInLibrary/ListInLibrary honour it when set.
	candidateLibrary map[string]domain.LibraryID
	gotLibraryID     domain.LibraryID
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

func (r *fakeImportCandidateRepo) GetInLibrary(ctx context.Context, libraryID domain.LibraryID, id string) (postgres.ImportCandidateRecord, error) {
	r.gotLibraryID = libraryID
	if r.candidateLibrary != nil {
		if lib, ok := r.candidateLibrary[id]; ok && lib != libraryID {
			return postgres.ImportCandidateRecord{}, &domain.Error{Category: domain.NotFound, Message: "import candidate not found"}
		}
	}
	return r.Get(ctx, id)
}

func (r *fakeImportCandidateRepo) ListInLibrary(ctx context.Context, libraryID domain.LibraryID, sourceID *string, status *string) ([]postgres.ImportCandidateRecord, error) {
	r.gotLibraryID = libraryID
	all, err := r.List(ctx, sourceID, status)
	if err != nil || r.candidateLibrary == nil {
		return all, err
	}
	var out []postgres.ImportCandidateRecord
	for _, c := range all {
		if r.candidateLibrary[c.ID] == libraryID {
			out = append(out, c)
		}
	}
	return out, nil
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

// #104: the candidate list, confirm, and reject handlers must scope to
// the request's active library. A candidate whose source is in another
// library must be a 404 on confirm/reject and absent from the list.
func TestImportHandlers_ScopeToActiveLibrary(t *testing.T) {
	const libA = domain.LibraryID("lib-a")
	const libB = domain.LibraryID("lib-b")
	ref, _ := domain.NewFileReference("ref-1", "epub", nil)
	now := time.Now().UTC().Truncate(time.Microsecond)

	newRepo := func() *fakeImportCandidateRepo {
		return &fakeImportCandidateRepo{
			candidates: map[string]postgres.ImportCandidateRecord{
				"cand-a": {ID: "cand-a", SourceID: "src-a", FileReference: ref, Status: postgres.ImportCandidateStatusPending, CreatedAt: now, UpdatedAt: now},
			},
			candidateLibrary: map[string]domain.LibraryID{"cand-a": libA},
		}
	}
	withLib := func(r *http.Request, lib domain.LibraryID) *http.Request {
		return r.WithContext(transporthttp.WithActiveLibrary(r.Context(), lib))
	}

	t.Run("list hides another library's candidate", func(t *testing.T) {
		repo := newRepo()
		h := transporthttp.ImportCandidatesListHandler(repo)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, withLib(httptest.NewRequest(http.MethodGet, "/api/v1/import/candidates", nil), libB))
		var res struct {
			Candidates []transporthttp.ImportCandidateWireDTO `json:"candidates"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if len(res.Candidates) != 0 {
			t.Fatalf("library B saw %d candidates, want 0", len(res.Candidates))
		}
	})

	t.Run("reject from another library is 404", func(t *testing.T) {
		repo := newRepo()
		svc := importer.NewService(nil, nil, nil, nil, nil, repo, nil, nil, nil)
		h := transporthttp.ImportCandidateRejectHandler(svc, repo)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/candidates/cand-a/reject", nil)
		req.SetPathValue("id", "cand-a")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, withLib(req, libB))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body %s", w.Code, w.Body.String())
		}
		if repo.candidates["cand-a"].Status != postgres.ImportCandidateStatusPending {
			t.Fatalf("candidate status changed to %q despite cross-library reject", repo.candidates["cand-a"].Status)
		}
	})

	t.Run("confirm from another library is 404", func(t *testing.T) {
		repo := newRepo()
		svc := importer.NewService(nil, nil, nil, nil, nil, repo, nil, nil, nil)
		h := transporthttp.ImportCandidateConfirmHandler(svc, repo, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/import/candidates/cand-a/confirm", bytes.NewReader([]byte(`{"action":"create_new"}`)))
		req.SetPathValue("id", "cand-a")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, withLib(req, libB))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body %s", w.Code, w.Body.String())
		}
	})
}

func TestImportCandidateCoverHandler(t *testing.T) {
	libA := domain.LibraryID("01JLIB0000000000000000000A")
	libB := domain.LibraryID("01JLIB0000000000000000000B")
	ref, _ := domain.NewFileReference("ref-1.epub", "epub", nil)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	// Minimal 1x1 JPEG bytes: FF D8 FF E0 00 10 4A 46 49 46 ...
	jpegBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xdb}
	svgBytes := []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>")

	repo := &fakeImportCandidateRepo{
		candidates: map[string]postgres.ImportCandidateRecord{
			"cand-jpeg": {
				ID:                "cand-jpeg",
				SourceID:          "src-1",
				FileReference:     ref,
				Status:            postgres.ImportCandidateStatusPending,
				ExtractedMetadata: []byte(fmt.Sprintf(`{"title":"Dune","coverBytes":%q}`, base64.StdEncoding.EncodeToString(jpegBytes))),
				CreatedAt:         now,
				UpdatedAt:         now,
			},
			"cand-svg": {
				ID:                "cand-svg",
				SourceID:          "src-1",
				FileReference:     ref,
				Status:            postgres.ImportCandidateStatusPending,
				ExtractedMetadata: []byte(fmt.Sprintf(`{"title":"Dune","coverBytes":"data:image/svg+xml;base64,%s"}`, base64.StdEncoding.EncodeToString(svgBytes))),
				CreatedAt:         now,
				UpdatedAt:         now,
			},
			"cand-nocover": {
				ID:                "cand-nocover",
				SourceID:          "src-1",
				FileReference:     ref,
				Status:            postgres.ImportCandidateStatusPending,
				ExtractedMetadata: []byte(`{"title":"Dune"}`),
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		},
		candidateLibrary: map[string]domain.LibraryID{
			"cand-jpeg":    libA,
			"cand-svg":     libA,
			"cand-nocover": libA,
		},
	}

	h := transporthttp.ImportCandidateCoverHandler(repo)
	withLib := func(r *http.Request, lib domain.LibraryID) *http.Request {
		return r.WithContext(transporthttp.WithActiveLibrary(r.Context(), lib))
	}

	t.Run("valid jpeg cover returns 200 binary", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/import/candidates/cand-jpeg/cover", nil)
		req.SetPathValue("id", "cand-jpeg")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, withLib(req, libA))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("Content-Type = %q, want image/jpeg", ct)
		}
		if cc := w.Header().Get("Cache-Control"); cc != "private, max-age=86400" {
			t.Errorf("Cache-Control = %q, want private, max-age=86400", cc)
		}
		if !bytes.Equal(w.Body.Bytes(), jpegBytes) {
			t.Errorf("body mismatch: got %v, want %v", w.Body.Bytes(), jpegBytes)
		}
	})

	t.Run("svg cover returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/import/candidates/cand-svg/cover", nil)
		req.SetPathValue("id", "cand-svg")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, withLib(req, libA))

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})

	t.Run("candidate without cover returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/import/candidates/cand-nocover/cover", nil)
		req.SetPathValue("id", "cand-nocover")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, withLib(req, libA))

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})

	t.Run("cross-library candidate returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/import/candidates/cand-jpeg/cover", nil)
		req.SetPathValue("id", "cand-jpeg")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, withLib(req, libB))

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}
