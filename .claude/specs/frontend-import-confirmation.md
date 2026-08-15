# Spec: Frontend import confirmation

| | |
|---|---|
| **Status** | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| **Phase** | `10-import` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0037`](../reviews/0037-phase10-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (7 Blocking across the batch, each found by one pass; 11 Major, 6 Minor, 2 Nit), all findings fixed; approved by maintainer 2026-08-15 |

## Context

`backend-import-pipeline.md` provides `/api/v1/import*`: trigger
discovery, list pending candidates, confirm or reject each one.
`frontend-source-management.md` (phase 08) is where a source's browse
view already lives — this spec adds the "import from this source"
action that view was missing (its own Non-goals named this explicitly:
"no 'add to library' action exists yet... phase 10's import flow").
`frontend-discover-screen.md` (phase 07) established this project's
pattern for rendering an unmatched candidate (title/author/cover, no
ownership fields) — this spec's own candidate cards follow the same
shape for a materially similar reason.

## Problem

Nothing triggers discovery or lets a user review/confirm/reject a
pending import candidate. `backend-import-pipeline.md`'s own narrow
auto-accept rule means most real imports will need this screen, not
skip it.

## Goals

- An "Import from this source" action on `frontend-source-management.md`'s
  existing source browse view, triggering discovery
- A pending-candidates screen: each candidate's extracted metadata,
  suggested matches with confidence, confirm/reject actions
- Batch progress visibility, using `backend-job-queue.md`'s existing
  status API indirectly (via `backend-import-pipeline.md`'s own
  candidate-status surface, not a direct frontend call into the job
  queue's internal API, which has no HTTP surface at all)
- Clear treatment of a `failed` candidate (permanently unextractable
  file), distinct from `pending`

## Non-goals

- Anything past confirming an import — no "start reading" action; that
  belongs to phase 11's reader, and doesn't exist yet
- Undoing a confirmed/auto-imported entry — `backend-import-pipeline.md`'s
  own named gap (Open questions); this screen has no undo to offer
  either
- Editing extracted metadata before confirming (correcting a
  mis-extracted title, for instance) — the three confirm actions
  (`attach_existing`/`use_open_library_match`/`create_new`) all use
  metadata exactly as extracted or matched; in-place editing is a
  library-management feature no current phase names

## User stories

- As **someone who just added a source**, I want a single "Import"
  action that discovers everything and shows me what needs my
  attention, so I'm not manually triggering discovery per file.
- As **someone reviewing a pending candidate**, I want to see the
  extracted title/author alongside suggested matches with their
  confidence, so I can quickly confirm an obviously-correct suggestion
  or pick a different one.
- As **someone whose file failed to import**, I want to know it failed
  and roughly why (corrupted file, unsupported format), not see it
  silently vanish from the list.

## Functional requirements

- **FR-1** `frontend-source-management.md`'s source browse view
  (`/sources/:id`) gains an "Import from this source" button, calling
  `POST /api/v1/import/discover`. On success, navigates to this spec's
  own candidates screen (`/import`, FR-2), scoped to that source via a
  `?sourceId=` query parameter — matching `frontend-library-screens.md`
  FR-2's URL-as-state precedent for a filter value.
- **FR-2** `/import` renders a `<Sidebar>`-framed `<ContentPane>`
  (`frontend-shell-and-routing.md` FR-3) listing pending candidates
  (`GET /api/v1/import/candidates?status=pending`) as cards: extracted
  `title`/`authors` (or a placeholder — "Untitled" — when extraction
  produced no title but the candidate still reached this screen via a
  path this spec doesn't otherwise create, since `backend-import-pipeline.md`
  FR-2 treats a titleless extraction (`ErrNoTitle`) as a permanent
  failure, landing it in FR-5's `failed` section instead; this
  placeholder exists purely as a defensive rendering choice, not an
  expected case), extracted `coverBytes` rendered as a data URL when
  present (nothing to fetch remotely — extraction already returned the
  bytes) falling back to `frontend-generated-covers.md`'s existing
  procedural cover when absent, and a list of `matchCandidates`
  (`backend-import-pipeline.md` FR-4's full `MatchCandidate` shape:
  `type`, `confidence`, `title`, `author`, `coverUrl`, plus
  `editionId` or `openLibraryWorkKey`) each showing its own confidence
  (`exact`/`high`/`medium`/`low` — a visible label, not just a color,
  per constitution §11's specificity bar and this project's existing
  accessibility conventions around not conveying information by color
  alone) and the candidate's own `title`/`author`/`coverUrl` as its
  identifying label — the exact fields FR-4 provides for this, no
  further lookup needed to render or to construct FR-3's confirm
  request body.
- **FR-3** Each candidate card offers three confirm actions matching
  `backend-import-pipeline.md` FR-7's three action shapes — "Use this
  match" per suggested candidate (`attach_existing` or
  `use_open_library_match`, depending on which kind of candidate was
  clicked), and "This isn't in Open Library — add from extracted info"
  (`create_new`) as a distinct, always-available action regardless of
  whether any match candidates exist. A "Reject" action is available
  independent of the confirm actions — rejecting doesn't require first
  considering every suggested match.
- **FR-4** Confirming or rejecting a candidate removes it from the
  visible pending list once the mutation resolves and its response is
  received — via TanStack Query's mutation-driven cache invalidation
  of the `GET /api/v1/import/candidates` query, the same *non*-
  optimistic pattern `frontend-collections-screens.md` FR-4 already
  established (that spec's own State transitions section explicitly
  treats rendering ahead of server confirmation as an illegal
  transition it deliberately avoids — this FR follows the same
  discipline, not an "optimistic" one; the card disappears because the
  server has already confirmed the new state, not in anticipation of
  it).
- **FR-5** A `failed` candidate (`backend-import-pipeline.md`'s
  permanent-extraction-failure case, including `ErrNoTitle`) is shown
  in a visually and textually distinct section from `pending` ones —
  "Couldn't be imported" — fetched via `GET
  /api/v1/import/candidates?status=failed&sourceId=<id>`
  (`backend-import-pipeline.md` FR-7's per-status query) — with the
  file's declared format (from `fileReference`, always present even
  when extraction itself failed) and a plain failure category derived
  from `lastError` (constitution §11: "This file appears to be damaged
  or is not a valid EPUB/PDF/CBZ file," not a raw error string). No
  confirm/reject actions apply to a `failed` candidate — there's
  nothing to confirm; the only available action is dismissing it from
  view (a client-side-only dismissal, since `backend-import-pipeline.md`
  provides no endpoint to delete a `failed` row, and doesn't need
  one — the row persists as a historical record, this screen simply
  stops showing a dismissed one, tracked in `localStorage` by
  candidate ID, the same zero-dependency justification
  `frontend-library-screens.md` FR-1 already gave for its own
  `localStorage` use).
- **FR-6** While an `import` job (`backend-job-queue.md`) is still
  extracting/matching a just-discovered file, that candidate stays at
  `status: 'queued'` (`backend-import-pipeline.md` FR-3's own six-value
  lifecycle — `'queued'` is a real, distinct, fetchable status, not an
  unobservable gap between discovery and `'pending'`) and so doesn't
  appear in either the `pending` or `failed` list yet — this screen
  shows a simple in-flight count instead ("Processing 12 of 40..."),
  computed as the count returned by `GET
  /api/v1/import/candidates?status=queued&sourceId=<id>` directly
  (`backend-import-pipeline.md` FR-7's status filter, not a derived
  client-side subtraction — the earlier draft's "queued minus observed
  terminal/pending" formula is replaced by this direct query, which is
  both simpler and correct even across a page reload with no
  client-cached discovery count to subtract from), refreshed by
  `frontend-shell-and-routing.md`'s existing TanStack Query polling
  convention (a fixed interval, not a websocket — no real-time
  transport exists in this project, consistent with every other
  screen).

## Non-functional requirements

- **Performance** — candidate cards render cover images already
  present as bytes (FR-2), no additional network request per card
  unlike `frontend-discover-screen.md`'s real Open Library covers;
  `frontend-library-screens.md` FR-4's virtualization threshold (100
  items) is reused unchanged if a pending list ever grows that large.
- **Security** — `coverBytes` rendered as a data URL is
  `backend-file-extractors.md`-extracted and validated content (an
  actual image, sniffed at extraction time), not arbitrary source-
  controlled bytes rendered blind; still rendered via a standard
  `<img src="data:...">`, React's own escaping applies to every other
  text field as usual.
- **Accessibility** — `frontend-accessibility.md`'s existing keyboard
  map and screen-reader conventions apply: each match candidate's
  confidence is announced as text (FR-2), not conveyed by color alone;
  confirm/reject actions are reachable and labelled per-candidate, not
  ambiguous when multiple cards are on screen; the in-flight progress
  count (FR-6) is announced via a live region on update, matching
  `frontend-discover-screen.md`'s existing degraded/loading-state
  announcement pattern.
- **Reliability** — a failed confirm/reject mutation (network error,
  `409` from a race with another confirmation) shows an inline error
  on that specific card, never a full-screen failure — the rest of the
  pending list remains usable.
- **Observability** — no new client-side logging beyond
  `frontend-shell-and-routing.md` FR-7's existing correlation-ID slot,
  reused unchanged.

## Domain model

No new domain types on the frontend. This screen renders
`backend-import-pipeline.md`'s wire shapes (`ImportCandidate`,
including its embedded `extractedMetadata`/`matchCandidates`)
directly.

## API and contracts

Consumes `backend-import-pipeline.md`'s full `/api/v1/import*` surface.
No new backend surface of its own.

## State transitions

Mirrors `backend-import-pipeline.md`'s own candidate state machine
exactly (Context, that spec) — this screen adds no client-side state
beyond FR-5's local dismissal tracking and FR-6's derived in-flight
count, neither of which is server-authoritative state.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Discovery (`POST /api/v1/import/discover`) fails | Non-2xx response | An inline error on the source browse view (FR-1), not a navigation to a broken `/import` screen | No navigation occurs; user can retry the action |
| Confirm/reject mutation fails | Non-2xx response, including `409` (a race — someone/something else already resolved this candidate) | Inline error on that specific candidate's card; a `409` specifically re-fetches the list (the card likely needs to disappear, since it's no longer `pending`) | Other cards remain interactive |
| A candidate's `matchCandidates` is empty (no existing-library or Open Library hits at all) | Empty array in the response | Only the "add from extracted info" action is shown, no "Use this match" options | Not an error state — a legitimate, expected case for an obscure book |
| Cover data URL fails to decode (corrupted `coverBytes`, in principle already validated at extraction but defended here too) | `<img>` `onError` | Falls back to `frontend-generated-covers.md`'s procedural cover, same as no cover at all | No error surfaced to the user for this specific case |

## Security considerations

- **No new trust boundary** — this screen talks only to this system's
  own `/api/v1/import*` endpoints; every hostile-content defence
  (extraction, matching) already happened server-side before this
  screen ever sees a candidate.
- **Rendered content is already validated** — `extracted_metadata`
  fields passed `backend-file-extractors.md`'s field-level validation
  before storage; `coverBytes` was content-sniffed as a real image at
  extraction time, not blindly trusted from a source.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Confidence-label rendering per `matchCandidates` entry; the three confirm-action button states per candidate shape (with/without existing-library match, with/without Open Library matches); `failed`-candidate distinct rendering; local dismissal persistence (FR-5) |
| Integration | Discover → candidates list → confirm/reject flow against MSW fixtures shaped like `backend-import-pipeline.md`'s real response types (tier-(a) contract-generated, `frontend-shell-and-routing.md`'s existing two-tier strategy); the in-flight count (FR-6) computed correctly from a `status=queued` fixture, including on a simulated fresh page load with no prior client-cached discovery response |
| Contract | Reuses `backend-import-pipeline.md`'s `kin-openapi` contract test — no separate frontend contract test |
| E2E | Add a source → import → an auto-imported file never appears as a pending card → a pending card's suggested match is confirmed → library reflects it; a deliberately-corrupted fixture appears in the `failed` section, dismissible |
| Accessibility | `frontend-accessibility.md`'s `axe`/keyboard-map requirements applied to candidate cards, confirm/reject actions, and the in-flight progress announcement |

Tests that must fail before implementation begins: a test asserting
confidence is rendered as visible text, not color alone; a test
asserting a `failed` candidate shows no confirm/reject actions; a test
asserting dismissing a `failed` candidate persists across a reload
(`localStorage`) without calling any backend endpoint.

## Acceptance criteria

- [ ] Triggering import from a source's browse view discovers and
      navigates to the candidates screen
- [ ] Pending candidates show extracted metadata and suggested matches
      with visible confidence labels
- [ ] All three confirm actions and reject work correctly against the
      real backend
- [ ] `failed` candidates are shown distinctly, with no confirm/reject
      actions, dismissible client-side
- [ ] In-flight progress is visible and updates without a page reload
- [ ] Accessibility checks pass for candidate cards and every action

## Open questions

- **No design-reference screen for import confirmation** — same gap
  named for Discover (phase 07) and source management (phase 08); this
  spec's card-list layout is a reasoned default, not derived from a
  capture.
- **Polling interval for in-flight progress (FR-6)** — no specific
  number fixed here; `frontend-shell-and-routing.md`'s existing
  convention is reused, but the actual interval value is an
  implementation detail this spec doesn't pin down, consistent with
  how other polling in this project has been left to that shared
  convention rather than re-specified per screen.

## References

- `backend-import-pipeline.md` (phase 10, sibling) — `/api/v1/import*`
  shapes this screen renders
- `frontend-source-management.md` (phase 08) — the source browse view
  this spec's FR-1 extends; its own Non-goals named this exact gap
- `frontend-discover-screen.md` (phase 07) — unmatched-candidate
  rendering precedent, degraded-state announcement pattern
- `frontend-collections-screens.md` (phase 06) FR-4 — non-optimistic
  mutation/cache-invalidation pattern reused (FR-4)
- `frontend-library-screens.md` (phase 06) FR-1, FR-2, FR-4 —
  `localStorage` justification precedent, URL-as-state precedent,
  virtualization threshold reused
- `frontend-generated-covers.md` (phase 04) — procedural cover fallback
- `frontend-accessibility.md` (phase 04) — keyboard/screen-reader
  requirements, including not conveying information by color alone
- Constitution §11 (interface copy)
