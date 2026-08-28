# Phase 05 (Desktop host) — task list

Full plan with context/approach/decisions: [`tasks/plan-phase05.md`](plan-phase05.md).
Execute in order; each task is RED → GREEN → Refactor, one commit. Stop
at every checkpoint. Two constitution stop-and-ask gates: this plan (now),
and after the security audit (E29 → E30).

## Decisions (all resolved by maintainer 2026-08-28)

- [x] D1 — **create the npm workspace root now** (`package.json` + `["web","electron"]`), `web/` becomes a member; Tier 0 proves phase-04 suite still green
- [x] D2 — **`electron-vite`** for `electron/`'s main/preload/boot build (maintainer override of the plan's `tsc` recommendation); boot asset is the renderer target, `loadFile` from `electron/out/`
- [x] D3 — `operations.ts` at `electron/src/shared/`, runtime-iterated by both `preload/` and `main/` (spec-aligned)
- [x] D4 — `tokens.css` copied into `electron/` build from `web/src/tokens.css`, build-time existence assert (spec-aligned; no symlink, no third copy)
- [x] D5 — CI `desktop` job, **`xvfb-run -a`** wrapping the command, `needs: [frontend, backend]`, downloads the server binary artifact
- [x] D6 — Linux+macOS parent-watch in `cmd/server` (new `internal/deskhost/parentwatch`); Windows Job Object in `electron/` main (per FR-6)
- [x] D7 — **land macOS/Windows to spec, unit-test seams, flag integration-UNVERIFIED** (Linux-only CI); stated in closure + audit, carried Open question for a future mac/win pass
- [x] Plan approved — run Tiers 0–7 autonomously through E29 (security audit), stop at the post-audit gate

## Tasks

**Tier 0 — Electron bootstrap**

- [ ] E1 — root `package.json` workspaces; `web/` → member; consolidate lockfile; prove `web/`'s full check suite green under the root; update `ci.yml` `frontend` paths
- [ ] E2 — `electron/` package: pinned `electron` + `electron-vite` + `typescript`, `electron.vite.config.ts` (main / preload / boot-renderer targets), `electron/tsconfig.json` (strict + `noUncheckedIndexedAccess`), `src/{main,preload,shared,renderer/boot}/`, minimal `main/index.ts` opening one window, `npm run -w electron build` → `electron/out/`
- [ ] E3 — shared root ESLint + Prettier for both packages; `electron/` gets `no-restricted-imports` guarding wildcard IPC
- [ ] E4 — `@playwright/test` `_electron` harness + `electron` project; smoke test: window opens, `contextIsolation`/`sandbox`/`nodeIntegration` asserted correct (RED against a mis-set flag first)
- [ ] E5 — CI `desktop` job (D5): `backend` job uploads the server binary; `desktop` `needs:[frontend,backend]`, builds `electron/`, `xvfb-run -a npx playwright test --project=electron`, + `tsc --noEmit`, ESLint, Vitest

**Checkpoint P5-A** — electron build/lint/typecheck/`_electron` smoke green locally + CI; ctx-isolation assertion fails on a mis-set flag (test branch); `web/`'s phase-04 suite still green under the workspace root

**Tier 1 — Process model**

- [ ] E6 — `resolveServerBinaryPath()` (FR-1), dev vs packaged, `.exe` on Windows, both branches unit-tested (mocked `app.isPackaged`); document the `go build -o bin/alexandryn-server ./cmd/server` pre-`npm run dev` step
- [ ] E7 — FR-5 config file: `0600`, atomic temp-file API (no predictable shared-temp path), path-only arg, delete on ready / on timeout, retain grace-period in memory; unit-tested (perms, atomicity, both deletion paths)
- [ ] E8 — single-instance lock (FR-7) BEFORE window + BEFORE spawn; `second-instance` focuses existing; non-primary `app.quit()`s, spawns nothing; integration test with a real second process
- [ ] E9 — spawn (FR-2): `child_process.spawn`, `['--config', path]`, piped+prefixed stdio, `PORT=<n>` captured; structural security test — never `exec`/`shell:true`/concatenated string
- [ ] E10 — readiness poll (FR-3): `GET /healthz` @250ms until 200 or 15s; timeout → `Failed`; integration test vs a real short-startup test binary
- [ ] E11 — shutdown (FR-5): `before-quit` → `preventDefault`, `SIGTERM`, ≤10s, `SIGKILL`; `window-all-closed → app.quit()` every platform (macOS override wired + unit-tested); integration test with a slow binary

**Checkpoint P5-B** — real cold start (lock→config→spawn→poll→ready, real wall-clock); second instance exits + spawns nothing; SIGTERM→SIGKILL proven; `window-all-closed` quits on Linux (macOS override unit-only per D7); no-`exec` structural test green

**Tier 2 — Crash recovery**

- [ ] E12 — post-ready crash → `Recovering` (distinct from first-start failure and from `Degraded`)
- [ ] E13 — bounded respawn: 3 attempts, 1s/4s/9s, re-resolve/re-config/re-capture-port/re-gate each; all fail → `Failed`, manual-retry-only, never unbounded. Unit: backoff schedule as pure fn (fake clock, no I/O). Integration: crashing test binary proves exactly 3 then stop; different-port test binary proves re-capture

**Checkpoint P5-C** — 3 respawns w/ backoff then stop at `Failed`; schedule unit-proven; different-port respawn captured; no unbounded loop (proven)

**Tier 3 — Window and serving**

- [ ] E14 — `BrowserWindow` (FR-1): 3 security flags, `titleBarStyle` per platform, `minWidth`/`minHeight` from `web/src/breakpoints.ts`, no menu; test reads the real construction call
- [ ] E15 — window-state persistence (FR-2): `{w,h,x,y}` JSON in `userData`, 500ms-debounced write, read-once, corrupt/missing → centered default, no crash; unit + integration (simulated restart, corrupt file)
- [ ] E16 — boot asset (FR-3): static HTML/CSS in `electron/src/renderer/boot/`, `loadFile` only, imports copied `tokens.css`, loading + error variants, `atStates` treatment/copy, semantic HTML + focus ring on Retry + `aria-live`; build-time `tokens.css` existence assert (hard fail, proven on a test branch); axe scan in `_electron`
- [ ] E17 — external-link interception (FR-4): `setWindowOpenHandler` deny + scheme-validated `shell.openExternal` (`http`/`https` only); `will-navigate` for non-loopback/LAN routed same; unit (scheme validator); integration (`file://`, `javascript:`, `http://evil.example` blocked; legit link opens external — mocked)
- [ ] E18 — `loadURL` sequencing (FR-5): after E10 ready, one `loadURL('http://127.0.0.1:<port>')`, never a boot-asset-internal redirect; illegal-transition test (`loadURL` never before ready)
- [ ] E19 — `Recovering` banner (FR-6): `insertCSS`(+`tokens.css`) + `executeJavaScript` one `role="status"`/`aria-live` node; on `Recovering` entry, removed on ready return; port change → fresh `loadURL` first; `Failed` → full `loadFile` to error asset. Integration (`_electron`): inject/remove, same-port vs different-port vs final-failure

**Checkpoint P5-D** — window opts verified from the real call; state persists across restart + survives corruption; boot asset resolves `tokens.css` and its absence fails the build; `file://`/`javascript:`/external nav blocked; real UI loads only after ready; `Recovering` banner appears/disappears incl. port-change reload — proven E2E

**Tier 4 — IPC surface**

- [ ] E20 — `electron/src/shared/operations.ts` descriptor array + Zod (pinned, §9 note); `main/ipc.ts` iterates → `ipcMain.handle` (`safeParse` first line, reject+log); `preload/index.ts` iterates → `contextBridge` (one channel per op); generated TS types
- [ ] E21 — `system.getAppVersion()` (FR-5): reads `package.json` version once at startup, no arg; unit + `_electron` integration
- [ ] E22 — `source.pickLocalFolder()` (FR-6): `dialog.showOpenDirectory` in main, no arg, path or `null`; main does no FS access; unit (mocked) + `_electron` integration
- [ ] E23 — structural security tests: (a) no `ipcMain.handle` outside the E20 iteration; (b) per-op hostile-arg test (oversized/wrong-type/extra-field) rejected pre-handler; (c) FR-3 checklist mechanical items (schema present, test exists) as a CI check

**Checkpoint P5-E** — every op only in `operations.ts` (structural test); both ops work E2E through the real boundary; every schema rejects a malformed shape; ctx-isolation/sandbox/nodeIntegration re-verified

**Tier 5 — Orphan prevention**

- [ ] E24 — Electron passes its own PID to the Go child (via the FR-5 config file)
- [ ] E25 — `internal/deskhost/parentwatch` (Go): `PR_SET_PDEATHSIG` (Linux), `kqueue`/`EVFILT_PROC`/`NOTE_EXIT` (macOS), no-op else; wired into `cmd/server` startup. Linux: re-exec-the-test-binary proof (like `spawn_linux_test.go`). macOS: unit seam + explicit unverified note
- [ ] E26 — Windows Job Object (`electron/src/main/`): native addon, `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` at spawn; package = Open question; unverified (D7), landed w/ code + seam + note

**Checkpoint P5-F** — Linux: killing Electron parent kills the Go server (real re-exec test); macOS + Windows code landed, unit-tested at seams, integration-unverified + flagged (D7)

**Tier 6 — E2E lifecycle walkthrough**

- [ ] E27 — full `_electron` walkthrough: cold start w/ PostgreSQL unreachable → loading → error (`atStates`) → manual Retry succeeds once PG up → real UI loads → keyboard-only "open a book" slice completes inside Electron
- [ ] E28 — hostile walkthrough: `window.open('https://evil.example')` blocked (FR-4); preload method w/ oversized/malformed arg blocked (FR-2 Zod) — in the `_electron` suite

**Checkpoint P5-G** — lifecycle walkthrough passes E2E in CI (`xvfb`); hostile blocks proven; keyboard-only book-open completes inside Electron

**Tier 7 — Closure**

- [ ] E29 — security audit (four-attacker; the renderer↔main privilege boundary is THE boundary; + spawn channel, config file, loopback server, external links). `/security-review` + `agent-skills:security-auditor` subagent + `agent-skills:security-and-hardening` checklist → `.claude/audits/0005-phase05-desktop-host.md`. Fix Critical/High w/ regression test each; document accepted Medium/Low.
- [ ] **═══ STOP GATE — present the audit, wait for maintainer sign-off ═══**
- [ ] E30 — closure: 3 specs → `VERIFIED` (acceptance criteria evidenced first); `roadmap/05-desktop-host/README.md` exit criteria walked w/ evidence; `.claude/specs/README.md`; docs (root README, `electron/README.md`, this todo); D7 limitation + every carried Open question recorded

**Checkpoint P5-H (final)** — full suite green (both workspace packages + `go build`/`go test ./...` incl. `parentwatch`); audit recorded, no open Critical/High; 3 specs `VERIFIED`; roadmap exit criteria cited; maintainer approval — phase 05 complete

## After approval

- [ ] Push, open PR (`/make-pr` conventions — no Co-Authored-By, no "Generated with Claude Code"); `gh pr checks --watch`
- [ ] Stop. Report. Don't sync main or delete the branch — wait to be asked.
