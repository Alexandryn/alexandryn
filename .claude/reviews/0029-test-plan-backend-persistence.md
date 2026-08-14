# Review: Test plan — backend persistence implementation, two independent agents

| | |
|---|---|
| **Subject** | `.claude/test-plans/backend-persistence.md` |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at review time (1 Blocking, 4 Major across both passes, 4 Minor) — see Resolution below for fixed status |

## Summary

Both agents read the test plan against `backend-persistence.md` (the spec it plans for, including its recent FR-6 DSN-redaction amendment), the test-plan template, and — independently — related specs it cites (`backend-test-harness.md`, `architecture-testing.md` FR-3, phase 03's roadmap README). No finding overlapped between the two passes, which is itself informative: pass A's strength was mechanism-level scrutiny (does the proposed test actually prove what it claims, or could a wrong implementation pass it by coincidence); pass B's strength was coverage-completeness (checking every Failure-modes row and Acceptance-criteria item against the plan line by line). Pass A's Blocking finding is the most consequential in the batch — the SQL-injection adversarial test, as originally written, provably could not distinguish real parameterization from a disciplined-but-wrong hand-escaping implementation.

## Findings

| # | Severity | Area | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | FR-3 adversarial proof | The hostile-payload round-trip test proved only that the specific payload chosen didn't break — a hand-rolled-escaping implementation (exactly the FR-3-violating pattern the spec prohibits) would pass the same assertions by coincidence, not because parameterization is in use. | Instrument the query path directly: capture the literal SQL text and argument list `pgx` sends via a `pgx.QueryTracer`, assert the SQL contains a placeholder at the hostile field's position with the value only in the separate argument list. | Pass A |
| 2 | Major | Adversarial coverage | Two adversarial-table rows (no-context-deadline pool block, concurrent unique-constraint race) had no corresponding test anywhere in Unit/Integration, contradicting the plan's own exit criteria claim that every adversarial case has a test. | Added both as concrete Integration-layer tests (bounded-wait-then-release pattern; two-goroutine race against a real constraint). | Pass A |
| 3 | Major | FR-4 mechanism specificity | The cross-connection atomicity test said the operation "is driven to fail partway through" without naming the mechanism — against a real database this could mean a constraint violation, a context cancellation, or an injected error, each exercising a different code path; two engineers would build different tests. | Named the concrete trigger: fixture data engineered to violate a real constraint on the merge's second write specifically. | Pass A |
| 4 | Major | Failure modes coverage | The spec's "production spawn step fails" failure-mode row (corrupted data directory, disk full) had no test anywhere and wasn't listed as excluded either — a silent gap, not an honest one. | Added a Unit-layer test with a fake command runner forcing the failure, asserting clean startup failure and no automatic recovery attempt. | Pass B |
| 5 | Major | Observability NFR | The spec's Observability requirement (migration success/failure logged at `info`/`error`, distinguishable from a routine connection failure) had no corresponding test. | Added an Integration-layer test asserting both the level split and a distinguishing field/fragment between migration and connection failures. | Pass B |
| 6 | Minor | Fixture citation | Claimed the DSN-shaped fixture "mirrors" one used in `backend-errors-and-logging.md`'s test plan — checked directly, that plan uses a different, generic sensitive-value fixture type, never a DSN-shaped string. Factually wrong citation. | Corrected to cite `backend-configuration.md`'s matching fixture only, and noted the `backend-errors-and-logging.md` difference explicitly rather than silently dropping the (inaccurate) claim. | Pass A |
| 7 | Minor | FR-5 security claim | The spec's Security considerations claim ("no Electron-supplied `DATABASE_URL`" reaches production) was asserted by the spec but the plan's FR-5 test only proves branch selection given a fake config, not the underlying claim, which really belongs to `architecture-system.md`'s process-spawn contract. | Added an explicit note attributing that specific guarantee's test coverage to the owning spec, not claiming it here. | Pass B |
| 8 | Minor | macOS CI dependency | The plan's "untestable in this authoring environment, not permanently untestable" framing didn't surface that no macOS CI job is actually named anywhere in `backend-test-harness.md` FR-8 or phase 03's own roadmap exit criteria — without one being added, FR-8 has no automated verification path at all, not just none from this plan. | Elevated in Risk assessment: flagged as a phase-level infrastructure gap to raise at the phase's security-audit gate, not a test-plan footnote. | Pass B |
| 9 | Minor | Acceptance criteria mapping | AC6 ("every FR maps to a line in phase 03's own exit criteria") was silently absent from the plan, unlike FR-2's explicit "no runtime test needed, here's why" treatment. | Added the same explicit non-test-but-satisfied-elsewhere note. | Pass B |

## Dimensions checked

- [x] Mechanism soundness — does the proposed test actually prove the claim, or pass by coincidence (source of finding #1, the Blocking one)
- [x] Adversarial-table-to-test traceability (source of finding #2)
- [x] Testability/mechanism specificity (source of finding #3)
- [x] Failure-modes-table coverage, row by row (source of finding #4)
- [x] Non-functional-requirement coverage (source of finding #5)
- [x] Cross-spec citation accuracy, checked against the actual cited documents, not just internal consistency (source of finding #6)
- [x] Claim-vs-actual-test-scope discipline (source of finding #7)
- [x] Infrastructure/CI dependencies implied but not verified to exist (source of finding #8)
- [x] Acceptance-criteria-to-plan mapping, item by item (source of finding #9)

## What was not reviewed

Neither agent executed or compiled anything — documents-only review, same discipline as prior reviews in this series. Neither agent evaluated whether the specific fixture engineering described for finding #3's fix (deleting a referenced `Edition` row between the merge's two writes) is the most natural way to trigger that constraint violation versus an alternative.

## Resolution

All 1 Blocking, 4 Major, and 4 Minor findings fixed in this pass:

- **#1 (SQL-injection test provable by coincidence)** — Fixed: FR-3's Integration test now instruments the actual query path via a tracer, asserting placeholder usage rather than only the outcome.
- **#2 (two untested adversarial rows)** — Fixed: both added as concrete Integration-layer tests; adversarial table rows updated to point at them.
- **#3 (FR-4 mechanism vague)** — Fixed: named the concrete constraint-violation trigger.
- **#4 (production spawn failure untested)** — Fixed: added Unit-layer test.
- **#5 (Observability NFR untested)** — Fixed: added Integration-layer test.
- **#6 (wrong fixture citation)** — Fixed: corrected.
- **#7 (FR-5 security claim over-scoped)** — Fixed: added attribution note.
- **#8 (macOS CI dependency underplayed)** — Fixed: elevated to a phase-level flag in Risk assessment.
- **#9 (AC6 silently unmapped)** — Fixed: added explicit note.

Test plan status remains `DRAFT` pending maintainer review — this record covers the plan document, not test code that doesn't exist yet. RED phase has not started. Finding #8 surfaces a real, unresolved phase-level question (does phase 03's CI need a macOS runner) worth raising with the maintainer directly, not just noting in this document.
