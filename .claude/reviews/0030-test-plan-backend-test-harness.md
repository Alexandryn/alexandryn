# Review: Test plan — backend test harness, two independent agents

| | |
|---|---|
| **Subject** | `.claude/test-plans/backend-test-harness.md` |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at review time (1 Blocking, confirmed independently by both; 2 further Major; 5 Minor; 1 Nit) — see Resolution below for fixed status |

## Summary

Both agents read the test plan against `backend-test-harness.md` (the spec it plans for — an unusual one, since it *is* the test-mechanics document other phase 03 specs point to rather than a feature with application behavior), the test-plan template, and — independently — related material (`architecture-testing.md`, review `0022`'s composability finding, the phase 03 roadmap README, and sibling test plans this one cross-references). Both passes independently converged on the same Blocking finding: the spec's own acceptance criteria 6 and 9 require scenario-specific proofs (the shutdown-grace-period timeout, a correlation ID reaching both a log line and a response) that this plan silently delegated to sibling plans without a traceable cross-reference — a reader of this document alone couldn't find where its own spec's ACs are satisfied. Beyond that overlap, pass A found the FR-2 subprocess-assertion mechanism underspecified at exactly its highest-risk point; pass B found a real Failure-modes-table gap (the `web/`-build-failure case) and several coverage/traceability gaps.

## Findings

| # | Severity | Area | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | AC 6 / AC 9 traceability | The spec's own acceptance criteria for `Clock` and `IDGenerator` require scenario-specific proofs (shutdown-grace-period timing, correlation ID in both log and response) that this plan doesn't perform — it tests generic fake mechanics only and mentions the delegation solely in prose within the Risk assessment, not as a structured, findable cross-reference. Both plans could end up assuming the other closes the gap. | Added explicit cross-references at each relevant Unit-layer bullet, naming which sibling plan's specific test satisfies which spec AC. | Both, independently |
| 2 | Major | FR-2 mechanism | "At least one test function's body is asserted to have never executed" for a *subprocess* had no stated mechanism — `go test` output for an early-exiting run doesn't reliably expose this per-function. | Named the concrete mechanism: a fixture test writes a sentinel file on entry, before any assertion; the outer test asserts the sentinel is absent after the subprocess exits, proving the function body was never entered. | Pass A |
| 3 | Major | Failure modes coverage | The spec's Failure modes row 3 ("`web/` build step skipped or fails before `go build`") was never exercised — the static YAML order check only confirms stage *text order*, and the one live-gating test used a failing unit test, a different failure mode entirely. | Added a second live CI-infrastructure test: a deliberate `web/`-build-specific breakage, confirming the workflow fails at that stage and never reaches later stages. | Pass B |
| 4 | Minor | Fixtures / FR-4 | The plan named four owning domain specs but not the actual aggregate names, and didn't address that three of the four representative aggregates carry a required foreign key into an already-persisted parent — complicating the "zero required overrides" claim without explanation. | Named the concrete aggregates (`Work`, `LibraryEntry`, `SourceOffering`, `ReadingProgress`) and clarified "zero overrides" means the factory constructs necessary parent rows internally, not that no FK exists. | Pass B |
| 5 | Minor | AC 9 (this spec's own) crosswalk | "Every FR maps to a line in phase 03's own exit criteria" is a documentation cross-check distinct from "FR maps to a test" — the plan's exit criteria only answered the latter. | Added the crosswalk as its own exit-criteria line. | Pass B |
| 6 | Minor | AC 1 substitution | The spec's AC 1 literally asks for proof "on a machine with no Postgres available at all"; the plan substituted a structural compile/vet check without stating why that's equivalent. | Added the reasoning (a file never parsed without the tag can't attempt a connection regardless of machine) and noted the literal run as a one-time environment check, not part of the repeatable suite. | Pass B |
| 7 | Minor | Test count ambiguity | The FR-3 composability-hazard description referred to "a third, independent test" when only two tests had been named in the same sentence — ambiguous for implementation. | Named all four tests explicitly (`TestIsolationA/B`, `TestIsolationComposability`, `TestIsolationComposabilityFollowup`). | Pass A |
| 8 | Minor | Adversarial coverage | FR-2's fail-loud check was only exercised for `TEST_DATABASE_URL` genuinely unset, not set to an empty string — a naive presence-only check could pass the unset case while still leaking through on an empty value. | Added the empty-string case to both the Integration layer and the adversarial table. | Pass A |
| 9 | Minor | Layer placement | The FR-3 parallelism check is static analysis (a grep scan), not a `go test`, but was filed under "Unit" alongside real runtime tests. | Moved to the CI infrastructure layer, grouped with this plan's other static checks. | Pass A |
| 10 | Nit | Citation accuracy | "No silent green" was attributed to `architecture-testing.md` FR-6, whose actual text concerns flaky-test retries, not skip-vs-fail semantics — an attribution inherited verbatim from the already-`APPROVED` spec. | Flagged explicitly as inherited rather than independently verified, rather than silently repeated. | Both, independently (A more precisely on the FR-6 mismatch, B on the general caveat-placement pattern) |

## Dimensions checked

- [x] Acceptance-criteria-to-test traceability, item by item (source of finding #1, the Blocking one)
- [x] Testability / mechanism specificity for the plan's highest-risk item (source of finding #2)
- [x] Failure-modes-table coverage, row by row (source of finding #3)
- [x] Fixture/dependency realism (FK relationships checked against the actual domain specs — source of finding #4)
- [x] Acceptance-criteria coverage beyond FR-to-test (AC 9's own cross-document requirement — source of finding #5)
- [x] Literal-vs-substituted proof reasoning (source of finding #6)
- [x] Internal clarity/ambiguity (source of finding #7)
- [x] Adversarial completeness against the spec's own stated mechanism (source of finding #8)
- [x] Layer-taxonomy consistency (source of finding #9)
- [x] Citation accuracy against the specs actually cited (source of finding #10)

## What was not reviewed

Neither agent executed or compiled anything — documents-only review, same discipline as prior reviews in this series. Neither agent verified whether GitHub Actions' branch-protection mechanics work exactly as described for the FR-8 live-gating test (a one-time operational step, not a `go test` case).

## Resolution

All findings fixed in this pass:

- **#1 (AC 6/AC 9 delegation untraceable)** — Fixed: each of the three fake-mechanics bullets (`Clock`, `FS`, `IDGenerator`) now names exactly which sibling plan's test satisfies the corresponding spec AC, including flagging one genuine follow-up (`backend-configuration.md`'s test plan should be checked/updated to cite the `FS` fake explicitly, not yet confirmed).
- **#2 (FR-2 subprocess mechanism)** — Fixed: sentinel-file mechanism named concretely.
- **#3 (`web/`-build failure untested)** — Fixed: added as a second live CI-infrastructure test.
- **#4 (fixture aggregates/FK)** — Fixed: concrete names and FK handling clarified.
- **#5 (AC 9 crosswalk)** — Fixed: added as its own exit-criteria line.
- **#6 (AC 1 substitution reasoning)** — Fixed: reasoning added, literal run scoped as one-time.
- **#7 (ambiguous test count)** — Fixed: all four tests named.
- **#8 (empty-string case)** — Fixed: added to Integration layer and adversarial table.
- **#9 (layer placement)** — Fixed: FR-3 check moved to CI infrastructure.
- **#10 (FR-6 citation)** — Fixed: flagged as inherited, not independently verified.

Test plan status remains `DRAFT` pending maintainer review — this record covers the plan document, not test code that doesn't exist yet. RED phase has not started. This is the sixth and final phase 03 test plan; all six backend-foundation specs now have reviewed, findings-fixed test plans.
