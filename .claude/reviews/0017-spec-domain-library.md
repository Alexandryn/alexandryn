# Review: domain-library.md

| | |
|---|---|
| **Subject** | `.claude/specs/domain-library.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (all four findings fixed — see Resolution below) |

## Summary

FR-2 (computed, not stored, "in library") and FR-5 (the README/roadmap
"local and unsynchronised" reconciliation) are the strongest parts of this
spec — a real apparent contradiction between two existing documents,
resolved with a precise distinction rather than picked one side. One
self-contradiction found: FR-6 talks about cascade-removing "the entry"
from a `Collection`, but FR-4 already established `Collection`s hold
`Work`s, not `LibraryEntry`s or `Edition`s — there's no entry-to-collection
link to cascade-remove, and the spec's own State Transitions section
already says the opposite (a `Work` can stay in a `Collection` with zero
`LibraryEntry`s). The Failure modes table half-noticed this with a hedge
("if entries were ever linked to collections directly") instead of fixing
the actual requirement.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Internal contradiction | FR-6 requires cascade-removing a `LibraryEntry` "from any `Collection` it was part of" — but FR-4 defines `Collection` membership at the `Work` level, not the `LibraryEntry`/`Edition` level, so there is no such link to remove. The State Transitions section already states the correct behavior (a `Work` can remain in a `Collection` with no `LibraryEntry`s) without reconciling it against FR-6's contradictory wording | Reword FR-6: removing a `LibraryEntry` MUST NOT touch `Collection` membership at all — that's a `Work`-level fact, entirely independent of whether any `Edition` of that `Work` is currently owned |
| 2 | Minor | Missing invariant | No stated limit on how many `LibraryEntry` records may reference the same `Edition` — nothing prevents (or defines the semantics of) adding the same `Edition` to the library twice | Add: at most one `LibraryEntry` per `Edition` — a second "add to library" for an already-owned `Edition` is a no-op or an explicit error, not a second row |
| 3 | Nit | Completeness | Whether an empty `Collection` (zero `Work`s) is legal isn't stated explicitly, though nothing suggests otherwise | State it explicitly — a newly-created "Want to Read" shelf with nothing in it yet is an obvious, common case worth not leaving implicit |
| 4 | Nit | Completeness | `LibraryEntry` has an added-at timestamp; `Collection` membership has no equivalent, though "sort this shelf by recently added" is an obviously useful, low-cost thing to support | Add an added-at timestamp per `Work`-in-`Collection` membership, same pattern as `LibraryEntry` |

## Dimensions checked

- [x] **Completeness** — findings 2 and 3 are real gaps
- [x] **Ambiguity** — FR-1 through FR-5 individually clear; FR-6 is exactly where ambiguity became contradiction
- [x] **Architecture** — correctly keeps `domain-source.md`'s availability out of this spec (FR-3), correctly references `Work`/`Edition` by ID only, never embeds
- [x] **Domain correctness** — finding 1 is precisely a domain-correctness defect: an invariant stated in one FR contradicted by the spec's own later section
- [x] **Security** — collection names correctly treated as hostile input, not exempted as "the user's own data"
- [ ] **Accessibility** — not applicable
- [x] **Testability** — every FR maps to a stated test approach; the property-based test for FR-2 is well-targeted (an invariant that should hold for *any* sequence of operations, not just hand-picked ones)
- [x] **Maintainability** — the README/roadmap reconciliation (FR-5) is exactly the kind of thing that prevents a future contributor from "fixing" what looks like a bug but isn't
- [x] **Evolution** — Open questions correctly flags that FR-5's "one copy" model isn't proven compatible with a future per-account collections feature, rather than silently assuming it is

## Contradictions and gaps

Finding 1 is the substantive one, and notably: I wrote the correct
behavior in State Transitions *and* the contradictory requirement in FR-6
in the same draft, without cross-checking them against each other before
calling the spec done. Worth naming as a general lesson, not just fixing
the one instance: a later section restating something more carefully is a
signal to go back and fix the earlier FR, not to let both stand.

## Resolution (2026-08-14)

All four findings fixed:

- **#1** — FR-6 reworded: removing a `LibraryEntry` never touches
  `Collection` membership at all, matching what State Transitions already
  said correctly
- **#2** — new FR-7: at most one `LibraryEntry` per `Edition`
- **#3** — FR-7 also states an empty `Collection` is legal
- **#4** — `Collection` membership now carries its own added-at timestamp,
  independent of any `LibraryEntry`'s

## What I did not review

Whether `Collection` needs any per-entry metadata (e.g. "why is this here"
notes, or an added-at timestamp the way `LibraryEntry` has one) — not
addressed either way, and not obviously wrong to omit, but not
deliberately considered either.
