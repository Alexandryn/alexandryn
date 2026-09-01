-- Phase 11: the reader. Three column additions, no new tables — the
-- reading_progress / bookmarks / highlights / reading_preferences tables
-- were created in migration 00002 (phase 02's domain schema) and their
-- repositories landed in R8.
--
-- `reading_progress.epoch` — domain-reading.md FR-2/FR-6 as amended
-- 2026-09-01 (review 0048 finding 1). Reconciliation is lexical `max`
-- over the total order `(epoch, percentage)`; the epoch is
-- server-assigned and monotonically non-decreasing, bumped only by an
-- explicit backward move (FR-7's OverrideProgress). NOT NULL DEFAULT 0
-- so the (currently empty) table's rows start at epoch 0, the same
-- value a first ProgressReport produces.
--
-- `bookmarks.created_at` / `highlights.created_at` — reading-data-export.md
-- FR-4. The export document (GET /api/v1/reading/export) records when
-- each mark was made; the domain Bookmark/Highlight types gain a
-- matching CreatedAt field, set at construction. DEFAULT now() covers
-- the empty table; new rows pass an explicit server time.

-- +goose Up
ALTER TABLE reading_progress ADD COLUMN epoch BIGINT NOT NULL DEFAULT 0;
ALTER TABLE bookmarks ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE highlights ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE highlights DROP COLUMN created_at;
ALTER TABLE bookmarks DROP COLUMN created_at;
ALTER TABLE reading_progress DROP COLUMN epoch;
