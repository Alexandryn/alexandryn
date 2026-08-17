# Phase 03 — Backend foundation

| | |
|---|---|
| **Status** | Original six specs approved, implementation not started; `deployment-container-packaging.md` (added 2026-08-17) `REVIEWED`, awaiting approval |
| **Depends on** | Phase 01, Phase 02 |
| **Blocks** | 05, 06, 12 |

## Objective

A Go service that starts, reads its configuration, connects to PostgreSQL,
serves health, logs usefully, fails clearly, and shuts down cleanly. It
exposes almost no functionality — and everything it does expose is
production-shaped.

This is also where CI becomes real: the first phase with code to build, lint,
typecheck and test.

## Why here

Every later backend phase inherits this phase's habits. If the error taxonomy,
the logging contract and the test layout are decided here and enforced, later
phases stay consistent for free. If they are improvised per feature, no
retrofit will ever catch up.

## Scope

**In**

- Service skeleton: entry point, dependency wiring, graceful shutdown
- Configuration: precedence, validation at startup, safe defaults, no silent
  fallbacks
- Structured logging with request correlation, and redaction as a property of
  the logger rather than a rule people remember
- The error taxonomy from phase 01, with the mapping to transport-level
  responses
- HTTP transport: router, middleware chain, timeouts, body size limits,
  panic recovery
- Persistence: PostgreSQL connection pool and lifecycle (engine decided —
  ADR 0004), migration runner, forward-only migration policy
  (`architecture-persistence.md` FR-4/FR-5, not ADR 0004 — that ADR
  decided the engine, not the migration policy); spawning and owning the
  bundled Postgres process itself, including orphan-prevention per
  platform (ADR 0007, `architecture-persistence.md` FR-8/FR-9/FR-10)
- Repository interfaces for the phase 02 domain, with one real implementation
- Health and readiness endpoints
- Test infrastructure: integration harness against a real PostgreSQL instance,
  deterministic fixtures, and a clock that tests control
- CI: build, vet, lint, test, race detector, coverage reporting, dependency
  audit
- Container deployment packaging: the Dockerfile and docker-compose.yml
  for the container-hosted target (ADR 0015), unreachable by design until
  phase 12/13 — the baseline packaging design, not release-time
  hardening, which stays phase 99's

**Out**

- Any domain feature endpoint — phase 06 onward
- Authentication — phase 12. This service binds to loopback and serves nothing
  sensitive until then.
- Message consumers — phase 09
- Metrics beyond health — phase 15

## Specifications

| Spec | Covers |
|---|---|
| `backend-service-lifecycle.md` | Startup, wiring, shutdown, failure to start |
| `backend-configuration.md` | Sources, precedence, validation, defaults, secrets |
| `backend-errors-and-logging.md` | Taxonomy, transport mapping, log contract, redaction |
| `backend-http-transport.md` | Router, middleware, limits, timeouts, response shape |
| `backend-persistence.md` | PostgreSQL connection lifecycle, migrations, repositories, transactions, corruption |
| `backend-test-harness.md` | Integration harness, fixtures, controllable clock, CI |
| `deployment-container-packaging.md` | Dockerfile, docker-compose.yml, container-target health checks and CI guard |

## Architecture decisions expected

- Router and middleware approach — standard library or a dependency, justified
  either way under constitution §9
- PostgreSQL driver/access layer (e.g. `database/sql` plus driver, or a query
  builder) — justified under constitution §9 same as any dependency
- Migration tooling (recovery *policy* already decided —
  `architecture-persistence.md` FR-4/FR-5: forward-only, fail loudly, no
  silent continuation past a partial migration; this phase picks the tool
  and implements the policy, doesn't re-decide it)
- Repositories return domain types — decided by
  `architecture-backend.md` FR-2 (repository interfaces live in
  `internal/domain`, implemented by persistence); this phase implements
  it, doesn't re-decide it
- Log format and level policy: what is debug, what is a real event
- How the clock, filesystem and randomness are injected so tests stay
  deterministic

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Redaction implemented as a convention people must remember | **High** | **High** — reading history or credentials leak into logs | Sensitive values carry types whose `String()` redacts; a test asserts a known secret never appears in captured output |
| Configuration with silent fallbacks | Medium | High — an insecure default applied unnoticed | Invalid or missing required config fails startup loudly; there is no "assume a default and continue" path |
| Migrations that cannot be rolled back and were never tested against real data | Medium | High — user data loss | Forward-only policy, tested against a populated database, with a documented backup step before applying |
| An integration harness slow enough that people stop running it | Medium | Medium | Budget agreed in the spec; exceeding it is a defect, not an inconvenience |
| Global state and package-level singletons creeping in early | High | Medium | Explicit constitution §17-style prohibition, enforced in review; everything is constructed and injected |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Config precedence and validation, error mapping, redaction, middleware in isolation |
| Integration | Migrations against a real PostgreSQL instance, repositories against a real PostgreSQL instance, the middleware chain end to end |
| Contract | Health endpoint shape, error response shape — both consumed by phase 04 |
| Concurrency | Race detector on all tests; graceful shutdown under in-flight requests |

The hardest thing to test here is shutdown: a request in flight when the signal
arrives must complete or be cleanly refused, never truncated. It gets an
explicit test rather than an assumption.

## Security considerations

Small surface, but the decisions here are the ones every later endpoint
inherits:

- **Request limits before parsing** — body size, header size, and timeouts
  applied by middleware, not per handler, so a new handler cannot forget
- **Panic recovery** that logs and returns a generic response, never a stack
  trace to the client
- **Error responses that reveal nothing** — no paths, no SQL, no internal
  identifiers; the correlation ID is how a user and a log line are connected
- **Loopback binding as the default**, with LAN binding physically absent
  rather than merely discouraged, until phase 13
- **Credentials never in the log, never in an error, never in a health
  response**, with a test proving it
- **Dependency audit in CI from the first dependency**

## Observability

The baseline the whole system inherits: structured logs with a correlation ID
per request, consistent field names, a startup line naming the version and
configuration source, and a health endpoint distinguishing "alive" from "ready"
— a service that has started but cannot reach PostgreSQL must not claim
readiness.

## Exit criteria

- [x] All six original specifications `APPROVED` with recorded reviews —
      self + independent review (`0022`), all findings fixed, approved
      by the maintainer 2026-08-14
- [ ] `deployment-container-packaging.md` `APPROVED`, added 2026-08-17 for
      ADR 0015's container target — currently `REVIEWED`, self-reviewed
      only, needs maintainer approval
- [ ] The service starts, serves health, and shuts down gracefully under load
- [ ] Migrations apply to an empty database and to a populated one
- [ ] Tests pass with the race detector enabled
- [ ] A test proves a known secret never reaches the logs
- [ ] The service refuses to bind to a non-loopback address
- [ ] CI runs build, vet, lint, test, race and dependency audit on every PR
- [ ] Startup fails loudly on invalid configuration, with a test
- [ ] Security audit recorded, no open Critical or High findings
- [ ] Maintainer approval recorded
