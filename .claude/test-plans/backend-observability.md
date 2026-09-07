# Test plan: Backend observability

| | |
|---|---|
| **Spec** | `.claude/specs/backend-observability.md` |
| **Status** | `DRAFT` |
| **Created** | 2026-09-07 |

## What we are trying to be confident about

1. Credentials, JWT tokens, home filesystem paths, book titles, and personal reading coordinates never appear in any log record or metric label across real execution paths.
2. In-process `expvar` metrics report accurate request latency distributions, queue depths, and connection pool state without external telemetry or excessive request path overhead.
3. `GET /api/v1/diagnostics` and `GET /api/v1/activity/events` are strictly gated to the `admin` role; requests with `reader` role tokens receive HTTP 403 Forbidden.
4. `system_events` table enforces multi-tenant library isolation (`WHERE library_id = $1 OR library_id IS NULL`) and foreign keys match existing `TEXT` primary keys without schema errors.
5. The background retention reaper reliably purges records older than `ACTIVITY_RETENTION_DAYS` (default 30 days) and logs failures gracefully without interrupting server operation.
6. Job action endpoints (`pause-all`, `cancel`, `retry`, `clear-completed`) safely mutate job states and update event feeds.

## Risk assessment

**Hardest to get right:**
- Redaction verification across all system layers. A superficial test asserting logger configuration flags will not prevent an unredacted log line in an error-handling path. The test must execute real end-to-end flows with injected marker strings and inspect captured `slog.Record` slices via `internal/testutil/slogspy.go`.
- Queue depth metric efficiency. Polling `ListJobs` on every scrape risks pulling thousands of job records into memory. The count query must use an efficient `Store.CountByState` aggregation.
- Foreign key type matching. All referenced IDs in the database schema (`jobs.id`, `libraries.id`, `users.id`) are `TEXT`. Mismatched foreign keys (`BIGINT` or `UUID`) will cause migration failures.

**Merely tedious:**
- Table-driven latency bucket calculations.
- Schema migration verification.
- HTTP contract schema validation for diagnostics and activity feeds.

## Layers

### Unit

- `LatencyHistogram`: table-driven tests verifying count, sum, min, max, and percentiles (p50, p95, p99) under synthetic duration streams.
- `SystemEventWriter`: unit tests verifying event payload serialization and refusal of payloads containing prohibited keys (`token`, `password`, `secret`).
- Retention cutoff calculation: tests verifying `purge_at` calculation across varied `ACTIVITY_RETENTION_DAYS` configurations.

### Integration

All integration tests run against real PostgreSQL using the test harness (`backend-test-harness.md`).

- **Migration 00012:** verifies creation of `system_events` table with `TEXT` foreign keys and required partial indexes.
- **Diagnostics endpoint (`GET /api/v1/diagnostics`):**
  - Unauthenticated request -> HTTP 401.
  - Reader role token -> HTTP 403.
  - Admin role token -> HTTP 200 with valid metrics, runtime, uptime, and build version.
  - Asserts response contains zero usernames, emails, or user IDs.
- **Activity event queries (`GET /api/v1/activity/events`):**
  - Tenant isolation: admin querying Library A does not see events belonging exclusively to Library B. Host-level events (`library_id IS NULL`) are visible.
- **Job action endpoints:**
  - `POST /api/v1/activity/pause-all` succeeds for admin, fails with 403 for reader.
  - `POST /api/v1/activity/jobs/:id/cancel` sets job status to `dead_letter` and cancels worker context.
  - `POST /api/v1/activity/jobs/:id/retry` inserts fresh job with `attempts = 0`.
  - `POST /api/v1/activity/jobs/clear-completed` removes completed jobs for the active library.
- **Retention reaper worker:**
  - Seeds rows with `purge_at` in the past and future.
  - Runs reaper sweep; asserts expired rows are deleted and future rows remain intact.
  - Simulates database query failure; asserts error is logged at `warn` level and ticker does not panic.

### Contract

- OpenAPI schema validation asserting contract compliance for `GET /api/v1/diagnostics` and `GET /api/v1/activity/events`.

### End to End & Redaction Verification (RED Gate)

- **`TestObservability_Redaction` (T1.6):**
  - Configures an integration test server using `testutil.NewSpyHandler()`.
  - Executes four real operational paths with distinct canary markers:
    1. Authenticate with username and password (`TEST-PASSWORD-CANARY`).
    2. Synchronize source with basic auth credentials (`TEST-SOURCE-SECRET`).
    3. Import book with title (`TEST-BOOK-TITLE-CANARY`).
    4. Write a `system_events` record.
  - Inspects all captured `slog.Record` instances. Asserts that none of the canary strings, JWT tokens, or reading progress keys appear in message bodies or structured attributes.
  - Must be observed to fail when an unredacted log call is introduced.

## Adversarial cases

| Input / Scenario | Expected behaviour |
|---|---|
| Request with role `reader` accesses `/api/v1/diagnostics` | HTTP 403 Forbidden; audit event logged at `warn` |
| Caller sends negative `limit` to `/api/v1/activity/events?limit=-5` | HTTP 400 Bad Request or clamped to default 50 |
| Caller sends enormous `limit=999999` | Clamped to maximum 100 |
| Malicious error payload wrapping basic auth password in job handler | Error string truncated and sanitized; canary never logged |
| Event insertion attempted while database is temporarily unavailable | Error logged at `warn`; calling job worker does not panic |
| Job cancel request for non-existent job ID | HTTP 404 Not Found |
| Job retry request for already running job | HTTP 409 Conflict |

## Fixtures and test data

- Standard migration test harness with migrated schema up to `00012`.
- Synthetic test library (`00000000-0000-0000-0000-000000000001`) and test users (admin and reader).
- Canary secret strings generated deterministically using test identifiers.

## What is deliberately not tested

- Long-term memory leaks in Go runtime heap statistics (relies on Go runtime `runtime.ReadMemStats` correctness).
- Kernel-level microsecond clock jitter in latency calculations.
- Direct hardware failure of PostgreSQL disk during reaper execution.

## Exit criteria

- [ ] Every functional requirement in `backend-observability.md` has a corresponding test.
- [ ] Automated redaction test fails when unredacted values are logged and passes when redaction is active.
- [ ] Diagnostics authorization test proves reader token receives 403.
- [ ] Database migration `00012` applies cleanly and rolls back cleanly.
- [ ] The suite passes with `go test -race ./...`.
