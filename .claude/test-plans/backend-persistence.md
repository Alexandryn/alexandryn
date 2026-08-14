# Test plan: Backend persistence implementation

| | |
|---|---|
| **Spec** | `.claude/specs/backend-persistence.md` |
| **Status** | `REVIEWED` (independent, findings fixed — [`0029`](../reviews/0029-test-plan-backend-persistence.md)) |
| **Created** | 2026-08-14 |

## What we are trying to be confident about

- No repository method builds SQL by string concatenation with any value
  that traces back to hostile input — parameterized queries only (FR-3),
  proven against a real, deliberately hostile value, not just by absence
  of a pattern in the source.
- A multi-step domain operation's transaction is genuinely atomic: a
  mid-operation failure leaves no partial write visible to *any* reader,
  not just invisible to the same transaction handle that failed.
- The migration runner (FR-6) both works (schema reaches head, including
  against a database already holding rows — phase 03's own named exit
  criterion) and doesn't leak the connection string into a log line on
  failure, the specific gap this spec was amended to close (security
  review, `0028`).
- A failed, partial migration is detected and refused on the *next*
  startup attempt (FR-7) — proven by actually causing a partial failure
  and restarting, not inferred from `goose`'s documentation.
- Every phase 02 aggregate that needs persistence has exactly one
  repository implementation satisfying its domain interface — the
  complete eleven-aggregate list FR-2 fixes, not a subset someone forgot.

## Risk assessment

Highest risk, concentrate here:

- **FR-3's SQL injection defense.** ADR 0012's hand-written-SQL choice
  means there's no query builder making this structurally hard to get
  wrong — the spec itself says as much. A grep-based absence check
  (the spec's own suggested mechanism) proves no *known-bad pattern*
  exists today; it doesn't prove a hostile value is actually handled
  safely. Both are needed here.
- **FR-6's DSN redaction on migration failure.** The specific amendment
  this session added after a security review found the gap — needs its
  own regression-guard test, the same discipline applied to the sibling
  specs that got the same fix (`backend-service-lifecycle.md`,
  `backend-configuration.md`).
- **FR-4's transaction atomicity, checked from outside the transaction.**
  A test that only queries via the same `pgx.Tx` handle that failed can't
  distinguish "nothing was written" from "something was written but this
  handle can't see it because it's rolled back." The real proof requires
  a second, independent connection reading concurrently or immediately
  after.
- **FR-7's partial-migration-restart detection.** Named explicitly by
  phase 03's own risk table ("migrations that cannot be rolled back and
  were never tested against real data"). The mechanism (`goose`'s own
  tracking table) is trusted by the spec rather than re-implemented —
  this plan's job is to prove that trust is warranted by actually
  causing the failure, not to re-verify `goose`'s own internals.

Lower risk, but flagged rather than silently skipped:

- **FR-8's macOS supervisor.** The spec itself carries a "Lower
  confidence... unverified in this environment with no macOS available"
  caveat on the design. This plan can unit-test the *portable* part
  (spawn-argument construction, which is a pure function with no
  platform dependency) on any machine, but the `kqueue`-based monitoring
  and actual spawn/kill behavior can only be verified against a real
  macOS runner — GitHub Actions provides one, so this isn't permanently
  untestable, only untestable in this authoring environment. This is more
  than a test-plan footnote, though: `backend-test-harness.md` FR-8's own
  CI workflow stages and phase 03's own roadmap exit criterion ("CI runs
  build, vet, lint, test, race and dependency audit on every PR") never
  name a macOS job or runner. Unless one is added to CI, FR-8's actual
  behavior has no automated verification at all, ever — not just "not
  verified by this authoring pass." Flagging this as a phase-level gap to
  raise at this phase's security-audit gate (constitution's
  stop-and-ask point), not something this test plan alone can resolve.
  See "What is deliberately not tested."

## Layers

### Unit

Pure logic, fakes/mocks for the pool and connection, no real Postgres:

- FR-1 pool construction, config-driven: given a `*config.Config` with a
  non-default `DB_POOL_MAX_CONNS`, the constructed `pgxpool.Config`'s
  `MaxConns` matches; `MinConns` stays fixed at 2 regardless of config
  (it has no key, per FR-1's own reasoning) — proven by inspecting the
  constructed `pgxpool.Config` before it's used to open a real pool, no
  real connection needed.
- FR-2 repository interface satisfaction: a compile-time check (`var _
  domain.WorkRepository = (*postgres.WorkRepository)(nil)`, and the same
  for all eleven aggregates FR-2 lists — `Work`, `Edition`, `Author`,
  `LibraryEntry`, `Collection`, `Source`, `SourceOffering`,
  `ReadingProgress`, `Bookmark`, `Highlight`, `ReadingPreferences`) — no
  runtime test needed, per the spec's own acceptance criterion; this plan
  exists to confirm the list used for these compile-time assertions
  matches FR-2's complete list exactly, not a subset.
- FR-3 static check: a grep-based or `go/analysis`-based scan of
  `internal/persistence/postgres` for SQL-shaped string
  concatenation/`fmt.Sprintf` building a query around a non-constant
  value — the spec's own suggested mechanism, made concrete rather than
  left as "may suffice."
- FR-4 transaction rollback, isolated: a repository method's transaction
  is given a deliberately failing second step (a fake/mock `pgx.Tx`);
  `tx.Rollback(ctx)` is asserted called, `tx.Commit(ctx)` asserted never
  called — the mechanical half of the atomicity proof; the *visibility*
  half (does a rollback actually leave nothing for another reader) is an
  Integration-layer concern below, since a mock can't demonstrate real
  cross-connection visibility.
- FR-5 branch selection: given a fake config exposing `DATABASE_URL`
  present vs. absent, the startup path taken is asserted (spawn-sequence
  function called vs. skipped, direct-connect function called vs.
  skipped) — proven with call-recording fakes, no real process spawn or
  real Postgres.
- FR-6 migration invocation, isolated: `goose.Up` is invoked with the
  embedded `migrations` filesystem and a `*sql.DB`; the `*sql.DB` is
  asserted closed (via a wrapping spy around the `pgx/v5/stdlib` opener)
  immediately after `goose.Up` returns, regardless of whether it
  succeeded or failed — the concrete proof behind FR-6's "closed
  immediately after… never passed to, or reused by, any repository."
- FR-6 DSN-redaction regression guard: a fake `goose.Up`/connection-open
  failure is constructed to return an `error` whose `.Error()` string
  contains a synthetic, obviously-fake DSN-shaped substring (mirroring
  `backend-configuration.md`'s and `backend-errors-and-logging.md`'s own
  fixture pattern) — the logged/returned failure detail is asserted to
  not contain that substring. A second case, a genuine SQL-execution
  failure (not a connection-open failure) from a fake `goose.Up`, is
  asserted to *retain* the migration filename and `goose`'s own error
  text — proving the redaction is scoped to the connection-failure case
  specifically, not applied so broadly that real diagnostic detail is
  lost for the common case.
- FR-8 spawn-argument construction (the portable subset): given a data
  directory path and platform, the constructed `postgres`/
  `cmd/pg-supervisor` argument list is asserted correct — pure function,
  runs on any platform, no macOS or real process needed.

### Integration

Real boundaries — a real (service-container) PostgreSQL, per
`backend-test-harness.md` FR-1/FR-2 (`TEST_DATABASE_URL`, `//go:build
integration`):

- FR-2/FR-3 combined, per aggregate: each of the eleven repository
  implementations' Create/Read/Update/Delete-as-applicable methods
  against a real database, using fixture factories
  (`backend-test-harness.md` FR-4, `internal/testutil`) — not a
  standalone SQL-injection-specific suite, but the same real-data path
  every repository method needs proven correct regardless.
- FR-3 adversarial proof, concretely: a fixture factory's overridable
  string field (e.g. a `Work`'s title) is set to a deliberately hostile
  value (`O'Brien'; DROP TABLE works; --`, a value containing a null
  byte, a value containing SQL comment syntax) and passed through a real
  repository `Create` call — the value round-trips byte-for-byte on
  read-back, the `works` table still exists afterward, and no syntax
  error occurs. This outcome alone is not sufficient proof: a
  hand-rolled-escaping implementation (e.g. doubling single quotes before
  concatenating) — exactly the FR-3-violating pattern this spec
  prohibits — would pass this same assertion by coincidence, not because
  parameterization is in use. The actual proof requires instrumenting the
  query path: a `pgx.QueryTracer` (or an equivalent wrapping of the pool
  used only in this test) captures the literal SQL text and argument list
  `pgx` actually sends for this call, and the test asserts the captured
  SQL contains a placeholder (`$1`) at the hostile field's position, with
  the hostile value present only in the separate argument list, never
  interpolated into the SQL string itself — this is the real behavioral
  proof the Unit-layer static check above can't provide on its own, and
  the only version of this test that can't be passed by a disciplined-but-
  wrong hand-escaping implementation.
- FR-4 transaction atomicity, cross-connection: a multi-step domain
  operation (the reversible Work merge, `domain-bibliographic.md`
  FR-4/FR-7, this spec's own named motivating example) is driven to fail
  on its second write specifically, via fixture data engineered to
  violate a real constraint at that point (e.g. a target `Edition` row
  the merge's second step must reference, deleted out from under it by
  the test between the merge's first and second write) — a real
  constraint violation, not a mocked or injected error, so the failure
  happens inside `pgx`'s real code path. A *second*, independently
  obtained connection from the same pool queries for the operation's
  expected partial effects (the first write's row) immediately after the
  failed call returns — asserted absent. This is what actually proves
  atomicity, not just that the failing transaction's own handle reports
  nothing.
- FR-6 migration to an empty database: schema reaches head from nothing,
  proven against the real embedded migration files (not the deliberately
  broken fixture below).
- FR-6 migration to a populated database
  (`backend-test-harness.md` FR-4's fixtures seed existing rows first):
  the new schema applies and the existing rows survive — phase 03's own
  named exit criterion ("migrations apply to... a populated one"), the
  one test case explicitly missing from every other phase 03 test plan's
  walkthrough (`backend-service-lifecycle.md`'s own test plan defers this
  exact case here).
- FR-7 partial-migration-restart detection: a deliberately broken
  migration file (checked into test fixtures only, never the real
  migrations directory — the same fixture concept
  `backend-service-lifecycle.md`'s test plan already names) is run once,
  fails partway, leaving `goose`'s own tracking table in a
  partially-applied state; `goose.Up` is invoked a second time (a real
  "next startup attempt") against the same database and asserted to fail
  at the same point again, never silently proceed past it — the concrete
  proof behind FR-7's trust in `goose`'s own bookkeeping.
- Pool exhaustion (Failure modes table row 1): the pool is filled to
  `MaxConns` with held connections; one further request, given a short
  request-context timeout, observes `pgx`'s own context-deadline
  behavior rather than an indefinite block — confirming the interaction
  this spec's pool sits behind, not re-testing
  `backend-http-transport.md`'s own request-timeout-to-`Unavailable`
  mapping, which that spec's test plan already owns.
- Pool exhaustion, no context deadline at all: the pool is filled to
  `MaxConns` as above; a further request with a context carrying no
  deadline is issued from its own goroutine — asserted still blocked
  (the goroutine has not returned) after a short wall-clock wait, then
  one held connection is released and the waiting request is asserted to
  complete promptly — proving `pgxpool`'s documented default queuing
  behavior (bounded wait, released on availability) rather than a hang
  or a silent drop, without depending on any application-level ceiling
  this spec doesn't specify.
- Concurrent unique-constraint race: two goroutines, each with its own
  connection from the same pool, both call a repository `Create` method
  with fixture data that violates the same unique constraint, launched
  as close to simultaneously as `sync.WaitGroup` allows — exactly one
  succeeds, the other receives a `Conflict`-category error
  (`backend-errors-and-logging.md` FR-3's translation), no deadlock, and
  a subsequent read confirms exactly one row exists, not a silently-lost
  or duplicated write.
- Production spawn failure (Failure modes table row 4,
  `architecture-persistence.md` FR-7's named cases: corrupted data
  directory, disk full): the spawn step's command runner is replaced
  with a fake that returns a non-zero exit/error unconditionally;
  startup fails cleanly, naming that the spawn step failed
  (`backend-service-lifecycle.md` FR-3), and no automatic recovery or
  data-directory modification is attempted — `architecture-persistence.md`
  FR-7's prohibition, confirmed by asserting the fake command runner was
  invoked exactly once, never retried in an unbounded loop.
- FR-6 Observability: migration success and migration failure each
  produce a captured log line at the level `backend-errors-and-logging.md`
  FR-9 defines (`info` for success, `error` for failure) — and the
  failure line is asserted distinguishable from a plain connection-
  refused line from the *earlier* connection step (FR-5), by checking
  for a field or message fragment naming "migration" specifically, so a
  reader (or a future log-based alert) can't confuse the two failure
  classes the spec itself requires be distinguishable.

### Contract

N/A — matches the spec's own Test strategy table; repository behavior
isn't part of the wire contract.

### End to end

The dedicated bundled-spawn suite (`architecture-testing.md` FR-3,
`backend-test-harness.md` FR-7's `//go:build spawn` tag), run against the
real bundled-Postgres mechanism, not a service container:

- Linux and Windows: full FR-5/FR-6 production path — data directory
  init, platform-specific spawn with orphan-prevention
  (`architecture-persistence.md` FR-8/FR-9), migrate, ready — exercised
  for real, runnable in this authoring environment's CI.
- macOS (FR-8): the same full path via `cmd/pg-supervisor`, including the
  `kqueue`-based monitoring and orphan-prevention-on-death behavior —
  requires a real macOS runner (GitHub Actions provides one) to exercise
  meaningfully; not verifiable by this plan's authoring environment
  directly, consistent with the spec's own "unverified... no macOS
  available" caveat. This suite is where that verification finally
  happens, once implementation exists — flagged here as the specific gap
  this plan cannot close on its own, not silently assumed covered.

### Accessibility

N/A — no UI (spec's own Non-functional requirements agree).

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| A repository `Create` call with a string field containing `'; DROP TABLE works; --` | Stored and round-tripped literally on read-back; table still exists; no syntax error (FR-3) |
| A repository field containing a null byte or other Postgres-hostile-but-not-SQL-syntax byte sequence | Handled per `pgx`'s own parameter encoding — either stored correctly or rejected with a named `InvalidInput`, never a raw driver panic or an un-translated error reaching the caller |
| A domain operation's transaction fails on its very last step (commit itself fails, not an intermediate step) | Same atomicity guarantee as any other mid-operation failure — nothing partially visible, `tx.Rollback` still called since `Commit` failing doesn't imply anything was durably written |
| Two concurrent callers both attempt a transactional write that would violate the same unique constraint | One succeeds, one receives a `Conflict` category error (`backend-errors-and-logging.md` FR-3's translation) — no deadlock, no silently-lost write (Integration layer, above) |
| A migration file's connection-open failure (simulated) whose underlying error text contains a synthetic DSN-shaped string | Logged/returned detail never contains that substring (FR-6 regression guard) |
| `goose.Up` called a second time against a database already fully migrated (idempotency) | No-op, succeeds, no re-application attempted — `goose`'s own documented behavior, confirmed here rather than assumed |
| Pool exhausted, request has no context deadline set at all | Blocks per `pgxpool`'s own default queuing, then completes once a connection frees up — proven, not assumed (Integration layer, above) |

## Fixtures and test data

- `internal/testutil` fixture factories (`backend-test-harness.md` FR-4)
  for all eleven aggregates, with overridable fields for the adversarial
  string cases above.
- A synthetic, obviously-fake DSN-shaped string
  (`postgres://u:p@host/db`), the same value already used in
  `backend-configuration.md`'s own test plan's redaction regression
  guard, for this spec's FR-6 equivalent
  (`backend-errors-and-logging.md`'s own test plan uses a different,
  generic sensitive-value fixture type for its FR-8, not a DSN-shaped
  string — not the same fixture, despite testing an analogous property).
- A deliberately broken migration file, checked into test fixtures only
  (never the real `internal/persistence/postgres/migrations` directory),
  for the FR-7 partial-migration-restart test — the same fixture concept
  `backend-service-lifecycle.md`'s test plan already introduces; this
  plan reuses the concept, not a shared literal file, since each plan's
  test needs the failure at a point convenient for its own assertions.
- A disposable, service-container PostgreSQL instance
  (`TEST_DATABASE_URL`, `backend-test-harness.md` FR-2) for every
  Integration-layer case.
- A wrapping spy around the `pgx/v5/stdlib` `*sql.DB` opener, for the
  FR-6 close-immediately-after assertion.

No real credentials, no real user data, no copyrighted content — the
adversarial string fixtures above are hostile-shaped but synthetic, never
real user input captured from anywhere.

## What is deliberately not tested

- FR-8's macOS-specific `kqueue` monitoring and actual orphan-prevention
  behavior, beyond the portable spawn-argument-construction subset — the
  spec's own caveat and this plan's Risk assessment both name this
  explicitly; real verification requires a macOS CI runner and happens in
  the End to end spawn suite once implementation exists, not in this
  plan's authoring pass.
- The exact pool size numbers (`MaxConns: 10`, `MinConns: 2`) being the
  *right* numbers for real load — the spec's own Open questions defer
  this to phase 06+ once real concurrent load exists; this plan tests
  that the configured/fixed values are respected, not that they're
  optimal.
- Whether spawn-argument-construction logic is actually shared between
  the Linux/Windows path and `cmd/pg-supervisor`'s macOS path — the
  spec's own Open questions leave this as an implementation choice, not
  a requirement; this plan tests each platform's argument construction
  correctness independently of whether the code is literally shared.
- Whether `/readyz` should check migration completeness — the spec's own
  Open questions leave this to `backend-http-transport.md` to decide;
  not this spec's requirement to test.
- FR-5's Security considerations claim that "there is no Electron-supplied
  `DATABASE_URL`" in the production path — this plan's FR-5 Unit test
  proves the *branch selection* (which path is taken given the key's
  presence/absence), not the underlying claim itself, which is really a
  property of `architecture-system.md`'s process-spawn contract (what
  Electron actually passes as environment to the spawned Go process) —
  that spec's own test coverage, not this one's, is where that guarantee
  would be proven.
- AC6 ("every FR maps to a line in phase 03's own exit criteria") — a
  documentation cross-check against the roadmap README, not a runtime
  test; satisfied by this plan's own FR coverage together with a manual
  read of `.claude/roadmap/03-backend-foundation/README.md`, the same
  treatment FR-2's compile-time-only note gets above.
- The macOS supervisor dying without the Go server having exited
  (Failure modes table row 4) — the spec itself names this as "a
  residual risk, not solved," with no detection mechanism specified to
  test.

## Exit criteria

- [ ] Every functional requirement (FR-1 through FR-8) maps to at least
      one test above
- [ ] Every adversarial case above has a test
- [ ] Tests were observed to fail before the implementation existed
- [ ] The suite is deterministic across repeated runs — fixture factories
      and `FakeClock`/`FS`/`IDGenerator` where relevant
      (`backend-test-harness.md` FR-5/FR-6), truncate-based teardown
      between tests (`backend-test-harness.md` FR-3), no test depending
      on another's leftover state
- [ ] The FR-6 DSN-redaction regression guard fails if run against a
      handler that naively forwards the underlying connection error's
      `.Error()` string, proving it can detect the exact defect it exists
      to catch
- [ ] The Linux/Windows half of the End to end spawn suite passes in this
      environment's CI; the macOS half is recorded as pending a real
      macOS runner, not silently assumed passing
