# 0025. JWT-based session mechanism with Argon2id password hashing and refresh token rotation

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-02 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 12 introduces user authentication, role-based access control (RBAC), and multi-library namespacing. Previous phases operated under a loopback-trust assumption (Constitution §6, ADR 0017) without user authentication.

Authentication must support both the packaged Electron desktop app (single local operator, local webview renderer) and multi-client LAN / remote web deployments. The session mechanism must:
1. Provide cryptographically verifiable credentials per request without exposing database roundtrips on every static/read query.
2. Support secure token revocation upon logout or password change.
3. Protect against brute force and token theft.
4. Adhere strictly to Constitution §8 (zero token/password leaks in logs) and Constitution §9 (avoiding heavy, vulnerable third-party dependencies where standard cryptographic primitives suffice).

## Decision

1. **Password Hashing: Argon2id**
   - We use Argon2id via `golang.org/x/crypto/argon2` (already pinned in `go.mod`).
   - Hash parameters follow OWASP recommendations:
     - Memory: 64 MiB (`64 * 1024` KiB = `65536` KiB).
     - Iterations (Time): `3`.
     - Parallelism (Threads): `4`.
     - Salt: 16 cryptographically random bytes (`crypto/rand`).
     - Key length: 32 bytes.
   - Hash representation format: Standard PHC string `$argon2id$v=19$m=65536,t=3,p=4$<b64salt>$<b64key>`.
   - Verification uses constant-time comparison (`crypto/subtle.ConstantTimeCompare`).

2. **Token Mechanism: JWT with HS256 & Refresh Token Rotation**
   - **Access Tokens**:
     - Format: Standard JSON Web Token (RFC 7519) signed with HMAC-SHA256 (`HS256`).
     - Lifetime: Short-lived (15 minutes).
     - Payload claims:
       - `sub`: User ID (UUID).
       - `username`: User display name / handle.
       - `role`: Global role (`admin` or `reader`).
       - `libraries`: Array of library IDs accessible by the user.
       - `jti`: Unique token identifier (UUID).
       - `iat`: Issued-at unix timestamp.
       - `exp`: Expiration unix timestamp.
       - `iss`: `"alexandryn"`.
     - Implementation: Implemented with Go standard library cryptographic primitives (`crypto/hmac`, `crypto/sha256`, `encoding/base64`, `encoding/json`, `crypto/subtle`) inside `internal/auth/jwt` to minimize third-party dependency vulnerabilities (Constitution §9).
   - **Refresh Tokens & Sessions**:
     - Refresh tokens are cryptographically random 32-byte tokens (hex/base64url encoded).
     - Stored in the database (`user_refresh_tokens`) as a SHA-256 hash (`token_hash`) with metadata: `user_id`, `expires_at` (e.g., 30 days), `revoked_at` (nullable), `user_agent`, `ip_address`, `created_at`.
     - Token rotation is enforced: Every call to `POST /api/v1/auth/refresh` revokes the used refresh token and issues a new refresh token + access token pair.
     - Revocation on logout (`POST /api/v1/auth/logout`) sets `revoked_at = now()` on the active refresh token.

3. **Rate Limiting**
   - In-memory rate limiting via `golang.org/x/time/rate` attached to sensitive auth endpoints:
     - Login: 5 requests / minute per IP.
     - Refresh: 30 requests / minute per IP.
     - Password Reset: 3 requests / 15 minutes per IP.
   - Returns `429 Too Many Requests` when limits are exceeded.

4. **Open Redirect Protection**
   - Client redirect URLs after login or password reset are strictly validated: relative paths starting with a single `/` only (excluding protocol-relative `//` or external schemes `http:`, `https:`, `javascript:`).

## Options Considered

### Option A: Hand-rolled HS256 JWT + DB-backed refresh token rotation (Chosen)
*For*: Zero new external dependencies (uses standard `crypto/hmac` and `golang.org/x/crypto/argon2`). Fast verification for stateless reading/browsing requests. Clean separation between short-lived stateless access and revocable stateful refresh sessions.
*Against*: Requires maintaining ~120 lines of JWT signing/verification code in `internal/auth/jwt`.

### Option B: Stateful HTTP Cookies with Redis / DB sessions
*For*: Instant revocation without token expiration window.
*Against*: Heavy database load on every single HTTP request; not suitable for lightweight Electron desktop environments without external Redis; cross-domain complexities if clients access the server via mobile or third-party adapters.

### Option C: Third-party library (`github.com/golang-jwt/jwt`)
*For*: Widely used library.
*Against*: Introduces third-party dependency supply chain risk (Constitution §9). Standard HS256 signing and verification in Go is simple, self-contained, and easily audited.

## Consequences

**Good**:
- Fast request processing with stateless JWT validation.
- Secure credential storage (Argon2id) resistant to GPU/ASIC cracking.
- Refresh token rotation prevents token reuse and replay attacks.
- Strict rate limiting mitigates brute-force credential stuffing.

**Bad / Trade-offs**:
- In-memory rate limiting is per-process (sufficient for single-node Alexandryn instances).
- Revocation of access tokens takes up to 15 minutes unless an in-memory blocklist or refresh token check is performed.

## Reversal Cost
Low. The authentication middleware interfaces abstract token validation. Swapping signature algorithms (e.g. Ed25519) or session mechanisms requires updating `internal/auth/jwt` without changing route handlers or frontend contracts.
