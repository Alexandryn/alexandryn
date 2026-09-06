package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type memPairedDevs struct {
	mu          sync.Mutex
	byID        map[domain.DeviceID]*domain.PairedDevice
	provisional map[domain.DeviceID]*provDev
	bySess      map[domain.PairingSessionID]domain.DeviceID
}

type provDev struct {
	id        domain.DeviceID
	label     string
	class     domain.DeviceClass
	via       domain.EnrolledVia
	sessID    domain.PairingSessionID
	createdAt time.Time
	revokedAt *time.Time
}

func newMemPairedDevs() *memPairedDevs {
	return &memPairedDevs{
		byID:        make(map[domain.DeviceID]*domain.PairedDevice),
		provisional: make(map[domain.DeviceID]*provDev),
		bySess:      make(map[domain.PairingSessionID]domain.DeviceID),
	}
}

func (m *memPairedDevs) FindByID(_ context.Context, id domain.DeviceID) (*domain.PairedDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "paired device not found"}
	}
	return d, nil
}

func (m *memPairedDevs) FindByOwner(_ context.Context, owner domain.UserID) ([]*domain.PairedDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []*domain.PairedDevice
	for _, d := range m.byID {
		if d.Owner() == owner {
			list = append(list, d)
		}
	}
	return list, nil
}

func (m *memPairedDevs) FindByPairingSessionID(_ context.Context, sessionID domain.PairingSessionID) (*domain.PairedDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	devID, ok := m.bySess[sessionID]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "paired device not found"}
	}
	d, ok := m.byID[devID]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "paired device is provisional (unassigned owner)"}
	}
	return d, nil
}

func (m *memPairedDevs) InsertProvisional(_ context.Context, id domain.DeviceID, label string, class domain.DeviceClass, via domain.EnrolledVia, sessID domain.PairingSessionID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.provisional[id] = &provDev{id: id, label: label, class: class, via: via, sessID: sessID, createdAt: now}
	m.bySess[sessID] = id
	return nil
}

func (m *memPairedDevs) AssignOwnerByPairingSession(_ context.Context, sessID domain.PairingSessionID, owner domain.UserID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	devID, ok := m.bySess[sessID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	p, ok := m.provisional[devID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	d, err := domain.NewPairedDevice(devID, owner, p.label, p.class, p.via, p.createdAt)
	if err != nil {
		return err
	}
	m.byID[devID] = d
	delete(m.provisional, devID)
	return nil
}

func (m *memPairedDevs) Save(_ context.Context, d *domain.PairedDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[d.ID()] = d
	return nil
}

func (m *memPairedDevs) Revoke(_ context.Context, id domain.DeviceID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "paired device not found or already revoked"}
	}
	return d.Revoke(now)
}

func (m *memPairedDevs) RevokeByPairingSessionID(_ context.Context, sessID domain.PairingSessionID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	devID, ok := m.bySess[sessID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	d, ok := m.byID[devID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "not found"}
	}
	return d.Revoke(now)
}

func (m *memPairedDevs) AdvanceCursor(_ context.Context, id domain.DeviceID, newCursor int64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "paired device not found or already revoked"}
	}
	return d.AdvanceCursor(newCursor, now)
}

func TestListDevicesHandler_OwnerScoped(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	repo := newMemPairedDevs()

	userA := domain.UserID("user-a")
	userB := domain.UserID("user-b")

	devA1, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	devA2, _ := domain.NewPairedDevice("dev-a2", userA, "iPad Air", domain.DeviceClassTablet, domain.EnrolledViaPairingCode, now)
	_ = devA2.Revoke(now.Add(time.Minute))

	devB, _ := domain.NewPairedDevice("dev-b1", userB, "Study Mac", domain.DeviceClassDesktop, domain.EnrolledViaPasswordLogin, now)

	_ = repo.Save(context.Background(), devA1)
	_ = repo.Save(context.Background(), devA2)
	_ = repo.Save(context.Background(), devB)

	handler := transporthttp.ListDevicesHandler(repo)

	// User A request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
		UserID: userA,
		Role:   domain.RoleReader,
	})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Devices []struct {
			ID          string  `json:"id"`
			Label       string  `json:"label"`
			DeviceClass string  `json:"deviceClass"`
			EnrolledVia string  `json:"enrolledVia"`
			CreatedAt   string  `json:"createdAt"`
			LastSeenAt  string  `json:"lastSeenAt"`
			LastSyncedAt *string `json:"lastSyncedAt"`
			RevokedAt   *string `json:"revokedAt"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Devices) != 2 {
		t.Fatalf("expected 2 devices for user A, got %d", len(resp.Devices))
	}

	// Ensure no devB device is returned
	for _, d := range resp.Devices {
		if d.ID == "dev-b1" {
			t.Fatal("IDOR violation: user A received user B's device")
		}
	}

	// User with 0 devices
	reqEmpty := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	ctxEmpty := transporthttp.WithUser(reqEmpty.Context(), &transporthttp.AuthenticatedUser{
		UserID: "user-empty",
		Role:   domain.RoleReader,
	})
	rrEmpty := httptest.NewRecorder()
	handler.ServeHTTP(rrEmpty, reqEmpty.WithContext(ctxEmpty))

	if rrEmpty.Code != http.StatusOK {
		t.Fatalf("expected status 200 for user with no devices, got %d", rrEmpty.Code)
	}
	var emptyResp struct {
		Devices []any `json:"devices"`
	}
	_ = json.Unmarshal(rrEmpty.Body.Bytes(), &emptyResp)
	if emptyResp.Devices == nil || len(emptyResp.Devices) != 0 {
		t.Fatalf("expected empty devices array, got %v", emptyResp.Devices)
	}
}

func TestRevokeDeviceHandler(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	repo := newMemPairedDevs()

	userA := domain.UserID("user-a")
	userB := domain.UserID("user-b")

	devA, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	devB, _ := domain.NewPairedDevice("dev-b1", userB, "Study Mac", domain.DeviceClassDesktop, domain.EnrolledViaPasswordLogin, now)

	_ = repo.Save(context.Background(), devA)
	_ = repo.Save(context.Background(), devB)

	handler := transporthttp.RevokeDeviceHandler(repo, func() time.Time { return now })

	// 1. Cross-user attempt: User A tries to revoke dev-b1 -> 404 Not Found (NOT 403)
	reqCross := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/dev-b1", nil)
	reqCross.SetPathValue("id", "dev-b1")
	ctxA := transporthttp.WithUser(reqCross.Context(), &transporthttp.AuthenticatedUser{
		UserID: userA,
		Role:   domain.RoleReader,
	})
	rrCross := httptest.NewRecorder()
	handler.ServeHTTP(rrCross, reqCross.WithContext(ctxA))

	if rrCross.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on cross-user revocation, got %d", rrCross.Code)
	}

	// Verify devB is still active
	bCheck, _ := repo.FindByID(context.Background(), "dev-b1")
	if bCheck.RevokedAt() != nil {
		t.Fatal("devB was revoked by cross-user attempt")
	}

	// 2. Non-existent device -> 404 Not Found
	reqNone := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/dev-none", nil)
	reqNone.SetPathValue("id", "dev-none")
	rrNone := httptest.NewRecorder()
	handler.ServeHTTP(rrNone, reqNone.WithContext(ctxA))

	if rrNone.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on nonexistent device revocation, got %d", rrNone.Code)
	}

	// 3. User A revokes devA1 -> 204 No Content
	reqValid := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/dev-a1", nil)
	reqValid.SetPathValue("id", "dev-a1")
	rrValid := httptest.NewRecorder()
	handler.ServeHTTP(rrValid, reqValid.WithContext(ctxA))

	if rrValid.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on valid revocation, got %d", rrValid.Code)
	}

	aCheck, _ := repo.FindByID(context.Background(), "dev-a1")
	if aCheck.RevokedAt() == nil {
		t.Fatal("devA was not marked revoked")
	}

	// 4. Double revoke -> 409 Conflict
	rrDouble := httptest.NewRecorder()
	handler.ServeHTTP(rrDouble, reqValid.WithContext(ctxA))

	if rrDouble.Code != http.StatusConflict {
		t.Fatalf("expected 409 on already-revoked device, got %d", rrDouble.Code)
	}
}

func TestSyncMiddleware_RevocationAndTouch(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	repo := newMemPairedDevs()

	userA := domain.UserID("user-a")
	devActive, _ := domain.NewPairedDevice("dev-act", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	devRevoked, _ := domain.NewPairedDevice("dev-rev", userA, "Old Phone", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devRevoked.Revoke(now.Add(time.Minute))

	_ = repo.Save(context.Background(), devActive)
	_ = repo.Save(context.Background(), devRevoked)

	touchTime := now.Add(10 * time.Minute)
	mw := transporthttp.SyncMiddleware(repo, func() time.Time { return touchTime })

	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	chain := mw(dummy)

	// 1. Active device on /api/v1/sync/reading -> succeeds, touch advances last_seen_at
	reqSync := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading", nil)
	reqSync.Header.Set("X-Device-Id", "dev-act")
	ctxA := transporthttp.WithUser(reqSync.Context(), &transporthttp.AuthenticatedUser{
		UserID: userA,
		Role:   domain.RoleReader,
	})
	rrSync := httptest.NewRecorder()
	chain.ServeHTTP(rrSync, reqSync.WithContext(ctxA))

	if rrSync.Code != http.StatusOK {
		t.Fatalf("expected 200 for active device on sync route, got %d: %s", rrSync.Code, rrSync.Body.String())
	}

	d, _ := repo.FindByID(context.Background(), "dev-act")
	if !d.LastSeenAt().Equal(touchTime) {
		t.Fatalf("expected lastSeenAt updated to %v, got %v", touchTime, d.LastSeenAt())
	}

	// 2. Revoked device on /api/v1/sync/reading -> 401 Unauthorized with {"error": "device revoked"}
	reqRevSync := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading", nil)
	reqRevSync.Header.Set("X-Device-Id", "dev-rev")
	rrRevSync := httptest.NewRecorder()
	chain.ServeHTTP(rrRevSync, reqRevSync.WithContext(ctxA))

	if rrRevSync.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked device on sync route, got %d", rrRevSync.Code)
	}

	// 3. Missing device on /api/v1/sync/* -> 400 Bad Request
	reqNoDevSync := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading", nil)
	rrNoDevSync := httptest.NewRecorder()
	chain.ServeHTTP(rrNoDevSync, reqNoDevSync.WithContext(ctxA))

	if rrNoDevSync.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing device on sync route, got %d", rrNoDevSync.Code)
	}

	// 4. Missing device on /api/v1/devices -> passes (host web admin case)
	reqNoDevList := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	rrNoDevList := httptest.NewRecorder()
	chain.ServeHTTP(rrNoDevList, reqNoDevList.WithContext(ctxA))

	if rrNoDevList.Code != http.StatusOK {
		t.Fatalf("expected 200 for missing device on devices route, got %d", rrNoDevList.Code)
	}

	// 5. Revoked device on /api/v1/devices -> 401
	reqRevList := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqRevList.Header.Set("X-Device-Id", "dev-rev")
	rrRevList := httptest.NewRecorder()
	chain.ServeHTTP(rrRevList, reqRevList.WithContext(ctxA))

	if rrRevList.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked device on devices route, got %d", rrRevList.Code)
	}
}

type memSyncStore struct {
	mu           sync.Mutex
	data         map[string]*transporthttp.ReadingSyncData
	progressSeqs map[domain.ReadingProgressID]int64
	seqCounter   int64
}

func newMemSyncStore() *memSyncStore {
	return &memSyncStore{
		data:         make(map[string]*transporthttp.ReadingSyncData),
		progressSeqs: make(map[domain.ReadingProgressID]int64),
	}
}

func (m *memSyncStore) key(userID domain.UserID, libraryID domain.LibraryID) string {
	return string(userID) + ":" + string(libraryID)
}

func (m *memSyncStore) GetReadingSyncData(_ context.Context, userID domain.UserID, libraryID domain.LibraryID, since int64) (*transporthttp.ReadingSyncData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.data[m.key(userID, libraryID)]
	if !ok {
		return &transporthttp.ReadingSyncData{Cursor: since}, nil
	}

	res := &transporthttp.ReadingSyncData{Cursor: since}
	var maxSeq int64 = since
	for _, p := range d.Progress {
		if p.SyncSequence > since {
			res.Progress = append(res.Progress, p)
			if p.SyncSequence > maxSeq {
				maxSeq = p.SyncSequence
			}
		}
	}
	for _, b := range d.Bookmarks {
		if b.SyncSequence > since {
			res.Bookmarks = append(res.Bookmarks, b)
			if b.SyncSequence > maxSeq {
				maxSeq = b.SyncSequence
			}
		}
	}
	for _, h := range d.Highlights {
		if h.SyncSequence > since {
			res.Highlights = append(res.Highlights, h)
			if h.SyncSequence > maxSeq {
				maxSeq = h.SyncSequence
			}
		}
	}
	res.Cursor = maxSeq
	return res, nil
}

func (m *memSyncStore) GetProgressSyncSequence(_ context.Context, progressID domain.ReadingProgressID) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.progressSeqs[progressID]; ok {
		return s, nil
	}
	m.seqCounter++
	m.progressSeqs[progressID] = m.seqCounter
	return m.seqCounter, nil
}

type memProgressRepo struct {
	mu   sync.Mutex
	rows map[string]*domain.ReadingProgress
}

func newMemProgressRepo() *memProgressRepo {
	return &memProgressRepo{rows: make(map[string]*domain.ReadingProgress)}
}

func (m *memProgressRepo) key(userID domain.UserID, libraryID domain.LibraryID, workID domain.WorkID) string {
	return string(userID) + ":" + string(libraryID) + ":" + string(workID)
}

func (m *memProgressRepo) FindByWorkAndUser(_ context.Context, userID domain.UserID, libraryID domain.LibraryID, workID domain.WorkID) (*domain.ReadingProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.rows[m.key(userID, libraryID, workID)]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "reading progress not found"}
	}
	return p, nil
}

func (m *memProgressRepo) FindByWorkAndUserForUpdate(_ context.Context, userID domain.UserID, libraryID domain.LibraryID, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return m.FindByWorkAndUser(context.Background(), userID, libraryID, workID)
}

func (m *memProgressRepo) SaveForUser(_ context.Context, userID domain.UserID, libraryID domain.LibraryID, p *domain.ReadingProgress) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[m.key(userID, libraryID, p.WorkID())] = p
	return nil
}

func (m *memProgressRepo) FindByWork(_ context.Context, _ domain.WorkID) (*domain.ReadingProgress, error) {
	return nil, &domain.Error{Category: domain.Internal, Message: "use user-scoped variant"}
}

func (m *memProgressRepo) FindByWorkForUpdate(_ context.Context, _ domain.WorkID) (*domain.ReadingProgress, error) {
	return nil, &domain.Error{Category: domain.Internal, Message: "use user-scoped variant"}
}

func (m *memProgressRepo) Save(_ context.Context, _ *domain.ReadingProgress) error {
	return &domain.Error{Category: domain.Internal, Message: "use user-scoped variant"}
}

type testTransactor struct{}

func (testTransactor) InTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type testIDGen struct{}

func (testIDGen) NewID() string { return "test-id-123" }

func TestSyncReadingHandler(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()

	userA := domain.UserID("user-a")
	userB := domain.UserID("user-b")
	libA := domain.LibraryID("lib-a")

	devA, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devRepo.Save(context.Background(), devA)

	// Populate data for User A
	syncStore.data[syncStore.key(userA, libA)] = &transporthttp.ReadingSyncData{
		Progress: []transporthttp.SyncProgressItem{
			{WorkID: "work-1", Percentage: 0.5, Epoch: 1, DeviceID: "dev-a1", ObservedAt: now, SyncSequence: 10},
			{WorkID: "work-2", Percentage: 0.8, Epoch: 1, DeviceID: "dev-a1", ObservedAt: now, SyncSequence: 20},
		},
		Bookmarks: []transporthttp.SyncBookmarkItem{
			{ID: "bm-1", EditionID: "ed-1", Position: "cfi-1", Label: "Note", CreatedAt: now, SyncSequence: 25},
		},
		Highlights: []transporthttp.SyncHighlightItem{
			{ID: "hl-1", EditionID: "ed-1", StartPosition: "cfi-s", EndPosition: "cfi-e", CreatedAt: now, SyncSequence: 30},
		},
	}

	// Populate data for User B (IDOR isolation)
	syncStore.data[syncStore.key(userB, libA)] = &transporthttp.ReadingSyncData{
		Progress: []transporthttp.SyncProgressItem{
			{WorkID: "work-secret", Percentage: 0.99, Epoch: 5, DeviceID: "dev-b1", ObservedAt: now, SyncSequence: 40},
		},
	}

	handler := transporthttp.SyncReadingHandler(syncStore, devRepo, func() time.Time { return now })

	// 1. Initial sync (since=0 or omitted) -> returns all items for user A, max cursor = 30
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading", nil)
	ctxA := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
		UserID: userA,
		Role:   domain.RoleReader,
	})
	ctxA = transporthttp.WithActiveLibrary(ctxA, libA)
	ctxA = transporthttp.WithDevice(ctxA, devA)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctxA))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Cursor     int64                              `json:"cursor"`
		Progress   []transporthttp.SyncProgressItem   `json:"progress"`
		Bookmarks  []transporthttp.SyncBookmarkItem   `json:"bookmarks"`
		Highlights []transporthttp.SyncHighlightItem  `json:"highlights"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Cursor != 30 {
		t.Fatalf("expected cursor 30, got %d", resp.Cursor)
	}
	if len(resp.Progress) != 2 || len(resp.Bookmarks) != 1 || len(resp.Highlights) != 1 {
		t.Fatalf("expected (2, 1, 1) records, got (%d, %d, %d)", len(resp.Progress), len(resp.Bookmarks), len(resp.Highlights))
	}

	// IDOR check: User A must never see User B's work-secret
	for _, p := range resp.Progress {
		if p.WorkID == "work-secret" {
			t.Fatal("IDOR violation: user A received user B's reading progress")
		}
	}

	// Verify device cursor advanced on server
	devCheck, _ := devRepo.FindByID(context.Background(), "dev-a1")
	if devCheck.SyncCursor() != 30 {
		t.Fatalf("expected device cursor 30, got %d", devCheck.SyncCursor())
	}

	// 2. Incremental sync (since=20) -> returns only records with sync_sequence > 20 (bookmarks, highlights)
	reqInc := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading?since=20", nil)
	rrInc := httptest.NewRecorder()
	handler.ServeHTTP(rrInc, reqInc.WithContext(ctxA))

	if rrInc.Code != http.StatusOK {
		t.Fatalf("expected 200 on incremental sync, got %d", rrInc.Code)
	}
	var respInc struct {
		Cursor     int64                             `json:"cursor"`
		Progress   []transporthttp.SyncProgressItem  `json:"progress"`
		Bookmarks  []transporthttp.SyncBookmarkItem  `json:"bookmarks"`
		Highlights []transporthttp.SyncHighlightItem `json:"highlights"`
	}
	_ = json.Unmarshal(rrInc.Body.Bytes(), &respInc)
	if len(respInc.Progress) != 0 {
		t.Fatalf("expected 0 progress records for since=20, got %d", len(respInc.Progress))
	}
	if len(respInc.Bookmarks) != 1 || len(respInc.Highlights) != 1 {
		t.Fatalf("expected 1 bookmark and 1 highlight, got %d, %d", len(respInc.Bookmarks), len(respInc.Highlights))
	}

	// 3. Far future cursor (since=100) -> empty delta, cursor unchanged at 100
	reqFuture := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading?since=100", nil)
	rrFuture := httptest.NewRecorder()
	handler.ServeHTTP(rrFuture, reqFuture.WithContext(ctxA))

	var respFuture struct {
		Cursor int64 `json:"cursor"`
	}
	_ = json.Unmarshal(rrFuture.Body.Bytes(), &respFuture)
	if respFuture.Cursor != 100 {
		t.Fatalf("expected cursor to remain 100, got %d", respFuture.Cursor)
	}
}

func TestSyncProgressHandler(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()
	progressRepo := newMemProgressRepo()
	libEntries := &memLibraryEntries{
		works: map[domain.WorkID]domain.LibraryID{
			"work-1": "lib-a",
		},
	}

	userA := domain.UserID("user-a")
	libA := domain.LibraryID("lib-a")

	devA, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devRepo.Save(context.Background(), devA)

	handler := transporthttp.SyncProgressHandler(
		progressRepo,
		libEntries,
		syncStore,
		devRepo,
		testTransactor{},
		testIDGen{},
		func() time.Time { return now },
	)

	// 1. Initial report for work-1 -> Advanced, canonical created
	body1 := `{"workId":"work-1","percentage":0.5,"observedEpoch":0,"deviceId":"dev-a1","reportedAt":"2026-09-06T12:00:00Z"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(body1))
	ctxA := transporthttp.WithUser(req1.Context(), &transporthttp.AuthenticatedUser{
		UserID: userA,
		Role:   domain.RoleReader,
	})
	ctxA = transporthttp.WithActiveLibrary(ctxA, libA)
	ctxA = transporthttp.WithDevice(ctxA, devA)

	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1.WithContext(ctxA))

	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr1.Code, rr1.Body.String())
	}

	var resp1 struct {
		Outcome  string `json:"outcome"`
		Cursor   int64  `json:"cursor"`
		Progress struct {
			WorkID     string  `json:"workId"`
			Percentage float64 `json:"percentage"`
			Epoch      int64   `json:"epoch"`
		} `json:"progress"`
	}
	if err := json.Unmarshal(rr1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp1.Outcome != "advanced" {
		t.Fatalf("expected outcome advanced, got %s", resp1.Outcome)
	}
	if resp1.Progress.Percentage != 0.5 {
		t.Fatalf("expected percentage 0.5, got %v", resp1.Progress.Percentage)
	}
	if resp1.Cursor <= 0 {
		t.Fatalf("expected cursor > 0, got %d", resp1.Cursor)
	}

	// 2. Stale report -> rejected, canonical unchanged
	bodyStale := `{"workId":"work-1","percentage":0.4,"observedEpoch":0,"deviceId":"dev-a1","reportedAt":"2026-09-06T12:00:00Z"}`
	reqStale := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(bodyStale))
	rrStale := httptest.NewRecorder()
	handler.ServeHTTP(rrStale, reqStale.WithContext(ctxA))

	if rrStale.Code != http.StatusOK {
		t.Fatalf("expected 200 for stale report, got %d", rrStale.Code)
	}
	var respStale struct {
		Outcome  string `json:"outcome"`
		Progress struct {
			Percentage float64 `json:"percentage"`
		} `json:"progress"`
	}
	_ = json.Unmarshal(rrStale.Body.Bytes(), &respStale)
	if respStale.Outcome != "unchanged" && respStale.Outcome != "rejected" {
		t.Fatalf("expected unchanged or rejected, got %s", respStale.Outcome)
	}
	if respStale.Progress.Percentage != 0.5 {
		t.Fatalf("expected canonical percentage to remain 0.5, got %v", respStale.Progress.Percentage)
	}

	// 3. Work ID not in library -> 404 Not Found
	bodyNotFound := `{"workId":"work-unknown","percentage":0.5,"observedEpoch":0,"deviceId":"dev-a1"}`
	reqNotFound := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(bodyNotFound))
	rrNotFound := httptest.NewRecorder()
	handler.ServeHTTP(rrNotFound, reqNotFound.WithContext(ctxA))

	if rrNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for work not in library, got %d", rrNotFound.Code)
	}

	// 4. Invalid percentage (> 1.0) -> 400 (domain.InvalidInput maps to 400)
	bodyBadPct := `{"workId":"work-1","percentage":1.5,"observedEpoch":0,"deviceId":"dev-a1"}`
	reqBadPct := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(bodyBadPct))
	rrBadPct := httptest.NewRecorder()
	handler.ServeHTTP(rrBadPct, reqBadPct.WithContext(ctxA))

	if rrBadPct.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for percentage > 1.0, got %d", rrBadPct.Code)
	}
}

