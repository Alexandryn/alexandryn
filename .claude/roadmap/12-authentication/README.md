# Phase 12 — Authentication

*Outline — expanded to a full phase document when phase 05 closes.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 05 |
| **Blocks** | 13 |

## Objective

Accounts, sessions, and authorization — the gate phase 13 cannot open
without. This is the ordering decision the whole roadmap is built around
(Constitution §6): no network exposure exists before this closes.

## Scope

**In**

- Account model, secure password hashing (Argon2id or equivalent)
- An ADR choosing the session mechanism (cookie-based session vs. JWT) —
  foundational to this phase's own issuance/validation work and to phase
  13's cookie-attribute and CSRF design, which can't be fixed until this
  is
- Session issuance and validation, auth middleware on every non-health route
- Password-reset lifecycle: time-limited, single-use tokens — not
  previously named in this outline, added per the 2026-08-18
  security-and-hardening review; a standard authentication requirement
  the original outline omitted
- Open-redirect protection for any post-authentication redirect target —
  already an open question in `decisions/README.md`, closed here rather
  than left implicit
- Brute-force rate limiting
- Setup and login UI

**Out**

- Binding to any non-loopback interface — phase 13.

## Exit criteria

- [ ] All non-health routes require authentication
- [ ] Password hashing verified with no plaintext exposure anywhere, including logs
- [ ] Rate limiting functional against brute force
- [ ] Password-reset tokens verified time-limited and single-use
- [ ] Post-authentication redirect targets validated, open-redirect tested
- [ ] Session-mechanism ADR recorded before this phase's issuance/validation work begins
- [ ] Security audit recorded, no open Critical or High findings
- [ ] Maintainer approval recorded
