# Spec: Desktop host architecture

| | |
|---|---|
| **Status** | `APPROVED` (maintainer confirmed 2026-08-20; DATABASE_URL provisioning, timeout tuning, asset token-drift, `atTablet`, and macOS/Windows hardware verification remain open, owned by phase 05 or a future ADR) |
| **Phase** | `01-architecture` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-13 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | `.claude/reviews/0007-spec-architecture-desktop-host.md` — Approved with changes, findings 2–7 fixed, finding 1 open by design |

## Context

`architecture-system.md` (`REVIEWED`, amended after its original approval —
see its own header note) decided the process model — Electron spawns the
Go server as a child, orphan-prevented via `prctl(PR_SET_PDEATHSIG)` on
Linux (ADR 0005). Three processes, four on macOS, once the Go server's own
bundled PostgreSQL (ADR 0007) is counted — that came after this spec was
first drafted, doesn't change anything this spec itself decided, and is
noted here only so this Context paragraph stops being the stale copy of a
fact that changed elsewhere. `architecture-system.md` explicitly left four
things owned by this spec: the macOS/Windows orphan-prevention mechanism,
second-instance handling, the config/secrets channel across the spawn
boundary, and the exact port-announcement/control-plane mechanism. This spec
closes all four, and defines the rest of what "Electron processes, IPC
surface, serving model, lifecycle" (phase 01's own description of this
spec) means concretely.

The design reference (re-synced 2026-08-13,
[`.design-reference/ANALYSIS.md`](../../.design-reference/ANALYSIS.md)) now
has real content for the screens this spec's lifecycle states surface to the
user: `atFirstRun`, `atSettings`, `atSystem`, and — newly confirmed with
actual markup, not just a nav label — `atStates` (empty/loading/error
reference). This spec cites them instead of inventing loading/error copy
from nothing.

## Problem

There is no written answer to: what exactly can the Electron preload expose
(constitution §5 requires an enumerated list, but names no specifics),
whether a second instance is possible, what stops a crashed-but-not-quit
Electron leaving an orphan on macOS or Windows (Linux is solved, per ADR
0005), and how configuration reaches the spawned Go server without putting a
secret in `ps` or `/proc`.

## Goals

- Define the preload surface as a pattern (fixed namespace, enumerated,
  validated) even though the concrete operation list grows with later
  feature phases
- Close `architecture-system.md`'s four open items owned by this spec
- Define window lifecycle: splash/loading while the Go server starts, ready
  state, degraded state, error state — tied to the design reference's
  `atStates` screen, not invented copy
- Define the navigation/external-link policy required by constitution §5
- Establish what the Playwright MCP already available in this environment
  actually covers for this layer (checked, not assumed — it's browser-page
  automation, not an Electron launcher) and what it doesn't, so
  `architecture-testing.md` inherits an accurate starting point instead of
  an overclaim

## Non-goals

- The exact preload operation list for library, sources, reader, etc. — each
  arrives with its own feature phase (06, 08, 11...), enumerated then, not
  guessed now
- Multi-window support — out of scope for v1. One main window. If this
  becomes wrong, it's a new spec, not a retrofit
- Packaging, installers, code-signing — phase 99
- HTTP API endpoint shapes — `architecture-contracts.md`
- Go server internals (package layout, config precedence) —
  `architecture-backend.md`
- Authentication UI/flow — phase 12. `atFirstRun`'s later steps may touch
  this; this spec only owns getting the window open and connected to a
  running Go server

## User stories

- As **a contributor implementing phase 05**, I want the orphan-prevention
  mechanism named for every platform, not just Linux, so I'm not the one
  inventing a Windows-specific fix under deadline pressure.
- As **a user starting the app**, I want to see something other than a blank
  window while the Go server comes up, so I know the app is working, not
  frozen.
- As **a security reviewer**, I want the preload surface to be a fixed,
  auditable list, so a new IPC operation can't be added without it being
  visible in one place.

## Functional requirements

- **FR-1** The preload script MUST expose exactly one namespaced object
  (e.g. `window.alexandryn`) via `contextBridge.exposeInMainWorld`, never a
  general Node/Electron primitive. Every method on it MUST be backed by a
  handler in the main process that validates argument shape and size before
  acting (constitution §4, §5). The concrete method list is out of this
  spec's scope (Non-goals) — the pattern is not.
- **FR-2** `contextIsolation` MUST be `true`, `sandbox` MUST be `true`,
  `nodeIntegration` MUST be `false`, on every `BrowserWindow` and any
  future webview/child window, with no per-window exception.
- **FR-3** Navigation MUST be blocked to any origin other than the Go
  server's own loopback (or, post-phase-13, LAN) address. `window.open` and
  `target="_blank"` MUST be intercepted and routed to `shell.openExternal`
  (the OS default browser), never allowed to open a second Electron window
  pointed at an external origin.
- **FR-4** The main process MUST call `app.requestSingleInstanceLock()`
  before creating any window. If the lock is not acquired (another instance
  already holds it), the new process MUST quit immediately after asking the
  existing instance to focus its window — it MUST NOT proceed to spawn a
  second Go server against the same PostgreSQL data.
- **FR-5** Configuration and secrets MUST reach the spawned Go server
  through a file, not argv and not an inherited environment variable: the
  main process creates a single-use file with owner-only permissions
  (`0600`) via the OS's atomic temp-file API (e.g. `os.CreateTemp` or
  equivalent — a predictable path in a shared temp directory is a
  TOCTOU/symlink-attack surface, not just a permissions question), passes
  only that *path* as the child's first argument (a path is not sensitive),
  and the Go server reads it once at startup. The main process deletes the
  file once the Go server's readiness check (loopback HTTP `/health`, per
  `architecture-system.md` FR-7) succeeds, or after a fixed timeout if it
  never does. **Updated for ADR 0007**: in production this channel no
  longer needs to carry `DATABASE_URL` — the Go server generates its own
  after spawning its bundled PostgreSQL (`architecture-persistence.md`).
  The dev path (an external `DATABASE_URL` from the Supabase CLI stack,
  ADR 0004's addendum) still uses this exact mechanism; production's
  version of this file, if it carries anything at all yet, doesn't need a
  database connection string in it.
- **FR-6** The main process MUST NOT create or show the main window's real
  content until the Go server passes its readiness check. Before that, it
  MUST show a loading state matching the design reference's `atStates`
  loading treatment — not a blank window, not the real UI with broken data.
  This resolves what was an open contradiction: the real UI (and `atStates`
  as a screen within it) is served *by* the Go server
  (`architecture-system.md` FR-6), so it cannot be what renders while the Go
  server isn't up yet. The loading/error chrome is therefore a **small,
  separate asset bundled into the Electron app itself** — loaded from disk
  (`file://` or equivalent), never fetched over HTTP from the Go server —
  implementing the same visual treatment as `atStates` without being served
  by the thing it's reporting on. It MUST be visually consistent with
  `atStates` (same design tokens once `architecture-frontend.md` extracts
  them) but MUST NOT depend on the frontend's build succeeding or the Go
  server responding to render.
- **FR-7** If the Go server fails to become ready within a bounded timeout
  (placeholder: 15 seconds — see Open questions), the main process MUST show
  an error state matching `atStates`' error treatment, offering retry (same
  bundled asset as FR-6, its error variant) — not a silently retried
  infinite spinner, not a raw stack trace (constitution §11), and not a page
  fetched from the Go server it's reporting as unreachable.
- **FR-8** On macOS, the Go server binary MUST monitor the Electron parent's
  PID via `kqueue` (`EVFILT_PROC`, `NOTE_EXIT`) and exit if that PID
  disappears, for the same reason FR-10 of `architecture-system.md` requires
  it on Linux. Unverified in this environment (no macOS available) — named
  and specified so phase 05 implements a plan, not a guess.
- **FR-9** On Windows, the main process MUST create a Job Object with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and assign the spawned Go process to
  it at spawn time, so the OS kills the child automatically if the job
  handle closes (including on main-process crash). Unverified in this
  environment (no Windows available) — same status as FR-8.
- **FR-10** External links anywhere in the renderer (the web UI itself, not
  just Electron chrome) MUST open via the same `shell.openExternal` path as
  FR-3 — the web UI served to a LAN browser tab doesn't have this problem
  (a browser already handles its own links), but the Electron-hosted
  instance of the same UI does, and the UI code is shared (FR-6 of
  `architecture-system.md`), so this is an Electron-side interception, not
  a web-UI-side behavior change.
- **FR-11** For v1, closing the main window MUST quit the application —
  no tray icon, no minimize-to-background. This is a scope decision, not an
  oversight: a tray/background mode is a real feature (keeps serving LAN
  clients with no window open) that phase 13 might eventually want, but
  deciding it now would be guessing at a requirement phase 13 hasn't
  specified. Close-means-quit is also what makes "the app closed" in FR-8/
  FR-9's orphan-prevention requirements unambiguous: window closed, main
  process exits, child processes must die with it, full stop.

## Non-functional requirements

- **Performance** — the loading state (FR-6) must appear within 200ms of
  window creation, before the Go server is necessarily ready — the user
  should never see a blank frame, even briefly. Placeholder, unmeasured.
- **Security** — see Security considerations below.
- **Accessibility** — the loading and error states (FR-6, FR-7) need the
  same keyboard/focus/announcement treatment as any other UI state
  (constitution §7) — an error state that traps focus or isn't announced to
  a screen reader is exactly the kind of thing that's easy to skip on a
  "temporary" startup screen and then never fix.
- **Reliability** — FR-4 through FR-9 are all reliability requirements
  restated as security-adjacent; second-instance and orphan prevention are
  data-integrity concerns (two processes writing to the same PostgreSQL
  data) as much as they are process-hygiene ones.
- **Observability** — the loading/error states (FR-6, FR-7) are also where
  a user-visible correlation ID should surface on error, so a bug report can
  be tied to a specific log line without asking the user to find a log file
  (constitution §8 already forbids logging what they read; this is the
  opposite problem — making the *right* thing visible).

## Domain model

Not applicable — no Alexandryn/metadata/source domain content here
(constitution §3). This spec's window/lifecycle states are UI chrome around
the system architecture `architecture-system.md` already defined.

## API and contracts

- **Preload surface (FR-1)**: `window.alexandryn.*`, contextBridge-exposed,
  each method backed by an IPC handler validated in main. No method list
  yet — see Non-goals.
- **Control-plane channel**, replacing `architecture-system.md`'s tentative
  "stdout first line" proposal with a decision: the Go server still prints
  `PORT=<n>` as its first stdout line (unchanged), and additionally reads
  its configuration from the file path passed as `argv[1]` (FR-5). Shutdown
  is still `SIGTERM` from the main process, per `architecture-system.md`
  FR-9.
- **Window ↔ Go server**: the `BrowserWindow` loads
  `http://127.0.0.1:<port>` directly once FR-6's readiness check passes —
  no special desktop-only endpoint, per `architecture-system.md` FR-6.

## State transitions

Extends `architecture-system.md`'s application-level states with what the
*window* shows at each one:

```
(app Starting)     -> window shows loading state (FR-6)
(app Ready)         -> window loads the real UI at the loopback URL
(app Degraded)       -> window shows a degraded banner over the real UI
                         (data that needs storage is unavailable, per
                         architecture-system.md's failure-mode table)
(app Failed)          -> window shows the error state (FR-7), not the real UI
(second instance)      -> no new window; existing window is focused (FR-4)
```

Illegal transitions, restated for the window layer:

- Showing the real UI before the readiness check passes (violates FR-6)
- A second window ever existing simultaneously with the first (violates
  FR-4)

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Go server not ready within timeout | FR-7's bounded wait expires | `atStates` error treatment, naming what failed, with retry | Main process keeps the Go server running (it may still come up), offers a manual retry of the readiness check rather than restarting everything |
| Second instance launched | `app.requestSingleInstanceLock()` returns false | Existing window is focused; no visible error, since nothing went wrong | New process quits immediately, spawns nothing |
| Config file unreadable by the Go server (permissions, deleted early) | Go server fails to start, exits non-zero | Same as `architecture-system.md`'s "PostgreSQL unreachable" failure mode if the file held `DATABASE_URL` — the error should distinguish "can't read my own config" from "can reach config but not the database" for anyone debugging it | Main process surfaces the distinct error rather than collapsing both into one generic message |
| Malformed IPC call from the renderer (finding this spec closes constitution §5's "renderer assumed compromised") | Main-process handler's own validation (FR-1) | Nothing visible if the call was benign-but-wrong (e.g. a stale renderer build); operation simply fails | Handler rejects and logs; never crashes the main process, never executes with unvalidated arguments |
| Renderer attempts navigation to an external origin | `will-navigate` / `setWindowOpenHandler` (FR-3) | The link opens in the OS browser instead | Electron window's own content is unaffected |

## Security considerations

- **Preload as the only bridge (FR-1, FR-2)** — restates constitution §5 as
  concrete Electron configuration, not just policy. A reviewer can check
  `contextIsolation`/`sandbox`/`nodeIntegration` values directly rather than
  trusting a description of intent.
- **Navigation and external-link policy (FR-3, FR-10)** — the renderer is
  assumed compromised; without this, a compromised renderer could navigate
  the whole window to an attacker-controlled origin and phish inside what
  looks like the app's own window.
- **Config/secrets spawn channel (FR-5)** — resolves `architecture-system.md`
  finding 5 concretely. A file with `0600` permissions, short-lived, is
  strictly better than argv (visible via `ps` to any local user) or an
  inherited env var (visible via `/proc/PID/environ` to same-user
  processes) — it's still readable by the same OS user, which is an
  accepted limitation (the threat model here is other *unprivileged* local
  processes and casual inspection, not a same-user attacker who already has
  broader access than this file would grant anyway).
- **Single-instance lock (FR-4)** — not purely a UX nicety: two Go server
  processes against one PostgreSQL database, both assuming they're the only
  writer, is a data-integrity risk `architecture-system.md`'s Open questions
  already flagged. This spec is where it gets resolved, not just named.
- **Orphan prevention on macOS/Windows (FR-8, FR-9)** — same reasoning as
  ADR 0005's Linux mechanism: an orphaned Go server keeps serving HTTP on a
  bound port after the user believes the app is closed, which is a
  loopback-only-but-still-real exposure window if the user's threat model
  includes other local processes or users.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Preload argument validation (FR-1), navigation-policy handlers (FR-3) — pure functions, no Electron runtime needed |
| Integration | Single-instance lock behavior (FR-4), config file lifecycle (FR-5) — needs a real spawned process, not a mock |
| Contract | N/A — `architecture-contracts.md` |
| E2E | Full lifecycle walkthrough (below). **Correction from this spec's first draft**: the Playwright *library* supports launching Electron apps directly (`_electron`), but the **Playwright MCP server actually available in this environment does not** — its exposed tools (`browser_navigate`, `browser_click`, etc.) are browser-page automation only, no Electron launcher, checked directly against its tool list. So: the *web UI itself* (once loaded in any browser context, including the one inside Electron's `BrowserWindow`) can be driven and snapshotted via this MCP, but the *lifecycle walkthrough* (spawn → loading → ready → degraded → error, across process boundaries) needs Playwright's own `@playwright/test` Electron support running as a real test suite in phase 05's CI, not this MCP. Two different tools for two different layers — decide the CI-side one in `architecture-testing.md`, not assumed here |
| Accessibility | Loading/error states' keyboard and screen-reader behavior — the *rendered content* can be checked via the Playwright MCP's accessibility snapshot once something is on screen; getting Electron to that point for the test is `architecture-testing.md`'s CI-harness job, not this MCP's |

- **Walkthrough** — cold start with PostgreSQL unreachable: window shows
  loading, then error (`atStates`), retry succeeds once PostgreSQL is
  available, window loads the real UI. This is the same "open a book"
  slice `architecture-system.md` traced, extended to include what the
  window actually shows at each step.
- **Hostile walkthrough** — a compromised renderer attempts
  `window.open('https://evil.example')` (blocked by FR-3/FR-10) and calls a
  preload method with an oversized/malformed argument (blocked by FR-1).

## Acceptance criteria

- [ ] All four items `architecture-system.md` left open for this spec
      (macOS/Windows orphan prevention, second-instance, config/secrets
      channel, control-plane mechanism) are closed here with a concrete
      decision, not re-deferred
- [ ] Loading and error states cite the design reference's `atStates`
      screen, not invented copy
- [ ] Every FR maps to an exit criterion in phase 05's own document
      (cross-reference, not duplicate)
- [ ] Playwright MCP's actual (verified, not assumed) coverage — web-content
      automation only — handed to `architecture-testing.md` accurately, with
      the CI-side Electron E2E tool decision left to that spec

## Open questions

- **Where does `DATABASE_URL` (FR-5's config file content) come from for a
  packaged end-user install?** Checked the full design reference for this:
  zero screens, in First-run or Settings, configure a database connection —
  `atFirstRun`'s "Storage location" and Settings' "Storage" tab are both
  file storage (book files on disk), not PostgreSQL. ADR 0004 decided
  production uses self-hosted PostgreSQL but never decided how it gets onto
  a user's machine — the Supabase CLI dev stack (ADR 0004's addendum) is a
  developer answer, not a shipped one. Most likely resolution: Alexandryn
  bundles and manages its own Postgres process the same way it manages the
  Go server (a third thing this spec's spawn/lifecycle machinery applies
  to), with FR-5's config file generated internally, never user-facing —
  but that's a real architectural decision, not assumed here. Owner:
  whoever picks this up next — plausibly a new ADR, plausibly
  `architecture-persistence.md`.
- **15-second readiness timeout (FR-7)** — a placeholder, same status as
  `architecture-system.md`'s 3-second startup budget and 10-second shutdown
  grace period. Phase 03/05 should replace with a measured number.
- **Who owns building the bundled loading/error asset (FR-6, FR-7)?**
  Resolved architecturally: it's a separate, disk-loaded bundle, not served
  by the Go server (was a real contradiction, not just a scope question —
  see review 0007 finding 6). Package ownership resolved too, by ADR 0008
  (monorepo layout): it lives in `electron/`, since it's Electron-bundled
  by definition. Still genuinely open: how it stays visually consistent
  with `atStates` without depending on `web/`'s React build — token drift
  between two independent implementations of the same design is a real
  risk ADR 0008 didn't solve, only named where the code lives.
- **`atTablet`'s actual surface** — `.design-reference/ANALYSIS.md` flags
  that the Tablet screen is captured inside the host/Admin canvas, not the
  Web canvas ADR 0003 assigned it to. This spec doesn't resolve it; noting
  it here because it's this spec's territory (desktop host) if the answer
  turns out to be "Tablet is actually a host-side reference view," and
  `architecture-frontend.md`'s territory if it's a Web-surface screen
  mis-filed during design work. Needs a decision from whoever owns the
  Claude Design project before either spec treats it as settled.
- **macOS/Windows mechanisms (FR-8, FR-9) remain unverified** — same
  caveat as ADR 0005: named and specified, not tested. Phase 05 needs
  actual hardware or CI runners for both platforms to close this for real.

## Relationship to phase 05's own specs

Phase 05's `desktop-host-process-model.md`, `-ipc-surface.md`, and
`-window-and-serving.md` cover almost exactly this spec's territory by name.
That's intentional layering, not redundancy: this spec fixes the pattern and
the requirements (FR-1 through FR-11); phase 05's specs own the actual
implementation, the measured numbers replacing this spec's placeholders, and
anything this spec left explicitly open (binary location in dev vs.
packaged, schema-vs-hand-maintained preload generation, mid-session crash
restart policy — none of those are decided here). Where this spec's FR-6/
FR-7 directly answers a question phase 05's own roadmap document listed as
its to decide (server-healthy timeout/failure UX), that document has been
updated to say so rather than leaving two specs quietly claiming the same
open question.

## References

- `architecture-system.md` — the four open items this spec closes
- ADR 0005 — process model, prototype-backed (Linux orphan-prevention
  mechanism this spec extends to macOS/Windows)
- ADR 0004 — persistence engine (what's in the FR-5 config file today)
- `.design-reference/ANALYSIS.md` — `atFirstRun`, `atSettings`, `atSystem`,
  `atStates` screens this spec's lifecycle states cite
- Constitution §4 (hostile input), §5 (Electron privilege boundary), §7
  (accessibility), §8 (observability without leakage), §11 (copy)
- `.claude/skills/README.md` — Playwright MCP's actual scope verified here
  (browser automation, not an Electron launcher); its tooling table entry
  ("drive a real browser") already didn't overclaim, checked for consistency
  for this layer
