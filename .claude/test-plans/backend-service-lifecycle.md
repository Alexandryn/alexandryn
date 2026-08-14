# Test plan: Backend service lifecycle

| | |
|---|---|
| **Spec** | `.claude/specs/backend-service-lifecycle.md` |
| **Status** | `REVIEWED` (independent, findings fixed — [`0023`](../reviews/0023-test-plan-backend-service-lifecycle.md)) |
| **Created** | 2026-08-14 |

## What we are trying to be confident about

- `cmd/server`'s `main` (or the `run` function it calls) executes FR-1's
  seven steps in the fixed order, and a step never begins before the
  previous one has completed successfully — this is the spec's central
  claim and the one most likely to erode silently as later phases add
  dependencies to wire in.
- The alive/ready split is real, not aspirational: `/healthz` answers 200
  while Postgres is still being obtained (the window FR-7 exists to fix),
  and `/readyz` cannot report 200 before the pool reference is actually
  populated — the reference itself is the source of truth, not a boolean
  that could drift from it.
- Startup fails loudly, once, non-zero, with a specific named cause — never
  a silent fallback, never an unbounded retry, except the one named
  exception (waiting for Postgres) which itself terminates in an ordinary
  FR-3 failure once its budget is spent.
- Shutdown never truncates an in-flight request and never leaves the
  connection pool in a state a future startup could misread: every request
  in flight when `SIGTERM` arrives either completes or is cleanly
  cancelled, and the pool closes only after `Shutdown(ctx)` returns.
- No dependency reaches its consumer through a package-level global —
  FR-2 is a structural property, checkable independently of behavior.

## Risk assessment

Highest risk, concentrate here:

- **Shutdown under concurrent load.** Phase 03's own README already names
  this "the hardest thing to test here." A single slow-request test can
  pass while a race between accept-loop closure, in-flight completion, and
  pool closure still exists under concurrency. This gets deliberately
  disproportionate test effort relative to its FR count.
- **The FR-1 step ordering, specifically step 3–4 before step 5.** This is
  the exact defect review `0022` caught in an earlier draft (listener
  bound after Postgres, which made FR-7's readiness window unreachable by
  construction). A test that only checks the end state (eventually
  `Ready`) would not have caught that regression — the test has to observe
  the window while it's open.
- **FR-3's bounded-retry carve-out.** It is the one place a loop is allowed
  on purpose; the most likely coding error is either no bound at all
  (matches an earlier ambiguity flagged in review `0022`, finding #16) or a
  bound so generous it's indistinguishable from unbounded in practice.

Lower risk, merely tedious, cover but don't over-invest:

- FR-5's grace-period value coming from config rather than a hardcoded
  constant — a straightforward wiring check.
- FR-6's pool-close-after-`Shutdown` ordering — a single sequencing
  assertion.

## Layers

### Unit

Pure sequencing and wiring logic, no real Postgres:

- FR-1 step order: fake/stub constructors for config, logger, router,
  pool, and Postgres-connect, each recording when it was invoked; assert
  the recorded order matches FR-1 exactly, and that a later step's fake
  is never invoked before an earlier step's fake has returned.
- FR-2 dependency injection: a static check, not a runtime test — the
  same import-boundary-style lint category `architecture-backend.md` FR-3
  established, extended to flag package-level mutable `var` of type
  config, logger, pool, or repository, or a dedicated check if that lint
  doesn't already cover package-level `var` declarations (spec's own
  Acceptance criteria leave the exact mechanism open).
- FR-3 failure-and-exit: for each step 1–4 and 6–7, a fake that returns an
  error causes `run` to return/exit non-zero without invoking any later
  step's fake, that the failing step's own fake was invoked exactly once
  (not retried), and the logged message names the failing step.
- FR-3 bounded retry: fake Postgres-connect that fails N times then
  succeeds — succeeds once budget allows; fake that always fails — process
  exits non-zero once the fixed budget is exhausted, and the number of
  attempts actually made is asserted, not just the outcome.
- FR-4/FR-5 shutdown signal wiring: sending the shutdown signal invokes
  `http.Server.Shutdown` with a context whose deadline matches the
  configured grace period (a fake clock, no real sleep).
- FR-6 ordering: pool's `Close` is called after `Shutdown` returns, never
  before, never if `Shutdown` hasn't been invoked at all.
- FR-7 readiness source of truth: `/readyz` handler reads the same atomic
  reference the startup sequence populates — a direct unit test on the
  handler with the reference unset (503) and set (200), independent of a
  real server.

### Integration

Real boundaries — a real (service-container) PostgreSQL, real HTTP:

- Cold start to `Ready` against an empty database: migrations run, pool
  connects, `/readyz` reaches 200, matching phase 03's own exit criterion.
- Cold start against a data directory holding a migration applied
  partway (the deliberately-broken-migration fixture below) — this
  exercises FR-1 step 5's failure path and the restart-detects-partial-state
  behavior `backend-persistence.md` owns; the process exits non-zero
  without reaching step 6.
- The alive-before-ready window, proven for real: delay the Postgres
  connect step (a real but artificially slowed dependency, or a Postgres
  container started late) and confirm `/healthz` already answers 200 while
  `/readyz` still answers 503, over real HTTP against the real bound
  listener — the concrete proof FR-7's acceptance criterion asks for.
- A real invalid config (missing required key) run through the actual
  `cmd/server` binary/entrypoint: process exits non-zero, nothing else
  starts, PostgreSQL is never touched.

### Concurrency

The shutdown-under-load test named as this plan's top risk, given its own
layer rather than folded into Unit or Integration prose, so its mechanism
is fixed rather than implied:

- Runs against a real `net.Listener` bound by the actual `http.Server`
  (not `httptest.Server`, which doesn't exercise `Shutdown`'s real accept-
  loop-closure path), started via the full startup sequence.
- At least 5 concurrent requests in flight, each hitting a handler with a
  fixed artificial delay (Fixtures, below) longer than half the configured
  grace period but shorter than the full period, plus at least one request
  whose delay deliberately exceeds the grace period.
- `SIGTERM` is sent once all requests have started but before any has
  returned.
- Assertions, per request: the ones with delay under the grace period
  MUST receive a normal 200 response; the one exceeding it MUST receive
  either a connection reset or a response reflecting the cancelled
  handler context — never a hang past the grace period and never a
  truncated body.
- The whole test runs under `go test -race` (phase 03's own CI requirement
  and exit criterion) — this is the mechanism that turns "looks correct by
  inspection" into a real check against the accept-loop/in-flight-
  completion/pool-close race this risk item names.
- A second `SIGTERM` sent immediately after the first: asserted not to
  panic and not to double-close the already-closing pool — the full extent
  of what's tested for the double-signal case (see "What is deliberately
  not tested" for what's explicitly out of scope here).

### Contract

No contract of its own; `/healthz` and `/readyz`'s response shape (status
codes, body text for each state) is fixed by `backend-http-transport.md`
FR-5, not this spec — this plan's tests assert against that shape, and
`architecture-contracts.md`'s role is limited to documenting it for
discoverability, not owning it.

### End to end

Out of scope for this spec — phase 03 has no user-facing journey; the
closest thing is the integration-layer cold-start test above, which is
already the full assembled process.

### Accessibility

Not applicable — no UI (spec's own Non-functional requirements agree).

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| Config missing a required key | Exit non-zero before logger, router, or Postgres connect are touched (FR-1 step 1, FR-3) |
| Config present but malformed (unparseable value) | Same as missing: fail-loud exit, no partial startup |
| Listener port already bound by another process | FR-1 step 4 fails, process exits non-zero, step 5 never runs |
| PostgreSQL never becomes reachable (container never starts) | Bounded retry, then ordinary FR-3 failure — exits non-zero, does not hang forever |
| PostgreSQL reachable but migration fails partway (e.g. a bad migration file in the test fixture) | Exits non-zero, distinct log line from "unreachable," step 6 never runs |
| `SIGTERM` received with zero requests in flight | Shuts down at least as fast as the grace period allows, no error |
| `SIGTERM` received with several slow concurrent requests in flight, all finishing within the grace period | Every request completes normally; pool closes only after all have returned |
| `SIGTERM` received with a request that outlives the grace period | That request's context is cancelled, client sees a clean error/reset, not a hang or a truncated body; process still exits |
| `SIGTERM` received twice in quick succession | Second signal does not panic and does not double-close the pool (Concurrency layer, above) — no FR names this explicitly; this is the full extent of what's asserted, see "What is deliberately not tested" |
| A step's fake dependency returns an error on the same call the previous step's fake used to signal success (ordering probe) | Confirms step N+1's fake is never invoked — the ordering test's actual mechanism, not a separate case |

## Fixtures and test data

- Fake/stub constructors for each FR-1 dependency (config loader, logger,
  router, Postgres-connect, pool), each instrumented to record call order
  and support forced failure — no real I/O, used only in the Unit layer.
- A fake clock for grace-period and retry-backoff assertions — no real
  `time.Sleep` in any test that can avoid it, so the shutdown-under-load
  test runs in milliseconds, not seconds.
- A disposable, service-container PostgreSQL instance for the Integration
  layer, per `backend-test-harness.md`'s harness — schema seeded via the
  real migration runner, never hand-written SQL that could drift from it.
- A deliberately broken migration file, checked into test fixtures only
  (never the real migrations directory), to exercise the partial-migration
  failure path.
- Deliberately slow handlers (a fixed artificial delay, not a real
  downstream call) for the shutdown-under-load test, so the test controls
  exactly how long a request stays in flight.

No real credentials, no real user library, no copyrighted content — none
of this spec's surface touches book content, so this is a low-risk area
for that rule, but the harness-provided `TEST_DATABASE_URL`
(`backend-test-harness.md` FR-2 — read directly by the harness, never
through `internal/config`, distinct from the application's own
`DATABASE_URL`) must still point at a disposable test instance, never a
developer's real database.

## What is deliberately not tested

- The exact bounded-retry count and backoff timing for "wait for Postgres"
  — the spec's own Open questions leave the number to
  `backend-persistence.md`; this plan tests that a bound exists and is
  respected, not a specific number, until that number is fixed elsewhere.
- Windows-specific shutdown signal delivery (`CTRL_CLOSE_EVENT` or
  equivalent) — the spec's own Open questions leave this unresolved
  pending confirmation from `architecture-desktop-host.md`; only `SIGTERM`
  is tested until that's settled.
- A fixed exit-code-per-failure-category scheme — the spec explicitly
  doesn't require one yet; tests assert non-zero, not a specific code.
- Double-`SIGTERM` behavior beyond "does not panic, does not double-close
  the pool" (Concurrency layer) — specifically not tested: whether the
  second signal restarts shutdown from scratch, extends the grace period,
  or is silently ignored. No FR specifies the intended behavior precisely
  enough to assert more than the two claims above; flagged here as a gap
  worth raising in review rather than silently test-and-guess.
- A migration applied to a database already holding real rows (as opposed
  to a database with a partial/broken migration, which this plan does
  cover) — the roadmap's own risk table names this ("migrations ...
  tested against a populated database") but the actual mechanics belong to
  `backend-persistence.md`'s migration runner, not this spec's startup
  sequence; that spec's own test plan is where this case must be covered,
  not silently dropped here.
- Postgres's own spawn/orphan-prevention mechanics — `backend-persistence.md`
  owns that; this plan's integration tests treat a reachable Postgres as a
  precondition it can wait on or fail against, not something it starts
  itself outside the test harness.
- Load/performance beyond the Concurrency layer's ~5 concurrent slow
  requests — no throughput target exists in this spec's Non-functional
  requirements to test against.

## Exit criteria

- [ ] Every functional requirement (FR-1 through FR-7) maps to at least
      one test above
- [ ] Every adversarial case above has a test
- [ ] Tests were observed to fail before the implementation existed
- [ ] The suite is deterministic across repeated runs — no real sleeps, no
      unseeded randomness, no reliance on real wall-clock timing for the
      grace period or retry backoff
- [ ] The Concurrency layer's shutdown-under-load test passes under
      `go test -race`, matching phase 03's own CI requirement
