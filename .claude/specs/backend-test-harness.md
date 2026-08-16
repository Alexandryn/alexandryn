# Spec: Backend test harness

| | |
|---|---|
| **Status** | `APPROVED` (amended post-approval — container-target test coverage added, [`0045`](../reviews/0045-spec-backend-test-harness-container-topology.md), needs maintainer re-confirmation) |
| **Phase** | `03-backend-foundation` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-16 |
| **Supersedes** | — |
| **Reviewed in** | [`0022`](../reviews/0022-phase03-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, fixed; approved by maintainer 2026-08-14. Amended post-approval, [`0045`](../reviews/0045-spec-backend-test-harness-container-topology.md) — FR-10 added for ADR 0015's container target, self-reviewed, needs maintainer re-confirmation |

## Context

`architecture-testing.md` fixed the system-wide testing layers, CI
platform (GitHub Actions), and the FR-3 distinction between routine
integration tests (a service-container Postgres) and the dedicated
bundled-spawn suite (the application's own
`architecture-persistence.md` mechanism). Phase 03's own README names
determinism, fixtures, and "a clock that tests control" as this phase's
specific responsibility. Every other phase 03 spec's Test strategy table
points here (or to `architecture-testing.md` directly) for the actual
mechanics rather than inventing its own. This spec is where those
mechanics get written down concretely enough for the first real test file
to be written against them.

## Problem

Nothing has fixed: how a Go integration test obtains a real PostgreSQL
connection in CI versus locally, what a fixture/seed function looks like,
how the clock is injected so a test can control "now," or the actual
GitHub Actions workflow shape phase 03's README names as this phase's own
deliverable ("the first phase with code to build, lint, typecheck and
test").

## Goals

- Fix how integration tests reach PostgreSQL: build-tag separation from
  unit tests, environment-variable-driven connection, fail-loud (not
  skip) when the tag is used without the variable set
- Fix the fixture/seed pattern: programmatic, not checked-in snapshots
  (`architecture-testing.md` FR-7)
- Fix the clock abstraction: an interface with a real and a fake
  implementation, injected everywhere time matters
- Fix the first GitHub Actions workflow's shape: the ordered stages
  `architecture-testing.md` FR-1/FR-8 already require, made concrete as
  actual jobs/steps
- Fix how the dedicated bundled-spawn suite
  (`architecture-testing.md` FR-3) is kept separate from the fast,
  per-PR integration suite

## Non-goals

- The specific `golangci-lint` rule configuration —
  `architecture-testing.md`'s own Non-goals already deferred this to
  phase 03/04's tuning; this spec fixes that the lint step exists and
  runs, not its rule set
- Docker/dependency vulnerability scanning tool specifics —
  `architecture-testing.md` FR-4/FR-5 name the requirement; specific
  scanner choice stays deferred to phase 99/04 as those FRs already state
- Electron E2E harness mechanics (`@playwright/test`'s `_electron`,
  `xvfb`) — `architecture-testing.md` FR-2 already fixed the tool; the
  actual test files are phase 05's, once there's a desktop host to test
- Frontend test tooling — phase 04's own pick

## User stories

- As **any phase 03 spec's own Test strategy section**, I want a fixed
  harness to point to, instead of each spec inventing its own fixture or
  connection convention.
- As **a contributor running `go test ./...` locally**, I want the
  default command to run only fast, no-external-dependency unit tests, so
  I'm not blocked by "no Postgres running" for code I'm not touching.
- As **CI**, I want one workflow file that builds, lints, and tests in a
  fixed, dependency-correct order (ADR 0008's `web/`-before-`go build`
  requirement, `architecture-testing.md` FR-8), gating merge.

## Functional requirements

- **FR-1** Integration tests (anything requiring a real PostgreSQL) MUST
  be separated from unit tests by a Go build tag (`//go:build
  integration`), in files suffixed `_integration_test.go`. `go test
  ./...` with no tag MUST run only unit tests and MUST NOT require
  PostgreSQL, Docker, or any external service to be running — this is
  what keeps the default, fastest command usable by any contributor at
  any time.
- **FR-2** Integration tests MUST connect using a `TEST_DATABASE_URL`
  environment variable, read directly by the test harness (not through
  `internal/config` — this is test-only plumbing, distinct from the
  application's own runtime configuration, `backend-configuration.md`'s
  concern). Running `go test -tags=integration ./...` with
  `TEST_DATABASE_URL` unset MUST fail the test run with a clear message
  naming the missing variable — MUST NOT silently skip, per
  `architecture-testing.md` FR-6's "no silent green": a contributor who
  explicitly asked for the integration tag gets a clear failure, not a
  quietly-passed suite that ran nothing.
- **FR-3** Each integration test MUST run against a database schema at
  the current migration head (ADR 0013's `goose.Up`, invoked by the
  harness's own setup, not assumed pre-applied) and MUST leave the
  database in a state that does not affect other tests. The default
  mechanism is **truncating affected tables in a test-level teardown**,
  not wrapping the whole test in one outer transaction rolled back at the
  end: `backend-persistence.md` FR-4 requires each repository method to
  open and commit *its own* transaction internally against the shared
  `*pgxpool.Pool`, and a pool hands separate connections to separate
  `Begin()` calls — an outer, test-level transaction does not compose
  with repository code that manages its own inner transactions, since
  the two would not share a connection or a transaction context. An
  earlier draft of this spec offered outer-transaction rollback as the
  "fast, preferred" default without accounting for this; review 0022
  caught the gap. Truncate-based teardown works uniformly regardless of
  how many transactions a test's own repository calls open internally,
  at the cost of being slower than a rollback would have been. Tests
  MUST NOT depend on running in a specific order or on data left behind
  by another test — `architecture-testing.md` FR-6's determinism
  requirement, restated as this harness's concrete isolation mechanism.
- **FR-4** Fixtures are constructed by Go factory functions in a shared
  `internal/testutil` (or equivalent) package — e.g.
  `testutil.NewWork(t, overrides...)` returning a valid, minimally
  populated phase 02 domain value a test can further customize — never a
  checked-in SQL dump, JSON snapshot, or `.sql` seed file
  (`architecture-testing.md` FR-7). A factory function's defaults MUST
  themselves satisfy every phase 02 invariant, so a test that doesn't
  care about a particular field never has to supply one just to get past
  validation.
- **FR-5** All code that reads "now" (session/observation timestamps in
  phase 02's domain — e.g. `domain-source.md`'s `SourceOffering`
  timestamp, `domain-reading.md`'s reported-at time — and any backend
  code, such as correlation-adjacent timing) MUST do so through an
  injected `Clock` interface (`Now() time.Time`), never a direct
  `time.Now()` call inside logic under test. `internal/testutil` provides
  a `FakeClock` (settable, advanceable) for tests; production code
  receives a real-clock implementation constructed once at startup
  (`backend-service-lifecycle.md` FR-2's no-globals rule applies here
  too — the clock is a constructed dependency, not a package-level
  `time.Now` substitute).
- **FR-6** Two further sources of non-determinism named in phase 03's own
  "architecture decisions expected" list ("how the clock, filesystem and
  randomness are injected so tests stay deterministic") get the same
  treatment FR-5 gives the clock:
  - **Filesystem**: the two real filesystem touchpoints this phase
    introduces are `backend-configuration.md` FR-5's config-file read and
    `architecture-persistence.md` FR-1's data-directory
    existence-check/init. Both MUST go through a small injected interface
    (e.g. `FS` with just `Stat`, `MkdirAll`, `ReadFile` — the handful of
    operations actually used, not a general-purpose filesystem
    abstraction) rather than calling `os` package functions directly
    inside logic under test. `internal/testutil` provides an in-memory
    fake; production code receives a real, `os`-backed implementation
    constructed once at startup, same injection pattern as `Clock`
    (FR-5) — no new dependency needed, since the interface is small
    enough to hand-write against the standard library.
  - **Randomness**: the one current use is correlation ID generation
    (`backend-errors-and-logging.md` FR-7's "randomly generated, never
    derived from anything request-supplied"). MUST go through an injected
    `IDGenerator` (or equivalent) interface, never a direct call to a
    UUID library inside the logging middleware. `internal/testutil`
    provides a deterministic fake (a fixed sequence or a seeded
    generator) for tests that need to assert on a specific correlation
    ID; production code receives a real cryptographically-random
    implementation constructed once at startup, same pattern again.
- **FR-7** The dedicated bundled-spawn suite
  (`architecture-testing.md` FR-3) lives in a separate build tag
  (`//go:build spawn`, or an equivalent distinct tag from `integration`)
  and a separate CI job from the fast per-PR integration suite — it
  exercises `architecture-persistence.md` FR-1/FR-8/FR-9/FR-10 and
  `backend-persistence.md` FR-5/FR-6/FR-8 against the actual
  bundled-Postgres mechanism, not a service container, and is expected to
  be slower and platform-sensitive (Linux/Windows/macOS orphan-prevention
  differs). It MUST run in CI but MAY run on a different trigger/cadence
  than every PR if its runtime becomes a real burden — not decided here
  as a fixed rule, since no measurement exists yet (Open questions).
- **FR-8** The GitHub Actions workflow, as this phase's own deliverable,
  MUST implement stages in this order: (1) build `web/` (ADR 0008), (2)
  `go build ./cmd/server` and `./cmd/pg-supervisor`
  (`architecture-testing.md` FR-8), (3) `go vet` and lint
  (`architecture-backend.md` FR-3's import-boundary check included), (4)
  unit tests (FR-1, no tag), (5) integration tests (FR-1/FR-2, tagged,
  against a GitHub Actions Postgres service container per
  `architecture-testing.md` FR-3), (6) contract test
  (`architecture-contracts.md` FR-3), (7) test coverage reporting
  (`.claude/roadmap/03-backend-foundation/README.md`'s own named CI
  scope, omitted from an earlier draft of this spec — review 0022 caught
  the gap; a specific coverage tool and threshold are not fixed here,
  only that the stage exists and its output is captured), (8) dependency
  vulnerability scan (`govulncheck`, `architecture-testing.md` FR-5), (9)
  the container-target test (FR-10, added 2026-08-16, ADR 0015) — builds
  the `Dockerfile` image and runs the `docker compose` startup assertion.
  Every stage MUST block merge on failure (`architecture-testing.md`
  FR-1) except where a later FR in this spec names an explicit exception
  (FR-7's spawn suite, potentially a different trigger).
- **FR-9** The race detector (`go test -race`) MUST be enabled for both
  the unit and integration test runs in CI — phase 03's own exit
  criterion ("tests pass with the race detector enabled"), made a
  concrete CI flag rather than left as an aspiration.
- **FR-10** (Added 2026-08-16, ADR 0015) A dedicated container-target test
  MUST exist alongside FR-7's bundled-spawn suite, exercising the
  container-hosted target's own startup path rather than the
  Electron-hosted target's: build the `Dockerfile` image, run
  `docker compose --profile bundled-db up` against the repository-root
  `docker-compose.yml`, and assert the `backend` container reaches
  `Ready` (`/readyz` returns 200) against the sibling `postgres`
  container — proving the `DATABASE_URL`-present branch
  (`backend-persistence.md` FR-5) actually connects, migrates
  (`architecture-persistence.md` FR-5), and serves, not just that the
  code compiles. Unlike FR-7's bundled-spawn suite, this test has no
  platform-specific spawn/orphan-prevention mechanism to exercise — it is
  expected to be fast and platform-independent (Docker itself, not the
  application, owns process supervision), so it MUST run in the same
  per-PR CI job as the routine integration suite rather than needing
  FR-7's separate, possibly-different-cadence job, unless measurement
  after implementation shows otherwise (same placeholder-pending-
  measurement pattern FR-7's own cadence question already uses).

## Non-functional requirements

- **Performance** — no CI pipeline duration budget exists yet
  (`architecture-testing.md`'s own Open questions already name this);
  this spec doesn't invent one, consistent with that spec's placeholder
  pattern.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable.
- **Reliability** — FR-3's test isolation and FR-1's tag separation are
  what make CI failures trustworthy — a red check always means a real
  regression, never cross-test pollution or an environment gap.
- **Observability** — not applicable beyond what CI itself reports
  (`architecture-testing.md`'s own scope for this).

## Domain model

Not applicable — this spec is test tooling, not the Alexandryn domain. It
does fix that fixture factories (FR-4) produce phase-02-valid values,
which is the concrete mechanism keeping test data from drifting out of
sync with domain invariants as those evolve.

## API and contracts

- **Test harness ↔ PostgreSQL**: `TEST_DATABASE_URL` (FR-2), a plain
  connection string to whatever Postgres the runner (CI service
  container, or a contributor's own local instance — e.g. the Supabase
  dev stack ADR 0004's addendum already establishes) provides.
- **Test harness ↔ fixtures**: Go functions (FR-4), not files.
- **Test harness ↔ time**: the `Clock` interface (FR-5), implemented by
  `internal/testutil.FakeClock` in tests and a real implementation in
  production.
- **Test harness ↔ filesystem and randomness**: the `FS` and
  `IDGenerator` interfaces (FR-6), same fake/real split as `Clock`.
- **CI workflow ↔ GitHub Actions**: `services:` block for the Postgres
  container (`architecture-testing.md` FR-3), ordered `jobs`/`steps` per
  FR-8.

## State transitions

Not applicable.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `go test -tags=integration` run locally with `TEST_DATABASE_URL` unset | FR-2's explicit check at test-suite setup | A clear, named failure ("TEST_DATABASE_URL not set"), not a silent skip | Test run fails immediately, before attempting any connection |
| A test leaves data behind that affects a later test | FR-3's isolation requirement violated | A flaky-looking failure in an unrelated test | This is the failure FR-3 exists to prevent; if it happens, it's a defect in that test's teardown, not a harness gap to route around |
| `web/` build step skipped or fails before `go build` | FR-8's ordering | CI red at the build stage, before any test runs | Workflow fails fast — no test stage runs against a stale/missing embedded frontend (ADR 0008's named risk) |
| Bundled-spawn suite (FR-7) takes long enough to slow every PR | Measured once it exists | A slower merge queue | Per FR-7, may move to a different trigger — not pre-decided, flagged as a real possibility |
| Container image builds but the `backend` container never reaches `Ready` against the sibling `postgres` container (FR-10) | The compose-startup assertion's own timeout | CI red at the container-target test stage | This is exactly the failure FR-10 exists to catch — a config or connection defect the unit/integration suites, which never build an image, cannot see |

## Security considerations

- **`TEST_DATABASE_URL` is a test-only, loopback/CI-local database** —
  never production data; still worth stating explicitly that this
  variable MUST NOT ever point at a real user's data directory, which the
  naming convention (distinct from `DATABASE_URL`) is meant to make hard
  to confuse.
- **`govulncheck` and the dependency scan (FR-8 stage 8) are this spec's
  concrete implementation of `architecture-testing.md` FR-5** — the
  actual blocking mechanism for constitution §9's "dependencies are
  liabilities," now wired into CI rather than a reviewer's memory.
- **CI secrets** — `architecture-testing.md`'s own Security
  considerations already flagged this as real-but-undesigned until a
  real secret exists; this spec introduces no new CI secret (the test
  Postgres service container needs no credential beyond what GitHub
  Actions' own `services:` block manages), so nothing new to design here.

## Test strategy

This spec doesn't have a separate "what tests this" table the way feature
specs do — it *is* the test-strategy mechanics document every other phase
03 spec's own Test strategy section points to. Its own verification is
the same walkthrough method used throughout phase 01/03: write one real
unit test and one real integration test against this harness's actual
conventions (FR-1 through FR-6), confirm both run correctly locally and in
a CI dry run, before phase 03's other specs are considered implementable
against it.

## Acceptance criteria

- [ ] `go test ./...` (no tag) passes with zero external dependencies
      running, proven on a machine with no Postgres available at all
- [ ] `go test -tags=integration ./...` without `TEST_DATABASE_URL` fails
      clearly, proven — not skips
- [ ] A fixture factory (FR-4) produces a phase-02-valid value with zero
      required overrides, proven by a test using only defaults
- [ ] A test using `FakeClock` (FR-5) controls and advances "now"
      deterministically, proven against a scenario internal to this
      phase — e.g. `backend-service-lifecycle.md` FR-5's shutdown grace
      period timeout firing at exactly the configured duration, without
      the test actually sleeping in real time. (An earlier draft of this
      criterion cited `domain-reading.md`'s furthest-wins conflict
      resolution, which compares `Percentage` magnitude, not time —
      review 0022 caught that it wasn't actually a time-dependent
      example; `domain-reading.md`'s own genuinely time-dependent case,
      the equal-`Percentage` tiebreaker, is explicitly unresolved in that
      spec's Open questions, so it isn't a safe example to cite either.)
- [ ] A test using the fake `FS` (FR-6) proves `backend-configuration.md`
      FR-5's config-file-absent path without touching the real filesystem
- [ ] A test using the fake `IDGenerator` (FR-6) proves a specific,
      predictable correlation ID reaches both the log line and the error
      response for a given request
- [ ] The GitHub Actions workflow (FR-8) exists and blocks merge on any
      stage failure, proven with a deliberately broken commit on a test
      branch
- [ ] `-race` is enabled on unit and integration CI runs (FR-9), proven
      by the workflow file itself, not just claimed
- [ ] The container-target test (FR-10) builds the image, brings the
      compose stack up, and asserts `Ready`, proven by a CI run — and
      proven to actually fail if the `backend`/`postgres` service
      definitions are deliberately broken, not just proven to pass once
- [ ] Every FR maps to a line in phase 03's own exit criteria

## Open questions

- **Bundled-spawn suite's CI trigger/cadence (FR-7)** — "every PR" versus
  "nightly/on-merge-to-main" is not decided; needs a real runtime
  measurement once the suite exists, same placeholder-pending-measurement
  pattern this whole phase has used.
- **Container-target test's exact tooling (FR-10)** — whether the compose-
  startup assertion is a shell script CI step, a Go test driving `docker
  compose` via `os/exec`, or a dedicated tool (e.g. `testcontainers-go`'s
  compose support) is not fixed here; FR-10 only requires that the
  assertion exists and blocks merge. Owner: the new deployment spec this
  ADR's amendment plan names (`.claude/audits/0002-topology-gap.md`
  A-02-11), since it also owns the `Dockerfile`/`docker-compose.yml`
  content this test runs against.
- **`internal/testutil`'s exact package location/name** — a naming
  placeholder in this spec (FR-4, FR-5, FR-6); `architecture-backend.md`'s
  `internal/` layout doesn't currently name it, so this spec's
  implementation should confirm it fits that layout (likely
  `internal/testutil`, imported only by `_test.go` files, never by
  production code — worth an explicit lint/review check that it isn't).
- **CI pipeline duration budget** — `architecture-testing.md`'s own Open
  questions already flagged this as phase 03's to set once measurable;
  restated here since this spec is where the actual workflow file that
  would be measured gets built.
- **Coverage tool and threshold (FR-8 stage 7)** — the stage's existence
  is required, a specific tool (Go's own `-cover`, or a third-party
  aggregator) and a minimum percentage are not fixed here, same
  shape-before-numbers pattern this phase uses throughout.

## References

- `architecture-testing.md` — FR-1 through FR-8, the system-wide testing
  policy this spec implements the backend-specific mechanics of
- ADR 0008 — the `web/`-before-`go build` ordering FR-8 encodes
- `architecture-contracts.md` FR-3 — the contract test FR-8 stage 6 runs
- `architecture-backend.md` FR-3 — import-boundary lint, FR-8 stage 3
- `.claude/roadmap/03-backend-foundation/README.md` — "coverage
  reporting," the CI scope item FR-8 stage 7 satisfies, and "how the
  clock, filesystem and randomness are injected," which FR-5/FR-6
  together satisfy
- `backend-persistence.md` FR-1 through FR-8 — the mechanics this
  harness's integration and spawn suites (FR-2, FR-7) exercise
- `backend-service-lifecycle.md` FR-2 — the no-globals rule FR-5/FR-6's
  injected-dependency pattern follows; FR-5 — the shutdown grace period
  cited as FR-5's (this spec's) revised `FakeClock` acceptance-criteria
  example
- `backend-configuration.md` FR-5 — the config-file read cited as FR-6's
  filesystem-injection example
- `backend-errors-and-logging.md` FR-7 — correlation ID generation, cited
  as FR-6's randomness-injection example
- Constitution §9 (dependencies), phase 03's own risk table and exit
  criteria (race detector, determinism)
- ADR 0015 — the container-hosted target FR-10 adds coverage for, and the
  new deployment spec its amendment plan names, which owns the actual
  `Dockerfile`/`docker-compose.yml` this test runs against
