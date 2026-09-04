# Phase 13 — Network access: task list

Plan with context, dependency graph, decisions, risks:
[`tasks/plan-phase13-network.md`](plan-phase13-network.md).

Execute in order. Each task: **ping the maintainer, get the go-ahead, then
RED** (write the failing tests — from the named spec FRs / test-strategy
rows) → GREEN → Refactor. Stop at every **Checkpoint**. Do not cross
**Gate 2** (security audit) without maintainer sign-off.

Branch: `feat/phase13-tier0-config` (has the uncommitted Tier 0). Cut a
fresh `feat/phase13-*` branch per tier from the previous tier's tip, or keep
one `feat/phase13-network-access` branch — maintainer's call at Checkpoint 0.
**Never `main`.** Strip `Co-Authored-By: Claude` / `Claude-Session:`
trailers.

Project-wide Definition of Done applies to every task: tests pass, no
regressions, `go vet` / lints clean, behaviour verified, docs updated.

## Progress

Umbrella branch: `feat/phase13-network-access` (one PR for the whole
phase, supersedes the Tier-0-only PR #79).

- [x] **Tier 0** — config keys + bind classifier + `run.go` TLS wrap.
      Commits `0a447d5`, `4449bb6`, `2a980de`. Code review: 3 findings
      fixed. Security review: clean. Checkpoint 0 passed.
- [x] **Tier 1** — pure pairing domain (`device_pairing.go`). Commit
      `0f862b4`. T1.1–T1.4 landed as one cohesive commit (single domain
      file; tests written first). Checkpoint 1 passed: `go test -race
      ./internal/domain/...` green, illegal-transition table + constant-
      time-`Equal` + FR-10 audit tests pass.
- [~] **Tier 2 part 1** — the self-contained units: `NewTLSConfig`,
      `SecurityHeaders`+`HSTS`, `CORS`, `OriginValidation`,
      `PublicRateLimit`, `auth.EnrolmentGrantSigner`,
      `pairing.GeneratePairingCode`. Commits `aaa0fd7`, `e3241ab`. All
      unit-tested in isolation, not yet mounted.
- [x] **Tier 2 part 2** — middleware-chain assembly + `NewTLSConfig`
      policy (T2.10), commits `f5e8e51`/`122ecba`/`cf2f5f1`. Chain:
      recovery→limits→logging→security-headers→HSTS→rate-limit→CORS→auth→routing.
- [x] **Tier 2 part 3** — `:80` HTTP→HTTPS redirect + ACME via `autocert`
      (T2.2/T2.3), commit `b857f43`. `Config.Reachability()`/`TLSMode()`;
      `NewACMEManager` (HostPolicy pinned); `HTTPSRedirect`; two-listener
      graceful shutdown. **Follow-up:** the Pebble cert-lifecycle
      integration test self-skips — issuance proven locally, the download
      assertion hits an `x/crypto` acme ↔ Pebble finalize incompatibility;
      + wire the CI Pebble service once green. Middleware-chain amendment
      (`backend-http-transport.md` FR-1) + `backend-configuration.md`
      FR-8 note still `DRAFT` pending Checkpoint 2.
- [ ] HKDF subkey wiring (`enrolment-grant-v1` + the two pairing-code
      subkeys) → Tier 4, with the login/pair handlers that consume them.
- [ ] Tier 3 — persistence · Tier 4 — HTTP API · Tier 5 — web UI
- [ ] Gate 2 — audit 0013 (stop and ask) · Close

---

## Tier 0 — config & bind (finish the uncommitted work + commit)

### T0.1 — six phase-13 config keys
**Description:** Verify and commit the uncommitted `ACME_ENABLED/DOMAIN/EMAIL/CACHE_DIR`, `CORS_ALLOWED_ORIGINS`, `DEVICE_PAIRING_SECRET` keys + `parseBool` / `parseOriginList` in `internal/config/config.go` (`backend-configuration.md` FR-4 amendment, `backend-network-transport.md` FR-1).
**Acceptance criteria:**
- [ ] Each key has a precedence test (default → file → env, each overriding) — `config_phase13_test.go`
- [ ] `CORS_ALLOWED_ORIGINS` rejects a malformed entry (path present, no scheme, has query/fragment/userinfo) with an error naming the entry
- [ ] `DEVICE_PAIRING_SECRET` is `RedactedString` and never appears in a logged `Config` or an error string (extend the FR-7 leak-then-clean test)
- [ ] `ACME_CACHE_DIR` empty ⇒ left empty here (resolved at startup where the data dir is known), documented
**Verification:**
- [ ] `go test ./internal/config/...`
- [ ] `go vet ./...`
**Dependencies:** None
**Files:** `internal/config/config.go`, `internal/config/config_phase13_test.go`, `internal/config/structural_test.go`
**Scope:** S

### T0.2 — `BIND_ADDRESS` classifier + static-cert matrix
**Description:** Finish `validateBindAddress` / `classifyBindHost` (`backend-network-transport.md` FR-2, ADR 0028 §1). Classification is stated, not left to the implementer: IP literal by range; `localhost` private; any other string public with **no DNS lookup**.
**Acceptance criteria:**
- [ ] Classifier test rows: `127.0.0.1`, `::1`, `localhost`, `10.x`/`192.168.x`/`172.16.x`, an IPv6 ULA → private; `library.example.com`, `foo.local`, a public IP, **`0.0.0.0`, `[::]`** → public (D-B)
- [ ] Private class: no cert ⇒ accepted; opt-in `TLS_CERT_FILE`+`TLS_KEY_FILE` both present ⇒ loaded/parsed/key-matched/in-window (no SAN check), stored on `Config.tlsCert`; present-but-broken ⇒ startup error (no plaintext fall-through); `ACME_ENABLED=true` ⇒ startup error naming the conflict
- [ ] Public class: valid static pair ⇒ accepted + `tlsCert` set; for a **named** host the leaf SANs must include it (`VerifyHostname`); expired / key-mismatch / wrong-SAN / no-cert-no-ACME / `ACME_ENABLED` (Tier 2 not wired) ⇒ startup error naming the specific condition, before any other subsystem initialises
- [ ] `x509`/`tls` reads go through the injected `readFile`, never a direct filesystem call
**Verification:**
- [ ] `go test ./internal/config/...` — the FR-2 acceptance/rejection matrix
- [ ] the previously-blanket-rejected "public + valid cert" case now **passes** (was RED)
**Dependencies:** T0.1
**Files:** `internal/config/bindaddress.go`, `internal/config/bindaddress_phase13_test.go`, `internal/config/tlsfixture_test.go`
**Scope:** M

### T0.3 — `run.go` in-process TLS listener wrap
**Description:** Verify `cmd/server/run.go` wraps the listener in `tls.NewListener` whenever `cfg.TLSCertificate() != nil`, so an accepted public (or private opt-in) bind serves TLS in-process, never plaintext. Minimal `tls.Config` here — cipher/version policy, ALPN, HSTS, `:80` are Tier 2.
**Acceptance criteria:**
- [ ] A `run` integration test: accepted public-cert config ⇒ the served listener speaks TLS; a plaintext request to the port fails
- [ ] loopback/private no-cert config ⇒ plain `http` listener, unchanged from phase 03
- [ ] startup logs the TLS mode once at `info`
**Verification:**
- [ ] `go test ./cmd/server/...`
- [ ] `go test -race -tags=integration ./cmd/server/...`
**Dependencies:** T0.2
**Files:** `cmd/server/run.go`, `cmd/server/run_tls_test.go`
**Scope:** S

### Checkpoint 0
- [ ] `go build ./...`, `go vet ./...` clean
- [ ] `go test ./internal/config/... ./cmd/server/...` green; `-tags=integration` green
- [ ] `scripts/check-import-boundaries.sh`, `scripts/check-parameterized-queries.sh` clean
- [ ] Tier 0 **committed** on a `feat/phase13-*` branch (not `main`); branch strategy for the rest of the phase confirmed with the maintainer
- [ ] `backend-configuration.md` FR-8 interim-note removal is left for **C.2** (spec amendment, maintainer re-confirm) — not done here

---

## Tier 1 — pure pairing domain (`domain-device-pairing.md`)

### T1.1 — `PairingCode` value object
**Description:** FR-1/FR-2. Wrap a string in a fixed Crockford-base32 shape; constant-time comparison is the only comparison path.
**Acceptance criteria:**
- [ ] Constructor accepts exactly 8 Crockford chars (case-folded, display hyphen optional → `XXXX-XXXX`); rejects 7 / 9 chars, a non-Crockford char (`I`/`L`/`O`/`U`), `=` padding, whitespace — one test per case
- [ ] Only equality path is `Equal(other) bool` via `crypto/subtle.ConstantTimeCompare` over normalized bytes; a structural test asserts the call; `grep` finds no `==` on the wrapped type, no `strings.EqualFold` of a code
- [ ] entropy is **not** asserted here (it is `backend-network-transport.md` FR-9's — T2.8)
**Verification:** `go test ./internal/domain/ -run PairingCode`
**Dependencies:** None (parallel-safe with Tier 2 up to T2.8)
**Files:** `internal/domain/device_pairing.go`, `internal/domain/device_pairing_test.go`
**Scope:** S

### T1.2 — `PairingSession` aggregate
**Description:** FR-3–FR-7. Owns the pairing lifecycle; rejects every illegal transition as a typed error; time via an injected clock.
**Acceptance criteria:**
- [ ] `PairingState ∈ {pending, verified, consumed, expired}`; legal moves only `pending→verified` (`Verify`), `verified→consumed` (`Consume`), `pending|verified→expired` (`ExpireAt`); every other move returns a typed error naming state + attempt and does not mutate — one test per illegal move
- [ ] construction: `ttl > 0` and `ttl ≤ 5m` enforced (a larger `ttl` errors, not clamps); `ExpiresAt == CreatedAt.Add(ttl)`
- [ ] `Verify`: expiry checked first (moves to `expired`, returns expiry error); wrong state → error; wrong code → generic mismatch error, **state stays `pending`** (no burn); match → sets `DeviceID`, state `verified`; zero `DeviceID` rejected
- [ ] `Verify` and `Consume` each independently reject a past-`ExpiresAt` session without a prior `ExpireAt` call
- [ ] `verified→consumed` is a single method call (usable inside one tx — T3.4)
**Verification:** `go test ./internal/domain/ -run PairingSession` — full legal path + illegal-transition table (must fail before implementation)
**Dependencies:** T1.1
**Files:** `internal/domain/device_pairing.go`, `internal/domain/device_pairing_test.go`
**Scope:** M

### T1.3 — `PairedDevice` entity
**Description:** FR-8/FR-9. Records a completed enrolment.
**Acceptance criteria:**
- [ ] fields: `DeviceID`, `Owner UserID`, `Label` (non-empty ≤ 100 runes trimmed), `DeviceClass ∈ {phone,tablet,desktop,tv,unknown}`, `EnrolledVia ∈ {pairing_code, password_login}` (phase 13 only ever writes `pairing_code`), `CreatedAt`, `LastSeenAt`, `RevokedAt *time`
- [ ] `Revoke(now)` sets `RevokedAt`; a second `Revoke` → typed error, no mutation
- [ ] `Touch(now)` moves `LastSeenAt` forward only (never backward under skew); errors on a revoked device. No phase-13 caller — covered by tests, defined for phase 14
- [ ] empty / over-long `Label` → typed validation error
**Verification:** `go test ./internal/domain/ -run PairedDevice`
**Dependencies:** T1.1
**Files:** `internal/domain/device_pairing.go`, `internal/domain/device_pairing_test.go`
**Scope:** S

### T1.4 — rehydration + boundary audit
**Description:** FR-3 Reliability NFR, FR-10. Rehydration constructors re-validate every invariant; no fingerprinting fields.
**Acceptance criteria:**
- [ ] `RehydratePairingSession(...)` and a `PairedDevice` rehydration path take every persisted field, return `(*T, error)`, and fail loudly on a malformed row (`ExpiresAt-CreatedAt > 5m`, unknown state string, unknown `DeviceClass`/`EnrolledVia`)
- [ ] repository never constructs either type by struct literal (matches `domain/rehydrate.go` pattern)
- [ ] import/field audit test: the package references no `net.IP`, `*http.Request`, raw `User-Agent`, screen resolution, MAC, or hardware id
**Verification:** `go test ./internal/domain/...` (whole package green)
**Dependencies:** T1.2, T1.3
**Files:** `internal/domain/device_pairing.go`, `internal/domain/rehydrate.go`, `internal/domain/device_pairing_test.go`
**Scope:** S

### Checkpoint 1
- [ ] `go test ./internal/domain/...` green
- [ ] illegal-transition table + constant-time-`Equal` structural test + import-audit test pass
- [ ] `grep` proves no `==` / `EqualFold` on the code type
- [ ] `scripts/check-import-boundaries.sh` clean

---

## Tier 2 — transport services (`backend-network-transport.md`)

### T2.1 — `tls.Config` builder
**Description:** FR-4 / ADR 0028 §3.
**Acceptance criteria:**
- [ ] `MinVersion = tls.VersionTLS12`; `CipherSuites` = exactly the six AEAD+ECDHE suites listed in ADR 0028 §3, in that order; `NextProtos = ["h2","http/1.1"]`; `PreferServerCipherSuites` unset
- [ ] table test: version floor, exact cipher list, ALPN order
**Verification:** `go test ./internal/transport/...  -run TLSConfig`
**Dependencies:** Checkpoint 0
**Files:** `internal/transport/tlsconfig.go` (or `internal/transport/http/`), `_test.go`
**Scope:** S

### T2.2 — `transport.Listeners(cfg, handler)`
**Description:** FR-3 / FR-11. One constructor returns the set of `*http.Server` to run, decided by the bind classification, plus the `:80` redirect listener on every public bind; graceful shutdown drains all of them.
**Acceptance criteria:**
- [ ] private + no cert → one plain `http.Server` on `BIND_ADDRESS`; startup logs once at `warn` that the bind is non-loopback + unencrypted (calm, no exclamation — §11)
- [ ] private + opt-in cert → `ServeTLS` with T2.1's config + the files
- [ ] public + static cert → `ServeTLS` + the `:80` 308-redirect listener (redirect only)
- [ ] public + ACME → `ServeTLS` with `GetCertificate = manager.GetCertificate` + `:80` serving `manager.HTTPHandler(redirect)`
- [ ] `:80` in ACME mode failing to bind → startup failure; otherwise a `warn`
- [ ] graceful shutdown calls `Shutdown(ctx)` on every listener concurrently, bounded by `SHUTDOWN_GRACE_PERIOD`, returns the first error, `Close()`s + names an undrained socket (FR-11, extends `backend-service-lifecycle.md` FR-5)
**Verification:**
- [ ] `go test ./internal/transport/... -run Listeners`
- [ ] `-tags=integration`: static-cert `ServeTLS` real TLS 1.3 + forced 1.2 handshake, ALPN asserted, plaintext-to-port fails; `:80` → 308; graceful-shutdown-two-listeners (in-flight drains, stuck is `Close()`d + logged)
**Dependencies:** T2.1
**Files:** `internal/transport/listeners.go`, `_test.go`, `_integration_test.go`; `cmd/server/run.go` (call site)
**Scope:** M

### T2.3 — ACME via `autocert` + Pebble
**Description:** FR-3 / ADR 0028 §2. No new Go module (`golang.org/x/crypto/acme/autocert`).
**Acceptance criteria:**
- [ ] `autocert.Manager`: `HostPolicy = HostWhitelist(ACMEDomain)`, `Cache = DirCache(ACMECacheDir)` created `0700`, `Email = ACMEEmail` if set, `Prompt = AcceptTOS`
- [ ] startup log + the eventual `/network/status` ACME copy name the CA directory URL and state that enabling ACME accepted the CA TOS on the operator's behalf (§11/§12)
- [ ] a client presenting a non-`ACMEDomain` SNI gets no cert and no issuance attempt
- [ ] runtime issuance failure → handshake error for that client + one `info` log; server keeps running; cached cert still served
**Verification:**
- [ ] `-tags=integration` Pebble: certificate issued on first handshake, then a forced near-expiry renewal asserted end to end (not a sleep). Pebble runs as a service container, not a `go.mod` dep
**Dependencies:** T2.2
**Files:** `internal/transport/acme.go`, `_integration_test.go`; CI workflow (Pebble service)
**Scope:** M

### T2.4 — security-headers + HSTS middlewares
**Description:** FR-5a (every bind) + FR-5 (TLS binds only) / ADR 0028 §9.
**Acceptance criteria:**
- [ ] security-headers middleware sets, on **every** response on every bind: app-origin `Content-Security-Policy` (`default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'` + `script-src`/`style-src`/`img-src`/`connect-src`/`font-src` tuned to the built SPA, **no `unsafe-inline` on `script-src`**), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy` denying unused features
- [ ] present on the SPA HTML, the SPA fallback route, every `/api/v1/*` success + error, the `429` and `403` bodies; a test documents the one uncovered path (limits-layer rejection → JSON error, not framed HTML)
- [ ] HSTS (`max-age=31536000`, no `preload`, no `includeSubDomains`) present on a TLS bind, **absent** on a plaintext bind — decided once at startup from the listener choice
- [ ] the Vite build emits no inline script (or uses hashes/nonces) — `npm run build` check
**Verification:** `go test ./internal/transport/http/... -run Headers|HSTS`; frontend build check
**Dependencies:** Checkpoint 0 (independent of T2.2/2.3; sequence anywhere in Tier 2)
**Files:** `internal/transport/http/security_headers.go`, `_test.go`; `web/vite.config.ts` if needed
**Scope:** M

### T2.5 — CORS middleware
**Description:** FR-6 / ADR 0028 §4.
**Acceptance criteria:**
- [ ] `CORSAllowedOrigins` empty → never any `Access-Control-Allow-*`; `OPTIONS` preflight → `204` no CORS headers
- [ ] entries present → exact byte-for-byte `Origin` match echoes `Access-Control-Allow-Origin` + `Vary: Origin` + `Allow-Methods` (route's) + `Allow-Headers: Authorization, Content-Type, X-Library-Id` + `Max-Age: 600`; non-match → no headers
- [ ] `Access-Control-Allow-Credentials` is **never** set
- [ ] near-miss tests: `http` vs `https`, trailing slash, subdomain, port — all emit nothing
**Verification:** `go test ./internal/transport/http/... -run CORS` (empty-allowlist test must fail before impl)
**Dependencies:** Checkpoint 0
**Files:** `internal/transport/http/cors.go`, `_test.go`
**Scope:** S

### T2.6 — `Origin` validation middleware
**Description:** FR-7 / ADR 0028 §5. Route-group wrapper on the unauthenticated pairing routes only.
**Acceptance criteria:**
- [ ] allowed set computed at startup: every non-loopback interface address at the bound port, in the scheme(s) actually served (`http://` and/or `https://`), + the configured mDNS `hostName` at that port + every `CORS_ALLOWED_ORIGINS` entry
- [ ] multi-address test: `pair/verify` from the LAN-IP origin **and** from the `.local` origin both pass; an unrelated origin → `403` `Forbidden`-category JSON
- [ ] no `Origin` header → allowed through
- [ ] `Referer` consulted only when `Origin` absent AND method has a body; present-non-matching `Referer` host → rejected
**Verification:** `go test ./internal/transport/http/... -run Origin`
**Dependencies:** T2.2 (needs the served scheme(s) + bound interfaces)
**Files:** `internal/transport/http/origin.go`, `_test.go`
**Scope:** M

### T2.7 — global unauthenticated rate-limit middleware
**Description:** FR-8. Reuse `auth.IPRateLimiter` (or extract it to `internal/transport/http`).
**Acceptance criteria:**
- [ ] applies to `/healthz`, `/readyz`, embedded static assets, `POST /api/v1/network/pair/verify`, and (though authenticated) `POST /api/v1/network/pair/initiate` gets its own stricter bucket
- [ ] limits (reasoned placeholders): health/static 60/min burst 30; `pair/verify` 10/min burst 5; `pair/initiate` 5/min burst 3; exceed → `429` `RateLimited`-category JSON
- [ ] client IP from `RemoteAddr` only; `X-Forwarded-For` ignored (audit test)
- [ ] does not double-limit phase-12's auth-endpoint limiter
**Verification:** `go test ./internal/transport/http/... -run RateLimit` (burst→429 on `/healthz` must fail before impl); `-race`
**Dependencies:** Checkpoint 0
**Files:** `internal/transport/http/ratelimit_global.go`, `_test.go`; maybe `internal/auth/ratelimit.go` (extract)
**Scope:** M

### T2.8 — HKDF subkeys + `GeneratePairingCode`
**Description:** FR-9a / FR-9.
**Acceptance criteria:**
- [ ] `cmd/server/run.go` derives three subkeys from the master key alongside the existing `jwt-signing-secret-v1` / `mfa-totp-master-v1` / `source-cursor-hmac-v1`: `pairing-code-enc-v1` (AES-256-GCM), `pairing-code-index-v1` (HMAC), `enrolment-grant-v1` (HS256) — distinct purposes, distinct keys
- [ ] `GeneratePairingCode() (domain.PairingCode, error)`: 5 bytes from `crypto/rand` → Crockford base32 (8 chars) → `XXXX-XXXX` → through `domain`'s validated constructor
- [ ] a `crypto/rand` read error is returned, never swallowed, never retried with a weaker source; **no `math/rand` import** in this phase's transport code (audit test)
- [ ] generator's own acceptance test asserts ≥ 40 bits (5 `crypto/rand` bytes)
**Verification:** `go test ./internal/transport/... ./cmd/server/... -run PairingCode|Subkey`
**Dependencies:** T1.1
**Files:** `internal/transport/pairingcode.go`, `_test.go`; `cmd/server/run.go`
**Scope:** S

### T2.9 — enrolment grant in `internal/auth`
**Description:** FR-10.
**Acceptance criteria:**
- [ ] `SignEnrolmentGrant(sid, now) (string, error)` + `VerifyEnrolmentGrant(token, now) (EnrolmentClaims, error)`, HS256 under `enrolment-grant-v1` (never the access-token key)
- [ ] claims `{typ:"enrol", sid, jti, exp, iat, iss:"alexandryn"}` — no `UserID`, no role; TTL ≤ 10 min
- [ ] verify rejects expired / tampered / wrong-subkey / wrong-`typ`; single-use is enforced by the login handler recording `jti` (T4.7), not here
- [ ] a grant presented on the access path is rejected by `VerifyAccessToken`'s type check (regression test — already true via `TokenTypeAccess`; add `enrol` to the reject list)
**Verification:** `go test ./internal/auth/... -run EnrolmentGrant`
**Dependencies:** T2.8
**Files:** `internal/auth/enrolment_grant.go`, `_test.go`, `internal/auth/jwt.go` (type constants)
**Scope:** S

### T2.10 — middleware chain assembly
**Description:** Amends `backend-http-transport.md` FR-1 + `architecture-backend.md` FR-6. FR-12 + FR-13 verification.
**Acceptance criteria:**
- [ ] ordered chain, outermost-in: `recovery → limits → logging → security-headers → HSTS(TLS binds) → CORS → global rate-limit(public paths) → auth(VerifyAccessToken + X-Library-Id) → Origin-validation(pairing route group) → routing` — a full ordering test
- [ ] CORS + rate-limit sit **before** auth; `Origin` validation is a route-group wrapper, not global
- [ ] FR-12: a test proves no config key (alone or combined) yields a reachable bind with `AuthMiddleware` bypassed; `IsPublicPath` gains only `POST /api/v1/network/pair/verify`
- [ ] FR-13 re-verify: `AuthMiddleware` rejects `mfa_ticket` / `enrol` / garbage with `401` even when signature-valid; `X-Library-Id` not in `claims.Libraries` → `403` (both already landed — add per-token-type + per-header middleware tests if missing)
- [ ] smoke-test all six `domain.Error` categories through the full chain end to end
**Verification:** `go test -race ./internal/transport/http/...`
**Dependencies:** T2.4, T2.5, T2.6, T2.7, T2.9
**Files:** `internal/transport/http/middleware.go`, `server.go`, `auth_ref.go`, `_test.go`; `cmd/server/run.go`
**Scope:** M

### Checkpoint 2
- [ ] `go test -race ./internal/config/... ./internal/auth/... ./internal/domain/... ./internal/transport/...` green
- [ ] `-tags=integration`: static-cert `ServeTLS`, Pebble issuance+renewal, `:80` redirect, two-listener graceful shutdown — all green
- [ ] `math/rand` audit test + header-keying audit test pass
- [ ] **maintainer re-confirms** the `DRAFT` middleware-chain amendments: `backend-http-transport.md` FR-1 + `architecture-backend.md` FR-6 (per `project_phase13_status` — required before this checkpoint closes)
- [ ] commit

---

## Tier 3 — persistence (`backend-network-api.md` FR-7/FR-8)

### T3.1 — migration `00010_phase13_network.sql`
**Acceptance criteria:**
- [ ] `pairing_sessions`: `id` pk, `initiated_by` fk `users` `ON DELETE CASCADE`, `code_ciphertext` bytea, `code_index` bytea `UNIQUE`, `state` text `CHECK` (4 values), `created_at`/`expires_at` `timestamptz`, `device_id` nullable, `initiator_ip` inet nullable; index `(state, expires_at)`. **No plaintext/hashed code column.**
- [ ] `paired_devices`: `id` pk, `owner_id` nullable fk `users` `ON DELETE CASCADE`, `label`, `device_class`/`enrolled_via` `CHECK` enums, `created_at`/`last_seen_at`, `revoked_at` nullable, `pairing_session_id` nullable fk `pairing_sessions` **`ON DELETE SET NULL`**
- [ ] `enrolment_grant_jtis`: `jti` pk, `spent_at`
- [ ] `network_settings`: single row — `host_name`, `remember_device_days`, `updated_at`
- [ ] applies to an empty DB and rolls back cleanly, incl. the `ON DELETE SET NULL`
**Verification:** `go test -tags=integration ./internal/persistence/postgres/ -run Migrat`
**Dependencies:** Checkpoint 1
**Files:** `internal/persistence/postgres/migrations/00010_phase13_network.sql`, `migrate_integration_test.go`
**Scope:** S

### T3.2 — `PairingSessionRepository`
**Acceptance criteria:**
- [ ] interface in `internal/domain` (matches `domain/repository.go`), impl in `internal/persistence/postgres`
- [ ] on write: encrypt the normalized code AES-256-GCM under `pairing-code-enc-v1` (ciphertext + nonce), compute the blind HMAC index under `pairing-code-index-v1`, store both
- [ ] on read: decrypt, hand `domain` a reconstructed **validated** `PairingCode`; encryption/HMAC live in the repository, `domain` stays crypto-free
- [ ] round-trip test: encrypt→store→decrypt yields the original code; the blind index matches a fresh HMAC of the same code
- [ ] all queries parameterised
**Verification:** `go test -race -tags=integration ./internal/persistence/postgres/ -run PairingSession`
**Dependencies:** T3.1, T2.8
**Files:** `internal/domain/repository.go`, `internal/persistence/postgres/pairing_session_repository.go`, `_integration_test.go`
**Scope:** M

### T3.3 — `PairedDeviceRepository` + `NetworkSettingsRepository` + grant-jti store
**Acceptance criteria:**
- [ ] CRUD for `paired_devices` (incl. `owner_id` assignment, `Revoke`), the one-row `network_settings` (read + upsert), `enrolment_grant_jtis` (insert + exists-check)
- [ ] no column stores a raw code, token, or `User-Agent` string
- [ ] all queries parameterised
**Verification:** `go test -race -tags=integration ./internal/persistence/postgres/ -run PairedDevice|NetworkSettings|GrantJti`
**Dependencies:** T3.1
**Files:** `internal/domain/repository.go`, `internal/persistence/postgres/{paired_device,network_settings,enrolment_grant}_repository.go`, `_integration_test.go`
**Scope:** M

### T3.4 — row-locked atomic verify
**Description:** FR-2 / ADR 0021. Follows `source_removal_atomicity_integration_test.go`.
**Acceptance criteria:**
- [ ] one transaction: `SELECT … FROM pairing_sessions WHERE code_index=$1 AND state='pending' AND expires_at > now() FOR UPDATE` → decrypt → rehydrate → `session.Verify(now, code, deviceID)` → `session.Consume(now)` + persist + insert `paired_devices` (owner unset)
- [ ] injected failure between `Consume` and the insert → full rollback (code still `pending`, no device row, no grant `jti`)
- [ ] concurrent double-submit (two goroutines, same code) → exactly one binds a device, the other gets no match; proven by an integration test (the `FOR UPDATE` lock)
**Verification:** `go test -race -tags=integration ./internal/persistence/postgres/ -run VerifyAtomic|DoubleSubmit`
**Dependencies:** T3.2, T3.3, T1.2
**Files:** `internal/persistence/postgres/pairing_verify.go` (or in the repo), `_integration_test.go`
**Scope:** M

### T3.5 — FR-8 background sweep
**Description:** D-G — a 5-minute lifecycle ticker (not the job queue).
**Acceptance criteria:**
- [ ] marks `pending` sessions past `expires_at` as `expired`; deletes `expired`/`consumed` sessions > 24h; deletes `paired_devices` still `owner_id IS NULL` > 1h; deletes `enrolment_grant_jtis` older than the max grant TTL
- [ ] not invoked on any request path
- [ ] seeded-fixture integration test asserts each transition/deletion
**Verification:** `go test -race -tags=integration ./... -run Sweep`
**Dependencies:** T3.3
**Files:** `internal/persistence/postgres/network_sweep.go` (or `cmd/server`), `_integration_test.go`; `cmd/server/run.go` (start the ticker)
**Scope:** S

### Checkpoint 3
- [ ] `go test -race -tags=integration ./internal/persistence/...` green
- [ ] migration up/down clean; atomicity + concurrent-double-submit + sweep tests pass
- [ ] `scripts/check-parameterized-queries.sh` clean
- [ ] commit

---

## Tier 4 — HTTP API + contract (`backend-network-api.md`)

### T4.1 — `POST /api/v1/network/pair/initiate`
**Acceptance criteria:**
- [ ] `RequireRole(admin)`; rate-limited (T2.7 `pair/initiate` bucket)
- [ ] `DEVICE_PAIRING_SECRET` configured → body must carry a matching `{"secret":...}` (constant-time) or `403` generic
- [ ] success → `201 {pairingId, code, payload, address, expiresAt}`; `payload` = `<scheme>://<address>/connect?c=<code>`; session persisted (code encrypted)
- [ ] the secret is never logged / never in an error string
**Verification:** `go test ./internal/transport/http/... -run PairInitiate`
**Dependencies:** Checkpoint 3, T2.10
**Files:** `internal/transport/http/network.go`, `network_test.go`
**Scope:** M

### T4.2 — `POST /api/v1/network/pair/verify`
**Acceptance criteria:**
- [ ] unauthenticated, `Origin`-checked (T2.6), rate-limited (T2.7)
- [ ] malformed code shape → `400` `InvalidInput`
- [ ] wrong / expired / never-existed → **byte-identical generic `404`** ("pairing code not recognised")
- [ ] success → `200 {enrolmentGrant, address, hostName}`; the enrolment grant carries the `PairingSessionID` + a fresh `jti`
- [ ] 4 KiB body cap; non-JSON `Content-Type` → `415`
**Verification:** `go test ./internal/transport/http/... -run PairVerify` (generic-404 test must fail before impl)
**Dependencies:** T4.1, T3.4
**Files:** `internal/transport/http/network.go`, `network_test.go`
**Scope:** M

### T4.3 — `GET /api/v1/network/pair/{id}/qr`
**Acceptance criteria:**
- [ ] admin-only, **only for a session the caller initiated** (`initiated_by = caller`) — another admin's session → `404` (no cross-admin enumeration)
- [ ] returns `{payload, address, code, expiresAt, state}`; code from decrypting the stored ciphertext
- [ ] `consumed`/`expired` session → terminal `state` with `payload` and `code` as empty strings
**Verification:** `go test ./internal/transport/http/... -run PairQR`
**Dependencies:** T4.1
**Files:** `internal/transport/http/network.go`, `network_test.go`
**Scope:** S

### T4.4 — `GET /api/v1/network/status`
**Acceptance criteria:**
- [ ] authenticated; **role-scoped**: `reader`/non-admin → `{reachability, tlsMode, authRequired:true, address}` (address = the single URL the caller's own request arrived on, from bind config + matched server-side origin, never the client `Host` header); `admin` → + `{addresses:[{scope,url}], hostName, acmeDomain}` (acmeDomain only when `tlsMode=="acme"`)
- [ ] `tlsMode ∈ {"none","static","acme"}`
- [ ] neither response contains: any filesystem path, the cert/key, the ACME cache dir, `ACME_EMAIL`, `DATABASE_URL` or a substring, `DEVICE_PAIRING_SECRET`, any FR-9a subkey, a raw driver error, the home dir — table test with a `Config` carrying recognisable fake values for each
- [ ] `authRequired` is always `true`; no code path writes it (grep + the T4.5 rejection test)
- [ ] no DB call (answers even when Postgres is down)
**Verification:** `go test ./internal/transport/http/... -run NetworkStatus` (never-include-list test must fail before impl)
**Dependencies:** T2.2 (needs the resolved bind + interface list)
**Files:** `internal/transport/http/network.go`, `network_test.go`
**Scope:** M

### T4.5 — `PATCH /api/v1/network/settings`
**Acceptance criteria:**
- [ ] admin-only; accept-list is **exactly** `hostName` (regex `^(?=.{1,63}$)[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.local)?$`) + `rememberDeviceDays` (integer `1..90`)
- [ ] any other key — `bindAddress`, `tlsCertFile`, `acmeDomain`, `authRequired`, unknown → `400` `InvalidInput` naming the key; restart-only keys say the value is set in the config file and takes effect after a restart
- [ ] returns `200` with the full effective settings; persisted to `network_settings`
- [ ] `rememberDeviceDays` is consumed by refresh-token issuance (T4.9 / `backend-authentication.md` FR-3/FR-4 amendment)
**Verification:** `go test ./internal/transport/http/... -run NetworkSettings`
**Dependencies:** T3.3
**Files:** `internal/transport/http/network.go`, `network_test.go`
**Scope:** M

### T4.6 — `DELETE /api/v1/network/pair/{id}`
**Acceptance criteria:**
- [ ] admin-only, initiator-only (`404` for a foreign `pairingId`)
- [ ] non-terminal session → expire it; `consumed` → `PairedDevice.Revoke(now)` on the device it produced; `204`
- [ ] response docs + the audit note: this does **not** yet invalidate that device's phase-12 refresh tokens — phase 14 (flagged, not skipped)
**Verification:** `go test ./internal/transport/http/... -run PairDelete`
**Dependencies:** T4.1, T3.3
**Files:** `internal/transport/http/network.go`, `network_test.go`
**Scope:** S

### T4.7 — login `enrolmentGrant` parameter
**Description:** FR-9 / ADR 0028 §6. Amends the phase-12 login contract.
**Acceptance criteria:**
- [ ] `POST /api/v1/auth/login` accepts optional `{"enrolmentGrant"?: string}`
- [ ] when present: verify under `enrolment-grant-v1`; check `jti` unspent; resolve `PairedDevice` via `sid → pairing_sessions.id → paired_devices.pairing_session_id`; **set `owner_id` to the now-authenticated user** (not `InitiatedBy`); record `jti` in `enrolment_grant_jtis`
- [ ] invalid / expired / replayed / wrong-subkey / wrong-`typ` grant → **ignored**: login still succeeds, no device association, logged at `info` with the correlation ID
- [ ] replay: same `jti` twice → the second login makes no device link
**Verification:** `go test -race -tags=integration ./internal/transport/http/... ./internal/persistence/... -run LoginGrant`
**Dependencies:** T2.9, T3.3
**Files:** `internal/transport/http/auth_handlers.go`, `auth_handlers_test.go`
**Scope:** M

### T4.8 — OpenAPI + contract tests
**Acceptance criteria:**
- [ ] new paths in `api/openapi.yaml` with an `example:` block per response (success + `400`/`403`/`404`/`415`/`429`)
- [ ] `npm run mocks:gen-fixtures` succeeds
- [ ] contract tests in `internal/testutil/contracttest/` — each response validates against its schema; `429`/`403`/`404`/`415` bodies match `architecture-contracts.md` FR-5
- [ ] `POST /api/v1/auth/login` schema amended for optional `enrolmentGrant`
**Verification:** `go test ./internal/testutil/contracttest/...`; `npm run mocks:gen-fixtures`
**Dependencies:** T4.1–T4.7
**Files:** `api/openapi.yaml`, `internal/testutil/contracttest/network_test.go`
**Scope:** M

### T4.9 — close-gate re-verification
**Description:** `roadmap/13-network-access/README.md` close gate + `backend-network-api.md` FR-13. Most of the code landed on the branch (PR #78 prelude) — this task is the **tests** that certify it.
**Acceptance criteria:**
- [ ] per-endpoint reading-API IDOR tests: user A cannot read / write / delete user B's `progress` / `bookmark` / `highlight` by ID
- [ ] `GET /api/v1/reading/export` isolation integration test: returns only the caller's rows
- [ ] `AuthMiddleware`: `X-Library-Id` not in `claims.Libraries` → `403`; a `reader` cannot set an arbitrary library header
- [ ] `AuthMiddleware`: `mfa_ticket` / `enrol` presented as a bearer token → `401`
- [ ] `rememberDeviceDays` change → the next issued refresh token's `expires_at` reflects it (integration test)
- [ ] `scripts/check-user-scoped-reading.sh` clean
**Verification:** `go test -race -tags=integration ./internal/transport/http/... ./internal/persistence/...`
**Dependencies:** T4.5, T4.7
**Files:** `internal/transport/http/{reading_test.go,auth_middleware_test.go}`, `internal/persistence/postgres/reading_export_repository_integration_test.go`
**Scope:** M

### Checkpoint 4
- [ ] `go test -race ./... && go test -race -tags=integration ./...` green
- [ ] contract suite green; `npm run mocks:gen-fixtures` clean
- [ ] every close-gate box in `roadmap/13-network-access/README.md` Exit criteria has a passing test
- [ ] commit; open the backend PR for review (two independent reviewers, per `feedback_make_improve_review_fix_cycle`)

---

## Tier 5 — web UI (`frontend-network-and-pairing.md`)

### T5.1 — `qrcode` dependency
**Acceptance criteria:**
- [ ] §9 record written: what it does, why not stdlib / hand-rolled, abandonment risk; a maintained-library check (recent releases, no unpatched advisories)
- [ ] `check:bundle-size` headroom result recorded
- [ ] **maintainer sign-off (D-E)** before merge; on a fail, vendor `qrcode-generator` (~4 KB) instead
- [ ] only the matrix-generation entry point imported (not canvas/terminal renderers)
**Verification:** `npm run check:bundle-size`; PR description carries the §9 record
**Dependencies:** Checkpoint 4
**Files:** `web/package.json`, `web/package-lock.json`, PR description
**Scope:** S

### T5.2 — `data/network.ts` hooks + MSW
**Acceptance criteria:**
- [ ] TanStack Query hooks (no inline `fetch`, `frontend-shell-and-routing.md` FR-2) for the six endpoints; hook types are the union of the role-scoped `/network/status` shapes
- [ ] `login()` in `web/src/data/auth.ts` gains an optional `enrolmentGrant` passed to `POST /api/v1/auth/login`
- [ ] MSW handlers seeded from the generated fixtures
**Verification:** `npm test -- data/network`
**Dependencies:** T5.1, T4.8
**Files:** `web/src/data/network.ts`, `web/src/data/auth.ts`, `web/src/mocks/handlers.ts`
**Scope:** M

### T5.3 — `NetworkSettings.tsx`
**Acceptance criteria:**
- [ ] renders under `hostOnly('network','Network')`; status card; `tlsMode` copy is honest for all four cases (FR-1 — incl. the "not encrypted unless a reverse proxy…" private-plaintext line)
- [ ] a **static** "Authentication is always on" row — no toggle, no toggle role in the tree
- [ ] read-only reachability row + read-only "Advanced" disclosure (keyboard, `aria-expanded`): bind address, TLS cert **"configured"/"not configured"** (never a path), ACME domain, mDNS `hostName` (editable) + "changes take effect after restarting" line; ACME-TOS line when `tlsMode:acme`
- [ ] editable mDNS name + "remember devices for N days" (1–90) → `PATCH` optimistic update + rollback + error toast
- [ ] no path / cert / secret ever in the DOM — test with a mock status carrying fake recognisable values
**Verification:** `npm test -- NetworkSettings`; `npm run check:token-styling`
**Dependencies:** T5.2
**Files:** `web/src/screens/Settings/NetworkSettings.tsx`, `.test.tsx`
**Scope:** M

### T5.4 — `DevicePairingModal.tsx`
**Acceptance criteria:**
- [ ] Radix `Dialog`; `initiate` on open (with `DEVICE_PAIRING_SECRET` field only when the server says one is required); QR rendered client-side from `payload`; `XXXX-XXXX` code in mono
- [ ] live countdown to `expiresAt`, `aria-live="polite"`, announced at 60/30/10/0s only; at zero → "This code expired" + "Generate a new code"
- [ ] "Revoke" and Escape both `DELETE /network/pair/{id}`; "Done" does not
- [ ] does **not** poll verify state (fire-and-forget)
**Verification:** `npm test -- DevicePairingModal`
**Dependencies:** T5.2
**Files:** `web/src/screens/Network/DevicePairingModal.tsx`, `.test.tsx`
**Scope:** M

### T5.5 — `/connect` route
**Acceptance criteria:**
- [ ] reads `?c=<code>`, prefills, then `history.replaceState` strips `?c=` immediately
- [ ] one "Pairing code" field (`XXXX-XXXX`, auto-uppercase, hyphen auto-insert) + optional "Name this device" + "Continue"
- [ ] submit → `POST /network/pair/verify`; on `200` navigate to phase-12 `/login` with `enrolmentGrant` + `hostName` in **router state, not the URL** (test asserts `location.search` clean)
- [ ] `404` → generic "pairing code not recognised" + retry; `429` → "Too many attempts. Wait a minute and try again."
**Verification:** `npm test -- connect` (grant-not-in-URL test must fail before impl)
**Dependencies:** T5.2
**Files:** `web/src/screens/Network/ConnectScreen.tsx`, `web/src/app/routes.tsx`, `.test.tsx`
**Scope:** M

### T5.6 — `/access` route
**Acceptance criteria:**
- [ ] reader-scoped `/network/status` shape + a small **fixed** capability list keyed off the current role (`admin` vs `reader`) + the library names from the JWT `libraries` claim + "Upload" only when the active library allows reader uploads
- [ ] "Sign out of this browser" → phase-12 `logout()`
- [ ] no `hostModes` cloud card
**Verification:** `npm test -- access`
**Dependencies:** T5.2
**Files:** `web/src/screens/Network/AccessScreen.tsx`, `web/src/app/routes.tsx`, `.test.tsx`
**Scope:** S

### T5.7 — routes, gating, a11y, tokens
**Acceptance criteria:**
- [ ] `routes.tsx`: `network` under `hostOnly`; `connect`/`access` are the real screens; a viewer hitting `/settings/network` gets the gate state with no host-only flash
- [ ] `check:token-styling`, `check:a11y-tabindex`, `check:a11y-hidden-text` clean over the new files; zero raw px/hex
- [ ] `@axe-core/playwright` zero violations on `NetworkSettings`, `DevicePairingModal` (open), `/connect`, `/access`
- [ ] `prefers-reduced-motion` respected (static countdown)
**Verification:** `npm run test:a11y`; the check scripts
**Dependencies:** T5.3–T5.6
**Files:** `web/src/app/routes.tsx`, the four screen files
**Scope:** M

### T5.8 — Playwright E2E
**Acceptance criteria:**
- [ ] host opens the modal → a second browser context opens `/connect?c=<code>` → verify → redirected to `/login` → logs in → lands in the library
- [ ] expired code → generic error; revoke from the host → the code no longer verifies
**Verification:** `npm run test:e2e -- pairing`
**Dependencies:** T5.7
**Files:** `web/e2e/pairing.spec.ts`
**Scope:** M

### Checkpoint 5
- [ ] `npm test`, `npm run build` green
- [ ] `check:token-styling` / `check:a11y-*` clean; axe zero violations
- [ ] Playwright pairing happy-path green
- [ ] commit; frontend PR for review

---

## Gate 2 — security audit `0013`  ──  STOP AND ASK  ──

### G2.1 — four-attacker + STRIDE pass
- [ ] full adversarial pass over the phase (prefer two independent subagents — `feedback_make_improve_review_fix_cycle`): malicious LAN client (unauth + paired-reader), on-path/MITM, phished-onto-hostile-page, attacker-with-the-QR
- [ ] record `.claude/audits/0013-phase13-network-access.md` from `.claude/templates/audit.md`; severities rated honestly (§10 — no inflate, no deflate)
- [ ] establishes (not "re-confirms") the app-origin CSP on every response path incl. the SPA fallback + error responses
- [ ] confirms ADR 0024 sanitisation + phase-11 reader CSP still hold for all LAN-served content

### G2.2 — re-verify the phase-12 authz prelude
- [ ] AUDIT-0012-C1: trace each reading/reader handler → repository → SQL, read the `user_id` + `library_id` predicate in the query text (not from the repo layer — CLAUDE.md Reflex)
- [ ] AUDIT-0012-C2: `VerifyAccessToken` asserts the type on the wired middleware path
- [ ] P12-4: `X-Library-Id` validated against `claims.Libraries` on the wired path

### G2.3 — gate
- [ ] no open Critical or High
- [ ] **report to the maintainer — what's done, what's unresolved, what's risky — and wait.** Do not cross this gate on own judgement (constitution Review gates)

---

## Close

### C.1 — exit-criteria walk
- [ ] walk `roadmap/13-network-access/README.md` Exit criteria box by box; each cites real evidence (test name / file / audit finding)
- [ ] the "session cookie attributes — N/A" box: record *why* in the walk (ADR 0025 / ADR 0028 §5), not implemented

### C.2 — phase-12 spec amendment re-confirmations (one at a time, in order)
- [ ] ADR 0025 (refresh-token lifetime operator-configurable `[1,90]`)
- [ ] `backend-authentication.md` (login `enrolmentGrant` param + response; access-token `typ` check)
- [ ] `backend-library-namespaces.md` (`X-Library-Id ∈ claims.Libraries`)
- [ ] `backend-reading-api.md` (per-user + per-library query-layer enforcement + CI guard)
- [ ] `backend-reader-content.md` (library-membership check before serving edition bytes)
- [ ] `backend-configuration.md` FR-4 (six keys) + FR-8 interim note **removed** (`ServeTLS` now exists)
- [ ] `architecture-backend.md` FR-6 / `backend-http-transport.md` FR-1 — already re-confirmed at Checkpoint 2

### C.3 — approval
- [ ] maintainer approval recorded in the phase-13 README header
- [ ] note: phase 12 still needs its own fuller independent authz re-audit before *it* is marked `Closed` (`roadmap/12-authentication/README.md` — separate from this phase)
