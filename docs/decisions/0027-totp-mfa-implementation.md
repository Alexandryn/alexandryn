# 0027. RFC 6238 TOTP Multi-Factor Authentication with AES-GCM Secret Storage and Recovery Codes

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-02 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Multi-Factor Authentication (MFA / 2FA) is required in Phase 12 to secure privileged administrator and reader accounts against credential stuffing and session hijacking.

The implementation must:
1. Comply with RFC 6238 (Time-Based One-Time Password Algorithm) and RFC 4226 (HMAC-Based One-Time Password Algorithm).
2. Work seamlessly with standard authenticator applications (Google Authenticator, Authy, 1Password, Bitwarden, Apple Passwords).
3. Protect TOTP shared secrets at rest against database leaks using authenticated symmetric encryption (AES-256-GCM).
4. Provide single-use backup recovery scratch codes for emergency account recovery.
5. Provide standard QR code rendering without relying on external third-party generation services (Constitution §4, §8).

## Decision

1. **Algorithm & Parameters**
   - HMAC Algorithm: SHA-1 (`crypto/sha1`) per RFC 6238 default compatibility with mobile authenticator apps.
   - Digits: 6 decimal digits.
   - Time Step: 30 seconds.
   - Verification Window: ±1 step (current window, previous 30s, next 30s) to account for slight clock drift between client devices and the server.
   - Replay Protection: A TOTP token successfully verified within a 30s window records `last_used_step` to prevent immediate replay.

2. **Secret Generation & Storage Encryption**
   - Secret Generation: 20 cryptographically random bytes (`crypto/rand`), Base32-encoded without padding (32 characters).
   - Secret Storage: Encrypted at rest in `mfa_totp_settings.encrypted_secret` using AES-256-GCM (`crypto/cipher`, `crypto/aes`) with a 12-byte random nonce prepended to the ciphertext.
   - Master Encryption Key: Derived from `config.MFASecretKey` or `config.JWTSecret` using HKDF-SHA256.

3. **Backup Recovery Codes**
   - 8 single-use recovery codes generated upon MFA enrollment.
   - Format: 8 alphanumeric characters (formatted as `XXXX-XXXX`).
   - Stored in database as Argon2id/SHA-256 hashes (`recovery_code_hashes`).
   - Using a recovery code invalidates that specific hash and logs a security audit event.

4. **QR Code Provisioning & URI Format**
   - Standard Key URI format: `otpauth://totp/Alexandryn:<username>?secret=<BASE32_SECRET>&issuer=Alexandryn&algorithm=SHA1&digits=6&period=30`.
   - The backend produces the `otpauth://` URI and returns it along with raw base32 secret. The frontend displays the URI, the manual secret, and generates the QR code via an SVG matrix renderer.

5. **Two-Step Enrollment & Verification Flow**
   - **Enrollment**:
     1. `POST /api/v1/auth/mfa/totp/setup` generates the secret, stores it as unconfirmed (`confirmed_at IS NULL`), and returns the secret + recovery codes.
     2. `POST /api/v1/auth/mfa/totp/confirm` verifies the first TOTP code. If valid, marks `confirmed_at = now()`, `enabled = true`.
   - **Login**:
     1. `POST /api/v1/auth/login` checks credentials. If user has MFA enabled, returns `200` with `{ mfaRequired: true, mfaTicket: "<signed-temporary-jwt>" }`.
     2. `POST /api/v1/auth/mfa/totp/verify` validates the `mfaTicket` and the 6-digit TOTP code (or recovery code). If valid, returns the full access token and refresh token.

## Options Considered

### Option A: Hand-rolled RFC 6238 TOTP using Go standard library + AES-256-GCM (Chosen)
*For*: Zero new external dependencies (RFC 6238 is <80 lines of straightforward standard Go code with `crypto/hmac`, `crypto/sha1`, `encoding/binary`). Full control over timing-attack mitigations, replay protection, and secret encryption.
*Against*: Must maintain and test the RFC 6238 algorithm against RFC test vectors.

### Option B: External third-party library (`github.com/pquerna/otp`)
*For*: Existing package.
*Against*: Pulls in extra transitive dependencies (`boombuler/barcode`). Increases supply chain attack surface when RFC 6238 logic is trivial to implement and verify with standard RFC test vectors.

## Consequences

**Good**:
- Full compatibility with all standard authenticator apps.
- Secrets are encrypted at rest; DB dump does not reveal plaintext TOTP keys.
- Standalone self-contained implementation with zero third-party packages.

**Bad / Trade-offs**:
- Requires secure key management for the master encryption key.

## Reversal Cost
Low. TOTP conforms to standard RFC 6238; can be toggled on/off per user or swapped with WebAuthn/FIDO2 in future phases.
