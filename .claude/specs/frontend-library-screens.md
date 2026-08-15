# Spec: Frontend library screens

| | |
|---|---|
| **Status** | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `06-library` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0033`](../reviews/0033-phase06-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`frontend-shell-and-routing.md` (phase 04) built the shell, routing, and
data-fetching layer against MSW-mocked fixtures for `/library` and
`/book/:id` — the routes `architecture-frontend.md` FR-1 already named.
`frontend-component-primitives.md` and `frontend-generated-covers.md`
built the primitives and cover system these screens compose from.
`backend-library-api.md` (this phase) now provides the real endpoints.
This spec is where the mock is retired and the real screens get built.

## Problem

Nothing has fixed: the library home screen's actual layout (grid/list
toggle, search/filter/sort controls, pagination UI), the work detail
screen's layout, how TanStack Query consumes `backend-library-api.md`'s
cursor pagination, or how the MSW mock for these specific routes gets
removed without breaking `frontend-shell-and-routing.md` FR-6's tier-(b)
fixtures for routes this phase doesn't touch.

## Goals

- Fix the library home screen: grid/list view, search/filter/sort
  controls, infinite-scroll or paginated load-more, wired to
  `backend-library-api.md` FR-1
- Fix the work detail screen: owned editions, collection memberships,
  wired to `backend-library-api.md` FR-5
- Fix the TanStack Query hook shape for cursor-based pagination
- Remove the tier-(b) MSW mock for `/api/v1/library` and
  `/api/v1/works/:id` specifically, per `frontend-shell-and-routing.md`
  FR-6's own hand-written-fixture-removal path

## Non-goals

- Collection screens — `frontend-collections-screens.md`
- The backend endpoints themselves — `backend-library-api.md`
- Search/filter/sort *logic* — already fixed server-side
  (`backend-library-api.md` FR-2–FR-4); this spec only owns the UI
  controls that produce those query parameters
- Any metadata- or source-derived content (cover images beyond the
  generated fallback, availability badges) — phase 07/08, matching
  `backend-library-api.md`'s own Non-goals

## User stories

- As **a user**, I want to browse my library in a grid of covers or a
  list of details, switch between them, and have my search/filter/sort
  choices reflected in the URL so I can share or bookmark a specific
  view.
- As **a user with a large library**, I want scrolling to load more
  results smoothly, not a "page 2" click-through that loses my scroll
  position.
- As **a user viewing a work's detail**, I want to see which editions I
  own and which collections it's in, in one screen, not several taps
  away.

## Functional requirements

- **FR-1** Library home (`/library`) renders a `<Sidebar>`-framed
  `<ContentPane>` (`frontend-shell-and-routing.md` FR-3) containing:
  a search input (debounced 300ms before updating the `q` query
  parameter — a reasoned placeholder balancing responsiveness against
  not firing a request per keystroke), filter/sort `<SegmentedControl>`s
  (`frontend-component-primitives.md` FR-1's Radix-wrapped bucket)
  reflecting `backend-library-api.md` FR-3/FR-4's closed vocabularies
  exactly (no UI option that doesn't map to a real backend value), a
  grid/list view toggle, and the results themselves rendered by a new,
  shared **`<WorkGrid>`** component — introduced here as the one
  rendering implementation both this screen and
  `frontend-collections-screens.md` FR-2's collection-detail screen
  depend on, taking a list of `WorkSummary` items
  (`backend-library-api.md`'s own response shape) and a `view:
  'grid' | 'list'` prop, with no knowledge of where its data came from
  (library query vs. collection query) — so the two consuming screens
  share one implementation of "render works as a grid or list" rather
  than two independent ones that could drift. The view toggle itself is
  persisted to `localStorage` (justified on its own terms: a plain,
  synchronous, zero-dependency Web API already available in every
  browser this app targets — no new library needed for a single
  string preference; if abandoned as a mechanism, the exit cost is
  trivial, a one-line read/write with no external API surface to
  migrate away from) — a UI-local preference, not server state
  (`architecture-frontend.md` FR-2's own UI-local-vs-server-data split:
  that FR licenses using plain state primitives over TanStack Query for
  this kind of value, distinct from and narrower than licensing
  `localStorage` specifically, which this spec justifies independently
  above).
- **FR-2** Search/filter/sort/view-toggle state lives in the URL's own
  query string (`?q=...&filter=...&sort=...`), not component-local
  state — `architecture-frontend.md` FR-1's bookmarkable-URL requirement
  applied concretely: reloading or sharing a filtered/sorted view
  reproduces it exactly. View-toggle (grid/list) is the one exception,
  staying in `localStorage` per FR-1, since it's a display preference
  about *how* results render, not *which* results — not meaningfully
  shareable the way a search/filter/sort choice is.
- **FR-3** Results use **infinite scroll**, not click-through pagination:
  a `useInfiniteQuery` (TanStack Query's own cursor-pagination primitive,
  matching `backend-library-api.md` FR-1's `cursor`/`nextCursor` shape
  directly — no adapter layer needed between the two) fetches the next
  page when the user scrolls near the end of the currently-loaded
  results, using the design reference's grid/list layout. Justified
  over click-through pagination: the design reference's own Library
  screen (`.design-reference/ANALYSIS.md`'s `atLibrary`) shows a
  continuous grid, not numbered pages, and infinite scroll matches that
  visual intent directly rather than requiring a design decision this
  spec would otherwise have to invent.
- **FR-4** `<WorkGrid>` (FR-1) virtualizes rendering (only visible rows'
  cover components actually mount) once its own item count exceeds
  **100** — `frontend-generated-covers.md` FR-4's own performance work
  (a single cover's cheap render cost) is what makes virtualization
  sufficient here without a separate performance budget for this screen
  specifically; below 100 items, virtualization overhead isn't worth
  its own complexity. Owning this threshold inside `<WorkGrid>` itself,
  not per consuming screen, means `frontend-collections-screens.md`'s
  own use of the component inherits the same behavior automatically —
  a large collection virtualizes the same way a large library page
  does, with no separate threshold for that spec to define.
- **FR-5** Work detail (`/book/:id`) renders: the work's title/author
  (real or generated cover per `frontend-generated-covers.md`'s
  degradation ladder, since no real cover exists until phase 07),
  a list of owned editions each showing its `added_at` date
  (`backend-library-api.md` FR-5's response shape), and a list of
  collection memberships each linking to that collection's detail
  screen (`frontend-collections-screens.md`). A work with zero owned
  editions (the "want to read" case, `domain-library.md`'s own State
  transitions) still renders correctly — no editions section, or an
  explicit "not yet in your library" state, never an empty/broken
  layout.
- **FR-6** The MSW mock (`frontend-shell-and-routing.md` FR-6) for
  `GET /api/v1/library` and `GET /api/v1/works/:id` is **removed
  outright**, not regenerated: at phase 04, `api/openapi.yaml` held only
  the health endpoint (`architecture-contracts.md`'s own Acceptance
  criteria), so these two routes' fixtures were always **tier-(b)**
  (hand-written, marked `// TODO(phase-06): replace with contract-
  generated fixture`) — never tier-(a) contract-generated fixtures being
  "retired," an earlier draft's wrong framing. `backend-library-api.md`
  FR-8 now adds these paths to `api/openapi.yaml` for real, but the
  correct next step per `frontend-shell-and-routing.md` FR-6's own
  migration path is deleting the hand-written fixture files entirely and
  routing these two paths to the real backend, not generating a fixture
  from the now-real contract only to keep using it. Concretely: the
  `TODO(phase-06)` marker on both files is the grep-able checklist that
  spec built for exactly this moment — this FR's own acceptance
  criterion is that marker's absence, not a vaguer "mock removed" claim.
  Every other route's MSW fixture (still tier-(b), no real endpoint yet)
  is untouched — this spec's own build/test setup routes these two
  specific paths to the real backend while MSW continues intercepting
  everything else, rather than an all-or-nothing switch.

## Non-functional requirements

- **Performance** — FR-4's virtualization threshold (100 results) is
  this spec's own concrete number, informed by but not identical to
  `frontend-generated-covers.md` FR-4's 500-cover benchmark (that
  number is about a single cover's own render cost; this one is about
  when virtualizing the surrounding grid becomes worth its complexity —
  a reasoned placeholder, not measured against real usage).
- **Security** — see Security considerations below.
- **Accessibility** — search/filter/sort controls follow
  `frontend-accessibility.md`'s keyboard map and focus-order rules
  directly; infinite scroll (FR-3) announces newly-loaded results via
  an `aria-live="polite"` region (never `"assertive"`, which would
  interrupt a screen-reader user mid-browse for a routine load) rather
  than relying on scroll position alone, which conveys nothing to a
  non-visual user.
- **Reliability** — FR-3's `useInfiniteQuery` inherits TanStack Query's
  own stale-while-revalidate behavior (`frontend-shell-and-routing.md`
  FR-2) — a transient fetch failure on page 2 doesn't blank page 1's
  already-loaded results.
- **Observability** — error states (a failed library fetch) render
  `frontend-shell-and-routing.md` FR-7's correlation-ID-visible error
  treatment, now against a real backend error for the first time.

## Domain model

Not applicable — this spec renders `domain-library.md`'s model via
`backend-library-api.md`'s shapes; it introduces no new domain concept.

## API and contracts

- **Library home ↔ `GET /api/v1/library`**: via `useInfiniteQuery`,
  query key `['library', { q, filter, sort }]` (TanStack Query's own
  cache-key convention, ensuring a change to any search/filter/sort
  parameter correctly invalidates and refetches rather than serving a
  stale cached page for different parameters).
- **Work detail ↔ `GET /api/v1/works/:id`**: via `useQuery`, query key
  `['work', id]`.
- **View-toggle preference ↔ `localStorage`**: a single key,
  main-process-uninvolved (this is web/renderer-only state, never
  touching Electron's `desktop-host-window-and-serving.md` FR-2 window-
  state persistence, a different concern at a different layer).

## State transitions

- Search/filter/sort: URL query string is the single source of truth
  (FR-2) — changing a control updates the URL (via React Router's own
  navigation, not a manual `history.pushState`), which `useInfiniteQuery`
  reads reactively.
- Infinite scroll: `idle → fetching-next-page → idle`, standard
  TanStack Query states, no custom state machine needed on top.

Illegal transitions, restated for this layer:

- Rendering results before the initial query resolves (violates
  `architecture-frontend.md`'s own "no optimistic host-only content"
  spirit, applied here to "no optimistic empty-vs-populated state" —
  the loading state, not an assumed-empty grid, renders first)
- A grid/list toggle change triggering a server refetch (it's UI-local,
  FR-1/FR-2 — changing it must never invalidate the TanStack Query cache)

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `GET /api/v1/library` fails (real backend error, not a mock) | TanStack Query's error state | `atStates` error treatment, correlation ID visible, retry option | Already-loaded pages (if any, from a prior successful fetch) stay visible per stale-while-revalidate |
| A work ID in the URL doesn't exist (`404` from `backend-library-api.md` FR-5) | TanStack Query's error state, `404`-specific | A "this book isn't in your library" state, distinct from a generic error — matching constitution §11's "specific, says what happened" bar, not a generic error treatment for a `404` specifically | No retry offered for a `404` (retrying an ID that doesn't exist won't succeed) — the shared error component reads the HTTP status to decide whether to offer retry |
| Infinite scroll fetch-next-page fails mid-scroll | TanStack Query's error state for that page | Loaded results stay visible; a small inline retry affordance at the point of failure, not a full-screen error replacing everything already shown | Distinguishes "the whole screen failed" from "the next page failed" — different severities, different UI |

## Security considerations

- **Search input is passed to the API unmodified, validated server-side
  only** — this spec doesn't attempt client-side sanitization of the
  search string (`backend-library-api.md` FR-2 already treats it as
  hostile input at the boundary that matters); the frontend's only
  responsibility is not rendering it unescaped anywhere, which React's
  default JSX escaping already covers.
- **No new trust boundary** — this spec adds no capability-gated
  (host-only) content; the library is visible to the loopback/LAN
  viewer surface identically, matching `architecture-frontend.md` FR-3's
  existing "every capability renders unconditionally until phase 12/13"
  rule.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Search debounce timing (fake timers, FR-1), URL-query-string ↔ control-state round trip (FR-2) |
| Component | Grid/list rendering per result count (including the virtualization threshold, FR-4), work-detail's zero-editions state (FR-5) |
| Integration | Full library-home flow against the real backend (`backend-library-api.md`, via `backend-test-harness.md`'s harness) — search, filter, sort, infinite scroll, each against real seeded data, not MSW |
| Accessibility | `axe` checks on both screens with real data density (not empty mocks); the infinite-scroll `aria-live` announcement (FR-1 above) |
| E2E | Browse → search → filter → open a work → view its editions and collections — the phase 06 slice of `architecture-system.md`'s reference walkthrough |

## Acceptance criteria

- [ ] Library home renders real data from `backend-library-api.md`, MSW
      no longer intercepting these two routes, and the `TODO(phase-06)`
      marker is gone from both fixture files, proven by a grep-based
      check (FR-6)
- [ ] A work reachable only via `filter=wanted` (in a collection, zero
      `LibraryEntry`s) appears in the default library-home view,
      proving the "want to read" case isn't lost in the frontend layer
- [ ] Search/filter/sort/view state round-trips through the URL, proven
      by reloading a URL with query parameters set and confirming the
      controls reflect them
- [ ] Infinite scroll loads additional pages correctly using the
      cursor, proven against a real dataset larger than one page
- [ ] Grid virtualizes above the 100-result threshold, proven by a
      render-count assertion, not visual inspection alone
- [ ] Work detail correctly renders zero-, one-, and multi-edition
      cases, and zero-, one-, and multi-collection-membership cases
- [ ] Every FR maps to an exit criterion in phase 06's own document

## Open questions

- **300ms search debounce** — reasoned placeholder, not measured against
  real typing behavior.
- **100-result virtualization threshold (FR-4)** — reasoned placeholder,
  not measured against real device performance.
- **Whether view-toggle preference should eventually sync server-side**
  (so it's consistent across devices, not per-browser `localStorage`) —
  not designed here; `domain-library.md`'s own "one shared database"
  reasoning for collections doesn't obviously extend to a pure display
  preference, but worth revisiting once multi-device usage is real
  (phase 14).

## References

- `backend-library-api.md` FR-1–FR-5 — the endpoints this spec consumes
- `frontend-shell-and-routing.md` FR-2, FR-6, FR-7 — TanStack Query
  usage pattern, the MSW migration path FR-6 executes, correlation-ID
  error display
- `frontend-generated-covers.md` FR-4 — the per-cover performance work
  FR-4 here builds on
- `frontend-component-primitives.md`, `frontend-accessibility.md` — the
  primitives and cross-cutting a11y rules this spec composes
- `frontend-collections-screens.md` FR-2 — the sibling spec consuming
  `<WorkGrid>` (FR-1), this spec's own new shared component
- `domain-library.md` — the model these screens render
- `.design-reference/ANALYSIS.md` — `atLibrary`, `atBook` screens this
  spec's layout follows
- Constitution §7 (accessibility), §11 (copy — error-state specificity)
