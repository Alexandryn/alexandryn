# Spec: Desktop host window and serving

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-31, phase 05 Tier 7 / E30 — implemented Tiers 1–6 (PR #68), audited `0005`, window state persistence, CSP-locked boot asset, single-instance lock, loopback origin lock & external link forwarding verified with unit and E2E tests) — was `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `05-desktop-host` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-31 |

| **Supersedes** | — |
| **Reviewed in** | `0032` (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`architecture-desktop-host.md` already fixed: `contextIsolation`/`sandbox`
on (FR-2), navigation locked to the Go server's own origin with external
links routed to the OS browser (FR-3, FR-10), the loading/error asset as
a separate disk-loaded bundle never served by the Go server (FR-6/FR-7),
and close-means-quit (FR-11). What it left open, named explicitly in its
own Open questions: how the bundled loading/error asset stays visually
consistent with the real UI's `atStates` treatment without depending on
`web/`'s React build succeeding — a real risk of two independent
implementations of the same design drifting apart silently.

## Problem

Nothing has fixed: the actual `BrowserWindow` construction options
beyond the three security flags, how the loading/error asset's styling
stays synced with `frontend-design-tokens.md`'s extracted tokens without
importing React, the concrete external-link interception implementation,
or native window chrome (title bar style, menu, restoration behavior)
phase 05's own roadmap names as in scope.

## Goals

- Fix `BrowserWindow` construction: security options plus every other
  option (title bar style, size/position restoration, menu)
- Fix the token-sync mechanism between `web/`'s Tailwind config and the
  Electron-bundled loading/error asset, closing `architecture-desktop-
  host.md`'s named open risk
- Fix external-link interception's concrete implementation
- Fix loopback asset loading: the window's `loadURL` call and its
  relationship to `desktop-host-process-model.md`'s readiness signal

## Non-goals

- Process spawning, health polling, crash recovery —
  `desktop-host-process-model.md`
- IPC operation definitions — `desktop-host-ipc-surface.md`
- The real UI's own component implementation — `frontend-*.md` (phase
  04); this spec only owns the small bundled asset shown *before* the
  real UI is reachable, and the window that eventually hosts it
- Packaging/code-signing — phase 99

## User stories

- As **a user**, I want the window to look and feel like a native app
  (correct title bar, remembered size/position), not a bare browser
  frame.
- As **a designer**, I want the loading/error screen to visually match
  the real app's `atStates` treatment exactly, not an approximation that
  drifts every time the design tokens change.
- As **a user who clicks a link to an external site** (in a future
  screen — Discover's source attribution, say), I want it to open in my
  regular browser, never inside the app's own window.

## Functional requirements

- **FR-1** `BrowserWindow` is constructed with: `contextIsolation: true`,
  `sandbox: true`, `nodeIntegration: false` (`architecture-desktop-
  host.md` FR-2, restated here as the literal constructor options this
  spec's own test verifies); `titleBarStyle: 'hiddenInset'` on macOS /
  default native chrome on Windows/Linux (matching the design
  reference's own custom titlebar treatment,
  `roadmap/04-frontend-foundation/README.md`'s shell composition, without
  fighting each platform's own window-manager conventions); a minimum
  size floor (`minWidth`/`minHeight`, values from
  `frontend-design-tokens.md`'s breakpoint tokens once extracted, so the
  native window can never be resized smaller than the shell's own
  responsive layout can meaningfully render) — no separate number
  invented here. No application menu bar on any platform for v1 (matches
  `architecture-desktop-host.md` FR-11's minimal-chrome v1 scope; a menu
  is a new spec if a real need for one emerges).
- **FR-2** Window size and position are persisted across sessions via a
  small JSON file in Electron's own `app.getPath('userData')` directory
  (not `localStorage`/renderer-side storage, which wouldn't survive a
  renderer crash or reload) — written on `resize`/`move` events debounced
  to avoid excessive disk writes (a fixed 500ms debounce, a reasoned
  placeholder), read once at window-creation time to set initial
  bounds. Corrupted or missing state file falls back to a fixed default
  size/centered position, never a crash.
- **FR-3** The loading/error asset (`architecture-desktop-host.md`
  FR-6/FR-7) is a static HTML/CSS bundle in `electron/src/renderer/
  boot/`, loaded via `loadFile` (never `loadURL` against any HTTP
  origin, matching that spec's "loaded from disk, never fetched over
  HTTP" requirement). **Token sync mechanism**: this bundle's CSS
  imports **`tokens.css`** — `frontend-design-tokens.md` FR-2's second
  extraction output, added to that spec specifically for this consumer
  (a cross-phase review caught this spec citing an artifact that didn't
  exist yet at authoring time; now it does) — a plain CSS custom-
  properties file, distinct from the Tailwind-specific theme config
  `web/` consumes directly, both generated from the same extraction
  pass, never hand-copied into this bundle separately. This closes
  `architecture-desktop-host.md`'s named open risk: token drift is now
  structurally prevented (both consumers read the same generated file)
  rather than relying on someone remembering to update two places by
  hand. **Build-time check**: `electron/`'s own build step asserts
  `tokens.css` exists at the expected path before bundling this asset —
  a plain existence check, not a staleness/content-diff check, since
  both consumers regenerate from the same extraction pass within the
  same CI run rather than caching an independently-timed copy, so
  drift-through-staleness isn't structurally possible as long as build
  *order* runs extraction before either consumer builds — the ordering
  requirement this spec's own build step enforces, named concretely
  here rather than left as "not yet designed."
- **FR-4** External-link interception: `webContents.setWindowOpenHandler`
  returns `{ action: 'deny' }` for every `window.open`/`target="_blank"`
  call and instead invokes `shell.openExternal(url)` — but only after
  validating the URL's scheme is `http:`/`https:` (constitution §4:
  `shell.openExternal` on an unvalidated URL can itself be a code-
  execution vector on some platforms for certain URL schemes, e.g.
  `file:`, custom protocol handlers — validated here even though every
  current call site's URL originates from this project's own rendered
  content, not external input, as defense in depth for a mechanism that
  will remain reachable as the app grows). `webContents.on('will-navigate')`
  is similarly intercepted: any navigation target whose origin isn't the
  Go server's own loopback (or, post-phase-13, LAN) address is prevented
  and routed through the same `shell.openExternal` path instead of
  being allowed to navigate the window itself — `architecture-desktop-
  host.md` FR-3/FR-10's requirement, both the Electron-chrome case and
  the web-UI-content case, made concrete as one shared handler rather
  than two separate implementations that could drift.
- **FR-5** Once `desktop-host-process-model.md` FR-3's readiness check
  succeeds, the window's `loadURL` is called against
  `http://127.0.0.1:<port>` (the port `architecture-desktop-host.md`'s
  API and contracts section already fixes as the Go server's own stdout
  announcement) — replacing the loading asset (FR-3) with the real UI in
  one navigation, never a client-side redirect from within the loading
  asset itself (which would mean the loading asset's own JS deciding
  when to navigate, duplicating logic `desktop-host-process-model.md`
  already owns).
- **FR-6** The `Recovering` and `Failed` states
  (`desktop-host-process-model.md` FR-4, added by that spec's own
  post-review revision — a cross-phase review found this entire
  rendering responsibility unowned in this spec's first draft) are
  rendered **without any renderer-side cooperation or IPC channel**:
  `Recovering` is a small, fixed-position banner
  (`webContents.insertCSS` for its styling, importing the same
  `tokens.css` FR-3 uses — visually consistent with the loading/error
  asset by construction, not a third independent implementation — plus
  `webContents.executeJavaScript` injecting one fixed DOM node with the
  banner's markup) laid directly over the already-loaded real UI by the
  main process, shown when `desktop-host-process-model.md` FR-4 begins a
  respawn attempt and removed (`webContents.removeInsertedCSS` plus
  removing the injected node) the instant that spec's readiness gate
  succeeds again. If the respawned process announced a different port
  than the one currently loaded (`desktop-host-process-model.md` FR-4's
  own port re-capture), removing the banner is preceded by a fresh
  `loadURL` against the new port (FR-5's own mechanism, re-invoked) —
  otherwise the banner is simply removed and the existing page, already
  pointed at the correct unchanged port, needs no reload. `Failed`
  (all 3 respawn attempts exhausted) does not use the banner mechanism
  at all — the window instead performs a full `loadFile` back to FR-3's
  error asset, replacing the real UI outright, the same treatment a
  first-start failure already gets, since at that point the app isn't
  meaningfully usable regardless of what's still visually present. This
  design deliberately avoids needing a main→renderer IPC push channel
  (which `desktop-host-ipc-surface.md` doesn't otherwise need to
  provide) or any cooperation from the real UI's own React code — the
  overlay works even if that code is itself the reason the app is in
  trouble.

## Non-functional requirements

- **Performance** — `architecture-desktop-host.md` FR-6's 200ms
  loading-state-visible budget is satisfied structurally: FR-3's asset
  is a static, disk-loaded file with no network dependency, so its
  paint time is bounded by disk I/O and Chromium's own startup, not by
  anything this spec adds.
- **Security** — see Security considerations below.
- **Accessibility** — the loading/error asset (FR-3) carries the same
  keyboard/focus/screen-reader requirements as any other UI state
  (constitution §7, `architecture-desktop-host.md`'s own NFR) — semantic
  HTML, a visible focus ring on the retry action (FR-7 of that spec),
  and an `aria-live` region announcing the loading→error transition,
  consistent with `frontend-accessibility.md`'s FR-3
  `<VisuallyHidden>`/live-region conventions even though this bundle
  doesn't share `web/`'s React component code (FR-3 above) — the
  *conventions* are shared even where the *implementation* isn't. FR-6's
  injected `Recovering` banner carries the same requirement: the
  injected DOM node includes `role="status"` and `aria-live="polite"` so
  its appearance is announced without stealing focus from whatever the
  user was doing in the real UI underneath it.
- **Reliability** — FR-2's corrupted-state-file fallback is this spec's
  own reliability property: a malformed persisted-window-state file
  never prevents the app from opening a window at all.
- **Observability** — window lifecycle events (created, resized,
  external link intercepted, navigation blocked) are logged by the main
  process, extending `architecture-desktop-host.md`'s own Observability
  section rather than a separate log stream.

## Domain model

Not applicable — window chrome and asset serving, not the Alexandryn
domain.

## API and contracts

- **Window ↔ persisted state (FR-2)**: a JSON file, this spec's own
  schema (`{ width, height, x, y }`), read/written only by the main
  process — never renderer-writable.
- **Loading asset ↔ design tokens (FR-3)**: `tokens.css`, shared source
  with `web/`'s Tailwind config, both produced by
  `frontend-design-tokens.md` FR-2's extraction process.
- **Window ↔ Go server (FR-5)**: `http://127.0.0.1:<port>`, loaded
  directly once ready — no desktop-only endpoint, matching
  `architecture-system.md` FR-6.
- **Main process ↔ already-loaded real UI (FR-6)**: `webContents.insertCSS`/
  `executeJavaScript`, a one-directional injection with no renderer
  cooperation or acknowledgment — main process alone decides when the
  banner appears and disappears.

## State transitions

Matches `desktop-host-process-model.md`'s own state diagram, this
spec's own rendering side of each transition, with a concrete mechanism
per state rather than an asserted claim (FR-6 closes the gap an earlier
draft left here): Starting → loading asset (FR-3); Ready → real UI at
loopback URL (FR-5); Recovering → banner injected over the real UI,
already loaded (FR-6); Ready again after Recovering → banner removed,
window reloaded first if the port changed (FR-6); Failed → error asset,
full replace (FR-3, via FR-6's own final branch).

Illegal transitions, restated for this layer:

- `loadURL` against the loopback address (FR-5) called before
  `desktop-host-process-model.md`'s readiness check has actually
  succeeded (violates `architecture-desktop-host.md`'s own "showing the
  real UI before the readiness check passes" prohibition)
- Any `will-navigate`/`window.open` target reaching Electron's own
  window content instead of being intercepted (FR-4)
- The `Recovering` banner (FR-6) removed without first checking whether
  the announced port changed — would leave the window silently pointed
  at a dead connection while claiming to be recovered

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Persisted window-state file is corrupted/unreadable | FR-2's JSON parse fails | Window opens at a default size/position | Fallback applied, no crash, corrupted file overwritten on next successful persist |
| A navigation attempt targets a non-loopback/non-LAN origin | FR-4's `will-navigate` handler | Navigation blocked; if `shell.openExternal`'s own scheme check passes, opens in the OS browser instead | Window's own content is unaffected |
| `shell.openExternal` called with a non-http(s) scheme (a compromised-renderer attempt) | FR-4's scheme validation | Nothing happens — the call is silently declined | Logged as a rejected external-open attempt, not surfaced to the user (this is an attack-shaped event, not a legitimate error the user needs to see) |
| `tokens.css` missing at electron build time (a build-order bug) | FR-3's build-time existence check | Build fails before packaging, never a runtime surprise | The check is a hard build failure, not a warning — a missing token file means the loading asset and `Recovering` banner (FR-6) would both silently fall back to unstyled content |
| `webContents.executeJavaScript` injection (FR-6) fails (page not in a state to accept it — mid-navigation, say) | The injection call's own promise rejects | Brief absence of the `Recovering` banner during a narrow timing window | Logged, retried on the next state-check tick rather than treated as fatal — the underlying respawn sequence (`desktop-host-process-model.md` FR-4) proceeds regardless of whether the banner itself rendered |

## Security considerations

- **`shell.openExternal` scheme validation (FR-4)** — a concrete,
  narrow mitigation against a known Electron footgun (certain URL
  schemes passed to platform-native "open" APIs have historically
  enabled unintended code execution on some OS/version combinations);
  restricting to `http:`/`https:` closes this regardless of whether any
  current call site could actually be reached with a hostile scheme.
- **Navigation lock (FR-4) restates constitution §5's renderer-assumed-
  compromised stance** — even this project's own served content isn't
  trusted to navigate the window arbitrarily; only the loopback/LAN
  origin the Go server serves is ever loaded directly.
- **Persisted window state is main-process-owned only (FR-2)** — a
  renderer, even if compromised, cannot write arbitrary bounds to this
  file, since it never has filesystem access (`nodeIntegration: false`,
  FR-1) or a path to it.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Window-state persistence and fallback (FR-2), URL scheme validation (FR-4) — pure functions, no Electron runtime needed |
| Integration | `BrowserWindow` construction options (FR-1) asserted directly; `will-navigate`/`window.open` interception (FR-4) via Electron's own test harness, real navigation attempts made and confirmed blocked; `loadURL` sequencing (FR-5) against a real (test) spawned Go server |
| Security | Automated check that `contextIsolation`/`sandbox`/`nodeIntegration` are set correctly (shared assertion with `desktop-host-ipc-surface.md`'s own equivalent check, same underlying `BrowserWindow` construction); a hostile-navigation test attempting `file://`, `javascript:`, and cross-origin `http://evil.example` targets, each confirmed blocked |
| Accessibility | Loading/error asset's keyboard and screen-reader behavior, via the Playwright MCP's accessibility-snapshot capability once the asset is rendered in any browser context (`architecture-desktop-host.md`'s own verified-capability note — snapshot inspection is real, launching Electron itself is not this MCP's job); the `Recovering` banner's `role="status"`/`aria-live` attributes (FR-6), same capability |
| Recovering overlay | FR-6, against a real (test) Electron window with real content loaded: the banner is injected on `Recovering` entry and removed on `Ready` return; a same-port respawn removes the banner with no reload; a different-port respawn triggers a `loadURL` before/with banner removal, confirmed by observing the window's actual loaded URL change; the final-failure path performs a full `loadFile` to the error asset, confirmed distinct from the banner-only path |

## Acceptance criteria

- [ ] `BrowserWindow` constructed with the exact security options
      (FR-1), verified by an automated test reading the actual
      construction call, not just documentation
- [ ] External link clicks verified to open in the system browser, never
      in Electron — phase 05's own named exit criterion, proven with a
      real (blocked) navigation attempt in a test
- [ ] A hostile navigation attempt (`file://`, `javascript:`, external
      origin) is blocked, proven per scheme/case
- [ ] The loading asset and `web/`'s real UI both resolve their color
      tokens from `tokens.css`, proven by the build-time existence check
      (FR-3) actually failing the build when the file is absent, not by
      visual comparison alone
- [ ] The `Recovering` banner appears on respawn and disappears on
      recovery, including the port-change reload case, proven end to
      end against a real test Electron window
- [ ] Window size/position persists across a simulated restart, and
      recovers to a default on a corrupted state file
- [ ] Every FR maps to an exit criterion in phase 05's own document

## Open questions

- **FR-2's 500ms debounce and default window size** — reasoned
  placeholders, not measured against real usage.
- **`atTablet`'s actual surface** — inherited from `architecture-desktop-
  host.md`'s own Open questions; if resolved as a host-side reference
  view, this spec (not `frontend-shell-and-routing.md`) would own
  rendering it, a scope addition not designed here.

## References

- `architecture-desktop-host.md` FR-2, FR-3, FR-6, FR-7, FR-10, FR-11 —
  every pattern this spec implements concretely, including its own
  named open risk (token drift) this spec's FR-3 resolves
- `frontend-design-tokens.md` FR-2 — the extraction process this spec's
  FR-3 shares a source with
- `desktop-host-process-model.md` FR-3/FR-4 — the readiness signal FR-5
  depends on, and the `Recovering`/`Failed` states FR-6 renders (that
  spec decides *when*, this spec decides *how*)
- `architecture-system.md` FR-6 — same-origin loading requirement
- Constitution §4 (hostile input — FR-4's scheme validation), §5
  (privilege boundary), §7 (accessibility)
