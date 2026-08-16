# Review: `backend-persistence.md`, amendment (container topology)

| | |
|---|---|
| **Subject** | `.claude/specs/backend-persistence.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16 |
| **Verdict** | Approved with changes (finding below fixed before this review) |

## Summary

FR-5 and Security considerations both described the `DATABASE_URL`-present
branch as a developer/CI/test-only bypass, forbidden in production. Amends
both to state that the same code path is also the container-hosted
target's normal, sole production path — no new logic, since the branch
already exists exactly as needed; only its status as "exception" or "rule"
depends on which target is running.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Minor | FR-5 wording | An early pass of this amendment kept "MUST be skipped entirely" phrased only around the Electron target's spawn step, without also stating plainly that the container target has no spawn step to skip in the first place — a reader could wrongly infer a "skipped" step still conceptually exists and runs a no-op | Fixed — FR-5 now states the container-hosted target has no spawn-and-own alternative to fall back to at all, not merely one it skips |

## Dimensions checked

- [x] **Completeness** — checked FR-5, its Security considerations
      counterpart, and the References section together, since the audit
      (`0002-topology-gap.md`) flagged this spec's independent restatement
      of the same DATABASE_URL policy `backend-configuration.md` also
      states
- [x] **Ambiguity** — a reader now knows which target makes the
      `DATABASE_URL`-present branch the exception versus the rule, rather
      than only being told it's conditional
- [x] **Architecture** — no change to the repository pattern (FR-1/FR-2),
      transaction boundaries (FR-4), or the macOS supervisor (FR-8) — this
      amendment touches only which branch of FR-5 is "normal"
- [ ] **Domain correctness** — not applicable
- [x] **Security** — confirms the actual security property (spawned
      binary path never influenced by external input, `cmd/pg-supervisor`
      restrictions) is unaffected; the container target introduces no new
      spawned-process attack surface since it spawns nothing
- [x] **Testability** — flags, but does not resolve here, that no test
      currently exercises the container target's own connection path —
      tracked as its own amendment to `backend-test-harness.md`, not
      folded into this one
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [x] **Observability** — migration success/failure logging (FR-6) is
      target-independent and needed no amendment, confirmed while
      reviewing
- [x] **Maintainability** — matches the same amendment-note pattern used
      across every other file in this batch
- [x] **Evolution** — the "which target makes this the exception vs. the
      rule" framing generalizes cleanly if a third target is ever added

## Contradictions and gaps

Found and fixed (Finding #1). Confirmed no contradiction remains between
this spec's restated policy and `backend-configuration.md`'s — both now
state the same target-dependent meaning in matching terms, checked side
by side.

## What I did not review

The actual test suite that would prove the container target's connection
path works — that's `backend-test-harness.md`'s amendment, reviewed
separately.
