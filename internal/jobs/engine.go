package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// terminalWriteTimeout bounds a completion/failure/reaper write that
// runs on a context detached from the poll loop's own — long enough for
// a healthy database, short enough that a stuck write cannot wedge
// shutdown.
const terminalWriteTimeout = 5 * time.Second

// Engine is the worker pool: Concurrency poller goroutines claiming and
// running jobs, plus one reaper goroutine reclaiming stale ones
// (backend-job-queue.md FR-4/FR-5). It is started once and shut down
// once.
type Engine struct {
	store    *Store
	registry *Registry
	clock    Clock
	ids      domain.IDGenerator
	cfg      Config
	logger   *slog.Logger
	live     *liveJobs

	// jitter yields a value in [0,1) for the backoff spread. Injected so
	// a test is deterministic; production uses math/rand/v2 (goroutine-
	// safe, no dependency).
	jitter func() float64

	startOnce sync.Once
	wg        sync.WaitGroup
	mu        sync.Mutex // guards cancel
	cancel    context.CancelFunc
	paused    atomic.Bool
}

// Pause halts claiming of new jobs by pollers.
func (e *Engine) Pause() {
	e.paused.Store(true)
}

// Resume resumes claiming of new jobs by pollers.
func (e *Engine) Resume() {
	e.paused.Store(false)
}

// IsPaused reports whether worker pollers are paused.
func (e *Engine) IsPaused() bool {
	return e.paused.Load()
}

// NewEngine builds a worker pool. Any zero field of cfg is filled from
// DefaultConfig.
func NewEngine(store *Store, registry *Registry, clock Clock, ids domain.IDGenerator, logger *slog.Logger, cfg Config, live *liveJobs) *Engine {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Engine{
		store:    store,
		registry: registry,
		clock:    clock,
		ids:      ids,
		cfg:      cfg.withDefaults(),
		logger:   logger,
		live:     live,
		jitter:   rand.Float64,
	}
}

// Start launches the pollers and the reaper. It returns immediately;
// the goroutines run until Shutdown or ctx's cancellation. Calling Start
// more than once is a no-op after the first.
func (e *Engine) Start(ctx context.Context) {
	e.startOnce.Do(func() {
		runCtx, cancel := context.WithCancel(ctx)
		e.mu.Lock()
		e.cancel = cancel
		e.mu.Unlock()

		for i := 0; i < e.cfg.Concurrency; i++ {
			workerID := "worker-" + e.ids.NewID()
			e.wg.Add(1)
			go e.poller(runCtx, workerID)
		}
		e.wg.Add(1)
		go e.reaper(runCtx)

		e.logger.Info("job worker pool started",
			"concurrency", e.cfg.Concurrency,
			"pollInterval", e.cfg.PollInterval.String())
	})
}

// Shutdown stops the pollers claiming new work and cancels every
// running handler's context, then waits for the goroutines to return.
// A handler still running when ctx expires is abandoned with its row
// left `running` — the reaper recovers it on the next process's sweep,
// exactly as a crash is recovered (FR-10). It never force-kills or
// corrupts a job row.
func (e *Engine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	cancel := e.cancel
	e.mu.Unlock()
	if cancel == nil {
		return nil // never started
	}
	cancel()

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		e.logger.Info("job worker pool stopped")
		return nil
	case <-ctx.Done():
		e.logger.Warn("job worker pool shutdown grace period expired; running handlers abandoned to the reaper")
		return ctx.Err()
	}
}

func (e *Engine) poller(ctx context.Context, workerID string) {
	defer e.wg.Done()

	kinds := e.registry.kinds()
	if len(kinds) == 0 {
		// Nothing registered: no handler exists to run anything. Idle
		// until shutdown rather than claiming jobs this process cannot
		// execute.
		<-ctx.Done()
		return
	}

	timer := time.NewTimer(e.cfg.PollInterval)
	defer timer.Stop()

	for {
		if ctx.Err() != nil {
			return
		}
		claimed := e.pollOnce(ctx, workerID, kinds)
		if claimed {
			continue // drain greedily while work remains
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(e.cfg.PollInterval)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

// pollOnce claims and runs at most one job. It returns whether a job was
// claimed.
func (e *Engine) pollOnce(ctx context.Context, workerID string, kinds []Kind) bool {
	if e.paused.Load() {
		return false
	}
	job, err := e.store.ClaimNext(ctx, workerID, kinds, e.clock.Now())
	if err != nil {
		if ctx.Err() == nil {
			e.logger.Error("job claim failed", "worker", workerID, "error", err.Error())
		}
		return false
	}
	if job == nil {
		return false
	}
	e.logTransition(job, StateRunning)
	e.execute(ctx, job)
	return true
}

// execute runs one claimed job: a per-job cancellable context, a
// heartbeat goroutine holding the lease, panic recovery, and exactly one
// fenced terminal write — unless the lease was reclaimed mid-flight or
// the process is shutting down, in which case no result is written and
// the reaper takes over (FR-5/FR-6/FR-10).
func (e *Engine) execute(parentCtx context.Context, job *Job) {
	reg, ok := e.registry.lookup(job.Kind)
	if !ok {
		// The kinds filter should make this unreachable; treat it as a
		// permanent failure so the row is not stuck `running`.
		e.finishFailure(parentCtx, job, fmt.Errorf("no handler registered for kind %q", job.Kind), true)
		return
	}

	jobCtx, cancelJob := context.WithCancel(parentCtx)
	defer cancelJob()

	// Register this handler's cancel so an admin cancel aborts it at
	// once rather than at the next heartbeat tick (audit 0016 #299).
	e.live.add(job.ID, cancelJob)
	defer e.live.remove(job.ID)

	var leaseLost atomic.Bool
	hbStopped := make(chan struct{})
	var hbWG sync.WaitGroup
	hbWG.Add(1)
	go func() {
		defer hbWG.Done()
		e.heartbeat(jobCtx, job, hbStopped, &leaseLost, cancelJob)
	}()

	report := func(current, total int) {
		if leaseLost.Load() {
			return
		}
		if err := e.store.UpdateProgress(parentCtx, job.ID, job.LeaseToken, current, total, e.clock.Now()); err != nil {
			e.logger.Warn("job progress write failed", "job", job.ID, "error", err.Error())
		}
	}

	runErr := e.runHandler(jobCtx, reg.handler, job.Payload, report)

	close(hbStopped)
	hbWG.Wait()

	if leaseLost.Load() {
		e.logger.Warn("job result discarded: lease was reclaimed mid-execution", "job", job.ID, "kind", job.Kind)
		return
	}
	if parentCtx.Err() != nil {
		// Shutdown cancelled the handler. Leave the row `running` for
		// the reaper — do not record a synthetic failure (FR-10).
		return
	}
	if jobCtx.Err() != nil {
		// The per-job context was cancelled while the process kept
		// running: an admin cancel already wrote dead_letter (audit 0016
		// #299), or the lease was reclaimed. The terminal row state is
		// owned elsewhere — don't write a synthetic failure over it.
		return
	}

	if runErr == nil {
		e.finishSuccess(parentCtx, job)
		return
	}
	e.finishFailure(parentCtx, job, runErr, IsPermanent(runErr) || job.Attempts >= reg.maxAttempts)
}

func (e *Engine) heartbeat(ctx context.Context, job *Job, stopped <-chan struct{}, leaseLost *atomic.Bool, cancelJob context.CancelFunc) {
	ticker := time.NewTicker(e.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopped:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			alive, err := e.store.Heartbeat(ctx, job.ID, job.LeaseToken, e.clock.Now())
			if err != nil {
				if ctx.Err() == nil {
					e.logger.Warn("job heartbeat write failed", "job", job.ID, "error", err.Error())
				}
				continue
			}
			if !alive {
				leaseLost.Store(true)
				e.logger.Warn("job lease reclaimed; cancelling handler", "job", job.ID, "kind", job.Kind)
				cancelJob()
				return
			}
		}
	}
}

// runHandler invokes h, converting a panic into an error so a bad
// handler never takes down the worker pool. The stack trace is logged
// server-side only; it never reaches last_error.
func (e *Engine) runHandler(ctx context.Context, h HandlerFunc, payload json.RawMessage, report ReportProgressFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("job handler panicked", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	return h(ctx, payload, report)
}

func (e *Engine) finishSuccess(ctx context.Context, job *Job) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalWriteTimeout)
	defer cancel()

	ok, err := e.store.Complete(writeCtx, job.ID, job.LeaseToken, e.clock.Now())
	if err != nil {
		e.logger.Error("job completion write failed", "job", job.ID, "error", err.Error())
		return
	}
	if !ok {
		e.logger.Warn("job completion discarded: lease was reclaimed", "job", job.ID)
		return
	}
	e.logTransition(job, StateCompleted)
}

func (e *Engine) finishFailure(ctx context.Context, job *Job, cause error, deadLetter bool) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalWriteTimeout)
	defer cancel()

	now := e.clock.Now()
	nextRun := now
	if !deadLetter {
		nextRun = now.Add(e.cfg.Backoff.For(job.Attempts, e.jitter()))
	}

	ok, err := e.store.Fail(writeCtx, job.ID, job.LeaseToken, now, nextRun, deadLetter, cause)
	if err != nil {
		e.logger.Error("job failure write failed", "job", job.ID, "error", err.Error())
		return
	}
	if !ok {
		e.logger.Warn("job failure discarded: lease was reclaimed", "job", job.ID)
		return
	}
	if deadLetter {
		e.logTransition(job, StateDeadLetter)
	} else {
		e.logTransition(job, StateRetrying)
	}
}

func (e *Engine) reaper(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.cfg.ReaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalWriteTimeout)
			n, err := e.store.RecoverStale(writeCtx, e.clock.Now(), func(attempts int) time.Duration {
				return e.cfg.Backoff.For(attempts, e.jitter())
			})
			cancel()
			switch {
			case err != nil && ctx.Err() == nil:
				e.logger.Error("job reaper sweep failed", "error", err.Error())
			case n > 0:
				e.logger.Warn("job reaper recovered stale jobs", "count", n)
			}
		}
	}
}

// logTransition records a state change at info (warn for dead_letter,
// which is work this system has given up retrying). It carries the job
// id and kind only — never the payload or last_error content
// (Observability).
func (e *Engine) logTransition(job *Job, to State) {
	if to == StateDeadLetter {
		e.logger.Warn("job moved to dead_letter", "job", job.ID, "kind", job.Kind)
		return
	}
	e.logger.Info("job state changed", "job", job.ID, "kind", job.Kind, "state", to)
}
