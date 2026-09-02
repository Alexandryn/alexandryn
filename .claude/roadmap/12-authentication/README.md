# Phase 12 — Authentication

| | |
|---|---|
| **Status** | Implemented on `feat/phase12-auth`, **not closed** — see "Correction (2026-09-02)" below. Specs `APPROVED` (Gate 1 maintainer sign-off 2026-09-02): `backend-authentication.md`, `backend-authorization-rbac.md`, `backend-library-namespaces.md`, `frontend-auth-and-tenancy.md`. ADRs 0025 (JWT sessions), 0026 (multi-library tenancy), 0027 (TOTP MFA). Security audit `0012` — **superseded** (see below). |
| **Depends on** | Phase 05 |
| **Blocks** | 13 |

## Correction (2026-09-02)

The phase-13 spec-package review
([`../../reviews/0050-phase13-spec-package-and-phase12-authz-review.md`](../../reviews/0050-phase13-spec-package-and-phase12-authz-review.md))
independently threat-modelled the phase-12 surface and found **two
High-severity authorization defects** in the wired code that security
audit `0012` had certified as controls-present:

- **AUDIT-0012-C1** — the reading API (`progress`, `bookmarks`,
  `highlights`, `preferences`, `export`, `reader/content`) is not user-
  or library-scoped; handlers call bare-ID repository methods.
  `GET /api/v1/reading/export` returns every user's private data.
  Horizontal IDOR.
- **AUDIT-0012-C2** — `AuthMiddleware`'s token verification does not
  check the token type; an MFA ticket is accepted as an access token.

Audit `0012` carries a "Post-audit correction" section with the honest
severity, the corrective directives, and where each is enforced.

**Phase 12 cannot be marked `Closed` until:**
1. C1 and C2 are fixed and re-verified (the fix lands on
   `feat/phase13-network-access` as a phase-12 hardening prelude; phase
   13's audit `0013` re-verifies both).
2. `X-Library-Id` is validated against the JWT `libraries` claim
   (review `0050` finding P12-4).
3. A **fuller independent re-audit of the phase-12 authorization
   surface** is run — C1–C3 showed the original audit inferred handler
   behaviour from the repository and migration layers rather than
   tracing the wired call path, so the rest of the RBAC / membership /
   refresh-rotation surface warrants a real second look, not a
   spot-check.

The forward directives from this correction now live in `CLAUDE.md`
(Reflexes + "easy to get wrong"), `.claude/templates/audit.md` (mandatory
call-path-tracing checklist item), and `.claude/templates/spec.md`
(name the enforcement point and the cross-tenant refusal test).

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
