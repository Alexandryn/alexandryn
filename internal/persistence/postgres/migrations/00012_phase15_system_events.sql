-- Observability system events log.

-- +goose Up

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

-- +goose Down

DROP INDEX IF EXISTS system_events_host_created_idx;
DROP INDEX IF EXISTS system_events_purge_idx;
DROP INDEX IF EXISTS system_events_library_created_idx;
DROP TABLE IF EXISTS system_events;
