# Phase 03 (Backend foundation) — implementation plan

## Context

Alexandryn is a self-hosted digital library. Phase 03 is the Go service
skeleton: config, structured logging/errors, HTTP transport, PostgreSQL
persistence, service lifecycle, and the test harness. All six phase-03 specs
are `APPROVED`. Nothing is implemented yet — no `go.mod`, no code at all.
Phase 03 is the only phase in this repo with test plans already written
(`.claude/test-plans/`, one per spec) — this plan uses those as the RED-step
basis throughout; it does not invent new test scenarios except where
explicitly flagged below.

This project runs strict TDD (RED → GREEN → Refactor, per FR, test before
code) and requires: `internal/domain` never importing transport/persistence,
no package-level globals for logger/pool/config, a closed six-category error
taxonomy, dual-interface (`slog.LogValuer` + `json.Marshaler`) redaction for
anything sensitive, and Clock/FS/IDGenerator injected everywhere instead of
`time.Now()`/`os.*`/direct randomness — all fixed by the six specs and
restated in `.claude/skills/purpose/go-backend-conventions/SKILL.md`.

**Key finding that reshapes the ordering**: phase 02 (Domain) is still
**In progress**, not `APPROVED` (`.claude/roadmap/02-domain/README.md`,
confirmed directly) — its exit criteria are unchecked, including "the model
compiles as pure Go." `backend-persistence.md` FR-2 requires 11 repository
implementations satisfying `internal/domain` interfaces that don't exist as
compilable Go yet. Everything else in phase 03 is buildable without them.
This plan sequences the 11-repository work (T24) and everything depending on
it (T25, T26) to the end, gated on a checkpoint that re-confirms phase 02's
status rather than silently stubbing it.

## Approach

Vertically sliced, dependency-ordered tasks (not "all tests then all code").
Each task cites the exact existing test-plan case(s) it turns into its RED
step; three tasks need a small number of new cases the test plans don't
cover yet (flagged explicitly — mostly from `backend-configuration.md`
FR-8's 2026-08-18 amendment, written after its test plan). Six decisions get
resolved once, explicitly, up front rather than silently mid-implementation.

### Decisions (resolve before/alongside the tasks that need them)

- **D0 — import-boundary lint mechanism** (undecided by any spec): start
  with an interim grep/file-walk script
  (`scripts/check-import-boundaries.sh`) checking (a) no
  `internal/domain/*.go` imports `internal/transport`/`internal/persistence`,
  (b) no `os.Getenv`/`os.LookupEnv`/TOML-decode import outside
  `internal/config`, (c) no package-level `var` holding a
  logger/pool/config. Revisit as `golangci-lint`/`go/analysis` once CI
  exists.
- **D1 — phase 02 gating** (the key finding above): T24's 11 repositories
  and everything after are gated on Checkpoint G, which re-checks phase 02's
  actual status rather than assuming it.
- **D2 — `web/dist` placeholder**: build FR-8's serving/fallback logic (T16)
  against a committed placeholder `embed.FS`
  (`internal/transport/http/webdist/placeholder/index.html`), tested via
  `fstest.MapFS` so tests don't depend on phase 04's timing. Swap the
  `//go:embed` target when phase 04 lands.
- **D3 — CI "contract test" stage**: ship as an explicit, named no-op step
  now (so the stage-presence test has something real to find), filled in
  once `architecture-contracts.md`'s contract test exists.
- **D4 — macOS `pg-supervisor` E2E**: build and test the portable
  spawn-argument construction now (T11); the full kqueue-based E2E (T25)
  needs a real macOS CI runner this environment doesn't have — tracked as a
  follow-up, not a phase-03 blocker.
- **D5 — spec-text cleanup**: `backend-configuration.md` still has stale
  loopback-only phrasing in three places (Failure-modes table ~line 275,
  Security considerations ~lines 309-314, Test-strategy table ~line 328)
  that predate the 2026-08-18 FR-8 amendment — a doc fix alongside T6, not
  a functional ambiguity (FR-8's own prose is authoritative and current).
- **D6 — branch-protection live-gating verification**: infra/GitHub-settings
  work, not Go/TDD — sequenced last (Checkpoint H).
- **D7 — `deployment-container-packaging.md` approval status**: it's
  `REVIEWED`, not `APPROVED` — CI stage 9 (T27) and `backend-test-harness.md`
  FR-10 depend on its FR-5/FR-6. Get maintainer approval before T27 relies
  on it.

### Task list

**Tier 0 — bootstrap and shared foundations**

- **T0.** Module bootstrap + D0's lint script. `go.mod`, empty
  `cmd/server/main.go`/`cmd/pg-supervisor/main.go`,
  `scripts/check-import-boundaries.sh`.
- **T1.** `internal/testutil`: `Clock`/`FakeClock`, `FS`/`FakeFS`,
  `IDGenerator`/`FakeIDGenerator`, a spy `slog.Handler`
  (`backend-test-harness.md` FR-5/FR-6 canaries).
- **T2.** Build-tag/`TEST_DATABASE_URL` convention (`backend-test-harness.md`
  FR-1/FR-2): structural proof both directions, fail-loud-not-skip via
  sentinel-file subprocess test.

  > **Checkpoint A** — T1+T2 green, `go vet ./...` clean, before any other
  > package's tests are written (matches the harness spec's own stated
  > precondition).

- **T3.** `internal/config` core (FR-1/2/3/4/6, all 9 non-`BIND_ADDRESS`
  keys): precedence, category discipline, type/enum validation, error
  content, adversarial cases — all from the existing test plan.
- **T4.** `internal/config` FR-5: config-file resolution, injected FS (no
  real filesystem access outside `t.TempDir()`).
- **T5.** `internal/config` FR-7: `DATABASE_URL` dual-interface redaction,
  the two-fixture leak-then-clean sequence.
- **T6.** `internal/config` FR-8: `BIND_ADDRESS` three-case classification
  (current ADR 0017 rule). **Two NEW test cases not in the written test
  plan** (it predates the 2026-08-18 amendment): a public IP *with* a valid
  cert accepted; cert invalid/expired/missing while public still rejected.
  Do D5's cleanup here.

  > **Checkpoint B** — `internal/config` fully green before `internal/logging`.

- **T7.** `internal/logging` + `internal/domain` bootstrap
  (`backend-errors-and-logging.md` FR-1/2/4/6/8/9, transport-independent
  half): `domain.Error{Category}` — **the only `internal/domain` file phase
  03 writes**, the 11 aggregates remain phase 02's — plus `slog.Logger`
  construction and the redaction/level-policy pattern.
- **T8.** `internal/persistence/postgres` FR-1: pool construction
  (`MinConns=2`, configurable `MaxConns`).
- **T9.** `internal/persistence/postgres` FR-3: parameterized-query static
  check, established before any repository file exists.

**Tier 1 — persistence plumbing that doesn't need domain types**

- **T10.** FR-6 migration runner (`goose.Up` via a short-lived
  `pgx/v5/stdlib` `*sql.DB`, closed immediately; DSN-redaction regression
  guard distinguishing connection failures from SQL failures).
- **T11.** FR-5 branch selection (`DATABASE_URL` present/absent) + FR-8
  spawn-argument construction, portable linux/windows subset (D4).

  > **Checkpoint C** — restate D1: FR-2's 11 repositories are blocked on
  > phase 02. Skip that slice, continue to transport and lifecycle, which
  > don't need real repositories.

- **T12.** FR-6/FR-7 Integration cases that don't need domain rows:
  migration-to-empty-database, partial-migration-restart detection.

**Tier 2 — HTTP transport**

- **T13.** `internal/transport/http` FR-1/FR-3/FR-4 (Unit): middleware
  composition order, panic recovery + fallback correlation ID, logging
  middleware skeleton.
- **T14.** FR-2/FR-6/FR-7: body-size limits (exact 10 MiB boundary),
  Content-Type, unmatched-route JSON 404, config-to-field wiring. The
  category→status mapping table lives here, exactly once.
- **T15.** FR-5: `/healthz` (never touches DB) and `/readyz` (3-state,
  DSN-redaction regression guard).

  > **Checkpoint E** — transport's Unit layer green before
  > Integration/Concurrency. Smoke-test all six `domain.Error` categories
  > through `WriteError` end to end once.

- **T16.** FR-8: embedded `web/dist` fallback (D2). **Three NEW cases not
  in the written test plan**: correct `Content-Type` by extension, SPA
  fallback to `index.html`, and routing precedence (`/api/v1/unknown` still
  hits FR-7's JSON 404, never the SPA fallback).

**Tier 3 — service lifecycle (the integration point, highest risk)**

- **T17.** `cmd/server` FR-1 steps 1-4 + FR-2 (no-globals) + FR-7: strict
  step-order proof via fake constructors, atomic pool reference wired to
  T15's `Readyz`.
- **T18.** FR-1 steps 5-6 + FR-3 (bounded retry, failure-and-exit): every
  step's failure table, retry budget via `FakeClock`. Repository
  construction stubbed empty here per D1, with an explicit `// TODO(D1)`
  comment — not a silent omission.
- **T19.** FR-4/5/6: graceful shutdown, `FakeClock`-driven grace-period
  deadline, `pool.Close()` strictly after `Shutdown` returns.

  > **Checkpoint F (highest risk in the whole plan)** — T17-T19's Unit-layer
  > fakes fully green and reviewed before T20's real-listener,
  > `-race`-gated Integration/Concurrency layers. This is the roadmap's own
  > named hardest case; worth an extra review pass specifically here.

- **T20.** Integration (cold start, partial-migration fixture,
  alive-before-ready window, real invalid config) + Concurrency
  (shutdown-under-load with ≥5 concurrent requests and a fixed grace-period
  boundary, double-SIGTERM) — all under `go test -race`. Flag explicitly:
  timing-based tests are the flakiest in this plan; run repeatedly before
  trusting them in CI gating.

**Tier 4 — remaining cross-cutting FRs, harness completion, domain-gated tail**

- **T21.** `backend-errors-and-logging.md` FR-3/5/7/10/11 against the now-real
  chain: Postgres error translation as a pure function, correlation-ID
  uniqueness with the *real* generator, FR-10's gap case against the full
  chain, FR-11's interim message-pattern check, concurrent panic isolation.
- **T22.** `backend-test-harness.md` FR-3 Variant A (isolation), schema-at-head,
  and the parallelism-hazard static check (`t.Parallel()` never appears in
  an `_integration_test.go` — truncate-teardown has no per-test isolation).
  Variant B (composability hazard) deferred to T26 — needs a real
  self-transacting repository.

  > **Checkpoint G — the domain-readiness gate.** Before T24, re-check phase
  > 02's actual status directly (don't assume). If the 11 aggregate types +
  > repository interfaces compile, proceed. If not, this is a stop-and-ask:
  > pause T24-T26 until phase 02 ships (recommended — matches the roadmap's
  > own dependency arrow), or explicitly author a provisional scaffold,
  > labeled throwaway. Don't pick silently.

- **T24.** `backend-persistence.md` FR-2: all 11 repository implementations
  (Work, Edition, Author, LibraryEntry, Collection, Source, SourceOffering,
  ReadingProgress, Bookmark, Highlight, ReadingPreferences) — compile-time
  interface satisfaction, per-aggregate CRUD, the hostile-string SQL-injection
  proof via `pgx.QueryTracer` literal-SQL capture, cross-connection
  transaction atomicity, migration-to-populated-database (finally
  testable), pool exhaustion (with/without context deadline), concurrent
  unique-constraint race.
- **T25.** Remaining persistence Integration/E2E: production spawn failure,
  migration observability logging, `//go:build spawn` Linux+Windows E2E.
  macOS E2E not run here (D4).
- **T26.** Harness FR-3 Variant B (the exact composability gap review
  `0022` found once already) + swap `cmd/server/run.go`'s T18 repository
  stub for real construction.
- **T27.** CI workflow (`backend-test-harness.md` FR-8/9/10): stage
  presence/order, `-race` static check, three-way tag separation, coverage
  presence. Stages 1 (`web/` build) and 6 (contract test) as explicit named
  no-ops (D2/D3). Stage 9 (container-target test) gated on D7's approval.

  > **Checkpoint H (final)** — D6's live branch-protection verification,
  > D5's cleanup PR, a full-suite run (`-race` unit, `-race` integration,
  > `-tags=spawn` on Linux/Windows, `go vet`, D0's lint script) checked
  > against the roadmap's own unchecked exit-criteria boxes one by one.

## Critical files

- `.claude/roadmap/03-backend-foundation/README.md` — phase scope/exit criteria
- `.claude/specs/backend-{service-lifecycle,configuration,errors-and-logging,http-transport,persistence,test-harness}.md` — all six, `APPROVED`
- `.claude/test-plans/` — same six filenames, the RED-step source for every task above
- `.claude/skills/purpose/go-backend-conventions/SKILL.md` — package layout, naming, the lint-boundary/no-globals/redaction rules
- `.claude/decisions/{0011-http-router-and-middleware,0012-postgres-driver,0013-migration-tool,0017-network-exposure-condition-gated}.md` — tooling/condition decisions this plan builds against directly
- `.claude/roadmap/02-domain/README.md` — the phase-02 gate T24 depends on; re-check at Checkpoint G, don't trust this snapshot
- `.claude/constitution.md` — binding throughout, especially §2 (tests before code), §4 (hostile input), §8 (no credential/secret leakage)

## Verification

- Per task: `go test ./...` (unit) and, where the task has one, `go test -race -tags=integration ./...` against a disposable Postgres via `TEST_DATABASE_URL` — both must be green before moving to the next task, per this repo's own RED→GREEN discipline.
- At each lettered checkpoint (A/B/C/E/F/G/H above): stop, don't proceed past it silently — most map to a real risk (harness self-test, service-lifecycle concurrency, phase-02 gating) or a real decision point.
- End of phase: `go build ./...`, `go vet ./...`, D0's lint script, `go test -race ./...`, `go test -race -tags=integration ./...`, `go test -tags=spawn ./...` (Linux/Windows), the CI workflow's own stage-presence/order tests, then walk `roadmap/03-backend-foundation/README.md`'s exit-criteria checklist item by item against what's actually built — including fixing the one line there ("the service refuses to bind to a non-loopback address") that still describes the pre-ADR-0017 rule and should be reworded to match the current three-case FR-8, not silently left stale.
