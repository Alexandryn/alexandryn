# Spec: Backend import pipeline

| | |
|---|---|
| **Status** | `IMPLEMENTED` (audit `0010`) |
| **Phase** | `10-import` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0037`](../reviews/0037-phase10-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (7 Blocking across the batch, each found by one pass; 11 Major, 6 Minor, 2 Nit), all findings fixed; approved by maintainer 2026-08-15 |

## Context

This spec is where several earlier phases' deliberately deferred
design decisions converge. `domain-source.md` FR-2's `SourceOffering`
("this `Source` claims to offer this `Edition`") has never been
constructed by any prior phase — `backend-source-adapter.md`'s own
Domain model section named exactly this deferral, describing a browsed
item as "a candidate that phase 10's import flow may, or may not, ever
turn into a real `domain-source.md` `SourceOffering`." `domain-bibliographic.md`
FR-4 deferred "the matching decision" to "phase 07/10" — this is where
it's closed. `domain-library.md`
FR-1's `LibraryEntry` gets its first real, non-fixture-seeded creation
path here. `backend-job-queue.md` (phase 09) provides the job
mechanism this spec's own handler registers against, including its
`job.Permanent(err)` sentinel and `ReportProgress` callback.
`backend-file-extractors.md` (this phase, sibling spec) is what turns
a resolved file's bytes into `ExtractedMetadata` — this spec picks up
from there: what to do with that metadata.

## Problem

Nothing exists yet to enumerate a source's contents for import, decide
whether an extracted file matches something already in the library or
on Open Library, apply a safe default for the unambiguous case, or let
a user resolve every other case — and nothing persists the result as
real domain data.

## Goals

- Discovery: list a source's contents via `backend-source-adapter.md`'s
  provider abstraction, skip anything already represented by an
  existing `SourceOffering`, enqueue one import job per remaining item
- An `import` job kind (`backend-job-queue.md` FR-2) that resolves,
  extracts (`backend-file-extractors.md`), and matches a single file
- Matching: candidates from the existing library (exact-identifier
  lookup) and from Open Library (`backend-metadata-adapter.md`), each
  with a confidence score
- A narrow, closed auto-accept rule — only an exact match to an
  `Edition` already owned in this library — everything else becomes a
  pending candidate for user review
- Confirm/reject endpoints that, on confirmation, construct real
  `Work`/`Edition`/`SourceOffering`/`LibraryEntry` data in one
  transaction

## Non-goals

- Extraction itself — `backend-file-extractors.md`'s job; this spec
  only calls it and acts on its output
- Reading the imported file — phase 11
- Any UI — `frontend-import-confirmation.md`'s job; this spec is the
  API surface that UI consumes
- Recurring/scheduled discovery — `backend-job-queue.md`'s own
  Non-goals already deferred scheduling generally; this phase's
  discovery is on-demand, user-triggered
- Merging two already-imported Works discovered to be duplicates after
  the fact — `domain-bibliographic.md` FR-4's `MergedInto` mechanism
  exists for this, but invoking it is a library-management feature no
  current phase names, not this import-time flow

## User stories

- As **someone who just added a source**, I want to discover what it
  offers and have obviously-already-owned files skip straight past me,
  so I'm not asked to confirm things I clearly already have.
- As **someone importing an ambiguous file**, I want to see what this
  system thinks it might be, with a confidence indicator, so I can
  correct or confirm it rather than trust an automatic guess.
- As **the maintainer**, I want it structurally impossible for this
  pipeline to silently create a duplicate `Work` for a book the user
  already owns, however confident the automatic matching looks.

## Functional requirements

- **FR-1** `POST /api/v1/import/discover` (body: `{ sourceId }`)
  enumerates the source's contents via `Provider.List`
  (`backend-source-adapter.md` FR-15, a Go-level capability that spec
  amended to expose alongside `Provider.Resolve`, called directly
  in-process — not through that spec's HTTP endpoint, avoiding a
  self-call). For each item, this spec skips it (does not create a new
  `import_candidates` row or job) if **either** of two conditions
  holds: an existing `SourceOffering` matches `(sourceId,
  fileReference)` (already fully imported), **or** an existing
  `import_candidates` row matches `(sourceId, fileReference)` in *any*
  status at all, including `pending`/`queued` (already discovered,
  whether still in flight, awaiting confirmation, or already resolved
  one way or another) — this second condition is what makes rejecting
  or permanently failing a candidate (FR-7) actually stick across a
  later re-discovery of the same source, not merely across the same
  session. Everything else gets a new `import_candidates` row
  (`status: 'queued'`, FR-3 — **not** `'pending'`; see FR-3's full
  status lifecycle, corrected from an earlier draft that conflated
  "discovered" with "ready for review") and one `import` job enqueued
  (`payload: { candidateId }`) per item — **never one job per whole
  source** (`backend-job-queue.md`'s own worked-example reasoning: one
  bad file's job failing must not block its siblings). Returns
  immediately with a count of items queued — discovery itself is
  synchronous, not a job (Open questions: this may need to change for
  a very large source, not solved speculatively here).
- **FR-2** `Register("import", maxAttempts: 3, handler)`
  (`backend-job-queue.md` FR-2) — the job handler: loads the
  `import_candidates` row (FR-3) by `candidateId`, calls
  `Provider.Resolve` (`backend-source-adapter.md`'s existing Go-level
  capability) to get the file's bytes, calls
  `backend-file-extractors.md`'s `Materialize`/`DetectFormat`/`Extract`,
  and on success proceeds to FR-4's matching step; reports progress via
  `ReportProgress` at each stage (`resolving`/`extracting`/`matching`).
  A permanent extraction failure (`ErrOversized`/`ErrTooManyEntries`/
  `ErrMalformed`/`ErrNoTitle` — `backend-file-extractors.md` FR-8's own
  category, handled here as permanent too, since a titleless file is
  no more retryable than a malformed one) is wrapped in
  `job.Permanent(err)`, transitioning the candidate directly to
  `status: 'failed'` — these are properties of the file itself, not
  transient, so retrying gains nothing (`backend-job-queue.md` FR-6's
  own mechanism, its concrete first real use). A `Provider.Resolve`
  failure (the source went unreachable mid-import) is an ordinary,
  retryable error — transient, unlike a malformed file — and leaves
  the candidate at `status: 'queued'` for `backend-job-queue.md`'s own
  retry to pick up again. On a successful extraction, FR-4's matching
  runs and the candidate transitions to exactly one of `'pending'`
  (FR-5's auto-accept condition wasn't met — needs manual review),
  `'auto_imported'` (FR-5/FR-6), or, in the rare case matching itself
  errors unexpectedly, back to a retryable state (Failure modes).
- **FR-3** An `import_candidates` table (migrated via
  `backend-persistence.md`'s `goose` mechanism): `id`, `sourceId`,
  `fileReference` (`jsonb`, `domain-source.md` FR-4's shape), `status`
  — **`'queued'`** (discovered, extraction/matching not yet run or
  still running) | **`'pending'`** (extraction and matching succeeded,
  no auto-accept condition met, awaiting user confirmation) |
  `'auto_imported'` | `'confirmed'` | `'rejected'` | `'failed'` — a
  six-value lifecycle, not five; `'queued'` is the state every row
  starts in (FR-1) and the only non-terminal state besides `'pending'`.
  `extractedMetadata` (`jsonb`, nullable —
  `backend-file-extractors.md`'s `ExtractedMetadata`, populated once
  extraction succeeds, so always present once `status` leaves
  `'queued'` except on a `'failed'` row from a pre-extraction failure),
  `matchCandidates` (`jsonb`, nullable array — FR-4's `MatchCandidate`
  shape, populated alongside `extractedMetadata`), `jobId` (references
  `backend-job-queue.md`'s `jobs.id`, for status traceability, not a
  foreign-key requiring the job to still exist — a job row can be
  pruned independently without breaking this reference's meaning as a
  historical pointer), `lastError` (text, nullable — extraction/
  matching failure detail, same redaction discipline
  `backend-job-queue.md` already applies to its own `last_error`),
  `createdAt`, `updatedAt`. Indexed on `(sourceId, status)` for both
  FR-1's dedup check and the resolution UI's own listing query
  (`frontend-import-confirmation.md`). **Wire/JSON field names are
  camelCase** (`sourceId`, `fileReference`, `extractedMetadata`,
  `matchCandidates`, `jobId`, `lastError`, `createdAt`, `updatedAt`),
  matching this project's existing convention for every other
  `/api/v1` response (`openLibraryWorkKey`, `hasCredential`,
  `nextCursor`) — the database's own column names may stay snake_case
  per `backend-persistence.md`'s existing convention; this FR fixes
  only the wire shape a caller (`frontend-import-confirmation.md`)
  actually sees.
- **FR-4** Matching, run after successful extraction (FR-2): (a) an
  **existing-library search** — if `extractedMetadata.isbn` is
  present, an exact lookup **joined through `LibraryEntry`**, not a
  bare scan of `Edition.isbn` — the query is "an `Edition` whose ISBN
  matches AND has at least one `LibraryEntry`," never just "an
  `Edition` whose ISBN matches." This distinction is load-bearing:
  `domain-library.md` FR-2 defines "owned" as computed from
  `LibraryEntry` existence, not from an `Edition` merely existing in
  the domain (an `Edition` can exist without ever being owned — a
  metadata-only record from some future path). A hit is a candidate
  with `confidence: "exact"`, `type: "existing_edition"`; **more than
  one hit** (two distinct owned `Edition`s sharing an ISBN — nothing
  in `domain-bibliographic.md` prevents this) is treated as *no*
  `"exact"` hit at all for FR-5's purposes — all matching `Edition`s
  are still included as separate `"exact"`-confidence candidates for
  the user to pick between, but FR-5's auto-accept requires exactly
  one, never an ambiguous set; (b) an **Open Library search** —
  `backend-metadata-adapter.md`'s `GET /api/v1/discover?q=<title>
  <author>` (this spec's own internal call to that endpoint's handler,
  not a second HTTP round trip against itself), producing zero or more
  candidates with `confidence: "high" | "medium" | "low"` derived from
  a simple, stated scoring rule: `"high"` — title and at least one
  author match closely (case-insensitive, whitespace-normalised exact
  match after those normalisations, not fuzzy string distance —
  intentionally strict, flagged in Open questions as a placeholder
  scoring rule); `"medium"` — title matches closely, author doesn't or
  is absent; `"low"` — neither matches closely, included only because
  Open Library's own search returned it as a top result. Both searches
  always run (an ISBN hit doesn't skip the Open Library search) —
  `matchCandidates` may contain both `existing_edition` and one or
  more `open_library_work` entries, letting the confirmation UI show
  the full picture rather than this spec silently picking one.

  **Known confusable-input shapes this scoring MUST NOT collapse into
  a higher confidence than the text actually supports.** Three
  distinct, recurring shapes look unambiguous and are not: (1) two
  titles differing only by a parenthetical qualifier (e.g. "Series"
  vs. "Series (Color)") — the case/whitespace normalisation above MUST
  NOT also strip a parenthetical, since that qualifier is frequently
  the *only* text distinguishing two genuinely different Editions; (2)
  two different, unrelated books that share a series name and a
  first-publication year but are actually different volumes; (3) a
  standalone, non-series book whose title happens to match an
  unrelated series' name. None of the three has a stronger signal than
  title text to disambiguate on with the fields this spec extracts, so
  a comparison against exactly this text MUST NOT score any of them
  above `"medium"` — never `"exact"`/`"high"` — leaving FR-5's
  auto-accept boundary intact and the candidate `pending` for a human
  to resolve ([`0049`](../reviews/0049-real-world-edge-case-conformity-review.md),
  finding 1).

  **`MatchCandidate`'s full field shape** (camelCase, FR-3's wire
  convention): `{ type: "existing_edition" | "open_library_work",
  confidence: "exact" | "high" | "medium" | "low", title: string,
  author: string | null, coverUrl: string | null }`, plus exactly one
  identifying field depending on `type` — `editionId` (this system's
  own internal `Edition` ID) for `existing_edition`; `openLibraryWorkKey`
  for `open_library_work` — the exact value `frontend-import-confirmation.md`
  needs to construct FR-7's `confirm` request body, and `title`/
  `author`/`coverUrl` are what that screen renders per candidate
  without a further lookup.
- **FR-5** The **auto-accept rule**, closed and narrow: a candidate
  auto-imports (`status: 'auto_imported'`, no user interaction) **if
  and only if** FR-4(a)'s existing-library search found **exactly one**
  `"exact"`-confidence hit. Every other case — zero hits, more than one
  hit, or an `"high"`-confidence Open Library match with no
  existing-library hit at all — leaves `status: 'pending'` for FR-7's
  manual confirmation. This is deliberately narrower than "high
  confidence is good enough": an existing-library match means "attach
  a newly-found file to a book already unambiguously owned," never
  "create a new `Work`," and this spec never automates the latter,
  however confident a match looks (roadmap's own named risk).
- **FR-6** Auto-import persistence (FR-5's case): in one transaction —
  find the matched, owned `Edition`, create a `SourceOffering`
  (`domain-source.md` FR-2: `Source` + `Edition` + `Format`, per that
  spec's own rule, `Format` sourced *from the `FileReference`*, not
  independently from `extractedMetadata.format` — **this FR corrects
  the persisted `FileReference`'s own `format` field to
  `extractedMetadata.format`, the content-sniffed, verified value,
  before it's used**, so `domain-source.md` FR-2's "`Format` from
  `FileReference`" rule and this spec's own "never trust a source's
  declared format" stance (`backend-file-extractors.md` FR-2) agree
  rather than silently diverging; `observed_at = now`), then create a
  `LibraryEntry` for that `Edition` via an `ON CONFLICT DO NOTHING`
  upsert keyed on `Edition` (`domain-library.md` FR-7's "at most one
  `LibraryEntry` per `Edition`" invariant, enforced at the database
  level, not just checked-then-inserted — `backend-job-queue.md`'s own
  worker pool runs multiple concurrent workers, so two auto-accepting
  jobs for the same `Edition` from two different sources, processed
  concurrently, must not both succeed at inserting), set
  `import_candidates.status = 'auto_imported'`.
- **FR-7** `GET /api/v1/import/candidates?sourceId=<id>&status=<value>`
  — `status` accepts any single value from FR-3's six-value enum, or
  is omittable entirely to return every status for that source (the
  mode `frontend-import-confirmation.md` FR-6 needs to compute its
  in-flight count, alongside the `status=pending`/`status=failed` calls
  its other views use); `sourceId` is likewise optional, omitted to
  list across every source. `POST /api/v1/import/candidates/:id/confirm`
  (body: `{ action: "attach_existing", editionId }` |
  `{ action: "use_open_library_match", openLibraryWorkKey }` |
  `{ action: "create_new" }`) persists synchronously (no job needed —
  this is a single, fast, user-triggered write, not the potentially-
  slow extraction work FR-2 already ran): `attach_existing` — FR-6's
  same persistence, targeting the user-specified `Edition` rather than
  an auto-matched one (covers the case a user recognises a match FR-4's
  strict scoring missed); `use_open_library_match` — constructs a new
  `Work`/`Edition` from `backend-metadata-adapter.md`'s own
  `GET /api/v1/discover/works/:openLibraryId` response (the confirmed
  match's full detail, fetched at confirmation time, not reused stale
  from FR-4's earlier search-result snippet) via that spec's
  `NormalisedWork`/`NormalisedEdition` shapes — **edition selection,
  stated explicitly**: prefer the one `NormalisedEdition` whose `isbn`
  matches `extractedMetadata.isbn` when present; otherwise construct
  the new `Edition` from `extractedMetadata` alone (title/publisher/
  language as extracted), attaching only the `Work`-level Open Library
  reference, never guessing among multiple unrelated editions — then
  FR-6's same `SourceOffering`/`LibraryEntry` creation against the new
  `Edition`; `create_new` — constructs a `Work`/`Edition` directly from
  `extractedMetadata` alone (no Open Library enrichment), for a book
  Open Library simply doesn't have. Each path sets `status:
  'confirmed'`. `POST /api/v1/import/candidates/:id/reject` sets
  `status: 'rejected'` — no library data is created; the candidate row
  is retained (not deleted, FR-1's own dedup check already covers
  this) as a record that this specific file was deliberately declined.
- **FR-8** Three distinct failure categories on FR-7's endpoints,
  matching `backend-errors-and-logging.md` FR-1's taxonomy rather than
  collapsing them: a malformed-shape `candidateId` (not a valid ID
  format at all), or an `action` referencing a syntactically malformed
  `editionId`/`openLibraryWorkKey`, is `400 InvalidInput`; a
  well-formed `candidateId` that doesn't correspond to any row, or a
  well-formed `editionId`/`openLibraryWorkKey` that doesn't correspond
  to anything real (a real `404` from `backend-metadata-adapter.md`
  FR-3 for the latter case), is `404 NotFound`; confirming or rejecting
  an already-terminal candidate (any status except `'pending'`) is
  `409 Conflict` — this project's existing category for a well-formed
  request the current state doesn't allow
  (`backend-source-adapter.md` FR-8's own precedent for the same shape
  of problem).

## Non-functional requirements

- **Performance** — no per-file time budget beyond
  `backend-file-extractors.md`'s own 30-second extraction timeout;
  discovery (FR-1) has no stated budget, flagged as a real gap in Open
  questions for a very large source.
- **Security** — see dedicated section below.
- **Accessibility** — not applicable; `frontend-import-confirmation.md`'s
  concern.
- **Reliability** — FR-5/FR-6's narrow auto-accept scope is this spec's
  core reliability property against silent data corruption; a matching
  bug degrades to "over-asks the user," never "silently creates wrong
  data" (roadmap's own framing, restated here as the concrete
  mechanism).
- **Observability** — an `import_candidates` status transition logs at
  `info` with the candidate `id` and new status, never
  `extractedMetadata`'s content or the source's filename (constitution
  §8's "what someone is importing" extension); `auto_imported`
  specifically logs at `info` too (not `warn` — unlike
  `backend-job-queue.md`'s `dead_letter` transition, an auto-import is
  the expected, intended outcome of FR-5's rule working correctly, not
  something needing attention).

No new dependency (constitution §9) — this spec composes
`backend-source-adapter.md`, `backend-metadata-adapter.md`,
`backend-file-extractors.md`, and `backend-job-queue.md`'s existing
capabilities.

## Domain model

This is the first spec to actually *construct* `domain-source.md`
`SourceOffering` rows and `domain-library.md` `LibraryEntry` rows
(FR-6/FR-7) — every earlier phase that touched these types only ever
read or deferred them. `import_candidates` (FR-3) is this spec's own
new, non-domain persistence type — a staging area between
`backend-file-extractors.md`'s DTO output and real domain data,
existing only until a candidate reaches a terminal status; it is never
itself a domain type and is never queried by `domain-library.md`/
`domain-source.md`-facing repositories.

## API and contracts

All endpoints follow `architecture-contracts.md`'s existing
conventions (base path `/api/v1`, error envelope, versioning).

- `POST /api/v1/import/discover` → `200 { queued: number }` → `400`, `404` (unknown `sourceId`)
- `GET /api/v1/import/candidates` → `200 { candidates: ImportCandidate[] }`
- `POST /api/v1/import/candidates/:id/confirm` → `200 { candidate: ImportCandidate }` → `400`, `404`, `409`
- `POST /api/v1/import/candidates/:id/reject` → `200 { candidate: ImportCandidate }` → `404`, `409`

## State transitions

`queued → pending` (FR-2, successful extraction/matching, no
auto-accept) | `queued → auto_imported` (FR-2/FR-5/FR-6, successful
extraction/matching, auto-accept condition met) | `queued → failed`
(FR-2, `backend-file-extractors.md`'s permanent errors via
`job.Permanent`) | `queued → queued` (a transient `Provider.Resolve`
failure, `backend-job-queue.md`'s own retry loop — not a status
change this spec's own state machine tracks separately, since the row
stays `queued` throughout) | `pending → confirmed` (FR-7, user-driven,
one of three sub-paths) | `pending → rejected` (FR-7). Illegal: any
transition out of `auto_imported`/`confirmed`/`rejected`/`failed` —
all four are terminal; a mistakenly-rejected or wrongly-auto-imported
file has no "undo" this spec provides (Open questions). Also illegal:
any transition directly from `queued` to `confirmed`/`rejected` —
FR-7's confirm/reject endpoints only ever act on a `pending` candidate
(FR-8's `409` otherwise), never one still awaiting extraction.

## Failure modes

| Failure | Detected how | Caller/user sees | System does |
|---|---|---|---|
| Extraction fails permanently (malformed/oversized/titleless file) | `backend-file-extractors.md`'s error, wrapped `job.Permanent` | Candidate `status: 'failed'`, `lastError` set to the error category | No retry (FR-2); visible in the resolution UI as a distinct failed state, not silently dropped |
| Source unreachable mid-resolve | `Provider.Resolve` error | Candidate stays `queued`, job retries per `backend-job-queue.md`'s normal backoff | Ordinary transient retry, not `job.Permanent` |
| Open Library search fails (`backend-metadata-adapter.md` FR-9's `Unavailable`) | That spec's own error mapping | Matching proceeds with existing-library results only, Open Library candidates simply absent this round | Not treated as this job's own failure — a degraded, not failed, matching pass |
| FR-4(a)'s existing-library search finds more than one `"exact"` ISBN hit | Multiple rows returned | Candidate `status: 'pending'`, both hits shown as separate `existing_edition` match candidates | FR-5's auto-accept requires exactly one hit; never auto-imports on an ambiguous set |
| Two concurrent auto-accepting jobs target the same `Edition` | `ON CONFLICT DO NOTHING` on `LibraryEntry`'s upsert (FR-6) | Both candidates reach `status: 'auto_imported'`; only one `LibraryEntry` row exists | No duplicate `LibraryEntry`, per `domain-library.md` FR-7 |
| Confirming or rejecting a nonexistent `candidateId` | Row lookup miss | `404 NotFound` | No mutation |
| Confirming an already-terminal candidate | FR-8's state check | `409 Conflict` | No double-persistence |
| `use_open_library_match` confirmation references a work Open Library no longer has | `backend-metadata-adapter.md` FR-3's `NotFound` at fetch time | `404` on the confirm call itself | Candidate remains `pending`, user can pick a different action |

## Security considerations

- **No new external trust boundary** — this spec composes existing
  adapters (`backend-source-adapter.md`, `backend-metadata-adapter.md`),
  neither of which this spec bypasses or re-implements; every hostile-
  input defence those specs and `backend-file-extractors.md` already
  established applies unchanged here.
- **The narrow auto-accept rule (FR-5) is itself the primary security-
  relevant property of this spec** — restated from Reliability above
  because it matters here specifically: a matching-logic bug has a
  bounded blast radius (asks unnecessarily), never an unbounded one
  (creates wrong data unsupervised).
- **`import_candidates.extractedMetadata`/`lastError` follow
  `backend-job-queue.md`'s existing redaction discipline** — no secret
  ever reaches either field, since neither ever contains a credential
  (sources are referenced by ID, per that spec's own FR-3 payload
  rule, restated here for this new storage location).
- **Concurrent auto-accept never double-inserts `LibraryEntry`** (FR-6)
  — enforced at the database level (`ON CONFLICT DO NOTHING` against a
  unique constraint on `Edition`), not merely checked-then-inserted in
  application code, which `backend-job-queue.md`'s real concurrent
  worker pool would otherwise be able to race.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Matching confidence scoring (FR-4) against fixture `ExtractedMetadata`/Open Library response pairs; the auto-accept rule's narrow boundary (exactly one exact-ISBN existing-library match auto-imports; zero, two, or a `"high"`-confidence Open-Library-only match does not) |
| Integration | Discovery → job enqueue (`status: 'queued'`) → extraction → matching → `pending`/`auto_imported`/`failed`, against a real PostgreSQL instance and a fake source/fake Open Library server (`backend-test-harness.md`'s harness, `backend-source-adapter.md`'s and `backend-metadata-adapter.md`'s own fake-server patterns reused); a re-discovery test asserting a `rejected`/`failed`/`confirmed` candidate is never re-created by a second `POST /api/v1/import/discover` call; confirm/reject endpoints against a real transaction, including the `404` (nonexistent) and `409` (terminal-state) cases; a concurrency test with two simultaneous auto-accepting jobs targeting the same `Edition`, asserting exactly one `LibraryEntry` row results; a multi-file batch where one file's job dead-letters, proving sibling jobs still complete |
| Contract | `architecture-contracts.md` FR-3's `kin-openapi` tool, extended to `/api/v1/import*` |
| E2E | Add a source → discover → an exact-ISBN-match file auto-imports → a no-match file appears pending → confirm via `create_new` → library reflects both entries |
| Accessibility | N/A at this layer — `frontend-import-confirmation.md`'s concern |

Tests that must fail before implementation begins: a test asserting a
`"high"`-confidence Open-Library-only match (no existing-library hit)
never auto-imports, only an exact-and-unambiguous ISBN existing-library
match does; a test asserting a permanently-failed extraction
(`job.Permanent`, including `ErrNoTitle`) never retries; a test
asserting confirming an already-`confirmed` candidate returns `409`,
not a second `SourceOffering`/`LibraryEntry`; a test asserting a
rejected candidate is never re-surfaced by a later discovery call
against the same source; a test asserting each of FR-4's three named
confusable shapes (a parenthetical-qualifier-only difference, a
same-name/same-year different-volume collision, and a standalone title
colliding with an unrelated series name) scores no higher than
`"medium"` confidence and never auto-imports.

## Acceptance criteria

- [ ] Discovery skips files already represented by a `SourceOffering`
      **or** an existing `import_candidates` row in any status, and
      enqueues one job per remaining file with `status: 'queued'`
- [ ] Exactly one unambiguous exact ISBN match to an owned `Edition`
      auto-imports without user interaction; zero or multiple matches
      do not
- [ ] Every other match confidence leaves the candidate `pending` for
      manual confirmation, proven by a test, not just this rule's prose
- [ ] All three confirmation actions (`attach_existing`,
      `use_open_library_match`, `create_new`) correctly construct
      `SourceOffering`/`LibraryEntry` data, with `use_open_library_match`'s
      edition-selection rule proven against a multi-edition fixture
- [ ] Rejecting a candidate creates no library data and isn't
      re-surfaced by a later discovery of the same source
- [ ] A permanently-failed file's job never retries; a transiently-
      failed one does, per `backend-job-queue.md`'s normal policy
- [ ] Two concurrent auto-accepting jobs for the same `Edition` never
      produce two `LibraryEntry` rows
- [ ] The contract test passes against `/api/v1/import*`

## Open questions

- **Discovery (FR-1) has no size/time budget for a very large source**
  — flagged, not solved; may need its own job if real usage shows
  synchronous discovery is too slow.
- **Matching confidence scoring (FR-4) is a strict, placeholder rule**
  (exact-after-normalisation, not fuzzy string distance) — deliberately
  conservative for a first version; revisit once real import usage
  shows whether it's too strict (too many `"low"`/no-match results for
  clearly-correct books) or appropriately cautious.
- **No "undo" for a wrongly-auto-imported or mistakenly-rejected
  candidate** — both are terminal states this spec provides no reversal
  path for; a wrongly-auto-imported `LibraryEntry` can only be removed
  via `backend-library-api.md`'s existing removal endpoint (a
  library-management action, not an import-specific undo), and a
  mistakenly-rejected candidate has no path back to `pending` at all —
  flagged as a real gap, not solved here.
- **FR-1's dedup key, `(sourceId, fileReference)`, doesn't match
  `domain-source.md`'s own `SourceOffering` uniqueness key of
  `(Source, Edition, Format)`** — if a source's `FileReference` for a
  given physical file changes between discoveries (a file replaced in
  place), this spec treats it as a new candidate and re-runs the whole
  pipeline; if that produces the same `(Edition, Format)` as an
  existing `SourceOffering`, this spec doesn't specify re-observation
  semantics (`domain-source.md` FR-2's own "re-observed... updated,
  not a new row" rule) distinctly from "genuinely new offering." A
  low-likelihood edge case, not solved here.
- **A source item legitimately spanning multiple files** — FR-1's
  discovery and FR-3's `import_candidates` model exactly one row per
  discovered file, and this spec's matching/auto-accept (FR-4/FR-5)
  has no representation for several files jointly constituting one
  reading unit rather than independent items. Whether this is out of
  scope entirely (each file imports as its own, separate candidate — a
  known-degraded outcome) or needs a real extension is unresolved;
  `domain-source.md` and `backend-file-extractors.md` note the same
  gap from their own sides
  ([`0049`](../reviews/0049-real-world-edge-case-conformity-review.md),
  finding 2).

## References

- `domain-source.md` (phase 02) FR-2, FR-4 — `SourceOffering`,
  `FileReference`, both constructed here for the first time
- `domain-library.md` (phase 02) FR-1, FR-7 — `LibraryEntry` creation
  and no-op-if-owned rule
- `domain-bibliographic.md` (phase 02) FR-4 — the matching-decision
  deferral this spec closes
- `backend-source-adapter.md` (phase 08, amended FR-15) — `Provider.List`/
  `Provider.Resolve` capabilities reused directly; FR-8's `409 Conflict`
  precedent
- `backend-metadata-adapter.md` (phase 07) — Open Library search/work
  lookup reused for matching and `use_open_library_match` enrichment
- `backend-file-extractors.md` (phase 10, sibling) — extraction this
  spec's job handler calls, including FR-8's `ErrNoTitle` category
- `backend-job-queue.md` (phase 09) — job registration, retry/
  `job.Permanent`, `ReportProgress`, redaction discipline reused
- `backend-persistence.md` (phase 03) — transaction pattern for FR-6's
  multi-row writes
- `backend-errors-and-logging.md` (phase 03) FR-1 — error taxonomy,
  the 400/404/409 split FR-8 follows
- Constitution §4 (hostile input, inherited from composed specs), §8
  (redaction), §9 (no new dependency)
