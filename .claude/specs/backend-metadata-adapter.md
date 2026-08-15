# Spec: Backend metadata adapter

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

`domain-bibliographic.md` (phase 02) fixed the domain model this adapter
normalises into: `Work` and `Edition` each carry an internal identifier as
primary identity, with an optional external reference (an Open Library
key) attached — never derived, never required. `architecture-backend.md`
(phase 01) fixed the package-layout and dependency rules a new external
client must respect. `backend-errors-and-logging.md` (phase 03) fixed the
error taxonomy (`NotFound`, `InvalidInput`, `Unauthorized`, `Conflict`,
`Unavailable`, `Internal`) this adapter's failures must map into.
Constitution §3 draws the line this whole spec exists to enforce: metadata
(what a book is, from Open Library) and the domain (the user's library)
are separate concerns, and nothing may let one leak into the other
unexamined.

Open Library's real API surface, confirmed against its own documentation
(`openlibrary.org/dev/docs/api/search`, `openlibrary.org/dev/docs/restful_api`,
`openlibrary.org/dev/docs/api/covers`, `openlibrary.org/developers/api`)
rather than assumed:

- **Search**: `GET https://openlibrary.org/search.json?q=<query>&fields=<csv>&limit=<n>&offset=<n>`
  → `{ start, num_found, docs: [{ key, title, author_name[], author_key[],
  first_publish_year, edition_count, cover_i, ia[], has_fulltext,
  language[], ... }] }`. `key` is a work key (`/works/OL...W`).
- **Work**: `GET https://openlibrary.org/works/OL...W.json` → work-level
  fields (`title`, `description`, `subjects[]`, `authors[]` as `{author:
  {key}}` references, `covers[]` as numeric cover IDs).
- **Editions of a work**: `GET https://openlibrary.org/works/OL...W/editions.json?limit=<n>`
  → a list of edition records.
- **Edition**: `GET https://openlibrary.org/books/OL...M.json` →
  edition-level fields (`title`, `publishers[]`, `publish_date`,
  `languages[]` as `{key: "/languages/eng"}` references, `covers[]`,
  `works[]` back-reference).
- **Author**: `GET https://openlibrary.org/authors/OL...A.json` → `{
  name, key, ... }`.
- **Covers**: `GET https://covers.openlibrary.org/b/$key/$value-$size.jpg`
  where `$key` is `isbn`/`olid`/`id`/`oclc`/`lccn`, `$size` is `S`/`M`/`L`.
  A missing cover returns a blank placeholder image by default;
  `?default=false` returns `404` instead — this adapter always requests
  `?default=false`, so a missing cover is a detectable absence, not a
  silently-served blank image mistaken for real data.
- **Usage policy**: unauthenticated requests are rate-limited to roughly 1
  request/second; an identified client (a descriptive `User-Agent`
  including an application name and contact email) gets a materially
  higher ceiling. Open Library's policy explicitly discourages bulk or
  commercial scraping and asks integrators to cache rather than
  re-request. This adapter is built around both facts: identify itself,
  and cache (`backend-metadata-caching.md`).

## Problem

Nothing exists yet to reach Open Library, turn its responses into
`domain-bibliographic.md` shapes, or protect the rest of the system from
a slow, wrong, or hostile response on the way in.

## Goals

- An HTTP client for Open Library's Search, Works, Editions, Authors, and
  Covers endpoints, self-identified per its usage policy
- A rate limiter that keeps this application under Open Library's stated
  ceiling regardless of how many concurrent requests the UI generates
- A normalisation layer translating Open Library JSON into
  `domain-bibliographic.md`'s `Work`/`Edition`/`Author` types, field by
  field, with every field treated as independently optional
- `GET /api/v1/discover` (search) and `GET /api/v1/discover/works/:openLibraryId`
  (fetch-and-normalise one work) as the HTTP surface the frontend calls
- A hard boundary: no handler, repository, or UI component downstream of
  this adapter package ever receives a raw Open Library response body

## Non-goals

- Caching itself — `backend-metadata-caching.md` owns storage,
  invalidation, and the cache-first lookup strategy; this spec only
  defines what gets cached (the normalised shape) and calls the cache
  interface that spec fixes
- Matching a Discover result against an existing library `Work` — phase
  10; `domain-bibliographic.md` FR-4 reserves that decision explicitly
- Any source other than Open Library — constitution §3, no exception
- Author disambiguation beyond what Open Library's own `author_key`
  provides — this adapter records what Open Library asserts, it does not
  resolve conflicting Open Library records against each other
- Full-text or specialised search operators beyond `?q=` — Open Library's
  own search grammar (author:, title:, etc.) is out of scope for this
  phase; a single free-text query is what FR-1 sends

## User stories

- As **someone using Discover**, I want to search for a book by title or
  author and see real results, so I can find something to add to my
  library later.
- As **the maintainer**, I want a field Open Library removes or renames
  tomorrow to degrade one field of one result, not break Discover
  entirely, so an upstream change doesn't take down a whole screen.
- As **the system**, I want to never send more than the allowed request
  rate to Open Library, so this application never gets rate-limited or
  blocked for bad behaviour.

## Functional requirements

- **FR-1** `GET /api/v1/discover?q=<string>&limit=<n>&offset=<n>` MUST
  proxy to Open Library's Search API (`/search.json?q=<q>&fields=key,title,
  author_name,author_key,first_publish_year,cover_i,edition_count,language&limit=<limit>&offset=<offset>`),
  MUST validate `q` is present, non-empty, and ≤ 200 characters (same
  bound `backend-library-api.md` FR-2 places on its own search field, for
  consistency across the two search surfaces), MUST validate `limit` is
  an integer in `[1, 50]` (default `20`) and `offset` is a non-negative
  integer (default `0`), and MUST return `InvalidInput` (per
  `backend-errors-and-logging.md` FR-1) for any violation. The response
  body is `{ items: NormalisedSearchResult[], total: number, limit:
  number, offset: number }`, where `total` is Open Library's `num_found`
  passed through as-is (an upstream-reported count, not a claim this
  system's cache holds that many rows). **Offset pagination, not
  `backend-library-api.md` FR-1's cursor pagination**: a deliberate
  divergence, not an inconsistency — this endpoint has no local table to
  paginate against; it is a thin, per-request proxy over Open Library's
  own Search API, which itself is offset/limit-based (confirmed against
  `openlibrary.org/dev/docs/api/search`) and returns no cursor of its
  own to forward. `backend-library-api.md` FR-1's cursor rationale
  (rows gained mid-browse causing skip/duplicate under offset
  pagination) doesn't transfer: Open Library's own index changing
  between two of this system's requests is an accepted, unfixable-from-
  here property of querying someone else's live search index, not a
  gap this endpoint's own pagination scheme could close either way.
- **FR-2** Each `NormalisedSearchResult` MUST contain only
  `domain-bibliographic.md`-shaped fields: `openLibraryWorkKey` (string,
  from `key`), `title` (validated per `domain-bibliographic.md` FR-6),
  `authors: NormalisedAuthor[]` (from `author_key[]`/`author_name[]`,
  zip-matched by index — `domain-bibliographic.md` FR-7 already makes
  zero authors legal, so an empty or mismatched-length pair produces
  zero authors, not an error), `firstPublishYear` (nullable integer),
  `coverUrl` (nullable string, FR-8 constructs it), and `editionCount`
  (integer, defaulting to `0` if absent). No other field of the Open
  Library `docs[]` entry MUST reach the response body.

  **`NormalisedAuthor`** is one shape, reused everywhere an author
  appears in this spec's output (this FR's search results, FR-3's
  work-detail authors): `{ openLibraryAuthorKey: string | null, name:
  string }`. `openLibraryAuthorKey` is `null` for a search-result author
  built from `author_key[]`/`author_name[]` zip-matching when Open
  Library's own arrays are mismatched in length for that index (a known
  Open Library data-quality gap, not this adapter's error); it is always
  present for an author resolved via FR-3's own Authors-API call, since
  that call is keyed by `author_key` to begin with.
- **FR-3** `GET /api/v1/discover/works/:openLibraryId` MUST validate
  `openLibraryId` matches Open Library's work-key shape (`OL` + digits +
  `W`) and return `InvalidInput` otherwise. On a valid ID, it MUST fetch
  `/works/OL...W.json`, normalise it into a `NormalisedWork` (`title`,
  `subtitle`, `description`, `subjects: Subject[]`, `authors:
  NormalisedAuthor[]`, `coverUrl`), and MUST additionally fetch
  `/works/OL...W/editions.json?limit=50` and normalise each into a
  `NormalisedEdition` (`title`, `publisher`, `publishDate`, `language`,
  `openLibraryEditionKey`, `coverUrl`). Editions beyond the first 50 MUST
  be silently omitted for this phase — no pagination of an individual
  work's edition list — flagged in Open questions as a placeholder, not a
  considered limit.

  `NormalisedWork.authors` is resolved via one Authors-API call per
  distinct `author.key` the work references, **capped at the first 20
  distinct authors** referenced (order as listed by Open Library) — a
  work with more than 20 credited authors is a genuine edge case, not
  the common path this cap is sized against, and the cap exists
  specifically to bound this FR's own fan-out against FR-7's shared rate
  limiter (a large-author-count work must not be able to monopolise the
  budget every other concurrent lookup and search also draws from).
  Authors beyond the cap are simply omitted from the list, the same
  "degrade that field, not the request" posture FR-4 already applies
  elsewhere. A single author's own fetch failing (FR-9's `Unavailable`,
  or a `404` for a since-removed author record) MUST degrade to that one
  author being omitted from `authors` — never fail FR-3's request as a
  whole — matching `domain-bibliographic.md` FR-7's "zero authors is
  legal" reasoning extended to "fewer authors than expected is legal."
- **FR-4** A field present in the upstream response but failing
  `domain-bibliographic.md`'s own construction-time validation (FR-5/FR-6:
  length bound, control-character rejection, BCP-47 language tag shape)
  MUST be dropped from the normalised result, not cause the whole
  request to fail. This is the concrete mechanism for "every field is
  independently optional": a Work with an oversized or malformed
  `description` becomes a Work with no `description`, not a `500`.

  This same per-field degradation MUST hold for numerically-typed fields
  (`cover_i`, `first_publish_year`, `edition_count`), not just strings:
  the JSON response is decoded into a permissive intermediate form
  (`map[string]any`/`json.RawMessage` per field, not a single strict
  struct unmarshal) so that a type-mismatched value for one field (e.g.
  Open Library returning a string where this system expects a number)
  fails only that field's own typed conversion — dropped per this FR —
  rather than aborting `encoding/json`'s unmarshal for the entire
  response, which a single strict struct tag would otherwise do.
- **FR-5** A field absent from the upstream response that
  `domain-bibliographic.md` requires for construction (a `Work` needs a
  `title` — FR-6's constrained-string validation still applies, but
  `Work` has no "optional title" case) MUST cause that one item to be
  omitted from a list response (FR-1's `items`, FR-3's editions list)
  rather than failing the whole request; for FR-3's own top-level Work
  fetch, a missing `title` MUST return `Internal` (this system asserting
  something Open Library's own schema guarantees, so its absence signals
  a normalisation bug or a genuinely corrupt upstream record worth
  surfacing loudly rather than silently dropping the entire lookup).
- **FR-6** The client MUST send a `User-Agent` header identifying this
  application by name and a maintainer contact (email or project URL),
  per Open Library's usage policy. This value is read from a new
  configuration key, **`OPEN_LIBRARY_USER_AGENT`** — an amendment to
  `backend-configuration.md` FR-4's authoritative key table, classified
  as *required, no default* (that spec's FR-3 category for a value this
  system cannot safely fabricate: a placeholder default would itself
  violate Open Library's usage policy by misidentifying the client, so
  "required" is the only category consistent with FR-6's own "never
  hardcoded to a placeholder" rule). Startup MUST fail
  (`backend-service-lifecycle.md`'s existing fail-to-start path) if this
  key is unset, the same discipline every other required key already
  gets. This header value MUST NOT be logged at a level a reader could
  mistake for a secret — it is not one, it is meant to be visible to
  Open Library — but this spec deliberately excludes it, and outbound
  request headers generally, from routine request logging as its own
  reasoned choice, not because `backend-errors-and-logging.md` mandates
  it (that spec's FR-8 redaction contract covers specific typed secret
  values, not outbound headers as a category).
- **FR-7** The client MUST rate-limit outbound Open Library requests
  (Search, Works, Editions, Authors, and Covers combined — one shared
  budget, since they all count against the same upstream policy) to no
  more than 3 requests/second, enforced by an in-process token bucket
  (`golang.org/x/time/rate`, justified under constitution §9 in Non-
  functional requirements below). A request arriving when the bucket is
  exhausted MUST wait up to a 5-second budget for a token before
  returning `Unavailable`; it MUST NOT queue unboundedly. Concretely:
  concurrent waiters for a token are bounded to **50** in-flight —
  itself bounded upstream by `backend-http-transport.md`'s existing
  concurrent-connection limits on this system's own inbound listener,
  so this figure is a defensive backstop, not this spec's primary
  control. A request arriving when 50 requests are already waiting MUST
  return `Unavailable` immediately, without joining the wait queue —
  this is what "MUST NOT queue unboundedly" concretely means, named as
  a number rather than left as an unmeasured intention. FR-3's own
  author-fetch cap (20 distinct authors per work-detail request) is the
  other concrete control against one caller's request monopolising this
  shared budget.
- **FR-8** `coverUrl` fields (FR-2, FR-3) MUST be constructed as **this
  system's own path**, `/api/v1/discover/covers/<cover_i>`
  (`backend-metadata-caching.md` FR-6's endpoint), when a numeric cover
  ID is present, and MUST be omitted (not an empty string) when absent.
  `coverUrl` MUST NOT be the literal Open Library covers URL
  (`covers.openlibrary.org/...`) — every cover request from a browser or
  LAN client goes through this system's own endpoint, never directly to
  Open Library, so that FR-7's rate limiter and
  `backend-metadata-caching.md`'s disk cache actually see and govern
  every cover fetch. This adapter MUST NOT itself fetch cover image
  bytes as part of a discover/search/work-detail request —
  `backend-metadata-caching.md` owns if/when the bytes are fetched from
  Open Library and cached, including constructing the actual
  `covers.openlibrary.org` URL server-side on a cache miss; this spec
  only ever hands back its own local path.
- **FR-9** Any Open Library HTTP call that times out (5-second budget per
  call — the same figure `backend-http-transport.md` FR-2 uses for this
  system's own *inbound* listener timeouts, reused here for consistency
  across the codebase's timeout values, not because that spec's
  discipline itself governs outbound third-party calls), returns a
  non-2xx status, or returns a
  response exceeding a 5 MiB body-size cap (constitution §4's shape/size
  check, applied to this external boundary) MUST map to `Unavailable`
  for that lookup — not `Internal` — since the failure is upstream, not
  this system's own defect, and `Unavailable`'s existing wire mapping
  (`backend-errors-and-logging.md`) already communicates "try again
  later" to the frontend without further design here.

## Non-functional requirements

- **Performance** — a single Discover search MUST complete in one
  upstream round trip (FR-1); a work-detail fetch (FR-3) is bounded to
  1 (work) + 1 (editions list) + up to 20 (authors, deduplicated by key,
  FR-3's own cap) upstream calls, each independently subject to FR-7's
  rate limit — named here as a real cost, not hidden, since a work with
  many distinct authors could serialise several author fetches behind
  the rate limiter; `backend-metadata-caching.md`'s cache is what keeps
  repeat lookups from re-paying this cost, not this spec.
- **Security** — see dedicated section below.
- **Accessibility** — not directly applicable to a backend adapter; the
  Discover screen consuming this API is `frontend-discover-screen.md`'s
  concern.
- **Reliability** — an Open Library outage degrades Discover (FR-9's
  `Unavailable` mapping) without affecting any other endpoint; this
  adapter has no shared mutable state with `backend-library-api.md`'s
  handlers beyond the rate limiter's own in-process counter, which is
  process-lifetime, not persisted (a restart resets the budget, an
  accepted tradeoff — this system is not trying to enforce a strict
  daily cap across restarts, only smooth burst behaviour within a
  session).
- **Observability** — a rate-limit rejection (FR-7, bucket or waiter
  cap exhausted) MUST log at `warn` with the endpoint and Open Library
  key/path involved, distinguishable from a genuine upstream `Unavailable`
  (FR-9) by a distinct log message, not just the shared HTTP status —
  this is the concrete mechanism for the roadmap's own requirement that
  a maintainer can tell "Open Library is down" apart from "this system
  is self-throttling." A per-field normalisation drop (FR-4) MUST log at
  `info` with the field name and the Open Library key of the record it
  was dropped from — never the field's own value, since a dropped value
  already failed validation and may itself be malformed/oversized. The
  search query string (`q`, FR-1) and any free-text field of a
  normalised result MUST NOT appear in any log line this adapter emits
  — restates constitution §8's "what someone is reading/searching for"
  concern, applied here to Discover searches specifically, not just
  library reads.

**Dependency justification (constitution §9)**: `golang.org/x/time/rate`
— what it does: a token-bucket rate limiter. Why not stdlib: Go's stdlib
has no rate limiter; hand-rolling one is a well-known place to get
burst/refill semantics subtly wrong, and `x/time` is maintained by the Go
team itself as a quasi-standard extension, not a third-party dependency
in the usual sense. What breaks if abandoned: it is small, stable,
API-frozen, and has had no breaking changes in years — vendoring the
~150 lines it would take to replace is a viable fallback if it were ever
archived, so abandonment risk is low-consequence.

## Domain model

No new domain types. `NormalisedSearchResult`/`NormalisedWork`/
`NormalisedEdition`/`NormalisedAuthor` are wire-level DTOs, not literal
`domain.Work`/`domain.Edition`/`domain.Author` instances — a Discover
result is metadata about a book that may never enter the user's library,
so this adapter never constructs a full domain aggregate (which would
require, at minimum, an internal ID `domain-bibliographic.md` FR-1
generates only "at creation" of a real domain entity). What this spec
*does* reuse from `domain-bibliographic.md` is its **field-level
validators** (FR-5/FR-6's length bounds, control-character rejection,
BCP-47 language-tag shape) — the same validation functions a real
`Work`/`Edition`/`Author` construction path calls, invoked here directly
against DTO fields, so a title or author name that would be rejected by
the domain layer is rejected identically here, without materialising a
domain object to do it. `Subject` and `Language` (FR-3) are
`domain-bibliographic.md`'s own constrained value types, used as-is
since they carry no identity concerns FR-1's DTO/domain distinction
would apply to. This spec introduces no `LibraryEntry` or `Collection`
reference — that only happens if/when phase 10's import flow attaches a
Discover result to the user's actual library, constructing a real domain
`Work` from a DTO at that point, not before. The boundary this spec
enforces (constitution §3) is: everything on the Open Library side of
`internal/metadata/openlibrary` (package name illustrative, per
`architecture-backend.md`'s layout) is untyped-by-this-system JSON;
everything past it is a validated DTO or nothing — never a raw
Open Library field, and never silently promoted to a full domain
aggregate by this adapter.

## API and contracts

Both endpoints follow `architecture-contracts.md`'s existing conventions
(base path `/api/v1`, error envelope `{ code, message, correlationId }`,
versioning).

- `GET /api/v1/discover?q=<string>&limit=<int>&offset=<int>`
  → `200 { items: NormalisedSearchResult[], total: number, limit: number, offset: number }`
  → `400` (`InvalidInput`), `503` (`Unavailable`)
- `GET /api/v1/discover/works/:openLibraryId`
  → `200 { work: NormalisedWork, editions: NormalisedEdition[] }`
  → `400` (`InvalidInput`), `503` (`Unavailable`), `500` (`Internal`, FR-5's missing-required-field case)

`NormalisedSearchResult`, `NormalisedWork`, `NormalisedEdition`,
`NormalisedAuthor` are new wire types, documented in this adapter
package and covered by the contract test (`architecture-contracts.md`
FR-3, `backend-library-api.md` FR-8's `kin-openapi` tooling reused here,
not re-decided).

## State transitions

Not applicable in the stateful-entity sense — this adapter is
stateless per request (the rate limiter's token count is the only
process-lifetime state, not a domain state machine). The meaningful
"transitions" are the failure-mode table below: a lookup moves from
`requested` to exactly one of `succeeded (full)`, `succeeded (partial,
per FR-4)`, or `failed (per FR-9/FR-5)` — never a partial commit to a
cache or anywhere else from this spec's own responsibility (caching is
`backend-metadata-caching.md`'s concern, called only after this spec's
normalisation fully succeeds for the item being cached).

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Open Library times out or is unreachable | FR-9's 5-second budget elapses / connection error | Discover shows an "Open Library is unavailable, try again" state, not a blank screen or a spinner forever | Returns `Unavailable`; logs at `warn`, not `error` (an upstream outage is not this system's defect) |
| Open Library returns a non-2xx status (e.g. `429`, `404` for a since-removed work) | HTTP status check | Same `Unavailable` degraded state for `429`; a `404` on FR-3's work lookup surfaces as `NotFound` specifically, since a missing work is a meaningful, distinct outcome from a rate limit | `429` maps to `Unavailable` (FR-9); a work-lookup `404` maps to `NotFound` |
| Response body exceeds the 5 MiB cap | Size check before full body read (constitution §4) | Same `Unavailable` state | Aborts the read, maps to `Unavailable`, logs the byte count that triggered it (not the body) |
| A field fails domain validation (oversized string, bad language tag) | `domain-bibliographic.md` construction-time validation | That one field is simply absent from the result; no error surfaced for a single dropped field | Drops the field (FR-4), logs at `info` which field/lookup, not the offending value |
| A required field (`title`) is missing from a list-item search result | Absence check during normalisation | That one item is missing from the results list; `total` still reflects Open Library's own count, so the UI may show fewer rendered items than `total` — a named, accepted discrepancy, not a bug | Omits the item (FR-5) |
| A required field is missing from FR-3's top-level work fetch | Absence check | Discover shows a generic lookup-failed state | Returns `Internal`, logs the Open Library key involved (not sensitive) |
| Rate limit bucket or waiter cap (FR-7) exhausted | Token bucket empty past the 5-second wait budget, or 50 waiters already queued | Same `Unavailable` degraded state | Returns `Unavailable` without ever sending the request upstream; logs at `warn` with a message distinct from FR-9's own upstream-outage `warn` line, so the two are distinguishable in logs |
| Malformed `openLibraryId` path parameter | Regex/shape check before any upstream call | `400` with a specific message, not a generic error | Returns `InvalidInput`, no upstream call made |
| Open Library's own response contains duplicate entries (e.g. the same work `key` twice in one `docs[]` array) | Key-collision check during normalisation | Discover shows the result once, not twice | Later duplicate entries for an already-seen key within one response are dropped, not appended — this system's own de-duplication, independent of whatever caused Open Library to return the duplicate |

## Security considerations

- **Trust boundary**: Open Library is an external, third-party service.
  Everything it returns is untrusted input, full stop — constitution §4
  applies here exactly as it applies to any LAN client request, with the
  same size/shape/timeout checklist (FR-9).
- **Injection**: normalised string fields eventually reach PostgreSQL
  (via `backend-metadata-caching.md`) through the same parameterized-query
  discipline `backend-persistence.md` FR-3 already established — no new
  risk introduced here, but named because this is the first time
  externally-sourced (as opposed to user-typed) strings reach that path.
- **SSRF**: this adapter only ever calls a fixed, hardcoded set of
  Open Library hostnames (`openlibrary.org`, `covers.openlibrary.org`) —
  it MUST NOT construct a request URL from any part of an inbound
  request beyond a validated ID/query string interpolated into a known
  path template; a caller cannot redirect this adapter to fetch an
  arbitrary URL. `q` (FR-1) and every other inbound value placed into an
  outbound Open Library URL MUST be encoded via Go's own URL-building
  API (`net/url`'s `Values.Encode()` or `url.URL.Query()`), never
  string-concatenated into the URL directly — closes query-parameter
  injection into the upstream request even though the host stays fixed.
- **Resource exhaustion**: FR-7's rate limiter and FR-9's size cap
  together bound how much this adapter can be made to do or receive by a
  high-volume caller or a misbehaving upstream; FR-9's 5-second timeout
  bounds how long a single request can hold a connection open.
- **Secret handling**: no secret crosses this boundary — Open Library's
  endpoints used here require no authentication. FR-6's `User-Agent` is
  deliberately not secret (it's meant to identify this app to Open
  Library) but is still excluded from routine request logging per
  `backend-errors-and-logging.md`'s general header-logging discipline.
- **Malformed JSON / deeply nested payloads**: the JSON decoder MUST be
  configured with a nesting-depth or token-count guard consistent with
  Go's `encoding/json` defaults plus the FR-9 size cap — a body that
  passes the size cap but is adversarially nested is still bounded by
  size, so no separate depth limit is required beyond what the size cap
  already constrains.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Normalisation of real, recorded Open Library response fixtures (captured from the actual API during spec grounding, not invented) into `NormalisedSearchResult`/`NormalisedWork`/`NormalisedEdition`/`NormalisedAuthor`; each FR-4/FR-5 dropped/omitted-field case exercised with a fixture missing that specific field, including a type-mismatched numeric field (FR-4); a fixture with more than 20 distinct authors asserting the cap (FR-3) and that one author's fetch failure omits only that author (FR-9 scoped to one author, not the whole request); a fixture with a duplicated work `key` in one `docs[]` array asserting de-duplication; rate limiter token accounting including the 50-waiter cap (FR-7); startup failing when `OPEN_LIBRARY_USER_AGENT` is unset (FR-6) |
| Integration | `/api/v1/discover*` handlers against a fake HTTP server standing in for Open Library (never the real service — deterministic, no live dependency in CI), covering the FR-9 timeout/size/status failure table |
| Contract | `architecture-contracts.md` FR-3's `kin-openapi` tool, extended to `/api/v1/discover*`'s request/response shapes |
| E2E | Search → click a result → see normalised detail, against the fake Open Library server; a second E2E run with the fake server returning `503` to exercise the degraded-state path end to end |
| Accessibility | Not applicable at this layer — covered by `frontend-discover-screen.md` |

Tests that must fail before implementation begins: a normalisation test
asserting a `docs[]` entry missing `author_name` produces zero authors,
not an error; a rate-limiter test asserting a 4th request within one
second waits rather than proceeding; a size-cap test asserting a 6 MiB
fake response is rejected before being fully read into memory.

## Acceptance criteria

- [ ] `GET /api/v1/discover` returns normalised, paginated search results
      backed by a real Open Library query
- [ ] `GET /api/v1/discover/works/:openLibraryId` returns a normalised
      work and its editions
- [ ] No test or manual inspection finds a raw Open Library field name
      (e.g. `author_name`, `cover_i`) anywhere in a response body
- [ ] A field-level normalisation failure never fails an entire
      request (FR-4)
- [ ] The rate limiter demonstrably keeps outbound request rate at or
      below 3/second under concurrent load
- [ ] `User-Agent` sent upstream is read from `OPEN_LIBRARY_USER_AGENT`,
      not hardcoded, and startup fails if it's unset
- [ ] A simulated Open Library outage produces `Unavailable`, not a
      crash or hang
- [ ] `coverUrl` in every response points at this system's own
      `/api/v1/discover/covers/:coverId` path, never a
      `covers.openlibrary.org` URL
- [ ] A work with more than 20 distinct authors returns exactly 20,
      and one author's fetch failure omits only that author
- [ ] The contract test passes against both endpoints

## Open questions

- **FR-3's 50-edition cap** — a reasoned placeholder (Open Library
  itself doesn't paginate a single work's edition list in its own UI in
  a way this project has modelled), not a load-tested limit; revisit if
  a real work with more editions surfaces the gap.
- **Author fetch fan-out (FR-3)** — capped at 20 distinct authors and
  bounded against the shared rate limiter (FR-7), closing the earlier
  unbounded-fan-out gap; whether 20 serialised author lookups behind the
  rate limiter is fast enough in practice for the rare work that hits
  the cap is still open, and is an open question for
  `backend-metadata-caching.md`'s cache-warming behaviour to help
  absorb, not resolved here.
- **Search relevance/ranking** — this spec passes `?q=` through as-is
  and trusts Open Library's own relevance ranking; no re-ranking or
  weighting is applied. Flagged in case Discover's real usage reveals
  Open Library's default ranking is a poor fit.

## References

- `domain-bibliographic.md` (phase 02) — normalisation target
- `architecture-backend.md` (phase 01) — package layout, dependency rules
- `backend-errors-and-logging.md` (phase 03) — error taxonomy, redaction
- `backend-configuration.md` (phase 03) — `User-Agent` config source;
  amended by FR-6 to add `OPEN_LIBRARY_USER_AGENT` to its key table
- `backend-service-lifecycle.md` (phase 03) — fail-to-start path FR-6
  reuses for a missing required key
- `backend-http-transport.md` (phase 03) — timeout figure reused for
  consistency, not literal precedent (FR-9)
- `backend-library-api.md` (phase 06) — 200-char search bound precedent,
  `kin-openapi` contract-test tool reused
- `backend-metadata-caching.md` (phase 07) — cache interface this
  adapter's normalised output is handed to
- Open Library API documentation: `openlibrary.org/dev/docs/api/search`,
  `openlibrary.org/dev/docs/restful_api`, `openlibrary.org/dev/docs/api/covers`,
  `openlibrary.org/developers/api`
- Constitution §3 (domain/metadata/source separation), §4 (hostile
  input), §9 (dependency justification)
