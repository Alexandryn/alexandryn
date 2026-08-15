# Phase 05 — Desktop host

| | |
|---|---|
| **Status** | Specs approved, implementation not started |
| **Depends on** | Phase 03, Phase 04 |
| **Blocks** | 12 |
| **Opened** | — |
| **Closed** | — |

## Objective

An Electron application that starts the Go server, serves the phase 04
frontend to its own window over loopback, exposes a minimal enumerated IPC
surface, and shuts everything down cleanly — with no network exposure beyond
loopback.

## Why here

It needs a real server to manage (phase 03) and a real UI to serve (phase
04), so it waits for both. It sits before authentication and network access
deliberately: this is where loopback binding becomes physically true, not
just a config default, and every phase after it inherits that constraint
until phase 12 gives it a reason to change.

## Scope

**In**

- Electron main process: lifecycle, single-instance lock, crash handling
- Spawning, health-checking, and gracefully stopping the phase 03 Go binary
- Preload script exposing an explicit, enumerated, typed set of IPC
  operations — never a general `invoke(channel, ...args)` passthrough
- Argument validation for every IPC operation, enforced in the main process,
  not trusted from the renderer
- Window creation and restoration, native window controls, menu (if any)
- Serving the frontend's static assets to the window over loopback HTTP,
  identically to how phase 13 will eventually serve it to the LAN
- External link interception: anything not an internal route opens in the
  system's default browser, never inside Electron
- Context isolation and sandboxing enabled, with a test proving both are on
- Integration tests across the main/preload/renderer boundary

**Out**

- Binding to any non-loopback interface — physically absent until phase 13,
  which itself waits on phase 12's authentication
- Auto-update and production installer packaging — phase 99

## Specifications

| Spec | Covers |
|---|---|
| `desktop-host-process-model.md` | Main process lifecycle, Go server spawn/health/shutdown, crash recovery |
| `desktop-host-ipc-surface.md` | Enumerated preload API, argument validation, what the renderer is never trusted to send |
| `desktop-host-window-and-serving.md` | Window management, loopback asset serving, external link handling |

## Architecture decisions expected

- How the Go binary is located and launched in dev versus a packaged build
- What "healthy" means for the Go server before the window is shown —
  **pattern decided** in phase 01's `architecture-desktop-host.md`
  (FR-6/FR-7): poll `/health`, show a loading state citing the design
  reference's `atStates` screen, error state with retry after a bounded
  (placeholder, unmeasured) timeout. This phase's
  `desktop-host-process-model.md` and `desktop-host-window-and-serving.md`
  still own the actual implementation and a measured timeout number — the
  phase 01 spec set the pattern, not the code
- Whether the preload surface is generated from a schema or hand-maintained,
  and how a new operation gets reviewed before it's addable at all —
  phase 01's spec fixed the *shape* (one namespaced object, every method
  validated in main) but left this question open too
- Restart behaviour if the Go server crashes mid-session — not addressed by
  phase 01's spec (which covers the *first* start, not mid-session crash
  recovery); still fully this phase's to decide

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Preload surface grows into a general-purpose bridge because one feature needed "just one more" passthrough channel | Medium | High — defeats the entire sandboxing model, Constitution §5 | Each new IPC operation requires its own reviewed entry in `desktop-host-ipc-surface.md`; no wildcard channels, enforced by lint |
| Go server binding to a non-loopback address by mistake in a packaged build | Low | Critical — unauthenticated network exposure | Startup asserts the bind address and refuses to start otherwise; a test proves it |
| Renderer treated as trusted because "it's our own UI" | Medium | High | Constitution §5: renderer is assumed compromised regardless of who wrote it; every IPC argument is validated in main |
| Crash-and-relaunch loop if the Go binary fails to start repeatedly | Low | Medium | Backoff and a visible failure state in the window, not a silent retry loop |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Argument validation for every IPC operation, including malformed and adversarial inputs |
| Integration | Main/preload/renderer round trip for each enumerated operation; Go server spawn/health/shutdown under normal and failure conditions |
| Security | Automated check that `contextIsolation` and `sandbox` are on in every window; a check that no channel accepts an arbitrary string as an operation name |

The hardest thing to test here is the shutdown path when the Go server is
mid-request — inherited from phase 03's own hard case, now with an added
question of what the window shows while it waits.

## Security considerations

This is one of the three trust boundaries named in phase 01
(`architecture-system.md`): renderer versus main process. Concretely:

- The preload surface is the entire trust boundary. It is enumerated,
  reviewed, and every operation validates its own arguments — never "the
  renderer already checked this"
- `contextIsolation: true` and `sandbox: true`, with `nodeIntegration` off,
  verified by an automated test rather than left as a configuration anyone
  could quietly revert
- External navigation and `window.open` are intercepted; nothing renders
  inside Electron except this app's own served assets
- The loopback HTTP server serving frontend assets is not yet
  authenticated, because there is no non-loopback path to it — that
  assumption is exactly what phase 13 has to revisit, deliberately

## Observability

Startup logs (via phase 03's logger, running in the spawned process) show
the Go server's health transitions. The Electron main process logs its own
lifecycle events — window created, server spawned, server became healthy,
shutdown initiated — locally, not sent anywhere.

## Exit criteria

- [ ] All three specifications `APPROVED` with recorded reviews
- [ ] Electron launches the Go backend automatically and waits for health before showing the window
- [ ] Preload API is fully enumerated; a test asserts no wildcard/passthrough channel exists
- [ ] `contextIsolation` and `sandbox` verified on by automated test
- [ ] External link clicks verified to open in the system browser, never in Electron
- [ ] App shuts down the backend process cleanly on exit, including mid-request
- [ ] All specs in this phase are `VERIFIED`
- [ ] Security audit recorded in `.claude/audits/` with no open Critical or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
