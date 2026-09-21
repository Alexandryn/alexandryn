# Spec: Backend library API

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-31, phase 06 Tier 6 / L24 — Tiers 0–5 / PR #71, audit `0006`, verified by maintainer) |
| **Phase** | `06-library` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-31 |

| **Supersedes** | — |
| **Reviewed in** | `0033` (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14. Amended post-approval, `0038` — FR-5 extended with a `formats` field for `frontend-library-screens.md`'s "Read" gate (phase 11), cross-spec-reviewed, findings fixed, re-confirmed by maintainer 2026-08-15 |

## Context

`domain-library.md` (phase 02) fixed the model: `LibraryEntry` references
one `Edition`, `Collection` contains `Work`s, "in library" is computed
not stored. `backend-persistence.md` (phase 03) fixed the repository
pattern those types are persisted through. `architecture-contracts.md`
(phase 01) fixed the wire-level shape (base path, error format,
versioning) but explicitly left pagination/filtering/sorting conventions
and the contract-test tool choice to "phase 06's first list endpoint" —
this spec is that endpoint, and where both open questions get answered
against a concrete case rather than guessed at in the abstract.

## Problem

Nothing has fixed: the actual endpoint list and their request/response
shapes, the pagination mechanism, the closed vocabulary for filter/sort
parameters, how a `Work`'s "in library" and collection-membership facts
get computed into a response without N+1 query patterns, or the
contract-test tool `architecture-contracts.md` FR-3 requires but never
named.

## Goals

- Fix every endpoint's request/response shape, added to `api/openapi.yaml`
- Fix pagination as a concrete mechanism
- Fix a closed filter/sort parameter vocabulary, not a generic query
  language
- Fix the contract-test tool, closing `architecture-contracts.md`'s own
  Open question
- Fix how `domain-library.md`'s computed facts (in-library, collection
  membership) reach a response efficiently

## Non-goals

- Any screen consuming this API — `frontend-library-screens.md`,
  `frontend-collections-screens.md`
- Metadata/source-derived fields (cover images, availability) — phase
  07/08; this phase's responses carry only locally-owned data
  (`domain-library.md`'s own model), with generated covers
  (`frontend-generated-covers.md`) as the frontend's own fallback, not
  a field this API returns
- Authentication on any endpoint — phase 12, same loopback-trust model
  every earlier phase shares
- Full-text search ranking/relevance tuning — FR-2 fixes that search
  exists and how, not how well-ranked results are; a quality concern for
  later once real usage exists to tune against

## User stories

- As **`frontend-library-screens.md`**, I want one endpoint returning a
  paginated, searchable, filterable, sortable list of works, so the
  library home screen is one API call plus query parameters, not several
  round trips stitched together client-side.
- As **`frontend-collections-screens.md`**, I want standard CRUD plus
  membership endpoints for collections, so collection management doesn't
  need a bespoke protocol per operation.
- As **a future phase 07/08**, I want work detail's response shape
  already accounting for fields that don't exist yet (cover URL,
  availability) as optional/absent rather than needing a breaking
  reshape when they arrive.

## Functional requirements

- **FR-1** `GET /api/v1/library` returns a paginated list of `Work`s —
  the *exact* set is FR-3's closed `filter` vocabulary, default `all`
  (owned ∪ wanted, defined there); this endpoint's result set is
  whatever `filter` resolves to, never independently hardcoded to
  "owned only" (an earlier draft of this FR did exactly that, directly
  contradicting FR-3's own `wanted`/`all` values one paragraph later —
  a cross-phase review caught the self-contradiction). Pagination is
  **cursor-based** (`?cursor=<opaque>&limit=<n>`, default `limit=50`,
  max `100`, both bounds enforced — see Failure modes) — not
  offset/limit, because offset pagination against a table that gains
  rows during active browsing (a real possibility once phase 10's
  import exists) produces skipped or duplicated results; a cursor
  doesn't have this problem, given a deterministic total ordering
  (FR-4). Justified as the correct default now rather than a retrofit
  once phase 10 makes the offset problem real. Response: `{ works:
  [...], nextCursor: string | null }`.
- **FR-2** Search is a `?q=<string>` query parameter, matched against
  `Work` title and author name via PostgreSQL full-text search
  (`tsvector`/`tsquery`, generated columns + a GIN index — no new
  dependency, constitution §9: PostgreSQL's own built-in capability,
  already the project's chosen engine, ADR 0004; a dedicated search
  library/service would be new infrastructure to justify for a
  single-household-scale library with no evidence yet that built-in
  full-text search is insufficient). `q`'s length is bounded at **200
  characters** — a reasoned placeholder (flagged in Open questions, not
  derived from any existing spec's own number: `backend-http-transport.md`
  fixes whole-request limits, not a per-field character bound, so this
  is this endpoint's own new number, not a citation of that spec's
  existing ones) — enforced by a handler-level length check before the
  value reaches a query as anything but a `tsquery`-safe parameterized
  argument (`backend-persistence.md` FR-3's parameterized-query
  discipline, restated for this endpoint specifically since it's the
  first one handling free-text user input).
- **FR-3** Filtering is a closed vocabulary, one query parameter,
  `?filter=owned|wanted|all` (default `all`): `owned` — works with at
  least one `LibraryEntry`; `wanted` — works in at least one
  `Collection` but with zero `LibraryEntry`s (the "want to read" case
  `domain-library.md`'s own State transitions section names explicitly);
  `all` — every work reachable via either condition (FR-1's own default
  result set). An unrecognized filter value is a `400 InvalidInput`
  (`backend-errors-and-logging.md` FR-1), never silently ignored or
  defaulted — the same fail-loud discipline `backend-configuration.md`
  already established for this project's config values, applied here to
  a request parameter. Each value is achievable in FR-9's single query
  via a conditional `EXISTS`/anti-join predicate selected by the
  `filter` value, not three structurally different queries.
- **FR-4** Sorting is a closed vocabulary, `?sort=added_at|title`
  (`added_at`: default, descending — newest first, matching what a
  "library home" screen's default view should show; `title`: ascending,
  the natural reading order for an alphabetical list), each with an
  implicit tiebreaker on `id` for stable pagination (FR-1's cursor
  depends on total ordering being deterministic; sorting on `title`
  alone, with duplicate titles possible, would otherwise make the cursor
  ambiguous). The cursor's encoded payload matches whichever sort field
  is active — `(added_at, id)` under the default sort, `(title, id)`
  under `sort=title` — never hardcoded to one field regardless of the
  active sort (an earlier draft only defined the default-sort cursor
  shape, silently breaking `sort=title`'s pagination). For a `filter=wanted`
  or `filter=all` result row with no `LibraryEntry` (no
  `LibraryEntry.added_at` exists for it), the `added_at` sort key is the
  **most recent `Collection`-membership `added_at`** across every
  collection the work belongs to (`domain-library.md` FR-9's per-
  membership timestamp) — falling back to `LibraryEntry.added_at` for
  an owned work, so every row in any `filter` value has one well-defined
  sort key regardless of which condition (owned, wanted, or both) it
  satisfies. An unrecognized sort value is `400 InvalidInput`, same
  discipline as FR-3.
- **FR-5** `GET /api/v1/works/:id` returns one `Work`'s detail: its own
  fields (`domain-bibliographic.md`), its owned `Edition`s (each with
  its `LibraryEntry`'s `added_at`, if any), and the `Collection`s it's a
  member of (id, name, per `domain-library.md` **FR-9**'s own added-at
  timestamp for that membership — not FR-4, which only fixes that
  collections contain `Work`s, not the timestamp itself; this spec's own
  FR-7 already cites FR-9 correctly for the same fact, and this
  reference is corrected to match). A `Work` ID that doesn't exist
  returns `404 NotFound`. A malformed ID (not a valid ID shape for
  whatever type `internal/domain` uses — constitution §4's shape check
  applied before the value reaches a query) is also `400 InvalidInput`,
  distinct from `404`: a lookup miss against a validly-shaped ID that
  simply doesn't exist, versus a value that was never a legal ID to
  begin with, are different failure classes and get different
  responses, not both collapsed into `404`. Fields for data this phase
  doesn't produce (cover image URL, per-source availability) are simply
  absent from the response object, not present-but-null — an additive,
  non-breaking shape for phase 07/08 to extend, per this spec's own
  Non-goals.

  **Amended for phase 11**: each owned `Edition` in this response
  additionally carries `formats: string[]` — the distinct `Format`
  values (`domain-source.md` FR-2) across every `SourceOffering`
  referencing that `Edition`, deduplicated. This is exactly the kind of
  additive, non-breaking field this FR's own text already anticipated
  ("per-source availability... for phase 07/08 to extend") — phase 11
  is the first caller that actually needs to know, without a separate
  lookup, whether a given owned `Edition` is available as an EPUB
  (`frontend-library-screens.md` FR-5's amended "Read" action gate).
  An `Edition` with zero `SourceOffering`s (a legal state, though an
  unusual one for something with a `LibraryEntry`) returns `formats:
  []`, never omits the field — unlike the cover-URL/availability
  fields this FR's original text describes, `formats` is now a field
  this phase *does* produce, so it's always present on every owned
  `Edition`, empty array or not.
- **FR-6** Collections: `POST /api/v1/collections` (body: `{ name:
  string }`, `201` with the created collection); `GET
  /api/v1/collections` (list, no pagination — a household's collection
  count is expected to stay small enough that this is safe to leave
  unpaginated now, flagged in Open questions as revisit-if-wrong);
  `GET /api/v1/collections/:id` (detail, including member `Work`
  summaries, each carrying its own FR-9-per-membership `added_at` —
  mirroring FR-5's own per-membership timestamp in the other direction,
  so both "which collections is this work in, and when" and "which
  works are in this collection, and when" are answerable from their
  respective endpoints without a second round trip);
  `PATCH /api/v1/collections/:id` (body: `{ name: string }`,
  rename only — membership changes go through FR-7, never bundled into
  a general-purpose PATCH that could silently also move works if a
  client sent extra fields); `DELETE /api/v1/collections/:id`
  (`domain-library.md` FR-10's non-cascading delete). Collection `name`
  validation matches `domain-bibliographic.md` FR-5/FR-6's existing
  hostile-input bar (length bound, control-character rejection) — this
  spec's own enforcement point for `domain-library.md`'s Security
  considerations, which named the requirement but not the wire-level
  check.
- **FR-7** Collection membership: `POST
  /api/v1/collections/:id/works` (body: `{ workId: string }`, adds a
  `Work` to a `Collection`, `domain-library.md` FR-9's own added-at
  timestamp recorded); `DELETE
  /api/v1/collections/:id/works/:workId` (removes membership — a hard
  delete of the specific `(collectionId, workId)` row; `domain-library.md`'s
  closest analog is FR-10's non-cascading `Collection` deletion, one
  level up — no FR of that spec addresses single-membership removal
  directly, so this behavior, restated here, is this spec's own to fix:
  the `Work`, `Edition`, and every *other* `Collection`'s membership are
  untouched). Adding a `Work` already in the collection is implemented
  as `INSERT ... ON CONFLICT (collection_id, work_id) DO NOTHING` — a
  no-op success (`200`, not `409 Conflict`, since a client retrying a
  timed-out request shouldn't get a spurious conflict for an operation
  that already succeeded) that **preserves the original `added_at`**,
  never resetting it to the retry's timestamp — this is what "idempotent"
  actually means for a fact FR-9 defines as "when did you want this":
  a retry must not silently rewrite that answer. A **remove, then a
  later re-add**, is a genuinely new membership, not a retry of the
  original — the prior `DELETE` already removed the row entirely (this
  FR's own hard-delete semantics above), so the subsequent `POST`
  legitimately gets a fresh `added_at`; this is expected and tested as
  its own case, not conflated with the same-request-retried idempotency
  case above.
- **FR-8** The contract test tool is **`kin-openapi`**
  (`github.com/getkin/kin-openapi`) — validates real HTTP responses
  against `api/openapi.yaml` at integration-test time, closing
  `architecture-contracts.md` FR-3's own Open question (left to "phase
  03's to pick," never actually picked before this spec, the first one
  that needed it to test something real). Justified under constitution
  §9: small, focused (schema validation only, no code generation to
  maintain), the de facto standard Go OpenAPI 3 validator. If abandoned,
  exit cost is low — it's a test-time-only dependency validating
  against a spec this project already hand-authors and owns (FR-1 of
  `architecture-contracts.md`); a replacement validator reads the same
  `api/openapi.yaml` file, no test logic beyond the validation call
  itself would need rewriting. The test asserts against every endpoint
  this spec adds, including deliberately malformed requests (FR-3/FR-4's
  invalid-filter/sort cases), closing the "does the contract test catch
  *missing* endpoints, or only *mismatched* ones" gap
  `architecture-contracts.md`'s own Failure modes table flagged as
  undesigned — this spec's own contract-test suite is written to fail if
  an endpoint exists in the Go router without a corresponding
  `api/openapi.yaml` entry, via a route-enumeration cross-check against
  the spec file's own path list, not just response-shape validation for
  paths the spec happens to already document.
- **FR-9** Computing `domain-library.md`'s derived facts (in-library,
  collection membership) for FR-1's list endpoint uses one query with
  joins/subqueries, never N+1 per-work lookups — `backend-persistence.md`
  FR-2's one-repository-type-per-aggregate pattern still holds (a
  `WorkRepository` method, not a bespoke query scattered in the HTTP
  handler), but that method's own SQL is written to compute "has a
  `LibraryEntry`" and "collection memberships" as part of the same
  round trip that fetches the page of works, not as a separate call per
  row. **Measurement mechanism**: a `pgx.QueryTracer`
  (`pgx`'s own native hook, ADR 0012 — no new dependency) registered
  only in the integration test's pool construction, counting
  `TraceQueryStart` invocations for the duration of one `GET
  /api/v1/library` call; the test asserts that count is exactly one
  regardless of how many works are returned or how many collections
  they belong to — a concrete, CI-runnable mechanism, not an asserted
  claim with no way to check it.

## Non-functional requirements

- **Performance** — FR-9's single-query requirement is this spec's own
  concrete performance property; no specific latency budget is set,
  since no real hardware/data-scale measurement exists yet (same
  placeholder-pattern status this project's numeric budgets have carried
  since phase 01).
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; this is an API, not UI.
- **Reliability** — FR-6's idempotent add-to-collection (FR-7) is this
  spec's own reliability property: a client retry after a network
  timeout never produces a spurious error for an operation that already
  succeeded.
- **Observability** — every endpoint inherits `backend-errors-and-
  logging.md`'s per-request correlation ID and completion logging
  automatically (`backend-http-transport.md` FR-4); no additional
  logging designed here beyond what every endpoint already gets.

## Domain model

Not applicable directly — this spec implements `domain-library.md`'s
already-fixed model as HTTP endpoints; it introduces no new domain
concept, only wire-level shapes for existing ones.

## API and contracts

- **`GET /api/v1/library`**: query params `cursor`, `limit`, `q`,
  `filter`, `sort` (FR-1–FR-4); response `{ works: WorkSummary[],
  nextCursor: string | null }`.
- **`GET /api/v1/works/:id`**: response is a `WorkDetail` object (FR-5).
- **`POST/GET/PATCH/DELETE /api/v1/collections[/:id]`**: FR-6.
- **`POST/DELETE /api/v1/collections/:id/works[/:workId]`**: FR-7.
- Every response uses `architecture-contracts.md` FR-5's shared success/
  error shape; every error uses `backend-errors-and-logging.md` FR-1's
  six categories, no new category invented for this endpoint set.
- `api/openapi.yaml` gains a `Work`, `WorkSummary`, `WorkDetail`,
  `Collection` schema set alongside these paths — the actual YAML
  content is this spec's implementation output, not enumerated field-
  by-field in this document beyond what FR-1/FR-5/FR-6 already fix.

## State transitions

Not applicable at the API layer — `domain-library.md` already owns the
state transitions these endpoints trigger (`LibraryEntry`/`Collection`
creation, removal); this spec only exposes them over HTTP.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `filter`/`sort` value outside the closed vocabulary (FR-3/FR-4) | Handler-level enum validation | `400 InvalidInput`, naming the invalid parameter and its value | Rejected before any repository call |
| `q` exceeds 200 characters (FR-2) | Handler-level length check | `400 InvalidInput` | Rejected before any repository call |
| `limit` is zero, negative, non-numeric, or exceeds 100 (FR-1) | Handler-level range check | `400 InvalidInput`, naming the invalid value — never silently clamped, same fail-loud discipline as `filter`/`sort`/`q` | Rejected before any repository call |
| `:id`/`:workId` references a nonexistent `Work`/`Collection` | Repository lookup miss (value was validly shaped) | `404 NotFound` | No partial response; the whole request fails clearly |
| `:id`/`:workId` is not a validly-shaped ID at all (FR-5) | Handler-level shape check, before any repository call | `400 InvalidInput`, distinct from `404` | Rejected before construction of a repository call, never reaching a driver-level parse error |
| A `POST /collections` body with an invalid `name` (`domain-bibliographic.md` FR-5/FR-6's bar) | Handler-level validation, same pattern as existing bibliographic fields | `400 InvalidInput`, naming what's wrong (too long, control character) | Rejected before construction |
| Adding an already-member `Work` to a `Collection` (FR-7) | `ON CONFLICT DO NOTHING`'s own no-op | `200`, no error — idempotent success | No duplicate row created; original `added_at` preserved, never reset |
| Removing then re-adding the same `Work`/`Collection` pair (FR-7) | Two separate requests, no special-cased detection | `200` on the re-add, a fresh `added_at` | Legitimate new membership — the prior removal was a hard delete, not a soft/tombstoned one |
| `cursor` value is malformed/tampered (FR-1) | Base64/timestamp decode failure | `400 InvalidInput`, generic "invalid cursor" — never echoing the malformed value back, which could reflect attacker-supplied content into a response | Rejected before any repository call |

## Security considerations

- **Every query parameter validated before reaching a query
  (FR-2/FR-3/FR-4)** — constitution §4's hostile-input discipline,
  applied to the first real user-facing endpoints in this project;
  `q`'s eventual use in a `tsquery` is parameterized, never string-built
  (`backend-persistence.md` FR-3).
- **Collection name validation (FR-6) closes the wire-level gap
  `domain-library.md`'s own Security considerations named but didn't
  enforce** — that spec flagged collection names as hostile input; this
  spec is where an HTTP request actually carrying one gets checked.
- **Cursor opacity (FR-1)** — the cursor is treated as an opaque token
  by clients, decoded and validated server-side, never echoed back
  unvalidated on a decode failure (Failure modes) — a minor but real
  instance of "never reflect attacker input back into an error"
  (`backend-http-transport.md`'s own error-message discipline).
- **No new trust boundary** — these endpoints inherit
  `backend-http-transport.md`'s existing limits/timeouts/panic-recovery
  middleware chain unchanged; nothing here needs its own auth or rate
  limiting ahead of phase 12/13.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Query-parameter validation (FR-2/FR-3/FR-4) table-driven, cursor encode/decode round-trip (FR-1) |
| Integration | Every endpoint against a real PostgreSQL instance (`backend-test-harness.md`), including FR-9's single-query claim verified by the `pgx.QueryTracer` mechanism named in FR-9 itself, not just correctness of the result |
| Contract | `kin-openapi` (FR-8) validating every endpoint's real response against `api/openapi.yaml`, including the route-enumeration completeness check |
| Cursor stability | Two concrete scenarios, not a vague "concurrently-modified dataset" claim: **(a)** under the default sort (`added_at` descending), fetch page 1, insert a new work (which sorts *above* the cursor position by construction — proving the common case doesn't naturally break, a sanity check rather than the real adversarial case), then fetch page 2 and assert no duplicate/missing rows relative to a snapshot taken before the insert; **(b)** the actual adversarial case: fetch page 1, **delete** a row that hasn't been fetched yet (sorts below the cursor), fetch page 2, and assert the result is exactly `expectedPage2 \ {deletedRow}` with no duplicate and no error — this is the scenario that can actually fail under a naive implementation, unlike (a); **(c)** the same two scenarios repeated under `sort=title`, which — unlike `added_at` descending — has no natural "new rows sort above the cursor" property, making it the sort mode most likely to expose a cursor-shape bug (FR-4's per-sort cursor encoding) |
| Adversarial | FR-2's search string with SQL-comment-shaped and `tsquery`-operator-shaped content (`&`, `|`, `!`, `:`), proving it's treated as literal search text, never as a `tsquery` operator injection; a 500-item collection to confirm FR-6's unpaginated list doesn't fail outright even though it's not optimized for that scale (Open questions covers whether it needs to be); FR-7's remove-then-re-add cycle, proving the second `added_at` is fresh, not the original |

## Acceptance criteria

- [ ] `GET /api/v1/library` returns paginated, cursor-based results
      correctly across all three `filter` values (owned, wanted, all —
      FR-3), including the "wanted" case actually appearing in the
      default (`all`) result set
- [ ] Cursor stability proven per the Test strategy's three concrete
      scenarios, including under `sort=title`, not just the default sort
- [ ] `q`, `filter`, `sort`, `limit` each proven against valid and
      invalid values, per FR-2/FR-3/FR-4/FR-1
- [ ] `GET /api/v1/works/:id` returns owned editions and collection
      memberships in one response, proven with a work in multiple
      collections and with multiple owned editions
- [ ] Full collection CRUD plus membership add/remove proven end to end,
      including the idempotent-add case (original `added_at` preserved)
      and the remove-then-re-add case (fresh `added_at`) — FR-7
- [ ] `kin-openapi` contract test fails on a deliberately malformed
      handler response and on a deliberately undocumented route, proving
      both directions (FR-8)
- [ ] FR-9's single-query claim proven via the `pgx.QueryTracer`
      assertion, not just result correctness
- [ ] Every FR maps to an exit criterion in phase 06's own document

## Open questions

- **`GET /api/v1/collections`'s lack of pagination** — assumed safe at
  household scale; revisit if a real usage pattern proves otherwise
  (Risks table, phase 06's own roadmap document).
- **Full-text search relevance/ranking** — FR-2 fixes that search
  exists and is safe; ranking quality is explicitly out of scope
  (Non-goals) pending real usage to tune against.
- **FR-2's 200-character search bound** — a reasoned placeholder, not
  derived from any existing spec's own number (`backend-http-transport.md`
  fixes whole-request limits, not a per-field bound); confirm or replace
  once real search usage exists to observe.
- **Cover image / availability fields' eventual shape** — FR-5
  deliberately leaves these absent rather than guessing their future
  shape; phase 07/08's own specs fix them when those phases arrive.

## References

- `domain-library.md` — the model this spec exposes over HTTP
- `domain-bibliographic.md` FR-5/FR-6 — the validation bar FR-6 reuses
  for collection names
- `architecture-contracts.md` FR-3–FR-5 — contract-test requirement
  (FR-8 closes its Open question), versioning, error shape
- `backend-persistence.md` FR-2, FR-3 — repository pattern, parameterized
  queries FR-9's single-query design and FR-2's `tsquery` safety depend on
- `backend-errors-and-logging.md` FR-1 — the six categories every error
  here maps to
- `backend-http-transport.md` — the middleware chain these endpoints
  inherit unchanged
- ADR 0004 — PostgreSQL as the engine, the reason FR-2 uses its built-in
  full-text search rather than a new dependency
- Constitution §4 (hostile input), §9 (dependencies — `kin-openapi`
  justification)
