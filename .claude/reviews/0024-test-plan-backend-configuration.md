# Review: Test plan — backend configuration, two independent agents

| | |
|---|---|
| **Subject** | `.claude/test-plans/backend-configuration.md` |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at review time (7 Major, 2 Minor across both passes, 2 findings raised independently by both) — see Resolution below for fixed status |

## Summary

Both agents read the test plan against `backend-configuration.md` (the spec it plans for), the test-plan template, and — independently — related specs it cites (`architecture-backend.md` FR-3's lint-tooling status, `backend-errors-and-logging.md`'s redaction contract) and the phase 03 roadmap README's risk table. Two findings were raised independently by both passes (the FR-1 lint citation being dangling, and the FR-7 dual-run mechanism being underspecified) — strong signal those were real. Each pass also found things the other didn't: pass A caught an untested Observability NFR and an untested failure-path leak case; pass B caught a coverage gap on one specific FR-4 key and a test-plan-level decision (`LOG_LEVEL` case-sensitivity) that should have been kicked back to the spec instead.

## Findings

| # | Severity | Area | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Major | FR-1 structural check | Claimed to extend "the same import-boundary-style lint category `architecture-backend.md` FR-3 established" — but that spec's own Open questions leave the lint tool unchosen; no such established mechanism exists yet to extend. | Reword to name concrete candidate mechanisms (a `go/analysis` pass, a `golangci-lint` custom rule, or an interim grep-based CI script) and stop claiming reuse of something not yet decided. | Both, independently |
| 2 | Major | FR-7 redaction test | "Running it twice… once with only `slog.LogValuer` implemented" hand-waved the mechanism — Go can't toggle which interfaces a type implements at runtime; two engineers would build different fixtures. | Name the concrete second fixture: a test-local `configLogValuerOnlyStub`, never shipped in production, run through the same case first to prove the test can detect the gap it exists to catch. | Both, independently |
| 3 | Major | FR-4 coverage | `HTTP_MAX_BODY_BYTES` — one of FR-4's nine keys — was never named in the type/enum validation bullet, the adversarial table, or the exclusions section. Silent coverage gap. | Add it to the type-validation bullet alongside `DB_POOL_MAX_CONNS`. | Both, independently |
| 4 | Major | Observability NFR | The spec's own Observability requirement (successful load logs which source provided each non-default value) had no corresponding test anywhere in the plan and wasn't listed as excluded. | Add a Unit-layer test asserting a per-key source-name log line, excluding `DATABASE_URL`'s value. | Pass A |
| 5 | Major | FR-7 acceptance criterion | The acceptance criterion requires `DATABASE_URL` never leak "whether successful or failed"; the plan's two FR-7 cases both covered only a successful load's log lines, never a failed `Load`'s returned error string. | Add a case: `Load` fails on an unrelated key while `DATABASE_URL` is set; assert the error string doesn't contain the value. | Pass A |
| 6 | Major | Risk assessment vs. Unit layer | Risk assessment claimed FR-3's test "needs a structural check that a key can't silently acquire context-dependent behavior undetected" — no such check existed in the Unit layer, only the three enumerated category cases. | Add the structural check: a table asserting every FR-4 key belongs to exactly one category, failing if a key appears in two. | Pass A |
| 7 | Major | Adversarial cases | `LOG_LEVEL` case-sensitivity row let the test plan unilaterally pick a behavior the spec doesn't state ("force the decision into the open" via a test) — constitution §1 says spec precedes implementation; a genuine spec gap belongs in a spec amendment and re-review, not a test-plan-level decision. | Reframe the row as unresolved/flagged, not decided; add an explicit exclusion note pointing at the needed spec amendment. | Pass B |
| 8 | Minor | Adversarial cases | Unrecognized-TOML-key "ignored" behavior was asserted as settled, inferred only from FR-6's failure list being exhaustive — same inference pattern as the `DATABASE_URL` non-connection-string row, but unlike that row, not flagged as an assumption. | Add the same "inferred, not explicit in the spec" caveat used for the `DATABASE_URL` row. | Pass B |

Also fixed, Minor, raised by pass A only: the `--config`-unreadable-file test relies on OS permission bits, unreliable when the test process runs as root (common in containers) — added a root-skip guard; and the plan's "fake sources" phrasing rode on the spec's own unresolved `config.Load(sources...)` vs. `config.Load(configPath)` signature inconsistency without flagging it — added to "What is deliberately not tested" as a spec clean-up item.

## Dimensions checked

- [x] Completeness (FR-to-test mapping against the spec directly — all 8 FRs, confirmed correct by pass A before other findings)
- [x] Testability (mechanism specificity — source of findings #1, #2)
- [x] Coverage (every FR-4 key, every FR-6 failure mode — source of finding #3)
- [x] Cross-spec accuracy (citations checked against the specs cited — source of finding #1)
- [x] Internal consistency (risk assessment's own claims vs. what the Unit layer actually contains — source of finding #6)
- [x] Alignment with acceptance criteria's exact wording ("whether successful or failed" — source of finding #5)
- [x] Spec-vs-test-plan boundary discipline (source of finding #7)

## What was not reviewed

Neither agent executed or compiled anything — documents-only review, same discipline as prior reviews in this series. Neither agent evaluated whether the proposed loopback/non-loopback address list (FR-8) is exhaustive beyond what's already table-driven.

## Resolution

All 7 Major and 2 Minor findings fixed in this pass:

- **#1 (dangling lint citation)** — Fixed: FR-1 structural check no longer claims to extend an established mechanism; names concrete candidate implementations and points at phase 03's own still-open lint-tool decision.
- **#2 (FR-7 dual-run mechanism)** — Fixed: names the concrete `configLogValuerOnlyStub` test-local fixture type and what it proves.
- **#3 (`HTTP_MAX_BODY_BYTES` gap)** — Fixed: added to the FR-4 type-validation bullet.
- **#4 (Observability NFR untested)** — Fixed: added a Unit-layer bullet testing per-key source-name logging.
- **#5 (failed-load leak path untested)** — Fixed: added a Unit-layer bullet testing the error string on a failed load with `DATABASE_URL` set.
- **#6 (missing structural check)** — Fixed: added the FR-3 structural check bullet the risk assessment already claimed existed.
- **#7 (`LOG_LEVEL` case-sensitivity decided unilaterally)** — Fixed: adversarial row reframed as unresolved; added to "What is deliberately not tested" with a pointer to the needed spec amendment.
- **#8 (inconsistent assumption-flagging)** — Fixed: unrecognized-TOML-key row now carries the same caveat as the `DATABASE_URL` row.
- Minor (root-privilege test skip) and Minor (signature-inconsistency flag) — both fixed as described above.

Test plan status remains `DRAFT` pending maintainer review — this record covers the plan document, not test code that doesn't exist yet. RED phase has not started. One item (`LOG_LEVEL` case-sensitivity) surfaced a genuine gap in the already-`APPROVED` spec `backend-configuration.md` — noted here for the maintainer; the spec itself is unchanged by this review.
