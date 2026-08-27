# Phase 04, Tier 4 — Shell and routing — implementation plan

Full phase context: [`tasks/plan-phase04.md`](plan-phase04.md). Task list:
[`tasks/todo-p04-tier4-shell-and-routing.md`](todo-p04-tier4-shell-and-routing.md).
Spec: [`frontend-shell-and-routing.md`](../.claude/specs/frontend-shell-and-routing.md)
(`APPROVED`).

## Context

Tier 0–3 are merged to `main` (PR #59/#60/#61/#62). Tier 4 builds the
application shell and routing: React Router v7 in data-router mode with
the full URL list, TanStack Query v5 as the only data-fetching seam, the
`Sidebar`/`Titlebar`/`ContentPane`/`MobileTabBar` shell composition, the
`useCapability()` gate for host-only routes, `<NotFound>`, the MSW
two-tier mock strategy, correlation-ID-visible error states, and
`<EmptyState>` wiring — all against mock data, zero real backend
(phase 06's job).

### Design-conformance check (performed at plan drafting, 2026-08-27)

Re-synced all 4 `.dc.html` canvases via `DesignSync` against project
`78075626-e444-438f-8437-205d57129a37` (`get_project` → still
`"Alexandryn interactive prototype"`; `list_files` → same 5 files;
`get_file` per canvas, string-compared against `.design-reference/`).
**Byte-identical, no drift since 2026-08-13** — `ANALYSIS.md`'s sync
date stands. Canvases consulted for this tier's surfaces:

- **`Alexandryn-Electron.dc.html`** (host shell) — persistent top bar
  (window controls, `ALEXANDRYN` wordmark, centred `⌘K` search,
  `HOSTING · 2 DEVICES` pill, theme toggle, avatar) = `<Titlebar>`;
  246px left rail (Library / Discover / Sources / Collections — divider —
  Activity / Import / Settings) = `<Sidebar>`; scrolling right region =
  `<ContentPane>`. Per-screen `<h1>` (e.g. "Library" + stats line +
  Filter/Sort/Grid-List) is content-pane content, not chrome. **Matches
  FR-3.**
- **`Alexandryn-Mobile.dc.html`** — single responsive layout, bottom tab
  bar `Library / Discover / Collections / More` (`tabbar()` in the data
  script). **Matches FR-3's `<MobileTabBar>` exactly.**
- **`Alexandryn-Electron-Admin.dc.html`** — `atStates`: every error card
  leads with a plain-language sentence, then an optional
  `Advanced details ▾` disclosure holding the technical block (mono:
  `HTTP 403 · SignatureDoesNotMatch / s3://… / 14:22:07`). FR-7's
  `correlationId` belongs in that secondary/advanced position, matching
  the spec's own "small, secondary" wording — no conflict, a placement
  cue. `atTablet`: see conformance finding 4 below.
- **`Alexandryn-Web.dc.html`** (viewer shell) — see conformance
  finding 3 below.

### Conformance findings — decisions (resolved 2026-08-27)

1. **`breakpoint` token → 768px.** Added to the token pipeline (T4),
   sourced from `atTablet`'s "768–1023px" prose band; sidebar `≥768`,
   `<MobileTabBar>` below.
2. **`api/openapi.yaml` created this tier** (health-only, T2).
3. **Host sidebar shell only.** Viewer routes registered as stubs; the
   Web canvas's top-nav chrome deferred to phase 11/12/13.
4. **`atTablet`** carried forward, not resolved (per tier brief).

Full reasoning for each, as presented:

### Conformance findings (as flagged)

1. **`breakpoint` token missing.** `frontend-design-tokens.md` FR-1
   mandates a `breakpoint` category feeding Tailwind's `screens` key,
   "the category `frontend-shell-and-routing.md` FR-3 depends on for its
   sidebar-to-tab-bar reflow." Tier 1's extraction (`web/scripts/tokens/`)
   never produced one; `web/src/theme.css` has no breakpoint token. The
   canvases contain **no `@media` rules** — they are fixed-width
   artboards. The only numeric evidence is `atTablet`'s prose: "768–1023px".
   Proposed resolution (T4): add a `breakpoint` category to the token
   pipeline with a single FR-3 reflow value of **768px** — sidebar at
   `≥768`, `<MobileTabBar>` below — sourced from that prose band (the
   Mobile canvas is a phone layout, consistent with `<768` = tab bar),
   recorded as a reasoned extraction, not an invented value, and
   `frontend-design-tokens.md` FR-1's now-satisfied status back-filled.
   Alternatives: 1024px, or escalate for a design-reference `@media`
   extension. **Decision needed.**
2. **`api/openapi.yaml` does not exist.** `architecture-contracts.md`
   FR-2 fixes its location and its Acceptance criteria require it "exist…
   even if it initially contains only the health endpoint" — phase 03
   did not deliver it. FR-6 tier (a) ("fixtures generated from
   `api/openapi.yaml` directly, never hand-written") therefore has no
   source. Proposed resolution (T2): Tier 4 creates the health-only
   `api/openapi.yaml` (implements an already-approved acceptance
   criterion; makes FR-6's tier-(a) mechanism real rather than vaporware).
   Alternative: treat every phase-04 fixture as tier (b) and file a
   separate issue for the missing contract file. **Decision needed.**
3. **Viewer shell diverges from FR-3.** `Alexandryn-Web.dc.html` uses a
   sticky **horizontal top nav bar** (wordmark + inline nav items +
   centred search + host badge + theme + avatar), not the sidebar FR-3
   describes; FR-3's Non-goals say the shell "runs identically" host vs
   LAN. The viewer routes are `/access` + `/connect` (phase 12/13,
   `ANALYSIS.md` classifies "correctly deferred, outline only") and
   `/reader/:id` (phase 11). Proposed resolution: Tier 4 builds **only
   the host sidebar shell** per FR-3, registers `/access`, `/connect`,
   `/reader/:id` as stub routes, and defers the viewer chrome to its
   owning phase. **Confirm this reading.**
4. **`atTablet` describes a third layout.** The canvas text: "768–1023px.
   The sidebar becomes a 60px icon rail … No bottom bar … Not a squeezed
   desktop." FR-3's shell model is binary (sidebar / tab-bar, one
   breakpoint); the spec's own Open questions already name this exact
   consequence and defer it, and roadmap `04/README.md`'s **Out** section
   lists `atTablet` as "deferred, not built assuming a settled surface."
   Carried forward unchanged — Tier 4 builds the binary FR-3 shell; the
   768–1023px band gets the `<MobileTabBar>` for now. Flagged again per
   the tier brief; **not this tier's to resolve.**

### New dependencies (constitution §9 — recorded in the PR)

All four are named and justified under §9 by an `APPROVED` spec already;
this plan is where they are actually added, and the PR body carries the
what / why-not-stdlib / abandonment cost per §9.

| Package | Pin | Spec | Abandonment cost |
|---|---|---|---|
| `react-router-dom` | `^7` | `frontend-shell-and-routing.md` FR-1 | Moderate — routes are declared data; migration rewrites the route-tree declaration, not every navigating component |
| `@tanstack/react-query` | `^5` | FR-2 | Contained — lives behind the data-fetching hooks (no component calls `fetch`); replace the hooks, not call sites |
| `msw` | `^2` | FR-6 | Low — network-interception boundary only; dev/test dependency, never in `web/dist` (`check:dist-msw` enforces) |
| `@axe-core/playwright` | `^4` (dev) | `frontend-accessibility.md` FR-4, this spec's Test strategy | Low — CI accessibility gate only; not a runtime dependency |

Bundle-budget note: `react-router-dom` + `@tanstack/react-query` land in
`web/dist` and count against `frontend-tooling.md`'s 250 KiB gzipped
budget; `msw` and `@axe-core/playwright` do not. `check:bundle-size`
stays a gate through every task.

## Decisions

- **D1 — file layout.** `web/src/app/` for shell/router/providers
  (`router.tsx`, `providers.tsx`, `queryClient.ts`, `shell/`,
  `capability/`); `web/src/screens/<Name>/` for route views (mirrors the
  `components/<Name>/` convention); `web/src/data/` for TanStack Query
  hooks (the only `fetch` callers); `web/src/mocks/` for MSW
  (`handlers.ts`, `browser.ts`, `node.ts`, `fixtures/generated/`,
  `fixtures/handwritten/`).
- **D2 — data-router in tests.** `createMemoryRouter` +
  `<RouterProvider>` in Vitest; `createBrowserRouter` in `main.tsx`.
  A shared `renderWithProviders` (QueryClientProvider + memory router)
  test helper.
- **D3 — responsive swap via `useMediaQuery` (matchMedia), not CSS
  alone.** FR-3's reflow and Checkpoint P4-E's "viewport-resize test"
  need the swap observable in jsdom; a `matchMedia`-backed hook keyed to
  the T4 breakpoint token makes the resize test real (mock `matchMedia`,
  flip it, assert Sidebar↔MobileTabBar). Both bars are semantic `<nav>`s
  with distinct `aria-label`s; only one is mounted at a time.
- **D4 — `useCapability()` shape.** `{ status: 'loading' | 'granted',
  can(name): boolean }`. No `denied` branch is reachable this phase
  (spec State transitions); the shape supports one later without a
  call-site change. Backed by a TanStack Query call to a mock
  `GET /api/bootstrap` that resolves after an artificial `delay()` —
  never instant, so the loading path is always exercised.
- **D5 — host-only routes.** `/settings`, `/system`, `/sources`,
  `/sources/:id`, `/import` (architecture-frontend.md FR-3's named
  set: "Settings, System, Sources configuration, Import"). Wrapped in
  `<RequireCapability>`, which renders the loading state until
  `status === 'granted'`, never the child optimistically. `/library`,
  `/book/:id`, `/collections`, `/collections/:id`, `/discover`,
  `/activity` render for everyone; `/access`, `/connect`, `/reader/:id`
  are viewer stubs (finding 3).
- **D6 — remove the Vite scaffold.** `src/App.tsx`, `src/App.css`,
  `src/index.css`, `src/assets/{hero.png,react.svg,vite.svg}`,
  `public/` demo assets — replaced wholesale by the shell, as
  `main.tsx`'s own comment anticipates.

## URL list (FR-1 — every one registered, resolving to a real component or `<NotFound>`)

| URL | Screen | Gating | Phase-04 form |
|---|---|---|---|
| `/` | — | — | redirect → `/library` |
| `/library` | `atLibrary` | shared | stub + real query hook (T9 empty-state demo) |
| `/book/:id` | `atBook` | shared | stub (focus target for the "open a book" walkthrough) |
| `/collections` | `atCollections` | shared | stub |
| `/collections/:id` | `atCollection` | shared | stub |
| `/discover` | `atDiscover` | shared | stub |
| `/sources` | `atSources` | host-only | stub behind `<RequireCapability>` |
| `/sources/:id` | `atSourceDetail` | host-only | stub behind `<RequireCapability>` |
| `/import` | `atImport` | host-only | stub behind `<RequireCapability>` |
| `/activity` | `atActivity` | shared | stub |
| `/settings` | `atSettings` | host-only | stub behind `<RequireCapability>` |
| `/system` | `atSystem` | host-only | stub behind `<RequireCapability>` |
| `/access` | `atAccess` (Web) | viewer | stub (chrome deferred, finding 3) |
| `/connect` | `atConnect` (Web) | viewer | stub (chrome deferred, finding 3) |
| `/reader/:id` | `atReader` (Web) | viewer | stub (phase 11) |
| `*` | — | — | `<NotFound>` via layout-route `errorElement` / catch-all |

`/first-run` (`atFirstRun`) is intentionally omitted — no phase claims a
first-run flow yet and `ANALYSIS.md` classifies `atFirstRun`
**Unclassified**; adding a route would be inventing scope.

## Dependency graph

```
deps + providers (T1)
  ├── api/openapi.yaml + MSW two-tier fixtures + marker test (T2)
  │     └── useCapability() + RequireCapability (T3)
  ├── breakpoint token  [gated on finding 1] (T4)
  │     └── Sidebar / Titlebar / ContentPane (T5)
  │           └── MobileTabBar + responsive reflow (T6)
  │                 └── router: full tree, stubs, host-only gating (T7)
  │                       ├── NotFound + errorElement + correlation-ID ErrorState (T8)
  │                       └── EmptyState wiring on /library (T9)
  │                             └── E2E: keyboard "open a book" + axe landmarks/reflow (T10)
```

## Task list

Each task: RED → GREEN → Refactor, one commit. Stop at Checkpoint P4-E
(no full regression / review / PR yet — that is steps 3–5 of the tier
process).

1. **T1 — dependencies + provider seam.** Add the four deps (pinned).
   `src/app/queryClient.ts` (a `QueryClient` factory with explicit
   retry/stale-time defaults), `src/app/providers.tsx`
   (`<QueryClientProvider>`), `src/test/renderWithProviders.tsx`.
   *RED:* a component calling `useQuery` throws outside the provider and
   renders inside `renderWithProviders`. *Files:* `package.json`,
   `package-lock.json`, `src/app/*`, `src/test/*`. *Scope:* S.
2. **T2 — contract file + MSW two-tier fixtures.** Create health-only
   `api/openapi.yaml` (finding 2). `src/mocks/{handlers,browser,node}.ts`;
   `scripts/gen-fixtures.ts` writing `src/mocks/fixtures/generated/` from
   `api/openapi.yaml` + an `npm run mocks:gen-fixtures` script and a
   staleness check (same pattern as `tokens:generate`).
   `src/mocks/fixtures/handwritten/` seeded with the bootstrap fixture,
   each file carrying `// TODO(phase-06): replace with contract-generated
   fixture`. MSW `setupServer` wired into `src/test/setup.ts`
   (`listen` / `resetHandlers` / `close`); dev `worker.start()` behind
   `import.meta.env.DEV` in `main.tsx`. *RED:* `fixtures.test.ts` fails
   when a `handwritten/` file lacks the marker and when a `generated/`
   file is hand-edited (regen diff). *Files:* `api/openapi.yaml`,
   `src/mocks/**`, `scripts/gen-fixtures.ts`, `src/test/setup.ts`,
   `package.json`. *Scope:* M.
3. **T3 — `useCapability()` + `<RequireCapability>`.** `GET /api/bootstrap`
   handler → all-granted after `delay()`. `src/app/capability/`:
   context, provider (`useQuery`), `useCapability()` (D4),
   `<RequireCapability>`. *RED:* hook starts `loading`, transitions to
   `granted`; `<RequireCapability>` — a probe asserting the host-only
   child is absent from the DOM at every tick from mount through the
   loading→granted transition (no flash), then present. *Files:*
   `src/app/capability/**`, `src/mocks/handlers.ts`,
   `src/mocks/fixtures/handwritten/bootstrap.ts`. *Scope:* M.
4. **T4 — `breakpoint` token.** *(Gated on finding 1's decision.)*
   Extend `scripts/tokens/extract.ts` + `generate-tokens.ts` to emit a
   `breakpoint` category into `@theme` (`--breakpoint-*`, Tailwind
   `screens`) and `tokens.css`. Regenerate; the CI staleness check must
   pass. Back-fill `frontend-design-tokens.md` FR-1's status note.
   *RED:* an `extract.test.ts` case for the breakpoint value; a test
   asserting the Tailwind theme exposes the `screens` key. *Files:*
   `scripts/tokens/extract.ts`, `scripts/generate-tokens.ts`,
   `src/theme.css`, `src/tokens.css`, `scripts/tokens/extract.test.ts`,
   `.claude/specs/frontend-design-tokens.md`. *Scope:* M.
5. **T5 — `Sidebar` / `Titlebar` / `ContentPane`.** `src/app/shell/`:
   `AppShell.tsx` (semantic `<header>` + `<nav>` + `<main>`, `<Outlet/>`
   inside `ContentPane`), `Sidebar.tsx` (`<nav aria-label="Primary">`,
   `NavLink` active state, the canvas's item order + divider),
   `Titlebar.tsx` (wordmark, search-affordance button stub, hosting-status
   stub, theme-toggle stub, avatar stub — matching the Electron top bar),
   `ContentPane.tsx`. Token-only styling (`check:token-styling` gate).
   *RED:* all three landmarks present; the shell does not remount across
   a route change (mounted-count probe on a shell child); `Sidebar`
   marks the active route. *Files:* `src/app/shell/**`. *Scope:* M.
6. **T6 — `MobileTabBar` + reflow.** `MobileTabBar.tsx`
   (`<nav aria-label="Primary">`, Library / Discover / Collections /
   More, bottom-fixed, semantic). `src/lib/useMediaQuery.ts` keyed to the
   T4 breakpoint token. `AppShell` mounts exactly one of Sidebar /
   MobileTabBar. *RED:* below breakpoint → MobileTabBar present, Sidebar
   absent; above → inverse; a resize test flipping mocked `matchMedia`
   and asserting the swap. *Files:* `src/app/shell/MobileTabBar.tsx`,
   `src/lib/useMediaQuery.ts`, `src/app/shell/AppShell.tsx`. *Scope:* M.
7. **T7 — router + stubs + host-only gating.** `src/app/router.tsx`
   (`createBrowserRouter`, layout route = `<AppShell>`, every URL from
   the table). `src/screens/<Name>/` stubs — each a real landmark +
   `<h1>` + a focus target; `/book/:id` reads `useParams`. Host-only
   routes wrapped per D5. `main.tsx` → `<RouterProvider>`; remove the
   scaffold (D6). *RED:* a table test — each URL renders its heading;
   `/nonsense` → `<NotFound>`; a host-only URL shows the loading state
   then its heading. *Files:* `src/app/router.tsx`, `src/screens/**`,
   `src/main.tsx`, deletions per D6. *Scope:* M.
8. **T8 — `<NotFound>` + `<ErrorState>` + correlation ID.**
   `src/screens/NotFound/` (composed from `EmptyState`/`Button`, plain
   copy, link to `/library`). `src/components/ErrorState/` — `message`
   prominent, `code` + `correlationId` in `font-mono` secondary
   placement (the `atStates` "Advanced details" treatment), a retry
   slot. Layout-route `errorElement` → `<ErrorState>` from
   `useRouteError()`; a TanStack Query error path renders the same.
   Every MSW error fixture carries a synthetic `correlationId` (FR-7).
   *RED:* `<NotFound>` home link works; `<ErrorState>` given
   `{code,message,correlationId}` shows the id; a forced query error
   renders `<ErrorState>` with the fixture's `correlationId` and retry
   re-runs the query. *Files:* `src/screens/NotFound/**`,
   `src/components/ErrorState/**`, `src/app/router.tsx`,
   `src/mocks/handlers.ts`. *Scope:* M.
9. **T9 — `<EmptyState>` wiring on `/library`.** `src/data/useLibraryItems.ts`
   (TanStack Query hook; tier-(b) fixture with the marker). MSW handler
   switchable between `{ items: [] }` and a populated list. `/library`
   stub: loading → `Skeleton`; error → `<ErrorState>`; empty-success →
   `<EmptyState>` ("Your library is waiting.", the `atStates` copy);
   populated → a plain list. *RED:* `[]` → `<EmptyState>` in DOM, no
   blank pane; populated → list, no `<EmptyState>`; both distinct from
   loading and error. *Files:* `src/screens/Library/**`,
   `src/data/useLibraryItems.ts`, `src/mocks/{handlers.ts,fixtures/handwritten/library.ts}`.
   *Scope:* M.
10. **T10 — E2E: keyboard "open a book" + axe.** Point
    `playwright.config.ts`'s `webServer` at the real app (a dedicated
    `e2e/app.vite.config.ts` serving `src/main.tsx` with MSW forced on),
    as a second Playwright project alongside the benchmark.
    `e2e/shell.spec.ts` — keyboard only (`Tab`/`Enter`, no pointer):
    `/library` → focus a book link → `Enter` → URL is `/book/:id` →
    focus is on the book `<h1>`. `e2e/a11y.spec.ts` —
    `@axe-core/playwright`, zero violations on the shell at a desktop and
    a mobile viewport; the mobile viewport shows `<MobileTabBar>`, the
    desktop one the sidebar (reflow proven in a real browser too).
    *RED:* both specs fail first (no route target / axe violations on the
    scaffold). *Files:* `e2e/**`, `playwright.config.ts`, `package.json`.
    *Scope:* M.

## Checkpoint P4-E (exit criteria — from `tasks/todo-phase04.md`)

- [ ] Every named URL resolves to a real component or `<NotFound>`
- [ ] Capability-gating test proves host-only content never flashes
      before disappearing (T3)
- [ ] `MobileTabBar` reflow proven by a viewport-resize test (T6, T10)
- [ ] Keyboard-only "open a book" walkthrough completes (T10)
- [ ] MSW verifiably absent from a production build (`check:dist-msw`,
      already wired — must stay green through T2)
- [ ] A mock error fixture's `correlationId` renders visibly (T8)
- [ ] An empty successful response renders `<EmptyState>` (T9)
- [ ] Every hand-written fixture carries the `TODO(phase-06)` marker,
      proven by the scanning test (T2)

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Finding 1 (breakpoint token) blocks T4→T6 | High for the tier's schedule | Surfaced now as a decision, not mid-build; T1–T3 are independent of it and proceed regardless |
| `react-router-dom` + `@tanstack/react-query` push `web/dist` over the 250 KiB budget | Medium | `check:bundle-size` runs every task; if breached, code-split the router or revisit the budget (Tier 3's FR-4 carry-forward already flags the budget as provisional) |
| `createBrowserRouter` + Vitest jsdom friction (history, `<RouterProvider>`) | Low | D2's `createMemoryRouter` is the supported test path; documented in `renderWithProviders` |
| E2E webServer wiring (T10) — first real-app Playwright run, currently benchmark-only | Medium | Separate Playwright project + its own vite config; proven by a green `npx playwright test` locally before the PR |
| MSW leaking into `web/dist` via a static `import` in `main.tsx` | High if it happens | Dynamic `import()` behind `import.meta.env.DEV`; `check:dist-msw` is the backstop and already a CI gate |

## Open questions (carried forward, not resolved here)

- **`atTablet` placement** (`ANALYSIS.md`, `architecture-frontend.md`) —
  needs the Claude Design project owner; conformance finding 4.
- **`text-3` WCAG AA contrast exception** (Tier 1,
  `check-token-contrast.ts`) — maintainer decision pending.
- **FR-4 numeric budgets** (250 KiB bundle; Tier 3's 100ms/500-cover/60fps)
  — confirm-or-replace once real screens exist (phase 06); this tier
  measures against them but does not ratify them.
- **The single FR-3 breakpoint value** — conformance finding 1; T4 is
  gated on it.
