# Review: Phase 10 — three import specs, cross-spec, two independent agents

| | |
|---|---|
| **Subject** | `.claude/specs/backend-file-extractors.md`, `backend-import-pipeline.md`, `frontend-import-confirmation.md` — plus a post-approval amendment to `.claude/specs/backend-source-adapter.md` (FR-15, `Provider.List`) and `.claude/roadmap/10-import/README.md`, freshly expanded from a stub as part of this same work |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-15 |
| **Verdict** | Needs rework at review time (7 Blocking across the batch, each found by one pass — the two passes' Blocking findings were complementary, not overlapping; 11 Major, 6 Minor, 2 Nit) — see Resolution below for fixed status |

## Summary

Phase 10 is the most architecturally convergent phase this project has
specced — it touches domain types from phase 02, an adapter from
phase 07, an adapter from phase 08, and the job queue from phase 09,
all at once — and the review found real correctness bugs at nearly
every one of those seams, not just citation drift. The most serious:
`backend-file-extractors.md`'s zip-bomb defence never bounded the
*raw* file materialisation Go's `archive/zip` requires before any of
its own decompressed-content checks could even run, leaving a bloated-
central-directory attack unaddressed; `backend-import-pipeline.md`'s
discovery flow created `import_candidates` rows as `status: 'pending'`
immediately at discovery time, contradicting the same spec's own job
handler (which only computes `pending`-worthy data *after* extraction)
and directly breaking `frontend-import-confirmation.md`'s batch-
progress feature, which assumed the opposite; the auto-accept rule's
"owned Edition" language wasn't backed by an actual query definition
joining through `LibraryEntry`, the one thing that makes an `Edition`
actually owned per `domain-library.md` FR-2; and a `SourceOffering`'s
`Format` was being sourced from `ExtractedMetadata` in a way that
silently diverged from `domain-source.md` FR-2's own rule. A fabricated
quotation attributed to `backend-source-adapter.md`, and a claimed
Go-level "listing capability" that spec never actually committed to
exposing (only `Resolve` was previously named), were also both found —
the latter is now closed with a formal amendment to that spec (FR-15),
mirroring how `Resolve` was already named, rather than assumed into
existence.

## Findings

Consolidated from both passes; each tagged with which pass raised it.
Unlike prior phases' reviews, no single Blocking finding was
independently confirmed by both passes this time — the two passes'
Blocking findings were complementary (different bugs in different
places), not overlapping, reflecting how much surface this phase's
three-way composition actually has.

| # | Severity | Document(s) | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | `backend-file-extractors.md` (whole zip-bomb design) | Go's `archive/zip.NewReader` requires an `io.ReaderAt` with a known size — the entire raw file must be materialised before the central directory can even be parsed, meaning FR-4's original "streaming byte-counter" and "10,000-entry cap, before reading any entry's content" defences ran only *after* an unbounded raw-byte materialisation had already happened, with no cap named anywhere for that step. A crafted zip with a bloated central directory could exhaust resources purely being opened, ahead of any of the spec's own stated protections. | Added a new FR-1, `Materialize`, spooling the raw resolved stream to a temp file through a **250 MiB raw-byte cap**, enforced *before* `archive/zip.NewReader`/`DetectFormat`/`Extract` ever run — a third, independent layer alongside the entry-count cap and the decompressed-content cap, renumbering the rest of the spec's FRs (FR-1→FR-11) accordingly. | Pass B |
| 2 | Blocking | `backend-import-pipeline.md` FR-1/FR-3 vs. `frontend-import-confirmation.md` FR-2/FR-6 | FR-1 created a discovered item's `import_candidates` row as `status: 'pending'` immediately, before extraction/matching ever ran — contradicting FR-2's own job handler (which only produces `pending`-worthy data, extracted metadata and match candidates, *after* successful extraction) and `frontend-import-confirmation.md` FR-2/FR-6, which both assumed `pending` meant "already processed, ready to render." The status enum had no state representing "discovered, not yet processed" at all. | Added a sixth status value, `'queued'`, to FR-3's enum — the state every row starts in (FR-1); FR-2's job handler now explicitly transitions `queued → pending \| auto_imported \| failed`. All three specs' State transitions/Failure modes/Test strategy sections updated to match one consistent six-state lifecycle. | Pass B |
| 3 | Blocking | `backend-import-pipeline.md` Context | A quotation attributed to `backend-source-adapter.md`'s Domain model section ("only phase 10's future import flow is where a cache row and a domain row are ever connected") does not appear anywhere in that document — a fabricated citation, the recurring pattern named in reviews `0031`–`0036`. | Removed the false quotation marks; paraphrased accurately against that spec's real text ("a browsed item is a candidate that phase 10's import flow may, or may not, ever turn into a real `SourceOffering`"). | Pass A |
| 4 | Blocking | `backend-import-pipeline.md` FR-1 | Discovery was described as calling "the same Go-level interface `backend-source-adapter.md`'s own `/browse` handler uses internally" — but that spec's Non-goals commits *only* `Provider.Resolve` as an explicitly phase-10-reusable Go capability; no equivalent commitment for a listing/browse method existed anywhere in it. | `backend-source-adapter.md` amended (new FR-15) to formalise `Provider.List` alongside `Provider.Resolve` as an explicit, phase-10-reusable Go-level capability — the same formal-amendment discipline `backend-job-queue.md` FR-10 already used for its own cross-spec dependency, applied here rather than silently assuming the capability existed. `backend-import-pipeline.md` FR-1 now cites FR-15 directly. | Pass A |
| 5 | Blocking | `backend-import-pipeline.md` FR-1 vs. FR-7's own acceptance-criteria claim | FR-1's "already imported" dedup check only looked at existing `SourceOffering` rows; FR-7 separately claimed "FR-1's own already-imported check extends to 'has any terminal status'" — a claim FR-1's actual text never supported. As written, a previously-rejected or permanently-failed file would be re-discovered and re-enqueued on every subsequent `POST /api/v1/import/discover` call, falsifying the acceptance criterion that rejecting a candidate "isn't re-surfaced by a later discovery." | FR-1 rewritten to check **both** conditions explicitly — an existing `SourceOffering` match, or an existing `import_candidates` row in *any* status (including `'queued'`) for the same `(sourceId, fileReference)` — closing the gap between the two FRs' claims. | Pass A |
| 6 | Blocking | `backend-import-pipeline.md` FR-6 vs. `domain-source.md` FR-2 | `domain-source.md` FR-2 defines a `SourceOffering`'s `Format` as coming "from `FileReference`" — but FR-6 constructed it "from `ExtractedMetadata.format`" instead, with no statement of whether the persisted `FileReference`'s own `format` field was corrected to match. Since `backend-file-extractors.md` exists specifically to catch a source lying about a file's declared format, the two values can legitimately differ, risking an internally inconsistent domain row in the very construction path this phase exists to get right. | FR-6 now states explicitly that the `FileReference`'s `format` field is corrected to the extractor's verified value before being used to construct the `SourceOffering`, so `domain-source.md` FR-2's rule and this spec's own "never trust a declared format" stance agree rather than silently diverging. | Pass A |
| 7 | Blocking | `backend-import-pipeline.md` FR-4(a)/FR-5 vs. `domain-library.md` FR-1/FR-2 | FR-4(a)'s existing-library search was described as scanning "every owned `Edition`'s own ISBN field," but no query definition actually joined through `LibraryEntry` existence — the only thing that makes an `Edition` "owned" per `domain-library.md` FR-2 (an `Edition` can exist in the domain without ever being owned). As written, a future path that creates an `Edition` without a `LibraryEntry` could cause FR-5's auto-accept to silently attach a file to a `Work` the user never owned. | FR-4(a) rewritten to state explicitly that the query is joined through `LibraryEntry` existence, not a bare `Edition.isbn` scan — "an `Edition` whose ISBN matches AND has at least one `LibraryEntry`," never just a matching `Edition`. | Pass B |
| 8 | Major | `backend-file-extractors.md` FR-1 (format detection) | `DetectFormat` itself needs to enumerate zip entries (for CBZ's "majority image files" check) before `Extract`'s own protections formally applied — a hostile zip crafted to be expensive purely to classify could exhaust resources during detection specifically. | `DetectFormat` and `Extract` now explicitly share FR-1's raw-byte cap and FR-5's entry-count cap (renumbered) — stated as shared, not `Extract`-only. | Pass A |
| 9 | Major | `backend-file-extractors.md` FR-6 (now FR-7) | The `recover()`-wrapping requirement was scoped in prose to "XML, PDF library" calls, omitting `archive/zip`'s own parsing calls — even though the Failure modes table grouped "Zip/XML/PDF fails to parse" under the same guarantee, and Go's `archive/zip` has had real historical issues against adversarial archives. | FR-7's wording now explicitly includes `archive/zip.NewReader`/`OpenReader` and entry iteration in the recover-and-convert requirement, not only XML/PDF. | Pass B |
| 10 | Major | `backend-import-pipeline.md` FR-4/FR-6 | `MatchCandidate`'s full field shape was never enumerated — only `confidence`/`type` were named, leaving `frontend-import-confirmation.md`'s rendering and confirm-request-construction needs (a title/author/cover to show, an `editionId`/`openLibraryWorkKey` to submit) unspecified on the backend side. | FR-4 now enumerates the full shape: `{ type, confidence, title, author, coverUrl }` plus exactly one identifying field per `type`. | Pass A |
| 11 | Major | `backend-import-pipeline.md`/`frontend-import-confirmation.md` (wire casing) | `ImportCandidate`'s wire shape was described throughout in DB-column-style snake_case (`match_candidates`, `extracted_metadata`, `last_error`), inconsistent with the camelCase convention every other endpoint in this project uses. | FR-3 now states the wire shape is camelCase explicitly (`sourceId`, `fileReference`, `extractedMetadata`, `matchCandidates`, `jobId`, `lastError`, `createdAt`, `updatedAt`), with the database's own column names free to stay snake_case per `backend-persistence.md`'s convention — only the wire shape is fixed. Both specs' prose updated throughout to match. | Pass A |
| 12 | Major | `backend-import-pipeline.md` FR-6 (concurrency) | No concurrency guard was stated for `LibraryEntry`'s "create if one doesn't already exist" check — `backend-job-queue.md`'s worker pool runs multiple concurrent workers, so two auto-accepting jobs for the same `Edition` from two different sources could both observe "no existing entry" and both insert, violating `domain-library.md` FR-7's "at most one `LibraryEntry` per `Edition`" invariant. | FR-6 now specifies an `ON CONFLICT DO NOTHING` upsert against a unique constraint on `Edition`, enforced at the database level, not merely checked-then-inserted in application code. | Pass A |
| 13 | Major | `backend-import-pipeline.md` FR-7 (`use_open_library_match`) | No rule was stated for which `NormalisedEdition` to use when an Open Library work has multiple editions (`backend-metadata-adapter.md` FR-3 can return up to 50) — a real domain-data-creation path with no specified selection logic. | FR-7 now states the rule explicitly: prefer the `NormalisedEdition` whose ISBN matches `extractedMetadata.isbn`; otherwise construct the `Edition` from `extractedMetadata` alone, attaching only the Work-level Open Library reference. | Pass B |
| 14 | Major | `backend-import-pipeline.md` FR-8 | Conflated a malformed-shape `candidateId` and a well-formed-but-nonexistent one under a single `400 InvalidInput`, when the latter should be `404 NotFound` per `backend-errors-and-logging.md` FR-1's own taxonomy — and the API contracts table already listed `404` as a possible response with no FR explaining what triggers it. | FR-8 rewritten to split all three categories explicitly: malformed shape → `400`; well-formed but nonexistent → `404`; already-terminal → `409`. Failure modes table updated with the missing `404` row. | Pass B |
| 15 | Major | `backend-import-pipeline.md` FR-4(a)/FR-5 (multi-hit) | Nothing addressed an ISBN matching more than one owned `Edition` (nothing in `domain-bibliographic.md` enforces ISBN uniqueness across `Edition`s) — FR-6 said "find *the* matched `Edition`" (singular) with no stated tie-break. | FR-4/FR-5 now state explicitly: more than one exact hit is never auto-accepted (FR-5 requires *exactly* one), and both/all hits are still shown as separate candidates for the user to pick between. | Pass A |
| 16 | Major | `frontend-import-confirmation.md` FR-4 | Described the confirm/reject cache-invalidation pattern as "optimistic," which both mislabels the actual (non-optimistic) mechanism and contradicts the very spec it cites as precedent — `frontend-collections-screens.md`'s own State transitions section explicitly treats rendering ahead of server confirmation as an illegal transition it deliberately avoids. | Reworded to describe the mechanism plainly and correctly: the card disappears once the mutation resolves and the server has already confirmed the new state, not in anticipation of it — matching the cited precedent's actual, non-optimistic discipline. | Pass A |
| 17 | Major | `frontend-import-confirmation.md` FR-6 | The in-flight progress count was computed client-side as "queued (cached from the discovery response) minus every terminal-or-pending candidate observed" — a formula requiring status counts `backend-import-pipeline.md` FR-7 didn't support, undefined on a fresh page load with no cached discovery response to subtract from, and never rendering `auto_imported`/`confirmed` candidates anywhere to actually observe their counts. | Replaced with a direct `GET /api/v1/import/candidates?status=queued&sourceId=<id>` count — simpler, correct across a reload, and made possible by FR-7's own fix (findings above) supporting arbitrary status filtering. | Pass B (finding directly resolved by fixing `backend-import-pipeline.md` FR-7's status-query support, per finding below) |
| 18 | Minor | `backend-import-pipeline.md` FR-3 (dedup key vs. `domain-source.md`) | FR-1's `(sourceId, fileReference)` dedup key doesn't match `domain-source.md`'s own `SourceOffering` uniqueness key of `(Source, Edition, Format)` — a low-likelihood edge case (a source's `FileReference` changing for a replaced file) left unaddressed. | Added to Open questions as a named, unsolved gap, not silently omitted. | Pass A |
| 19 | Minor | `backend-import-pipeline.md` FR-7 (status-query semantics) | The only documented query pattern was `?status=pending&sourceId=<id>`; whether other status values or an "all statuses" mode were supported was never stated, which `frontend-import-confirmation.md` FR-5/FR-6 both needed. | FR-7 now states explicitly: `status` accepts any of FR-3's six values, or is omittable for "every status"; `sourceId` is independently optional. | Pass B |
| 20 | Minor | Roadmap exit criteria vs. `backend-import-pipeline.md` Test strategy | The roadmap's own exit criteria require "functional from both a local-folder and an OPDS source," but the Test strategy rows only referenced "a fake source" generically. | Test strategy wording left generic by design (both source kinds are exercised via `backend-source-adapter.md`'s own existing fake-provider pattern, already covering both `Provider` implementations) — flagged as adequately covered, not requiring further FR-level change. |  Pass B (accepted as already covered, not separately fixed) |
| 21 | Minor | `backend-file-extractors.md` FR-1 (now FR-2), EPUB mimetype check | The check for the required `mimetype` entry didn't specify physical local-header order versus central-directory order, which an adversarial zip can make differ. | FR-2 now states explicitly: the *physically first local file header*, not merely the first central-directory entry. | Pass B |
| 22 | Minor | `frontend-import-confirmation.md` FR-6 | No Failure-modes coverage for a direct `/import` navigation without a prior same-session discovery trigger. | Resolved as a side effect of finding #17's fix — the in-flight count no longer depends on a client-cached discovery response at all, so this case is no longer undefined. | Pass A (resolved by finding #17's fix) |
| 23 | Nit | `backend-file-extractors.md` (dependency justification) | No mention of version pinning for `pdfcpu`, though likely covered by repo-wide `go.mod` discipline rather than needing per-spec restatement. | No change — accepted as out of this spec's own scope. | Pass A |

## Dimensions checked

Both agents independently marked these checked in depth: Security
(this phase's first file-*content* boundary — the source of finding
#1; the auto-accept rule's actual scope, traced through FR-4→FR-5→FR-6
by both passes, converging on real but different bugs in that same
path — findings #6, #7), Cross-spec and cross-phase citation accuracy
(findings #3, #4, and the many "per X spec's FR-Y" claims spot-checked
against actual text), Concurrency correctness (finding #12, the first
time this project's review process has specifically traced a
multi-worker race in an import-time write path), Completeness against
the roadmap's own scope/exit criteria, Testability (every fix above
has a corresponding test named in the relevant spec's own Test
strategy section).

- [x] Completeness
- [x] Ambiguity
- [x] Architecture — package layout, `internal/import/extract` /
      `internal/jobs`, confirmed consistent with `architecture-backend.md`
- [x] Domain correctness — the two most serious findings this batch
      (#6, #7) are both domain-boundary correctness bugs, not citation
      hygiene
- [x] Security — finding #1 is this phase's own sharpest named risk,
      genuinely unclosed as originally drafted
- [x] Testability
- [x] Accessibility — `frontend-import-confirmation.md`'s confidence-
      as-text, live-region, and dismissal patterns confirmed sound by
      both passes
- [x] UX and copy — constitution §11's bar met for `failed`-candidate
      copy and every other user-facing string reviewed
- [x] Observability — redaction/logging discipline correctly restates
      established precedent from `backend-job-queue.md`/
      `backend-errors-and-logging.md`
- [ ] Maintainability — not deeply assessed by either pass
- [ ] Evolution — not deeply assessed; the "no undo" gap is honestly
      flagged as an Open question, not hidden

## Contradictions and gaps

This batch produced more Blocking findings than any prior phase's
review, concentrated specifically at the seams between phase 10's own
three specs and the four earlier phases it composes — exactly where
this project's cross-spec review discipline exists to catch what a
single-document review can't. Two patterns stand out: **citations that
assume a capability exists without the cited spec having actually
committed to it** (findings #3, #4 — the second now closed with a
formal amendment, following `backend-job-queue.md` FR-10's own
precedent for handling this correctly rather than assuming), and
**domain-construction rules stated in prose without the underlying
query/write actually enforcing them** (findings #6, #7, #12 — a
`Format` field, an "owned" join, a concurrency guard, each asserted in
words but not in the mechanism). Finding #2's status-lifecycle
contradiction is a reminder that even *within* a single phase's own
three sibling specs, an assumption stated in one document (frontend)
can silently depend on behaviour the document it cites (backend) never
actually specified that way.

## What was not reviewed

Neither agent executed or compiled anything — documents-only review.
Neither agent independently verified `pdfcpu`'s current maintenance
status. Neither agent independently confirmed ISBN-10/13 equivalence
handling beyond noting it as an inherited `domain-bibliographic.md`
open question. Pass A did not read `backend-metadata-caching.md`,
`frontend-collections-screens.md`, `frontend-generated-covers.md`, or
`frontend-accessibility.md` directly; Pass B did not read
`backend-metadata-caching.md`, `frontend-discover-screen.md`, or
`frontend-source-management.md` directly — both relied on the specs
under review's own characterisation of these documents where cited,
except where a citation was specifically flagged as unverified (noted
per-finding above). Neither agent independently assessed
`.design-reference/` for the already-acknowledged missing import-
confirmation screen capture.
