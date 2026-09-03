# Implementation Plan: Phase 13 — Network access

## Overview

Open the Alexandryn host beyond loopback for the first time, under ADR 0017's
two fail-closed modes: private-only bind (Mode B, upstream/opt-in TLS) or
publicly routable bind (Mode A, in-process TLS via static cert or ACME).
Adds device pairing as a thin bootstrap over phase-12 accounts, CORS
deny-by-default, an app-origin CSP + security headers, `Origin` validation
on the one unauthenticated route, and a global rate limiter for public
surfaces. Authentication is never disable-able (constitution §6, ADR 0028
§7).

Authoritative scope: `.claude/roadmap/13-network-access/README.md`
(Scope In/Out, Exit criteria). Gate-1 decision record + finding→resolution
map: `.claude/roadmap/13-network-access/implementation-plan.md` (§1–§7,
kept as the audit trail; this file replaces its §8 tiered sketch with the
executable breakdown). Specs (`APPROVED`): `domain-device-pairing.md`,
`backend-network-transport.md`, `backend-network-api.md`,
`frontend-network-and-pairing.md`. ADR 0028, ADR 0017.

Task checklist: `tasks/todo-phase13-network.md`. RED → GREEN → Refactor per
FR, test before code (constitution §2). **Ping the maintainer before each
tier's RED step** (plan directive; TDD skill).

## Current state (what's already built)

| Area | State | Files |
|---|---|---|
| `BIND_ADDRESS` classifier | **Tier 0 in progress, uncommitted** on `feat/phase13-tier0-config` (8 files). Classifies loopback/private/public without DNS; accepts a valid static-cert public bind and a private opt-in cert; rejects `ACME_ENABLED` on a public bind ("Tier 2"); `Config.TLSCertificate()` returns the validated `*tls.Certificate`. | `internal/config/bindaddress.go`, `config.go` |
| Six phase-13 config keys | **In tree, uncommitted**: `ACME_ENABLED/DOMAIN/EMAIL/CACHE_DIR`, `CORS_ALLOWED_ORIGINS` (validated origin list), `DEVICE_PAIRING_SECRET` (`RedactedString`). | `internal/config/config.go` |
| `run.go` TLS wrap | **In tree, uncommitted**: `tls.NewListener` when `cfg.TLSCertificate() != nil` (minimal `tls.Config` — policy is Tier 2). | `cmd/server/run.go` |
| Phase-12 authz hardening prelude | **Landed** on branch: `JWTSigner.VerifyAccessToken` asserts token type (AUDIT-0012-C2); `AuthMiddleware` validates `X-Library-Id` against `claims.Libraries` (`libraryInClaims`, P12-4); `reading.go` calls `…AndUser` scoped repo methods (AUDIT-0012-C1). Tier 4 re-verifies with IDOR tests; audit 0013 certifies. | `internal/auth/jwt.go`, `internal/transport/http/auth_middleware.go`, `reading.go` |
| HTTP server | `NewServer` + timeouts + graceful shutdown + `/healthz` `/readyz` + embedded SPA. `ListenAndServe` only — no `ServeTLS`, no `:80`, no ACME. | `internal/transport/http/server.go`, `static.go`, `health.go` |
| Middleware chain | `Chain(handler, mw...)`; Recovery (outermost) → limits → logging → auth → routing. No CORS, CSP, Origin, global rate-limit. | `internal/transport/http/middleware.go`, `limits.go`, `auth_middleware.go` |
| Rate limiter | `auth.IPRateLimiter` — per-IP token bucket over `golang.org/x/time/rate`, TTL eviction, `Allow(ip)` / `Cleanup(now)`. Auth endpoints only. Reusable. | `internal/auth/ratelimit.go` |
| Domain | No pairing types. `UserID`/`LibraryID`/`Role` exist; `rehydrate.go`/`id.go` patterns established. | `internal/domain/` |
| Persistence | pgx pool, goose migrations latest `00009`. No pairing tables. `scripts/check-parameterized-queries.sh` active. | `internal/persistence/postgres/`, `migrations/` |
| Frontend | `hostOnly()` gate exists; `settings`/`system` gated; `access`/`connect` are `ScreenPlaceholder`s. `web/src/data/http.ts` attaches the token. No `qrcode` dep. | `web/src/app/routes.tsx`, `web/src/data/` |
| `golang.org/x/crypto` | Direct dep (Argon2id) → `acme/autocert` is a sub-package, **no new Go module**. `golang.org/x/time` direct. | `go.mod` |

## Architecture Decisions (locked, 2026-09-03 maintainer)

- **D-A — first-run flow held.** The new `Alexandryn Setup.dc.html` canvas
  (LAN config + domain verification) is **not** built in phase 13. Its own
  spec + ADR come after phase 13 closes. `atFirstRun` stays Unclassified.
  Phase 13 builds the standing Settings → Network panel + pairing only; the
  first-run flow will reuse the `NetworkSettings` component, not fork it.
- **D-B — LAN bind = the detected private interface IP, fail-closed.** No
  all-interfaces (`0.0.0.0`/`::`) carve-out in the classifier: `0.0.0.0`
  classifies **public** (current Tier 0 code already does this — `net.IP`
  is non-nil, not loopback, not private), so it demands a cert and does not
  bind plaintext-everywhere. Writing the specific private IP is the
  **first-run flow's** job (D-A, deferred). The ADR for that flow must
  state what happens on a multi-interface host and on a DHCP/network change
  — "rebind on change" (small design) vs "user reconfigures". **Phase 13
  action:** a classifier test asserting `0.0.0.0:8080` and `[::]:8080`
  classify public; nothing else.
- **D-C — domain-mode reachability = ACME-as-proof.** No third-party echo
  service, no Alexandryn-operated infrastructure (§6). A completed ACME
  HTTP-01 challenge is the evidence of inbound reachability. **Caveat to
  carry into the first-run ADR:** HTTP-01 proves reachability on port 80
  (the challenge listener), not necessarily port 443 / the serving port; if
  they differ the flow must say "reachability on the challenge port"
  rather than implying full proof. **Phase 13 action:** none — phase 13
  builds the `autocert` HTTP-01 path (Tier 2); the precondition-checker UI
  is the deferred first-run flow.
- **D-D — menu-bar / background hosting deferred.** `architecture-desktop-host.md`
  FR-11 (close-means-quit, v1) stands unchanged. The Setup canvas's "Keep
  hosting when the window is closed" toggle is not built and not decided
  under phase 13.
- **D-E — `qrcode` npm dependency: approved in principle**, §9 record
  brought at Tier 5 (what it does / why not stdlib / abandonment risk) with
  a maintained-library check and the `check:bundle-size` headroom result,
  before `frontend-network-and-pairing.md` Tier-5 code merges. Fallback:
  vendor `qrcode-generator` (~4 KB), same component boundary.
- **D-F — no `TLS_ENABLED`, no `PORT` key** (ADR 0028 §1, plan C-0/C-8) —
  already honoured in Tier 0.
- **D-G — FR-8 sweep mechanism** (`backend-network-api.md` FR-8): 5-minute
  lifecycle ticker, not the job queue — less machinery for a trivial task.
  Pick confirmed at Tier 3; either satisfies the FR.

## Dependency graph

```
Tier 0  config keys + bind classifier + run.go TLS wrap
   │
   ├── Tier 1  pure pairing domain  (no I/O — parallel-safe with Tier 2)
   │      │
   ├── Tier 2  transport services: TLS policy, Listeners, ACME, CORS,
   │      │    CSP/headers, Origin, rate-limit, HKDF subkeys,
   │      │    GeneratePairingCode (needs Tier 1's PairingCode),
   │      │    enrolment grant, middleware chain assembly
   │      │
   │      └────────────┐
   │                   ▼
   └── Tier 3  persistence: migration 00010, 4 repos (encrypt + blind
              index, needs Tier 2's HKDF subkeys), row-locked atomic
              verify (needs Tier 1 Consume + Tier 3 repos), sweep
                   │
                   ▼
        Tier 4  HTTP API: 6 endpoints, /network/status scrubbing,
                login enrolmentGrant param, OpenAPI + contract tests,
                close-gate IDOR/X-Library-Id/token-type re-verification
                   │
                   ▼
        Tier 5  web UI: qrcode dep, data hooks, NetworkSettings,
                DevicePairingModal, /connect, /access, a11y, Playwright
                   │
                   ▼
        Gate 2  security audit 0013 (four-attacker + STRIDE),
                re-verify AUDIT-0012-C1/C2  ── STOP AND ASK ──
                   │
                   ▼
        Close   exit-criteria walk, phase-12 amendment re-confirmations,
                maintainer approval
```

## Task List

Full task bodies (acceptance criteria, verification, files, size) in
`tasks/todo-phase13-network.md`. Index:

### Tier 0 — config & bind (finish + commit)
- T0.1 — verify + commit the uncommitted six config keys and parsers
- T0.2 — bind classifier: finish tests incl. `0.0.0.0`/`[::]` → public (D-B); private opt-in cert; public static accept/reject matrix
- T0.3 — `run.go` in-process TLS listener wrap (verify against T0.2)
- **Checkpoint 0** — `go test ./internal/config/... ./cmd/server/...` green, `go vet`, import-boundary + parameterised-query lints clean; Tier 0 committed

### Tier 1 — pure pairing domain (`domain-device-pairing.md`)
- T1.1 — `PairingCode` value object (FR-1/FR-2): Crockford-base32 8-char shape, constant-time `Equal` only, no `==` path
- T1.2 — `PairingSession` aggregate (FR-3–FR-7): state machine `pending→verified→consumed`/`expired`, injected clock, `ttl ∈ (0, 5m]`, `Verify` no-burn-on-wrong-guess, single-call `Consume`
- T1.3 — `PairedDevice` entity (FR-8/FR-9): `Owner` set at login not initiation, coarse `DeviceClass`, `EnrolledVia`, `Revoke` idempotent-error, `Touch` forward-only
- T1.4 — rehydration constructors (`RehydratePairingSession`, `PairedDevice`) re-validating every invariant; import/field audit test (no `net`/`crypto`/`http`/UA/hardware id)
- **Checkpoint 1** — `go test ./internal/domain/...` green; illegal-transition table + constant-time-`Equal` structural test pass; grep proves no `==` on the code type

### Tier 2 — transport services (`backend-network-transport.md`)
- T2.1 — `tls.Config` builder (FR-4): `MinVersion` TLS 1.2, the six AEAD+ECDHE suites, `NextProtos ["h2","http/1.1"]`
- T2.2 — `transport.Listeners(cfg, handler)` (FR-3): listener selection by class (private plain / private+cert / public static / public ACME); the `:80` 308-redirect listener on every public bind; `warn` log on non-loopback plaintext
- T2.3 — ACME via `autocert` (FR-3): `Manager` with `HostWhitelist(ACMEDomain)`, `DirCache(ACMECacheDir)` at `0700`, `AcceptTOS`; **Pebble integration test** — issuance on first handshake + forced near-expiry renewal
- T2.4 — security-headers middleware (FR-5a): app-origin CSP (`frame-ancestors 'none'`, `default-src 'self'`, no `unsafe-inline` on `script-src`), `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy` — on every bind; HSTS middleware (FR-5) — TLS binds only, decided once at startup
- T2.5 — CORS middleware (FR-6): deny-by-default, exact-string-match echo, `Vary: Origin`, never `Allow-Credentials`
- T2.6 — `Origin` validation middleware (FR-7): startup-computed allowed set (every non-loopback interface addr + mDNS `hostName` + `CORS_ALLOWED_ORIGINS`, in the served scheme(s)); present-non-matching → 403; absent → allowed; `Referer` fallback only when `Origin` absent + body present
- T2.7 — global unauthenticated rate-limit middleware (FR-8): reuse/extract `IPRateLimiter`; `/healthz` `/readyz` static 60/min, `pair/verify` 10/min, `pair/initiate` 5/min; key on `RemoteAddr`, ignore `X-Forwarded-For`
- T2.8 — `run.go` HKDF subkeys (FR-9a): `pairing-code-enc-v1`, `pairing-code-index-v1`, `enrolment-grant-v1`; `GeneratePairingCode()` adapter (FR-9) — 5 `crypto/rand` bytes → Crockford → `domain.PairingCode`, propagate rand error, no `math/rand`
- T2.9 — enrolment grant in `internal/auth` (FR-10): `SignEnrolmentGrant`/`VerifyEnrolmentGrant`, HS256 under the distinct subkey, `{typ:"enrol", sid, jti, exp, iat, iss}`, TTL ≤ 10 min, single-use `jti`
- T2.10 — middleware chain assembly (amends `backend-http-transport.md` FR-1, `architecture-backend.md` FR-6): `recovery → limits → logging → security-headers → HSTS → CORS → global rate-limit → auth → Origin(pairing group) → routing`; FR-12 no-auth-off test (`IsPublicPath` gains only `pair/verify`); FR-13 re-verify (token-type + `X-Library-Id` middleware tests per token type)
- **Checkpoint 2** — `go test -race ./internal/config/... ./internal/auth/... ./internal/transport/...` + `-tags=integration` (static-cert `ServeTLS`, Pebble, `:80` redirect, graceful-shutdown-two-listeners) green; `math/rand`/header-keying audit tests pass. **Middleware-chain spec amendment (`backend-http-transport.md` FR-1 + `architecture-backend.md` FR-6, currently `DRAFT`) re-confirmed by maintainer before this checkpoint closes.**

### Tier 3 — persistence (`backend-network-api.md` FR-7/FR-8)
- T3.1 — migration `00010_phase13_network.sql`: `pairing_sessions` (`code_ciphertext` bytea, `code_index` bytea UNIQUE, `state` CHECK, `initiator_ip` inet, index `(state, expires_at)`), `paired_devices` (`owner_id` nullable fk, `device_class`/`enrolled_via` CHECK, `pairing_session_id` fk `ON DELETE SET NULL`), `enrolment_grant_jtis` (`jti` pk, `spent_at`), `network_settings` (one row). Applies + rolls back.
- T3.2 — `PairingSessionRepository` (interface in `domain`, impl in `postgres`): encrypt code AES-256-GCM under `pairing-code-enc-v1`, store blind HMAC index under `pairing-code-index-v1`, decrypt + rehydrate a validated `PairingCode` on read; parameterised queries only
- T3.3 — `PairedDeviceRepository` + `NetworkSettingsRepository` + `enrolment_grant_jtis` store; CRUD integration tests, parameterised
- T3.4 — row-locked atomic verify (FR-2, ADR 0021): `SELECT … WHERE code_index=$1 AND state='pending' AND expires_at>now() FOR UPDATE` → decrypt → `session.Verify` → `Consume` + insert `PairedDevice` in one tx; injected mid-tx failure rolls both back; concurrent double-submit binds exactly one device
- T3.5 — FR-8 sweep (D-G: 5-min lifecycle ticker): expire stale `pending`, delete terminal sessions > 24h, delete null-owner devices > 1h, delete stale grant `jti`s; not on any request path
- **Checkpoint 3** — `go test -race -tags=integration ./internal/persistence/...` green; migration up/down clean; atomicity + concurrent-double-submit + sweep tests pass; `check-parameterized-queries.sh` clean

### Tier 4 — HTTP API + contract (`backend-network-api.md`)
- T4.1 — `POST /network/pair/initiate` (admin, +optional `DEVICE_PAIRING_SECRET` constant-time) → `201 {pairingId,code,payload,address,expiresAt}`; rate-limited (T2.7 bucket)
- T4.2 — `POST /network/pair/verify` (unauth, `Origin`-checked, rate-limited): malformed shape → `400`; wrong/expired/never-existed → **byte-identical generic `404`**; success → `200 {enrolmentGrant,address,hostName}`
- T4.3 — `GET /network/pair/{id}/qr` (admin, initiator-only, `404` for another admin's) → `{payload,address,code,expiresAt,state}`; terminal state → empty `payload`/`code`
- T4.4 — `GET /network/status` (authed, **role-scoped**): `reader` → `{reachability,tlsMode,authRequired:true,address}`; `admin` → + `{addresses[],hostName,acmeDomain}`. Never any path / cert / key / ACME cache dir / `ACME_EMAIL` / `DATABASE_URL` substring / pairing secret / subkey — table test with fake recognisable values
- T4.5 — `PATCH /network/settings` (admin): accept-list is exactly `hostName` (DNS-label regex) + `rememberDeviceDays` (1–90); any other key incl. `authRequired`/`bindAddress` → `400` naming it (restart-only ones say so); persists to `network_settings`
- T4.6 — `DELETE /network/pair/{id}` (admin, initiator-only): non-terminal → expire; `consumed` → `PairedDevice.Revoke`; `204`. Flag (docs + audit): does **not** yet cascade the device's phase-12 refresh tokens — phase 14
- T4.7 — phase-12 `POST /api/v1/auth/login` gains optional `enrolmentGrant` (FR-9): verify under `enrolment-grant-v1`, check `jti` unspent, resolve `PairedDevice` via `sid`, assign `owner_id` = the authenticating user, record `jti`. Invalid/expired/replayed grant → **ignored**, login still succeeds, logged at `info`
- T4.8 — OpenAPI: new paths + `example:` block per response; `npm run mocks:gen-fixtures` succeeds; contract tests in `internal/testutil/contracttest/` incl. `429`/`403`/`404`/`415` shapes
- T4.9 — **close-gate re-verification**: per-endpoint reading-API IDOR tests (user A cannot read/write/delete user B's row by ID) + `GET /api/v1/reading/export` isolation integration test; `AuthMiddleware` `X-Library-Id`-not-in-claims → `403` and wrong-`typ` → `401` handler tests; `rememberDeviceDays` change → next issued refresh token `expires_at` reflects it
- **Checkpoint 4** — `go test -race ./... && go test -race -tags=integration ./...` green; contract suite green; `npm run mocks:gen-fixtures` clean; every close-gate test in the roadmap Exit criteria present and passing

### Tier 5 — web UI (`frontend-network-and-pairing.md`)
- T5.1 — `qrcode` dependency: §9 record + maintained-library check + `check:bundle-size` headroom → **maintainer sign-off (D-E)**; add dep, import only the matrix entry point
- T5.2 — `web/src/data/network.ts` TanStack Query hooks for the six endpoints; `login()` gains optional `enrolmentGrant`; MSW handlers seeded from generated fixtures
- T5.3 — `NetworkSettings.tsx` under `hostOnly('network','Network')`: status card + honest `tlsMode` copy (the four cases from FR-1), static "Authentication is always on" row (**no toggle** — §7), read-only reachability + Advanced disclosure ("configured/not configured", never a path), editable mDNS name + remember-days via `PATCH`
- T5.4 — `DevicePairingModal.tsx` (Radix `Dialog`): `initiate` on open, client-side QR from `payload`, `XXXX-XXXX` mono code, live countdown (`aria-live`, announce 60/30/10/0 only), Revoke + Escape → `DELETE`, "Done" → no delete, expiry → "Generate a new code"
- T5.5 — `/connect` route: `?c=` prefill then `history.replaceState` strip, code field auto-format, `verify` → navigate `/login` with grant in **router state not URL**, generic `404` + `429` copy
- T5.6 — `/access` route: reader-scoped status + fixed role-keyed capability list + library names from claims + sign-out; no `hostModes` cloud card
- T5.7 — routes + capability gate wiring (`routes.tsx` `network` under `hostOnly`, `connect`/`access` real); tokens-only styling; `@axe-core/playwright` zero violations on all three surfaces + modal; reduced-motion
- T5.8 — Playwright E2E: host initiates → second context opens `/connect?c=` → verify → `/login` → library; expired code → generic error; revoke → code no longer verifies
- **Checkpoint 5** — `npm test`, `npm run build`, `check:token-styling`/`check:a11y-*` clean, axe zero violations, Playwright pairing happy-path green

### Gate 2 — security audit `0013`  ── STOP AND ASK ──
- G2.1 — four-attacker + STRIDE pass over the whole phase (two independent subagents preferred, per `feedback_make_improve_review_fix_cycle`); record `.claude/audits/0013-phase13-network-access.md`, severities rated honestly (§10)
- G2.2 — re-verify AUDIT-0012-C1 (reading IDOR closed) and C2 (token-type asserted) as fixed, tracing handler → repo → SQL (not from the repo layer — CLAUDE.md Reflex / audit template)
- G2.3 — confirm no open Critical/High; **report to maintainer and wait** (constitution Review gates — do not cross on own judgement)

### Close
- C.1 — walk `roadmap/13-network-access/README.md` Exit criteria box by box, each citing real evidence
- C.2 — the six phase-12 spec amendments (`DRAFT`) re-confirmed one at a time in order: ADR 0025 → `backend-authentication.md` → `backend-library-namespaces.md` → `backend-reading-api.md` → `backend-reader-content.md` → `backend-configuration.md` (FR-8 interim note removed); + `architecture-backend.md` FR-6 / `backend-http-transport.md` FR-1 (done at Checkpoint 2)
- C.3 — maintainer approval recorded; phase 12 also needs its fuller independent authz re-audit before *it* closes (separate directive, `roadmap/12-authentication/README.md`)

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Uncommitted Tier 0 in the working tree (another session's) | Med | T0.1–T0.3 verify it compiles + all `internal/config` / `cmd/server` tests pass before anything builds on it; commit at Checkpoint 0 |
| Pebble as a new CI service container | Med | Tier 2's only new infra; model on the existing Postgres-in-integration pattern (`backend-network-transport.md` test strategy) — not a `go.mod` dep |
| Middleware-chain reorder breaks a phase ≤12 assumption | High | Only the auth slot expands; T2.10 has a full ordered-chain test + smoke-test of all six `domain.Error` categories end to end; the spec amendment is maintainer-re-confirmed first |
| `0.0.0.0` / multi-interface / DHCP change | Low (phase 13) | D-B: classifier keeps `0.0.0.0` → public → fail-closed; the real handling is the deferred first-run flow's ADR, flagged there |
| `qrcode` bundle budget | Low | D-E gate before Tier-5 merge; `qrcode-generator` (~4 KB) vendored fallback, same component boundary |
| `localStorage` token XSS (ADR 0025, carried) | Med | Not phase 13's to fix; T2.4's CSP is the backstop; audit 0013 re-confirms ADR 0024 sanitisation holds for LAN-served content |
| Timing-based tests (countdown, grace period, cert expiry) flaky in CI | Med | Injected clocks in domain + `FakeClock` in transport; Pebble renewal uses a forced near-expiry not a sleep; run repeatedly before trusting in gating |
| Encrypted-code repo layer (two subkeys + blind index) subtle | Med | T3.2 round-trips encrypt→store→decrypt and asserts the blind index matches; T3.4 keeps the domain constant-time compare as defence-in-depth after the index lookup |

## Open Questions (non-blocking — resolve at the tier that hits them)

- **mDNS / `alexandryn.local` advertisement owner** — Go server (needs a §9
  library) vs Electron host. `/network/status` reports the name regardless.
  Deferred to `backend-network-transport.md`'s own follow-up; phase 13 does
  not advertise. Confirm at Tier 4.
- **FR-8 sweep** — ticker vs job queue. D-G picks the ticker; revisit only
  if a general settings/activity store lands.
- **`ACME_EMAIL` requiredness** — left optional; `/network/status` can
  surface "no ACME contact email set" as advice. Confirm at Tier 4.
- **Static-cert hot reload** — deferred to phase 16 (`fsnotify` watch);
  phase 13 documents "renewed static cert needs a restart".
- **Trusted-proxy `X-Forwarded-For`** — phase 13 keys on `RemoteAddr` only;
  a `TRUSTED_PROXY_CIDRS` config is a deliberate future addition, not a
  phase-13 gap.

## Critical files

- `.claude/roadmap/13-network-access/README.md` — Scope, Exit criteria, close gate
- `.claude/roadmap/13-network-access/implementation-plan.md` — Gate-1 decision record (§1–§7)
- `.claude/decisions/0028-…md`, `0017-…md`, `0025-…md`, `0021-…md`
- `.claude/specs/{domain-device-pairing,backend-network-transport,backend-network-api,frontend-network-and-pairing}.md`
- `.claude/specs/backend-configuration.md` FR-4/FR-8, `backend-http-transport.md` FR-1, `architecture-backend.md` FR-6
- `internal/config/bindaddress.go`, `internal/auth/{jwt,ratelimit}.go`, `internal/transport/http/{middleware,server,auth_middleware}.go`
- `internal/persistence/postgres/{migrate.go,source_removal_atomicity_integration_test.go}` — migration + atomicity-test patterns
- `web/src/app/routes.tsx`, `web/src/data/http.ts`
- `scripts/check-{parameterized-queries,user-scoped-reading,import-boundaries}.sh`
- `.claude/constitution.md` §3/§4/§5/§6/§8/§9/§10/§11/§12
