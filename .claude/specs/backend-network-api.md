# Spec: Backend — network API (pairing, status, settings) and persistence

| | |
|---|---|
| **Status** | `VERIFIED` — implemented across Tiers 3–4, certified in Gate 2 security audit `0013`, and verified green by integration test suites. |
| **Phase** | `13-network-access` |
| **Author** | Claude (Sonnet 5) |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-05 |
| **Reviewed in** | [`0050`](../reviews/0050-phase13-spec-package-and-phase12-authz-review.md) (two independent agents) — Needs rework at review time, findings addressed |
| **Design reference** | The one **binding** ledger entry for this surface is `sgNetwork`'s "Advanced" disclosure row in `Alexandryn-Electron-Admin.dc.html` (**Binding, contents uncaptured** — `.design-reference/ANALYSIS.md` synced **2026-08-13**, classification pass **2026-08-17**): the endpoint shapes behind it are this spec's to define. The `atConnect`/`atAccess` (Web) pairing/session screens are **Binding**, outline only. **Not relied on as authority:** the first-run `fr4` step (`atFirstRun` is **Unclassified** in the ledger) and `sgDevices` (no ledger row). D-1 (passphrase vs. account login) → ADR 0028 §6; D-2 ("require authentication" toggle) → ADR 0028 §7 (no writable toggle); D-4 (device-management phase split) → roadmap + ADR 0028 §6, not `sgDevices`; D-5 ("allow access from this network" toggle) → ADR 0028 §8 (read-only statement). |

## Context

`domain-device-pairing.md` gives the pure pairing types;
`backend-network-transport.md` gives the listeners, TLS, rate limiting,
`crypto/rand` code generation, and the enrolment grant. This spec is the
HTTP surface a client and the host UI actually call, plus the two
PostgreSQL tables that persist pairing sessions and paired devices.

## Problem

No endpoints exist for: initiating a pairing, verifying a code, fetching
a QR payload, reading network status for the settings panel, or changing
the runtime-safe network settings. No tables persist a `PairingSession`
across the ≤ 5-minute window or a `PairedDevice` for phase 14 to build on.

## Goals

- Five endpoints under `/api/v1/network/*`, contract-tested, with
  `example:` blocks for every JSON response so `npm run mocks:gen-fixtures`
  produces frontend fixtures.
- Two migrations-added tables (`pairing_sessions`, `paired_devices`) with
  repositories using parameterised queries only.
- `pair/verify` consumes the session and writes the device row in **one
  transaction** (ADR 0021).
- `/network/status` returns what the `sgNetwork` panel needs and **leaks
  no secret, no private key path, no home-directory path, no
  `DATABASE_URL`** (constitution §8, `backend-http-transport.md` FR-5's
  precedent).
- The phase-12 login handler gains an optional enrolment-grant parameter
  so a paired device's first session is associated with its
  `PairedDevice` row.

## Non-goals

- The standing device inventory UI and its endpoints (list all, rename,
  per-device sessions) — **phase 14**. This spec ships
  `DELETE /api/v1/network/pair/{id}` to revoke a pairing the operator
  just made (plan C-3) and nothing more of device management.
- Transport concerns (TLS, CORS, rate limiting, `Origin` validation) —
  `backend-network-transport.md`.
- The pairing state machine and invariants — `domain-device-pairing.md`.
- Changing bind address / TLS cert / ACME domain at runtime — config-file
  + restart only (ADR 0028 §8). `PATCH /network/settings` does **not**
  touch them.
- mDNS advertisement. `/network/status` reports the configured name;
  whether anything broadcasts it is `backend-network-transport.md`'s open
  question.
- Invalidating a revoked device's phase-12 refresh tokens — the
  `device_id` cascade on `user_refresh_tokens` is phase 14
  (`domain-device-pairing.md` Open questions). Phase 13 records the
  revocation; the cascade comes later. **Flagged, not silently skipped.**

## User stories

- As **the host operator on the Network settings panel**, I want to press
  "Open on another device", see a QR and a short code, and have a new
  device join by scanning — without typing anything on that device except
  my password once.
- As **a new device**, I want to submit the code I see and be handed
  straight to a sign-in screen that already knows which host it is
  talking to.
- As **the settings panel**, I want one `GET /network/status` call that
  tells me whether the server is reachable beyond loopback, on what
  addresses, under which TLS mode — with nothing sensitive in the
  response.

## Functional requirements

- **FR-1** `POST /api/v1/network/pair/initiate` — **admin only**
  (`RequireRole(admin)`), rate-limited per `backend-network-transport.md`
  FR-8. When `DEVICE_PAIRING_SECRET` is configured, the request body MUST
  include `{ "secret": string }` matching it (constant-time compare) or
  the response is `403`. On success: create a `PairingSession` (TTL = 5
  min, `InitiatedBy` = the caller's `UserID`, code from
  `GeneratePairingCode`), persist it (code stored **encrypted at rest**,
  FR-7), and return `201` with `{ pairingId, code, payload, address,
  expiresAt }` where `payload` is the `connect` URL with the code (FR-3)
  and `address` is the server's LAN/public URL. The `code` is returned
  here **and** is re-fetchable by the initiating admin via
  `GET .../qr` (FR-3) until the session is terminal — a host operator who
  reloads the settings page must be able to re-display the QR. It is not
  reachable by any *other* principal or after the session ends.
- **FR-2** `POST /api/v1/network/pair/verify` — **unauthenticated**,
  `Origin`-validated and rate-limited by `backend-network-transport.md`
  FR-7/FR-8. Body: `{ "code": string, "label"?: string }`. The handler:
  1. constructs a `domain.PairingCode` from `code` — a malformed shape
     (FR-2 of `domain-device-pairing.md`: not 8 Crockford chars) → `400`
     `InvalidInput`, distinct from a valid-shape-but-unknown code.
  2. computes the **blind index** (HMAC of the normalized code under the
     `pairing-code-index-v1` subkey) and, in a transaction, does
     `SELECT … FROM pairing_sessions WHERE code_index = $1 AND state IN
     ('pending') AND expires_at > now() FOR UPDATE` — the **row lock** is
     mandatory (S-M1): without it two devices racing the same code both
     read `pending` and both bind a device. No match → generic `404`.
  3. decrypts the row's code ciphertext (`pairing-code-enc-v1` subkey),
     rehydrates the `PairingSession`, calls
     `session.Verify(now, submittedCode, newDeviceID)`. `ErrPairingCodeMismatch`
     (blind-index collision — vanishingly rare, defence in depth) or
     `ErrPairingExpired` → **generic `404` `"pairing code not
     recognised"`** — identical body for wrong / expired / never-existed,
     no oracle for an unauthenticated caller. (`GET .../qr`, which is
     admin-only, *does* expose the real terminal state to the host
     operator, who is not an attacker.)
  4. on success, still in the same transaction:
     `session.Consume(now)` + persist the session +
     insert a **`PairedDevice` with `Owner` left unset / provisional** —
     ownership is assigned at login (FR-9, ADR 0028 §6), not here. The
     row records `pairing_session_id`, `DeviceClass` (derived from
     `User-Agent`), `Label` (body value or an API default), `EnrolledVia
     = pairing_code`. An injected failure between `Consume` and the
     insert MUST roll both back (integration test).
  5. return `200` with `{ enrolmentGrant, address, hostName }` —
     `enrolmentGrant` from `backend-network-transport.md` FR-10, carrying
     the `PairingSessionID` and a fresh `jti`.
- **FR-3** `GET /api/v1/network/pair/{pairingId}/qr` — **admin only**,
  and only for a session **the caller initiated** (`initiated_by =
  caller` — a session another admin initiated → `404`, not `403`, no
  cross-admin enumeration). Returns `{ payload, address, code, expiresAt,
  state }`. `payload` is `<scheme>://<address>/connect?c=<code>` (scheme
  matches the bind mode). The code comes from decrypting the stored
  ciphertext (FR-7) — possible precisely because codes are encrypted, not
  hashed (ADR 0028 §6 / Option F). The client renders the QR from
  `payload` (ADR 0028 Option D — server returns **no image**). A `consumed` /
  `expired` session returns its terminal `state` with `payload` and
  `code` as empty strings, so the UI shows "this code was used / expired"
  and offers a new one.
- **FR-4** `GET /api/v1/network/status` — **authenticated**, response
  **scoped by role** (S-M4):
  - **`reader`** (or any non-admin): `{ reachability, tlsMode,
    authRequired: true, address }` — where `address` is the single URL
    the *caller's own request* arrived on (from the bind config + the
    matched server-side origin, never the client `Host` header). A reader
    — possibly a guest in one library — does not learn the host's other
    interfaces.
  - **`admin`**: the above plus `{ addresses: [{ scope, url }], hostName,
    acmeDomain }` — the full non-loopback interface list, the mDNS name,
    and (only when `tlsMode == "acme"`) the ACME domain.
  Neither response MUST include: any filesystem path, the certificate or
  its key, the ACME cache directory, **`ACME_EMAIL`**, `DATABASE_URL` or
  any substring, the `DEVICE_PAIRING_SECRET`, any FR-9a subkey, a raw
  driver error, or the process's home directory. `tlsMode` values:
  `"none"` (plaintext), `"static"` (in-process, file cert — public or
  private opt-in), `"acme"` (in-process, `autocert`). A test builds a
  `Config` carrying recognisable fake values for each forbidden item and
  asserts none appears in either response.
- **FR-5** `PATCH /api/v1/network/settings` — **admin only**. Accepts a
  partial `{ "hostName"?: string, "rememberDeviceDays"?: integer }`.
  `hostName` MUST be a valid single DNS label with an optional `.local`
  suffix: `^(?=.{1,63}$)[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.local)?$` — no
  leading/trailing hyphen (it is advertised via mDNS) — else `400`.
  `rememberDeviceDays` MUST be an integer `1..90` else `400`; its
  **consumer** is refresh-token issuance — `backend-authentication.md`
  FR-3/FR-4 (amended for ADR 0028 §10) read `network_settings.remember_device_days`
  when minting a refresh token and set `expires_at = now +
  rememberDeviceDays` instead of a fixed 30 days; existing tokens are
  unchanged. Any other key — `bindAddress`, `tlsCertFile`, `acmeDomain`,
  `authRequired`, or anything unknown → `400` `InvalidInput` naming the
  key; for the restart-only ones the message says the value is set in the
  config file and takes effect after a restart. Returns `200` with the
  full effective settings. Persisted in a one-row `network_settings`
  table.
- **FR-6** `DELETE /api/v1/network/pair/{pairingId}` — **admin only**,
  only for a session the caller initiated (`404` otherwise). If the
  session is non-terminal → expire it. If it is `consumed` → call
  `PairedDevice.Revoke(now)` on the device it produced. Returns `204`.
  **The only device-management operation in phase 13** (plan C-3);
  listing, renaming, per-device sessions are phase 14. Phase 14 also owns
  the refresh-token cascade — this endpoint revokes the `PairedDevice`
  row; it does **not** yet invalidate that device's outstanding phase-12
  refresh tokens (`domain-device-pairing.md` Open questions). Flagged in
  the response docs and the audit, not silently skipped.
- **FR-7** Migration `00010_phase13_network.sql` MUST create:
  - `pairing_sessions` — `id` (pk), `initiated_by` (fk `users` `ON
    DELETE CASCADE`), `code_ciphertext` (bytea — AES-256-GCM of the
    normalized code under `pairing-code-enc-v1`, with its nonce),
    `code_index` (bytea — HMAC of the normalized code under
    `pairing-code-index-v1`, `UNIQUE`, the blind lookup key for FR-2
    step 2), `state` (text, `CHECK` against the four `PairingState`
    values), `created_at`, `expires_at`, `device_id` (nullable),
    `initiator_ip` (inet, nullable — the admin's own IP for their
    "who paired what" view; deleted with the row by FR-8's sweep, so its
    retention is bounded to ≤ 24h past terminal state; not a device
    fingerprint, not covered by `domain-device-pairing.md` FR-10 which
    bounds the *domain types*). Index on `(state, expires_at)` for the
    sweep. **No plaintext or hashed code column.**
  - `paired_devices` — `id` (pk), `owner_id` (nullable fk `users` `ON
    DELETE CASCADE` — nullable only in the brief window between FR-2's
    insert and FR-9's ownership assignment; a sweep prunes any row still
    null after 1h), `label`, `device_class` (`CHECK` enum),
    `enrolled_via` (`CHECK` enum), `created_at`, `last_seen_at`,
    `revoked_at` (nullable), `pairing_session_id` (nullable fk
    `pairing_sessions` **`ON DELETE SET NULL`** — the FR-8 sweep deletes
    old sessions and must not cascade-delete the device rows phase 14
    builds on).
  - `enrolment_grant_jtis` — `jti` (pk), `spent_at` — records a grant as
    used so a replay is rejected (FR-9); rows older than the max grant
    TTL are pruned by the sweep.
  - `network_settings` — one row: `host_name`, `remember_device_days`,
    `updated_at`.
  All timestamps `timestamptz`. No column stores a raw code, a token
  value, or a `User-Agent` string (a derived `device_class` only).
- **FR-8** A background sweep (a ticker in the service lifecycle, or the
  job queue — either satisfies this) MUST: mark `pending` `pairing_sessions`
  past `expires_at` as `expired`; delete `expired`/`consumed` sessions
  older than 24h (this is what bounds `initiator_ip` retention); delete
  `paired_devices` rows still `owner_id IS NULL` after 1h (an abandoned
  `verify`); delete `enrolment_grant_jtis` older than the max grant TTL.
  It is not on any request path.
- **FR-9** The phase-12 `POST /api/v1/auth/login` handler MUST accept an
  optional `{ "enrolmentGrant"?: string }`. When present:
  1. verify the grant against the `enrolment-grant-v1` subkey
     (`backend-network-transport.md` FR-10) — wrong subkey / `typ` /
     expired / malformed → the grant is **ignored** (see below).
  2. check `enrolment_grant_jtis` for the grant's `jti`; already present
     → ignored (replay).
  3. resolve the `PairedDevice` via the grant's `sid` →
     `pairing_sessions.id` → `paired_devices.pairing_session_id`.
  4. **assign ownership**: set `paired_devices.owner_id` to the
     **now-authenticated user** — whoever just logged in owns the device
     (ADR 0028 §6). The `PairingSession.InitiatedBy` (the admin) is
     **not** consulted for ownership; an admin pairing a device for a
     reader who then signs in as themselves is the normal case and the
     device is the reader's. (There is no "still matches `InitiatedBy`,
     else create a new row" branch — the draft's version was unreachable
     and let a reader silently take an admin's device.)
  5. record the `jti` in `enrolment_grant_jtis` (single-use).
  An **invalid, expired, replayed, or otherwise unusable grant is
  ignored**: login still succeeds, no device association is made, and the
  fact is logged at `info` with the correlation ID. A `401` here would
  break login for a slow device, and the grant is a convenience, not an
  authentication factor — the login itself fully authenticates the user.
  `backend-authentication.md` carries the owning amendment to the login
  request/response contract.
- **FR-10** Every endpoint validates body size (inherited from
  `backend-http-transport.md` FR-2's 10 MiB cap — these bodies are tiny;
  a stricter per-route cap of 4 KiB is applied), rejects a non-JSON
  `Content-Type` with `415`, and returns `backend-errors-and-logging.md`
  FR-5's shared error shape for every non-2xx. No handler constructs its
  own error body.

## Non-functional requirements

- **Performance** — pairing is a rare, human-paced operation; no budget
  beyond "a single indexed row lookup". `/network/status` reads config +
  a cached interface list, no DB call. The FR-8 sweep runs off the request
  path.
- **Security** — see Security considerations.
- **Accessibility** — API layer; the panel/modal accessibility is
  `frontend-network-and-pairing.md`.
- **Reliability** — a crash between `Consume` and the device row insert is
  impossible: they are one transaction (FR-2). A crash after `verify`
  returns but before the device logs in leaves a `consumed` session and a
  `paired_devices` row with no associated session — harmless, swept in 24h,
  and the device can pair again.
- **Observability** — every pairing action logs at `info` with:
  correlation ID, `pairingId`, `deviceId` (once assigned), `initiated_by`
  / `owner_id`, and grant `jti` at spend time. **Never logged**: the
  code (plaintext or ciphertext or index), the enrolment grant string,
  the `DEVICE_PAIRING_SECRET`, any FR-9a subkey. `/network/status`
  responses are not logged as bodies.

## Domain model

No new domain types (they are `domain-device-pairing.md`'s). This spec
adds: repositories (`PairingSessionRepository`, `PairedDeviceRepository`,
`NetworkSettingsRepository`) as interfaces in `domain` per the existing
repository pattern (`domain/repository.go`), implemented in
`internal/persistence/postgres/`. Code storage is a persistence concern:
the repository encrypts the `PairingCode` (AES-256-GCM,
`pairing-code-enc-v1` subkey) and stores a blind HMAC index alongside
(`pairing-code-index-v1`); on read it decrypts and hands the domain a
reconstructed, validated `PairingCode` (ADR 0028 §6 / Option F — a hash
would break `PairingSession.Verify`'s own constant-time compare and the
`/qr` re-render). The encryption/HMAC live in the repository, not the
domain, which stays crypto-free.

## API and contracts

New paths in `api/openapi.yaml`, all under `/api/v1/network/`:

| Method | Path | Auth | Success | Errors |
|---|---|---|---|---|
| POST | `/network/pair/initiate` | admin (+secret) | `201` `{pairingId,code,payload,address,expiresAt}` | `403` bad/missing secret, `429` |
| POST | `/network/pair/verify` | none (Origin-checked) | `200` `{enrolmentGrant,address,hostName}` | `400` malformed code, `404` not recognised, `429`, `403` bad Origin |
| GET | `/network/pair/{pairingId}/qr` | admin | `200` `{payload,address,code,expiresAt,state}` | `404` not yours / unknown |
| DELETE | `/network/pair/{pairingId}` | admin | `204` | `404` |
| GET | `/network/status` | any authed | `200` — **role-scoped** (FR-4): `reader` gets `{reachability,tlsMode,authRequired,address}`; `admin` also gets `{addresses[],hostName,acmeDomain}` | — |
| PATCH | `/network/settings` | admin | `200` full settings | `400` unknown/invalid key |

Every response body gets an inline `example:` block (roadmap /
`npm run mocks:gen-fixtures` requirement). Contract tests in
`internal/testutil/contracttest/` assert each response validates against
its schema, including the `429`/`403`/`404` shapes reusing
`architecture-contracts.md` FR-5.

`POST /api/v1/auth/login` schema amended (FR-9) to add optional
`enrolmentGrant`.

## State transitions

Server-side, per `pairing_sessions.state`, mirrors
`domain-device-pairing.md`'s `PairingState` exactly — the repository
persists the domain aggregate's state, it does not define its own. The
only DB-level transition not driven by a domain method is FR-8's sweep
(`pending`/`verified` → `expired` by `expires_at`), which is the
persistence of `ExpireAt` applied in bulk.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `pair/initiate` without the configured secret | FR-1 | `403` generic | No session created; logged (not the secret) |
| `pair/verify` with a wrong / expired / never-existed code | FR-2 | `404` `"pairing code not recognised"` — identical for all three | No state leak; rate limiter counts the attempt |
| `pair/verify` race: two devices submit the same code concurrently | FR-2 step 2's `SELECT … FOR UPDATE` serialises them | Second device gets generic `404` | First transaction consumes the session and commits; the second re-reads and finds `state != 'pending'` → no match → `404`. **Without the row lock both would bind a device** — the lock is mandatory, and a concurrent-double-submit integration test proves it |
| Crash mid-`verify` transaction | Postgres rolls back the whole tx | Device retries, code still `pending` | No partial device row, no half-consumed session, no orphaned grant `jti` |
| `/network/status` when Postgres is down | Not a DB call — still answers | Correct status, `tlsMode` etc. | Unaffected (deliberately no DB dependency) |
| `PATCH /network/settings` with `bindAddress` in the body | FR-5 | `400` naming the key + "changes to the bind address require editing the config file and restarting" | Nothing persisted |
| Slow device: enrolment grant expired by the time it logs in | FR-9 | Login still succeeds; no device link | `info` log; device unassociated, can re-pair |
| `pairing_sessions` fills with abandoned rows | FR-8 sweep | — | Bulk-expired and pruned after 24h |
| Enormous / non-JSON body to any endpoint | FR-10 (4 KiB cap, `415`) | `400` / `415` shared error shape | Rejected before handler logic |

## Security considerations

- **Trust boundary** — `pair/verify` is the only unauthenticated
  state-changing endpoint in the whole system after phase 12. It is
  defended by: `Origin` validation (`backend-network-transport.md` FR-7),
  a 10/min rate limit (FR-8 there), a 4 KiB body cap, a strict
  `PairingCode` shape check (`domain`), a single-use ≤ 5-minute session,
  constant-time code comparison (`domain`), and a generic `404` that does
  not distinguish wrong / expired / absent.
- **Codes are stored encrypted at rest, not plaintext and not a bare
  hash (FR-7, ADR 0028 §6 / Option F).** `code_ciphertext` is AES-256-GCM
  of the normalized code under a dedicated HKDF subkey; `code_index` is an
  HMAC of the normalized code under a *separate* subkey, `UNIQUE`, used as
  the blind lookup key so `pair/verify` never scans plaintext. A read of
  the table (leaked backup, SQL injection elsewhere) yields neither the
  code nor a rainbow-table-able hash. Encryption rather than a hash
  because the value object must be reconstructable for
  `PairingSession.Verify`'s own constant-time compare and for the `/qr`
  re-render (a hash breaks both — the `DRAFT` spec tried hash-only and
  the review caught the verify path was unbuildable). `pair/verify` does
  `WHERE code_index = $1 AND state = 'pending' AND expires_at > now() FOR
  UPDATE`, decrypts, then still runs the domain's constant-time `Equal`
  as defence in depth against an HMAC collision or a normalization
  mismatch. The ≤5-minute TTL + single-use + rate limit is what bounds
  guessing; the storage format bounds a DB-read attacker in that window.
- **`pair/initiate` is admin-only + optional shared secret.** An admin
  token is required to mint a code at all; `DEVICE_PAIRING_SECRET` adds a
  factor the host console holds even if an admin token leaks (ADR 0028
  §6). Constant-time compare, redacted config type, never logged.
- **No cross-admin enumeration.** `GET .../qr` and `DELETE` on a session
  another admin initiated return `404`, not `403` — an admin cannot probe
  which pairing IDs exist for other admins.
- **`/network/status` scrubbing (FR-4)** — the explicit never-include
  list restates `backend-http-transport.md` FR-5's `/readyz` precedent:
  a status endpoint names *that* something is configured, never *what*
  the secret or path is. `addresses` is composed server-side from bind
  config + OS interface enumeration, never from the request `Host` header
  (which a client controls and could use to make the panel display an
  attacker URL).
- **`PATCH /network/settings` allowlist (FR-5)** — an explicit accept-list
  of two fields; every other key including `authRequired` is rejected.
  This is the structural guarantee that ADR 0028 §7's "authentication
  cannot be disabled" has no runtime bypass, and §8's "bind is
  restart-only" has no runtime bypass.
- **Migration parameterisation (FR-7)** — every repository query is
  parameterised; `scripts/check-parameterized-queries.sh` covers the new
  files. No string-built SQL.
- **Enrolment-grant ignore-on-invalid (FR-9)** — a `401` on a bad grant
  would let a stale grant break an otherwise valid login; ignoring it
  (login succeeds, no device link) is fail-safe and cannot be used to
  bypass anything (the login itself still fully authenticates).
- **Phase-13 close gate — the phase-12 authorization hardening prelude
  (review `0050`, audit `0012`-C1/C2/P12-4).** Opening the bind on top of
  the phase-12 API is unsafe until:
  1. **Every reading/reader endpoint** (`progress`, `bookmarks`,
     `highlights`, `preferences`, `export`, `reader/content`) enforces
     `user_id` = the authenticated user **and** `library_id` = the
     validated active library at the **query layer** — the handler calls
     the `…AndUser` repository method, not the bare-ID variant.
     Per-endpoint IDOR tests (user A cannot read/write/delete user B's row
     by ID) and an `export` isolation integration test.
     `backend-reading-api.md` / `backend-reader-content.md` own these FRs;
     `scripts/check-user-scoped-reading.sh` enforces the pattern in CI.
  2. **`AuthMiddleware` validates `X-Library-Id` against
     `claims.Libraries`** — a request whose header is not in the token's
     library list → `403`. (`backend-library-namespaces.md` owns the FR;
     `backend-network-transport.md` FR-13 carries the middleware change.)
  3. **`AuthMiddleware` asserts the access-token `typ`** — an MFA ticket
     or an enrolment grant is rejected on the access path even with a
     valid signature. (`backend-authentication.md` FR-7 owns the FR.)
  Phase 13's exit criteria (`roadmap/13-network-access/README.md`) do not
  pass until all three are verified; audit `0013` re-verifies
  AUDIT-0012-C1/C2 as fixed.
- **STRIDE** — *Spoofing*: admin-gated initiation, `Origin` check on
  verify, generic 404, distinct signing subkey per token purpose.
  *Tampering*: parameterised SQL, row-locked one-transaction verify,
  checked-enum columns. *Repudiation*: every pairing action logged with
  IDs + correlation ID. *Information disclosure*: encrypted-at-rest codes
  + blind index, role-scoped status, no cross-admin enumeration, no
  `User-Agent` stored, `ACME_EMAIL` on the never-include list. *Denial of
  service*: rate limits (transport spec), 4 KiB body cap, FR-8 sweep
  bounds the table. *Elevation of privilege*: pairing yields an enrolment
  grant (no authority, distinct key); the resulting session is
  exactly the account that logged in; `X-Library-Id` claim check closes
  the horizontal path.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit (handlers) | Each endpoint: happy path; auth/role gates (reader → 403 on admin routes); `pair/initiate` secret required/absent/wrong; `pair/verify` malformed code → 400, wrong/expired/never-existed → identical generic 404, success → 200 + grant; `/network/status` **role-scoped** (reader gets the reduced shape, admin the full one) and never contains any string from the never-include list — table-driven with a `Config` carrying a fake cert path, fake `DATABASE_URL`, fake `DEVICE_PAIRING_SECRET`, fake `ACME_EMAIL`, fake subkeys; `PATCH /settings` accepts the two fields, rejects `bindAddress`/`authRequired`/unknown with a naming error; `DELETE` revokes a consumed pairing's device and 404s a foreign one; `/qr` re-fetch by the initiating admin works and returns `404` for another admin's session; enrolment-grant replay (same `jti` twice) → second login makes no device link. `X-Library-Id` not in claims → 403; MFA-ticket / enrolment-grant presented as a bearer token → 401 (FR-13). |
| Integration | `-tags=integration`. Migration `00010` applies and rolls back, incl. the `ON DELETE SET NULL` on `paired_devices.pairing_session_id`. Repository CRUD for all four tables, parameterised; code round-trips through encrypt→store→decrypt and the blind index matches. **`pair/verify` atomicity**: inject a failure between `Consume` and the device insert, assert full rollback (code still `pending`, no device row, no `jti`) — the `source_removal_atomicity_integration_test.go` pattern. **`pair/verify` concurrent double-submit**: two goroutines, same code — exactly one binds a device, the other gets `404` (proves the `FOR UPDATE` lock). FR-8 sweep: seed expired + old-consumed + null-owner + stale-`jti` rows, run, assert transitions and deletions. FR-9: login with a valid grant links the device to the **logging-in user** (not the initiating admin); expired/replayed grant → login succeeds, no link. **Reading-API IDOR** (close gate): user A cannot read/write/delete user B's progress/bookmark/highlight by ID; `GET /api/v1/reading/export` returns only the caller's rows. `rememberDeviceDays` change → next issued refresh token's `expires_at` reflects it. |
| Contract | Every response validates against `api/openapi.yaml`; `example:` blocks present and schema-valid; `npm run mocks:gen-fixtures` produces fixtures without error; `429`/`403`/`404`/`415` bodies match FR-5. |
| E2E | The full pairing flow is in `frontend-network-and-pairing.md`'s Playwright test (initiate on host → scan/enter code → verify → login → library). |
| Accessibility | N/A at this layer. |

Must fail before implementation: the `pair/verify` generic-404 test
(no endpoint exists), the `/network/status` never-include-list test, the
`pair/verify` atomicity integration test, the `X-Library-Id` claim-check
403 test.

## Acceptance criteria

- [ ] `POST /network/pair/initiate` (admin) returns a one-time code +
      payload; the code is re-fetchable via `/qr` by the initiating admin
      until terminal, and by no other principal or after the session
      ends.
- [ ] `POST /network/pair/verify` returns a byte-identical generic `404`
      for a wrong, an expired, and a never-existed code.
- [ ] A malformed code shape returns `400` `InvalidInput`, not `404`.
- [ ] `pair/verify` consumes the session and writes the `paired_devices`
      row in one row-locked transaction; an injected mid-transaction
      failure rolls both back; a concurrent double-submit binds exactly
      one device — all proven by integration tests.
- [ ] The pairing code is stored encrypted (never plaintext, never a bare
      hash); the code round-trips encrypt→store→decrypt and the blind
      index matches — proven by a repository test.
- [ ] `GET /network/status` is role-scoped and contains none of: a
      filesystem path, the cert/key, the ACME cache dir, `ACME_EMAIL`,
      `DATABASE_URL` or a substring, the pairing secret, a signing subkey
      — proven with a `Config` carrying recognisable values for each.
- [ ] `PATCH /network/settings` changes only `hostName` and
      `rememberDeviceDays`; `bindAddress`, `authRequired`, and unknown
      keys are each rejected `400` naming the key (restart-only ones say
      so). A changed `rememberDeviceDays` changes the next issued refresh
      token's `expires_at`.
- [ ] `authRequired` in `/network/status` is always `true` and no code
      path writes it — proven by the `PATCH` rejection test and a grep.
- [ ] `DELETE /network/pair/{id}` revokes the device a consumed pairing
      produced; a foreign `pairingId` returns `404`.
- [ ] Enrolment grant: single-use (`jti` replay → no second device
      link); ownership goes to the logging-in user, not the initiating
      admin; an invalid grant does not fail the login.
- [ ] **Close gate — phase-12 hardening:** every reading/reader endpoint
      enforces per-user + per-library scoping at the query layer, proven
      by a per-endpoint IDOR test and an `export` isolation test;
      `AuthMiddleware` rejects `X-Library-Id` not in `claims.Libraries`
      (`403`) and a wrong-`typ` token (`401`).
- [ ] Migration `00010` applies and rolls back cleanly (incl.
      `ON DELETE SET NULL`); every new query is parameterised
      (`check-parameterized-queries.sh` clean).
- [ ] The FR-8 sweep expires/prunes stale sessions, null-owner devices,
      and stale grant `jti`s, and never runs on a request path.
- [ ] Every response has an `example:` block and `npm run mocks:gen-fixtures`
      succeeds.
- [ ] Every FR maps to a line in phase 13's exit criteria.

## Open questions

- **`410 Gone` vs generic `404` for an expired-but-real code — resolved
  (review `0050`).** `pair/verify` (unauthenticated, attacker-facing)
  returns a byte-identical generic `404` for wrong / expired /
  never-existed — no oracle. The **admin-only** `GET .../qr` exposes the
  real terminal `state` to the host operator, who is not an attacker, so
  the settings UI can say "this code expired, generate a new one". No
  `410` anywhere.
- **Where `network_settings` lives.** A dedicated one-row table is the
  simplest thing; if a general application-settings store lands in
  phase 14/15 this folds into it. Not blocking.
- **The FR-8 sweep mechanism** — job queue vs. lifecycle ticker. The job
  queue (`backend-job-queue.md`) is built and idiomatic; a 5-minute
  ticker is less machinery for a trivial task. Pick at implementation;
  either satisfies the FR.
- **Label derivation from `User-Agent`.** `domain-device-pairing.md`
  deferred this here. Proposal: a tiny allow-list mapper
  (`"CriOS"|"Chrome" → "Chrome"`, `"iPhone" → "iPhone"`, …) producing
  `"iPhone · Safari"`-style labels, `"Unknown device"` fallback; never
  store the raw UA. Confirm the mapping is small enough to not be a
  parsing liability at scope review.

## References

- ADR 0028 §6 (pairing model), §7 (no auth-off — FR-4/FR-5 enforce it),
  §8 (runtime settings scope)
- ADR 0021 — the transaction contract FR-2's atomic verify relies on
- ADR 0025 — the login flow FR-9 extends
- `domain-device-pairing.md` — every pairing type and method
- `backend-network-transport.md` — `GeneratePairingCode`, the enrolment
  grant (FR-10 there), `Origin` validation, rate limiting
- `backend-authentication.md` FR-7 (`IsPublicPath`, extended), the login
  handler FR-9 amends
- `backend-http-transport.md` FR-2 (body cap), FR-5 (health-response
  scrubbing precedent this spec's `/network/status` follows)
- `backend-errors-and-logging.md` FR-5 (shared error shape)
- `architecture-contracts.md` FR-3 (contract tests), FR-5 (error shape)
- `internal/persistence/postgres/source_removal_atomicity_integration_test.go`
  — the atomicity test pattern FR-2 follows
- Constitution §3, §4, §6, §8, §11, §12
