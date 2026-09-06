-- Phase 14: Devices & Sync
-- (backend-device-sync.md FR-4/FR-5/FR-8, ADR 0029).

-- +goose Up

-- 1. Global sync sequence for reading data
CREATE SEQUENCE sync_seq START WITH 1 INCREMENT BY 1;

-- 2. Extend paired_devices with sync state
ALTER TABLE paired_devices
    ADD COLUMN sync_cursor BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN last_synced_at TIMESTAMPTZ;

-- 3. Extend reading data tables with sync_sequence
ALTER TABLE reading_progress
    ADD COLUMN sync_sequence BIGINT NOT NULL DEFAULT nextval('sync_seq');

ALTER TABLE bookmarks
    ADD COLUMN sync_sequence BIGINT NOT NULL DEFAULT nextval('sync_seq');

ALTER TABLE highlights
    ADD COLUMN sync_sequence BIGINT NOT NULL DEFAULT nextval('sync_seq');

-- 4. Triggers to advance sync_sequence on insert or update
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_sync_sequence()
RETURNS TRIGGER AS $$
BEGIN
    NEW.sync_sequence = nextval('sync_seq');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_reading_progress_sync_seq
BEFORE INSERT OR UPDATE ON reading_progress
FOR EACH ROW EXECUTE FUNCTION set_sync_sequence();

CREATE TRIGGER trg_bookmarks_sync_seq
BEFORE INSERT OR UPDATE ON bookmarks
FOR EACH ROW EXECUTE FUNCTION set_sync_sequence();

CREATE TRIGGER trg_highlights_sync_seq
BEFORE INSERT OR UPDATE ON highlights
FOR EACH ROW EXECUTE FUNCTION set_sync_sequence();

-- 5. Indexes for scoped incremental sync query
CREATE INDEX reading_progress_sync_seq_idx ON reading_progress (user_id, library_id, sync_sequence);
CREATE INDEX bookmarks_sync_seq_idx ON bookmarks (user_id, library_id, sync_sequence);
CREATE INDEX highlights_sync_seq_idx ON highlights (user_id, library_id, sync_sequence);

-- +goose Down

DROP INDEX IF EXISTS highlights_sync_seq_idx;
DROP INDEX IF EXISTS bookmarks_sync_seq_idx;
DROP INDEX IF EXISTS reading_progress_sync_seq_idx;

DROP TRIGGER IF EXISTS trg_highlights_sync_seq ON highlights;
DROP TRIGGER IF EXISTS trg_bookmarks_sync_seq ON bookmarks;
DROP TRIGGER IF EXISTS trg_reading_progress_sync_seq ON reading_progress;
DROP FUNCTION IF EXISTS set_sync_sequence();

ALTER TABLE highlights DROP COLUMN IF EXISTS sync_sequence;
ALTER TABLE bookmarks DROP COLUMN IF EXISTS sync_sequence;
ALTER TABLE reading_progress DROP COLUMN IF EXISTS sync_sequence;

ALTER TABLE paired_devices DROP COLUMN IF EXISTS last_synced_at;
ALTER TABLE paired_devices DROP COLUMN IF EXISTS sync_cursor;

DROP SEQUENCE IF EXISTS sync_seq;
