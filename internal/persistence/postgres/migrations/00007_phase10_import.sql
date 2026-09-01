-- Phase 10: Import pipeline (backend-import-pipeline.md FR-3).
-- Staging and lifecycle table for discovered source items pending extraction,
-- matching, or user confirmation before domain entity construction.

-- +goose Up

CREATE TABLE import_candidates (
    id                  TEXT PRIMARY KEY,
    source_id           TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    file_reference      JSONB NOT NULL,
    status              TEXT NOT NULL DEFAULT 'queued'
                        CHECK (status IN ('queued', 'pending', 'auto_imported', 'confirmed', 'rejected', 'failed')),
    extracted_metadata  JSONB,
    match_candidates    JSONB,
    job_id              TEXT,
    last_error          TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- FR-3: Indexed on (source_id, status) for both discovery deduplication checks
-- and the resolution UI's listing queries.
CREATE INDEX import_candidates_source_status_idx ON import_candidates (source_id, status);

-- +goose Down

DROP INDEX IF EXISTS import_candidates_source_status_idx;
DROP TABLE IF EXISTS import_candidates;
