# Spec: Frontend shell and routing

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-28, phase 04 Tier 6 / F27 — implemented Tier 4 (PR #63), audited `0004`, acceptance criteria walked in [`roadmap/04`](../roadmap/04-frontend-foundation/README.md#spec-verification); FR-6 tier-(b) fixtures carry `TODO(phase-06)`, `atTablet` shell question still open) — was `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `04-frontend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | `0031` (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`architecture-frontend.md` fixed real URL routing (FR-1, React Router as
the reasonable default, final choice belongs here), a dedicated
data-fetching layer (FR-2, TanStack Query as the reasonable default),
and server-told capability gating (FR-3, shape reserved, gate not built
until phase 12/13). `roadmap/04-frontend-foundation/README.md` names the
shell itself (sidebar, titlebar, content pane, mobile tab bar, responsive
breakpoints) and the mock/contract-stub strategy for the data layer as
open decisions. This spec makes all of it concrete.

## Problem

Nothing has fixed: the router library (final choice), the data-fetching
library (final choice), the shell's actual layout composition per
breakpoint, how a route decides host-only vs. viewer-only rendering
concretely (FR-3's reserved shape, given real structure), or the
mock/stub strategy standing in for phase 03's real backend until phase
06.

## Goals

- Finalize router and data-fetching library choices, justified under
  constitution §9
- Fix the shell's layout composition: sidebar/titlebar/content pane
  (desktop), tab bar (mobile), the responsive breakpoints between them
- Give FR-3's capability-gating shape a concrete implementation: a
  context/hook every host-only route checks, defaulting to "loading"
  until a value arrives
- Fix the mock/contract-stub strategy: what stands in for phase 03's API
  until phase 06 wires the real one

## Non-goals

- Any specific screen's actual content — phase 06 onward builds inside
  this shell
- The real capability value's source — phase 12/13's own concern; this
  spec only fixes the shape every host-only route already checks
  against, per `architecture-frontend.md` FR-3
- Phase 05's IPC surface — this spec's shell runs identically whether
  served to Electron's window or a LAN browser tab (`architecture-system.md`
  FR-6's same-build-artifact requirement); Electron-specific concerns
  are phase 05's

## User stories

- As **a user**, I want the shell (navigation, layout) to feel identical
  and instant across every screen, since it's not re-rendered per
  route, only the content pane changes.
- As **phase 06's first real screen**, I want to drop into an existing
  shell with routing, data-fetching, and capability-gating already
  working against mock data, so building the screen means writing its
  content, not re-solving app scaffolding.
- As **a mobile/tablet user** (once phase 13 permits LAN access), I want
  the same shell to reflow into the tab-bar layout the design reference
  specifies, not a degraded desktop layout squeezed into a small screen.

## Functional requirements

- **FR-1** Routing uses **React Router** (v7, data-router mode, pinned
  major version in `package.json`) — the de facto standard for CSR
  React apps, with built-in loader/action patterns that compose
  naturally with FR-2's data-fetching choice. Justified under
  constitution §9: widely used, actively maintained, and the
  alternative (hand-rolling history-API routing) would be reinventing a
  solved problem for no project-specific reason. If abandoned, exit
  cost is moderate: routes are declared data, not scattered imperative
  calls, so a migration means rewriting the route-tree declaration and
  loader wiring, not every component that navigates. Routes mirror
  `architecture-frontend.md` FR-1's named URL structure exactly
  (`/library`, `/book/:id`, `/collections`, `/discover`, `/sources`,
  `/access`, `/connect`, `/reader/:id`, etc.) — one route tree, with
  host-only and viewer-only routes both registered, gated per FR-4
  below, not two separate route trees requiring two builds
  (`architecture-system.md` FR-6's same-artifact requirement).
- **FR-2** Data-fetching uses **TanStack Query** (v5, pinned major
  version) — stale-while-revalidate caching out of the box (what
  `architecture-frontend.md` FR-2's Failure modes table requires:
  cached data stays visible where stale-but-useful, not blanked on
  every transient failure), and the de facto standard pairing with
  React Router's data-router mode. Justified under constitution §9 the
  same way as FR-1: solving a solved problem (cache invalidation,
  request deduplication, retry logic) by hand would be strictly worse
  for no project-specific reason. If abandoned, exit cost is contained
  to the data-fetching hooks themselves (a well-defined seam, not
  spread through component bodies, since FR-2 already requires no
  component call `fetch` directly). Every server-derived value flows
  through a TanStack Query hook; no component calls `fetch` directly
  (restates `architecture-frontend.md` FR-2's own requirement, now with
  a real library backing it).
- **FR-3** Shell composition: a persistent `<Sidebar>` (navigation,
  primary sections) and `<Titlebar>` (current screen title, primary
  actions) frame a `<ContentPane>` that swaps per route — the shell
  itself never remounts on navigation, only `<ContentPane>`'s children
  change, per React Router's nested-route layout mechanism. Below a
  defined breakpoint (`frontend-design-tokens.md`'s spacing/breakpoint
  tokens, once extracted), the shell reflows to a `<MobileTabBar>`
  (library/discover/collections/more, per
  `roadmap/04-frontend-foundation/README.md`'s mobile spec) replacing
  the sidebar entirely, matching `.design-reference/ANALYSIS.md`'s
  confirmed single-responsive-layout Mobile canvas.
- **FR-4** Capability gating (`architecture-frontend.md` FR-3's reserved
  shape, given real structure): a `useCapability()` hook backed by a
  React context, populated by a TanStack Query call against whatever
  bootstrap endpoint eventually exists (phase 12's concern) — until
  then, populated by this phase's mock/stub layer (FR-6) returning "all
  capabilities granted" unconditionally, matching
  `architecture-frontend.md` FR-3's own current-state rule ("every
  connection is loopback-only and therefore trusted... every capability
  renders unconditionally until [phase 12/13]"). Every host-only route
  wraps its content in a check against this hook's value; while the
  value is loading (never instant, even against a mock, to keep the
  loading-state code path real and tested), the route renders a loading
  state, never the host-only content optimistically
  (`architecture-frontend.md`'s own Illegal-transitions rule, State
  transitions section).
- **FR-5** Not-found handling: React Router's own `errorElement`/catch-all
  route renders a real "this page doesn't exist" state
  (`frontend-component-primitives.md`'s primitives, composed into a
  dedicated `<NotFound>` view), never a blank page or the router's
  undecorated default — `architecture-frontend.md`'s own Failure modes
  table requirement, now wired to a concrete component.
- **FR-6** The mock/contract-stub strategy is **MSW (Mock Service
  Worker)** intercepting requests at the network layer, pinned major
  version. `architecture-contracts.md`'s own Acceptance criteria state
  `api/openapi.yaml` "initially contains only the health endpoint" —
  library, book, collections, discover, sources, import, and activity
  endpoints are phase 06's own addition to that contract, not
  available at phase 04 implementation time. FR-6's fixture strategy
  therefore has two tiers, not one: **(a)** for any endpoint the
  contract already defines (health, and any endpoint phase 06 adds
  before phase 04's own implementation catches up, if ordering allows),
  fixtures are generated from `api/openapi.yaml` directly, never
  hand-written; **(b)** for every endpoint phase 04's shell needs that
  the contract doesn't cover yet, fixtures are hand-written, but MUST
  follow the same response-shape conventions the contract already fixes
  generally (`architecture-contracts.md` FR-5's error shape:
  `code`/`message`/`correlationId`), and MUST be marked with a
  `// TODO(phase-06): replace with contract-generated fixture` comment
  — a grep-able marker, not just a verbal intention, so phase 06's own
  work has a concrete checklist rather than having to rediscover every
  hand-written stub by reading the whole codebase. This is honest about
  what FR-6 can actually deliver at phase 04's point in the dependency
  graph, rather than claiming a drift-proof mechanism this phase can't
  fully have yet.
- **FR-7** Every error state a `atStates`-treated component renders
  (FR-5's `<NotFound>`, and any TanStack Query error state per FR-2)
  displays the error object's `correlationId` field (small, secondary
  placement, matching `architecture-frontend.md`'s own "even if
  small/secondary" wording) whenever the error response carries one —
  including from a mock (FR-6's fixtures include a synthetic
  correlation ID in every error fixture, so this code path is real and
  tested from phase 04, not deferred to phase 06 despite the real ID
  only existing once phase 03 is wired in).
- **FR-8** Empty states (`architecture-frontend.md` FR-6's third named
  state, alongside loading and error — `.design-reference/ANALYSIS.md`'s
  `atStates` screen covers all three) use a dedicated `<EmptyState>`
  component (`frontend-component-primitives.md`'s primitive set,
  composed from existing primitives — an icon/illustration slot, a
  message, an optional action button — not a net-new complex primitive)
  wherever a data-fetching component's successful response contains a
  genuinely empty result (an empty library, an empty search result),
  distinguished from the loading and error states already covered by
  FR-2/FR-7 — never a blank content pane silently doing nothing.

## Non-functional requirements

- **Performance** — the shell itself (FR-3) contributes to
  `frontend-tooling.md`'s bundle budget; FR-1/FR-2's libraries are
  already accounted for in that same budget, not a separate one.
- **Security** — see Security considerations below.
- **Accessibility** — the shell's own landmarks (`<nav>` for sidebar,
  `<header>` for titlebar, `<main>` for content pane) are semantic HTML
  from the start, not `<div>`s with ARIA roles bolted on — constitution
  §7 and `architecture-frontend.md` FR-5's structural requirement,
  satisfied here directly (no specific FR in `frontend-accessibility.md`
  states a landmark rule; that spec's own cross-cutting rules apply on
  top of this, not in place of it).
- **Reliability** — FR-2's stale-while-revalidate behavior is what
  makes a transient API failure (once phase 06 wires the real backend)
  non-catastrophic for the user; already true against the mock layer
  (FR-6), since MSW can simulate a failure the same way a real API
  would.
- **Observability** — `architecture-frontend.md`'s own Observability
  requirement (a correlation ID visible in error states) is satisfied
  by FR-7 below, not implicitly by FR-2 alone — FR-2 fixes caching
  behavior, not error-UI content, and asserting otherwise was a defect
  this batch's own cross-spec review caught (an NFR claiming something
  its cited FR doesn't actually support, the same pattern review `0022`
  flagged for phase 03).

## Domain model

Not applicable — shell and routing, not the Alexandryn domain. FR-6's
mock fixtures are generated from the contract's schema, which does
reflect domain shapes, but this spec doesn't define or interpret them.

## API and contracts

- **Frontend ↔ mock backend (FR-6)**: MSW intercepting at the fetch
  layer, fixtures from `api/openapi.yaml` — the same contract phase 06
  switches to consuming for real, with no client-code change beyond
  removing the MSW interception layer.
- **Route ↔ capability hook (FR-4)**: every host-only route reads
  `useCapability()`, never infers its own privilege level — restates
  `architecture-frontend.md` FR-3's server-told requirement as the
  concrete per-route contract.

## State transitions

- Capability value: `loading` → `granted` (all capabilities, until
  phase 12/13) — no `denied` state is reachable yet, since phase 12/13
  haven't introduced a reason one would occur; this spec's `useCapability()`
  hook is shaped to support one later without a call-site change.
- Route navigation: standard React Router client-side transitions — no
  full page reload on any in-app navigation, `architecture-frontend.md`
  FR-1's bookmarkable-URL requirement combined with an SPA's expected
  navigation behavior.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Mock API returns an error (FR-6 simulating a real failure) | TanStack Query's error state | `atStates` error treatment (`architecture-frontend.md` FR-6), retry option | Cached data (if any) stays visible per FR-2's stale-while-revalidate behavior |
| Capability value never resolves (a stuck mock, or a real future timeout) | `useCapability()`'s own loading state persists | Loading state indefinitely — no fallback timeout is designed here | Named as a residual gap; a real timeout/retry policy is phase 12's concern once there's a real endpoint to time out against |
| Navigation to an unregistered route | FR-5 | `<NotFound>` component | React Router's catch-all route, never a blank page |
| MSW bundle present in a production build | `frontend-tooling.md`'s build-time check (Security considerations) | N/A — caught in CI before shipping | Build fails |
| A hand-written fixture (FR-6 tier b) is never replaced once phase 06 adds the real contract endpoint | The `TODO(phase-06)` grep marker (FR-6) | N/A — a phase 06 process gap, not a phase 04 runtime failure | Named as a residual risk this spec's marker mitigates but phase 06 must actually act on |

## Security considerations

- **Capability gating is server-told, never client-inferred (FR-4)** —
  direct restatement of `architecture-frontend.md`'s own load-bearing
  security property; this spec's contribution is making the hook real
  rather than leaving it a documented intention.
- **Mock/stub boundary excluded from production** — the actual build
  check lives in `frontend-tooling.md`'s Security considerations (its
  build pipeline produces the artifact being checked); this spec is
  the reason that check exists — FR-6's MSW dependency is real and
  necessary for development but must never reach a shipped build,
  restating phase 04's own named risk table entry.
- **No secrets in the bundle** — inherited from `frontend-tooling.md`'s
  own equivalent check; the shell and routing layer introduces no new
  secret-shaped surface.
- **MSW dependency abandonment** — if MSW were abandoned, exit cost is
  low: it operates entirely at the network-interception boundary
  (FR-6), never touching component code, so a replacement mocking tool
  would mean rewriting fixture-serving setup, not any component or hook
  that consumes the mocked data.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | `useCapability()` hook's state transitions (FR-4), route-to-component mapping, correlation-ID rendering given a mock error fixture (FR-7), `<EmptyState>` rendering given an empty successful response (FR-8) |
| Integration | Full routing (FR-1) end to end against MSW-mocked data (FR-6); capability-gating (FR-4) with the mock withholding then granting the value, confirming no host-only flash |
| Contract | For FR-6 tier-(a) fixtures (contract-covered endpoints): generated from `api/openapi.yaml` — a fixture drifting from the real schema is what `architecture-contracts.md` FR-3's contract test catches on the backend side. For tier-(b) fixtures (not yet in the contract): a test asserting every hand-written fixture file carries the `TODO(phase-06)` marker, so the tier itself is enforced, not just documented |
| E2E | A **`@playwright/test`** smoke test — a real CI dependency, not the Playwright MCP (`architecture-testing.md` FR-2 fixed that the MCP is interactive-only, scoped to a Claude Code session, and cannot run unattended in CI; this project already had to correct this exact mistake once, `architecture-desktop-host.md`'s self-review) — one full path through the shell with mock data, proving routing and shell composition work end to end |
| Accessibility | Shell landmarks and the `<MobileTabBar>` reflow, via `@axe-core/playwright` atop the same `@playwright/test` runner (`frontend-accessibility.md` FR-4), not the Playwright MCP's accessibility snapshot — that capability is interactive/dev-time only, per the same `architecture-testing.md` FR-2 distinction |

## Acceptance criteria

- [ ] All URLs from `architecture-frontend.md` FR-1's named list are
      registered routes, resolving to a real component (even a stub) or
      `<NotFound>` (FR-5) — never an unhandled router error
- [ ] Capability gating proven with a test that withholds the mock
      capability value and confirms host-only content never flashes
      before disappearing — the exact acceptance criterion
      `architecture-frontend.md` itself already names, satisfied here
      concretely
- [ ] The shell reflows to `<MobileTabBar>` below the defined breakpoint,
      proven with a viewport-resize test
- [ ] A keyboard-only pass completes the "open a book" reference
      walkthrough (`architecture-system.md`'s own named slice) through
      the shell, using mock data
- [ ] MSW's bundle is verifiably absent from a production build
- [ ] A mock error fixture's `correlationId` renders visibly in the
      corresponding error state (FR-7)
- [ ] An empty successful mock response renders `<EmptyState>`, never a
      blank content pane (FR-8)
- [ ] Every hand-written FR-6 tier-(b) fixture carries the
      `TODO(phase-06)` marker, proven by a test scanning fixture files

## Open questions

- **The exact responsive breakpoint value** — depends on
  `frontend-design-tokens.md`'s spacing/breakpoint tokens once
  extracted; not fixed as a number here.
- **Capability-loading timeout/retry policy** — no real endpoint exists
  yet to time out against meaningfully; phase 12's concern once one
  does.
- **`atTablet`'s placement** — inherited from `architecture-frontend.md`'s
  own Open questions; affects whether a tablet-specific shell
  composition is this spec's concern or the viewer surface's. A further
  consequence not yet resolved: FR-3's shell model is binary
  (sidebar-desktop or tab-bar-mobile) — if `atTablet` turns out to need
  a genuinely distinct third layout rather than reusing one of the two,
  FR-3's architecture itself needs revising, not just its ownership.
- **FR-6 tier-(b) fixture burden** — how many endpoints phase 04 needs
  that the contract doesn't have yet isn't known until implementation
  enumerates every screen's data needs; if the count is large, revisit
  whether `architecture-contracts.md` should be amended to add stub
  schemas for phase 04's benefit ahead of phase 06's real
  implementation, rather than accepting a large hand-written-fixture
  surface.

## References

- `architecture-frontend.md` FR-1 (routing), FR-2 (data-fetching), FR-3
  (capability gating), FR-6 (loading/error states), FR-7 (CSR-only) —
  every FR this spec makes concrete
- `roadmap/04-frontend-foundation/README.md` — shell composition scope,
  mock/stub strategy open decision, the production-leak security risk
- `.design-reference/ANALYSIS.md` — Mobile canvas's confirmed
  single-responsive-layout shape (FR-3)
- `architecture-contracts.md` FR-3 (contract test), Acceptance criteria
  (contract "initially contains only the health endpoint" — the reason
  FR-6 needs two fixture tiers, not one)
- `architecture-system.md` FR-6 — same-build-artifact requirement, the
  reason this spec doesn't split host/viewer route trees
- `architecture-testing.md` FR-2 — the Playwright MCP's interactive-only,
  not-CI-capable scope, the reason FR-7/Test strategy's E2E and
  accessibility rows use `@playwright/test`/`@axe-core/playwright`
  directly rather than the MCP
- `architecture-desktop-host.md` — self-review precedent for the exact
  MCP-vs-`@playwright/test` distinction this spec's Test strategy now
  follows
- `frontend-design-tokens.md`, `frontend-component-primitives.md` — the
  tokens and primitives this spec's shell composes, including the two
  net-new primitives (`EmptyState`, FR-8) this spec requires
- `frontend-accessibility.md` FR-4 — the `axe-core`/`@playwright/test`
  pairing this spec's own accessibility test row uses
- Constitution §7 (accessibility — landmarks), §9 (dependencies — React
  Router, TanStack Query, MSW justifications)
