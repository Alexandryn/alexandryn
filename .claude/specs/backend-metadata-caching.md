# Spec: Backend metadata caching

| | |
|---|---|
| **Status** | `REVIEWED` |
| **Phase** | `07-metadata` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0034`](../reviews/0034-phase07-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (1 Blocking, confirmed independently by both; 8 Major, 2 confirmed independently by both), all findings fixed; awaiting maintainer approval |

## Context

`backend-metadata-adapter.md` produces normalised `NormalisedWork`/
`NormalisedEdition`/`NormalisedAuthor`/`NormalisedSearchResult` shapes
from Open Library and, per its own Non-goals, does not decide where
those results are stored between requests or how long they live. Open
Library's usage policy explicitly asks integrators to cache rather than
re-request, and `07-metadata/README.md`'s own risk table names unbounded
cover-image disk growth as a real risk this spec has to bound.
`backend-persistence.md` (phase 03) already fixed the PostgreSQL
connection/migration/repository pattern this spec reuses rather than
inventing a second storage mechanism.

## Problem

Nothing fixes where a normalised Open Library lookup is stored, how a
repeat lookup is served from that store instead of re-hitting Open
Library, how a cached row goes stale, or where cover image bytes live
and how large that footprint is allowed to grow.

## Goals

- A cache-first lookup path: `backend-metadata-adapter.md`'s handlers
  check this cache before calling Open Library, and write through to it
  after a successful normalisation
- A defined key: Open Library's own key (`OL...W`, `OL...M`, `OL...A`),
  distinct from any internal domain identifier, per
  `domain-bibliographic.md` FR-1/FR-2's identity separation
- A defined staleness policy: a cached row has an age past which it's
  treated as a miss and re-fetched, not served forever
- Cover image storage with a bounded footprint
- No new storage engine — reuse `backend-persistence.md`'s PostgreSQL
  connection and migration mechanism

## Non-goals

- Any decision about *what* gets normalised or *when* a fetch happens —
  `backend-metadata-adapter.md` owns that; this spec only owns storage,
  lookup, invalidation, and eviction of what the adapter hands it
- Background/scheduled refresh of cached entries independent of a user
  action — this cache is populated and refreshed lazily, on lookup, not
  by a cron-like job; a scheduled-refresh feature is future work, not
  named in any current phase
- Caching Discover *search result lists* (FR-1's paginated response) —
  only individual `NormalisedWork`/`NormalisedEdition`/`NormalisedAuthor`
  records and cover image bytes are cached; a search result list is a
  composition of already-cacheable pieces plus Open Library's own
  ranking, which itself isn't meaningfully cacheable per distinct query
  string without unbounded key-space growth

## User stories

- As **someone re-opening a Discover result they looked at minutes
  ago**, I want it to load instantly from the local cache, so I'm not
  waiting on a network round trip to Open Library again.
- As **the maintainer**, I want cover image storage to have a real,
  enforced ceiling, so a heavily-used Discover screen doesn't quietly
  fill the disk over months of use.
- As **the system**, I want a cached record older than its staleness
  window to be treated as absent, so a book whose Open Library record
  changed (a corrected title, a newly added cover) eventually reflects
  that without a manual cache-clear.

## Functional requirements

- **FR-1** A new PostgreSQL schema, migrated via `backend-persistence.md`'s
  existing `goose` mechanism (ADR 0013), MUST add tables for cached
  metadata: `metadata_works`, `metadata_editions`, `metadata_authors`,
  each keyed by its Open Library key (text primary key, e.g. `OL...W`)
  plus a `fetched_at timestamptz` column and the normalised fields
  themselves (mirroring `backend-metadata-adapter.md`'s
  `NormalisedWork`/`NormalisedEdition`/`NormalisedAuthor` shapes). This
  is new storage, not a repurposing of any `domain-bibliographic.md`-backed
  table — a cached metadata row is not a `Work`/`Edition`/`Author` domain
  row and MUST NOT be queried as one; only phase 10's future import flow
  is where a cache row and a domain row are ever connected, and that
  connection is out of this spec's scope.

  The relational shape (`architecture decisions expected` in
  `07-metadata/README.md` explicitly assigns this to this spec): `metadata_editions`
  has a `work_key` foreign key column referencing `metadata_works` (an
  edition always belongs to exactly one cached work, matching
  `domain-bibliographic.md` FR-2's own one-Work-per-Edition rule, mirrored
  here as a storage-layer fact about the cache, not a domain constraint).
  Author membership is many-to-many on both sides (a work can list
  several authors; an author can appear on several cached works) and is
  modelled as a join table, `metadata_work_authors(work_key, author_key)`,
  rather than an array column, so a single cached `metadata_authors` row
  is shared across every work that credits that author instead of being
  duplicated per work.
- **FR-2** A lookup (`backend-metadata-adapter.md`'s FR-3 work-detail
  fetch) MUST check this cache by Open Library key before calling Open
  Library. A row present and with `fetched_at` within the staleness
  window (FR-3) is a cache hit — served without any upstream call. A row
  absent, or present but stale, is a cache miss — the adapter proceeds
  to Open Library, and on success this spec's write-through (FR-4)
  stores the fresh result. This check applies **independently to each of
  the three cached entity types** — the work row, the editions-list rows,
  and each author row — not only to the top-level work fetch: a
  work-detail request MAY be a cache hit for the work itself while
  simultaneously a cache miss for one newly-added author not yet cached
  (fetched live for that one author, written through per FR-4, while the
  rest of the response is served from cache). This is what makes the
  cache actually useful at the granularity FR-4's per-entity write-through
  already assumes, rather than only ever caching or missing a whole
  work-detail response as one unit.
- **FR-3** The staleness window is **30 days** for `metadata_works` and
  `metadata_editions`/`metadata_authors` rows alike — a single, uniform
  value, not a per-field policy, chosen because Open Library's own
  records (published books, historical bibliographic data) change
  rarely enough that a month-old cache entry being briefly wrong is a
  low-cost, low-likelihood event, and a uniform window is simpler to
  reason about and test than a per-type one. Flagged in Open questions
  as a placeholder subject to revision once real usage data exists.
- **FR-4** After `backend-metadata-adapter.md` successfully normalises a
  work, its editions, and their authors (a cache miss path), this spec's
  write-through MUST upsert all three into their respective tables,
  plus FR-1's `metadata_work_authors` join rows linking the work to each
  of its authors, in one transaction (`backend-persistence.md`'s
  existing transaction pattern), keyed by Open Library key, with
  `fetched_at` set to the current time. A partial normalisation failure
  (per `backend-metadata-adapter.md` FR-4's per-field drop) still writes
  through — a row with some fields null because Open Library omitted
  them is a valid, cacheable state, not a reason to skip caching
  entirely.
- **FR-5** Cover image bytes are cached to local disk under
  `backend-configuration.md` FR-5's existing app-data directory family
  (the same directory family other non-PostgreSQL local files already
  live under — not `architecture-persistence.md`'s Postgres-specific data
  directory, a different thing), in a `covers/` subdirectory, filename
  derived from the Open Library cover ID (e.g. `covers/<cover_i>-M.jpg`)
  — never from a user- or Open-Library-supplied string used as a path
  component without this fixed, numeric-ID-only naming scheme, closing
  the path-traversal risk named in Security considerations. A cover is
  fetched and cached lazily, on first render request (`GET
  /api/v1/discover/covers/:coverId`, FR-6), not proactively when a
  work/edition is normalised — avoids paying the image-fetch cost for
  results a user never actually views.
- **FR-6** `GET /api/v1/discover/covers/:coverId` MUST validate
  `coverId` is a positive integer, return `InvalidInput` otherwise, serve
  the cached file if present (`Content-Type` per the cached extension,
  a long `Cache-Control` max-age since a given `coverId`'s image is
  effectively immutable), and on a cache miss fetch it from Open
  Library's Covers API (`backend-metadata-adapter.md`'s existing rate
  limiter budget applies to this fetch too — one shared budget across
  every Open Library call this system makes), cache it, then serve it.
  A `404` from Open Library's Covers API (no cover exists) MUST cache
  that *absence* too (a small sentinel row or file, so a known-missing
  cover doesn't get re-requested from Open Library on every render) and
  return `NotFound`.

  The fetch itself is bounded by the same 10 MiB response-size cap and
  5-second timeout `backend-metadata-adapter.md` FR-9 applies to its own
  Open Library calls (a larger cap than that spec's 5 MiB, since a cover
  image is expected to be meaningfully larger than a JSON metadata
  response — this spec's own reasoned number, not a citation of FR-9's
  literal figure). The fetched bytes MUST also be validated as a
  well-formed image (a magic-byte/format check, e.g. Go's
  `image.DecodeConfig` succeeding against a known format) before being
  written to disk or served — constitution §4's shape check applied to
  this boundary specifically, since a size cap alone doesn't guarantee
  the bytes are actually an image. A file that fails either check is
  treated as a fetch failure (`Unavailable`), not cached.

  The write to disk MUST be atomic: fetched bytes are written to a
  temporary file in the same directory and renamed into place
  (`os.Rename`, atomic on the same filesystem) only after the full write
  succeeds and the format check passes — a reader MUST NOT ever be able
  to open a partially-written file. Two concurrent requests for the same
  uncached `coverId` MAY both reach Open Library and both fetch
  independently (an accepted, narrow redundant-fetch race, the same
  honest tradeoff FR-7's eviction race makes below, rather than adding
  per-key fetch locking for a low-likelihood, low-cost duplication); the
  atomic rename means whichever write finishes last simply becomes the
  cached file, with no torn-file risk either way.
- **FR-7** Total cover-image disk usage MUST be bounded to **500 MiB**,
  enforced by a least-recently-used eviction check running after each
  new cover is written: if total size exceeds the bound, the
  oldest-by-last-served-time files are deleted until back under it.
  "Last served" (not "last fetched") is tracked via an `accessed_at`
  column in a `metadata_covers` table (Open Library cover ID, file path,
  byte size, `fetched_at`, `accessed_at`) alongside the cached file, so
  a frequently-viewed cover survives eviction even if it was originally
  fetched long ago.
- **FR-8** A cache row (metadata or cover) MUST be readable and usable
  even when Open Library is entirely unreachable — this is the whole
  point of caching being separate from the adapter's live-fetch path.
  This spec's read path (FR-2, FR-6's cache-hit branch) has no
  dependency on network reachability at all.

## Non-functional requirements

- **Performance** — a cache hit (FR-2) MUST resolve in one indexed
  primary-key lookup, no join beyond what FR-1's schema itself requires;
  a cache-hit work-detail response should be materially faster than a
  live Open Library round trip, though no specific millisecond budget is
  fixed here (no load data yet to justify one — flagged in Open
  questions).
- **Security** — see dedicated section below.
- **Accessibility** — not applicable; this is a storage-layer spec.
- **Reliability** — FR-8's offline-readable guarantee is the core
  reliability property this spec adds; a corrupt or missing cover file
  on disk (deleted out-of-band, filesystem issue) MUST be treated as a
  cache miss (re-fetch), not a crash — the `metadata_covers` row and the
  actual file are allowed to independently disagree, and the read path
  MUST check the file exists before trusting the row.
- **Observability** — a cover-write failure (disk full, DB error mid-
  transaction) MUST log at `warn`, including the Open Library key or
  cover ID involved, never the cached bytes or field values themselves.
  An eviction pass (FR-7) MUST log at `info` how many files were removed
  and the resulting total size, so the bound is independently observable
  without inspecting the filesystem directly. Cache hit/miss is not
  separately logged per request (that granularity belongs in this
  spec's own `X-Metadata-Cache` response header, API and contracts
  below, not in the log stream) — restated here only to be explicit
  that this spec adds no per-request log line beyond
  `backend-http-transport.md`'s existing one.

No new dependency — this spec reuses `backend-persistence.md`'s existing
`pgx`/`pgxpool`/`goose` stack and Go's stdlib `os`/`io` for file storage;
constitution §9 accordingly has nothing new to justify here.

## Domain model

No `domain-bibliographic.md` types are introduced or modified. This
spec's `metadata_works`/`metadata_editions`/`metadata_authors`/
`metadata_covers` tables are a new, separate schema area — explicitly
*not* domain tables, per constitution §3's metadata/domain separation:
a cache row describes what Open Library asserts about a book, not
anything about this user's library. `backend-persistence.md`'s existing
repository pattern is reused for the Go-level access to these tables,
but the repository interfaces themselves are new (`MetadataCacheRepository`
or equivalent), not extensions of any `domain-library.md`-facing
repository.

## API and contracts

- `GET /api/v1/discover/covers/:coverId` → binary image body,
  `Content-Type` set from the cached file, `Cache-Control: max-age=<large>`
  → `400` (`InvalidInput`), `404` (`NotFound`), `503` (`Unavailable`,
  an Open Library Covers API failure on a genuine miss)

No other new HTTP surface — FR-2's cache-first lookup is internal to
`backend-metadata-adapter.md`'s existing `/api/v1/discover*` handlers,
transparent to the caller; a cache hit and a cache miss produce
identically-shaped `200` responses, differing only in latency and in a
`X-Metadata-Cache: hit|miss` response header for observability (not a
contract guarantee the frontend should branch on).

## State transitions

A cache row moves `absent → fresh (fetched_at within window) → stale
(past window) → refreshed (back to fresh, on next successful lookup) |
absent (never explicitly deleted by staleness alone — a stale row is
just treated as a miss and overwritten on next successful fetch, per
FR-2; it is not proactively purged)`. A cover file moves `absent →
cached → evicted (FR-7, LRU) → absent` or `absent → known-missing
(FR-6's 404 sentinel) → absent` (re-attempted only if the sentinel
itself is evicted or explicitly cleared — no case in this spec clears
it early).

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Cache hit but underlying cover file deleted out-of-band | File-exists check before serving (FR-8, Reliability) | Cover re-fetches transparently; a brief extra latency, not an error | Treats as a cache miss, re-fetches from Open Library, re-caches |
| Cache write (FR-4) fails mid-transaction (e.g. disk full, DB error) | Transaction error | The already-successful normalisation is still returned to the caller — a cache-write failure MUST NOT fail the request that triggered it | Logs the write failure at `warn`; the lookup that triggered it succeeds anyway (cache is an optimisation, not a correctness dependency) |
| Cover eviction (FR-7) races with a concurrent read of the same file | File-not-found on read after eviction deleted it mid-request | Treated as a cache miss for that one request; re-fetched | Re-fetches from Open Library rather than erroring; a narrow, accepted race, not designed away with locking given its low likelihood and low cost |
| `metadata_covers` row exists but Open Library's Covers API is unreachable on a genuine re-fetch attempt | `backend-metadata-adapter.md` FR-9's `Unavailable` mapping | Cover fails to load; UI shows its placeholder-cover fallback (`frontend-generated-covers.md`'s existing degradation ladder, reused, not reinvented) | Returns `Unavailable`, does not cache anything for this attempt |
| Disk write for a cover exceeds available space | Write error | Same placeholder-cover fallback | Logs at `error`, does not crash the request, cover simply isn't cached this time |

## Security considerations

- **Path traversal**: FR-5's filename scheme is a fixed template using
  only a validated positive integer (`coverId`) — never a string
  Open Library or a caller supplies verbatim — closing the traversal
  risk a naive `covers/<upstream-string>.jpg` scheme would open.
- **Disk exhaustion**: FR-7's 500 MiB bound with LRU eviction is the
  concrete mitigation for the unbounded-growth risk `07-metadata/README.md`'s
  risk table names; enforced after every write, not on a delayed sweep.
- **Cached data is not more trusted than its source**: a row cached from
  Open Library carries the exact same trust level as a live Open Library
  response — `backend-metadata-adapter.md`'s FR-4 field-level validation
  happens before FR-4 (this spec) ever writes a row, so nothing
  unvalidated is cached; this spec does not re-validate on read, relying
  on write-time validation having already run.
- **No new trust boundary**: this cache is read/written only by this
  system's own backend process; no LAN client or IPC surface reaches it
  directly, consistent with every other phase's loopback-trust model.
- **No secrets stored**: cached rows are public bibliographic metadata
  and cover images — nothing here falls under constitution §8's "never
  log" list, and nothing here requires the redaction discipline
  `backend-errors-and-logging.md` applies to credentials/paths/reading
  history.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Staleness-window calculation (FR-3), LRU eviction selection logic (FR-7), cover filename derivation (FR-5) rejecting any non-numeric input |
| Integration | Cache-hit and cache-miss paths against a real PostgreSQL instance (`backend-test-harness.md`'s harness) and a real temp-directory filesystem; a write-through (FR-4) followed by a read confirming the exact normalised shape round-trips, including `metadata_work_authors` join rows (FR-1); an eviction test seeding >500 MiB of fake cover files and asserting the oldest-accessed are removed first; a work-detail lookup with one already-cached author and one not-yet-cached author asserting only the uncached one triggers a fetch (FR-2); a non-image byte stream rejected by the format check (FR-6) and never written to disk; a concurrent-request test asserting no reader ever observes a partially-written cover file |
| Contract | `GET /api/v1/discover/covers/:coverId`'s shape covered by the same `kin-openapi` contract test `backend-metadata-adapter.md` extends `/api/v1/discover*` with |
| E2E | Look up a work, confirm the second identical lookup is served from cache (via the `X-Metadata-Cache` header or a controlled-clock/mock-adapter assertion that Open Library was not called a second time) |
| Accessibility | Not applicable — storage layer |

Tests that must fail before implementation begins: a test asserting a
second identical work lookup makes zero upstream HTTP calls; a test
asserting a `coverId` containing `../` or non-numeric characters is
rejected before any filesystem operation; a test asserting eviction
brings total cover storage back under 500 MiB after a seeded overage.

## Acceptance criteria

- [ ] A repeat lookup of the same Open Library work is served without
      an upstream call, within the 30-day staleness window
- [ ] A stale row is transparently refreshed on next lookup, not served
      indefinitely
- [ ] Cover images are cached to disk and served from cache on repeat
      requests
- [ ] Total cover storage stays at or under 500 MiB under sustained use,
      verified by the eviction test
- [ ] A missing/deleted cache file degrades to a re-fetch, not a crash
- [ ] No cache-write failure ever fails the triggering request
- [ ] `coverId` path handling rejects any non-integer input before
      touching the filesystem

## Open questions

- **30-day staleness window (FR-3)** — a reasoned placeholder with no
  usage data behind it yet; revisit once real Discover usage shows
  whether staler or fresher data actually matters to users.
- **500 MiB cover bound (FR-7)** — sized as "generous for a personal
  self-hosted library, not unlimited," not derived from a specific disk
  budget the maintainer has stated; open to revision.
- **No cache-hit latency budget** — Non-functional requirements
  deliberately doesn't fix a millisecond number; revisit once phase 07
  is implemented and real numbers exist.

## References

- `backend-metadata-adapter.md` (phase 07) — produces what this spec
  caches, owns the live-fetch path this spec sits in front of
- `backend-persistence.md` (phase 03) — PostgreSQL connection, `goose`
  migrations, transaction pattern, repository pattern, all reused
- `backend-configuration.md` (phase 03) FR-5 — app-data directory family
  this spec's `covers/` directory lives under
- `domain-bibliographic.md` (phase 02) — the identity-separation
  reasoning (FR-1/FR-2) this spec's key design follows
- `frontend-generated-covers.md` (phase 04) — placeholder-cover
  degradation ladder reused on a cover-fetch failure
- Constitution §3 (metadata/domain separation), §4 (hostile input,
  applied to the size/path checks here), §8 (what must never be logged),
  §9 (no new dependency, so nothing to justify)
