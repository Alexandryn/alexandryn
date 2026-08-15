# Spec: Backend reading API

| | |
|---|---|
| **Status** | `REVIEWED` |
| **Phase** | `11-reader` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0038`](../reviews/0038-phase11-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (7 Blocking across the batch, 3 confirmed independently by both passes; 9 Major, 6 Minor), all findings fixed; awaiting maintainer approval |

## Context

`domain-reading.md` (phase 02) fixed the model — `ReadingProgress`
(singleton per `Work`, FR-1), `Bookmark`/`Highlight` (per `Edition`,
FR-3), `ReadingPreferences` (per `DeviceID`, FR-5), and
`ReconcileProgress` (FR-6/FR-7, a pure domain function reconciling an
incoming `ProgressReport` into the canonical `ReadingProgress`) — but
never wired any of it to persistence or a transport, leaving that
explicitly to "phase 03/14" (its own Non-goals). This spec is where
that wiring happens for the first time: real storage, real endpoints,
and `ReconcileProgress` actually called by something.

`DeviceID` (`domain-reading.md` FR-2/FR-5) has no concrete source yet
— no phase before this one has defined how a specific device gets
identified. This spec fixes it for the single-device-class this
project currently supports (a browser tab, desktop or LAN): a
client-generated UUID, persisted client-side, sent on every request.
Phase 12 (authentication) and phase 14 (cross-device sync) may extend
this later; this spec doesn't assume what they'll add, only that
`DeviceID` needs a real value now, before either exists.

## Problem

Nothing exists yet to store or serve a `ReadingProgress`,
`Bookmark`, `Highlight`, or `ReadingPreferences`, or to call
`ReconcileProgress` against a real incoming report.

## Goals

- `GET`/report endpoints for `ReadingProgress`, wired through
  `ReconcileProgress`
- CRUD endpoints for `Bookmark` and `Highlight`, scoped per `Edition`
- `GET`/`PUT` for `ReadingPreferences`, scoped per `DeviceID`
- A concrete, minimal `DeviceID` source: a client-generated UUID sent
  as a header, validated shape-wise, nothing more assumed about it yet

## Non-goals

- Any UI — `frontend-reader.md`'s job; this spec is the API surface
  that screen consumes
- Real multi-device sync transport (push notifications, WebSockets) —
  phase 14; this spec's endpoints are request/response only, the same
  pattern every other API in this project already uses
- Any authentication/authorization on `DeviceID` — phase 12; a
  `DeviceID` here is a self-reported, unauthenticated client value,
  same loopback-trust model every phase through 11 shares
- EPUB content itself — `backend-reader-content.md`'s job

## User stories

- As **`frontend-reader.md`**, I want to report reading progress as
  the user reads and have it reconciled correctly against whatever
  this device (or another) last reported, so progress never regresses
  from an out-of-order report.
- As **someone who highlights a passage**, I want it saved against the
  exact edition I'm reading, so it's there next time I open that same
  book.
- As **someone switching between a phone-sized window and a desktop
  one**, I want each to remember its own font/theme preference, not
  fight over a shared one.

## Functional requirements

- **FR-1** The progress-report endpoints (FR-2) and preferences
  endpoints (FR-8) — the two places `domain-reading.md` actually
  attaches a `DeviceID` to anything (`ReadingProgress`'s provenance
  fields, FR-2; `ReadingPreferences`'s own scoping key, FR-5) —
  require an `X-Device-Id` header: a client-generated UUID (v4),
  validated as UUID-shaped (`400 InvalidInput` otherwise), never
  validated for authenticity beyond shape (Non-goals — this is a
  self-reported value, matching every other phase's loopback-trust
  model, not a new exception to it). `frontend-reader.md` owns
  generating and persisting this value client-side; this spec only
  consumes it. **Bookmark/highlight endpoints (FR-6/FR-7) do not
  require this header** — `domain-reading.md`'s own `Bookmark`/
  `Highlight` types carry no `DeviceID` field at all (only `Edition`),
  so requiring one there would be collected and silently discarded,
  which this spec deliberately doesn't do.
- **FR-2** `GET /api/v1/reading/works/:workId/progress` returns the
  canonical `ReadingProgress` for that `Work` (`domain-reading.md`
  FR-1's singleton), or `{ progress: null }` if none exists yet (a
  legal, common state — a `Work` nobody has started). `POST
  /api/v1/reading/works/:workId/progress` (body: `{ percentage: number,
  precisePosition: { editionId, cfi: string } | null }`) constructs a
  `ProgressReport` (`domain-reading.md` FR-2: this `Work` ID, the
  reported `percentage`, optional `precisePosition`, the `X-Device-Id`
  header's value, current server time as reported-at) and calls
  `ReconcileProgress` against the current canonical value (or treats
  this report as the first canonical value directly if none exists —
  `domain-reading.md`'s own State transitions section's "no
  reconciliation needed, nothing to reconcile against" case), then
  persists the result in one transaction. **The read of the current
  canonical value and the write of the reconciled result MUST happen
  within the same transaction, with the read acquiring a row lock
  (`SELECT ... FOR UPDATE` against the singleton `ReadingProgress` row
  for this `Work`, or an equivalent `INSERT ... ON CONFLICT` compare-
  and-swap keyed on the row's own last-written value) — without this,
  two concurrent reports for the same `Work` can both read the same
  stale canonical value, both compute their own "reconciled" result
  independently, and whichever commits second silently overwrites the
  first's result even if the first's was the furthest-wins outcome,
  exactly the lost-update bug `ReconcileProgress`'s own
  commutative/associative guarantee (`domain-reading.md` FR-6) exists
  to prevent — a guarantee about the pure function's own mathematics,
  not a guarantee about a read-then-write sequence with no isolation
  around it.** Returns the new canonical `ReadingProgress`, which the
  caller MUST NOT assume equals what it just submitted —
  `ReconcileProgress`'s furthest-wins rule (`domain-reading.md` FR-6)
  may return the *existing* value unchanged if the incoming report is
  behind it.
- **FR-3** `percentage` MUST be validated `[0.0, 1.0]` at this
  boundary (`400 InvalidInput` otherwise) — restating
  `domain-reading.md` FR-2's own construction-time invariant at the
  transport layer, the same "validate again at the boundary, don't
  assume the domain layer's check is the only one a caller will ever
  hit" discipline this project has applied everywhere else external
  input crosses into a domain construction call.
- **FR-4** `precisePosition.cfi` (when present) MUST pass a **shallow
  structural check** — begins with the literal `epubcfi(`, ends with
  `)`, brackets/parentheses balanced, and contains only characters the
  CFI grammar's own step/offset/assertion syntax permits — `400
  InvalidInput` otherwise. This is deliberately **not** full W3C/IDPF
  grammar validation: the roadmap's own Architecture decisions
  expected section already named EPUB CFI's grammar as "a genuinely
  intricate addressing grammar" and chose to lean on `foliate-js`'s
  own `epubcfi.js` for *generation and resolution*, precisely to avoid
  hand-rolling that grammar — this FR is a narrower, different
  problem: rejecting an obviously-malformed or hostile string (an
  oversized value, a value containing characters the grammar could
  never produce) before it's ever stored, not verifying the string is
  a semantically valid CFI a resolver could act on. A structurally-
  shallow-valid-but-semantically-nonsensical CFI (one this narrow check
  passes but no real position resolves to) is an accepted, named case
  (Open questions) `frontend-reader.md`'s own rendering engine handles
  via its documented `Percentage`-fallback path, not something this
  endpoint is responsible for catching — this is `domain-reading.md`'s
  own "opaque to this spec" `PrecisePosition` value finally given a
  concrete, *shape*-validated form for the EPUB format (roadmap's own
  named architecture decision, narrowed here to be honest about what
  "validated" actually means at this boundary).
- **FR-5** `precisePosition.editionId` MUST belong to the same `Work`
  as `:workId` (`400 InvalidInput` otherwise) — the transport-layer
  restatement of `domain-reading.md`'s own illegal-transition rule ("a
  `PrecisePosition` whose tagged `Edition` does not belong to the
  `ReadingProgress`'s own `Work`").
- **FR-6** `GET /api/v1/reading/editions/:editionId/bookmarks` and
  `POST` (body: `{ cfi: string, label: string | null }`) — `domain-reading.md`
  FR-3/FR-4's `Bookmark`, one position (FR-4's own CFI validation
  reused), optional label (validated per `domain-bibliographic.md`
  FR-5's string discipline, this project's existing pattern for every
  user-typed string). `DELETE /api/v1/reading/bookmarks/:bookmarkId`
  removes one. No `PATCH` — a bookmark's position is fixed at creation;
  changing it is delete-and-recreate, not an update, since a bookmark
  MUST represent an exact position (Non-goals: this spec adds no
  "move a bookmark" concept `domain-reading.md` doesn't already have).
- **FR-7** `GET /api/v1/reading/editions/:editionId/highlights` and
  `POST` (body: `{ startCfi: string, endCfi: string, note: string |
  null, category: string | null }`) — `domain-reading.md` FR-3/FR-4's
  `Highlight`. `endCfi` MUST NOT sort before `startCfi` per the CFI
  specification's own defined ordering (`400 InvalidInput` otherwise
  — restating `domain-reading.md`'s own "end position before start
  position" illegal-transition rule at this boundary). `PATCH
  /api/v1/reading/highlights/:highlightId` updates `note`/`category`
  only, never the position (same reasoning as FR-6). `DELETE
  /api/v1/reading/highlights/:highlightId` removes one.
- **FR-8** `GET /api/v1/reading/preferences` and `PUT` (body:
  `{ font: string, fontSize: number, theme: string, lineSpacing:
  number }` — the concrete field set `domain-reading.md` FR-5 left to
  this phase, matching `frontend-reader.md`'s own typography/theme
  UI) — scoped entirely by the `X-Device-Id` header (FR-1), no
  `deviceId` in the URL or body (there's exactly one legal source for
  it per request, so a second one would only create a spoofing
  surface with no benefit). `GET` for a `DeviceID` with no stored
  preferences yet returns system defaults (`domain-reading.md` FR-5's
  own "new device starts from system defaults" rule) rather than
  `404` — a missing preferences row is not an error state.

## Non-functional requirements

- **Performance** — no specific budget beyond this project's existing
  per-request expectations; every endpoint here is a single-row read
  or a small, indexed write.
- **Security** — see dedicated section below.
- **Accessibility** — not applicable; `frontend-reader.md`'s concern.
- **Reliability** — `ReconcileProgress`'s own commutative/associative
  property (`domain-reading.md` FR-6) is what makes FR-2's reconcile-
  and-persist step safe under concurrent reports from multiple
  devices; this spec adds no additional concurrency control beyond a
  single-row transactional update, since the domain function itself
  already guarantees order-independence.
- **Observability** — per `domain-reading.md`'s own Security
  considerations (restated as this spec's own load-bearing
  requirement, not merely inherited): no `Work`/`Edition` title,
  highlight text, bookmark label, or CFI value ever appears in a log
  line. A reconciliation event logs at `info` with only the `Work` ID
  and which device's report won — never the position or percentage
  value itself, which is reading-progress information, not merely an
  identifier.

No new dependency (constitution §9) — this spec composes
`domain-reading.md`'s existing pure functions with
`backend-persistence.md`'s existing storage patterns.

## Domain model

This is the first spec to persist `domain-reading.md`'s types for
real — `ReadingProgress`, `Bookmark`, `Highlight`, `ReadingPreferences`
all get real tables here (migrated via `backend-persistence.md`'s
`goose` mechanism), and `ReconcileProgress` gets its first real caller.
No new domain types are introduced; this spec is purely the
persistence/transport wiring `domain-reading.md` itself deferred.

## API and contracts

This spec's package, `internal/reader/api`, sits alongside
`internal/reader/content` in `architecture-backend.md` FR-1's layout —
it depends on `internal/domain` for `ReconcileProgress` and the
`ReadingProgress`/`Bookmark`/`Highlight`/`ReadingPreferences` types
directly, unlike `internal/reader/content`, which never touches
`domain-reading.md`'s types at all.

All endpoints follow `architecture-contracts.md`'s existing
conventions (base path `/api/v1`, error envelope, versioning); FR-1
specifies exactly which endpoints require the `X-Device-Id` header.

- `GET /api/v1/reading/works/:workId/progress` → `200 { progress: ReadingProgress | null }`
- `POST /api/v1/reading/works/:workId/progress` → `200 { progress: ReadingProgress }` → `400`
- `GET /api/v1/reading/editions/:editionId/bookmarks` → `200 { bookmarks: Bookmark[] }`
- `POST /api/v1/reading/editions/:editionId/bookmarks` → `201 { bookmark: Bookmark }` → `400`
- `DELETE /api/v1/reading/bookmarks/:bookmarkId` → `204` → `404`
- `GET /api/v1/reading/editions/:editionId/highlights` → `200 { highlights: Highlight[] }`
- `POST /api/v1/reading/editions/:editionId/highlights` → `201 { highlight: Highlight }` → `400`
- `PATCH /api/v1/reading/highlights/:highlightId` → `200 { highlight: Highlight }` → `400`, `404`
- `DELETE /api/v1/reading/highlights/:highlightId` → `204` → `404`
- `GET /api/v1/reading/preferences` → `200 { preferences: ReadingPreferences }` (defaults if none stored)
- `PUT /api/v1/reading/preferences` → `200 { preferences: ReadingPreferences }` → `400`

## State transitions

Mirrors `domain-reading.md`'s own State transitions section exactly —
this spec adds no state machine of its own; `ReconcileProgress` (FR-2)
is the one already-specified transition this spec actually invokes.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Missing/malformed `X-Device-Id` | FR-1's shape check | `400 InvalidInput` | No request processed |
| `percentage` outside `[0.0, 1.0]` | FR-3's boundary check | `400 InvalidInput` | No write |
| Malformed/hostile CFI string | FR-4's shallow structural check | `400 InvalidInput` | No write |
| `precisePosition.editionId` belongs to a different `Work` | FR-5's check | `400 InvalidInput` | No write |
| `endCfi` sorts before `startCfi` | FR-7's ordering check | `400 InvalidInput` | No write |
| Two devices submit progress concurrently for the same `Work` | `ReconcileProgress`'s own deterministic function, called serially per request | Each caller sees the reconciled canonical value at the time their own request completed | No lost update — `domain-reading.md` FR-6's commutative/associative guarantee holds regardless of arrival order |
| Deleting a nonexistent bookmark/highlight | Row lookup miss | `404 NotFound` | No-op |

## Security considerations

- **What someone reads never appears in logs** — restated as this
  spec's own load-bearing requirement (`domain-reading.md`'s own
  Security considerations), not merely inherited: no title, highlight
  text, bookmark label, CFI value, or percentage in any log line this
  spec's handlers emit.
- **`X-Device-Id` is unauthenticated** — explicitly not a security
  boundary (Non-goals); any client can claim any `DeviceID`, same as
  every other loopback-trusted request in this project through phase
  11. Named here so it isn't mistaken for an access-control mechanism
  it was never designed to be.
- **Highlight notes and bookmark labels are hostile input** —
  `domain-reading.md`'s own Security considerations already named
  this; this spec is where a real HTTP boundary actually receives one,
  validated per `domain-bibliographic.md` FR-5's existing discipline.
- **No new trust boundary** — still loopback-only, still no
  authentication between the LAN client and this host.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | CFI shape validation against real and malformed fixture strings; `percentage`/ordering boundary checks |
| Integration | Full progress-report → reconcile → persist round trip against a real PostgreSQL instance (`backend-test-harness.md`'s harness); a two-device-concurrent-report test proving `ReconcileProgress`'s order-independence holds through this spec's own persistence layer, not just in the pure domain function's own already-existing tests; bookmark/highlight/preferences CRUD |
| Contract | `architecture-contracts.md` FR-3's `kin-openapi` tool, extended to `/api/v1/reading*` |
| E2E | Report progress from one simulated device, then a "further" report from another → canonical progress reflects the furthest; create a bookmark and a highlight → both persist and are retrievable |
| Accessibility | N/A at this layer — `frontend-reader.md`'s concern |

Tests that must fail before implementation begins: a test asserting a
"behind" progress report doesn't move the canonical value backward
without an explicit override; a test asserting an obviously malformed/hostile CFI string
is rejected before any write; a test asserting no reading-related
content ever appears in captured log output across every endpoint in
this spec.

## Acceptance criteria

- [ ] Progress reporting correctly reconciles via `ReconcileProgress`,
      proven under concurrent multi-device reports
- [ ] `PrecisePosition`'s CFI value is validated at the boundary, not
      only trusted from the client
- [ ] Bookmark/highlight CRUD works against a real PostgreSQL instance
- [ ] Preferences are correctly scoped per `DeviceID`, with new-device
      defaults matching `domain-reading.md` FR-5
- [ ] No reading-related content appears in logs, proven by a test
- [ ] The contract test passes against `/api/v1/reading*`

## Open questions

- **`DeviceID`'s long-term relationship to phase 12's user accounts**
  — this spec treats it as a bare, unauthenticated client value;
  whether phase 12 wraps it in a real identity, replaces it, or leaves
  it as-is (a device concept orthogonal to a user concept) is that
  phase's own question, not resolved here.
- **No CFI *grammar* or *resolution* validation, only a shallow
  structural check** — FR-4 checks bracket/parenthesis balance and
  character-set plausibility, not full W3C/IDPF grammar validity and
  not that the referenced position actually exists within the specific
  Edition's content (either would require either a real Go CFI grammar
  library — not currently justified under constitution §9, since no
  such library has been evaluated — or re-parsing the EPUB server-side
  for every progress report, a real cost this spec doesn't take on); a
  shallow-valid but grammar-invalid or semantically nonsensical CFI is
  accepted and stored as-is — `frontend-reader.md`'s own rendering
  engine is what would surface such a value as unusable (via its
  documented `Percentage`-fallback path), not this API. If real usage
  shows hostile or malformed CFI values causing problems downstream, a
  real Go CFI grammar library, properly justified, is the concrete
  next step — not a reason to hand-roll the full grammar now.

## References

- `domain-reading.md` (phase 02) — the full domain model this spec
  persists and transports for the first time
- `domain-bibliographic.md` (phase 02) FR-5 — string validation
  discipline reused for labels/notes
- `backend-persistence.md` (phase 03) — storage/transaction patterns
- `backend-errors-and-logging.md` (phase 03) FR-1 — error taxonomy
- `backend-test-harness.md` (phase 03) — integration harness
- `backend-metadata-caching.md`/`backend-import-pipeline.md` — no
  direct reuse, but this spec follows the same "wire an existing
  domain/pure-function layer to real persistence" shape those phases'
  own backend specs already established
- EPUB Canonical Fragment Identifiers specification (W3C/IDPF) — the
  grammar FR-4 validates against
- Constitution §4 (boundary validation restated), §8 (reading privacy)
