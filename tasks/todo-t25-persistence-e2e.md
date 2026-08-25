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

- [ ] E5 — replace `cmd/server/main.go`'s `spawnPostgres` stub with a real implementation, injectable command-runner; production-spawn-failure test (fake command runner, non-zero exit, clean failure, named step, no data-directory modification, invoked exactly once at `postgresMaxAttempts: 1`)

**Checkpoint E-B** — production spawn failure proven (fakes only)

**Tier 2 — migration observability (small, mostly test-only)**

- [ ] E6 — integration test: real Postgres, real migration success (info) and real migration failure (error), captured via `SpyHandler`-backed logger, failure line distinguishable from an earlier connection-refused line by a "migration" field/fragment; if no production code needs to change, say so plainly

**Checkpoint E-C** — migration observability proven against real Postgres

**Tier 3 — the E2E suite**

- [ ] E7 — `//go:build spawn` suite, Linux: full FR-1/FR-5/FR-6 path for real (fresh data dir + already-initialized data dir cases); skip with a named reason if `postgres`/`initdb` aren't on PATH
- [ ] E8 — same suite, Windows: written and reviewed here, **cannot be run/verified in this environment** — first real verification on Windows CI (T27's job to wire in)
- [ ] E9 — hostile walkthrough: corrupted data directory reaches `Failed`, never auto-modifies the data directory (Linux only)

**Checkpoint E-D (final)** — full suite green: `go build`/`go vet` (incl. `GOOS=windows go build ./...`), `-race` unit, `-race` integration (`-p 1`), `spawn`-tagged suite green on Linux here (Windows deferred to CI). Report back into `tasks/todo.md`: T25 done, T26/T27 still open.
