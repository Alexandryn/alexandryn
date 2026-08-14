# Review: ADR 0006 — Documentation and website live in separate repos

| | |
|---|---|
| **Subject** | `.claude/decisions/0006-docs-and-website-repos.md` |
| **Reviewer** | Luann Moreira (pending confirmation — drafted by Claude as a first pass, not self-approved) |
| **Date** | 2026-08-13 |
| **Verdict** | Pending |

## Summary

Same conflict-of-interest caveat as the other backfilled reviews. Lower
technical risk than 0004/0005 — this is a repo-topology decision, not a
runtime one, and it directly implements what you asked for rather than
resolving an open architectural question under uncertainty. Main thing worth
your eyes: I read "openapi" as the OpenAPI specification format (industry
standard term), not literally checked against you — flagging the inference
rather than treating it as certain.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Minor | Ambiguity | "an openapi" was read as the OpenAPI spec format. High-confidence reading, but not confirmed word-for-word | None required unless wrong — say so if it is |
| 2 | Nit | Scope | `architecture-contracts.md` (phase 01) still owns the real API design (endpoints, versioning, who owns the schema) — this ADR only fixes the format, not the design. Spec table and open-questions entry both say this already, but worth restating here so the ADR isn't read as having settled more than it did | None required, already stated correctly in the ADR and roadmap |

## Dimensions checked

- [x] **Completeness** — context, decision, three options, consequences, reversal, confidence
- [x] **Ambiguity** — finding 1 is the one soft spot
- [x] **Architecture** — consistent with `.claude/README.md`'s own reasoning for why `.claude/` stays versioned with the code (ADR 0001); doesn't touch runtime architecture at all
- [ ] **Domain correctness** — not applicable
- [ ] **Security** — not applicable; no trust boundary here
- [ ] **Testability** — not applicable; this is a repo/process decision, not behavior
- [ ] **Accessibility** — not applicable
- [x] **UX and copy** — n/a to review, but the roadmap/phase-99 edits use plain language consistent with the rest of the project
- [ ] **Observability** — not applicable
- [x] **Maintainability** — explicit that `docs`/`website` content design is deferred to phase 99, not decided now — avoids the "confident fiction" failure mode the roadmap already names elsewhere
- [x] **Evolution** — reversal cost section is honest that history/redirects get harder to unwind the longer this stands unreversed

## Contradictions and gaps

None found against the constitution, other ADRs, or the roadmap. Cross-checked
against ADR 0001 (why `.claude/` stays in-repo) and phase 01's open-questions
list (API contract ownership) — both consistent with this ADR's claims.

## What I did not review

Whether `docs` and `website` should be public/private at creation, who owns
them under the `Alexandryn` GitHub org, or their own CODEOWNERS — correctly
out of this ADR's scope, deferred to phase 99, not reviewed because there's
nothing yet to review.
