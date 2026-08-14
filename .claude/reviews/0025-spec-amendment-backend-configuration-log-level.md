# Review: Spec amendment — `backend-configuration.md`, `LOG_LEVEL` case-sensitivity

| | |
|---|---|
| **Subject** | `.claude/specs/backend-configuration.md` FR-4 |
| **Reviewer** | Self-reviewed |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (the amendment itself) |

## Summary

Writing the test plan for this spec (`.claude/reviews/0024`) surfaced a
genuine gap: `LOG_LEVEL`'s FR-4 row declared an enum of four values but
never stated whether matching was case-sensitive. Pass B of that review
correctly flagged this as a spec question, not something a test plan
should decide unilaterally (constitution §1: spec precedes
implementation). The test plan itself was left with the adversarial case
unresolved and an explicit note pointing back here.

## Decision

`LOG_LEVEL` matching is case-insensitive: the value is lowercased before
comparison against `debug`/`info`/`warn`/`error`. Reasoning: an
environment variable's incidental casing (`LOG_LEVEL=Info` vs. `info`) is
not the class of misconfiguration constitution §11's fail-loudly stance
exists to catch — that stance is about catching genuinely wrong or
missing values, not punishing a keyboard habit. `LOG_LEVEL` carries no
sensitive data, so this doesn't interact with FR-7's redaction
requirements or FR-6's validation-error contract in any way that needs
separate treatment; an unrecognized value after lowercasing still fails
FR-6 exactly as before.

## Findings

None beyond the gap itself — this is a one-line clarification, not a
redesign. No other FR-4 key has an analogous ambiguity (the other enum-
shaped or typed keys — `BIND_ADDRESS`, the duration/integer keys — don't
involve string-literal matching against a fixed vocabulary the way
`LOG_LEVEL` does).

## Dimensions checked

- [x] Completeness (confirmed no other key shares this gap)
- [x] Consistency (confirmed no interaction with FR-6/FR-7)
- [ ] Independent review — not run; a genuinely independent two-agent
      pass is disproportionate for a single-line enum-matching
      clarification with no design tradeoff. Self-review only, per this
      project's own judgment call on proportionality
      (`.claude/reviews/README.md` doesn't mandate independent review for
      every change, and the make/improve/review/fix discipline this
      project follows explicitly allows self-review for small/mechanical
      steps).

## Resolution

Fixed in this pass: FR-4's `LOG_LEVEL` row now states matching is
case-insensitive; a short rationale note added after the FR-4 table,
alongside the two prior post-approval amendment notes it already carries
(`DATABASE_URL`'s category, the `HTTP_REQUEST_TIMEOUT` split).

`.claude/test-plans/backend-configuration.md`'s `LOG_LEVEL` adversarial
row and its corresponding "What is deliberately not tested" entry are now
stale (they described this as unresolved) — updating both is the next
step, tracked separately, not part of this review record.

Maintainer re-confirmation of this specific amendment is still
outstanding, consistent with how this project has already handled every
other post-approval spec amendment (`architecture-system.md`'s ADR 0007
amendment carries the same "needs re-confirmation" status).
