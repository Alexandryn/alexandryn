# Review: ADR 0001 — We record architecture decisions in this directory

| | |
|---|---|
| **Subject** | `.claude/decisions/0001-record-architecture-decisions.md` |
| **Reviewer** | Luann Moreira |
| **Date** | 2026-08-13 |
| **Verdict** | Approved |

## Summary

Backfilled review — this ADR was marked `Accepted` on 2026-08-12 with no
recorded review, which violates `reviews/README.md`'s own rule that every ADR
gets one before reaching `Accepted`. Content-wise it's sound: a well-known
practice (Nygard-style ADRs), the alternatives are real and fairly weighed,
and the failure mode it accepts (abandonment) is named honestly rather than
hidden.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Minor | Process | This ADR predates its own enforcement — it was the one that established the `.claude/decisions/` practice, but was itself accepted without the review that practice (via `reviews/README.md`) now requires of everything after it | None required; noting for the record. Chronology explains it, doesn't excuse skipping the backfill |

## Dimensions checked

- [x] **Completeness** — context, decision, four options, consequences, reversal cost, confidence all present
- [x] **Ambiguity** — decision is a clear directive, no two readers would diverge
- [ ] **Architecture** — not applicable; this is a documentation-practice decision
- [ ] **Domain correctness** — not applicable
- [ ] **Security** — not applicable
- [ ] **Testability** — not applicable; nothing here becomes a test
- [ ] **Accessibility** — not applicable
- [x] **UX and copy** — plain, no jargon beyond what's needed
- [ ] **Observability** — not applicable
- [x] **Maintainability** — explicit about its own failure mode (quiet neglect) and that the risk is visible, not silent
- [x] **Evolution** — reversal cost section correctly rates this low and says why

## Contradictions and gaps

None found against the constitution or later ADRs. This ADR is the reason the
other two being reviewed alongside it (0003, 0004) exist as a durable record
at all.

## What I did not review

I did not independently verify claims about industry ADR practice (Nygard
style) beyond taking them as reasonably well-known; not load-bearing to the
decision either way.
