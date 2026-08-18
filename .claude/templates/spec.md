# Spec: <feature name>

| | |
|---|---|
| **Status** | `DRAFT` |
| **Phase** | `NN-phase-name` |
| **Author** | |
| **Created** | YYYY-MM-DD |
| **Last updated** | YYYY-MM-DD |
| **Supersedes** | — |
| **Reviewed in** | `.claude/reviews/…` (once reviewed) |
| **Design reference** | Canvas(es) consulted (per ADR 0003) and the `.design-reference/ANALYSIS.md` sync date as read at drafting time — e.g. `Alexandryn-Electron.dc.html`, synced 2026-08-13. If no UI surface, `N/A`. If the surface's canvas has no captured screen for this spec as of that sync date, say so explicitly rather than leaving this blank. |

<!--
Status moves DRAFT → REVIEWED → APPROVED → IMPLEMENTED → VERIFIED and never
backwards. If reality contradicts an APPROVED spec, amend the spec and note the
change here — do not let the code become the new truth silently.
-->

## Context

Why this exists. What the user is trying to do, and what the system looks like
today. Someone who joins in a year should understand the situation from this
paragraph alone.

## Problem

The specific thing that is wrong or missing. One or two sentences.

## Goals

- What must be true when this is done

## Non-goals

- What this deliberately does not do, and where that work lives instead

Non-goals are as load-bearing as goals. They are what stops scope creep from
being litigated in a code review.

## User stories

- As a **<who>**, I want **<what>**, so that **<why>**.

## Functional requirements

Numbered, testable, unambiguous. Use RFC 2119 keywords.

- **FR-1** The system MUST …
- **FR-2** The system MUST NOT …
- **FR-3** When <condition>, the system SHALL …

If a requirement can't be turned into a test, it isn't specific enough yet.

## Non-functional requirements

- **Performance** — budgets with numbers, not adjectives
- **Security** — see the dedicated section below
- **Accessibility** — keyboard path, accessible names, announcements
- **Reliability** — what happens on restart, on partial failure
- **Observability** — what is logged, measured, and correlated

## Domain model

Entities, value objects, and events this touches. Which are new, which change.
Where the boundary sits between this and the metadata/source/library split
(constitution §3).

## API and contracts

Endpoints, message schemas, IPC surface. Request and response shapes, error
codes, versioning and compatibility implications.

## State transitions

The states this feature can be in and the legal moves between them. Diagram if
it clarifies. Name the illegal transitions explicitly — those become tests.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| | | | |

Cover: unavailable, slow, malformed, partial, duplicated, enormous, and
unauthorised. Degraded behaviour is a designed behaviour, not an accident.

## Security considerations

Threats and concrete mitigations. Walk the checklist that applies:

- Trust boundary crossed, and what's assumed on each side
- Input validation: shape, size limit, timeout
- Authentication and authorisation
- Injection, traversal, SSRF, XSS, CSRF
- Resource exhaustion and race conditions
- Secret handling and what must never be logged

"Validated" is not a mitigation. Say what is checked and what happens when the
check fails.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | |
| Integration | |
| Contract | |
| E2E | |
| Accessibility | |

Name the tests that must fail before implementation begins.

## Acceptance criteria

- [ ] Measurable, checkable statements
- [ ] One per functional requirement, at minimum

## Open questions

Anything unresolved, with who or what would resolve it. An empty list here on a
first draft usually means the draft isn't finished.

## References

Related specs, ADRs, audits, design screens.
