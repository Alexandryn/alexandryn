# Review: ADR 0015 — Container topology

| | |
|---|---|
| **Subject** | `.claude/decisions/0015-container-topology.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16 |
| **Verdict** | Approved with changes (maintainer-directed changes folded in before this review) |

## Summary

Records that Alexandryn ships a second deployment target — the backend
containerized, composed with PostgreSQL via Docker Compose — alongside the
existing Electron-hosted target, with no new backend service and no change
to the domain, contract, or error-taxonomy boundaries. The maintainer
confirmed the topology reading (one backend, containerized) directly, and
separately confirmed the database-connection design (single `DATABASE_URL`,
a Compose profile gating the bundled Postgres container) before this ADR
was finalized. Both are folded into the ADR text under review here, not
left as follow-up.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Relationship to prior ADRs | An earlier draft handled ADR 0005/0007's rejected options via a bare "needs a scope-narrowing addendum" assertion, with no argument for why an addendum (not a supersession) was the correct instrument | Fixed — added a "Relationship to ADR 0005 and ADR 0007" section walking each rejected option through what was rejected, why, what changed, and why it's read as correct now, with the competing (supersession) reading stated explicitly as unresolved rather than dismissed |
| 2 | Minor | Database connection | First draft left "what does DB config look like for this target" as an open question rather than a decision | Fixed — folded in as a resolved design point (single `DATABASE_URL`, Compose `profiles` gating the bundled instance), with the constitution §6 boundary explicitly stated as untouched by it |

## Dimensions checked

- [x] **Completeness** — covers topology, database connection, and Electron packaging's fate, all three the request named
- [x] **Ambiguity** — the addendum-vs-supersession question is stated as genuinely open rather than resolved by assertion; the honest position is that this ADR cannot settle it alone
- [x] **Architecture** — confirmed no new backend service, no change to `internal/domain` boundary, no split configuration surface
- [x] **Domain correctness** — not applicable; this ADR doesn't touch the Alexandryn/metadata/source split
- [x] **Security** — confirms the loopback/authentication gate (constitution §6) is unaffected by deployment location; flags the container-network-namespace nuance in `BIND_ADDRESS`'s loopback check as still open, not silently assumed solved
- [x] **Testability** — the amendment plan (`.claude/audits/0002-topology-gap.md`) already names the missing test coverage this ADR's decision requires
- [ ] **Accessibility** — not applicable, no UI
- [ ] **UX and copy** — not applicable, no user-facing text
- [x] **Observability** — flagged in the amendment plan as needing an `architecture-system.md` amendment, not solved here
- [x] **Maintainability** — the addendum-not-supersession framing keeps the existing ADR history intact and legible rather than rewriting it
- [x] **Evolution** — explicitly additive to the Electron target, reversal cost stated asymmetrically and honestly

## Contradictions and gaps

The one substantive gap this ADR itself names and does not resolve: whether
ADR 0005's and ADR 0007's rejections were meant as permanent (making this a
reversal, needing supersession) or deliberately deferred (making this an
extension, addendum is correct). The ADR states both readings with their
textual evidence and defers the final call to the maintainer rather than
picking one silently. Proceeding with the addendum instrument for both,
per the maintainer's approval of this ADR as drafted.

## What I did not review

The actual `Dockerfile` and `docker-compose.yml` content — not designed in
this ADR, planned as a new spec (`.claude/audits/0002-topology-gap.md`
A-02-11). Whether "Docker for simplifying things" implies any specific base
image, multi-stage build layout, or registry — none of that is decided
here and none of it was asked to be.
