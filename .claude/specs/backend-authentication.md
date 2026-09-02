# Spec: Backend Authentication & Session Lifecycle

| | |
|---|---|
| **Status** | `APPROVED` (Maintainer Gate 1 sign-off 2026-09-02). **Amended 2026-09-02 (`DRAFT` amendments, pending maintainer re-confirmation) for phase-13 review [`0050`](../reviews/0050-phase13-spec-package-and-phase12-authz-review.md):** FR-7 now requires the auth middleware to assert the access-token **type** (AUDIT-0012-C2 — an MFA ticket was being accepted as a bearer token); FR-3/FR-4 add the optional login `enrolmentGrant` parameter (ADR 0028 §6) and make the refresh-token lifetime operator-configurable `[1,90]` days, default 30 (ADR 0028 §10). The **implementation** as shipped on `feat/phase12-auth` did not enforce the FR-7 type check — that is the phase-13 hardening prelude (`backend-network-transport.md` FR-13). |
| **Phase** | `12-auth` |
| **Author** | Claude (Sonnet 4.6), approved by Luann Moreira |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-02 |
| **Supersedes** | — |
| **Reviewed in** | Gate 1 Review Batch; phase-13 amendments in [`0050`](../reviews/0050-phase13-spec-package-and-phase12-authz-review.md) |

## Context

Alexandryn requires robust user authentication before enabling multi-client and network exposure (Constitution §6). This spec defines the identity, credential hashing (Argon2id), JWT session lifecycle, refresh token rotation, in-memory rate limiting, and password reset flows.

## Problem

Prior to Phase 12, all requests were loopback-trusted without user identities, sessions, or credential protection.

## Goals

- Secure user registration and initial admin bootstrap (`/api/v1/auth/setup`).
- Credential authentication via Argon2id password hashing and RFC 7519 JWT issuance (`/api/v1/auth/login`).
- Refresh token lifecycle with rotation and explicit revocation (`/api/v1/auth/refresh`, `/api/v1/auth/logout`).
- Single-use, time-limited password reset flow (`/api/v1/auth/password-reset/request`, `/confirm`).
- Authentication middleware validating `Authorization: Bearer <token>` on all protected endpoints.
- In-memory rate limiting on authentication routes to prevent brute-force attacks.
- Open-redirect protection on authentication callbacks.
- Zero password, hash, or token leakage in logs (Constitution §8).

## Non-goals

- OAuth2 / OpenID Connect / SAML external identity providers (deferred to future enterprise integration).
- WebAuthn / Passkeys (deferred to future authentication extension).

## Functional Requirements

- **FR-1: Setup Bootstrap**:
  - `GET /api/v1/auth/setup/status` returns `{ isSetup: boolean }`. Returns `false` if `users` table has zero records.
  - `POST /api/v1/auth/setup` takes `{ username: string, email: string, password: string }`.
  - When zero users exist, creates the initial user with role `admin`, associates them with the `Default Library` as admin, hashes password using Argon2id, and returns `{ user: UserSummary, accessToken: string, refreshToken: string }`.
  - If any user already exists, returns `409 Conflict`.
- **FR-2: Password Hashing (Argon2id)**:
  - Hashing uses `golang.org/x/crypto/argon2` with parameters: Memory = 65536 KiB (64 MiB), Iterations = 3, Parallelism = 4, Salt = 16 bytes, KeyLen = 32 bytes.
  - Formats hash as standard PHC string: `$argon2id$v=19$m=65536,t=3,p=4$<b64salt>$<b64key>`.
  - Passwords must be 8–128 characters long.
  - Password comparison uses constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
- **FR-3: User Login**:
  - `POST /api/v1/auth/login` accepts `{ emailOrUsername: string, password: string }`.
  - Looks up user by email (case-insensitive) or username (case-insensitive).
  - If credentials are invalid, returns `401 Unauthorized` with a generic `"invalid credentials"` message (no user-enumeration signal).
  - If TOTP MFA is enabled on the account, returns `200 OK` with `{ mfaRequired: true, mfaTicket: string }`.
  - If TOTP MFA is not enabled, generates a 15-minute HS256 Access Token and a cryptographically random Refresh Token. Stores SHA-256 hash of refresh token in `user_refresh_tokens` and returns `{ user: UserSummary, accessToken: string, refreshToken: string }`.
  - **Amended by phase 13 (2026-09-02, `DRAFT` — ADR 0028 §6, §10; `backend-network-transport.md` FR-10, `backend-network-api.md` FR-9):**
    - The request body MAY carry an optional `{ enrolmentGrant?: string }`.
      When present it is verified against the **`enrolment-grant-v1`** HKDF
      subkey (never the access-token key), its `typ` must be `"enrol"`,
      and its `jti` must not already be in `enrolment_grant_jtis`. On a
      valid grant, **after** the password (and MFA, if any) fully
      authenticate the user, the handler assigns the `PairedDevice` for
      the grant's pairing session to **the now-authenticated user** and
      records the `jti` as spent. An invalid, expired, replayed, or
      wrong-type grant is **ignored** — login still succeeds with no
      device association, logged at `info`. The grant is never an
      authentication factor; a `401` is never returned *because of* the
      grant.
    - The refresh token's lifetime is `now + rememberDeviceDays` where
      `rememberDeviceDays` is `network_settings.remember_device_days`
      (default 30, operator-settable to `[1, 90]` via
      `PATCH /api/v1/network/settings`, ADR 0028 §10). "30-day" wherever
      it appeared in this spec now means "the configured value, default
      30". Existing tokens are not retroactively re-dated.
- **FR-4: Token Refresh & Rotation**:
  - `POST /api/v1/auth/refresh` accepts `{ refreshToken: string }`.
  - Hashes the token with SHA-256 and searches `user_refresh_tokens`.
  - If not found, expired, or `revoked_at IS NOT NULL`, returns `401 Unauthorized`.
  - If valid, atomically marks the used refresh token as `revoked_at = now()`, creates a new refresh token row (its `expires_at` also `now + rememberDeviceDays` per the FR-3 amendment), and returns a new access token + refresh token pair.
- **FR-5: Logout & Revocation**:
  - `POST /api/v1/auth/logout` accepts `{ refreshToken: string }` and marks `revoked_at = now()` on the corresponding refresh token row. Returns `204 No Content`.
- **FR-6: Password Reset Flow**:
  - `POST /api/v1/auth/password-reset/request` accepts `{ email: string }`. Always returns `200 OK` with `{ message: "If the account exists, a reset link has been dispatched" }` regardless of whether the email was found.
  - Generates 32 cryptographically random bytes, saves SHA-256 hash to `password_resets` table with `expires_at = now() + 1 hour`.
  - `POST /api/v1/auth/password-reset/confirm` accepts `{ token: string, newPassword: string }`. Validates token hash, expiration, and `used_at IS NULL`.
  - Atomically updates the user's password hash, marks reset token `used_at = now()`, and revokes all active refresh tokens for that user.
- **FR-7: Authentication Middleware**:
  - Validates `Authorization: Bearer <token>` on all `/api/v1/*` routes except public health checks (`/healthz`, `/readyz`) and unauthenticated auth routes (`/api/v1/auth/setup/status`, `/setup`, `/login`, `/refresh`, `/password-reset/*`, `/mfa/totp/verify`).
  - Injects `AuthenticatedUser` (`UserID`, `Role`, `Username`, `Libraries`) into the request context.
  - Returns `401 Unauthorized` on missing, expired, or invalid token.
  - **Amended by phase 13 (2026-09-02, `DRAFT` — review [`0050`](../reviews/0050-phase13-spec-package-and-phase12-authz-review.md) AUDIT-0012-C2, `CLAUDE.md` token-type Reflex):** the middleware MUST **assert the token type**. The verifier it calls (a new `VerifyAccessToken`, or `Verify` grown a required check) accepts **only** an access token — `typ` empty or `"access"` — and rejects any token whose `typ` is `"mfa_ticket"`, `"enrol"`, or anything else, with `401`, **even when the signature is valid**. Distinct token purposes are signed with distinct HKDF subkeys (access: `jwt-signing-secret-v1`; enrolment grant: `enrolment-grant-v1`), so this is defence in depth over a structural separation, not the only barrier. Test matrix: access ✓, `mfa_ticket` ✗, `enrol` ✗, unsigned/garbage ✗.
  - (The `X-Library-Id`-against-`claims.Libraries` check is
    `backend-library-namespaces.md`'s amendment; both middleware changes
    ship together in phase 13's hardening prelude,
    `backend-network-transport.md` FR-13.)
- **FR-8: Rate Limiting**:
  - In-memory token bucket rate limiting on auth endpoints:
    - `/api/v1/auth/login`: 5 req/min per IP.
    - `/api/v1/auth/refresh`: 30 req/min per IP.
    - `/api/v1/auth/password-reset/*`: 3 req/15min per IP.
  - Exceeding returns `429 Too Many Requests`.
- **FR-9: Open Redirect Validation**:
  - Any query parameter `redirect` or `returnTo` must be a relative path starting with `/` (excluding `//` or external URI schemes).

## Non-functional Requirements

- **Security**: Zero password, hash, or token data in logs or client-facing stack traces. Argon2id parameters resist offline cracking.
- **Performance**: In-memory JWT verification takes < 0.1ms per request with no database query on standard reads.

## API Contracts

- `GET /api/v1/auth/setup/status` -> `200 { isSetup: boolean }`
- `POST /api/v1/auth/setup` -> `201 { user, accessToken, refreshToken }` | `409`
- `POST /api/v1/auth/login` — request `{ emailOrUsername, password, enrolmentGrant? }` (phase-13 amendment) -> `200 { user, accessToken, refreshToken }` | `200 { mfaRequired: true, mfaTicket }` | `401`. A present-but-invalid `enrolmentGrant` never changes the status code.
- `POST /api/v1/auth/refresh` -> `200 { accessToken, refreshToken }` | `401`
- `POST /api/v1/auth/logout` -> `204`
- `POST /api/v1/auth/password-reset/request` -> `200 { message: string }`
- `POST /api/v1/auth/password-reset/confirm` -> `200 { success: true }` | `400` | `401`

## Acceptance Criteria

- [ ] Setup endpoint boots the first admin account and locks thereafter.
- [ ] Passwords hashed with Argon2id matching OWASP parameters.
- [ ] JWT tokens carry proper claims and are validated statelessly by auth middleware.
- [ ] Refresh token rotation revokes old tokens and issues new ones.
- [ ] Rate limiter blocks brute force attempts with 429.
- [ ] Zero sensitive tokens or passwords in slog logs.
