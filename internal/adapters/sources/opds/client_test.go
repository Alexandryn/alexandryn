package opds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
)

func TestClient_RedirectIsNeverFollowed(t *testing.T) {
	var secondHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		secondHit = true
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := newHTTPClient(sources.NewSemaphore(50), sources.Credential{}, false)
	_, ferr := c.get(context.Background(), srv.URL+"/redirect")
	if ferr == nil || ferr.detail != sources.DetailHTTP3xxUnsupported {
		t.Fatalf("redirect: ferr = %v, want http-3xx-unsupported", ferr)
	}
	if secondHit {
		t.Fatal("the redirect target was fetched — redirects are not disabled")
	}
}

func TestClient_BodySizeCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		buf := make([]byte, 64*1024)
		for i := 0; i < (maxResponseBytes/len(buf))+2; i++ {
			_, _ = w.Write(buf)
		}
	}))
	defer srv.Close()

	c := newHTTPClient(sources.NewSemaphore(50), sources.Credential{}, false)
	_, ferr := c.get(context.Background(), srv.URL)
	if ferr == nil || ferr.detail != sources.DetailUnparseable {
		t.Fatalf("oversize body: ferr = %v, want unparseable", ferr)
	}
}

func TestClient_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := newHTTPClient(sources.NewSemaphore(50), sources.Credential{}, false)
	c.hc.Timeout = 200 * time.Millisecond
	_, ferr := c.get(context.Background(), srv.URL)
	if ferr == nil || ferr.detail != sources.DetailTimeout {
		t.Fatalf("slow server: ferr = %v, want timeout", ferr)
	}
}

func TestClient_StatusClassification(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{401, sources.DetailAuthRejected},
		{403, sources.DetailAuthRejected},
		{404, sources.DetailHTTP4xx},
		{429, sources.DetailHTTP4xx},
		{500, sources.DetailHTTP5xx},
		{503, sources.DetailHTTP5xx},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))
		c := newHTTPClient(sources.NewSemaphore(50), sources.Credential{}, false)
		_, ferr := c.get(context.Background(), srv.URL)
		srv.Close()
		if ferr == nil || ferr.detail != tc.want {
			t.Errorf("status %d: ferr = %v, want %s", tc.status, ferr, tc.want)
		}
	}
}

func TestClient_SendsBasicAuthWhenConfigured(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/opds+json")
		_, _ = w.Write([]byte(`{"links":[{"rel":"self","href":"/x"}]}`))
	}))
	defer srv.Close()

	cred, _ := sources.NewCredential("reader", "p@ss word")
	c := newHTTPClient(sources.NewSemaphore(50), cred, true)
	if _, ferr := c.get(context.Background(), srv.URL); ferr != nil {
		t.Fatalf("get: %v", ferr)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("Authorization header = %q, want Basic", gotAuth)
	}

	// No credential → no header.
	gotAuth = ""
	c2 := newHTTPClient(sources.NewSemaphore(50), sources.Credential{}, false)
	_, _ = c2.get(context.Background(), srv.URL)
	if gotAuth != "" {
		t.Fatalf("unauthenticated request still sent Authorization: %q", gotAuth)
	}
}

func TestClient_SemaphoreFullRejectsImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	sem := sources.NewSemaphore(1)
	if !sem.TryAcquire() {
		t.Fatal("could not pre-fill the semaphore")
	}
	c := newHTTPClient(sem, sources.Credential{}, false)

	start := time.Now()
	_, ferr := c.get(context.Background(), srv.URL)
	if ferr == nil {
		t.Fatal("get succeeded despite a full semaphore")
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("full-semaphore rejection took %v — it should be immediate", time.Since(start))
	}
}
