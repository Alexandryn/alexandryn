# Specifications

One specification per coherent feature or subsystem. Start from
[`../templates/spec.md`](../templates/spec.md).

## Status lifecycle

```
DRAFT → REVIEWED → APPROVED → IMPLEMENTED → VERIFIED
```

| Status | Means |
|---|---|
| `DRAFT` | Being written. Nobody should build against it. |
| `REVIEWED` | A reviewer has been through it and recorded findings in `../reviews/`. |
| `APPROVED` | Findings resolved, maintainer signed off. **Implementation may begin.** |
| `IMPLEMENTED` | Code exists and tests pass. |
| `VERIFIED` | Audited, accessibility-checked, and confirmed against acceptance criteria. |

Status only moves forward. If implementation shows the spec was wrong, amend
the spec, note the change, and move it back through review deliberately —
don't let the code quietly become the specification.

**Nothing below `APPROVED` gets implemented.** Constitution §1.

## Naming

`<area>-<feature>.md`, kebab-case. Examples: `sources-opds-provider.md`,
`reader-progress-sync.md`, `import-metadata-matching.md`.

Group by area, not by phase — features outlive the phase that introduced them.

## Index

| Spec | Area | Phase | Status |
|---|---|---|---|
| [`architecture-system.md`](architecture-system.md) | Process model, boundaries, data flow, deployment shape | 01 | `REVIEWED` (amended post-approval, needs re-confirmation — ADR 0007) |
| [`architecture-desktop-host.md`](architecture-desktop-host.md) | Electron processes, IPC surface, serving model, lifecycle | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-persistence.md`](architecture-persistence.md) | Engine, schema ownership, migrations, corruption and backup | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-contracts.md`](architecture-contracts.md) | API and event schemas, versioning, contract testing | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-backend.md`](architecture-backend.md) | Go package layout, dependency rules, transport, errors, config | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-frontend.md`](architecture-frontend.md) | React layering, state ownership, data fetching, routing | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-testing.md`](architecture-testing.md) | Layers, tooling, fixtures, determinism, CI shape | 01 | `REVIEWED` (self, approved with changes) |
| [`domain-bibliographic.md`](domain-bibliographic.md) | Work, edition, author, subject, language, identity and dedup | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-library.md`](domain-library.md) | Library membership, collections, ownership, availability | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-source.md`](domain-source.md) | Source identity, capability model, availability, file references | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-reading.md`](domain-reading.md) | Position, progress, bookmarks, highlights, preferences | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-events.md`](domain-events.md) | Events the domain emits, and what may consume them | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`backend-service-lifecycle.md`](backend-service-lifecycle.md) | Startup, wiring, shutdown, failure to start | 03 | `APPROVED` |
| [`backend-configuration.md`](backend-configuration.md) | Sources, precedence, validation, defaults, secrets | 03 | `APPROVED` (amended post-approval — `LOG_LEVEL` case-sensitivity, `0025`, needs re-confirmation) |
| [`backend-errors-and-logging.md`](backend-errors-and-logging.md) | Taxonomy, transport mapping, log contract, redaction | 03 | `APPROVED` |
| [`backend-http-transport.md`](backend-http-transport.md) | Router, middleware, limits, timeouts, response shape | 03 | `APPROVED` |
| [`backend-persistence.md`](backend-persistence.md) | PostgreSQL connection lifecycle, migrations, repositories, transactions, corruption | 03 | `APPROVED` |
| [`backend-test-harness.md`](backend-test-harness.md) | Integration harness, fixtures, controllable clock, CI | 03 | `APPROVED` |
