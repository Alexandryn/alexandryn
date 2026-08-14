# Review: domain-source.md

| | |
|---|---|
| **Subject** | `.claude/specs/domain-source.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (both findings fixed — see Resolution below) |

## Summary

FR-2/FR-3 (availability as a timestamped observation, never a fact) and
FR-4 (`FileReference` as a constrained type, not a path) directly close
the two risks phase 02's own roadmap names for this spec. One real gap:
`SourceOffering` never states a uniqueness constraint, and the State
Transitions section's "re-observed... updates the same row, not a new
one" directly contradicts the Open Questions note acknowledging a source
can offer the same `Edition` in multiple formats (which needs *multiple*
rows, not one) — same pattern as `domain-library.md`'s FR-6
self-contradiction: a later section noticing the right answer without the
earlier one being fixed to match.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Internal contradiction / missing invariant | No uniqueness constraint stated for `SourceOffering`. State Transitions says re-observation "updates the same row, not a new one" (implying uniqueness by `Source`+`Edition` alone), but Open Questions separately acknowledges a source offering two formats of the same `Edition` needs two `SourceOffering` rows — which requires `Format` (part of `FileReference`) in the uniqueness key, not just `Source`+`Edition`. As written, the two sections disagree on what identifies a `SourceOffering` | State the uniqueness key explicitly: `Source` + `Edition` + `Format`. Re-observing the same format updates that row; a new format is a new row. Fix State Transitions' wording to match |
| 2 | Minor | Completeness | `Source` has no "kind"/category field, even non-protocol-specific (e.g. for a UI to show a generic icon) — plausibly out of scope, but not stated as a deliberate exclusion, just absent | Either add a coarse, protocol-agnostic category (not full protocol detail) or explicitly non-goal it with reasoning |

## Dimensions checked

- [x] **Completeness** — finding 1 is a real, load-bearing gap; finding 2 minor
- [x] **Ambiguity** — FR-1 through FR-6 individually clear; the contradiction in finding 1 is between sections, not within a single FR
- [x] **Architecture** — correctly keeps protocol specifics out (Non-goals), correctly references `Edition`/`LibraryEntry` by ID only
- [x] **Domain correctness** — finding 1 is exactly this: an entity's identity wasn't actually pinned down
- [x] **Security** — `FileReference`'s path-exclusion (FR-4) is concrete and testable; observation-not-fact (FR-2/FR-3) correctly framed as a security property, not just a freshness one
- [ ] **Accessibility** — not applicable
- [x] **Testability** — every FR maps to a stated test; the property-based test for FR-5 is well-targeted
- [x] **Maintainability** — Non-goals correctly keeps this spec from creeping into phase 08's protocol territory
- [x] **Evolution** — Open questions honestly separates "real gap, no owner" (re-check policy) from "known limitation, may need revisiting" (one format per offering) — though finding 1 shows the "known limitation" framing undersold how unresolved it actually was

## Contradictions and gaps

Finding 1 is the substantive one, and it's the same category of mistake
`domain-library.md`'s review (0017, finding 1) caught: a later section of
the same document states the correct behavior without the earlier
requirement being reconciled against it. Worth treating as a checklist
item for the remaining domain specs, not just fixing per instance: before
calling a spec done, check whether any two sections describe the same
entity's identity or invariants differently.

## Resolution (2026-08-14)

- **#1** — FR-2 now states the uniqueness key explicitly (`Source` +
  `Edition` + `Format`); State Transitions reworded to match; Domain
  model and Open Questions updated for consistency
- **#2** — new optional `Kind` tag on `Source` (FR-1), explicitly a
  display category, not protocol configuration

## What I did not review

Whether `FileReference`'s "opaque, Source-scoped identifier" is
sufficient for phase 08 to actually resolve it back to bytes, or whether
it needs more structure (a source-defined type/discriminator beyond just
format) — reasonable as a minimal contract, not stress-tested against a
real OPDS response shape since that's explicitly phase 08's job, not
this spec's.
