# Reviews

Records of specification and change reviews. Start from
[`../templates/review.md`](../templates/review.md).

A review recorded here is a gate, not a comment thread. It exists so that
"this was approved" is a checkable fact months later, and so the reasoning
behind an approval survives the pull request UI.

## What gets a recorded review

- Every specification, before it can reach `APPROVED`
- Every architecture decision record, before it reaches `Accepted`
- Changes that alter a trust boundary, a domain boundary, or a public contract

Ordinary pull requests are reviewed in GitHub and don't need a file here.

## Verdicts

| Verdict | Means |
|---|---|
| Approved | Proceed. |
| Approved with changes | Proceed once the listed changes are made; no second review needed. |
| Needs rework | Substantial gaps. Revise and resubmit for a full review. |
| Rejected | The approach is wrong, not just incomplete. The record explains why. |

## Reviewing well

Look for contradictions before style. A spec that disagrees with the
constitution, with an ADR, or with itself is a more expensive problem than an
awkward sentence.

Ask what two different engineers would build from the same document. If the
answers differ, the spec is ambiguous regardless of how well written it is.

Mark the dimension checklist honestly. An unchecked box is useful information.
A falsely checked one is worse than no checklist at all, because it stops the
next person from looking.

## Naming

`NNNN-<subject>.md`, sequential.

## Index

| Review | Subject | Date | Verdict |
|---|---|---|---|
| [0001](0001-adr-record-architecture-decisions.md) | ADR 0001 — Record architecture decisions | 2026-08-13 | Approved |
| [0002](0002-adr-design-canvas-split.md) | ADR 0003 — Design canvas split | 2026-08-13 | Approved |
| [0003](0003-adr-persistence-engine-postgresql.md) | ADR 0004 — Persistence engine PostgreSQL | 2026-08-13 | Approved |
| [0004](0004-spec-architecture-system.md) | Spec — `architecture-system.md` | 2026-08-13 | Approved with changes (all findings fixed) |
| [0005](0005-adr-process-model.md) | ADR 0005 — Process model, prototype-backed | 2026-08-13 | Approved |
| [0006](0006-adr-docs-and-website-repos.md) | ADR 0006 — Docs and website repos | 2026-08-13 | Approved |
| [0007](0007-spec-architecture-desktop-host.md) | Spec — `architecture-desktop-host.md` | 2026-08-14 | Approved with changes (findings 2–7 fixed, finding 1 open by design), self-reviewed — needs independent read |
| [0008](0008-adr-postgres-provisioning.md) | ADR 0007 — Production PostgreSQL bundled and managed | 2026-08-14 | Pending, self-reviewed — needs independent read |
| [0009](0009-spec-architecture-persistence.md) | Spec — `architecture-persistence.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0010](0010-spec-architecture-contracts.md) | Spec — `architecture-contracts.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0011](0011-spec-architecture-backend.md) | Spec — `architecture-backend.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0012](0012-spec-architecture-frontend.md) | Spec — `architecture-frontend.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0013](0013-spec-architecture-testing.md) | Spec — `architecture-testing.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0014](0014-review-skill-first-run.md) | `/review` skill's first run, two independent agent reviews | 2026-08-14 | Approved with changes, genuinely independent (not self-reviewed) |
| [0015](0015-adr-monorepo-layout.md) | ADR 0008 — Monorepo layout, two independent agent reviews | 2026-08-14 | Needs rework at review time (Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0016](0016-spec-domain-bibliographic.md) | Spec — `domain-bibliographic.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0017](0017-spec-domain-library.md) | Spec — `domain-library.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0018](0018-spec-domain-source.md) | Spec — `domain-source.md` | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0019](0019-spec-domain-reading.md) | Spec — `domain-reading.md`, ADR 0009, ADR 0010 | 2026-08-14 | Approved with changes (all findings fixed), self-reviewed — needs independent read |
| [0020](0020-spec-domain-events.md) | Spec — `domain-events.md` | 2026-08-14 | Approved with changes (both findings fixed), self-reviewed — needs independent read |
| [0021](0021-phase02-cross-spec-review.md) | Phase 02 — all five specs, cross-spec, two independent agents | 2026-08-14 | Needs rework at review time (3 Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0022](0022-phase03-cross-spec-review.md) | Phase 03 — all six specs and ADRs 0011–0013, cross-spec, two independent agents | 2026-08-14 | Needs rework at review time (2 Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0023](0023-test-plan-backend-service-lifecycle.md) | Test plan — `backend-service-lifecycle.md`, two independent agents | 2026-08-14 | Needs rework at review time (1 Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0024](0024-test-plan-backend-configuration.md) | Test plan — `backend-configuration.md`, two independent agents | 2026-08-14 | Needs rework at review time (7 Major, now fixed), genuinely independent (not self-reviewed) |
| [0025](0025-spec-amendment-backend-configuration-log-level.md) | Spec amendment — `backend-configuration.md`, `LOG_LEVEL` case-sensitivity | 2026-08-14 | Approved with changes, self-reviewed — needs independent read |
| [0026](0026-test-plan-backend-errors-and-logging.md) | Test plan — `backend-errors-and-logging.md`, two independent agents | 2026-08-14 | Needs rework at review time (1 Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0027](0027-test-plan-backend-http-transport.md) | Test plan — `backend-http-transport.md`, two independent agents | 2026-08-14 | Needs rework at review time (6 Major, now fixed), genuinely independent (not self-reviewed) |
| [0028](0028-spec-amendment-dsn-redaction.md) | Spec amendment — DSN redaction in startup/migration/config-parse error logging, `/security-review` finding ([`audit 0001`](../audits/0001-phase03-backend-specs.md)) | 2026-08-14 | Approved with changes, self-reviewed — needs independent read |
| [0029](0029-test-plan-backend-persistence.md) | Test plan — `backend-persistence.md`, two independent agents | 2026-08-14 | Needs rework at review time (1 Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0030](0030-test-plan-backend-test-harness.md) | Test plan — `backend-test-harness.md`, two independent agents | 2026-08-14 | Needs rework at review time (1 Blocking, confirmed independently by both, now fixed), genuinely independent (not self-reviewed) |
| [0031](0031-phase04-cross-spec-review.md) | Phase 04 — all six frontend-foundation specs, cross-spec, two independent agents | 2026-08-14 | Needs rework at review time (1 Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0032](0032-spec-amendments-phase05-cross-phase-findings.md) | Phase 05 — three desktop-host specs, cross-spec, two independent agents; plus cross-phase amendments to `backend-http-transport.md` and `frontend-design-tokens.md` | 2026-08-14 | Needs rework at review time (3 Blocking, now fixed), genuinely independent (not self-reviewed) |
| [0033](0033-phase06-cross-spec-review.md) | Phase 06 — three library specs, cross-spec, two independent agents | 2026-08-14 | Needs rework at review time (1 Blocking, confirmed independently by both, now fixed), genuinely independent (not self-reviewed) |
| [0034](0034-phase07-cross-spec-review.md) | Phase 07 — three metadata specs, cross-spec, two independent agents; plus a cross-phase amendment to `backend-configuration.md` | 2026-08-15 | Needs rework at review time (1 Blocking, confirmed independently by both, now fixed), genuinely independent (not self-reviewed) |
| [0035](0035-phase08-cross-spec-review.md) | Phase 08 — two source specs, cross-spec, two independent agents; plus a cross-phase amendment to `desktop-host-ipc-surface.md` | 2026-08-15 | Needs rework at review time (2 Blocking, 8 Major with 2 confirmed independently by both, now fixed), genuinely independent (not self-reviewed) |
| [0036](0036-phase09-cross-spec-review.md) | Phase 09 — ADR 0014 and `backend-job-queue.md`, two independent agents; plus a cross-phase amendment to `backend-service-lifecycle.md` | 2026-08-15 | Needs rework at review time (1 Blocking, confirmed independently by both, now fixed), genuinely independent (not self-reviewed) |
| [0037](0037-phase10-cross-spec-review.md) | Phase 10 — three import specs, cross-spec, two independent agents; plus a cross-phase amendment to `backend-source-adapter.md` | 2026-08-15 | Needs rework at review time (7 Blocking, complementary not overlapping across passes, now fixed), genuinely independent (not self-reviewed) |
