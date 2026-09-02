# Spec: Backend Authentication & Session Lifecycle

| | |
|---|---|
| **Status** | `APPROVED` (Maintainer Gate 1 sign-off 2026-09-02) |
| **Phase** | `12-auth` |
| **Author** | Claude (Sonnet 4.6), approved by Luann Moreira |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-02 |
| **Supersedes** | — |
| **Reviewed in** | Gate 1 Review Batch |

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
  - If TOTP MFA is not enabled, generates a 15-minute HS256 Access Token and a 30-day cryptographically random Refresh Token. Stores SHA-256 hash of refresh token in `user_refresh_tokens` and returns `{ user: UserSummary, accessToken: string, refreshToken: string }`.
- **FR-4: Token Refresh & Rotation**:
  - `POST /api/v1/auth/refresh` accepts `{ refreshToken: string }`.
  - Hashes the token with SHA-256 and searches `user_refresh_tokens`.
  - If not found, expired, or `revoked_at IS NOT NULL`, returns `401 Unauthorized`.
  - If valid, atomically marks the used refresh token as `revoked_at = now()`, creates a new refresh token row, and returns a new access token + refresh token pair.
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
- `POST /api/v1/auth/login` -> `200 { user, accessToken, refreshToken }` | `200 { mfaRequired: true, mfaTicket }` | `401`
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
