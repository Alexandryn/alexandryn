-- Phase 08: Source configuration, credential storage, and health state
-- (backend-source-adapter.md FR-1, FR-2, FR-5, FR-6, FR-13).
--
-- domain-source.md fixed the abstract Source (identity, label, declared
-- capabilities) and phase 02's migration created the `sources` table for
-- that. This migration adds what the domain deliberately does not know
-- (architecture-backend.md FR-2): a provider kind's connection config (a
-- filesystem path or a base URL), an encrypted-at-rest Basic Auth
-- credential, the result of the last reachability probe, and the search
-- link discovered and origin-validated once at probe time (FR-11).
--
-- One physical row per Source aggregate — no child table. FileReference
-- is not stored here: a browsed candidate is an adapter DTO, never a
-- persisted SourceOffering (backend-source-adapter.md FR-10).
--
-- The credential is two BYTEA columns (AES-256-GCM ciphertext + nonce),
-- null together when the source has no credential. The key never touches
-- the database — it lives in a 0600 key file under the app-data
-- directory (FR-13), read by internal/adapters/crypto.

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

-- Phase 02 created can_list/can_search/can_download as NOT NULL with no
-- default. A Source created before its first health check has unknown
-- capabilities (FR-5 re-detects them on every probe), so default them to
-- false; existing callers that pass explicit values are unaffected.
ALTER TABLE sources
    ALTER COLUMN can_list     SET DEFAULT false,
    ALTER COLUMN can_search   SET DEFAULT false,
    ALTER COLUMN can_download SET DEFAULT false;

-- `kind` is deliberately NOT constrained here. domain-source.md FR-1
-- fixes it as a display-only tag the domain never validates, and
-- backend-source-adapter.md's closed `local-folder`/`opds` vocabulary is
-- enforced where hostile input actually enters — the HTTP handler's
-- line-1 validation (constitution §4) — not as a schema constraint that
-- every future fixture would have to know about.

ALTER TABLE sources
    ADD CONSTRAINT sources_health_status_check
        CHECK (health_status IN ('unknown', 'reachable', 'unreachable'));

-- FR-6's closed detail vocabulary. NULL when the source is reachable or
-- not yet checked.
ALTER TABLE sources
    ADD CONSTRAINT sources_health_detail_check
        CHECK (health_detail IS NULL OR health_detail IN (
            'timeout', 'connection-refused', 'http-4xx', 'http-5xx',
            'http-3xx-unsupported', 'unparseable-response',
            'path-not-found', 'path-not-readable', 'auth-rejected'
        ));

-- FR-1: a credential is Basic Auth for an OPDS source only — a
-- local-folder source has nothing to authenticate to.
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
