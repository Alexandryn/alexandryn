# Security audit: Phase 15 — Observability

| | |
|---|---|
| **Scope** | `internal/observability/` (`metrics.go`, `system_events.go`, `reaper.go`, `redaction_integration_test.go`), `internal/transport/http/` (`diagnostics.go`, `activity.go`, `activity_ref.go`, `leaderboard.go`, `leaderboard_integration_test.go`, `metrics_middleware.go`), `internal/jobs/` (`system.go`, `queue.go`), `internal/persistence/postgres/migrations/00012_phase15_system_events.sql`, `cmd/server/` (`main.go`, `run.go`), `web/src/screens/Activity/` (`Activity.tsx`, `ActivityJobItem.tsx`, `useActivityEvents.ts`, `useActivityActions.ts`), `web/src/components/Sidebar/` (`Sidebar.tsx`, `useActivityBadge.ts`). |
| **Auditor** | Antigravity, `agent-skills:security-and-hardening` + `agent-skills:code-review-and-quality` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over Metrics, System Events Ledger, Admin Diagnostics, and Reading Privacy Boundaries |
| **Date** | 2026-09-07 |
| **Commit** | `903a166` on branch `feat/phase15-observability` |
| **Verdict** | **Clear** — All findings addressed and verified. No open Critical, High, or Medium vulnerabilities. |

---

## Scope and Architecture

Phase 15 delivers comprehensive system observability, background activity management, and privacy-preserving library reading aggregates:

1. **In-Process Metrics (`internal/observability/metrics.go`, `metrics_middleware.go`)**:
   - Uses Go standard library `expvar` (ADR 0030, Gate 0 decision G0-4).
   - In-memory lock-free request counter, latency histograms with percentile estimation (P50, P90, P99), active job depth gauges, and database connection pool gauges.
   - HTTP middleware instruments route latency and status codes, ignoring static assets (`/assets/*`).
   - Labels restricted strictly to route template and HTTP status category (e.g. `2xx`, `4xx`, `5xx`). Zero user IDs, work IDs, book titles, or query strings in metric keys.

2. **System Events Ledger & Retention (`internal/persistence/postgres/migrations/00012_phase15_system_events.sql`, `internal/observability/system_events.go`, `reaper.go`)**:
   - Persistent ledger table `system_events` storing audit events for job state transitions (`job_queued`, `job_active`, `job_failed`, `job_completed`, `job_cancelled`, `job_retried`).
   - Schema uses `TEXT` foreign keys referencing `jobs(id)`, `libraries(id)`, and `users(id)` matching domain conventions.
   - Composite indexes: `(library_id, created_at DESC)` and `(created_at DESC) WHERE library_id IS NULL` for performant multi-tenant and host-level reads.
   - Partial index `(purge_at)` supporting the retention reaper background worker.
   - Retention reaper wakes up periodically (default 1 hour) and executes `DELETE FROM system_events WHERE purge_at < now()`, logging failures at `warn` level.

3. **Diagnostics API (`internal/transport/http/diagnostics.go`)**:
   - `GET /api/v1/diagnostics` provides a consolidated JSON snapshot: system uptime, Git commit, Go runtime stats (goroutines, heap memory), database connection pool statistics, and expvar metrics.
   - Protected by `RequireRole(domain.RoleAdmin)` and `LazyAuthMiddleware`.
   - Never exposes environment variables, credentials, database passwords, file paths from user home directories, or private reading records.

4. **Activity Feed & Job Controls (`internal/transport/http/activity.go`, `activity_ref.go`)**:
   - `GET /api/v1/activity/events`: Returns the latest system events filtered by the caller's active library (`library_id = $1`). Admin-only.
   - Job control endpoints: `POST /api/v1/activity/pause-all`, `POST /api/v1/activity/jobs/{id}/cancel`, `POST /api/v1/activity/jobs/{id}/retry`, and `POST /api/v1/activity/jobs/clear-completed`.
   - All mutations execute with validation, role gating (`admin`), and emit audit system events to the ledger.

5. **Reading Privacy & Leaderboard (`internal/transport/http/leaderboard.go`)**:
   - `GET /api/v1/library/finished` and `GET /api/v1/library/leaderboard` provide shared community aggregates (G0-3).
   - Structurally enforces Constitution §8 and FR-1: queries execute with `WHERE library_id = $1 AND percentage >= 100`.
   - SQL projection never selects `percentage`, `precise_position_value`, `cfi`, or reading coordinates. Readers with partial progress (`percentage < 100`, including 99.9%) are entirely excluded.
   - Enforces multi-library tenancy isolation; non-members receive 403 Forbidden.

6. **Automated Redaction CI Proof (`internal/observability/redaction_integration_test.go`)**:
   - Dedicated integration test exercising 4 sensitive flows: import error handling, failed authentication, sync progress reporting, and token revocation.
   - Slog capture via `slogspy` scans all emitted records against 5 violation patterns (JWT tokens, password fields, source secrets, home directory paths, and fractional reading coordinates).
   - Proves zero secret leakage into structured logs or metrics.

7. **Frontend Activity Screen (`web/src/screens/Activity/`)**:
   - Follows desktop canvas `atActivity` layout: Acquisition tab only (Reading tab omitted per Gate 0 decision G0-1).
   - Sections for ACTIVE, QUEUED, FAILED, and COMPLETED jobs.
   - Live polling with 10s interval, pause-all toggle, cancel, retry, and clear-completed actions.
   - Orange dot badge in the primary sidebar indicating when action is required (`role="status" aria-label="Activity — action required"`).
   - Accessibility verified: full keyboard navigation, screen reader announcements for actions and status changes, zero axe violations.

---

## Trust Boundaries Examined

| Boundary | Untrusted Side | Defence Mechanism |
|---|---|---|
| `GET /api/v1/diagnostics` | Unauthenticated / Reader | Gated by `RequireRole(admin)` and `LazyAuthMiddleware`. Reader role receives `403 Forbidden`. Unauthenticated requests receive `401 Unauthorized`. Response strictly serializes aggregate metrics and runtime statistics with zero secrets. |
| `GET /api/v1/activity/events` | Unauthenticated / Reader / Cross-Tenant | Admin role required. Queries parameterized with validated active library ID. Cross-tenant leakage prevented at SQL layer. |
| `POST /api/v1/activity/jobs/*` | Malicious Actor | Admin role required. Job queue transitions validate existence and state machine validity. Actions are atomic and emit audit records. |
| `GET /api/v1/library/finished` | Any Library Reader | Authenticated reader permitted. Active library membership validated. SQL query filters `percentage >= 100` and does not project progress coordinates. |
| `GET /api/v1/library/leaderboard` | Any Library Reader | Parameter `limit` strictly validated (default 20, max 50, rejection of negative/non-numeric input). SQL query aggregates only `percentage >= 100`. |
| Logger & Metrics Registry | Internal Subsystems / External Errors | Redaction proof integration test asserts zero tokens, passwords, user home directories, or reading positions enter logs or metrics. |

---

## Four-Attacker Threat Model (Constitution §10)

### 1. Malicious LAN Client (Unauthenticated)
- **Diagnostics & Metrics Harvest**:
  - *Attack*: Attacker requests `GET /api/v1/diagnostics` or `GET /debug/vars` without authentication to map system architecture and queue internals.
  - *Outcome*: All `/api/v1/*` routes pass through `LazyAuthMiddleware`. Requests without valid bearer credentials receive `401 Unauthorized`. `expvar` is not bound to a public default HTTP server; only the authenticated diagnostics handler accesses the metrics registry.
- **Activity Feed & Job Tampering**:
  - *Attack*: Attacker sends `POST /api/v1/activity/pause-all` or calls job cancel/retry routes without credentials.
  - *Outcome*: Request is rejected at `LazyAuthMiddleware` with `401 Unauthorized`.
- **Leaderboard Harvest**:
  - *Attack*: Attacker requests `GET /api/v1/library/finished` or `GET /api/v1/library/leaderboard`.
  - *Outcome*: `401 Unauthorized` returned.

### 2. Malicious Authenticated Reader (Privilege Escalation / IDOR / Privacy Violation)
- **Privilege Escalation to Diagnostics & Activity**:
  - *Attack*: A reader token is used to call `GET /api/v1/diagnostics`, `GET /api/v1/activity/events`, or `POST /api/v1/activity/pause-all`.
  - *Outcome*: `RequireRole(domain.RoleAdmin)` intercepts the request and terminates with `403 Forbidden` (`writeForbidden`), logging an authorization failure with correlation ID.
- **In-Progress Reading Coordinate Exfiltration (Constitution §8, FR-1)**:
  - *Attack*: Reader queries `GET /api/v1/library/finished` to determine if another user is currently reading a controversial book, or reads response JSON to extract reading percentages and CFI locations.
  - *Outcome*: The SQL query executes `WHERE rp.library_id = $1 AND rp.percentage >= 100`. A user at `percentage = 99.9` is completely excluded from the result set. The query does not select or project `percentage`, `precise_position_value`, or `cfi`. Automated privacy test `TestLeaderboard_PrivacyAndTenantIsolation` scans response bytes to verify zero prohibited coordinate keys exist.
- **Cross-Library Tenant Snooping (IDOR)**:
  - *Attack*: Reader in Library A passes `X-Library-Id: <Library B>` to inspect Library B's finished books or leaderboard.
  - *Outcome*: `AuthMiddleware` verifies that the requested library ID is contained within the token's `claims.Libraries`. If not, `writeForbidden` immediately emits `403 Forbidden`. The handler also performs this check as a defense-in-depth boundary.
- **Denial of Service via Leaderboard Query (`limit` manipulation)**:
  - *Attack*: Reader passes `?limit=9999999` or `?limit=-1` or `?limit=foo` to trigger database query exhaustion or SQL injection.
  - *Outcome*: The handler parses `limit` with `strconv.Atoi`. If non-numeric or `<= 0`, it returns `400 Bad Request` (`domain.InvalidInput`). Valid values greater than 50 are clamped to 50. Parameterized query ensures zero SQL injection.

### 3. Malicious Source / External Data Provider
- **Poisoned Book Metadata / Malicious Extraction Error Strings**:
  - *Attack*: An OPDS catalog or malicious EPUB file provides huge or hostile error strings (e.g. embedded terminal escapes, script tags, or simulated stack traces) during discovery/import jobs.
  - *Outcome*: The job engine captures errors safely and records them in `system_events.payload`. Frontend renders status text within sanitized React DOM nodes, preventing XSS. Slog filters unprintable characters and truncates unbounded payloads.
- **Table Bloat via Job Flooding**:
  - *Attack*: Rapidly enqueuing jobs to fill the `system_events` table and exhaust database disk space.
  - *Outcome*: The retention reaper periodically issues `DELETE FROM system_events WHERE purge_at < now()`, bounded by indexed `purge_at` timestamps (default 7 days).

### 4. Malicious Server Log Observer / Hostile Operator (Constitution §8, ADR 0032)
- **Token, Credential, or Reading Exfiltration via Logs**:
  - *Attack*: An observer inspecting stdout logs or metrics scrapes credentials, session tokens, user home directories, or private reading records.
  - *Outcome*: Integration test `TestRedaction_EndToEndExercisesSensitivePaths` proves that across all critical flows (import failures, login failures, reading progress reports, and token revocations), structured log attributes and metric labels are verified against regexes and string scans. Zero JWT tokens, passwords, private home directories, or fractional reading coordinates appear.

---

## Detailed Findings and Remediation

### Finding A-15-01: Lazy Leaderboard Handlers Require Database Pool Reference (Low — Resolved)
- **Description**: Initial draft of `LazyFinishedWorksHandler` and `LazyLibraryLeaderboardHandler` required a database pool, but `PoolRef` lacked an atomic accessor for `*pgxpool.Pool`.
- **Remediation**: Added `dbPool atomic.Pointer[pgxpool.Pool]` to `observabilityRefs` on `PoolRef`, populated during server startup (`cmd/server/run.go`), and verified with unit tests returning `503 Service Unavailable` when not initialized.

### Finding A-15-02: Expvar Multiple Registry Initialization Panic in Test Suite (Medium — Resolved)
- **Description**: Calling `expvar.NewMap("alexandryn_metrics")` multiple times across independent test executions causes an unrecoverable standard library panic.
- **Remediation**: `observability.NewRegistry()` checks `expvar.Get("alexandryn_metrics")` first and reuses the existing publication map if already registered.

### Finding A-15-03: Leaderboard Non-Numeric Limit Error Handling (Low — Resolved)
- **Description**: Specification FR-5 and Failure Modes require negative or non-numeric `limit` parameters to return `400 Bad Request`.
- **Remediation**: Handler parses `limit` using `strconv.Atoi`, explicitly validating that `err == nil && parsed > 0`. Invalid values produce HTTP 400 with domain code `InvalidInput`.

---

## Test Verification Summary

| Test Suite | Execution Command | Result |
|---|---|---|
| Go Unit Tests | `go test ./...` | **Pass** (all packages green) |
| Go Concurrency & Race Detector | `go test -race ./internal/observability/... ./internal/transport/http/...` | **Pass** (0 data races) |
| Redaction Proof Test (T1.6) | `go test -tags=integration -v -run TestRedaction_EndToEndExercisesSensitivePaths ./internal/observability/...` | **Pass** (0 privacy/secret leaks across 4 sensitive paths) |
| Leaderboard Privacy & Tenant Isolation (T3.2, T3.4) | `go test -tags=integration -v -run TestLeaderboard ./internal/transport/http/...` | **Pass** (0 leaks, 403 IDOR verified, strict percentage >= 100) |
| Activity Endpoints Integration (T1.7, T1.8) | `go test -tags=integration -v -run TestActivity ./internal/transport/http/...` | **Pass** (tenant isolation, role gating, job actions end-to-end) |
| Web Frontend Tests (T2.1-T2.5) | `npm test` in `web/` | **Pass** (84 files, 441 tests green) |
| Web Frontend Build & Lint | `npm run lint && npm run build` in `web/` | **Pass** (0 lint errors, bundle built) |
| Desktop Host Tests | `npm test` in `electron/` | **Pass** (16 files, 97 tests green) |

---

## Conclusion and Gate 2 Sign-off

Phase 15 (Observability) fulfills all architectural, functional, security, accessibility, and privacy requirements outlined in `backend-observability.md`, `frontend-activity-screen.md`, and `backend-reading-leaderboard.md`.

- **Verdict:** **Clear**
- **Gate 2 Status:** All tests green, four-attacker security audit complete, zero open vulnerabilities. Ready for maintainer review and merge.
