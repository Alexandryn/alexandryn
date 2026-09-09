package http_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type memPairingSessions struct {
	mu       sync.Mutex
	byID     map[domain.PairingSessionID]*domain.PairingSession
	indexKey []byte
}

func newMemPairingSessions() *memPairingSessions {
	return &memPairingSessions{
		byID:     make(map[domain.PairingSessionID]*domain.PairingSession),
		indexKey: []byte("test-index-key-32-bytes-long!!!"),
	}
}

func (m *memPairingSessions) CodeIndex(code domain.PairingCode) []byte {
	h := hmac.New(sha256.New, m.indexKey)
	h.Write([]byte(code.Normalized()))
	return h.Sum(nil)
}

func (m *memPairingSessions) Save(_ context.Context, s *domain.PairingSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[s.ID()] = s
	return nil
}

func (m *memPairingSessions) SaveWithInitiatorIP(_ context.Context, s *domain.PairingSession, _ string) error {
	return m.Save(context.Background(), s)
}

func (m *memPairingSessions) FindByID(_ context.Context, id domain.PairingSessionID) (*domain.PairingSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.byID[id]; ok {
		return s, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "pairing session not found"}
}

func (m *memPairingSessions) FindByCodeIndex(_ context.Context, codeIndex []byte) (*domain.PairingSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.byID {
		if bytes.Equal(m.CodeIndex(s.Code()), codeIndex) {
			return s, nil
		}
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "pairing session not found"}
}

func (m *memPairingSessions) FindPendingByCodeIndexForUpdate(_ context.Context, codeIndex []byte, now time.Time) (*domain.PairingSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.byID {
		if bytes.Equal(m.CodeIndex(s.Code()), codeIndex) && s.State() == domain.PairingPending && s.ExpiresAt().After(now) {
			return s, nil
		}
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "pairing session not found"}
}

func (m *memPairingSessions) Delete(_ context.Context, id domain.PairingSessionID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; !ok {
		return &domain.Error{Category: domain.NotFound, Message: "pairing session not found"}
	}
	delete(m.byID, id)
	return nil
}

type fakeIDGen struct {
	val string
}

func (f *fakeIDGen) NewID() string {
	return f.val
}

func adminUser() *transporthttp.AuthenticatedUser {
	return &transporthttp.AuthenticatedUser{
		UserID:    "user-admin-1",
		Username:  "admin",
		Role:      domain.RoleAdmin,
		Libraries: []domain.LibraryID{domain.DefaultLibraryID},
	}
}

func readerUser() *transporthttp.AuthenticatedUser {
	return &transporthttp.AuthenticatedUser{
		UserID:    "user-reader-1",
		Username:  "reader",
		Role:      domain.RoleReader,
		Libraries: []domain.LibraryID{domain.DefaultLibraryID},
	}
}

func fixedCodeGen(codeStr string) func() (domain.PairingCode, error) {
	return func() (domain.PairingCode, error) {
		return domain.NewPairingCode(codeStr)
	}
}

func TestPairInitiate_Success(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }
	idGen := &fakeIDGen{val: "ps-fixed-123"}
	codeGen := fixedCodeGen("ABCD-EFGH")

	handler := transporthttp.InitiatePairingHandler(
		sessionRepo,
		codeGen,
		"", // no secret required
		"192.168.1.50:8080",
		"http",
		idGen,
		nowFn,
		nil,
	)

	req := httptest.NewRequest("POST", "/api/v1/network/pair/initiate", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	ctx := transporthttp.WithUser(req.Context(), adminUser())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var resp transporthttp.InitiatePairingResponseWire
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.PairingID != "ps-fixed-123" {
		t.Errorf("pairingId = %q, want ps-fixed-123", resp.PairingID)
	}
	if resp.Code != "ABCD-EFGH" {
		t.Errorf("code = %q, want ABCD-EFGH", resp.Code)
	}
	if resp.Address != "192.168.1.50:8080" {
		t.Errorf("address = %q, want 192.168.1.50:8080", resp.Address)
	}
	expectedPayload := "http://192.168.1.50:8080/connect?c=ABCD-EFGH"
	if resp.Payload != expectedPayload {
		t.Errorf("payload = %q, want %q", resp.Payload, expectedPayload)
	}
	expectedExpiry := now.Add(5 * time.Minute)
	if !resp.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expiresAt = %v, want %v", resp.ExpiresAt, expectedExpiry)
	}

	// Verify session persisted
	saved, err := sessionRepo.FindByID(context.Background(), "ps-fixed-123")
	if err != nil {
		t.Fatalf("session not found in repo: %v", err)
	}
	if saved.InitiatedBy() != "user-admin-1" {
		t.Errorf("saved InitiatedBy = %q, want user-admin-1", saved.InitiatedBy())
	}
	if saved.State() != domain.PairingPending {
		t.Errorf("saved State = %q, want pending", saved.State())
	}
}

func TestPairInitiate_SecretRequirement(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Now().UTC()
	idGen := &fakeIDGen{val: "ps-secret-1"}
	codeGen := fixedCodeGen("1234-5678")

	handler := transporthttp.InitiatePairingHandler(
		sessionRepo,
		codeGen,
		"top-secret-phrase",
		"localhost:8080",
		"http",
		idGen,
		func() time.Time { return now },
		nil,
	)

	// 1. Missing secret -> 403
	req := httptest.NewRequest("POST", "/api/v1/network/pair/initiate", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("missing secret: status = %d, want 403", rec.Code)
	}

	// 2. Wrong secret -> 403
	bodyWrong, _ := json.Marshal(map[string]string{"secret": "wrong-secret"})
	req = httptest.NewRequest("POST", "/api/v1/network/pair/initiate", bytes.NewReader(bodyWrong))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("wrong secret: status = %d, want 403", rec.Code)
	}

	// 3. Matching secret -> 201
	bodyRight, _ := json.Marshal(map[string]string{"secret": "top-secret-phrase"})
	req = httptest.NewRequest("POST", "/api/v1/network/pair/initiate", bytes.NewReader(bodyRight))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("matching secret: status = %d, want 201", rec.Code)
	}
}

func TestPairInitiate_RoleEnforcement(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Now().UTC()
	idGen := &fakeIDGen{val: "ps-role-1"}
	codeGen := fixedCodeGen("1234-5678")

	handler := transporthttp.InitiatePairingHandler(
		sessionRepo,
		codeGen,
		"",
		"localhost:8080",
		"http",
		idGen,
		func() time.Time { return now },
		nil,
	)

	// Reader user -> 403
	req := httptest.NewRequest("POST", "/api/v1/network/pair/initiate", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(transporthttp.WithUser(req.Context(), readerUser()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("reader user status = %d, want 403", rec.Code)
	}

	// No user in context -> 401
	req = httptest.NewRequest("POST", "/api/v1/network/pair/initiate", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d, want 401", rec.Code)
	}
}

func TestPairInitiate_ValidationAndBodyCap(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Now().UTC()
	idGen := &fakeIDGen{val: "ps-cap-1"}
	codeGen := fixedCodeGen("1234-5678")

	handler := transporthttp.InitiatePairingHandler(
		sessionRepo,
		codeGen,
		"",
		"localhost:8080",
		"http",
		idGen,
		func() time.Time { return now },
		nil,
	)

	// Non-JSON Content-Type -> 415
	req := httptest.NewRequest("POST", "/api/v1/network/pair/initiate", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "text/plain")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON Content-Type status = %d, want 415", rec.Code)
	}

	// Body > 4 KiB -> 400
	hugeBody := strings.Repeat("a", 5*1024)
	req = httptest.NewRequest("POST", "/api/v1/network/pair/initiate", strings.NewReader(hugeBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("over-large body status = %d, want 400", rec.Code)
	}
}

type fakePairingVerifier struct {
	sessionRepo   *memPairingSessions
	injectedErr   error
	capturedDevID domain.DeviceID
	capturedLabel string
	capturedClass domain.DeviceClass
}

func (f *fakePairingVerifier) VerifyAndConsume(
	ctx context.Context,
	code domain.PairingCode,
	deviceID domain.DeviceID,
	label string,
	deviceClass domain.DeviceClass,
	now time.Time,
) (*domain.PairingSession, error) {
	if f.injectedErr != nil {
		return nil, f.injectedErr
	}
	f.capturedDevID = deviceID
	f.capturedLabel = label
	f.capturedClass = deviceClass

	idx := f.sessionRepo.CodeIndex(code)
	session, err := f.sessionRepo.FindPendingByCodeIndexForUpdate(ctx, idx, now)
	if err != nil {
		return nil, err
	}
	if err := session.Verify(now, code, deviceID); err != nil {
		return nil, err
	}
	if err := session.Consume(now); err != nil {
		return nil, err
	}
	_ = f.sessionRepo.Save(ctx, session)
	return session, nil
}

func TestPairVerify_Success(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }
	idGen := &fakeIDGen{val: "jti-grant-1"}

	code, _ := domain.NewPairingCode("2345-6789")
	session, _ := domain.NewPairingSession("ps-verify-1", "user-admin-1", code, 5*time.Minute, now)
	_ = sessionRepo.Save(context.Background(), session)

	verifier := &fakePairingVerifier{sessionRepo: sessionRepo}
	grantSigner := auth.NewEnrolmentGrantSigner([]byte("enrolment-sub-key-32-bytes-long!"), "alexandryn", idGen)

	handler := transporthttp.VerifyPairingHandler(
		verifier,
		grantSigner,
		"192.168.1.50:8080",
		"library.local",
		idGen,
		nowFn,
		nil,
	)

	body, _ := json.Marshal(map[string]string{
		"code":  "2345-6789",
		"label": "My iPhone",
	})
	req := httptest.NewRequest("POST", "/api/v1/network/pair/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp transporthttp.VerifyPairingResponseWire
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Address != "192.168.1.50:8080" {
		t.Errorf("address = %q, want 192.168.1.50:8080", resp.Address)
	}
	if resp.HostName != "library.local" {
		t.Errorf("hostName = %q, want library.local", resp.HostName)
	}
	if resp.EnrolmentGrant == "" {
		t.Fatal("expected non-empty enrolment grant")
	}

	// Verify the grant is cryptographically valid and carries the session ID
	claims, err := grantSigner.Verify(resp.EnrolmentGrant, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("grant verification failed: %v", err)
	}
	if claims.SessionID != "ps-verify-1" {
		t.Errorf("grant SessionID = %q, want ps-verify-1", claims.SessionID)
	}

	// Verify device class was derived from User-Agent
	if verifier.capturedClass != domain.DeviceClassPhone {
		t.Errorf("captured class = %q, want phone", verifier.capturedClass)
	}
	if verifier.capturedLabel != "My iPhone" {
		t.Errorf("captured label = %q, want My iPhone", verifier.capturedLabel)
	}
}

func TestPairVerify_MalformedCode(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Now().UTC()
	verifier := &fakePairingVerifier{sessionRepo: sessionRepo}
	grantSigner := auth.NewEnrolmentGrantSigner([]byte("enrolment-sub-key-32-bytes-long!"), "alexandryn", &fakeIDGen{val: "1"})

	handler := transporthttp.VerifyPairingHandler(
		verifier,
		grantSigner,
		"localhost:8080",
		"library.local",
		&fakeIDGen{val: "1"},
		func() time.Time { return now },
		nil,
	)

	// Code not 8 Crockford chars -> 400 InvalidInput
	body, _ := json.Marshal(map[string]string{"code": "tooshort"})
	req := httptest.NewRequest("POST", "/api/v1/network/pair/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPairVerify_ByteIdenticalGeneric404(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }
	idGen := &fakeIDGen{val: "1"}

	// Session expired
	codeExp, _ := domain.NewPairingCode("EXPR-1234")
	sessionExp, _ := domain.NewPairingSession("ps-exp", "user-admin-1", codeExp, 5*time.Minute, now.Add(-10*time.Minute))
	_ = sessionRepo.Save(context.Background(), sessionExp)

	verifier := &fakePairingVerifier{sessionRepo: sessionRepo}
	grantSigner := auth.NewEnrolmentGrantSigner([]byte("enrolment-sub-key-32-bytes-long!"), "alexandryn", idGen)

	handler := transporthttp.VerifyPairingHandler(
		verifier,
		grantSigner,
		"localhost:8080",
		"library.local",
		idGen,
		nowFn,
		nil,
	)

	// 1. Never-existed code (valid shape)
	body1, _ := json.Marshal(map[string]string{"code": "9999-9999"})
	req1 := httptest.NewRequest("POST", "/api/v1/network/pair/verify", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusNotFound {
		t.Fatalf("never-existed: status = %d, want 404", rec1.Code)
	}

	// 2. Expired code
	body2, _ := json.Marshal(map[string]string{"code": "EXPR-1234"})
	req2 := httptest.NewRequest("POST", "/api/v1/network/pair/verify", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expired: status = %d, want 404", rec2.Code)
	}

	// Decode both error bodies to compare shape & message
	var err1, err2 map[string]any
	_ = json.Unmarshal(rec1.Body.Bytes(), &err1)
	_ = json.Unmarshal(rec2.Body.Bytes(), &err2)

	if err1["code"] != "NotFound" || err2["code"] != "NotFound" {
		t.Errorf("error codes: %v vs %v", err1["code"], err2["code"])
	}
	if err1["message"] != "pairing code not recognised" || err2["message"] != "pairing code not recognised" {
		t.Errorf("messages: %q vs %q, want 'pairing code not recognised'", err1["message"], err2["message"])
	}
}

func TestPairQR_Success(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)

	code, _ := domain.NewPairingCode("3456-789A")
	session, _ := domain.NewPairingSession("ps-qr-1", "user-admin-1", code, 5*time.Minute, now)
	_ = sessionRepo.Save(context.Background(), session)

	handler := transporthttp.PairingQRHandler(
		sessionRepo,
		"192.168.1.50:8080",
		"http",
		nil,
	)

	req := httptest.NewRequest("GET", "/api/v1/network/pair/ps-qr-1/qr", nil)
	req.SetPathValue("id", "ps-qr-1")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp transporthttp.PairingQRResponseWire
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Code != "3456-789A" {
		t.Errorf("code = %q, want 3456-789A", resp.Code)
	}
	if resp.Payload != "http://192.168.1.50:8080/connect?c=3456-789A" {
		t.Errorf("payload = %q", resp.Payload)
	}
	if resp.State != "pending" {
		t.Errorf("state = %q, want pending", resp.State)
	}
}

func TestPairQR_CrossAdminEnumerationPrevention(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Now().UTC()

	code, _ := domain.NewPairingCode("3456-789A")
	session, _ := domain.NewPairingSession("ps-other-admin", "user-admin-OTHER", code, 5*time.Minute, now)
	_ = sessionRepo.Save(context.Background(), session)

	handler := transporthttp.PairingQRHandler(
		sessionRepo,
		"192.168.1.50:8080",
		"http",
		nil,
	)

	// Admin 1 tries to access Admin OTHER's session -> MUST be 404, NOT 403
	req := httptest.NewRequest("GET", "/api/v1/network/pair/ps-other-admin/qr", nil)
	req.SetPathValue("id", "ps-other-admin")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-admin QR access: status = %d, want 404", rec.Code)
	}
}

func TestPairQR_TerminalStates(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	now := time.Now().UTC()

	code, _ := domain.NewPairingCode("4567-89AB")
	session, _ := domain.NewPairingSession("ps-term-1", "user-admin-1", code, 5*time.Minute, now)
	_ = session.Verify(now.Add(time.Minute), code, "dev-1")
	_ = session.Consume(now.Add(2 * time.Minute))
	_ = sessionRepo.Save(context.Background(), session)

	handler := transporthttp.PairingQRHandler(
		sessionRepo,
		"192.168.1.50:8080",
		"http",
		nil,
	)

	req := httptest.NewRequest("GET", "/api/v1/network/pair/ps-term-1/qr", nil)
	req.SetPathValue("id", "ps-term-1")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp transporthttp.PairingQRResponseWire
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.State != "consumed" {
		t.Errorf("state = %q, want consumed", resp.State)
	}
	if resp.Code != "" || resp.Payload != "" {
		t.Errorf("terminal session must return empty code & payload, got code=%q payload=%q", resp.Code, resp.Payload)
	}
}

func TestNetworkStatus_ReaderScope(t *testing.T) {
	info := transporthttp.NetworkInfo{
		Reachability: "lan",
		TLSMode:      "none",
		BindAddress:  "192.168.1.50:8080",
		HostName:     "library.local",
		Addresses: []transporthttp.NetworkAddressWire{
			{Scope: "lan", URL: "http://192.168.1.50:8080"},
			{Scope: "loopback", URL: "http://127.0.0.1:8080"},
		},
	}

	handler := transporthttp.NetworkStatusHandler(func() transporthttp.NetworkInfo { return info }, nil)

	req := httptest.NewRequest("GET", "/api/v1/network/status", nil)
	req.Host = "attacker-domain.evil.com" // Host header must NOT be reflected
	req = req.WithContext(transporthttp.WithUser(req.Context(), readerUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var raw map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if raw["reachability"] != "lan" {
		t.Errorf("reachability = %v, want lan", raw["reachability"])
	}
	if raw["tlsMode"] != "none" {
		t.Errorf("tlsMode = %v, want none", raw["tlsMode"])
	}
	if raw["authRequired"] != true {
		t.Errorf("authRequired = %v, want true", raw["authRequired"])
	}
	if raw["address"] == "http://attacker-domain.evil.com" {
		t.Fatal("address must never reflect client Host header")
	}

	// Reader must NOT see admin-scoped fields
	if _, ok := raw["addresses"]; ok {
		t.Error("reader must not see addresses list")
	}
	if _, ok := raw["hostName"]; ok {
		t.Error("reader must not see hostName")
	}
	if _, ok := raw["acmeDomain"]; ok {
		t.Error("reader must not see acmeDomain")
	}
}

// TestNetworkStatus_HostMatchIsExactNotSubstring reproduces the bug where
// matching the client's Host header against known server addresses used
// strings.Contains(addr.URL, r.Host) — a substring match, not host[:port]
// equality. "10.0.0.15:8443" is a literal substring of "110.0.0.15:8443",
// so a request actually reaching the server via 110.0.0.15 could match and
// echo back the wrong configured address entry.
func TestNetworkStatus_HostMatchIsExactNotSubstring(t *testing.T) {
	info := transporthttp.NetworkInfo{
		Reachability: "lan",
		TLSMode:      "none",
		BindAddress:  "0.0.0.0:8443",
		HostName:     "library.local",
		Addresses: []transporthttp.NetworkAddressWire{
			{Scope: "public", URL: "http://110.0.0.15:8443"},
			{Scope: "lan", URL: "http://10.0.0.15:8443"},
		},
	}

	handler := transporthttp.NetworkStatusHandler(func() transporthttp.NetworkInfo { return info }, nil)

	req := httptest.NewRequest("GET", "/api/v1/network/status", nil)
	req.Host = "10.0.0.15:8443"
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var raw map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if raw["address"] != "http://10.0.0.15:8443" {
		t.Fatalf("address = %v, want the exact-matching entry http://10.0.0.15:8443 (substring match picked the wrong one)", raw["address"])
	}
}

func TestNetworkStatus_AdminScope(t *testing.T) {
	info := transporthttp.NetworkInfo{
		Reachability: "public",
		TLSMode:      "acme",
		BindAddress:  "0.0.0.0:443",
		ACMEDomain:   "books.example.com",
		HostName:     "library.local",
		Addresses: []transporthttp.NetworkAddressWire{
			{Scope: "public", URL: "https://books.example.com"},
		},
	}

	handler := transporthttp.NetworkStatusHandler(func() transporthttp.NetworkInfo { return info }, nil)

	req := httptest.NewRequest("GET", "/api/v1/network/status", nil)
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp transporthttp.NetworkStatusAdminWire
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Reachability != "public" || resp.TLSMode != "acme" || !resp.AuthRequired {
		t.Errorf("resp base fields unexpected: %+v", resp)
	}
	if resp.ACMEDomain != "books.example.com" {
		t.Errorf("acmeDomain = %q, want books.example.com", resp.ACMEDomain)
	}
	if resp.HostName != "library.local" {
		t.Errorf("hostName = %q, want library.local", resp.HostName)
	}
	if len(resp.Addresses) != 1 {
		t.Errorf("addresses len = %d, want 1", len(resp.Addresses))
	}
}

func TestNetworkStatus_ScrubbingAudit(t *testing.T) {
	// Canary secrets that MUST NEVER appear in any status response (FR-4)
	canaries := []string{
		"/home/operator/alexandryn/data",
		"C:\\Users\\Operator\\cert.pem",
		"/etc/letsencrypt/live/key.pem",
		"operator-private@example.com",
		"postgres://user:supersecretpass@db.local:5432/alexandryn",
		"pairing-secret-ultra-classified",
		"hkdf-subkey-bytes-must-never-leak",
		"sql: connection refused to driver",
	}

	info := transporthttp.NetworkInfo{
		Reachability: "lan",
		TLSMode:      "none",
		BindAddress:  "192.168.1.10:8080",
		HostName:     "alexandryn.local",
		Addresses: []transporthttp.NetworkAddressWire{
			{Scope: "lan", URL: "http://192.168.1.10:8080"},
		},
	}

	handler := transporthttp.NetworkStatusHandler(func() transporthttp.NetworkInfo { return info }, nil)

	// Check reader response
	reqReader := httptest.NewRequest("GET", "/api/v1/network/status", nil)
	reqReader = reqReader.WithContext(transporthttp.WithUser(reqReader.Context(), readerUser()))
	recReader := httptest.NewRecorder()
	handler.ServeHTTP(recReader, reqReader)
	readerBody := recReader.Body.String()

	for _, canary := range canaries {
		if strings.Contains(readerBody, canary) {
			t.Errorf("reader response leaked canary secret %q", canary)
		}
	}

	// Check admin response
	reqAdmin := httptest.NewRequest("GET", "/api/v1/network/status", nil)
	reqAdmin = reqAdmin.WithContext(transporthttp.WithUser(reqAdmin.Context(), adminUser()))
	recAdmin := httptest.NewRecorder()
	handler.ServeHTTP(recAdmin, reqAdmin)
	adminBody := recAdmin.Body.String()

	for _, canary := range canaries {
		if strings.Contains(adminBody, canary) {
			t.Errorf("admin response leaked canary secret %q", canary)
		}
	}
}

type memNetworkSettings struct {
	mu       sync.Mutex
	settings *domain.NetworkSettings
}

func newMemNetworkSettings() *memNetworkSettings {
	return &memNetworkSettings{
		settings: &domain.NetworkSettings{
			HostName:           "alexandryn.local",
			RememberDeviceDays: 30,
			UpdatedAt:          time.Now().UTC(),
		},
	}
}

func (m *memNetworkSettings) Get(_ context.Context) (*domain.NetworkSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &domain.NetworkSettings{
		HostName:           m.settings.HostName,
		RememberDeviceDays: m.settings.RememberDeviceDays,
		UpdatedAt:          m.settings.UpdatedAt,
	}, nil
}

func (m *memNetworkSettings) Upsert(_ context.Context, s *domain.NetworkSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings = &domain.NetworkSettings{
		HostName:           s.HostName,
		RememberDeviceDays: s.RememberDeviceDays,
		UpdatedAt:          s.UpdatedAt,
	}
	return nil
}

func TestNetworkSettings_PatchSuccess(t *testing.T) {
	repo := newMemNetworkSettings()
	now := time.Date(2026, 9, 5, 14, 0, 0, 0, time.UTC)
	handler := transporthttp.UpdateNetworkSettingsHandler(repo, func() time.Time { return now }, nil)

	body, _ := json.Marshal(map[string]any{
		"hostName":           "my-library.local",
		"rememberDeviceDays": 60,
	})
	req := httptest.NewRequest("PATCH", "/api/v1/network/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp transporthttp.NetworkSettingsResponseWire
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.HostName != "my-library.local" {
		t.Errorf("hostName = %q, want my-library.local", resp.HostName)
	}
	if resp.RememberDeviceDays != 60 {
		t.Errorf("rememberDeviceDays = %d, want 60", resp.RememberDeviceDays)
	}
	if !resp.UpdatedAt.Equal(now) {
		t.Errorf("updatedAt = %v, want %v", resp.UpdatedAt, now)
	}

	// Verify persisted
	persisted, _ := repo.Get(context.Background())
	if persisted.HostName != "my-library.local" || persisted.RememberDeviceDays != 60 {
		t.Errorf("persisted settings mismatch: %+v", persisted)
	}
}

func TestNetworkSettings_PartialPatch(t *testing.T) {
	repo := newMemNetworkSettings()
	now := time.Now().UTC()
	handler := transporthttp.UpdateNetworkSettingsHandler(repo, func() time.Time { return now }, nil)

	// Only hostName
	body1, _ := json.Marshal(map[string]any{"hostName": "newhost"})
	req1 := httptest.NewRequest("PATCH", "/api/v1/network/settings", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(transporthttp.WithUser(req1.Context(), adminUser()))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("patch hostName only: status = %d", rec1.Code)
	}

	persisted1, _ := repo.Get(context.Background())
	if persisted1.HostName != "newhost" || persisted1.RememberDeviceDays != 30 {
		t.Errorf("expected hostName changed to newhost and rememberDeviceDays retained 30, got: %+v", persisted1)
	}

	// Only rememberDeviceDays
	body2, _ := json.Marshal(map[string]any{"rememberDeviceDays": 14})
	req2 := httptest.NewRequest("PATCH", "/api/v1/network/settings", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(transporthttp.WithUser(req2.Context(), adminUser()))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("patch rememberDeviceDays only: status = %d", rec2.Code)
	}

	persisted2, _ := repo.Get(context.Background())
	if persisted2.HostName != "newhost" || persisted2.RememberDeviceDays != 14 {
		t.Errorf("expected hostName retained newhost and rememberDeviceDays changed to 14, got: %+v", persisted2)
	}
}

func TestNetworkSettings_Validation(t *testing.T) {
	repo := newMemNetworkSettings()
	handler := transporthttp.UpdateNetworkSettingsHandler(repo, time.Now, nil)

	invalidCases := []map[string]any{
		{"hostName": "-leading-hyphen"},
		{"hostName": "trailing-hyphen-"},
		{"hostName": "Upper-Case"},
		{"hostName": "has space"},
		{"hostName": strings.Repeat("a", 64)},
		{"rememberDeviceDays": 0},
		{"rememberDeviceDays": -5},
		{"rememberDeviceDays": 91},
	}

	for _, tc := range invalidCases {
		body, _ := json.Marshal(tc)
		req := httptest.NewRequest("PATCH", "/api/v1/network/settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("case %+v: status = %d, want 400", tc, rec.Code)
		}
	}
}

func TestNetworkSettings_ForbiddenKeys(t *testing.T) {
	repo := newMemNetworkSettings()
	handler := transporthttp.UpdateNetworkSettingsHandler(repo, time.Now, nil)

	forbiddenCases := []struct {
		key      string
		body     map[string]any
		errMatch string
	}{
		{
			key:      "bindAddress",
			body:     map[string]any{"bindAddress": "0.0.0.0:8080"},
			errMatch: "bind address",
		},
		{
			key:      "tlsCertFile",
			body:     map[string]any{"tlsCertFile": "/path/to/cert"},
			errMatch: "TLS",
		},
		{
			key:      "authRequired",
			body:     map[string]any{"authRequired": false},
			errMatch: "authentication",
		},
		{
			key:      "unknownKey",
			body:     map[string]any{"unknownKey": "foo"},
			errMatch: "unknownKey",
		},
	}

	for _, tc := range forbiddenCases {
		body, _ := json.Marshal(tc.body)
		req := httptest.NewRequest("PATCH", "/api/v1/network/settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("key %q: status = %d, want 400", tc.key, rec.Code)
		}
		bodyStr := strings.ToLower(rec.Body.String())
		if !strings.Contains(bodyStr, strings.ToLower(tc.errMatch)) {
			t.Errorf("key %q: error body %q did not contain %q", tc.key, rec.Body.String(), tc.errMatch)
		}
	}
}

// provisionalRecord mirrors a paired_devices row with owner_id still NULL —
// the same state the Postgres repository can't rehydrate into a
// domain.PairedDevice (Owner is a mandatory field), so it's tracked here
// separately from byID rather than papered over with a placeholder owner.
type provisionalRecord struct {
	label     string
	class     domain.DeviceClass
	via       domain.EnrolledVia
	createdAt time.Time
	revokedAt *time.Time
}

type memPairedDevices struct {
	mu          sync.Mutex
	byID        map[domain.DeviceID]*domain.PairedDevice
	bySess      map[domain.PairingSessionID]domain.DeviceID
	provisional map[domain.DeviceID]*provisionalRecord
}

func newMemPairedDevices() *memPairedDevices {
	return &memPairedDevices{
		byID:        make(map[domain.DeviceID]*domain.PairedDevice),
		bySess:      make(map[domain.PairingSessionID]domain.DeviceID),
		provisional: make(map[domain.DeviceID]*provisionalRecord),
	}
}

func (m *memPairedDevices) Save(_ context.Context, d *domain.PairedDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[d.ID()] = d
	return nil
}

func (m *memPairedDevices) InsertProvisional(_ context.Context, id domain.DeviceID, label string, class domain.DeviceClass, via domain.EnrolledVia, sessID domain.PairingSessionID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bySess[sessID] = id
	m.provisional[id] = &provisionalRecord{label: label, class: class, via: via, createdAt: now}
	return nil
}

func (m *memPairedDevices) AssignOwnerByPairingSession(_ context.Context, sessID domain.PairingSessionID, owner domain.UserID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	devID, ok := m.bySess[sessID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "device not found"}
	}
	p, ok := m.provisional[devID]
	if !ok {
		// Already owned, or never provisioned: mirrors "owner_id IS NULL" matching 0 rows.
		return &domain.Error{Category: domain.NotFound, Message: "provisional paired device not found for session"}
	}
	if p.revokedAt != nil {
		// Mirrors the "AND revoked_at IS NULL" guard: a device revoked while still
		// provisional must not be claimable by a later login (security fix).
		return &domain.Error{Category: domain.NotFound, Message: "provisional paired device not found for session"}
	}
	dev, _ := domain.NewPairedDevice(devID, owner, p.label, p.class, p.via, p.createdAt)
	m.byID[devID] = dev
	delete(m.provisional, devID)
	return nil
}

func (m *memPairedDevices) FindByID(_ context.Context, id domain.DeviceID) (*domain.PairedDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.byID[id]; ok {
		return d, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "device not found"}
}

func (m *memPairedDevices) FindByPairingSessionID(_ context.Context, sessID domain.PairingSessionID) (*domain.PairedDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	devID, ok := m.bySess[sessID]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "device not found"}
	}
	if d, ok := m.byID[devID]; ok {
		return d, nil
	}
	// Provisional (owner_id NULL) — matches scanDevice's real-repo behaviour.
	return nil, &domain.Error{Category: domain.NotFound, Message: "paired device is provisional (unassigned owner)"}
}

func (m *memPairedDevices) FindByOwner(_ context.Context, owner domain.UserID) ([]*domain.PairedDevice, error) {
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

func (m *memPairedDevices) Revoke(_ context.Context, id domain.DeviceID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "device not found"}
	}
	if err := d.Revoke(now); err != nil {
		return err
	}
	return nil
}

// RevokeByPairingSessionID revokes the device tied to a pairing session
// regardless of whether it has an assigned owner yet — the fix for the bug
// where an admin's revoke silently no-op'd on a still-provisional device.
func (m *memPairedDevices) RevokeByPairingSessionID(_ context.Context, sessID domain.PairingSessionID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	devID, ok := m.bySess[sessID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "paired device not found for session or already revoked"}
	}
	if p, ok := m.provisional[devID]; ok {
		if p.revokedAt != nil {
			return &domain.Error{Category: domain.NotFound, Message: "paired device not found for session or already revoked"}
		}
		revokedAt := now
		p.revokedAt = &revokedAt
		return nil
	}
	d, ok := m.byID[devID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "paired device not found for session or already revoked"}
	}
	return d.Revoke(now)
}

func (m *memPairedDevices) AdvanceCursor(_ context.Context, id domain.DeviceID, newCursor int64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "device not found"}
	}
	return d.AdvanceCursor(newCursor, now)
}

func (m *memPairedDevices) UpdateLastSeen(_ context.Context, id domain.DeviceID, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "device not found"}
	}
	return d.Touch(now)
}

func (m *memPairedDevices) isProvisionalRevoked(id domain.DeviceID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.provisional[id]
	return ok && p.revokedAt != nil
}

func TestPairDelete_PendingSession(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	devRepo := newMemPairedDevices()
	now := time.Now().UTC()

	code, _ := domain.NewPairingCode("5678-9ABC")
	session, _ := domain.NewPairingSession("ps-del-pend", "user-admin-1", code, 5*time.Minute, now)
	_ = sessionRepo.Save(context.Background(), session)

	handler := transporthttp.DeletePairingHandler(sessionRepo, devRepo, func() time.Time { return now }, nil)

	req := httptest.NewRequest("DELETE", "/api/v1/network/pair/ps-del-pend", nil)
	req.SetPathValue("id", "ps-del-pend")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	// Session is now expired
	updated, _ := sessionRepo.FindByID(context.Background(), "ps-del-pend")
	if updated.State() != domain.PairingExpired {
		t.Errorf("expected session state expired, got %q", updated.State())
	}
}

func TestPairDelete_ConsumedSessionRevokesDevice(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	devRepo := newMemPairedDevices()
	now := time.Now().UTC()

	code, _ := domain.NewPairingCode("6789-ABCD")
	session, _ := domain.NewPairingSession("ps-del-cons", "user-admin-1", code, 5*time.Minute, now)
	_ = session.Verify(now.Add(time.Minute), code, "dev-to-revoke")
	_ = session.Consume(now.Add(2 * time.Minute))
	_ = sessionRepo.Save(context.Background(), session)

	dev, _ := domain.NewPairedDevice("dev-to-revoke", "user-admin-1", "Tablet", domain.DeviceClassTablet, domain.EnrolledViaPairingCode, now)
	_ = devRepo.Save(context.Background(), dev)
	devRepo.bySess["ps-del-cons"] = "dev-to-revoke"

	handler := transporthttp.DeletePairingHandler(sessionRepo, devRepo, func() time.Time { return now.Add(3 * time.Minute) }, nil)

	req := httptest.NewRequest("DELETE", "/api/v1/network/pair/ps-del-cons", nil)
	req.SetPathValue("id", "ps-del-cons")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	// Device is now revoked
	revokedDev, err := devRepo.FindByID(context.Background(), "dev-to-revoke")
	if err != nil {
		t.Fatalf("devRepo.FindByID: %v", err)
	}
	if revokedDev.RevokedAt() == nil {
		t.Fatal("expected device to be revoked")
	}
}

// TestPairDelete_ConsumedSessionStillProvisionalDeviceIsRevoked reproduces
// the bug where an admin's DELETE /network/pair/{id} silently did nothing
// for a device that verified but hasn't completed login yet (owner_id still
// NULL): FindByPairingSessionID returned NotFound for a provisional device,
// so the handler's `err == nil && dev != nil` guard skipped Revoke entirely
// while still returning 204.
func TestPairDelete_ConsumedSessionStillProvisionalDeviceIsRevoked(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	devRepo := newMemPairedDevices()
	now := time.Now().UTC()

	code, _ := domain.NewPairingCode("789A-BCD1")
	session, _ := domain.NewPairingSession("ps-del-prov", "user-admin-1", code, 5*time.Minute, now)
	_ = session.Verify(now.Add(time.Minute), code, "dev-still-provisional")
	_ = session.Consume(now.Add(2 * time.Minute))
	_ = sessionRepo.Save(context.Background(), session)

	// Device verified but never logged in: still provisional, owner_id NULL.
	_ = devRepo.InsertProvisional(context.Background(), "dev-still-provisional", "Tablet", domain.DeviceClassTablet, domain.EnrolledViaPairingCode, "ps-del-prov", now)

	handler := transporthttp.DeletePairingHandler(sessionRepo, devRepo, func() time.Time { return now.Add(3 * time.Minute) }, nil)

	req := httptest.NewRequest("DELETE", "/api/v1/network/pair/ps-del-prov", nil)
	req.SetPathValue("id", "ps-del-prov")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	if !devRepo.isProvisionalRevoked("dev-still-provisional") {
		t.Fatal("expected still-provisional device to be revoked, but revoke silently no-op'd")
	}

	// Closing the gap the finding raised: a device revoked while provisional
	// must not become claimable by a later login completing with the same grant.
	err := devRepo.AssignOwnerByPairingSession(context.Background(), "ps-del-prov", "user-victim")
	if err == nil {
		t.Fatal("expected AssignOwnerByPairingSession to fail for a revoked provisional device, got nil error")
	}
}

func TestPairDelete_CrossAdminEnumerationPrevention(t *testing.T) {
	sessionRepo := newMemPairingSessions()
	devRepo := newMemPairedDevices()
	now := time.Now().UTC()

	code, _ := domain.NewPairingCode("789A-BCDE")
	session, _ := domain.NewPairingSession("ps-del-other", "user-admin-OTHER", code, 5*time.Minute, now)
	_ = sessionRepo.Save(context.Background(), session)

	handler := transporthttp.DeletePairingHandler(sessionRepo, devRepo, func() time.Time { return now }, nil)

	req := httptest.NewRequest("DELETE", "/api/v1/network/pair/ps-del-other", nil)
	req.SetPathValue("id", "ps-del-other")
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-admin delete status = %d, want 404", rec.Code)
	}
}

type memGrantJTIs struct {
	mu    sync.Mutex
	spent map[string]time.Time
}

func newMemGrantJTIs() *memGrantJTIs {
	return &memGrantJTIs{
		spent: make(map[string]time.Time),
	}
}

func (m *memGrantJTIs) Record(_ context.Context, jti string, spentAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.spent[jti]; ok {
		return &domain.Error{Category: domain.Conflict, Message: "jti already spent"}
	}
	m.spent[jti] = spentAt
	return nil
}

func (m *memGrantJTIs) Exists(_ context.Context, jti string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.spent[jti]
	return ok, nil
}

func TestLogin_WithValidEnrolmentGrant(t *testing.T) {
	users := newMemUsers()
	creds := newMemCredentials()
	mfaRepo := newMemMFA()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	hasher := auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())
	signer := auth.NewJWTSigner([]byte("test-jwt-secret-at-least-32-bytes!"), "alexandryn")
	idGen := &fakeIDGen{val: "id-fixed-1"}
	limiter := auth.NewIPRateLimiter(rate.Inf, 100, time.Hour)

	user, _ := domain.NewUser("user-reader-1", "reader1", "reader1@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), user)

	hash, _ := hasher.HashPassword("secretpass")
	cred, _ := domain.NewUserCredentials(user.ID(), hash, time.Now())
	_ = creds.Save(context.Background(), cred)

	devRepo := newMemPairedDevices()
	jtiRepo := newMemGrantJTIs()
	grantSigner := auth.NewEnrolmentGrantSigner([]byte("enrolment-sub-key-32-bytes-long!"), "alexandryn", idGen)

	// Provisional device for session ps-grant-sess
	now := time.Now().UTC()
	_ = devRepo.InsertProvisional(context.Background(), "dev-prov-1", "Provisional Device", domain.DeviceClassTablet, domain.EnrolledViaPairingCode, "ps-grant-sess", now)

	// Mint grant for session ps-grant-sess
	grant, err := grantSigner.Sign("ps-grant-sess", now)
	if err != nil {
		t.Fatalf("grantSigner.Sign: %v", err)
	}

	loginHandler := transporthttp.LoginHandler(
		users,
		creds,
		mfaRepo,
		mems,
		rtRepo,
		hasher,
		signer,
		idGen,
		limiter,
		transporthttp.WithEnrolmentGrant(grantSigner, devRepo, jtiRepo, nil),
	)

	body, _ := json.Marshal(map[string]string{
		"emailOrUsername": "reader1",
		"password":        "secretpass",
		"enrolmentGrant":  grant,
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	loginHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Verify device now owned by user-reader-1
	claimedDev, err := devRepo.FindByID(context.Background(), "dev-prov-1")
	if err != nil {
		t.Fatalf("devRepo.FindByID: %v", err)
	}
	if claimedDev.Owner() != user.ID() {
		t.Errorf("device owner = %q, want %q", claimedDev.Owner(), user.ID())
	}

	// Verify JTI was recorded
	claims, _ := grantSigner.Verify(grant, now)
	exists, err := jtiRepo.Exists(context.Background(), claims.JTI)
	if err != nil || !exists {
		t.Errorf("expected jti to be recorded as spent, exists=%v, err=%v", exists, err)
	}

	// Second login with SAME grant (replay) must succeed but ignore grant
	reqReplay := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	reqReplay.Header.Set("Content-Type", "application/json")
	recReplay := httptest.NewRecorder()
	loginHandler.ServeHTTP(recReplay, reqReplay)

	if recReplay.Code != http.StatusOK {
		t.Fatalf("replay login status = %d, want 200", recReplay.Code)
	}
}

func TestLogin_WithInvalidGrantIgnored(t *testing.T) {
	users := newMemUsers()
	creds := newMemCredentials()
	mfaRepo := newMemMFA()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	hasher := auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())
	signer := auth.NewJWTSigner([]byte("test-jwt-secret-at-least-32-bytes!"), "alexandryn")
	idGen := &fakeIDGen{val: "id-fixed-2"}
	limiter := auth.NewIPRateLimiter(rate.Inf, 100, time.Hour)

	user, _ := domain.NewUser("user-reader-2", "reader2", "reader2@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), user)

	hash, _ := hasher.HashPassword("secretpass")
	cred, _ := domain.NewUserCredentials(user.ID(), hash, time.Now())
	_ = creds.Save(context.Background(), cred)

	devRepo := newMemPairedDevices()
	jtiRepo := newMemGrantJTIs()
	grantSigner := auth.NewEnrolmentGrantSigner([]byte("enrolment-sub-key-32-bytes-long!"), "alexandryn", idGen)

	loginHandler := transporthttp.LoginHandler(
		users,
		creds,
		mfaRepo,
		mems,
		rtRepo,
		hasher,
		signer,
		idGen,
		limiter,
		transporthttp.WithEnrolmentGrant(grantSigner, devRepo, jtiRepo, nil),
	)

	// Tampered grant string
	body, _ := json.Marshal(map[string]string{
		"emailOrUsername": "reader2",
		"password":        "secretpass",
		"enrolmentGrant":  "invalid.jwt.token",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	loginHandler.ServeHTTP(rec, req)

	// Login MUST STILL SUCCEED (FR-9: invalid grant is ignored, login does not fail)
	if rec.Code != http.StatusOK {
		t.Fatalf("login with bad grant: status = %d, want 200", rec.Code)
	}
}

// TestLogin_MFARequired_DoesNotConsumeEnrolmentGrant reproduces the bug
// where LoginHandler processed the enrolment grant (assigning device
// ownership, burning the JTI) BEFORE the TOTP-MFA check, so a password-only
// request against an MFA-enabled account durably committed the device
// assignment and spent the grant even though login never completed (no
// tokens issued — only mfaRequired: true).
func TestLogin_MFARequired_DoesNotConsumeEnrolmentGrant(t *testing.T) {
	users := newMemUsers()
	creds := newMemCredentials()
	mfaRepo := newMemMFA()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	hasher := auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())
	signer := auth.NewJWTSigner([]byte("test-jwt-secret-at-least-32-bytes!"), "alexandryn")
	idGen := &fakeIDGen{val: "id-fixed-mfa-1"}
	limiter := auth.NewIPRateLimiter(rate.Inf, 100, time.Hour)

	user, _ := domain.NewUser("user-mfa-1", "mfauser1", "mfauser1@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), user)

	hash, _ := hasher.HashPassword("secretpass")
	cred, _ := domain.NewUserCredentials(user.ID(), hash, time.Now())
	_ = creds.Save(context.Background(), cred)

	confirmedAt := time.Now()
	totpSettings, _ := domain.NewTOTPSettings(user.ID(), []byte("encrypted-secret-placeholder"), nil, true, &confirmedAt, time.Now())
	_ = mfaRepo.Save(context.Background(), totpSettings)

	devRepo := newMemPairedDevices()
	jtiRepo := newMemGrantJTIs()
	grantSigner := auth.NewEnrolmentGrantSigner([]byte("enrolment-sub-key-32-bytes-long!"), "alexandryn", idGen)

	now := time.Now().UTC()
	_ = devRepo.InsertProvisional(context.Background(), "dev-prov-mfa", "MFA Device", domain.DeviceClassTablet, domain.EnrolledViaPairingCode, "ps-mfa-sess", now)

	grant, err := grantSigner.Sign("ps-mfa-sess", now)
	if err != nil {
		t.Fatalf("grantSigner.Sign: %v", err)
	}

	loginHandler := transporthttp.LoginHandler(
		users, creds, mfaRepo, mems, rtRepo, hasher, signer, idGen, limiter,
		transporthttp.WithEnrolmentGrant(grantSigner, devRepo, jtiRepo, nil),
	)

	body, _ := json.Marshal(map[string]string{
		"emailOrUsername": "mfauser1",
		"password":        "secretpass",
		"enrolmentGrant":  grant,
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	loginHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (mfaRequired response); body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		MFARequired bool   `json:"mfaRequired"`
		MFATicket   string `json:"mfaTicket"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.MFARequired || resp.MFATicket == "" {
		t.Fatalf("expected mfaRequired response with a ticket, got %+v", resp)
	}
	if resp.AccessToken != "" {
		t.Fatal("expected no access token on an mfaRequired response")
	}

	// The device must still be provisional — the grant must not have been
	// consumed by a login that never actually completed.
	if _, err := devRepo.FindByID(context.Background(), "dev-prov-mfa"); err == nil {
		t.Fatal("expected device to remain unassigned/provisional, but it was claimed")
	}
	claims, _ := grantSigner.Verify(grant, now)
	if exists, _ := jtiRepo.Exists(context.Background(), claims.JTI); exists {
		t.Fatal("expected enrolment grant JTI to remain unspent after an mfaRequired response")
	}
}

// TestTOTPVerify_WithEnrolmentGrant proves the grant is instead processed
// once login actually completes via the TOTP step, when the client resends
// the same enrolmentGrant alongside the TOTP code.
func TestTOTPVerify_WithEnrolmentGrant(t *testing.T) {
	users := newMemUsers()
	mfaRepo := newMemMFA()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	signer := auth.NewJWTSigner([]byte("test-jwt-secret-at-least-32-bytes!"), "alexandryn")
	idGen := &fakeIDGen{val: "id-fixed-totp-1"}
	totpEngine := auth.NewTOTPEngine("Alexandryn")
	masterKey := []byte("totp-master-key-32-bytes-long!!")

	user, _ := domain.NewUser("user-totp-1", "totpuser1", "totpuser1@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), user)

	secret, err := totpEngine.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	encryptedSecret, err := auth.EncryptSecret([]byte(secret), masterKey)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	confirmedAt := time.Now()
	totpSettings, _ := domain.NewTOTPSettings(user.ID(), encryptedSecret, nil, true, &confirmedAt, time.Now())
	_ = mfaRepo.Save(context.Background(), totpSettings)

	devRepo := newMemPairedDevices()
	jtiRepo := newMemGrantJTIs()
	grantSigner := auth.NewEnrolmentGrantSigner([]byte("enrolment-sub-key-32-bytes-long!"), "alexandryn", idGen)

	now := time.Now().UTC()
	_ = devRepo.InsertProvisional(context.Background(), "dev-prov-totp", "TOTP Device", domain.DeviceClassTablet, domain.EnrolledViaPairingCode, "ps-totp-sess", now)

	grant, err := grantSigner.Sign("ps-totp-sess", now)
	if err != nil {
		t.Fatalf("grantSigner.Sign: %v", err)
	}

	mfaTicket, err := signer.SignMFATicket(user.ID(), now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("SignMFATicket: %v", err)
	}

	code, err := totpEngine.GenerateCode(secret, now)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	verifyHandler := transporthttp.TOTPVerifyHandler(
		mfaRepo, users, rtRepo, mems, totpEngine, signer, idGen, masterKey, nil, nil,
		transporthttp.WithEnrolmentGrant(grantSigner, devRepo, jtiRepo, nil),
	)

	body, _ := json.Marshal(map[string]string{
		"mfaTicket":      mfaTicket,
		"code":           code,
		"enrolmentGrant": grant,
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/mfa/totp/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	verifyHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	claimedDev, err := devRepo.FindByID(context.Background(), "dev-prov-totp")
	if err != nil {
		t.Fatalf("devRepo.FindByID: %v", err)
	}
	if claimedDev.Owner() != user.ID() {
		t.Errorf("device owner = %q, want %q", claimedDev.Owner(), user.ID())
	}

	claims, _ := grantSigner.Verify(grant, now)
	exists, err := jtiRepo.Exists(context.Background(), claims.JTI)
	if err != nil || !exists {
		t.Errorf("expected jti to be recorded as spent, exists=%v, err=%v", exists, err)
	}
}

// TestTOTPVerify_RateLimited is the audit 0016 #89 regression: an
// attacker with the victim's password must not be able to grind the TOTP
// space. The per-IP limiter throttles a single address; the per-user
// limiter throttles the account even as the address rotates.
func TestTOTPVerify_RateLimited(t *testing.T) {
	users := newMemUsers()
	mfaRepo := newMemMFA()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	signer := auth.NewJWTSigner([]byte("test-jwt-secret-at-least-32-bytes!"), "alexandryn")
	idGen := &fakeIDGen{val: "id-fixed-rl-1"}
	totpEngine := auth.NewTOTPEngine("Alexandryn")
	masterKey := []byte("totp-master-key-32-bytes-long!!")

	user, _ := domain.NewUser("user-rl-1", "rluser1", "rluser1@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), user)

	secret, _ := totpEngine.GenerateSecret()
	encryptedSecret, _ := auth.EncryptSecret([]byte(secret), masterKey)
	confirmedAt := time.Now()
	totpSettings, _ := domain.NewTOTPSettings(user.ID(), encryptedSecret, nil, true, &confirmedAt, time.Now())
	_ = mfaRepo.Save(context.Background(), totpSettings)

	now := time.Now().UTC()

	post := func(h http.Handler, remoteAddr string) int {
		ticket, _ := signer.SignMFATicket(user.ID(), now.Add(5*time.Minute))
		body, _ := json.Marshal(map[string]string{"mfaTicket": ticket, "code": "000000"})
		req := httptest.NewRequest("POST", "/api/v1/auth/mfa/totp/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	t.Run("per-IP limiter trips", func(t *testing.T) {
		ipLimiter := auth.NewIPRateLimiter(rate.Every(time.Hour), 3, time.Hour)
		h := transporthttp.TOTPVerifyHandler(mfaRepo, users, rtRepo, mems, totpEngine, signer, idGen, masterKey, ipLimiter, nil)
		var got429 bool
		for i := 0; i < 10; i++ {
			if post(h, "203.0.113.9:1234") == http.StatusTooManyRequests {
				got429 = true
				break
			}
		}
		if !got429 {
			t.Fatal("per-IP limiter never returned 429 over 10 attempts")
		}
	})

	t.Run("per-user limiter trips across rotating IPs", func(t *testing.T) {
		userLimiter := auth.NewIPRateLimiter(rate.Every(time.Hour), 3, time.Hour)
		h := transporthttp.TOTPVerifyHandler(mfaRepo, users, rtRepo, mems, totpEngine, signer, idGen, masterKey, nil, userLimiter)
		var got429 bool
		for i := 0; i < 10; i++ {
			if post(h, "198.51.100."+strconv.Itoa(i)+":5555") == http.StatusTooManyRequests {
				got429 = true
				break
			}
		}
		if !got429 {
			t.Fatal("per-user limiter never returned 429 despite rotating source addresses")
		}
	})
}

func TestLogin_WithRememberDeviceDays(t *testing.T) {
	users := newMemUsers()
	creds := newMemCredentials()
	mfaRepo := newMemMFA()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	hasher := auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())
	signer := auth.NewJWTSigner([]byte("test-jwt-secret-at-least-32-bytes!"), "alexandryn")
	idGen := &fakeIDGen{val: "rt-fixed-1"}
	limiter := auth.NewIPRateLimiter(rate.Inf, 100, time.Hour)

	user, _ := domain.NewUser("user-reader-3", "reader3", "reader3@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), user)

	hash, _ := hasher.HashPassword("secretpass")
	cred, _ := domain.NewUserCredentials(user.ID(), hash, time.Now())
	_ = creds.Save(context.Background(), cred)

	settingsRepo := newMemNetworkSettings()
	settingsRepo.settings.RememberDeviceDays = 14

	loginHandler := transporthttp.LoginHandler(
		users,
		creds,
		mfaRepo,
		mems,
		rtRepo,
		hasher,
		signer,
		idGen,
		limiter,
		transporthttp.WithNetworkSettings(settingsRepo),
	)

	body, _ := json.Marshal(map[string]string{
		"emailOrUsername": "reader3",
		"password":        "secretpass",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	loginHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", rec.Code)
	}

	// Verify refresh token expires in 14 days
	var foundRT *domain.RefreshToken
	for _, rt := range rtRepo.byHash {
		if rt.UserID() == user.ID() {
			foundRT = rt
			break
		}
	}
	if foundRT == nil {
		t.Fatal("expected refresh token to be saved")
	}

	approx14Days := time.Now().Add(14 * 24 * time.Hour)
	diff := foundRT.ExpiresAt().Sub(approx14Days)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("expected refresh token expiry in ~14 days, got: %v (diff %v)", foundRT.ExpiresAt(), diff)
	}
}

func TestRefresh_WithRememberDeviceDays(t *testing.T) {
	users := newMemUsers()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	signer := auth.NewJWTSigner([]byte("test-jwt-secret-at-least-32-bytes!"), "alexandryn")
	idGen := &fakeIDGen{val: "rt-new-token-1"}
	limiter := auth.NewIPRateLimiter(rate.Inf, 100, time.Hour)

	user, _ := domain.NewUser("user-reader-4", "reader4", "reader4@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), user)

	initialRawToken := "initial-refresh-token-12345"
	h1 := sha256.Sum256([]byte(initialRawToken))
	initialHash := hex.EncodeToString(h1[:])
	initialExpiry := time.Now().Add(24 * time.Hour)
	rt, err := domain.NewRefreshToken("rt-1", user.ID(), initialHash, initialExpiry, time.Now())
	if err != nil {
		t.Fatalf("domain.NewRefreshToken: %v", err)
	}
	_ = rtRepo.Save(context.Background(), rt)

	settingsRepo := newMemNetworkSettings()
	settingsRepo.settings.RememberDeviceDays = 7

	refreshHandler := transporthttp.RefreshHandler(
		rtRepo,
		users,
		mems,
		signer,
		idGen,
		limiter,
		transporthttp.WithNetworkSettings(settingsRepo),
	)

	body, _ := json.Marshal(map[string]string{
		"refreshToken": initialRawToken,
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	refreshHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if res.RefreshToken == "" {
		t.Fatal("expected rotated refresh token in response")
	}

	h2 := sha256.Sum256([]byte(res.RefreshToken))
	newHash := hex.EncodeToString(h2[:])
	newRT, err := rtRepo.FindByTokenHash(context.Background(), newHash)
	if err != nil || newRT == nil {
		t.Fatalf("expected new refresh token to be found by hash: %v", err)
	}

	approx7Days := time.Now().Add(7 * 24 * time.Hour)
	diff := newRT.ExpiresAt().Sub(approx7Days)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("expected refresh token expiry in ~7 days, got: %v (diff %v)", newRT.ExpiresAt(), diff)
	}
}

// audit 0016 #143: GET /api/v1/network/settings returns the saved
// settings for an admin, and 403 for a non-admin.
func TestNetworkSettings_GetForAdmin(t *testing.T) {
	repo := newMemNetworkSettings()
	_ = repo.Upsert(context.Background(), &domain.NetworkSettings{
		HostName: "shelf.local", RememberDeviceDays: 12, UpdatedAt: time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
	})
	handler := transporthttp.GetNetworkSettingsHandler(repo, time.Now)

	req := httptest.NewRequest("GET", "/api/v1/network/settings", nil)
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	var resp transporthttp.NetworkSettingsResponseWire
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.HostName != "shelf.local" || resp.RememberDeviceDays != 12 {
		t.Fatalf("resp = %+v, want shelf.local / 12", resp)
	}
}

func TestNetworkSettings_GetForbiddenForReader(t *testing.T) {
	handler := transporthttp.GetNetworkSettingsHandler(newMemNetworkSettings(), time.Now)
	req := httptest.NewRequest("GET", "/api/v1/network/settings", nil)
	req = req.WithContext(transporthttp.WithUser(req.Context(), readerUser()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// A NotFound from the repository is served as the default settings, not
// an error (the update handler synthesises the same defaults).
func TestNetworkSettings_GetDefaultsWhenUnset(t *testing.T) {
	repo := &notFoundNetworkSettings{}
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	handler := transporthttp.GetNetworkSettingsHandler(repo, func() time.Time { return now })

	req := httptest.NewRequest("GET", "/api/v1/network/settings", nil)
	req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp transporthttp.NetworkSettingsResponseWire
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.HostName != "alexandryn.local" || resp.RememberDeviceDays != 30 {
		t.Fatalf("resp = %+v, want the defaults", resp)
	}
}

type notFoundNetworkSettings struct{}

func (notFoundNetworkSettings) Get(context.Context) (*domain.NetworkSettings, error) {
	return nil, &domain.Error{Category: domain.NotFound, Message: "no settings"}
}
func (notFoundNetworkSettings) Upsert(context.Context, *domain.NetworkSettings) error { return nil }
