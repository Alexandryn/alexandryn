# Review: Test plan — backend errors and logging, two independent agents

| | |
|---|---|
| **Subject** | `.claude/test-plans/backend-errors-and-logging.md` |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at review time (1 Blocking, 6 Major, 3 Minor/Nit across both passes, 1 finding raised independently by both) — see Resolution below for fixed status |

## Summary

Both agents read the test plan against `backend-errors-and-logging.md` (the spec it plans for), the test-plan template, and — independently — related specs it cites (`architecture-contracts.md` FR-5, `backend-configuration.md`'s sibling FR-7 test plan, `backend-http-transport.md`'s middleware order, phase 03's roadmap README). Both passes independently caught the same Blocking gap — FR-6 (logger construction: JSON, single injected instance, no globals) had zero test anywhere in the document despite the plan's own exit criteria claiming full FR coverage. Beyond that overlap, each pass found different things: pass A caught an unacknowledged deviation from the spec's own Test strategy table (FR-11 relocated from the Contract layer to a bespoke Integration-layer handler) and a testing-the-function-not-the-wiring gap on FR-3; pass B caught a self-contradiction in the Concurrency section, two places the plan baked in unresolved implementation details as if fixed (`errors.As`, a specific panic-injection site), and a missing adversarial-case test.

## Findings

| # | Severity | Area | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | FR-6 coverage | FR-6 (single injected `*slog.Logger`, JSON-structured, never package-level) had no test anywhere — not Unit, not Risk assessment, not excluded — while the plan's own exit criteria claimed every FR mapped to a test. | Add a Unit-layer test (JSON handler output, structural no-globals check) and a Risk assessment entry. | Both, independently |
| 2 | Major | FR-11 / Contract layer | The spec's own Test strategy table assigns FR-11 to run "as part of" `architecture-contracts.md`'s contract test; the plan silently substituted a bespoke test-only Integration-layer handler instead, with no acknowledgment of the deviation. | Keep the substitution (phase 03 has no real domain endpoints for the real contract test to exercise yet) but state it explicitly as a reasoned, temporary deviation, with a named point (phase 06+) where it moves back into the contract test proper. | Pass A |
| 3 | Major | Concurrency section | Self-contradiction: the Concurrency section claimed "no concurrency property is unique to error/logging behavior specifically," while the Adversarial table's own row argued the opposite and no Layer section actually contained a test for it. | Reconcile: name the one concurrency property this spec does own (concurrent panic isolation) and add its actual test to the Integration layer. | Pass B |
| 4 | Major | FR-3 wiring | Both the Unit and Integration FR-3 tests called the translation function directly; neither called a real repository method end-to-end, so the spec's own named risk (a method *forgetting* to call the translator) went untested even though the translator itself was well-covered. | Add an Integration-layer test calling a real repository method directly, asserting its own returned error already carries the right category. | Pass B |
| 5 | Major | FR-3 wrapping mechanism | The Unit-layer wrapping test asserted recovery via `errors.As` specifically — an implementation detail the spec's own Open questions leave unresolved (whether `domain.Error` even carries a wrapped field). Overclaimed certainty the spec doesn't have. | Reword to test against Go's general `errors.Is`/`errors.As` wrapping contract without committing to one extraction mechanism. | Pass B |
| 6 | Major | FR-10 mechanism | "Panics from inside the limits middleware specifically" named no injection mechanism — limits middleware normally returns errors, not panics, so two engineers would solve this differently. | Specify a test-only middleware stub inserted at the same chain position (after recovery, before logging) whose only job is to panic unconditionally. | Pass B |
| 7 | Minor | FR-11 handler construction | The test-only handler's error construction path wasn't specified as routing through real `domain.Error`/FR-5 helper construction, risking the pattern check proving only that the regex works, not that the real pipeline is clean. | State explicitly that the handler triggers each category via real `domain.Error` and the real FR-5 helper. | Pass A |
| 8 | Nit | FR-7 assertions | Tests asserted a correlation ID value was correct but never the literal log field key (`correlationId`) — a field-name typo would pass every listed assertion. | Add an explicit field-key assertion. | Pass A |
| 9 | Minor | FR-1/FR-2 totality test | "Asserting the function's own switch has exactly six cases" isn't achievable by black-box testing and duplicated the FR-4 test's job. | Reworded to iterate FR-1's own category list as the shared source of truth between the mapping function and the test. | Pass B |

Also fixed, Minor, raised by pass B: Adversarial table's non-`error`-panic-value row had no corresponding Unit/Integration test bullet despite the exit criteria's blanket coverage claim — added.

## Dimensions checked

- [x] Completeness (FR-to-test mapping against the spec directly — source of finding #1)
- [x] Testability (mechanism specificity — source of findings #6, #7)
- [x] Cross-spec accuracy / layering discipline (source of finding #2, checked against `architecture-contracts.md` and `backend-http-transport.md` directly)
- [x] Internal consistency (Concurrency section vs. Adversarial table — source of finding #3)
- [x] Risk-vs-actual-coverage (does the plan test the risk the spec's own Failure modes table names, not just an adjacent function — source of finding #4)
- [x] Spec-vs-test-plan boundary discipline (not baking in unresolved implementation choices — source of finding #5)
- [x] Fixture independence (FR-8's fixture checked against `backend-configuration.md`'s own plan directly — confirmed genuinely independent, no finding)

## What was not reviewed

Neither agent executed or compiled anything — documents-only review, same discipline as prior reviews in this series. Neither agent evaluated the specific choice of SQLSTATE codes named in the FR-3 examples (`23505`, `23502`) for completeness against what this codebase will eventually need to map.

## Resolution

All 1 Blocking, 6 Major, and 3 Minor/Nit findings fixed in this pass:

- **#1 (FR-6 untested)** — Fixed: added a Unit-layer test for JSON-structured output and a structural no-globals check (mirroring the pattern already used for FR-1/FR-2 in `backend-configuration.md` and FR-2 in `backend-service-lifecycle.md`), plus a Risk assessment entry.
- **#2 (FR-11 undisclosed deviation)** — Fixed: the Integration-layer FR-11 bullet now explicitly names this as a deliberate, temporary deviation from the spec's Contract-layer assignment, with the reason (no real domain endpoints exist yet) and the point where it reverts (phase 06+).
- **#3 (Concurrency self-contradiction)** — Fixed: Concurrency section now names concurrent panic isolation as this spec's one owned property; the actual test was added to the Integration layer, run under `go test -race`.
- **#4 (FR-3 wiring untested)** — Fixed: added an Integration-layer test calling a real repository method directly.
- **#5 (`errors.As` overclaim)** — Fixed: reworded to Go's general wrapping contract, not a specific extraction mechanism.
- **#6 (FR-10 mechanism unspecified)** — Fixed: named the test-only middleware-stub injection mechanism and its chain position.
- **#7 (FR-11 handler construction path)** — Fixed: now specifies real `domain.Error`/FR-5 helper construction.
- **#8 (missing field-key assertion)** — Fixed: added to FR-7's Unit test.
- **#9 (untestable switch-case assertion)** — Fixed: reworded to a shared-source-of-truth list iteration.
- Non-`error` panic value test (Minor, adversarial row) — Fixed: added as a second FR-10 Unit-layer bullet.

Test plan status remains `DRAFT` pending maintainer review — this record covers the plan document, not test code that doesn't exist yet. RED phase has not started.
