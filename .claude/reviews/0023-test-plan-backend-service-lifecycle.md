# Review: Test plan — backend service lifecycle, two independent agents

| | |
|---|---|
| **Subject** | `.claude/test-plans/backend-service-lifecycle.md` |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at review time (1 Blocking, 4 Major, 3 Minor across both passes) — see Resolution below for fixed status |

## Summary

Both agents read the test plan against `backend-service-lifecycle.md` (the spec it plans for), the test-plan template, and — independently — the other phase 03 specs it cites (`backend-test-harness.md`, `backend-http-transport.md`, `backend-configuration.md`, `backend-persistence.md`) and the phase 03 roadmap README. Neither agent overlapped on a single finding, which is itself informative: pass A caught two citation errors (a wrong config variable, a fabricated fixture citation) by checking claims against the specs referenced; pass B caught a structural gap (no dedicated layer for the plan's own stated top risk) by checking the plan's claims about itself for follow-through.

## Findings

| # | Severity | Area | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | Layers | Plan names shutdown-under-load "highest risk" and claims "disproportionate test effort," but no Layer section actually specifies a concurrency test — the only content was adversarial-table prose with no fixed mechanism (request count, transport, `-race` requirement, per-outcome assertion). Two engineers would build different tests, or none. | Add a dedicated Concurrency layer specifying transport, request count, timing, and exact per-outcome assertions; require `go test -race`. | Pass B |
| 2 | Major | Fixtures | `DATABASE_URL` named as the harness connection variable; `backend-test-harness.md` FR-2 fixes this as `TEST_DATABASE_URL`, explicitly distinct and never routed through `internal/config`. Conflating them undoes the separation that spec exists to enforce. | Correct to `TEST_DATABASE_URL`, cite FR-2. | Pass A |
| 3 | Major | Integration | "Pending migrations already applied partway" is self-contradictory phrasing, and cites a `backend-test-harness.md` fixture mechanism that doesn't exist there — conflates the partial-migration-failure case (which the Fixtures section already covers correctly) with the separate populated-database migration case the roadmap's risk table actually asks for. | Rewrite the bullet to describe only the partial-migration-failure path accurately; disclaim the populated-database case explicitly (see #6). | Pass A |
| 4 | Major | Adversarial cases vs "What is deliberately not tested" | Adversarial row for double-`SIGTERM` asserted three claims (no panic, no double-close, no restart-from-scratch); the "not tested" section then narrowed to two, contradicting the row above it. | Align both sections to the same two claims; explicitly scope out the third. | Pass B |
| 5 | Major | "What is deliberately not tested" | The roadmap's own risk table names "migrations tested against a populated database" — a real-data forward-migration case distinct from the partial-failure case this plan does cover. Not disclaimed anywhere, unlike other deferred concerns (e.g. Postgres spawn mechanics), so it could silently fall through the gap between this plan and `backend-persistence.md`'s. | Add an explicit disclaiming line attributing this case to `backend-persistence.md`'s own test plan. | Pass B |
| 6 | Minor | Contract | Attributed `/healthz`/`/readyz` response-shape ownership to `architecture-contracts.md`; `backend-http-transport.md` FR-5 actually fixes the shape, `architecture-contracts.md` only documents it. Error was inherited near-verbatim from the spec's own Test strategy table. | Correct attribution to `backend-http-transport.md` FR-5. | Pass A |
| 7 | Minor | Unit (FR-2) | "a static check (see below, not a runtime test)" — no later section elaborates; dangling forward reference, and doesn't name a concrete mechanism. | Describe the lint mechanism inline (extend `architecture-backend.md` FR-3's import-boundary lint, or a dedicated check). | Both, independently |
| 8 | Minor | Unit (FR-3) | Failure tests for steps 1–4/6–7 didn't assert the failing step's own fake was invoked exactly once, unlike the step-5 retry test which explicitly counts attempts — "no unbounded retry" was inferred from control flow, not directly proven for the non-exempt steps. | Add an explicit invoked-exactly-once assertion to those tests. | Pass A |

Not raised as findings, noted by pass A as explicitly checked and passing: FR-to-test mapping is complete and correct for all seven FRs; risk prioritization is traceable to the spec and to review `0022`'s prior finding, not arbitrary; "what is deliberately not tested" otherwise honestly matches the spec's Non-goals/Open questions; adversarial coverage meets constitution §10's bar given this spec's actual surface (no raw byte parsing, so "enormous" doesn't apply here the way it would to a source adapter).

## Dimensions checked

- [x] Completeness (FR-to-test mapping, against the spec directly)
- [x] Testability (mechanism specificity — the source of finding #1)
- [x] Cross-spec accuracy (citations checked against the specs cited, not just internal consistency — the source of findings #2, #3, #6)
- [x] Internal consistency (adversarial table vs. exclusions section — the source of finding #4)
- [x] Determinism (fixtures — no real sleeps, no unseeded randomness; confirmed sound)
- [x] Alignment with phase 03's own risk table and exit criteria (source of finding #5)

## What was not reviewed

Neither agent executed or compiled anything — documents-only review, same as `0022`. Neither agent evaluated whether the proposed concurrent-request count (5) or delay values are well-chosen numbers, only that the mechanism is now specified precisely enough to implement consistently.

## Resolution

All 1 Blocking, 4 Major, and 3 Minor findings fixed in this pass:

- **#1 (no Concurrency layer)** — Fixed: added a dedicated Concurrency layer section specifying real `net.Listener`/`http.Server` (not `httptest`), ≥5 concurrent requests with fixed delays spanning the grace-period boundary, exact per-outcome assertions, mandatory `go test -race`, and the double-`SIGTERM` safety check moved here from the adversarial table's prose.
- **#2 (`DATABASE_URL`/`TEST_DATABASE_URL` conflation)** — Fixed: Fixtures section corrected to `TEST_DATABASE_URL`, cited to `backend-test-harness.md` FR-2.
- **#3 (self-contradictory/fabricated migration bullet)** — Fixed: Integration bullet rewritten to describe only the partial-migration-failure path, tied to the Fixtures section's actual broken-migration fixture; the populated-database case addressed separately per #5.
- **#4 (adversarial/exclusions contradiction)** — Fixed: both sections now claim only "no panic, no double-close of the pool" for double-`SIGTERM`; restart-from-scratch behavior explicitly named as out of scope in both places.
- **#5 (populated-migration risk gap)** — Fixed: added an explicit line to "What is deliberately not tested" attributing the populated-database migration case to `backend-persistence.md`'s own test plan.
- **#6 (contract ownership misattribution)** — Fixed: Contract section now attributes response shape to `backend-http-transport.md` FR-5, `architecture-contracts.md`'s role narrowed to documentation.
- **#7 (dangling "see below")** — Fixed: FR-2 bullet now names the mechanism inline.
- **#8 (missing invoked-exactly-once assertion)** — Fixed: FR-3 failure-and-exit bullet now includes it.

Test plan status remains `DRAFT` pending maintainer review — this record covers the plan document, not test code that doesn't exist yet. RED phase (writing the actual failing tests) has not started.
