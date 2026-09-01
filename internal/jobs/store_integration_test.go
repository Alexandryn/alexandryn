//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/jobs"
)

const testLease = 60 * time.Second

func newStore(t *testing.T) *jobs.Store {
	t.Helper()
	return jobs.NewStore(migratedPool(t), seqIDs(), testLease)
}

func mustEnqueue(t *testing.T, s *jobs.Store, id, kind string, maxAttempts int, availableAt, now time.Time) {
	t.Helper()
	err := s.Enqueue(context.Background(), jobs.NewJob{
		ID:          jobs.ID(id),
		Kind:        jobs.Kind(kind),
		Payload:     json.RawMessage(`{"n":1}`),
		MaxAttempts: maxAttempts,
		AvailableAt: availableAt,
		Now:         now,
	})
	if err != nil {
		t.Fatalf("Enqueue(%s): %v", id, err)
	}
}

func TestStore_EnqueueThenGetJob(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "job-1", "synthetic", 5, baseTime, baseTime)

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.State != jobs.StateQueued {
		t.Fatalf("State = %q, want queued", got.State)
	}
	if got.Attempts != 0 || got.MaxAttempts != 5 {
		t.Fatalf("attempts=%d max=%d, want 0/5", got.Attempts, got.MaxAttempts)
	}
	var payload struct {
		N int `json:"n"`
	}
	if err := json.Unmarshal(got.Payload, &payload); err != nil || payload.N != 1 {
		t.Fatalf("payload = %s (unmarshal err %v)", got.Payload, err)
	}
	if got.Progress != nil {
		t.Fatalf("progress = %+v, want nil", got.Progress)
	}
}

func TestStore_GetJob_NotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.GetJob(context.Background(), "nope")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestStore_ClaimNext_MovesToRunningAndIncrementsAttempts(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "job-1", "synthetic", 3, baseTime, baseTime)

	claimed, err := s.ClaimNext(ctx, "worker-a", nil, baseTime)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed == nil {
		t.Fatal("ClaimNext returned nil, want the queued job")
	}
	if claimed.State != jobs.StateRunning {
		t.Fatalf("State = %q, want running", claimed.State)
	}
	if claimed.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", claimed.Attempts)
	}
	if claimed.LeaseToken == "" || claimed.LockedBy != "worker-a" {
		t.Fatalf("lease=%q lockedBy=%q, want set", claimed.LeaseToken, claimed.LockedBy)
	}
	if claimed.LockedUntil == nil || !claimed.LockedUntil.Equal(baseTime.Add(testLease)) {
		t.Fatalf("LockedUntil = %v, want %v", claimed.LockedUntil, baseTime.Add(testLease))
	}
}

func TestStore_ClaimNext_NothingClaimable(t *testing.T) {
	s := newStore(t)
	// available_at in the future
	mustEnqueue(t, s, "job-1", "synthetic", 3, baseTime.Add(time.Hour), baseTime)

	claimed, err := s.ClaimNext(context.Background(), "worker-a", nil, baseTime)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed != nil {
		t.Fatalf("ClaimNext = %+v, want nil (not yet available)", claimed)
	}
}

func TestStore_ClaimNext_OldestFirstAndKindFilter(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "old-import", "import", 3, baseTime.Add(-2*time.Minute), baseTime)
	mustEnqueue(t, s, "new-sync", "sync", 3, baseTime.Add(-1*time.Minute), baseTime)

	// Filtered to "sync" only: skips the older import job.
	claimed, err := s.ClaimNext(ctx, "w", []jobs.Kind{"sync"}, baseTime)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed == nil || claimed.ID != "new-sync" {
		t.Fatalf("claimed = %+v, want new-sync", claimed)
	}

	// Unfiltered: the remaining (older) import job.
	claimed, err = s.ClaimNext(ctx, "w", nil, baseTime)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed == nil || claimed.ID != "old-import" {
		t.Fatalf("claimed = %+v, want old-import", claimed)
	}
}

func TestStore_Heartbeat_ExtendsLeaseAndFences(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "job-1", "synthetic", 3, baseTime, baseTime)
	claimed, _ := s.ClaimNext(ctx, "w", nil, baseTime)

	later := baseTime.Add(20 * time.Second)
	alive, err := s.Heartbeat(ctx, claimed.ID, claimed.LeaseToken, later)
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if !alive {
		t.Fatal("Heartbeat reported not-alive for the current lease token")
	}
	got, _ := s.GetJob(ctx, claimed.ID)
	if !got.LockedUntil.Equal(later.Add(testLease)) {
		t.Fatalf("LockedUntil = %v, want %v", got.LockedUntil, later.Add(testLease))
	}

	// A stale token heartbeats zero rows.
	alive, err = s.Heartbeat(ctx, claimed.ID, "stale-token", later)
	if err != nil {
		t.Fatalf("Heartbeat(stale): %v", err)
	}
	if alive {
		t.Fatal("Heartbeat with a stale token reported alive")
	}
}

func TestStore_Complete_FencedOnLeaseToken(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "job-1", "synthetic", 3, baseTime, baseTime)
	claimed, _ := s.ClaimNext(ctx, "w", nil, baseTime)

	ok, err := s.Complete(ctx, claimed.ID, "wrong-token", baseTime)
	if err != nil {
		t.Fatalf("Complete(wrong): %v", err)
	}
	if ok {
		t.Fatal("Complete with a wrong token affected a row")
	}

	ok, err = s.Complete(ctx, claimed.ID, claimed.LeaseToken, baseTime.Add(time.Second))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !ok {
		t.Fatal("Complete with the right token affected no row")
	}
	got, _ := s.GetJob(ctx, claimed.ID)
	if got.State != jobs.StateCompleted || got.CompletedAt == nil {
		t.Fatalf("state=%q completedAt=%v, want completed/set", got.State, got.CompletedAt)
	}
}

func TestStore_Fail_RetryingThenDeadLetter(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "job-1", "synthetic", 2, baseTime, baseTime)

	// Attempt 1 fails, under the limit -> retrying.
	claimed, _ := s.ClaimNext(ctx, "w", nil, baseTime)
	nextRun := baseTime.Add(5 * time.Second)
	ok, err := s.Fail(ctx, claimed.ID, claimed.LeaseToken, baseTime, nextRun, false, errString("temporary glitch"))
	if err != nil || !ok {
		t.Fatalf("Fail #1: ok=%v err=%v", ok, err)
	}
	got, _ := s.GetJob(ctx, claimed.ID)
	if got.State != jobs.StateRetrying || !got.AvailableAt.Equal(nextRun) {
		t.Fatalf("after fail #1: state=%q availableAt=%v", got.State, got.AvailableAt)
	}
	if got.LastError.String() != "temporary glitch" {
		t.Fatalf("LastError = %q", got.LastError)
	}

	// Attempt 2 (claimed from retrying) fails at the limit -> dead_letter.
	claimed, err = s.ClaimNext(ctx, "w", nil, nextRun)
	if err != nil || claimed == nil {
		t.Fatalf("ClaimNext retry: %v / %v", claimed, err)
	}
	if claimed.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2", claimed.Attempts)
	}
	ok, err = s.Fail(ctx, claimed.ID, claimed.LeaseToken, nextRun, nextRun, true, errString("still broken"))
	if err != nil || !ok {
		t.Fatalf("Fail #2: ok=%v err=%v", ok, err)
	}
	got, _ = s.GetJob(ctx, claimed.ID)
	if got.State != jobs.StateDeadLetter {
		t.Fatalf("after fail #2: state=%q, want dead_letter", got.State)
	}

	// A dead-lettered job is never claimable again.
	claimed, err = s.ClaimNext(ctx, "w", nil, nextRun.Add(time.Hour))
	if err != nil {
		t.Fatalf("ClaimNext after dead-letter: %v", err)
	}
	if claimed != nil {
		t.Fatalf("dead-lettered job was claimed again: %+v", claimed)
	}
}

func TestStore_RecoverStale_ReclaimsWithNewTokenAndRetryLogic(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "crashed", "synthetic", 3, baseTime, baseTime)
	claimed, _ := s.ClaimNext(ctx, "dead-worker", nil, baseTime)
	originalToken := claimed.LeaseToken

	// Sweep while the lease is still valid: nothing to do.
	n, err := s.RecoverStale(ctx, baseTime.Add(30*time.Second), func(int) time.Duration { return time.Second })
	if err != nil {
		t.Fatalf("RecoverStale (valid lease): %v", err)
	}
	if n != 0 {
		t.Fatalf("recovered %d, want 0 while the lease is valid", n)
	}

	// Sweep after the lease expired: reclaimed as a retry.
	afterExpiry := baseTime.Add(90 * time.Second)
	n, err = s.RecoverStale(ctx, afterExpiry, func(attempts int) time.Duration {
		if attempts != 1 {
			t.Errorf("backoffFor got attempts=%d, want 1 (already incremented at claim)", attempts)
		}
		return 10 * time.Second
	})
	if err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if n != 1 {
		t.Fatalf("recovered %d, want 1", n)
	}
	got, _ := s.GetJob(ctx, "crashed")
	if got.State != jobs.StateRetrying {
		t.Fatalf("state = %q, want retrying", got.State)
	}
	if got.LeaseToken == originalToken {
		t.Fatal("lease token was not regenerated on reclaim")
	}
	if got.LastError.String() != "worker lease expired without heartbeat" {
		t.Fatalf("LastError = %q", got.LastError)
	}
	if !got.AvailableAt.Equal(afterExpiry.Add(10 * time.Second)) {
		t.Fatalf("AvailableAt = %v, want %v", got.AvailableAt, afterExpiry.Add(10*time.Second))
	}

	// The original worker's fenced writes now hit zero rows.
	alive, _ := s.Heartbeat(ctx, "crashed", originalToken, afterExpiry)
	if alive {
		t.Fatal("stale worker heartbeat succeeded after reclaim")
	}
	ok, _ := s.Complete(ctx, "crashed", originalToken, afterExpiry)
	if ok {
		t.Fatal("stale worker completion succeeded after reclaim")
	}
}

func TestStore_RecoverStale_DeadLettersAtAttemptLimit(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "j", "synthetic", 1, baseTime, baseTime)
	_, _ = s.ClaimNext(ctx, "w", nil, baseTime) // attempts -> 1, == max

	n, err := s.RecoverStale(ctx, baseTime.Add(2*time.Minute), func(int) time.Duration { return time.Second })
	if err != nil || n != 1 {
		t.Fatalf("RecoverStale: n=%d err=%v", n, err)
	}
	got, _ := s.GetJob(ctx, "j")
	if got.State != jobs.StateDeadLetter {
		t.Fatalf("state = %q, want dead_letter (attempts == max_attempts)", got.State)
	}
}

func TestStore_UpdateProgress(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "j", "synthetic", 3, baseTime, baseTime)
	claimed, _ := s.ClaimNext(ctx, "w", nil, baseTime)

	if err := s.UpdateProgress(ctx, claimed.ID, claimed.LeaseToken, 3, 40, baseTime); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}
	got, _ := s.GetJob(ctx, claimed.ID)
	if got.Progress == nil || got.Progress.Current != 3 || got.Progress.Total != 40 {
		t.Fatalf("Progress = %+v, want {3,40}", got.Progress)
	}

	// Stale token: silently a no-op, not an error.
	if err := s.UpdateProgress(ctx, claimed.ID, "stale", 9, 40, baseTime); err != nil {
		t.Fatalf("UpdateProgress(stale) returned error: %v", err)
	}
	got, _ = s.GetJob(ctx, claimed.ID)
	if got.Progress.Current != 3 {
		t.Fatalf("stale progress write took effect: %+v", got.Progress)
	}
}

func TestStore_ListJobs_FilterByKindAndState(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	mustEnqueue(t, s, "a", "import", 3, baseTime, baseTime)
	mustEnqueue(t, s, "b", "import", 3, baseTime.Add(time.Second), baseTime.Add(time.Second))
	mustEnqueue(t, s, "c", "sync", 3, baseTime.Add(2*time.Second), baseTime.Add(2*time.Second))
	claimed, _ := s.ClaimNext(ctx, "w", []jobs.Kind{"import"}, baseTime.Add(time.Minute))
	_, _ = s.Complete(ctx, claimed.ID, claimed.LeaseToken, baseTime.Add(time.Minute))

	all, err := s.ListJobs(ctx, jobs.JobFilter{})
	if err != nil || len(all) != 3 {
		t.Fatalf("ListJobs(all) = %d jobs, %v", len(all), err)
	}
	imports, _ := s.ListJobs(ctx, jobs.JobFilter{Kind: "import"})
	if len(imports) != 2 {
		t.Fatalf("ListJobs(import) = %d, want 2", len(imports))
	}
	completed, _ := s.ListJobs(ctx, jobs.JobFilter{State: jobs.StateCompleted})
	if len(completed) != 1 || completed[0].ID != "a" {
		t.Fatalf("ListJobs(completed) = %+v, want [a]", completed)
	}
}

// TestStore_ConcurrentClaim_NeverDoubleClaims is review 0036's required
// proof: N goroutines on real pooled connections race ClaimNext against
// M < N claimable jobs. Exactly M claims, zero duplicates, attempts == 1
// on every claimed row.
func TestStore_ConcurrentClaim_NeverDoubleClaims(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	const jobCount = 8
	const workers = 20
	for i := 0; i < jobCount; i++ {
		mustEnqueue(t, s, "job-"+itoa(i), "synthetic", 3, baseTime, baseTime)
	}

	var (
		mu      sync.Mutex
		claimed = map[jobs.ID]int{}
		start   = make(chan struct{})
		wg      sync.WaitGroup
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for {
				job, err := s.ClaimNext(ctx, "worker-"+itoa(w), nil, baseTime)
				if err != nil {
					t.Errorf("worker %d ClaimNext: %v", w, err)
					return
				}
				if job == nil {
					return
				}
				if job.Attempts != 1 {
					t.Errorf("job %s claimed with attempts=%d", job.ID, job.Attempts)
				}
				mu.Lock()
				claimed[job.ID]++
				mu.Unlock()
			}
		}(w)
	}
	close(start)
	wg.Wait()

	if len(claimed) != jobCount {
		t.Fatalf("claimed %d distinct jobs, want %d", len(claimed), jobCount)
	}
	for id, n := range claimed {
		if n != 1 {
			t.Fatalf("job %s claimed %d times — double execution", id, n)
		}
	}
}

// errString is a tiny error whose message is exactly s.
type errStringT string

func (e errStringT) Error() string { return string(e) }

func errString(s string) error { return errStringT(s) }
