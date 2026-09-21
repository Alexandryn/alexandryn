# Phase 07 — Metadata

| | |
|---|---|
| **Status** | Specs approved, implementation not started |
| **Depends on** | Phase 06 |
| **Blocks** | 10 |
| **Opened** | — |
| **Closed** | — |

## Objective

An Open Library adapter that searches, fetches, normalises, and caches
work and edition metadata, with the normalisation boundary (constitution
§3) preventing Open Library's response shapes from reaching the domain
or the UI directly. At the end of this phase, a user can search Open
Library from a Discover screen, see normalised results backed by the
phase 02 domain model, and the adapter behaves correctly offline,
rate-limited, or given malformed upstream responses.

## Why here

It needs `domain-bibliographic.md`'s `Work`/`Edition`/`Author` model
(phase 02) as its normalisation target, and it needs a working library
(phase 06) so a Discover result has somewhere real to attach to later
(phase 10, import). It sits before phase 08 (sources) and phase 10
(import) deliberately: phase 06 proved the local-data path with zero
external integration, this phase adds exactly one external integration
(Open Library, metadata only, no files), and phase 08 adds a structurally
different one (source protocols, file acquisition) — keeping them
sequential means a defect is isolated to one adapter, not tangled with
another. It sits before phase 10 because import's matching step needs a
metadata search/lookup to match against.

## Scope

**In**

- Open Library HTTP client: Search API (`/search.json`), Works API
  (`/works/OL...W.json`), Editions API (`/works/OL...W/editions.json`,
  `/books/OL...M.json`), Authors API (`/authors/OL...A.json`), Covers API
  (`covers.openlibrary.org/b/...`)
- Rate limiting and self-identification per Open Library's usage policy
  (1 req/s default, higher with an identified `User-Agent` + contact
  info), timeout, retry-or-fail behaviour
- Normalisation of Open Library response shapes into `domain-bibliographic.md`'s
  `Work`/`Edition`/`Author` types, enforced at the adapter boundary — no
  raw Open Library JSON crosses it
- Local caching of normalised metadata and of cover images, so repeat
  lookups and repeat renders don't re-hit Open Library
- `GET /api/v1/discover` (or equivalent) — search Open Library through
  the adapter, returning normalised, paginated results
- `GET /api/v1/discover/works/:openLibraryId` — fetch-and-normalise a
  single work (and its editions) on demand, for the case a search result
  is clicked into
- Discover screen: search, results grid, work detail view, offline /
  rate-limited / partial-data UI states
- Partial-data handling: an Open Library record missing a field the
  domain model expects (e.g. no cover, no author) is not a fetch
  failure — construct what's constructible per `domain-bibliographic.md`
  FR-3/FR-7 (zero-edition Works, zero-author Works are legal)

**Out**

- File acquisition of any kind — Open Library is metadata only. Getting
  an actual book file is phase 08 (sources) and phase 10 (import).
- Matching a Discover result against an existing library `Work` (dedup,
  "is this the same book I already own") — phase 10; `domain-bibliographic.md`
  FR-4 explicitly reserves that decision for later
- Any custom or user-configured metadata source — this phase is Open
  Library only, per constitution §3's source/metadata separation
- Bulk or scheduled metadata refresh — this phase caches what's been
  looked up, not a background sync job

## Specifications

| Spec | Covers |
|---|---|
| `backend-metadata-adapter.md` | Open Library client, rate limiting, normalisation, the `/api/v1/discover*` endpoints |
| `backend-metadata-caching.md` | Local storage/invalidation of normalised metadata and cover images |
| `frontend-discover-screen.md` | Discover screen: search, results, work detail, offline/degraded states |

## Architecture decisions expected

- Cache storage: a new PostgreSQL table (reusing `backend-persistence.md`'s
  existing engine, no new dependency) versus a separate cache store —
  `backend-metadata-caching.md`'s to justify under constitution §9
- Cache key and identity: Open Library's own key (`OL...W`, `OL...M`,
  `OL...A`) as the cache key, distinct from the domain's internal IDs
  (`domain-bibliographic.md` FR-1/FR-2 already keep these separate) —
  `backend-metadata-caching.md` fixes how a cache row maps to that
  external reference
- Cover image storage: cache the image bytes locally (disk, under the
  same app-data root `architecture-persistence.md` already owns) versus
  caching only the upstream URL and re-fetching on render —
  `backend-metadata-caching.md`'s to decide, weighed against Open
  Library's usage policy discouraging being treated as a bulk image CDN
- Rate limiter shape: a simple token bucket in-process (no new
  dependency) versus a library — `backend-metadata-adapter.md`'s to
  justify under constitution §9, leaning toward stdlib given this
  project's established preference (`architecture-backend.md`)

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Open Library response shape drifts from what this phase's specs assume (fields renamed, removed, or newly missing) | Medium | High | `backend-metadata-adapter.md`'s normalisation layer treats every field as optional at the boundary and fails closed per-field, not per-response — a missing field degrades that field, not the whole lookup |
| Rate limit exceeded during normal use (a user searching repeatedly) causes visible failures | Medium | Medium | `backend-metadata-adapter.md` fixes an in-process limiter below Open Library's stated ceiling, queuing or rejecting locally before Open Library ever sees an excess request |
| Cover image caching grows unbounded local disk usage over time | Medium | Low | `backend-metadata-caching.md` fixes an eviction/bound policy, not unlimited retention |
| A malformed or hostile-shaped upstream response (oversized payload, unexpected type, deeply nested JSON) reaches the adapter | Low | High | Constitution §4 applied to this external boundary for the first time in this project — size limit, shape check, timeout, named explicitly in `backend-metadata-adapter.md`'s Security considerations |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Response normalisation (real Open Library response fixtures, not invented shapes), rate limiter behaviour, cache key derivation |
| Integration | Adapter-to-cache round trip against a real PostgreSQL instance (`backend-test-harness.md`'s harness); Open Library itself is never called in tests — a recorded-fixture or fake HTTP server stands in, so tests stay deterministic and don't depend on, or hammer, the real upstream |
| Contract | `architecture-contracts.md` FR-3's contract test, extended to `/api/v1/discover*` |
| E2E | Search Discover → open a result → view normalised detail — offline and rate-limited variants included, not just the happy path |
| Accessibility | `frontend-accessibility.md`'s requirements applied to Discover's search, results grid, and degraded-state UI |

## Security considerations

The first phase where an external, third-party network boundary exists:

- **Every Open Library response is hostile input** — constitution §4:
  size limit, shape check, timeout, applied to this boundary explicitly,
  not assumed inherited from phase 03's internal-only discipline
- **The normalisation boundary is the enforcement point** — constitution
  §3: nothing downstream of `backend-metadata-adapter.md` ever sees a
  raw Open Library JSON object; a normalisation failure produces a
  domain-shaped error, not a passthrough
- **Self-identification, not spoofing** — the `User-Agent` and contact
  info this phase sends upstream per Open Library's usage policy
  identifies this application truthfully; it is not a bypass mechanism
- **No user credentials cross this boundary** — Open Library requires no
  authentication for the endpoints this phase uses; if that changes, it
  is a spec amendment, not a silent addition of a new secret
- **No new trust boundary on the loopback side** — still loopback-only,
  still no authentication between the LAN client and this host, same
  accepted model every phase through 11 shares

## Observability

Per-request logging (`backend-errors-and-logging.md`) extends to
`/api/v1/discover*`; additionally, rate-limit rejections and
normalisation failures (a specific field dropped, not the whole
response) are logged at a level that lets a maintainer tell "Open
Library is down" apart from "Open Library changed shape" without
reading raw response bodies — constitution §8 still applies: no source
credentials, no reading history, nothing sensitive in these logs, and
metadata logging in particular must not log what a user searched for as
personally identifying behaviour beyond what's needed for the immediate
request's own error context.

## Exit criteria

- [ ] All three specifications `APPROVED` with recorded reviews
- [ ] Search and lookup functional with local caching of metadata and
      covers
- [ ] No raw Open Library JSON reaches the UI, enforced at the adapter
      boundary and checked by a test, not just by code review
- [ ] Degrades cleanly offline, rate-limited, or given malformed
      responses
- [ ] The contract test passes against `/api/v1/discover*`
- [ ] Test coverage across unit, integration, and E2E for this slice
- [ ] All specs in this phase are `VERIFIED`
- [ ] Security audit recorded in `.claude/audits/` with no open Critical
      or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
