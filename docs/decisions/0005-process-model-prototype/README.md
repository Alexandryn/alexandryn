# Prototype: Electron-spawns-Go-child process model

Evidence for [ADR 0005](../0005-process-model.md). Throwaway spike code, not
shipped — no `go.mod`/`package.json` on purpose, run directly with `go run`
equivalents and plain `node`. Requires Go and Node on PATH.

`node-parent/parent.js` stands in for Electron's main process: it uses the
same `child_process.spawn` API Electron's main process runs on, so this
validates the OS-level spawn/kill/orphan mechanics without needing a display
to run a real Electron window.

Two Go child variants:

- `go-child-baseline/` — no orphan-prevention mechanism
- `go-child-pdeathsig/` — Linux `prctl(PR_SET_PDEATHSIG)` fix candidate,
  stdlib only (`syscall.Syscall` + `SYS_PRCTL`), no dependency added

## Running it

```
cd go-child-baseline && go build -o child-baseline main.go && cd ..
cd go-child-pdeathsig && go build -o child-pdeathsig main.go && cd ..
./run.sh
```

`run-output.txt` is the captured output from the run this ADR cites
(2026-08-13, Linux, local dev machine). Four scenarios, each empirically
checked rather than reasoned about:

1. Baseline: spawn, HTTP health check, graceful shutdown — confirms the
   basic spawn/transport/shutdown mechanics work at all
2. Pdeathsig variant: same, confirms the fix doesn't break the normal path
3. Baseline: `SIGKILL` the parent (simulates Electron crashing, no cleanup
   code runs) — child survives as an orphan, still serving HTTP. This is the
   FR-10 problem, demonstrated rather than assumed.
4. Pdeathsig variant: same `SIGKILL`, child exits on its own within the
   check interval — the fix works, on Linux.

## What this does not prove

macOS and Windows were not available to test here. The README and ADR 0005
name the analogous mechanisms for each (kqueue `EVFILT_PROC`/`NOTE_EXIT` for
macOS, a Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` for Windows) as
well-known patterns, not as verified claims. Phase 05 needs to actually build
and test those before FR-10 is closed cross-platform.
