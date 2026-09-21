# Spec: Backend observability

| | |
|---|---|
| **Status** | `APPROVED` |
| **Phase** | `15-observability` |
| **Author** | Claude (Sonnet 5), approved by maintainer |
| **Created** | 2026-09-07 |
| **Last updated** | 2026-09-07 |
| **Supersedes** | — |
| **Reviewed in** | Self-review against Gate 1 maintainer-approved scope |
| **Design reference** | `Alexandryn-Electron.dc.html`, synced 2026-08-13 per `.design-reference/ANALYSIS.md` (screen `atActivity` for activity feed and job action controls). For `/api/v1/diagnostics`: `N/A` (backend-only, no canvas per G0-2). |

## Context

Phase 03 established basic backend logging, request correlation IDs, and unauthenticated `/healthz` and `/readyz` probes. Phases 09 through 14 introduced asynchronous background job execution (`internal/jobs`), network exposure and administrative authentication (`internal/auth`, `internal/transport/http`), and cross-device synchronization state.

The system lacks operational visibility: operators and desktop host administrative consumers cannot observe HTTP request latency distributions, background queue depths, or database connection pool saturation. Furthermore, while background jobs perform file extraction and source catalog synchronizations, there is no persistent history of job and ingestion lifecycle events to power the desktop host's Activity screen (`atActivity`). Finally, Constitution §8 demands automated proof that credentials, tokens, filesystem layouts, book titles, and personal reading positions never leak into log records or metric labels.

ADR 0030 fixes in-process metrics on Go standard library `expvar`. ADR 0031 fixes a single `system_events` table with JSONB payloads, background retention reaping, and admin-only access. ADR 0032 fixes the redaction verification test using `log/slog` and `internal/testutil/slogspy.go`.

## Problem

Administrators have no programmatic means to inspect system performance, queue saturation, or pool health without external tooling. The Activity screen has no backend event ledger or control API to display and manage background acquisitions. System log output lacks an automated integration test verifying that sensitive credentials, tokens, book titles, and reading data are never emitted.

## Goals

- Collect in-process HTTP request latency histograms, background job queue depths, and PostgreSQL pool gauges via `expvar` with zero external dependencies.
- Provide an authenticated, admin-only `GET /api/v1/diagnostics` endpoint returning operational metrics, Go runtime metrics, uptime, and build metadata.
- Implement a `system_events` table and an internal `SystemEventWriter` recording background job and import events.
- Implement an automated background retention reaper purging expired system events past `ACTIVITY_RETENTION_DAYS`.
- Provide an authenticated, admin-only `GET /api/v1/activity/events` endpoint serving library-scoped and host-level events for the desktop host Activity screen.
- Provide administrative job management endpoints (`pause-all`, `cancel`, `retry`, `clear-completed`) backing the Activity screen controls.
- Implement an automated CI redaction test proving that credentials, session tokens, user home paths, book titles, and reading progress never appear in log records or metric labels.

## Non-goals

- Any external telemetry push or collector integration (Prometheus remote-write, OpenTelemetry, Datadog). All metrics remain in-process and pull-based.
- Public or unauthenticated diagnostics interfaces. The Phase 03 `/healthz` probe remains untouched for container and process liveness.
- User-facing UI for diagnostics (Gate 0 decision G0-2: backend only).
- Reading activity event feeds or social reading leaderboards (owned by `backend-reading-leaderboard.md`).
- Activity feed access for non-admin LAN clients (`RoleReader`). Operational events are restricted to administrators.

## User stories

- As an **administrator or desktop host**, I want to query `GET /api/v1/diagnostics`, so that I can inspect request latency, memory usage, queue depth, and database pool health in one call.
- As the **desktop host Activity screen**, I want to fetch recent acquisition and job events via `GET /api/v1/activity/events`, so that I can render the ACTIVE, QUEUED, FAILED, and COMPLETED sections shown in `atActivity`.
- As an **administrator**, I want to pause active acquisitions, retry failed jobs, or clear completed jobs from the Activity screen, so that I can control background ingestion.
- As a **security auditor and user**, I want automated CI proof that private credentials, tokens, book titles, and reading coordinates never reach log streams, so that privacy is guaranteed by regression tests rather than assumption.

## Functional requirements

### Metrics and Diagnostics

- **FR-1** The system MUST collect in-process metrics using the Go standard library `expvar` package, registered under a private namespace rather than exposing the default unauthenticated `/debug/vars` HTTP handler.
- **FR-2** The HTTP middleware chain MUST observe the duration of completed HTTP requests and record latencies into route-sharded histograms bucketed by route template (e.g., `/api/v1/library`, `/api/v1/works/:id`), tracking count, sum, and approximate percentiles (p50, p95, p99). Static asset requests under `/assets/*` MUST NOT pollute API route histograms.
- **FR-3** The metrics subsystem MUST expose PostgreSQL connection pool statistics polled from `pgxpool.Pool.Stat()` at read time: `acquired_conns`, `idle_conns`, `total_conns`, and `max_conns`.
- **FR-4** The job queue subsystem (`internal/jobs`) MUST provide a method `CountByState(ctx context.Context) (map[State]int, error)` executing an optimized `COUNT(*) ... GROUP BY status` query against the `jobs` table. The metrics subsystem MUST expose current job counts for states `queued`, `running`, `retrying`, and `dead_letter`.
- **FR-5** The server MUST provide `GET /api/v1/diagnostics`. The endpoint MUST require valid authentication (`AuthMiddleware`) and the `admin` role (`RequireRole(domain.RoleAdmin)`). Requests with valid credentials but role `reader` MUST receive HTTP 403 Forbidden. Unauthenticated requests MUST receive HTTP 401 Unauthorized.
- **FR-6** The `GET /api/v1/diagnostics` response MUST return a JSON object with keys `metrics` (latencies, queue depths, pool stats), `runtime` (goroutines, heap alloc bytes, total alloc bytes, GC cycles), `uptime_seconds`, and `version` (commit hash and build time). The response MUST NOT contain any usernames, email addresses, library names, or user identifiers.

### Activity Event Store and Reaper

- **FR-7** Database migration `00012` MUST create the `system_events` table:
  ```sql
  CREATE TABLE system_events (
      id          BIGSERIAL PRIMARY KEY,
      event_kind  TEXT NOT NULL,
      job_id      TEXT REFERENCES jobs(id) ON DELETE SET NULL,
      library_id  TEXT REFERENCES libraries(id) ON DELETE CASCADE,
      user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
      payload     JSONB NOT NULL DEFAULT '{}',
      created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
      purge_at    TIMESTAMPTZ NOT NULL
  );
  CREATE INDEX system_events_library_created_idx ON system_events (library_id, created_at DESC);
  CREATE INDEX system_events_purge_idx ON system_events (purge_at);
  CREATE INDEX system_events_host_created_idx ON system_events (created_at DESC) WHERE library_id IS NULL;
  ```
- **FR-8** The system MUST provide an internal `SystemEventWriter` interface. Job lifecycle hooks in `internal/jobs` (enqueue, claim, retry, dead-letter, complete) and import workflows in `internal/importer` MUST emit events through this interface.
- **FR-9** Event payloads in `system_events.payload` MUST be JSON-serializable and MUST NOT contain plain text passwords, authentication tokens, session secrets, book titles, or reading positions. Allowed fields are limited to IDs, error reason categories, byte counts, and item counts.
- **FR-10** The server MUST run an automated background retention reaper on a periodic 1-hour interval. The reaper MUST execute:
  ```sql
  DELETE FROM system_events WHERE purge_at < $1;
  ```
  where `$1` is the current time. `purge_at` MUST be calculated at insertion time as `created_at + (ACTIVITY_RETENTION_DAYS * 24 hours)`. The default retention window MUST be 30 days if `ACTIVITY_RETENTION_DAYS` is unset or zero. If the reaper query fails, it MUST log a warning at `warn` level with the database error and MUST NOT terminate server execution.

### Activity Feed and Job Action Endpoints

- **FR-11** The server MUST provide `GET /api/v1/activity/events`. The endpoint MUST require valid authentication and the `admin` role. The query MUST filter by `WHERE (library_id = $1 OR library_id IS NULL)` where `$1` is the active library ID resolved from request context (`X-Library-Id` or session default). The endpoint MUST support pagination via query parameter `limit` (default 50, maximum 100).
- **FR-12** The server MUST provide the following administrative job actions for the Activity screen:
  - `POST /api/v1/activity/pause-all`: sets a global acquisition pause flag or pauses active background workers; active jobs report paused status.
  - `POST /api/v1/activity/jobs/:id/cancel`: cancels a queued or running job if not in a terminal state, releasing locks and transitioning the job to `dead_letter` with error reason `cancelled_by_admin`.
  - `POST /api/v1/activity/jobs/:id/retry`: re-enqueues a failed or dead-lettered job by inserting a new job record with `attempts = 0` and initial available timestamp.
  - `POST /api/v1/activity/jobs/clear-completed`: deletes or flags completed job records older than 1 hour for the active library.
  All action endpoints MUST require `admin` role authorization.

### Redaction CI Verification

- **FR-13** The test suite MUST include a dedicated integration test executed with a `testutil.SpyHandler` capturing standard library `log/slog` output across the entire server process.
- **FR-14** The redaction test MUST exercise four concrete execution paths with known injected markers:
  1. Source catalog synchronization with basic auth credentials.
  2. User login yielding JWT access and refresh tokens.
  3. Import ingestion of a test book with a known title.
  4. Insertion of a `system_events` row.
- **FR-15** The redaction test MUST inspect all captured `slog.Record` messages and attributes, failing the test (`t.Errorf`) if any record matches:
  - Password or basic authentication credential patterns.
  - JWT-shaped strings (three base64url segments separated by `.`).
  - Home directory path prefixes (`/Users/`, `/home/`, `/root/`).
  - Reading coordinate keys (`position`, `cfi`, `location`, `chapter`, `percentage`) combined with scalar values.
  - Verbatim imported book titles.

## Non-functional requirements

- **Performance:** Recording a request duration in the HTTP middleware MUST use atomic counters or pre-allocated buckets with sub-microsecond overhead (< 5 μs per request). Polling `pgxpool.Stat()` during `GET /api/v1/diagnostics` MUST NOT execute SQL queries. `CountByState` on `jobs` MUST complete within 20 ms under 10,000 job rows.
- **Security:** Diagnostics and activity endpoints MUST enforce strict role-based access control. All queries against `system_events` MUST enforce tenant separation via `library_id`.
- **Accessibility:** Not applicable (headless backend endpoints).
- **Reliability:** Failure in the background retention reaper MUST NOT disrupt request handling or job processing. Transient database errors during event writing MUST NOT cause job worker panics.
- **Observability:** Event reaper executions and purged row counts MUST log at `debug` level. Reaper database failures MUST log at `warn` level.

## Domain model

This specification introduces no new aggregates into `internal/domain`.
- `SystemEvent`: persistence-layer model representing an audit/activity event:
  - `ID`: int64
  - `EventKind`: string (`job.enqueued`, `job.running`, `job.completed`, `job.failed`, `job.dead_letter`, `import.started`, `import.finished`, `source.sync`)
  - `JobID`: nullable string (references `jobs.id`)
  - `LibraryID`: nullable string (references `libraries.id`)
  - `UserID`: nullable string (references `users.id`)
  - `Payload`: map[string]any
  - `CreatedAt`: time.Time
  - `PurgeAt`: time.Time

## API and contracts

### `GET /api/v1/diagnostics`

- **Headers:** `Authorization: Bearer <token>`
- **Response 200 OK:**
  ```json
  {
    "uptime_seconds": 86400,
    "version": {
      "commit": "cec9589",
      "build_time": "2026-09-07T03:45:22Z"
    },
    "runtime": {
      "goroutines": 42,
      "heap_alloc_bytes": 16777216,
      "total_alloc_bytes": 67108864,
      "gc_cycles": 128
    },
    "metrics": {
      "latencies": {
        "/api/v1/library": {
          "count": 1200,
          "sum_ms": 14400,
          "p50_ms": 10.2,
          "p95_ms": 25.4,
          "p99_ms": 45.1
        }
      },
      "queue_depth": {
        "queued": 4,
        "running": 2,
        "retrying": 1,
        "dead_letter": 0
      },
      "db_pool": {
        "acquired_conns": 3,
        "idle_conns": 7,
        "total_conns": 10,
        "max_conns": 20
      }
    }
  }
  ```
- **Response 401 Unauthorized:** unauthenticated.
- **Response 403 Forbidden:** authenticated user lacks `admin` role.

### `GET /api/v1/activity/events`

- **Headers:** `Authorization: Bearer <token>`, `X-Library-Id: <library_id>`
- **Query Parameters:** `limit` (integer, default 50, max 100)
- **Response 200 OK:**
  ```json
  {
    "events": [
      {
        "id": 1042,
        "event_kind": "job.completed",
        "job_id": "job-uuid-1",
        "library_id": "lib-uuid-1",
        "created_at": "2026-09-07T03:30:00Z",
        "payload": {
          "kind": "import.process",
          "items_total": 1,
          "duration_ms": 1420
        }
      }
    ]
  }
  ```

### Job Actions

- `POST /api/v1/activity/pause-all` -> `200 OK` `{"paused": true}`
- `POST /api/v1/activity/jobs/{id}/cancel` -> `200 OK` `{"cancelled": true}`
- `POST /api/v1/activity/jobs/{id}/retry` -> `200 OK` `{"new_job_id": "job-uuid-2"}`
- `POST /api/v1/activity/jobs/clear-completed` -> `200 OK` `{"cleared_count": 5}`

## State transitions

`system_events` records state transitions of asynchronous jobs governed by `internal/jobs`:
```
queued -> running -> completed (job.completed written)
                  -> retrying  (job.failed written)
                  -> dead_letter (job.dead_letter written)
```
The `system_events` rows are immutable audit records; once written, rows transition only to purged status via the background retention reaper.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Non-admin requests diagnostics | Middleware role check | HTTP 403 `{"error": "forbidden"}` | Request rejected, logged at `warn` |
| Database pool stats fail | N/A (`Stat()` is in-memory) | Diagnostics returned normally | N/A |
| `CountByState` query times out | SQL context deadline | Diagnostics returns `queue_depth: null` | Logged at `error`, partial response served |
| Event insertion fails | Database error return | Background job continues normally | Error logged at `warn`; job lifecycle unaffected |
| Event reaper query fails | Cron ticker error check | None (background task) | Error logged at `warn`; retries next hour |
| Cross-tenant event access | Query `WHERE library_id = $1` | Events filtered to caller's library | Events from other libraries excluded |

## Security considerations

- **STRIDE Threat Modeling:**
  - *Spoofing:* All endpoints require JWT authentication.
  - *Tampering:* System events are append-only. No HTTP mutation endpoint exists for events.
  - *Repudiation:* Critical job lifecycle changes are captured in `system_events`.
  - *Information Disclosure:* Diagnostics and activity feeds are strictly gated to `admin`. The redaction test guarantees that credentials, tokens, book titles, and reading positions never escape into log streams or metric labels.
  - *Denial of Service:* Latency tracking uses bounded memory structures. The event reaper prevents unbounded table growth. Diagnostics route is subject to standard API rate limits.
  - *Elevation of Privilege:* Reader tokens attempting to access `/diagnostics` or `/activity/*` receive HTTP 403.
- **Enforcement Point:** Handlers verify `UserFromContext(ctx).Role == RoleAdmin`. Database repository queries enforce `WHERE (library_id = $1 OR library_id IS NULL)`.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Latency histogram calculation and bucket allocations, `SystemEvent` struct serialization, retention date cutoff math. |
| Integration | `GET /api/v1/diagnostics` schema validation and 403/401 checks, `SystemEventWriter` persistence in PostgreSQL, retention reaper cleanup verification. |
| Contract | OpenAPI contract assertions for `/diagnostics` and `/activity/events`. |
| Redaction (RED) | Integration test using `internal/testutil/slogspy.go` verifying that running auth, source sync, import, and event logging emits zero credentials, tokens, home paths, book titles, or reading positions. |

## Acceptance criteria

- [ ] In-process metrics registry exposes request latency histograms, queue depth by state, and database pool stats without external collectors.
- [ ] `Store.CountByState` executes a single aggregate query on `jobs` returning counts per state.
- [ ] `GET /api/v1/diagnostics` returns accurate server statistics and requires the `admin` role.
- [ ] Requests to `/diagnostics` with `RoleReader` receive HTTP 403 Forbidden.
- [ ] Migration `00012` creates `system_events` with `TEXT` foreign keys and required indexes.
- [ ] `SystemEventWriter` records job and import transitions with sanitised payloads.
- [ ] Background retention reaper automatically purges expired `system_events` past `ACTIVITY_RETENTION_DAYS`.
- [ ] `GET /api/v1/activity/events` returns library-scoped event history for administrators.
- [ ] Job action endpoints (`pause-all`, `cancel`, `retry`, `clear-completed`) function and require the `admin` role.
- [ ] Redaction-proof integration test fails when unredacted strings are logged and passes when redaction invariants are satisfied.

## Open questions

- **Histogram bucket tuning:** Initial latency buckets are set at 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1000ms, 2500ms. These are empirical placeholders subject to operational adjustment.
- **Acquisition pause implementation:** Whether `pause-all` temporarily suspends worker pool dequeue loops or sets an in-memory pause flag in `internal/jobs/engine.go` is left to implementation detail.

## References

- ADR 0030: In-process metrics via `expvar`
- ADR 0031: Activity log store schema and retention
- ADR 0032: Redaction-proof test design
- `docs/specs/backend-job-queue.md`: Job engine and store interfaces
- `internal/testutil/slogspy.go`: Structured logging test spy
- Constitution §8: Observability without sensitive data leakage
