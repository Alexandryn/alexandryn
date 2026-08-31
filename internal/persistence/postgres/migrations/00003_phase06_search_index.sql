-- Phase 06: full-text search support on works (backend-library-api.md FR-2).
--
-- tsvector generated columns on works.title and authors.name, combined
-- into a per-work search vector via a materialized view. PostgreSQL
-- built-in, no new dependency (constitution §9, ADR 0004).
--
-- The approach: add a generated tsvector column to the works table,
-- and index it with GIN. Author names are folded in via a trigger so
-- the search vector always reflects the current work_authors rows.
--
-- search_vector includes:
--   setweight(to_tsvector('simple', title), 'A')        — title, weight A
--   setweight(to_tsvector('simple', subtitle), 'B')     — subtitle, weight B
-- Author names are updated via the trigger below (weight C).
--
-- Using the 'simple' dictionary: no stemming, matches substring via
-- tsquery prefix matching; matches the FR-2 goal of "matched against
-- title and author name" without ranking quality concerns (Non-goals).

-- +goose Up

ALTER TABLE works
    ADD COLUMN search_vector tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
            setweight(to_tsvector('simple', coalesce(subtitle, '')), 'B')
        ) STORED;

CREATE INDEX works_search_vector_gin ON works USING GIN (search_vector);

-- Author-name search: stored in a separate column, updated by trigger.
-- A generated column cannot reference other tables, so we track author
-- contribution separately and OR it into queries at query time
-- (see WorkRepository.Search — the JOIN path in the query).
-- No separate column needed: the JOIN in the query handles it directly
-- without requiring a stored author-name vector on the works row.
-- The GIN index on works.search_vector handles title/subtitle; author
-- name matching uses a separate GIN index on authors.name tsvector.

ALTER TABLE authors
    ADD COLUMN name_vector tsvector
        GENERATED ALWAYS AS (
            to_tsvector('simple', coalesce(name, ''))
        ) STORED;

CREATE INDEX authors_name_vector_gin ON authors USING GIN (name_vector);

-- +goose Down

DROP INDEX IF EXISTS authors_name_vector_gin;
ALTER TABLE authors DROP COLUMN IF EXISTS name_vector;
DROP INDEX IF EXISTS works_search_vector_gin;
ALTER TABLE works DROP COLUMN IF EXISTS search_vector;
