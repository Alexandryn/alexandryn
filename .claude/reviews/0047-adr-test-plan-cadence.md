# Review: ADR 0016 — Test plan cadence

| | |
|---|---|
| **Subject** | `.claude/decisions/0016-test-plan-cadence.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-17 |
| **Verdict** | Approved with changes (maintainer selected this option directly before drafting) |

## Summary

Records that test plans are written per-phase, at that phase's own RED
step, not per-spec immediately upon approval — ratifying phase 03's
actual historical pattern as the intended cadence rather than treating 37
specs without a test plan as a backlog. Chosen directly by the maintainer
from three presented options (defer-with-trigger, backfill, amend the
rule text) before this document was drafted, so the content decision
itself isn't this review's to second-guess — this review checks execution
against that choice.

## Findings

None. Drafted directly against the option already selected; no
alternative content was written and then corrected.

## Dimensions checked

- [x] **Completeness** — states the decision, the count that prompted it,
      why phase 03 is treated as the pattern rather than an exception, and
      the real enforcement gap this ADR alone doesn't close
- [x] **Ambiguity** — a reader can now tell, without re-deriving it, that
      "before the implementation" in `test-plans/README.md` means
      per-phase timing, not per-spec-approval timing
- [ ] **Architecture** — not applicable
- [ ] **Domain correctness** — not applicable
- [ ] **Security** — not applicable
- [x] **Testability** — this ADR is itself about test-plan process; its
      own "Bad" consequence honestly states it isn't mechanically
      enforced, rather than implying it is
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [ ] **Observability** — not applicable
- [x] **Maintainability** — the one-line pointer added to
      `test-plans/README.md` means a future reader lands on this ADR
      instead of re-deriving the same ambiguity
- [x] **Evolution** — names the exit-criteria-checklist pairing as a real,
      not-yet-done follow-up rather than silently assuming it

## Contradictions and gaps

None found against the constitution, `roadmap/README.md`'s own
"detail decreases with distance" reasoning (which this ADR's Option A
explicitly extends by analogy), or any other ADR.

## What I did not review

Whether to actually add the exit-criteria checklist item this ADR names
as a gap — explicitly out of scope for this ADR and this review, named as
a follow-up only.
