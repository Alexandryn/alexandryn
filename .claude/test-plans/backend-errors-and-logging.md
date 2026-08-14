# Test plan: Backend errors and logging

| | |
|---|---|
| **Spec** | `.claude/specs/backend-errors-and-logging.md` |
| **Status** | `REVIEWED` (independent, findings fixed — [`0026`](../reviews/0026-test-plan-backend-errors-and-logging.md)) |
| **Created** | 2026-08-14 |

## What we are trying to be confident about

- Every one of FR-1's six categories maps to exactly one HTTP status
  (FR-2), and anything that doesn't already carry a category defaults to
  `Internal` (FR-4) — the mapping is total on both ends, not just for the
  cases someone thought to test.
- A raw `pgx` driver error never crosses `internal/persistence/postgres`'s
  boundary un-translated — a unique-constraint violation becomes
  `Conflict`, not a generic 500 or a leaked driver-specific message.
- A type carrying a sensitive value is structurally unable to leak it
  through `log/slog`, whether logged as its own attribute or nested
  inside a containing struct logged as one attribute — this spec's FR-8
  generalizes `backend-configuration.md` FR-7's mechanism beyond
  `DATABASE_URL` specifically, so this plan proves the *general*
  contract with its own fixture type, not by re-testing the config case
  already covered in `backend-configuration.md`'s own plan.
- A panic never reaches the client as a stack trace, regardless of which
  middleware layer it originates in — including the layer that runs
  before a correlation ID has been assigned, the specific gap review
  `0022` caught in an earlier draft of `backend-service-lifecycle.md`
  and this spec's FR-10 was written to close on the logging side.
- A correlation ID is always server-generated, never trusted from a
  client-supplied header, and ties every log line for one request
  together.

## Risk assessment

Highest risk, concentrate here:

- **FR-8's dual-interface redaction, generalized.** The mechanism is the
  same one `backend-configuration.md` FR-7 uses for `DATABASE_URL`, but
  FR-8 is the general contract every *future* sensitive type must follow.
  A test suite that only re-exercises the `DATABASE_URL` case would prove
  nothing about whether the general mechanism actually generalizes — this
  needs its own fixture type, independent of configuration.
- **FR-10's correlation-ID-before-assignment gap.** The exact defect class
  review `0022` found once already (a panic in a middleware layer running
  before the logging middleware has attached an ID). The fix is a
  fallback in the recovery middleware itself; the test has to actually
  panic *before* that assignment point, not just anywhere.
- **FR-3's Postgres error translation.** The one place this spec depends
  on a real external system's actual error shape (`pgx`'s `*pgconn.PgError`
  with a Postgres error `Code`) rather than a shape this codebase
  controls — a synthetic/unit-level test can get the mapping logic right
  while still missing that `pgx` doesn't actually produce the shape the
  mapping function assumes.
- **FR-11's message-pattern check has almost nothing to exercise yet.**
  Phase 03 registers no domain endpoints (Non-goals) — only `/healthz`
  and `/readyz`, neither of which is expected to produce an FR-1-category
  error in normal operation. Without a deliberately error-producing test
  endpoint, this acceptance criterion is vacuously true: a check that
  scans zero real error responses. This is a genuine risk that the
  criterion looks satisfied while proving nothing.

Lower risk, merely tedious, cover but don't over-invest:

- FR-2's status-mapping table itself (six rows, table-driven, no logic
  beyond a lookup).
- FR-6's logger-construction properties (JSON handler, single injected
  instance) — straightforward structural/output-format checks, no
  branching logic to get wrong the way FR-3's translation or FR-8's
  dual-interface redaction have.
- FR-9's level-policy call sites that live in *other* specs' own code
  (startup-step success lines, request-completion lines) — each is
  verified in the test plan that owns that code
  (`backend-service-lifecycle.md`, `backend-http-transport.md`); this
  plan owns only the cross-cutting default-level-excludes-debug property
  and the specific examples FR-9 names as belonging to this spec's own
  surface (a recovered panic logged at `error`).

## Layers

### Unit

Pure mapping and type logic, no real HTTP server, no real Postgres:

- FR-1/FR-2 totality: every one of the six categories maps to exactly one
  status, table-driven, iterating FR-1's own category list rather than a
  hand-copied list in the test, so the test and the mapping function are
  checked against the same source of truth.
- FR-6 logger construction: the constructed `*slog.Logger` uses a JSON
  handler — a captured line, parsed as JSON, round-trips without error —
  and is passed into the code under test as a constructor argument or
  struct field, never read from a package-level variable; the latter is a
  structural check, not a runtime test, the same category
  `backend-configuration.md`'s FR-1 check and
  `backend-service-lifecycle.md`'s FR-2 check already use for their own
  no-globals properties (a dedicated grep-based/lint check flagging a
  package-level `var` of type `*slog.Logger` outside the one construction
  site).
- FR-4 default-to-`Internal`: a `domain.Error` constructed with an
  invalid/zero-value category, and a bare non-`domain.Error` passed to
  the mapping function, both resolve to `Internal`/500 — never an
  unmapped status, never a panic from the mapping function itself.
- FR-3 category construction and wrapping: `domain.Error` carries the
  category it was constructed with through at least one layer of
  standard-library wrapping (`fmt.Errorf("...: %w", domainErr)`), and the
  mapping function still recovers the original category — proving
  wrapping doesn't silently lose the category. This test is written
  against Go's standard `errors.Is`/`errors.As` wrapping contract in
  general, not a specific extraction mechanism inside the mapping
  function: the spec's own Open questions leave `domain.Error`'s wrapping
  shape unresolved, so this test doesn't commit to `errors.As`
  specifically over another valid implementation.
- FR-3 Postgres error translation, as a synthetic-error unit test rather
  than requiring a real database for the mapping logic itself: construct
  a `*pgconn.PgError` (ADR 0012's driver type) with a known Postgres
  `Code` (e.g. `23505` unique violation), pass it through
  `internal/persistence/postgres`'s translation function, assert
  `Conflict`; repeat for at least one other named SQLSTATE this codebase
  is expected to translate (e.g. a not-null violation, `23502`, if this
  codebase maps it — otherwise assert the fallback-to-`Internal` case for
  an unrecognized code, closing the loop with FR-4). This is the cheap,
  precise version of the correctness proof; the Integration layer below
  is what confirms `pgx` actually produces this shape for real.
- FR-5 wire-shape helper: given a `domain.Error`, the shared helper
  produces exactly `{code, message, correlationId}` — no extra fields, no
  missing ones — and `message` is asserted, per FR-5's own bar, to never
  contain a file path pattern (`/`, `\`, a drive letter), a SQL keyword
  fragment, or a stack-trace-shaped string, for every FR-1 category's
  constructed error, not just the `Internal` case.
- FR-7 correlation ID generation: two separate simulated requests produce
  two different IDs (proving randomness, not a counter or a fixed value);
  a request carrying a client-supplied `X-Correlation-Id` header produces
  a response whose `correlationId` is the server-generated value, not the
  client-supplied one — the client's header is asserted absent from
  influence on the outcome, not merely "a valid ID was returned." Both
  cases also assert the captured log line's field key is the literal
  `correlationId` (spec's own API and contracts section), not just that
  some field carries a matching value — a field-name typo would otherwise
  pass every other assertion here.
- FR-8 redaction, both interfaces, both call sites, using a fixture type
  independent of `backend-configuration.md`'s `Config`/`DatabaseURL` (a
  standalone `internal/logging` test type — call it
  `sensitiveValueFixture` — implementing both `slog.LogValuer` and
  `json.Marshaler`): (a) the value logged directly via `slog.Any` shows
  the redacted placeholder; (b) the value nested inside a containing
  struct logged as one `slog.Any` attribute still shows the placeholder,
  never the real value, at the reflected-JSON position it would otherwise
  occupy. Mirrors `backend-configuration.md`'s own FR-7 test structure
  (including its two-fixture regression guard — a `logValuerOnlyStub`
  proving case (b) actually detects the gap it exists to catch) but
  against this spec's general contract, not the config-specific instance.
- FR-9 default-level-excludes-debug: a logger constructed at the default
  `info` level, given one `debug`-level call and one `info`-level call,
  captures only the `info` line — the cross-cutting property every other
  spec's per-call-site level choice depends on being enforced somewhere.
- FR-10 panic recovery, isolated from the rest of the middleware chain: a
  handler that panics is recovered, produces an `Internal` response with
  no stack trace or panic message in the body, and the stack trace
  appears in a structured `error`-level log line server-side only.
- FR-10 non-`error` panic values: a handler that panics with a string and
  a handler that panics with `nil` are both recovered the same way, never
  producing a second panic from the recovery path's own attempt to format
  the panic value.

### Integration

Real boundaries — a real (service-container) PostgreSQL, a real assembled
middleware chain over real HTTP:

- FR-3 confirmed against reality: a genuine unique-constraint violation
  triggered against a real PostgreSQL instance (per
  `backend-test-harness.md`'s harness) surfaces as `pgx`'s actual
  `*pgconn.PgError` shape, and the same translation function used in the
  Unit-layer synthetic test correctly categorizes it as `Conflict` — this
  is what proves the Unit layer's synthetic fixture wasn't testing an
  imagined shape.
- FR-3 wiring, not just the translation function in isolation: at least
  one real `internal/persistence/postgres` repository method (whichever
  the phase 02 domain's repository interfaces provide a first real
  implementation for — `backend-persistence.md` FR-2) is called directly,
  triggering the same real unique-constraint violation, and its own
  returned `error` is asserted to already be `domain.Error{Category:
  Conflict}` — this is what the spec's Failure modes table actually names
  as the risk (a method *forgetting* to call the translator), which a
  test against the translation function alone can't catch no matter how
  correct that function is.
- FR-10's correlation-ID-before-assignment gap, against the real
  middleware chain: a handler wired behind the *full* chain
  (`backend-http-transport.md`'s fixed order — recovery outermost, ahead
  of logging) panics from inside a layer positioned before the logging
  middleware. Since the limits middleware normally rejects with an error
  rather than panicking, the injection mechanism is a test-only
  middleware stub inserted at that same position in the chain (before
  logging, after recovery) whose sole job is to panic unconditionally —
  never a search for a genuine panic-inducing edge case in production
  limits-checking code. The response still carries a non-empty
  `correlationId` — the concrete proof behind FR-10's fallback-generation
  requirement, not provable at the Unit layer alone since it depends on
  the real chain's actual order.
- FR-11's message-pattern check, deliberately exercised: since phase 03
  registers no domain endpoint that can naturally produce an FR-1-category
  error, this plan adds one dedicated test-only handler (behind the real
  chain, registered only in the test binary, never in production
  routing) that deterministically triggers each of the six FR-1
  categories via real `domain.Error` construction, through the real FR-5
  shared helper — never a hand-built wire response — so the pattern check
  proves the real construction-to-wire pipeline is clean, not just that
  the regex itself works. This closes the Risk assessment's top concern
  about this criterion being vacuously satisfied. This is a deliberate
  deviation from the spec's own Test strategy table, which assigns FR-11
  to run "as part of" `architecture-contracts.md`'s own OpenAPI-validated
  contract test (FR-3 there): that mechanism validates schema shape
  against real, currently-registered endpoints, and phase 03 has none
  that produce FR-1-category errors, so there is nothing for it to
  exercise yet. This plan's test-only handler is the interim proof until
  phase 06+ registers real domain endpoints, at which point FR-11's
  check moves into the contract test proper, exercising those — recorded
  here as an explicit, reasoned deviation, not a silent substitution.
- Concurrent panic isolation: two panicking requests fired concurrently
  against the real assembled chain, each with a deliberately slow panic
  path (so their recovery windows overlap in wall-clock time) — each
  response carries its own distinct `correlationId`, its own `Internal`
  body, and neither request's recovery observably affects the other's
  (e.g. no shared mutable state in the recovery middleware leaking a
  stack trace or ID across requests). Run under `go test -race`, the same
  mechanism `backend-service-lifecycle.md`'s own Concurrency layer
  requires for its shutdown-under-load test, since a race here would be
  the same class of defect.

### Contract

`architecture-contracts.md`'s own contract test validates the wire shape
(FR-5) end to end, per the spec's own Test strategy table — not
duplicated here; this plan's Unit-layer FR-5 test covers the shape at the
helper-function level, cheaper and independent of the contract test's own
schedule. FR-11's check is deliberately run outside this layer for now;
see the Integration section above for why.

### Concurrency

Beyond what `backend-http-transport.md`'s own test plan covers for the
middleware chain's shutdown-under-load behavior (a different property),
this spec owns one concurrency case of its own: two panics on two
different concurrent requests, each recovered independently. See the
Adversarial cases table below; the test lives in the Integration layer
above (behind the real middleware chain, since panic isolation across
concurrent requests isn't provable against a single in-process mapping
function the way the Unit-layer tests are).

### Accessibility

N/A — no UI (spec's own Non-functional requirements agree).

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| A `domain.Error` constructed with a category value outside FR-1's six (a made-up or zero-value category) | Maps to `Internal`/500 (FR-4), never a panic in the mapping function, never an unmapped status |
| A bare `error` (not `domain.Error`) reaching the transport boundary | Maps to `Internal`/500 (FR-4) — the "unexpected panic, un-translated library error" case named explicitly |
| A `pgx` error with an unrecognized/unhandled Postgres error `Code` | Falls back to `Internal`, not to a guessed category — proves FR-3's translation function has a closed, defaulting mapping of its own, mirroring FR-4's transport-level default one layer down |
| A panic whose value is not an `error` (e.g. a panic with a string or `nil`) | Recovery middleware still produces a valid `Internal` response and a server-side log line — never a second panic from the recovery path itself trying to format a non-error panic value |
| Two panics in rapid succession on two different concurrent requests | Each gets its own correlation ID and its own recovered response — one panic's recovery must not affect the other's in-flight state (this is a narrower, error-path-specific case than `backend-service-lifecycle.md`'s own concurrency layer, which covers shutdown under load, not panic isolation) |
| A client sends a request with an empty or malformed `X-Correlation-Id` header | Ignored the same as a well-formed one would be — the header is never trusted regardless of its own validity, so malformed input in it specifically produces no different behavior than valid input in it |
| A sensitive value is logged through `fmt.Sprintf`/string concatenation instead of a structured `slog` call | Explicitly NOT caught by this spec's mechanism — named in Security considerations and Open questions as a residual risk; this plan does not claim to test the absence of every possible non-`slog` call site (see "What is deliberately not tested") |
| The redacted placeholder value itself (`"[redacted]"`) happens to appear in a legitimately non-sensitive log field | Not a false-positive concern this spec addresses — the placeholder is a fixed string chosen for clarity, not for uniqueness guarantees against coincidental collision; out of scope, not flagged as a defect |

## Fixtures and test data

- A standalone `internal/logging` test-only sensitive-value fixture type,
  independent of `backend-configuration.md`'s `Config`, implementing both
  `slog.LogValuer` and `json.Marshaler` — plus a `logValuerOnlyStub`
  decoy implementing only the first, as the regression guard proving the
  redaction test can detect the gap it exists to catch (mirrors
  `backend-configuration.md`'s own FR-7 fixture pattern).
- Synthetic `*pgconn.PgError` values constructed directly with known
  Postgres error `Code`s, for the Unit-layer FR-3 translation test — no
  real database connection involved.
- A disposable, service-container PostgreSQL instance (per
  `backend-test-harness.md`) for the Integration-layer FR-3 confirmation,
  with a schema migration that includes at least one unique constraint to
  violate on demand.
- A fake/spy `slog.Handler` capturing structured output for every
  redaction and level-policy test, so assertions run against parsed
  captured attributes, not string-matching raw output.
- A test-only handler, registered only in the test binary, that
  deterministically triggers each FR-1 category in turn via real
  `domain.Error` construction and the real FR-5 helper, for the
  Integration-layer FR-11 exercise — never registered in production
  routing.
- A test-only middleware stub, inserted into the real chain at the
  position between recovery and logging, whose sole job is to panic
  unconditionally — for the Integration-layer FR-10
  correlation-ID-before-assignment test — never registered in production
  routing.
- At least one real `internal/persistence/postgres` repository method
  (whichever phase 02's domain interfaces provide a first implementation
  for) called directly against the disposable PostgreSQL instance, for
  the Integration-layer FR-3 wiring test.

No real credentials, no real user data — synthetic Postgres error values
and log fixtures only, consistent with `backend-configuration.md`'s own
fixture discipline.

## What is deliberately not tested

- Whether every future non-`slog` logging call site (a `fmt.Println`, a
  string-concatenated error message) is free of sensitive values — the
  spec's own Security considerations and Open questions name this as a
  residual risk with no lint rule yet built to catch it; this plan tests
  the known, correct (`slog`-routed) path's redaction, not the absence of
  an incorrect path nobody has written yet.
- `domain.Error`'s optional wrapped-underlying-error field shape — the
  spec's own Open questions leave whether `domain.Error` carries a
  separate server-side-only wrapped error unresolved; this plan tests
  category propagation through generic Go error wrapping (`%w`), not a
  specific field this spec doesn't yet fix.
- Log destination, rotation, or shipping beyond stdout/stderr — explicit
  Non-goal.
- Metrics or tracing correlation beyond the log-line correlation ID —
  explicit Non-goal (phase 15).
- The specific error codes/messages for domain features that don't exist
  yet (a "book not found" `NotFound` instance) — explicit Non-goal; this
  plan tests the six categories and the mapping mechanism generically,
  using synthetic errors, not feature-specific ones.

## Exit criteria

- [ ] Every functional requirement (FR-1 through FR-11) maps to at least
      one test above
- [ ] Every adversarial case above has a test
- [ ] Tests were observed to fail before the implementation existed
- [ ] The suite is deterministic across repeated runs — no real sleeps,
      no unseeded randomness in test assertions (the correlation ID
      generator itself is random by design, but tests assert
      *inequality* between two IDs, never a specific expected value)
- [ ] FR-11's message-pattern check runs against at least one real
      exercised error response per FR-1 category, not zero
- [ ] The concurrent-panic-isolation test passes under `go test -race`
- [ ] At least one real repository method's own returned error (not the
      translation function in isolation) is asserted to already carry
      the correct FR-1 category (FR-3 wiring, not just correctness)
