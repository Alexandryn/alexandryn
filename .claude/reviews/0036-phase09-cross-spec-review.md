# Review: Phase 09 — ADR 0014 and its one spec, two independent agents

| | |
|---|---|
| **Subject** | `.claude/decisions/0014-job-queue-backend-postgresql.md`, `.claude/specs/backend-job-queue.md` — plus a post-approval amendment to `.claude/specs/backend-service-lifecycle.md` (FR-6, job-worker-pool shutdown ordering) and `.claude/roadmap/09-async-jobs/README.md`, freshly expanded from a stub as part of this same work |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination, both explicitly asked to trace the claim/lease concurrency logic in depth |
| **Date** | 2026-08-15 |
| **Verdict** | Needs rework at review time (1 Blocking, independently confirmed by both passes; 6 Major across both passes; 8 Minor/Nit) — see Resolution below for fixed status |

## Summary

Both agents traced the queue's concurrency mechanism carefully and
independently converged on the same real, not merely theoretical,
double-execution gap: the reaper's crash-recovery sweep reclaimed a
`running` job purely from `locked_until < now()`, with no fencing
between that reclaim and the *original* worker's own later writes
(heartbeat, completion). A worker whose heartbeat was merely delayed
— not truly crashed — could have its lease reassigned by the reaper
while still alive and still running, and its eventual completion write
would then land unconditionally, silently clobbering whatever the
reclaiming worker had done. This directly contradicted the spec's own
Failure modes claim ("no double-execution, by construction") and the
roadmap's own exit criterion, and wasn't exercised by the originally-
described test suite, which only simulated a heartbeat that never
arrives at all, not one that arrives late. Fixed with a `lease_token`
fencing column: every write after the initial claim (heartbeat,
completion, retry, dead-letter) is now conditioned on the exact token
that claim holds, and a stale write affects zero rows and is discarded
rather than applied. Both passes also independently found the Test
strategy's "controllable clock, no real sleeps" claim was incompatible
with the FRs as originally written, which computed claimability and
lease expiry via SQL's own server-evaluated `now()` — a value the
test harness's injected `Clock` has no way to reach. Fixed by having
the claim and reaper queries take the current time as an explicit
parameter sourced from the injected clock.

## Findings

Consolidated from both passes; duplicate findings merged, each tagged
with which pass(es) raised it.

| # | Severity | Document(s) | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | `backend-job-queue.md` FR-5, FR-6 | No fencing between the reaper's reclaim and the original worker's own later writes: once `locked_until` is judged expired, the reaper reassigns the job with no way for the original (possibly still-alive) worker to learn its lease was revoked, and FR-6's completion write was unconditional — a delayed-but-alive worker could overwrite a reclaiming worker's result, a real double-execution path the spec's own Failure modes table claimed didn't exist. | Added a `lease_token` column (FR-1), regenerated on every claim/reclaim (FR-4/FR-5). Every subsequent write — heartbeat (FR-5) and the completion/retry/dead-letter write (FR-6) — is now conditioned on `WHERE id = $1 AND lease_token = $2`; a write affecting zero rows means the lease was already reclaimed, the worker cancels its handler's context and discards its own result rather than writing over the reclaiming worker's. Named explicitly as a *bounded* risk, not an eliminated one: the fencing protects the database write, not any non-idempotent side effect a stale handler performed before detecting the mismatch — stated as a handler-authoring discipline, the same honest posture FR-3 already used for payload secrets. | Both, independently |
| 2 | Major | `backend-job-queue.md` FR-4, FR-5, Test strategy | The claim query (`available_at <= now()`) and reaper sweep (`locked_until < now()`) used PostgreSQL's own server-evaluated `now()`, which the test harness's injected `Clock` has no way to reach — the Test strategy's "controllable clock... without real sleeps" claim was unachievable as the SQL was actually written. | Both queries now take the current time as an explicit `$now`/`$1` parameter, sourced from the injected `Clock` — the same interface `backend-test-harness.md` FR-6 already provides — so advancing the fake clock genuinely governs claimability and lease expiry in tests, with no real sleep required. | Both, independently |
| 3 | Major | `roadmap/09-async-jobs/README.md` Risks table vs. `backend-job-queue.md` | The roadmap's risk table claimed "a job handler's own logic decides whether a given failure is retryable at all before this spec's mechanism ever re-queues it — the boundary is stated explicitly" — but no such mechanism existed in the spec; `HandlerFunc` returned a plain `error`, and every failure went through the identical attempts-based retry comparison regardless of whether it was transient or permanent. | Added a `job.Permanent(err)` sentinel wrapper (FR-6) a handler can return to force immediate `dead_letter` regardless of remaining `attempts` — the concrete mechanism the roadmap's claim now actually describes. | Pass A |
| 4 | Major | `backend-job-queue.md` FR-10, citing `backend-service-lifecycle.md` | FR-10 cited "`backend-service-lifecycle.md`'s existing grace period" as though the job worker pool's shutdown was already integrated into that spec's sequence. It wasn't: that spec's FR-4 (HTTP shutdown) and FR-6 (close the `pgxpool`) have no step for a job worker pool, and closing the shared pool while a handler/heartbeat still uses it would be a raw connection error, not the clean cancellation FR-10 described. | `backend-job-queue.md` FR-10 now states the required ordering explicitly (HTTP shutdown → job worker pool shutdown → pool close) as an amendment this spec depends on, not assumed integration. `backend-service-lifecycle.md` FR-6 amended directly to add this step and the ordering, with its header updated to record the post-approval amendment, pending maintainer re-confirmation alongside this phase's approval. | Pass A |
| 5 | Major | `backend-job-queue.md` Security considerations, citing `backend-errors-and-logging.md` FR-8 | The original wording claimed `last_error` redaction shared "the same redaction posture `backend-errors-and-logging.md` FR-8 already established" — but the redaction was only conditional ("MUST be truncated/sanitised... if a handler's error could plausibly wrap something sensitive"), a discretionary judgment call left to each future job-kind implementer, not FR-8's structural, type-level guarantee. | `last_error` redaction is now unconditional on every write, routed through one function (not scattered call sites) using the same `slog.LogValuer`/`json.Marshaler` filter FR-8's typed secrets already use — this is the one piece of the spec that genuinely achieves FR-8's structural bar. FR-3's payload-secrets discipline is left honestly labelled as the weaker, non-structural case it actually is, not conflated with this one. | Pass A |
| 6 | Major | `backend-job-queue.md` — no FR | No FR fixed which `internal/` package the queue/worker-pool code lives in, or its position in `architecture-backend.md`'s dependency-direction rules — contrast every prior backend spec's explicit package path. | FR-1 now states `internal/jobs`, its dependency on `internal/persistence` for pool access (not a second pool), and that it must not be imported by `internal/domain`, matching `architecture-backend.md` FR-2's direction; a future handler package is free to import both. | Pass B |
| 7 | Minor | `backend-job-queue.md` FR-4 | Original text attributed double-claim prevention to `SKIP LOCKED`; the actual correctness property is `FOR UPDATE`'s row lock — `SKIP LOCKED` is a throughput property (a blocked worker moves on) layered on top, and doesn't do anything once a row is already `running` (the claim query's own `WHERE status IN (...)` filter excludes it regardless). | Reworded FR-4 to attribute the properties correctly and to state explicitly what actually excludes an already-running row from being reclaimed by the *claim* query (the `status` filter) versus what protects against the *reclaim* path (FR-5's fencing). | Pass B |
| 8 | Minor | `backend-job-queue.md` FR-7 | `baseDelay = 5s` didn't describe the actual first-retry wait, since `attempts` is already `1` at the first failure (incremented at claim time, FR-4) — the formula as written made the first retry wait `~10s`, not `~5s`. | Backoff formula changed to `baseDelay * 2^(attempts - 1)`, so the first retry genuinely waits close to `baseDelay`, with the reasoning stated inline. | Pass B |
| 9 | Minor | `backend-job-queue.md` FR-5/FR-6 | Didn't explicitly state whether the reaper's retry/dead-letter transition reuses `attempts` from the original claim or increments it again. | FR-6 now states explicitly: the reaper reuses the `attempts` value the original claim already set, never re-incrementing independently — a lease expiry counts as exactly the one failed attempt the claim already accounted for. | Pass B |
| 10 | Minor | `backend-job-queue.md` Non-goals | Cited both `backend-metadata-adapter.md` and `backend-source-adapter.md` as precedent for deferring a status HTTP endpoint. Only the `backend-source-adapter.md` half held up — `backend-metadata-adapter.md` built its own `/api/v1/discover*` endpoints as Goals in the same phase they were needed, the opposite pattern. | Citation corrected to `backend-source-adapter.md` alone. | Pass B |
| 11 | Minor | `backend-job-queue.md` FR-1 | `id`'s generation mechanism (Go-side vs. a Postgres extension default) was unstated — the latter would be an unaddressed new infrastructure dependency the "no new dependency" claim didn't cover. | FR-1 now specifies Go-side generation via the same `IDGenerator` interface `backend-test-harness.md` FR-6 already uses for correlation IDs. | Pass A |
| 12 | Minor | `backend-job-queue.md` FR-5 (reaper) | No specified `last_error` text for a reaper-triggered failure, as distinct from a handler-returned error, leaving the Failure modes table's claim under-specified for that row. | Reaper now writes a fixed message, `"worker lease expired without heartbeat"`. | Pass A |
| 13 | Minor | `backend-job-queue.md` Non-functional requirements | No mention of the job worker pool contending with HTTP request handling for `backend-persistence.md` FR-1's shared, bounded `pgxpool` — a real capacity question once phase 10 adds DB-heavy handlers. | Added to Open questions, not resolved (no load data exists yet to resolve it with). | Pass A |
| 14 | Nit | `decisions/0014-job-queue-backend-postgresql.md` Options considered | No existing Postgres-`SKIP LOCKED`-based Go queue library (e.g. `river`) was named and weighed, despite this project's own ADR convention of recording options that lost. | Added Option D, naming the tradeoff honestly — including that this project's own hand-rolled fencing needed a real fix during this same review (finding #1), a point in the existing library's favour worth recording, not glossing over. | Pass A |
| 15 | Nit | `decisions/0014-job-queue-backend-postgresql.md` Consequences (Good) | "The mechanism is well-understood... not a novel design" was stated broadly enough to imply the whole crash-recovery layer, when only the initial `SKIP LOCKED` claim query is actually a standard, off-the-shelf pattern — the heartbeat/lease/reaper layer is this project's own bespoke design (and, per finding #1, needed a real correctness fix). | Narrowed the claim to the claim query specifically; added the bespoke-design admission to Consequences (Bad) and Reversal cost, distinguishing what's genuinely portable (the public API) from what isn't (FR-4 through FR-8's poll/lease-specific internals). | Both, independently |

## Dimensions checked

Both agents independently marked these checked in depth: Security
(redaction posture — finding #5), Concurrency correctness (the entire
claim/lease/reaper mechanism, traced in detail — finding #1 is the
result), Testability (finding #2's clock-reachability gap; both
passes separately noted the originally-described test suite wouldn't
have exercised finding #1's own scenario), Cross-spec and cross-phase
citation accuracy (findings #3, #4, #5, #10), Completeness against the
roadmap's own risk table and exit criteria (finding #3), ADR quality
(constitution §12's honesty bar — findings #14, #15).

- [x] Completeness
- [x] Ambiguity
- [x] Architecture (finding #6)
- [ ] Domain correctness — N/A, this spec deliberately owns no domain types, confirmed clean by pass B against `domain-source.md`
- [x] Security
- [x] Testability
- [ ] Accessibility — N/A, no UI this phase
- [ ] UX and copy — N/A, no UI this phase
- [x] Observability (lightly checked — logging levels/fields found reasonable, no finding beyond what's folded into finding #1's fix)
- [x] Maintainability (findings #7–#9 are legibility gaps in exactly the concurrency logic finding #1 turned on)
- [x] Evolution (ADR's Reversal cost/Confidence sections — finding #15)

## Contradictions and gaps

Finding #1 is this project's first Blocking finding in a *concurrency
correctness* mechanism specifically, distinct from the SSRF/traversal
category phase 08's review (`0035`) found twice — worth noting as a
new risk category this project's reviews now have a track record of
catching: a spec's own stated guarantee ("no double-execution, by
construction") that doesn't survive tracing the actual state machine
step by step. Findings #2 and #5 are both instances of a citation
*overstating* what a cited mechanism actually guarantees (a
Test-strategy claim referencing a clock that can't reach the SQL in
question; a redaction claim borrowing FR-8's structural language for a
discretionary discipline) — a variant of the citation-drift pattern
named recurring since review `0031`, here showing up as overstatement
rather than fabrication specifically.

## What was not reviewed

Neither agent executed or compiled anything — documents-only review.
Neither agent independently verified the numeric placeholders (worker
count, poll interval, lease duration, heartbeat interval, reaper sweep,
backoff base/max) against real load data — all are explicitly flagged
as untuned placeholders in the spec's own Open questions, and both
reviewers accepted that framing rather than relitigating unmeasured
numbers. Pass B did not independently read `backend-metadata-adapter.md`
(not in its required list), so finding #10 was confirmed only against
the `backend-source-adapter.md` half by that pass; pass A confirmed the
full finding. Neither agent independently assessed Go-level panic-
recovery implementation feasibility beyond what FR-6 states, since no
code exists yet.
