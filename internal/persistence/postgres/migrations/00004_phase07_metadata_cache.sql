-- Phase 07: Metadata caching tables (backend-metadata-caching.md FR-1, FR-6, FR-7).
--
-- Stores normalised Open Library work, edition, and author records with fetched_at
-- timestamps for TTL staleness checking (30 days), and metadata_covers table for
-- LRU cover image tracking and missing cover sentinels.

-- +goose Up

CREATE TABLE metadata_works (
    key          TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    subtitle     TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    subjects     TEXT[] NOT NULL DEFAULT '{}',
    cover_url    TEXT,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX metadata_works_fetched_at_idx ON metadata_works (fetched_at);

CREATE TABLE metadata_authors (
    key          TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX metadata_authors_fetched_at_idx ON metadata_authors (fetched_at);

CREATE TABLE metadata_work_authors (
    work_key     TEXT NOT NULL REFERENCES metadata_works(key) ON DELETE CASCADE,
    author_key   TEXT NOT NULL REFERENCES metadata_authors(key) ON DELETE CASCADE,
    position     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (work_key, author_key)
);

CREATE TABLE metadata_editions (
    key          TEXT PRIMARY KEY,
    work_key     TEXT NOT NULL REFERENCES metadata_works(key) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    publisher    TEXT NOT NULL DEFAULT '',
    publish_date TEXT NOT NULL DEFAULT '',
    language     TEXT NOT NULL DEFAULT '',
    cover_url    TEXT,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX metadata_editions_work_key_idx ON metadata_editions (work_key);
CREATE INDEX metadata_editions_fetched_at_idx ON metadata_editions (fetched_at);

CREATE TABLE metadata_covers (
    cover_id     BIGINT PRIMARY KEY,
    file_path    TEXT NOT NULL DEFAULT '',
    byte_size    BIGINT NOT NULL DEFAULT 0,
    content_type TEXT NOT NULL DEFAULT 'image/jpeg',
    is_missing   BOOLEAN NOT NULL DEFAULT FALSE,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    accessed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX metadata_covers_accessed_at_idx ON metadata_covers (accessed_at);

-- +goose Down

DROP TABLE IF EXISTS metadata_covers;
DROP TABLE IF EXISTS metadata_editions;
DROP TABLE IF EXISTS metadata_work_authors;
DROP TABLE IF EXISTS metadata_authors;
DROP TABLE IF EXISTS metadata_works;
