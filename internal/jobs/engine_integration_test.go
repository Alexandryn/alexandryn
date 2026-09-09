//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// fastConfig runs the pool on millisecond intervals so a test does not
// wait out the production 2s/20s/30s cadence. LeaseDuration stays long
// (relative to a sub-second test) so a happy-path job is never reaped.
func fastConfig() jobs.Config {
	return jobs.Config{
		Concurrency:         2,
		PollInterval:        5 * time.Millisecond,
		LeaseDuration:       60 * time.Second,
		HeartbeatInterval:   5 * time.Millisecond,
		ReaperInterval:      5 * time.Millisecond,
		Backoff:             jobs.Backoff{Base: 2 * time.Millisecond, Max: 50 * time.Millisecond},
		ShutdownGracePeriod: time.Second,
	}
}

// waitForState polls GetJob until the job reaches want or the deadline
// passes.
func waitForState(t *testing.T, q *jobs.Queue, id jobs.ID, want jobs.State) jobs.Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := q.GetJob(context.Background(), id)
		if err == nil && job.State == want {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	job, _ := q.GetJob(context.Background(), id)
	t.Fatalf("job %s never reached %q (last state %q, attempts %d, lastErr %q)", id, want, job.State, job.Attempts, job.LastError)
	return jobs.Job{}
}

func newSystem(t *testing.T, clock jobs.Clock, cfg jobs.Config) *jobs.System {
	t.Helper()
	return jobs.NewSystem(migratedPool(t), seqIDs(), clock, discardLogger(), cfg)
}

// TestEngine_SyntheticWalkthrough is the spec's E2E row: a job that
// fails transiently twice then succeeds on the third attempt, with
// GetJob reflecting the transitions.
func TestEngine_SyntheticWalkthrough(t *testing.T) {
	sys := newSystem(t, jobs.SystemClock{}, fastConfig())

	var attempts atomic.Int32
	var sawRetrying atomic.Bool
	sys.Queue().Register("flaky", 5, func(ctx context.Context, payload json.RawMessage, report jobs.ReportProgressFunc) error {
		n := attempts.Add(1)
		report(int(n), 3)
		if n < 3 {
			return errors.New("transient upstream failure")
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	id, err := sys.Queue().Enqueue(ctx, "flaky", map[string]string{"ref": "abc"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Observe a retry along the way.
	go func() {
		for i := 0; i < 200; i++ {
			if j, err := sys.Queue().GetJob(context.Background(), id); err == nil && j.State == jobs.StateRetrying {
				sawRetrying.Store(true)
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	final := waitForState(t, sys.Queue(), id, jobs.StateCompleted)
	if attempts.Load() != 3 {
		t.Fatalf("handler ran %d times, want 3", attempts.Load())
	}
	if final.Attempts != 3 {
		t.Fatalf("final Attempts = %d, want 3", final.Attempts)
	}
	if final.Progress == nil || final.Progress.Current != 3 {
		t.Fatalf("final Progress = %+v, want current 3", final.Progress)
	}
	if !sawRetrying.Load() {
		t.Fatal("never observed the job in the retrying state")
	}
}

// TestEngine_PermanentFailureDeadLettersImmediately: a handler returning
// jobs.Permanent(err) is dead-lettered on the first attempt regardless
// of the attempt budget.
func TestEngine_PermanentFailureDeadLettersImmediately(t *testing.T) {
	sys := newSystem(t, jobs.SystemClock{}, fastConfig())

	var runs atomic.Int32
	sys.Queue().Register("permanent", 5, func(context.Context, json.RawMessage, jobs.ReportProgressFunc) error {
		runs.Add(1)
		return jobs.Permanent(errors.New("source returned 404"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	id, _ := sys.Queue().Enqueue(ctx, "permanent", nil)
	job := waitForState(t, sys.Queue(), id, jobs.StateDeadLetter)
	if job.Attempts != 1 {
		t.Fatalf("dead-lettered after %d attempts, want 1", job.Attempts)
	}
	time.Sleep(50 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("handler ran %d times, want 1 (no retry on a Permanent failure)", runs.Load())
	}
}

// TestEngine_ExhaustsAttemptsThenDeadLetters: an always-failing handler
// retries up to max_attempts, then dead-letters.
func TestEngine_ExhaustsAttemptsThenDeadLetters(t *testing.T) {
	sys := newSystem(t, jobs.SystemClock{}, fastConfig())

	var runs atomic.Int32
	sys.Queue().Register("always-fails", 3, func(context.Context, json.RawMessage, jobs.ReportProgressFunc) error {
		runs.Add(1)
		return errors.New("still broken")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	id, _ := sys.Queue().Enqueue(ctx, "always-fails", nil)
	job := waitForState(t, sys.Queue(), id, jobs.StateDeadLetter)
	if job.Attempts != 3 {
		t.Fatalf("dead-lettered after %d attempts, want 3", job.Attempts)
	}
}

// TestEngine_PanicIsRecoveredAndTreatedAsFailure: a panicking handler
// never takes down the pool; the job is retried like any error.
func TestEngine_PanicIsRecoveredAndTreatedAsFailure(t *testing.T) {
	sys := newSystem(t, jobs.SystemClock{}, fastConfig())

	var runs atomic.Int32
	sys.Queue().Register("panics", 2, func(context.Context, json.RawMessage, jobs.ReportProgressFunc) error {
		if runs.Add(1) == 1 {
			panic("boom")
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	id, _ := sys.Queue().Enqueue(ctx, "panics", nil)
	// First attempt panics -> retry; second succeeds.
	waitForState(t, sys.Queue(), id, jobs.StateCompleted)
}

// TestEngine_FencingAfterReclaim: the worker's heartbeat is delayed
// past the lease (HeartbeatInterval > LeaseDuration here, simulating the
// GC-pause / starved-goroutine race FR-5 names). The reaper reclaims the
// job with a fresh token while the original handler still runs; the late
// heartbeat then detects the lost lease and cancels the handler, and the
// original worker writes no result — the row stays the reaper's, never a
// completion (the database-write half of "no double execution").
func TestEngine_FencingAfterReclaim(t *testing.T) {
	cfg := jobs.Config{
		Concurrency:       1,
		PollInterval:      5 * time.Millisecond,
		LeaseDuration:     80 * time.Millisecond,
		HeartbeatInterval: time.Second, // fires well after the lease expires
		ReaperInterval:    10 * time.Millisecond,
		Backoff:           jobs.Backoff{Base: time.Second, Max: time.Minute},
	}
	sys := newSystem(t, jobs.SystemClock{}, cfg)

	cancelled := make(chan struct{})
	release := make(chan struct{})
	sys.Queue().Register("slow", 5, func(ctx context.Context, _ json.RawMessage, _ jobs.ReportProgressFunc) error {
		select {
		case <-ctx.Done():
			close(cancelled)
			return ctx.Err()
		case <-release:
			return nil
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() {
		close(release)
		_ = sys.Shutdown(context.Background())
	})

	id, _ := sys.Queue().Enqueue(ctx, "slow", nil)
	running := waitForState(t, sys.Queue(), id, jobs.StateRunning)
	originalToken := running.LeaseToken

	// The reaper reclaims the job once its 80ms lease lapses.
	reclaimed := waitForState(t, sys.Queue(), id, jobs.StateRetrying)
	if reclaimed.LeaseToken == originalToken {
		t.Fatal("lease token was not regenerated by the reaper")
	}
	if reclaimed.LastError.String() != "worker lease expired without heartbeat" {
		t.Fatalf("LastError = %q", reclaimed.LastError)
	}

	// The delayed heartbeat eventually detects the loss and cancels the
	// handler.
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("handler was never cancelled after its lease was reclaimed")
	}

	// The stale worker's completion attempt is fenced: the row stays the
	// reaper's retrying state, never flips to completed.
	time.Sleep(100 * time.Millisecond)
	job, _ := sys.Queue().GetJob(context.Background(), id)
	if job.State == jobs.StateCompleted {
		t.Fatal("stale worker overwrote the reaper's result with a completion")
	}
}

// TestEngine_ReaperRecoversAbandonedJob: a job claimed but never
// heartbeated (a crashed worker) is returned to the queue by the
// engine's reaper goroutine.
func TestEngine_ReaperRecoversAbandonedJob(t *testing.T) {
	clock := testutil.NewFakeClock(baseTime)
	pool := migratedPool(t)
	store := jobs.NewStore(pool, seqIDs(), 60*time.Second)

	// Enqueue and claim directly — no engine poller runs this kind.
	if err := store.Enqueue(context.Background(), jobs.NewJob{
		ID: "abandoned", Kind: "unheld", Payload: json.RawMessage(`{}`),
		MaxAttempts: 3, AvailableAt: baseTime, Now: baseTime,
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := store.ClaimNext(context.Background(), "crashed-worker", nil, baseTime); err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}

	// An engine with no registered handlers: pollers idle, reaper runs.
	sys := jobs.NewSystem(pool, seqIDs(), clock, discardLogger(), fastConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	clock.Advance(120 * time.Second)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := store.GetJob(context.Background(), "abandoned")
		if err == nil && job.State == jobs.StateRetrying {
			if job.LastError.String() != "worker lease expired without heartbeat" {
				t.Fatalf("LastError = %q", job.LastError)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("reaper never recovered the abandoned job")
}

// TestEngine_ShutdownCancelsHandlersAndLeavesRowIntact: on shutdown a
// running handler's context is cancelled; a cooperative handler returns
// promptly and shutdown completes cleanly, and the job row is left
// `running` for the reaper with no synthetic failure written (FR-10).
func TestEngine_ShutdownCancelsHandlersAndLeavesRowIntact(t *testing.T) {
	cfg := fastConfig()
	cfg.Concurrency = 1
	cfg.HeartbeatInterval = 10 * time.Second // don't fire during the test
	sys := newSystem(t, jobs.SystemClock{}, cfg)

	handlerEntered := make(chan struct{})
	sys.Queue().Register("blocks", 5, func(ctx context.Context, _ json.RawMessage, _ jobs.ReportProgressFunc) error {
		close(handlerEntered)
		<-ctx.Done()
		return ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)

	id, _ := sys.Queue().Enqueue(ctx, "blocks", nil)
	<-handlerEntered
	running, _ := sys.Queue().GetJob(context.Background(), id)
	if running.State != jobs.StateRunning {
		t.Fatalf("state = %q, want running", running.State)
	}

	shutdownCtx, sc := context.WithTimeout(context.Background(), 2*time.Second)
	defer sc()
	if err := sys.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown err = %v, want nil (the handler cooperates with cancellation)", err)
	}

	after, _ := sys.Queue().GetJob(context.Background(), id)
	if after.State != jobs.StateRunning {
		t.Fatalf("state after shutdown = %q, want still running (reaper will recover it)", after.State)
	}
	if after.LeaseToken != running.LeaseToken {
		t.Fatal("shutdown mutated the lease token")
	}
}

// TestEngine_ShutdownAbandonsUncooperativeHandler: a handler that ignores
// its cancelled context is abandoned when the grace period expires;
// Shutdown returns the deadline error and the row is left intact.
func TestEngine_ShutdownAbandonsUncooperativeHandler(t *testing.T) {
	cfg := fastConfig()
	cfg.Concurrency = 1
	cfg.HeartbeatInterval = 10 * time.Second
	sys := newSystem(t, jobs.SystemClock{}, cfg)

	entered := make(chan struct{})
	release := make(chan struct{})
	sys.Queue().Register("uncoop", 5, func(context.Context, json.RawMessage, jobs.ReportProgressFunc) error {
		close(entered)
		<-release // ignores ctx entirely
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { close(release) })

	id, _ := sys.Queue().Enqueue(ctx, "uncoop", nil)
	<-entered

	shutdownCtx, sc := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer sc()
	if err := sys.Shutdown(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown err = %v, want DeadlineExceeded", err)
	}

	after, _ := sys.Queue().GetJob(context.Background(), id)
	if after.State != jobs.StateRunning {
		t.Fatalf("state = %q, want still running", after.State)
	}
}

// audit 0016 #299: an admin cancel must abort the running handler's
// context immediately, not leave it running until the next heartbeat.
// The heartbeat interval here is far longer than the test's patience.
func TestEngine_CancelJobAbortsHandlerContextImmediately(t *testing.T) {
	cfg := fastConfig()
	cfg.HeartbeatInterval = 30 * time.Second

	sys := newSystem(t, jobs.SystemClock{}, cfg)

	started := make(chan struct{})
	var ctxErr atomic.Value // error
	finished := make(chan struct{})
	sys.Queue().Register("blocker", 1, func(ctx context.Context, _ json.RawMessage, _ jobs.ReportProgressFunc) error {
		close(started)
		<-ctx.Done()
		ctxErr.Store(ctx.Err())
		close(finished)
		return ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sys.Start(ctx)
	t.Cleanup(func() { _ = sys.Shutdown(context.Background()) })

	id, err := sys.Queue().Enqueue(ctx, "blocker", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never started")
	}

	if err := sys.Queue().CancelJob(context.Background(), id); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("handler context was not cancelled within 2s of the admin cancel (heartbeat interval is 30s)")
	}
	if err, _ := ctxErr.Load().(error); !errors.Is(err, context.Canceled) {
		t.Fatalf("handler ctx.Err() = %v, want context.Canceled", err)
	}

	job := waitForState(t, sys.Queue(), id, jobs.StateDeadLetter)
	if job.State != jobs.StateDeadLetter {
		t.Fatalf("state = %q, want dead_letter", job.State)
	}
}
