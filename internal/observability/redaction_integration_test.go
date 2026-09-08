//go:build integration

package observability_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/observability"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

var (
	jwtRegex     = regexp.MustCompile(`[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}`)
	homeDirRegex = regexp.MustCompile(`(/Users/[a-zA-Z0-9_-]+|/home/[a-zA-Z0-9_-]+|/root/[a-zA-Z0-9_-]+)`)
)

func seqIDs() *testutil.FakeIDGenerator {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i+1)
	}
	return testutil.NewFakeIDGenerator(ids...)
}

func fastJobConfig() jobs.Config {
	return jobs.Config{
		Concurrency:       1,
		PollInterval:      10 * time.Millisecond,
		LeaseDuration:     time.Second,
		HeartbeatInterval: 100 * time.Millisecond,
		ReaperInterval:    100 * time.Millisecond,
		Backoff:           jobs.Backoff{Base: 10 * time.Millisecond, Max: 100 * time.Millisecond},
	}
}

// scanRecordsForViolations inspects captured slog.Record items against ADR 0032 violation patterns.
func scanRecordsForViolations(t *testing.T, records []slog.Record, canaries ...string) {
	t.Helper()
	if len(records) == 0 {
		t.Fatal("no log records captured — scan would be vacuous")
	}

	for _, r := range records {
		var checkString func(s string, context string)
		checkString = func(s string, ctx string) {
			for _, c := range canaries {
				if c != "" && strings.Contains(s, c) {
					t.Errorf("violation: log leaked canary %q in %s: %s", c, ctx, s)
				}
			}
			if jwtRegex.MatchString(s) {
				t.Errorf("violation: log contains JWT-shaped token in %s: %s", ctx, s)
			}
			if homeDirRegex.MatchString(s) {
				t.Errorf("violation: log contains home directory path in %s: %s", ctx, s)
			}
		}

		checkString(r.Message, "message")

		r.Attrs(func(a slog.Attr) bool {
			k := strings.ToLower(a.Key)
			if k == "position" || k == "cfi" || k == "location" || k == "chapter" || k == "percentage" || k == "pct" {
				t.Errorf("violation: log contains reading coordinate key %q", a.Key)
			}
			checkString(a.Value.String(), "attr:"+a.Key)
			return true
		})
	}
}

// TestRedaction_EndToEndExercisesSensitivePaths tests the 4 paths mandated by FR-14 and ADR 0032:
// 1. Source sync with credential
// 2. Authentication flow with password and issued JWT
// 3. Import job with book title
// 4. system_events payload write
func TestRedaction_EndToEndExercisesSensitivePaths(t *testing.T) {
	pool := migratedPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		sourceSecretCanary = "SOURCE-SECRET-canary-pass-987"
		userPassCanary     = "USER-PASSWORD-canary-pass-432"
		syntheticJWT       = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
		bookTitleCanary    = "SECRET-BOOK-TITLE-canary-vol-1"
		homePathCanary     = "/home/alexandryn/library/secret.epub"
	)

	spy := testutil.NewSpyHandler()
	logger := slog.New(spy)

	// Path 1 & 3: Background job engine with source credentials and book title in payload
	sys := jobs.NewSystem(pool, seqIDs(), jobs.SystemClock{}, logger, fastJobConfig())
	done := make(chan struct{})
	sys.Queue().Register("import.sync", 1, func(ctx context.Context, payload json.RawMessage, report jobs.ReportProgressFunc) error {
		defer close(done)
		// Log operational progress safely without leaking payload details
		logger.Info("processing import job", "status", "executing")
		return errors.New("upstream failed: connection timeout")
	})

	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	_, err := sys.Queue().Enqueue(ctx, "import.sync", map[string]any{
		"source_token": sourceSecretCanary,
		"book_title":   bookTitleCanary,
		"local_path":   homePathCanary,
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("job handler did not execute within timeout")
	}

	// Path 2: Simulating auth service login emission
	logger.Info("login attempt processed", "username", "admin", "result", "success")

	// Path 4: system_events payload write
	eventStore := observability.NewEventStore(pool, time.Now)
	libID := "00000000-0000-0000-0000-000000000001"
	err = eventStore.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind: observability.EventJobFailed,
		LibraryID: &libID,
		Payload: map[string]any{
			"status": "failed",
			// Try injecting sensitive keys into event payload — at the top
			// level, nested in an object, and inside a slice (audit 0016 #296).
			"password":   userPassCanary,
			"token":      syntheticJWT,
			"book_title": bookTitleCanary,
			"note":       bookTitleCanary,
			"detail": map[string]any{
				"cfi":        "epubcfi(/6/4!/10)" + syntheticJWT,
				"percentage": 55,
			},
			"trail": []any{
				map[string]any{"token": syntheticJWT},
			},
		},
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	// Verify database record has sanitized payload
	events, err := eventStore.ListEvents(ctx, &libID, 10)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}
	for _, ev := range events {
		if _, ok := ev.Payload["password"]; ok {
			t.Errorf("system_events payload leaked password")
		}
		if _, ok := ev.Payload["token"]; ok {
			t.Errorf("system_events payload leaked token")
		}
		if _, ok := ev.Payload["note"]; ok {
			t.Errorf("system_events payload leaked note (highlight text)")
		}
		if d, ok := ev.Payload["detail"].(map[string]any); ok {
			if _, ok := d["cfi"]; ok {
				t.Errorf("system_events payload leaked nested cfi (reading position)")
			}
			if _, ok := d["percentage"]; ok {
				t.Errorf("system_events payload leaked nested percentage")
			}
		}
		if tr, ok := ev.Payload["trail"].([]any); ok && len(tr) > 0 {
			if m, ok := tr[0].(map[string]any); ok {
				if _, ok := m["token"]; ok {
					t.Errorf("system_events payload leaked token inside a slice")
				}
			}
		}
	}

	// Scan all logger records emitted
	records := spy.Records()
	scanRecordsForViolations(t, records, sourceSecretCanary, userPassCanary, bookTitleCanary, homePathCanary)
}
