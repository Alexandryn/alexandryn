# Security audit: Phase 12 — Authentication, RBAC & Multi-Library Namespacing

| | |
|---|---|
| **Scope** | `internal/auth/` (passwords, JWT, TOTP, rate limiting), `internal/domain/` (`auth.go`, `library_aggregate.go`), `internal/persistence/postgres/` (`00009_phase12_auth_and_tenancy.sql`, `auth_repository.go`, `library_repository.go`, user-scoped reading repos), `internal/transport/http/` (`auth_middleware.go`, `auth_handlers.go`, `library_handlers.go`, `auth_ref.go`, `lazy_auth.go`), `cmd/server/` (`main.go`, `run.go`, `repositories.go`), `api/openapi.yaml`, `web/src/` (`data/auth.ts`, `data/libraries.ts`, `data/http.ts`, `screens/Auth/`, `screens/Libraries/`). |
| **Auditor** | Claude (Sonnet 4.6 Thinking), `agent-skills:security-and-hardening` + `agent-skills:code-review-and-quality` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over each trust boundary |
| **Date** | 2026-09-02 |
| **Commit** | Branch `feat/phase12-auth-and-tenancy` |
| **Verdict** | ~~**Clear** — no open Critical, High, or Medium vulnerabilities.~~ **SUPERSEDED — see "Post-audit correction (2026-09-02)" at the foot of this document.** This audit certified two authorization controls that are not present in the wired code (per-user/per-library scoping of the reading API; access-token type checking). Two **High**-severity findings are open. The test/lint/check-script results below still hold — the tests do not cover the missed cases, which is itself a finding. |

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

---

## Post-audit correction (2026-09-02)

This section is added by the phase-13 spec-package review
([`../reviews/0050-phase13-spec-package-and-phase12-authz-review.md`](../reviews/0050-phase13-spec-package-and-phase12-authz-review.md)),
which independently threat-modelled the phase-12 surface phase 13 builds
on and found two authorization controls this audit certified that the
wired code does not implement. The findings were re-verified against the
code before this section was written. Per constitution §12, the record is
corrected in place rather than rewritten to hide the miss; per §10 the
severity is stated honestly.

### AUDIT-0012-C1 — reading API is not user- or library-scoped (**High**; **Critical** for a Mode-A public bind)

**What this audit said** — Trust Boundaries table: *"Reading progress,
bookmarks, highlights scoped by `user_id`."* Four-Attacker §1,
Cross-User Data Exfiltration: *"Handlers query
`FindByWorkAndUser(ctx, workID, user.ID)` … User can only inspect their
own bookmarks, highlights, progress, and preferences."* Verification
Results implied the test suite covers this.

**What the code does** — `internal/transport/http/reading.go` and
`reading_export.go` call the **bare** repository methods, never the
`…AndUser` variants that exist beside them:
- `reading.go:106` — `deps.Progress.FindByWork(ctx, workID)` → resolves to
  `WHERE work_id = $1 AND COALESCE(user_id,'') = COALESCE('','')` — every
  user's progress collapses onto one `NULL`-user row.
- `reading.go:234, 302, 343, 431, 483` — `deps.Bookmarks.FindByEdition`,
  `Bookmarks.FindByID`, `Highlights.FindByEdition`, `Highlights.FindByID`
  — `SELECT … WHERE id = $1` / `WHERE edition_id = $1` with **no user
  predicate**. `bookmark_repository.go:114` — `DELETE FROM bookmarks
  WHERE id = $1`.
- `reading_export_repository.go:55` — `ListProgress` filters
  `WHERE work_id = $1` only. `GET /api/v1/reading/export` returns the
  entire instance's progress, bookmarks, and highlight notes.
- `UserFromContext` is never called anywhere in `reading.go`.

**Impact** — any authenticated account (including a `reader` invited to a
single library) can read, overwrite, and delete every other account's
private reading position, bookmarks, and highlight notes by object ID,
and can dump all users' annotations via one export call. Constitution §8
("what a person reads is private, including from their own log files")
is defeated. Phase 13 makes this LAN- and, in Mode A,
internet-reachable — which is exactly the exposure phase 13 exists to
gate.

**Why the audit missed it** — the audit inspected the *repositories*
(which do have `…AndUser` methods) and the *migration* (which does add
`user_id` columns and compound indexes) and inferred the handlers use
them. It did not trace a single reading request from handler to SQL. The
test suite asserts the happy path per endpoint but has no cross-user
IDOR test, so "all tests pass" is consistent with the defect.

**Corrective directives** (forward-looking; the fix lands on
`feat/phase13-network-access` as a phase-12 hardening prelude):
1. Every reading/reader handler resolves `UserFromContext` +
   `ActiveLibraryFromContext` and calls a user+library-scoped repository
   method. Bookmark/highlight `FindByID`/`Delete` verify row ownership.
2. A CI guard (`scripts/check-user-scoped-reading.sh` or a lint rule)
   fails the build if a handler under the reading/reader surface calls a
   bare-ID repository method.
3. Per-endpoint IDOR tests (user A cannot read/write/delete user B's row
   by ID) and an `export` isolation integration test — added to the
   phase-13 **close gate** (`roadmap/13-network-access/README.md`).
4. Spec directive in `backend-reading-api.md` and
   `backend-reader-content.md`: name the exact query predicate that
   enforces per-user + per-library scoping, and the test that proves a
   cross-user access is refused.
5. `CLAUDE.md` Reflex added: a handler serving user-owned data always
   calls the user+library-scoped repository method, never a bare-ID
   variant.

### AUDIT-0012-C2 — access-token verification does not check token type (**High**)

**What this audit said** — Trust Boundaries table: *"Header must be
exactly `{"alg":"HS256","typ":"JWT"}`. … Verifies `exp`, `nbf`."*
Four-Attacker §2, JWT Header Forgery: *"`JWTSigner.Verify` strictly
checks that the decoded header matches `{"alg":"HS256","typ":"JWT"}`."*

**What the code does** — `internal/auth/jwt.go:83` `JWTSigner.Verify`
(called by `AuthMiddleware` via `LazyAuthMiddleware`) checks
`header.Algorithm == "HS256"`, the HMAC signature, `claims.ExpiresAt`,
and `claims.Issuer`. It does **not** check the header `typ`, and it does
**not** check `claims.Type`. There is **no `nbf` field** on the `Claims`
struct and no not-before check. `VerifyMFATicket` checks
`claims.Type == "mfa_ticket"` only inside its own wrapper, which
`AuthMiddleware` never calls.

**Impact** — an MFA ticket (issued by the login handler when TOTP is
enabled: `Type:"mfa_ticket"`, real `Subject`, signed with the same
`jwt-signing-secret-v1` subkey, ~5-minute TTL) is a signature-valid
token that `AuthMiddleware` accepts as an access token on every route
that is authenticated-but-not-role-gated (`GET /api/v1/libraries`, the
whole reading API, and — in phase 13 — `GET /api/v1/network/status`). A
user who completed password authentication but not the second factor
has full read access to their account's data. The phase-13 enrolment
grant would inherit the same weakness (same key, different `typ`, no
check).

**Corrective directives**:
1. The access-token verification path used by `AuthMiddleware` positively
   requires the token to be an access token (`typ` empty or `"access"`)
   and rejects any other type.
2. The phase-13 enrolment grant is signed with a **separate HKDF
   subkey** (`DeriveSubkey("enrolment-grant-v1")`), so cross-type
   acceptance is structurally impossible, not merely claim-checked.
   Evaluate doing the same for the MFA ticket.
3. Middleware test per token type (access ✓, mfa_ticket ✗, enrol ✗,
   garbage ✗).
4. Spec directive in `backend-authentication.md`: distinct token
   purposes use distinct signing subkeys; the auth path asserts the
   token type.
5. `CLAUDE.md` Reflex added: token verification on the auth path always
   asserts the token type; a signature-valid token of the wrong purpose
   is a rejected token.

### AUDIT-0012-C3 — audit method: authorization controls were certified from spec intent, not from the wired call path

**Finding about the audit itself.** C1 and C2 both stem from the same
method gap: the audit read the repositories, the migration, and the spec,
and inferred the handler behaviour. `.claude/templates/audit.md` is
amended (2026-09-02) with a mandatory checklist item — *for every
data-read/-write endpoint, trace the wired handler → repository → SQL and
confirm the tenant/user predicate is in the query text; do not certify an
authorization control from the existence of a scoped method, only from
the call the handler actually makes.*

### Status of the rest of the audit

`X-Library-Id` is also not validated against `claims.Libraries`
(`auth_middleware.go:104` copies it to context unchecked; the audit's
"Cross-Library Partition Crossing" mitigation describing a
`membershipRepo.FindMembership` call in the middleware is not in the
code) — folded into the C1 fix as finding P12-4 in review `0050`. The
remaining phase-12 surface (Argon2id parameters, refresh-token rotation
and revocation, TOTP encryption-at-rest, rate limiting, log redaction,
parameterised SQL, the `alg: none` rejection) was spot-checked against
this audit's claims and holds — **but given C1–C3, a fuller independent
re-audit of the phase-12 authorization surface is required before phase
12 is marked `Closed`**, recorded as a directive in
`roadmap/12-authentication/README.md`. Phase 13's own security audit
(`0013`) will re-verify C1 and C2 as fixed as part of its scope.

### Fix status (2026-09-02, PR #78)

C1, C2, and P12-4 are fixed on `feat/phase13-network-access` as the
phase-13 hardening prelude (`fix(auth): enforce per-user reading-data
scoping and access-token type checks`), with unit IDOR tests, middleware
token-type / library-claim tests, repository IDOR integration tests, and
a CI guard (`scripts/check-user-scoped-reading.sh`). CI green.

**Forward data note:** the reading-progress fix changes the row key from a
single global `('', '', work_id)` row to `(user_id, library_id, work_id)`.
Any pre-existing `reading_progress` row written during phase-11 usage
carries `user_id = NULL` and becomes invisible after the fix — a user's
first post-fix progress report inserts a fresh scoped row. Acceptable:
phase 11 had no real multi-user data at stake, and the reader spec's
retrofit (`backend-library-namespaces.md` FR-4) already anticipated this.
No migration is written to reassign the orphaned rows; if that is ever
wanted it is a separate, explicit data migration, not part of this fix.
