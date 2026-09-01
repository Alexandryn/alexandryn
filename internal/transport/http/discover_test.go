package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type fakeOpenLibraryClient struct {
	searchFunc     func(ctx context.Context, q string, limit, offset int) (*openlibrary.NormalisedSearchResponse, error)
	getWorkFunc    func(ctx context.Context, openLibraryID string) (*openlibrary.DiscoverWorkDetail, error)
	fetchCoverFunc func(ctx context.Context, coverID int64) ([]byte, string, error)
}

func (f *fakeOpenLibraryClient) Search(ctx context.Context, q string, limit, offset int) (*openlibrary.NormalisedSearchResponse, error) {
	if f.searchFunc != nil {
		return f.searchFunc(ctx, q, limit, offset)
	}
	return &openlibrary.NormalisedSearchResponse{Items: []openlibrary.NormalisedSearchResult{}}, nil
}

func (f *fakeOpenLibraryClient) GetWork(ctx context.Context, openLibraryID string) (*openlibrary.DiscoverWorkDetail, error) {
	if f.getWorkFunc != nil {
		return f.getWorkFunc(ctx, openLibraryID)
	}
	return &openlibrary.DiscoverWorkDetail{}, nil
}

func (f *fakeOpenLibraryClient) FetchCover(ctx context.Context, coverID int64) ([]byte, string, error) {
	if f.fetchCoverFunc != nil {
		return f.fetchCoverFunc(ctx, coverID)
	}
	return []byte("fake-cover"), "image/jpeg", nil
}

type fakeMetadataCacheRepo struct {
	getWorkCalls int
	saveWorkCalls int
	workStore    map[string]*openlibrary.DiscoverWorkDetail
}

func (f *fakeMetadataCacheRepo) GetWork(ctx context.Context, key string) (*openlibrary.DiscoverWorkDetail, bool, error) {
	f.getWorkCalls++
	if f.workStore != nil {
		if detail, ok := f.workStore[key]; ok {
			return detail, true, nil
		}
	}
	return nil, false, nil
}

func (f *fakeMetadataCacheRepo) GetAuthor(ctx context.Context, key string) (*openlibrary.NormalisedAuthor, bool, error) {
	return nil, false, nil
}

func (f *fakeMetadataCacheRepo) SaveWork(ctx context.Context, workKey string, detail *openlibrary.DiscoverWorkDetail) error {
	f.saveWorkCalls++
	if f.workStore == nil {
		f.workStore = make(map[string]*openlibrary.DiscoverWorkDetail)
	}
	f.workStore[workKey] = detail
	return nil
}

func (f *fakeMetadataCacheRepo) SaveAuthor(ctx context.Context, author *openlibrary.NormalisedAuthor) error {
	return nil
}

type fakeCoverCacheRepo struct {
	getCalls     int
	saveCalls    int
	missingCalls int
	missingMap   map[int64]bool
	fileMap      map[int64]string
}

func (f *fakeCoverCacheRepo) GetCover(ctx context.Context, coverID int64) (string, string, bool, bool, error) {
	f.getCalls++
	if f.missingMap != nil && f.missingMap[coverID] {
		return "", "", true, true, nil
	}
	if f.fileMap != nil && f.fileMap[coverID] != "" {
		return f.fileMap[coverID], "image/jpeg", false, true, nil
	}
	return "", "", false, false, nil
}

func (f *fakeCoverCacheRepo) SaveCover(ctx context.Context, coverID int64, contentType string, data []byte) (string, error) {
	f.saveCalls++
	if f.fileMap == nil {
		f.fileMap = make(map[int64]string)
	}
	f.fileMap[coverID] = "/tmp/cover.jpg"
	return "/tmp/cover.jpg", nil
}

func (f *fakeCoverCacheRepo) MarkMissing(ctx context.Context, coverID int64) error {
	f.missingCalls++
	if f.missingMap == nil {
		f.missingMap = make(map[int64]bool)
	}
	f.missingMap[coverID] = true
	return nil
}

func TestDiscoverSearchHandler(t *testing.T) {
	t.Run("line-1 validation rejects invalid query parameters", func(t *testing.T) {
		client := &fakeOpenLibraryClient{}
		handler := transporthttp.DiscoverSearchHandler(client)

		cases := []struct {
			name       string
			url        string
			wantStatus int
		}{
			{"empty query", "/api/v1/discover", http.StatusBadRequest},
			{"whitespace query", "/api/v1/discover?q=%20%20", http.StatusBadRequest},
			{"query over 200 runes", "/api/v1/discover?q=" + strings.Repeat("a", 201), http.StatusBadRequest},
			{"query with control character", "/api/v1/discover?q=test%07bell", http.StatusBadRequest},
			{"limit out of range (0)", "/api/v1/discover?q=test&limit=0", http.StatusBadRequest},
			{"limit out of range (51)", "/api/v1/discover?q=test&limit=51", http.StatusBadRequest},
			{"limit non-integer", "/api/v1/discover?q=test&limit=abc", http.StatusBadRequest},
			{"negative offset", "/api/v1/discover?q=test&offset=-1", http.StatusBadRequest},
			{"offset non-integer", "/api/v1/discover?q=test&offset=abc", http.StatusBadRequest},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, tc.url, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != tc.wantStatus {
					t.Errorf("got status %d, want %d", rec.Code, tc.wantStatus)
				}
				var errBody map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&errBody); err != nil {
					t.Fatalf("failed to decode error body: %v", err)
				}
				if errBody["code"] != string(domain.InvalidInput) {
					t.Errorf("got code %q, want %s", errBody["code"], domain.InvalidInput)
				}
			})
		}
	})

	t.Run("successful search returns 200 OK with normalised JSON", func(t *testing.T) {
		authorKey := "OL123A"
		client := &fakeOpenLibraryClient{
			searchFunc: func(ctx context.Context, q string, limit, offset int) (*openlibrary.NormalisedSearchResponse, error) {
				return &openlibrary.NormalisedSearchResponse{
					Items: []openlibrary.NormalisedSearchResult{
						{
							OpenLibraryWorkKey: "OL82563W",
							Title:              "Middlemarch",
							Authors: []openlibrary.NormalisedAuthor{
								{OpenLibraryAuthorKey: &authorKey, Name: "George Eliot"},
							},
							EditionCount: 10,
						},
					},
					Total:  1,
					Limit:  20,
					Offset: 0,
				}, nil
			},
		}

		handler := transporthttp.DiscoverSearchHandler(client)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/discover?q=Middlemarch", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}

		var resp openlibrary.NormalisedSearchResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Total != 1 || len(resp.Items) != 1 || resp.Items[0].Title != "Middlemarch" {
			t.Errorf("unexpected response content: %+v", resp)
		}
	})
}

func TestDiscoverWorkDetailHandler(t *testing.T) {
	t.Run("line-1 validation rejects invalid openLibraryId", func(t *testing.T) {
		client := &fakeOpenLibraryClient{}
		cacheRepo := &fakeMetadataCacheRepo{}
		handler := transporthttp.DiscoverWorkDetailHandler(client, cacheRepo)

		for _, invalidID := range []string{"invalid", "OL123", "OL123A", "123W"} {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/works/"+invalidID, nil)
			req.SetPathValue("openLibraryId", invalidID)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("id %q: got status %d, want 400", invalidID, rec.Code)
			}
		}
	})

	t.Run("read-through cache serves hit without calling upstream", func(t *testing.T) {
		upstreamCalled := false
		client := &fakeOpenLibraryClient{
			getWorkFunc: func(ctx context.Context, openLibraryID string) (*openlibrary.DiscoverWorkDetail, error) {
				upstreamCalled = true
				return nil, nil
			},
		}

		cacheRepo := &fakeMetadataCacheRepo{
			workStore: map[string]*openlibrary.DiscoverWorkDetail{
				"OL82563W": {
					Work: openlibrary.NormalisedWork{
						Title: "Middlemarch (Cached)",
					},
					Editions: []openlibrary.NormalisedEdition{},
				},
			},
		}

		handler := transporthttp.DiscoverWorkDetailHandler(client, cacheRepo)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/works/OL82563W", nil)
		req.SetPathValue("openLibraryId", "OL82563W")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}
		if xmc := rec.Header().Get("X-Metadata-Cache"); xmc != "hit" {
			t.Errorf("got X-Metadata-Cache %q, want 'hit'", xmc)
		}
		if upstreamCalled {
			t.Error("expected cache hit to not call upstream client")
		}

		var detail openlibrary.DiscoverWorkDetail
		if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
			t.Fatalf("failed to decode detail: %v", err)
		}
		if detail.Work.Title != "Middlemarch (Cached)" {
			t.Errorf("got title %q, want 'Middlemarch (Cached)'", detail.Work.Title)
		}
	})

	t.Run("cache miss fetches upstream and populates cache", func(t *testing.T) {
		client := &fakeOpenLibraryClient{
			getWorkFunc: func(ctx context.Context, openLibraryID string) (*openlibrary.DiscoverWorkDetail, error) {
				return &openlibrary.DiscoverWorkDetail{
					Work: openlibrary.NormalisedWork{
						Title: "Fresh Upstream Work",
					},
				}, nil
			},
		}

		cacheRepo := &fakeMetadataCacheRepo{}
		handler := transporthttp.DiscoverWorkDetailHandler(client, cacheRepo)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/works/OL82563W", nil)
		req.SetPathValue("openLibraryId", "OL82563W")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}
		if xmc := rec.Header().Get("X-Metadata-Cache"); xmc != "miss" {
			t.Errorf("got X-Metadata-Cache %q, want 'miss'", xmc)
		}
		if cacheRepo.saveWorkCalls != 1 {
			t.Errorf("expected 1 SaveWork call on cache miss, got %d", cacheRepo.saveWorkCalls)
		}
	})
}

func TestDiscoverCoverHandler(t *testing.T) {
	t.Run("line-1 validation rejects invalid coverId", func(t *testing.T) {
		client := &fakeOpenLibraryClient{}
		cacheRepo := &fakeCoverCacheRepo{}
		handler := transporthttp.DiscoverCoverHandler(client, cacheRepo)

		for _, invalidID := range []string{"0", "-1", "abc", ""} {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/covers/"+invalidID, nil)
			req.SetPathValue("coverId", invalidID)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("coverId %q: got status %d, want 400", invalidID, rec.Code)
			}
		}
	})

	t.Run("sentinel hit returns 404 immediately", func(t *testing.T) {
		upstreamCalled := false
		client := &fakeOpenLibraryClient{
			fetchCoverFunc: func(ctx context.Context, coverID int64) ([]byte, string, error) {
				upstreamCalled = true
				return nil, "", nil
			},
		}
		cacheRepo := &fakeCoverCacheRepo{
			missingMap: map[int64]bool{99999: true},
		}

		handler := transporthttp.DiscoverCoverHandler(client, cacheRepo)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/covers/99999", nil)
		req.SetPathValue("coverId", "99999")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want 404", rec.Code)
		}
		if upstreamCalled {
			t.Error("expected sentinel hit to not call upstream")
		}
	})

	t.Run("upstream 404 records sentinel and returns 404", func(t *testing.T) {
		client := &fakeOpenLibraryClient{
			fetchCoverFunc: func(ctx context.Context, coverID int64) ([]byte, string, error) {
				return nil, "", &domain.Error{Category: domain.NotFound, Message: "cover not found"}
			},
		}
		cacheRepo := &fakeCoverCacheRepo{}

		handler := transporthttp.DiscoverCoverHandler(client, cacheRepo)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/covers/12345", nil)
		req.SetPathValue("coverId", "12345")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want 404", rec.Code)
		}
		if cacheRepo.missingCalls != 1 {
			t.Errorf("expected 1 MarkMissing call, got %d", cacheRepo.missingCalls)
		}
	})

	t.Run("cache hit serves local file with immutable cache header", func(t *testing.T) {
		tempFile := t.TempDir() + "/test-cover.jpg"
		_ = os.WriteFile(tempFile, []byte("cached-image-bytes"), 0644)

		client := &fakeOpenLibraryClient{}
		cacheRepo := &fakeCoverCacheRepo{
			fileMap: map[int64]string{1001: tempFile},
		}

		handler := transporthttp.DiscoverCoverHandler(client, cacheRepo)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/covers/1001", nil)
		req.SetPathValue("coverId", "1001")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200", rec.Code)
		}
		if xmc := rec.Header().Get("X-Metadata-Cache"); xmc != "hit" {
			t.Errorf("got X-Metadata-Cache %q, want 'hit'", xmc)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=2592000, immutable" {
			t.Errorf("got Cache-Control %q, want 'public, max-age=2592000, immutable'", cc)
		}
		if body := rec.Body.String(); body != "cached-image-bytes" {
			t.Errorf("got body %q, want 'cached-image-bytes'", body)
		}
	})
}
