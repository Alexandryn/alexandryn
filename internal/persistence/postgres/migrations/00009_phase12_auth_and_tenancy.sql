-- Phase 12: Authentication, RBAC & Multi-Library Namespacing
-- (backend-authentication.md, backend-authorization-rbac.md, backend-library-namespaces.md, ADR 0025, ADR 0026, ADR 0027).

-- +goose Up

-- 1. Multi-Library namespaces
CREATE TABLE libraries (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    allow_reader_uploads BOOLEAN NOT NULL DEFAULT FALSE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed canonical default library
INSERT INTO libraries (id, name, description, allow_reader_uploads, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Library', 'Default System Library', FALSE, now(), now())
ON CONFLICT (id) DO NOTHING;

-- 2. User identities & credentials
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    username   TEXT NOT NULL UNIQUE,
    email      TEXT NOT NULL UNIQUE,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'reader')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_credentials (
    user_id       TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_refresh_tokens (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX user_refresh_tokens_user_idx ON user_refresh_tokens (user_id);

CREATE TABLE mfa_totp_settings (
    user_id              TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    encrypted_secret     BYTEA NOT NULL,
    recovery_code_hashes TEXT[] NOT NULL DEFAULT '{}',
    enabled              BOOLEAN NOT NULL DEFAULT FALSE,
    confirmed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE library_memberships (
    id         TEXT PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'reader')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (library_id, user_id)
);
CREATE INDEX library_memberships_user_idx ON library_memberships (user_id);

CREATE TABLE library_invitations (
    id         TEXT PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'reader')),
    token_hash TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE password_resets (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. Backfill and namespace existing tables with library_id
ALTER TABLE library_entries ADD COLUMN library_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES libraries(id) ON DELETE CASCADE;
ALTER TABLE library_entries DROP CONSTRAINT IF EXISTS library_entries_edition_id_key;
ALTER TABLE library_entries ADD CONSTRAINT library_entries_library_edition_uniq UNIQUE (library_id, edition_id);

ALTER TABLE collections ADD COLUMN library_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES libraries(id) ON DELETE CASCADE;
ALTER TABLE sources ADD COLUMN library_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES libraries(id) ON DELETE CASCADE;
ALTER TABLE source_offerings ADD COLUMN library_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES libraries(id) ON DELETE CASCADE;

-- 4. Retrofit user & library scoping on reading data (resolves A-11-01)
ALTER TABLE reading_progress ADD COLUMN user_id TEXT;
ALTER TABLE reading_progress ADD COLUMN library_id TEXT REFERENCES libraries(id) ON DELETE CASCADE;
ALTER TABLE reading_progress DROP CONSTRAINT IF EXISTS reading_progress_work_id_key;
CREATE UNIQUE INDEX reading_progress_user_library_work_uniq ON reading_progress (COALESCE(user_id, ''), COALESCE(library_id, ''), work_id);

ALTER TABLE bookmarks ADD COLUMN user_id TEXT;
ALTER TABLE bookmarks ADD COLUMN library_id TEXT REFERENCES libraries(id) ON DELETE CASCADE;
CREATE INDEX bookmarks_user_library_edition_idx ON bookmarks (user_id, library_id, edition_id);

ALTER TABLE highlights ADD COLUMN user_id TEXT;
ALTER TABLE highlights ADD COLUMN library_id TEXT REFERENCES libraries(id) ON DELETE CASCADE;
CREATE INDEX highlights_user_library_edition_idx ON highlights (user_id, library_id, edition_id);

ALTER TABLE reading_preferences ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE reading_preferences DROP CONSTRAINT IF EXISTS reading_preferences_pkey;
ALTER TABLE reading_preferences ADD PRIMARY KEY (user_id, device_id);

-- +goose Down

ALTER TABLE reading_preferences DROP CONSTRAINT IF EXISTS reading_preferences_pkey;
ALTER TABLE reading_preferences DROP COLUMN IF EXISTS user_id;
ALTER TABLE reading_preferences ADD PRIMARY KEY (device_id);

DROP INDEX IF EXISTS highlights_user_library_edition_idx;
ALTER TABLE highlights DROP COLUMN IF EXISTS library_id;
ALTER TABLE highlights DROP COLUMN IF EXISTS user_id;

DROP INDEX IF EXISTS bookmarks_user_library_edition_idx;
ALTER TABLE bookmarks DROP COLUMN IF EXISTS library_id;
ALTER TABLE bookmarks DROP COLUMN IF EXISTS user_id;

DROP INDEX IF EXISTS reading_progress_user_library_work_uniq;
ALTER TABLE reading_progress DROP COLUMN IF EXISTS library_id;
ALTER TABLE reading_progress DROP COLUMN IF EXISTS user_id;
ALTER TABLE reading_progress ADD CONSTRAINT reading_progress_work_id_key UNIQUE (work_id);

ALTER TABLE source_offerings DROP COLUMN IF EXISTS library_id;
ALTER TABLE sources DROP COLUMN IF EXISTS library_id;
ALTER TABLE collections DROP COLUMN IF EXISTS library_id;

ALTER TABLE library_entries DROP CONSTRAINT IF EXISTS library_entries_library_edition_uniq;
ALTER TABLE library_entries DROP COLUMN IF EXISTS library_id;
ALTER TABLE library_entries ADD CONSTRAINT library_entries_edition_id_key UNIQUE (edition_id);

DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS library_invitations;
DROP TABLE IF EXISTS library_memberships;
DROP TABLE IF EXISTS mfa_totp_settings;
DROP TABLE IF EXISTS user_refresh_tokens;
DROP TABLE IF EXISTS user_credentials;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS libraries;
