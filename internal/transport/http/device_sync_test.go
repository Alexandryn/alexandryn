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

func (m *memPairedDevs) UpdateLastSeen(_ context.Context, id domain.DeviceID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "paired device not found or already revoked"}
	}
	return d.Touch(now)
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
			ID           string  `json:"id"`
			Label        string  `json:"label"`
			DeviceClass  string  `json:"deviceClass"`
			EnrolledVia  string  `json:"enrolledVia"`
			CreatedAt    string  `json:"createdAt"`
			LastSeenAt   string  `json:"lastSeenAt"`
			LastSyncedAt *string `json:"lastSyncedAt"`
			RevokedAt    *string `json:"revokedAt"`
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
	gotSince     int64 // last `since` GetReadingSyncData was called with
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
	m.gotSince = since
	d, ok := m.data[m.key(userID, libraryID)]
	if !ok {
		return &transporthttp.ReadingSyncData{Cursor: since}, nil
	}

	res := &transporthttp.ReadingSyncData{Cursor: since}
	maxSeq := since
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

func (m *memSyncStore) GetSyncSequenceCeiling(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	max := m.seqCounter
	for _, d := range m.data {
		for _, p := range d.Progress {
			if p.SyncSequence > max {
				max = p.SyncSequence
			}
		}
		for _, b := range d.Bookmarks {
			if b.SyncSequence > max {
				max = b.SyncSequence
			}
		}
		for _, h := range d.Highlights {
			if h.SyncSequence > max {
				max = h.SyncSequence
			}
		}
	}
	return max, nil
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
		Cursor     int64                             `json:"cursor"`
		Progress   []transporthttp.SyncProgressItem  `json:"progress"`
		Bookmarks  []transporthttp.SyncBookmarkItem  `json:"bookmarks"`
		Highlights []transporthttp.SyncHighlightItem `json:"highlights"`
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

	// 2. After step 1 the device cursor is 30. An incremental sync that
	//    asks with a stale since=20 is floored to the device's own cursor
	//    (#114), so it returns nothing — the device already has every row
	//    up to sequence 30.
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
	if len(respInc.Progress) != 0 || len(respInc.Bookmarks) != 0 || len(respInc.Highlights) != 0 {
		t.Fatalf("expected an empty delta for a stale since below the device cursor, got (%d, %d, %d)",
			len(respInc.Progress), len(respInc.Bookmarks), len(respInc.Highlights))
	}

	// 3. Far future cursor (since=100) -> empty delta, since clamped to cluster sequence ceiling (40), cursor = 40
	reqFuture := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading?since=100", nil)
	rrFuture := httptest.NewRecorder()
	handler.ServeHTTP(rrFuture, reqFuture.WithContext(ctxA))

	var respFuture struct {
		Cursor int64 `json:"cursor"`
	}
	_ = json.Unmarshal(rrFuture.Body.Bytes(), &respFuture)
	if respFuture.Cursor != 40 {
		t.Fatalf("expected cursor to be clamped to cluster ceiling (40), got %d", respFuture.Cursor)
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
		&memEditions{byID: map[domain.EditionID]*domain.Edition{}},
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

// TestSyncProgressHandler_PushCursorNotUsableAsPullCursor is the audit
// 0016 #90 regression. Device A's pull position is 5. Another device
// writes a highlight at sequence 6. Device A then pushes progress, which
// lands at sequence 7. The push response's `cursor` must NOT jump to 7 —
// if A persisted 7 as its next `since`, the highlight at 6 would never be
// delivered.
func TestSyncProgressHandler_PushCursorNotUsableAsPullCursor(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()
	progressRepo := newMemProgressRepo()
	libEntries := &memLibraryEntries{works: map[domain.WorkID]domain.LibraryID{"work-1": "lib-a"}}

	userA := domain.UserID("user-a")
	libA := domain.LibraryID("lib-a")
	devA, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devA.AdvanceCursor(5, now) // A has pulled up to sequence 5
	_ = devRepo.Save(context.Background(), devA)

	// A's own progress write will resolve to sequence 7 — sequence 6 was
	// another device's highlight, written in the gap.
	syncStore.progressSeqs["test-id-123"] = 7
	syncStore.seqCounter = 7

	handler := transporthttp.SyncProgressHandler(
		progressRepo, libEntries, &memEditions{byID: map[domain.EditionID]*domain.Edition{}}, syncStore, devRepo,
		testTransactor{}, testIDGen{}, func() time.Time { return now },
	)

	body := `{"workId":"work-1","percentage":0.5,"observedEpoch":0,"deviceId":"dev-a1","reportedAt":"2026-09-06T12:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(body))
	ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{UserID: userA, Role: domain.RoleReader})
	ctx = transporthttp.WithActiveLibrary(ctx, libA)
	ctx = transporthttp.WithDevice(ctx, devA)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Cursor int64 `json:"cursor"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Cursor != 5 {
		t.Fatalf("push response cursor = %d, want 5 (must not jump past the unpulled sequence 6)", resp.Cursor)
	}
	updated, _ := devRepo.FindByID(context.Background(), "dev-a1")
	if updated.SyncCursor() != 5 {
		t.Fatalf("device cursor advanced to %d, want it left at 5", updated.SyncCursor())
	}
}

func TestSyncMiddleware_TouchDoesNotClobberCursor(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	touchTime := now.Add(10 * time.Minute)
	repo := newMemPairedDevs()
	user := domain.UserID("u1")
	dev, _ := domain.NewPairedDevice("dev-1", user, "Phone", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = dev.AdvanceCursor(50, now)
	_ = repo.Save(context.Background(), dev)

	mw := transporthttp.SyncMiddleware(repo, func() time.Time { return touchTime })
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading", nil)
	req.Header.Set("X-Device-Id", "dev-1")
	ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{UserID: user, Role: domain.RoleReader})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	updated, err := repo.FindByID(context.Background(), "dev-1")
	if err != nil {
		t.Fatal(err)
	}
	if !updated.LastSeenAt().Equal(touchTime) {
		t.Fatalf("expected lastSeenAt to be %v, got %v", touchTime, updated.LastSeenAt())
	}
	if updated.SyncCursor() != 50 {
		t.Fatalf("expected syncCursor to remain 50, got %d", updated.SyncCursor())
	}
}

type nilDevRepo struct{}

func (nilDevRepo) FindByID(_ context.Context, _ domain.DeviceID) (*domain.PairedDevice, error) {
	return nil, nil // Common Go (nil, nil) not found convention
}
func (nilDevRepo) FindByOwner(_ context.Context, _ domain.UserID) ([]*domain.PairedDevice, error) {
	return nil, nil
}
func (nilDevRepo) FindByPairingSessionID(_ context.Context, _ domain.PairingSessionID) (*domain.PairedDevice, error) {
	return nil, nil
}
func (nilDevRepo) InsertProvisional(_ context.Context, _ domain.DeviceID, _ string, _ domain.DeviceClass, _ domain.EnrolledVia, _ domain.PairingSessionID, _ time.Time) error {
	return nil
}
func (nilDevRepo) AssignOwnerByPairingSession(_ context.Context, _ domain.PairingSessionID, _ domain.UserID) error {
	return nil
}
func (nilDevRepo) Save(_ context.Context, _ *domain.PairedDevice) error { return nil }
func (nilDevRepo) Revoke(_ context.Context, _ domain.DeviceID, _ time.Time) error {
	return nil
}
func (nilDevRepo) RevokeByPairingSessionID(_ context.Context, _ domain.PairingSessionID, _ time.Time) error {
	return nil
}
func (nilDevRepo) AdvanceCursor(_ context.Context, _ domain.DeviceID, _ int64, _ time.Time) error {
	return nil
}
func (nilDevRepo) UpdateLastSeen(_ context.Context, _ domain.DeviceID, _ time.Time) error {
	return nil
}

func TestSyncMiddleware_NilDeviceHandling(t *testing.T) {
	mw := transporthttp.SyncMiddleware(nilDevRepo{}, time.Now)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Sync route with (nil, nil) -> 401 without panic
	reqSync := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading", nil)
	reqSync.Header.Set("X-Device-Id", "unknown-dev")
	ctx := transporthttp.WithUser(reqSync.Context(), &transporthttp.AuthenticatedUser{UserID: "u1", Role: domain.RoleReader})
	rrSync := httptest.NewRecorder()
	handler.ServeHTTP(rrSync, reqSync.WithContext(ctx))

	if rrSync.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown device on sync route, got %d", rrSync.Code)
	}

	// Non-sync route with (nil, nil) -> passes through without panic
	reqNonSync := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	reqNonSync.Header.Set("X-Device-Id", "unknown-dev")
	rrNonSync := httptest.NewRecorder()
	handler.ServeHTTP(rrNonSync, reqNonSync.WithContext(ctx))

	if rrNonSync.Code != http.StatusOK {
		t.Fatalf("expected 200 on non-sync route, got %d", rrNonSync.Code)
	}
}

type spyLastSeenRepo struct {
	nilDevRepo
	dev         *domain.PairedDevice
	updateCalls int
}

func (s *spyLastSeenRepo) FindByID(_ context.Context, id domain.DeviceID) (*domain.PairedDevice, error) {
	if s.dev != nil && s.dev.ID() == id {
		return s.dev, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}

func (s *spyLastSeenRepo) UpdateLastSeen(_ context.Context, _ domain.DeviceID, _ time.Time) error {
	s.updateCalls++
	return nil
}

func TestSyncMiddleware_ThrottlesUpdateLastSeen(t *testing.T) {
	baseTime := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	currentTime := baseTime
	nowFn := func() time.Time { return currentTime }

	dev, err := domain.NewPairedDevice("dev-123", "u1", "Phone", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, baseTime)
	if err != nil {
		t.Fatalf("NewPairedDevice failed: %v", err)
	}

	repo := &spyLastSeenRepo{dev: dev}
	mw := transporthttp.SyncMiddleware(repo, nowFn)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	makeReq := func() *http.Response {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading", nil)
		req.Header.Set("X-Device-Id", "dev-123")
		ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{UserID: "u1", Role: domain.RoleReader})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.WithContext(ctx))
		return rr.Result()
	}

	// 1st request at baseTime: LastSeenAt == baseTime, diff == 0, so DB update is throttled
	// Wait, NewPairedDevice sets lastSeenAt = baseTime!
	// If currentTime is baseTime + 10s: diff < 60s -> no DB update
	currentTime = baseTime.Add(10 * time.Second)
	res1 := makeReq()
	if res1.StatusCode != http.StatusOK {
		t.Fatalf("request 1 failed: %d", res1.StatusCode)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected 0 calls when last_seen is fresh (diff=10s), got %d", repo.updateCalls)
	}

	// 2nd request at baseTime + 70s: diff >= 60s -> DB update occurs
	currentTime = baseTime.Add(70 * time.Second)
	res2 := makeReq()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("request 2 failed: %d", res2.StatusCode)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected 1 call after 70s elapsed, got %d", repo.updateCalls)
	}

	// 3rd request at baseTime + 80s: diff == 10s from last touch -> throttled
	currentTime = baseTime.Add(80 * time.Second)
	res3 := makeReq()
	if res3.StatusCode != http.StatusOK {
		t.Fatalf("request 3 failed: %d", res3.StatusCode)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected still 1 call (diff=10s from last update), got %d", repo.updateCalls)
	}
}

func TestRevokeDeviceHandler_NilDeviceHandling(t *testing.T) {
	handler := transporthttp.RevokeDeviceHandler(nilDevRepo{}, time.Now)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/unknown-dev", nil)
	req.SetPathValue("id", "unknown-dev")
	ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{UserID: "u1", Role: domain.RoleReader})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for (nil, nil) device revocation, got %d", rr.Code)
	}
}

func TestSyncProgressHandler_ClampsFutureReportedAt(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()
	progressRepo := newMemProgressRepo()
	libEntries := &memLibraryEntries{
		works: map[domain.WorkID]domain.LibraryID{
			"work-1": domain.DefaultLibraryID,
		},
	}

	userA := domain.UserID("user-a")
	devA1, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devRepo.Save(context.Background(), devA1)

	handler := transporthttp.SyncProgressHandler(progressRepo, libEntries, &memEditions{byID: map[domain.EditionID]*domain.Edition{}}, syncStore, devRepo, inlineTx{}, &seqID{}, func() time.Time { return now })
	ctxA := transporthttp.WithDevice(
		transporthttp.WithActiveLibrary(
			transporthttp.WithUser(context.Background(), &transporthttp.AuthenticatedUser{UserID: userA, Role: domain.RoleReader}),
			domain.DefaultLibraryID,
		),
		devA1,
	)

	// Send future timestamp (year 3000)
	body := `{"workId":"work-1","percentage":0.5,"observedEpoch":0,"deviceId":"dev-a1","reportedAt":"3000-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctxA))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	saved, _ := progressRepo.FindByWorkAndUser(context.Background(), userA, domain.DefaultLibraryID, "work-1")
	if saved.ObservedAt().After(now.Add(time.Minute)) {
		t.Fatalf("expected future reportedAt to be clamped to now (%v), got %v", now, saved.ObservedAt())
	}
}

func TestSyncReadingHandler_FutureSinceDoesNotAdvancePersistedCursor(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()

	userA := domain.UserID("user-a")
	devA1, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devRepo.Save(context.Background(), devA1)

	handler := transporthttp.SyncReadingHandler(syncStore, devRepo, func() time.Time { return now })
	ctxA := transporthttp.WithDevice(
		transporthttp.WithActiveLibrary(
			transporthttp.WithUser(context.Background(), &transporthttp.AuthenticatedUser{UserID: userA, Role: domain.RoleReader}),
			domain.DefaultLibraryID,
		),
		devA1,
	)

	// Send far future cursor when store has no updates
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading?since=9999999", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctxA))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	d, _ := devRepo.FindByID(context.Background(), "dev-a1")
	if d.SyncCursor() != 0 {
		t.Fatalf("persisted sync cursor should NOT advance on empty updates, got %d", d.SyncCursor())
	}

	var resp struct {
		Cursor int64 `json:"cursor"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Cursor != 0 {
		t.Fatalf("expected echoed cursor to be clamped to ceiling (0), got %d", resp.Cursor)
	}
}

// #114: SyncReadingHandler must floor `since` at the device's own pull
// cursor. A device that has already synced up to sequence N and then asks
// with since=0 must not make the store re-scan from the beginning.
func TestSyncReadingHandler_FloorsSinceAtDeviceCursor(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()

	userA := domain.UserID("user-a")
	devA, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devA.AdvanceCursor(15, now)
	_ = devRepo.Save(context.Background(), devA)
	// give the store a ceiling above the cursor so it is not clamped down
	syncStore.seqCounter = 100

	handler := transporthttp.SyncReadingHandler(syncStore, devRepo, func() time.Time { return now })
	ctxA := transporthttp.WithDevice(
		transporthttp.WithActiveLibrary(
			transporthttp.WithUser(context.Background(), &transporthttp.AuthenticatedUser{UserID: userA, Role: domain.RoleReader}),
			domain.DefaultLibraryID,
		),
		devA,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/reading?since=0", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctxA))

	if syncStore.gotSince != 15 {
		t.Fatalf("store queried with since=%d, want 15 (floored at device cursor)", syncStore.gotSince)
	}
}

// #111: SyncProgressHandler must reject a precise position whose edition
// belongs to a different work than the one being reported.
func TestSyncProgressHandler_RejectsForeignEditionInPrecisePosition(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()
	progressRepo := newMemProgressRepo()
	libEntries := &memLibraryEntries{works: map[domain.WorkID]domain.LibraryID{"work-1": "lib-a"}}

	lang, _ := domain.NewLanguage("en")
	// ed-other belongs to work-2, not the reported work-1.
	edOther, _ := domain.NewEdition("ed-other", "work-2", lang, nil, "", nil, nil)
	editions := &memEditions{byID: map[domain.EditionID]*domain.Edition{"ed-other": edOther}}

	userA := domain.UserID("user-a")
	libA := domain.LibraryID("lib-a")
	devA, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devRepo.Save(context.Background(), devA)

	handler := transporthttp.SyncProgressHandler(
		progressRepo, libEntries, editions, syncStore, devRepo,
		testTransactor{}, testIDGen{}, func() time.Time { return now },
	)

	body := `{"workId":"work-1","percentage":0.5,"observedEpoch":0,"deviceId":"dev-a1","precisePosition":{"editionId":"ed-other","cfi":"epubcfi(/6/4)"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(body))
	ctx := transporthttp.WithDevice(
		transporthttp.WithActiveLibrary(
			transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{UserID: userA, Role: domain.RoleReader}),
			libA,
		),
		devA,
	)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a precise position tagged with a foreign edition; body %s", rr.Code, rr.Body.String())
	}
}

// #109: a sync push with override=true applies a backward correction (a
// re-read) instead of rejecting it as a stale automatic report.
func TestSyncProgressHandler_OverrideAppliesBackwardCorrection(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	devRepo := newMemPairedDevs()
	syncStore := newMemSyncStore()
	progressRepo := newMemProgressRepo()
	libEntries := &memLibraryEntries{works: map[domain.WorkID]domain.LibraryID{"work-1": "lib-a"}}
	editions := &memEditions{byID: map[domain.EditionID]*domain.Edition{}}

	userA := domain.UserID("user-a")
	libA := domain.LibraryID("lib-a")
	devA, _ := domain.NewPairedDevice("dev-a1", userA, "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	_ = devRepo.Save(context.Background(), devA)

	handler := transporthttp.SyncProgressHandler(
		progressRepo, libEntries, editions, syncStore, devRepo,
		testTransactor{}, testIDGen{}, func() time.Time { return now },
	)
	ctxA := transporthttp.WithDevice(
		transporthttp.WithActiveLibrary(
			transporthttp.WithUser(context.Background(), &transporthttp.AuthenticatedUser{UserID: userA, Role: domain.RoleReader}),
			libA,
		),
		devA,
	)
	post := func(body string) map[string]any {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/progress", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.WithContext(ctxA))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}

	post(`{"workId":"work-1","percentage":0.8,"observedEpoch":0,"deviceId":"dev-a1"}`)

	// Plain backward report is rejected, canonical unchanged.
	plain := post(`{"workId":"work-1","percentage":0.3,"observedEpoch":0,"deviceId":"dev-a1"}`)
	if prog := plain["progress"].(map[string]any); prog["percentage"].(float64) != 0.8 {
		t.Fatalf("plain backward report changed canonical to %v, want 0.8", prog["percentage"])
	}

	// Override backward report wins.
	ov := post(`{"workId":"work-1","percentage":0.3,"observedEpoch":0,"override":true,"deviceId":"dev-a1"}`)
	if ov["outcome"] != "overridden" {
		t.Fatalf("outcome = %v, want overridden", ov["outcome"])
	}
	prog := ov["progress"].(map[string]any)
	if prog["percentage"].(float64) != 0.3 {
		t.Fatalf("override canonical = %v, want 0.3", prog["percentage"])
	}
	if prog["epoch"].(float64) != 1 {
		t.Fatalf("override epoch = %v, want 1", prog["epoch"])
	}
}
