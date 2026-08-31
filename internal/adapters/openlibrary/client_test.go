package openlibrary_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestClient_Search(t *testing.T) {
	t.Run("sends User-Agent and query parameters correctly", func(t *testing.T) {
		var receivedUA, receivedQuery string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUA = r.Header.Get("User-Agent")
			receivedQuery = r.URL.Query().Get("q")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"num_found": 1, "docs": [{"key": "/works/OL1W", "title": "Test"}]}`))
		}))
		defer ts.Close()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		client := openlibrary.NewClient(ts.URL, "AlexandrynTest/1.0 (test@example.com)", logger, nil, ts.Client())

		resp, err := client.Search(context.Background(), "tolkien", 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receivedUA != "AlexandrynTest/1.0 (test@example.com)" {
			t.Errorf("expected custom UA, got %q", receivedUA)
		}
		if receivedQuery != "tolkien" {
			t.Errorf("expected query 'tolkien', got %q", receivedQuery)
		}
		if resp.Total != 1 || len(resp.Items) != 1 {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("upstream 429 or 503 maps to domain.Unavailable", func(t *testing.T) {
		for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInternalServerError} {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			client := openlibrary.NewClient(ts.URL, "UA", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, ts.Client())

			_, err := client.Search(context.Background(), "test", 10, 0)
			ts.Close()
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", status)
			}
			domErr, ok := err.(*domain.Error)
			if !ok || domErr.Category != domain.Unavailable {
				t.Errorf("status %d: expected domain.Unavailable, got %v", status, err)
			}
		}
	})

	t.Run("response exceeding 5 MiB cap is rejected as domain.Unavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Stream 6 MiB of whitespace/JSON
			chunk := make([]byte, 1024*1024)
			for i := range chunk {
				chunk[i] = ' '
			}
			for i := 0; i < 6; i++ {
				_, _ = w.Write(chunk)
			}
		}))
		defer ts.Close()

		client := openlibrary.NewClient(ts.URL, "UA", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, ts.Client())
		_, err := client.Search(context.Background(), "huge", 10, 0)
		if err == nil {
			t.Fatal("expected error for 6 MiB payload, got nil")
		}
		domErr, ok := err.(*domain.Error)
		if !ok || domErr.Category != domain.Unavailable {
			t.Errorf("expected domain.Unavailable, got %v", err)
		}
	})
}

func TestClient_GetWork(t *testing.T) {
	t.Run("validates openLibraryID shape", func(t *testing.T) {
		client := openlibrary.NewClient("http://localhost", "UA", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
		invalidIDs := []string{"", "12345", "OL123", "OL123A", "OL123M", "OLW"}
		for _, id := range invalidIDs {
			_, err := client.GetWork(context.Background(), id)
			if err == nil {
				t.Errorf("expected error for invalid ID %q, got nil", id)
			}
			domErr, ok := err.(*domain.Error)
			if !ok || domErr.Category != domain.InvalidInput {
				t.Errorf("expected domain.InvalidInput for ID %q, got %v", id, err)
			}
		}
	})

	t.Run("upstream 404 maps to domain.NotFound", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		client := openlibrary.NewClient(ts.URL, "UA", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, ts.Client())
		_, err := client.GetWork(context.Background(), "OL999999W")
		if err == nil {
			t.Fatal("expected error for 404, got nil")
		}
		domErr, ok := err.(*domain.Error)
		if !ok || domErr.Category != domain.NotFound {
			t.Errorf("expected domain.NotFound, got %v", err)
		}
	})

	t.Run("caps author lookups at 20 distinct authors and tolerates single-author failures", func(t *testing.T) {
		var authorLookups int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/works/OL82563W.json":
				// Return work referencing 25 authors
				authorsList := ""
				for i := 1; i <= 25; i++ {
					if i > 1 {
						authorsList += ","
					}
					authorsList += fmt.Sprintf(`{"author": {"key": "/authors/OL%dA"}}`, i)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{
					"key": "/works/OL82563W",
					"title": "Many Authors Work",
					"authors": [%s]
				}`, authorsList)

			case "/works/OL82563W/editions.json":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"entries": []}`))

			default:
				// Author lookup /authors/OL...A.json
				atomic.AddInt32(&authorLookups, 1)
				if r.URL.Path == "/authors/OL5A.json" {
					// Simulate 1 author lookup failing (404/500)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"name": "Author Name"}`))
			}
		}))
		defer ts.Close()

		client := openlibrary.NewClient(ts.URL, "UA", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, ts.Client())
		detail, err := client.GetWork(context.Background(), "OL82563W")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if atomic.LoadInt32(&authorLookups) != 20 {
			t.Errorf("expected exactly 20 author lookups, got %d", authorLookups)
		}
		// 20 were requested, but OL5A failed, so exactly 19 authors should be in the list
		if len(detail.Work.Authors) != 19 {
			t.Errorf("expected 19 authors after 1 dropped, got %d", len(detail.Work.Authors))
		}
	})
}

func TestRateLimiter_ConcurrencyAndWaiterLimit(t *testing.T) {
	t.Run("rejects immediately when waiter cap of 50 is exceeded", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		// Create a rate limiter that is completely exhausted and takes long to refill
		limiter := openlibrary.NewRateLimiter(0.0001, 1, 50, logger)

		// Acquire the single token
		ctx := context.Background()
		if err := limiter.Wait(ctx); err != nil {
			t.Fatalf("unexpected error acquiring first token: %v", err)
		}

		// Now launch 50 background waiters that will block
		var startWg sync.WaitGroup
		startWg.Add(50)
		doneCh := make(chan struct{})

		for i := 0; i < 50; i++ {
			go func() {
				startWg.Done()
				_ = limiter.Wait(ctx)
			}()
		}

		startWg.Wait()
		// Give goroutines a moment to enter Wait
		time.Sleep(50 * time.Millisecond)

		// The 51st waiter should be rejected immediately with domain.Unavailable
		err := limiter.Wait(ctx)
		if err == nil {
			t.Fatal("expected 51st waiter to be rejected, got nil")
		}
		domErr, ok := err.(*domain.Error)
		if !ok || domErr.Category != domain.Unavailable {
			t.Errorf("expected domain.Unavailable, got %v", err)
		}

		close(doneCh)
	})
}

func TestClient_FetchCover(t *testing.T) {
	t.Run("fetches binary cover image successfully", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/b/id/12345-M.jpg" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00"))
		}))
		defer ts.Close()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		client := openlibrary.NewClient("", "UA", logger, nil, ts.Client(), openlibrary.WithCoversBaseURL(ts.URL))

		data, contentType, err := client.FetchCover(context.Background(), 12345)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if contentType != "image/jpeg" {
			t.Errorf("expected image/jpeg, got %s", contentType)
		}
		if len(data) == 0 {
			t.Error("expected non-empty image data")
		}
	})

	t.Run("rejects non-positive cover ID as domain.InvalidInput", func(t *testing.T) {
		client := openlibrary.NewClient("", "UA", nil, nil, nil)
		for _, invalidID := range []int64{0, -1, -999} {
			_, _, err := client.FetchCover(context.Background(), invalidID)
			if err == nil {
				t.Fatalf("expected error for invalid ID %d, got nil", invalidID)
			}
			domErr, ok := err.(*domain.Error)
			if !ok || domErr.Category != domain.InvalidInput {
				t.Errorf("expected domain.InvalidInput, got %v", err)
			}
		}
	})

	t.Run("upstream 404 maps to domain.NotFound", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		client := openlibrary.NewClient("", "UA", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, ts.Client(), openlibrary.WithCoversBaseURL(ts.URL))
		_, _, err := client.FetchCover(context.Background(), 99999)
		if err == nil {
			t.Fatal("expected error for 404, got nil")
		}
		domErr, ok := err.(*domain.Error)
		if !ok || domErr.Category != domain.NotFound {
			t.Errorf("expected domain.NotFound, got %v", err)
		}
	})

	t.Run("upstream 503 maps to domain.Unavailable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer ts.Close()

		client := openlibrary.NewClient("", "UA", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, ts.Client(), openlibrary.WithCoversBaseURL(ts.URL))
		_, _, err := client.FetchCover(context.Background(), 12345)
		if err == nil {
			t.Fatal("expected error for 503, got nil")
		}
		domErr, ok := err.(*domain.Error)
		if !ok || domErr.Category != domain.Unavailable {
			t.Errorf("expected domain.Unavailable, got %v", err)
		}
	})
}

