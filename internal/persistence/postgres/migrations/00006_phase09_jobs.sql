-- Background job queue storage.
--
-- Supports concurrent worker claiming via SELECT ... FOR UPDATE SKIP LOCKED.
-- Uses lease fencing tokens to prevent stale writes after worker timeouts.

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

-- Partial index on queued/retrying jobs for fast worker polling.
CREATE INDEX jobs_claim_idx ON jobs (available_at)
    WHERE status IN ('queued', 'retrying');

-- Partial index for dead-worker recovery sweeps.
CREATE INDEX jobs_reaper_idx ON jobs (locked_until)
    WHERE status = 'running';

-- +goose Down

DROP INDEX IF EXISTS jobs_reaper_idx;
DROP INDEX IF EXISTS jobs_claim_idx;
DROP TABLE IF EXISTS jobs;
