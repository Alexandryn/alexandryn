# Phase 09 — Async jobs

*Outline — expanded to a full phase document when phase 08 closes.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 08 |
| **Blocks** | 10, 15 |

## Objective

Background job infrastructure introduced at the first point real
asynchronous work exists: source sync and, next phase, import. Whether that
means RabbitMQ or something simpler is this phase's own analysis to make and
record as an ADR — not assumed in advance.

## Scope

**In**

- Job queue engine and storage, worker pool, retry with backoff, dead
  letter handling
- Job status and progress reporting
- The broker-or-not ADR

**Out**

- The import job handler itself — phase 10. Sync job handlers — phase 14.

## Exit criteria

- [ ] ADR recorded justifying the queue engine choice for this scale
- [ ] Enqueue, execute, retry, dead-letter all functional and tested
- [ ] Job status visible to callers
- [ ] Maintainer approval recorded
