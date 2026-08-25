# T25 — remaining persistence Integration/E2E

## Overview

`tasks/plan.md`'s own T25 line: "Remaining persistence Integration/E2E:
production spawn failure, migration observability logging, `//go:build
spawn` Linux+Windows E2E. macOS E2E not run here (D4)." T24 built and wired
all 11 repositories against real Postgres; T25 is what's left of
`backend-persistence.md`/`architecture-persistence.md`'s own test strategy
that T24 didn't touch — all of it downstream of `cmd/server/main.go`'s
`spawnPostgres`, which today is a hardcoded stub:

```go
func spawnPostgres(ctx context.Context) error {
	return errors.New("spawning a managed PostgreSQL instance is not implemented yet (backend-persistence.md FR-8, tracked as T25)")
}
```

Three pieces, unequal in size:

- **Migration observability logging** — smallest. `cmd/server/run.go`
  already logs `"step", "migrate"` distinctly on both success (`info`) and
  failure (`error`) — checked directly against the running code this
  session. What's actually missing is the Integration-layer *test* proving
  this against a real Postgres migration failure (not the unit-level fakes
  `run_test.go` already has), not new production code.
- **Production spawn failure** — small-to-medium. Needs `spawnPostgres` to
  exist for real, with an injectable command-runner the test can fail
  deliberately (same DI pattern as every other `runDeps` field), and the
  failure-path behavior (fail cleanly, name the step, never retry inside
  `spawnPostgres` itself, never touch the data directory).
- **`//go:build spawn` Linux+Windows E2E** — the large piece. This is where
  `spawnPostgres` actually has to work: locate real `postgres`/`initdb`
  binaries, initialize a data directory, pick a port, spawn the process with
  platform-specific orphan-prevention (`Pdeathsig` on Linux, a Job Object on
  Windows), wait for it to accept connections, and be safe to retry.

## What's actually needed beyond the one-line task

Reading `architecture-persistence.md` FR-1/FR-2/FR-5/FR-7/FR-8/FR-9 and
`backend-persistence.md` FR-5/FR-6/FR-7/FR-8 directly (not just the
one-paragraph task description) surfaces four real decisions no spec has
made yet — this plan's own job, the same way T24's plan expanded once R1's
interface gaps were checked directly instead of assumed away.

## Decisions (resolve before/alongside the tasks that need them)

- **T25-D1 — Windows orphan-prevention (FR-9), raw syscalls, no new
  dependency.** No stdlib API creates a Windows Job Object. The two
  options are `golang.org/x/sys/windows` (already a transitive dependency
  at v0.47.0 per `go.sum`, but promoting it to direct still needs its own
  justification — R0's `internal/idgen` decision already declined the same
  argument for `github.com/google/uuid` on comparably small surface, and
  this project's own precedent — `.claude/decisions/0005-process-model-prototype/go-child-pdeathsig/main.go`'s
  comment block — explicitly prefers "one direct syscall, no dependency to
  justify") or raw `kernel32.dll` calls via `syscall.NewLazyDLL` +
  `NewProc("CreateJobObjectW")` / `("SetInformationJobObject")` /
  `("AssignProcessToJobObject")`. **Chosen: raw syscalls**, matching the
  existing prototype's own stated approach and constitution §9's bar. This
  code cannot be empirically verified in this Linux authoring
  environment — stated plainly per constitution §12, not claimed working
  until a Windows CI run actually exercises it (same caveat class
  `architecture-persistence.md` FR-10 already carries for the macOS
  supervisor).
- **T25-D2 — Port selection via throwaway-listener probe, TOCTOU accepted.**
  Postgres has no "OS-assigned port" mode analogous to `net.Listen`'s
  `:0`. The standard technique — bind a real TCP listener on
  `127.0.0.1:0`, read the OS-assigned port from it, close it, pass that
  number to `postgres -p` — has a small window between close and Postgres's
  own bind where another process could steal the port. **Chosen: accept
  this as a recorded tradeoff**, not solved further — it's the same
  technique `embedded-postgres`-style tools use elsewhere (ADR 0007's own
  "established pattern... not verified" framing already named this
  category of tool), and closing the TOCTOU window fully would need
  Postgres itself to support ephemeral-port binding, which it doesn't.
- **T25-D3 — Retry-safe spawn via Postgres's own data-directory lock, not
  hand-rolled state.** `run.go`'s existing `waitForPostgres` calls
  `obtainPostgres` (and therefore `spawnPostgres`) repeatedly on failure,
  bounded by `postgresMaxAttempts`. A naive `spawnPostgres` that
  unconditionally runs `initdb` and starts a fresh `postgres` process on
  every call would, on a second retry, either corrupt a data directory
  `initdb` is already writing to, or try to start a second `postgres`
  against a directory a still-starting first attempt already locked.
  **Chosen: lean on Postgres's own `postmaster.pid` lock file**, which
  `postgres` itself refuses to start over (a well-documented, existing
  Postgres behavior, not something this plan invents) — `spawnPostgres`
  always attempts init (a no-op if `PG_VERSION` already exists in the data
  directory) and always attempts to start `postgres`; when that start
  fails specifically because another `postgres` already holds the
  directory's lock, `spawnPostgres` treats that as "already
  running" and proceeds straight to the connectivity-wait phase rather
  than surfacing it as a hard failure. Distinguishing a lock-conflict exit
  from a genuine corruption exit (both fail fast, per Postgres's own
  behavior) needs real empirical checking against a real `postgres`
  binary — flagged in Risks below as exactly the kind of detail the
  implementation task must nail down against reality, not assume from
  documentation alone.
- **T25-D4 — Binary location via `PATH` lookup, explicit and named.**
  ADR 0007 explicitly defers *bundling* real `postgres`/`initdb` binaries
  to phase 99 ("packaging must now bundle postgres binaries per platform
  ... not investigated here"). T25 cannot solve that; it can only assume
  the binaries are reachable in whatever environment runs this code today
  (a developer machine, CI). **Chosen: `exec.LookPath("postgres")` /
  `exec.LookPath("initdb")`**, the same assumption
  `.claude/test-plans/backend-persistence.md`'s own "runnable in this
  authoring environment's CI" line already makes for the E2E suite. A
  binary not found on `PATH` MUST produce a clear, named failure
  ("postgres binary not found on PATH" style, `backend-service-lifecycle.md`
  FR-3's own "specific, named error" bar) — never a bare `exec: "postgres":
  executable file not found in $PATH` surfacing as the only detail.

## Task list

**Tier 0 — spawn mechanism, portable + Linux**

- **E1.** `internal/persistence/postgres/supervisor` package (naming per
  `backend-persistence.md` FR-5's own "naming placeholder," kept as
  written unless a better name surfaces during implementation): data
  directory init (`initdb`, skipped if `PG_VERSION` exists — FR-1),
  binary location via `exec.LookPath` (T25-D4), port selection (T25-D2).
  Pure/testable pieces first — RED: given a data directory path and a
  fake `exec.LookPath`/command runner, the constructed `initdb`/`postgres`
  argument lists are asserted correct (extends `PostgresArgs`, which today
  only has `-D dataDir`); a data directory that already has `PG_VERSION`
  skips the init call entirely.
- **E2.** Linux spawn: `os/exec` with `SysProcAttr.Pdeathsig` (FR-8) —
  platform-gated file (`supervisor_linux.go` or equivalent build-tag
  split, since `syscall.SysProcAttr`'s fields are OS-specific and this
  won't compile cross-platform otherwise). RED: a real spawned child
  process (a stand-in binary, not real Postgres, for this specific test)
  observably dies when its parent is killed without a graceful signal —
  the same empirical proof style the existing Pdeathsig prototype used,
  applied to the *child*-configuring direction this time.
- **E3.** Windows spawn (T25-D1): raw `kernel32.dll` Job Object syscalls in
  a `supervisor_windows.go` build-tag file. RED: unit-level tests of the
  argument/flag construction that don't require a live Windows process
  (compiles under `GOOS=windows` cross-compilation, checked via `go
  build`/`go vet` with `GOOS=windows go vet ./...`, since this environment
  can't run Windows code) — the actual kill-on-parent-death behavior is
  unverified here per T25-D1, proven for real only once E5's Windows E2E
  job runs in CI.
- **E4.** Connectivity-wait phase: after a successful (or already-running,
  T25-D3) start, poll-connect to the chosen port with a bounded timeout,
  returning once Postgres accepts connections or the timeout expires.
  RED: given a fake listener that starts accepting connections after a
  configurable delay, the wait phase returns success once it does, and
  times out cleanly (a specific, named error) if it never does.

  > **Checkpoint E-A** — the supervisor package builds and passes its own
  > unit suite on Linux; `GOOS=windows go build`/`go vet` succeed for the
  > Windows half without running it.

**Tier 1 — wire spawnPostgres for real**

- **E5.** Replace `cmd/server/main.go`'s `spawnPostgres` stub with a real
  implementation calling E1-E4's supervisor package, with an injectable
  command-runner (matching every other `runDeps` field's DI pattern) so a
  test can fail it deliberately. RED: `TestRun_ProductionSpawnFailure` (or
  equivalent name) — the spawn step's command runner is replaced with a
  fake returning a non-zero exit/error unconditionally; startup fails
  cleanly, the failing step is named in the log, and the fake command
  runner is asserted invoked exactly once for a `postgresMaxAttempts: 1`
  configuration (isolating the assertion the way `backend-service-lifecycle.md`
  FR-3's own bounded-retry tests already do elsewhere in `run_test.go`) —
  and no automatic data-directory modification is attempted
  (`architecture-persistence.md` FR-7's prohibition).

  > **Checkpoint E-B** — production spawn failure proven at the unit
  > level (fakes only, no real Postgres needed for this specific proof,
  > matching the test plan's own Integration-layer placement using a fake
  > command runner rather than a real corrupted data directory).

**Tier 2 — migration observability (small, mostly test-only)**

- **E6.** Integration test proving `cmd/server/run.go`'s existing
  migration-step logging (`"step", "migrate"`, `info` on success, `error`
  on failure) against a **real** Postgres and a **real** migration
  failure (extending `run_integration_test.go`'s existing
  `TestIntegration_PartialMigrationStopsBeforePool` fixture, or a sibling
  test), asserting: the failure line is distinguishable from an earlier,
  separate connection-refused line from the *connect* step (FR-5) by a
  field/message fragment naming "migration" specifically — captured via a
  `testutil.SpyHandler`-backed logger (swapped in for `quietLogger`,
  matching `run_test.go`'s own `recordingDeps` pattern) rather than
  `quietLogger`'s discard-everything default. If this test passes without
  any production code change, that is itself the expected, correct
  outcome — say so plainly rather than manufacturing a change to justify
  the task.

  > **Checkpoint E-C** — migration observability proven against real
  > Postgres.

**Tier 3 — the E2E suite**

- **E7.** `//go:build spawn` suite (`backend-test-harness.md` FR-7's own
  tag), Linux: the full FR-1/FR-5/FR-6 production path for real — fresh
  data directory, `spawnPostgres` initializes it, spawns real `postgres`,
  waits for readiness, runs real migrations, reaches `Ready` — driven
  through the full `cmd/server` `run()` sequence the same way
  `run_integration_test.go`'s cold-start test already does for the
  *connect* path, but with `DatabaseURL` empty so the *spawn* path
  actually runs. A second case: existing (already-initialized) data
  directory reaches `Ready` faster (no init step) — the spec's own named
  "second walkthrough" (`architecture-persistence.md`'s Test strategy).
  Requires real `postgres`/`initdb` binaries on `PATH` in whatever
  environment runs this — if absent, the test MUST skip with a clear,
  named reason (`t.Skip("postgres binary not found on PATH...")`), never
  silently pass or produce a cryptic failure.
- **E8.** Same suite, Windows: written and reviewed here, but **cannot be
  run or empirically verified in this Linux authoring environment** — flagged
  explicitly per constitution §12, the same honesty standard
  `architecture-persistence.md` FR-10 already applies to its own
  macOS-unverified caveat. First real verification happens whenever this
  runs on a Windows CI runner (T27's own job to wire in, not built here).
- **E9.** Hostile walkthrough: a data directory deliberately corrupted
  between runs (`architecture-persistence.md`'s own named case) — the
  system MUST reach `Failed`, MUST NOT attempt automatic
  deletion/reinitialization (FR-7). Also under `//go:build spawn`, Linux
  only (Windows corruption handling is the same code path as E8, not
  re-verified separately unless E8 surfaces a platform-specific gap).

  > **Checkpoint E-D (final)** — full suite green: `go build`/`go vet`
  > (including `GOOS=windows go build ./...`), `-race` unit, `-race`
  > integration (`-p 1`, unchanged from T24), and the `spawn`-tagged suite
  > green on Linux in this environment (Windows deferred to CI, per E8).
  > Report back into `tasks/todo.md`: T25 done, T26/T27 still open.

## Critical files

- `.claude/specs/architecture-persistence.md` — FR-1/FR-2/FR-5/FR-7/FR-8/FR-9/FR-10,
  this plan's primary source
- `.claude/specs/backend-persistence.md` — FR-5 (spawn/connect branch,
  names `internal/persistence/postgres/supervisor` directly), FR-6/FR-7
  (migration + observability + restart detection), FR-8 (macOS, out of
  scope here per D4)
- `.claude/specs/backend-test-harness.md` FR-7 — the `//go:build spawn`
  tag and separate-CI-job convention this plan's E7-E9 build against
- `.claude/test-plans/backend-persistence.md` — "Production spawn
  failure," "FR-6 Observability," and "End to end" sections, the RED-step
  source for E5/E6/E7-E9
- `internal/persistence/postgres/startup.go` — `SelectStartupPath`,
  `PostgresArgs` (T11), extended by E1, not replaced
- `cmd/server/main.go` — `spawnPostgres`'s current stub, `obtainPostgres`,
  `connectPostgres` (unchanged, already real) — E5 replaces the stub only
- `cmd/server/run.go` — `waitForPostgres`'s existing bounded-retry loop
  (T25-D3's own interaction target), the existing `"step", "migrate"`
  logging E6 proves rather than rewrites
- `.claude/decisions/0005-process-model-prototype/go-child-pdeathsig/main.go` —
  the empirically-verified Linux Pdeathsig precedent (self-protection
  direction), and its own comment block's Windows Job Object discussion
  (T25-D1's precedent)
- `.claude/decisions/0007-postgres-provisioning.md` — binary
  bundling/acquisition explicitly deferred to phase 99, the boundary this
  plan does not cross (T25-D4)
- `cmd/pg-supervisor/main.go` — empty stub, macOS-only (FR-10/FR-8 of
  `backend-persistence.md`), **not touched by this plan** (D4)

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Windows Job Object syscalls (E3) are entirely unverified in this environment | High if wrong — silent failure to prevent orphaning on Windows | Written against the well-documented Win32 API shape, cross-compiled and vetted (`GOOS=windows go vet`), but explicitly flagged as unverified (T25-D1) until a real Windows CI run exercises it — not claimed working before then |
| Windows orphan-prevention (E3) has a real, known residual race — job assignment happens after `cmd.Start()`, so a crash in that exact window still orphans PostgreSQL | Low-medium — narrow window, but the failure mode is the one FR-9 exists to prevent entirely | Job creation/configuration reordered to before `Start()`, narrowing the window to one syscall instead of three (found and fixed during E3 via a post-commit security review); a fully race-free fix needs `CREATE_SUSPENDED`, unreachable through `os/exec`'s public API — deferred to a future pass with real Windows CI feedback available, not attempted blind in an environment that can't verify it |
| Distinguishing a lock-conflict `postgres` exit from a genuine corruption exit (T25-D3) needs real empirical checking against a real binary, not just documentation | Medium — a wrong classification could either loop forever "recovering" from a real corruption, or treat a real corruption as "already running" and skip straight to a connectivity wait that will time out uninformatively | E1/E5's own RED steps include this distinction as an explicit test case against a real `postgres` binary in this environment (Linux), not assumed from Postgres's docs alone |
| `//go:build spawn` E7 needs real `postgres`/`initdb` on `PATH` in whatever environment runs it | Medium — a CI runner without those binaries would otherwise fail cryptically | E7's own test skips with a named, clear reason when the binaries aren't found (T25-D4), rather than failing opaquely |
| Port-selection TOCTOU (T25-D2) | Low — a narrow race window, same category of risk every `embedded-postgres`-style tool already accepts | Named and accepted explicitly, not silently assumed safe |

## Open questions

- **Windows Job Object behavior is unverified in this environment** — same
  honesty standard as `architecture-persistence.md` FR-10's own
  macOS-unverified caveat; first real verification happens on Windows CI,
  not here (T25-D1).
- **Windows orphan-prevention's residual race (`Start()` → job assignment)
  is only narrowed, not closed** — a real fix needs `CREATE_SUSPENDED` via
  a hand-rolled `syscall.CreateProcess` call, bypassing `os/exec.Cmd`
  entirely (its own internals close the new process's thread handle
  immediately and never expose it, so a suspended start can't be resumed
  through the public API). Not attempted in this environment — that much
  new, low-level, untestable-here Windows code is a worse risk trade than
  the current narrowed window. Revisit once real Windows CI feedback
  exists to develop and verify it against.
- **Whether `postmaster.pid`-based idempotency (T25-D3) is sufficient**
  once a real production restart scenario (not just `waitForPostgres`'s
  bounded retry) is considered — e.g. Alexandryn itself being force-quit
  and restarted while an old `postgres` process is still shutting down.
  This plan's scope is `waitForPostgres`'s own retry loop specifically;
  a broader "stale orphan detected on next startup" story is
  `architecture-persistence.md`'s own named Failure mode ("PostgreSQL
  orphaned... next startup attempt's port-bind step fails clearly") —
  already covered by that spec at the port-bind level, not reopened here.
- **`postgres`/`initdb` binary versions** — this plan assumes whatever
  version is on `PATH` in the dev/CI environment matches what production
  will eventually bundle (phase 99's problem). No version pinning or
  compatibility check is built here; flagged as a real gap phase 99
  inherits, not silently assumed fine.
