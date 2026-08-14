# Review: domain-bibliographic.md

| | |
|---|---|
| **Subject** | `.claude/specs/domain-bibliographic.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (all four findings fixed — see Resolution below) |

## Summary

FR-1/FR-2/FR-3's identity strategy and FR-4's merge-not-compute dedup
approach are sound and directly answer phase 02's own open questions. Two
real modeling gaps found on self-review, both from phase 02's own named
hard cases: **omnibus editions are impossible to represent** under FR-2's
"exactly one Work" containment, and **language was modelled on the wrong
type** — a translation is a different-language Edition of the same Work,
which means Edition needs its own language field, not just Work's single
"primary language."

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Blocking | Modelling | FR-2 requires an `Edition` to belong to exactly one `Work`. An omnibus edition (one physical/file edition containing several distinct works, e.g. a trilogy bound as one volume) cannot be represented — phase 02's own risk table names omnibus editions explicitly as a hard case this spec must handle, and as written, it can't | Add a `Work`-to-`Work` "contains" relationship (an omnibus is its own `Work` with optional constituent-work references), keeping FR-2's clean one-`Work`-per-`Edition` containment intact rather than weakening it to many-to-many |
| 2 | Major | Modelling | `Work.language` is described as "primary language," but a translation is naturally a different-language `Edition` of the *same* `Work` — the language that actually varies is the edition's, not the work's. As written, there's nowhere to record that a Spanish and an English edition of the same novel are in different languages | Add `Edition.language` (the edition's actual language); keep `Work.language` but reinterpret it explicitly as "the work's original/first-written language" — a real, different, valid bibliographic concept, not the same field duplicated |
| 3 | Minor | Completeness | FR-4 defines recording a merge but not undoing one. Phase 10 (import matching) is where merges get created, plausibly automatically or semi-automatically — a wrong merge with no undo path is a real operational gap, not just a nice-to-have | Add a requirement that a recorded merge can be reversed (or add as an explicit open question with an owner, if genuinely out of this spec's scope) |
| 4 | Minor | Completeness | `Author` has no merge/dedup path, only `Work` does (FR-4). Duplicate author records are a common real-world Open Library data-quality issue (same person, differently-spelled name, or a pen name recorded separately) — the same reasoning that justified `Work`-level merging applies at the `Author` level and isn't addressed either way | Extend FR-4's merge pattern to `Author`, or explicitly non-goal it with a reason if author-level dedup genuinely doesn't matter at this project's scale |

## Dimensions checked

- [x] **Completeness** — findings 1 and 2 are real unstated-requirement gaps, directly against this phase's own named hard cases
- [x] **Ambiguity** — FR-1 through FR-8 individually clear
- [x] **Architecture** — correctly stays inside `internal/domain`'s zero-outward-dependency rule; no persistence/HTTP/vendor shape leaks in
- [x] **Domain correctness** — this spec *is* the domain correctness check for this slice; findings 1/2 are exactly the kind of error the phase's own risk table warned about
- [x] **Security** — FR-5/FR-6's construction-time validation is concrete and testable, not just asserted
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable, no user-facing text in a domain model
- [ ] **Observability** — correctly deferred to `domain-events.md`
- [x] **Testability** — every FR maps to a stated test approach; property-based testing correctly proposed for the invariants that are universal (no cycles, no orphans)
- [x] **Maintainability** — Non-goals section correctly keeps this spec from creeping into matching algorithms, persistence, or availability, all of which belong to sibling specs or later phases
- [x] **Evolution** — FR-4's merge-and-resolve-through design means a wrong dedup decision doesn't require a schema change to fix, just a corrected record (once finding 3 is addressed)

## Contradictions and gaps

Findings 1 and 2 are both cases where the spec's own stated goal
("the hard cases phase 02's risk table names explicitly") wasn't actually
satisfied by the model as drafted — the Test strategy section *named* these
cases as fixtures to test against, but the model underneath didn't yet
support representing them correctly. Writing the test list surfaced the
gap the model itself didn't.

## Resolution (2026-08-14)

All four findings fixed:

- **#1** — new FR-9: an omnibus is its own `Work` with optional "contains"
  references to constituent `Work`s, no cycles allowed. `Edition`'s
  one-`Work` containment (FR-2) stays exactly as strict as originally
  written
- **#2** — new FR-10: `Edition` gets its own `Language` field;
  `Work.language` reinterpreted as original/first-written language, not
  removed or duplicated
- **#3** — FR-4 now requires a recorded merge be reversible
- **#4** — FR-7 extended: `Author` gets the same merge pattern as `Work`

## What I did not review

Whether `Author` should support the same merge/dedup pattern as `Work`
(FR-4) — two records for the same real author (a common Open Library data
quality issue) seems likely to need the same treatment, but isn't
addressed either way in this draft.
