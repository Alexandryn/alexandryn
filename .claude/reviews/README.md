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
