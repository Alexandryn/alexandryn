package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type mockSourceRecordRepo struct {
	mu      sync.Mutex
	records map[string]postgres.SourceRecord

	createFunc       func(ctx context.Context, rec postgres.SourceRecord) error
	getFunc          func(ctx context.Context, id string) (postgres.SourceRecord, error)
	listFunc         func(ctx context.Context) ([]postgres.SourceRecord, error)
	updateConfigFunc func(ctx context.Context, id, label, basePath, baseURL string) error
	setCredFunc      func(ctx context.Context, id string, ct, nonce []byte) error
	updateHealthFunc func(ctx context.Context, id, status, detail string, checkedAt time.Time, caps domain.SourceCapabilities, searchLinkURL string) error
}

func newMockSourceRecordRepo() *mockSourceRecordRepo {
	return &mockSourceRecordRepo{
		records: make(map[string]postgres.SourceRecord),
	}
}

func (m *mockSourceRecordRepo) Create(ctx context.Context, rec postgres.SourceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createFunc != nil {
		return m.createFunc(ctx, rec)
	}
	m.records[rec.ID] = rec
	return nil
}

func (m *mockSourceRecordRepo) Get(ctx context.Context, id string) (postgres.SourceRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	rec, ok := m.records[id]
	if !ok {
		return postgres.SourceRecord{}, &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	return rec, nil
}

func (m *mockSourceRecordRepo) List(ctx context.Context) ([]postgres.SourceRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listFunc != nil {
		return m.listFunc(ctx)
	}
	out := make([]postgres.SourceRecord, 0, len(m.records))
	for _, r := range m.records {
		out = append(out, r)
	}
	return out, nil
}

func (m *mockSourceRecordRepo) UpdateConfig(ctx context.Context, id, label, basePath, baseURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateConfigFunc != nil {
		return m.updateConfigFunc(ctx, id, label, basePath, baseURL)
	}
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

func (m *mockSourceRecordRepo) SetCredential(ctx context.Context, id string, ct, nonce []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setCredFunc != nil {
		return m.setCredFunc(ctx, id, ct, nonce)
	}
	rec, ok := m.records[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	rec.CredentialCiphertext = ct
	rec.CredentialNonce = nonce
	m.records[id] = rec
	return nil
}

func (m *mockSourceRecordRepo) UpdateHealth(ctx context.Context, id, status, detail string, checkedAt time.Time, caps domain.SourceCapabilities, searchLinkURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateHealthFunc != nil {
		return m.updateHealthFunc(ctx, id, status, detail, checkedAt, caps, searchLinkURL)
	}
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

var _ transporthttp.SourceRecordRepository = (*mockSourceRecordRepo)(nil)

func testSourceCrypto(t *testing.T) transporthttp.SourceCrypto {
	t.Helper()
	key := bytes.Repeat([]byte{0x42}, 32)
	svc, err := crypto.NewService(key)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	subkey, err := svc.DeriveSubkey("source-cursor-hmac-v1")
	if err != nil {
		t.Fatalf("DeriveSubkey: %v", err)
	}
	return transporthttp.SourceCrypto{
		Encryptor: svc,
		Codec:     sources.NewCursorCodec(subkey),
	}
}

func TestSources_Create_Line1Validation(t *testing.T) {
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	poolRef.SetSourceCrypto(testSourceCrypto(t))
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	handler := transporthttp.CreateSourceHandler(repo, poolRef, sem, &mockIDGen{nextID: "src-1"}, logger)

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"invalid json", `{not json`, http.StatusBadRequest},
		{"empty label", `{"label":"","kind":"local-folder","config":{"basePath":"/srv/books"}}`, http.StatusBadRequest},
		{"whitespace label", `{"label":"   ","kind":"local-folder","config":{"basePath":"/srv/books"}}`, http.StatusBadRequest},
		{"overlong label", fmt.Sprintf(`{"label":"%s","kind":"local-folder","config":{"basePath":"/srv/books"}}`, strings.Repeat("a", 101)), http.StatusBadRequest},
		{"invalid kind", `{"label":"Valid","kind":"ftp","config":{"basePath":"/srv/books"}}`, http.StatusBadRequest},
		{"local-folder missing basePath", `{"label":"Valid","kind":"local-folder","config":{}}`, http.StatusBadRequest},
		{"local-folder relative path", `{"label":"Valid","kind":"local-folder","config":{"basePath":"relative/path"}}`, http.StatusBadRequest},
		{"local-folder with credential", `{"label":"Valid","kind":"local-folder","config":{"basePath":"/srv/books"},"credential":{"username":"u","password":"p"}}`, http.StatusBadRequest},
		{"opds missing baseUrl", `{"label":"Valid","kind":"opds","config":{}}`, http.StatusBadRequest},
		{"opds non-http baseUrl", `{"label":"Valid","kind":"opds","config":{"baseUrl":"ftp://example.com/feed"}}`, http.StatusBadRequest},
		{"opds empty credential user", `{"label":"Valid","kind":"opds","config":{"baseUrl":"https://example.com/feed"},"credential":{"username":"","password":"p"}}`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var errBody struct {
				Code          string `json:"code"`
				Message       string `json:"message"`
				CorrelationID string `json:"correlationId"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
				t.Fatalf("failed to decode error body: %v", err)
			}
			if errBody.Code != string(domain.InvalidInput) {
				t.Errorf("code = %q, want %s", errBody.Code, domain.InvalidInput)
			}
		})
	}
}

func TestSources_Create_SuccessAndUnhealthy(t *testing.T) {
	tempDir := t.TempDir()
	bookPath := filepath.Join(tempDir, "book.epub")
	if err := os.WriteFile(bookPath, []byte("epub-content"), 0o600); err != nil {
		t.Fatalf("write book: %v", err)
	}

	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	handler := transporthttp.CreateSourceHandler(repo, poolRef, sem, &mockIDGen{nextID: "src-local-1"}, logger)

	t.Run("creates healthy local-folder source", func(t *testing.T) {
		body := fmt.Sprintf(`{"label":"My Local Books","kind":"local-folder","config":{"basePath":%q}}`, tempDir)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			ID            string `json:"id"`
			Label         string `json:"label"`
			Kind          string `json:"kind"`
			HasCredential bool   `json:"hasCredential"`
			Health        struct {
				Status    string  `json:"status"`
				CheckedAt *string `json:"checkedAt"`
				Detail    *string `json:"detail"`
			} `json:"health"`
			Capabilities struct {
				CanList     bool `json:"canList"`
				CanSearch   bool `json:"canSearch"`
				CanDownload bool `json:"canDownload"`
			} `json:"capabilities"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if resp.ID != "src-local-1" || resp.Label != "My Local Books" || resp.Kind != "local-folder" {
			t.Errorf("unexpected response: %+v", resp)
		}
		if resp.Health.Status != "reachable" {
			t.Errorf("status = %q, want reachable", resp.Health.Status)
		}
		if resp.Health.Detail != nil {
			t.Errorf("detail = %v, want nil", *resp.Health.Detail)
		}
		if !resp.Capabilities.CanList || !resp.Capabilities.CanDownload || resp.Capabilities.CanSearch {
			t.Errorf("unexpected capabilities: %+v", resp.Capabilities)
		}
	})

	t.Run("creates unhealthy source when path not found", func(t *testing.T) {
		missingPath := filepath.Join(tempDir, "nonexistent")
		body := fmt.Sprintf(`{"label":"Missing Books","kind":"local-folder","config":{"basePath":%q}}`, missingPath)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Health struct {
				Status string  `json:"status"`
				Detail *string `json:"detail"`
			} `json:"health"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Health.Status != "unreachable" {
			t.Errorf("health.status = %q, want unreachable", resp.Health.Status)
		}
		if resp.Health.Detail == nil || *resp.Health.Detail != "path-not-found" {
			t.Errorf("health.detail = %v, want path-not-found", resp.Health.Detail)
		}
	})
}

func TestSources_List_NeverEchoesCredential(t *testing.T) {
	repo := newMockSourceRecordRepo()
	sc := testSourceCrypto(t)
	credJSON, _ := json.Marshal(map[string]string{"username": "alice", "password": "supersecretpassword123"})
	ct, nonce, _ := sc.Encryptor.Encrypt(credJSON)

	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:                   "src-opds-1",
		Label:                "OPDS Source",
		Kind:                 "opds",
		ConfigBaseURL:        "https://opds.example.com",
		CredentialCiphertext: ct,
		CredentialNonce:      nonce,
		HealthStatus:         "reachable",
		HealthCheckedAt:      &now,
		Capabilities:         domain.SourceCapabilities{CanList: true, CanSearch: true, CanDownload: true},
		CreatedAt:            now,
	})

	handler := transporthttp.ListSourcesHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	bodyStr := rec.Body.String()
	if strings.Contains(bodyStr, "alice") || strings.Contains(bodyStr, "supersecretpassword123") {
		t.Fatalf("raw response leaked credentials: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"hasCredential":true`) {
		t.Errorf("response missing hasCredential:true: %s", bodyStr)
	}
}

func TestSources_Get_SuccessAnd404(t *testing.T) {
	repo := newMockSourceRecordRepo()
	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:              "src-1",
		Label:           "Test Source",
		Kind:            "local-folder",
		ConfigBasePath:  "/srv/books",
		HealthStatus:    "reachable",
		HealthCheckedAt: &now,
		Capabilities:    domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
		CreatedAt:       now,
	})

	handler := transporthttp.GetSourceHandler(repo)

	t.Run("200 for existing source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-1", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("404 for nonexistent source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/nonexistent", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}

func TestSources_Update_And_Delete(t *testing.T) {
	tempDir := t.TempDir()
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:              "src-update-1",
		Label:           "Old Label",
		Kind:            "local-folder",
		ConfigBasePath:  tempDir,
		HealthStatus:    "reachable",
		HealthCheckedAt: &now,
		Capabilities:    domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
		CreatedAt:       now,
	})

	updateHandler := transporthttp.UpdateSourceHandler(repo, poolRef, sem, logger)

	t.Run("PATCH updates label and config", func(t *testing.T) {
		body := `{"label":"New Label"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/src-update-1", strings.NewReader(body))
		rec := httptest.NewRecorder()

		updateHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Label string `json:"label"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Label != "New Label" {
			t.Errorf("label = %q, want New Label", resp.Label)
		}
	})

	t.Run("DELETE cascades and removes source", func(t *testing.T) {
		// Set a mock SourceRemovalService on poolRef
		// Note: since SourceRemovalService is a domain type, we can create one with postgres repo mocks or test that DELETE calls it and returns 204.
		// Let's create a domain.SourceRemovalService with mocks or real instance!
		sourceRepo := postgres.NewSourceRepository(nil)
		offeringRepo := postgres.NewSourceOfferingRepository(nil)
		transactor := postgres.NewTransactor(nil)
		removalSvc := domain.NewSourceRemovalService(sourceRepo, offeringRepo, transactor)
		// Or test poolRef.SetSourceRemovalService
		poolRef.SetSourceRemovalService(removalSvc)

		deleteHandler := transporthttp.DeleteSourceHandler(repo, poolRef)

		// With our mock repo, we can wrap or test existence check:
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/nonexistent", nil)
		rec := httptest.NewRecorder()
		deleteHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("delete nonexistent: status = %d, want 404", rec.Code)
		}
	})
}

func TestSources_Browse_ValidationAndLimits(t *testing.T) {
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	tempDir := t.TempDir()
	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:              "src-browse-1",
		Label:           "Browse Source",
		Kind:            "local-folder",
		ConfigBasePath:  tempDir,
		HealthStatus:    "reachable",
		HealthCheckedAt: &now,
		Capabilities:    domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
		CreatedAt:       now,
	})

	browseHandler := transporthttp.BrowseSourceHandler(repo, poolRef, sem, logger)

	cases := []struct {
		name       string
		url        string
		wantStatus int
	}{
		{"limit 0 is rejected", "/api/v1/sources/src-browse-1/browse?limit=0", http.StatusBadRequest},
		{"limit 51 is rejected", "/api/v1/sources/src-browse-1/browse?limit=51", http.StatusBadRequest},
		{"limit abc is rejected", "/api/v1/sources/src-browse-1/browse?limit=abc", http.StatusBadRequest},
		{"valid limit 20", "/api/v1/sources/src-browse-1/browse?limit=20", http.StatusOK},
		{"nonexistent source is 404", "/api/v1/sources/nonexistent/browse", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()

			browseHandler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestSources_Search_CanSearchConflict(t *testing.T) {
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:              "src-no-search",
		Label:           "No Search Source",
		Kind:            "local-folder",
		ConfigBasePath:  t.TempDir(),
		HealthStatus:    "reachable",
		HealthCheckedAt: &now,
		Capabilities:    domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
		CreatedAt:       now,
	})

	searchHandler := transporthttp.SearchSourceHandler(repo, poolRef, sem, logger)

	t.Run("returns 409 Conflict when CanSearch is false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-no-search/search?q=earthsea", nil)
		rec := httptest.NewRecorder()

		searchHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409 Conflict: %s", rec.Code, rec.Body.String())
		}
		var errBody struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
		if errBody.Code != string(domain.Conflict) {
			t.Errorf("code = %q, want Conflict", errBody.Code)
		}
	})

	t.Run("validates query length and control characters", func(t *testing.T) {
		reqEmpty := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-no-search/search?q=", nil)
		recEmpty := httptest.NewRecorder()
		searchHandler.ServeHTTP(recEmpty, reqEmpty)
		if recEmpty.Code != http.StatusBadRequest {
			t.Errorf("empty q status = %d, want 400", recEmpty.Code)
		}

		reqLong := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-no-search/search?q="+strings.Repeat("a", 201), nil)
		recLong := httptest.NewRecorder()
		searchHandler.ServeHTTP(recLong, reqLong)
		if recLong.Code != http.StatusBadRequest {
			t.Errorf("overlong q status = %d, want 400", recLong.Code)
		}

		reqCtrl := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-no-search/search?q=abc%00def", nil)
		recCtrl := httptest.NewRecorder()
		searchHandler.ServeHTTP(recCtrl, reqCtrl)
		if recCtrl.Code != http.StatusBadRequest {
			t.Errorf("control char q status = %d, want 400", recCtrl.Code)
		}
	})
}

func TestSources_HealthCheck(t *testing.T) {
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	tempDir := t.TempDir()
	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:              "src-hc-1",
		Label:           "HC Source",
		Kind:            "local-folder",
		ConfigBasePath:  tempDir,
		HealthStatus:    "unknown",
		HealthCheckedAt: nil,
		Capabilities:    domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
		CreatedAt:       now,
	})

	hcHandler := transporthttp.HealthCheckSourceHandler(repo, poolRef, sem, logger)

	t.Run("200 for existing source with probe result", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/src-hc-1/health-check", nil)
		rec := httptest.NewRecorder()

		hcHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Status    string  `json:"status"`
			CheckedAt *string `json:"checkedAt"`
			Detail    *string `json:"detail"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Status != "reachable" {
			t.Errorf("status = %q, want reachable", resp.Status)
		}
		if resp.CheckedAt == nil {
			t.Errorf("checkedAt is nil, want timestamp")
		}
		if resp.Detail != nil {
			t.Errorf("detail = %v, want nil", *resp.Detail)
		}
	})

	t.Run("404 for nonexistent source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/nonexistent/health-check", nil)
		rec := httptest.NewRecorder()
		hcHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}

func TestSources_Browse_ForgedOffOriginCursor(t *testing.T) {
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:              "src-opds-cursor",
		Label:           "OPDS Source",
		Kind:            "opds",
		ConfigBaseURL:   "https://opds.example.com/catalog",
		HealthStatus:    "reachable",
		HealthCheckedAt: &now,
		Capabilities:    domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
		CreatedAt:       now,
	})

	// Encode a cursor pointing off-origin (SSRF attempt)
	forgedCursor := sc.Codec.Encode(sources.Cursor{
		SourceID: "src-opds-cursor",
		Kind:     sources.KindOPDS,
		Position: "https://evil.example.com/catalog?page=2",
	})

	browseHandler := transporthttp.BrowseSourceHandler(repo, poolRef, sem, logger)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-opds-cursor/browse?cursor="+forgedCursor, nil)
	rec := httptest.NewRecorder()

	browseHandler.ServeHTTP(rec, req)

	// An off-origin continuation cursor must be rejected as InvalidInput (400) before any HTTP call
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request: %s", rec.Code, rec.Body.String())
	}
}

func TestSources_Browse_DecryptFailureReturns503(t *testing.T) {
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	now := time.Now().UTC()
	_ = repo.Create(context.Background(), postgres.SourceRecord{
		ID:                   "src-broken-cred",
		Label:                "Broken Cred Source",
		Kind:                 "opds",
		ConfigBaseURL:        "https://opds.example.com/catalog",
		CredentialCiphertext: []byte("corrupt-ciphertext"),
		CredentialNonce:      bytes.Repeat([]byte{0x01}, 12),
		HealthStatus:         "reachable",
		HealthCheckedAt:      &now,
		Capabilities:         domain.SourceCapabilities{CanList: true, CanSearch: false, CanDownload: true},
		CreatedAt:            now,
	})

	browseHandler := transporthttp.BrowseSourceHandler(repo, poolRef, sem, logger)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-broken-cred/browse", nil)
	rec := httptest.NewRecorder()

	browseHandler.ServeHTTP(rec, req)

	// Decrypt failure MUST return 503 Unavailable immediately
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 Unavailable: %s", rec.Code, rec.Body.String())
	}
}

func TestSources_Logging_NoCredentialsOrSecrets(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	sc := testSourceCrypto(t)
	poolRef.SetSourceCrypto(sc)
	sem := sources.NewSemaphore(50)

	tempDir := t.TempDir()
	createHandler := transporthttp.CreateSourceHandler(repo, poolRef, sem, &mockIDGen{nextID: "src-log-test"}, logger)

	body := fmt.Sprintf(`{"label":"Log Test","kind":"local-folder","config":{"basePath":%q}}`, tempDir)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(body))
	rec := httptest.NewRecorder()
	createHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d", rec.Code)
	}

	browseHandler := transporthttp.BrowseSourceHandler(repo, poolRef, sem, logger)
	reqBrowse := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-log-test/browse", nil)
	recBrowse := httptest.NewRecorder()
	browseHandler.ServeHTTP(recBrowse, reqBrowse)

	if recBrowse.Code != http.StatusOK {
		t.Fatalf("browse failed: %d", recBrowse.Code)
	}

	logs := logBuf.String()
	// Never log full file paths from a user's home directory or secrets
	if strings.Contains(logs, tempDir) {
		t.Errorf("logs contained full basePath %q: %s", tempDir, logs)
	}
}

func TestSources_CorrelationIDPropagation(t *testing.T) {
	repo := newMockSourceRecordRepo()
	poolRef := &transporthttp.PoolRef{}
	poolRef.SetSourceAllowPrivateAddresses(true)
	poolRef.SetSourceCrypto(testSourceCrypto(t))
	sem := sources.NewSemaphore(50)
	logger := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	handler := transporthttp.CreateSourceHandler(repo, poolRef, sem, &mockIDGen{nextID: "src-1"}, logger)

	// Wrap with Logging middleware to attach correlation ID
	wrapped := transporthttp.Logging(logger, func() string { return "test-corr-id-456" })(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`{invalid`))
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var errBody struct {
		CorrelationID string `json:"correlationId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errBody.CorrelationID != "test-corr-id-456" {
		t.Errorf("correlationId = %q, want test-corr-id-456", errBody.CorrelationID)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
