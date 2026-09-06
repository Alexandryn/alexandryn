package contracttest_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil/contracttest"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// fakeT is a minimal testing.TB that records whether Error/Errorf/Fatal
// was called, letting the outer test assert that a validator failure was
// triggered without failing the outer test itself.
type fakeT struct {
	testing.TB
	failed  bool
	t       *testing.T
	cleanup []func()
}

func (f *fakeT) Helper()                         {}
func (f *fakeT) Log(args ...any)                 {}
func (f *fakeT) Logf(format string, args ...any) {}
func (f *fakeT) Error(args ...any)               { f.failed = true }
func (f *fakeT) Errorf(format string, _ ...any)  { f.failed = true }
func (f *fakeT) Fatal(args ...any)               { f.failed = true; panic("fakeT.Fatal") }
func (f *fakeT) Fatalf(format string, _ ...any)  { f.failed = true; panic("fakeT.Fatalf") }
func (f *fakeT) Cleanup(fn func())               { f.cleanup = append(f.cleanup, fn) }

// mustRequest builds an *http.Request, failing the test on error.
func mustRequest(t *testing.T, method, path string, body io.Reader) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestSpecLoadsAndIsValid proves the spec file is reachable and passes
// OpenAPI 3.x validation. This is the minimal "does the contract test
// infrastructure itself work" check (L02).
func TestSpecLoadsAndIsValid(t *testing.T) {
	v := contracttest.New(t)
	doc := v.Doc()
	if doc == nil {
		t.Fatal("expected non-nil doc")
	}

	// Confirm all Phase 06 and Phase 07 paths are present in the spec (L01 + L02
	// cross-check). A missing path means openapi.yaml was not updated.
	phasePaths := []string{
		"/api/v1/library",
		"/api/v1/works/{id}",
		"/api/v1/collections",
		"/api/v1/collections/{id}",
		"/api/v1/collections/{id}/works",
		"/api/v1/collections/{id}/works/{workId}",
		"/api/v1/discover",
		"/api/v1/discover/works/{openLibraryId}",
		"/api/v1/discover/covers/{coverId}",
		"/api/v1/sources",
		"/api/v1/sources/{id}",
		"/api/v1/sources/{id}/health-check",
		"/api/v1/sources/{id}/browse",
		"/api/v1/sources/{id}/search",
		"/api/v1/import/discover",
		"/api/v1/import/candidates",
		"/api/v1/import/candidates/{id}/confirm",
		"/api/v1/import/candidates/{id}/reject",
		// Phase 11 — reader content + reading API + export.
		"/api/v1/library/editions/{editionId}/reader/content/{path}",
		"/api/v1/reading/works/{workId}/progress",
		"/api/v1/reading/editions/{editionId}/bookmarks",
		"/api/v1/reading/bookmarks/{bookmarkId}",
		"/api/v1/reading/editions/{editionId}/highlights",
		"/api/v1/reading/highlights/{highlightId}",
		"/api/v1/reading/preferences",
		"/api/v1/reading/export",
		// Phase 13 — network access & device pairing.
		"/api/v1/network/pair/initiate",
		"/api/v1/network/pair/verify",
		"/api/v1/network/pair/{id}/qr",
		"/api/v1/network/status",
		"/api/v1/network/settings",
		"/api/v1/network/pair/{id}",
		// Phase 14 — devices & sync.
		"/api/v1/devices",
		"/api/v1/devices/{id}",
		"/api/v1/sync/reading",
		"/api/v1/sync/progress",
	}
	for _, p := range phasePaths {
		if doc.Paths.Find(p) == nil {
			t.Errorf("spec missing path: %s", p)
		}
	}
}

// TestBrokenHandlerFailsContractTest proves — in the negative direction —
// that ValidateResponse catches a handler returning a schema-violating
// body (backend-library-api.md FR-8: "the contract test fails on a
// deliberately malformed handler response").
//
// Uses fakeT to intercept the validation failure so it doesn't propagate
// as an outer-test failure — the outer test asserts the inner failure
// occurred.
func TestBrokenHandlerFailsContractTest(t *testing.T) {
	v := contracttest.New(t)

	// Handler returns 200 with an empty JSON object, which is NOT a valid
	// LibraryPage (missing required 'works' and 'nextCursor' fields).
	badHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	inner := &fakeT{t: t}
	req := mustRequest(t, "GET", "/api/v1/library", nil)

	// ValidateResponse will call inner.Errorf when the schema mismatch is
	// detected. We recover from any panic fakeT.Fatal/Fatalf might cause.
	func() {
		defer func() { recover() }() //nolint:errcheck
		v.ValidateResponse(inner, badHandler, req)
	}()

	if !inner.failed {
		t.Error("expected contract validation to detect schema mismatch for {} body, but it did not")
	}
}

// TestUndocumentedRouteNotInSpec proves that a path not present in
// api/openapi.yaml is detectable via Doc().Paths.Find (the
// route-completeness direction of FR-8's dual check). This does not hit
// an actual handler — it only inspects the spec's path set.
func TestUndocumentedRouteNotInSpec(t *testing.T) {
	v := contracttest.New(t)
	doc := v.Doc()

	// /api/v1/nonexistent is not a real endpoint and should not appear.
	if doc.Paths.Find("/api/v1/nonexistent") != nil {
		t.Error("spec unexpectedly contains /api/v1/nonexistent — spec may have stale content")
	}
}

// TestValidLibraryPagePassesContractTest proves the happy path: a
// correctly shaped LibraryPage response passes validation without error.
func TestValidLibraryPagePassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	validBody := `{"works":[],"nextCursor":null}`
	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	})

	req := mustRequest(t, "GET", "/api/v1/library", nil)
	// If the validator calls t.Error, this outer test fails — which is
	// exactly what we want: a valid response must not trigger an error.
	// The body is consumed by the validator, so we only check status.
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestValidDiscoverSearchPassesContractTest proves that a correctly shaped
// NormalisedSearchResponse passes contract validation.
func TestValidDiscoverSearchPassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	validBody := `{
		"items": [
			{
				"openLibraryWorkKey": "OL82563W",
				"title": "Middlemarch",
				"authors": [
					{
						"openLibraryAuthorKey": "OL21594A",
						"name": "George Eliot"
					}
				],
				"firstPublishYear": 1871,
				"coverUrl": "/api/v1/discover/covers/8256301",
				"editionCount": 42
			}
		],
		"total": 1,
		"limit": 20,
		"offset": 0
	}`
	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	})

	req := mustRequest(t, "GET", "/api/v1/discover?q=middlemarch", nil)
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestValidDiscoverWorkDetailPassesContractTest proves that a correctly shaped
// DiscoverWorkDetail passes contract validation.
func TestValidDiscoverWorkDetailPassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	validBody := `{
		"work": {
			"title": "Middlemarch",
			"subtitle": "A Study of Provincial Life",
			"description": "A novel by George Eliot.",
			"subjects": ["Fiction"],
			"authors": [
				{
					"openLibraryAuthorKey": "OL21594A",
					"name": "George Eliot"
				}
			],
			"coverUrl": "/api/v1/discover/covers/8256301"
		},
		"editions": [
			{
				"title": "Middlemarch",
				"publisher": "Penguin Classics",
				"publishDate": "2003",
				"language": "en",
				"openLibraryEditionKey": "OL7353617M",
				"coverUrl": "/api/v1/discover/covers/8256301"
			}
		]
	}`
	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	})

	req := mustRequest(t, "GET", "/api/v1/discover/works/OL82563W", nil)
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestValidDiscoverCoverPassesContractTest proves that a correctly shaped
// cover binary image response with caching headers passes contract validation.
func TestValidDiscoverCoverPassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Metadata-Cache", "hit")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00"))
	})

	req := httptest.NewRequest("GET", "/api/v1/discover/covers/8256301", nil)
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// ── Phase 08: Sources ────────────────────────────────────────────────
// backend-source-adapter.md FR-2/FR-6/FR-7. These validate the spec's
// own shapes against hand-built valid bodies — the real handlers land in
// Tier 3 and get their own contract coverage then.

func serveJSON(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

func TestValidSourceListPassesContractTest(t *testing.T) {
	v := contracttest.New(t)
	body := `{"sources":[{
		"id":"01JXXXXXXXXXXXXXXXXXXXXXXZ","label":"Personal OPDS","kind":"opds",
		"config":{"baseUrl":"https://opds.example.org/catalog"},
		"hasCredential":true,
		"health":{"status":"reachable","checkedAt":"2026-08-31T12:00:00Z","detail":null},
		"capabilities":{"canList":true,"canSearch":true,"canDownload":true}
	}]}`
	rr := v.ValidateResponse(t, serveJSON(http.StatusOK, body), mustRequest(t, "GET", "/api/v1/sources", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestValidSourceHealthPassesContractTest(t *testing.T) {
	v := contracttest.New(t)
	body := `{"status":"unreachable","checkedAt":"2026-08-31T12:10:00Z","detail":"timeout"}`
	req := mustRequest(t, "POST", "/api/v1/sources/01JXXXXXXXXXXXXXXXXXXXXXXZ/health-check", nil)
	rr := v.ValidateResponse(t, serveJSON(http.StatusOK, body), req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestValidSourceBrowsePassesContractTest(t *testing.T) {
	v := contracttest.New(t)
	body := `{"items":[{
		"title":"The Left Hand of Darkness","author":"Ursula K. Le Guin",
		"fileReference":{"referenceId":"left-hand.epub","format":"EPUB","sizeBytes":512000},
		"coverUrl":null
	}],"nextCursor":null}`
	req := mustRequest(t, "GET", "/api/v1/sources/01JXXXXXXXXXXXXXXXXXXXXXXZ/browse", nil)
	rr := v.ValidateResponse(t, serveJSON(http.StatusOK, body), req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestValidSourceSearchConflictPassesContractTest(t *testing.T) {
	v := contracttest.New(t)
	body := `{"code":"conflict","message":"this source does not support search","correlationId":"00000000-0000-0000-0000-000000000000"}`
	req := mustRequest(t, "GET", "/api/v1/sources/01JXXXXXXXXXXXXXXXXXXXXXXZ/search?q=earthsea", nil)
	rr := v.ValidateResponse(t, serveJSON(http.StatusConflict, body), req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestBrokenSourceResponseFailsContractTest(t *testing.T) {
	v := contracttest.New(t)
	// Missing required `capabilities` — must be rejected.
	body := `{"sources":[{"id":"x","label":"L","kind":"opds","config":{},"hasCredential":false,
		"health":{"status":"unknown","checkedAt":null,"detail":null}}]}`
	inner := &fakeT{t: t}
	func() {
		defer func() { recover() }() //nolint:errcheck
		v.ValidateResponse(inner, serveJSON(http.StatusOK, body), mustRequest(t, "GET", "/api/v1/sources", nil))
	}()
	if !inner.failed {
		t.Error("expected contract validation to reject a Source missing `capabilities`")
	}
}

type contractMockSourceRecordRepo struct {
	mu      sync.Mutex
	records map[string]postgres.SourceRecord
}

func (m *contractMockSourceRecordRepo) Create(ctx context.Context, rec postgres.SourceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[rec.ID] = rec
	return nil
}

func (m *contractMockSourceRecordRepo) Get(ctx context.Context, id string) (postgres.SourceRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return postgres.SourceRecord{}, &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	return rec, nil
}

func (m *contractMockSourceRecordRepo) List(ctx context.Context) ([]postgres.SourceRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]postgres.SourceRecord, 0, len(m.records))
	for _, r := range m.records {
		out = append(out, r)
	}
	return out, nil
}

func (m *contractMockSourceRecordRepo) UpdateConfig(ctx context.Context, id, label, basePath, baseURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	rec.Label = label
	rec.ConfigBasePath = basePath
	rec.ConfigBaseURL = baseURL
	m.records[id] = rec
	return nil
}

func (m *contractMockSourceRecordRepo) SetCredential(ctx context.Context, id string, ct, nonce []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	rec.CredentialCiphertext = ct
	rec.CredentialNonce = nonce
	m.records[id] = rec
	return nil
}

func (m *contractMockSourceRecordRepo) UpdateHealth(ctx context.Context, id, status, detail string, checkedAt time.Time, caps domain.SourceCapabilities, searchLinkURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	rec.HealthStatus = status
	rec.HealthDetail = detail
	rec.HealthCheckedAt = &checkedAt
	rec.Capabilities = caps
	rec.SearchLinkURL = searchLinkURL
	m.records[id] = rec
	return nil
}

func TestRealSourcesHandlersPassContractTest(t *testing.T) {
	v := contracttest.New(t)

	repo := &contractMockSourceRecordRepo{records: make(map[string]postgres.SourceRecord)}
	poolRef := &transporthttp.PoolRef{}
	key := bytes.Repeat([]byte{0x42}, 32)
	svc, _ := crypto.NewService(key)
	subkey, _ := svc.DeriveSubkey("source-cursor-hmac-v1")
	poolRef.SetSourceCrypto(transporthttp.SourceCrypto{
		Encryptor: svc,
		Codec:     sources.NewCursorCodec(subkey),
	})

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:              "01JXXXXXXXXXXXXXXXXXXXXXXZ",
		Label:           "Personal OPDS",
		Kind:            "opds",
		ConfigBaseURL:   "https://opds.example.org/catalog",
		HealthStatus:    "reachable",
		HealthCheckedAt: &now,
		Capabilities:    domain.SourceCapabilities{CanList: true, CanSearch: true, CanDownload: true},
		CreatedAt:       now,
	})

	t.Run("GET /api/v1/sources", func(t *testing.T) {
		h := transporthttp.ListSourcesHandler(repo)
		req := mustRequest(t, "GET", "/api/v1/sources", nil)
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/sources/{id}", func(t *testing.T) {
		h := transporthttp.GetSourceHandler(repo)
		req := mustRequest(t, "GET", "/api/v1/sources/01JXXXXXXXXXXXXXXXXXXXXXXZ", nil)
		req.SetPathValue("id", "01JXXXXXXXXXXXXXXXXXXXXXXZ")
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

type contractMockImportCandidateRepo struct {
	mu         sync.Mutex
	candidates map[string]postgres.ImportCandidateRecord
}

func (m *contractMockImportCandidateRepo) Create(ctx context.Context, rec postgres.ImportCandidateRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.candidates[rec.ID] = rec
	return nil
}

func (m *contractMockImportCandidateRepo) Get(ctx context.Context, id string) (postgres.ImportCandidateRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.candidates[id]
	if !ok {
		return postgres.ImportCandidateRecord{}, &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	return c, nil
}

func (m *contractMockImportCandidateRepo) List(ctx context.Context, sourceID *string, status *string) ([]postgres.ImportCandidateRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []postgres.ImportCandidateRecord
	for _, c := range m.candidates {
		list = append(list, c)
	}
	return list, nil
}

func (m *contractMockImportCandidateRepo) UpdateStatus(ctx context.Context, id string, status string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	c.Status = status
	c.UpdatedAt = now
	m.candidates[id] = c
	return nil
}

func (m *contractMockImportCandidateRepo) UpdateExtractedAndMatches(ctx context.Context, id string, status string, extracted []byte, matches []byte, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	c.Status = status
	c.ExtractedMetadata = extracted
	c.MatchCandidates = matches
	c.UpdatedAt = now
	m.candidates[id] = c
	return nil
}

func (m *contractMockImportCandidateRepo) UpdateFailed(ctx context.Context, id string, lastError string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "candidate not found"}
	}
	c.Status = postgres.ImportCandidateStatusFailed
	c.LastError = &lastError
	c.UpdatedAt = now
	m.candidates[id] = c
	return nil
}

func (m *contractMockImportCandidateRepo) ExistsBySourceAndFileRefID(ctx context.Context, sourceID string, fileRefID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.candidates {
		if c.SourceID == sourceID && c.FileReference.ReferenceID == fileRefID {
			return true, nil
		}
	}
	return false, nil
}

type contractMockDiscoveryRunner struct{}

func (d *contractMockDiscoveryRunner) Discover(ctx context.Context, sourceID string, now time.Time) (importer.DiscoverResult, error) {
	return importer.DiscoverResult{
		DiscoveredCount: 2,
		SkippedCount:    0,
		JobIDs:          []string{"01JJOB1", "01JJOB2"},
	}, nil
}

func TestRealImportHandlersPassContractTest(t *testing.T) {
	v := contracttest.New(t)

	ref, _ := domain.NewFileReference("ref-1.epub", "epub", nil)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	candRepo := &contractMockImportCandidateRepo{
		candidates: map[string]postgres.ImportCandidateRecord{
			"01JCANDIDATE1": {
				ID:                "01JCANDIDATE1",
				SourceID:          "01JSOURCE1",
				FileReference:     ref,
				Status:            "pending",
				ExtractedMetadata: []byte(`{"title":"Dune","authors":["Frank Herbert"]}`),
				MatchCandidates:   []byte(`[{"type":"open_library_work","confidence":"high","title":"Dune"}]`),
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		},
	}

	t.Run("POST /api/v1/import/discover", func(t *testing.T) {
		h := transporthttp.ImportDiscoverHandler(&contractMockDiscoveryRunner{})
		req := mustRequest(t, "POST", "/api/v1/import/discover", bytes.NewReader([]byte(`{"sourceId":"01JSOURCE1"}`)))
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/import/candidates", func(t *testing.T) {
		h := transporthttp.ImportCandidatesListHandler(candRepo)
		req := mustRequest(t, "GET", "/api/v1/import/candidates?sourceId=01JSOURCE1&status=pending", nil)
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("POST /api/v1/import/candidates/{id}/reject", func(t *testing.T) {
		svc := importer.NewService(nil, nil, nil, nil, nil, candRepo, nil, nil, nil)
		h := transporthttp.ImportCandidateRejectHandler(svc, candRepo)
		req := mustRequest(t, "POST", "/api/v1/import/candidates/01JCANDIDATE1/reject", nil)
		req.SetPathValue("id", "01JCANDIDATE1")
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

// --- Phase 11: reader content + reading API contract responses ---------

type ctReadingProgress struct{ p *domain.ReadingProgress }

func (r ctReadingProgress) FindByWork(context.Context, domain.WorkID) (*domain.ReadingProgress, error) {
	if r.p == nil {
		return nil, &domain.Error{Category: domain.NotFound, Message: "none"}
	}
	return r.p, nil
}
func (r ctReadingProgress) FindByWorkForUpdate(ctx context.Context, w domain.WorkID) (*domain.ReadingProgress, error) {
	return r.FindByWork(ctx, w)
}
func (r ctReadingProgress) FindByWorkAndUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, w domain.WorkID) (*domain.ReadingProgress, error) {
	return r.FindByWork(ctx, w)
}
func (r ctReadingProgress) FindByWorkAndUserForUpdate(ctx context.Context, _ domain.UserID, _ domain.LibraryID, w domain.WorkID) (*domain.ReadingProgress, error) {
	return r.FindByWork(ctx, w)
}
func (r ctReadingProgress) Save(context.Context, *domain.ReadingProgress) error { return nil }
func (r ctReadingProgress) SaveForUser(context.Context, domain.UserID, domain.LibraryID, *domain.ReadingProgress) error {
	return nil
}

type ctExport struct{}

func (ctExport) WorkExists(context.Context, string) (bool, error) { return true, nil }
func (ctExport) ListProgress(context.Context, domain.UserID, domain.LibraryID, string) ([]postgres.ExportProgress, error) {
	return nil, nil
}
func (ctExport) ListMarks(context.Context, domain.UserID, domain.LibraryID, string) ([]postgres.ExportMark, error) {
	return nil, nil
}

func TestPhase11ContractResponses(t *testing.T) {
	v := contracttest.New(t)
	now := func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }

	poolRef := &transporthttp.PoolRef{}
	poolRef.SetReadingAPI(transporthttp.ReadingAPI{
		Progress: ctReadingProgress{},
		Export:   ctExport{},
	})

	// The reading handlers now require an authenticated user + active
	// library in context (AUDIT-0012-C1). The auth middleware supplies it
	// in production; here it is injected directly.
	withReadingScope := func(req *http.Request) *http.Request {
		ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{UserID: "ct-user"})
		ctx = transporthttp.WithActiveLibrary(ctx, domain.DefaultLibraryID)
		return req.WithContext(ctx)
	}

	t.Run("GET /api/v1/reading/works/{workId}/progress → null", func(t *testing.T) {
		h := transporthttp.ReadingProgressGetHandler(poolRef)
		req := withReadingScope(mustRequest(t, "GET", "/api/v1/reading/works/work-1/progress", nil))
		req.SetPathValue("workId", "work-1")
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/reading/export → versioned document", func(t *testing.T) {
		h := transporthttp.ReadingExportHandler(poolRef, nil, now)
		req := withReadingScope(mustRequest(t, "GET", "/api/v1/reading/export", nil))
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

type ctUsers struct{}

func (ctUsers) FindByID(context.Context, domain.UserID) (*domain.User, error) { return nil, nil }
func (ctUsers) FindByEmail(context.Context, string) (*domain.User, error)     { return nil, nil }
func (ctUsers) FindByUsername(context.Context, string) (*domain.User, error)  { return nil, nil }
func (ctUsers) CountUsers(context.Context) (int, error)                       { return 0, nil }
func (ctUsers) Save(context.Context, *domain.User) error                      { return nil }

type ctLibraries struct{}

func (ctLibraries) FindByID(context.Context, domain.LibraryID) (*domain.Library, error) {
	return nil, nil
}
func (ctLibraries) FindAll(context.Context) ([]*domain.Library, error) {
	lib, _ := domain.NewLibrary(domain.DefaultLibraryID, "Default Library", "Main", false, time.Now(), time.Now())
	return []*domain.Library{lib}, nil
}
func (ctLibraries) FindByUser(context.Context, domain.UserID) ([]*domain.Library, error) {
	lib, _ := domain.NewLibrary(domain.DefaultLibraryID, "Default Library", "Main", false, time.Now(), time.Now())
	return []*domain.Library{lib}, nil
}
func (ctLibraries) Save(context.Context, *domain.Library) error    { return nil }
func (ctLibraries) Delete(context.Context, domain.LibraryID) error { return nil }

type ctMemberships struct{}

func (ctMemberships) FindMembership(context.Context, domain.LibraryID, domain.UserID) (*domain.LibraryMembership, error) {
	return nil, nil
}
func (ctMemberships) FindByLibrary(context.Context, domain.LibraryID) ([]*domain.LibraryMembership, error) {
	return nil, nil
}
func (ctMemberships) FindByUser(context.Context, domain.UserID) ([]*domain.LibraryMembership, error) {
	return nil, nil
}
func (ctMemberships) Save(context.Context, *domain.LibraryMembership) error         { return nil }
func (ctMemberships) Delete(context.Context, domain.LibraryID, domain.UserID) error { return nil }

func TestPhase12ContractResponses(t *testing.T) {
	v := contracttest.New(t)

	t.Run("GET /api/v1/auth/setup/status → { isSetup: false }", func(t *testing.T) {
		h := transporthttp.SetupStatusHandler(ctUsers{})
		req := mustRequest(t, "GET", "/api/v1/auth/setup/status", nil)
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/libraries → list of libraries", func(t *testing.T) {
		h := transporthttp.ListLibrariesHandler(ctLibraries{}, ctMemberships{})
		req := mustRequest(t, "GET", "/api/v1/libraries", nil)
		user := &transporthttp.AuthenticatedUser{
			UserID: "u-1",
			Role:   domain.RoleAdmin,
		}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

type ctIDGen struct{}

func (ctIDGen) NewID() string { return "ct-id-12345" }

type ctPairingSessions struct {
	mu       sync.Mutex
	sessions map[domain.PairingSessionID]*domain.PairingSession
}

func (c *ctPairingSessions) FindByID(_ context.Context, id domain.PairingSessionID) (*domain.PairingSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.sessions[id]; ok {
		return s, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "pairing session not found"}
}

func (c *ctPairingSessions) FindByCodeIndex(_ context.Context, _ []byte) (*domain.PairingSession, error) {
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}

func (c *ctPairingSessions) FindPendingByCodeIndexForUpdate(_ context.Context, _ []byte, _ time.Time) (*domain.PairingSession, error) {
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}

func (c *ctPairingSessions) Save(_ context.Context, s *domain.PairingSession) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[s.ID()] = s
	return nil
}

func (c *ctPairingSessions) SaveWithInitiatorIP(_ context.Context, s *domain.PairingSession, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[s.ID()] = s
	return nil
}

func (c *ctPairingSessions) Delete(_ context.Context, _ domain.PairingSessionID) error {
	return nil
}

type ctVerifier struct {
	session *domain.PairingSession
}

func (v ctVerifier) VerifyAndConsume(_ context.Context, _ domain.PairingCode, _ domain.DeviceID, _ string, _ domain.DeviceClass, _ time.Time) (*domain.PairingSession, error) {
	return v.session, nil
}

type ctPairedDevices struct {
	devices map[domain.DeviceID]*domain.PairedDevice
}

func (d ctPairedDevices) FindByID(_ context.Context, id domain.DeviceID) (*domain.PairedDevice, error) {
	if d.devices != nil {
		if dev, ok := d.devices[id]; ok {
			return dev, nil
		}
	}
	return nil, nil
}
func (d ctPairedDevices) FindByOwner(_ context.Context, owner domain.UserID) ([]*domain.PairedDevice, error) {
	if d.devices != nil {
		var list []*domain.PairedDevice
		for _, dev := range d.devices {
			if dev.Owner() == owner {
				list = append(list, dev)
			}
		}
		return list, nil
	}
	return nil, nil
}
func (ctPairedDevices) FindByPairingSessionID(_ context.Context, _ domain.PairingSessionID) (*domain.PairedDevice, error) {
	return nil, nil
}
func (ctPairedDevices) InsertProvisional(_ context.Context, _ domain.DeviceID, _ string, _ domain.DeviceClass, _ domain.EnrolledVia, _ domain.PairingSessionID, _ time.Time) error {
	return nil
}
func (ctPairedDevices) AssignOwnerByPairingSession(_ context.Context, _ domain.PairingSessionID, _ domain.UserID) error {
	return nil
}
func (ctPairedDevices) Save(_ context.Context, _ *domain.PairedDevice) error {
	return nil
}
func (ctPairedDevices) Revoke(_ context.Context, _ domain.DeviceID, _ time.Time) error {
	return nil
}
func (ctPairedDevices) RevokeByPairingSessionID(_ context.Context, _ domain.PairingSessionID, _ time.Time) error {
	return nil
}
func (ctPairedDevices) AdvanceCursor(_ context.Context, _ domain.DeviceID, _ int64, _ time.Time) error {
	return nil
}

type ctNetworkSettings struct {
	settings *domain.NetworkSettings
}

func (s *ctNetworkSettings) Get(_ context.Context) (*domain.NetworkSettings, error) {
	if s.settings != nil {
		return s.settings, nil
	}
	return &domain.NetworkSettings{
		HostName:           "alexandryn.local",
		RememberDeviceDays: 30,
		UpdatedAt:          time.Now(),
	}, nil
}

func (s *ctNetworkSettings) Upsert(_ context.Context, settings *domain.NetworkSettings) error {
	s.settings = settings
	return nil
}

func TestPhase13ContractResponses(t *testing.T) {
	v := contracttest.New(t)
	now := func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	code, err := domain.NewPairingCode("3ABCDEFG")
	if err != nil {
		t.Fatalf("unexpected error creating pairing code: %v", err)
	}
	session, err := domain.NewPairingSession("sess-1", "admin-1", code, 5*time.Minute, now())
	if err != nil {
		t.Fatalf("unexpected error creating pairing session: %v", err)
	}

	sessionRepo := &ctPairingSessions{
		sessions: map[domain.PairingSessionID]*domain.PairingSession{
			"sess-1": session,
		},
	}
	deviceRepo := ctPairedDevices{}
	settingsRepo := &ctNetworkSettings{}
	signer := auth.NewEnrolmentGrantSigner([]byte("01234567890123456789012345678901"), "alexandryn-test", ctIDGen{})
	verifier := ctVerifier{session: session}

	t.Run("POST /api/v1/network/pair/initiate → 201 InitiatePairingResponse", func(t *testing.T) {
		h := transporthttp.InitiatePairingHandler(sessionRepo, func() (domain.PairingCode, error) {
			return code, nil
		}, "", "alexandryn.local:4000", "http", ctIDGen{}, now, nil)

		req := mustRequest(t, "POST", "/api/v1/network/pair/initiate", bytes.NewReader([]byte("{}")))
		user := &transporthttp.AuthenticatedUser{UserID: "admin-1", Role: domain.RoleAdmin}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rr.Code)
		}
	})

	t.Run("POST /api/v1/network/pair/verify → 200 VerifyPairingResponse", func(t *testing.T) {
		h := transporthttp.VerifyPairingHandler(verifier, signer, "alexandryn.local:4000", "alexandryn.local", ctIDGen{}, now, nil)

		req := mustRequest(t, "POST", "/api/v1/network/pair/verify", bytes.NewReader([]byte(`{"code":"3ABCDEFG"}`)))
		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/network/pair/{id}/qr → 200 PairingQRResponse", func(t *testing.T) {
		h := transporthttp.PairingQRHandler(sessionRepo, "alexandryn.local:4000", "http", nil)

		req := mustRequest(t, "GET", "/api/v1/network/pair/sess-1/qr", nil)
		req.SetPathValue("id", "sess-1")
		user := &transporthttp.AuthenticatedUser{UserID: "admin-1", Role: domain.RoleAdmin}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("GET /api/v1/network/status → 200 NetworkStatusReader", func(t *testing.T) {
		infoProvider := func() transporthttp.NetworkInfo {
			return transporthttp.NetworkInfo{
				Reachability: "local_network",
				TLSMode:      "none",
				BindAddress:  "0.0.0.0:4000",
				HostName:     "alexandryn.local",
				Addresses: []transporthttp.NetworkAddressWire{
					{Scope: "local", URL: "http://192.168.1.50:4000"},
				},
			}
		}
		h := transporthttp.NetworkStatusHandler(infoProvider, nil)

		req := mustRequest(t, "GET", "/api/v1/network/status", nil)
		user := &transporthttp.AuthenticatedUser{UserID: "reader-1", Role: domain.RoleReader}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/network/status → 200 NetworkStatusAdmin", func(t *testing.T) {
		infoProvider := func() transporthttp.NetworkInfo {
			return transporthttp.NetworkInfo{
				Reachability: "local_network",
				TLSMode:      "none",
				BindAddress:  "0.0.0.0:4000",
				HostName:     "alexandryn.local",
				Addresses: []transporthttp.NetworkAddressWire{
					{Scope: "local", URL: "http://192.168.1.50:4000"},
				},
			}
		}
		h := transporthttp.NetworkStatusHandler(infoProvider, nil)

		req := mustRequest(t, "GET", "/api/v1/network/status", nil)
		user := &transporthttp.AuthenticatedUser{UserID: "admin-1", Role: domain.RoleAdmin}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("PATCH /api/v1/network/settings → 200 NetworkSettingsResponse", func(t *testing.T) {
		h := transporthttp.UpdateNetworkSettingsHandler(settingsRepo, now, nil)

		req := mustRequest(t, "PATCH", "/api/v1/network/settings", bytes.NewReader([]byte(`{"hostName":"alexandryn.local","rememberDeviceDays":14}`)))
		user := &transporthttp.AuthenticatedUser{UserID: "admin-1", Role: domain.RoleAdmin}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("DELETE /api/v1/network/pair/{id} → 204 No Content", func(t *testing.T) {
		h := transporthttp.DeletePairingHandler(sessionRepo, deviceRepo, now, nil)

		req := mustRequest(t, "DELETE", "/api/v1/network/pair/sess-1", nil)
		req.SetPathValue("id", "sess-1")
		user := &transporthttp.AuthenticatedUser{UserID: "admin-1", Role: domain.RoleAdmin}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rr.Code)
		}
	})
}

type ctSyncStore struct {
	data *postgres.ReadingSyncData
}

func (s *ctSyncStore) GetReadingSyncData(_ context.Context, _ domain.UserID, _ domain.LibraryID, _ int64) (*postgres.ReadingSyncData, error) {
	if s.data != nil {
		return s.data, nil
	}
	return &postgres.ReadingSyncData{
		Cursor:     10,
		Progress:   []postgres.SyncProgressItem{},
		Bookmarks:  []postgres.SyncBookmarkItem{},
		Highlights: []postgres.SyncHighlightItem{},
	}, nil
}

func (s *ctSyncStore) GetProgressSyncSequence(_ context.Context, _ domain.ReadingProgressID) (int64, error) {
	return 10, nil
}

type ctLibEntriesChecker struct{}

func (ctLibEntriesChecker) WorkInLibrary(_ context.Context, _ domain.WorkID, _ domain.LibraryID) (bool, error) {
	return true, nil
}

type ctTransactor struct{}

func (ctTransactor) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type ctIDs struct{}

func (ctIDs) NewID() string {
	return "test-id"
}

func TestPhase14ContractResponses(t *testing.T) {
	v := contracttest.New(t)
	now := func() time.Time { return time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC) }

	dev, err := domain.RehydratePairedDevice(
		"dev-1", "user-1", "My Phone", domain.DeviceClassPhone, domain.EnrolledViaPairingCode,
		now(), now(), nil, 5, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error rehydrating device: %v", err)
	}

	devRepo := ctPairedDevices{
		devices: map[domain.DeviceID]*domain.PairedDevice{
			"dev-1": dev,
		},
	}

	t.Run("GET /api/v1/devices → 200 ListDevicesResponse", func(t *testing.T) {
		h := transporthttp.ListDevicesHandler(devRepo)
		req := mustRequest(t, "GET", "/api/v1/devices", nil)
		user := &transporthttp.AuthenticatedUser{UserID: "user-1", Role: domain.RoleReader}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("DELETE /api/v1/devices/{id} → 204 No Content", func(t *testing.T) {
		h := transporthttp.RevokeDeviceHandler(devRepo, now)
		req := mustRequest(t, "DELETE", "/api/v1/devices/dev-1", nil)
		req.SetPathValue("id", "dev-1")
		user := &transporthttp.AuthenticatedUser{UserID: "user-1", Role: domain.RoleReader}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/sync/reading → 200 ReadingSyncResponse", func(t *testing.T) {
		syncStore := &ctSyncStore{
			data: &postgres.ReadingSyncData{
				Cursor: 12,
				Progress: []postgres.SyncProgressItem{
					{
						WorkID:       "work-1",
						Percentage:   0.45,
						Epoch:        1,
						DeviceID:     "dev-1",
						ObservedAt:   now(),
						SyncSequence: 12,
					},
				},
				Bookmarks:  []postgres.SyncBookmarkItem{},
				Highlights: []postgres.SyncHighlightItem{},
			},
		}
		h := transporthttp.SyncReadingHandler(syncStore, devRepo, now)
		req := mustRequest(t, "GET", "/api/v1/sync/reading?since=0", nil)
		user := &transporthttp.AuthenticatedUser{UserID: "user-1", Role: domain.RoleReader}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))
		req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), "lib-1"))
		req = req.WithContext(transporthttp.WithDevice(req.Context(), dev))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("POST /api/v1/sync/progress → 200 SyncProgressResponse", func(t *testing.T) {
		pct, err := domain.NewPercentage(0.3)
		if err != nil {
			t.Fatalf("unexpected error creating percentage: %v", err)
		}
		progress := domain.RehydrateReadingProgress("prog-1", "work-1", pct, 0, nil, "dev-1", now())
		progRepo := ctReadingProgress{p: progress}
		syncStore := &ctSyncStore{}
		tx := ctTransactor{}
		ids := ctIDs{}

		h := transporthttp.SyncProgressHandler(progRepo, ctLibEntriesChecker{}, syncStore, devRepo, tx, ids, now)
		body := `{"workId":"work-1","percentage":0.5,"observedEpoch":0,"deviceId":"dev-1"}`
		req := mustRequest(t, "POST", "/api/v1/sync/progress", bytes.NewReader([]byte(body)))
		user := &transporthttp.AuthenticatedUser{UserID: "user-1", Role: domain.RoleReader}
		req = req.WithContext(transporthttp.WithUser(req.Context(), user))
		req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), "lib-1"))
		req = req.WithContext(transporthttp.WithDevice(req.Context(), dev))

		rr := v.ValidateResponse(t, h, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}


