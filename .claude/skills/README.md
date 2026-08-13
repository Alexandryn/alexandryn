# Skills

Reusable practice guides — the things a contributor (human or automated) should
know how to do here, written once instead of re-explained per review.

## Two kinds

**`general/`** — engineering practice that would apply to any serious project:
how a code review is conducted here, how an API is designed, how a security
review is structured, how to write a test plan.

**`purpose/`** — Alexandryn-specific knowledge that a generalist wouldn't have:
the domain model, the source provider contract, Open Library's quirks, EPUB
handling, the Electron IPC conventions, RabbitMQ naming.

Do not duplicate a general skill inside a purpose skill. A purpose skill
assumes the general one and adds only what is specific.

## When to write one

After the practice has stabilised, not before. A skill written speculatively
describes an imagined workflow and then quietly diverges from the real one.

The signal to write one: the same guidance has been given in review twice.

## Shape

Short, actionable, and opinionated. A skill is a checklist with reasoning, not
an essay. If it can't be applied while working, it belongs in `docs/` instead.

## Status

**Empty by design.** The practices these would describe don't exist yet — the
first ones become writable once Phase 01 settles the architecture and Phase 02
settles the domain. Writing them now would be fiction.

Planned, roughly in the order they'll become real:

| Skill | Kind | Writable after |
|---|---|---|
| Domain model and boundaries | purpose | Phase 02 |
| Go backend conventions | purpose | Phase 03 |
| PostgreSQL access and migration conventions (ADR 0004) | purpose | Phase 03 |
| Frontend component conventions | purpose | Phase 04 |
| Code review | general | Phase 03 |
| Security review | general | Phase 03 |
| Accessibility review | general | Phase 04 |
| Electron IPC conventions | purpose | Phase 05 |
| Open Library integration | purpose | Phase 07 |
| Source provider contract | purpose | Phase 08 |
| RabbitMQ conventions | purpose | Phase 09 |
