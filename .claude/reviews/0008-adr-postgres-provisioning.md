# Review: ADR 0007 — Production PostgreSQL is bundled and managed

| | |
|---|---|
| **Subject** | `.claude/decisions/0007-postgres-provisioning.md` |
| **Reviewer** | Luann Moreira (pending confirmation — drafted by Claude as a first pass, not self-approved) |
| **Date** | 2026-08-14 |
| **Verdict** | Pending |

## Summary

Resolves review 0007's finding 1 (no screen anywhere configures a database
connection) with the reading the design's silence actually supports: bundle
and manage Postgres invisibly, don't invent a setup screen to match an
assumption. Consequence handled honestly rather than smoothed over — this
makes Postgres a third (sometimes fourth, macOS) process, which required
amending an already-`APPROVED` spec rather than quietly living with the
contradiction.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Informational | Rigor asymmetry | Confidence section already flags this honestly: Option A wasn't prototyped the way ADR 0005's process model was, and the bundled-vs-service-daemon comparison "wasn't weighed as rigorously." Noting only that this is a real, not cosmetic, gap — if this decision turns out wrong, it's the least-verified major decision in the roadmap so far | No change required — already disclosed. Consider whether phase 03 or 99 should include an actual packaging spike, the way ADR 0005 got one, before this is fully load-bearing |
| 2 | Minor | Completeness | The ADR names `embedded-postgres`-style tooling as an existing pattern but doesn't distinguish "a Go library that downloads/manages a postgres binary for you" from "manually invoking `initdb`/`pg_ctl` ourselves" as two different implementation shapes with different dependency implications (constitution §9) | Not a decision this ADR needs to make (that's phase 03/99's job), but the option space is narrower than "bundled" implies — worth a one-line acknowledgment that a dependency-justification step is still owed |

## Dimensions checked

- [x] **Completeness** — context, decision, three options, consequences, reversal, confidence
- [x] **Ambiguity** — decision is concrete: Go server spawns and owns Postgres, no user-visible config
- [x] **Architecture** — directly drives the `architecture-system.md` amendment; consistency checked against it, not just cross-referenced
- [ ] **Domain correctness** — not applicable
- [x] **Security** — doesn't weaken any trust boundary; Postgres stays loopback-only, Go-server-exclusive access unchanged from ADR 0004
- [ ] **Testability** — not applicable to the ADR itself; `architecture-persistence.md` carries the testable requirements
- [ ] **Accessibility** — not applicable
- [x] **UX and copy** — n/a to review, but correctly grounds the decision in what the design reference actually shows rather than inventing a screen
- [ ] **Observability** — not applicable at this decision's level
- [x] **Maintainability** — the "consequence acknowledged, not smoothed over" framing is exactly right — most ADRs would bury the FR-1 contradiction in a "Neutral" bullet; this one points straight at it
- [x] **Evolution** — reversal cost correctly rated high post-implementation, honest that nothing's built yet so it's cheap now

## Contradictions and gaps

None against ADR 0004 (engine choice unchanged) or the constitution. The one
real gap (finding 1) is already disclosed by the ADR itself, which is the
right outcome even though it means the decision carries real residual risk.

## What I did not review

Whether `embedded-postgres`-style Go tooling is mature enough for a
production desktop app's actual user data (as opposed to disposable test
fixtures, which is what it's typically used for) — I don't have hands-on
experience with any specific library to verify this claim beyond "the
general pattern is real." Flagged as unverified in the ADR itself; I'm not
adding false confidence here either.
