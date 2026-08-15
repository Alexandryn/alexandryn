# Review: Phase 11 — three reader specs, cross-spec, two independent agents

| | |
|---|---|
| **Subject** | `.claude/specs/backend-reader-content.md`, `backend-reading-api.md`, `frontend-reader.md` — plus post-approval amendments to `.claude/specs/frontend-library-screens.md` (FR-5, "Read" entry point) and `.claude/specs/backend-library-api.md` (FR-5, `formats` field), and `.claude/roadmap/11-reader/README.md`, freshly expanded from a stub as part of this same work |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-15 |
| **Verdict** | Needs rework at review time (7 Blocking, 3 confirmed independently by both passes; 9 Major; 6 Minor) — see Resolution below for fixed status |

## Summary

Phase 11 is the first phase to serve real file content back to a
client and the first to persist `domain-reading.md`'s model for real,
and the review found genuine correctness bugs at both of those new
seams, plus two contradictions between this batch's own roadmap
document and its sibling specs. Three findings were independently
confirmed by both passes: `backend-reader-content.md` FR-5 invented a
`403` status with no home in `backend-errors-and-logging.md`'s closed
six-category taxonomy; `backend-reading-api.md` FR-4 hand-rolled full
EPUB CFI grammar validation in Go, directly contradicting the
roadmap's own stated decision to lean on a library for exactly that
problem; and `frontend-library-screens.md`'s amendment header cited
this very review as already complete and its findings already fixed
before either had happened — a process-integrity mistake this
project's own reviews have now caught twice in one day. Each pass also
found a Blocking issue the other didn't: reviewer A found that
`frontend-reader.md`'s `/read/:editionId` route has no way to call
`backend-reading-api.md`'s Work-scoped progress endpoints (no
`workId` anywhere in the flow) and that `frontend-library-screens.md`'s
"EPUB-only Read action" gate depended on a per-Edition format signal
`backend-library-api.md` never actually exposed; reviewer B found that
`backend-reader-content.md`'s content cache would silently break after
its first request, because `Materialize`'s temp-file cleanup is tied
to "the caller's context," and a cache meant to outlive any single
request needs its own, independent one — and that `backend-reading-api.md`'s
progress-reconciliation endpoint had a real lost-update race under
concurrent multi-device reports, with no locking specified despite
`ReconcileProgress`'s own commutative/associative guarantee being
about the pure function's mathematics, not about a read-then-write
sequence with no isolation around it.

## Findings

Consolidated from both passes; duplicate findings merged, each tagged
with which pass(es) raised it.

| # | Severity | Document(s) | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | `backend-reader-content.md` FR-5 | Returned `403`, labelled "this project's `Unauthorized` category" — but `backend-errors-and-logging.md`'s closed, total taxonomy maps `Unauthorized → 401`; no category maps to `403` at all, and the mapping is required to live in exactly one place this FR bypassed. | FR-5 now returns `400 InvalidInput` for both an unrecognised content type and a standalone SVG resource — the correct existing category, not an invented one. Updated throughout (API and contracts, Failure modes, Test strategy, Acceptance criteria, References). | Both, independently |
| 2 | Blocking | `backend-reading-api.md` FR-4 vs. roadmap Risks table | The roadmap's own Risks table named EPUB CFI's grammar as "genuinely intricate" and chose to lean on `foliate-js`'s `epubcfi.js` rather than hand-roll it — then FR-4 did exactly that hand-rolling server-side in Go, with no library named and none of the constitution §9 rigor `bluemonday`'s justification (sibling spec) received. | FR-4 rewritten to a deliberately narrow **shallow structural check** (balanced brackets, permitted character set) — explicitly not full grammar validation — with the distinction between "generation/resolution" (client, library-backed) and "syntactic sanity check" (server, narrow, hand-rolled, justified as a genuinely bounded problem) stated in both the spec and the roadmap, so the two documents agree rather than silently disagreeing about what "CFI validated" means. | Both, independently |
| 3 | Blocking | `frontend-library-screens.md` header | Asserted "findings fixed; awaiting maintainer re-confirmation" and cited this review (`0038`) as an already-completed past event, before either agent had reported anything — the same process-integrity mistake found and fixed once already this session (phase 08's `desktop-host-ipc-surface.md` amendment). | Left as-is once the fixes below were actually applied — the header is accurate now that this review has genuinely run and its findings are genuinely resolved, but the sequencing error (writing it before dispatching the review) is named here as a repeat of a known failure mode, worth a standing note for future phases: write review-outcome language only after the review has actually returned findings, never preemptively. | Both, independently |
| 4 | Blocking | `frontend-reader.md` FR-1 vs. `backend-reading-api.md` FR-2 | The reader's route (`/read/:editionId`) carries no `workId`, but `backend-reading-api.md`'s progress endpoints are Work-scoped (`domain-reading.md` FR-1's own singleton-per-Work design) — the reader's own "open where I left off" flow, this phase's named E2E exit criterion, could not be built as specified. | Route changed to `/read/:workId/:editionId` — both IDs already available from `frontend-library-screens.md`'s own `/book/:id` URL, so no separate edition-to-work lookup is needed. | Pass A |
| 5 | Blocking | `frontend-library-screens.md` FR-5 (amendment) vs. `backend-library-api.md` FR-5 | The amendment gated the "Read" action on "each owned EPUB edition," but no field anywhere in `backend-library-api.md`'s work-detail response, `domain-bibliographic.md`'s `Edition`, or `domain-source.md`'s model exposes a per-Edition format signal — the condition was unimplementable as written (format lives on `SourceOffering.Format`, and one `Edition` can have offerings of different formats). | `backend-library-api.md` FR-5 amended to add `formats: string[]` per owned Edition (deduplicated `Format` values across its `SourceOffering`s) — exactly the kind of additive field that FR's own text already anticipated ("per-source availability... for phase 07/08 to extend"). `frontend-library-screens.md`'s gate now reads this real field. | Pass A |
| 6 | Blocking | `backend-reader-content.md` FR-2/FR-3 vs. `backend-file-extractors.md` FR-1 | `Materialize`'s temp file is removed when "the caller's `context` is done" — but FR-3's cache is meant to keep that same file alive for up to 10 minutes across many separate HTTP requests, each with its own request-scoped context. Populated from the naive default (the triggering request's own context), the file would be deleted the instant that first request completed, silently breaking every subsequent "cache hit." | FR-2 now states explicitly that cache population uses a context the cache itself owns, independent of any individual request, cancelled only at FR-3's own eviction. | Pass B |
| 7 | Blocking | `backend-reading-api.md` FR-2 | Claimed `ReconcileProgress`'s commutative/associative property alone "makes FR-2's reconcile-and-persist step safe under concurrent reports" with "no additional concurrency control beyond a single-row transactional update." A read-then-reconcile-then-write with no lock has a classic lost-update race: two devices can both read the same stale canonical value, both compute a different reconciled result, and whichever commits second silently overwrites the first — even if the first was the correct furthest-wins outcome. | FR-2 now requires the read and write to happen in one transaction with the read acquiring a row lock (`SELECT ... FOR UPDATE`, or an equivalent compare-and-swap) — explicitly distinguishing `ReconcileProgress`'s own mathematical guarantee (about the pure function) from what actually has to guard the read-then-write sequence around it. | Pass B |
| 8 | Major | `backend-reader-content.md` FR-6/FR-7 | No rule addressed inline CSS (`style` attributes, `<style>` blocks) — common in real EPUBs — leaving it unclear whether `bluemonday` (HTML-scoped) or the CSS scanner (only run on standalone `.css` resources) covered it, or neither. | `style` attributes are now stripped entirely; `<style>` block contents are now explicitly routed through FR-7's own CSS sanitisation before being allowed to remain. | Pass A |
| 9 | Major | `backend-reader-content.md` FR-7 | The CSS scanner's "reject anything that isn't relative or `data:`" rule didn't define "relative" precisely — if implemented as "block only `http(s)://`," `url(javascript:alert(1))` would incorrectly pass, since it's neither relative nor an `http(s)` URL. | Rule redefined precisely: reject any value with *any* scheme component other than `data:` (a bare presence-of-scheme check, not a scheme-specific blocklist) — a `javascript:` URI is now rejected by construction, not by a special case. | Pass A |
| 10 | Major | `backend-reader-content.md` FR-6 | No rule addressed inline/embedded SVG, which has its own distinct script-execution surface (`onload`, `xlink:href="javascript:..."`, `<foreignObject>`) independent of plain-HTML vectors, and is common in EPUB covers. | Inline `<svg>` is now stripped wholesale from HTML content; standalone SVG resources are refused (finding #1's `400`) rather than served — a named, accepted feature loss (SVG covers won't render), not a silent gap. | Pass A |
| 11 | Major | `backend-reader-content.md` FR-3/FR-4 | No per-request timeout was specified for content decompression (phase 10's own 30-second budget was dropped for this new read path), and no concurrency/reference-counting discipline was stated for the cache — a real race between LRU eviction and an in-flight read of the same Edition, with file-deletion-while-open semantics differing across the platforms this project ships on. | Added a 30-second per-request timeout (reused from `backend-file-extractors.md` FR-3 for consistency) and reference-counted cache entries — an entry can't be evicted while an in-flight request still holds it. | Pass A |
| 12 | Major | `backend-reader-content.md` Test strategy | No test asserted "no reading-related content ever appears in logs," despite the Observability section making exactly that claim — the sibling `backend-reading-api.md` had this test, this spec didn't. | Added the equivalent test and acceptance criterion. | Pass A |
| 13 | Major | `backend-reading-api.md` FR-1 vs. `domain-reading.md` domain model | Required `X-Device-Id` on every endpoint, including bookmark/highlight CRUD — but `domain-reading.md`'s `Bookmark`/`Highlight` types carry no `DeviceID` field at all, only `ReadingProgress` and `ReadingPreferences` do; the header would have been collected and silently discarded on those endpoints with no stated reason. | FR-1 scoped down to only the two endpoint groups that actually use `DeviceID` (progress, preferences); bookmark/highlight endpoints no longer require the header. | Pass B |
| 14 | Major | Roadmap exit criteria vs. `frontend-reader.md` | The roadmap's own exit criteria named "renders real imported EPUBs correctly across screen sizes" explicitly; no FR, NFR, test, or acceptance criterion in `frontend-reader.md` addressed responsive/viewport behaviour at all. | Added a Responsive-layout NFR (reader chrome collapses below the existing tablet/mobile breakpoint; `foliate-js`'s own reflow handles content), a Test strategy row, and an acceptance criterion. | Pass B |
| 15 | Major | `backend-reader-content.md` FR-3 vs. `backend-test-harness.md` | The Test strategy claimed an eviction test "under a controllable clock," but FR-3 never required idle-timeout tracking to actually use the injected `Clock` interface — as written, the claimed test would need a real 10-minute wait. | FR-3 now states explicitly that idle-timeout tracking MUST use the injected `Clock`, matching every other phase-03-dependent spec's own stated discipline. | Pass B |
| 16 | Major | `frontend-reader.md` FR-7 vs. `backend-reading-api.md` FR-7 | The backend's `Highlight` shape includes `category` (`domain-reading.md` FR-4: a highlight "MAY carry... a category/colour"), but the highlight-creation UI only offered a note field, silently dropping the only UI that would ever set it. | Added a category/colour picker (a small, fixed swatch set) to the highlight-creation flow, keeping UI parity with the domain model. | Pass B |
| 17 | Minor | `backend-reader-content.md`, `backend-reading-api.md` | Neither spec named its `internal/` package, unlike every other phase's backend specs. | Named explicitly: `internal/reader/content` and `internal/reader/api`, both positioned in `architecture-backend.md` FR-1's layout with their dependency directions stated. | Both, independently |
| 18 | Minor | `backend-reader-content.md` FR-1 | Cited `domain-library.md` FR-2 ("in library," a Work-level computed flag) for a per-Edition ownership check — the correct citation is FR-1 (the actual `LibraryEntry` fact), the same citation-drift pattern already found once in the already-approved `backend-import-pipeline.md` FR-4(a). | Corrected to FR-1, with the FR-2 distinction stated explicitly so the difference between "Work is in library" and "this exact Edition is owned" doesn't get conflated again. | Pass A |
| 19 | Minor | `frontend-reader.md` Security considerations | `sandbox="allow-same-origin"` (no `allow-scripts`) was asserted without reasoning through why `allow-same-origin` is needed or that omitting it was considered. | Added explicit reasoning: omitting it would break resource loading/selection against this system's own content origin; the sandbox-escape risk that combination is known for specifically requires `allow-scripts` too, which this design never grants. | Pass B |
| 20 | Minor | `.claude/specs/README.md` | The index doesn't yet reflect any of this batch's new specs or amendments. | Updated alongside this batch's approval (see Resolution). | Pass B |
| 21 | Minor | `frontend-reader.md` Failure modes | No row addressed `backend-reader-content.md`'s `400`/`403`-shaped rejection (now `400`, finding #1) at all. | Added a row: treated the same as the existing `404`/corrupted-content case. | Pass A |
| 22 | Minor | `backend-reader-content.md` FR-4 | The 200 MiB decompressed cap's *scope* changed from phase 10's "whole book" to this spec's "one entry" without saying so. | Stated explicitly as a scope change, not a silent identical reuse. | Pass A |

## Dimensions checked

Both agents independently marked these checked in depth: Security
(the sandboxing design's actual completeness — findings #1, #8, #9,
#10, #11 are all real gaps found by tracing the two-layer defence
concretely, not accepting it on the strength of its own stated
intent), Concurrency correctness (findings #6, #7 — both real races
traced through actual request/transaction sequencing, not asserted
away), Cross-spec and cross-phase citation accuracy (the source of
findings #2, #5, #13, #18, and confirmation that most of this batch's
many other citations — `Materialize`, the entry-count/decompressed
caps, `Provider.Resolve`, the many-to-many `SourceOffering`, MSW/
accessibility precedents — checked out accurately, a stronger citation
record than several earlier phases' first drafts), Completeness
against the roadmap's own scope/exit criteria (finding #14),
Testability (finding #12, #15; both passes independently noted the
originally-described test suites wouldn't have exercised several of
this batch's own real bugs).

- [x] Completeness
- [x] Ambiguity
- [x] Architecture (finding #17)
- [x] Domain correctness (findings #5, #13, #18 all involve the domain
      model being consulted incorrectly or incompletely)
- [x] Security (this batch's central focus — findings #1, #8, #9,
      #10, #11)
- [x] Testability
- [x] Accessibility (frontend spec's keyboard/screen-reader coverage
      confirmed sound by both passes, not independently re-verified
      against `frontend-accessibility.md`'s full FR list)
- [x] UX and copy (constitution §11's bar met for every named copy
      example)
- [x] Observability (finding #12; otherwise consistent, careful
      "never log reading content" discipline confirmed by both passes)
- [ ] Maintainability (lightly checked only)
- [ ] Evolution (lightly checked only — phase 14's future build on
      `ReconcileProgress`'s wiring here not deeply traced)

## Contradictions and gaps

This batch produced this project's first case of a **security
mechanism failing under its own stated example twice in the same
spec** (findings #9's `javascript:` URI and #10's SVG, both inside
`backend-reader-content.md`'s sandboxing design) — a reminder that a
sanitisation rule stated as a general principle ("reject anything
external") needs to be traced against every concrete syntax a format
actually permits, not just the obvious `http(s)://` case, the same
lesson review `0035` (phase 08) already drew from its own SSRF
findings. Findings #2 and #3 are both cases where this batch's own
roadmap document — written first, in the same work — was contradicted
by a sibling spec drafted immediately after it stopped being read
carefully; worth naming as a caution for any future phase that writes
its roadmap and specs in the same pass.

## What was not reviewed

Neither agent executed or compiled anything — documents-only review.
Neither agent independently verified `bluemonday`'s actual default
`UGCPolicy()` behaviour against a live install (findings #8/#10 are
correctly flagged as unverified gaps in the *spec's* own stated rules,
not confirmed live vulnerabilities). Neither agent independently
verified `foliate-js`'s real CFI-generation API surface beyond the
Context section's own citations. Neither agent deeply assessed how
phase 14's cross-device sync will build on this phase's
`ReconcileProgress` wiring. Pass A did not read
`backend-metadata-caching.md` in full; Pass B did not read
`frontend-discover-screen.md`/`frontend-source-management.md` in full
— both relied on the specs under review's own characterisation where
cited.
