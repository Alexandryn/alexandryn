# Phase 01 — Architecture

| | |
|---|---|
| **Status** | In progress |
| **Depends on** | Phase 00 |
| **Blocks** | 02, 03, 04 |

## Objective

Decide the shape of the system, and write the reasoning down. At the end of
this phase someone can say where any given piece of code belongs, what it is
allowed to import, and how it is tested — without asking.

Still no application code. This phase produces specifications and ADRs.

## Why here

Every structural decision made implicitly during implementation gets made
badly, because it is made under pressure to finish something else. Repository
layout, the domain's dependency direction, and where the API contract lives are
all cheap now and expensive in six months.

The temptation here is to over-design. The counterweight: only decide what the
next three phases actually need. Anything else goes on the open-questions list.

## Scope

**In**

- Repository layout and monorepo tooling
- Layering rules: what may import what, and how that is enforced
- Backend architecture — package boundaries, dependency direction, transport
  separation, error and configuration strategy
- Frontend architecture — component/state/data-fetching layers, where domain
  logic lives, routing
- Desktop host architecture — process model, IPC boundary shape, how the web
  UI is served to both the window and the network
- Persistence architecture — engine, migrations, ownership of schema
- API contract — where it is defined, who owns it, how compatibility is checked
- Messaging architecture — the *conditions* under which work becomes
  asynchronous, without committing to a broker yet
- Testing architecture — layers, tooling, fixtures, what runs in CI
- Configuration and secrets handling

**Out**

- The domain model itself — phase 02
- Any implementation — phase 03 onward
- Deployment topology beyond single-host self-hosting — not a goal of v1
- Broker selection and queue design — phase 09, when there is work to queue

## Specifications

| Spec | Covers |
|---|---|
| `architecture-system.md` | Process model, boundaries, data flow, deployment shape |
| `architecture-backend.md` | Go package layout, dependency rules, transport, errors, config |
| `architecture-frontend.md` | React layering, state ownership, data fetching, routing |
| `architecture-desktop-host.md` | Electron processes, IPC surface, serving model, lifecycle |
| `architecture-persistence.md` | Engine, schema ownership, migrations, corruption and backup |
| `architecture-contracts.md` | API and event schemas (OpenAPI, ADR 0006), versioning, contract testing |
| `architecture-testing.md` | Layers, tooling, fixtures, determinism, CI shape |

## Architecture decisions expected

- Monorepo layout, and the tool that manages it
- Persistence engine decided — self-hosted PostgreSQL, ADR 0004. Production
  provisioning also decided — bundled, spawned and owned by the Go server,
  ADR 0007 (`architecture-persistence.md`) — a consequence of this was
  amending `architecture-system.md`'s process count (FR-1, three processes,
  four on macOS). Schema ownership, connection lifecycle, and the
  forward-only migration policy are designed in `architecture-persistence.md`;
  the concurrent-access assumption ADR 0004 left at medium confidence is
  still unwalked
- Process model decided — two processes, Electron spawns the Go server as a
  child, prototype-backed (ADR 0005, `architecture-system.md`). This phase's
  remaining specs build on it rather than reopening it
- How the web UI is served identically to the desktop window and to the LAN
- API contract format fixed as OpenAPI (ADR 0006); this phase still owns
  ownership, versioning, and the actual schema design in
  `architecture-contracts.md`
- Frontend data-fetching and cache strategy
- Configuration precedence and where secrets are stored on the host
- Error taxonomy shared across the stack
- Layering enforcement: lint rules, not documentation

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Over-engineering for scale that a single-household library will never see | **High** | High — complexity is permanent | Explicit non-goal: this serves one household. Every abstraction must name the concrete case it handles today. |
| Architecture specified in the abstract, unworkable in practice | Medium | High | Each spec names a concrete slice from the design prototype and walks it end to end before approval |
| Layering rules that exist only in documentation | High | Medium | Enforced by import-boundary lint rules in CI, or they are not real rules |
| Electron and Go process model chosen for elegance rather than packaging reality | Medium | High — reversal is expensive | Prototype the packaging path before deciding; record it in the ADR |
| The truncated design reference hides a requirement that changes the frontend architecture | Medium | Medium | Obtain the complete export before `architecture-frontend.md` is approved |

## Test strategy

Nothing to execute; the deliverables are documents. Two verification techniques
replace tests:

- **Walkthrough** — take one real slice (open a book from the library, on a
  second device) and trace it through every layer of the proposed
  architecture. Gaps surface immediately.
- **Hostile walkthrough** — trace the same slice as a malicious LAN client and
  as a compromised renderer.

`architecture-testing.md` defines how everything after this phase is tested,
making it the highest-leverage document here.

## Security considerations

This phase decides where the trust boundaries *are*, which determines every
later audit. Three must be explicit and drawn on a diagram:

1. **Renderer ↔ main process** — the renderer is assumed compromised
2. **Host ↔ network client** — the client is assumed hostile and unauthenticated
   until phase 12
3. **System ↔ external source or metadata provider** — the response is assumed
   malicious

The architecture must also make the safe thing structural rather than
disciplined: loopback binding as the default that requires action to change,
and a preload surface that is an enumerated list rather than a pattern anyone
can extend without noticing.

## Observability

`architecture-backend.md` specifies the logging contract, the error taxonomy,
and request correlation before any handler exists. Retrofitted logging is
uniformly worse than designed logging, and by then nobody has time.

## Exit criteria

- [ ] All seven specifications reviewed and `APPROVED` — all seven now
      `REVIEWED` (self, approved with changes), none yet `APPROVED`; needs
      the maintainer's independent read per spec, not just this checkbox
- [x] Each has a recorded review in `.claude/reviews/`
- [ ] ADRs written for every decision listed above, with rejected options recorded
- [ ] One concrete slice traced end to end through the proposed architecture
- [ ] The three trust boundaries diagrammed
- [ ] Layering rules expressed as enforceable lint configuration, not prose
- [ ] Open questions either resolved or explicitly deferred to a named phase
- [ ] Maintainer approval recorded
