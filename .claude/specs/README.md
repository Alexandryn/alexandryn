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
| `APPROVED` | Findings resolved, self-reviewed against the maintainer-approved scope. **Implementation may begin.** |
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
| [`architecture-system.md`](architecture-system.md) | Process model, boundaries, data flow, deployment shape | 01 | `REVIEWED` (amended post-approval twice, both need re-confirmation — ADR 0007, ADR 0015) |
| [`architecture-desktop-host.md`](architecture-desktop-host.md) | Electron processes, IPC surface, serving model, lifecycle | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-persistence.md`](architecture-persistence.md) | Engine, schema ownership, migrations, corruption and backup | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-contracts.md`](architecture-contracts.md) | API and event schemas, versioning, contract testing | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-backend.md`](architecture-backend.md) | Go package layout, dependency rules, transport, errors, config | 01 | `REVIEWED` (self, approved with changes; amended post-review — container topology, ADR 0015, needs re-confirmation) |
| [`architecture-frontend.md`](architecture-frontend.md) | React layering, state ownership, data fetching, routing | 01 | `REVIEWED` (self, approved with changes) |
| [`architecture-testing.md`](architecture-testing.md) | Layers, tooling, fixtures, determinism, CI shape | 01 | `REVIEWED` (self, approved with changes) |
| [`domain-bibliographic.md`](domain-bibliographic.md) | Work, edition, author, subject, language, identity and dedup | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-library.md`](domain-library.md) | Library membership, collections, ownership, availability | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-source.md`](domain-source.md) | Source identity, capability model, availability, file references | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-reading.md`](domain-reading.md) | Position, progress, bookmarks, highlights, preferences | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`domain-events.md`](domain-events.md) | Events the domain emits, and what may consume them | 02 | `REVIEWED` (self + independent, approved with changes) |
| [`backend-service-lifecycle.md`](backend-service-lifecycle.md) | Startup, wiring, shutdown, failure to start | 03 | `APPROVED` (amended post-approval three times — DSN redaction `0028`, job-worker-pool shutdown ordering `0036` re-confirmed, container topology `0044`; `0028`/`0044` need re-confirmation) |
| [`backend-configuration.md`](backend-configuration.md) | Sources, precedence, validation, defaults, secrets | 03 | `APPROVED` (amended post-approval four times — `LOG_LEVEL` case-sensitivity `0025`, DSN redaction `0028`, `OPEN_LIBRARY_USER_AGENT` `0034` re-confirmed, container topology `0042`; `0025`/`0028`/`0042` need re-confirmation) |
| [`backend-errors-and-logging.md`](backend-errors-and-logging.md) | Taxonomy, transport mapping, log contract, redaction | 03 | `APPROVED` |
| [`backend-http-transport.md`](backend-http-transport.md) | Router, middleware, limits, timeouts, response shape | 03 | `APPROVED` (amended post-approval — static asset serving FR-8, `0032`, re-confirmed 2026-08-14) |
| [`backend-persistence.md`](backend-persistence.md) | PostgreSQL connection lifecycle, migrations, repositories, transactions, corruption | 03 | `APPROVED` (amended post-approval twice — DSN redaction `0028`, container topology `0043`, both need re-confirmation) |
| [`backend-test-harness.md`](backend-test-harness.md) | Integration harness, fixtures, controllable clock, CI | 03 | `APPROVED` (amended post-approval — container-target test coverage, `0045`, needs re-confirmation) |
| [`frontend-tooling.md`](frontend-tooling.md) | Bundler, lint/format/typecheck, bundle budget, CI hooks | 04 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`frontend-design-tokens.md`](frontend-design-tokens.md) | Token taxonomy, extraction process, dark-palette decision | 04 | `APPROVED` (amended post-approval — `tokens.css` second output, `0032`, re-confirmed 2026-08-14) |
| [`frontend-component-primitives.md`](frontend-component-primitives.md) | Every primitive: states, a11y contract, Radix-vs-hand-built split | 04 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`frontend-generated-covers.md`](frontend-generated-covers.md) | Layer composition, seeding, degradation ladder, performance budget | 04 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`frontend-shell-and-routing.md`](frontend-shell-and-routing.md) | Shell composition, router/data-fetching libraries, capability gating, mock strategy | 04 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`frontend-accessibility.md`](frontend-accessibility.md) | Keyboard map, focus order, screen-reader text conventions, axe CI gate | 04 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`desktop-host-process-model.md`](desktop-host-process-model.md) | Binary location, spawn, health, mid-session crash recovery, shutdown | 05 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`desktop-host-ipc-surface.md`](desktop-host-ipc-surface.md) | Enumerated preload API, validation, addition/review process | 05 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`desktop-host-window-and-serving.md`](desktop-host-window-and-serving.md) | Window construction, token-synced loading asset, external-link interception | 05 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`backend-library-api.md`](backend-library-api.md) | `/api/v1/library`, `/works/:id`, `/collections` endpoints, pagination, contract test tool | 06 | `APPROVED` (amended post-approval — `formats` field for phase 11, `0038`, re-confirmed 2026-08-15) |
| [`frontend-library-screens.md`](frontend-library-screens.md) | Library home, work detail: browse/search/filter/sort, real backend | 06 | `APPROVED` (amended post-approval — "Read" entry point for phase 11, `0038`, re-confirmed 2026-08-15) |
| [`frontend-collections-screens.md`](frontend-collections-screens.md) | Collections index/detail, create/rename/delete, membership | 06 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| [`backend-metadata-adapter.md`](backend-metadata-adapter.md) | Open Library client, rate limiting, normalisation, `/api/v1/discover*` | 07 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`backend-metadata-caching.md`](backend-metadata-caching.md) | Local caching of normalised metadata and cover images | 07 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`frontend-discover-screen.md`](frontend-discover-screen.md) | Discover screen: search, results, work detail, offline/degraded states | 07 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`backend-source-adapter.md`](backend-source-adapter.md) | Source CRUD, credential encryption, health checks, local-folder/OPDS providers, `/api/v1/sources*` | 08 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`frontend-source-management.md`](frontend-source-management.md) | Source management screen: add/edit/remove, health status, browse/search | 08 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`backend-job-queue.md`](backend-job-queue.md) | Job storage, enqueue/dequeue, worker pool, retry/backoff, dead-letter, status API | 09 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`backend-file-extractors.md`](backend-file-extractors.md) | Format detection, EPUB/PDF/CBZ extraction, adversarial-file defences | 10 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`backend-import-pipeline.md`](backend-import-pipeline.md) | Discovery, import job handler, matching, auto-accept, confirm/reject, SourceOffering/LibraryEntry persistence | 10 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`frontend-import-confirmation.md`](frontend-import-confirmation.md) | Resolution UI: pending candidates, suggested matches, confirm/reject, batch progress | 10 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`backend-reader-content.md`](backend-reader-content.md) | EPUB content resolution, sanitisation, sandbox-supporting response headers | 11 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`backend-reading-api.md`](backend-reading-api.md) | ReadingProgress/Bookmark/Highlight/ReadingPreferences persistence, `/api/v1/reading*` | 11 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`frontend-reader.md`](frontend-reader.md) | Reader UI: rendering engine, pagination/scrolling, typography, TOC, bookmarks/highlights | 11 | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| [`deployment-container-packaging.md`](deployment-container-packaging.md) | Dockerfile, docker-compose.yml, container-target health checks and CI guard | 03 | `REVIEWED` (self, approved with changes) |
