# Spec: Frontend architecture

| | |
|---|---|
| **Status** | `APPROVED` (maintainer confirmed 2026-08-20) |
| **Phase** | `01-architecture` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`.claude/reviews/0012-spec-architecture-frontend.md`](../reviews/0012-spec-architecture-frontend.md) — Approved with changes, both findings fixed; self-reviewed, independent read still pending |

## Context

The design reference is now mostly complete (four of five canvases,
`.design-reference/ANALYSIS.md`) and uses a state-driven internal
router (`s.route === 'library'`) as a design-tool convenience — CLAUDE.md is
explicit that the prototype is authoritative for visual intent, not code
structure, and this spec is where that line matters most: a real app needs
bookmarkable URLs, browser back/forward, and shareable links to a specific
book, none of which a design canvas's internal state needed.

`architecture-system.md` FR-6 requires the web UI served to the Electron
window and to a LAN device be the *same build artifact* — which raises a
question no prior spec answered: if it's the same bundle either way, how
does the UI know whether to show host-only screens (Settings, System,
Sources configuration, Import) versus the LAN-viewer's restricted set
(ADR 0003's surface split)? It cannot be a client-side check (the renderer
doesn't get to decide its own privilege level, constitution §5's spirit
applied to capability, not just IPC) — it has to be server-told.

## Problem

Nothing has decided: URL structure and routing library, state-management
approach for server data versus UI-local state, design-token extraction
from the design reference into Tailwind, or — the one genuinely open
question — the mechanism by which one shared bundle shows different
capabilities to a host window versus a LAN client.

## Goals

- Real URL-based routing (browser history, deep-linkable), not the
  prototype's internal state-variable routing
- Fix the state-management split: server data (fetched from the API) vs.
  UI-local state, and pick tooling for each, justified under constitution
  §9
- Fix how host-only vs. viewer-only capability is determined — server-told,
  never client-inferred, with a concrete placeholder shape for the
  not-yet-existing phase 12/13 gate
- Define design-token extraction from the design reference into Tailwind's
  theme config, so implementation doesn't re-derive tokens from raw hex
  values scattered through `.dc.html` files by hand
- Fix accessibility as a structural requirement (constitution §7), not a
  later pass

## Non-goals

- The actual component library / design system catalog — extracted from
  the design reference during phase 04 implementation, not enumerated here
- Any specific screen's behavior (library browse, book detail, etc.) —
  phase 06 onward
- The real phase 12/13 capability-gating logic — this spec reserves the
  *shape*, phase 12/13 implements the actual gate
- Local dev's cross-port CORS/proxy setup — flagged as an open question in
  `architecture-contracts.md`, owned here but not resolved in this draft
- Native mobile apps — the Mobile canvas in the design reference is a
  responsive *web* layout (`.design-reference/ANALYSIS.md`: "single
  responsive layout driven by a tab bar"), not a separate native app; no
  non-goal needed beyond noting this isn't ambiguous

## User stories

- As **a user**, I want to bookmark a specific book's page or share it with
  another device on the LAN, which requires a real URL, not an in-memory
  route variable.
- As **phase 12/13**, I want the frontend already structured around a
  server-told capability flag, so adding real authentication doesn't
  require restructuring how every host-only screen decides to render.
- As **a keyboard or screen-reader user**, I want every screen usable
  without a mouse from the day it ships, not retrofitted later.

## Functional requirements

- **FR-1** Routing MUST use real browser history (a client-side router
  library, e.g. React Router — final choice justified under constitution
  §9 during phase 04 implementation, not fixed here), with URLs mirroring
  the design reference's screens (`/library`, `/book/:id`,
  `/collections`, `/discover`, `/sources`, etc. for the host surface;
  `/access`, `/connect`, `/reader/:id`, etc. for the viewer surface). The
  prototype's `s.route==='x'` pattern MUST NOT be ported as-is — it was a
  design-tool convenience, not a routing architecture (CLAUDE.md).
- **FR-2** Server-derived data (anything from the API) MUST be managed by a
  dedicated data-fetching/caching layer (e.g. TanStack Query — final
  choice justified under constitution §9 during phase 04), never fetched
  ad hoc in component `useEffect` calls with hand-rolled caching. UI-local
  state (form inputs, open/closed panels, the current theme toggle) MUST
  use React's own state primitives — no global state library (Redux or
  similar) unless a specific, named cross-cutting need justifies one later;
  none is assumed here.
- **FR-3** Which capabilities render (host-only: Settings, System, Sources
  configuration, Import; versus viewer-only: the restricted read/access
  surface) MUST be determined by a value the Go server provides — not by
  any client-side inference (which port was used, whether the app detects
  it's inside Electron, the request's own origin). Until phase 12/13
  exist, every connection is loopback-only (constitution §6) and therefore
  trusted at the same level as Electron's own renderer — including a plain
  browser tab opened to `127.0.0.1:<port>` directly, which is not
  "Electron" but is equally the host at the network level. Every
  capability renders unconditionally until then — this FR reserves the
  shape for a future server-told capability field, it does not build the
  gate.
- **FR-4** Design tokens (colors, spacing, typography — the CSS custom
  properties already present throughout the design reference's `.dc.html`
  files, e.g. `--bg`, `--sf`, `--tx`, `--ac`) MUST be extracted once into
  Tailwind's theme configuration, not re-derived ad hoc per component from
  raw hex values. The `frontend-design` skill (already available,
  `skills/README.md`) is the tool for this extraction work during phase 04.
- **FR-5** Every interactive element MUST be reachable by keyboard, have a
  visible focus state, and an accessible name — constitution §7 as a
  structural requirement checked per component, not a conformance pass
  applied at the end of phase 04.
- **FR-6** Loading, empty, and error states for any data-fetching component
  MUST use the design reference's `atStates` treatment (now fully
  captured, `.design-reference/ANALYSIS.md`) — this is the same visual
  language `architecture-desktop-host.md` FR-6/FR-7 already committed the
  Electron-bundled splash asset to matching; this FR is where the *real*
  app's own loading/error states (for a slow API call, not a not-yet-ready
  Go server) use the identical design language, kept in sync because both
  ultimately derive from the same design-reference source, not because
  they share a build target (`architecture-desktop-host.md` FR-6 already
  established the bundled splash is deliberately independent of the
  frontend's build succeeding).
- **FR-7** The frontend MUST be purely client-side rendered — a static
  build (HTML shell, JS, CSS) served by the Go server's own file-serving
  capability, no server-side rendering. This isn't a style preference: the
  Go server cannot run React SSR without embedding or spawning a Node
  runtime, and this project's process model
  (`architecture-system.md` FR-1, already three processes, four on macOS)
  has no room for a Node runtime as a fifth. CSR is the only shape
  consistent with what's already been decided elsewhere.

## Non-functional requirements

- **Performance** — not budgeted here; no component exists yet to measure.
  Phase 04 sets real numbers once there's something to profile.
- **Security** — see Security considerations below.
- **Accessibility** — FR-5 is the requirement; constitution §7 is the bar.
- **Reliability** — FR-6's loading/error state requirement means a slow or
  failed API call is a designed state, not an unhandled promise rejection.
- **Observability** — API errors reaching the frontend carry a correlation
  ID (`architecture-contracts.md` FR-5); this spec requires that ID be
  visible somewhere in the error state's UI (even if small/secondary), not
  just logged where a user reporting a bug can't reference it.

## Domain model

Not applicable — this spec is UI architecture, not the Alexandryn domain
(constitution §3). It does require that domain-shaped data arriving from
the API (via `architecture-contracts.md`'s contract) stays in FR-2's
data-fetching layer's cache shape, not duplicated into ad hoc component
state that can drift from the server's version of the truth.

## API and contracts

- **Frontend ↔ Go server**: `architecture-contracts.md`'s OpenAPI contract,
  consumed through FR-2's data-fetching layer exclusively — no component
  talks to `fetch` directly for server data.
- **Frontend ↔ Electron main**: the preload-exposed enumerated surface
  (`architecture-desktop-host.md` FR-1) for anything that isn't an HTTP
  call — native file dialogs, if any exist in host-only screens.
- **Capability signal (FR-3)**: shape reserved, not designed — plausibly a
  field on whatever session/bootstrap endpoint phase 12 introduces. Not
  invented here ahead of that phase's actual authentication design.

## State transitions

Not applicable at the application-lifecycle level (`architecture-system.md`
owns that). At the routing level: navigating to a host-only URL as a
viewer-capability client (post-phase-13) MUST redirect or show an
explicit "not available" state — never a broken or partially-rendered
host screen. Illegal: rendering a host-only component before FR-3's
capability value has actually been read from the server (a flash of
host-only content that then disappears is worse than a brief loading
state first).

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| API request fails (network, 5xx) | FR-2's data-fetching layer's error state | `atStates` error treatment, correlation ID visible, retry option | Cached data (if any) stays visible where stale-but-useful, per FR-2's tooling's own stale-while-revalidate behavior — not blanked on every transient failure |
| Capability value (FR-3) hasn't loaded yet | Same data-fetching layer, applied to this specific value | Loading state, not host-only content flashing then disappearing | Host-only routes/components wait for the capability value before rendering, never render optimistically |
| Navigation to a URL for a screen that doesn't exist yet (most of phase 06+) | Router's own not-found handling | A real 404-equivalent state, not a blank page | Standard router behavior, explicitly required rather than left as whatever the router defaults to |

## Security considerations

- **Capability gating is server-told (FR-3)** — the load-bearing security
  property of this spec. A client-side "am I the host" check is
  trivially spoofable by anyone who can open devtools; this FR exists
  specifically so that when phase 12/13 land, the enforcement point is the
  server (which already validates every request per
  `architecture-system.md`'s trust boundaries), and the frontend's
  rendering logic is a UX convenience on top of a real server-side check,
  never the check itself.
- **No secrets in frontend code or bundle** — inherited from constitution
  §8/§4 generally; stated here because a data-fetching cache (FR-2) is
  exactly the kind of place a credential could accidentally linger in
  memory or a dev-tools-visible cache longer than necessary once phase 12
  exists. Not designed in detail here — flagged for phase 12 to actually
  address when there's a credential to protect.
- **XSS via metadata or book content** — CLAUDE.md's own standing warning
  ("Letting an Open Library response shape leak past its adapter") applies
  at the frontend too: anything rendered from metadata or EPUB content
  (phase 07, phase 11) must be treated as untrusted at render time, not
  just at the API boundary. Named here as a cross-cutting concern this
  spec's component architecture must not make harder to enforce later.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Component rendering logic, FR-2's data-fetching hooks in isolation |
| Integration | Routing (FR-1) end to end, capability-gating (FR-3) with a mocked server value |
| Contract | Consumes `architecture-contracts.md`'s contract test indirectly — a frontend built against a stale contract is what that test prevents |
| E2E | Real browser automation — the Playwright MCP already available in this environment genuinely applies here (confirmed capability, unlike the Electron-launcher claim `architecture-desktop-host.md`'s self-review corrected: this is exactly the browser-page automation it actually supports) |
| Accessibility | Playwright MCP's accessibility snapshot capability, applied to FR-5's keyboard/focus/naming requirements directly — same confirmed-capability note |

## Acceptance criteria

- [ ] Real URL routing implemented, no `s.route===` pattern ported from the
      prototype
- [ ] Capability gating (FR-3) proven with a test that withholds the
      capability value and confirms host-only content doesn't flash before
      disappearing
- [ ] Design tokens extracted into Tailwind config, traceable back to the
      design reference's CSS custom properties
- [ ] A keyboard-only pass (no mouse) completes the "open a book" walkthrough
      `architecture-system.md` already established as this project's
      reference slice

## Open questions

- **Router library choice** — React Router is the reasonable default,
  final choice and constitution §9 justification belongs to phase 04
  implementation, not fixed here.
- **Data-fetching library choice** — same status, TanStack Query is the
  reasonable default, not fixed here.
- **`atTablet`'s actual surface** — still unresolved from
  `architecture-desktop-host.md`'s Open questions (captured inside the
  host/Admin canvas, not the Web canvas ADR 0003 assigned it to). This
  spec inherits the same unresolved question; still needs the Claude
  Design project owner's confirmation before either spec treats it as
  settled.
- **Local dev CORS/proxy** — named as this spec's to resolve by
  `architecture-contracts.md`'s own Open questions; not resolved in this
  draft either. Real gap, needs phase 04 implementation planning to close.

## References

- CLAUDE.md — "The design reference is truncated" section (now updated),
  "prototype is authoritative for visual intent, not code structure"
- `.design-reference/ANALYSIS.md` — current canvas completeness, the
  `atTablet` open question
- ADR 0003 — design canvas split, host vs. viewer surface boundary
- `architecture-system.md` FR-6 — same build artifact for both surfaces,
  the reason FR-3 here can't be a client-side check
- `architecture-contracts.md` — FR-5 error shape, the local-dev CORS open
  question this spec inherits
- `architecture-desktop-host.md` FR-6/FR-7 — the bundled splash asset this
  spec's FR-6 stays visually consistent with, deliberately independent
  build targets
- `.claude/skills/README.md` — `frontend-design` skill (FR-4), Playwright
  MCP (confirmed real capability at this layer, Test strategy)
- Constitution §3 (domain boundaries), §5 (privilege boundary, applied to
  capability), §7 (accessibility), §8 (no logging what's read), §9
  (dependencies)
