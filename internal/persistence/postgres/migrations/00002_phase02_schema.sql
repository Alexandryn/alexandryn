-- Core domain schema: works, authors, editions, library entries,
-- collections, sources, offerings, reading progress, and outbox tables.
--
-- TEXT primary keys throughout, matching string-based ID types.
-- Column names are snake_case.
--
-- Embedded value collections receive dedicated child tables.

-- +goose Up

CREATE TABLE works (
    id                 TEXT PRIMARY KEY,
    title              TEXT NOT NULL,
    subtitle           TEXT NOT NULL DEFAULT '',
    original_language  TEXT,
    merged_into        TEXT REFERENCES works(id)
);

-- Work may reference zero or more Authors.
CREATE TABLE work_authors (
    work_id    TEXT NOT NULL REFERENCES works(id),
    author_id  TEXT NOT NULL,
    PRIMARY KEY (work_id, author_id)
);

-- Subject is a value type (no identity of its own) — the subject text
-- itself, plus which Work it's attached to, is the whole row.
CREATE TABLE work_subjects (
    work_id  TEXT NOT NULL REFERENCES works(id),
    subject  TEXT NOT NULL,
    PRIMARY KEY (work_id, subject)
);

-- ExternalReference (Source, ID) is a value type, reused identically for
-- Work, Edition, and Author — three child tables, same two-column shape.
CREATE TABLE work_external_references (
    work_id      TEXT NOT NULL REFERENCES works(id),
    source       TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    PRIMARY KEY (work_id, source, external_id)
);

-- Work containment graph: container_work_id contains containee_work_id.
CREATE TABLE work_contains (
    container_work_id  TEXT NOT NULL REFERENCES works(id),
    containee_work_id  TEXT NOT NULL REFERENCES works(id),
    PRIMARY KEY (container_work_id, containee_work_id)
);

CREATE TABLE authors (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    merged_into  TEXT REFERENCES authors(id)
);

CREATE TABLE author_external_references (
    author_id    TEXT NOT NULL REFERENCES authors(id),
    source       TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    PRIMARY KEY (author_id, source, external_id)
);

-- An Edition cannot exist without a parent Work.
CREATE TABLE editions (
    id                 TEXT PRIMARY KEY,
    work_id            TEXT NOT NULL REFERENCES works(id),
    language           TEXT NOT NULL,
    isbn               TEXT,
    publisher          TEXT NOT NULL DEFAULT '',
    publication_year   INTEGER
);

CREATE TABLE edition_external_references (
    edition_id   TEXT NOT NULL REFERENCES editions(id),
    source       TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    PRIMARY KEY (edition_id, source, external_id)
);

-- At most one LibraryEntry per Edition.
CREATE TABLE library_entries (
    id          TEXT PRIMARY KEY,
    edition_id  TEXT NOT NULL UNIQUE REFERENCES editions(id),
    added_at    TIMESTAMPTZ NOT NULL
);

CREATE TABLE collections (
    id    TEXT PRIMARY KEY,
    name  TEXT NOT NULL
);

-- Each membership carries its own added_at timestamp.
CREATE TABLE collection_members (
    collection_id  TEXT NOT NULL REFERENCES collections(id),
    work_id        TEXT NOT NULL REFERENCES works(id),
    added_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (collection_id, work_id)
);

CREATE TABLE sources (
    id            TEXT PRIMARY KEY,
    label         TEXT NOT NULL,
    can_list      BOOLEAN NOT NULL,
    can_search    BOOLEAN NOT NULL,
    can_download  BOOLEAN NOT NULL,
    kind          TEXT NOT NULL DEFAULT ''
);

-- Uniquely identified by (Source, Edition, Format).
-- file_reference_* columns represent FileReference value fields.
CREATE TABLE source_offerings (
    id                          TEXT PRIMARY KEY,
    source_id                   TEXT NOT NULL REFERENCES sources(id),
    edition_id                  TEXT NOT NULL REFERENCES editions(id),
    file_reference_id           TEXT NOT NULL,
    file_reference_format       TEXT NOT NULL,
    file_reference_size_bytes   BIGINT,
    observed_at                 TIMESTAMPTZ NOT NULL,
    UNIQUE (source_id, edition_id, file_reference_format)
);

-- At most one ReadingProgress per Work.
-- precise_position_* columns represent PrecisePosition fields.
CREATE TABLE reading_progress (
    id                            TEXT PRIMARY KEY,
    work_id                       TEXT NOT NULL UNIQUE REFERENCES works(id),
    percentage                    DOUBLE PRECISION NOT NULL,
    precise_position_edition_id   TEXT REFERENCES editions(id),
    precise_position_value        TEXT,
    device_id                     TEXT NOT NULL,
    observed_at                   TIMESTAMPTZ NOT NULL
);

CREATE TABLE bookmarks (
    id          TEXT PRIMARY KEY,
    edition_id  TEXT NOT NULL REFERENCES editions(id),
    position    TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE highlights (
    id              TEXT PRIMARY KEY,
    edition_id      TEXT NOT NULL REFERENCES editions(id),
    start_position  TEXT NOT NULL,
    end_position    TEXT NOT NULL,
    note            TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT ''
);

-- One row per DeviceID with settings JSONB bag.
CREATE TABLE reading_preferences (
    device_id  TEXT PRIMARY KEY,
    settings   JSONB NOT NULL DEFAULT '{}'
);

-- Transactional outbox for events written in the same transaction as state changes.
CREATE TABLE outbox (
    id            TEXT PRIMARY KEY,
    event_type    TEXT NOT NULL,
    aggregate_id  TEXT NOT NULL,
    payload       JSONB,
    occurred_at   TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE outbox;
DROP TABLE reading_preferences;
DROP TABLE highlights;
DROP TABLE bookmarks;
DROP TABLE reading_progress;
DROP TABLE source_offerings;
DROP TABLE sources;
DROP TABLE collection_members;
DROP TABLE collections;
DROP TABLE library_entries;
DROP TABLE edition_external_references;
DROP TABLE editions;
DROP TABLE author_external_references;
DROP TABLE authors;
DROP TABLE work_contains;
DROP TABLE work_external_references;
DROP TABLE work_subjects;
DROP TABLE work_authors;
DROP TABLE works;
