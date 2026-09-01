-- Phase 09: the background job queue's storage (backend-job-queue.md
-- FR-1, ADR 0014).
--
-- One `jobs` table, claimed by a worker pool via
-- `SELECT ... FOR UPDATE SKIP LOCKED`. No child table: `payload` and
-- `progress` are JSONB because they are opaque to this table — a job's
-- kind decides their shape, and nothing here queries inside them.
--
-- TEXT primary key, matching every other table in this schema (migration
-- 00002's header): the id *value* is a UUID string generated Go-side by
-- internal/idgen through the same domain.IDGenerator the rest of the
-- codebase uses, so tests can inject a deterministic sequence.
--
-- `lease_token` is the fencing token (FR-4/FR-5): regenerated on every
-- claim and every reaper reclaim, it is what makes a delayed worker's
-- heartbeat or completion write hit zero rows after its lease was
-- reassigned. `locked_by` (the worker id) is a diagnostic only — not
-- load-bearing, but it makes a stuck job's owner visible.
--
-- `last_error` is written only through internal/jobs' single
-- truncate-and-redact helper (FR-6, Security considerations); the column
-- itself carries no constraint beyond being text.

-- +goose Up

CREATE TABLE jobs (
    id            TEXT PRIMARY KEY,
    kind          TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'queued',
    attempts      INTEGER     NOT NULL DEFAULT 0,
    max_attempts  INTEGER     NOT NULL,
    available_at  TIMESTAMPTZ NOT NULL,
    locked_until  TIMESTAMPTZ,
    lease_token   TEXT,
    locked_by     TEXT,
    last_error    TEXT,
    progress      JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,

    CONSTRAINT jobs_status_check
        CHECK (status IN ('queued', 'running', 'retrying', 'completed', 'dead_letter')),
    CONSTRAINT jobs_attempts_non_negative
        CHECK (attempts >= 0),
    CONSTRAINT jobs_max_attempts_positive
        CHECK (max_attempts >= 1)
);

-- FR-4's claim query: WHERE status IN ('queued','retrying') AND
-- available_at <= $now ORDER BY available_at LIMIT 1 FOR UPDATE SKIP
-- LOCKED. A partial index on exactly that predicate keeps the scan off
-- the completed/dead-letter rows that accumulate over time.
CREATE INDEX jobs_claim_idx ON jobs (available_at)
    WHERE status IN ('queued', 'retrying');

-- FR-5's reaper sweep: WHERE status = 'running' AND locked_until < $now.
CREATE INDEX jobs_reaper_idx ON jobs (locked_until)
    WHERE status = 'running';

-- +goose Down

DROP INDEX IF EXISTS jobs_reaper_idx;
DROP INDEX IF EXISTS jobs_claim_idx;
DROP TABLE IF EXISTS jobs;
