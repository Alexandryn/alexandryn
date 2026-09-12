-- Multi-factor authentication ticket single-use replay defence.

-- +goose Up

-- Spent MFA-ticket JTIs. Mirrors enrolment_grant_jtis: jti is the primary
-- key so an INSERT ... ON CONFLICT DO NOTHING is the atomic claim, and the
-- spent_at index lets the network sweep delete rows past the ticket TTL.
CREATE TABLE mfa_ticket_jtis (
    jti      TEXT PRIMARY KEY,
    spent_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX mfa_ticket_jtis_spent_at_idx ON mfa_ticket_jtis (spent_at);

-- +goose Down

DROP TABLE IF EXISTS mfa_ticket_jtis;
