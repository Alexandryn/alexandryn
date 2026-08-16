# Review: `backend-test-harness.md`, amendment (container topology)

| | |
|---|---|
| **Subject** | `.claude/specs/backend-test-harness.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16 |
| **Verdict** | Approved with changes (finding below fixed before this review) |

## Summary

Adds FR-10: a dedicated container-target test (build the image, `docker
compose up`, assert `Ready` against the sibling Postgres container),
closing the gap the topology audit named as the single largest hole in
test coverage — nothing previously exercised the container target's own
startup path at all. Updates FR-8's CI stage list, Failure modes,
Acceptance criteria, and Open questions to match.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Minor | FR-10 cadence | An early draft of FR-10 mirrored FR-7's "MAY run on a different trigger/cadence" language without arguing for it — FR-7's own reasoning for a possibly-different cadence is platform-sensitivity (macOS/Windows/Linux orphan-prevention differ), which has no equivalent for a container test that's the same on every CI runner | Fixed — FR-10 states the opposite default (same per-PR job as routine integration tests) and gives the actual reason (Docker owns process supervision, no platform-specific spawn mechanism to be slow or flaky), rather than defaulting to FR-7's shape for no stated reason |

## Dimensions checked

- [x] **Completeness** — updated every section FR-10 touches (FR-8's
      stage list, Failure modes, Acceptance criteria, Open questions,
      References), not only the FR itself — this is the same class of
      gap review 0022 caught in an earlier phase 03 spec (a stage added
      to prose but not the actual CI ordering list)
- [x] **Ambiguity** — a reader now knows FR-10's test is expected to run
      every PR, not "sometime," and knows why that's a different default
      than FR-7's
- [x] **Architecture** — no change to FR-1 through FR-6's harness
      mechanics (fixtures, clock, filesystem, randomness injection) — this
      is purely additive
- [ ] **Domain correctness** — not applicable
- [x] **Security** — confirmed the container-target test introduces no new
      CI secret (same reasoning `architecture-testing.md`'s own Security
      considerations already gives for the routine integration suite: a
      disposable, credential-free-beyond-what-Compose-manages Postgres)
- [x] **Testability** — this whole amendment *is* testability — FR-10 is
      itself the missing test, not a description of one
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [x] **Observability** — no new logging surface; FR-10's own failure is
      observable as a CI-red stage, consistent with every other stage in
      FR-8's list
- [x] **Maintainability** — the exact tooling for FR-10's assertion is
      deliberately left to the new deployment spec (Open questions),
      rather than guessed at here and likely wrong
- [x] **Evolution** — FR-10 is written so its own cadence question can be
      revisited by measurement later, same pattern FR-7 already uses

## Contradictions and gaps

Found and fixed (Finding #1). No contradiction found between FR-10 and any
other FR in this spec.

## What I did not review

The actual `Dockerfile`/`docker-compose.yml` content this test would run
against — not designed here, deliberately deferred to the new deployment
spec (Open questions, this file).
