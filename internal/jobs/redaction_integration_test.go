//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// TestEngine_NoPayloadOrErrorValueIsLogged is the spec's acceptance
// criterion "No job payload, error, or progress value logs a value it
// shouldn't per Security considerations, proven by a test." A handler
// fails with an error containing a secret-shaped marker, against a
// payload containing another; every log record the engine emits while
// the job runs and dead-letters is scanned, and none may carry either
// marker. The stored last_error is truncated to the 512-byte bound.
func TestEngine_NoPayloadOrErrorValueIsLogged(t *testing.T) {
	const payloadSecret = "PAYLOAD-SECRET-9f3a"
	const errorSecret = "ERROR-SECRET-hunter2"

	spy := testutil.NewSpyHandler()
	sys := jobs.NewSystem(migratedPool(t), seqIDs(), jobs.SystemClock{}, slog.New(spy), fastConfig())

	sys.Queue().Register("leaky", 1, func(context.Context, json.RawMessage, jobs.ReportProgressFunc) error {
		return errors.New("upstream rejected the request: token=" + errorSecret + " " + strings.Repeat("x", 2000))
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	id, err := sys.Queue().Enqueue(ctx, "leaky", map[string]string{"apiKey": payloadSecret})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	job := waitForState(t, sys.Queue(), id, jobs.StateDeadLetter)

	// The stored value is truncated; it is allowed to contain the error
	// marker, but never the payload.
	if len(job.LastError.String()) > 512 {
		t.Fatalf("stored last_error is %d bytes, want <= 512", len(job.LastError.String()))
	}
	if strings.Contains(job.LastError.String(), payloadSecret) {
		t.Fatal("stored last_error somehow contains the payload secret")
	}

	// Give any trailing transition logs a moment to land.
	time.Sleep(50 * time.Millisecond)

	records := spy.Records()
	if len(records) == 0 {
		t.Fatal("engine emitted no log records at all — the scan would be vacuous")
	}
	if spy.Contains(payloadSecret) {
		t.Fatal("a log record leaked the job payload")
	}
	if spy.Contains(errorSecret) {
		t.Fatal("a log record leaked the handler error text")
	}

	// The dead_letter transition was logged — proving the scan above
	// actually covered the sensitive-adjacent path, not an empty set.
	sawDeadLetter := false
	for _, r := range records {
		if strings.Contains(r.Message, "dead_letter") {
			sawDeadLetter = true
		}
	}
	if !sawDeadLetter {
		t.Fatal("never saw the dead_letter transition logged")
	}
}
