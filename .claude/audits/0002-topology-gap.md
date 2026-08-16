# Spec-gap audit: container/Compose deployment topology

| | |
|---|---|
| **Scope** | Every approved spec and Accepted ADR touching process topology, Postgres provisioning, or configuration delivery, checked against the container/Compose deployment target confirmed in `.claude/decisions/0015-container-topology.md` |
| **Auditor** | Claude (Sonnet 5), for review by Luann Moreira |
| **Date** | 2026-08-16 |
| **Commit** | Pre-existing tree at `da8b431` (phase 11 spec approval), no code |
| **Verdict** | Findings open — 9 documents require amendment or addendum, 4 of them already `APPROVED` (implementation-ready) and must move back through the review gate before their areas can be implemented |

This is not a security audit — no vulnerability is being hunted. It reuses
this template's shape (per the request that produced it) because the
underlying discipline is the same one `.claude/templates/audit.md` exists
to enforce: name every finding concretely, rate its real severity honestly,
and don't close by going quiet. Read "Adversarial questions" below as "spec-
coverage questions," and "Findings" as amendment items, not vulnerabilities.

## Scope and method

Read-only documentation review. Every spec and ADR named in the request
(`architecture-backend.md`, `backend-service-lifecycle.md`,
`backend-persistence.md`, `backend-test-harness.md`,
`architecture-contracts.md`, `frontend-tooling.md`, ADR 0008, ADR 0004, the
`supabase/` local-dev scaffold) was read in full, plus `architecture-system.md`,
`backend-configuration.md`, ADR 0005, ADR 0007, `architecture-desktop-host.md`,
and `architecture-testing.md`, which surfaced as implicated during the read.
No code exists in this repository to inspect (confirmed in the prior audit
pass, 2026-08-16) — this review is entirely against the 34 approved/reviewed
specs, 14 ADRs, and their cross-references.

## Boundaries examined

Not trust boundaries (no attacker model applies to a documentation gap) —
the **topology boundary** each document assumes, and whether the
container/Compose target crosses it cleanly or is foreclosed by it.

| Document | Topology it assumes | Container target crosses cleanly? |
|---|---|---|
| `architecture-system.md` FR-1/FR-2 | Exactly 3 processes (4 on macOS), Electron always present, Postgres always Go-server-spawned | No — the enumeration is exhaustive and excludes the container target by construction |
| ADR 0005 | Go server is always Electron's spawned child, never a system service | No, for the container target specifically; yes, still, for the Electron target |
| ADR 0007 | Postgres is always bundled and spawned by the Go server; external/user-managed Postgres explicitly rejected as "not a v1 goal" | No, for the container target; yes, still, for the Electron target |
| `backend-configuration.md` FR-4, Security considerations | `DATABASE_URL` is a dev/CI/test-only signal, never present in production | No — this is the exact mechanism the container target needs as its normal production path |
| `backend-persistence.md` FR-5, Security considerations | Same as above, independently stated | Same as above |
| `architecture-backend.md` FR-5 | "The config file... is how a packaged instance is always configured" | No — treats one config-delivery mechanism as exhaustive |
| `backend-service-lifecycle.md` FR-1 step 5 | Already implements the DATABASE_URL-present branch, but labels it dev/CI/test only | Yes, mechanically — needs only a re-scoping, not new logic |
| ADR 0008, `architecture-contracts.md`, `frontend-tooling.md` | `go:embed`, one binary, one wire contract, one build pipeline | Yes — none of these depend on which process runs the resulting binary |
| `backend-test-harness.md` FR-7/FR-8 | Tests the Electron-hosted bundled-spawn path; no equivalent for Compose | N/A — silent gap, not a contradiction |
| `supabase/` scaffold, ADR 0004 addendum | Local dev = Supabase CLI's full stack (API gateway, Studio, Realtime, Storage, Auth) | N/A — no relationship asserted either way to a lean Compose file |

## Spec-coverage questions asked

- What does a contributor building against `architecture-system.md` alone
  conclude about whether a headless, Electron-less deployment is legal?
  **Answer: that it isn't** — FR-1's "MUST consist of" enumeration and "never
  spawned by anything other than the Go server" leave no room for it.
- What does a contributor building `backend-persistence.md` FR-5 as written
  conclude happens if `DATABASE_URL` is set when Alexandryn is *not* being
  developed or tested? **Answer: that this must not happen** — the spec
  says the bypass path "MUST NOT be reachable" outside dev/CI/test.
- Does any spec currently test, or even describe testing, the container
  target end to end (build image, compose up, backend reaches Postgres,
  passes readiness)? **Answer: no.**
- Does local development currently produce anything resembling the
  container target's own Postgres provisioning? **Answer: no** — Supabase's
  stack is a superset with a materially different shape and different
  service composition.
- Do any of the wire-level specs (`architecture-contracts.md`,
  `frontend-tooling.md`) implicitly assume Electron? **Answer: no** —
  verified by full read; both are genuinely topology-independent.

## Findings

Ordered by required amendment sequence — each depends on the ones above it
either being settled first (because it cites them by name) or being drafted
in the same review batch (because both amend the same upstream fact).

| ID | Document | Status today | Change required | Invalidates approval? |
|---|---|---|---|---|
| A-02-01 | `.claude/decisions/0015-container-topology.md` | Proposed (this pass) | Needs `Accepted` before anything below is drafted against it | New document, not applicable |
| A-02-02 | `architecture-system.md` | `REVIEWED` (already amended once, by ADR 0007) | Second amendment: FR-1 (drop "MUST consist of three... or four" as exhaustive; state it as the Electron-hosted target's topology specifically), FR-2 (same), Security considerations (drop "it is not how a shipped instance gets its database" — a shipped instance now has two valid shapes) | Yes — must be re-confirmed by the maintainer before `architecture-backend.md`/`architecture-desktop-host.md` are treated as final, per its own Acceptance criteria |
| A-02-03 | ADR 0005 | Accepted | Addendum scoping "two processes, never a system service" to the Electron-hosted target explicitly | Recommend review — narrows an Accepted finding's applicability, more than additive |
| A-02-04 | ADR 0007 | Accepted | Addendum scoping "bundled and managed, never user-configured" to the Electron-hosted target; explicitly un-rejects Option B's shape for the container target only | Recommend review — same reasoning, stronger case (Option B was explicitly rejected, not merely unaddressed) |
| A-02-05 | ADR 0008 | Accepted | Light addendum naming the new `Dockerfile`/`docker-compose.yml` artifact alongside `web/`/`electron/`; no FR changes | Recommend review, low risk — no existing decision is contradicted |
| A-02-06 | `architecture-backend.md` | `REVIEWED` | FR-5 amendment: name the container target's config delivery (Compose-injected environment/`.env`) as a third, equally legitimate source, not a dev-only override | Not yet `APPROVED` — amend before first approval, cheaper than the specs below |
| A-02-07 | `backend-configuration.md` | `APPROVED` (already carries 3 pending post-approval amendments) | FR-4 table: `DATABASE_URL` row's "Source of the requirement" column and Security considerations' "never read in production" line both need the container-target case named | **Yes** — moves back to `REVIEWED` per constitution §1 and `specs/README.md`'s own rule; this is the fourth amendment this spec has needed |
| A-02-08 | `backend-persistence.md` | `APPROVED` | FR-5 and Security considerations: same re-scoping, this spec's independent statement of the same policy | **Yes** — moves back to `REVIEWED` |
| A-02-09 | `backend-service-lifecycle.md` | `APPROVED` | FR-1 step 5's own prose already implements the needed branch — only the surrounding Context/Non-goals language needs re-scoping; Observability NFR needs a paragraph for the no-Electron-log-stream case | **Yes**, technically — even a small amendment to an `APPROVED` spec moves it back per the stated rule |
| A-02-10 | `backend-test-harness.md` | `APPROVED` | New FR (or FR-7 extension): a dedicated Compose-target test — build the image, `docker compose up`, assert the backend reaches `Ready` against the sibling Postgres container | **Yes** — moves back to `REVIEWED` |
| A-02-11 | New spec (not yet named — suggest `deployment-container-packaging.md`) | Does not exist | Image build strategy (multi-stage Dockerfile: build `web/`, then `go build ./cmd/server`, matching ADR 0008's existing ordering), the `docker-compose.yml` itself (service names, network, named volume, healthcheck-based `depends_on`), and which phase owns it (currently nothing in the roadmap names one — phase 99's outline mentions "Docker/Compose self-hosting stack" as a release-packaging line item, not a spec) | New document — enters the gate at `DRAFT`, same as any new spec |
| A-02-12 | `CONTRIBUTING.md` / possible ADR 0004 addendum | Current (Supabase-only local dev) | Local-dev parity decision: whether local dev adopts the new Compose file, keeps Supabase for Electron-target work only, or runs both | **Not yet draftable** — this is a decision only the maintainer can make (named in ADR 0015's Consequences/Neutral as unresolved, not a mechanical amendment) |

### A-02-02 — `architecture-system.md`'s exhaustive process enumeration

**Component:** `.claude/specs/architecture-system.md` FR-1 (lines 94-107),
FR-2 (lines 108-115), Security considerations (lines 283-291)

**Description** — FR-1 states a running instance "MUST consist of" exactly
three processes (four on macOS), all descended from Electron, and that
"Postgres is never spawned by anything other than the Go server." Security
considerations states directly that the `DATABASE_URL`-external-config path
"is not how a shipped instance gets its database." Both are true today and
false the moment the container target ships.

**Impact** — every spec that inherits this spec's process model
(`architecture-backend.md`, `backend-configuration.md`,
`backend-persistence.md`, `backend-service-lifecycle.md`) inherits the same
exhaustiveness claim, transitively, per this spec's own Context section
("they should not contradict it... if one needs to, this spec was wrong").
Fixing this spec first is what unblocks every amendment below it.

**Preconditions** — none; this is a documentation defect, not an exploit.

**Recommendation** — amend FR-1/FR-2 to state two legal topologies by name
(Electron-hosted: as currently written; container-hosted: Go server +
sibling Postgres container, no Electron), amend Security considerations to
drop the "not how a shipped instance gets its database" absolute, and record
the change the same way ADR 0007's amendment was recorded — in the spec's
own header note, moved back to `REVIEWED`.

**Resolution** — not yet applied; this audit is planning-only per the
request that produced it.

### A-02-07 — `backend-configuration.md`'s `DATABASE_URL` is a dev-only signal

**Component:** `.claude/specs/backend-configuration.md` FR-4 (line 108),
Security considerations (lines 255-259)

**Description** — the FR-4 table marks `DATABASE_URL` optional with "absence
is a meaningful signal," sourced to "ADR 0004's addendum (Supabase dev
stack)" only. Security considerations restates `architecture-system.md`
verbatim: "`DATABASE_URL` never read in production... this spec's config
surface exists for the dev/CI path."

**Impact** — the exact mechanism the container target needs (an externally-
supplied `DATABASE_URL` reaching a sibling Postgres container) is the thing
this spec's own security reasoning currently rules out for production use.
Implementing the container target against this spec as written would either
require ignoring the spec's own security section or misclassifying the
container target as "development" to satisfy it — neither is acceptable.

**Impact if left unfixed** — a future contributor reading only this spec
would reasonably refuse to wire `DATABASE_URL` into a production path,
correctly, because the spec tells them not to; the fix has to happen here,
not be worked around downstream.

**Recommendation** — add a third row/branch to FR-4's `DATABASE_URL`
treatment distinguishing "container target: legitimate production source"
from "Electron target: dev/CI/test only, never present"; rewrite the
Security considerations paragraph to state both cases instead of one
absolute.

**Resolution** — not yet applied.

## What was not examined

Frontend specs (`frontend-*` beyond `frontend-tooling.md`), desktop-host
specs beyond the process-model cross-references already covered via
`architecture-system.md`, and every domain spec (`domain-*`) — none of
these were named in the request and a first-pass check of their contents
during the parent audit found no topology dependency; not re-verified line
by line here. The actual Dockerfile/compose-file content is not designed in
this pass — planning only, per the request. Whether the container target
should bind `BIND_ADDRESS` to `0.0.0.0` inside its own container namespace
without violating constitution §6's loopback-until-phase-12/13 rule is
named as an open question in ADR 0015 but not resolved here — it needs its
own small design pass, likely as part of A-02-11's new spec, not decided by
implication in this audit.
