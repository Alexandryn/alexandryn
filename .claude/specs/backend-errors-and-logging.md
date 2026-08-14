# Spec: Backend errors and logging

| | |
|---|---|
| **Status** | `REVIEWED` (independent, approved with changes) |
| **Phase** | `03-backend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0022`](../reviews/0022-phase03-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, now fixed; maintainer's own read still pending |

## Context

`architecture-backend.md` FR-4 fixed the error taxonomy's *shape* — a
small, fixed set of categories with a deterministic HTTP-status mapping,
decided at the transport boundary, never inside the domain.
`architecture-contracts.md` FR-5 fixed the wire-level error shape (`code`,
`message`, `correlationId`). Neither names the actual category list, and
neither designs the logging contract: what gets logged, at what level, with
what fields, and — the single highest-risk item in phase 03's own risk
table — how redaction is a property of the logger rather than a rule
someone has to remember per call site.

## Problem

Nothing has fixed: the actual error categories and which domain/
persistence failures map to which one, the log record's field names and
levels, how a correlation ID is generated and threaded through a request,
or the mechanism that makes a secret structurally unable to reach a log
line.

## Goals

- Fix the concrete error category list (`architecture-backend.md` FR-4's
  shape, filled in)
- Fix the category → HTTP status mapping, and the category →
  `architecture-contracts.md` FR-5 wire-shape mapping
- Fix the structured logging contract: fields, levels, correlation ID
  generation and propagation
- Fix redaction as a type-level guarantee, not a convention, with a test
  proving a known secret never reaches captured output — phase 03's own
  named exit criterion
- Fix what "debug" versus "a real event" means as a concrete level policy

## Non-goals

- Metrics, tracing, or anything beyond structured logs and health —
  phase 15
- The specific error codes for features that don't exist yet (a "book not
  found" code) — those arrive with each feature phase, built on this
  spec's fixed categories, not invented here
- Log storage, rotation, or shipping to an external system — out of scope
  for a phase whose whole footprint is one self-hosted process; logs go to
  stdout/stderr, and what happens to that stream after (a file, a
  systemd journal, nothing) is an operational concern for phase 99's
  packaging, not this spec

## User stories

- As **any handler in `internal/transport/http`**, I want to return a
  domain error and have it become the correct HTTP status and wire shape
  automatically, so I never hand-construct an error response.
- As **a contributor writing a new repository method**, I want a fixed,
  small set of error categories to map a Postgres failure into, so I'm not
  inventing a new category per method.
- As **the maintainer reading a log during an incident**, I want every
  request's log lines tied together by one correlation ID, and I want to
  know with certainty that whatever book someone was reading, or whatever
  credential was in play, is not sitting in that log.

## Functional requirements

- **FR-1** The fixed error category set is: `NotFound`, `InvalidInput`,
  `Unauthorized`, `Conflict`, `Unavailable`, `Internal` —
  `architecture-backend.md` FR-4's own parenthetical list, ratified here
  as the actual, closed set. A new category MUST NOT be added without
  amending this spec — restates `architecture-backend.md` FR-4's "one
  place" requirement as a closed enumeration, not an open one a future PR
  could silently extend.
- **FR-2** Category → HTTP status mapping, fixed and total (every category
  maps to exactly one status):

  | Category | HTTP status |
  |---|---|
  | `NotFound` | 404 |
  | `InvalidInput` | 400 |
  | `Unauthorized` | 401 |
  | `Conflict` | 409 |
  | `Unavailable` | 503 |
  | `Internal` | 500 |

  This mapping MUST live in exactly one place in
  `internal/transport/http` (`architecture-backend.md` FR-4's "one place,
  not scattered per handler"), as a function from category to status, not
  duplicated per handler.
- **FR-3** A domain or persistence error crossing into
  `internal/transport/http` MUST already carry one of FR-1's categories —
  `internal/domain` defines a typed error (e.g. `domain.Error` with a
  `Category` field) that repository implementations and domain logic both
  construct or wrap into; transport never receives a bare `error` and has
  to guess its category by inspecting its message or type at the
  boundary. A Postgres-specific error (e.g. a unique-constraint violation)
  MUST be translated to a domain category (e.g. `Conflict`) inside
  `internal/persistence/postgres`, before it crosses into
  `internal/domain`-typed territory — the raw driver error (ADR 0012:
  `pgx`'s own error types) MUST NOT be returned from a repository method
  un-translated.
- **FR-4** Any error that does not already carry one of FR-1's categories
  by the time it would cross the transport boundary (an unexpected panic,
  an un-translated third-party library error) MUST be treated as
  `Internal` by default — never surfaced with an unmapped or invented
  status code. This is the same "closed, total mapping" property as FR-2,
  stated for the input side: nothing reaches the client without going
  through one of the six categories.
- **FR-5** Every wire-level error response MUST use
  `architecture-contracts.md` FR-5's shape (`code`, `message`,
  `correlationId`), constructed by one shared helper in
  `internal/transport/http`, never hand-built per handler. `message` MUST
  meet constitution §11's bar (specific, no apology, says what to do next
  where applicable) and MUST NOT contain a file path, a SQL fragment, a
  stack trace, or an internal identifier not meaningful to the caller —
  restates `architecture-contracts.md`'s own Security considerations as
  an enforceable requirement on this spec's helper function specifically.
- **FR-6** Logging uses the standard library's `log/slog`, structured as
  JSON, one logger constructed once at startup
  (`backend-service-lifecycle.md` FR-1 step 2) and passed by injection —
  never a package-level default logger, same no-globals rule
  `backend-service-lifecycle.md` FR-2 already states generally.
- **FR-7** A correlation ID MUST be generated once per incoming HTTP
  request, by the logging middleware
  (`architecture-backend.md` FR-6's fixed position: logging middleware,
  before routing), attached to the request's context, and included in
  every log line produced while handling that request and in the error
  response's `correlationId` field (FR-5) if the request fails. The ID
  MUST be a randomly generated value (e.g. a UUID or equivalent), never
  derived from anything request-supplied — a client-supplied correlation
  ID MUST NOT be trusted as the log-tying value, since that would let a
  malicious client inject an ID chosen to collide with or spoof another
  request's logs.
- **FR-8** Any type carrying a value that must never be logged (a
  connection string, a future credential, any value `backend-
  configuration.md` FR-7 already marks) MUST implement **both**
  `slog.LogValuer` and `json.Marshaler`, each returning a fixed redacted
  placeholder (e.g. `"[redacted]"`) instead of the real value — this is
  what makes redaction a property of the type, checkable once, rather
  than a discipline every future `slog.Info` call site has to remember.
  Both interfaces are required, not either: `slog.LogValuer` alone
  protects a value logged as its own attribute, but `log/slog`'s JSON
  handler (FR-6) falls back to `encoding/json`'s reflection-based
  marshaling — which only respects `json.Marshaler`, not
  `slog.LogValuer` — when a *containing* struct is logged as a single
  attribute (e.g. `slog.Any("config", cfg)` with the whole `Config`
  struct, rather than each field individually). A type implementing only
  one of the two interfaces would still leak its raw value through the
  path the other interface doesn't cover. Passing such a value directly
  (not through its redacting type), or logging a containing struct as one
  attribute without every sensitive field's type implementing both
  interfaces, is a defect, not a style preference.
- **FR-9** Log level policy: `debug` — verbose, per-step detail useful only
  while actively diagnosing something (e.g. a per-request trace finer than
  the completion line `backend-http-transport.md` FR-4 already logs at
  `info`); `info` — a real event worth keeping in normal operation
  (server ready, migration applied, shutdown initiated, **and each
  startup step succeeding** — `backend-service-lifecycle.md` FR-1's "log
  a line on success" is classified `info`, not `debug`, specifically
  because these lines occur once per process lifetime and exist to make a
  slow-but-eventually-successful start diagnosable under the *default*
  log level, not only when someone has already anticipated trouble and
  raised the level in advance); `warn` — a recovered or degraded
  condition that didn't fail the request but is worth noticing (a
  retried Postgres connection attempt, `architecture-system.md`'s
  `Degraded` state entered); `error` — a request or startup step actually
  failed. The default level (`backend-configuration.md`'s `LOG_LEVEL`,
  default `info`) MUST NOT emit `debug` lines in normal operation —
  restates constitution §8's spirit that what's logged by default should
  be deliberately chosen, not maximal.
- **FR-10** A panic in any handler MUST be recovered by the outermost
  middleware (`architecture-backend.md` FR-6), logged at `error` level
  with the stack trace **server-side only**, and MUST produce an
  `Internal`-category response (FR-1/FR-2) to the client containing no
  stack trace and no panic message — restates `architecture-backend.md`
  FR-6's requirement with this spec's concrete category and logging
  mechanics filled in. If the request's context does not yet carry a
  correlation ID when recovery triggers (a panic in a middleware layer
  that runs before the logging middleware — `backend-http-transport.md`
  FR-1's fixed order places recovery outermost, ahead of logging), the
  recovery middleware itself MUST generate one, so FR-5's response never
  has an empty `correlationId` field regardless of which layer panicked.
- **FR-11** An automated check (a contract test, per
  `architecture-contracts.md` FR-3's own mechanism, or a dedicated test
  in this package) MUST assert that no error response's `message` field,
  across every endpoint the contract test exercises, contains a
  stack-trace-shaped pattern, a filesystem path, or a SQL fragment —
  `architecture-contracts.md`'s Security considerations proposed this
  general, automated version of "error responses reveal nothing"; this
  spec is where it becomes a concrete, required check rather than staying
  a proposal only the panic-specific case (FR-10) tests directly.

## Non-functional requirements

- **Performance** — structured logging via `log/slog` to stdout has no
  stated budget here; not expected to be a bottleneck at this project's
  scale, and no measurement exists yet to budget against.
- **Security** — see Security considerations below; this entire spec is
  substantially a security spec, per phase 03's own risk table naming
  redaction as its highest-likelihood, highest-impact risk.
- **Accessibility** — not applicable; no UI.
- **Reliability** — FR-4's "unmapped error defaults to `Internal`, never an
  unmapped status" is the reliability property: a client always receives
  a well-formed error response, never a raw 500 with no body or a
  framework's own generic error page.
- **Observability** — this spec effectively *is* the observability layer
  for phase 03: FR-7's correlation ID is what ties a user-visible error to
  a specific log line, satisfying `architecture-contracts.md` FR-5's
  entire reason for including the field.

## Domain model

Not applicable directly — `domain.Error`'s category field (FR-3) is a
cross-cutting type every domain operation may return, not a new business
entity. It respects constitution §3: the error taxonomy is process-layer
plumbing, not a metadata/source/library boundary concern.

## API and contracts

- **`internal/domain` → `internal/transport/http`**: `domain.Error` with a
  fixed `Category` (FR-1), constructed at the point an operation fails —
  never inferred later from a generic `error`'s message text.
- **`internal/persistence/postgres` → `internal/domain`**: Postgres/`pgx`
  errors (ADR 0012) translated to `domain.Error` categories at the
  repository boundary (FR-3) — e.g. a unique-violation Postgres error code
  becomes `domain.Error{Category: Conflict}`.
- **`internal/transport/http` → the wire**: `architecture-contracts.md`
  FR-5's shape, constructed by FR-5's shared helper, status code from
  FR-2's mapping.
- **Logging**: `log/slog`'s `Logger`, JSON handler, correlation ID via
  `slog.String("correlationId", …)` on every line within a request's
  context (FR-7).

## State transitions

Not applicable — errors and log lines are per-request events, not
long-lived state.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Repository method returns a raw `pgx` error, un-translated | FR-3's requirement, enforceable by a lint/review check on `internal/persistence/postgres` return sites, or a runtime type-assertion test | (if it slipped through) a possible raw error leaking as `Internal`'s generic message — FR-4 still ensures no raw *text* reaches the client, but the intent (a `Conflict`, not a 500) is lost | FR-4's default-to-`Internal` catches the client-facing failure; this is still a defect to fix at the source, not a behavior to rely on |
| A handler panics | FR-10's recovery middleware | Generic `Internal` response with a correlation ID, never a stack trace | Panic recovered, stack trace logged server-side at `error`, response constructed via FR-5's helper |
| A value implementing `slog.LogValuer`'s redaction is logged via `fmt.Sprintf` or string concatenation instead of a structured `slog` call | Not automatically — this is exactly the "convention people must remember" failure phase 03's risk table names, for the one path this spec's type-level fix doesn't cover | A secret in the log, if it happens | Mitigated, not eliminated: a test (Acceptance criteria) asserts the known-secret-never-appears property against the actual logging call sites this codebase uses, and code review is expected to catch a new call site that bypasses `slog` — named as a residual risk in Security considerations, not claimed fully solved |
| Client supplies its own `X-Correlation-Id` header hoping it's trusted | FR-7's requirement that the ID is always server-generated | The client-supplied header is ignored; the response carries the server-generated ID | Logging middleware never reads a client-supplied correlation value as authoritative |

## Security considerations

- **Redaction as a type property (FR-8) is this spec's central security
  mechanism** — restates phase 03's own risk table's highest-priority
  item as a concrete design: a redacting type closes the leak once, at
  the type, rather than needing every future log call site reviewed by
  hand. The dual-interface requirement (`slog.LogValuer` *and*
  `json.Marshaler`) exists because a single interface leaves a real gap:
  `slog.LogValuer` alone doesn't protect a value nested inside a struct
  that's logged as one attribute, since `log/slog`'s JSON handler falls
  back to `encoding/json` reflection in that case, which only respects
  `json.Marshaler`.
- **Residual risk named, not hidden** — a value passed to a non-`slog`
  logging path (`fmt.Println`, string concatenation into an error
  message) bypasses FR-8's protection entirely. This spec's mitigation is
  a test proving known call sites are clean (Acceptance criteria) plus
  the general project discipline of routing all logging through the one
  injected `log/slog.Logger` (FR-6) — but it's honest that "nothing
  outside `slog` calls exist" is enforced by review and convention at the
  call-site level, the same category of gap FR-8 exists to close for the
  *type* level. A future lint rule flagging any string-formatting call
  that includes a `Config` or credential-typed value would close this
  more completely; not built here, named in Open questions.
- **Error messages reveal nothing (FR-5)** — direct restatement of
  `architecture-contracts.md`'s Security considerations, made enforceable
  here via the one shared helper function.
- **Panics never reach the client (FR-10)** — direct restatement of
  `architecture-backend.md` FR-6/phase 03's own security list.
- **Correlation IDs are server-generated only (FR-7)** — prevents a
  client from spoofing or colliding with another request's log
  correlation, a minor but real integrity property for anyone later
  trying to reconstruct what happened from logs.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Category → status mapping (FR-2), totality of the mapping (FR-4, every category and the unmapped-default case), `domain.Error` construction and wrapping (FR-3), redaction (FR-8) in isolation |
| Integration | Postgres-error-to-category translation (FR-3) against a real constraint violation |
| Contract | `architecture-contracts.md`'s own contract test validates the wire shape (FR-5) end to end; FR-11's general message-pattern check runs as part of this same contract test |
| Concurrency | N/A beyond what `backend-http-transport.md` already covers for the middleware chain itself |

The specific test phase 03's own exit criteria already names as
mandatory: a known secret value is deliberately logged (through the
correct, `slog`-routed path) and asserted absent from captured output —
this is the literal proof behind FR-8, not just a unit test of the type in
isolation. FR-8's dual-interface requirement gets a second version of the
same test: the secret logged as part of a *containing* struct (e.g. the
whole `Config`), not just the field alone, since that's the specific path
`json.Marshaler` closes that `slog.LogValuer` doesn't.

## Acceptance criteria

- [ ] Every FR-1 category maps to exactly one HTTP status (FR-2), proven
      exhaustively, not spot-checked
- [ ] A deliberately unmapped/unexpected error defaults to `Internal`
      (FR-4), proven with a test
- [ ] A test proves a known secret value, run through the actual
      `slog`-based logging path, never appears in captured stdout —
      phase 03's own named exit criterion, satisfied here concretely
- [ ] The same test repeated with the secret logged as part of a
      containing struct rather than as its own attribute, proving both
      `slog.LogValuer` and `json.Marshaler` are actually implemented and
      both paths are covered (FR-8)
- [ ] A panic in a handler produces a correlation-ID-bearing `Internal`
      response with no stack trace in the body, and the stack trace does
      appear in the server-side log — both halves proven, not just one
- [ ] A panic triggered before the logging middleware assigns a
      correlation ID still produces a response with a non-empty
      `correlationId` field, proven with a test that panics inside the
      limits middleware specifically (FR-10)
- [ ] A Postgres constraint violation surfaces as `Conflict`, not
      `Internal` or a raw driver error, proven against a real database
- [ ] The contract test asserts no error `message` field across every
      exercised endpoint contains a stack-trace pattern, a filesystem
      path, or a SQL fragment (FR-11)

## Open questions

- **No lint rule yet catching a non-`slog` logging call carrying a
  sensitive value** — named as a residual risk in Security
  considerations; a real gap, not proposed to be closed in this spec
  without a concrete tool choice, which would need its own evaluation.
- **Log destination beyond stdout/stderr** (file, rotation, shipping) —
  explicitly out of scope (Non-goals); flagged here so it isn't forgotten
  when phase 99's packaging is designed.
- **Should `domain.Error` carry a wrapped underlying `error` for
  server-side-only detail, separate from the client-facing `message`?**
  Likely yes (this is roughly how FR-10's panic handling already works —
  full detail logged, generic detail returned), but not fully specified
  as a `domain.Error` field shape here; left for implementation to settle
  within this spec's fixed categories and the FR-5 wire shape's
  constraints.

## References

- `architecture-backend.md` FR-4 (error taxonomy shape), FR-6 (middleware
  order, panic recovery position)
- `architecture-contracts.md` FR-5 (wire error shape), Security
  considerations (error messages reveal nothing)
- `architecture-system.md` Observability section (correlation ID
  requirement), `Degraded` state (FR-9's `warn` level example)
- ADR 0012 — `pgx`, the source of the raw driver errors FR-3 requires be
  translated before crossing into domain-typed territory
- `backend-service-lifecycle.md` FR-1 step 2 (logger construction), FR-2
  (no-globals rule FR-6 here restates for logging specifically)
- `backend-configuration.md` FR-7 — the redaction contract this spec's
  FR-8 generalizes beyond configuration specifically
- Constitution §8 (never log secrets — the whole point of FR-8), §11
  (copy — FR-5's message bar)
- Phase 03's own risk table and exit criteria — redaction as the
  highest-priority risk, the known-secret test as a named exit criterion
