-- Phase 13: Network Access & Device Pairing
-- (backend-network-api.md FR-7/FR-8, ADR 0021, ADR 0028).

-- +goose Up

-- 1. Pairing sessions
CREATE TABLE pairing_sessions (
    id              TEXT PRIMARY KEY,
    initiated_by    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_ciphertext BYTEA NOT NULL,
    code_index      BYTEA NOT NULL UNIQUE,
    state           TEXT NOT NULL CHECK (state IN ('pending', 'verified', 'consumed', 'expired')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    device_id       TEXT,
    initiator_ip    INET
);

CREATE INDEX pairing_sessions_state_expires_idx ON pairing_sessions (state, expires_at);

-- 2. Paired devices
CREATE TABLE paired_devices (
    id                 TEXT PRIMARY KEY,
    owner_id           TEXT REFERENCES users(id) ON DELETE CASCADE,
    label              TEXT NOT NULL DEFAULT '',
    device_class       TEXT NOT NULL CHECK (device_class IN ('phone', 'tablet', 'desktop', 'tv', 'unknown')),
    enrolled_via       TEXT NOT NULL CHECK (enrolled_via IN ('pairing_code', 'password_login')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at         TIMESTAMPTZ,
    pairing_session_id TEXT REFERENCES pairing_sessions(id) ON DELETE SET NULL
);

CREATE INDEX paired_devices_owner_idx ON paired_devices (owner_id);
CREATE INDEX paired_devices_pairing_session_idx ON paired_devices (pairing_session_id);

-- 3. Enrolment grant JTIs (single-use replay defence)
CREATE TABLE enrolment_grant_jtis (
    jti      TEXT PRIMARY KEY,
    spent_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. Network settings (single row)
CREATE TABLE network_settings (
    id                   TEXT PRIMARY KEY DEFAULT 'default' CHECK (id = 'default'),
    host_name            TEXT NOT NULL DEFAULT '',
    remember_device_days INT NOT NULL DEFAULT 30 CHECK (remember_device_days >= 1 AND remember_device_days <= 90),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO network_settings (id, host_name, remember_device_days, updated_at)
VALUES ('default', '', 30, now())
ON CONFLICT (id) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS network_settings;
DROP TABLE IF EXISTS enrolment_grant_jtis;
DROP TABLE IF EXISTS paired_devices;
DROP TABLE IF EXISTS pairing_sessions;
