package http_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// --- in-memory fakes ----------------------------------------------------

type memReadingProgress struct {
	byWork map[domain.WorkID]*domain.ReadingProgress
}

func (m *memReadingProgress) FindByWork(_ context.Context, w domain.WorkID) (*domain.ReadingProgress, error) {
	if p, ok := m.byWork[w]; ok {
		return p, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memReadingProgress) FindByWorkForUpdate(ctx context.Context, w domain.WorkID) (*domain.ReadingProgress, error) {
	return m.FindByWork(ctx, w)
}
func (m *memReadingProgress) Save(_ context.Context, p *domain.ReadingProgress) error {
	m.byWork[p.WorkID()] = p
	return nil
}

type memBookmarks struct {
	byID map[domain.BookmarkID]*domain.Bookmark
}

func (m *memBookmarks) FindByID(_ context.Context, id domain.BookmarkID) (*domain.Bookmark, error) {
	if b, ok := m.byID[id]; ok {
		return b, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memBookmarks) FindByEdition(_ context.Context, e domain.EditionID) ([]*domain.Bookmark, error) {
	var out []*domain.Bookmark
	for _, b := range m.byID {
		if b.EditionID() == e {
			out = append(out, b)
		}
	}
	return out, nil
}
func (m *memBookmarks) Save(_ context.Context, b *domain.Bookmark) error {
	m.byID[b.ID()] = b
	return nil
}
func (m *memBookmarks) Delete(_ context.Context, id domain.BookmarkID) error {
	delete(m.byID, id)
	return nil
}

type memHighlights struct {
	byID map[domain.HighlightID]*domain.Highlight
}

func (m *memHighlights) FindByID(_ context.Context, id domain.HighlightID) (*domain.Highlight, error) {
	if h, ok := m.byID[id]; ok {
		return h, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memHighlights) FindByEdition(_ context.Context, e domain.EditionID) ([]*domain.Highlight, error) {
	var out []*domain.Highlight
	for _, h := range m.byID {
		if h.EditionID() == e {
			out = append(out, h)
		}
	}
	return out, nil
}
func (m *memHighlights) Save(_ context.Context, h *domain.Highlight) error {
	m.byID[h.ID()] = h
	return nil
}
func (m *memHighlights) Delete(_ context.Context, id domain.HighlightID) error {
	delete(m.byID, id)
	return nil
}

type memPrefs struct {
	byDevice map[domain.DeviceID]*domain.ReadingPreferences
}

func (m *memPrefs) FindByDevice(_ context.Context, d domain.DeviceID) (*domain.ReadingPreferences, error) {
	if p, ok := m.byDevice[d]; ok {
		return p, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memPrefs) Save(_ context.Context, p *domain.ReadingPreferences) error {
	m.byDevice[p.DeviceID()] = p
	return nil
}

type memEditions struct {
	byID map[domain.EditionID]*domain.Edition
}

func (m *memEditions) FindByID(_ context.Context, id domain.EditionID) (*domain.Edition, error) {
	if e, ok := m.byID[id]; ok {
		return e, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memEditions) FindByWork(_ context.Context, _ domain.WorkID) ([]*domain.Edition, error) {
	return nil, nil
}
func (m *memEditions) Save(_ context.Context, _ *domain.Edition) error { return nil }

type inlineTx struct{}

func (inlineTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type seqID struct{ n int }

func (s *seqID) NewID() string { s.n++; return "id-" + string(rune('a'+s.n-1)) }

type memExport struct {
	progress []postgres.ExportProgress
	marks    []postgres.ExportMark
	works    map[string]bool
}

func (m *memExport) WorkExists(_ context.Context, id string) (bool, error) { return m.works[id], nil }
func (m *memExport) ListProgress(_ context.Context, workID string) ([]postgres.ExportProgress, error) {
	if workID == "" {
		return m.progress, nil
	}
	var out []postgres.ExportProgress
	for _, p := range m.progress {
		if p.WorkID == workID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (m *memExport) ListMarks(_ context.Context, _ string) ([]postgres.ExportMark, error) {
	return m.marks, nil
}

func readingServer(t *testing.T, api transporthttp.ReadingAPI, spy *testutil.SpyHandler) http.Handler {
	t.Helper()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetReadingAPI(api)
	fixed := func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/reading/works/{workId}/progress", transporthttp.ReadingProgressGetHandler(poolRef))
	mux.Handle("POST /api/v1/reading/works/{workId}/progress", transporthttp.ReadingProgressReportHandler(poolRef, fixed))
	mux.Handle("GET /api/v1/reading/editions/{editionId}/bookmarks", transporthttp.ReadingBookmarksListHandler(poolRef))
	mux.Handle("POST /api/v1/reading/editions/{editionId}/bookmarks", transporthttp.ReadingBookmarkCreateHandler(poolRef, fixed))
	mux.Handle("DELETE /api/v1/reading/bookmarks/{bookmarkId}", transporthttp.ReadingBookmarkDeleteHandler(poolRef))
	mux.Handle("GET /api/v1/reading/editions/{editionId}/highlights", transporthttp.ReadingHighlightsListHandler(poolRef))
	mux.Handle("POST /api/v1/reading/editions/{editionId}/highlights", transporthttp.ReadingHighlightCreateHandler(poolRef, fixed))
	mux.Handle("PATCH /api/v1/reading/highlights/{highlightId}", transporthttp.ReadingHighlightPatchHandler(poolRef))
	mux.Handle("DELETE /api/v1/reading/highlights/{highlightId}", transporthttp.ReadingHighlightDeleteHandler(poolRef))
	mux.Handle("GET /api/v1/reading/preferences", transporthttp.ReadingPreferencesGetHandler(poolRef))
	mux.Handle("PUT /api/v1/reading/preferences", transporthttp.ReadingPreferencesPutHandler(poolRef))
	mux.Handle("GET /api/v1/reading/export", transporthttp.ReadingExportHandler(poolRef, spyLogger(spy), fixed))
	return transporthttp.Chain(mux, transporthttp.Recovery(nil, func() string { return "test" }))
}

func spyLogger(s *testutil.SpyHandler) *slog.Logger {
	if s == nil {
		return nil
	}
	return slog.New(s)
}

func newReadingAPI() (transporthttp.ReadingAPI, *memExport) {
	exp := &memExport{works: map[string]bool{}}
	return transporthttp.ReadingAPI{
		Progress:    &memReadingProgress{byWork: map[domain.WorkID]*domain.ReadingProgress{}},
		Bookmarks:   &memBookmarks{byID: map[domain.BookmarkID]*domain.Bookmark{}},
		Highlights:  &memHighlights{byID: map[domain.HighlightID]*domain.Highlight{}},
		Preferences: &memPrefs{byDevice: map[domain.DeviceID]*domain.ReadingPreferences{}},
		Editions:    &memEditions{byID: map[domain.EditionID]*domain.Edition{}},
		Transactor:  inlineTx{},
		IDs:         &seqID{},
		Export:      exp,
	}, exp
}

const validDevice = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

func do(t *testing.T, srv http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

// --- tests ------------------------------------------------------------

func TestReadingProgress_GetNullThenReportThenReconcile(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)

	rr := do(t, srv, http.MethodGet, "/api/v1/reading/works/work-1/progress", "", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"progress":null`) {
		t.Fatalf("expected null progress, got %d %s", rr.Code, rr.Body.String())
	}

	rr = do(t, srv, http.MethodPost, "/api/v1/reading/works/work-1/progress",
		`{"percentage":0.4,"observedEpoch":0}`, map[string]string{"X-Device-Id": validDevice})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"outcome":"advanced"`) {
		t.Fatalf("first report: %d %s", rr.Code, rr.Body.String())
	}

	// A behind report is rejected, canonical unchanged.
	rr = do(t, srv, http.MethodPost, "/api/v1/reading/works/work-1/progress",
		`{"percentage":0.1,"observedEpoch":0}`, map[string]string{"X-Device-Id": validDevice})
	if !strings.Contains(rr.Body.String(), `"outcome":"rejected"`) || !strings.Contains(rr.Body.String(), `"percentage":0.4`) {
		t.Fatalf("behind report should be rejected with canonical intact: %s", rr.Body.String())
	}
}

func TestReadingProgress_MissingDeviceIDIs400(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/works/work-1/progress", `{"percentage":0.4,"observedEpoch":0}`, nil)
	if rr.Code != 400 {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestReadingProgress_HostileCFIIs400(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/works/work-1/progress",
		`{"percentage":0.4,"observedEpoch":0,"precisePosition":{"editionId":"e1","cfi":"<script>alert(1)</script>"}}`,
		map[string]string{"X-Device-Id": validDevice})
	if rr.Code != 400 {
		t.Fatalf("status = %d, want 400 for a hostile CFI; body %s", rr.Code, rr.Body.String())
	}
}

func TestReadingProgress_OverrideBumpsEpoch(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)
	_ = do(t, srv, http.MethodPost, "/api/v1/reading/works/work-1/progress",
		`{"percentage":0.9,"observedEpoch":0}`, map[string]string{"X-Device-Id": validDevice})
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/works/work-1/progress",
		`{"percentage":0.1,"observedEpoch":0,"override":true}`, map[string]string{"X-Device-Id": validDevice})
	if !strings.Contains(rr.Body.String(), `"outcome":"overridden"`) || !strings.Contains(rr.Body.String(), `"epoch":1`) {
		t.Fatalf("override should bump epoch: %s", rr.Body.String())
	}
}

func TestReadingBookmarks_CRUD(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)

	rr := do(t, srv, http.MethodPost, "/api/v1/reading/editions/ed-1/bookmarks",
		`{"cfi":"epubcfi(/6/4!/4/10)","label":"the turn"}`, nil)
	if rr.Code != 201 {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		Bookmark struct {
			ID string `json:"id"`
		} `json:"bookmark"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	rr = do(t, srv, http.MethodGet, "/api/v1/reading/editions/ed-1/bookmarks", "", nil)
	if !strings.Contains(rr.Body.String(), "the turn") {
		t.Fatalf("list missing the bookmark: %s", rr.Body.String())
	}

	rr = do(t, srv, http.MethodDelete, "/api/v1/reading/bookmarks/"+created.Bookmark.ID, "", nil)
	if rr.Code != 204 {
		t.Fatalf("delete: %d", rr.Code)
	}
	rr = do(t, srv, http.MethodDelete, "/api/v1/reading/bookmarks/does-not-exist", "", nil)
	if rr.Code != 404 {
		t.Fatalf("delete missing: %d, want 404", rr.Code)
	}
}

func TestReadingHighlights_EndBeforeStartIs400(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/editions/ed-1/highlights",
		`{"startCfi":"epubcfi(/6/4!/4/10)","endCfi":"epubcfi(/6/4!/4/2)"}`, nil)
	if rr.Code != 400 {
		t.Fatalf("status = %d, want 400 for endCfi before startCfi", rr.Code)
	}
}

func TestReadingHighlights_PatchNoteOnly(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/editions/ed-1/highlights",
		`{"startCfi":"epubcfi(/6/4!/4/2/1:0)","endCfi":"epubcfi(/6/4!/4/2/1:20)","category":"blue"}`, nil)
	var created struct {
		Highlight struct {
			ID string `json:"id"`
		} `json:"highlight"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	rr = do(t, srv, http.MethodPatch, "/api/v1/reading/highlights/"+created.Highlight.ID, `{"note":"a thought"}`, nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "a thought") || !strings.Contains(rr.Body.String(), `"category":"blue"`) {
		t.Fatalf("patch note only: %d %s", rr.Code, rr.Body.String())
	}
}

func TestReadingPreferences_DefaultsThenPersist(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)

	rr := do(t, srv, http.MethodGet, "/api/v1/reading/preferences", "", map[string]string{"X-Device-Id": validDevice})
	if !strings.Contains(rr.Body.String(), `"theme":"light"`) || !strings.Contains(rr.Body.String(), `"layoutMode":"paginated"`) {
		t.Fatalf("defaults: %s", rr.Body.String())
	}

	rr = do(t, srv, http.MethodPut, "/api/v1/reading/preferences",
		`{"font":"Newsreader","fontSize":22,"lineSpacing":1.7,"theme":"sepia","layoutMode":"scroll","columnWidth":"wide"}`,
		map[string]string{"X-Device-Id": validDevice})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"theme":"sepia"`) {
		t.Fatalf("put: %d %s", rr.Code, rr.Body.String())
	}

	rr = do(t, srv, http.MethodPut, "/api/v1/reading/preferences",
		`{"font":"x","fontSize":22,"lineSpacing":1.7,"theme":"neon","layoutMode":"scroll","columnWidth":"wide"}`,
		map[string]string{"X-Device-Id": validDevice})
	if rr.Code != 400 {
		t.Fatalf("invalid theme should be 400, got %d", rr.Code)
	}
}

func TestReadingExport_VersionedDocumentAndScopedNotFound(t *testing.T) {
	api, exp := newReadingAPI()
	exp.works["work-1"] = true
	exp.progress = []postgres.ExportProgress{{WorkID: "work-1", Percentage: 0.5, Epoch: 2, ObservedAt: time.Unix(0, 0)}}
	exp.marks = []postgres.ExportMark{{ID: "b1", EditionID: "ed-1", Kind: "bookmark", StartCFI: "epubcfi(/6/4!/4)", Label: "x", CreatedAt: time.Unix(0, 0)}}
	srv := readingServer(t, api, nil)

	rr := do(t, srv, http.MethodGet, "/api/v1/reading/export", "", nil)
	if rr.Code != 200 {
		t.Fatalf("export: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"schemaVersion":1`) {
		t.Fatalf("missing schemaVersion: %s", rr.Body.String())
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("missing attachment disposition: %q", cd)
	}

	rr = do(t, srv, http.MethodGet, "/api/v1/reading/export?workId=nope", "", nil)
	if rr.Code != 404 {
		t.Fatalf("unknown workId should be 404, got %d", rr.Code)
	}
}

// §8: no reading content (CFI, label, note, percentage, title) in logs —
// only counts.
func TestReadingExport_LogsCountsOnly(t *testing.T) {
	api, exp := newReadingAPI()
	exp.progress = []postgres.ExportProgress{{WorkID: "work-1", Percentage: 0.5, Epoch: 1, ObservedAt: time.Unix(0, 0)}}
	exp.marks = []postgres.ExportMark{{ID: "h1", EditionID: "ed-1", Kind: "highlight", StartCFI: "epubcfi(/6/4!/4/2/1:0)", EndCFI: "epubcfi(/6/4!/4/2/1:9)", Note: "SECRET NOTE", CreatedAt: time.Unix(0, 0)}}
	spy := testutil.NewSpyHandler()
	srv := readingServer(t, api, spy)

	_ = do(t, srv, http.MethodGet, "/api/v1/reading/export", "", nil)

	for _, leak := range []string{"SECRET NOTE", "epubcfi(", "0.5"} {
		if spy.Contains(leak) {
			t.Fatalf("log leaked %q", leak)
		}
	}
	if !spy.Contains("ed-1") && len(spy.Records()) == 0 {
		t.Fatalf("expected an export log record")
	}
}
