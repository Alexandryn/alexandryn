# Security audit: Phase 09 — Async jobs (background job queue)

| | |
|---|---|
| **Scope** | `internal/jobs/` (`job.go`, `backoff.go`, `errortext.go`, `errors.go`, `registry.go`, `config.go`, `queue.go`, `store.go`, `engine.go`, `system.go`), migration `internal/persistence/postgres/migrations/00006_phase09_jobs.sql`, and the `cmd/server` lifecycle wiring (`run.go`, `main.go`). |
| **Auditor** | Claude (Sonnet 5), `agent-skills:security-and-hardening` + `agent-skills:security-auditor` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over each trust boundary |
| **Date** | 2026-09-01 |
| **Commit** | Branch `feat/phase09-async-jobs` |
| **Verdict** | **Clear** — no open Critical or High findings. Two Low, two Informational, all accepted with reasons or already mitigated. Full `go test -race` unit + integration suites, `golangci-lint`, `go vet`, and the repo check scripts pass. |

## Scope and method

Phase 09 builds generic background-job infrastructure: a `jobs` table, an
enqueue/query Go API, a worker pool that claims rows with
`SELECT ... FOR UPDATE SKIP LOCKED`, retry with exponential backoff,
dead-lettering, and a lease-plus-heartbeat reaper for crash recovery. It
registers **no job kinds** in production — the only handlers are
test-only (spec Non-goals: "one synthetic, worked-example handler
only").

Method: full code reading of every file in scope; STRIDE over each trust
boundary; the four-attacker adversarial pass; review of the SQL for
parameterisation and for `FOR UPDATE`/fencing correctness; review of
every `log/slog` call for leakage; `-race` execution of the concurrent-
claim and fencing tests; dependency check (`go.mod` unchanged).

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| `Queue.Enqueue(kind, payload)` | The calling Go code (a future job handler package) | The caller is this codebase's own code, not an external boundary — there is no HTTP surface this phase (spec Non-goals). `kind` and `payload` are still validated (registered-kind check, JSON-marshal check). |
| `jobs` table rows → worker | The database row's `kind`, `payload`, `attempts` | A row is only ever written by this system's own `Store`; still, `kind` is matched against the in-process registry (never used to load code), `payload` is passed to the handler as opaque `json.RawMessage`, and `attempts`/`max_attempts` bounds are enforced by CHECK constraints. |
| `HandlerFunc` return value / panic | A future handler's error text | Attacker-influenced in the general case (a handler can wrap a source's HTTP response). Truncated to 512 bytes through one function on every write; the `Job.LastError` type redacts under `slog`/JSON. |
| `context.Context` passed to a handler | n/a (outbound) | Cancelled on shutdown and on lease loss; a handler that ignores it is abandoned, never force-killed. |
| The shared `pgxpool.Pool` | n/a | The job subsystem draws from the same bounded pool as HTTP request handling (spec Open questions flags the capacity question; no new pool). |

The `jobs` table lives in the same PostgreSQL instance every other phase
already trusts. Phase 09 introduces **no new network listener, no new
credential, no new outbound request, and no new dependency** (`go.mod`
and `go.sum` are unchanged).

## Adversarial questions asked (Constitution §10)

**What does a malicious user of this instance achieve here?**
Nothing directly — there is no HTTP route, IPC message, or LAN-reachable
surface for the job queue this phase. A LAN client cannot enqueue,
claim, inspect, or cancel a job. The only reachable effect a future
phase's HTTP handler could expose (job status) is explicitly deferred
(spec Non-goals). When a future phase does add an enqueue trigger, that
phase owns validating its own payload — FR-3's "payloads reference
entities by ID, never carry a secret" is restated in `Queue.Enqueue`'s
doc comment as a caller discipline.

**What does a malicious source achieve — metadata, redirects,
filenames, file contents?**
Out of reach this phase: no job handler fetches anything. A *future*
source-sync handler's fetched bytes are that phase's constitution-§4
concern; this phase's queue mechanism never touches a source. The one
attacker-influenced value that does flow through phase 09 is a handler's
**error text**, and it is bounded (512 bytes, rune-safe truncation
through `redactError`) and typed to redact under logging.

**What does a device on the local network achieve without credentials?**
Nothing. No bind, no route.

**What happens when input is malformed, empty, enormous, duplicated,
slow, or never arrives?**
- *Malformed kind*: `Enqueue` returns `InvalidInput` before any row is
  written (FR-2). A row whose `kind` is somehow unregistered at claim
  time is fenced-failed as permanent, never left `running`.
- *Non-serialisable payload*: `Enqueue` returns `InvalidInput`
  (`json.Marshal` failure).
- *Enormous error text*: truncated to 512 bytes on every write.
- *Enormous payload*: not bounded by this phase (see Finding A-09-01).
- *Duplicated claim*: `FOR UPDATE`'s row lock + the `status` filter make
  a double-claim impossible; proven under real concurrency
  (`TestStore_ConcurrentClaim_NeverDoubleClaims`, 20 goroutines / 8
  jobs, zero duplicates).
- *Slow / never-returning handler*: the heartbeat holds the lease while
  it runs; on shutdown the handler's context is cancelled and, if it
  ignores that, the job is abandoned to the reaper — never force-killed,
  never a corrupted row (`TestEngine_ShutdownAbandonsUncooperativeHandler`).
- *Crashed worker*: the reaper reclaims the job once its lease expires
  with no heartbeat, applying the normal retry/dead-letter path
  (`TestEngine_ReaperRecoversAbandonedJob`).

**What is logged that shouldn't be?**
Nothing. Status-transition log lines carry `job` id + `kind` + `state`
only — never `payload`, never `last_error`. Proven by
`TestEngine_NoPayloadOrErrorValueIsLogged` (a handler fails with a
secret-shaped error against a secret-shaped payload; the full log record
set is scanned; neither marker appears; the `dead_letter` transition is
confirmed logged so the scan is not vacuous). A recovered panic logs the
stack trace **server-side only**, and the stack never reaches
`last_error` (only the fixed string `"handler panicked: <value>"`, then
truncated).

**What happens if two of these run at once?**
- Two workers, one row → exactly one claim (row lock).
- A worker and the reaper on the same row → `FOR UPDATE SKIP LOCKED` in
  `reclaimOne`; the loser skips the row.
- A reclaimed job's original worker and the reclaiming path → every
  worker write (`Heartbeat`/`Complete`/`Fail`/`UpdateProgress`) is
  `WHERE id = $1 AND lease_token = $2`; the reaper regenerates the
  token, so the stale worker's writes hit zero rows and are discarded
  (`TestStore_RecoverStale_*`, `TestEngine_FencingAfterReclaim`).
- The engine's `Start` called twice → `sync.Once`, no double pool.

## STRIDE summary

| Threat | Assessment |
|---|---|
| **Spoofing** | No identity crossing a boundary. `locked_by` (worker id) is diagnostic only; `lease_token` is the authorisation token for a write and is a `crypto/rand` UUID from `internal/idgen`, unguessable, regenerated on every claim/reclaim. |
| **Tampering** | All SQL is parameterised — static backtick literals, values as separate `pgx` args, verified by reading every query and by `scripts/check-parameterized-queries.sh` (the script's scope is `internal/persistence/postgres`; `internal/jobs` SQL was reviewed by hand to the same standard). CHECK constraints enforce the `status` vocabulary and attempt bounds at the schema. No row transition out of a terminal state is possible (`status = 'running'` guard on every worker write; reaper only touches `status = 'running'`). |
| **Repudiation** | Every state transition logs at `info` (`warn` for `dead_letter`) with the job id and kind — an audit trail of what the queue did, without the sensitive content. |
| **Information disclosure** | `last_error` truncated + typed-redacted; payload never logged; no DSN or credential path in any job-subsystem log line; `translateError` returns a fixed generic client-facing message and keeps the raw error only in `Err` for server-side logging, matching `postgres.TranslateError`. |
| **Denial of service** | Worker pool size is bounded (default 4). Poll interval bounds claim-query load. `last_error` is size-capped. Backoff has a `Max` cap and ±20% jitter to avoid synchronised retry storms. The reaper sweep is a single indexed query plus one short transaction per stale row. **Not bounded**: job payload size, and total queue depth — see findings. |
| **Elevation of privilege** | `kind` is a registry key, never a code path selector — an unregistered kind is rejected, not executed. No handler runs with more context than the job's own cancellable `context.Context`. The preload/IPC surface is untouched. |

## Findings

| ID | Severity | Title | Status |
|---|---|---|---|
| A-09-01 | Low | Job payload size is not bounded at enqueue | Accepted (documented) |
| A-09-02 | Low | Queue depth / enqueue rate is not bounded | Accepted (documented) |
| A-09-03 | Informational | Reaper processes stale rows unboundedly in one sweep | Accepted |
| A-09-04 | Informational | `context.WithoutCancel` on terminal writes can outlive process shutdown intent | Accepted (bounded) |

### A-09-01 — Job payload size is not bounded at enqueue

**Severity:** Low

**Component:** `internal/jobs/queue.go` (`Enqueue`), `internal/jobs/store.go` (`Enqueue`)

**Description** — `Queue.Enqueue` marshals the caller's `payload any` to
JSON and writes it to a `JSONB` column with no size check. A caller
passing a very large structure would store a large row.

**Impact** — Minimal. The caller is this codebase's own Go code, not an
external boundary (there is no HTTP enqueue path this phase). A future
phase that builds an HTTP trigger for a job must apply Constitution §4's
line-1 validation (size cap) at *that* boundary — the same posture
`backend-source-adapter.md` used for its own deferred endpoint. The
worst case today is a developer bug bloating a row, not an attacker
action.

**Preconditions** — Ability to call `Queue.Enqueue` directly, i.e. write
Go code in this module.

**Recommendation** — When the first real HTTP-triggered job kind lands
(phase 10/14), cap the request body at that handler and cap the derived
payload before `Enqueue`. Optionally add a defensive `len(raw)` ceiling
in `Enqueue` itself at that point. Not worth adding now against a
caller that is trusted Go code — noted so the next phase does not
inherit it silently.

**Resolution** — Accepted for phase 09; carried to phase 10/14 scope.

### A-09-02 — Queue depth and enqueue rate are not bounded

**Severity:** Low

**Component:** `internal/jobs/queue.go`

**Description** — Nothing limits how many jobs can be `queued` at once.
A runaway loop calling `Enqueue` (a bug in a future handler that
enqueues follow-up work) could grow the table without limit.

**Impact** — Low, and self-hosted-single-user context bounds it
further: the `jobs` table shares the household-scale PostgreSQL instance
this project already manages; unbounded growth is a disk-usage problem,
not a security boundary crossing. There is no multi-tenant blast radius.

**Preconditions** — A buggy or malicious job handler (future phase)
enqueueing in a loop.

**Recommendation** — Phase 15 (observability) is the natural place to
surface queue depth as a metric with an alert. A hard cap on `queued`
count could be added to `Enqueue` if a real runaway is ever observed;
premature today.

**Resolution** — Accepted; flagged for phase 15.

### A-09-03 — Reaper processes all stale rows in a single sweep

**Severity:** Informational

**Component:** `internal/jobs/store.go` (`RecoverStale`)

**Description** — `RecoverStale` selects every `running` row past its
lease and reclaims each in its own transaction, with no `LIMIT`. If a
large number of jobs were abandoned at once (a crash under heavy load),
one sweep would do a lot of work.

**Impact** — None security-relevant. Each reclaim is a short indexed
transaction; the sweep runs on its own goroutine and does not block
pollers. At household scale the number of concurrently-running jobs is
tiny (worker pool default 4).

**Recommendation** — Add a `LIMIT` to the candidate query if job volume
ever grows past this project's stated scale (ADR 0014 already says that
scenario would trigger a broader revisit).

**Resolution** — Accepted.

### A-09-04 — `context.WithoutCancel` on terminal writes

**Severity:** Informational

**Component:** `internal/jobs/engine.go` (`finishSuccess`, `finishFailure`, `reaper`)

**Description** — The completion/failure/reaper database writes run on a
context detached from the poll loop's (`context.WithoutCancel`) with a
5-second timeout, so a handler that finishes right as shutdown begins
still records its outcome. This means a terminal write can run for up to
5 seconds after `Shutdown` was requested.

**Impact** — None. The write is a single short `UPDATE`; the 5-second
ceiling is far below any realistic concern, and `gracefulShutdown`
sequences the pool close *after* the engine's `Shutdown` returns, so the
pool is still open for these writes. The alternative (cancelling the
write on shutdown) would leave more jobs stuck `running` for the reaper
— strictly worse.

**Recommendation** — None. Documented so the 5-second constant is not
mistaken for arbitrary.

**Resolution** — Accepted (intentional, bounded).

## Constitution checklist

- **§2 tests before code** — RED integration tests for the schema and
  store preceded the migration and `store.go`; the concurrency and
  fencing proofs were written against the spec's Test strategy. Engine
  and lifecycle wiring were the two places test and code were written
  close together (goroutine-heavy glue) — stated plainly, not hidden.
- **§3 domain boundaries** — `internal/domain` does not import
  `internal/jobs` (checked). `internal/jobs` imports `internal/domain`
  only for `IDGenerator` and `Error`. `Job` is a persistence-layer type,
  never a `domain-*` concept.
- **§4 hostile input** — handler error text bounded and redacted;
  `kind` validated against the registry; CHECK constraints on `status`
  and attempt bounds. Payload size deferred with a named reason
  (A-09-01).
- **§8 observability without leakage** — structured logs from the first
  line; no payload, `last_error`, credential, DSN, or home-path in any
  job log line; proven by test.
- **§9 dependencies** — none added. `go.mod`/`go.sum` unchanged.
  `SELECT ... FOR UPDATE SKIP LOCKED` and `math/rand/v2` are stdlib /
  PostgreSQL built-ins.
- **§10 adversarial review** — this document.
- **§12 uncertainty stated** — the placeholder tuning numbers are
  flagged as untuned in `config.go` and the plan; the payload-size and
  queue-depth gaps are findings, not silent omissions.

## What was not examined

- **Real job handlers** — none exist yet. The synthetic handlers are
  test-only. Phase 10's import handler and phase 14's source-sync
  handler each own their own §4 audit of what they do with fetched
  data.
- **Load behaviour** — the worker-pool tuning (concurrency 4, poll 2s,
  lease 60s, etc.) is not load-tested; it cannot be until a real job
  kind exists (spec Open questions).
- **`pgxpool` contention between jobs and HTTP handling** — spec Open
  question; no data until phase 10 adds DB-heavy job work.
- **Multi-process job workers** — out of scope; this project is one Go
  server process (ADR 0014).
