# Review: Real-world edge-case conformity pass over the phase 02/08/10 ingest and domain specs

| | |
|---|---|
| **Subject** | `domain-bibliographic.md`, `domain-source.md`, `domain-reading.md` (phase 02), `backend-source-adapter.md` (phase 08), `backend-file-extractors.md`, `backend-import-pipeline.md` (phase 10), and `.claude/roadmap/14-devices-and-sync/README.md` |
| **Reviewer** | Claude (Sonnet 5), single pass, maintainer-directed |
| **Date** | 2026-08-20 |
| **Verdict** | Approved with changes — six correctness/completeness gaps fixed, two genuine open questions recorded rather than solved; maintainer approved the scope of this pass 2026-08-20, per-spec amendments recorded in each status cell pending the maintainer's next read |

## Summary

This pass asks a narrower question than a structural review: given how
self-hosted library software behaves once it runs against real libraries —
real folder shapes, real malformed files, real matching failures, real
multi-device sync — where do these specs' first-principles assumptions not
yet cover a case that recurs in practice? None of the findings below
overturn a modelling decision; every one either sharpens an existing rule
with a concrete case it didn't yet name, or records an unresolved gap
honestly instead of leaving it implicit. Several areas checked came back
clean — `domain-bibliographic.md` FR-4/FR-7's refusal to compute identity
from string similarity, `backend-file-extractors.md`'s MOBI/AZW3 exclusion,
and `domain-bibliographic.md` FR-9's omnibus-as-structure-not-detection
boundary all hold up against exactly the failure patterns a comparable
system's real operating history would be expected to surface, and none of
the three needed a change.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | `backend-import-pipeline.md` FR-4 | Three distinct, recurring input shapes look unambiguous to a strict exact-match scorer and are not: a title differing from another only by a parenthetical qualifier that whitespace/case normalisation could strip as noise; two unrelated books sharing a series name and first-publication year but actually different volumes; a standalone book's title colliding with an unrelated series' name. FR-4's strict-match posture is the right defence in principle, but none of the three shapes were named anywhere the scoring rule or its test strategy could be held to | **Fixed** — FR-4 now states explicitly that none of the three shapes may score above `"medium"`; the test-strategy's must-fail-before-implementation list names all three as required fixtures |
| 2 | Major | `domain-source.md` FR-4, `backend-file-extractors.md` FR-3, `backend-import-pipeline.md` FR-1–FR-5 | The entire ingest chain is built around one file producing at most one `Edition` — one `FileReference`, one `Extract` call, one `import_candidates` row. Nothing in any of the three specs represents a source item that is legitimately one logical reading unit split across more than one file (a real, supported shape for at least one comic-adjacent format family). This is not a malformed-input case to degrade gracefully from; the model has no representation for it at all | **Open** — recorded as a cross-referenced Open question in all three specs, deliberately not solved here: whether this is out of scope (each file imports independently, a known-degraded outcome) or needs a real modelling extension is a design decision this pass declined to make unilaterally |
| 3 | Minor | `backend-file-extractors.md` FR-9, FR-10 | Zip/CBZ archive entry names are not guaranteed to decode as valid UTF-8 — legacy, pre-Unicode encodings from mainstream archive tooling reproduce this reliably, most visibly with non-Latin filenames. Neither FR-9's OPF-path resolution nor FR-10's page-order sort stated what happens when an entry name fails to decode; the practical failure mode without a stated rule is silently missing or misordered content, not a clean error | **Fixed** — FR-9 now treats an undecodable `<rootfile>` target as `ErrMalformed` (whole-file, since the OPF is structurally required); FR-10 now excludes an undecodable entry from the CBZ page list (per-item degradation, consistent with FR-2's sidecar-exclusion precedent); both added to the Failure modes table |
| 4 | Minor | `backend-file-extractors.md` FR-2, FR-10 | Archives produced by a common, ordinary macOS archiving workflow carry a `__MACOSX/` sidecar directory of small placeholder entries, one per real file, sorting adjacent to the real entries they shadow. FR-2's classification step only checked a majority-image-type threshold and FR-10's page sort had no rule excluding these non-content entries, so an unfiltered sort could place a corrupt placeholder as the first page a reader loads | **Fixed** — FR-2 now excludes `__MACOSX/*` and dotfile entries from both the classification count and FR-10's page enumeration, before either runs |
| 5 | Minor | `domain-bibliographic.md` FR-6, `backend-file-extractors.md` FR-8 | A field can be present but empty or whitespace-only — a real, recurring shape of upstream metadata (a summary or title that technically has a value with no actual content). Neither FR-6's construction-time validation nor FR-8's `ErrNoTitle` rule stated whether this counts as "present" or is rejected the same as an absent field, leaving the distinction to be discovered downstream | **Fixed** — FR-6 now rejects an empty or whitespace-only string identically to one exceeding the length bound; FR-8 now treats a title that fails this check as `ErrNoTitle`, the same as a genuinely absent title |
| 6 | Minor | `backend-source-adapter.md` FR-3 | Local-folder path validation named "absolute, existing, a directory, readable" as its validation dimensions but not maximum path length or UNC/network-share path shape — both are real, recurring failure shapes for exactly this kind of source (a Windows-plus-network-storage-capable deployment target), not theoretical | **Fixed** — FR-3 now names an overlong resolved path as a create/update-time shape violation, and states explicitly that a Windows UNC path (`\\server\share\...`) must be accepted as a valid absolute-path shape |
| 7 | FYI | `domain-reading.md` Open questions | The `PrecisePosition` format is correctly deferred to phase 11, but the open question didn't yet name what a concrete format needs to survive: the same addressed text or location occurring more than once within one rendered unit, and the renderer's own output for "the same" content shifting between reads without any edit to the book itself. Both are known, recurring failure modes for content-tree-based position schemes in mature reading software | **Fixed** — both properties added to the existing Open question; no scope change, phase 11 still owns the concrete format decision |
| 8 | FYI | `domain-reading.md` Open questions, `.claude/roadmap/14-devices-and-sync/README.md` | `ReconcileProgress`'s conflict-resolution math (FR-6) assumes the system and every device already agree on which file a progress report concerns. In practice this agreement can fail asymmetrically — one sync direction working, the other silently not, for what should be the same file — which is a different failure than anything FR-6 resolves, since reconciliation never runs until both sides already agree what they're reconciling | **Fixed** as a forward-looking record — a new Open question in `domain-reading.md` and a Risks entry in phase 14's outline; phase 14 doesn't exist as a full spec yet, so nothing here is a fix, only a flagged starting point for when it's scoped |

## Dimensions checked

- [x] **Completeness** — the focus of this pass; findings 1–6 are each a case the existing rule's own stated intent already implied but didn't name.
- [ ] **Ambiguity** — not the focus; no two-engineers-build-different-things case was found or looked for.
- [ ] **Architecture** — not in scope; no mechanism or layering changed.
- [x] **Domain correctness** — finding 2 is squarely this: the ingest chain's one-file-to-one-`Edition` assumption is a domain-shape question, recorded rather than silently assumed universal.
- [x] **Security** — finding 3's undecodable-entry-name case sits inside `backend-file-extractors.md`'s own "every file is hostile input" frame; naming a defined outcome closes an unstated-behaviour gap in a spec that otherwise treats every parse failure explicitly.
- [x] **Testability** — finding 1 specifically: three named fixtures were added to the must-fail-before-implementation list precisely because "the rule is strict" was previously untested against the shapes that actually matter.
- [ ] **Accessibility** — not applicable to any subject reviewed.
- [ ] **UX and copy** — out of scope for this pass by direction; no interface surface was reviewed.
- [x] **Observability** — finding 3's failure-mode table rows are the concrete mechanism that makes the new behaviour visible as a named category rather than a silent miscount.
- [x] **Maintainability** — every fix was written as an addition to an existing FR or Open question, not a new FR, to avoid renumbering churn across the seven files' own cross-references.
- [x] **Evolution** — finding 2 is the clearest instance: recording the gap as an Open question rather than inventing a mechanism keeps the decision available for whichever phase actually needs to make it, instead of foreclosing it with an unreviewed guess.
- [ ] **Mechanism properties** — not applicable; no fix in this pass changed how an existing mechanism works, only what cases it's now stated to cover.
- [x] **Indexes** — `.claude/specs/README.md`'s status column updated for all seven amended specs.

## Contradictions and gaps

None found between the subjects reviewed and the constitution, an ADR, or a
sibling spec. Finding 2 is the one genuine modelling gap this pass
surfaced; it is recorded, not resolved, because resolving it is a design
decision (whether the domain needs a multi-file-per-reading-unit concept
at all) outside this pass's mandate.

## What I did not review

- Every other spec in `.claude/specs/` not listed under Subject — this pass
  was scoped to ingest, format-handling, matching/identity, and reading
  progress, per the maintainer's direction, not a full re-review.
- Any code. None exists for the phases these specs describe.
- The design reference. This pass is backend/domain-only by direction; no
  UI surface was consulted or is affected by any finding here.
- `domain-reading.md` FR-6/FR-7's own already-flagged, already-scheduled
  amendment (furthest-wins-with-override's non-commutativity) — pre-existing,
  unrelated to this pass's findings, and left untouched here.
