# Test plan: Backend test harness

| | |
|---|---|
| **Spec** | `.claude/specs/backend-test-harness.md` |
| **Status** | `REVIEWED` (independent, findings fixed — [`0030`](../reviews/0030-test-plan-backend-test-harness.md)) |
| **Created** | 2026-08-14 |

## What we are trying to be confident about

This spec is unusual among phase 03's six: it *is* the test-mechanics
document every other spec's own Test strategy section points to, rather
than a feature with its own behavior to test. "Testing the test harness"
means proving its mechanisms work as advertised, using them, before any
other spec's tests are considered implementable against it — the spec's
own Test strategy section says exactly this.

- The default `go test ./...` command never requires PostgreSQL, Docker,
  or any external service — proven structurally (build-tag exclusion),
  not just by it happening to pass on a machine that has Postgres
  running anyway.
- Running the integration tag without `TEST_DATABASE_URL` set fails
  loudly and immediately, never silently skips — the spec's own FR-2
  requirement, worded there as "no silent skip" and attributed to
  `architecture-testing.md` FR-6's determinism rule; that FR-6's own text
  is actually about flaky tests/retries rather than skip-vs-fail
  semantics specifically, an attribution this plan inherits from the
  already-`APPROVED` spec rather than introduces, flagged here rather
  than silently repeated as if independently verified.
- Truncate-based test isolation (FR-3) actually isolates — including
  under the specific composability hazard review `0022` already caught
  once (an outer test transaction not composing with a repository
  method's own internal transaction) and a second, not-yet-named hazard
  this plan checks for: whether truncate-based teardown is safe if a
  future contributor adds `t.Parallel()` to an integration test.
- The `Clock`, `FS`, and `IDGenerator` fakes (FR-5/FR-6) genuinely
  decouple a test from real time, real disk, and real randomness — proven
  by a canary test for each that would fail if the fake didn't actually
  intercept the real thing.
- The CI workflow (FR-8) implements its eight stages in order and each
  one actually blocks merge on failure, not just exists in the YAML file
  unwired.

## Risk assessment

Highest risk, concentrate here:

- **FR-2's fail-loud-not-skip requirement.** This is a well-known Go
  testing anti-pattern in the wrong direction — a missing environment
  variable silently skipping a whole test file is the default behavior
  many test setups fall into by accident (`t.Skip` in a `TestMain` or an
  early-return guard). Proving the harness fails loudly instead requires
  actually running the tagged suite with the variable unset and reading
  the outcome, not just reading the code.
- **FR-3's truncate-based isolation, both hazards.** The composability
  hazard review `0022` already found once (outer transaction vs.
  repository-internal transactions) is exactly the kind of defect that
  looks fixed in prose but silently regresses if a future repository
  method changes its own transaction handling. The parallelism hazard
  (truncate-based teardown and `t.Parallel()` sharing overlapping tables)
  isn't named anywhere in the spec at all — this plan's own contribution
  is surfacing it rather than assuming FR-3's silence on the topic means
  it's safe.
- **FR-8/FR-9's CI workflow, as an actual gate, not just a YAML file that
  looks right.** A stage existing in a workflow file and a stage that
  actually blocks merge on failure are different claims; only the second
  one is what phase 03's own exit criteria require ("CI runs build, vet,
  lint, test, race and dependency audit **on every PR**").

Lower risk, merely tedious, cover but don't over-invest:

- FR-4's fixture-factory default-value validity — a single canary
  assertion per aggregate type, not a design decision with tradeoffs.
- FR-5/FR-6's `Clock`/`FS`/`IDGenerator` fakes' basic mechanics, once one
  canary proves the pattern works — the pattern itself (construct once,
  inject, fake in tests) is already proven correct in
  `backend-service-lifecycle.md`'s and other sibling specs' own plans;
  this plan owns proving the *fakes themselves* work, not re-proving the
  injection pattern generally.

## Layers

Traditional Unit/Integration/Contract/E2E layering fits awkwardly here,
since this document's own subject is test infrastructure, not
application behavior. Retained where it maps cleanly; a CI infrastructure
layer is added for what doesn't.

### Unit

Canary tests proving each fake's own mechanics in isolation, no real
Postgres, no real CI:

- FR-5 `Clock`: a `FakeClock` is constructed at a known time; code that
  reads elapsed duration via `Clock.Now()` twice, with `FakeClock.Advance()`
  called between reads, observes exactly the advanced duration — proven
  without any real `time.Sleep`. This plan owns proving the fake's own
  mechanics; it does **not** re-run the spec's own AC 6 scenario (the
  shutdown-grace-period timeout firing at exactly the configured
  duration) — that scenario-specific proof is
  `backend-service-lifecycle.md`'s test plan's Concurrency layer, which
  already implements it against `FakeClock`. AC 6 is satisfied by that
  plan, cross-referenced here rather than duplicated.
- FR-6 `FS`: a fake `FS` is constructed with no files "present"; code
  calling `Stat` on a path through the fake observes a not-exists result
  without touching the real filesystem at all (proven by running the
  test in an environment where the real path, if checked, would error
  differently — e.g. a path syntactically invalid for the real OS but
  valid as a fake-map key). This plan owns the fake's own mechanics; the
  spec's own AC 5 scenario (`backend-configuration.md` FR-5's
  config-file-absent path, proven via the fake) is that spec's own test
  plan's responsibility once written — not yet cross-referenceable since
  `backend-configuration.md`'s test plan predates this FS-fake detail and
  should be checked/updated to cite it explicitly (flagged here as a
  follow-up, not silently assumed already covered).
- FR-6 `IDGenerator`: a fake constructed with a fixed sequence produces
  exactly that sequence across repeated calls, proven deterministic
  across multiple test runs. This plan owns proving the fake is
  deterministic; it does **not** re-run the spec's own AC 9 scenario (a
  specific correlation ID reaching both the log line and the error
  response) — that scenario-specific proof is
  `backend-errors-and-logging.md`'s test plan's FR-7 Unit tests, which
  already assert the same field key and value across both log and
  response using this fake. AC 9 is satisfied by that plan, cross-
  referenced here rather than duplicated.
- FR-1 structural proof (not a runtime test), both directions: (a) the
  negative control — a file suffixed `_integration_test.go` with
  `//go:build integration` at its top, containing an intentionally-broken
  reference, does not fail `go build ./...` or `go vet ./...` when the
  `integration` tag is not passed; (b) the positive control — the same
  file, built *with* `-tags=integration`, does fail, proving the file's
  contents are genuinely parsed/compiled when the tag is supplied, not
  that the toolchain simply ignores the file's directory unconditionally.
  Both directions together are what make "no external service required
  by default, but real when asked for" true by construction, not just
  the negative half in isolation.

### Integration

A real (service-container) PostgreSQL, using the harness's own
conventions to test the harness's own conventions. `internal/testutil`
below is a naming placeholder per the spec's own Open questions — tests
are written against the behavior the package provides, not a final,
fixed import path:

- FR-2 fail-loud-not-skip, for real: `go test -tags=integration ./...`
  is actually invoked as a subprocess with `TEST_DATABASE_URL` explicitly
  unset, and the outcome is captured — asserted: non-zero exit, output
  naming `TEST_DATABASE_URL` specifically. To distinguish a real
  fail-fast from a per-test skip that still reports "0 failed, N
  skipped" (which satisfies "didn't silently pass" far more weakly than
  FR-2 requires), a fixture integration test in the tagged package writes
  a sentinel file on entry (its first line, before any assertion) — after
  the subprocess exits, the outer test asserts that sentinel file does
  not exist, proving the fixture test's body was never entered at all,
  not merely that it reported a failure or a skip.
- FR-2 fail-loud-not-skip, empty-vs-unset: the same subprocess invocation
  is repeated with `TEST_DATABASE_URL` set to an empty string rather than
  left unset entirely — asserted to fail the same way, closing the gap a
  naive presence check (`if val, ok := os.LookupEnv(...); ok` without
  also checking `val != ""`) would leave open.
- FR-3 isolation, real and adversarial, two named variants: **variant
  A** — two integration tests (`TestIsolationA`, `TestIsolationB`) are
  written against the *same* table, deliberately run in the same `go
  test` invocation with real teardown between them; `TestIsolationB`
  asserts zero rows exist at its own start, proving `TestIsolationA`'s
  data didn't leak forward. **Variant B** — a third test,
  `TestIsolationComposability`, uses a fixture factory to call a real
  repository method (which opens and commits its own internal
  transaction, per `backend-persistence.md` FR-4), and a fourth test,
  `TestIsolationComposabilityFollowup`, runs immediately after it in the
  same invocation and asserts the table is empty — proving truncate-based
  teardown composes with self-transacting repository code specifically
  (the exact gap review `0022` found), not just with simple
  single-statement test setup like variant A.
- FR-3 schema-at-head: an integration test run against a freshly
  provisioned, unmigrated database connection (harness setup applies
  `goose.Up` itself, per FR-3's own requirement) succeeds without the
  test itself needing to run or know about migrations — proving the
  harness's setup step, not the test author, is responsible for reaching
  migration head.
- FR-4 fixture factory defaults: one representative aggregate per owning
  domain spec — `testutil.NewWork(t)` (`domain-bibliographic.md`),
  `testutil.NewLibraryEntry(t)` (`domain-library.md`),
  `testutil.NewSourceOffering(t)` (`domain-source.md`),
  `testutil.NewReadingProgress(t)` (`domain-reading.md`) — not all
  eleven, that duplication belongs to `backend-persistence.md`'s own
  per-aggregate integration tests. Three of these four carry a required
  foreign key into an already-persisted parent (`LibraryEntry` into
  `Work`, `SourceOffering` into `Edition`, `ReadingProgress` into
  `LibraryEntry`) — "zero required overrides" (spec AC 3) means the
  factory itself constructs and persists the necessary parent row(s)
  internally when the caller supplies none, not that the aggregate has
  no foreign key at all. Each factory call with zero overrides is
  asserted to produce a value passing every phase 02 invariant and
  persistable via a real repository `Create` call without a foreign-key
  or validation error — proving the factory's internal parent-
  construction handles the dependency, not just that a caller who
  already has a parent row can pass it in.

### CI infrastructure

Not a `go test` layer — verification against the actual GitHub Actions
workflow file and, where necessary, a live CI run:

- FR-8 stage presence and order: a static check (parsing the workflow
  YAML, not running it) asserts the eight stages appear in the specified
  order: `web/` build, `go build` (both binaries), vet/lint, unit tests,
  integration tests, contract test, coverage reporting, dependency scan.
- FR-8 actual gating, live: a deliberately broken commit (a failing unit
  test, per the spec's own acceptance criterion) is pushed to a test
  branch with a PR opened against it; the PR's required-status-check
  state is confirmed red and merge is confirmed blocked by branch
  protection — this is the one case in this plan that cannot be a `go
  test` function at all, since it verifies GitHub's own merge-blocking
  behavior, not application code. Recorded as a one-time operational
  verification step when the workflow is first stood up, re-run whenever
  the workflow file changes in a way that could affect gating.
- FR-9 race detector, static: the workflow YAML's unit-test and
  integration-test steps are checked for the `-race` flag actually
  present in the invoked command, not merely claimed in a comment or a
  job name.
- FR-8 stage 7 (coverage) presence: the workflow YAML is checked for a
  coverage-reporting step existing and producing captured output — no
  threshold assertion, since the spec's own Open questions leave the
  tool and threshold unfixed.
- FR-8 stage 3 (`web/` build failure), live: a deliberate breakage of the
  `web/` build specifically (not a unit-test failure) is pushed to a test
  branch — the workflow is confirmed to fail at that stage and never
  reach the later test stages, proving `web/`-before-`go build` ordering
  actually gates, the specific scenario Failure modes table row 3 names
  and the FR-8 stage-order static check above doesn't exercise on its
  own (order in the YAML file and actual early-exit-on-failure are
  different claims).
- FR-3 parallelism hazard, static analysis (not a `go test`, grouped here
  with this layer's other static checks rather than under Unit): a
  grep-based scan of every `_integration_test.go` file for
  `t.Parallel()` — none MUST call it, since truncate-based teardown
  (FR-3) has no per-test isolation mechanism (like a per-test schema or
  transaction) that would make concurrent truncation of overlapping
  tables safe. This is a gap the spec itself doesn't address; this
  plan's contribution is closing it with an enforceable rule rather than
  leaving it to be discovered later as a flaky-test investigation.

### Contract

N/A — this spec's own Test strategy table doesn't define a contract
surface; `architecture-contracts.md` FR-3's contract test is one of the
eight CI stages this plan verifies exists and is ordered correctly
(above), not a contract this spec itself exposes.

### End to end

N/A — no user-facing journey; this is test infrastructure consumed by
other specs' own test suites, not a feature with its own E2E path. The
closest analog, the dedicated bundled-spawn suite (FR-7), is verified as
its own item below rather than under this heading, since it isn't a
user-facing journey either.

### FR-7: the bundled-spawn suite's own separation

- A file tagged `//go:build spawn` is confirmed excluded from both the
  default `go test ./...` and the `-tags=integration` run — three-way
  tag separation (none/`integration`/`spawn`), not just two-way, proven
  the same structural way as FR-1's check above.
- The spawn suite's CI job is confirmed distinct from the integration
  job in the workflow YAML (FR-8's static check, extended) — this plan
  doesn't assert a specific trigger/cadence for it, since the spec's own
  Open questions leave that undecided.

### Accessibility

N/A — no UI (spec's own Non-functional requirements agree).

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| `go test -tags=integration ./...` with `TEST_DATABASE_URL` set to an unreachable host | A clear connection-failure message naming the harness's own connection attempt, not a raw driver panic or a hang past any reasonable timeout |
| `go test -tags=integration ./...` with `TEST_DATABASE_URL` set to an empty string, distinct from genuinely unset | Fails the same way as unset (Integration layer, above) — closes the gap a naive `LookupEnv`-only presence check would leave open |
| `go test -tags=integration ./...` with `TEST_DATABASE_URL` set to a syntactically invalid connection string | A clear parse-failure message, distinguishable from the unreachable-host case above |
| An `_integration_test.go` file missing the `//go:build integration` tag (author error) | Runs under the default `go test ./...` and attempts a real connection — this plan does not add a lint rule catching this specific naming-vs-tag mismatch (the tag, not the filename, is what Go actually reads); named here as a known, unenforced gap rather than silently assumed impossible |
| A future contributor adds `t.Parallel()` to an integration test | Caught by the FR-3 static analysis check (CI infrastructure layer, above) before merge, not discovered later as an intermittent failure |
| The bundled-spawn suite (FR-7) accidentally included in the fast per-PR integration run (tag misconfiguration) | Caught by the FR-7 three-way tag-separation check (above) — the spawn suite's slower, platform-sensitive tests never silently join the fast suite's runtime budget |
| Two contributors' local `TEST_DATABASE_URL` values point at genuinely different databases (one stale, one fresh) | Out of this harness's control — the harness's own responsibility (FR-3) is isolating tests *within* one run against one database, not reconciling two contributors' differing local setups; not a defect this plan tests for |

## Fixtures and test data

- Intentionally-broken sentinel files (a `_integration_test.go` with an
  unresolvable reference, used only to prove build-tag exclusion) —
  never committed to the real test suite, constructed and deleted within
  the meta-test itself or kept in a clearly-marked fixture directory
  excluded from normal builds by its own tag.
- A disposable, service-container PostgreSQL instance for every
  Integration-layer case, following this spec's own FR-2 convention
  (eating its own dog food).
- A temporary, disposable Git branch and PR for the FR-8 live-gating
  verification — never the real `main` branch, cleaned up after the
  check.
- A representative fixture-factory instance per owning domain spec —
  `Work` (`domain-bibliographic.md`), `LibraryEntry` (`domain-library.md`),
  `SourceOffering` (`domain-source.md`), `ReadingProgress`
  (`domain-reading.md`) — for the FR-4 canary, not all eleven, which is
  `backend-persistence.md`'s own exhaustive coverage.

No real credentials, no real user data, no copyrighted content.

## What is deliberately not tested

- Every individual repository method's own correctness against real
  Postgres — `backend-persistence.md`'s own test plan owns this
  exhaustively; this plan's FR-4 canary uses a representative subset
  only, to prove the *factory* mechanism works, not to re-prove every
  aggregate's persistence correctness.
- The specific coverage tool and minimum threshold (FR-8 stage 7) — the
  spec's own Open questions leave this unfixed; this plan verifies the
  stage exists, not a number that doesn't exist yet.
- CI pipeline duration budget — the spec's own Open questions defer this
  to a future measurement; nothing to test against yet.
- The bundled-spawn suite's actual CI trigger/cadence (every PR vs.
  nightly) — the spec's own Open questions leave this undecided; this
  plan verifies the suite is *separated*, not which cadence it eventually
  runs on.
- `internal/testutil`'s exact final package name/location — the spec's
  own Open questions flag this as a naming placeholder; this plan's tests
  are written against the behavior the package provides, adaptable to
  whatever the final import path turns out to be.
- The spec's own AC 1 asks this be "proven on a machine with no Postgres
  available at all" — this plan substitutes the FR-1 structural
  compile/vet check (above) as the practical equivalent, reasoning that
  a file whose contents are never parsed without the tag can't attempt a
  connection regardless of what machine it runs on; the literal "no
  Postgres available" run is a one-time environment check worth doing
  once when the harness is first implemented, not a repeatable `go test`
  case, so it isn't part of this plan's own re-run suite.

## Exit criteria

- [ ] Every functional requirement (FR-1 through FR-9) maps to at least
      one test or verification step above
- [ ] Every adversarial case above has a test or an explicit, honest
      exclusion note
- [ ] Tests were observed to fail before the implementation existed —
      including the FR-8 live-gating check, observed to correctly block
      merge on the deliberately broken test commit
- [ ] The suite is deterministic across repeated runs — no real sleeps
      (`FakeClock` throughout), no real filesystem or network access in
      Unit-layer cases, truncate-based teardown between every
      Integration-layer case
- [ ] The FR-3 parallelism structural check fails if run against a
      fixture `_integration_test.go` file that does call `t.Parallel()`,
      proving it can detect the exact hazard it exists to catch
- [ ] Every FR maps to a line in phase 03's own exit criteria (the
      spec's own AC 9) — a documentation cross-check against
      `.claude/roadmap/03-backend-foundation/README.md`, distinct from
      the FR-to-test mapping above; satisfied by this plan's own FR
      coverage together with a manual read of that README, not a
      separate runtime test
