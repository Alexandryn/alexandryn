# Spec: Desktop host process model

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-31, phase 05 Tier 7 / E30 — implemented Tiers 1–6 (PR #68), audited [`0005`](../audits/0005-phase05-desktop-host.md), all acceptance criteria proven with 97 Vitest tests, 14 Playwright E2E tests, Linux re-exec parentwatch test, D7 Windows job object seam documented) — was `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `05-desktop-host` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-31 |

| **Supersedes** | — |
| **Reviewed in** | [`0032`](../reviews/0032-spec-amendments-phase05-cross-phase-findings.md) (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`architecture-desktop-host.md` (phase 01) already fixed the pattern for
almost everything this spec covers: the config/secrets file channel
(FR-5), the readiness-gated window (FR-6/FR-7, 15-second placeholder
timeout), macOS `kqueue` and Windows Job Object orphan-prevention
mechanisms (FR-8/FR-9), single-instance locking (FR-4), and close-means-
quit (FR-11). ADR 0005 verified the Linux `pdeathsig` mechanism against a
real prototype and left macOS/Windows named-but-unverified. What phase 01
explicitly left to this spec: where the Go binary actually lives in a dev
checkout versus a packaged build, and what happens if the Go server
crashes *after* the window is already showing the real UI — a case
distinct from FR-7's first-start timeout, which `architecture-desktop-
host.md` itself says it doesn't cover.

## Problem

Nothing has fixed: how the main process locates the Go binary to spawn in
`npm run dev` versus a packaged `.app`/`.exe`, the measured (not
placeholder) readiness timeout, or a mid-session crash restart policy —
phase 05's own roadmap risk table already names the failure mode
("crash-and-relaunch loop") this spec has to prevent.

## Goals

- Fix binary location resolution: dev vs. packaged, one function, not
  scattered path-guessing
- Replace `architecture-desktop-host.md` FR-7's 15-second placeholder
  with either a measured number or an explicit "still a placeholder,
  here's why" note, consistent with this project's own placeholder
  discipline
- Fix a mid-session crash restart policy: bounded retry with backoff,
  never a silent infinite loop, matching phase 05's own named risk
  mitigation
- Fix where each platform's orphan-prevention code actually lives (some
  of it is Go-binary-side, some is Electron-main-side — not obvious from
  `architecture-desktop-host.md` alone)

## Non-goals

- The preload IPC surface — `desktop-host-ipc-surface.md`
- Window creation, loading-asset rendering, external-link handling —
  `desktop-host-window-and-serving.md`
- The Go server's own internal startup sequence — `backend-service-
  lifecycle.md`; this spec owns the Electron-side half of spawning it,
  not what happens inside the child process once spawned
- Packaging/installer mechanics that produce the packaged build this
  spec's binary-resolution logic reads from — phase 99

## User stories

- As **a contributor running `npm run dev`**, I want the Go server
  located and spawned automatically from a freshly-built binary, without
  a manual path edit.
- As **a packaged-app user**, I want the bundled Go server to start the
  same way every time, regardless of install location.
- As **a user whose Go server crashes mid-session** (a bug, a resource
  exhaustion event), I want the app to try recovering a bounded number of
  times and then tell me clearly it's stuck, never a spinner that quietly
  retries forever or a silent freeze.

## Functional requirements

- **FR-1** Binary location is resolved by one function,
  `resolveServerBinaryPath()`: in development (`process.env.NODE_ENV !==
  'production'`, or equivalently Electron's own `app.isPackaged` check),
  resolves to `<repo-root>/bin/alexandryn-server` — ADR 0008 fixed the
  Go module at the repository root with no `server/` subdirectory
  (`cmd/server`, `cmd/pg-supervisor` directly under root), so the build
  output this spec reads from follows that same layout, produced by a
  documented `go build -o bin/alexandryn-server ./cmd/server` step that
  must run before `npm run dev` — a build-order dependency this spec
  names explicitly, not left implicit (corrected from an earlier draft
  that assumed a `server/` subdirectory ADR 0008 doesn't have — a
  cross-phase review caught the mismatch); in a packaged build, resolves
  to `path.join(process.resourcesPath, 'server',
  platformBinaryName())`, where `platformBinaryName()` appends `.exe` on
  Windows and nothing elsewhere — the packaged layout is Electron's own
  resources convention, independent of ADR 0008's source-tree layout, so
  it doesn't need to match it. One function, one call site (the spawn
  call in FR-2), never a path re-derived ad hoc elsewhere in the
  codebase.
- **FR-2** The main process spawns the Go server via `child_process.spawn`
  (never `exec`, which shells out and reintroduces injection risk this
  project has no reason to accept — the binary path and arguments are
  passed as an argument array, never concatenated into a shell string),
  passing exactly two arguments: `['--config', configPath]` — matching
  `backend-configuration.md` FR-5's actual flag name exactly (`--config
  <path>`, not a bare positional argument; an earlier draft of this spec
  passed the path alone, which doesn't match what the Go server's own
  config loader actually reads — a cross-phase review caught this,
  independently, in both review passes). `stdout`/`stderr` are piped and
  logged by the main process (prefixed to distinguish Electron's own
  logs from the child's), not inherited directly — inheriting would let
  the child's `PORT=<n>` announcement line (`architecture-desktop-
  host.md`'s API and contracts section) interleave unpredictably with
  Electron's own output.
- **FR-3** Readiness is polled via `GET http://127.0.0.1:<port>/healthz`
  (`backend-http-transport.md` FR-5 — the correct endpoint name;
  `architecture-desktop-host.md` FR-5 itself says `/health`, a naming
  inconsistency this spec follows the later, `APPROVED` backend spec's
  actual name for rather than propagating — `/healthz`, not `/readyz`: the
  window only needs "the process is alive and serving," not "PostgreSQL
  is confirmed" — `architecture-system.md`'s `Degraded` state (Go server
  alive, PostgreSQL unreachable, `/readyz` 503) is a condition the
  already-loaded real UI surfaces through its own API-error handling
  once it's making real requests (`frontend-shell-and-routing.md` FR-2's
  TanStack Query error states, `architecture-frontend.md` FR-6's
  `atStates` treatment) — not this spec's or `desktop-host-window-and-
  serving.md`'s concern at the Electron-chrome layer at all, and
  deliberately not conflated with this spec's own `Recovering` state
  (FR-4), which means the entire Go process has exited, not merely lost
  its database), at a fixed interval
  (250ms — frequent enough that the loading-to-ready transition feels
  immediate once the server is actually up, cheap enough that polling
  itself is never the bottleneck), until either a 200 response arrives
  or `architecture-desktop-host.md` FR-7's timeout expires. The timeout
  value itself remains **15 seconds**, the placeholder that spec named —
  this spec does not have a measured number to replace it with either
  (no packaged build has been profiled yet); kept as an explicit,
  named placeholder rather than silently inventing a different one,
  flagged again in Open questions for phase 05's own implementation to
  measure and replace.
- **FR-4** Mid-session crash policy: if the spawned process exits with a
  non-zero code (or any code) at a point *after* FR-3's readiness check
  already succeeded once this session — distinguishing "crashed after
  working" from "never started," which get different treatment — the
  main process attempts an automatic respawn (FR-2, same binary
  resolution, re-capturing the freshly-spawned process's own `PORT=<n>`
  stdout announcement — `architecture-desktop-host.md`'s API and
  contracts section — since `BIND_ADDRESS`'s compiled default is an
  OS-assigned ephemeral port per `backend-configuration.md` FR-4, a
  respawned process is not guaranteed the same port as the one that
  crashed) up to **3 times**, with exponential backoff between attempts
  (1s, 4s, 9s), each attempt going through the same FR-3 readiness gate
  against whatever port that attempt announced. The window enters a
  **`Recovering`** state during each retry (named distinctly from
  `architecture-system.md`'s own `Degraded` state — that state means
  "the Go server is alive but can't reach PostgreSQL," a running HTTP
  server answering `/healthz` 200 and `/readyz` 503; this spec's
  `Recovering` means the entire Go process has exited and no HTTP server
  exists to answer anything — conflating the two under one label was an
  earlier draft's mistake, caught by cross-phase review), rendered per
  `desktop-host-window-and-serving.md` FR-6 — not the loading state
  (`architecture-desktop-host.md` FR-6) — the user was already looking
  at a working app, so the UI communicates "recovering," not "starting
  up." If all 3 attempts fail, the main process transitions to the
  `Failed` state (`atStates` error treatment, same as FR-7 of
  `architecture-desktop-host.md`) and stops retrying automatically — a
  manual "Retry" action (same handler, reset attempt counter) is the
  only path back, never an unbounded background loop. This is the
  concrete mechanism behind phase 05's own named risk ("Crash-and-
  relaunch loop... backoff and a visible failure state, not a silent
  retry loop").
- **FR-5** Shutdown: on `app.before-quit` (covers window-close-quits per
  `architecture-desktop-host.md` FR-11, and explicit quit via menu/OS
  signal alike), the main process sends `SIGTERM` to the spawned Go
  process and waits up to the shutdown grace period before sending
  `SIGKILL` if the process hasn't exited. The grace period value is
  **10 seconds** — `backend-service-lifecycle.md` FR-5's own compiled
  default (`backend-configuration.md`'s `SHUTDOWN_GRACE_PERIOD` key) —
  used directly by Electron rather than "read from the same config,"
  which is impossible in the general case: `architecture-desktop-host.md`
  FR-5 requires the config file be deleted once the readiness check
  succeeds, long before shutdown normally runs, so there is no file left
  to read from by then (an earlier draft's claim, corrected here — a
  cross-phase review caught it). If a future config ever overrides this
  key to a non-default value, Electron already knows the value at the
  moment it authors the config file (FR-2) and retains it in memory for
  this purpose, rather than re-reading a file that may no longer exist.
  `app.before-quit`'s default quit is deferred (`event.preventDefault()`)
  until this shutdown sequence completes, so Electron never exits with
  the child still running. On macOS specifically, Electron's own default
  behavior (`window-all-closed` does *not* quit the app) is overridden:
  this spec wires `window-all-closed` to `app.quit()` unconditionally on
  every platform, since `architecture-desktop-host.md` FR-11 requires
  close-means-quit with no platform exception, and leaving macOS's
  framework default in place would silently violate that requirement on
  one platform only.
- **FR-6** Orphan-prevention code location, named explicitly since it
  isn't obvious from `architecture-desktop-host.md` alone: **Linux**
  (`prctl(PR_SET_PDEATHSIG)`, ADR 0005) and **macOS** (`kqueue`
  `EVFILT_PROC`/`NOTE_EXIT` monitoring the Electron parent's PID,
  `architecture-desktop-host.md` FR-8) are both implemented **inside the
  Go server binary itself** (`cmd/server`), not in Electron main — the
  child monitors its own parent and self-exits, so this spec's own
  responsibility for those two platforms is limited to spawning the
  child correctly (FR-2) and nothing more. **Windows**
  (`architecture-desktop-host.md` FR-9's Job Object with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`) is implemented **in Electron
  main**, at spawn time (FR-2), via a **native Node addon** — Electron
  itself exposes no built-in Job Object API (an earlier draft presented
  this as an equally-available option; it isn't, and this FR now matches
  the honest uncertainty this spec's own Open questions already carried)
  — the one platform where this spec's own process carries the orphan-
  prevention responsibility directly. Exact addon package choice: Open
  questions, same unverified status ADR 0005 and `architecture-desktop-
  host.md` FR-9 already carry for this platform generally.
- **FR-7** Single-instance lock: `app.requestSingleInstanceLock()` is
  called **before** any window is created and **before** FR-2's spawn
  call — `architecture-desktop-host.md` FR-4's own reasoning made
  concrete: acquiring the lock first is what prevents a second process
  from ever spawning a second Go server against the same PostgreSQL
  data directory, the exact data-integrity risk that spec's Security
  considerations names. If the lock isn't acquired, this process quits
  immediately (`app.quit()`, no window created, no spawn attempted) after
  calling `app.requestSingleInstanceLock()`'s own paired mechanism to
  ask the existing instance to focus its window (the `second-instance`
  event handler registered on the primary instance, per Electron's own
  API for this pattern) — an earlier draft of this spec's own Context
  section asserted this was "already fixed" by the phase-01 spec and
  needed no further treatment here, which review caught as leaving the
  actual implementation home unnamed, the same gap FR-6 avoided for
  orphan-prevention but this had not.

## Non-functional requirements

- **Performance** — FR-3's 250ms poll interval and `architecture-desktop-
  host.md` FR-6's 200ms loading-state-appears budget are this spec's own
  concrete numbers; `architecture-desktop-host.md` FR-7's 15-second
  readiness timeout remains an unmeasured placeholder (FR-3).
- **Security** — see Security considerations below.
- **Accessibility** — not directly applicable to process management
  itself; the *states* this spec's FR-4 drives (`Recovering`, `Failed`)
  render through `desktop-host-window-and-serving.md`'s components,
  which own the actual accessibility contract.
- **Reliability** — FR-4 is this spec's own central reliability property:
  bounded, visible recovery, never a silent infinite loop or a hung
  process.
- **Observability** — every state transition this spec drives (spawn
  attempted, ready, crashed, respawn attempt N of 3, failed) is logged
  by the main process (`architecture-desktop-host.md`'s own Observability
  section: "window created, server spawned, server became healthy,
  shutdown initiated" — this spec's FR-4 crash/retry sequence extends
  that same log, not a separate log stream).

## Domain model

Not applicable — process lifecycle, not the Alexandryn domain.

## API and contracts

- **Main process ↔ Go server (spawn)**: `child_process.spawn`,
  `['--config', configPath]` (FR-2), matching `backend-configuration.md`
  FR-5's actual flag name.
- **Main process ↔ existing instance (FR-7)**: Electron's own
  `requestSingleInstanceLock`/`second-instance` event pair — no custom
  IPC needed, this is the framework's own built-in mechanism.
- **Main process ↔ Go server (readiness)**: `GET /healthz` over loopback
  HTTP (FR-3), matching `backend-http-transport.md` FR-5's response
  shape.
- **Main process ↔ Go server (shutdown)**: `SIGTERM`, then `SIGKILL` on
  timeout (FR-5), matching `backend-service-lifecycle.md` FR-4's own
  expectation of receiving `SIGTERM` specifically.

## State transitions

Extends `architecture-desktop-host.md`'s own window-level states with
the mid-session case that spec explicitly doesn't cover:

```
Ready -> (spawned process exits unexpectedly) -> Recovering
      -> (FR-4: respawn attempt 1/2/3, each gated by FR-3's readiness check
          against that attempt's own announced port)
      -> success: -> Ready (window reloads if the port changed, FR-6 of
          desktop-host-window-and-serving.md)
      -> all 3 exhausted: -> Failed (manual retry only, FR-4)

(second instance launched) -> no new window; existing instance's window
                               focused (FR-7); new process exits immediately
```

Illegal transitions, restated for this layer:

- An automatic respawn attempt beyond the fixed budget of 3 (violates
  FR-4's bounded-retry requirement)
- `app.before-quit`'s default behavior completing before FR-5's shutdown
  sequence has finished (would exit Electron with the Go server still
  running)
- A window created, or FR-2's spawn attempted, before FR-7's single-
  instance lock has been acquired

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Go binary missing at the resolved path (FR-1) | `spawn` itself fails (ENOENT) | `Failed` state, naming that the server binary couldn't be found — distinct message from a readiness timeout | No retry attempted; this is a packaging/build defect, not a transient condition FR-4's retry logic is meant for |
| Go server crashes before ever becoming ready (first start) | FR-3's readiness check never succeeds within 15s | `Failed` state (`architecture-desktop-host.md` FR-7), retry offered | Same as that spec's own FR-7 behavior — this spec doesn't change the first-start case, only adds FR-4 for the post-ready case |
| Go server crashes after becoming ready (mid-session) | Process `exit` event fires post-readiness | `Recovering` overlay during retries, `Failed` state if all 3 attempts fail | FR-4's bounded respawn sequence |
| A successful respawn (FR-4) announces a different port than the crashed process had | FR-4's port re-capture | Brief reload of the real UI, not a broken/stale connection | `desktop-host-window-and-serving.md` FR-6 re-`loadURL`s against the new port once `Recovering` clears |
| Shutdown grace period expires with the Go server still running | FR-5's own timeout | N/A — app is quitting regardless | `SIGKILL` sent, Electron proceeds to quit; the Go server's own FR-4 (`backend-service-lifecycle.md`) governs what happens to in-flight requests up to that point |
| A second instance is launched while one is already running | `app.requestSingleInstanceLock()` returns `false` (FR-7) | Existing window is focused; no visible error, since nothing went wrong | New process quits immediately, spawns nothing — never a second Go server against the same PostgreSQL data |

## Security considerations

- **`spawn`, never `exec` (FR-2)** — argument-array invocation avoids
  shell interpolation entirely; there is no user- or renderer-controlled
  input anywhere in the spawn call (the binary path is resolved
  internally, FR-1; the one argument is a path this process itself
  created, `architecture-desktop-host.md` FR-5), so this is defense in
  depth rather than closing a live injection path, but it's the correct
  default regardless.
- **Bounded retry (FR-4) is itself a security-adjacent property** — an
  unbounded crash-loop is a denial-of-service against the user's own
  machine (CPU/battery spent respawning a process that will never
  succeed); phase 05's own risk table already names this explicitly.
- **Orphan prevention (FR-6) restates `architecture-desktop-host.md`'s
  own reasoning** — an orphaned Go server keeps a loopback port bound
  after the user believes the app is closed, a real (if narrow) exposure
  window for other local processes/users.
- **Single-instance lock (FR-7) is a data-integrity control, not purely a
  UX nicety** — two Go server processes against one PostgreSQL data
  directory, both assuming they're the only writer, is the exact risk
  `architecture-desktop-host.md`'s own Security considerations names;
  acquiring the lock *before* FR-2's spawn call is what makes this
  actually preventive rather than a check that runs too late to matter.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | `resolveServerBinaryPath()` (FR-1) under both dev and packaged conditions, mocked `app.isPackaged`; the backoff schedule (FR-4) as a pure function of attempt number — **this** is where the fake clock applies, isolated from any real process/network I/O, not the integration test below |
| Integration | Real spawn/health/shutdown against a real (test) Go binary, all timing real wall-clock, deliberately short (a test binary with a fast startup, a test-specific short grace period) rather than faked — composing a fake clock with real child-process exit events and real HTTP polling is a known-hard test-timing problem this spec doesn't attempt to solve; a deliberately crashing test binary exercises FR-4's full 3-attempt sequence at its real (short-for-testing) backoff values and confirms it stops, not loops forever; single-instance lock (FR-7) tested by launching a second real process against a lock already held by the first, confirming it exits immediately and spawns nothing |
| Security | A test asserting the spawn call is `child_process.spawn` with an argument array, never a string passed to `exec`/`shell: true` — a static/grep-based check, the same category of structural check this project's backend specs already use for their own no-globals rules |

The hardest thing to test here is the same one phase 05's own roadmap
document already names: shutdown when the Go server is mid-request —
inherited from `backend-service-lifecycle.md`'s own hardest case, with
this spec's own addition being what the *window* shows while the grace
period elapses (`desktop-host-window-and-serving.md`'s concern for the
actual UI, this spec's concern for the timing being correct underneath
it).

## Acceptance criteria

- [ ] `resolveServerBinaryPath()` returns the correct path in both dev
      and packaged conditions, proven by tests for both
- [ ] A crash after successful readiness triggers exactly 3 respawn
      attempts with the specified backoff, then stops — proven with a
      deliberately crashing test binary at real, test-scaled-down timing
      (Test strategy); the backoff *schedule itself* additionally proven
      as a pure function via a fake clock, isolated from process I/O
- [ ] A successful respawn that announces a different port triggers a
      window reload against the new port, proven with a test binary that
      deliberately picks a different port on its second start
- [ ] `SIGTERM` is sent on quit and the app waits for the configured
      grace period before `SIGKILL`, proven with a deliberately slow
      test binary
- [ ] `window-all-closed` triggers `app.quit()` on every platform,
      including a test proving the macOS-default-override is actually
      wired, not just documented
- [ ] A second launched instance exits immediately and focuses the
      existing window, without spawning a second Go server, proven with
      a real second-process test
- [ ] The spawn call is proven to never use a shell/`exec` path
- [ ] Every FR maps to an exit criterion in phase 05's own document

## Open questions

- **Windows Job Object native addon package** — which specific,
  small/audited addon to depend on is not fixed here (FR-6 now names the
  mechanism — a native addon — but not the package); needs a real
  Windows environment to verify regardless, same unverified status ADR
  0005 and `architecture-desktop-host.md` FR-9 already carry.
- **FR-3's 15-second readiness timeout** — still the placeholder
  `architecture-desktop-host.md` named; no measurement exists yet from a
  packaged build. Phase 05 implementation should measure a real cold
  start (including `backend-persistence.md`'s bundled-Postgres spawn
  path) before replacing this number.
- **FR-4's retry count and backoff schedule (3 attempts, 1s/4s/9s)** —
  reasoned placeholders (roughly enough total wall-clock time to survive
  a transient resource blip, short enough not to feel broken), not
  measured against a real failure distribution, since none exists yet.

## References

- `architecture-desktop-host.md` FR-4 through FR-9, FR-11 — the patterns
  this spec implements concretely
- ADR 0005 — process model, Linux `pdeathsig` verified, macOS/Windows
  named
- `backend-service-lifecycle.md` FR-4/FR-5 — the Go server's own
  shutdown contract this spec's FR-5 depends on
- `backend-http-transport.md` FR-5 — `/healthz` response shape FR-3
  polls
- `backend-configuration.md` FR-4, FR-5 — `SHUTDOWN_GRACE_PERIOD`'s
  compiled default FR-5 uses directly, and the `--config` flag name
  FR-2's spawn arguments match
- ADR 0008 — monorepo layout, the Go-module-at-root path FR-1's dev
  binary resolution follows
- `desktop-host-window-and-serving.md` FR-6 — the `Recovering` overlay
  and port-change reload this spec's FR-4 drives but doesn't itself
  render
- `.claude/roadmap/05-desktop-host/README.md` — the crash-loop risk
  FR-4 mitigates, the single-instance-lock scope item FR-7 closes
- Constitution §4 (hostile input — FR-2's argument-array discipline),
  §5 (privilege boundary)
