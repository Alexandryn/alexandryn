# Review: domain-events.md

| | |
|---|---|
| **Subject** | `.claude/specs/domain-events.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (both findings fixed — see Resolution below) |

## Summary

FR-1/FR-3's structural enforcement (a compile-time-fixed `Sensitive()`
method, a consumer-registration barrier that refuses `Sensitive` events)
is the right shape for making constitution §8 a type-system guarantee
instead of a convention. One real inconsistency found: FR-2 argues
`LibraryEntryAdded`/`LibraryEntryRemoved` must be `Sensitive` because "a
list of what someone owns is still revealing" (a deliberately protective
reading of constitution §8) — but the same spec classifies
`CollectionMemberAdded`/`CollectionMemberRemoved` as non-sensitive without
applying that same reasoning, despite a "want to read" collection
revealing the same kind of interest a library entry does.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Inconsistent classification | FR-2's stated reasoning for classifying `LibraryEntry` events `Sensitive` ("a list of what someone owns is still revealing," constitution §8's protective reading) applies equally to `Collection` membership — a curated "want to read" list reveals book-level interest just as directly. The spec classifies one `Sensitive` and the other not, with no stated justification for the difference | Classify `CollectionMemberAdded`/`CollectionMemberRemoved` as `Sensitive` too, consistent with the reasoning already used for `LibraryEntry`. `CollectionCreated` (no book reference) stays non-sensitive — it reveals nothing about specific interest |
| 2 | Nit | Completeness | `WorkMergeUndone` is listed in Domain model's non-sensitive events but doesn't appear in `domain-bibliographic.md`'s FRs by that exact name (FR-4 there requires merges be reversible but doesn't name the undo event) | Confirm the event name matches what `domain-bibliographic.md` actually specifies, or note it as a name this spec is introducing on that spec's behalf |

## Dimensions checked

- [x] **Completeness** — finding 1 is a real classification gap, not just a naming issue
- [x] **Ambiguity** — FR-1 through FR-5 individually clear
- [x] **Architecture** — correctly keeps transport/bus mechanism out (Non-goals), correctly enumerates events from all four sibling specs rather than inventing new domain concepts
- [x] **Domain correctness** — this spec's whole job is a classification correctness question; finding 1 is exactly that
- [x] **Security** — the FR-3/FR-4 split (system logs barred from `Sensitive` events, first-party Activity feed permitted) is a real, defensible reading of constitution §8 rather than a blanket "never" that would make an Activity feature impossible
- [ ] **Accessibility** — not applicable
- [x] **Testability** — FR-5's honesty about not being type-system-enforceable (Failure modes table) is exactly right — it says what it can't guarantee instead of overclaiming
- [x] **Maintainability** — FR-1's compile-time requirement is what prevents this spec from needing a "did you remember to update the sensitivity list" process step later
- [x] **Evolution** — Open questions correctly flags that new sibling-domain events need deliberate classification, not automatic inheritance of some default

## Contradictions and gaps

Finding 1 is the substantive one — and notably, this document explicitly
argued for the protective reading of constitution §8 in one place (FR-2's
`LibraryEntry` reasoning) and didn't carry that same argument through to
a structurally identical case a few lines later. The lesson from
`domain-library.md`'s review (0017) and `domain-source.md`'s review
(0018) generalizes again here: check whether a stated principle was
actually applied everywhere it should have been, not just where it was
first stated.

## Resolution (2026-08-14)

- **#1** — `CollectionMemberAdded`/`CollectionMemberRemoved` reclassified
  `Sensitive`, consistent with the reasoning already applied to
  `LibraryEntry` events. `CollectionCreated` stays non-sensitive (no book
  referenced)
- **#2** — `WorkMergeUndone` now noted explicitly as named on
  `domain-bibliographic.md`'s behalf, flagged for confirmation rather
  than presented as if already specified there

## What I did not review

Whether `SourceOfferingObserved` should be `Sensitive` — it reveals what
a *source* has available, not what a specific user is reading or owns, so
non-sensitive seems right, but it wasn't stress-tested against a scenario
where source configuration itself could be revealing (e.g., a source
named after a niche interest). Judged out of scope for this pass.
