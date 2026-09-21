# Phase 09 — Async jobs

| | |
|---|---|
| **Status** | Implementation complete on `feat/phase09-async-jobs` (2026-09-01); security audit `0009` clear; awaiting maintainer review to close |
| **Depends on** | Phase 08 |
| **Blocks** | 10, 15 |
| **Opened** | 2026-09-01 |
| **Closed** | — |

## Objective

Background job infrastructure — enqueue, execute, retry with backoff,
dead-letter, and status reporting — introduced at the first point real
asynchronous work exists in this project: phase 08's sources need
periodic re-checking (deferred from phase 08 itself, which only ever
health-checks synchronously) and phase 10's import pipeline needs a
place to run discovery/extraction/matching without blocking an HTTP
request for the duration. This phase builds the generic infrastructure;
it deliberately does not build any specific job's handler.

## Why here

It needs phase 08's source model to have a real reason to exist (a
source re-check job is the concrete motivating case, even though this
phase doesn't implement that job itself) and it needs to land before
phase 10 (import), which is the first phase that actually needs to
enqueue and run a long job. It blocks phase 15 (observability) because
job status/progress is exactly the kind of operational state that
phase's dashboards would want to surface.

## Scope

**In**

- A job queue: storage, enqueue, worker pool that dequeues and executes,
  retry with exponential backoff, dead-letter handling after a bounded
  number of attempts
- Job status and progress reporting, queryable by callers (an internal
  Go API for now — no dedicated UI screen this phase, since no job
  *type* exists yet for a screen to describe meaningfully)
- The broker-or-not ADR: PostgreSQL-backed versus a dedicated message
  broker (RabbitMQ, Redis Streams, etc.), decided and recorded, not
  assumed

**Out**

- The import job handler itself (discovery → extraction → matching →
  confirmation) — phase 10
- Source sync job handlers (the periodic re-check phase 08 deferred) —
  phase 14, per that phase's own scope
- Any UI surfacing job status to an end user — deferred until a real
  job type exists to describe; this phase's status API is consumed
  internally (by whichever phase adds the first real job) and by tests,
  not by a screen

## Specifications

| Spec | Covers |
|---|---|
| `backend-job-queue.md` | Job storage, enqueue/dequeue, worker pool, retry/backoff, dead-letter, status API |

## Architecture decisions expected

- **The broker-or-not ADR** — this phase's central decision, recorded
  as its own ADR (not folded into the spec as an aside): PostgreSQL-
  backed (a `jobs` table, `SELECT ... FOR UPDATE SKIP LOCKED` for
  worker dequeue, no new infrastructure or process) versus a dedicated
  broker. Leaning PostgreSQL-backed, consistent with this project's
  established pattern of preferring the already-running database over
  new infrastructure at this project's scale (phase 06 chose
  PostgreSQL full-text search over a dedicated search service for the
  same underlying reason) — but the ADR is where this gets argued
  concretely, not assumed here.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| A PostgreSQL-backed queue's polling-based dequeue adds latency a message broker's push model wouldn't have | Medium | Low | Named explicitly in the ADR's Consequences, weighed against this project's household scale — a few seconds of enqueue-to-start latency is not a meaningful cost for a source sync or an import job, unlike a request/response path |
| Worker pool sizing chosen without real load data | Medium | Low | A conservative, small default (this spec's own to fix), documented as a placeholder in Open questions, not presented as load-tested |
| Retry/backoff design doesn't compose cleanly with a future job type's own domain-specific retry semantics (e.g. "don't retry a 4xx from a source, only a timeout") | Medium | Medium | This spec's retry policy is generic (attempt count, backoff curve); a job handler's own logic decides whether a given failure is retryable at all before this spec's mechanism ever re-queues it — the boundary is stated explicitly, not left implicit |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Backoff curve calculation, dead-letter threshold logic, job-state transition legality |
| Integration | Enqueue → dequeue → execute → complete/retry/dead-letter, against a real PostgreSQL instance (`backend-test-harness.md`'s harness), including concurrent workers claiming distinct jobs via `SKIP LOCKED` (no double-execution) |
| Contract | N/A — this phase's status API is internal (Go-to-Go), not a public `/api/v1` surface consumers outside this codebase depend on |
| E2E | A synthetic job type (this spec's own worked example, not a real feature) enqueued, retried past a transient failure, and completed, proving the whole mechanism end to end |
| Accessibility | N/A — no UI this phase |

## Security considerations

- **Job payloads are internal, not external input** — every job this
  phase's infrastructure runs is enqueued by this system's own code
  (a future source-sync or import handler), never directly by a LAN
  client or an external source; constitution §4's hostile-input
  discipline still applies to whatever a job's *handler* does with
  data it fetches (a future phase's concern), not to this phase's
  queue mechanism itself
- **No new trust boundary** — job storage lives in the same PostgreSQL
  instance every other phase already trusts, no new credential or
  network exposure

## Observability

Job status/progress (Scope) is this phase's own concrete observability
surface, ahead of phase 15's own dashboards — a job's current state
(`queued`/`running`/`completed`/`retrying`/`dead_letter`), attempt
count, and last error (redacted per `backend-errors-and-logging.md`'s
existing discipline, since a future job's error could easily wrap a
source credential or similar) are all queryable, giving phase 15
something real to build on rather than starting from nothing.

## Exit criteria

- [x] The broker-or-not ADR recorded and `Accepted` (ADR `0014`)
- [x] `backend-job-queue.md` `APPROVED` with a recorded review (`0036`)
- [x] Enqueue, execute, retry, dead-letter all functional and tested,
      including concurrent-worker correctness (no double-execution) —
      `TestStore_ConcurrentClaim_NeverDoubleClaims` (20 goroutines / 8
      jobs, zero duplicates), `TestEngine_SyntheticWalkthrough`,
      `TestEngine_ExhaustsAttemptsThenDeadLetters`,
      `TestEngine_PermanentFailureDeadLettersImmediately`
- [x] Job status and progress queryable by callers — `Queue.GetJob` /
      `Queue.ListJobs`, `Store.UpdateProgress`
- [x] Test coverage across unit, integration, and E2E for this slice —
      `internal/jobs` unit (backoff, state machine, registry, redaction,
      queue validation) + integration (store operations, concurrency,
      fencing, reaper, shutdown, synthetic walkthrough); `cmd/server`
      lifecycle-ordering tests
- [ ] The spec in this phase is `VERIFIED` — pending maintainer close
- [x] Security audit recorded in `.claude/audits/` with no open Critical
      or High findings — `0009-phase09-async-jobs.md` (2 Low, 2
      Informational, all accepted)
- [x] Documentation updated — spec + roadmap + specs/audits indexes;
      `backend-service-lifecycle.md` FR-6's phase-09 amendment now realised
- [ ] Maintainer approval recorded — pending
