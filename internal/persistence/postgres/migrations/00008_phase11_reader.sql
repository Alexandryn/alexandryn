-- Reader state columns: reading progress epochs for reconciliation order
-- and created_at timestamps for bookmarks and highlights.

-- +goose Up
ALTER TABLE reading_progress ADD COLUMN epoch BIGINT NOT NULL DEFAULT 0;
ALTER TABLE bookmarks ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE highlights ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE highlights DROP COLUMN created_at;
ALTER TABLE bookmarks DROP COLUMN created_at;
ALTER TABLE reading_progress DROP COLUMN epoch;
