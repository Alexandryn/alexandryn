# Phase 06 — Library

| | |
|---|---|
| **Status** | Specs approved, implementation not started |
| **Depends on** | Phase 03, Phase 04 |
| **Blocks** | 07, 08, 11 |
| **Opened** | — |
| **Closed** | — |

## Objective

A working library backed by the real Go server and real PostgreSQL data —
browse, search, filter, sort, view a work's detail, and manage
collections — with zero external integration. At the end of this phase,
`domain-library.md`'s model (phase 02) has a real API in front of it
(this phase's own backend spec) and a real screen behind it (this
phase's own frontend specs), and the mock-backed shell phase 04 built is
retired in favor of the genuine article. This is the first point the
domain and backend foundation are proven to actually compose, not just
independently pass their own test suites.

## Why here

It needs a real backend to call (phase 03) and a real shell to build
inside (phase 04), so it waits for both — same reasoning
`architecture-frontend.md` already gave for why phase 04 itself couldn't
start earlier. It sits before phase 07 (metadata) and phase 08 (sources)
deliberately: this phase proves the domain model and the local-data path
work end to end with **zero** external integration, so a defect later
can be isolated to "the Open Library adapter" or "the source protocol"
rather than being tangled up with "does browsing even work." It sits
before phase 11 (reader) because reading requires something to read,
which requires a library screen to reach a book from.

## Scope

**In**

- `GET /api/v1/library` — browse/search/filter/sort over `Work`s,
  paginated, including both owned (`LibraryEntry`-backed) and "want to
  read" (`Collection`-only membership, `domain-library.md`'s own named
  case) works per the endpoint's own closed filter vocabulary
- `GET /api/v1/works/:id` — work detail, including which `Edition`s are
  owned and the work's collection memberships
- `POST`/`GET`/`PATCH`/`DELETE /api/v1/collections` and membership
  endpoints — collection create/edit/delete, `Work` add/remove
- Library home, work detail, collections index, collection detail
  screens — the screens phase 04's shell was built to eventually host,
  now with real content
- Search, filter, sort, and grid/list view toggle, wired to the real API
  (retiring `frontend-shell-and-routing.md` FR-6's MSW mock layer for
  every endpoint this phase's backend spec actually implements)
- The first real exercise of `architecture-contracts.md`'s contract test
  against endpoints that aren't just `/healthz`

**Out**

- Open Library / any external metadata — phase 07
- Remote sources, source configuration — phase 08
- Import (discovery → extraction → matching → confirmation) — phase 10;
  this phase's library data is assumed to already exist (seeded via
  fixtures/tests), not arrived at through a real import flow
- Reading, position, preferences — phase 11
- Any authentication/authorization on these endpoints — phase 12; same
  loopback-trust model every earlier phase has used

## Specifications

| Spec | Covers |
|---|---|
| `backend-library-api.md` | `/api/v1/library`, `/api/v1/works/:id`, `/api/v1/collections` endpoints — shapes, query params, pagination, repository wiring |
| `frontend-library-screens.md` | Library home, work detail: browse/search/filter/sort/grid-list-view, wired to the real API |
| `frontend-collections-screens.md` | Collections index, collection detail, create/edit/membership UI |

## Architecture decisions expected

- Pagination mechanism (offset/limit vs. cursor-based) for
  `GET /api/v1/library` — `architecture-contracts.md` didn't fix this,
  since no paginated endpoint existed yet
- Search implementation: PostgreSQL full-text search (`tsvector`/`tsquery`,
  no new dependency) versus a dedicated search library — a real choice
  this phase has to justify under constitution §9, not assumed
- Filter/sort query-parameter shape — a small, closed vocabulary
  (`sort=title|added_at`, `filter=owned|wanted|all`) versus a more generic
  query language; `backend-library-api.md`'s own to decide, leaning
  toward the closed vocabulary given this project's own preference for
  small, explicit surfaces over generic ones (constitution §5's spirit
  applied beyond IPC specifically)

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Search/filter/sort query parameters grow into an ad hoc, undocumented mini-language as real screens reveal new needs | Medium | Medium | `backend-library-api.md` fixes a closed parameter vocabulary up front; a new parameter is a spec amendment, not a silent addition |
| `LibraryEntry`/`Collection` repository methods (already speced in `backend-persistence.md`) turn out not to support an access pattern a real screen needs (e.g. a join `domain-library.md` didn't anticipate) | Medium | Medium | This phase's own backend spec is where that gap would surface; treated as a `backend-persistence.md` amendment if found, following this project's established cross-phase-amendment discipline (review `0032`), not worked around ad hoc in a handler |
| The MSW-mocked frontend (phase 04) and the real backend disagree on response shape in some field the mock happened to get "close enough" | Medium | High — exactly the risk `frontend-shell-and-routing.md` FR-6's contract-generated-fixture design exists to prevent | `architecture-contracts.md` FR-3's contract test, exercised for real against these endpoints for the first time this phase, is the concrete check |
| Grid/list view and responsive layout (desktop/tablet/mobile) reveal a genuine `atTablet` requirement the design reference's placement question (still open since phase 01) can no longer be deferred | Medium | Medium | If this phase is blocked on it, escalate for a Claude Design project owner decision rather than guessing — same standing instruction every earlier phase's Open questions have carried |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Query-parameter parsing/validation (`backend-library-api.md`), component rendering logic (both frontend specs) |
| Integration | Full endpoint-to-repository round trip against a real PostgreSQL instance (`backend-test-harness.md`'s harness); frontend screens against the real backend, not MSW, for the first time |
| Contract | `architecture-contracts.md` FR-3's contract test, now exercising real endpoints beyond `/healthz` |
| E2E | Browse → search → open a work → view detail → add to a collection → view the collection — the fullest version yet of `architecture-system.md`'s own "open a book" reference walkthrough |
| Accessibility | `frontend-accessibility.md`'s `axe`/keyboard-map requirements, applied to real screens with real (if fixture-seeded) data density, not the empty/mock states phase 04 tested against |

The hardest thing to test here is the same class of problem
`frontend-generated-covers.md` already named for phase 04: a library
screen with real data density (hundreds of works, grid view) has to
stay responsive and accessible, not just functionally correct — a
concern closer to a performance/UX review than a pure correctness test,
inherited and extended rather than newly invented this phase.

## Security considerations

Small incremental surface, but the first time real domain data crosses
the wire:

- **Every query parameter is untrusted input** — constitution §4 applied
  to a real endpoint for the first time in this project: search terms,
  filter values, sort keys all get shape/bound validation before
  reaching a repository query, per `backend-persistence.md` FR-3's
  parameterized-query discipline
- **Collection names remain hostile input** — `domain-library.md`'s own
  Security considerations already named this; this phase is where a real
  HTTP boundary actually receives one
- **No new trust boundary** — still loopback-only, still no
  authentication, same accepted model every phase through 11 shares;
  named again here only because this is the first phase where real user
  data (a library, not just process/config state) crosses it

## Observability

Per-request logging already established (`backend-errors-and-logging.md`)
extends naturally to these endpoints; nothing new required beyond
ensuring the correlation-ID-in-error-state UI slot
(`frontend-shell-and-routing.md` FR-7) has a real error to display when
this phase's own endpoints produce one, not just a mock.

## Exit criteria

- [ ] All three specifications `APPROVED` with recorded reviews
- [ ] Browse, search, filter, sort functional against real local data
- [ ] Work detail and collection management functional
- [ ] Responsive across desktop, tablet, mobile layouts
- [ ] The contract test passes against these real endpoints, not just
      `/healthz`
- [ ] The tier-(b) hand-written MSW fixtures for every endpoint this
      phase implements are removed entirely, `TODO(phase-06)` markers
      gone — production builds already excluded MSW regardless (phase
      04's own build-time check), what this phase actually changes is
      the dev/test fixture tier, not shipped-build composition
- [ ] Test coverage across unit, integration, and E2E for this slice
- [ ] All specs in this phase are `VERIFIED`
- [ ] Security audit recorded in `.claude/audits/` with no open Critical
      or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
