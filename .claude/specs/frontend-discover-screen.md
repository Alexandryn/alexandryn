# Spec: Frontend Discover screen

| | |
|---|---|
| **Status** | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| **Phase** | `07-metadata` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0034`](../reviews/0034-phase07-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (1 Blocking, confirmed independently by both; 8 Major, 2 confirmed independently by both), all findings fixed; approved by maintainer 2026-08-15 |

## Context

`frontend-shell-and-routing.md` (phase 04) built the shell and a
`Discover` nav entry pointing at a not-yet-built route, mocked behind
MSW. `frontend-library-screens.md` (phase 06) introduced `<WorkGrid>`
and established this project's grid/list, search, infinite-scroll
patterns against a real backend for the first time. `backend-metadata-adapter.md`
and `backend-metadata-caching.md` (both phase 07) now provide a real
`/api/v1/discover*` API returning normalised Open Library results,
including real cover images — the first screen in this project to
render a cover that isn't `frontend-generated-covers.md`'s procedural
fallback.

## Problem

Nothing renders `/api/v1/discover*`. The Discover nav entry currently
leads nowhere real, and no screen exists to search Open Library, view a
result, or see it in a degraded (offline/rate-limited) state.

## Goals

- A search-driven Discover screen reusing this project's established
  patterns (`<Sidebar>`/`<ContentPane>` shell, `<WorkGrid>`-style
  rendering) rather than inventing new ones
- A work-detail view for a single Discover result, showing normalised
  work/edition data and real cover images
- Degraded-state UI for offline, rate-limited, and partial-data
  responses — not a blank screen or an unhandled spinner
- Retirement of the Discover route's `frontend-shell-and-routing.md`
  FR-6 MSW mock, same tier-(b) removal discipline
  `frontend-library-screens.md` FR-6 already established for
  `/api/v1/library*`

## Non-goals

- Adding a Discover result to the user's actual library — that's
  phase 10's import flow; this screen only *views* Open Library data,
  it has no "add to library" action yet (a nav dead-end acknowledged
  in Open questions, not solved here)
- Any source other than Open Library, any custom-source search UI —
  phase 08
- Search-result caching or offline-first behaviour beyond what
  `backend-metadata-caching.md`'s server-side cache already provides —
  this screen has no client-side cache of its own beyond TanStack
  Query's normal request-level cache

## User stories

- As **someone looking for a book to add later**, I want to search
  Open Library by title or author and see real results with covers, so
  I can find the right edition before deciding to add it.
- As **someone with no internet connection**, I want Discover to tell me
  plainly that it can't reach Open Library right now, rather than
  spinning forever or showing a blank grid.
- As **someone who clicks into a result with a missing cover or missing
  author**, I want the screen to show what's known and gracefully omit
  what isn't, so a partial Open Library record doesn't look broken.

## Functional requirements

- **FR-1** Discover home (`/discover`) renders a `<Sidebar>`-framed
  `<ContentPane>` (`frontend-shell-and-routing.md` FR-3) containing: a
  search input (debounced 300ms, matching `frontend-library-screens.md`
  FR-1's own debounce value for consistency across this project's two
  search surfaces) bound to `backend-metadata-adapter.md` FR-1's `q`
  parameter, and results rendered by a new **`<DiscoverResultGrid>`**
  component — deliberately not a reuse of `<WorkGrid>`: `<WorkGrid>`
  (`frontend-library-screens.md` FR-1) takes `WorkSummary` items, whose
  fields include the ownership/collection-membership facts
  `backend-library-api.md` FR-9 computes per work — a
  `NormalisedSearchResult` (`backend-metadata-adapter.md` FR-2) has no
  equivalent for any of them; forcing one shape to fit both would mean
  `<WorkGrid>` either grows optional fields it doesn't always have data
  for, or `<DiscoverResultGrid>` fabricates placeholder library fields
  that don't exist for a result the user has never added — this spec
  chooses a second, small, focused component over either distortion.
  `<DiscoverResultGrid>` internally reuses the same cover-tile,
  title/author-text, and grid/list layout primitives
  `frontend-component-primitives.md` and `frontend-generated-covers.md`
  already established, so the two grids look consistent without sharing
  a data contract. Unlike `<WorkGrid>`, `<DiscoverResultGrid>` has **no
  grid/list view toggle** — Discover renders as a grid only; a list view
  makes sense for a personal library someone scans row-by-row for
  metadata (added date, ownership) that Discover results don't carry,
  but adds no value for search results being evaluated primarily by
  cover and title. `<DiscoverResultGrid>` also does not adopt
  `<WorkGrid>` FR-4's 100-item virtualization threshold — a deliberate
  omission, not an oversight: FR-3 below caps a single Discover page at
  `backend-metadata-adapter.md` FR-1's `limit` maximum of 50 items,
  which never reaches the threshold `<WorkGrid>`'s virtualization exists
  for.
- **FR-2** Search state (`q`) lives in the URL's query string
  (`?q=...`), matching `frontend-library-screens.md` FR-2's URL-as-state
  discipline for bookmarkable/shareable search results
  (`architecture-frontend.md` FR-1). No filter or sort controls exist
  for Discover — `backend-metadata-adapter.md` FR-1 exposes none (it
  passes Open Library's own relevance ranking through unmodified), so
  there is no closed vocabulary here for a UI control to reflect.
- **FR-3** Results use **click-through pagination** (`Previous`/`Next`,
  or a "load more" affordance — implementation detail left to
  `frontend-component-primitives.md`'s existing pattern set), driven by
  `backend-metadata-adapter.md` FR-1's `limit`/`offset` parameters via
  TanStack Query's standard paginated-query primitive (not
  `useInfiniteQuery` — `frontend-library-screens.md` FR-3 chose infinite
  scroll because its cursor shape and the design reference's continuous-
  grid intent both pointed there; neither applies here: Discover has no
  cursor to page with (`backend-metadata-adapter.md` FR-1's own
  divergence, offset-based because it proxies Open Library's own
  offset-based API), and no design-reference screen exists yet for
  Discover's specific layout to match against — flagged in Open
  questions as a design-reference gap, not silently guessed past).
- **FR-4** Clicking a result navigates to `/discover/works/:openLibraryId`,
  rendering `backend-metadata-adapter.md` FR-3's response: the work's
  title, subtitle, description (when present — omitted entirely from
  layout when absent, not rendered as an empty section), subjects as a
  tag list, authors (each linking to nothing yet — no author-detail
  screen exists in any current phase), a real cover image
  (`coverUrl`, when present) falling back to
  `frontend-generated-covers.md`'s existing procedural-cover ladder
  when absent (the same fallback mechanism, not a new one — a missing
  Open Library cover is exactly the "no real cover" case that ladder
  was already built for), and a list of editions each showing
  publisher/publish date/language when present.
- **FR-5** A `503`/`Unavailable` response from any `/api/v1/discover*`
  call (Open Library unreachable or rate-limited, per
  `backend-metadata-adapter.md` FR-9) renders a distinct, named
  degraded state — "Open Library is unavailable right now, try again
  shortly" (constitution §11: plain, specific, no apology, no
  exclamation marks) — with a retry action, not a generic error
  boundary. A `404`/`NotFound` on a work-detail fetch (a since-removed
  Open Library work) renders a distinct "This book couldn't be found on
  Open Library" state, separately worded from the `503` case since the
  two mean different things to a user (try again later, versus this
  specific thing is gone).
- **FR-6** The Discover route's `frontend-shell-and-routing.md` FR-6
  MSW mock and any `TODO(phase-06)`-adjacent placeholder MUST be
  removed once wired to the real `/api/v1/discover*` endpoints —
  production builds already excluded MSW regardless (phase 04's
  build-time check); what this FR actually changes is the dev/test
  fixture tier, matching `frontend-library-screens.md` FR-6's own
  precedent and wording exactly, applied to Discover's endpoints
  instead of the library ones.
- **FR-7** This is the first screen in the project loading *network*
  images rather than `frontend-generated-covers.md`'s purely
  client-rendered procedural layers (that spec's own FR-1: "No layer
  fetches external data" — it has no lazy-loading or image-load concept
  to reuse; this FR introduces one fresh, on its own merits, not as a
  citation of an existing discipline). A real cover image (FR-4) MUST
  render inside a container with explicit reserved dimensions (matching
  the aspect ratio `frontend-component-primitives.md`'s cover-tile
  primitive already uses) *before* the image loads, so a slow load never
  shifts surrounding layout, MUST use `loading="lazy"` for any cover
  below the fold, and MUST fall back to `frontend-generated-covers.md`'s
  existing procedural cover (FR-4) on a load error — reusing that
  spec's fallback *rendering path* once network loading has failed, not
  its (nonexistent) loading discipline.

## Non-functional requirements

- **Performance** — FR-7's reserved-dimension containers bound layout
  shift from network images to zero regardless of load latency; no
  separate numeric image-load budget is fixed here, since
  `backend-metadata-caching.md`'s server-side cache is what actually
  governs real latency, not anything this screen controls.
- **Security** — see dedicated section below.
- **Accessibility** — `frontend-accessibility.md`'s existing keyboard
  map and screen-reader conventions apply unchanged: search input has
  an accessible label, results are announced as a live region when a
  new search completes, pagination controls are reachable and labelled,
  the degraded states (FR-5) are announced, not just visually styled.
- **Reliability** — FR-5's degraded states are this screen's entire
  reliability story; no client-side retry-with-backoff is implemented
  here beyond the single explicit user-triggered retry action, since
  `backend-metadata-adapter.md` FR-9's own `Unavailable` mapping already
  represents "this system decided the request should stop being retried
  automatically" — a client auto-retry would fight that decision.
- **Observability** — no new client-side logging beyond
  `frontend-shell-and-routing.md` FR-7's existing correlation-ID-in-
  error-state UI slot, reused unchanged for `/api/v1/discover*` errors.

## Domain model

No new domain types on the frontend beyond the wire shapes
`backend-metadata-adapter.md` already defines
(`NormalisedSearchResult`, `NormalisedWork`, `NormalisedEdition`,
`NormalisedAuthor`) — this screen renders those directly, with no
client-side transformation beyond what TanStack Query's own cache
requires. `<DiscoverResultGrid>` and the work-detail view have no
concept of "in library" or "collection membership" — those facts don't
exist for a work the user hasn't added, and FR-1 deliberately keeps
this component from pretending otherwise.

## API and contracts

Consumes `backend-metadata-adapter.md`'s `GET /api/v1/discover` and
`GET /api/v1/discover/works/:openLibraryId`, and
`backend-metadata-caching.md`'s `GET /api/v1/discover/covers/:coverId`
(the `coverUrl` field's target, requested by the browser directly as an
`<img src>`, not fetched via TanStack Query — it's an image resource,
not JSON data). No new contract beyond what those two specs already
fix; this spec adds no backend surface of its own.

## State transitions

`/discover`: `idle (no query) → loading → results | empty (zero
results, a distinct "no matches" state from "unavailable") | degraded
(FR-5)`. `/discover/works/:openLibraryId`: `loading → detail | not-found
(FR-5) | degraded (FR-5)`. No state here persists across a route change
beyond what the URL (FR-2) and TanStack Query's own request cache
already carry — no Discover-specific client state survives a reload
beyond the search query itself.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Open Library unavailable (backend returns `503`) | TanStack Query error state on the `/api/v1/discover*` call | "Open Library is unavailable right now, try again shortly" with a retry action | No automatic retry beyond TanStack Query's own default (bounded, not infinite); the named state renders on final failure |
| Work not found (`404`) | Query error with `NotFound` code | "This book couldn't be found on Open Library" | Renders the not-found state; no retry action offered (retrying an unchanged ID won't change the outcome) |
| Zero search results | `200` with empty `items` | "No results for '<query>'" — distinct from the unavailable state | Renders the empty state, no error logged (not a failure) |
| A result or work-detail item has a missing field (no cover, no description, no authors) | Field simply absent in the normalised response | That field's UI section is omitted, not rendered empty or as a placeholder text | No special handling needed — FR-1/FR-4 already only render what's present |
| Real cover image fails to load client-side (network blip after a valid `coverUrl` was returned) | `<img>` `onError` | Falls back to the procedural generated cover, same as a genuinely absent `coverUrl` | FR-7's fallback path, no distinct error shown to the user for this specific case |

## Security considerations

- **No new trust boundary from the browser's perspective**: this screen
  talks only to this system's own `/api/v1/discover*` endpoints, never
  directly to Open Library — the browser never learns Open Library's
  hostname or makes a cross-origin request to it; `backend-metadata-adapter.md`
  and `backend-metadata-caching.md` are the only things that ever
  reach Open Library.
- **Rendered content is normalised, not raw**: every string this screen
  renders (title, description, subjects) has already passed through
  `backend-metadata-adapter.md`'s domain-model construction-time
  validation (length bound, control-character rejection) before this
  screen ever sees it — React's own default text-escaping is still the
  primary XSS defense (no `dangerouslySetInnerHTML` anywhere in this
  spec), but the upstream validation means this screen isn't the first
  line of defense against a hostile Open Library field either.
- **Cover image requests go through this system's own proxy endpoint**
  (`backend-metadata-caching.md` FR-6), never a direct `<img src>` to
  `covers.openlibrary.org` — keeps every outbound request to Open
  Library funneled through the rate-limited, cached adapter path, and
  means a LAN client never needs direct internet access to Open Library
  to see a cover, only to this host.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | `<DiscoverResultGrid>` rendering with/without covers, with/without authors; degraded-state components render the correct copy for each of FR-5's distinct cases |
| Integration | Search → results → click-through → detail flow against MSW fixtures shaped exactly like `backend-metadata-adapter.md`'s real response types (tier-(a) contract-generated, per `frontend-shell-and-routing.md`'s existing two-tier mock strategy) |
| Contract | Reuses `backend-metadata-adapter.md`'s `kin-openapi` contract test — no separate frontend contract test, per this project's existing pattern |
| E2E | Search Discover → open a result → view detail, against the real backend and a fake Open Library server (`backend-metadata-adapter.md`'s own E2E fixture); a second E2E run with the fake server returning `503` to exercise FR-5's degraded path visually, not just at the API layer |
| Accessibility | `frontend-accessibility.md`'s `axe`/keyboard-map requirements applied to search, results, pagination, and each degraded state |

Tests that must fail before implementation begins: a test asserting a
work with no `description` renders no description section at all (not
an empty one); a test asserting the `503` and `404` states render
distinct, differently-worded copy; a test asserting a failed cover
image load falls back to the procedural cover, not a broken-image icon.

## Acceptance criteria

- [ ] Searching Discover returns and renders real Open Library results
- [ ] Clicking a result shows normalised work detail with real cover
      images where available
- [ ] Offline/unavailable, not-found, and empty-results states are each
      visually and textually distinct
- [ ] A missing field (cover, description, authors) degrades that field
      only, never the whole screen
- [ ] The Discover route's MSW mock and `TODO`-marked fixtures are
      fully removed
- [ ] Accessibility checks pass for search, results, pagination, and
      every degraded state

## Open questions

- **No design-reference screen for Discover** — `.design-reference/`
  (per `CLAUDE.md`'s own note) covers Library, Collections, and other
  screens but has no captured Discover layout; FR-3's click-through-
  pagination choice and FR-1's grid layout are this spec's own
  reasoned defaults, not derived from a design capture. Escalate for a
  Claude Design project owner decision if a captured Discover screen
  later contradicts this spec's choices — same standing instruction
  every earlier phase's design-reference gaps have carried.
- **No "add to library" action yet** — a Discover result is currently a
  dead end past viewing it; acceptable for this phase (phase 10 owns
  that action) but worth flagging so it isn't mistaken for an oversight
  during review.
- **Click-through vs. infinite scroll for Discover specifically** — this
  spec chose click-through over infinite scroll partly because
  Open Library's `num_found` can be very large (tens of thousands for a
  broad query) and unbounded infinite scroll against an external,
  rate-limited API felt like the wrong default; open to revision once a
  design capture exists.

## References

- `backend-metadata-adapter.md` (phase 07) — `/api/v1/discover*` shapes
  this screen renders
- `backend-metadata-caching.md` (phase 07) — cover image proxy endpoint
- `frontend-library-screens.md` (phase 06) — `<WorkGrid>` precedent,
  URL-as-state pattern, infinite-scroll precedent (and why it doesn't
  transfer here)
- `frontend-shell-and-routing.md` (phase 04) — shell composition, MSW
  two-tier mock strategy, correlation-ID error UI slot
- `frontend-generated-covers.md` (phase 04) — procedural-cover fallback
  ladder, reused for missing/failed real covers
- `frontend-accessibility.md` (phase 04) — keyboard/screen-reader
  requirements applied here
- `architecture-frontend.md` (phase 01) — URL-as-state, UI-local-vs-
  server-data split
- Constitution §11 (interface copy), §7 (accessibility as part of done)
