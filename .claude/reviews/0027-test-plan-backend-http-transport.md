# Review: Test plan — backend HTTP transport, two independent agents

| | |
|---|---|
| **Subject** | `.claude/test-plans/backend-http-transport.md` |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at review time (6 Major across both passes, 2 pairs independently confirmed, plus 2 Minor, 2 Nit) — see Resolution below for fixed status |

## Summary

Both agents read the test plan against `backend-http-transport.md` (the spec it plans for), the test-plan template, and — independently — related specs and the phase 03 roadmap README. Two findings were raised independently by both passes: the `/healthz`-stays-200-while-`/readyz`-flips-503 independence claim was proven by two separate tests at two different layers rather than one combined test, and the "fake/injectable clock" branch for timeout tests named a mechanism (`net/http.Server` clock injection) that doesn't exist in the stdlib. Beyond that overlap, pass A caught a misattributed justification in the Risk assessment (borrowed from the spec's own wording, which itself borrowed the wrong roadmap claim) and a config-wiring gap; pass B caught a vacuously-passable `/healthz` test, a header-size test placed in a layer that can't actually exercise it, and a missing test for health endpoints going through the same limits middleware as everything else.

## Findings

| # | Severity | Area | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Major | Risk assessment | Justified timeout-testing priority by citing "phase 03's own emphasis" on this being the hardest thing to test — but the roadmap's actual README names *graceful shutdown under in-flight requests* as that (`backend-service-lifecycle.md`'s concern), not timeouts. The claim was inherited verbatim from the spec's own Test strategy table, which uses the same wording. | Reworded to cite the real justification (constitution §4, FR-2's unproven placeholder numbers) instead of the misattributed claim. | Pass A |
| 2 | Major | Concurrency mechanism | "A fake/injectable clock if the test harness's timeout-testing convention supports it... otherwise a short value" left a fictional branch — `net/http.Server`'s timeout fields have no clock-injection hook of any kind; `backend-test-harness.md` FR-5's `Clock` interface is scoped to application code calling `Now()`, not stdlib server internals. | Reworded to state directly that a short, explicit test-specific duration is the only mechanism, with the reason named. | Both, independently |
| 3 | Major | `/healthz` independence proof | The spec's acceptance criterion requires proving, in one scenario, that `/healthz` stays 200 while `/readyz` flips 503 when Postgres drops — the plan tested them separately (Unit layer for `/healthz`, Integration layer for `/readyz`), which proves each works alone, not that they're independent under the same failure. | Combined into one Integration-layer test asserting both endpoints in the same run against the same dropped connection. | Both, independently |
| 4 | Major | `/healthz` Unit test | "A fake dependency that would fail if called" was underspecified to the point of being vacuously passable: if `/healthz` genuinely takes no DB dependency at all, there's nothing to wire the fake into, and the test would just assert 200 without proving DB-avoidance. | Named the concrete mechanism: a shared pool-reference parameter with `/readyz`, a poisoned spy in that slot for `/healthz`'s test, and an explicit zero-call-count assertion. | Pass B |
| 5 | Major | FR-2 header-size test placement | Placed in the Unit layer, whose fixtures are `httptest.NewRecorder`/`NewRequest` — those construct an `http.Request` directly and never parse raw header bytes off a connection, so they cannot exercise `net/http.Server.MaxHeaderBytes` at all. | Moved to the Concurrency layer, using the same raw-`net.Conn`-against-a-real-listener mechanism the timeout tests already use. | Pass B |
| 6 | Major | Limits middleware on health endpoints | FR-1 places limits ahead of routing for every request, meaning `/healthz`/`/readyz` are inside the same timeout/body-size enforcement as any other path — nothing in the plan tested this, leaving a gap where an implementation could special-case health endpoints outside the composed chain undetected. | Added a Concurrency-layer case routing a slow request at a health endpoint through the real composed chain. | Pass B |
| 7 | Minor | Config wiring | No test confirmed `backend-configuration.md`'s four HTTP config keys actually reach the constructed `net/http.Server`'s fields — all limit/timeout tests used ad hoc test values, never values sourced from config. | Added an explicit config-to-field wiring test and a Risk assessment note. | Pass A |
| 8 | Minor | Exit criteria | The spec's own acceptance criterion 6 ("every FR maps to a line in phase 03's own exit criteria") had no corresponding checkbox in the plan's Exit criteria. | Added the checkbox. | Pass B |
| 9 | Nit | Template completeness | No explicit "End to end: N/A" section, unlike the Accessibility section which does state N/A. | Added. | Pass A |
| 10 | Nit | OpenAPI documentation | The spec requires `/healthz`/`/readyz` be documented in `api/openapi.yaml`; no test or exclusion note addressed this. | Added to "What is deliberately not tested," pointed at a CI/lint check as the more appropriate mechanism. | Pass B |

## Dimensions checked

- [x] Completeness (FR-to-test mapping against the spec directly)
- [x] Testability (mechanism specificity — source of findings #2, #4, #5, #6)
- [x] Cross-spec accuracy (citations checked against `backend-errors-and-logging.md`, `backend-service-lifecycle.md`, `backend-test-harness.md`, the phase 03 roadmap README directly — source of finding #1)
- [x] Acceptance-criteria-to-test traceability (source of finding #3)
- [x] Failure-modes-table coverage (all six rows checked; no silent omission found beyond finding #3's combination issue)
- [x] Non-duplication claims verified against the actual sibling plan content, not just asserted (`backend-errors-and-logging.md`'s FR-3/FR-7 delegation — confirmed accurate, no finding)

## What was not reviewed

Neither agent executed or compiled anything — documents-only review, same discipline as prior reviews in this series. Neither agent evaluated whether the specific short timeout durations the Concurrency layer will eventually use (not yet numbered, left as "e.g. 100ms") are well-chosen.

## Resolution

All 6 Major, 2 Minor, and 2 Nit findings fixed in this pass:

- **#1 (misattributed risk justification)** — Fixed: reworded to cite constitution §4 and FR-2's unproven placeholders; the roadmap's actual shutdown-emphasis claim is now correctly attributed to `backend-service-lifecycle.md`.
- **#2 (fictional clock-injection branch)** — Fixed: commits directly to short test-specific durations, names why (no stdlib hook, `backend-test-harness.md` FR-5 scoped elsewhere).
- **#3 (`/healthz`/`/readyz` independence)** — Fixed: combined into one Integration-layer test.
- **#4 (vacuous `/healthz` test)** — Fixed: named the shared-parameter/spy/zero-call-count mechanism.
- **#5 (misplaced header-size test)** — Fixed: moved to Concurrency layer.
- **#6 (health endpoints bypassing limits)** — Fixed: added explicit Concurrency-layer case.
- **#7 (config wiring untested)** — Fixed: added a dedicated wiring test and Risk assessment note.
- **#8 (missing exit-criteria checkbox)** — Fixed: added.
- **#9 (missing End to end section)** — Fixed: added, N/A.
- **#10 (OpenAPI documentation untested)** — Fixed: added to exclusions with a pointer to the right mechanism.

Test plan status remains `DRAFT` pending maintainer review — this record covers the plan document, not test code that doesn't exist yet. RED phase has not started.
