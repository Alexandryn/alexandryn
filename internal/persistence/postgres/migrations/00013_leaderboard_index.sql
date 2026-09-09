-- Phase 16 (#116): index the leaderboard / finished-works hot path.
--
-- FinishedWorksHandler (GET /api/v1/library/finished) and
-- LibraryLeaderboardHandler (GET /api/v1/library/leaderboard) both scan
-- reading_progress with WHERE library_id = $1 AND percentage >= 100.
-- With no supporting index the planner sequential-scans the whole table;
-- a partial index on (library_id, observed_at) restricted to completed
-- reads keeps that path bounded and pre-orders the finished-works result.

-- +goose Up

CREATE INDEX reading_progress_library_finished_idx
    ON reading_progress (library_id, observed_at)
    WHERE percentage >= 100;

-- +goose Down

DROP INDEX IF EXISTS reading_progress_library_finished_idx;
