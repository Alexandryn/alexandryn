# Phase 04, Tier 4 — Shell and routing — task list

Full plan: [`tasks/plan-p04-tier4-shell-and-routing.md`](plan-p04-tier4-shell-and-routing.md).
Each task: RED → GREEN → Refactor, one commit. Stop at Checkpoint P4-E
(no full regression / review / PR yet).

## Decisions (resolve once)

- [ ] D1 — file layout: `src/app/` (shell/router/providers), `src/screens/<Name>/`, `src/data/`, `src/mocks/`
- [ ] D2 — tests use `createMemoryRouter`; `createBrowserRouter` in `main.tsx`; shared `renderWithProviders`
- [ ] D3 — responsive swap via `useMediaQuery` (matchMedia) keyed to the breakpoint token, one `<nav>` mounted at a time
- [ ] D4 — `useCapability()` → `{ status: 'loading' | 'granted', can(name) }`, backed by a delayed mock `GET /api/bootstrap`
- [ ] D5 — host-only: `/settings`, `/system`, `/sources`, `/sources/:id`, `/import`; viewer stubs: `/access`, `/connect`, `/reader/:id`
- [ ] D6 — remove the Vite scaffold (`App.tsx`, `App.css`, `index.css`, demo assets)

## Maintainer decisions (resolved 2026-08-27, plan §Conformance findings)

- [x] F1 — `breakpoint` token value: **768px** (sidebar ≥768, MobileTabBar below), sourced from `atTablet`'s "768–1023px" band, recorded as a reasoned extraction
- [x] F2 — create health-only `api/openapi.yaml` this tier (T2)
- [x] F3 — host sidebar shell only; viewer top-nav chrome deferred to phase 11/12/13; `/access` `/connect` `/reader/:id` registered as stubs
- [x] F4 — `atTablet` third layout: carried forward, not resolved here (acknowledged only)

## Tasks

- [ ] T1 — dependencies (`react-router-dom@^7`, `@tanstack/react-query@^5`, `msw@^2`, `@axe-core/playwright@^4`) + `queryClient.ts` + `providers.tsx` + `renderWithProviders` (RED: `useQuery` throws outside provider)
- [ ] T2 — `api/openapi.yaml` (health-only) + MSW `handlers`/`browser`/`node` + `gen-fixtures.ts` + two-tier `fixtures/` + marker-scan test + `setup.ts` wiring + dev `worker.start()` behind `import.meta.env.DEV`
- [ ] T3 — `useCapability()` + `CapabilityProvider` + `<RequireCapability>` (RED: host-only child absent from DOM at every tick through loading→granted, then present)
- [ ] T4 — `breakpoint` token in the extraction pipeline (gated on F1); regenerate `theme.css`/`tokens.css`; back-fill `frontend-design-tokens.md` FR-1 status
- [ ] T5 — `AppShell` + `Sidebar` + `Titlebar` + `ContentPane`, semantic landmarks, token-only styling (RED: 3 landmarks; shell no-remount across route change; active-route mark)
- [ ] T6 — `MobileTabBar` + `useMediaQuery` + one-nav-mounted swap (RED: viewport-resize test flips mocked `matchMedia`, asserts Sidebar↔MobileTabBar)
- [ ] T7 — `router.tsx` full URL tree + `src/screens/` stubs + host-only gating + `main.tsx` → `<RouterProvider>` + scaffold removal (RED: per-URL heading table; `/nonsense` → `<NotFound>`; host-only shows loading then heading)
- [ ] T8 — `<NotFound>` + `<ErrorState>` (correlation ID, secondary placement) + layout `errorElement` + query-error path + synthetic `correlationId` in every error fixture (RED: id renders; retry re-runs query)
- [ ] T9 — `<EmptyState>` wiring on `/library` + `useLibraryItems` hook + switchable MSW handler (RED: `[]` → `<EmptyState>`, populated → list, both distinct from loading/error)
- [ ] T10 — E2E: real-app Playwright project; `shell.spec.ts` keyboard-only "open a book"; `a11y.spec.ts` `@axe-core/playwright` zero violations + reflow at desktop/mobile viewports

## Checkpoint P4-E

- [ ] Every named URL resolves to a real component or `<NotFound>`
- [ ] Capability-gating test proves no host-only flash (T3)
- [ ] `MobileTabBar` reflow proven by a viewport-resize test (T6, T10)
- [ ] Keyboard-only "open a book" walkthrough completes (T10)
- [ ] MSW verifiably absent from a production build (`check:dist-msw` green)
- [ ] Mock error fixture's `correlationId` renders visibly (T8)
- [ ] Empty successful response renders `<EmptyState>` (T9)
- [ ] Every hand-written fixture carries `TODO(phase-06)`, proven by the scanning test (T2)

Then: full regression → code-review fork (high) → PR → `gh pr checks --watch`.
