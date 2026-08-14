# Test plan: Backend HTTP transport

| | |
|---|---|
| **Spec** | `.claude/specs/backend-http-transport.md` |
| **Status** | `REVIEWED` (independent, findings fixed — [`0027`](../reviews/0027-test-plan-backend-http-transport.md)) |
| **Created** | 2026-08-14 |

## What we are trying to be confident about

- The middleware chain is composed in the exact fixed order
  (`architecture-backend.md` FR-6, this spec's FR-1) — recovery outermost,
  then limits, then logging, then routing — and recovery genuinely wraps
  routing itself, not just a bare handler tested in isolation, so a panic
  anywhere inside that whole chain is caught.
- Every one of FR-2's five concrete limits (body size, header size, read/
  write/idle timeout) actually rejects or bounds a request that exceeds
  it — a configured number nobody ever observes triggering is the same as
  no limit at all.
- `/healthz` answers independently of PostgreSQL state and `/readyz`
  answers correctly from PostgreSQL state, including the specific
  security property phase 03 names as an exit criterion: `/readyz`'s
  failure body never contains `DATABASE_URL`, even when the underlying
  `pgx` connection error would itself have embedded the DSN.
- Every response — success, error, health, unmatched route — carries
  `Content-Type: application/json` and, for anything that isn't a
  success, the one shared error shape from `backend-errors-and-logging.md`
  FR-5 — never `ServeMux`'s own bundled plain-text 404.

## Risk assessment

Highest risk, concentrate here:

- **The three connection timeouts (read/write/idle) under concurrent slow
  clients.** Not because phase 03's roadmap names this the hardest thing
  to test overall — it doesn't; the roadmap's own README names graceful
  shutdown under in-flight requests as that (`backend-service-lifecycle.md`'s
  concern, correctly owned there). This spec's own Test strategy table
  uses similar wording for timeouts specifically, but the real
  justification, restated accurately here: constitution §4's five
  concrete numbers (FR-2) are unproven placeholders (spec's own Open
  questions), and a configured `net/http.Server` timeout field is easy to
  set and easy to never actually observe firing in a test suite that only
  checks the happy path.
- **`/readyz`'s `DATABASE_URL`/DSN leak prevention.** This is phase 03's
  own named security exit criterion for this spec specifically ("never in
  ... a health response, with a test proving it"). The failure mode isn't
  hypothetical: the spec itself notes `pgx` connection errors can embed
  the DSN in their `Error()` string, so a naive `/readyz` handler that
  logs or returns that string directly is a real, easy mistake, not an
  edge case.
- **FR-1's middleware order, specifically recovery-wraps-routing.** The
  acceptance criterion explicitly distinguishes "a panic from inside
  routing" from "a panic from a handler" — a test that only panics a bare
  handler function in isolation, never routed through the actual
  composed `http.Handler` including `ServeMux` dispatch, wouldn't prove
  the position claim at all, only that recovery works as a function.

Lower risk, merely tedious, cover but don't over-invest:

- FR-2's body-size limit and max header size (a stdlib default, untouched
  by this spec) — straightforward boundary checks, though the header-size
  one requires the same real-connection mechanism as the Concurrency
  layer (see Layers, below) rather than `httptest`.
- FR-6's `Content-Type` header — a single assertion repeated per endpoint
  kind.
- FR-7's unmatched-route handling — one request, one assertion.
- Whether `backend-configuration.md`'s `HTTP_MAX_BODY_BYTES`/
  `HTTP_READ_TIMEOUT`/`HTTP_WRITE_TIMEOUT`/`HTTP_IDLE_TIMEOUT` values
  actually reach the constructed `net/http.Server`'s fields — this plan's
  limit/timeout tests all construct the server with explicit test values
  directly, never through `config.Load`; the config-to-field wiring itself
  is a single, low-risk assignment this plan calls out explicitly rather
  than leaving implicit (see Layers, below).

## Layers

### Unit

Pure middleware logic and endpoint handlers, no real network beyond
`httptest`, no real Postgres:

- FR-1 composition order (structural, not yet the recovery-wraps-routing
  claim — see Integration below for that): given a request, each
  middleware layer's marker (a header it sets, or a call-order slice it
  appends to) fires in the fixed sequence — recovery, limits, logging,
  [reserved auth slot, unexercised], routing.
- FR-2 body size: a request with a body under 10 MiB is read normally; a
  request with a body at or over 10 MiB is rejected with `InvalidInput`
  (`backend-errors-and-logging.md` FR-1) before the handler function is
  ever invoked — proven by a handler that records whether it ran, not
  just by the response status.
- FR-4 logging middleware, scoped to what this spec owns (not
  re-testing `backend-errors-and-logging.md` FR-7's correlation-ID
  randomness/client-header-rejection properties, already proven in that
  spec's own test plan): a request produces a `debug`-level line at
  request start and an `info`-level line at completion containing method,
  path, status code, and a duration value greater than zero — both lines
  carry the same correlation ID.
- FR-5 `/healthz`: the router/handler constructor takes the same
  atomically-held pool-reference parameter `/readyz`'s closure reads
  (`backend-service-lifecycle.md` FR-1 step 3/6); `/healthz` is
  constructed with a poisoned spy in that slot — one whose method panics
  or fails loudly if called at all — and a request to `/healthz` returns
  200 with the spy's call count still zero afterward. This is what
  proves "never touches the database dependency," not merely "returns
  200 in the cases tested": a `/healthz` implementation that doesn't
  actually take the parameter, or takes it but never uses it, both pass
  this test for the right reason: the spy is provably never invoked.
- FR-5 `/readyz` against a faked pool reference: reference unset → 503,
  body naming "not yet started"; reference set and a fake liveness query
  succeeds → 200; reference set and the fake liveness query fails → 503,
  body naming "lost the connection" — the three states, table-driven,
  distinguished from each other by body text as the spec requires, not
  only by status code.
- FR-5 DSN-redaction regression guard: the fake liveness-query failure
  above is constructed to return an `error` whose `.Error()` string
  contains a synthetic, obviously-fake DSN-shaped substring (e.g.
  `postgres://u:p@host/db`) — the `/readyz` response body is asserted to
  not contain that substring anywhere, proving the handler uses a fixed
  message rather than forwarding the underlying error's text. This is
  the regression guard: a handler that naively did
  `fmt.Sprintf("lost the connection: %v", err)` would fail this
  specific test, not just look correct by inspection.
- FR-6 `Content-Type`: every handler and middleware-generated response
  this spec owns (`/healthz`, `/readyz` in all three states, the FR-7
  unmatched-route response, a recovered-panic `Internal` response) sets
  `Content-Type: application/json` — table-driven across all of them,
  not spot-checked on one.
- FR-7 unmatched route: a request to a path matching no registered
  pattern returns the shared `NotFound` JSON error shape
  (`backend-errors-and-logging.md` FR-5's helper), asserted by shape and
  by `Content-Type`, never `ServeMux`'s own default 404 page (asserted
  by confirming the body is valid JSON matching the shared shape, not
  `ServeMux`'s plain-text body).
- Config-to-field wiring: given a `*config.Config` with non-default
  values for all four HTTP keys, the constructed `*net/http.Server`'s
  `ReadTimeout`/`WriteTimeout`/`IdleTimeout` fields and the limits
  middleware's body-size cutoff match those values exactly — a single
  assignment-level test, independent of the Concurrency layer's own
  tests (which use their own short test-specific values for speed, not
  values sourced from config).

### Integration

Real boundaries — a real (service-container) PostgreSQL, the full
assembled middleware chain over a real `net/http.Server`:

- FR-1 recovery-wraps-routing, for real: the *complete* composed
  `http.Handler` (recovery → limits → logging → routing, exactly as
  `cmd/server` would construct it) is served over a real listener; a
  request routed to a registered handler that panics is recovered with
  an `Internal` response and a correlation ID — proving recovery's
  position is genuinely outside `ServeMux`'s own dispatch, not provable
  by unit-testing recovery against a bare handler function directly.
- FR-5 `/readyz` and `/healthz` together, against a real PostgreSQL (per
  `backend-test-harness.md`), in one test run: `/readyz` reachable at
  test start → 200, `/healthz` also 200 in the same run; PostgreSQL
  connection dropped mid-test (the real instance stopped or the pool's
  connections forcibly closed) → `/readyz` 503 "lost the connection,"
  `/healthz` still 200, checked in the same test, not a separate one —
  proving actual independence (both endpoints observed under the same
  failure condition at the same time), not just that each happens to
  pass its own isolated test. This is the real-world confirmation the
  Unit layer's faked-dependency versions can't provide on their own.
- FR-3's correlation-ID-fallback claim, cross-referenced: already owned
  and tested by `backend-errors-and-logging.md`'s own Integration layer
  (the test-only middleware stub positioned before logging); not
  duplicated here, since this spec's FR-3 explicitly delegates
  fallback-generation behavior to that spec.

### Contract

`architecture-contracts.md` FR-3's contract test validates `/api/v1`
responses against `api/openapi.yaml`, per the spec's own Test strategy
table — not duplicated here for the (currently nonexistent) `/api/v1`
surface. Unlike `backend-errors-and-logging.md`'s FR-11, this spec's own
FR-6/FR-7 properties don't need to wait for phase 06's domain endpoints
to be provable at all: `/healthz`, `/readyz`, and the unmatched-route
catch-all already exist in phase 03 and are exercised directly at the
Unit layer above — no vacuous-check gap here.

### Concurrency

The spec's own named hardest case, given a dedicated layer with a fixed
mechanism rather than left as prose:

- **Read timeout**: a client that connects and sends headers but
  deliberately withholds the body past the configured `HTTP_READ_TIMEOUT`
  — the server closes the connection at approximately that timeout, not
  earlier (false positive) and not indefinitely (missing enforcement).
  Uses a raw `net.Conn` against the real bound listener (not `httptest`,
  which doesn't exercise `net/http.Server`'s real timeout fields). The
  mechanism is a short, explicitly-configured test-specific timeout value
  (e.g. 100ms), never the real 5s default — `net/http.Server`'s
  `ReadTimeout`/`WriteTimeout`/`IdleTimeout` fields are enforced by the
  stdlib's own internal timers with no clock-injection hook of any kind
  (`backend-test-harness.md` FR-5's `Clock` interface is scoped to
  application code calling `Now()`, not these fields), so a short real
  duration is the only mechanism, not one option among several.
- FR-2 header size: a request whose total header size exceeds `net/http`'s
  own default `MaxHeaderBytes` (1 MiB) is rejected by the stock
  `net/http.Server` behavior — this spec introduces no custom logic here
  (FR-2 explicitly leaves it at Go's default). This test needs the same
  raw-`net.Conn`-against-a-real-listener mechanism as the timeout cases,
  not `httptest.NewRequest` (which constructs an `http.Request` directly
  and never parses raw header bytes off a connection, so it cannot
  exercise `MaxHeaderBytes` at all) — it exists to confirm the server is
  actually configured with that default active, not to test Go's own
  standard library.
- **Health endpoints through the same limits middleware**: FR-1 places
  limits ahead of routing for every request, so `/healthz` and `/readyz`
  are inside the same timeout/body-size enforcement as any other path —
  nothing in FR-5 exempts them. A slow client hitting `/healthz` past the
  read timeout is cut off exactly like any other route; this closes the
  gap where an implementation might special-case health endpoints outside
  the composed chain entirely, which none of the Unit-layer FR-5 tests
  (constructed against `httptest`, bypassing the real chain) could catch.
- **Write timeout**: a handler that deliberately writes its response
  slower than the configured `HTTP_WRITE_TIMEOUT` allows — the
  connection is cut and the client observes an incomplete/reset response,
  never a hang past the timeout.
- **Idle timeout**: a client that completes one request/response cycle
  and then holds the connection open (keep-alive) without sending another
  request past `HTTP_IDLE_TIMEOUT` — the server closes the idle
  connection; a second client that sends a follow-up request *within* the
  idle window reuses the connection successfully (proving the timeout
  doesn't fire early on legitimately idle-but-still-wanted connections).
- All three run under `go test -race`, the same requirement
  `backend-service-lifecycle.md`'s and `backend-errors-and-logging.md`'s
  own Concurrency layers already carry, since a timing race here is the
  same class of defect.
- At least 3 concurrent slow clients (not just one) for the read-timeout
  case specifically, proving the limits middleware's timeout enforcement
  is per-connection, not accidentally shared or serialized across
  concurrent requests.

### End to end

N/A — no user-facing journey; this spec is transport-layer plumbing over
endpoints with no client application yet.

### Accessibility

N/A — no UI (spec's own Non-functional requirements agree).

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| Request body exactly at the 10 MiB boundary | Accepted (limit is "exceeds," not "reaches") — the off-by-one case the round-number description alone doesn't settle |
| Request body one byte over 10 MiB | Rejected, `InvalidInput`, before the handler runs |
| A request that never completes sending its body | Cut off at the read timeout, never held indefinitely (Concurrency layer) |
| A handler that panics with a non-`error` value (string, `nil`) mid-response, after some bytes are already written | Already-written bytes can't be un-sent (an HTTP reality, not a defect); the connection is closed rather than left hanging — this plan asserts no server crash/goroutine leak, not a clean error body in this specific partial-write case, since that's not achievable once headers/body bytes are already flushed |
| A client sends a request to `/api/v1/../../etc/passwd`-shaped or otherwise malformed path | `ServeMux`'s own path-cleaning/matching behavior applies; the result (whether a redirect, a 404, or a matched route) still goes through FR-7's unmatched-route handling if nothing matches — never a raw filesystem access, since no handler in this phase serves files from a request-controlled path |
| PostgreSQL reachable at `/readyz` handler entry but the connection is lost between the reference check and the liveness query executing | Treated the same as any other liveness-query failure — 503, "lost the connection" — no special-cased race window carved out, since the liveness query itself is what's authoritative, not the reference's mere non-nil-ness |
| Two connections open concurrently, one idle past `HTTP_IDLE_TIMEOUT`, one actively mid-request | Only the idle one is closed; the active one is unaffected by the other's timeout (Concurrency layer) |
| `/readyz` called at extremely high frequency (a monitoring tool polling every 100ms) | Each call performs its own liveness query independently — this plan does not add caching/rate-limiting to `/readyz` (no FR requires it, and the spec's own Open questions flag rate limiting generally as out of scope for this phase) |

## Fixtures and test data

- `httptest.NewRecorder`/`httptest.NewRequest` for all Unit-layer cases
  that don't need real timeout enforcement.
- A raw `net.Conn` dialed against a real bound `net.Listener` (not
  `httptest.Server`, which doesn't exercise `net/http.Server`'s
  `ReadTimeout`/`WriteTimeout`/`IdleTimeout` fields realistically) for
  every Concurrency-layer case.
- A fake pool-reference/liveness-check dependency for the Unit-layer
  `/readyz` tests, injectable independently of a real `*pgxpool.Pool`.
- A synthetic, obviously-fake DSN-shaped string
  (`postgres://u:p@host/db`, never a real credential shape) embedded in
  a fake liveness-check error, for the FR-5 DSN-redaction regression
  guard.
- A disposable, service-container PostgreSQL instance (per
  `backend-test-harness.md`) for the Integration-layer `/readyz`
  confirmation, with a mechanism to force a connection drop mid-test
  (stopping the container or forcibly closing pool connections).
- A test-registered route whose handler deliberately panics, for the
  Integration-layer FR-1 recovery-wraps-routing test — never registered
  in production routing.

No real credentials, no real user data — the DSN fixture above is
syntactically DSN-shaped but never a real, resolvable connection string.

## What is deliberately not tested

- Whether FR-2's five limit *values* (10 MiB, 5s/30s/120s, 1 MiB header
  default) are the *right* numbers for real traffic — the spec's own Open
  questions say these are reasoned placeholders, confirmed or replaced
  once phase 06 produces real traffic to observe. This plan tests that
  each configured value is actually enforced, not that the value itself
  is correct.
- Whether `/readyz` should also verify migration state, not just
  connectivity — the spec's own Open questions leave this explicitly
  undecided pending `backend-persistence.md`; this plan tests the
  connectivity-only behavior as currently specified.
- Rate limiting on any endpoint, including `/readyz` under high-frequency
  polling — the spec's own Open questions name this as out of scope for
  this phase.
- CORS behavior — explicit Non-goal (no cross-origin scenario exists
  yet).
- The authentication middleware's reserved slot's actual contents —
  explicit Non-goal (phase 12); this plan confirms the slot exists in the
  composition order (Unit layer, FR-1) but not any behavior inside it,
  since none exists yet.
- `backend-errors-and-logging.md` FR-7's correlation-ID randomness and
  client-header-rejection properties — already covered by that spec's own
  test plan; this plan only confirms the logging middleware's position
  and its own two log lines (FR-4), not re-proving properties owned
  elsewhere.
- Whether `/healthz`/`/readyz` are actually documented in
  `api/openapi.yaml` — the spec's own API and contracts section requires
  this for discoverability, but it's a documentation-presence check, not
  a runtime behavior; out of scope for this plan's tests, better suited
  to a lint/CI check on the OpenAPI file itself if one doesn't already
  exist (`architecture-contracts.md`'s concern, not this spec's).

## Exit criteria

- [ ] Every functional requirement (FR-1 through FR-7) maps to at least
      one test above
- [ ] Every adversarial case above has a test
- [ ] Tests were observed to fail before the implementation existed
- [ ] The suite is deterministic across repeated runs — Concurrency-layer
      tests use short, test-specific timeout values, never the
      production-scale 5s/30s/120s defaults, so the suite doesn't spend
      minutes per run
- [ ] All Concurrency-layer tests pass under `go test -race`
- [ ] The FR-5 DSN-redaction regression guard fails if run against a
      handler that naively forwards the underlying error's `.Error()`
      string, proving it can detect the exact defect it exists to catch
- [ ] Every FR maps to a line in phase 03's own exit criteria, matching
      the spec's own acceptance criterion 6
