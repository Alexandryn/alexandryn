# 0028. Phase 13 network transport: bind mode is derived from the resolved address and certificate state, not a flag

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-02 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

<!-- Status: Proposed | Accepted | Rejected | Superseded | Deprecated -->

Headline: the listener the server binds is chosen from the resolved
`BIND_ADDRESS` class plus the certificate/ACME material actually present,
fail-closed, never from a configuration flag. Around that: ACME via
`golang.org/x/crypto/acme/autocert` (no new module); an app-origin CSP and
security headers on every bind; CORS deny-by-default; no CSRF synchroniser
tokens under Bearer auth (`Origin` validation on the unauthenticated
pairing routes instead); device pairing as a thin bootstrap over phase-12
accounts, never a second credential system; authentication that cannot be
disabled.

## Context

Phase 13 opens the host beyond loopback for the first time. ADR 0017
already fixed the *rule* that gates a bind (authentication enforced, plus
one of two mutually exclusive TLS modes, all checked fail-closed against
the address and certificate state actually present). ADR 0017 deliberately
left three things to phase 13: the exact certificate-validation depth for
Mode A, the certificate *provisioning* mechanism (static file vs. ACME),
and every non-TLS transport concern the roadmap added on 2026-08-18 —
CORS, CSRF, session-cookie attributes, general rate limiting, device
pairing.

The `DRAFT` form of this ADR and its four sibling specs were reviewed by
two independent agents (`.claude/reviews/0050-phase13-spec-package-and-phase12-authz-review.md`).
That review confirmed the transport core below is sound and reshaped six
of the decisions (encrypted-at-rest pairing codes rather than hash-only;
a distinct signing subkey per token purpose; an app-origin CSP as a
phase-13 deliverable, not a phase-16 one; opt-in in-process TLS for a
private bind; the `Origin` allowed-set enumerated for multi-address LAN
deployments; device ownership assigned at login, not at pairing
initiation). It also surfaced two **High**-severity phase-12
authorization defects in already-shipped code — the reading API is not
user- or library-scoped, and access-token verification does not check the
token type — which audit `0012` had certified as controls-present. Those
are corrected in audit `0012` and fixed as a phase-12 hardening prelude on
the phase-13 branch; this ADR's decision 6 and decision 9 depend on the
fixes.

Two facts on the ground shape these decisions and were not all true when
the roadmap outline was written:

- **Phase 12 chose Bearer-token sessions, not cookies** (ADR 0025,
  2026-09-02). Access tokens are HS256 JWTs carried in
  `Authorization: Bearer`, held in the browser's `localStorage`, attached
  by one function (`web/src/data/http.ts` `defaultHeaders()`). Nothing in
  the codebase sets or reads a cookie. The roadmap's phase-13 line item
  "session cookie attributes (`Secure`, `SameSite`, `Domain` scope)"
  describes a mechanism that does not exist.
- **`golang.org/x/crypto` is already a direct dependency** (Argon2id,
  phase 12). `golang.org/x/crypto/acme/autocert` is a sub-package of it —
  ACME support adds no new module (constitution §9).

The design reference (`.design-reference/ANALYSIS.md`, synced 2026-08-13,
classified 2026-08-17) has exactly one **binding** entry for this
surface — `sgNetwork`'s "Advanced" disclosure row — and it is
**contents-uncaptured** (`ANALYSIS.md`'s ledger). The first-run `fr4`
step and `atSettings`'s other tabs are classified **Unclassified**; there
is no `sgDevices` ledger row at all. So the design contradictions this
ADR resolves are read off the one binding row plus the roadmap text, not
off `fr4`:

- **D-2** — `sgNetwork` draws a "Require authentication" toggle that can be
  switched off (decision 7 forbids it).
- **D-5** — `sgNetwork:223` draws an "Allow access from this network"
  toggle that would change the bind at runtime (decision 8 makes the bind
  restart-only config; the toggle becomes a read-only reachability
  statement).
- **D-1** — `atConnect` shows a shared "Library passphrase" sign-in rather
  than per-user login (decision 6 keeps per-user login and adds pairing as
  a bootstrap, not a passphrase).

`fr4`'s and `sgDevices`'s Unclassified/absent status is itself flagged in
the frontend spec as an open item for the maintainer, per CLAUDE.md's
stop-and-ask rule for unclassified surfaces — this ADR does not lean on
them as authority.

## Decision

### 1. TLS mode is derived from the resolved bind address and certificate state, never from a configuration flag

There is no `TLS_ENABLED` key. `config.Load` classifies `BIND_ADDRESS`'s
resolved host (the FR-8 classifier that already exists in
`internal/config/bindaddress.go`) and the server selects its listener from
that classification plus the certificate/ACME state actually present.

**How a host is classified — stated explicitly** (ADR 0017 left it
implicit; the current `isLoopbackOrPrivate` rejects every non-`localhost`
name outright):

- An **IP literal** → classified by `net.IP.IsLoopback()` / `IsPrivate()`
  / IPv6-ULA check. Loopback or private → Mode B. Anything else → Mode A.
- The literal string **`localhost`** → loopback → Mode B.
- **Any other host string** (a DNS name) → classified **publicly
  routable → Mode A, without any DNS resolution.** The config layer never
  performs network I/O (`architecture-testing.md` FR-6 determinism), and
  the safe default for "a name we cannot classify locally" is the
  fail-closed one: require in-process TLS. An operator who runs
  `alexandryn.local` behind a reverse proxy and wants Mode B sets
  `BIND_ADDRESS` to the private IP the proxy forwards to, not the name.

Then, by class:

- **Loopback or private range** (`127.0.0.0/8`, `::1`, `localhost`, RFC
  1918, IPv6 ULA) → **Mode B by default**: plain
  `http.Server.ListenAndServe`; TLS, if any, terminates upstream; no
  certificate required of this process. **Opt-in in-process TLS**: if
  `TLS_CERT_FILE`/`TLS_KEY_FILE` are both present on a private bind, the
  server uses `ServeTLS` with them (same validation as Mode A minus the
  SAN check, since a private bind is often reached by IP). ACME is **not**
  available for a private bind — HTTP-01 domain validation cannot succeed
  for a private address. This is recorded as an **ADR 0017 Mode-B
  sub-case**: ADR 0017 says a reverse proxy "may" terminate TLS; it does
  not forbid the process also holding a cert, and a self-hoster who wants
  LAN traffic encrypted without running a proxy should be able to. ADR
  0017 stays `Accepted`; this ADR extends it.
- **Publicly routable** → `http.Server.ServeTLS` is **mandatory**. The
  certificate comes from one of two sources:
  - `ACME_ENABLED=false` (default): a static `TLS_CERT_FILE` /
    `TLS_KEY_FILE` pair, loaded and validated at startup (well-formed,
    key matches leaf, within validity window, and — the depth decision
    ADR 0017 left open — the leaf's DNS SANs must include the host portion
    of `BIND_ADDRESS` when that host is a name; a bare-IP public bind
    validates expiry and key-match only, since a public bare-IP bind is an
    unusual operator choice and a name-match check on an IP is
    meaningless).
  - `ACME_ENABLED=true`: an `autocert.Manager` issues and renews the
    certificate for `ACME_DOMAIN` via the ACME HTTP-01 challenge.
  - Neither present, or the static pair invalid/expired/mismatched-SAN, or
    `ACME_DOMAIN` unset while `ACME_ENABLED=true` → **startup fails**
    before any other subsystem initializes. No degrade to a plaintext
    listener on a public address, ever.
  - A **`:80` HTTP→HTTPS redirect listener** runs for *every* public bind,
    not only the ACME one — a static-cert public deployment should not
    leave `http://` connection-refused. In ACME mode the same listener
    also serves `manager.HTTPHandler` for the challenge. It is
    rate-limited and serves only `/.well-known/acme-challenge/*` (ACME
    mode) plus 308 redirects; never application content.

`ACME_ENABLED` is not a security toggle: both Mode A branches fail closed
identically. It selects a certificate *source*, which is a real operator
choice, not an assertion about the network topology (the thing ADR 0017
Option B was rejected for trusting).

### 2. ACME uses `golang.org/x/crypto/acme/autocert`

- Challenge type: **HTTP-01 only.** The `:80` listener from decision 1
  serves `manager.HTTPHandler(redirect)`, where `redirect` 308s every
  non-challenge request to the HTTPS address. This listener never serves
  application content.
- `manager.HostPolicy = autocert.HostWhitelist(ACME_DOMAIN)` — issuance
  is attempted for exactly one name. A client presenting any other SNI
  gets no certificate and no issuance attempt. This is what prevents the
  server doing ACME work on an attacker's behalf.
- `manager.Prompt = autocert.AcceptTOS` — `autocert` requires this to
  register an ACME account, so enabling ACME **accepts the CA's Terms of
  Service on the operator's behalf**. The Network panel's ACME setup copy
  states this plainly and links the configured CA's TOS, and the startup
  log names the CA directory URL, so the operator knows what was agreed
  to (constitution §11, §12).
- Cache: `autocert.DirCache(ACME_CACHE_DIR)`, created with `0700`
  permissions. Contents (the account key and issued certificate keys) are
  secrets on disk; the directory is not world- or group-readable.
- Renewal is `autocert`'s own background behaviour (it renews when a
  certificate is within 30 days of expiry, on demand at handshake time).
  The phase's exit criterion ("tested against a real certificate
  lifecycle") is met with **Pebble** (Let's Encrypt's test ACME server)
  in the integration suite: issuance on first handshake, then a forced
  near-expiry renewal, asserted end to end. Pebble is a test service
  container, not a `go.mod` dependency — the same shape as PostgreSQL in
  integration tests.
- **§9 record for `acme/autocert`** — *what it does*: ACME client,
  certificate issuance/renewal, on-disk cache, HTTP-01 handler. *Why not
  stdlib*: `crypto/tls` and `golang.org/x/crypto/acme` (the lower-level
  package) give the primitives, but the account lifecycle, renewal
  scheduling, `GetCertificate` integration, and cache are non-trivial and
  security-sensitive to hand-roll. *Why no new module*: `autocert` is a
  sub-package of `golang.org/x/crypto`, already a direct dependency
  (Argon2id). *What breaks if abandoned*: `golang.org/x/crypto` is
  maintained by the Go team; if `autocert` were ever removed, a public
  bind would need either a manual/static-cert workflow (already the
  default path — `ACME_ENABLED=false` keeps working) or a replacement ACME
  library. No data-format lock-in: the cache is standard PEM files.

### 3. TLS version and cipher policy

`tls.Config` for the in-process listener (Mode A):

- `MinVersion: tls.VersionTLS12`. TLS 1.3 is negotiated when the client
  supports it.
- For TLS 1.2, an explicit `CipherSuites` list restricted to AEAD suites
  with forward secrecy:
  `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`,
  `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`,
  `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`,
  `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`,
  `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`,
  `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`.
  (TLS 1.3 cipher suites are not configurable in Go and are all safe.)
- `NextProtos: ["h2", "http/1.1"]` — ALPN offers HTTP/2 and HTTP/1.1.
  Go's `net/http` serves HTTP/2 automatically over TLS. The Go toolchain
  is pinned to a version carrying the HTTP/2 rapid-reset mitigation
  (CVE-2023-44487, fixed in Go 1.21.3+ / already far below the module's
  `go 1.26` line) and the server sets an explicit
  `http2.Server{MaxConcurrentStreams, MaxReadFrameSize}` rather than
  relying on defaults.
- **HSTS** (`Strict-Transport-Security: max-age=31536000`, no `preload`,
  no `includeSubDomains` by default — a self-hoster's subdomain layout is
  theirs) is sent on **any bind that is actually serving TLS in-process**
  — a public bind, or a private bind with opt-in in-process TLS
  (decision 1). It is **never** sent on a plaintext bind: Mode B may
  legitimately be plaintext behind a proxy, and HSTS asserted over
  plaintext, or on a bare LAN IP, is pointless and occasionally harmful.
- The security-response-header set (`Content-Security-Policy`,
  `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`,
  `Permissions-Policy`) is **decision 9** and applies to every bind,
  plaintext included.

### 4. CORS is deny-by-default; the allowlist is exactly the configured origins

`CORS_ALLOWED_ORIGINS` is a comma-separated list, **empty by default**.

- Empty list → the CORS middleware never emits an
  `Access-Control-Allow-Origin` header and never answers a preflight with
  a permissive response. Same-origin requests (the SPA the Go server
  itself serves — `architecture-system.md` FR-6) do not exercise CORS and
  are unaffected. This is the expected posture for every default
  deployment.
- Non-empty list → an incoming `Origin` is echoed back **only on exact
  string match** against a configured entry. No scheme-insensitive match,
  no port wildcard, no suffix/prefix/regex match, no trailing-slash
  normalization beyond a single documented rule. `Access-Control-Allow-Credentials`
  is never set to `true` in combination with a wildcard origin (the
  combination is invalid per the Fetch standard and a known bypass
  shape); since the API is Bearer-authenticated, not cookie-authenticated,
  `Allow-Credentials` is not needed at all and is omitted.

The one cross-origin case this supports: a user running a reverse proxy
that terminates TLS on a hostname the Go process doesn't know itself by.
That operator adds their external origin to `CORS_ALLOWED_ORIGINS`.

### 5. No CSRF token machinery; Origin validation on unauthenticated pairing routes

Because sessions are `Authorization: Bearer` from `localStorage` and not
cookies, a forged cross-site request cannot ride an ambient credential —
the browser does not attach the `Authorization` header to a request the
attacker's page initiates. Classic CSRF (synchronizer token,
double-submit cookie) would defend a vector that does not exist here, and
adding it would be security theater (constitution §4).

Instead:

- Every state-changing route already requires a valid `Authorization`
  header (phase 12 middleware). This is, by construction, CSRF-resistant.
- The **unauthenticated** pairing endpoints (`POST /api/v1/network/pair/verify`
  in particular) are the one place a forged cross-origin POST could
  matter. These validate the request `Origin` against an **enumerated
  allowed set** and **reject only a present-and-non-matching** `Origin`
  with `403`:
  - the allowed set is: every non-loopback interface address the server
    is bound to, at the bound port, in both `http://` and `https://` form
    as appropriate to the bind mode; the configured mDNS `hostName`
    (`http(s)://alexandryn.local:port`); and every `CORS_ALLOWED_ORIGINS`
    entry. A default LAN deployment is reached at the interface IP *and*
    at the `.local` name, so both must pass — enumerating them is what
    stops an implementer loosening the check to a substring match.
  - a request with **no `Origin` header** is **allowed through** — a
    native client, `curl`, or a server-to-server call sends none, and
    `Origin` absence is not evidence of a forged request. The pairing
    code's single-use + ≤5-minute TTL + rate limit is the actual control;
    `Origin` validation only closes the "hostile web page in a victim's
    browser" path, and browsers always send `Origin` on cross-origin
    fetch/XHR and cross-origin form POST. `Referer` is consulted only as a
    fallback when `Origin` is absent *and* the method has a body, and a
    present-non-matching `Referer` host is likewise rejected.
- This reasoning is recorded in `backend-network-transport.md`'s Security
  considerations so a future reviewer does not read the absence of a CSRF
  token as an oversight (constitution §12).

If a later phase reintroduces a cookie-held token (e.g. a device-scoped
session cookie), this decision is void for that surface and `SameSite` +
a CSRF token become required. The pairing model below is chosen partly to
avoid that.

### 6. Device pairing is a thin bootstrap over phase-12 accounts, not a second credential system

- A LAN or remote browser client authenticates with the **phase-12
  account flow** (`POST /api/v1/auth/login`, username/password → JWT +
  refresh token). ADR 0025 is unchanged. RBAC, multi-library membership,
  and audit all key off `UserID` for every client, local or remote.
- **Pairing** exists to remove the friction of typing a host address and
  credentials on a device with a poor keyboard (a TV, a phone). The host
  operator (an `admin`) initiates a pairing; the server mints a
  `PairingSession` holding a one-time `PairingCode` (8 Crockford-base32
  characters = 40 bits, single-use, TTL ≤ 5 minutes, invalidated on first
  successful verification, compared in constant time — see
  `domain-device-pairing.md`). The code is delivered as a QR payload
  (host URL + code) and as displayed text.
- **Code storage — encrypted at rest, not hash-only.** The
  `pairing_sessions` row stores the code as AES-256-GCM ciphertext under a
  dedicated HKDF subkey (`DeriveSubkey("pairing-code-enc-v1")`), never
  plaintext and never a bare hash. Encryption (reversible) rather than a
  hash because: the domain's `PairingCode` value object must be
  reconstructable on rehydration to run its own constant-time `Equal`
  (`domain-device-pairing.md` FR-6), and `GET /api/v1/network/pair/{id}/qr`
  must re-render the QR after a host page reload. A hash would break both.
  The ≤5-minute TTL, single-use rule, per-IP rate limit, and `Origin`
  check remain the real protection; encryption-at-rest keeps a DB-read
  attacker (leaked backup, SQL injection elsewhere) from harvesting live
  codes in that window. `pair/verify` looks the row up by a **blind index**
  (HMAC of the normalized code under a separate subkey) so the lookup is
  not a plaintext scan, then decrypts and runs the domain's constant-time
  compare.
- The new device submits the code to `POST /api/v1/network/pair/verify`.
  On success it receives a short-lived **enrolment grant** — not a
  session:
  - the grant is an HS256 token signed with a **distinct HKDF subkey**
    (`DeriveSubkey("enrolment-grant-v1")`), *not* the access-token key.
    A distinct key makes it structurally impossible for the grant to be
    accepted on the access path — the phase-12 defect where an MFA ticket
    passed as a bearer token (audit `0012`-C2) is not repeated here.
  - it carries `{ typ: "enrol", sid: <PairingSessionID>, jti, exp, iat,
    iss }` — **no `UserID`, no role**. TTL ≤ 10 minutes (widened from the
    draft's 2 minutes — the whole point is "type your password on a TV
    keyboard", which is slow) and **single-use**: the `jti` is recorded
    when the grant is spent at login, and a replay is rejected.
  - the device's sign-in screen presents the grant alongside credentials
    to `POST /api/v1/auth/login`; the server verifies the grant, then
    authenticates the user normally, then associates the device.
- **Device ownership is assigned at login, not at pairing initiation.**
  The `PairedDevice` row is created (or its `owner_id` set) when a user
  **completes `/auth/login` with the grant** — the device belongs to
  *that* user. The initiating admin is not an owner of it. If the grant's
  session was `InitiatedBy` a different user than the one now logging in,
  that is fine — a reader can be paired by an admin and then sign in as
  themselves; the device is the reader's. The row records `owner_id`
  (the authenticating user), a coarse `DeviceClass` derived from the
  `User-Agent` (never the raw string), `EnrolledVia = pairing_code`, and
  timestamps, for phase 14's device inventory to build on.
- `DEVICE_PAIRING_SECRET` (config) is an optional operator-set value that,
  when present, `pair/initiate` additionally requires. Its value is
  **defence in depth, and narrow**: a leaked admin token already grants
  full control of libraries, members, sources, and settings, so gating
  only `pair/initiate` behind the extra secret does not contain that
  breach — it does raise the bar for a specific "admin token seen, host
  console not controlled" case, and it lets an operator who wants pairing
  to require a shared out-of-band value have that. It is **not** a login
  passphrase and is never accepted in place of a user credential.

### 7. Authentication cannot be disabled

There is no configuration key, domain state, or UI control that turns
authentication off for any reachable bind. Constitution §6 admits no
exception and neither does this system. The design reference's "Require
authentication" toggle is not built; the Network panel states plainly
that authentication is always on (Spec D).

### 8. Runtime-mutable settings are limited to genuinely safe values

`PATCH /api/v1/network/settings` may change the advertised mDNS name and
the "remember this device" duration (decision 10). `BIND_ADDRESS`,
`TLS_CERT_FILE` / `TLS_KEY_FILE`, `ACME_DOMAIN`, and `CORS_ALLOWED_ORIGINS`
are resolved once at startup (`backend-configuration.md`'s Non-goals: no
runtime-mutable configuration) and changing them is a config-file edit
plus a restart. The UI shows these values and a line saying a restart is
required. A network-reconfiguration primitive reachable over the network
is exactly the kind of privileged surface not to expose. The handler's
accept-list is exactly two keys; a body carrying `bindAddress`,
`authRequired`, or any unknown key is a `400` naming the rejected key —
this is the structural guarantee that decisions 7 and 8 have no runtime
bypass.

### 9. Security response headers and an app-origin CSP, on every bind

Phase 13 is the first time the SPA is served to anything but a loopback
client, and it is currently served with **no** `Content-Security-Policy`,
`X-Frame-Options`, `X-Content-Type-Options`, or `Referrer-Policy` (the
only CSP in the codebase is on the phase-11 reader iframe). A
security-headers middleware applies to **every** response on **every**
bind (plaintext included — clickjacking and content-sniffing do not
require TLS):

- `Content-Security-Policy` for the app origin:
  `default-src 'self'; object-src 'none'; base-uri 'none';
  frame-ancestors 'none'; form-action 'self'` — with `script-src`,
  `style-src`, `img-src`, `connect-src`, `font-src` tuned to exactly what
  the built SPA needs (no `unsafe-inline` on `script-src`; the Vite build
  is configured to emit no inline scripts, or to use nonces/hashes).
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY` (belt-and-braces with `frame-ancestors 'none'`)
- `Referrer-Policy: no-referrer`
- `Permissions-Policy` denying every feature the app does not use
  (camera, microphone, geolocation, USB, …).

This closes: clickjacking of authenticated actions (a hostile page framing
the SPA and tricking a logged-in victim into a destructive click — the
Bearer-not-cookie property does **not** help here, since the framed app
attaches its own `localStorage` token); and XSS → token theft has a
real backstop for the first time (`localStorage` token exposure is ADR
0025's accepted risk, but "accepted" should not mean "undefended"). The
reader iframe keeps its own stricter CSP (ADR 0024) — this is the
*outer* app document's policy, which did not exist.

The phase-13 security audit **establishes** this CSP (it does not
"re-confirm" an existing one — the `DRAFT` spec's wording was wrong) and
verifies it is present on every response path including the SPA fallback
and error responses.

### 10. `rememberDeviceDays` is the one setting that changes a token-issuance parameter

`PATCH /api/v1/network/settings`'s `rememberDeviceDays` (1–90, default 30)
overrides ADR 0025's fixed 30-day refresh-token lifetime **at issuance
time**: when a session is created (login, or refresh rotation), the new
refresh token's `expires_at` is `now + rememberDeviceDays`. Existing
tokens are not retroactively changed. This is a deliberate, bounded
exception to decision 8's "runtime settings don't change security
parameters" — it is the "remember this browser for N days" control the
`atConnect` screen draws, it is range-checked, and a shorter value only
tightens security. ADR 0025 is amended to note the lifetime is
operator-configurable within `[1, 90]` days rather than fixed at 30;
`backend-authentication.md` FR-3/FR-4 carry the reference. A test proves a
changed value changes the next issued token's `expires_at`.

## Options considered

### Option A — a `TLS_ENABLED` / `remote mode` flag (rejected)

*For* — one boolean is simpler to reason about than address classification
plus certificate validation.

*Against* — this is ADR 0017 Option B under a new name, rejected there for
the same reason: a flag is a claim about the topology that can be wrong,
stale, or copied between environments. The resolved bind address plus the
certificate material actually on disk is a property of the running system,
and it fails closed. Adding the flag back would reintroduce exactly the
failure mode ADR 0017 removed.

### Option B — cookie-based sessions for browser clients, keeping Bearer for native (rejected for phase 13)

*For* — `HttpOnly` cookies are not reachable from JavaScript, which would
close the `localStorage`-XSS exposure ADR 0025 accepts; it would also make
the roadmap's "session cookie attributes" line item real.

*Against* — it re-opens ADR 0025 mid-phase, doubles the session code
paths, and pulls CSRF token machinery back in for the cookie surface. The
`localStorage` exposure is real but is phase 12's accepted trade-off and
is better revisited deliberately (phase 16 hardening) than as a side
effect of opening the bind. Phase 13 keeps one session mechanism.

### Option C — a shared "library passphrase" as the LAN credential (rejected)

*For* — it matches the `atConnect` screen as literally drawn; one secret
to distribute; no per-device account setup.

*Against* — one secret means no per-user attribution (RBAC and
multi-library membership both require a `UserID`), no clean revocation
(rotating the passphrase logs every device out), and a single
brute-forceable credential guarding the whole library. It is a weaker
model than the per-user accounts phase 12 already built. The pairing
bootstrap (decision 6) recovers the *convenience* the passphrase screen
was reaching for without giving up accountability.

### Option D — server-side QR image generation with `rsc.io/qr` (rejected)

*For* — a rendered PNG works before the SPA has loaded (the Electron
first-run screen).

*Against* — a new module dependency (constitution §9) for something the
client can do. The QR payload is a short string (URL + code); the React
client renders it with a small JS library already within the frontend
bundle budget, and the payload is a trivially contract-testable string
rather than an opaque image. If the first-run-before-SPA case turns out
to need it, that is a separate, justified addition then.

### Option E — DNS-01 ACME challenge instead of HTTP-01 (rejected)

*For* — DNS-01 can issue wildcard certificates and does not require an
inbound listener on port 80.

*Against* — DNS-01 requires API credentials for the operator's DNS
provider, a large and provider-specific configuration surface, and a new
class of secret for the server to hold. HTTP-01 needs only that the
domain resolves to the host and port 80 is reachable — which, for a
publicly routable bind, is already true. Wildcard issuance is not a
phase-13 need.

### Option F — store the pairing code as a bare SHA-256 hash (rejected)

*For* — a hash is irreversible, so a DB-read attacker gets nothing usable;
it is the reflex choice for anything credential-shaped.

*Against* — it breaks two things the design needs. The domain's
`PairingCode` value object runs its own constant-time `Equal` and must be
reconstructable when a `PairingSession` is rehydrated from the row
(`domain-device-pairing.md` FR-6) — a hash cannot be turned back into the
code. And `GET /api/v1/network/pair/{id}/qr` must re-render the QR (which
embeds the code) after the host operator reloads the settings page — again
impossible from a hash. The `DRAFT` spec tried hash-only and the review
(`0050` finding 1) caught that the verify path and `/qr` were both
unbuildable against it. AES-256-GCM under a dedicated subkey (decision 6)
keeps the code recoverable for exactly these two internal uses while still
denying a DB-read attacker the plaintext, and the code's ≤5-minute
single-use lifetime plus the rate limiter is what actually bounds
guessing — not the storage format.

## Consequences

**Good** — one session mechanism, one place the bind decision is made
(address + cert state, fail-closed), no new Go module, per-user
accountability preserved for every client, an app-origin CSP where none
existed, and a CORS/CSRF posture that is honest about what the
Bearer-token design does and does not need. The `autocert` path gives real
automated certificates for a user-operated remote deployment without
Alexandryn operating any infrastructure (constitution §6, ADR 0017).
Opt-in private-bind TLS means a LAN self-hoster is not forced to run a
reverse proxy to encrypt local traffic.

**Bad** — more transport code than a loopback-only server: up to two
listeners on a public bind (`:80` redirect/ACME, `:443` app), an
`autocert.Manager` to wire and shut down cleanly, CORS / `Origin` /
security-headers middlewares, encrypted-at-rest pairing codes with two
subkeys and a blind index, and Pebble in CI. The `localStorage`-XSS
exposure from ADR 0025 is still carried forward — but decision 9 now gives
it a CSP backstop, so "accepted risk" no longer means "undefended";
full mitigation (a move off `localStorage`) stays a phase-16 item. The
static-certificate SAN check and the new "DNS name ⇒ Mode A" rule are new
startup failure modes that need clear messages (constitution §11). The
`rememberDeviceDays` exception (decision 10) means ADR 0025's
refresh-token lifetime is no longer a single fixed number — a small
increase in the surface a reviewer must reason about.

**Neutral** — the roadmap's "session cookie attributes" line item is
resolved as *not applicable* rather than *implemented*; the phase-13
exit-criteria walk records why. The "Require authentication" and "Allow
access from this network" toggles from the design reference are resolved
as *not built* (read-only statements instead).

## Reversal cost

**Medium.** Decisions 1–3 (bind derivation, ACME, cipher policy) change a
validated startup path and add a CI service; reversing means re-tightening
`validateBindAddress` and removing listener-selection logic real remote
deployments would depend on. Decisions 4, 5, 9 (CORS, CSRF, headers) are
middleware and cheap to change. Decision 6 (pairing model, encrypted-code
storage, subkeys) is the expensive one to reverse after phase 14 builds a
device inventory and after codes are persisted under a versioned subkey;
low cost until then. Decisions 7 and 10 touch ADR 0025 / constitution §6
and are not reversible without amending those.

## Confidence

**High** on decisions 1, 2, 3, 4, 7, 9 — they follow directly from ADR
0017, constitution §6/§9, Go's TLS defaults, and standard web security
headers. **Medium-high** on decision 5 (CSRF): the Bearer-not-cookie
reasoning is sound and was independently confirmed in review `0050`;
the `Origin` allowed-set enumeration is the part most likely to need a
tweak once a real multi-interface deployment is tested. **Medium** on
decision 6: the pairing bootstrap is a new flow with no captured design
behind its exact steps, so the edge cases (a code claimed but login
abandoned, two devices racing a code, a reader signing in on an
admin-initiated pairing) are worked out in `domain-device-pairing.md` and
`backend-network-api.md` (row-locked verify transaction) rather than
validated against a prototype. **Medium** on decision 10 — operator-tunable
token lifetime is a reasonable feature but it is the one place this ADR
loosens an ADR 0025 invariant, and 90 days is a judgement call on the
upper bound.
