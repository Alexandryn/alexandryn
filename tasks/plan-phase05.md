# Phase 05 (Desktop host) — implementation plan

## Context

Alexandryn is an Electron desktop app that hosts a Go server, which serves
the React frontend to the Electron window and (from phase 13) to LAN
devices. Phase 03 (Go backend foundation) closed 2026-08-26. Phase 04
(frontend foundation, fully mocked) closed 2026-08-28. Phase 05 builds the
Electron half: the main process spawns / health-checks / cleanly stops the
phase-03 Go binary, exposes a minimal enumerated IPC surface, constructs a
sandboxed `BrowserWindow`, serves a disk-loaded loading/error asset before
the Go server is ready, intercepts external links, and proves the whole
lifecycle with `@playwright/test`'s `_electron` harness. It binds nothing
beyond loopback — that constraint becomes physically true here and every
later phase inherits it until phase 12.

Three specs, all `APPROVED`, governed by `architecture-desktop-host.md`
(phase 01, `APPROVED` 2026-08-20):

- `desktop-host-process-model.md` — binary resolution (dev vs packaged),
  `child_process.spawn`, `/healthz` poll, mid-session crash policy
  (bounded respawn), `SIGTERM` shutdown, single-instance lock, where
  each platform's orphan-prevention code lives.
- `desktop-host-ipc-surface.md` — one `window.alexandryn` namespace,
  `operations.ts` as the single source of truth, Zod validation in main,
  runtime iteration (no codegen), the FR-3 addition checklist,
  `system.getAppVersion` (the worked example) + `source.pickLocalFolder`
  (phase-08 amendment).
- `desktop-host-window-and-serving.md` — `BrowserWindow` options,
  window-state persistence, `tokens.css`-synced boot asset, external-link
  interception, `loadURL` sequencing, the `Recovering` banner injected
  by the main process with no renderer cooperation.

Like phase 04, these specs are near-exhaustively concrete — nearly every
API, library, and mechanism is already fixed. This plan is sequencing,
tooling setup (no `electron/` exists yet), TDD mechanics, and a small
number of genuine decisions (D1–D7 below), not new behaviour.

**`architecture-testing.md` FR-2 already resolved the Electron E2E tool**:
`@playwright/test`'s `_electron`, run on a Linux GitHub Actions runner
behind `xvfb` — never the Playwright MCP. This plan picks the exact xvfb
setup step (D5).

## Design-conformance check (2026-08-27, this session — re-verified for phase 05)

Re-synced all four `.dc.html` canvases via `DesignSync` against project
`78075626-e444-438f-8437-205d57129a37` (`get_project`, `list_files`,
`get_file` per path) and diffed each against `.design-reference/`:
**SHA-256 identical**, no drift since the 2026-08-13 sync
(`.design-reference/ANALYSIS.md`). `list_files` re-checked at plan time:
same four canvases, no new files.

Phase 05's only UI surface is the **disk-loaded loading/error boot asset**
(and the injected `Recovering` banner), which both specs and
`architecture-desktop-host.md` FR-6/FR-7 tie to the **`atStates`** screen
(`Alexandryn-Electron-Admin.dc.html`). Per `ANALYSIS.md`'s classification
table: `atStates` is **Binding** ("actively relied on … for loading/error
copy conventions"); its file-location question ("final home") stays open
but does not block. `atFirstRun` is **Binding** but out of phase 05's
scope (the spec owns "getting the window open", not first-run steps —
phase 12). The host window's titlebar treatment (`titleBarStyle:
'hiddenInset'` on macOS) matches the design reference's custom-titlebar
shell composition without a captured host-chrome screen of its own — no
conflict, no owed spec.

**`atTablet` flagged again** (carried, unchanged): captured in the
Electron/Admin canvas, not the Web canvas ADR 0003 assigned it to;
`ANALYSIS.md` row "Binding, location TBD". `architecture-desktop-host.md`
Open questions notes phase 05 would own rendering it *if* the answer is
"host-side reference view". Not resolved here; phase 05 builds no
tablet-specific surface. Carried to whoever owns the Claude Design
project, tracked in phase 05's own Open questions and F27-equivalent.

No captured screen contradicts phase 05's intended scope. No stop-and-ask
triggered by the design reference.

## Decisions (resolve before/alongside the tasks that need them)

- **D1 — npm workspace layout: create the ADR 0008 root now.** ADR 0008
  fixes `package.json` at repo root with `"workspaces": ["web",
  "electron"]`. It doesn't exist yet — `web/` is currently standalone.
  Tier 0 creates the root `package.json` and moves `web/` under it as a
  workspace member. **Risk to manage:** phase 04's CI is green with
  `working-directory: web` and `cache-dependency-path:
  web/package-lock.json`; workspace-ification replaces the two lockfiles
  with one root `package-lock.json` and changes `npm ci` semantics.
  Tier 0's PR proves `web/`'s full check suite still passes under the
  workspace root before any `electron/` code lands. Rejected: keeping
  `electron/` standalone like `web/` — it defers an Accepted-ADR
  requirement and means two `npm install`s / two dependency trees for
  two TypeScript packages that will share tooling (ESLint, Prettier,
  Playwright, Zod).
- **D2 — Electron main/preload/boot build: `electron-vite`** (maintainer
  decision, 2026-08-28 — overriding the plan's initial `tsc`-only
  recommendation). Aligns `electron/`'s toolchain with `web/`'s Vite,
  gives main-process HMR during dev, and produces the three build
  targets (main, preload, the disk-loaded boot renderer) from one
  config. `electron-vite` is pinned (constitution §9: what it does —
  the standard Vite integration for Electron's three-target build; why
  not stdlib / hand-rolled — it wraps Vite's own main/preload/renderer
  presets which would otherwise be three hand-maintained configs; exit
  cost — it is a thin config layer over Vite, so a move back to plain
  `tsc` or to another integration rewrites `electron.vite.config.ts`
  only, not the main/preload source). The boot asset (static HTML/CSS,
  Tier 3) is `electron-vite`'s renderer target, `loadFile`-loaded from
  `electron/out/` (never `loadURL`). Electron itself is pinned too
  (§9: the desktop runtime; there is no stdlib desktop runtime; exit
  cost — the main/preload code is ~10 files against a well-documented
  API).
- **D3 — the IPC `operations.ts` runtime-iteration mechanism is shared
  code, imported by both `preload/` and `main/`.** The spec fixes
  "iterate the same array" on both sides. `operations.ts` lives at
  `electron/src/shared/operations.ts`, imported by
  `electron/src/preload/index.ts` (builds the `contextBridge` object) and
  `electron/src/main/ipc.ts` (registers `ipcMain.handle`). The handler
  functions live in `main/` and are referenced from the descriptor array;
  the preload side uses only `name` (never imports a handler). A
  structural test (Tier 4) asserts no `ipcMain.handle` call exists
  outside the iteration over that array.
- **D4 — `tokens.css` reaches `electron/` by a copy step in `electron/`'s
  build, reading `web/src/tokens.css` directly, with a build-time
  existence assertion (spec FR-3).** Both files regenerate from the same
  `web/scripts/tokens/` extraction pass in the same CI run, so a content
  diff isn't needed — only build *order* (extraction before either
  consumer builds) and an existence check. The CI `desktop` job runs
  `npm run tokens:generate` (in `web/`) before building `electron/`, same
  as the `frontend` job already does for its own build. Rejected:
  symlink (breaks on Windows checkouts / in the packaged build); the
  token generator emitting a third copy into `electron/` (two
  independently-timed outputs is exactly the drift FR-3 prevents).
- **D5 — Electron E2E in CI: a new `desktop` job, `xvfb-run` wrapping the
  Playwright command directly** (not a marketplace action). `needs:
  [frontend, backend]` — it needs `web/dist` embedded in a built Go
  binary to load the real UI, and its own `electron/out`. The job:
  download the `backend`'s built binary artifact (add an upload step to
  the `backend` job), build `electron/`, `xvfb-run -a npx playwright test
  --project=electron`. `architecture-testing.md` FR-2 fixes the tool and
  that a display server must exist; this picks `xvfb-run -a` as the
  wrapper (already the pattern the repo's shell-check scripts assume a
  plain CLI, no action dependency).
- **D6 — Linux + macOS parent-death monitoring is added to `cmd/server`
  (Go); Windows Job Object is in `electron/` main via a native addon.**
  Per `desktop-host-process-model.md` FR-6. The Go server currently has
  **no** parent-PID monitoring (the existing `Pdeathsig` code is in
  `internal/persistence/postgres/supervisor/` — that's pg-supervisor
  watching *Postgres*, a different parent/child pair). Phase 05 adds a
  new `internal/deskhost/parentwatch` (or similar) package:
  `prctl(PR_SET_PDEATHSIG, SIGKILL)` on Linux (mirroring the existing
  supervisor code, ADR 0005), `kqueue`/`EVFILT_PROC`/`NOTE_EXIT` on
  macOS, no-op elsewhere, wired into `cmd/server`'s startup so the Go
  server self-exits if the Electron PID that spawned it disappears. The
  Electron main process passes its own PID to the child (a new,
  non-secret arg or a line in the FR-5 config file). Windows: a native
  Node addon assigned at spawn time in `electron/src/main/`.
- **D7 — macOS and Windows paths are implemented-to-spec and unit-tested
  where a unit boundary exists, but their integration behaviour is
  NOT verified in phase 05** — CI is Linux-only, and no macOS/Windows
  hardware or runner is in scope (`architecture-desktop-host.md` FR-8/FR-9,
  ADR 0005, and both phase-05 specs already carry this exact caveat).
  The Linux lifecycle is fully E2E-tested. macOS/Windows orphan
  prevention, `titleBarStyle`, and the Job Object addon are landed with
  their code, their unit tests, and an explicit "unverified on real
  hardware — see Open questions" note in the phase-05 closure and the
  security audit's "What was not examined". This is a stated limitation,
  not a hidden gap.

Everything else is fixed by the specs, not re-decided: `child_process.spawn`
with `['--config', path]`, `GET /healthz` at 250ms until a 15s timeout,
3 respawn attempts at 1s/4s/9s, `SIGTERM` then `SIGKILL` after the 10s
grace period, `window-all-closed → app.quit()` on every platform, Zod for
IPC validation, `webContents.setWindowOpenHandler` + `will-navigate`
routed through a scheme-validated `shell.openExternal`, window state in a
`userData` JSON file with a 500ms debounce, the `Recovering` banner via
`insertCSS` + `executeJavaScript`.

## Task list

**Tier 0 — Electron bootstrap** (`architecture-desktop-host.md` FR-2,
ADR 0008, D1–D5)

- **E1.** Root `package.json` (`"workspaces": ["web", "electron"]`),
  single root `package-lock.json`; move `web/` to a workspace member
  (its own `package.json` stays, lockfile consolidates). Prove `web/`'s
  entire check suite (build, lint, test, storybook, all `check:*`,
  playwright) still passes under the workspace root. Update
  `.github/workflows/ci.yml`'s `frontend` job paths.
- **E2.** `electron/` package: `electron` + `electron-vite` pinned, `typescript`,
  `electron.vite.config.ts` (main / preload / boot-renderer targets),
  `electron/tsconfig.json` (`strict`, `noUncheckedIndexedAccess`), `electron/src/main/`,
  `electron/src/preload/`, `electron/src/shared/`, a minimal
  `main/index.ts` that creates one `BrowserWindow` and loads a static
  placeholder. `npm run -w electron build` (electron-vite) → `electron/out/`.
- **E3.** Shared ESLint + Prettier config at the workspace root, covering
  both `web/` and `electron/` (electron gets `no-restricted-imports` for
  the wildcard-IPC rule, a lint-enforced constitution §5 guard).
- **E4.** `@playwright/test` `_electron` harness: a `playwright.config.ts`
  for `electron/` with an `electron` project, one smoke test that
  launches the built app, asserts a window opens, and asserts
  `contextIsolation` / `sandbox` / `nodeIntegration` are correct
  (`architecture-desktop-host.md` FR-2 — the automated proof, not a
  config comment). RED first: the assertion fails against a deliberately
  mis-set flag.
- **E5.** CI `desktop` job (D5): `needs: [frontend, backend]`; the
  `backend` job gains a `go build -o` + `upload-artifact` step for the
  server binary; `desktop` downloads it, builds `electron/`, runs
  `xvfb-run -a npx playwright test --project=electron`. Also: `tsc
  --noEmit` for `electron/`, the ESLint step, unit tests (Vitest,
  reusing `web/`'s runner config or a sibling).

  > **Checkpoint P5-A** — `npm run -w electron build` + lint + typecheck
  > + the `_electron` smoke test all green locally and in CI; the
  > contextIsolation/sandbox assertion proven to fail on a mis-set flag
  > (test branch); `web/`'s full phase-04 suite still green under the new
  > workspace root.

**Tier 1 — Process model: spawn, health, shutdown, single-instance**
(`desktop-host-process-model.md` FR-1, FR-2, FR-3, FR-5, FR-7)

- **E6.** `resolveServerBinaryPath()` (FR-1) — one function, dev
  (`<root>/bin/alexandryn-server`) vs packaged
  (`process.resourcesPath/server/<platformBinaryName()>`), `.exe` on
  Windows. Unit-tested both branches with mocked `app.isPackaged`.
  Document the `go build -o bin/alexandryn-server ./cmd/server` step that
  must precede `npm run dev`.
- **E7.** Config file authoring (FR-5, `architecture-desktop-host.md`
  FR-5) — main process writes a `0600` file via the atomic temp-file API
  (never a predictable shared-temp path — TOCTOU), passes only the path.
  Deletes it once readiness succeeds or after the timeout. Retains the
  in-memory grace-period value for FR-5 shutdown. Unit-tested:
  permissions, atomic creation, deletion-on-ready, deletion-on-timeout.
- **E8.** Single-instance lock (FR-7) — `app.requestSingleInstanceLock()`
  **before** any window and **before** the spawn call; `second-instance`
  handler focuses the existing window; a non-primary process
  `app.quit()`s immediately, spawns nothing. Integration test: a real
  second process against a held lock.
- **E9.** Spawn (FR-2) — `child_process.spawn`, `['--config',
  configPath]`, `stdout`/`stderr` piped and prefix-logged, `PORT=<n>`
  first-line captured. A grep/structural security test: the spawn call is
  never `exec`/`shell: true`/a concatenated string (D-style check,
  `scripts/`-pattern).
- **E10.** Readiness poll (FR-3) — `GET http://127.0.0.1:<port>/healthz`
  every 250ms until 200 or the 15s timeout; on timeout → `Failed`
  (Tier 3 renders it). Integration test against a real short-startup test
  binary.
- **E11.** Shutdown (FR-5) — `app.before-quit` → `event.preventDefault()`,
  `SIGTERM`, wait ≤ 10s, `SIGKILL` if still alive, then quit.
  `window-all-closed → app.quit()` on **every** platform (override
  macOS's framework default — test proves the override is wired).
  Integration test with a deliberately slow test binary.

  > **Checkpoint P5-B** — cold start against a real test Go binary:
  > lock acquired → config written → spawn → `/healthz` polled → ready,
  > all real wall-clock; a second instance exits immediately and spawns
  > nothing; `SIGTERM`-then-`SIGKILL` shutdown proven with a slow binary;
  > `window-all-closed` quits on Linux (macOS override wired + unit-tested,
  > integration-unverified per D7); the no-`exec` structural test passes.

**Tier 2 — Crash recovery** (`desktop-host-process-model.md` FR-4)

- **E12.** Post-ready crash detection — the child `exit` event *after*
  readiness succeeded once this session enters `Recovering` (distinct
  from first-start failure, distinct from `architecture-system.md`'s
  `Degraded`).
- **E13.** Bounded respawn — 3 attempts, backoff 1s/4s/9s, each attempt
  re-resolving the binary, re-writing the config file, re-capturing
  `PORT=<n>`, re-running the E10 readiness gate against that attempt's
  announced port. All 3 fail → `Failed`, retry only via a manual action
  that resets the counter — never an unbounded loop.
  - Unit: the backoff schedule as a pure function of attempt number,
    fake clock, isolated from process I/O.
  - Integration: a deliberately-crashing test binary at real
    test-scaled-down backoff proves exactly 3 attempts then stop.
  - Integration: a test binary that picks a different port on its second
    start proves the port re-capture (Tier 3's reload consumes it).

  > **Checkpoint P5-C** — a crash after readiness triggers exactly 3
  > respawns with the specified backoff then stops at `Failed`; the
  > schedule is separately unit-proven as a pure function; a
  > different-port respawn is captured correctly; no code path loops
  > unbounded (proven, not asserted).

**Tier 3 — Window and serving** (`desktop-host-window-and-serving.md`
FR-1–FR-6, D4)

- **E14.** `BrowserWindow` construction (FR-1) — the three security
  flags, `titleBarStyle: 'hiddenInset'` (macOS) / native
  (Windows/Linux), `minWidth`/`minHeight` from
  `web/src/breakpoints.ts`'s reflow token, no menu bar. A test reads the
  actual construction call, not the docs.
- **E15.** Window-state persistence (FR-2) — `{width,height,x,y}` JSON in
  `app.getPath('userData')`, written debounced 500ms on `resize`/`move`,
  read once at creation, corrupted/missing → centered default, never a
  crash. Unit-tested (pure fns) + integration (simulated restart,
  corrupted file).
- **E16.** The boot asset (FR-3) — static HTML/CSS in
  `electron/src/renderer/boot/`, `loadFile` only, imports the copied
  `tokens.css` (D4), loading + error variants, `atStates` visual
  treatment + copy, semantic HTML, visible focus ring on Retry,
  `aria-live` on the loading→error transition. Build-time existence
  assertion for `tokens.css` — a hard build failure when absent (proven
  on a test branch). Accessibility check via the Playwright MCP
  accessibility snapshot (dev-time) + an `@axe-core/playwright` scan in
  the `_electron` suite.
- **E17.** External-link interception (FR-4) —
  `setWindowOpenHandler → {action:'deny'}` + scheme-validated
  `shell.openExternal` (`http:`/`https:` only); `will-navigate` routed
  through the same handler for any non-loopback/non-LAN origin. Unit:
  the scheme validator. Integration (`_electron`): real `file://`,
  `javascript:`, `http://evil.example` navigation attempts each blocked;
  a legitimate external link opens via `shell.openExternal` (mocked).
- **E18.** `loadURL` sequencing (FR-5) — once E10's readiness succeeds,
  one `loadURL('http://127.0.0.1:<port>')` replaces the boot asset; never
  a redirect from inside the boot asset's own JS. Illegal-transition
  test: `loadURL` never fires before readiness.
- **E19.** The `Recovering` banner (FR-6) — `insertCSS` (importing
  `tokens.css`) + `executeJavaScript` injecting one fixed
  `role="status"` `aria-live="polite"` node over the already-loaded real
  UI, on Tier 2's `Recovering` entry; removed on readiness return;
  a port change triggers a fresh `loadURL` before removal; `Failed` does
  a full `loadFile` back to the error asset (not the banner). Integration
  (`_electron`) against a real window with real content: inject/remove,
  same-port (no reload) vs different-port (reload) vs final-failure (full
  replace).

  > **Checkpoint P5-D** — `BrowserWindow` options verified by reading the
  > real call; window state persists across a simulated restart and
  > survives a corrupted file; the boot asset resolves tokens from
  > `tokens.css` and its absence fails the build; `file://`/`javascript:`/
  > external-origin navigation each blocked; the real UI loads only after
  > readiness; the `Recovering` banner appears/disappears including the
  > port-change reload, proven end to end.

**Tier 4 — IPC surface** (`desktop-host-ipc-surface.md` FR-1–FR-6, D3)

- **E20.** `electron/src/shared/operations.ts` — the descriptor array
  (`{name, schema, handler}`), Zod pinned (constitution §9 note in the
  PR). `main/ipc.ts` iterates it for `ipcMain.handle` (schema
  `.safeParse` as the handler's first line, reject + log on failure,
  never business logic first); `preload/index.ts` iterates it for the
  `contextBridge` namespace (one channel per operation, never a shared
  operation-name channel). Generated TS types so a renderer call-site
  mismatch is a compile error.
- **E21.** `system.getAppVersion(): Promise<string>` (FR-5) — reads
  `package.json` version once at main startup, no argument, non-sensitive
  return. Unit + `_electron` integration through the real
  preload/main boundary.
- **E22.** `source.pickLocalFolder(): Promise<{path:string}|null>`
  (FR-6) — `dialog.showOpenDirectory` in main, no argument, returns the
  chosen absolute path or `null` on cancel; main does no filesystem
  access with the path. Unit (mocked dialog) + `_electron` integration.
- **E23.** Structural security tests — (a) no `ipcMain.handle` anywhere
  outside the E20 iteration (grep/AST check, `scripts/`-pattern); (b)
  per operation, a hostile-argument test (oversized string, wrong type,
  extra fields) rejected by the Zod schema before the handler acts; (c)
  the FR-3 checklist's mechanical items (schema present, test exists) as
  a CI check.

  > **Checkpoint P5-E** — every operation declared only in
  > `operations.ts`, proven by a structural test; `system.getAppVersion`
  > and `source.pickLocalFolder` work end to end through the real
  > preload/main boundary; every schema rejects a malformed shape;
  > `contextIsolation`/`sandbox`/`nodeIntegration` re-verified here
  > (shared assertion with Tier 0).

**Tier 5 — Orphan prevention** (`desktop-host-process-model.md` FR-6,
`architecture-desktop-host.md` FR-8/FR-9, ADR 0005, D6/D7)

- **E24.** Electron passes its own PID to the Go child (a non-secret
  spawn arg or an FR-5 config-file field — pick the config file, keeps
  argv minimal).
- **E25.** `internal/deskhost/parentwatch` (Go) — `PR_SET_PDEATHSIG`
  (Linux, mirrors the existing supervisor code), `kqueue` `EVFILT_PROC`/
  `NOTE_EXIT` on the passed PID (macOS), no-op elsewhere. Wired into
  `cmd/server` startup. Linux: re-exec-the-test-binary proof (the same
  pattern `spawn_linux_test.go` already uses). macOS: unit-testable
  seam + an explicit unverified note (D7).
- **E26.** Windows Job Object (`electron/src/main/`) — a small, audited
  native addon assigned at spawn time with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. Exact package: an Open question
  (both specs carry this). Unverified on real Windows (D7) — landed with
  the code + a unit seam + the note.

  > **Checkpoint P5-F** — Linux: killing the Electron parent kills the Go
  > server, proven by a real re-exec test; macOS `kqueue` + Windows Job
  > Object code landed, unit-tested at their seams, integration-unverified
  > and explicitly flagged (D7, Open questions).

**Tier 6 — E2E lifecycle walkthrough**
(`architecture-desktop-host.md` Test strategy, `architecture-system.md`'s
"open a book" slice)

- **E27.** The full `_electron` walkthrough — cold start with PostgreSQL
  unreachable: window shows loading → error (`atStates`), a manual Retry
  succeeds once PostgreSQL is up, the window loads the real UI, and the
  keyboard-only "open a book" slice completes inside the Electron window
  against real (phase-06-absent, so still MSW-or-stub) data. This reuses
  phase 04's own shell E2E, now driven through `_electron` instead of a
  plain browser.
- **E28.** The hostile walkthrough — a "compromised renderer" attempts
  `window.open('https://evil.example')` (blocked, FR-4) and calls a
  preload method with an oversized/malformed argument (blocked, FR-2's
  Zod) — proven in the `_electron` suite.

  > **Checkpoint P5-G** — the lifecycle walkthrough passes end to end in
  > CI (`xvfb`); the hostile walkthrough's blocks are proven; the
  > keyboard-only book-open slice completes inside Electron.

**Tier 7 — Closure**

- **E29.** Security audit — constitution §10 four-attacker pass, scoped
  to phase 05's real trust boundary (**the renderer ↔ main-process
  privilege boundary** — the one this whole phase exists to enforce),
  plus the spawn channel, the config file, the loopback HTTP server, and
  external-link handling. Run `/security-review` + an
  `agent-skills:security-auditor` subagent + the
  `agent-skills:security-and-hardening` checklist; reconcile into
  `.claude/audits/0005-phase05-desktop-host.md`. Fix Critical/High with a
  regression test per fix; document accepted Medium/Low.
- **E30.** Closure — the three specs → `VERIFIED` (each acceptance
  criterion evidenced first); `roadmap/05-desktop-host/README.md` exit
  criteria walked item-by-item with real evidence; `.claude/specs/README.md`
  updated; docs (root README, `electron/README.md`, this plan's todo)
  swept; the D7 macOS/Windows-unverified limitation and every carried
  Open question (DATABASE_URL provisioning, the 15s / retry / debounce
  placeholders, the Windows addon package, `atTablet`) recorded
  explicitly.

  > **Checkpoint P5-H (final)** — full suite green (both workspace
  > packages: `web/`'s phase-04 suite + `electron/`'s build / lint /
  > typecheck / Vitest / `_electron` Playwright / structural security
  > checks / `npm audit`); `go build ./...` + `go test ./...` green
  > (the new `internal/deskhost/parentwatch`); security audit recorded
  > with no open Critical/High; all three specs `VERIFIED`; roadmap exit
  > criteria checked with evidence; maintainer approval recorded — phase
  > 05 complete.

## Constitution stop-and-ask gates

1. **Now — this plan.** Present it, wait for go-ahead before Tier 0.
   (No new spec or ADR is being drafted — the three specs are `APPROVED`
   and `architecture-testing.md` already resolved the E2E-tool question;
   D1–D7 are implementation choices within the approved specs, the same
   category as phase 04's D1–D3. If Tier 0 surfaces that D1's
   workspace-ification needs an ADR, that's its own check-in.)
2. **After the security audit (E29), before closure (E30).** Hard stop —
   present the audit and its findings, wait for maintainer sign-off.
   Then E30, then a second hard stop at Checkpoint P5-H for final
   approval. Neither gate crossable on the automated contributor's own
   judgement (constitution §10 / Review-gates).

## Critical files

- `.claude/specs/architecture-desktop-host.md` — governing spec, FR-1–FR-11
- `.claude/specs/desktop-host-process-model.md` — Tiers 1, 2, 5
- `.claude/specs/desktop-host-ipc-surface.md` — Tier 4
- `.claude/specs/desktop-host-window-and-serving.md` — Tier 3
- `.claude/specs/architecture-testing.md` FR-2 — the `_electron` / `xvfb`
  CI decision, already made
- ADR 0008 — `electron/` at root, npm workspaces, `web/dist` `go:embed`
- ADR 0005 — the Linux `pdeathsig` prototype the Go-side parent-watch
  mirrors
- `internal/persistence/postgres/supervisor/spawn_*.go` — the existing
  per-platform spawn pattern E25 follows
- `cmd/server/` — gains the `parentwatch` wiring (E24/E25)
- `web/src/tokens.css`, `web/scripts/tokens/` — the D4 token-sync source
- `.github/workflows/ci.yml` — the new `desktop` job, the `backend`
  binary-artifact upload, the `frontend` job's workspace-path update

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Workspace-ification (D1) breaks phase 04's green CI | High if unnoticed | Tier 0's own PR proves `web/`'s entire suite passes under the root before any `electron/` code; the `frontend` job is updated and re-run in that same PR |
| macOS / Windows orphan-prevention and `titleBarStyle` land unverified (D7) | Medium — a real gap on two of three platforms | Explicit, not hidden: unit seams where they exist, an "unverified on hardware" line in the closure, the security audit's "What was not examined", and a carried Open question for a future macOS/Windows CI pass |
| The Windows Job Object native addon has no vetted package chosen | Medium | Both specs already carry this as an Open question; E26 lands the mechanism + seam, the package choice is made when a Windows environment exists to verify it |
| `_electron` tests flake under `xvfb` on CI | Medium | Playwright's own retry (phase 04's `playwright.config.ts` already uses `retries: process.env.CI ? 2 : 1`); the lifecycle test uses real-but-short test binaries, no fake clock composed with process events |
| The 15s readiness timeout / 3-attempt backoff / 500ms debounce stay placeholders | Low for phase 05 | Every one is a spec Open question already; phase 05 measures a real cold start (incl. the bundled-Postgres path) where it can and records the number, or records that it's still a placeholder and why |
| `DATABASE_URL` provisioning for a packaged build is unresolved | Low for phase 05's Linux-dev lifecycle | `cmd/pg-supervisor` exists (ADR 0007); the dev path uses the FR-5 file as designed; the packaged-install provisioning question is carried, owned by a future ADR / `architecture-persistence.md`, not phase 05's to close |

## Open questions (carried into implementation / past phase 05)

- **`DATABASE_URL` for a packaged end-user install** —
  `architecture-desktop-host.md` Open questions; ADR 0007 decided
  bundled Postgres, `cmd/pg-supervisor` exists, but how the connection
  string is provisioned in a shipped build is still open. Not phase 05's
  to resolve; the dev lifecycle works via the FR-5 file.
- **The 15-second readiness timeout, the 3×(1s/4s/9s) respawn schedule,
  the 500ms window-state debounce** — reasoned placeholders in all three
  specs. Phase 05 measures where it can (a real Linux cold start),
  records the result, and carries the rest.
- **The Windows Job Object native addon package** — unfixed; needs a
  Windows environment to verify.
- **macOS `kqueue` / Windows Job Object integration behaviour** — landed
  to spec, unit-tested at their seams, integration-unverified (D7).
- **`atTablet`'s surface** — still open from `architecture-frontend.md` /
  `architecture-desktop-host.md`; phase 05 builds no tablet surface and
  does not resolve it.
