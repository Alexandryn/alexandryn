# Spec: API contract architecture

| | |
|---|---|
| **Status** | `REVIEWED` (self, approved with changes) |
| **Phase** | `01-architecture` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`.claude/reviews/0010-spec-architecture-contracts.md`](../reviews/0010-spec-architecture-contracts.md) — Approved with changes, all findings fixed; self-reviewed, independent read still pending |

## Context

ADR 0006 fixed the contract format as OpenAPI and fixed that `alexandryn`
carries the spec file itself, not prose API documentation. It didn't decide
where the file lives, whether it's hand-written or generated from Go code,
how versioning works, or how conformance gets tested. Phase 01's own
open-questions list still has "ownership/versioning/design" marked open for
this spec specifically.

## Problem

Nothing says: where the OpenAPI file lives, whether the spec or the Go code
is the authored source of truth, what a version number means when the
client is never deployed separately from the server that serves it, what an
error response looks like at the wire level, or how a contract violation
gets caught before it ships.

## Goals

- Decide spec-first vs. code-first, and justify it against this project's
  existing spec-before-code discipline rather than treating it as a
  tooling preference
- Fix the file's location, versioned with the code (ADR 0006)
- Define a versioning scheme that accounts for the fact this API has no
  independently-deployed client — the web UI is served fresh by the Go
  server on every load (`architecture-system.md` FR-6), so there's no
  client/server skew the way a public API has to survive
- Fix the error response shape (not the taxonomy — phase 03's job)
- Define how contract conformance is tested, not just documented

## Non-goals

- The actual endpoint list — arrives with each feature phase (06 onward),
  not invented here
- Event/realtime schema (WebSocket, SSE, or similar) for phase 14's
  cross-device sync — genuinely undesigned, not guessed at; a v1 concern is
  request/response HTTP only
- Error taxonomy (which codes exist, what they mean) — phase 03's
  `backend-errors-and-logging.md`; this spec fixes the response *shape* the
  taxonomy fills in
- The Electron IPC surface — a separate boundary
  (`architecture-desktop-host.md`), not an HTTP contract at all
- Authentication scheme specifics — phase 12 designs the credential; this
  spec fixes where an auth header/token would go in the contract shape,
  not what it contains

## User stories

- As **phase 03**, I want to implement Go handlers against a contract that
  already exists, not invent request/response shapes ad hoc per endpoint.
- As **phase 06 (library, the first real feature)**, I want to know where
  to add its first endpoints to the contract, and what "done" looks like
  for contract conformance.
- As **a future external integrator** (if one ever exists), I want a
  versioning scheme that means something, even though today's only
  consumer is this project's own always-fresh web UI.

## Functional requirements

- **FR-1** The OpenAPI specification MUST be hand-authored and versioned in
  this repository (ADR 0006) as the source of truth — **spec-first, not
  generated from Go code annotations.** This matches the project's own
  discipline everywhere else (spec before implementation, constitution §1);
  generating the contract from code would make the code the de facto spec,
  exactly the failure mode constitution §1 already prohibits for behavior
  generally.
- **FR-2** The OpenAPI file MUST live at `api/openapi.yaml` (or `.json`) at
  the repository root, reviewed the same way any other spec-adjacent
  artifact is — a change to it is a change to the contract, not an
  implementation detail. Confirmed, not provisional: ADR 0008 (monorepo
  layout) fixes the repository root as a stable location, resolving what
  this FR originally flagged as pending.
- **FR-3** Go handlers MUST be verified against the OpenAPI spec by an
  automated contract test — the spec and the implementation MUST NOT be
  allowed to drift silently. The specific tool (schema validation against
  real responses, or generated-server-stub conformance) is phase 03's to
  pick; this spec only requires that drift is a test failure, not a
  documentation-review catch.
- **FR-4** Versioning is path-based (`/api/v1/...`), and its purpose here is
  narrower than a public API's, not absent: the web UI is fetched fresh
  from the Go server on every *new* page load, so there's no long-lived
  installed client drifting out of sync the way an app-store app does.
  But an already-open LAN browser tab (a phone, a tablet) keeps running
  its already-loaded JS across a host restart or update — if that update
  ships a breaking API change, that tab's stale frontend calling the new
  server *is* skew, just bounded to "however long a tab stays open across
  an update" rather than months. The version exists for that case, for a
  deliberate deprecation-able breaking change, and for any future external
  integration.
- **FR-5** Every error response MUST share one shape across every endpoint:
  a machine-readable code, a human-readable message meeting constitution
  §11's bar (specific, no apology, says what to do next), and a
  correlation ID matching the one in the server's structured logs
  (`architecture-system.md`'s Observability section). The taxonomy of
  which codes exist is phase 03's; this shape is fixed here so no endpoint
  invents its own error format.
- **FR-6** The contract MUST reserve a place for an auth credential (a
  header, per OpenAPI's `securitySchemes`) from the start, even though
  every endpoint is unauthenticated until phase 12 — adding auth to an
  existing, already-shipped contract shape is cheaper than retrofitting it
  into endpoints designed without the field.

## Non-functional requirements

- **Performance** — not applicable at this spec's level; no endpoint exists
  yet to budget.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; this is a machine contract, not UI.
- **Reliability** — FR-3's drift-is-a-test-failure requirement is the
  reliability property: a contract nobody enforces degrades into
  documentation nobody trusts, which is worse than no contract, because it
  looks authoritative while being wrong.
- **Observability** — FR-5's correlation ID requirement ties every error
  response directly to a log line, which is what makes "How it was
  verified" in a bug report actually answerable.

## Domain model

Not applicable — this spec is transport/contract shape, not the
Alexandryn/metadata/source domain (constitution §3). It does fix that
domain errors (e.g. "book not found") flow through the one error shape
(FR-5), never a bespoke per-endpoint format.

## API and contracts

This spec *is* the API-and-contracts layer for the system as a whole —
there's no separate section to point elsewhere for once. Concretely:

- **Location**: `api/openapi.yaml`, repo root, versioned with the code.
- **Base path**: `/api/v1`.
- **Error shape**: `{ "error": { "code": "...", "message": "...",
  "correlationId": "..." } }` — exact field names subject to phase 03
  bikeshedding, but the three fields and their purpose are fixed here.
- **Auth**: reserved (`securitySchemes`), unused until phase 12.

## State transitions

Not applicable — this spec has no runtime state of its own.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Go handler doesn't match the OpenAPI spec | FR-3's contract test, in CI | Nothing (caught before merge) | Build fails; this is the whole point of FR-3 |
| A new endpoint ships without an OpenAPI entry | Same contract test, if implemented to check completeness rather than only response-shape of *documented* endpoints | Nothing, if caught; an undocumented endpoint if not | Phase 03's contract-test tool choice needs to cover this specifically, not just "does the response match what little the spec says" — flagged as an implementation requirement, not assumed satisfied by any tool |
| Error response missing a correlation ID | Contract test against FR-5's shape | A bug report that can't be tied to a log line | Same as above — this is exactly the failure mode FR-5 exists to prevent, and it's only prevented if FR-3's test actually checks for it |

## Security considerations

- **Error responses reveal nothing** — restates constitution's existing
  bar (already stated in phase 03's own security considerations: "no
  paths, no SQL, no internal identifiers"). FR-5's shape makes this
  checkable: a contract test can assert the `message` field never contains
  a stack trace pattern, a file path, or a SQL fragment, the way a
  prose-only guideline can't be automatically enforced.
- **Auth field reserved, not implemented (FR-6)** — no security *action*
  yet, but a contract that has to grow an auth field later, into endpoints
  that already shipped without one, is a worse position than reserving it
  now at zero cost.
- **Versioning as a deprecation path (FR-4)** — matters for security too:
  a breaking security fix that must land as `/v2` needs the versioning
  scheme to already exist, not be invented under pressure during an
  incident.
- **Request limits are the contract's job to document, not just enforce**
  — constitution §4 requires every request reaching the host to have a
  size limit, shape check, and timeout. The actual limit values are phase
  03's (`backend-http-transport.md`), but the OpenAPI spec itself should
  state them per-endpoint where they're not the global default, so "what's
  the limit here" has one place to look instead of requiring a read of the
  Go middleware source.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Error-shape helper functions (constructing the FR-5 shape) |
| Integration | Contract test: real Go handlers validated against `api/openapi.yaml` |
| Contract | This entire spec, in a sense — FR-3 is the test strategy |
| E2E | Not applicable at this spec's level |
| Accessibility | Not applicable |

The hardest thing to test here, per phase 03's own note about its hardest
case: none of it exists until there's a real endpoint. This spec is
verified by walkthrough, same as `architecture-system.md`'s own approach —
trace the health endpoint (the one thing that already conceptually exists,
`architecture-system.md` FR-7) through this contract shape and confirm it
fits without contortion.

## Acceptance criteria

- [ ] `api/openapi.yaml` exists at the fixed location, even if it initially
      contains only the health endpoint
- [ ] A contract test exists and fails when a handler's response doesn't
      match the spec — proven with a deliberately broken handler, not just
      asserted
- [ ] Error shape (FR-5) is implemented as a shared type/helper, not
      hand-constructed per handler
- [ ] Phase 06's first real endpoints (library browse) can be added to the
      contract without this spec needing amendment

## Open questions

- **Contract-test tool** — phase 03's to pick (FR-3 leaves this open
  deliberately, same layering as every other phase 01 spec's relationship
  to phase 03/05's implementation-level specs).
- **Does FR-3's contract test catch *missing* endpoints, or only
  *mismatched* ones?** Flagged in Failure modes — a real gap in what "the
  contract is enforced" actually guarantees if not designed carefully.
- **Pagination, filtering, sorting conventions** — real API design
  questions, not decided here, arrive with phase 06's first list endpoint
  (library browse) and should be decided against a concrete slice, not
  guessed at in the abstract — same reasoning phase 01's own risk table
  already uses to justify prototyping against a concrete case.
- **Local dev: frontend dev server ↔ backend dev server, different ports**
  — a CORS or dev-proxy problem entirely unaddressed here. This spec's
  "no skew" reasoning (FR-4) is about the *production* same-origin
  arrangement; local development during phase 04 almost certainly runs two
  dev servers on two ports. Owner: `architecture-frontend.md` or phase
  03/04's own dev-tooling setup, not decided here.

## References

- ADR 0006 — API contract format is OpenAPI, `alexandryn` carries the
  contract, not prose documentation
- `architecture-system.md` — FR-6 (shared UI/serving model), FR-7 (health
  endpoint), Observability (correlation ID)
- `.claude/roadmap/03-backend-foundation/README.md` — owns the error
  taxonomy and contract-test tool implementation
- `.claude/roadmap/06-library/README.md` — first real feature to exercise
  this contract
- Constitution §1 (spec precedes implementation), §11 (copy)
