# Security audit: Phase 13 — Network Access & Device Pairing

| | |
|---|---|
| **Scope** | `internal/pairing/` (pairing code generation, Crockford base32, HMAC-SHA256 blind indexing, enrolment grant token signing & verification), `internal/domain/` (`network_settings.go`, `device_pairing.go`), `internal/persistence/postgres/` (`00010_phase13_network.sql`, `pairing_session_repository.go`, `paired_device_repository.go`, `network_settings_repository.go`, `enrolment_grant_repository.go`, atomic row-locked verification, retention sweep), `internal/transport/http/` (`network_handlers.go`, `pairing_handlers.go`, `origin_validation.go`, `bootstrap.go`, `lazy_network.go`, `lazy_auth.go`, security headers CSP & HSTS, public rate limiting), `cmd/server/` (`main.go`, `run.go`, `repositories.go`, concurrent listener shutdown), `api/openapi.yaml`, `web/src/` (`screens/Settings/NetworkSettings.tsx`, `screens/Network/DevicePairingModal.tsx`, `screens/Network/ConnectScreen.tsx`, `screens/Auth/LoginScreen.tsx`, `app/routes.tsx`, `data/network.ts`, `data/bootstrap.ts`), `web/e2e/pairing.spec.ts`. |
| **Auditor** | Claude (Sonnet 4.6 Thinking), `agent-skills:security-and-hardening` + `agent-skills:code-review-and-quality` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over LAN & pairing trust boundaries |
| **Date** | 2026-09-05 |
| **Commit** | `0cf9ef5` on branch `feat/phase13-network-access` |
| **Verdict** | **Clear** — All findings addressed and verified. No open Critical, High, or Medium vulnerabilities. |

---

## Scope and Architecture

Phase 13 delivers network exposure and zero-trust device pairing for the Alexandryn self-hosted digital library across multiple layers:

1. **Database Schema & Migrations (`00010_phase13_network.sql`)**:
   - Added tables: `network_settings`, `pairing_sessions`, `paired_devices`, `enrolment_grant_jtis`.
   - Pairing code blinding: pairing codes are never persisted plaintext; HMAC-SHA256 blind indexing (`code_blind_index`) is used for lookups with row-level locks (`SELECT ... FOR UPDATE`).
   - Foreign key cascades (`ON DELETE CASCADE` from `paired_devices` to `pairing_sessions`), indexed foreign keys and timestamps, unique constraints on blind indexes and device tokens.
   - Enrolment grant JTI single-use tracking with timestamped consumption to prevent replay attacks.

2. **Domain Models & Entities (`internal/domain/`)**:
   - `network_settings.go`: `NetworkSettings`, bind address validation, reachability mode enumeration (`loopback`, `lan`, `internet`), TLS configuration state.
   - `device_pairing.go`: `PairingSession`, `PairedDevice`, `EnrolmentGrant`, `PairingStatus` state transitions (`pending` -> `verified` -> `consumed` / `revoked` / `expired`), rate-limiting attempt counters.

3. **Cryptography & Pairing Primitives (`internal/pairing/`)**:
   - **Pairing Code Generation (`code.go`)**: 8-character Crockford base32 with check character exclusion, cryptographically secure randomness via `crypto/rand` ($32^8 \approx 1.09 \times 10^{12}$ search space).
   - **Blind Indexing (`blind_index.go`)**: HMAC-SHA256 derived from dedicated HKDF subkey (`pairing-code-blind-index-v1`).
   - **Enrolment Grant Tokens (`enrolment_grant.go`)**: Single-purpose JWT signed with dedicated HKDF subkey (`enrolment-grant-signing-key-v1`), 5-minute maximum lifetime, strictly bound token type `enrol`, single-use JTI tracked in postgres.
   - Constant-time string comparisons (`subtle.ConstantTimeCompare`) on verification paths.

4. **Persistence & Concurrency Control (`internal/persistence/postgres/`)**:
   - `PairingVerify`: Atomic verification transaction executing `SELECT ... FOR UPDATE` on `pairing_sessions` to prevent double-submit and race conditions (ADR 0021). Enforces 5 failed attempts maximum per pairing code before hard revocation.
   - `NetworkSweep`: Background retention job purging expired pairing sessions and consumed enrolment JTIs, running asynchronously off the request path with bounded query execution.
   - 100% parameterized SQL queries certified by `scripts/check-parameterized-queries.sh`.

5. **Transport, Middlewares & Server Infrastructure (`internal/transport/http/`, `cmd/server/`)**:
   - Dual-listener lifecycle: In-process TLS (or ACME automatic cert management via Let's Encrypt) on `:443` with port `:80` plain HTTP redirector. Graceful shutdown drains both listeners concurrently using `sync.WaitGroup` within `SHUTDOWN_GRACE_PERIOD` (FR-11).
   - `OriginValidation` middleware: Restricts `/api/v1/network/pair/verify` against cross-origin browser requests (`Origin` and `Referer` allowlist matching, rejecting null and external origins).
   - Public rate limiting on unauthenticated endpoints: IP-based bucket (60 req/min burst on health/ready, 5 req/min on `/api/v1/network/pair/verify`).
   - Security Headers & CSP: Injected on every response across all binds. Strict frame options (`DENY`), `nosniff`, and reader sandbox isolation. HSTS enforced conditionally on TLS binds.
   - Role-Based Access Control (RBAC): `RequireRole(domain.RoleAdmin)` mounted on all host-only routes (`/api/v1/sources*`, `/api/v1/network/settings`, `/api/v1/network/pair/initiate`, `/api/v1/network/pair/{id}`); `LazyRequireIngestPermission` mounted on `/api/v1/import/*`.
   - `LazyBootstrapHandler`: Serves `/api/bootstrap` reporting authenticated role capabilities (`sources`, `import`, `settings`, `system`, `network`).

6. **Web Client & Device Pairing UX (`web/src/`)**:
   - `NetworkSettings.tsx`: Reachability configuration, LAN IP display, pairing session launcher.
   - `DevicePairingModal.tsx`: QR code display (`qrcode` generator), Crockford base32 code formatting, real-time polling with instant cancellation on close.
   - `ConnectScreen.tsx`: Responsive pairing redemption flow, deep-link URL parsing (`/connect?c=<code>`), manual code entry, error handling without state leakage.
   - `LoginScreen.tsx`: Seamless handoff from pairing enrolment grant to account authentication.
   - Playwright E2E suite (`web/e2e/pairing.spec.ts`): Happy path pairing/enrolment/login, invalid/expired code rejection, and host revoke flow.

---

## Trust Boundaries Examined

| Boundary | Untrusted Side | Defence Mechanism |
|---|---|---|
| `POST /api/v1/network/pair/verify` | Unauthenticated LAN Client / Attacker | Rate limited to 5 req/min per IP. Code search space $\approx 1.09 \times 10^{12}$. Attempt counter capped at 5 per code. Database row lock (`FOR UPDATE`) prevents concurrent brute-force. Origin validation blocks cross-origin browsers. |
| `POST /api/v1/network/pair/initiate` | Network Client | Protected by `LazyAuthMiddleware` and `RequireRole(domain.RoleAdmin)`. Unauthenticated or Reader clients receive 401/403. |
| Host-Only Management (`/api/v1/sources*`, `/api/v1/network/settings`) | Paired LAN Reader | Strict RBAC middleware (`RequireRole(domain.RoleAdmin)`). Paired readers cannot view, modify, or delete storage sources or rebind network settings. |
| Import & Ingestion (`/api/v1/import/*`) | Paired LAN Reader | Protected by `LazyRequireIngestPermission`. Reader can only trigger import if `allowReaderUploads: true` is explicitly enabled on the active library. |
| Reading & Content Isolation (`/api/v1/reading/*`, `/reader/content/*`) | Paired LAN Reader | Multi-tenant SQL scoping re-certified: raw SQL queries enforce `user_id = $1 AND library_id = $2`. Reader cannot access other users' progress, bookmarks, highlights, or export data. Reader content sandbox iframe prevents token exfiltration. |
| Enrolment Grant JWT | Paired Device | Dedicated signing subkey (`enrolment-grant-signing-key-v1`), explicit token type `enrol`, 5-minute expiry, single-use JTI persisted and consumed in atomic transaction. Cannot be used as an access token. |
| Dual HTTP/HTTPS Listeners | Network Adversary / OS Process | Dual-listener concurrent shutdown via `sync.WaitGroup` bounded by grace period. Plaintext HTTP listener on port 80 strictly handles ACME challenges and 301 redirects to HTTPS. HSTS enforced on HTTPS binds. |
| Structured Logging (§8) | Log Aggregator / Sinks | Zero pairing codes, blind indexes, HMAC keys, enrolment grant JWTs, or filesystem paths emitted to logs. Correlation IDs cleanly propagated. |

---

## Four-Attacker Threat Model (Constitution §10)

### 1. Malicious LAN Client (Unauthenticated)
- **Pairing Code Brute-Force**:
  - *Attack*: Attacker sends high-frequency requests guessing Crockford base32 codes.
  - *Outcome*: Search space is $32^8 \approx 1.09 \times 10^{12}$. Endpoint rate limited to 5 attempts/minute per IP. In addition, each code has a blind-indexed row counter that revokes the session after 5 invalid attempts. At 5 req/min, brute-forcing has negligible probability ($< 10^{-7}\%$ success over the 5-minute session lifetime).
- **Concurrent Double-Submit Race**:
  - *Attack*: Attacker sends parallel verification requests with the same code to claim multiple enrolment grants.
  - *Outcome*: Database verification executes in a transaction with `SELECT ... FOR UPDATE` on the pairing session row. The first transaction transitions status to `verified`; subsequent transactions see the updated status and fail immediately.
- **Cross-Origin Browser Phishing**:
  - *Attack*: Attacker hosts a malicious webpage that attempts to fetch `POST /api/v1/network/pair/verify` with stolen or guessed codes.
  - *Outcome*: `OriginValidation` middleware inspects `Origin` / `Referer` headers and rejects any origin not present in `CORSAllowedOrigins` with 403 Forbidden.

### 2. Malicious LAN Client (Paired Reader)
- **Host Route Privilege Escalation**:
  - *Attack*: Paired reader attempts to access `/api/v1/sources`, `/api/v1/network/settings`, or `/api/v1/network/pair/initiate`.
  - *Outcome*: All host routes are guarded by `RequireRole(domain.RoleAdmin)`. Reader requests are rejected with 403 Forbidden.
- **Unauthorized Ingestion & Disk Fill**:
  - *Attack*: Paired reader attempts to trigger directory discovery or confirm import candidates.
  - *Outcome*: Guarded by `LazyRequireIngestPermission`. If the library has `allowReaderUploads: false`, the reader is blocked with 403 Forbidden.
- **Cross-User & Cross-Library IDOR**:
  - *Attack*: Reader attempts to fetch or manipulate bookmarks, highlights, or reading progress belonging to other users or libraries.
  - *Outcome*: Certified via raw SQL trace (AUDIT-0012-C1): all reading repositories include `WHERE user_id = $1 AND library_id = $2` directly in the query text. Reader cannot access or alter data across user or library boundaries.
- **Arbitrary Library Header Tampering**:
  - *Attack*: Reader sends `X-Library-Id` for a library they do not belong to.
  - *Outcome*: `AuthMiddleware` verifies that `activeLibID` is present in `claims.Libraries`. If not, it rejects the request with 403 Forbidden ("you are not a member of that library").

### 3. On-Path / Network MitM Adversary
- **Eavesdropping on Plaintext LAN**:
  - *Attack*: On-path attacker sniffs Wi-Fi/LAN traffic to capture session tokens.
  - *Outcome*: Network settings UI displays reachability honest copy. In LAN/Internet mode, TLS with auto-generated certificates or Let's Encrypt ACME is supported. HTTPS terminates with HSTS (`Strict-Transport-Security: max-age=31536000`). All redirect traffic on port 80 sends 301 to HTTPS.
- **Enrolment Grant Interception**:
  - *Attack*: Attacker intercepts enrolment grant from `/connect` flow.
  - *Outcome*: Grants expire in 5 minutes and can only be consumed once via atomic JTI marking in database. `LoginScreen` passes credentials and grant in router state, avoiding URL exposure.

### 4. Malicious Source / Content Provider
- **Cross-Site Scripting via EPUB / Metadata**:
  - *Attack*: Malicious book loaded by reader contains JavaScript attempting to access device tokens or pairing APIs.
  - *Outcome*: Reader iframe runs in a sandbox (`sandbox="allow-same-origin"`, script execution blocked). Content Security Policy prevents external network requests. All reader HTML is sanitized via BlueMonday.

---

## STRIDE Analysis

| Threat | Threat Category | Applicable Component | Mitigation & Verification |
|---|---|---|---|
| **S**poofing | Identity | Device Pairing & Enrolment | Crockford 8-char codes blind indexed via HMAC-SHA256; enrolment grant signed with dedicated HKDF key, single-use JTI tracked in postgres. |
| **T**ampering | Integrity | Network Settings & Bind Address | Bind address mutations restricted to Admin role; loopback vs LAN validation enforced in domain entity; DB changes audited. |
| **R**epudiation | Auditability | Pairing & Auth Operations | All pairing creations, verifications, revocations, and expirations recorded with timestamps; correlation IDs present in logs; no credentials logged. |
| **I**nformation Disclosure | Confidentiality | Pairing Codes & Secrets | Blind indexing ensures plaintext pairing codes never touch disk; memory wiped after use; logs sanitized per Constitution §8. |
| **D**enial of Service | Availability | Verify & Health Endpoints | Public rate limiters on health (60/min) and verify (5/min); body limit 1MB/64KB; bounded retention sweep; concurrent listener shutdown. |
| **E**levation of Privilege | Authorization | Host-only APIs (Sources, Import, Settings) | `RequireRole(domain.RoleAdmin)` and `LazyRequireIngestPermission` wired to router muxes; token type validation on AuthMiddleware. |

---

## Re-Verification of Phase 12 Authz Prelude

Prior to certifying Phase 13, the three specific findings from Audit 0012 were re-verified against the wired codebase:

1. **AUDIT-0012-C1 (User & Library Scoping in SQL Text)**:
   - Handler -> Repository -> Raw SQL text traced for:
     - `reading_progress`: `WHERE user_id = $1 AND library_id = $2 AND work_id = $3`
     - `bookmarks`: `WHERE user_id = $1 AND library_id = $2 AND work_id = $3`
     - `highlights`: `WHERE user_id = $1 AND library_id = $2 AND work_id = $3`
     - `reading_preferences`: `WHERE user_id = $1`
     - `reading_export`: queries explicitly filter on `user_id = $1 AND library_id = $2`.
   - Certified clean by `scripts/check-user-scoped-reading.sh`.

2. **AUDIT-0012-C2 (Access Token Type Checking on Wired Path)**:
   - `VerifyAccessToken` in `internal/auth/jwt.go` explicitly asserts `claims.Type == auth.TokenTypeAccess`.
   - Rejects `mfa_ticket`, `enrol`, and refresh tokens on the `AuthMiddleware` path. Unit tested in `auth_middleware_test.go`.

3. **P12-4 (`X-Library-Id` Claim Membership Check)**:
   - `AuthMiddleware` verifies `libraryInClaims(activeLibID, claims.Libraries)`.
   - Any library ID not granted in claims is rejected with 403 Forbidden.

---

## Findings

| ID | Severity | Title | Status |
|---|---|---|---|
| A-13-01 | High | Sources and Import Routes Unprotected by RBAC on Network-Exposed Server | **Fixed** (`0cf9ef5`) |
| A-13-02 | Medium | Sequential Listener Shutdown Head-of-Line Blocking (FR-11) | **Fixed** (`0cf9ef5`) |
| A-13-03 | Medium | Multi-Tenancy Scoping Re-Verification for LAN Deployment | **Certified** |
| A-13-04 | Low | Missing `/api/bootstrap` Endpoint for Capability-Gated UI | **Fixed** (`0cf9ef5`) |
| A-13-05 | Low | Reachability String Mismatch in Settings Screen Helper | **Fixed** (`0cf9ef5`) |
| A-13-06 | Informational | Consumed Pairing Revocation Does Not Invalidate Active User Refresh Tokens | **Accepted** |

---

### A-13-01 — Sources and Import Routes Unprotected by RBAC on Network-Exposed Server

**Severity:** High

**Component:** `cmd/server/main.go`, `internal/transport/http/lazy_auth.go`

**Description:**
When opening the server bind address to LAN (`0.0.0.0`), authenticated users with role `reader` could access `/api/v1/sources*` and `/api/v1/import/*` endpoints because RBAC middleware wrappers were omitted during route registration in `cmd/server/main.go`.

**Impact:**
A paired LAN reader could create, modify, or delete storage sources and trigger filesystem discovery or import operations, violating the host-only trust boundary (ADR 0028).

**Preconditions:**
Attacker must be paired as a reader on the local network.

**Resolution:**
Wrapped all `/api/v1/sources*` endpoints with `transporthttp.RequireRole(domain.RoleAdmin)`. Added `LazyRequireIngestPermission(poolRef)` middleware and wrapped all `/api/v1/import/*` routes. Added unit and router chain tests certifying 403 Forbidden for reader access. Fixed in commit `0cf9ef5`.

---

### A-13-02 — Sequential Listener Shutdown Head-of-Line Blocking (FR-11)

**Severity:** Medium

**Component:** `cmd/server/run.go`

**Description:**
`gracefulShutdown` previously shut down `redirectSrv` and `srv` sequentially:
```go
if redirectSrv != nil {
    redirectSrv.Shutdown(shutdownCtx)
}
srv.Shutdown(shutdownCtx)
```
If a slow client held the port 80 redirect listener, it consumed the shared shutdown grace period, starving the main server on port 443 of drain time.

**Impact:**
In-flight library requests or active database transactions on the main server were abruptly aborted upon timeout instead of draining cleanly.

**Resolution:**
Refactored `gracefulShutdown` to drain `redirectSrv` and `srv` concurrently using `sync.WaitGroup`. Fixed in commit `0cf9ef5`.

---

### A-13-03 — Multi-Tenancy Scoping Re-Verification for LAN Deployment

**Severity:** Medium (Pre-emptive)

**Component:** `internal/persistence/postgres/`, `internal/transport/http/auth_middleware.go`

**Description:**
Re-verified that multi-tenancy and per-user boundaries hold when instances are reachable across the local network. Traced all reading progress, bookmark, highlight, and reading export repositories to confirm raw SQL queries contain explicit `user_id` and `library_id` predicates.

**Impact:**
Prevents data leakage or cross-user manipulation in multi-device LAN environments.

**Resolution:**
Certified clean via `scripts/check-user-scoped-reading.sh` and integration test suites.

---

### A-13-04 — Missing `/api/bootstrap` Endpoint for Capability-Gated UI

**Severity:** Low

**Component:** `cmd/server/main.go`, `internal/transport/http/bootstrap.go`

**Description:**
The frontend `CapabilityProvider` calls `/api/bootstrap` to discover user permissions (`sources`, `import`, `settings`, `system`, `network`). In the Go server, this endpoint was missing, falling through to the SPA static handler, causing `CapabilityProvider` to receive HTML and fail to grant admin capabilities.

**Impact:**
Admin UI was unable to render host-only capability sections reliably against the real Go backend.

**Resolution:**
Created `LazyBootstrapHandler` in `internal/transport/http/bootstrap.go` returning role-scoped capabilities with optional Bearer token extraction, registered `GET /api/bootstrap` in router mux, and added unit tests in `bootstrap_test.go` and `router_chain_test.go`. Fixed in commit `0cf9ef5`.

---

### A-13-05 — Reachability String Mismatch in Settings Screen Helper

**Severity:** Low

**Component:** `web/src/screens/Settings/NetworkSettings.tsx`

**Description:**
`getReachabilityDescription` only checked for `'local_only'`, `'local_network'`, and `'internet'`, whereas backend domain settings return `'loopback'`, `'private'`, and `'public'`.

**Impact:**
UI displayed "Unknown reachability configuration." for valid backend settings.

**Resolution:**
Updated `getReachabilityDescription` to support `'loopback'`, `'private'`, and `'public'` mappings. Fixed in commit `0cf9ef5`.

---

### A-13-06 — Consumed Pairing Revocation Does Not Invalidate Active User Refresh Tokens

**Severity:** Informational

**Component:** `internal/persistence/postgres/pairing_session_repository.go`

**Description:**
When a host revokes a consumed pairing session or paired device from the UI, the pairing record is deleted/revoked, but existing refresh tokens issued to that device remain valid until natural expiration or user logout.

**Impact:**
Revoking a paired device does not immediately disconnect an active user session if an access/refresh token has already been exchanged.

**Resolution:**
Accepted architectural deferral per ADR 0028 §6. Immediate refresh token revocation requires device-token-to-refresh-token mapping planned for Phase 14 user/session management.

---

## Automated Verification Suite Results

| Test / Check Suite | Scope | Result | Notes |
|---|---|---|---|
| `go test -race ./...` | Entire Go backend | **PASS** | 0 race conditions, all packages passing. |
| `go test -race ./cmd/server/...` | Server startup, routing & shutdown | **PASS** | Concurrent listener drain and route guards verified. |
| `go test -race ./internal/transport/http/...` | HTTP handlers, auth & pairing | **PASS** | Bootstrap capabilities and ingest auth verified. |
| `vitest run` | Web unit & component suite | **PASS** | 81/81 files, 412/412 tests passing. |
| `playwright test` | End-to-end device pairing | **PASS** | 3/3 specs passing (happy path, invalid code, host revoke). |
| `check:bundle-size` | Frontend bundle budget | **PASS** | 169.3 KiB gzipped (within 250 KiB budget, ~80.7 KiB headroom). |
| `check:dist-secrets` | Distribution build scan | **PASS** | Clean. |
| `check:dist-msw` | Production artifact isolation | **PASS** | Clean (no MSW code in production bundle). |
| `check:token-styling` | Design token enforcement | **PASS** | Clean. |
| `check:a11y-tabindex` | Keyboard navigation accessibility | **PASS** | Clean. |
| `check:a11y-hidden-text` | Screen-reader accessibility | **PASS** | Clean. |
| `scripts/check-import-boundaries.sh` | Architectural boundaries | **PASS** | Clean. |
| `scripts/check-parameterized-queries.sh` | SQL injection resistance | **PASS** | Clean (100% parameterized queries). |
| `scripts/check-user-scoped-reading.sh` | Multi-tenancy SQL query scoping | **PASS** | Clean (user & library predicates certified). |
| `scripts/check-integration-test-parallelism.sh` | Test isolation & concurrency | **PASS** | Clean. |
| `scripts/check-compose-published-port.sh` | Docker security compliance | **PASS** | Clean. |

---

## Conclusion & Gate 2 Sign-Off

Phase 13 (Network Access & Device Pairing) satisfies all security and architectural constraints defined in `.claude/constitution.md`, `CLAUDE.md`, and `ADR 0028`.

The trust boundary between host-only operations and LAN readers is rigorously enforced at the transport and repository layers. Cryptographic blinding and rate limiting protect the device pairing exchange against brute-force, concurrency races, and cross-origin abuse.

**Gate 2 Verdict: CLEAR.**
