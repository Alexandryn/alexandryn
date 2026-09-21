# Spec: Go backend architecture

| | |
|---|---|
| **Status** | `APPROVED` (maintainer confirmed 2026-08-20 — the container-topology amendment and FR-7's `golangci-lint` choice both ratified as written) |
| **Phase** | `01-architecture` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-18 |
| **Supersedes** | — |
| **Reviewed in** | `.claude/reviews/0011-spec-architecture-backend.md` — Approved with changes, all findings fixed; self-reviewed, independent read still pending. Amendment covered by `0041`. FR-7 added 2026-08-18 (lint tool selection, ADR 0018) — self-reviewed, needs maintainer re-confirmation alongside the existing amendment |

## Context

Phase 01's own exit criteria already commit to something this spec has to
deliver: "Layering rules expressed as enforceable lint configuration, not
prose." Phase 02's risk table separately names the exact failure this
guards against: "The domain quietly acquiring database or JSON tags." This
spec is where the package layout and the enforcement mechanism both get
decided, so phase 03 implements against a fixed target instead of
improvising layering under deadline pressure — the same reasoning phase 01
already used to justify this whole phase's existence.

## Problem

Nothing has fixed: the package layout, which packages may import which,
how that's enforced automatically rather than by code-review vigilance, the
shape of the error taxonomy shared across the stack, config-source
precedence, or the transport middleware ordering.

## Goals

- Fix a package layout using Go's own `internal/` visibility as a real
  enforcement mechanism, not just a naming convention
- Fix the dependency direction: domain has zero outward dependencies,
  everything else depends on domain-defined interfaces, never the reverse
- Satisfy phase 01's own exit criterion: layering enforced by lint in CI,
  not documentation
- Fix the error taxonomy's *shape* (categories, HTTP-status mapping) that
  feeds `architecture-contracts.md` FR-5's error response shape
- Fix config-source precedence, tying in ADR 0007's config-file mechanism
  as one concrete source
- Fix transport middleware ordering (constitution §4: validate at the
  point input enters the system)

## Non-goals

- The actual list of error codes/taxonomy entries — this spec fixes the
  *shape*, phase 03's `backend-errors-and-logging.md` fills it in
- Repository interface signatures, domain type mapping — phase 03's
  `backend-persistence.md`, building on `architecture-persistence.md`
- The specific lint tool (a specific import-linter, `go vet` extension, or
  hand-rolled AST check) — phase 03 picks it; this spec requires that
  *something* enforces the rule automatically
- Router library choice — phase 01's own "architecture decisions expected"
  list already assigns this to phase 03 ("standard library or a
  dependency, justified under constitution §9")
- Monorepo layout above the Go module itself — decided, ADR 0008: Go
  module at the repository root, `web/` and `electron/` as npm workspace
  packages alongside it

## User stories

- As **phase 02 (domain)**, I want the domain package's zero-dependency
  rule enforced by tooling, not by every reviewer remembering to check.
- As **phase 03**, I want to implement against a fixed package layout and
  error taxonomy shape instead of inventing both while also building the
  first real service.
- As **a contributor two years from now**, I want to know why the domain
  package can't just import the Postgres driver "just this once" — because
  a lint failure tells me immediately, not because I have to find and read
  this spec first.

## Functional requirements

- **FR-1** The Go module MUST use `internal/` for every package except
  `cmd/*` entrypoints — this makes cross-boundary imports a compile error
  for anything outside the module, not just a convention inside it. Layout:
  `internal/domain` (business logic, phase 02's model), `internal/transport/http`
  (handlers, routing, middleware), `internal/persistence/postgres`
  (repository implementations, Postgres spawn/lifecycle per
  `architecture-persistence.md`), `internal/config`, `cmd/server` (the
  entrypoint `architecture-system.md` describes), `cmd/pg-supervisor` (the
  macOS-only supervisor from `architecture-persistence.md` FR-10 — a
  separate binary, same module).
- **FR-2** `internal/domain` MUST NOT import `internal/transport/*` or
  `internal/persistence/*`, directly or transitively. Transport and
  persistence packages depend on interfaces *defined in* `internal/domain`,
  never the reverse — dependency inversion, not just a naming convention
  that happens to look layered.
- **FR-3** This dependency direction MUST be enforced by an automated lint
  check in CI that fails the build on violation — satisfying phase 01's own
  exit criterion. This is not redundant with FR-1: Go's `internal/`
  visibility only blocks imports from *outside* the module — it does
  nothing to stop `internal/domain` importing `internal/persistence`,
  since both are internal to the same module. FR-1 keeps the outside world
  out; FR-3 is the only thing keeping the *inside* honest. A human
  reviewer catching a violation in code review is not sufficient either —
  by the time a PR is open, it should already be a red CI check.
- **FR-4** Errors crossing from `internal/domain` outward MUST be one of a
  fixed, small set of categories (e.g. not-found, invalid-input,
  unauthorized, conflict, unavailable, internal) with a deterministic
  mapping to HTTP status codes at the transport boundary. The domain layer
  MUST NOT know about HTTP status codes — that mapping lives in
  `internal/transport/http`, one place, not scattered per handler.
- **FR-5** Configuration MUST be resolved in a fixed precedence order:
  compiled-in defaults, then a config file if one is present, then
  environment variables as an override. Two deployment targets deliver
  configuration differently at that second and third step, neither more
  "packaged" than the other (amended 2026-08-16, ADR 0015; this FR
  previously named only the first as how "a packaged instance is always
  configured," written before a second target existed). **Electron-hosted
  target**: the config file ADR 0007/`architecture-desktop-host.md` FR-5
  delivers at spawn time is the normal source; environment variables are
  an *additional* override available when running `cmd/server` directly
  outside Electron entirely (a backend developer iterating with `go run`,
  with no spawn-time file to read). **Container-hosted target**: there is
  no spawn-time file delivery mechanism at all — Compose's own
  `environment:`/`.env` handling is the normal source, read the same way
  the Electron target's dev/`go run` override already is, not as a
  workaround. `config.Load`'s per-key precedence algorithm
  (`backend-configuration.md` FR-2) does not need to know which target
  it's running under to apply this correctly — it resolves the same three
  steps regardless of source, which is exactly what makes both targets
  legitimate without new logic. No silent fallback if a required value is
  missing at any level, in either target. Startup MUST fail loudly and
  specifically, not proceed with a guessed default.
- **FR-6** The HTTP middleware chain MUST apply, in this order, to every
  request before it reaches a handler: panic recovery (outermost — a
  panic must never reach the client as a raw stack trace, constitution
  §11), request size and timeout limits (constitution §4), structured
  logging with a correlation ID (`architecture-system.md`'s Observability
  section), then routing. Authentication inserts once phase 12 exists,
  between logging and routing — this spec reserves the slot, phase 12
  designs what fills it.

  **Amended by phase 13 (2026-09-02, `DRAFT` — `backend-network-transport.md`,
  ADR 0028):** the "between logging and routing" region — reserved above
  for authentication alone — holds, once the host can bind beyond
  loopback: security-response headers (CSP / `X-Frame-Options` /
  `nosniff` / `Referrer-Policy`), HSTS (TLS binds only), CORS
  (deny-by-default), a global rate limiter for unauthenticated public
  paths, then authentication (which now also asserts the access-token
  type and validates the active-library header against the token claims),
  then `Origin` validation scoped to the unauthenticated pairing route
  group. Recovery stays strictly outermost and routing strictly
  innermost; only the single reserved slot expands into an ordered group.
  `backend-http-transport.md` FR-1 carries the concrete order.
- **FR-7** (Added 2026-08-18, ADR 0018) General Go static analysis in CI
  MUST run through `golangci-lint`, configured by a project-root
  `.golangci.yml`. This is separate from FR-2/FR-3's import-boundary
  check, which is not a stock lint rule any general-purpose Go linter
  ships with opinions about — that check stays `tasks/plan.md`'s D0
  interim grep/file-walk script (`scripts/check-import-boundaries.sh`)
  until reimplemented as a `golangci-lint` custom rule or `go/analysis`
  pass, per D0's own stated revisit condition. FR-7 resolves this spec's
  previously-open "lint tool choice" question; ADR 0018 carries the full
  options considered and the constitution §9 dependency justification.

## Non-functional requirements

- **Performance** — not applicable at this spec's level; no code exists to
  budget yet.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; backend has no UI.
- **Reliability** — FR-5's "fail loudly, no silent fallback" is the
  reliability property that matters most here: a service that starts with
  a guessed config value is worse than one that refuses to start.
- **Observability** — FR-6 fixes where the correlation ID enters the
  request lifecycle (logging middleware, before routing) — every handler
  downstream inherits it without having to generate or thread it manually.

## Domain model

Not applicable to this spec directly — it's the layer *around* the domain
(phase 02's job), not the domain itself. FR-1/FR-2 are exactly the
enforcement mechanism that keeps this spec's boundary and phase 02's model
from drifting into each other.

## API and contracts

- **Domain ↔ transport**: Go interfaces defined in `internal/domain`,
  implemented by `internal/transport/http` handlers calling into domain
  services — not the domain calling out to transport.
- **Domain ↔ persistence**: same pattern — repository interfaces in
  `internal/domain`, implementations in `internal/persistence/postgres`.
- **Transport ↔ the wire**: `architecture-contracts.md` owns the actual
  HTTP contract shape; this spec only fixes where in the package layout
  that contract gets implemented.

## State transitions

Not applicable — this spec is static structure (packages, dependency
direction), not runtime state. `architecture-system.md` owns the
application lifecycle state machine this backend implements.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| A PR adds an import from `internal/domain` to `internal/persistence` | FR-3's CI lint check | Nothing (caught before merge) | Build fails, PR blocked |
| Required config missing at startup | FR-5's validation | Startup fails with a specific, named error (constitution §11) | Process exits non-zero, does not guess a default |
| Unhandled panic in a handler | FR-6's recovery middleware | A generic error response (FR-4's "internal" category), correlation ID included | Panic is caught, logged with a stack trace *server-side only*, never sent to the client |

## Security considerations

- **Import-boundary enforcement (FR-2, FR-3) is a security control, not
  just tidiness** — it's what keeps a future contributor from having the
  domain layer accidentally trust something the transport layer should
  have validated first, or from a repository implementation leaking a raw
  SQL error into a domain-level error category that transport then exposes
  verbatim.
- **Panic recovery (FR-6) as the outermost middleware** — restates
  constitution §11 and phase 03's existing risk table concretely: recovery
  must wrap everything, including future middleware added later, which
  means it needs to be structurally first, not just documented as
  "supposed to be first."
- **Config fail-loudly (FR-5)** — an insecure default applied silently
  (phase 03's own risk table already names this) is exactly what a fixed
  precedence with no fallback prevents.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Error-category-to-HTTP-status mapping (FR-4), config precedence resolution (FR-5) |
| Integration | Full middleware chain end to end (FR-6), against a real request |
| Contract | `architecture-contracts.md`'s contract test validates the transport layer's actual responses |
| Concurrency | Import-boundary lint (FR-3) MUST run in CI, not just locally — the actual CI mechanics are `architecture-testing.md`'s ("CI shape"), same pattern as `architecture-contracts.md` FR-3's contract test |
| Accessibility | Not applicable |

The hardest thing to test here, same as phase 03's own note: shutdown with
a request in flight. This spec doesn't re-decide that
(`architecture-system.md` FR-9 already does); it just confirms the
middleware chain doesn't introduce a new place for that request to get lost.

## Acceptance criteria

- [ ] `internal/` layout exists with the packages named in FR-1
- [ ] A deliberately-introduced boundary violation (domain importing
      persistence) fails CI, proven not asserted
- [ ] Config startup fails loudly on a missing required value, proven with
      a test that removes one
- [ ] Every FR maps to an exit criterion in phase 03's own document
- [ ] `golangci-lint` runs in CI (FR-7) and blocks merge on a finding,
      proven not asserted

## Open questions

- **Lint tool choice** — resolved 2026-08-18, FR-7 / ADR 0018:
  `golangci-lint` for general Go lint. The import-boundary check itself
  stays `tasks/plan.md`'s D0 interim script, unaffected by this
  resolution.
- **Exact error category list** — phase 03's `backend-errors-and-logging.md`.
- **`cmd/pg-supervisor`'s relationship to `internal/persistence/postgres`**
  — does the supervisor binary share code with the main server's Postgres-
  spawning logic, or duplicate the small amount it needs? Not decided;
  likely a thin binary importing shared internal code, but that's an
  implementation call for phase 03/05, not fixed here.

## References

- Phase 01's own exit criteria — "layering rules expressed as enforceable
  lint configuration, not prose" is this spec's central requirement
- Phase 02's risk table — "the domain quietly acquiring database or JSON
  tags," the exact failure FR-2/FR-3 prevent
- `architecture-contracts.md` — FR-5's error shape, which FR-4 here feeds
- `architecture-persistence.md` — FR-1/FR-8/FR-9/FR-10, implemented inside
  `internal/persistence/postgres` and `cmd/pg-supervisor`
- `architecture-desktop-host.md` FR-5 — the config-file source FR-5 here
  incorporates into precedence
- ADR 0018 — `golangci-lint` selection (FR-7) and its constitution §9
  dependency justification
- ADR 0015 — the container-hosted target FR-5 was amended 2026-08-16 to
  cover, alongside the Electron-hosted target
- `docs/roadmap/03-backend-foundation/README.md` — owns the actual
  implementation of everything this spec fixes the pattern for
- Constitution §3 (domain boundaries), §4 (hostile input), §9
  (dependencies), §11 (copy)
