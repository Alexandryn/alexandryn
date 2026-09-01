# Spec: Backend job queue

| | |
|---|---|
| **Status** | `IMPLEMENTED` (branch `feat/phase09-async-jobs`, 2026-09-01; `APPROVED` via independent review `0036`, findings fixed, maintainer signed off 2026-08-15; security audit `0009` clear — no open Critical/High). `VERIFIED` pending maintainer close. |
| **Phase** | `09-async-jobs` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-09-01 |
| **Supersedes** | — |
| **Reviewed in** | [`0036`](../reviews/0036-phase09-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (1 Blocking, confirmed independently by both; 6 Major), all findings fixed; approved by maintainer 2026-08-15 |

## Context

ADR `0014` fixed the engine: a PostgreSQL-backed queue, `SELECT ...
FOR UPDATE SKIP LOCKED` for worker dequeue, no dedicated broker
process. `domain-events.md` Non-goals explicitly deferred "the event
bus/transport mechanism... phase 03/09's implementation choice" and
"retry/delivery guarantees, ordering across event types — phase 09" to
this spec — this is that phase, though this spec's own queue is a
distinct mechanism from `domain-events.md`'s event envelope (a job is
work to be *done*; a domain event is a fact that *happened* — this spec
does not consume or produce `domain-events.md` events, and nothing here
changes that spec). `backend-persistence.md` (phase 03) fixed the
connection/migration/transaction patterns this spec's `jobs` table
reuses. `backend-service-lifecycle.md` (phase 03) fixed the graceful-
shutdown grace period this spec's worker shutdown must respect.

## Problem

Nothing exists yet to store a job, claim one safely under concurrent
workers, retry a failed one with backoff, give up on one that keeps
failing, recover a job whose worker crashed mid-execution, or report a
job's current status and progress to a caller.

## Goals

- A `jobs` table and a Go API to enqueue work by a registered `kind`
- A worker pool that claims and executes jobs, safely under
  concurrency (no double-execution)
- Retry with exponential backoff up to a bounded attempt count, then
  dead-letter
- Crash recovery: a job whose worker died mid-execution is detected and
  retried, not lost or stuck `running` forever
- A status/progress query API for callers
- Graceful shutdown integration with `backend-service-lifecycle.md`'s
  existing grace period

## Non-goals

- Any specific job's handler (source sync, import) — phase 10/14 each
  register their own `kind` against this spec's registry; this spec
  ships one synthetic, worked-example handler only, to prove the
  mechanism, not to be a real feature
- A public HTTP endpoint for job status — this phase's status API is
  Go-to-Go (a package other backend code calls directly); no
  `/api/v1/jobs*` surface exists yet, since no caller outside this
  codebase needs one until a future phase's UI does (the same
  "don't ship a feature with nothing downstream to use it" reasoning
  `backend-source-adapter.md` Non-goals already applied to its own
  deferred file-download endpoint)
- `domain-events.md`'s event bus/transport — a distinct mechanism
  (Context above); this spec neither implements nor depends on it
- Any UI — no job type exists yet for a screen to meaningfully describe
- Scheduled/cron-style recurring jobs — every job this phase's API
  enqueues is a one-shot unit of work; a future phase wanting periodic
  re-enqueueing (phase 14's source-sync cadence, for instance) builds
  that as its own small scheduler on top of this spec's `Enqueue`,
  not as a feature of the queue itself

## User stories

- As **a future job handler (phase 10's import, phase 14's source
  sync)**, I want to register a handler function and enqueue work
  against it, so I don't have to build my own retry/concurrency/
  persistence logic.
- As **the maintainer**, I want a job whose worker process crashed
  mid-execution to be automatically retried, not silently stuck
  `running` forever, so a bad shutdown doesn't leave phantom work.
- As **a caller checking on a long-running job**, I want its current
  status and progress, so I can show "3 of 40 processed" rather than a
  bare spinner.

## Functional requirements

- **FR-1** A `jobs` table, migrated via `backend-persistence.md`'s
  existing `goose` mechanism: `id` (uuid, generated Go-side through the
  same `IDGenerator` interface `backend-test-harness.md` FR-6 already
  uses for correlation IDs, for the same testability — deterministic
  IDs under test — rather than a Postgres extension default, which
  would be a new, unaddressed infrastructure dependency), `kind`
  (text), `payload` (`jsonb`), `status` (`queued` | `running` |
  `retrying` | `completed` | `dead_letter`), `attempts` (integer,
  default `0`), `max_attempts` (integer, set from the handler's own
  registered value at enqueue time — FR-2), `available_at`
  (`timestamptz`, when this job becomes claimable), `locked_until`
  (`timestamptz`, nullable — FR-5's lease mechanism), **`lease_token`**
  (uuid, nullable — FR-4/FR-5's fencing token, regenerated on every
  claim or reclaim), `last_error` (text, nullable, truncated/redacted
  unconditionally per Security considerations), `progress` (`jsonb`,
  nullable — FR-8), `created_at`, `updated_at`, `completed_at`
  (nullable). Indexed on `(status, available_at)` for the claim query
  (FR-4), and on `(status, locked_until)` for the reaper sweep (FR-5).
  This package (and the `jobs`/worker-pool code generally) lives in a
  new `internal/jobs` package, sitting alongside
  `internal/persistence`/`internal/transport` in `architecture-backend.md`
  FR-1's existing layout — it depends on `internal/persistence` for its
  own `pgxpool` access (the same shared pool `backend-persistence.md`
  FR-1 already provisions, not a second pool) but MUST NOT be imported
  by `internal/domain`, matching that spec's FR-2 dependency direction;
  a future job handler (phase 10 onward) living in its own package is
  free to import both `internal/jobs` (to register itself) and
  `internal/domain` (to do real work), so this spec's own package
  introduces no new layering exception.
- **FR-2** A handler registers itself at process startup:
  `Register(kind string, maxAttempts int, handler HandlerFunc)` — an
  in-process map, not a database table, since handlers are Go code
  compiled into this binary, not dynamically discoverable. `Enqueue(ctx,
  kind string, payload any) (jobID, error)` MUST return an error if
  `kind` has no registered handler (`InvalidInput` — this project's
  existing fail-loud discipline, `backend-configuration.md` FR-6's
  precedent applied to job kinds instead of config keys) — a typo'd
  `kind` fails at enqueue time, never silently sits unclaimed forever
  because no worker knows how to run it.
- **FR-3** `payload` MUST be JSON-serializable and MUST NOT itself
  contain a secret (a credential, a token) — job payloads reference
  entities by ID (a `Source` ID, a `Work` ID) for the handler to look
  up via the normal repository path, never carrying sensitive values
  directly. This is a discipline stated here for every future job-kind
  implementer to follow, not something this spec can enforce
  structurally (it doesn't know what a future payload contains) —
  named explicitly rather than left as an unstated assumption, the
  same posture `backend-source-adapter.md` FR-4 used for its own
  named accepted risk.
- **FR-4** A worker pool of **4** goroutines (a conservative default,
  flagged as an untuned placeholder in Open questions) each run a poll
  loop, interval **2 seconds** (also a placeholder): claim one
  claimable job via a single short transaction — `SELECT id FROM jobs
  WHERE status IN ('queued', 'retrying') AND available_at <= $1 ORDER
  BY available_at LIMIT 1 FOR UPDATE SKIP LOCKED`, then `UPDATE ... SET
  status = 'running', locked_until = $1 + interval '60 seconds',
  lease_token = $2, attempts = attempts + 1`, committed immediately —
  `$1` is the current time as read from this system's injected `Clock`
  (`backend-test-harness.md` FR-6's controllable-clock interface),
  never SQL's own server-evaluated `now()`, specifically so a test can
  govern claimability and lease expiry deterministically by advancing
  the fake clock, with no real sleep required; `$2` is a freshly
  generated `lease_token` (FR-1), the value this claim — and only this
  claim — owns until it's next reclaimed. The claiming transaction is
  short-lived and never held open for the job's own execution
  duration, avoiding the long-open-transaction cost a naive "hold the
  lock for the whole job" design would carry.

  The correctness property here is **`FOR UPDATE`'s row lock**, held
  until the claiming transaction commits — it is what prevents two
  concurrent workers from both reading the same claimable row and both
  writing `status = 'running'` to it. `SKIP LOCKED` is a throughput
  property layered on top: a worker that would otherwise block waiting
  for a row another worker is mid-claim on instead moves on to a
  different claimable row. Neither clause does anything once a row is
  already `'running'` — the `WHERE status IN ('queued', 'retrying')`
  filter means a `'running'` row is never even considered by this
  query in the first place; the *actual* exclusion of a second claim
  against an already-running job is the `status` value itself, not the
  lock. FR-5's `lease_token` fencing is what protects the *next* layer
  down: once a row is claimed and running, what stops a delayed
  original worker from writing over a reclaim.
- **FR-5** A running job's worker MUST **heartbeat** every **20
  seconds** while the handler executes (a background goroutine per
  claimed job, cancelled when the handler returns): `UPDATE jobs SET
  locked_until = $1 + interval '60 seconds' WHERE id = $2 AND
  lease_token = $3` — conditioned on the exact `lease_token` this
  worker's own claim (FR-4) set. **If this heartbeat affects zero
  rows, the worker's lease has already been reclaimed** (FR-4's
  `lease_token` no longer matches, because the reaper below reassigned
  it) — the worker MUST cancel the `context.Context` passed to its
  handler immediately upon detecting this, and MUST NOT write any
  further result for this job (no completion, no retry, no
  dead-letter — FR-6's writes are themselves fencing-conditioned on
  the same token, so a stale write would simply affect zero rows even
  if attempted, but the worker stops trying at the heartbeat failure
  rather than relying on that alone).

  A **reaper**, running on its own periodic sweep (interval **30
  seconds**, using the injected `Clock`, not SQL `now()` — same
  reasoning as FR-4), finds every job with `status = 'running' AND
  locked_until < $1` and, in one short transaction per job, reclaims
  it: generates a **new** `lease_token`, applies FR-6's normal
  retry/dead-letter logic (treating the expired lease as a failed
  attempt), and writes `last_error = "worker lease expired without
  heartbeat"`. Reassigning `lease_token` here — not just changing
  `status` — is what makes the fencing in FR-4/FR-6 actually work: the
  original worker's `lease_token` is now stale everywhere, not only
  where `status` happens to be checked.

  **Bounded race, named explicitly**: a heartbeat that is merely
  *delayed* (not truly crashed — GC pause, a CPU-bound handler
  starving its own heartbeat goroutine, database contention) past the
  60-second lease can still be reaped as if crashed, which is why the
  heartbeat interval (20s) is set to a third of the lease duration
  (60s) — two consecutive missed heartbeats, not one, before the lease
  actually expires, giving real margin against a single delayed write.
  This margin is a tuned safety factor, not a proof of impossibility;
  a sufficiently pathological delay (all three heartbeat attempts
  starved past 60s while the handler is still genuinely alive and
  about to succeed) remains possible and would trigger a reclaim.
  FR-4/FR-6's fencing is what keeps that scenario from producing a
  double-*commit* (only one of the two lease-holders' writes can ever
  succeed, whichever holds the current `lease_token` at write time) —
  it does not prevent the reclaimed job's *handler code* from
  continuing to run to completion in the background with a stale
  token, doing real work (a partial file write, a partial import
  step) whose side effects aren't rolled back merely because its
  database write is fenced off. Handlers that perform non-idempotent,
  irreversible side effects should check `ctx.Err()` between steps and
  abort early — a general handler-authoring discipline this spec
  states but cannot enforce structurally, the same posture as FR-3's
  no-secrets-in-payload rule.
- **FR-6** On handler completion, the worker's write is **fencing-
  conditioned**: `UPDATE jobs SET status = ... WHERE id = $1 AND
  lease_token = $2` — the exact token this worker's claim (FR-4) holds.
  If this write affects zero rows, the lease was already reclaimed
  (FR-5) and the worker's result is discarded — logged at `warn`
  (Observability) as a detected-but-harmless late write, not treated
  as a new failure. Otherwise: success sets `status = 'completed'`,
  `completed_at = $now`. Failure (the handler returns an error, panics
  — recovered by the worker, treated identically to a returned error)
  compares `attempts` against `max_attempts` (FR-2, already
  incremented once at claim time — this comparison never increments
  `attempts` a second time): if under the limit, `status = 'retrying'`,
  `available_at = $now + backoff(attempts)` (FR-7); at or over the
  limit, `status = 'dead_letter'`, no further automatic retry. A
  handler MAY instead return an error wrapped in `job.Permanent(err)`
  to force immediate `dead_letter` regardless of `attempts` remaining
  — the mechanism by which a handler signals "this specific failure
  will never succeed on retry" (e.g. a source's `4xx`, the roadmap's
  own named example), distinct from an ordinary transient error, which
  always goes through the normal attempts-based comparison. `last_error`
  is set on every failure (truncated/redacted unconditionally,
  Security considerations), overwritten each attempt (only the most
  recent failure's message is kept, not a full history — this spec's
  own reasoned scope cut, flagged in Open questions). FR-5's reaper
  path reuses this same comparison and the `attempts` value already
  set by the original claim — the reaper never re-increments
  `attempts` independently; a lease expiry counts as exactly one
  failed attempt, the one the original claim already accounted for.
- **FR-7** Backoff: `min(baseDelay * 2^(attempts - 1), maxDelay)` with
  **±20% jitter** (a random delta in that range, avoiding synchronized
  retry storms if many jobs fail at once) — `attempts - 1`, not
  `attempts`, because FR-4's claim already increments `attempts` to
  `1` for the very first attempt, and the first *retry* (the second
  attempt) should wait close to `baseDelay` itself, not `baseDelay *
  2`; `baseDelay = 5s`, `maxDelay = 5m`, both this spec's own
  placeholder numbers, flagged in Open questions as untuned against
  real job behaviour, not load-tested claims.
- **FR-8** A handler MAY report progress via a callback passed into
  `HandlerFunc` (`ReportProgress(current, total int)`), which updates
  the job's `progress` column (`{ current, total }`) in a small,
  independent transaction — progress reporting MUST NOT be blocked by,
  or block, the job's own main transactional work (if any); a handler
  that never calls it simply has `progress: null`, a legal, common
  case (many jobs have no meaningful sub-progress to report).
- **FR-9** `GetJob(ctx, jobID) (Job, error)` and `ListJobs(ctx, filter)
  ([]Job, error)` (filterable by `kind`/`status`) are this spec's
  status/progress query API — internal Go functions, called directly
  by other backend packages, not an HTTP handler (Non-goals). Both
  return the job's full row shape (status, attempts, `last_error`,
  `progress`, timestamps) — nothing here further restricts what a
  Go caller sees, since every caller is this codebase's own code, not
  an external boundary.
- **FR-10** On shutdown, the worker pool MUST stop claiming new jobs
  immediately and cancel the context passed to every currently-running
  handler, giving each a chance to return promptly — but MUST NOT
  force-kill or corrupt a job's row state: a handler that doesn't
  return before the process exits is abandoned, leaving that job's row
  `status = 'running'` with a `locked_until` that will eventually
  expire — FR-5's reaper, on this or a future process's next run, is
  what recovers it, exactly the same path a genuine crash takes.
  Shutdown does not need its own special-cased recovery logic distinct
  from crash recovery, since the two produce the identical on-disk
  state.

  This requires an amendment to `backend-service-lifecycle.md`, not
  merely reuse of its existing sequence: that spec's FR-4 (HTTP
  `Shutdown(ctx)`) and FR-6 (close the `pgxpool`) have no step for a
  job worker pool today. The worker pool's own shutdown MUST be
  signalled *before* FR-6 there closes the shared `pgxpool` — a
  heartbeat or completion write (FR-5/FR-6 above) attempted against an
  already-closed pool would be a raw connection error, not the clean
  cancellation this FR describes — and MUST use
  `backend-service-lifecycle.md` FR-5's same configured grace period as
  its own upper bound for "give running handlers a chance to return,"
  not a second, independently-configured timeout. This ordering
  requirement is the concrete content of the amendment
  `backend-service-lifecycle.md` needs; it is not optional wiring left
  to implementation.

## Non-functional requirements

- **Performance** — the claim query (FR-4) is a single indexed lookup
  under a short transaction; poll interval and worker count (FR-4) are
  this spec's own tuning knobs, not fixed by any external requirement.
  No specific job-execution-time budget is set here — that depends
  entirely on what a future job's handler does, out of this spec's
  control.
- **Security** — see dedicated section below.
- **Accessibility** — not applicable; no UI this phase.
- **Reliability** — FR-5/FR-10 together are this spec's core
  reliability property: no job can be silently lost to a crash or a
  shutdown, only delayed until the reaper recovers it.
- **Observability** — a job's status transition (`queued → running`,
  `running → retrying`/`dead_letter`/`completed`) MUST log at `info`
  with the job's `id` and `kind`, never the `payload` or `last_error`
  content directly (Security considerations) — only that a transition
  happened and to what state. A `dead_letter` transition specifically
  MUST log at `warn`, since it represents work this system has given
  up on automatically retrying, worth a maintainer's attention.

**Dependency justification (constitution §9)**: no new dependency.
`SELECT ... FOR UPDATE SKIP LOCKED` is a PostgreSQL built-in feature,
reached through `backend-persistence.md`'s existing `pgx`/`pgxpool`
stack — no queue library, no broker client. The worker pool itself is
a small amount of Go code (goroutines, a `time.Ticker` per poller) —
not enough surface to justify a third-party worker-pool library at
this scale.

## Domain model

No `domain-*` types. `Job` is a new, persistence-layer type owned
entirely by this spec — not a `domain-bibliographic.md`/
`domain-library.md`/`domain-source.md`/`domain-reading.md` concept,
and not itself a `domain-events.md` event (Context above). A future job
handler (phase 10's import, for instance) is free to construct and
persist real domain types (a `Work`, a `LibraryEntry`) as part of its
own execution — this spec has no opinion on that; it only owns the
`Job` row describing that the work happened, is happening, or is
scheduled.

## API and contracts

No new `/api/v1` HTTP surface (Non-goals). The Go API:

```go
func Register(kind string, maxAttempts int, handler HandlerFunc)
func Enqueue(ctx context.Context, kind string, payload any) (jobID string, err error)
func GetJob(ctx context.Context, jobID string) (Job, error)
func ListJobs(ctx context.Context, filter JobFilter) ([]Job, error)

type HandlerFunc func(ctx context.Context, payload json.RawMessage, report ReportProgressFunc) error
type ReportProgressFunc func(current, total int)
```

## State transitions

`queued → running → completed` (success) | `running → retrying →
running → ...` (failure, under `max_attempts`) | `running →
dead_letter` (failure, at/over `max_attempts`) | `running →
retrying|dead_letter` (via the reaper, FR-5, when a lease expires with
no heartbeat — indistinguishable from a handler-reported failure from
the state machine's point of view, only the trigger differs). Illegal:
any transition out of `completed` or `dead_letter` — both are terminal;
a job that needs to run again is a *new* enqueue, never a resurrection
of a finished row.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Handler returns an error | Direct return value | `GetJob` shows `status: retrying` or `dead_letter`, `last_error` set | Retried per FR-6/FR-7, or dead-lettered |
| Handler panics | Worker's own `recover()` | Same as a returned error | Panic converted to an error, treated identically — never crashes the worker pool itself |
| Worker process crashes mid-job | FR-5's reaper, lease expiry | `GetJob` shows `status: running` until the reaper sweep runs, then `retrying`/`dead_letter` | Recovered on the next sweep (up to the sweep interval's own delay, a bounded, accepted detection latency) |
| `Enqueue` called with an unregistered `kind` | FR-2's registry check | `Enqueue` returns `InvalidInput` immediately | Nothing persisted |
| Two workers attempt to claim the same row concurrently | `FOR UPDATE`'s row lock (FR-4) | One succeeds, the other's claim query simply returns no row (not an error) | No double-claim of an already-`queued`/`retrying` row, by construction |
| A worker's heartbeat is delayed past the lease and the reaper reclaims the job while the original worker is still alive (FR-5) | `lease_token` mismatch on the worker's next heartbeat or completion write | `GetJob` reflects whichever write held the current `lease_token` first — the reclaiming worker's outcome, not the stale one | The stale worker's write affects zero rows, is discarded and logged at `warn`; the worker cancels its handler's context on detecting the mismatch. Named explicitly as a bounded, not eliminated, risk (FR-5) — this fences the *database write*, not any non-idempotent side effect the stale handler performed before detecting it |
| Shutdown grace period elapses with a job still running | Process exit | `GetJob` shows `status: running` with an aging `locked_until` | Recovered by the reaper on next startup — no special-cased shutdown logic (FR-10) |

## Security considerations

- **Job payloads must never carry secrets (FR-3)** — restated as a
  discipline for every future job-kind implementer, not structurally
  enforced by this spec (it can't know a future payload's shape),
  named honestly as weaker than a type-level guarantee — the same
  admission `backend-source-adapter.md` FR-4 made for its own accepted
  risk, not overstated as equivalent to a structural one.
- **`last_error` MUST be truncated to a fixed maximum length (512
  bytes) and passed through the same `slog.LogValuer`/`json.Marshaler`
  redaction filter `backend-errors-and-logging.md` FR-8's typed secret
  values already use, unconditionally, on every write** — not "if a
  handler's error could plausibly wrap something sensitive," a
  judgment call this spec should not leave to each future job-kind
  implementer to independently get right. This is the one piece of
  this spec that genuinely achieves FR-8's structural bar (unlike
  FR-3's payload discipline above, which honestly can't): every write
  path to `last_error` goes through one function, not scattered call
  sites, so the redaction filter is applied once, centrally, the same
  "closes the leak once, at the type" property FR-8 itself is built
  around.
- **No new trust boundary** — the `jobs` table lives in the same
  PostgreSQL instance every other phase already trusts; nothing this
  spec introduces is reachable from a LAN client or an external source
  directly.
- **No arbitrary code execution surface** — `kind` is matched against
  an in-process registry (FR-2), never used to dynamically load or
  execute code named by a caller; an unregistered `kind` is rejected,
  not a mechanism for injecting behaviour.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Backoff calculation (FR-7) across a range of attempt counts, including the jitter bound and the `maxDelay` cap; state-transition legality (illegal transitions out of `completed`/`dead_letter` rejected) |
| Integration | Enqueue → claim → complete, against a real PostgreSQL instance (`backend-test-harness.md`'s harness, its controllable `Clock` driving `available_at`/`locked_until` assertions via the `$now` parameter FR-4/FR-5 pass explicitly — no real sleeps, and no reliance on SQL's own `now()`); a concurrent-worker test asserting `FOR UPDATE` prevents double-claiming the same row under real concurrent connections; a reaper test asserting a job whose `locked_until` has expired with no heartbeat at all is recovered on the next sweep; a **fencing test** asserting that when a reclaim happens *while the original worker is still alive* (simulated: advance the fake clock past the lease without stopping the original worker's own heartbeat/completion goroutine, so both a stale and a fresh `lease_token` exist briefly), only the write holding the current `lease_token` succeeds — this is the test finding #1 (review `0036`) required, distinct from the simpler "heartbeat never arrives at all" case; a dead-letter test asserting a job exceeding `max_attempts` stops retrying, and a separate test asserting `job.Permanent(err)` dead-letters immediately regardless of remaining attempts |
| Contract | N/A — no HTTP surface this phase |
| E2E | A synthetic worked-example job type (Non-goals) enqueued, failing twice (transient, controlled), succeeding on the third attempt, with `GetJob` reflecting each transition along the way |
| Accessibility | N/A — no UI this phase |

Tests that must fail before implementation begins: a test asserting two
concurrent workers polling the same claimable job result in exactly one
claim, never zero or two; a test asserting a job whose worker stops
heartbeating entirely is retried by the reaper, not left `running`
forever; a test asserting a *fenced* stale write (the original worker's
heartbeat or completion call after its `lease_token` was reassigned)
affects zero rows and is discarded, never overwriting the reclaiming
worker's own result; a test asserting a job at `max_attempts`
transitions to `dead_letter` and is never claimed again.

## Acceptance criteria

- [ ] A registered job kind can be enqueued and executed by the worker
      pool
- [ ] Concurrent workers never double-claim an unclaimed job, proven
      under real concurrency
- [ ] A stale worker's write (after its lease is reclaimed) never
      overwrites the reclaiming worker's result, proven by the fencing
      test (Test strategy) — the database-write half of "no double
      execution"; a handler's own non-idempotent side effects remain a
      handler-authoring discipline (FR-5), not something this spec
      structurally guarantees
- [ ] A failing job retries with backoff up to `max_attempts`, then
      moves to `dead_letter`; a `job.Permanent(err)` failure
      dead-letters immediately regardless of remaining attempts
- [ ] A job whose worker crashes (simulated: stop heartbeating) is
      recovered by the reaper, not lost
- [ ] `GetJob`/`ListJobs` reflect accurate status and progress at every
      stage
- [ ] Shutdown cancels running handlers' contexts and never corrupts a
      job's row state
- [ ] No job payload, error, or progress value logs a value it
      shouldn't per Security considerations, proven by a test

## Open questions

- **Worker count (4), poll interval (2s), lease duration (60s),
  heartbeat interval (20s), reaper sweep (30s), backoff base/max (5s/5m)**
  — every number in this spec is a reasoned placeholder, not load-tested
  against a real job type, since none exists yet. Revisit once phase 10
  or 14's real handlers give this spec actual data to tune against.
- **`last_error` keeps only the most recent failure (FR-6)** — a
  reasoned scope cut (a full attempt history would need its own table),
  not a considered limit; revisit if debugging a flaky job type in
  practice needs more than the latest failure.
- **No recurring/scheduled jobs** — explicitly out of scope (Non-goals);
  whichever future phase needs periodic re-enqueueing builds its own
  small scheduler on top of `Enqueue`, not as this spec's own feature.
- **`domain-events.md`'s event-bus/transport question remains
  genuinely unresolved** — that spec deferred it explicitly to "phase
  03/09's implementation choice," and this spec (phase 09) has now
  passed on it too, on the reasoning that a job queue and an event bus
  are different mechanisms (Context). That reasoning may be right, but
  it leaves a two-phase-old deferred commitment with no assigned owner
  — flagged here explicitly rather than left to go quietly unnoticed a
  third time; a future phase (or a `domain-events.md` amendment) needs
  to either claim it or formally close it as no-longer-needed.
- **Job worker pool competing with HTTP request handling for
  `backend-persistence.md` FR-1's shared, bounded `pgxpool`** — not
  addressed here; once phase 10's import handlers add real DB-heavy
  work inside jobs, whether the job subsystem needs its own reserved
  connection budget (rather than drawing from the same pool general
  request handling uses) is a real capacity question with no data yet
  to answer it.

## References

- ADR `0014` — PostgreSQL-backed queue engine choice, this spec's
  concrete implementation of it
- `domain-events.md` (phase 02) — explicitly deferred the event
  bus/transport mechanism here; distinguished from, not implemented by,
  this spec (Context)
- `backend-persistence.md` (phase 03) — connection/migration/transaction
  patterns reused
- `backend-service-lifecycle.md` (phase 03) — amended (FR-6) to add
  the job-worker-pool shutdown ordering this spec's FR-10 requires
- `backend-configuration.md` (phase 03) FR-6 — fail-loud discipline
  precedent, applied to unregistered job kinds (FR-2)
- `backend-errors-and-logging.md` (phase 03) FR-8 — redaction posture
  restated for `last_error` (Security considerations)
- `backend-test-harness.md` (phase 03) — controllable clock, reused for
  backoff/lease-expiry tests without real sleeps
- `backend-source-adapter.md` (phase 08) FR-4, Non-goals — precedent
  for naming an accepted risk explicitly (FR-3) rather than assuming
  it away; precedent for deferring an endpoint with nothing downstream
  to use it (Non-goals)
- `architecture-backend.md` (phase 01) FR-1/FR-2 — package layout and
  dependency-direction rules `internal/jobs` (FR-1) sits within
- Constitution §9 (dependency justification), §8 (redaction, extended
  to a new storage location)
