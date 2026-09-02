# Security audit: Phase 12 — Authentication, RBAC & Multi-Library Namespacing

| | |
|---|---|
| **Scope** | `internal/auth/` (passwords, JWT, TOTP, rate limiting), `internal/domain/` (`auth.go`, `library_aggregate.go`), `internal/persistence/postgres/` (`00009_phase12_auth_and_tenancy.sql`, `auth_repository.go`, `library_repository.go`, user-scoped reading repos), `internal/transport/http/` (`auth_middleware.go`, `auth_handlers.go`, `library_handlers.go`, `auth_ref.go`, `lazy_auth.go`), `cmd/server/` (`main.go`, `run.go`, `repositories.go`), `api/openapi.yaml`, `web/src/` (`data/auth.ts`, `data/libraries.ts`, `data/http.ts`, `screens/Auth/`, `screens/Libraries/`). |
| **Auditor** | Claude (Sonnet 4.6 Thinking), `agent-skills:security-and-hardening` + `agent-skills:code-review-and-quality` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over each trust boundary |
| **Date** | 2026-09-02 |
| **Commit** | Branch `feat/phase12-auth-and-tenancy` |
| **Verdict** | **Clear** — no open Critical, High, or Medium vulnerabilities. Full `go test -race ./internal/... ./cmd/...` (all packages passing), contract tests suite passing, Vitest frontend suite (75 files, 379 tests passing), TypeScript compile clean (`tsc -b`), ESLint clean (0 errors), and all four check scripts (`check-import-boundaries.sh`, `check-parameterized-queries.sh`, `check-integration-test-parallelism.sh`, `check-compose-published-port.sh`) pass with clean status. |

---

## Scope and Architecture

Phase 12 delivers the complete identity, authentication, role-based access control, and multi-tenant library namespacing across six layers:

1. **Database Schema & Migrations (`00009_phase12_auth_and_tenancy.sql`)**:
   - Added tables: `libraries`, `users`, `user_credentials`, `user_refresh_tokens`, `mfa_totp_settings`, `library_memberships`, `library_invitations`, `password_resets`.
   - Foreign keys with `ON DELETE CASCADE`, indexed foreign key columns, default library backfill.
   - User-scoped reading data: `user_id` columns and compound unique indexes on `reading_progress`, `bookmarks`, `highlights`, `reading_preferences`.

2. **Domain Models & Entities (`internal/domain/auth.go`, `library_aggregate.go`)**:
   - Strict pure domain types: `User`, `Role` (`admin`, `reader`), `UserCredentials`, `RefreshToken`, `TOTPSettings`, `PasswordResetToken`, `Library`, `LibraryMembership`, `LibraryInvitation`.
   - Domain invariants: email normalization & regex validation, username format checks, bounded text lengths. Zero third-party crypto imports inside `internal/domain`.

3. **Cryptography & Authentication Services (`internal/auth/`)**:
   - **Passwords (`password.go`)**: Argon2id with OWASP-recommended parameters (64 MiB memory, 3 iterations, 4 parallelism, 16-byte random salt, 32-byte key) with constant-time equality check (`subtle.ConstantTimeCompare`).
   - **JWT Tokens (`jwt.go`)**: Standard-library HS256 signer and verifier. Strict header validation (explicit `typ: JWT`, `alg: HS256`), explicit rejection of `alg: none`, `nbf`/`exp` boundary checks, single-purpose MFA ticket validation.
   - **TOTP Engine (`totp.go`)**: RFC 6238 compliant (HMAC-SHA1, 6-digit, 30-second time step, ±1 step drift tolerance window). TOTP secret keys encrypted at rest using AES-256-GCM. 8 random backup recovery codes hashed with SHA-256 for one-time recovery.
   - **Rate Limiter (`ratelimit.go`)**: In-memory token bucket (`rate.Limiter`) keyed by IP address with periodic map cleanup and concurrency safety via `sync.RWMutex`.

4. **Persistence Repositories (`internal/persistence/postgres/`)**:
   - `UserRepository`, `CredentialRepository`, `RefreshTokenRepository`, `MFARepository`, `PasswordResetRepository`, `LibraryRepository`, `LibraryMembershipRepository`, `LibraryInvitationRepository`.
   - 100% parameterized SQL queries (`$1, $2...`) — verified by `check-parameterized-queries.sh`.
   - Reading repositories updated with user & library scoping (`FindByWorkAndUser`, `SaveForUser`, `FindByEditionAndUser`, `FindByUserAndDevice`).

5. **HTTP Transport & Middleware (`internal/transport/http/`)**:
   - `LazyAuthMiddleware`: Validates `Authorization: Bearer <token>`, extracts claims, binds `UserContext` and active `LibraryContext` to `r.Context()`. Allows public unauthenticated paths (`/api/v1/auth/setup/status`, `/setup`, `/login`, `/refresh`, `/password-reset/*`, `/mfa/totp/verify`, `/healthz`, `/readyz`).
   - Handlers for authentication lifecycle, MFA enrollment/verification, password reset, and library member management.
   - Subkey derivation via HKDF (`jwt-signing-secret-v1`, `mfa-totp-master-v1`) from the master application key in `cmd/server/run.go`.

6. **Frontend Web Client (`web/src/`)**:
   - `auth.ts` & `libraries.ts`: Typesafe client API, token storage, active library switcher.
   - UI Screens: `SetupScreen`, `LoginScreen`, `MfaPromptModal`, `MfaSetupModal`, `AcceptInviteScreen`, `LibrarySwitcher`, `LibraryManagement`.
   - `RequireAuth` route guard enforcing setup initialization and authentication redirects.

---

## Trust Boundaries Examined

| Boundary | Untrusted Side | Defence Mechanism |
|---|---|---|
| `/api/v1/auth/setup` | Public Client | Only permitted when `UserRepository.Count() == 0`. Once admin is created, any subsequent setup call returns 409 Conflict. |
| `/api/v1/auth/login` | Public Client | In-memory IP rate limiter (5 req/min burst). Argon2id constant-time verification. Zero timing leakage on unknown username vs bad password. Returns generic 401 error. |
| `/api/v1/auth/refresh` | Public Client | SHA-256 hashed refresh token in database. Rotated on every use (single-use semantics). Revocation on logout. IP rate limited. |
| `/api/v1/auth/mfa/totp/verify` | Public Client | Requires valid `mfaTicket` signed by JWT with claim `purpose: mfa_challenge`. Single-use, 5-minute expiry. Rate-limited TOTP code verification with replay prevention. |
| JWT Bearer Token | Client / Attacker | Signed with HS256 using subkey `jwt-signing-secret-v1`. Header must be exactly `{"alg":"HS256","typ":"JWT"}`. Rejects `alg: none`. Verifies `exp`, `nbf`. Short-lived (15 min). |
| TOTP Secret Storage | Database at Rest | Secrets encrypted with AES-256-GCM using subkey `mfa-totp-master-v1` before persisting to `mfa_totp_settings`. Recovery codes stored strictly as SHA-256 hashes. |
| Multi-Library Scoping | Authenticated User | Active library ID validated against `LibraryMembershipRepository`. If user is not an active member, returns 403 Forbidden. Reading progress, bookmarks, highlights scoped by `user_id`. |
| Ingestion & Sources RBAC | Authenticated User | Endpoints for source management and ingestion require `admin` role OR `allowReaderUploads: true` on the active library. Checked via `RequireIngestPermission`. |
| Log Redaction (§8) | Log Aggregator / Sinks | Zero credentials, raw passwords, password hashes, JWT tokens, TOTP secrets, recovery codes, or reset tokens emitted to structured logs. |

---

## Four-Attacker Threat Model (Constitution §10)

### 1. The Unprivileged / Malicious User (Reader Role)
- **Privilege Escalation Attempt**: Reader user attempts to access `/api/v1/sources` or `/api/v1/libraries/{id}/members`.
  - *Outcome*: `RequireRole(domain.RoleAdmin)` middleware rejects the request with HTTP 403 Forbidden.
- **Cross-User Data Exfiltration**: Reader user queries `/api/v1/reading/works/{id}/progress` or `/api/v1/reading/export`.
  - *Outcome*: Handlers query `FindByWorkAndUser(ctx, workID, user.ID)` and `reading_preferences` scoped by `user.ID`. User can only inspect their own bookmarks, highlights, progress, and preferences.
- **Cross-Library Partition Crossing**: Reader sends `X-Library-Id: <other-library-uuid>`.
  - *Outcome*: `LazyAuthMiddleware` executes `membershipRepo.FindMembership(ctx, libraryID, user.ID)`. If membership does not exist or user is not a member, access to library routes is blocked with HTTP 403.
- **Unauthorized Ingest / Upload**: Reader attempts to upload or trigger import discovery.
  - *Outcome*: `RequireIngestPermission` checks if user is Admin OR if `library.AllowReaderUploads` is true. If false, returns 403 Forbidden.

### 2. The Network Adversary (LAN / MitM)
- **Token Interception / Replay**: Network adversary captures network traffic.
  - *Outcome*: Access tokens expire in 15 minutes. Refresh tokens are rotated immediately upon use. All authentication headers use standard `Authorization: Bearer`. (Phase 13 introduces TLS for LAN/WAN deployments).
- **JWT Header Forgery / Algorithm Confusion**: Adversary crafts JWT with `{"alg": "none"}` or RSA public key confusion.
  - *Outcome*: `JWTSigner.Verify` strictly checks that the decoded header matches `{"alg":"HS256","typ":"JWT"}` and computes HMAC-SHA256 with the derived secret key. All other algorithms or unsigned tokens are rejected.
- **Brute-Force & Credential Stuffing**: Adversary floods login and password reset endpoints.
  - *Outcome*: `IPRateLimiter` enforces strict token-bucket rate limiting per IP address. Failed attempts trigger HTTP 429 Too Many Requests before cryptographic hashing occurs.

### 3. Malicious File / OPDS Provider
- **Credential Exfiltration via Feed**: Malicious OPDS server or crafted EPUB file attempts to steal session tokens.
  - *Outcome*: Content iframe sandbox (`allow-same-origin`, no `allow-scripts`) and blue-monday HTML/CSS sanitizers (Phase 11) prevent JavaScript execution or cross-origin token leaks. Session tokens are stored in browser memory/localStorage and never injected into document frames.

### 4. Compromised Server / Database Dump / Dependency
- **Offline Password Cracking from DB Dump**: Attacker gains read-only access to PostgreSQL `user_credentials` table.
  - *Outcome*: Passwords are hashed with Argon2id using 64 MiB memory, 3 iterations, 4 threads, and unique 16-byte random salts. Fast GPU-based hash cracking is computationally infeasible.
- **TOTP Secret Exfiltration from DB Dump**: Attacker dumps `mfa_totp_settings`.
  - *Outcome*: TOTP secrets are stored encrypted with AES-256-GCM. Without the derived `mfa-totp-master-v1` key, the ciphertext cannot be decrypted. Recovery codes are stored only as SHA-256 hashes.
- **Dependency Compromise**:
  - *Outcome*: `internal/domain` has zero third-party dependencies. Crypto implementation relies on Go standard library (`crypto/subtle`, `crypto/rand`, `crypto/sha256`, `crypto/hmac`, `crypto/cipher`) and official `golang.org/x/crypto/argon2` and `golang.org/x/time/rate`.

---

## STRIDE Analysis

| Threat | Description & Assessment |
|---|---|
| **Spoofing** | **Mitigated**. Strong authentication via Argon2id, signed HS256 JWTs, optional RFC 6238 TOTP MFA with encrypted secrets and hashed recovery codes. `LazyAuthMiddleware` validates token signature and expiration on every authenticated request. |
| **Tampering** | **Mitigated**. Database integrity enforced with foreign key cascades, unique compound indexes, and 100% parameterized SQL queries. JWT tokens cannot be modified without invalidating HMAC-SHA256 signature. |
| **Repudiation** | **Mitigated**. All authentication operations emit structured correlation IDs. Auth events (login, failed login, setup, password reset, MFA verification) log IP address and correlation ID without leaking credentials. |
| **Information Disclosure** | **Mitigated**. Sensitive authentication artifacts (raw passwords, password hashes, JWT signatures, TOTP secrets, recovery codes) are redacted from logs and error messages. Error responses return standard generic error envelopes (`architecture-contracts.md FR-5`). |
| **Denial of Service** | **Mitigated**. In-memory token-bucket rate limiting protects login, refresh, and password reset routes. Argon2id resource parameters are bounded. Database queries use indexed lookups. |
| **Elevation of Privilege** | **Mitigated**. Strict RBAC middleware (`RequireRole`, `RequireIngestPermission`) validates user role from validated JWT claims before handler execution. Multi-library membership verified at boundary. |

---

## Verification Results

1. **Go Unit & Race Tests**:
   - `go test -race ./internal/... ./cmd/...`: **PASS** across all 21 packages.
2. **OpenAPI 3.1 Contract Tests**:
   - `go test -v ./internal/testutil/contracttest`: **PASS** for all auth, library, reading, source, and discovery endpoints.
3. **Frontend Vitest & Accessibility Tests**:
   - `npm --prefix web test`: **PASS** (75 test files, 379 tests passing).
4. **Frontend TypeScript & ESLint**:
   - `npm --prefix web run lint`: **PASS** (0 errors, 0 warnings).
   - `npm --prefix web run build`: **PASS** (clean Vite + TypeScript compile).
5. **Project Quality & Boundary Scripts**:
   - `scripts/check-import-boundaries.sh`: **clean**
   - `scripts/check-parameterized-queries.sh`: **clean**
   - `scripts/check-integration-test-parallelism.sh`: **clean**
   - `scripts/check-compose-published-port.sh`: **clean**
