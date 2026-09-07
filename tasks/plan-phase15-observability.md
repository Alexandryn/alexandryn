# Implementation Plan: Phase 15 — Observability

## Overview

Build the observability layer that ties together everything phases 09–14 added:
in-process `expvar` metrics (request latency, job queue depth, DB pool
utilisation), a `/api/v1/diagnostics` endpoint (admin-only, backend-only, no
UI), a `system_events` activity log store, the Activity screen backed by real
job state, a library-visible finished/leaderboard feature with a named privacy
test, and an automated CI test that proves log redaction is real, not assumed.

Authoritative scope: `.claude/roadmap/15-observability/README.md`. Gate 0
decisions recorded there (§"Gate 0 decisions"). ADRs: 0030 (Accepted), 0031
(Proposed), 0032 (Proposed). Task checklist: `tasks/todo-phase15-observability.md`.

Branch: `feat/phase15-observability`, cut from `origin/main` (`454af67`).
Never `main`.

## Dependency on existing code

| What | Where | This phase uses it for |
|---|---|---|
| Job engine + `Queue.ListJobs` / `GetJob` | `internal/jobs/` | Reading job state for the Activity screen API and the `system_events` writer |
| `backend-errors-and-logging.md` redaction contract | `internal/transport/http/`, `internal/jobs/` | The redaction test's baseline — it asserts the contract is actually enforced |
| `PairedDevice`, `SyncMiddleware`, RBAC middleware | `internal/domain/`, `internal/transport/http/` | Admin-role gate on diagnostics + activity endpoints |
| `reading_progress` table with `user_id`, `library_id`, `percentage` | `internal/persistence/postgres/` | Leaderboard aggregate: `WHERE library_id = $1 AND percentage >= 100` |

## Tiers

### Tier 0 — ADRs (before any spec)

Draft and accept ADR 0030 (expvar — already decided), then produce proposals for
ADR 0031 (activity log store model + retention + LAN access) and ADR 0032
(redaction test design). Surface both proposals at Gate 1 alongside the spec
scopes. ADRs 0031 and 0032 move to Accepted when the maintainer approves the
scope of the specs that depend on them.

### Tier 1 — Backend observability (backend-observability.md)

Dependency: ADR 0030 (Accepted), ADR 0031 (Accepted), ADR 0032 (Accepted).

Sub-tasks in order:
- T1.1 — `expvar` collector: request latency histogram by route (middleware
  hook), job queue depth gauges (polled from `Queue.ListJobs`), DB pool
  utilisation (polled from `pgxpool.Pool.Stat()`). Zero new Go modules.
- T1.2 — `system_events` migration (migration `00012`): event type, job ID FK
  (nullable), library ID FK (nullable), payload JSONB, created_at, purge_at.
- T1.3 — `SystemEventWriter`: called by the job engine's completion/failure
  hooks and by the import pipeline on enqueue. Writes one row per transition.
  No credential, token, title, or position in any payload field.
- T1.4 — Retention reaper: background goroutine (or ticker reusing the phase-09
  engine's shutdown discipline) that `DELETE FROM system_events WHERE purge_at < now()`.
  Logs `warn` on failure, not panic.
- T1.5 — `GET /api/v1/diagnostics`: admin-only, returns `expvar` snapshot +
  uptime + Go runtime stats + build version. Response shape defined in the spec.
- T1.6 — **Redaction CI test** (RED first): integration test that runs an import
  job and an auth path, captures log output, and asserts no violation pattern
  appears. This is the RED test — it must fail before T1.3/T1.5 are implemented.
- T1.7 — `GET /api/v1/activity/events`: admin-only, returns the last N
  `system_events` rows for the authenticated user's active library.

Commit per sub-task. Each sub-task's test must fail before implementation.

### Tier 2 — Activity screen (frontend-activity-screen.md)

Dependency: T1.7 live.

Sub-tasks:
- T2.1 — `ActivityScreen` component with Acquisition tab only (no Reading tab
  label rendered — G0-1). Acquisition tab: ACTIVE / QUEUED / FAILED / COMPLETED
  sections, matching the canvas layout exactly.
- T2.2 — Wire to `GET /api/v1/activity/events`. Polling interval defined in
  spec (canvas does not specify; 10 seconds is the proposal — decision surfaces
  at Gate 1).
- T2.3 — Pause-all, Cancel, Clear-completed actions (endpoints defined in spec).
  Failed items: "Fix source" link navigates to Sources, "Retry" calls re-enqueue
  endpoint.
- T2.4 — Nav badge: orange dot present when at least one item is ACTIVE or FAILED.
- T2.5 — Accessibility: keyboard-navigable list, each action button keyboard-
  reachable, status updates announced, badge has accessible name (e.g.
  "Activity — action required").

### Tier 3 — Leaderboard (backend-reading-leaderboard.md)

Dependency: Tier 1 complete. Spec independently drafted and approved.

Sub-tasks:
- T3.1 — `GET /api/v1/library/finished`: returns, for each work in the active
  library, an array of library members who have `percentage >= 100`. No
  position, chapter, percentage value, or time-remaining in the response.
- T3.2 — **Privacy test** (RED first): a test that queries the endpoint and
  asserts the response contains no field other than "who finished this work" —
  specifically that no fractional-percentage, position, or chapter value is
  present, including in partial responses and error paths.
- T3.3 — `GET /api/v1/library/leaderboard`: returns works ranked by finished-
  count within the active library. Same privacy constraints.
- T3.4 — Leaderboard IDOR test: user A in library X cannot see user B in library
  Y's finished status.

### Tier 4 — Review, audit, close

- Code review (two passes per constitution §10).
- Security audit (`0015-phase15-observability.md`): four-attacker pass on the
  diagnostics endpoint, the activity events endpoint, the leaderboard aggregate,
  and the redaction test itself (is the test testing the right paths?).
- Gate 2: post audit findings + severities; wait for maintainer sign-off.
- CI green across Frontend / Backend / Desktop before reporting done.

## Architecture decisions locked at open

| ADR | Decision | Status |
|---|---|---|
| 0030 | `expvar` for in-process metrics | Accepted (G0-4) |
| 0031 | Activity log store: schema, retention, LAN access | Proposed → surfaces at Gate 1 |
| 0032 | Redaction test: violation patterns, capture mechanism, fail mode | Proposed → surfaces at Gate 1 |

## Risks

| Risk | Mitigation |
|---|---|
| Redaction test catches violations only on the paths it runs, missing others | ADR 0032 decides the minimum path set (import + auth + sync); spec names any gaps as deliberately-not-tested |
| Leaderboard privacy line violated by a poorly scoped query | RED first: privacy test fails before T3.1 is written |
| `system_events` write on every job transition adds latency to the job engine's hot path | Writer is async (goroutine with a channel); job engine doesn't wait on the write |
| `expvar` endpoint exposed without the admin role check | Diagnostics route mounted behind the same `RequireRole(admin)` middleware as other admin routes; IDOR test at Tier 1 Gate |
