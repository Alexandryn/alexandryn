# Review: `backend-test-harness.md`, amendment (container topology)

| | |
|---|---|
| **Subject** | `.claude/specs/backend-test-harness.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16, mechanism finding added 2026-08-17 |
| **Verdict** | Approved with changes (both findings fixed before this review) |

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
| 2 | Blocking | FR-10 mechanism | The version of FR-10 first drafted said the test would "assert the backend container reaches Ready... against the sibling postgres container" without naming a mechanism, which was about to be implemented as a network-based check from a test runner or sibling container — genuinely impossible against `backend-configuration.md` FR-8's loopback-only bind, since nothing outside a container's own namespace can reach its loopback address. Caught before drafting the deployment spec that would have had to implement it. | Fixed — FR-10 now specifies `HEALTHCHECK` in the `Dockerfile`, executing inside the container's own namespace (the same relationship `docker exec` has to the container, confirmed against Docker's own Compose reference documentation), observed via `docker compose ... --wait`'s exit code against the daemon's own health status — never a network path crossing the loopback boundary. No amendment to `backend-configuration.md` FR-8 needed; constitution §6 is not reinterpreted |

**On Finding 2:** this was raised as a stop condition in this session rather
than resolved unilaterally — two approved documents (this FR as first
drafted, and `backend-configuration.md` FR-8) appeared to conflict, with no
code yet to settle which one was wrong. Verified against Docker's own
documentation before acting on the correction, per the same "cite before
you claim it" discipline this project's review process already requires
everywhere else.

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
