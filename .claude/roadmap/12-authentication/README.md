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
- Session issuance and validation, auth middleware on every non-health route
- Brute-force rate limiting
- Setup and login UI

**Out**

- Binding to any non-loopback interface — phase 13.

## Exit criteria

- [ ] All non-health routes require authentication
- [ ] Password hashing verified with no plaintext exposure anywhere, including logs
- [ ] Rate limiting functional against brute force
- [ ] Security audit recorded, no open Critical or High findings
- [ ] Maintainer approval recorded
