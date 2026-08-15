# Architecture decision records

A decision belongs here when it would be expensive to reverse, when it
constrains other work, or when a future contributor would otherwise reasonably
ask "why on earth is it like this?"

Not every choice needs an ADR. Which HTTP library — probably not. Whether the
domain layer may import the HTTP library — yes.

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

## Open questions not yet ADRs

Things known to need deciding, with the phase that will force the question:

| Question | Forced by |
|---|---|
| Frontend data-fetching and state approach | Phase 01 |
| Where the API contract is defined, and who owns it — format fixed as OpenAPI (ADR 0006); ownership/versioning/design still open | Phase 01 |
| ~~Whether RabbitMQ is warranted, and for exactly which work~~ — addressed by ADR 0014 (PostgreSQL-backed, not RabbitMQ) | Phase 09 |
| How the reader renders EPUB, and in what sandbox | Phase 11 |
| Credential storage on the host | Phase 12 |
| Transport security on the LAN | Phase 13 |
