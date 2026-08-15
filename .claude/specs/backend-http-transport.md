# Spec: Backend HTTP transport

| | |
|---|---|
| **Status** | `APPROVED` (amended post-approval — static asset serving, FR-8, cross-phase review finding, self-reviewed, re-confirmed by maintainer 2026-08-14, see [`0032`](../reviews/0032-spec-amendments-phase05-cross-phase-findings.md)) |
| **Phase** | `03-backend-foundation` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0022`](../reviews/0022-phase03-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, fixed; approved by maintainer 2026-08-14. Amended post-approval, [`0032`](../reviews/0032-spec-amendments-phase05-cross-phase-findings.md) — FR-8 static asset serving added, closing a real conflict phase 05's cross-spec review found (FR-6's JSON-only rule left no way for the Go server to serve the frontend at all), self-reviewed, re-confirmed by maintainer 2026-08-14 |

## Context

`architecture-backend.md` FR-6 fixed the middleware chain's order. ADR
0011 fixed the mechanism (stdlib `ServeMux`, hand-rolled middleware,
composed by direct function wrapping). `backend-errors-and-logging.md`
fixed what the logging middleware writes and how errors become responses.
What's left: the actual limit numbers (body size, header size, timeouts —
`architecture-backend.md`'s own risk table names these as constitution
§4's "request limits before parsing"), the health/readiness endpoints
themselves, and how `backend-service-lifecycle.md`'s router construction
step (FR-1 step 5) assembles all of this into one `http.Handler`.

## Problem

Nothing has fixed: concrete byte/time limits for incoming requests, the
health and readiness endpoint shapes, or how the fixed middleware order
(`architecture-backend.md` FR-6) is actually composed against ADR 0011's
stdlib `ServeMux`.

## Goals

- Fix concrete limits: max body size, max header size, read/write/idle
  timeouts — numbers, not "reasonable limits"
- Fix the health and readiness endpoints' shapes, satisfying
  `architecture-system.md` FR-7's "alive vs. ready" distinction and
  `architecture-contracts.md`'s eventual contract-test coverage
- Fix the router/middleware composition concretely, against ADR 0011
- Fix what each middleware layer does, in the fixed order
  `architecture-backend.md` FR-6 already specifies

## Non-goals

- Any feature endpoint — phase 06 onward
- Authentication middleware's actual contents — phase 12; this spec only
  confirms the reserved slot `architecture-backend.md` FR-6 already
  placed
- CORS — no LAN-serving-to-a-different-origin scenario exists yet
  (`architecture-system.md` FR-6: same origin, same build, served
  fresh); revisit if phase 13's LAN story ever needs it
- WebSocket/SSE transport — `architecture-contracts.md`'s own Non-goals
  already deferred this to phase 14

## User stories

- As **constitution §4**, I want every request rejected on size or time
  before its body is ever parsed by a handler, enforced once in
  middleware, not per handler.
- As **`architecture-system.md`'s Electron main process**, I want a
  health endpoint that tells me "the process is up" and a readiness
  endpoint that tells me "it can actually serve data," as two distinct,
  reliable answers.
- As **phase 06's first real handler**, I want to be handed a request
  that has already passed size/timeout/logging/recovery middleware, so
  the handler itself only implements its own business logic.

## Functional requirements

- **FR-1** The middleware chain, composed in the fixed order
  `architecture-backend.md` FR-6 requires, is: panic recovery (outermost)
  → request limits → structured logging → \[auth, reserved, phase 12\] →
  routing (`ServeMux`, ADR 0011). Implemented as direct function
  composition (ADR 0011), each layer a `func(http.Handler) http.Handler`.
- **FR-2** Request limits, applied by the limits middleware before a
  handler sees the request:
  - Maximum request body size: 10 MiB, enforced via
    `http.MaxBytesReader` wrapping the request body — a request whose
    body exceeds this MUST be rejected with `InvalidInput`
    (`backend-errors-and-logging.md` FR-1) before the handler attempts to
    read it. Configurable via `backend-configuration.md`'s
    `HTTP_MAX_BODY_BYTES` key; 10 MiB is this spec's proposed default,
    sized against the largest payload any currently-scoped phase-06+
    request shape is expected to need (metadata/JSON payloads, not book
    file uploads — `domain-source.md`'s file references point at
    externally-hosted files, this server doesn't receive file bytes in
    its request bodies in the current design).
  - Maximum header size: `net/http.Server.MaxHeaderBytes`, left at Go's
    own default (1 MiB) — no phase-01/02 requirement argues for a
    different number, and inventing one without a reason would be a
    number for its own sake.
  - Read timeout: 5 seconds (`http.Server.ReadTimeout`, configurable via
    `backend-configuration.md`'s `HTTP_READ_TIMEOUT`) — bounds how long a
    slow or hostile client can hold a connection open while sending a
    request. 5 seconds is generous for a JSON request body on a loopback
    or home LAN connection (round-trip latency is single-digit
    milliseconds at most; even a slow mobile client on the same LAN
    should complete well inside this) while still being short enough
    that a client that never finishes sending is cut off quickly rather
    than holding a connection indefinitely.
  - Write timeout: 30 seconds (`http.Server.WriteTimeout`, configurable
    via `HTTP_WRITE_TIMEOUT`) — six times the read budget, since some
    responses (a library listing with many entries, once phase 06 exists)
    legitimately take longer to serve than any request body takes to
    receive; still bounded, never unlimited.
  - Idle timeout: 120 seconds (`http.Server.IdleTimeout`, configurable via
    `HTTP_IDLE_TIMEOUT`) — four times the write budget, a generous
    keep-alive window that avoids needless reconnection overhead for a
    client polling `/readyz` or making successive requests, without
    holding sockets open indefinitely once a client goes away.
  These four are round-number defaults, not derived from a specific
  measured scenario the way the body-size limit above is — flagged
  explicitly as such, distinct from claiming they're scenario-derived.
  All are placeholder-but-directionally-reasoned defaults, not measured
  against real traffic (none exists yet) — flagged in Open questions as
  confirm-or-replace once phase 06 has real endpoints to observe, the
  same placeholder pattern `architecture-system.md` already used for its
  own startup/shutdown budgets.
- **FR-3** The panic recovery middleware MUST be the outermost layer
  (FR-1), catching any panic from every layer inside it including routing
  and handlers, and MUST invoke `backend-errors-and-logging.md` FR-10's
  behavior exactly (log server-side with stack trace, respond with a
  generic `Internal` error via the shared response helper).
  `backend-errors-and-logging.md` owns *what* happens on recovery; this
  spec owns *that* it wraps every request. Because recovery sits outside
  the logging middleware (FR-1's fixed order), a panic originating in the
  limits middleware — before logging has generated a correlation ID —
  would otherwise leave `backend-errors-and-logging.md` FR-5's error
  response with no real ID to report;
  `backend-errors-and-logging.md` FR-10 requires recovery itself to
  generate a fallback ID in exactly this case, so the response's
  `correlationId` field is never empty regardless of which layer panicked.
- **FR-4** The logging middleware MUST generate the correlation ID
  (`backend-errors-and-logging.md` FR-7), attach it to the request's
  context, log a line at request start (`debug` level:
  `backend-errors-and-logging.md` FR-9) and a line at request completion
  (`info` level, including method, path, status code, and duration) — the
  completion line is what makes response time and status observable per
  request without needing metrics infrastructure phase 15 hasn't built
  yet.
- **FR-5** Two endpoints exist outside any versioned API path (they are
  infrastructure, not the domain contract `architecture-contracts.md`
  FR-4 versions):
  - `GET /healthz` — returns 200 with an empty or minimal body the
    instant the process can accept HTTP connections at all, regardless of
    PostgreSQL state. Answers "is the process alive" — never blocks on a
    database call. Reachable from the moment `backend-service-lifecycle.md`
    FR-1's listener binds, which happens *before* Postgres is connected —
    this is what makes "alive" observable independently of "ready"
    (`architecture-system.md` FR-7), rather than the two collapsing into
    one moment because the listener happened to bind only after Postgres
    was already confirmed.
  - `GET /readyz` — reads an atomically-held reference to the connection
    pool (`backend-service-lifecycle.md` FR-1's readiness mechanism,
    `backend-persistence.md` FR-1) that starts empty at process start. While
    empty, returns 503 with a body naming "not yet started"
    (`architecture-system.md`'s `Starting` state,
    `backend-errors-and-logging.md`'s `Unavailable` category). Once the
    reference is populated (Postgres connected and migrated), performs a
    lightweight liveness query (e.g. `SELECT 1`) against it on every call —
    success returns 200; failure returns 503 with a body naming "lost the
    connection" (`Degraded`), distinguishing the two 503 cases from each
    other in the response body, not just by status code. This is the
    concrete implementation of `architecture-system.md` FR-7's
    alive-vs-ready distinction.
  Neither endpoint requires authentication once phase 12 exists — a
  health check that itself requires a credential defeats its own purpose
  for automated monitoring; this is a deliberate, named exception to the
  eventual auth middleware, not an oversight (see Security
  considerations). Neither endpoint's response body, in any state, MUST
  ever include `DATABASE_URL` or any other credential-typed value — a
  connection failure surfaced by `/readyz` names *that* the connection
  failed, never the connection string or any driver-level detail that
  could embed it (`pgx` connection errors can include the DSN in their
  error text; the handler MUST use a fixed, generic message for this
  case, never the raw driver error's `Error()` string).
- **FR-6** Every `/api/v1/...`, `/healthz`, and `/readyz` response,
  success or error, MUST set `Content-Type: application/json` — no
  endpoint under those paths returns HTML, plain text, or an undeclared
  content type, keeping the contract uniform for
  `architecture-contracts.md`'s eventual contract test to validate against
  a single expectation. FR-8 below is this rule's one deliberate
  exception, scoped to a disjoint path space, not a carve-out inside the
  API surface itself.
- **FR-7** The router (`ServeMux`, ADR 0011) MUST register
  `/api/v1/...` paths (`architecture-contracts.md` FR-4) separately from
  `/healthz`/`/readyz` (FR-5), and MUST return a `NotFound`-category
  (`backend-errors-and-logging.md`) JSON response, via the same shared
  error helper every other endpoint uses, for any unmatched `/api/v1/...`
  path — never `ServeMux`'s own default plain-text 404 page, which would
  violate FR-6 and bypass `backend-errors-and-logging.md` FR-5's shared
  response shape. FR-8 below governs unmatched paths outside
  `/api/v1/...`.
- **FR-8** (Added 2026-08-14, cross-phase review finding — see amendment
  note below) The router additionally serves the embedded frontend build
  (`web/dist`, `frontend-tooling.md` FR-1/FR-6, embedded via `go:embed`
  per ADR 0008) at every path that is neither `/api/v1/...` nor
  `/healthz`/`/readyz`: a request matching a real file in the embedded
  filesystem is served with the `Content-Type` its extension implies
  (`.html` → `text/html`, `.js` → `text/javascript`, `.css` → `text/css`,
  via Go's standard `mime.TypeByExtension`, never a hardcoded map that
  drifts from the standard library's own registry); a request matching
  no real file falls back to serving `index.html` (200, `text/html`) —
  the standard SPA-fallback pattern `frontend-shell-and-routing.md`
  FR-1's client-side routing requires, since a direct navigation or
  reload at e.g. `/library` has no corresponding file in the build
  output and must still resolve to the app shell, not a 404. This uses
  Go's standard `http.FileServerFS` (or an equivalent thin wrapper)
  against the embedded `fs.FS`, never constructing a filesystem path
  from the request URL by hand — the embed is compiled into the binary
  as read-only content, so classic path-traversal-to-arbitrary-file-read
  isn't reachable the way it would be against a real filesystem, but
  using the standard library's own safe serving primitive is still the
  correct default rather than reason to hand-roll path joining.

## Non-functional requirements

- **Performance** — FR-2's timeout values are this spec's actual
  performance budget at the transport layer; no endpoint-specific budget
  exists yet since no endpoint exists yet (phase 06 onward sets those
  against real handlers).
- **Security** — see Security considerations below; FR-2 is constitution
  §4's "request limits before parsing" made concrete.
- **Accessibility** — not applicable; this is a machine transport layer.
- **Reliability** — FR-5's alive/ready distinction is what lets
  `architecture-system.md`'s `Degraded` state be observed externally
  (by Electron, and eventually by any monitoring) rather than only
  inferred from application logs.
- **Observability** — FR-4's per-request completion log line (status,
  duration, correlation ID) is the baseline every later endpoint inherits
  automatically, without adding its own logging.

## Domain model

Not applicable — this spec is transport-layer plumbing; it fixes where
domain errors (`backend-errors-and-logging.md`'s categories) become HTTP
responses, not the domain itself.

## API and contracts

- **`/healthz`, `/readyz`**: outside `architecture-contracts.md`'s
  versioned `/api/v1` surface, not tracked in `api/openapi.yaml` under
  the same versioning promise (FR-4's skew concern doesn't apply — a
  health check has no client-side state to go stale) but still documented
  in the OpenAPI file for discoverability, per
  `architecture-contracts.md` FR-2's "the file is the source of truth."
- **`/api/v1/...`**: `architecture-contracts.md` owns the actual shapes;
  this spec owns only that the router registers this prefix and the
  middleware chain applies uniformly to everything under it.
- **Error responses**: `backend-errors-and-logging.md` FR-5's shared
  helper, used by FR-7's not-found handling and by every future handler
  equally — no endpoint constructs its own error body.

## State transitions

Not applicable — this spec's endpoints (`/healthz`, `/readyz`) *read*
`architecture-system.md`'s application state; they don't define new
states of their own. `/readyz`'s 200/503 split is a direct projection of
`Ready`/`Starting`/`Degraded` onto an HTTP status.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Request body exceeds 10 MiB | FR-2's `MaxBytesReader` | `InvalidInput` JSON response, request rejected before full body read | Limits middleware rejects before the handler runs |
| Client holds connection open past the read timeout | FR-2's `ReadTimeout` | Connection closed | `net/http` enforces the timeout; no custom handling needed beyond configuring it |
| PostgreSQL unreachable before the pool reference is populated | `/readyz` reads an empty reference | 503, body naming "not yet started" | `/readyz` returns 503; `/healthz` still returns 200 (the process itself is fine) |
| PostgreSQL connection lost after the pool reference is populated | `/readyz`'s liveness query fails | 503, body naming "lost the connection," never the raw driver error text | `/readyz` returns 503 with a fixed generic message, not `pgx`'s own error string (which can embed the DSN) |
| Unmatched `/api/v1/...` route requested | `ServeMux`'s no-match case, scoped to that path prefix | `NotFound` JSON response, not a plain-text 404 | FR-7's explicit handling, not `ServeMux`'s bundled default |
| A handler panics mid-request | Recovery middleware (FR-3) | `Internal` JSON response with correlation ID | Recovered, logged server-side with stack trace, generic response returned |
| A request outside `/api/v1/...`/health paths matches no real file in the embedded build | FR-8's own fallback logic | `index.html` served (200), never a 404 | SPA-fallback routing lets the client-side router (`frontend-shell-and-routing.md` FR-1) resolve the path |

## Security considerations

- **Request limits before parsing (FR-2) is constitution §4's own
  wording**, made concrete with real numbers for the first time in this
  project.
- **`/healthz`/`/readyz` deliberately unauthenticated (FR-5)** — named
  explicitly as an intentional exception once phase 12 exists, not an
  oversight the auth middleware forgot to cover. The risk this accepts:
  an unauthenticated caller on the loopback address (or, later, the LAN
  once phase 13 permits it) learns whether the process is up and whether
  it can reach its database — no domain data, no credentials, no
  filenames. Judged acceptable because the alternative (an authenticated
  health check) breaks compatibility with essentially every external
  monitoring convention and buys little: an attacker already on the
  loopback address, or already on the authenticated LAN once phase 13
  exists, is not meaningfully more dangerous for knowing this than for
  not.
- **`ServeMux`'s default 404 replaced (FR-7)** — a small but real
  consistency requirement: an inconsistent error shape on one code path
  (the framework's default) versus every other path (this project's own
  shared helper) is exactly the kind of drift `architecture-contracts.md`
  FR-3's contract test is meant to catch, so this spec closes it at the
  source rather than relying on the contract test to notice.
- **`Content-Type` always `application/json` within the API surface
  (FR-6)** — closes off a category of content-sniffing/XSS-adjacent risk
  that only matters for HTML responses; since `/api/v1` and the health
  endpoints never intentionally serve HTML, declaring the type explicitly
  (never leaving it to `net/http`'s content-sniffing default) removes any
  ambiguity for a client that might otherwise guess wrong. FR-8's static
  serving is a disjoint path space with its own, correct content type per
  file — not an exception inside the JSON API's own surface.
- **FR-8's static serving reads only from the compiled-in embed, never
  the host filesystem** — `http.FileServerFS` against an `embed.FS`
  cannot be redirected to read an arbitrary host path regardless of what
  a request URL contains, unlike serving from a real directory would be;
  this is what makes FR-8 safe without needing its own path-traversal
  validation logic layered on top.
- **Health responses never leak `DATABASE_URL` or driver detail (FR-5)** —
  restates phase 03's own named Security consideration ("credentials
  never in the log, never in an error, never in a health response, with a
  test proving it") concretely: `/readyz`'s failure body is always a
  fixed message, never `pgx`'s own `Error()` string, which can otherwise
  contain the DSN it failed to connect with.
- **Loopback-only binding is enforced one layer down, in
  `backend-configuration.md` FR-8** — this spec's listener (FR-1) binds to
  whatever `BIND_ADDRESS` `config.Load` already validated; it does not
  re-validate the address itself, since that would duplicate a check
  this project's own precedence rules already place at the config layer.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Middleware composition order (FR-1), limits enforcement (FR-2) against an oversized/slow synthetic request, `/healthz`/`/readyz` logic against a faked database dependency |
| Integration | `/readyz` against a real PostgreSQL: reachable → 200, connection dropped mid-test → 503, full middleware chain end to end (`architecture-backend.md`'s own Test strategy names this explicitly) |
| Contract | `architecture-contracts.md` FR-3's contract test validates `/api/v1` responses against `api/openapi.yaml`, including this spec's FR-6 (`Content-Type`) and FR-5's error shape |
| Static serving | FR-8: a request for a real embedded file gets the correct `Content-Type` and body; a request for an unknown path outside `/api/v1`/health falls back to `index.html`; an `/api/v1/...` unmatched path still gets FR-7's JSON 404, never the SPA fallback — the boundary between the two fallback behaviors proven, not assumed |
| Concurrency | Idle/read/write timeout behavior under concurrent slow clients — proven, not assumed, given phase 03's own emphasis on this being the hardest thing to get right |

## Acceptance criteria

- [ ] A request over 10 MiB is rejected with `InvalidInput` before the
      handler runs, proven with a test, not just configured
- [ ] `/healthz` returns 200 whenever the process is accepting
      connections, independent of database state, proven by stopping the
      test database and confirming `/healthz` is unaffected while
      `/readyz` correctly flips to 503
- [ ] A test proves `/readyz`'s failure body never contains
      `DATABASE_URL` or any substring of it, including when the
      underlying `pgx` connection error itself would have embedded the
      DSN — phase 03's own named exit criterion ("credentials never in...
      a health response, with a test proving it")
- [ ] The middleware order (FR-1) is proven by a test asserting recovery
      catches a panic thrown from inside routing, not just from a handler
- [ ] An unmatched route returns the shared JSON error shape, not
      `ServeMux`'s default, proven with a test
- [ ] Every FR maps to a line in phase 03's own exit criteria
- [ ] A real embedded static file is served with the correct
      `Content-Type` (FR-8), proven per extension
- [ ] A path outside `/api/v1`/health matching no real file falls back
      to `index.html`, while the same kind of unmatched path *under*
      `/api/v1` still gets FR-7's JSON 404 — both proven in the same
      test to confirm the boundary is real, not assumed

## Open questions

- **FR-2's five numbers are reasoned placeholders, not measurements** —
  same status every other phase 01/03 budget has had; confirm or replace
  once phase 06 produces real traffic to observe, consistent with how
  `architecture-system.md`'s own placeholders were framed.
- **Should `/readyz` also verify the migration state (no partial
  migration pending), not just connectivity?** Leaning yes — a
  half-migrated database is not "ready" even if reachable — but not
  fixed here; `backend-persistence.md` is better positioned to say
  exactly what "migration state" means as a checkable fact.
- **Rate limiting** — not raised by any prior spec, not designed here.
  Not needed before phase 12/13 introduce a reason (authenticated,
  potentially LAN-exposed traffic) to think about abuse specifically;
  flagged so it isn't silently assumed covered by FR-2's limits, which
  bound a single request's size/time, not request *rate*.

## References

- ADR 0008 — `go:embed`/monorepo layout, the embedded `web/dist` FR-8
  serves
- `frontend-tooling.md` FR-1/FR-6 — `web/dist`'s build output, the exact
  artifact FR-8 embeds and serves
- `frontend-shell-and-routing.md` FR-1 — client-side routing, the reason
  FR-8 needs an `index.html` fallback rather than a 404 for unknown paths
- `architecture-backend.md` FR-4 (error category shape, filled in by
  `backend-errors-and-logging.md`), FR-6 (middleware order, implemented
  here)
- `architecture-system.md` FR-7 (alive vs. ready), `Degraded` state
  (`/readyz`'s 503 case)
- `architecture-contracts.md` FR-3 (contract test), FR-4 (versioning),
  FR-5 (error shape, reused via `backend-errors-and-logging.md`'s helper)
- ADR 0011 — router/middleware mechanism this spec composes against
- `backend-errors-and-logging.md` — FR-5 (shared response helper), FR-7
  (correlation ID), FR-9 (log levels), FR-10 (panic recovery contents)
- `backend-configuration.md` FR-4 — `HTTP_MAX_BODY_BYTES`,
  `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT` keys this
  spec's FR-2 fills in; FR-8 — the loopback-only `BIND_ADDRESS`
  validation this spec's listener binds to the result of
- `backend-persistence.md` FR-1 — the pool reference `/readyz` (FR-5)
  reads
- `backend-service-lifecycle.md` FR-1 steps 3–4 — where router
  construction and listener binding sit in the startup sequence
- Constitution §4 (request limits before parsing), §11 (copy)
