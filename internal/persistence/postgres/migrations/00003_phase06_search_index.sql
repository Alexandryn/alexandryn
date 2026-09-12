-- Full-text search support on works and authors.
--
-- Adds generated tsvector columns indexed with GIN.
-- Uses the 'simple' dictionary for unstemmed prefix matching.

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
