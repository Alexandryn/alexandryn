# 0016. Test plans are written per-phase, at RED-step time, not per-spec at approval time

| | |
|---|---|
| **Status** | Accepted (2026-08-28, phase 04 Tier 6 / F27 — maintainer decision D3) |
| **Date** | 2026-08-17 (proposed); 2026-08-28 (accepted) |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

<!-- Status: Proposed | Accepted | Rejected | Superseded | Deprecated -->

> **Accepted 2026-08-28** (phase 04 Tier 6 closure). Phases 04's six
> specs did carry their test strategy before each tier's RED step — in
> the per-tier `tasks/plan-p04-<tier>.md` docs, not in separate
> `.claude/test-plans/*.md` files. That is ratified here as a valid form
> of "a test plan written before the implementation" for a phase whose
> work is sliced into tiers: the plan doc's Test-strategy section is the
> test plan. `test-plans/README.md` is amended to say so, and Option A's
> named enforcement gap is closed by adding a test-plan line to each
> phase's Exit criteria (done for phase 04 in its Tier 6 closure; later
> phases add theirs when they open).

## Context

`test-plans/README.md` states: *"A test plan accompanies each specification
and is written before the implementation."* Read one way, that means
immediately upon each spec's approval, regardless of whether that spec's
phase is about to be implemented. Read the other way, it means before
*that spec's own* implementation begins — which, for every phase but 03,
hasn't happened yet.

The actual project history matches the second reading, not the first:
phase 03's six specs were approved (`49f64ab`), and test plans for all six
followed immediately after (`3e5fdaa`…`179c6df`), right at the point
implementation was slated to begin. No test plan has been written for any
spec since — but no phase past 03 has reached implementation either. A
count taken 2026-08-17 found 43 total specs, 6 with test plans (all phase
03), 37 without. Of those 37, 7 are phase-01 `architecture-*` specs whose
own Test strategy sections already state *"verified by walkthrough, not
execution"* — arguably self-exempt, since they produce no directly-testable
code artifact of their own. The remaining 30 have real implementation
surface and genuinely have no test plan yet.

Whether this is a process violation needing a backlog, a deliberate
cadence that was never wrong, or a rule whose wording overclaimed a
stricter timing than intended, needed a decision rather than being left
ambiguous a second time.

## Decision

Test plans for a phase's specs are written immediately before that
phase's own RED step — the point its implementation actually begins — not
immediately upon each spec's own approval. Phase 03's history is the
pattern this decision ratifies, not an exception to it: its test plans
existed exactly when they needed to, at the boundary between "specs
approved" and "implementation starts," and no earlier. Phases 04 through
11 write theirs when their own implementation begins, not before. This is
not a backlog of overdue work — nothing in any of those phases implements
before RED regardless of when its test plan exists, so no phase is
currently at risk of skipping test-first discipline by this decision.

`test-plans/README.md`'s own wording is not amended by this decision — it
already permits this reading; this ADR states which reading governs,
rather than leaving it to be re-derived, or misread as a violation, the
next time someone counts.

## Options considered

### Option A — Defer with an explicit trigger, recorded here (chosen)

*For* — zero new test-plan-writing cost right now; matches the pattern
phase 03 already set, rather than treating that pattern as luck; the
roadmap's own "detail decreases with distance" reasoning
(`roadmap/README.md`) already argues against writing detailed
requirements — phase docs, in that case — far ahead of when they're
needed, because it produces "confident fiction that later gets treated as
a decision." The identical risk applies to test plans: writing 30 of them
against specs that are still 8 phases away from implementation, several of
which (`backend-configuration.md` alone, four times) have already needed
post-approval amendment once real work started, front-loads work that may
need redoing.

*Against* — real, and named without softening: this decision records
*intent*, not a mechanical gate. Nothing currently stops a future phase
from reaching its own RED step without a test plan existing — this ADR
states the rule, it doesn't enforce it. Pairing it with an explicit
exit-criteria checklist item (each phase's own `Exit criteria` section
gaining a line naming its test plans) would close that gap; not done as
part of this ADR, since it wasn't asked for here — named as a natural
follow-up, not assumed solved by this decision alone.

### Option B — Backfill all 30 now

*For* — the rule as most literally read is satisfied immediately, no
interpretation required, no risk of the trigger being missed later.

*Against* — rejected. Roughly 30 documents at the depth of the existing
six (each 200–300+ lines: layers, fixtures, adversarial cases, acceptance
criteria) is a large amount of work against specs that may still shift
before their own phase's implementation begins — the same "confident
fiction" risk named above, at a much larger scale than Option A's
residual enforcement gap.

### Option C — Amend the rule text directly instead of recording a decision

*For* — smaller diff; states the cadence as the rule itself rather than
as an interpretation of an ambiguous one.

*Against* — rejected in favor of Option A specifically because it would
frame the original wording as having been imprecise all along, when the
honest account is that a real interpretive choice is being made now, with
reasoning that didn't exist when `test-plans/README.md` was first
written. Recording that as a decision — what was ambiguous, what's chosen,
why — is the more honest instrument, per constitution §12 ("a design
decision was a guess... record it as a guess"). The rule's wording stays
stable and simple; this ADR carries the nuance, with a one-line pointer
added to `test-plans/README.md` so a reader lands here.

## Consequences

**Good** — no effort spent writing test plans against specs likely to
shift before their phase's own implementation starts. Phase 03's existing
practice is confirmed as the intended pattern for every later phase, not
retroactively excused as a one-off.

**Bad** — the trigger is enforced by discipline and review, not
mechanically, until and unless a phase's own exit criteria gain an
explicit checklist line for it — a real gap, named here rather than
implied to be closed by this ADR's existence alone.

**Neutral** — `test-plans/README.md`'s text is unchanged; this ADR is the
authoritative reading of what "before the implementation" in that
document has always meant, going forward.

## Reversal cost

Low. This is a process/cadence decision with no code or schema built
against it — revisiting it later (e.g. deciding to backfill after all, or
adding the exit-criteria checklist item Option A's "Against" names) costs
nothing beyond writing the additional test plans or the checklist line
itself; nothing already written needs to change.

## Confidence

Medium. High confidence that no phase is currently harmed by this
decision — nothing has implemented ahead of a test plan, and nothing is
about to. Lower confidence on whether phase 03's history was genuinely
*intended* as this cadence at the time, or was simply what happened to
occur before the practice was abandoned for reasons unrelated to
cadence — the same honesty caveat already used for this session's ADR 0005
and ADR 0007 addenda: a reading of past intent, not a fact the record
settles unassisted.
