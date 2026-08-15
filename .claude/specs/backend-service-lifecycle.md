# Spec: Backend service lifecycle

| | |
|---|---|
| **Status** | `APPROVED` (amended post-approval twice — DSN redaction in startup failure logging, needs maintainer re-confirmation, [`0028`](../reviews/0028-spec-amendment-dsn-redaction.md); job-worker-pool shutdown ordering added for phase 09, [`0036`](../reviews/0036-phase09-cross-spec-review.md), both need maintainer re-confirmation) |
| **Phase** | `03-backend-foundation` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0022`](../reviews/0022-phase03-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (a self-contradiction between this spec's own startup ordering and its readiness claim), fixed; approved by maintainer 2026-08-14. Amended post-approval, [`0028`](../reviews/0028-spec-amendment-dsn-redaction.md) — DSN redaction gap found by security review, self-reviewed, needs maintainer re-confirmation. Amended again, [`0036`](../reviews/0036-phase09-cross-spec-review.md) — FR-6 extended with job-worker-pool shutdown ordering for `backend-job-queue.md` (phase 09), cross-spec-reviewed, needs maintainer re-confirmation |

## Context

`architecture-system.md` FR-1/FR-2 fixed that the Go server is a spawned
child process whose lifetime is a subset of Electron's, and that it in turn
spawns and owns PostgreSQL, one level down. `architecture-persistence.md`
extended `Starting` into a concrete sequence (init data directory, spawn
Postgres, wait for it, run migrations, `Ready`). Neither spec says what
*inside* `cmd/server` actually executes that sequence, in what order its
own internal dependencies get constructed, or what "shut down cleanly"
means as real Go code rather than a state-diagram arrow.

This spec is `cmd/server`'s own entry point: what it does between `func
main()` and either serving its first request or exiting non-zero, and the
same in reverse on the way down.

## Problem

Nothing has fixed: what `cmd/server`'s `main` function actually does step
by step, how its dependencies (config, logger, database pool, router) get
constructed and wired together, what "failure to start" looks like as an
exit code and a message, or the exact mechanics of a graceful shutdown
under an in-flight request — the thing phase 03's own README already
names as "the hardest thing to test here."

## Goals

- Fix the startup sequence as an ordered list, naming what each step
  depends on and what happens if it fails
- Fix dependency construction and wiring as explicit, testable
  composition — no package-level globals (phase 03's own risk table)
- Fix shutdown as a sequence with a bounded grace period, consistent with
  `architecture-system.md` FR-9
- Fix what "failed to start" means concretely: exit code, what's printed,
  what's logged
- Give `backend-test-harness.md` something concrete to build its
  shutdown-under-load test against

## Non-goals

- Configuration sourcing and precedence itself — `backend-configuration.md`;
  this spec only says *when* config is loaded in the startup sequence
- The migration runner's own mechanics — `backend-persistence.md`, building
  on ADR 0013; this spec only says *when* migrations run in the sequence
- The HTTP middleware chain's contents — `backend-http-transport.md`; this
  spec only says when the router is constructed and handed off to
  `http.Server`
- The control-plane channel to Electron (port announcement on stdout,
  `SIGTERM` handling) — `architecture-desktop-host.md` already proposed
  the mechanism; this spec implements the Go-server side of receiving it,
  doesn't redesign it
- Postgres's own spawn/lifecycle as a subprocess — `backend-persistence.md`,
  building on `architecture-persistence.md` FR-1/FR-8/FR-9/FR-10; this spec
  treats "Postgres is reachable" as a precondition its startup sequence
  waits on, not something it implements directly

## User stories

- As **`cmd/server`'s own maintainer**, I want a fixed, ordered startup
  sequence written down, so adding a new dependency (a cache, a second
  repository) has an obvious place to slot in rather than an ad hoc one.
- As **the Electron main process**, I want the Go server to either report
  ready or exit with a specific, loggable reason within a bounded time, so
  I'm never stuck waiting on a process that will never become ready.
- As **a contributor writing the shutdown test**, I want the shutdown
  sequence specified precisely enough that "did it complete the in-flight
  request or cleanly refuse it" is a fact the code either satisfies or
  doesn't, not a judgement call.

## Functional requirements

- **FR-1** `cmd/server`'s `main` function MUST perform, in this order:
  (1) load configuration (`backend-configuration.md`) and validate it,
  failing loudly on any error before anything else runs; (2) construct the
  structured logger (`backend-errors-and-logging.md`); (3) construct the
  HTTP router and middleware chain (`backend-http-transport.md`), with
  `/healthz` and `/readyz` wired to an atomically-held reference to the
  connection pool (`backend-persistence.md` FR-1) that starts empty —
  `/api/v1` routes, once phase 06 adds any, are registered here too, but
  phase 03 itself registers none (Non-goals); (4) bind the listening
  socket and start `http.Server.Serve` — the process is now "alive,"
  `/healthz` reachable, `/readyz` correctly reporting not-yet-started;
  (5) obtain a reachable PostgreSQL (in production, initialize the data
  directory if absent and spawn the platform-appropriate managed instance,
  `backend-persistence.md` FR-5; when a `DATABASE_URL` value is present
  instead — the developer/CI/test path, `backend-configuration.md` FR-4's
  third category — connect to it directly and skip the spawn step
  entirely) and run pending migrations (`architecture-persistence.md`
  FR-5, ADR 0013); (6) construct the connection pool
  (`backend-persistence.md` FR-1) and store it in the atomic reference
  step 3's handlers already read — `/readyz` now performs its real
  liveness check instead of reporting not-yet-started — then construct
  repository implementations against the pool; (7) report readiness
  (`architecture-system.md` FR-7), now also externally observable via
  `/readyz` returning 200, not only as internal process state. Steps 1–2
  and 5–6 are each a strict precondition chain (no step begins before the
  prior one completes successfully); step 3–4 (router construction and
  serving) intentionally happens *before* step 5 so that `/healthz`
  answers "the process is alive" during however long Postgres's own
  startup takes, rather than that entire window being unobservable from
  outside the process — the whole reason `architecture-system.md` FR-7
  requires the alive/ready distinction to exist in the first place.
- **FR-2** Every dependency (config, logger, database pool, repositories,
  router) MUST be constructed explicitly in `main` (or a `run` function
  `main` calls, for testability) and passed to what needs it as a
  constructor argument or struct field. No package-level `var` holding a
  shared logger, database pool, or config MUST exist anywhere in the
  module — restates phase 03's own risk table ("global state and
  package-level singletons creeping in early") as a concrete, testable
  rule: a dependency that isn't a parameter or field is a violation, full
  stop, not a judgement call about whether this particular global is
  "probably fine."
- **FR-3** If any startup step (FR-1) fails, the process MUST log a
  specific, named error identifying which step failed and why (constitution
  §11: no vague "startup failed"), then exit with a non-zero status code —
  distinct non-zero codes per failure category are not required, but the
  logged message MUST be specific enough that Electron's own failure
  surface (`architecture-system.md`'s Failure modes table) can show the
  user something more useful than "couldn't start." The process MUST NOT
  retry the failed step internally in an unbounded loop — a bounded retry
  is acceptable only for the "wait for Postgres to become reachable" step
  (FR-1 step 5), consistent with `architecture-system.md`'s `Degraded`
  state existing for exactly this case; every other step fails once and
  exits. Once that step's retry budget is exhausted without a successful
  connection, it is treated as an ordinary FR-3 startup failure like any
  other — logged, process exits non-zero — the bounded retry is a ceiling
  on how long `Degraded` is shown before giving up, never a path to
  retrying forever. When the failing step is FR-1 step 5 (obtaining a
  reachable PostgreSQL) and a `DATABASE_URL` value is in play
  (`backend-configuration.md` FR-4's third category), the logged message
  MUST use a fixed, generic description of the failure (e.g. "could not
  connect to the configured database") and MUST NOT include the
  underlying driver error's `Error()` string verbatim — `pgx` connection
  and parse errors can embed the DSN itself, the same failure mode
  `backend-http-transport.md` FR-5 already redacts for `/readyz`'s
  response body, applied here to this spec's startup log line instead
  (security review finding, 2026-08-14).
- **FR-4** On receiving a shutdown signal (`SIGTERM`, or the Electron
  control-plane channel's equivalent per `architecture-system.md`'s
  Open questions), the server MUST stop accepting new connections
  immediately, then call `http.Server.Shutdown(ctx)` with a bounded
  context timeout, allowing in-flight requests to complete within that
  window. Requests still in flight when the timeout expires MUST be
  cleanly cancelled (their handler's context is cancelled, producing a
  clean error response if the handler observes it in time) rather than
  the process being hard-killed out from under them without
  `Shutdown` having been given the chance to try.
- **FR-5** The shutdown grace period MUST be a configured value
  (`backend-configuration.md`), defaulting to the 10-second placeholder
  `architecture-system.md` FR-9 already named, not a hardcoded constant
  buried in `main` — this is what lets phase 03 replace the placeholder
  with a measured number without a code change beyond the default.
- **FR-6** After `http.Server.Shutdown` returns (whether by completing
  gracefully or by timing out), the server MUST close the database
  connection pool before the process exits, so a forcibly-terminated
  request never leaves a pooled connection in an indeterminate state for
  the next startup to inherit. **Amended for phase 09**: if a job worker
  pool (`backend-job-queue.md`) is running, its own shutdown (stop
  claiming, cancel running handlers' contexts, using this FR-5's same
  grace period as its bound) MUST be signalled between FR-4's HTTP
  shutdown and this FR's pool close — after the HTTP server stops
  accepting new work, before the shared `pgxpool` a running job's
  heartbeat/completion write depends on is closed out from under it.
  Ordering: FR-4 (HTTP `Shutdown`) → job worker pool shutdown → this
  FR-6 (close `pgxpool`) → process exit.
- **FR-7** The readiness endpoint (`architecture-system.md` FR-7,
  `backend-http-transport.md` FR-5) MUST reflect "alive but not ready"
  from the moment the HTTP listener is bound (FR-1 step 4) until
  PostgreSQL is confirmed reachable and migrated (FR-1 step 6) — this
  window is real and observable precisely because FR-1 orders "bind and
  serve" (step 4) *before* "connect to PostgreSQL" (step 5), not after:
  an earlier draft of this spec ordered the listener bind after the
  PostgreSQL step, which would have made this FR's own "not ready" window
  unreachable by construction. The atomically-held pool reference
  (FR-1 step 3/6) is the concrete mechanism — `/readyz` reads it directly,
  it is not a separately-tracked boolean that could drift from the pool's
  actual construction state.

## Non-functional requirements

- **Performance** — cold start to `Ready` inherits `architecture-system.md`'s
  3-second placeholder budget; this spec adds no new number, since nothing
  in this sequence is expected to dominate that budget over Postgres's own
  startup time (`architecture-persistence.md`'s concern, not this spec's).
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; no UI.
- **Reliability** — FR-3's "fail once, don't loop" and FR-4's "let
  in-flight work finish or cleanly refuse it" are the two reliability
  properties this spec owns. Between them: a service that never gets stuck
  retrying forever, and never drops work it was already doing.
- **Observability** — every startup step (FR-1) MUST log a line on
  success, not just on failure, at `info` level
  (`backend-errors-and-logging.md` FR-9 — promoted from an earlier
  `debug` classification specifically because the default log level is
  `info`, and a line gated behind a level nobody enables in normal
  operation doesn't actually make anything diagnosable) — this is what
  makes a slow-but-eventually-successful start diagnosable (which step
  took long) rather than only a failed one, without requiring anyone to
  have anticipated the slowness and raised the log level in advance.

## Domain model

Not applicable — this spec is process lifecycle, not the Alexandryn domain.
It does fix where repository construction (wiring domain interfaces to
`internal/persistence/postgres` implementations, `architecture-backend.md`
FR-2) happens in the startup sequence (FR-1 step 6).

## API and contracts

- **`cmd/server` ↔ Electron (control plane)**: receives `SIGTERM` (or the
  documented Windows/control-channel equivalent) for shutdown (FR-4);
  writes its bound port to stdout on startup per
  `architecture-system.md`'s proposed mechanism — this spec's FR-1 step 4
  is where that write happens, immediately after the listener binds.
- **`cmd/server` ↔ PostgreSQL**: `backend-persistence.md` owns the
  connection mechanics; this spec only fixes *when* in the sequence the
  connection is obtained and migrations run (FR-1 step 5).
- **`cmd/server` ↔ its own dependencies**: constructor injection only
  (FR-2) — the "contract" here is that nothing reaches for global state.

## State transitions

Extends `architecture-persistence.md`'s already-extended `Starting`
sequence with the parts that were still abstract there:

```
Starting -> (FR-1.1: load + validate config; fail -> Failed)
         -> (FR-1.2: construct logger)
         -> (FR-1.3: construct router + middleware chain; /healthz,
             /readyz wired to an empty pool reference)
         -> (FR-1.4: bind listener, start Serve; fail -> Failed —
             process is now "alive," /healthz reachable)
         -> (FR-1.5: connect to Postgres, wait if not yet reachable
             [bounded retry, FR-3], run migrations; fail -> Failed)
         -> (FR-1.6: construct pool, populate the atomic reference
             [/readyz now performs its real liveness check],
             construct repositories)
         -> (FR-1.7: report Ready)
Ready -> ShuttingDown (FR-4: signal received, stop accepting new conns)
ShuttingDown -> (FR-4: Shutdown(ctx) with bounded timeout)
             -> (FR-6: close database pool)
             -> Stopped
```

Illegal transitions, restated for this layer:

- `Ready` reported before the listener is actually bound and accepting
  connections (violates FR-1's ordering — step 7 strictly after step 4)
- `/readyz` returning 200 before the pool reference is populated (step 6)
  — the reference, not a separately-tracked flag, is the single source of
  truth for this
- Any startup step beginning before the previous one has completed
  successfully (violates FR-1's "no step may begin before the prior one
  has completed")
- The process exiting without attempting `Shutdown(ctx)` first, when the
  signal was a clean `SIGTERM` rather than an unrecoverable crash (violates
  FR-4)

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Config invalid or missing required value | FR-1 step 1's validation | (via Electron) startup failure naming the missing/invalid key | Exits non-zero immediately, no other step runs (FR-3) |
| Listener bind fails (port in use) | FR-1 step 4 | `Failed`, naming the port conflict | Exits non-zero (FR-3); does not retry the identical bind indefinitely; never reaches step 5 |
| PostgreSQL unreachable at startup | FR-1 step 5's connection attempt | `Degraded`, per `architecture-system.md`, with bounded retry — `/healthz` still 200 (process is alive), `/readyz` still 503 | Retries with backoff up to a bounded limit; does not proceed to step 6 until connected; retry budget exhausted → ordinary FR-3 failure, exits non-zero |
| Migration fails partway | FR-1 step 5, `architecture-persistence.md` FR-5 | `Failed`, distinct from "unreachable" | Exits non-zero; does not proceed to step 6; next startup detects the partial state and also refuses (owned by `backend-persistence.md`) |
| Shutdown signal received with a request in flight | FR-4 | Request either completes normally or receives a clean cancellation, never a truncated response | `Shutdown(ctx)` waits up to the configured grace period (FR-5), then the context is cancelled |
| Shutdown grace period expires with requests still in flight | FR-4's timeout | Those specific requests see a cancelled/reset connection, not a hang | Process proceeds to FR-6 (close pool) and exits regardless — the grace period is a ceiling, not a guarantee every request finishes |

## Security considerations

- **Fail-loud startup (FR-3) is a security property, not just a
  reliability one** — a service that starts anyway on invalid config
  (restates `architecture-backend.md` FR-5's reasoning) could start with,
  for example, an unintentionally permissive bind address if that were
  ever config-driven; failing loudly closes that path structurally rather
  than relying on every future config field being individually reviewed
  for this risk.
- **No secrets in startup logs** — FR-1's "log a line on success" and
  FR-3's "log a specific error" both cross `backend-configuration.md`'s
  and `backend-errors-and-logging.md`'s redaction boundary; this spec
  requires that boundary be respected at every log call it introduces, not
  just the ones those specs directly own.
- **Shutdown must not leave a connection pool in a state the next startup
  trusts incorrectly (FR-6)** — closing the pool explicitly, rather than
  letting process exit implicitly reclaim it, is what keeps "the process
  died" and "the process shut down cleanly" from looking the same to
  PostgreSQL's own connection accounting.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Startup sequence ordering (FR-1) with fake dependencies substituted at each step; dependency-injection wiring (FR-2) — a test asserting no package-level mutable state exists is possible via lint (see below) rather than a runtime test |
| Integration | Full startup against a real (service-container) PostgreSQL: cold start to `Ready`, and the "data directory has pending migrations" path (`backend-test-harness.md`) |
| Contract | N/A directly — `architecture-contracts.md`'s health-endpoint shape is what FR-7's readiness surface must satisfy |
| Concurrency | The shutdown-under-load test named in phase 03's own README: several requests in flight (not just one) when `SIGTERM` arrives MUST each complete or be cleanly refused, proven with deliberately slow concurrent handlers and a timer, never assumed from reading the code |

The hardest test here, same one phase 03's own document already flags:
shutdown under load. It gets its own set of deliberately slow, concurrent
test handlers, a `SIGTERM` sent partway through, and an assertion on which
of "completed" or "cleanly cancelled" happened for each — never a
truncated or hung response, for any of them.

## Acceptance criteria

- [ ] `cmd/server`'s `main` implements FR-1's seven steps in the specified
      order, provable by a test that fails if steps are reordered
- [ ] `/healthz` is reachable and returns 200 before PostgreSQL is
      connected, proven by a test that delays step 5 and confirms step 4's
      listener already answers `/healthz` during that window — the
      concrete proof behind FR-7's readiness-window fix
- [ ] A deliberately invalid config causes exit non-zero before any other
      step runs, proven with a test
- [ ] No package-level mutable state exists — enforced by the same
      import-boundary-style lint category `architecture-backend.md` FR-3
      established, or a dedicated check if the existing lint doesn't cover
      package-level `var`
- [ ] The shutdown-under-load test (Test strategy, above) passes
      deterministically, not flakily (`architecture-testing.md` FR-6)
- [ ] Every FR maps to a line in phase 03's own exit criteria

## Open questions

- **Bounded retry count/backoff for "wait for Postgres to become
  reachable" (FR-1 step 5, FR-3's exception)** — a real number is needed;
  not proposed here, since it depends on how long `architecture-
  persistence.md`'s own spawn sequence can reasonably take, which
  `backend-persistence.md` is better positioned to measure.
- **Windows shutdown signal equivalent** — FR-4 assumes `SIGTERM` is
  available; Windows's actual signal story (console control handlers,
  `CTRL_CLOSE_EVENT`) needs confirming against what Electron's own
  `child_process` actually delivers on that platform. Not resolved here;
  `architecture-desktop-host.md`'s Open questions already track the
  control-plane channel generally, this is a specific instance of it.
- **Exit codes** — FR-3 requires non-zero but doesn't fix a specific
  code-per-failure-category scheme. Left open: Electron's own failure
  surface (`architecture-system.md`'s Failure modes table) currently
  keys off the logged message, not the exit code specifically; a fixed
  scheme could be added later without breaking anything decided here.

## References

- `architecture-system.md` — FR-1, FR-2, FR-7, FR-9, the process and
  lifecycle model this spec implements the Go-server side of
- `architecture-persistence.md` — the `Starting` sequence this spec
  extends with concrete steps
- `architecture-backend.md` — FR-2 (repository interfaces), FR-5 (config
  precedence, restated more concretely by `backend-configuration.md`),
  FR-6 (middleware order, constructed at FR-1 step 3)
- ADR 0013 — migration tool, invoked at FR-1 step 5
- `backend-configuration.md` FR-2 — the precedence resolution `DATABASE_URL`'s
  presence-as-signal (FR-1 step 5) relies on
- `.claude/roadmap/03-backend-foundation/README.md` — names shutdown
  under load as "the hardest thing to test here," restated in this spec's
  Test strategy
- Constitution §11 (copy), phase 03's own risk table ("global state and
  package-level singletons creeping in early")
