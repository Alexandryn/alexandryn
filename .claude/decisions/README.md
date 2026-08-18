# Architecture decision records

A decision belongs here when it would be expensive to reverse, when it
constrains other work, or when a future contributor would otherwise reasonably
ask "why on earth is it like this?"

Not every choice needs an ADR. Which HTTP library — probably not. Whether the
domain layer may import the HTTP library — yes.

**A decision meeting that bar gets its own ADR even when it's first made
while drafting a spec, not only when drafting an ADR directly.** Two
choices of the same weight as the backend's own ADR 0011–0014 —
`frontend-shell-and-routing.md` FR-2 (TanStack Query as the data-fetching
library, constraining every later frontend spec that fetches data) and
`architecture-contracts.md` FR-4 (the API's path-based versioning scheme,
constraining every endpoint-owning spec since) — were settled as spec FRs
only, with no standalone ADR, found during the 2026-08-16/17 spec-set
review. Both meet the criterion above (constrains later specs; a
reviewer would reasonably ask "why is it like this"); neither is
retroactively getting an ADR now, since retro-fitting one for an
already-settled, working decision has less value than fixing the gap
going forward — a spec's FR is where a decision meeting this bar is
*applied*, and may cite an ADR, but does not substitute for filing one.
When drafting or reviewing a spec, a decision inside it that independently
meets this page's own criterion needs its own ADR, filed alongside the
spec, not folded silently into the FR text.

## Conventions

- Filename: `NNNN-short-kebab-title.md`, numbered sequentially, never reused
- Start from [`../templates/adr.md`](../templates/adr.md)
- Title is a statement, not a question: *"The host binds to loopback by
  default"*, not *"Binding behaviour?"*
- Status: `Proposed` → `Accepted` / `Rejected`, and later `Superseded` or
  `Deprecated`
- Superseding creates a new ADR and updates both records. The old one stays.
- **Extending, not superseding, gets an in-place `## Addendum — <what
  changed> (<date>)` section instead of a new ADR.** Use this when a later
  decision adds to or extends an Accepted ADR's finding without
  contradicting it (e.g. ADR 0007 adding a third process to what ADR
  0005's two-process finding still correctly describes at the Electron↔Go
  layer). If the *original* decision itself would need to change, that's
  supersession — a new ADR, not an addendum. (ADR 0004 and ADR 0005 both
  already used this pattern before it was written down here.)

Record the options that lost. The next person will think of them too and
deserves to know they were already weighed.

## Index

| # | Decision | Status |
|---|---|---|
| [0001](0001-record-architecture-decisions.md) | We record architecture decisions in this directory | Accepted |
| [0002](0002-project-licence.md) | Project licence | **Proposed — needs a decision** |
| [0003](0003-design-canvas-split.md) | Design reference split into per-surface canvases (Desktop/Host, Web/Remote viewer, States, Design system) | Accepted |
| [0004](0004-persistence-engine-postgresql.md) | Persistence engine is self-hosted PostgreSQL; Supabase is dev/test tooling only | Accepted |
| [0005](0005-process-model.md) | Go server is a spawned child process, never embedded into one binary; prototype-backed | Accepted |
| [0006](0006-docs-and-website-repos.md) | Documentation and the landing page live in separate repos (`docs`, `website`), created at phase 99 release | Accepted |
| [0007](0007-postgres-provisioning.md) | Production PostgreSQL is bundled and managed by the Go server, never user-configured | Accepted |
| [0008](0008-monorepo-layout.md) | Monorepo layout: Go module at root, two npm workspace packages, no build-orchestration tool | Accepted |
| [0009](0009-progress-attachment.md) | Reading progress attaches to Work, Edition-scoped precise position as fallback-capable secondary | Accepted |
| [0010](0010-bibliographic-identity-strategy.md) | Bibliographic identity is internal-ID-primary; external references optional, never required | Accepted |
| [0011](0011-http-router-and-middleware.md) | HTTP routing uses the standard library's `ServeMux`; middleware is hand-rolled, no router framework | Accepted |
| [0012](0012-postgres-driver.md) | PostgreSQL access uses `pgx` natively, not `database/sql` | Accepted |
| [0013](0013-migration-tool.md) | Schema migrations use `goose`, embedded via `go:embed` | Accepted |
| [0014](0014-job-queue-backend-postgresql.md) | Background job queue is PostgreSQL-backed, not a dedicated message broker | Accepted |
| [0015](0015-container-topology.md) | A second deployment target ships the backend as a container, composed with PostgreSQL, alongside the Electron-hosted target | Accepted |
| [0016](0016-test-plan-cadence.md) | Test plans are written per-phase, at RED-step time, not per-spec at approval time | Proposed |
| [0017](0017-network-exposure-condition-gated.md) | Network exposure is gated on TLS and authentication being verifiably true, not on a phase number or deployment target | Accepted |
| [0018](0018-go-lint-tool.md) | General Go static analysis uses `golangci-lint`; the import-boundary check stays a separate, interim script | Accepted |
| [0019](0019-toml-library.md) | Config-file parsing uses `pelletier/go-toml/v2`, decoded into a `map[string]any` for the per-key lookup `internal/config` already uses | Accepted |

## Open questions not yet ADRs

Things known to need deciding, with the phase that will force the question:

| Question | Forced by |
|---|---|
| ~~Frontend data-fetching and state approach~~ — resolved without an ADR, see `frontend-shell-and-routing.md` FR-2 (TanStack Query, v5, pinned major) | Phase 01 |
| ~~Where the API contract is defined, and who owns it~~ — format fixed as OpenAPI (ADR 0006); ownership/versioning/design resolved without an ADR, see `architecture-contracts.md` FR-4 (path-based versioning, `/api/v1/...`) | Phase 01 |
| ~~Whether RabbitMQ is warranted, and for exactly which work~~ — addressed by ADR 0014 (PostgreSQL-backed, not RabbitMQ) | Phase 09 |
| ~~How the reader renders EPUB, and in what sandbox~~ — resolved without an ADR, see `frontend-reader.md` FR-1 (`foliate-js` rendering engine; `<iframe sandbox="allow-same-origin">`, no `allow-scripts`) and `backend-reader-content.md` FR-6/FR-9 (server-side HTML/CSS sanitisation, `Content-Security-Policy: default-src 'self'; script-src 'none'...`) | Phase 11 |
| Credential storage on the host | Phase 12 |
| CSRF protection for state-changing requests, once sessions are cookie-based | Phase 12 |
| Open-redirect protection for any post-authentication redirect target | Phase 12 |
| Transport security on the LAN | Phase 13 |
| ~~Non-loopback bind for the container-hosted target (ADR 0015)~~ — the *rule* is resolved: ADR 0017 replaces the phase-gate with a two-mode condition (in-process TLS on a public bind, or upstream TLS with the process bound private-only), both authentication-gated and fail-closed. `backend-configuration.md` FR-8 and `deployment-container-packaging.md` FR-6 are amended to match. What's still Phase 12/13's: actually building authentication and the certificate/reverse-proxy configuration surface the rule depends on | Phase 12/13 (implementation only — the rule itself is decided) |
