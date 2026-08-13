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

Record the options that lost. The next person will think of them too and
deserves to know they were already weighed.

## Index

| # | Decision | Status |
|---|---|---|
| [0001](0001-record-architecture-decisions.md) | We record architecture decisions in this directory | Accepted |
| [0002](0002-project-licence.md) | Project licence | **Proposed — needs a decision** |
| [0003](0003-design-canvas-split.md) | Design reference split into per-surface canvases (Desktop/Host, Web/Remote viewer, States, Design system) | Accepted |
| [0004](0004-persistence-engine-postgresql.md) | Persistence engine is self-hosted PostgreSQL; Supabase is dev/test tooling only | Accepted |

## Open questions not yet ADRs

Things known to need deciding, with the phase that will force the question:

| Question | Forced by |
|---|---|
| Monorepo layout and tooling | Phase 01 |
| Migration tooling, and recovery from a failed partial migration (engine decided — ADR 0004) | Phase 01 |
| Frontend data-fetching and state approach | Phase 01 |
| Where the API contract is defined, and who owns it | Phase 01 |
| Whether RabbitMQ is warranted, and for exactly which work | Phase 09 |
| How the reader renders EPUB, and in what sandbox | Phase 11 |
| Credential storage on the host | Phase 12 |
| Transport security on the LAN | Phase 13 |
