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
func (m *memReadingProgress) FindByWorkAndUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, w domain.WorkID) (*domain.ReadingProgress, error) {
	return m.FindByWork(ctx, w)
}
func (m *memReadingProgress) FindByWorkAndUserForUpdate(ctx context.Context, _ domain.UserID, _ domain.LibraryID, w domain.WorkID) (*domain.ReadingProgress, error) {
	return m.FindByWork(ctx, w)
}
func (m *memReadingProgress) Save(_ context.Context, p *domain.ReadingProgress) error {
	m.byWork[p.WorkID()] = p
	return nil
}
func (m *memReadingProgress) SaveForUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, p *domain.ReadingProgress) error {
	return m.Save(ctx, p)
}

// memBookmarks is user-aware: SaveForUser records the owner, and the
// AndUser lookups honour it. The bare methods (FindByID/Save/Delete) are
// kept only to satisfy the interface — the reading handlers must not call
// them, and check-user-scoped-reading.sh enforces that.
type memBookmarks struct {
	byID  map[domain.BookmarkID]*domain.Bookmark
	owner map[domain.BookmarkID]domain.UserID
}

func (m *memBookmarks) FindByID(_ context.Context, id domain.BookmarkID) (*domain.Bookmark, error) {
	if b, ok := m.byID[id]; ok {
		return b, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memBookmarks) FindByIDAndUser(_ context.Context, userID domain.UserID, id domain.BookmarkID) (*domain.Bookmark, error) {
	if b, ok := m.byID[id]; ok && m.owner[id] == userID {
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
func (m *memBookmarks) FindByEditionAndUser(_ context.Context, userID domain.UserID, _ domain.LibraryID, e domain.EditionID) ([]*domain.Bookmark, error) {
	var out []*domain.Bookmark
	for id, b := range m.byID {
		if b.EditionID() == e && m.owner[id] == userID {
			out = append(out, b)
		}
	}
	return out, nil
}
func (m *memBookmarks) Save(_ context.Context, b *domain.Bookmark) error {
	m.byID[b.ID()] = b
	return nil
}
func (m *memBookmarks) SaveForUser(_ context.Context, userID domain.UserID, _ domain.LibraryID, b *domain.Bookmark) error {
	m.byID[b.ID()] = b
	m.owner[b.ID()] = userID
	return nil
}
func (m *memBookmarks) Delete(_ context.Context, id domain.BookmarkID) error {
	delete(m.byID, id)
	return nil
}
func (m *memBookmarks) DeleteAndUser(_ context.Context, userID domain.UserID, id domain.BookmarkID) error {
	if _, ok := m.byID[id]; !ok || m.owner[id] != userID {
		return &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	delete(m.byID, id)
	delete(m.owner, id)
	return nil
}

type memHighlights struct {
	byID  map[domain.HighlightID]*domain.Highlight
	owner map[domain.HighlightID]domain.UserID
}

func (m *memHighlights) FindByID(_ context.Context, id domain.HighlightID) (*domain.Highlight, error) {
	if h, ok := m.byID[id]; ok {
		return h, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memHighlights) FindByIDAndUser(_ context.Context, userID domain.UserID, id domain.HighlightID) (*domain.Highlight, error) {
	if h, ok := m.byID[id]; ok && m.owner[id] == userID {
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
func (m *memHighlights) FindByEditionAndUser(_ context.Context, userID domain.UserID, _ domain.LibraryID, e domain.EditionID) ([]*domain.Highlight, error) {
	var out []*domain.Highlight
	for id, h := range m.byID {
		if h.EditionID() == e && m.owner[id] == userID {
			out = append(out, h)
		}
	}
	return out, nil
}
func (m *memHighlights) Save(_ context.Context, h *domain.Highlight) error {
	m.byID[h.ID()] = h
	return nil
}
func (m *memHighlights) SaveForUser(_ context.Context, userID domain.UserID, _ domain.LibraryID, h *domain.Highlight) error {
	m.byID[h.ID()] = h
	m.owner[h.ID()] = userID
	return nil
}
func (m *memHighlights) UpdateNoteCategoryAndUser(_ context.Context, userID domain.UserID, id domain.HighlightID, note, category string) error {
	h, ok := m.byID[id]
	if !ok || m.owner[id] != userID {
		return &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	// Rebuild preserving edition/positions/createdAt — the fake has no
	// separate library field, so this mirrors the SQL's "note/category
	// only" update.
	m.byID[id] = domain.NewHighlight(h.ID(), h.EditionID(), h.StartPosition(), h.EndPosition(), note, category, h.CreatedAt())
	return nil
}
func (m *memHighlights) Delete(_ context.Context, id domain.HighlightID) error {
	delete(m.byID, id)
	return nil
}
func (m *memHighlights) DeleteAndUser(_ context.Context, userID domain.UserID, id domain.HighlightID) error {
	if _, ok := m.byID[id]; !ok || m.owner[id] != userID {
		return &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	delete(m.byID, id)
	delete(m.owner, id)
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
func (m *memPrefs) FindByUserAndDevice(ctx context.Context, _ domain.UserID, d domain.DeviceID) (*domain.ReadingPreferences, error) {
	return m.FindByDevice(ctx, d)
}
func (m *memPrefs) Save(_ context.Context, p *domain.ReadingPreferences) error {
	m.byDevice[p.DeviceID()] = p
	return nil
}
func (m *memPrefs) SaveForUser(ctx context.Context, _ domain.UserID, p *domain.ReadingPreferences) error {
	return m.Save(ctx, p)
}

// memLibraryEntries maps edition -> the single library that owns it. When
// permissive is set (the default from newReadingAPI), every edition/work
// is treated as in-library — so tests that don't care about the
// library-ownership gate keep working; the IDOR tests set an explicit map.
type memLibraryEntries struct {
	inLib      map[domain.EditionID]domain.LibraryID
	works      map[domain.WorkID]domain.LibraryID
	permissive bool
}

func (m *memLibraryEntries) FindByEdition(_ context.Context, e domain.EditionID) (*domain.LibraryEntry, error) {
	if _, ok := m.inLib[e]; ok || m.permissive {
		return domain.NewLibraryEntry("le-1", e, time.Time{}), nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (m *memLibraryEntries) EditionInLibrary(_ context.Context, e domain.EditionID, lib domain.LibraryID) (bool, error) {
	if m.permissive {
		return true, nil
	}
	return m.inLib[e] == lib, nil
}
func (m *memLibraryEntries) WorkInLibrary(_ context.Context, wk domain.WorkID, lib domain.LibraryID) (bool, error) {
	if m.permissive {
		return true, nil
	}
	return m.works[wk] == lib, nil
}
func (m *memLibraryEntries) Save(_ context.Context, _ *domain.LibraryEntry) error        { return nil }
func (m *memLibraryEntries) DeleteByEdition(_ context.Context, _ domain.EditionID) error { return nil }

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

type memExportUser struct {
	progress []postgres.ExportProgress
	marks    []postgres.ExportMark
}

type memExport struct {
	progress []postgres.ExportProgress
	marks    []postgres.ExportMark
	works    map[string]bool
	byUser   map[domain.UserID]memExportUser
	gotUser  domain.UserID
}

func (m *memExport) WorkExists(_ context.Context, id string) (bool, error) { return m.works[id], nil }

// gotUser records the userID the last ListProgress/ListMarks call was
// scoped to, so a handler test can assert the handler passed the context
// user (AUDIT-0012-C1) rather than an empty string.
func (m *memExport) ListProgress(_ context.Context, userID domain.UserID, _ domain.LibraryID, workID string) ([]postgres.ExportProgress, error) {
	m.gotUser = userID
	rows := m.progress
	if u, scoped := m.byUser[userID]; scoped {
		rows = u.progress
	}
	if workID == "" {
		return rows, nil
	}
	var out []postgres.ExportProgress
	for _, p := range rows {
		if p.WorkID == workID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (m *memExport) ListMarks(_ context.Context, userID domain.UserID, _ domain.LibraryID, _ string) ([]postgres.ExportMark, error) {
	m.gotUser = userID
	if u, scoped := m.byUser[userID]; scoped {
		return u.marks, nil
	}
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
	return transporthttp.Chain(mux, transporthttp.Recovery(nil, func() string { return "test" }), testUserMW)
}

// testUserMW stands in for the auth middleware in reading-handler tests:
// it injects an authenticated user (X-Test-User header, default
// "u-default") and active library (X-Library-Id header, default
// DefaultLibraryID) into the request context, so the handlers'
// readingScope resolves. A request with X-Test-User: "" gets no user —
// used to prove a handler 401s without one.
func testUserMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		uid := r.Header.Get("X-Test-User")
		if _, explicit := r.Header["X-Test-User"]; !explicit {
			uid = "u-default"
		}
		if uid != "" {
			ctx = transporthttp.WithUser(ctx, &transporthttp.AuthenticatedUser{UserID: domain.UserID(uid)})
			lib := r.Header.Get("X-Library-Id")
			if lib == "" {
				lib = string(domain.DefaultLibraryID)
			}
			ctx = transporthttp.WithActiveLibrary(ctx, domain.LibraryID(lib))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func spyLogger(s *testutil.SpyHandler) *slog.Logger {
	if s == nil {
		return nil
	}
	return slog.New(s)
}

func newReadingAPI() (transporthttp.ReadingAPI, *memExport) {
	exp := &memExport{works: map[string]bool{}, byUser: map[domain.UserID]memExportUser{}}
	return transporthttp.ReadingAPI{
		Progress:       &memReadingProgress{byWork: map[domain.WorkID]*domain.ReadingProgress{}},
		Bookmarks:      &memBookmarks{byID: map[domain.BookmarkID]*domain.Bookmark{}, owner: map[domain.BookmarkID]domain.UserID{}},
		Highlights:     &memHighlights{byID: map[domain.HighlightID]*domain.Highlight{}, owner: map[domain.HighlightID]domain.UserID{}},
		Preferences:    &memPrefs{byDevice: map[domain.DeviceID]*domain.ReadingPreferences{}},
		Editions:       &memEditions{byID: map[domain.EditionID]*domain.Edition{}},
		LibraryEntries: &memLibraryEntries{permissive: true},
		Transactor:     inlineTx{},
		IDs:            &seqID{},
		Export:         exp,
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

// hdr builds a header map for the do() helper.
func hdr(pairs ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return m
}

// TestReading_PerUserIDOR is the AUDIT-0012-C1 close-gate test: user B
// must not be able to read, modify, delete, or export user A's reading
// data by knowing an id. Without the per-user scoping fix, every one of
// these assertions fails.
func TestReading_PerUserIDOR(t *testing.T) {
	api, exp := newReadingAPI()
	srv := readingServer(t, api, nil)
	alice := hdr("X-Test-User", "alice")
	bob := hdr("X-Test-User", "bob")

	// Alice creates a bookmark and a highlight.
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/editions/ed-1/bookmarks",
		`{"cfi":"epubcfi(/6/4!/4/10)","label":"alice-secret"}`, alice)
	if rr.Code != 201 {
		t.Fatalf("alice create bookmark: %d %s", rr.Code, rr.Body.String())
	}
	var bm struct {
		Bookmark struct{ ID string } `json:"bookmark"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &bm)

	rr = do(t, srv, http.MethodPost, "/api/v1/reading/editions/ed-1/highlights",
		`{"startCfi":"epubcfi(/6/4!/4/2/1:0)","endCfi":"epubcfi(/6/4!/4/2/1:9)","note":"alice-note"}`, alice)
	if rr.Code != 201 {
		t.Fatalf("alice create highlight: %d %s", rr.Code, rr.Body.String())
	}
	var hl struct {
		Highlight struct{ ID string } `json:"highlight"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &hl)

	t.Run("bob cannot list alice's bookmarks by edition", func(t *testing.T) {
		rr := do(t, srv, http.MethodGet, "/api/v1/reading/editions/ed-1/bookmarks", "", bob)
		if strings.Contains(rr.Body.String(), "alice-secret") {
			t.Fatalf("bob saw alice's bookmark: %s", rr.Body.String())
		}
	})

	t.Run("bob cannot list alice's highlights by edition", func(t *testing.T) {
		rr := do(t, srv, http.MethodGet, "/api/v1/reading/editions/ed-1/highlights", "", bob)
		if strings.Contains(rr.Body.String(), "alice-note") {
			t.Fatalf("bob saw alice's highlight note: %s", rr.Body.String())
		}
	})

	t.Run("bob cannot delete alice's bookmark by id", func(t *testing.T) {
		rr := do(t, srv, http.MethodDelete, "/api/v1/reading/bookmarks/"+bm.Bookmark.ID, "", bob)
		if rr.Code != 404 {
			t.Fatalf("bob delete alice's bookmark: %d, want 404", rr.Code)
		}
		// Alice's bookmark still exists.
		rr = do(t, srv, http.MethodGet, "/api/v1/reading/editions/ed-1/bookmarks", "", alice)
		if !strings.Contains(rr.Body.String(), "alice-secret") {
			t.Fatalf("bob's failed delete removed alice's bookmark: %s", rr.Body.String())
		}
	})

	t.Run("bob cannot patch alice's highlight by id", func(t *testing.T) {
		rr := do(t, srv, http.MethodPatch, "/api/v1/reading/highlights/"+hl.Highlight.ID,
			`{"note":"bob-was-here"}`, bob)
		if rr.Code != 404 {
			t.Fatalf("bob patch alice's highlight: %d, want 404", rr.Code)
		}
	})

	t.Run("bob cannot delete alice's highlight by id", func(t *testing.T) {
		rr := do(t, srv, http.MethodDelete, "/api/v1/reading/highlights/"+hl.Highlight.ID, "", bob)
		if rr.Code != 404 {
			t.Fatalf("bob delete alice's highlight: %d, want 404", rr.Code)
		}
	})

	t.Run("export is scoped to the calling user", func(t *testing.T) {
		exp.byUser["alice"] = memExportUser{
			marks: []postgres.ExportMark{{ID: "m-a", EditionID: "ed-1", Kind: "bookmark", StartCFI: "x", Label: "alice-secret"}},
		}
		exp.byUser["bob"] = memExportUser{
			marks: []postgres.ExportMark{{ID: "m-b", EditionID: "ed-1", Kind: "bookmark", StartCFI: "y", Label: "bob-only"}},
		}
		rr := do(t, srv, http.MethodGet, "/api/v1/reading/export", "", bob)
		if exp.gotUser != "bob" {
			t.Fatalf("export was not scoped to the caller: gotUser=%q", exp.gotUser)
		}
		if strings.Contains(rr.Body.String(), "alice-secret") {
			t.Fatalf("bob's export leaked alice's data: %s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "bob-only") {
			t.Fatalf("bob's export missing bob's own data: %s", rr.Body.String())
		}
	})
}

func TestReading_RequiresAuthenticatedUser(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)
	// X-Test-User: "" -> testUserMW injects no user -> handler 401s.
	rr := do(t, srv, http.MethodGet, "/api/v1/reading/works/work-1/progress", "", hdr("X-Test-User", ""))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without an authenticated user, got %d", rr.Code)
	}
}

// TestReading_WritesGatedOnLibraryOwnership covers the PR #78 review
// finding: the C1 fix gated reads on library ownership but left the write
// path open. A bookmark/highlight/progress write against an edition or
// work outside the caller's active library is a 404, not a stored row.
func TestReading_WritesGatedOnLibraryOwnership(t *testing.T) {
	api, _ := newReadingAPI()
	// "ed-mine" is in the default library; "ed-foreign" is in another.
	api.LibraryEntries = &memLibraryEntries{
		inLib: map[domain.EditionID]domain.LibraryID{"ed-mine": domain.DefaultLibraryID},
		works: map[domain.WorkID]domain.LibraryID{"work-mine": domain.DefaultLibraryID},
	}
	srv := readingServer(t, api, nil)

	cases := []struct {
		name, method, path, body string
	}{
		{"bookmark on a foreign edition", http.MethodPost, "/api/v1/reading/editions/ed-foreign/bookmarks", `{"cfi":"epubcfi(/6/4!/4/10)"}`},
		{"highlight on a foreign edition", http.MethodPost, "/api/v1/reading/editions/ed-foreign/highlights", `{"startCfi":"epubcfi(/6/4!/4/2/1:0)","endCfi":"epubcfi(/6/4!/4/2/1:9)"}`},
		{"progress on a work with no edition in the library", http.MethodPost, "/api/v1/reading/works/work-foreign/progress", `{"percentage":0.4,"observedEpoch":0}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := map[string]string{}
			if c.method == http.MethodPost && strings.Contains(c.path, "/progress") {
				h["X-Device-Id"] = validDevice
			}
			rr := do(t, srv, c.method, c.path, c.body, h)
			if rr.Code != http.StatusNotFound {
				t.Fatalf("%s: status = %d, want 404; body %s", c.name, rr.Code, rr.Body.String())
			}
		})
	}

	// A write against the owned edition still works.
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/editions/ed-mine/bookmarks", `{"cfi":"epubcfi(/6/4!/4/10)"}`, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("bookmark on an owned edition: status = %d, want 201; body %s", rr.Code, rr.Body.String())
	}
}

// TestReadingHighlights_PatchDoesNotRelocate covers the PR #78 review
// finding: a PATCH must update note/category only, never re-scope the
// highlight into the request's active library. Here it is exercised at
// the handler level — the PATCH succeeds even when X-Library-Id names a
// different (but still valid) library, and does not error.
func TestReadingHighlights_PatchDoesNotRelocate(t *testing.T) {
	api, _ := newReadingAPI()
	srv := readingServer(t, api, nil)

	// Create as the default user with the default active library.
	rr := do(t, srv, http.MethodPost, "/api/v1/reading/editions/ed-1/highlights",
		`{"startCfi":"epubcfi(/6/4!/4/2/1:0)","endCfi":"epubcfi(/6/4!/4/2/1:9)","note":"first"}`, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		Highlight struct{ ID string } `json:"highlight"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	// PATCH with a different X-Library-Id — must still update, not error.
	rr = do(t, srv, http.MethodPatch, "/api/v1/reading/highlights/"+created.Highlight.ID,
		`{"note":"second"}`, hdr("X-Library-Id", "some-other-library"))
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "second") {
		t.Fatalf("patch did not apply: %s", rr.Body.String())
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

func TestReadingProgressReport_RevokedDeviceRejected(t *testing.T) {
	api, _ := newReadingAPI()
	devRepo := newMemPairedDevs()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	dev, err := domain.NewPairedDevice("3f2504e0-4f89-41d3-9a0c-0305e82c3301", "u-default", "Phone", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.Revoke(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	_ = devRepo.Save(context.Background(), dev)
	api.Devices = devRepo
	srv := readingServer(t, api, nil)

	rr := do(t, srv, http.MethodPost, "/api/v1/reading/works/work-1/progress",
		`{"percentage":0.4,"observedEpoch":0}`, map[string]string{"X-Device-Id": "3f2504e0-4f89-41d3-9a0c-0305e82c3301"})
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), "device revoked") {
		t.Fatalf("expected 401 device revoked, got %d: %s", rr.Code, rr.Body.String())
	}
}

