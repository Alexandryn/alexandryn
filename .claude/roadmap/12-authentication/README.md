# Phase 12 — Authentication

| | |
|---|---|
| **Status** | Closed. Specs `APPROVED` (Gate 1 maintainer sign-off 2026-09-02): `backend-authentication.md`, `backend-authorization-rbac.md`, `backend-library-namespaces.md`, `frontend-auth-and-tenancy.md`. ADRs 0025 (JWT sessions), 0026 (multi-library tenancy), 0027 (TOTP MFA). Security audit `0012` — superseded by the correction below, itself resolved. |
| **Depends on** | Phase 05 |
| **Blocks** | 13 |
| **Opened** | 2026-09-02 |
| **Closed** | 2026-09-11 |

## Correction (2026-09-02) — resolved 2026-09-11

All three items below are now resolved; kept as a record of what was found
and how it was closed, per Constitution §12.

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

**Phase 12 was marked `Closed` once:**
1. C1 and C2 were fixed and re-verified — landed on
   `feat/phase13-network-access` as a phase-12 hardening prelude; audit
   `0013` re-verified both fixed, merged to `main` via PR #80
   (2026-09-05).
2. `X-Library-Id` was validated against the JWT `libraries` claim
   (review `0050` finding P12-4) — verified in audit `0013`.
3. The **fuller independent re-audit of the phase-12 authorization
   surface** happened across three passes rather than one: audit `0013`
   traced the RBAC/route-registration surface (A-13-01, network-exposed
   routes), audit `0014` traced library-membership checks on the
   sync/device surface, and audit `0016`'s whole-application sweep
   traced the invitation/membership call path end-to-end (#262,
   escalated Medium → High during that trace, then fixed). No open
   Critical/High finding remains against the RBAC/membership/refresh-
   rotation surface as of phase 16's close (2026-09-11).

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

- [x] All non-health routes require authentication
- [x] Password hashing verified with no plaintext exposure anywhere, including logs
- [x] Rate limiting functional against brute force
- [x] Password-reset tokens verified time-limited and single-use
- [x] Post-authentication redirect targets validated, open-redirect tested
- [x] Session-mechanism ADR recorded before this phase's issuance/validation work begins — ADR 0025
- [x] Security audit recorded, no open Critical or High findings — audit `0012`, superseded and
      corrected by audits `0013`/`0014`/`0016` (see "Correction" above); 0 open Critical/High as of
      phase 16's close
- [x] Maintainer approval recorded — 2026-09-15, confirming the correction items above are resolved
      on `main` and this phase's status/exit-criteria record was left stale after the fix landed
