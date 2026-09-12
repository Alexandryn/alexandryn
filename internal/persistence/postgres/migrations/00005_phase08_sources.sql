-- Source configuration, encrypted credential storage, and health state.
--
-- Adds connection configuration (filesystem path or base URL),
-- AES-256-GCM encrypted credentials, health check status, and search link URLs.

-- +goose Up

ALTER TABLE sources
    ADD COLUMN config_base_path      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN config_base_url       TEXT        NOT NULL DEFAULT '',
    ADD COLUMN credential_ciphertext BYTEA,
    ADD COLUMN credential_nonce      BYTEA,
    ADD COLUMN health_status         TEXT        NOT NULL DEFAULT 'unknown',
    ADD COLUMN health_detail         TEXT,
    ADD COLUMN health_checked_at     TIMESTAMPTZ,
    ADD COLUMN search_link_url       TEXT,
    ADD COLUMN created_at            TIMESTAMPTZ NOT NULL DEFAULT now();

-- A source created before its first health check has unknown
-- capabilities, so default them to false; callers that pass
-- explicit values are unaffected.
ALTER TABLE sources
    ALTER COLUMN can_list     SET DEFAULT false,
    ALTER COLUMN can_search   SET DEFAULT false,
    ALTER COLUMN can_download SET DEFAULT false;

ALTER TABLE sources
    ADD CONSTRAINT sources_health_status_check
        CHECK (health_status IN ('unknown', 'reachable', 'unreachable'));

-- Detailed health vocabulary. NULL when the source is reachable or not yet checked.
ALTER TABLE sources
    ADD CONSTRAINT sources_health_detail_check
        CHECK (health_detail IS NULL OR health_detail IN (
            'timeout', 'connection-refused', 'http-4xx', 'http-5xx',
            'http-3xx-unsupported', 'unparseable-response',
            'path-not-found', 'path-not-readable', 'auth-rejected'
        ));

-- Credentials apply to OPDS sources only.
ALTER TABLE sources
    ADD CONSTRAINT sources_credential_requires_opds
        CHECK (credential_ciphertext IS NULL OR kind = 'opds');

-- Ciphertext and nonce are written and cleared as a pair.
ALTER TABLE sources
    ADD CONSTRAINT sources_credential_pair
        CHECK ((credential_ciphertext IS NULL) = (credential_nonce IS NULL));

CREATE INDEX sources_created_at_idx ON sources (created_at);

-- +goose Down

DROP INDEX IF EXISTS sources_created_at_idx;

ALTER TABLE sources
    DROP CONSTRAINT IF EXISTS sources_credential_pair,
    DROP CONSTRAINT IF EXISTS sources_credential_requires_opds,
    DROP CONSTRAINT IF EXISTS sources_health_detail_check,
    DROP CONSTRAINT IF EXISTS sources_health_status_check;

ALTER TABLE sources
    ALTER COLUMN can_list     DROP DEFAULT,
    ALTER COLUMN can_search   DROP DEFAULT,
    ALTER COLUMN can_download DROP DEFAULT;

ALTER TABLE sources
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS search_link_url,
    DROP COLUMN IF EXISTS health_checked_at,
    DROP COLUMN IF EXISTS health_detail,
    DROP COLUMN IF EXISTS health_status,
    DROP COLUMN IF EXISTS credential_nonce,
    DROP COLUMN IF EXISTS credential_ciphertext,
    DROP COLUMN IF EXISTS config_base_url,
    DROP COLUMN IF EXISTS config_base_path;
