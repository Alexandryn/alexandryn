# Review: `backend-service-lifecycle.md`, amendment (container topology)

| | |
|---|---|
| **Subject** | `.claude/specs/backend-service-lifecycle.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16 |
| **Verdict** | Approved with changes (no findings — clean on first pass) |

## Summary

FR-1 step 5 labeled the `DATABASE_URL`-present branch "the developer/CI/
test path," the same stale framing already corrected in
`backend-configuration.md` and `backend-persistence.md`. Amends the same
sentence to name the container-hosted target's production use as the
branch's other legitimate reason to run, with no change to the step
ordering, the retry/backoff behavior (FR-3), or any other part of the
startup sequence.

## Findings

None. This is the smallest of the four spec amendments in this batch — one
sentence, mechanically consistent with the two already-reviewed changes it
mirrors.

## Dimensions checked

- [x] **Completeness** — checked the whole spec for other places the
      DATABASE_URL branch or "production" is named as a topology (FR-3's
      failure-mode table, the State transitions diagram) — both already
      say "production" / "dev/test" generically without asserting *which*
      target is production, so neither needed a change
- [x] **Ambiguity** — FR-1 step 5 now names both legitimate reasons the
      branch runs, rather than one
- [x] **Architecture** — the seven-step startup sequence (FR-1) keeps its
      exact ordering and preconditions; this amendment only names *why* a
      given process might be on the DATABASE_URL branch, not *when* it
      runs
- [ ] **Domain correctness** — not applicable
- [x] **Security** — no change; fail-loud startup (FR-3) and no-secrets-
      in-logs (FR-1/FR-3) apply identically regardless of which target
      put the process on this branch
- [x] **Testability** — the shutdown-under-load test and startup-ordering
      tests (Test strategy) are unaffected; neither depends on which
      target supplied `DATABASE_URL`
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [x] **Observability** — this spec's own Observability NFR (log a line on
      every startup step) is target-independent and needed no change;
      `architecture-system.md`'s cross-process correlation requirement was
      the one that needed a container-target clause, already amended
      separately
- [x] **Maintainability** — completes the set of three specs
      (`backend-configuration.md`, `backend-persistence.md`, this one)
      that each independently restated the same DATABASE_URL policy —
      now consistent across all three
- [x] **Evolution** — no structural change; this spec's step ordering was
      already topology-agnostic, only its prose commentary needed fixing

## Contradictions and gaps

None found. Confirmed this spec's restated policy now agrees with both
`backend-configuration.md`'s and `backend-persistence.md`'s amended
versions, checked against each directly.

## What I did not review

The GitHub Actions CI pipeline this spec's own FR-1 sequence gets tested
against — unaffected, out of scope for this amendment, not re-verified.
