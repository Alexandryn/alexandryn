# 0031. Activity log store: schema, retention, and LAN-client read access

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-07 |
| **Deciders** | Maintainer (Gate 1, 2026-09-07) |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 15 adds a `system_events` table that records job lifecycle transitions
(enqueued, started, completed, failed, dead-lettered), import history, and
source acquisition events. This table is the backing store for the Activity
screen and the `/api/v1/activity/events` read endpoint.

Three design questions must be settled before the backend-observability spec is
drafted — they affect the migration schema, the repository interface, and the
read-access rules:

1. **Schema:** a single typed-event table with a JSONB payload, or separate
   typed tables per event kind?
2. **Retention:** who runs the expiry logic, on what schedule, and what is the
   configurable key + default?
3. **LAN-client read access:** can a reader-role LAN user see their own
   acquisition events, or is the Activity feed admin/host-only?

The canvas places `atActivity` on the Electron/Host surface only (no Web or
Mobile equivalent), which lends weight to host-only access — but the spec must
state the decision explicitly, not inherit it from canvas placement alone.

## Decision

### Schema — single typed table with event_kind discriminator and JSONB payload

We use one `system_events` table:

```sql
CREATE TABLE system_events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_kind  TEXT NOT NULL,          -- 'job.enqueued', 'job.completed', 'import.started', etc.
    job_id      TEXT REFERENCES jobs(id) ON DELETE SET NULL,     -- nullable
    library_id  TEXT REFERENCES libraries(id) ON DELETE CASCADE, -- nullable: host-level events have no library
    user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,    -- nullable
    payload     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    purge_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX ON system_events (library_id, created_at DESC);
CREATE INDEX ON system_events (created_at DESC) WHERE library_id IS NULL;
CREATE INDEX ON system_events (purge_at);
```

`job_id`, `library_id`, and `user_id` are `TEXT` — every referenced primary key in
the schema is `TEXT` (`jobs.id`, migration 00006; `libraries.id` and `users.id`,
migration 00009). A `BIGINT` or `UUID` FK column would fail the migration on a
type mismatch. `id` is a database-generated identity because the event log is
append-only and never needs an application-assigned id.

`library_id` is nullable. Host-level events — server start/stop, network rebind,
reaper failure — belong to no library. The Activity screen query filters for the
active `library_id` **plus** `library_id IS NULL` when the viewer is the host
admin; the partial index above serves the host-level feed.

Event kinds are string constants defined in `internal/observability/events.go`.
Payload fields are event-kind-specific and must not contain credentials, tokens,
book content, reading positions, or percentage values. A registry of valid
payload shapes is part of the spec (not enforced at the SQL layer — the
`SystemEventWriter` is the enforcement point).

**Why not separate typed tables:** The event set will grow as new job types are
added (source-sync jobs, future phases). Separate tables would require a
migration for each new event kind, and queries that span kinds (the Activity
screen mixes job events and import events) would need UNION. A single table with
a discriminator and indexed columns supports the Activity screen's mixed-kind
read with one query.

**Why JSONB and not typed columns:** Event-kind-specific fields vary too much to
normalise without a large number of nullable columns. JSONB keeps the table lean
while the `SystemEventWriter` enforces payload shape at write time.

### Retention — a background reaper goroutine seeded by `purge_at`

`purge_at` is set at write time: `created_at + retention_duration`. The
retention duration is read from the server configuration key
`ACTIVITY_RETENTION_DAYS` (integer, default 30). The reaper is a goroutine
started in the server's lifecycle (same pattern as the phase-09 engine's job
reaper), runs once per hour, executes `DELETE FROM system_events WHERE purge_at < now()`,
and logs the count of purged rows at `debug`. On failure it logs at `warn` and
continues — it does not panic or stop the server.

**Why not PostgreSQL `pg_cron`:** `pg_cron` is not available in a standard
PostgreSQL installation without a superuser extension install. The project
targets self-hosted PostgreSQL (ADR 0004) without requiring superuser setup.

**Why not a handler-triggered delete:** Coupling retention to a read request
means no rows are ever purged if the Activity screen is never visited. A
background goroutine runs unconditionally.

### LAN-client read access — host-only (admin role required)

The `/api/v1/activity/events` and `/api/v1/diagnostics` endpoints require the
`admin` role. LAN-paired readers (reader role) cannot reach either endpoint.

Reasons:
- The canvas places `atActivity` on the Electron/Host surface only — no
  Web or Mobile Activity screen exists.
- System events include job failures, import errors, and source names. Exposing
  them to reader-role LAN clients reveals operational detail those clients have
  no need for.
- The leaderboard feature (G0-3) is the correct surface for readers to see
  library-level completion data. The Activity feed is operator/admin data.

This decision does **not** preclude a future phase adding a reader-facing event
feed if a canvas is drawn and scoped for it.

## Options considered

### Option A — single typed table with JSONB ✓ chosen

See Decision above.

### Option B — separate tables per event kind

**Pros:** Strong SQL-level schema per kind; trivially enforced payload shape.
**Cons:** Migration required for each new kind; cross-kind Activity screen queries
use UNION or separate requests; prematurely commits to a closed event vocabulary.
**Why not chosen:** See schema rationale above.

### Option C — reader-role access to own acquisition events

Readers could see events scoped to their own user ID (the books they acquired).
**Pros:** LAN reader can see their own download queue without being admin.
**Cons:** No canvas exists for this; it would require a second read-path with
different scoping logic; increases the audit surface (IDOR risk: reader A
seeing reader B's events). The leaderboard is the right reader-facing surface.
**Why not chosen:** Canvas absence + no defined reader need + IDOR complexity.

## Consequences

**Good:** Single migration, single repository interface, single reaper. The
Activity screen query is one `SELECT ... ORDER BY created_at DESC LIMIT n`
scoped to `library_id = $1 OR library_id IS NULL`.

**Bad:** JSONB payload is not schema-enforced at the SQL layer. The
`SystemEventWriter` is the only enforcement point; a future caller that bypasses
it can write an invalid payload. The spec must name this and require that all
writes go through `SystemEventWriter`.

**Neutral:** `purge_at` is set at write time, so changing `ACTIVITY_RETENTION_DAYS`
only affects new rows, not existing ones. Existing rows purge on their original
schedule. Acceptable at this scale.

## Reversal cost

Medium. Changing from JSONB to typed tables would require a migration and a
rewrite of the repository. Changing retention from application-level to
`pg_cron` would require superuser access on the target PostgreSQL instance.
Neither is likely to be forced by the use case this phase covers.

## Confidence

Medium. The schema choice (JSONB vs. typed tables) is a judgement call. The
access-control choice (admin-only) follows directly from the canvas and the
leaderboard's existence as the reader-facing alternative. If the project adds
reader-facing activity views in a future phase, this ADR will need a targeted
amendment for that access path — it should not be reopened wholesale.
