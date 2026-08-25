# T25 — remaining persistence Integration/E2E — task list

Full plan: [`tasks/plan-t25-persistence-e2e.md`](plan-t25-persistence-e2e.md).

## Decisions

- [ ] T25-D1 — Windows orphan-prevention: raw kernel32.dll syscalls, no new dependency; unverified in this environment
- [ ] T25-D2 — port selection: throwaway-listener probe, TOCTOU accepted
- [ ] T25-D3 — retry-safe spawn via Postgres's own postmaster.pid lock, not hand-rolled state
- [ ] T25-D4 — binary location via PATH lookup (exec.LookPath), explicit named failure if missing

## Tasks

**Tier 0 — spawn mechanism, portable + Linux**

- [x] E1 — `internal/persistence/postgres/supervisor` package: data directory init (skip if `PG_VERSION` exists), binary location via `exec.LookPath`, port selection; extends `PostgresArgs` — `LocateBinaries`/`EnsureDataDir`/`SelectPort`, all with injected dependencies (`LookupFunc`/`CommandRunner`+`StatFunc`) except `SelectPort` itself (a real OS interaction, no faithful fake); `PostgresArgs` extended to `(dataDir string, port int)`, both call sites (tests only) updated
- [x] E2 — Linux spawn: `SpawnWithOrphanPrevention` in `spawn_linux.go`, `SysProcAttr.Pdeathsig` set to `SIGKILL`; real empirical proof via a re-exec'd helper process (flag-based dispatch, not env-var, to stay within the import-boundary check's own env-var restriction) — a real `sleep` grandchild is confirmed to die within 3s of its spawned parent being SIGKILLed, checked via `syscall.Kill(pid, 0)` liveness polling
- [x] E3 — Windows spawn: raw kernel32.dll Job Object syscalls (`CreateJobObjectW`/`SetInformationJobObject`/`AssignProcessToJobObject` via `syscall.NewLazyDLL`) in `spawn_windows.go`, using `os.Process.WithHandle` for a race-free process handle (Go 1.19+, no separate `OpenProcess` call needed); self-review caught a real gap — a job-object-setup failure after `cmd.Start()` succeeded was leaving the spawned process running unprotected — fixed by killing it before returning the error; a post-commit security review then found a real logic race (job assignment happening entirely after `Start()`, leaving the process unprotected for a window) — fixed by reordering job creation/configuration to before `Start()`, narrowing the window to one syscall; the window isn't fully closed (would need `CREATE_SUSPENDED`, unreachable through `os/exec`'s public API), documented explicitly as a follow-up rather than attempted blind; unit-level + `GOOS=windows` cross-compile/vet checks only, no live-run proof possible in this environment
- [x] E4 — connectivity-wait phase: `WaitForConnection` polls an injected `DialFunc` with a bounded `context.Context`, returns cleanly on success or a named timeout error wrapping `context.DeadlineExceeded`

**Checkpoint E-A** — supervisor package builds + unit suite green on Linux; `GOOS=windows go build`/`go vet` clean — done

**Tier 1 — wire spawnPostgres for real**

- [x] E5 — replaced `cmd/server/main.go`'s `spawnPostgres` stub with a real implementation: new `cmd/server/spawn.go` (`//go:build linux || windows`) composes E1-E4 (`LocateBinaries` → `EnsureDataDir` → `SelectPort` → `SpawnWithOrphanPrevention` → `WaitForConnection`) behind an injectable `supervisorDeps` bundle, with `spawnState` closed over by the returned closure so `waitForPostgres`'s own bounded retry within one `run()` invocation reuses the port and skips re-init/re-spawn (T25-D3, refined during implementation from the plan's original postmaster.pid-reading proposal — see plan's Open questions); `cmd/server/spawn_darwin.go` keeps macOS buildable with a scoped-down stub (D4, unchanged behavior from before this task); self-review found and fixed a real bug — `WaitForConnection` was inheriting `run()`'s own undeadlined lifetime context, so a Postgres that started but never became ready would block a single retry attempt forever; fixed with `connectPostgresWithTimeout`'s own established per-attempt-timeout pattern, now injectable via `supervisorDeps.attemptTimeout` and proven with a dedicated bounded-timeout test; production-spawn-failure proven both at `spawnPostgresOnce` directly and through the full `run()` chain (`postgresMaxAttempts: 1`, command runner invoked exactly once, named step in the log)

**Checkpoint E-B** — production spawn failure proven (fakes only) — done

**Tier 2 — migration observability (small, mostly test-only)**

- [x] E6 — two new integration tests in `run_integration_test.go`: `TestIntegration_MigrationSuccessLogsInfoNamingTheStep` and `TestIntegration_MigrationFailureLogsErrorDistinguishableFromPostgresStep`, both against real Postgres, both using a `SpyHandler`-backed logger (`spyLogger`) swapped in for `quietLogger`; both passed on the first run against unmodified production code — confirming the hypothesis from planning that `cmd/server/run.go`'s existing `"step", "migrate"` logging already satisfies FR-6 Observability — **no production code changed for this task**, only the missing test coverage

**Checkpoint E-C** — migration observability proven against real Postgres — done

**Tier 3 — the E2E suite**

- [x] E7 — `//go:build spawn` suite, `cmd/server/spawn_e2e_test.go`: `TestSpawn_FreshDataDirectoryReachesReady` (full FR-1/FR-5/FR-6 path, driven through the real `run()` sequence with `newObtainPostgres()`, `DatabaseURL` empty so the spawn branch actually runs) and `TestSpawn_ExistingDataDirectoryReachesReady` (data directory pre-initialized via a direct `EnsureDataDir` call, then the same cold-start proving initdb is genuinely skipped through a real `run()` cycle, not just E5's in-memory unit proof); both gated by `requirePostgresBinaries(t)`, skipping with a named reason when `postgres`/`initdb` aren't on PATH — true in this session's own sandbox (Supabase's local stack runs Postgres in Docker, not exposed as a host binary), confirmed skipping cleanly rather than failing or silently passing
- [x] E8 — same file/suite, no platform restriction beyond the `spawn` tag: the orchestration (`newObtainPostgres` → `spawnPostgresOnce`) is already platform-agnostic, so one test file covers Linux and Windows identically once binaries are present — written and reviewed here, cross-compiled and vetted (`GOOS=windows go vet -tags=spawn`), **cannot be run or verified in this environment** — first real execution on Windows CI (T27's job to wire in)
- [x] E9 — `TestSpawn_CorruptedDataDirectoryFailsCleanly`: `pg_control` truncated after a real `initdb` (a realistic corruption simulation, distinct from removing `PG_VERSION` — that would just look like "uninitialized," an already-covered case) — asserts non-zero exit and that neither `PG_VERSION` nor the corrupted `pg_control` itself were touched by any recovery attempt (FR-7's prohibition); also gated by `requirePostgresBinaries(t)`, confirmed skipping cleanly here

**Checkpoint E-D (final)** — full suite green: `go build`/`go vet` (incl. `GOOS=windows`/`GOOS=darwin` build+vet, and `GOOS=windows go vet -tags=spawn`), `-race` unit, `-race` integration (`-p 1`), `spawn`-tagged suite compiles and skips cleanly here (no real postgres/initdb binaries in this sandbox; genuine execution deferred to wherever CI or a dev machine has them) — done. T25 complete; T26/T27 still open, report back into `tasks/todo.md`.
