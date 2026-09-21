# Spec: Backend — network transport (bind, TLS, ACME, CORS, rate limiting)

| | |
|---|---|
| **Status** | `VERIFIED` — implemented across Tiers 0–4, certified in Gate 2 security audit `0013`, and verified green by automated test suites. |
| **Phase** | `13-network-access` |
| **Author** | Claude (Sonnet 5) |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-05 |
| **Reviewed in** | `0050` (two independent agents) — Needs rework at review time, findings addressed |
| **Design reference** | `N/A` — transport layer, no UI. The configuration surface this spec's keys feed is `sgNetwork`'s "Advanced" disclosure row in `Alexandryn-Electron-Admin.dc.html` — the **one binding, contents-uncaptured** entry for this surface (`.design-reference/ANALYSIS.md` synced 2026-08-13, classified 2026-08-17); `atFirstRun`/`fr4` is Unclassified and is not relied on. Covered by `frontend-network-and-pairing.md`. |

## Context

From phase 05 the host has bound to loopback only. `backend-http-transport.md`
built the middleware chain and `http.Server` with timeouts and graceful
shutdown but calls `ListenAndServe` only — no TLS path exists. Phase 12
added `AuthMiddleware` (Bearer JWT) and an in-memory per-IP rate limiter
(`internal/auth/ratelimit.go`) scoped to auth endpoints. `internal/config`
already classifies `BIND_ADDRESS` per ADR 0017 FR-8 but **currently rejects
every publicly routable bind outright** because nothing calls `ServeTLS`
(the FR-8 "interim note", 2026-08-26).

Phase 13 opens the bind. ADR 0028 fixes how: TLS mode is derived from the
resolved address plus certificate/ACME state (never a flag), ACME uses
`golang.org/x/crypto/acme/autocert` (no new module), CORS is
deny-by-default, there is no CSRF token machinery under Bearer auth, and
there is no way to disable authentication. This spec is the server-side
transport implementation of all of that.

## Problem

Nothing implements: `ServeTLS` wiring and listener selection from the
address classification; static-certificate validation depth for a public
bind; ACME issuance/renewal and its HTTP-01 challenge listener; the
HTTP→HTTPS redirect; a CORS middleware; `Origin` validation for
unauthenticated routes; a rate limiter covering unauthenticated public
surfaces (health, static assets, `pair/initiate`, `pair/verify`); the
`crypto/rand` adapter that generates a `PairingCode`; or the config keys
(`ACME_*`, `CORS_ALLOWED_ORIGINS`, `DEVICE_PAIRING_SECRET`).

## Goals

- One place that turns a validated `*config.Config` into the right
  listener(s): plain HTTP for loopback/private, `ServeTLS` for public,
  plus an ACME/redirect listener when ACME is enabled.
- Fail-closed at bind: a public bind with no valid certificate and no ACME
  configuration does not start, does not degrade to plaintext.
- Modern TLS: 1.2 floor, 1.3 preferred, AEAD+PFS cipher list for 1.2,
  ALPN `h2`/`http/1.1`, HSTS on public binds only.
- CORS that emits nothing permissive by default and exact-matches a
  configured origin otherwise.
- `Origin`/`Referer` validation on the unauthenticated pairing routes,
  rejecting forged cross-origin POSTs.
- A global rate limiter for unauthenticated public endpoints, reusing the
  phase-12 limiter type.
- Graceful shutdown that drains every listener within
  `SHUTDOWN_GRACE_PERIOD`.

## Non-goals

- The pairing HTTP handlers and their persistence — `backend-network-api.md`.
- The pure pairing domain — `domain-device-pairing.md`.
- The network settings/pairing UI — `frontend-network-and-pairing.md`.
- Changing the phase-12 session mechanism. ADR 0025 stands; this spec adds
  no cookie, no CSRF token, no alternative auth path (ADR 0028 §5, §7).
- mDNS/DNS-SD service advertisement implementation. The `sgNetwork` panel
  shows an `alexandryn.local` host name; whether the Go server advertises
  it via mDNS or the Electron host does is deferred to
  `backend-network-api.md` (`/network/status` reports the name) and, if it
  needs a library, its own scope review. Not assumed done here.
- DNS-01 ACME (ADR 0028 Option E, rejected).
- Certificate provisioning UIs, upload flows — there is no cert upload;
  `TLS_CERT_FILE`/`TLS_KEY_FILE` are filesystem paths in config.

## User stories

- As **`backend-service-lifecycle.md`'s startup sequence**, I want a
  single call that returns the configured `http.Server` plus any auxiliary
  listeners already wired, so step "bind the listener" stays one step even
  though there may now be two sockets.
- As **a user running a public remote deployment**, I want the server to
  refuse to start — loudly, naming what is wrong — rather than ever serve
  my library over plaintext on a routable address.
- As **constitution §6**, I want it to be structurally impossible to reach
  a running configuration where a non-loopback bind has authentication
  turned off.
- As **an unauthenticated LAN client hitting `/healthz` in a loop**, I
  want to be rate-limited the same as any other anonymous caller now that
  the surface is broader than loopback.

## Functional requirements

- **FR-1** `config.Config` gains: `ACMEEnabled bool` (default `false`),
  `ACMEDomain string` (default empty), `ACMEEmail string` (default empty),
  `ACMECacheDir string` (default: an `acme/` subdirectory of the per-user
  data directory `architecture-persistence.md` FR-1 already fixes),
  `CORSAllowedOrigins []string` (parsed from a comma-separated value,
  default empty), `DevicePairingSecret config.RedactedString` (default
  empty — redacted type per `backend-configuration.md` FR-7 since it is a
  shared secret). Each is added to `backend-configuration.md` FR-4's table
  by the amendment this phase makes. There is **no** `TLS_ENABLED` key
  and **no** `PORT` key (ADR 0028 §1, plan C-0/C-8).
- **FR-2** `config.Load` MUST classify `BIND_ADDRESS`'s host and enforce
  the rule per class (ADR 0028 §1). The **classification rule is stated,
  not left to the implementer** (the current `isLoopbackOrPrivate` rejects
  every non-`localhost` name outright, and this FR must say what replaces
  that):
  - host is an **IP literal** → `net.IP.IsLoopback()` / `IsPrivate()` /
    IPv6-ULA → **private class**; anything else → **public class**.
  - host is the literal `localhost` → **private class**.
  - host is **any other string** (a DNS name) → **public class**, decided
    **without any DNS resolution** (`architecture-testing.md` FR-6 — the
    config layer performs no network I/O; the fail-closed default for "a
    name we cannot classify locally" is "require in-process TLS"). A test
    asserts `library.example.com:443` and `foo.local:8080` both classify
    public.
  Then:
  - **private class** → legal unconditionally. If `TLS_CERT_FILE` and
    `TLS_KEY_FILE` are **both** present, they MUST load/parse/key-match/be
    in-window (no SAN check — a private bind is often reached by IP) and
    the server serves `ServeTLS`; a present-but-invalid pair on a private
    bind is still a startup failure (a half-configured cert is a mistake,
    not a fall-through to plaintext). ACME on a private bind is a config
    error (`ACME_ENABLED=true` with a private `BIND_ADDRESS` → startup
    failure naming the conflict — HTTP-01 cannot validate a private
    address).
  - **public class** → `ServeTLS` mandatory, accepted when **either**:
    (a) `ACMEEnabled` is `false` and `TLS_CERT_FILE`/`TLS_KEY_FILE`
    load/parse/key-match/in-window, and — when the host is a DNS name —
    the leaf's DNS SANs include that name; **or** (b) `ACMEEnabled` is
    `true`, `ACMEDomain` non-empty, `ACMECacheDir` set, and (if the host
    is a name) it equals `ACMEDomain`. Any other public case (no cert and
    no ACME; cert invalid/expired/SAN-mismatch; ACME with no domain) MUST
    fail `config.Load` with an error naming the specific condition
    (`backend-configuration.md` FR-6), before any other subsystem
    initializes. No degrade to plaintext on a public bind, ever.
  The FR-8 "interim note" in `backend-configuration.md` is removed by this
  phase's amendment because `ServeTLS` now exists.
- **FR-3** A `transport.Listeners` constructor MUST take `*config.Config`
  and the assembled `http.Handler` and return the set of listeners to
  run, decided by FR-2's classification:
  - **private, no cert** → one `http.Server`, `ListenAndServe` on
    `BIND_ADDRESS`, no `tls.Config`. Startup logs, once, at `warn`:
    that the bind is non-loopback and unencrypted, and that a reverse
    proxy or the opt-in `TLS_CERT_FILE`/`TLS_KEY_FILE` is recommended
    (constitution §11 — plain, specific; no exclamation).
  - **private, opt-in cert** → `ServeTLS` on `BIND_ADDRESS` with FR-4's
    `tls.Config` and the cert/key files. HSTS applies (FR-5).
  - **public, static cert** → `ServeTLS` on `BIND_ADDRESS` with FR-4's
    `tls.Config` and the cert/key files, **plus** the `:80` listener
    below (redirect only, no ACME handler).
  - **public, ACME** → `ServeTLS` on `BIND_ADDRESS` with
    `tls.Config.GetCertificate = manager.GetCertificate`, **plus** the
    `:80` listener below serving `manager.HTTPHandler(redirect)`. The
    `autocert.Manager` uses `DirCache(ACMECacheDir)` (created `0700`),
    `HostPolicy: HostWhitelist(ACMEDomain)`, `Email: ACMEEmail` if set,
    `Prompt: autocert.AcceptTOS` (this **accepts the CA Terms of Service
    on the operator's behalf** — `/network/status`'s ACME setup copy and
    the startup log state this and name the CA directory URL, ADR 0028
    §2).
  - **The `:80` listener** (present on *every* public bind): an
    `http.Server` on `:80` that 308-redirects to the `https://` form of
    the same host+path, and — in ACME mode only — serves
    `/.well-known/acme-challenge/*` via `manager.HTTPHandler`. It is
    covered by FR-8's rate limiter, serves nothing else, and never
    serves application content. If `:80` cannot be bound (in use), that
    is a startup failure in ACME mode (ACME cannot function without it)
    and a `warn` log otherwise (the redirect is a convenience).
- **FR-4** The `tls.Config` for every `ServeTLS` path MUST set:
  `MinVersion = tls.VersionTLS12`; `CipherSuites` = the six AEAD+ECDHE
  suites listed in ADR 0028 §3 (ignored by Go for TLS 1.3, which is fine);
  `NextProtos = []string{"h2", "http/1.1"}`. `PreferServerCipherSuites` is
  left unset (Go 1.17+ ignores it and orders by hardware AES support,
  which is the better behaviour).
- **FR-5** On any bind that is **actually serving TLS in-process** (a
  public bind, or a private bind with the opt-in cert — FR-3), a
  middleware MUST add `Strict-Transport-Security: max-age=31536000`
  (no `preload`, no `includeSubDomains`) to every response. It MUST NOT be
  added on a plaintext bind (a private bind with no cert): HSTS asserted
  over plaintext or on a bare LAN IP is pointless and occasionally
  harmful. The decision is made once at startup from FR-3's listener
  choice, not per-request.
- **FR-5a** A **security-headers middleware** MUST add, to **every
  response on every bind** (plaintext included — clickjacking and
  content-sniffing do not require TLS), per ADR 0028 §9:
  - `Content-Security-Policy` for the app document:
    `default-src 'self'; object-src 'none'; base-uri 'none';
    frame-ancestors 'none'; form-action 'self'`, with `script-src`,
    `style-src`, `img-src`, `connect-src`, `font-src` set to exactly what
    the built SPA needs and **no `unsafe-inline` on `script-src`** (the
    Vite build is configured to emit no inline script, or to use hashes).
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: no-referrer`
  - `Permissions-Policy` denying every browser feature the app does not
    use.
  This is the **app document's** policy and did not exist before phase 13
  (the only prior CSP is the phase-11 reader iframe's, ADR 0024, which is
  stricter and unchanged). The middleware sits just inside logging (chain
  order, API and contracts), so it covers the SPA HTML, the SPA fallback
  route, every `/api/v1/*` response (success and error), and the
  rate-limiter's `429` and CORS/Origin `403`. The one uncovered path is a
  request the **limits** layer rejects before this middleware runs (an
  oversized body) — that response is a JSON error, not a framed HTML
  document, so the gap carries no clickjacking or sniffing risk; a test
  documents it rather than pretending full coverage. The phase-13 audit
  **establishes** this policy (it does not "re-confirm" a pre-existing
  one).
- **FR-6** A **CORS middleware** MUST, per ADR 0028 §4:
  - With `CORSAllowedOrigins` empty: never set any
    `Access-Control-Allow-*` header; answer an `OPTIONS` preflight with
    `204` and no CORS headers (a non-CORS `OPTIONS`, effectively a no-op),
    so a cross-origin browser request fails the browser's own CORS check.
  - With entries present: on a request whose `Origin` header **exactly**
    (byte-for-byte) matches a configured entry, set
    `Access-Control-Allow-Origin: <that origin>`,
    `Vary: Origin`,
    `Access-Control-Allow-Methods` to the methods the route allows,
    `Access-Control-Allow-Headers: Authorization, Content-Type, X-Library-Id`,
    and `Access-Control-Max-Age: 600`. On a non-matching `Origin`: no CORS
    headers (same as empty). `Access-Control-Allow-Credentials` is **never**
    set (the API is Bearer-authenticated, not cookie-authenticated).
  - Matching MUST be exact string equality — no scheme folding, no port
    wildcard, no suffix match, no regex. A configured entry MUST be a
    well-formed `scheme://host[:port]` with no path; `config.Load` rejects
    a malformed entry.
- **FR-7** An **`Origin` validation middleware** MUST wrap the
  unauthenticated pairing routes (`POST /api/v1/network/pair/verify`, and
  any future unauthenticated state-changing route). Per ADR 0028 §5:
  - The **allowed set** MUST be computed at startup as the union of:
    every non-loopback interface address the server is bound to, at the
    bound port, in the scheme(s) the bind actually serves (`http://` for
    a plaintext private bind, `https://` for a TLS bind, both listed if
    the `:80` redirect is up); the configured mDNS `hostName` at that
    port in the same scheme(s); and every `CORS_ALLOWED_ORIGINS` entry.
    A default LAN deployment is reached both at the interface IP and at
    `alexandryn.local` — **both** must be in the set. Enumerating it is
    what prevents an implementer loosening the check to a substring
    match. A test exercises a multi-address deployment: a `pair/verify`
    from the LAN-IP origin and from the `.local` origin both pass; an
    unrelated origin gets `403`.
  - A request whose `Origin` header is **present and not in the set** →
    `403` with a `Forbidden`-category JSON body
    (`backend-errors-and-logging.md` FR-5).
  - A request with **no `Origin` header** (a native client, `curl`, a
    server-to-server call) → **allowed through**. `Origin` absence is not
    evidence of a forged request, and browsers always send `Origin` on
    cross-origin fetch/XHR and cross-origin form POST. The pairing code's
    single-use + ≤5-min TTL + per-IP rate limit is the real control;
    `Origin` validation only closes the "hostile page in a victim's
    browser" path.
  - `Referer` is consulted **only** when `Origin` is absent AND the
    method has a body; a present-and-non-matching `Referer` host is
    likewise rejected.
- **FR-8** A **global unauthenticated rate-limit middleware** MUST apply a
  per-client-IP token bucket (reusing `internal/auth`'s `IPRateLimiter`,
  or a shared extraction of it into `internal/transport/http`) to every
  request that `IsPublicPath` returns true for **and** that is not already
  covered by phase-12's auth-endpoint limiter — concretely: `/healthz`,
  `/readyz`, the embedded static assets, `POST /api/v1/network/pair/verify`,
  and (though it is authenticated) `POST /api/v1/network/pair/initiate`
  gets its own stricter bucket. Limits (all per IP, tunable later, flagged
  as reasoned placeholders per the pattern `backend-http-transport.md`
  FR-2 used): health/static 60/min burst 30; `pair/verify` 10/min burst 5;
  `pair/initiate` 5/min burst 3. Exceeding → `429` with a
  `RateLimited`-category JSON body. The client IP MUST be taken from the
  connection's `RemoteAddr`, **not** from `X-Forwarded-For` unless a
  future, explicit "trusted proxy" configuration exists (it does not in
  phase 13 — trusting a client-supplied header for rate-limit keying is
  itself the bypass).
- **FR-9** A `PairingCode` generation adapter MUST live here (it needs
  `crypto/rand`): `GeneratePairingCode() (domain.PairingCode, error)`
  reads 5 bytes from `crypto/rand`, encodes them as Crockford base32
  (8 characters), regroups as `XXXX-XXXX`, and returns the value through
  `domain`'s validated `PairingCode` constructor (`domain-device-pairing.md`
  FR-1/FR-2). A `crypto/rand` read error is returned, never swallowed or
  retried with a weaker source. The generator's acceptance criteria — not
  `domain`'s — assert the ≥ 40-bit entropy (5 `crypto/rand` bytes).
- **FR-9a** `cmd/server/run.go` MUST derive **three new HKDF subkeys** from
  the master application key, alongside the existing
  `jwt-signing-secret-v1` / `mfa-totp-master-v1` / `source-cursor-hmac-v1`:
  - `pairing-code-enc-v1` — AES-256-GCM key, encrypts the `PairingCode`
    at rest (`backend-network-api.md` FR-7).
  - `pairing-code-index-v1` — HMAC key, produces the blind lookup index
    for `pair/verify` so the row lookup is not a plaintext scan.
  - `enrolment-grant-v1` — HS256 signing key for the enrolment grant
    (FR-10), **distinct from the access-token key**.
  Distinct purposes, distinct keys — the phase-12 defect where an
  MFA ticket signed with the access-token key was accepted as a bearer
  token (audit `0012`-C2, `CLAUDE.md` Reflex) is not repeated.
- **FR-10** The **enrolment grant** minted by `pair/verify` MUST be
  produced and verified in `internal/auth` alongside the JWT signer:
  - an HS256 token signed with the **`enrolment-grant-v1` subkey**
    (FR-9a), never the access-token key;
  - claims `{ typ: "enrol", sid: <PairingSessionID>, jti: <uuid>, exp,
    iat, iss: "alexandryn" }` — **no `UserID`, no role**;
  - TTL ≤ **10 minutes** (the flow is "type your password on a TV
    keyboard", which is slow — 2 minutes from the draft was too tight);
  - **single-use**: `pair/verify` mints it; `POST /api/v1/auth/login`
    verifies it, and on success records the `jti` as spent
    (`backend-network-api.md` FR-9). A grant whose `jti` is already
    recorded is rejected — a replay cannot re-associate a device.
  - it authorises exactly one thing: `POST /api/v1/auth/login`
    associating the resulting session's device with the `PairingSession`.
  - an expired, malformed, wrong-subkey, wrong-`typ`, or replayed grant
    is rejected with `401` **by the login handler** — but per ADR 0028
    §6 / `backend-network-api.md` FR-9, an invalid grant does **not**
    fail the login itself; login proceeds without a device association
    and logs the fact. The grant is a convenience, not a gate.
- **FR-11** Graceful shutdown MUST call `Shutdown(ctx)` on every listener
  from FR-3 (the main server and, in ACME mode, the `:80` server)
  concurrently, bounded by `SHUTDOWN_GRACE_PERIOD`, and MUST return the
  first non-nil error. A shutdown that exceeds the grace period MUST
  `Close()` the remaining listeners and log which ones did not drain
  (`backend-service-lifecycle.md` FR-5's contract, extended to the second
  socket).
- **FR-12** **Authentication is not disable-able (ADR 0028 §7).** No
  configuration key, alone or combined, produces a reachable bind with
  `AuthMiddleware` bypassed, and this phase adds none. A test asserts
  `IsPublicPath` gains only FR-8's additions
  (`POST /api/v1/network/pair/verify`) and that no other route becomes
  public; a second test asserts `PATCH /api/v1/network/settings` rejects
  an `authRequired` key (`backend-network-api.md` FR-5).
- **FR-13** This phase carries the **phase-12 authorization hardening
  prelude** (review `0050`, audit `0012`-C1/C2/P12-4). The middleware
  changes land in this spec's territory (`internal/transport/http`):
  - **`AuthMiddleware` MUST assert the access-token type.** Token
    verification on the auth path rejects any token whose `typ` is not
    the access type — an MFA ticket, an enrolment grant, anything else —
    with `401` even when the signature is valid. `internal/auth`'s
    verifier gains a `VerifyAccessToken` (or `Verify` grows a required
    type check); `AuthMiddleware` calls it. Test: access ✓, `mfa_ticket`
    ✗, `enrol` ✗, garbage ✗. (Enforces `CLAUDE.md`'s token-type Reflex;
    `backend-authentication.md` FR-7 carries the owning amendment.)
  - **`AuthMiddleware` MUST validate `X-Library-Id` against the token's
    `libraries` claim.** A request whose `X-Library-Id` is not in
    `claims.Libraries` → `403 Forbidden`. Test proves a `reader` cannot
    set an arbitrary library header. (`backend-library-namespaces.md`
    carries the owning amendment.)
  - The reading/reader **handler** fixes (per-user + per-library scoping)
    are `backend-reading-api.md` / `backend-reader-content.md`'s to own;
    this spec only notes that the phase-13 close gate
    (`roadmap/13-network-access/README.md`) does not pass until all of
    FR-13 is verified, because opening the bind on an API where any
    account can read every account's data is the failure this phase
    exists to prevent.

## Non-functional requirements

- **Performance** — TLS handshake cost is per-connection, not per-request;
  keep-alive (`IdleTimeout`, already 120s) amortizes it. The CORS,
  `Origin`, HSTS, and rate-limit middlewares are O(1) header/map work;
  the rate limiter's per-IP map is bounded by TTL eviction (already in
  `IPRateLimiter`). No new per-request allocation on the hot path beyond
  a header set.
- **Security** — see Security considerations; FR-2's fail-closed bind is
  constitution §6 / ADR 0017 made executable for the public case for the
  first time.
- **Accessibility** — N/A (transport).
- **Reliability** — a certificate that expires while the server runs:
  static-cert mode does **not** hot-reload (documented limitation — a
  restart picks up a renewed file; a future watch is a phase-16 item);
  ACME mode renews automatically via `autocert`. A `:80` listener that
  fails to bind (port in use) in ACME mode is a **startup failure**, not a
  degrade — ACME cannot function without it. HTTP/2 (offered via ALPN) is
  served with an explicit `http2.Server{MaxConcurrentStreams,
  MaxReadFrameSize}` rather than defaults, and the Go toolchain pin
  carries the rapid-reset mitigation (CVE-2023-44487) — noted so a
  toolchain downgrade is caught in review.
- **Observability** — startup logs, once, at `info`: the resolved bind
  address, the classification (`private` / `public`), the TLS mode
  (`none` / `static` / `acme`), and — for ACME — the domain, the CA
  directory URL (so the operator knows which CA's TOS was accepted, ADR
  0028 §2), and the cache directory (a path under the data dir, not a
  home-relative path beyond what `architecture-persistence.md` already
  logs). A **non-loopback plaintext bind** additionally logs once at
  `warn` (FR-3): the bind is unencrypted and a reverse proxy or the
  opt-in cert is recommended. Never logged: the certificate private key,
  the `DEVICE_PAIRING_SECRET`, a `PairingCode`, an enrolment grant, the
  ACME account key, or any of FR-9a's subkeys. A rejected bind logs the
  reason at `error` before the process exits.

## Domain model

None new. This spec consumes `domain.PairingCode` (constructs it in FR-9)
and references `domain.PairingSessionID` (FR-10). It defines Go types in
`internal/transport/http` / a new `internal/transport` helper only —
`Listeners`, the middlewares, the grant type in `internal/auth`.

## API and contracts

- **`internal/config` ↔ everything** — the new keys travel in `*Config`,
  same contract as `backend-configuration.md` FR-1.
- **`transport.Listeners(cfg, handler)` → `([]*http.Server, error)`** (or
  a small struct) — the one new construction contract; `cmd/server` /
  `backend-service-lifecycle.md`'s sequence calls it.
- **Middleware chain — this phase amends `backend-http-transport.md` FR-1
  and `architecture-backend.md` FR-6** (both reserve a single slot for
  auth "between logging and routing"; the amendment expands that slot into
  an ordered group). Full order, outermost-in:
  `panic recovery → request limits → structured logging →
  security headers (FR-5a, all binds) → HSTS (FR-5, TLS binds) →
  CORS (FR-6) → global rate limit (FR-8, public paths) →
  auth (FR-13: access-token-type + X-Library-Id checks) →
  Origin validation (FR-7, pairing route group wrapper) → routing`.
  - Recovery stays strictly outermost; limits and logging keep their
    phase-03 positions; routing stays innermost. Only the auth slot grows.
  - CORS and the rate limiter sit **before** auth: a preflight carries no
    credentials and an unauthenticated flood should be shed before token
    verification.
  - Origin validation is a **route-group wrapper on the pairing routes
    only**, not a global layer — every other state-changing route is
    already `Authorization`-gated and CSRF-safe by construction (ADR 0028
    §5).
  - A limits-layer rejection (oversized body → JSON error, not framed
    HTML) not carrying the security headers is acceptable.
  Each layer is a `func(http.Handler) http.Handler` (ADR 0011).
- **Enrolment grant** — `internal/auth` gains
  `SignEnrolmentGrant(sid, now) (string, error)` and
  `VerifyEnrolmentGrant(token, now) (EnrolmentClaims, error)`; consumed
  by `backend-network-api.md`'s `pair/verify` handler and by the
  phase-12 login handler (which gains an optional grant parameter).
- **OpenAPI** — this spec adds no paths; `backend-network-api.md` does.
  The `429`/`403` error bodies reuse `architecture-contracts.md` FR-5's
  shape, already in the schema.

## State transitions

The server process gains no new application state
(`architecture-system.md`'s `Starting`/`Ready`/`Degraded` are unchanged).
The bind classification is a one-time startup decision, not a runtime
state. ACME certificate state (`none` → `issued` → `renewing` → `issued`)
is entirely inside `autocert` and not modelled here.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Public bind, no cert, no ACME | FR-2 in `config.Load` | Startup failure naming the address and that a cert or ACME config is required | `config.Load` returns an error; process exits non-zero before binding |
| Public bind, static cert expired / key mismatch / wrong SAN | FR-2 | Startup failure naming which check failed | Same |
| ACME enabled, `ACME_DOMAIN` unset | FR-2 | Startup failure naming the missing key | Same |
| ACME mode, `:80` already in use | FR-3 bind | Startup failure naming the port conflict | Process exits; no partial "HTTPS up, ACME down" state |
| ACME issuance fails at runtime (rate limit, DNS not pointing here) | `autocert` handshake error | TLS handshake fails for that client; `info` log line | Server keeps running; `autocert` retries per its own backoff; existing cached cert (if any) still served |
| Cross-origin browser POST to `pair/verify` from a hostile page | FR-7 `Origin` check | `403 Forbidden` JSON | Request rejected before the handler; logged with correlation ID, not the body |
| Anonymous client floods `/healthz` | FR-8 rate limiter | `429` JSON after the burst | Bucket drains; entry TTL-evicted when the client stops |
| `X-Forwarded-For` spoofed to evade the rate limiter | FR-8 uses `RemoteAddr` only | No effect — header ignored | Limiter keys on the real peer address |
| Static cert renewed on disk while running | Reliability NFR | No change until restart | Documented: static mode does not hot-reload; restart picks it up |
| Shutdown exceeds grace period with a slow ACME connection | FR-11 | — | Remaining listeners `Close()`d; log names the undrained socket |
| `crypto/rand` read fails during code generation | FR-9 | `pair/initiate` returns `500` `Internal` JSON | Error returned, no code minted, never falls back to `math/rand` |

## Security considerations

- **Trust boundary — host ↔ network client.** Before this phase the only
  client was loopback (trusted, `architecture-system.md`). After it, a LAN
  or public client is assumed hostile (`architecture-system.md` "a LAN
  client is assumed hostile"). Every new endpoint validates shape, size,
  and time before its handler runs (constitution §4) — the request-limits
  middleware (`backend-http-transport.md` FR-2) already covers body size
  and timeouts; this spec adds rate and origin checks.
- **Fail-closed public bind (FR-2)** — the core constitution §6 / ADR
  0017 property, now executable for Mode A. A public address with nothing
  to encrypt or authenticate the connection does not produce a listener.
  The check is against the resolved address and the certificate/ACME
  material actually present, never a flag (ADR 0028 §1).
- **TLS policy (FR-4)** — 1.2 floor rules out SSLv3/TLS1.0/1.1
  downgrade; the cipher list is AEAD-only with ECDHE, so no CBC padding
  oracles and forward secrecy throughout; 1.3 preferred where available.
- **ACME `HostPolicy` (FR-3)** — pinned to one domain. Without it,
  `autocert` would attempt issuance for any SNI a client sends, which is
  an abuse vector against the CA's rate limits and a way to make the
  server do work on an attacker's behalf. This is why `ACME_DOMAIN` is
  required, not inferred.
- **ACME cache permissions (FR-3)** — `0700`. The directory holds the
  ACME account private key and issued certificate private keys; these are
  secrets on disk (`backend-configuration.md` Open questions flagged
  filesystem permissions for secrets — this is the first concrete one).
- **CSRF — deliberately no token machinery (ADR 0028 §5).** Sessions are
  `Authorization: Bearer` from `localStorage`, not cookies. A cross-site
  request cannot carry the `Authorization` header (the browser will not
  attach it to a request an attacker's page makes), so a synchronizer
  token or double-submit cookie would guard a path that does not exist.
  The authenticated API is CSRF-resistant by construction. The one real
  cross-origin exposure — an unauthenticated `pair/verify` POST from a
  page in a victim's browser — is closed by FR-7's `Origin` check, and
  even without it the code is single-use, ≤ 5-minute-lived, rate-limited,
  and yields only an enrolment grant (not a session). This reasoning is
  recorded so a future reviewer does not read the absent CSRF token as an
  oversight (constitution §12).
- **CORS deny-by-default (FR-6)** — the default deployment emits no CORS
  headers; the same-origin SPA never needs them. Exact-match only closes
  the family of `Origin`-suffix and scheme-fold bypasses. `Allow-Credentials`
  is never combined with anything permissive because it is never sent.
- **Rate-limit keying (FR-8)** — on `RemoteAddr`, never a client header.
  Trusting `X-Forwarded-For` for keying is the standard rate-limit bypass;
  a "trusted proxy" configuration that would make it safe does not exist
  in phase 13 and is a separate, explicit future decision.
- **Enrolment grant scope (FR-10)** — signed with its own subkey
  (`enrolment-grant-v1`), no `UserID`, no role, `typ: "enrol"`, single-use
  via `jti`, ≤ 10 minutes. It authorises exactly one association step at
  `POST /api/v1/auth/login` and nothing else: a stolen grant cannot read
  the library, cannot be refreshed, is rejected on the access path by the
  FR-13 type check *and* by the distinct signing key, and is consumed the
  first time it is spent. It expires before it is useful for anything but
  the login it was minted for.
- **`DEVICE_PAIRING_SECRET` (FR-1)** — a redacted config type
  (`backend-configuration.md` FR-7), never logged, never in an error
  string, never returned by `/network/status`. It is an optional extra
  factor on `pair/initiate`, not a login credential (ADR 0028 §6).
- **Residual risk carried from ADR 0025 — `localStorage` token exposure.**
  Bearer tokens in `localStorage` are XSS-reachable, and opening the bind
  widens who can attempt to serve a malicious payload to the app origin.
  This spec does not fix it (out of scope — ADR 0025's accepted
  trade-off, revisit in phase 16). The phase 13 **audit** MUST re-confirm
  the phase-11 CSP and `bluemonday` sanitisation posture (ADR 0024) holds
  for all LAN-served content and that no endpoint added in this phase
  reflects unsanitised input into an HTML response.
- **STRIDE** — *Spoofing*: auth middleware unchanged + `Origin` check on
  pairing. *Tampering*: TLS integrity (Mode A); no new parser of
  untrusted bytes except the `PairingCode` shape check (in `domain`).
  *Repudiation*: startup + per-request logs with correlation IDs
  (unchanged), pairing events logged with IDs not secrets.
  *Information disclosure*: FR-Observability's never-log list; ACME cache
  `0700`; `/network/status` scrubbing (`backend-network-api.md`).
  *Denial of service*: FR-8 rate limits, existing request timeouts,
  `HostPolicy` prevents ACME amplification. *Elevation of privilege*: the
  enrolment grant carries no authority; pairing never sets a role.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | FR-2 classification + acceptance/rejection matrix (loopback ✓ no cert; private ✓ no cert; public + valid static cert ✓; public + expired/mismatched/wrong-SAN ✗ each with the right error; public + ACME configured ✓; public + nothing ✗; ACME + no domain ✗) — extends `internal/config/bindaddress_test.go`. FR-4 `tls.Config` builder: version floor, exact cipher list, ALPN order. FR-6 CORS middleware: empty allowlist emits nothing; exact match echoes; near-miss (`http` vs `https`, trailing slash, subdomain) emits nothing; preflight behaviour. FR-7 `Origin` middleware: same-origin ✓, absent ✓, foreign ✗ 403, `Referer` fallback. FR-8 rate limiter: burst then 429, TTL eviction, `X-Forwarded-For` ignored. FR-9 code generation: format, entropy, `crypto/rand` error propagation, no `math/rand` import (audit test). FR-10 grant sign/verify: expiry, tamper, no-UserID. FR-5 HSTS present on a synthetic public classification, absent on loopback. |
| Integration | `-tags=integration`. **Static-cert `ServeTLS`**: generate a throwaway cert for `127.0.0.1` (already have `tlsfixture_test.go`), bind, complete a real TLS 1.3 and a forced TLS 1.2 handshake, assert ALPN and that a plaintext request to the port fails. **ACME against Pebble** (ADR 0028 §2, plan C-7): run Pebble as a service, point `autocert` at its directory URL, serve HTTP-01 on the challenge port, assert a certificate is issued on first handshake, then set the cached cert's `NotAfter` near now (or use Pebble's profile) and assert a renewal occurs. **HTTP→HTTPS redirect**: a request to `:80` gets a 308 to the `https://` form; an ACME challenge path is served, not redirected. **Graceful shutdown** with both listeners: an in-flight slow request drains within grace; a stuck one is `Close()`d and logged. |
| Contract | No new paths here; `backend-network-api.md` owns contract tests. The `429`/`403` bodies are asserted to match `architecture-contracts.md` FR-5 in that spec's contract suite. |
| E2E | Covered via `frontend-network-and-pairing.md`'s Playwright pairing flow (runs against a loopback build with a self-signed cert or plain HTTP). |
| Accessibility | N/A. |

Must fail before implementation: the FR-2 acceptance matrix (public +
valid cert currently rejected → must pass); the static-cert `ServeTLS`
integration test (nothing calls `ServeTLS` today); the CORS empty-allowlist
test; the global rate-limit 429 test on `/healthz`.

## Acceptance criteria

- [ ] A publicly routable `BIND_ADDRESS` with a valid static certificate
      **starts and serves HTTPS**; the same address with no certificate
      and no ACME **fails to start** with a specific error — both proven,
      replacing the current blanket rejection and removing
      `backend-configuration.md`'s FR-8 interim note.
- [ ] A publicly routable bind with `ACME_ENABLED=true` and a valid
      `ACME_DOMAIN` obtains a real certificate from Pebble and renews it
      within one test — the roadmap's "real certificate lifecycle" exit
      criterion.
- [ ] `ServeTLS` negotiates TLS 1.3 with a modern client and TLS 1.2 (from
      the fixed cipher list) with a 1.2-only client; a plaintext request
      to the TLS port is refused; ALPN offers `h2` then `http/1.1`.
- [ ] HSTS is present on a public bind and absent on a loopback bind.
- [ ] With `CORS_ALLOWED_ORIGINS` empty, no response carries any
      `Access-Control-Allow-*` header; with one origin configured, only a
      byte-exact `Origin` is echoed and `Allow-Credentials` is never sent.
- [ ] A cross-origin `POST /api/v1/network/pair/verify` with a foreign
      `Origin` gets `403`; the same request with no `Origin` is allowed
      to the handler.
- [ ] `/healthz`, `/readyz`, static assets, and `pair/verify` are
      rate-limited per source IP; the limiter ignores `X-Forwarded-For`;
      exceeding returns `429` in the shared error shape.
- [ ] Authentication cannot be disabled: a test proves no config key,
      alone or combined, produces a reachable bind with `AuthMiddleware`
      bypassed, and `IsPublicPath` gains only the FR-8 additions.
- [ ] Graceful shutdown drains both the main and the ACME/redirect
      listener within `SHUTDOWN_GRACE_PERIOD`; an undrained socket is
      `Close()`d and named in a log line.
- [ ] `GeneratePairingCode` produces a `XXXX-XXXX` Crockford code with ≥
      40 bits of entropy and propagates a `crypto/rand` failure without
      falling back.
- [ ] No file in this phase's transport code imports `math/rand`; no
      rate-limit or auth decision reads a client-supplied header for its
      key — both proven by an audit test.
- [ ] Every FR maps to a line in phase 13's exit criteria.

## Open questions

- **mDNS/`alexandryn.local` advertisement owner.** The design shows an
  mDNS host name; whether the Go server advertises it (needs a library —
  `github.com/grandcat/zeroconf` or `hashicorp/mdns`, a §9 decision) or
  the Electron host process does (it already owns OS integration) is not
  settled. `backend-network-api.md`'s `/network/status` reports the name
  regardless; advertisement is deferred to that spec's scope review or a
  follow-up. Flagged, not assumed.
- **Static-certificate hot reload.** Deferred — a `fsnotify` watch on the
  cert files is a phase-16 hardening item; phase 13 documents that a
  renewed static cert needs a restart. ACME renews without one.
- **Trusted-proxy `X-Forwarded-For` handling.** A user behind their own
  reverse proxy will see every client as the proxy IP for rate-limiting.
  Phase 13 keys on `RemoteAddr` (correct and safe by default); a
  `TRUSTED_PROXY_CIDRS` config that would let `X-Forwarded-For` be
  honoured from known proxies is a deliberate future addition, not a
  phase-13 gap to paper over now.
- **`ACME_EMAIL` requiredness.** `autocert` works without an account
  email but the CA cannot send expiry warnings. Left optional; the
  `/network/status` view can surface "no ACME contact email set" as
  advice. Confirm at scope review.

## References

- ADR 0028 — every decision this spec implements (§1 TLS derivation, §2
  ACME, §3 cipher policy, §4 CORS, §5 CSRF, §6 pairing, §7 no-auth-off,
  §8 runtime settings)
- ADR 0017 — the fail-closed bind rule; FR-2 is its Mode A made executable
- ADR 0025 — the Bearer session mechanism this spec does not change
- `backend-configuration.md` FR-4 (key table, amended by this phase), FR-6
  (fail-loudly), FR-7 (redacted types), FR-8 (classifier + interim note
  removed by this phase)
- `backend-http-transport.md` FR-1 (middleware chain this spec extends),
  FR-2 (body/timeout limits already in place), Non-goals (CORS
  un-deferred by this phase's amendment)
- `backend-service-lifecycle.md` FR-1 (startup sequence), FR-5 (shutdown
  grace contract, extended to two sockets)
- `domain-device-pairing.md` — `PairingCode` (FR-9 constructs), the
  pairing lifecycle
- `backend-network-api.md` — the endpoints, persistence, `/network/status`
  scrubbing
- `internal/config/bindaddress.go`, `internal/auth/ratelimit.go`,
  `internal/config/tlsfixture_test.go` — existing code this builds on
- Constitution §4, §6, §8, §9, §11, §12
