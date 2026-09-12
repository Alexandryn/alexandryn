package http_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/testutil"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// Full-chain error and logging integration tests: exercises panic recovery
// and logging behavior against the real assembled chain (Recovery, Limits,
// Logging via transporthttp.Chain) rather than individual middlewares wired by hand.

// A panic in a layer positioned before Logging (Limits' position in the real chain)
// must still produce a response with a non-empty correlationId — Recovery's own fallback,
// since Logging never gets a chance to assign one. Distinct from
// TestRecovery_GeneratesAFallbackCorrelationIDWhenNoneIsSet (no Logging in that chain at all)
// and TestRecovery_WrappingLogging_UsesLoggingsRealCorrelationID (panic happens after Logging
// assigns an ID) — this tests the case where Logging is present in the chain but never reached.
func TestFullChain_PanicBeforeLoggingRuns_StillGetsAFallbackCorrelationID(t *testing.T) {
	logger := slog.New(testutil.NewSpyHandler())
	recoveryIDs := testutil.NewFakeIDGenerator("recovery-fallback-id")
	loggingIDs := testutil.NewFakeIDGenerator("logging-should-never-run")

	// Stands in for "a layer positioned before logging" (Limits' own
	// position in the real chain) that panics unconditionally — never a
	// search for a genuine panic-inducing edge case in production
	// limits-checking code, per the test plan's own framing.
	panicBeforeLogging := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("simulated panic before logging assigns a correlation ID")
		})
	}

	routing := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("routing must never run — recovery should have already caught the panic")
	})

	chain := transporthttp.Chain(routing,
		transporthttp.Recovery(logger, recoveryIDs.NewID),
		panicBeforeLogging,
		transporthttp.Logging(logger, loggingIDs.NewID),
	)

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var body struct {
		CorrelationID string `json:"correlationId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body isn't valid JSON: %v", err)
	}
	if body.CorrelationID != "recovery-fallback-id" {
		t.Fatalf("correlationId = %q, want recovery's own fallback (\"recovery-fallback-id\") — logging never ran to assign one", body.CorrelationID)
	}
}

// forbiddenMessagePatterns and messageLeaksInternals ensure error responses
// never leak sensitive internal details, paths, or query fragments to clients.
var forbiddenMessagePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\.go:\d+`),                                                   // stack-trace-shaped (file.go:123)
	regexp.MustCompile(`(?i)goroutine \d+`),                                          // stack trace header
	regexp.MustCompile(`(/[\w.\-]+){2,}`),                                            // unix-shaped filesystem path
	regexp.MustCompile(`[A-Za-z]:\\`),                                                // Windows drive-letter path
	regexp.MustCompile(`(?i)\b(select|insert into|update|delete from|drop table)\b`), // SQL fragment
}

func messageLeaksInternals(msg string) bool {
	for _, pat := range forbiddenMessagePatterns {
		if pat.MatchString(msg) {
			return true
		}
	}
	return false
}

// Regression guard proving the check above actually has teeth — the same
// discipline this codebase already uses for its redaction fixtures
// (a logValuerOnlyStub proving that test can detect the gap it exists to
// catch). Without this, a check that always returns false would pass the
// real exercise below vacuously.
func TestMessageLeaksInternals_DetectsKnownBadPatterns(t *testing.T) {
	cases := []string{
		"panic: runtime error at /home/user/project/internal/foo.go:42",
		"goroutine 7 [running]:",
		"query failed: SELECT * FROM users WHERE id = $1",
		`C:\Users\dev\project\main.go`,
	}
	for _, msg := range cases {
		if !messageLeaksInternals(msg) {
			t.Fatalf("messageLeaksInternals(%q) = false, want true — the check must catch known-bad patterns", msg)
		}
	}
}

// Exercises the construction-to-wire pipeline: a test handler deterministically
// triggers each of the error categories via domain.Error and WriteError behind the
// real middleware chain, verifying no internal details are leaked to clients.
func TestFullChain_ErrorMessagesAcrossAllCategoriesNeverLeakInternals(t *testing.T) {
	logger := slog.New(testutil.NewSpyHandler())
	idCounter := 0
	var idMu sync.Mutex
	newID := func() string {
		idMu.Lock()
		defer idMu.Unlock()
		idCounter++
		return fmt.Sprintf("id-%d", idCounter)
	}

	// The real, production-shaped message each category's own real call
	// site in this codebase actually uses today (NotFoundHandler,
	// Limits, TranslateError, Readyz), not a contrived string chosen to
	// trivially pass the check.
	messages := map[domain.Category]string{
		domain.NotFound:     "no such endpoint",
		domain.InvalidInput: "request body exceeds the maximum allowed size",
		domain.Unauthorized: "authentication is required",
		domain.Conflict:     "the requested change conflicts with an existing record",
		domain.Unavailable:  "not yet started",
		domain.Internal:     "an internal error occurred",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cat := domain.Category(r.URL.Query().Get("category"))
		derr := &domain.Error{Category: cat, Message: messages[cat]}
		transporthttp.WriteError(w, domain.CategoryOf(derr), derr.Error(), transporthttp.CorrelationIDFromContext(r.Context()))
	})

	chain := transporthttp.Chain(handler,
		transporthttp.Recovery(logger, newID),
		transporthttp.Limits(1<<20),
		transporthttp.Logging(logger, newID),
	)

	for cat := range messages {
		t.Run(string(cat), func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/?category="+string(cat), nil)
			chain.ServeHTTP(rec, req)

			var body struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body isn't valid JSON: %v", err)
			}
			if messageLeaksInternals(body.Message) {
				t.Fatalf("category %s: message %q leaks internal detail", cat, body.Message)
			}
		})
	}
}

// Concurrency layer: two panicking requests fired concurrently against
// the real chain, each with a deliberately slow panic path so their
// recovery windows overlap in wall-clock time. Each response must carry
// its own distinct correlationId — proof the correlation-ID mechanism
// (the per-request mutable holder in the request's own context) has no
// shared mutable state that could leak one request's ID or stack trace
// into another's response. Run under go test -race, per the test plan's
// own requirement for this case.
func TestFullChain_ConcurrentPanicsAreIsolated(t *testing.T) {
	logger := slog.New(testutil.NewSpyHandler())
	ids := testutil.NewFakeIDGenerator("concurrent-id-1", "concurrent-id-2")

	slowPanic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		panic("boom from request " + r.URL.Query().Get("req"))
	})

	chain := transporthttp.Chain(slowPanic,
		transporthttp.Recovery(logger, ids.NewID),
		transporthttp.Logging(logger, ids.NewID),
	)

	const n = 2
	results := make(chan string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?req=%d", i), nil)
			chain.ServeHTTP(rec, req)

			var body struct {
				CorrelationID string `json:"correlationId"`
				Code          string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Errorf("request %d: response body isn't valid JSON: %v", i, err)
				results <- ""
				return
			}
			if body.Code != string(domain.Internal) {
				t.Errorf("request %d: code = %q, want Internal", i, body.Code)
			}
			results <- body.CorrelationID
		}(i)
	}
	wg.Wait()
	close(results)

	seen := make(map[string]bool, n)
	for id := range results {
		if id == "" {
			t.Fatal("a concurrent panic produced an empty correlationId")
		}
		if seen[id] {
			t.Fatalf("duplicate correlationId %q across concurrent panics — one request's recovery observably affected the other's", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d distinct correlation IDs, want %d", len(seen), n)
	}
}
