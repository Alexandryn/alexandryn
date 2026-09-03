# Phase 13 — Network access: Implementation Plan

**Status (2026-09-02): Gate 1 + Gate 1.5 CLEARED.** Scope + all decisions
maintainer-approved; ADR 0028 `Accepted`; the four phase-13 specs
independently reviewed (review `0050`, two agents), reworked, and moved to
`APPROVED`; phase-12 spec amendments written (`DRAFT`, pending
re-confirmation, non-blocking); audit `0012` corrected; forward directives
added to `CLAUDE.md` + templates. **Next: per-spec test plans (ADR 0016
cadence), then RED → GREEN.** Implementation order: phase-12 authorization
hardening prelude first (AUDIT-0012-C1/C2, `X-Library-Id`), then phase-13
tiers 0–5. Maintainer to be pinged before test-plan / RED work so the
right skills (`test-driven-development`, `build`, `security-and-hardening`)
are invoked.

### Gate 1 outcome — locked decisions

| # | Decision (as approved) |
|---|---|
| C-0 | No `TLS_ENABLED` key. TLS mode derives from address class + cert/ACME state, fail-closed at bind. `ACME_ENABLED` selects a provisioning source only. |
| C-1 | **(a)** LAN/remote clients authenticate with phase-12 account login (username/password → JWT, ADR 0025 untouched). Pairing is a thin additive layer: a 5-minute, single-use, ≥40-bit enrolment code that transfers host address + bootstraps a normal session. The code is never a credential. |
| C-2 | **No "authentication off" path exists** — not in config, not in domain, not in UI. Constitution §6 is absolute. The panel row is display-only ("Authentication is always on") or absent. |
| C-3 | Phase 13 ships *pair a device* + *revoke a pairing*. Phase 14 owns the standing device inventory, rename, session-expiry policy, cross-device progress. |
| C-4 | QR rendered client-side from a server payload string (`{ payload, address, expiresAt }`). No new Go module. Payload carries only host address + enrolment code — never a token or `DEVICE_PAIRING_SECRET`. |
| C-5 | CORS deny-by-default. Empty `CORS_ALLOWED_ORIGINS` ⇒ no `Access-Control-Allow-Origin` ever emitted. Exact-origin match only. Never `*` with credentials. |
| C-6 | No CSRF token machinery (Bearer-from-localStorage has no ambient credential). `Origin`/`Referer` allow-list validation on unauthenticated pairing routes, reject on mismatch. Rationale recorded in Spec A per §12. |
| C-7 | ACME issuance + renewal tested against **Pebble** in the integration suite. `autocert.Manager.HostPolicy` pinned to exactly `ACME_DOMAIN`. |
| C-8 | No separate `PORT` key. `BIND_ADDRESS` (`host:port`) is the single source of truth. |
| C-9 | `PATCH /api/v1/network/settings` touches only runtime-safe values (mDNS name, device-remember TTL). Bind address, TLS cert, ACME domain are config-file + restart only. |
| ADR | One ADR — **0028**. |
| Branch | `feat/phase13-network-access`, cut from `feat/phase12-auth` (done 2026-09-02). |
| **Gate on close** | Phase 13 does **not** close until `X-Library-Id` is validated against the JWT `libraries` claim (`internal/transport/http/auth_middleware.go:104` — currently unchecked; Low at loopback, High once the bind widens). Added to the phase 13 audit checklist + a handler test. |

Design contradictions D-1/D-2/D-3/D-4 are resolved by C-1 (D-1), C-2 (D-2),
Spec A/D filling the uncaptured Advanced panel (D-3), and C-3 (D-4).

---

## Gate 1.5 — spec-package review outcome (2026-09-02)

Two independent reviewers (one quality / cross-spec, one security /
four-attacker + STRIDE) reviewed the `DRAFT` package. Findings and
resolutions are recorded in
[`../../reviews/0050-phase13-spec-package-and-phase12-authz-review.md`](../../reviews/0050-phase13-spec-package-and-phase12-authz-review.md).
Maintainer response 2026-09-02: *"everything you mentioned needs fixing …
all needs to be documented — not the problems themselves, but directives
in audits, phases and specs … we'll go with your recommendations."*

### Two confirmed phase-12 defects — verified in code, not spec gaps

| ID | Defect | Verified at | Resolution |
|---|---|---|---|
| **H-1** | The reading API (`progress`, `bookmarks`, `highlights`, `preferences`, `export`, `reader/content`) is **not** user- or library-scoped. Handlers call the bare-ID / bare-work repository methods (`FindByWork`, `FindByEdition`, `FindByID`, `Save`, `Delete`), never the `…AndUser` variants; `UserFromContext` is not called in `reading.go`. `GET /api/v1/reading/export` returns every user's data instance-wide. Horizontal IDOR by object ID. | `internal/transport/http/reading.go`, `reading_export.go`; `internal/persistence/postgres/bookmark_repository.go:35,114`, `reading_progress_repository.go:63`, `reading_export_repository.go:55` | Every reading/reader handler MUST resolve `UserFromContext` + `ActiveLibraryFromContext` and call a user+library-scoped repository method. Bookmark/highlight `FindByID`/`Delete` MUST verify row ownership. A CI guard (`scripts/check-user-scoped-reading.sh` or a lint) fails the build if a reading handler calls a bare-ID method. Per-endpoint IDOR tests (user A cannot read/write/delete user B's row by ID) + an `export` isolation integration test. Folded into the phase-13 **close gate**. |
| **H-3** | `JWTSigner.Verify` (used by `AuthMiddleware`) checks `alg`/signature/`exp`/`iss` but **not `claims.Type`**. An MFA ticket (`typ:"mfa_ticket"`, real `Subject`, same signing key, ~5 min) is accepted as a bearer token on every authenticated-not-role-gated route. | `internal/auth/jwt.go:83` (`Verify` body, no `Type` check) | `Verify` (or a dedicated `VerifyAccessToken`) MUST positively require the access-token type and reject `mfa_ticket` / `enrol` / any other `typ`. The enrolment grant (`backend-network-transport.md` FR-10) MUST use a **separate HKDF subkey** (`DeriveSubkey("enrolment-grant-v1")`, matching `cmd/server/run.go`'s existing `jwt-signing-secret-v1` / `mfa-totp-master-v1` pattern), so cross-type acceptance is structurally impossible. Middleware test per token type. |

Audit `0012` certified both controls as present. **Audit `0012` is
corrected** (post-audit correction section added, honest severity, §12) —
not rewritten to hide the miss, but annotated with what was actually wired
and the corrective directives. This is not a rewrite of phase 12; it is
the phase-13 close gate doing its job (the gate always covered
`X-Library-Id`; it now covers the whole reading-authz surface).

### Phase-13 spec findings — all being fixed (no decision needed)

| ID | Fix |
|---|---|
| **B1/B2** | Pairing code is **encrypted at rest** (AES-GCM, dedicated HKDF subkey `pairing-code-enc-v1`), not hash-only. Resolves the domain-compare / `/qr` / FR-1-vs-FR-3 contradiction: the code is recoverable to render the QR and to run the domain's constant-time `Equal` on rehydration; `/qr` works after a page reload; a DB-read attacker in the ≤5-min window still faces the rate limiter + `Origin` check + single-use. FR-1 reworded ("returned in the initiate response and re-fetchable via `/qr` by the initiating admin until terminal"). |
| **H-2** | New FR in `backend-network-transport.md`: a **security-headers middleware on all binds** (not public-only) — `Content-Security-Policy` for the app origin (`frame-ancestors 'none'`, `default-src 'self'`, `object-src 'none'`, `base-uri 'none'`, no `unsafe-inline` on `script-src`), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy` minimal. The phase-13 audit **establishes** the app-origin CSP (it does not "re-confirm" one — none exists outside the reader iframe). |
| **H-4** | Private/LAN bind gains **opt-in in-process TLS**: `TLS_CERT_FILE`/`TLS_KEY_FILE` present on a private bind ⇒ `ServeTLS` (no ACME — domain validation can't work on a private range). Recorded as an ADR 0017 Mode-B sub-case in ADR 0028 §1 (0017 stays Accepted; 0028 extends it). Regardless of TLS: when `tlsMode == "none"` and `reachability != "loopback"`, the Network panel states plainly that LAN traffic is unencrypted unless a reverse proxy terminates TLS, and startup logs the condition once at `warn`. |
| **M4** | `rememberDeviceDays` is **wired** to override ADR 0025's fixed 30-day refresh-token lifetime at issuance. `backend-authentication.md` + ADR 0025 amended; a test proves a changed value changes the issued refresh token's `expires_at`. |
| **M5** | `fr4` / `atFirstRun` (Unclassified in `ANALYSIS.md`) and `sgDevices` (no ledger row) are **dropped as cited design authority**. D-2/D-4 rest solely on `sgNetwork`'s "Advanced panel" ledger row (**Binding, contents uncaptured**) and the roadmap text. Each spec's Design-reference header corrected. **Resolved 2026-09-03 (maintainer): a first-run network step is out of scope for phase 13** — `roadmap/13-network-access/README.md` Scope Out; the whole first-run surface is deferred, `atFirstRun` stays Unclassified. |
| **M6** | **D-5 recorded**: the "Allow access from this network" toggle (`sgNetwork:223`) contradicts C-9 (bind is restart-only config) the same way D-2's toggle does. Resolved identically — the toggle becomes a read-only reachability statement. Added to every relevant spec's contradiction list. |
| **B3** | `roadmap/13-network-access/README.md` Scope In / Exit criteria **amended** for ADR 0028: the "session cookie attributes" bullet and criteria become `N/A` with a pointer to ADR 0028 §5; the CSRF criterion is reworded to the `Origin`-validation-plus-recorded-rationale form. Makes "every FR maps to an exit criterion" satisfiable. |
| **M1** | The actual amendment text is written to the specs that own the changed behaviour: `backend-authentication.md` (login `enrolmentGrant` param + response; access-token `typ` check), `backend-library-namespaces.md` (`X-Library-Id` ∈ `claims.Libraries` check), `backend-reading-api.md` (per-user + per-library query-layer enforcement + CI guard), `backend-reader-content.md` (library-membership check before serving edition bytes). |
| **M2** | `backend-http-transport.md` FR-1 amended with the full new ordered middleware chain (recovery → security headers → HSTS(public) → CORS → limits → logging → global rate-limit(public) → auth → Origin-validation(pairing) → routing) and the rationale; `architecture-backend.md` FR-6 checked. |
| **M3 / M-2** | `backend-network-transport.md` FR-2 states the DNS-name classification rule **explicitly**: a `BIND_ADDRESS` host that is neither an IP literal nor `localhost` is classified **publicly routable without DNS resolution** (Mode A required, fail-closed); never resolved. Classifier test rows added (`library.example.com:443` → public; `foo.local:8080` → public). |
| **M7** | `PairedDevice` ownership defined precisely: the device belongs to whoever completes login with the grant; the initiating admin is not an owner. The unreachable `else` branch and the silent-takeover path are removed. A grant whose session was `InitiatedBy` a different user than the one now logging in is ignored (I-4). Second-user login from a shared device is specified. |
| **M8** | `domain-device-pairing.md` FR-2 reworded as a fixed 8-char Crockford **shape/length** invariant on the constructor; the ≥40-bit **entropy** assertion moves to `GeneratePairingCode`'s acceptance criteria (`backend-network-transport.md` FR-9). |
| **M-1** | `backend-network-api.md` FR-2: the verify transaction MUST take a row lock (`SELECT … FOR UPDATE`) on the `pairing_sessions` row before the domain `Verify`→`Consume`; a concurrent double-submit integration test is added to the must-fail-first list. |
| **M-3** | `backend-network-transport.md` FR-7: the pairing-route `Origin` allowed set is enumerated — every non-loopback interface address at the bound port, the configured mDNS `hostName`, and any `CORS_ALLOWED_ORIGINS` entry; multi-address test matrix added. |
| **M-4** | `/network/status` response is **split by role**: `reader` gets `reachability` + `tlsMode` + the single address they connected on; `admin` gets the full `addresses[]` + `acmeDomain`. `ACME_EMAIL` added to the never-include list. |
| minors | stale `claimed` state removed from this plan; static-cert public bind gets a `:80` HTTP→HTTPS redirect too (not ACME-only); enrolment grant is single-use within its TTL; `tlsMode` UI copy made honest for `private` + `none`; Let's Encrypt TOS auto-accept surfaced in the ACME setup copy; `paired_devices.pairing_session_id` FK is `ON DELETE SET NULL`; `hostName` regex tightened to a valid DNS label; `/connect` strips `?c=` via `history.replaceState`; ACME `:80` listener rate-limited and path-restricted; `pairing_sessions.initiator_ip` retention bound stated; ADR 0028 title trimmed; Pebble §9 record gains the "what breaks if abandoned" clause; `qrcode` npm dependency gets its full §9 record + `check:bundle-size` headroom check before Spec D reaches `APPROVED`; audit `0012`'s `nbf`/`typ` over-claims corrected. |

### Directives added to prevent recurrence (maintainer: "not the problems, directives")

- **`CLAUDE.md`** — new Reflex + "Things easy to get wrong" entry: *every
  handler serving user-owned data resolves the authenticated user and
  active library and calls the user+library-scoped repository method,
  never a bare-ID variant; CI enforces this.* And: *token verification
  on the auth path always asserts the token type; a signature-valid
  token of the wrong purpose is a rejected token.*
- **`.claude/templates/audit.md`** — new mandatory checklist item: *for
  every data-read/-write endpoint, trace the wired handler → repository →
  SQL and confirm the tenant/user predicate is in the query text. Do not
  certify an authorization control from spec intent or from the
  existence of a scoped method — only from the call the handler actually
  makes.*
- **`.claude/templates/spec.md`** — Security considerations guidance: *for
  each authorization requirement, name the exact query predicate (or
  middleware check) that enforces it, and the test that proves a
  cross-tenant/cross-user access is refused.*
- **`.claude/roadmap/12-authentication/README.md`** — post-close
  correction note pointing at review `0050` and audit `0012`'s
  correction; phase 12 cannot be marked `Closed` until H-1/H-3 are fixed
  and re-verified.
- **`.claude/audits/0012-phase12-auth.md`** — "Post-audit correction
  (2026-09-02)" section: what was actually wired, the two missed
  findings at honest severity, the corrective directives, and a pointer
  to where each is now enforced.

### Future feature preserved, not built (maintainer note)

A library-visible **"completed by"** signal — after a user *finishes* a
book, other members of the same library see it marked read (and, later, a
per-library **most-read leaderboard**) — **without exposing anyone's
position or progress**. This is a distinct read model, **out of scope for
phase 13**, belonging in **phase 15 (Observability / Activity)** or its
own spec. The H-1 fix explicitly preserves it: progress rows keep
`user_id` + `library_id`, so a future
`WHERE work_id = $1 AND library_id = $2 AND percentage >= 100` aggregate
over the shared library is a straightforward addition on top of the
per-user-private base. Recorded in `backend-reading-api.md` Non-goals and
`roadmap/15-observability/README.md`.

This document replaces the phase 11 plan (in git history at commit
`27c88dd`, "docs(plan): phase 11 implementation plan and Gate 1 outcome").
Phase 11 is `IMPLEMENTED`, security audit `0011` cleared, awaiting maintainer
PR approval for `VERIFIED`.

**Branch:** `feat/phase13-network-access`, to be cut from
`feat/phase12-auth` (phase 12 is not yet merged to `main`; phase 13 depends
on phase 12's auth middleware, JWT signer, RBAC, and multi-library model —
see current-state assessment). If the maintainer merges phase 11 + phase 12
to `main` first, re-cut from `main`. **Not `main`.** ([[feedback_never_commit_on_main]])

**Governing documents:**

- `CLAUDE.md`, `.claude/constitution.md` (§1–§12, with §3/§4/§5/§6/§8/§9/§10/§11 load-bearing here)
- `.claude/roadmap/13-network-access/README.md` (full phase doc — Scope In/Out, Exit criteria)
- `.claude/decisions/0017-network-exposure-condition-gated.md` (**the rule this phase's bind logic enforces**)
- `.claude/decisions/0025-jwt-session-mechanism.md` (phase 12's session mechanism — Bearer/JWT/localStorage, **no cookies**)
- `.claude/decisions/0026-multi-library-tenancy-model.md`, `.claude/decisions/0027-totp-mfa-implementation.md`
- `.claude/specs/backend-configuration.md` FR-8 (`BIND_ADDRESS` classifier — already implemented, `internal/config/bindaddress.go`)
- `.claude/specs/backend-http-transport.md` (middleware chain, FR-2 limits, `/healthz`/`/readyz`; CORS explicitly deferred to "phase 13's LAN story")
- `.claude/specs/backend-authentication.md` (phase 12 — `APPROVED` 2026-09-02)
- `.claude/specs/architecture-system.md` FR-3, FR-6 (same build, same origin), trust boundaries
- `.claude/specs/frontend-shell-and-routing.md` FR-3/FR-4 (capability gating — `hostOnly()` gate already exists)
- `.design-reference/ANALYSIS.md` (synced 2026-08-13; scope-classification pass 2026-08-17)

---

## 0. Design reference conformance check (ADR 0003)

Canvases re-read fresh at planning time, not from memory.

### Canvases consulted

| Canvas | Blocks read | `ANALYSIS.md` sync date |
|---|---|---|
| `Alexandryn-Electron-Admin.dc.html` | `sgNetwork` panel (lines 193–236); First-run "Network access" step `fr4` (lines 1064–1095); `sgDevices` panel (lines 238–254); `DEVICES` mock (lines 1131–1134) | 2026-08-13 (2nd sync) |
| `Alexandryn-Web.dc.html` | `atConnect` (lines 532–563); `atAccess` (lines 565–601+); `hostModes`/`perms` state (line ~817+) | 2026-08-13 (2nd sync) |

### Classification (from `ANALYSIS.md` scope-classification ledger, 2026-08-17)

| Screen / surface | Classification | Note |
|---|---|---|
| `atSettings` → Network tab → Advanced panel (bind address, TLS cert, mDNS) | **Binding, contents uncaptured** | Roadmap names this as phase 13's UI home. "Open advanced" panel body is not drawn — phase 13 fills it in, does not invent it from nothing. |
| `atConnect`, `atAccess` (Web) | **Binding**, correctly deferred (outline only) | Pairing/access flow for a LAN client — Phases 12/13. |
| `sgDevices` (Settings → Devices; revoke) + `atConnect`'s "Settings → Devices → Pending" reference | **Binding** (device list); phase assignment ambiguous | Roadmap assigns "Device management" to **phase 14**. See conflict D-4. |
| Cloud relay (`hostModes` `'cloud'`) | Exploratory — **declined** 2026-08-18 | Not built. Canvas stays visual reference only. |
| `atTablet` placement | Binding, location TBD | Open question in `ANALYSIS.md`; not phase-13-blocking. |

### Contradictions found — flagged for Gate 1, NOT reasoned around

**D-1 — `atConnect` shows a "Library passphrase" sign-in, not phase 12's
username/password login.** The captured `atConnect` screen ("Sign in to
Home Library") has a single **"Library passphrase"** field, a "Remember
this browser for 30 days" checkbox, and the note *"Or approve this browser
from the host computer: Settings → Devices → Pending."* Phase 12 built
`POST /api/v1/auth/login` taking `emailOrUsername` + `password`, issuing a
15-minute JWT + 30-day refresh token (ADR 0025). The design's mental model
is a **shared library passphrase + host-side device approval**, which is
the pairing model this phase's `DEVICE_PAIRING_SECRET` / pairing-code scope
describes — but it does not obviously reconcile with per-user
username/password accounts. Decision needed: **C-1**.

**D-2 — `sgNetwork` and `fr4` show a "Require authentication" toggle that
can be turned OFF.** Both captured panels render "Require authentication"
as a switch (drawn ON). Constitution §6 and ADR 0017 are absolute: there
is *no* build or configuration in which the library is reachable from
another machine without a credential. A user-facing toggle that implies
"LAN access without auth" is a captured design that contradicts a
non-negotiable rule. Proposed reconciliation: the toggle does not exist, or
is present-but-locked-on with explanatory copy, or is reframed
("Remember approved devices"). Decision needed: **C-2**. Per CLAUDE.md this
is a stop-and-ask before the UI spec is drafted, not a footnote.

**D-3 — no domain field / no ACME UI anywhere.** `ANALYSIS.md` already
records this: "no domain field, no cert-upload/ACME UI exists in any
canvas." ADR 0017 Mode A (public bind, in-process TLS, ACME) has zero
captured UI. Phase 13 must design this surface from the roadmap text alone.
Flagged, not blocking — but the "Advanced" panel contents are a genuine
design gap the spec fills and the maintainer should be aware it is
un-refereed by the prototype.

**D-4 — device-management UI straddles the phase 13 / phase 14 line.**
`sgDevices` (list connected devices, "Revoke") and the "Settings → Devices →
Pending" approval queue are captured and Binding, but the roadmap puts
"Device management, progress across devices" in **phase 14**. Proposed
split: phase 13 owns *pairing a new device* (initiate → code/QR → verify →
device gets a session) and the *pending-approval* action needed to complete
a pairing; phase 14 owns the *standing device list, rename, revoke,
per-device session expiry*. Decision needed: **C-3**.

---

## 1. Current-state assessment

What already exists that this phase builds on or extends:

| Area | State | File(s) |
|---|---|---|
| `BIND_ADDRESS` classifier (ADR 0017 Mode A/B) | Implemented; **currently rejects all public binds** regardless of cert, because `ServeTLS` is not wired. `validatePublicBindCertificate` is written, tested, ready. | `internal/config/bindaddress.go`, `backend-configuration.md` FR-8 interim note |
| Config struct + keys | `BindAddress`, `TLSCertFile`, `TLSKeyFile` present. No `PORT`, `TLS_ENABLED`, `ACME_*`, `CORS_ALLOWED_ORIGINS`, `DEVICE_PAIRING_SECRET`. | `internal/config/config.go` |
| HTTP server bootstrap | `http.Server` with timeouts, graceful shutdown, `/healthz` `/readyz`, SPA static serving. **`ListenAndServe` only — no `ServeTLS`, no HTTP→HTTPS redirect, no ACME listener.** | `internal/transport/http/server.go`, `static.go`, `health.go` |
| Middleware chain | panic recovery → limits → logging → **auth (JWT Bearer)** → routing. No CORS, no CSRF, no global rate-limit middleware. | `internal/transport/http/middleware.go`, `auth_middleware.go` |
| Auth | Full phase 12: `internal/auth` (jwt, password/Argon2id, totp, ratelimit), `AuthMiddleware`, `RequireRole`, RBAC, multi-library `X-Library-Id`. **Bearer token in `Authorization` header, stored in `localStorage`. No `Set-Cookie` anywhere in the codebase.** | `internal/auth/*`, `internal/transport/http/auth_*`, `web/src/data/auth.ts`, `web/src/data/http.ts` |
| Rate limiter | `auth.IPRateLimiter` — per-IP token bucket over `golang.org/x/time/rate`, TTL eviction. Attached to auth endpoints only. Reusable for a global middleware. | `internal/auth/ratelimit.go` |
| Domain | No `DevicePairing` / `PairedDevice` / `PairingCode`. `UserID`, `LibraryID`, `Role` exist. | `internal/domain/auth.go` |
| Persistence | pgx pool, goose migrations (latest `00009`). No paired-devices table. | `internal/persistence/postgres/` |
| Frontend routing | `hostOnly()` capability gate exists; `settings`/`system` host-gated; `access`/`connect` are viewer-side `ScreenPlaceholder`s. Token attached by `web/src/data/http.ts` `defaultHeaders()`. | `web/src/app/routes.tsx` |
| Dependencies | `golang.org/x/crypto` **already direct** (Argon2id) → `acme/autocert` is a sub-package, **no new module**. `golang.org/x/time` already direct. **No QR-code encoder.** | `go.mod` |

---

## 2. Decisions required at Gate 1

Each of these changes what gets built. None is mine to resolve
(constitution Review gates; "if a rule here conflicts with an instruction
you have been given, say so and propose the safer path").

### C-0 — `TLS_ENABLED` config key contradicts ADR 0017 (recommend: drop it)

The task scope lists a `TLS_ENABLED` boolean. ADR 0017's **Option B was
explicitly rejected**: "Trust a config flag ('remote mode: on') instead of
inspecting the bind address … a flag is something that can be wrong,
forgotten, or left on by accident." The decision is: TLS mode is *derived*
from `BIND_ADDRESS`'s resolved class + certificate/ACME state, fail-closed
at bind time, never from a flag asserting intent.

**Recommendation:** no `TLS_ENABLED` key. Behaviour is fully determined by:
(a) address class (loopback/private ⇒ Mode B, upstream TLS or none;
public ⇒ Mode A, in-process TLS required); (b) within Mode A, `ACME_ENABLED`
selects ACME issuance vs. static `TLS_CERT_FILE`/`TLS_KEY_FILE`. `ACME_ENABLED`
is not a security toggle (both Mode A paths fail closed) — it selects a
provisioning *source*, which is legitimately a choice. If the maintainer
wants `TLS_ENABLED` for a private-range bind (opportunistic local TLS
without a public address), that is a *new* sub-case ADR 0017 doesn't cover
and needs its own ADR paragraph.

### C-1 — LAN client authentication model (design D-1)

Does a LAN/remote browser client authenticate by:

- **(a) phase 12 username/password + JWT**, unchanged — `atConnect`'s
  "Library passphrase" is reinterpreted as "your account password", device
  pairing is an *additive* convenience (QR pre-fills the host address +
  optionally a short-lived enrolment token so the user doesn't type
  credentials on a TV browser); or
- **(b) a shared library passphrase** (`DEVICE_PAIRING_SECRET`) that gates
  *device enrolment*, after which the device holds a long-lived
  device-scoped refresh token ("Remember this browser 30 days") and no
  per-user login happens on that device; or
- **(c) both** — passphrase enrols the device, then the user still logs in
  per-account within it (two factors: device + user).

**Recommendation: (a) + a thin pairing layer.** It preserves ADR 0025
untouched, keeps per-user accountability (RBAC, multi-library, audit all
key off `UserID`), and treats pairing as what the QR flow is *for*:
transferring the host address and a 5-minute one-time enrolment code to a
new device so login there is a tap, not a typed URL + typed password. The
`atConnect` "passphrase" copy becomes "Sign in" (account password) and the
"approve from host" path becomes the QR/code pairing. This also matches
`atAccess`'s per-session permissions view.

### C-2 — "Require authentication" toggle (design D-2)

Constitution §6 forbids an unauthenticated reachable build. Options:

- **(a)** remove the toggle entirely from the Advanced/Network panel;
- **(b)** keep it visible, locked ON, with copy explaining it cannot be
  disabled (transparency about the security posture);
- **(c)** relabel it to what it actually controls
  (e.g. "Remember approved devices for 30 days" — the refresh-token TTL).

**Recommendation: (b) for the "authentication" row (locked ON, explanatory
copy — calm, specific, constitution §11), plus (c) as a separate real
toggle for device-remember duration.** Escalating because a captured
Binding screen contradicts a non-negotiable rule.

### C-3 — phase 13 / phase 14 device-management split (design D-4)

**Recommendation:** phase 13 = pair (initiate/verify/QR) + the minimal
"pending devices" list needed to approve a pairing from the host +
issue/deny. Phase 14 = standing device inventory, rename, revoke, session
expiry policy, cross-device progress. The `sgDevices` "Revoke" affordance:
phase 13 ships revoke-a-pairing (fail-safe: you can always undo an approval
you just made); phase 14 generalises it.

### C-4 — new dependency: QR-code encoder (constitution §9 — ask before adding)

The QR payload must be encodable to an actual scannable matrix. Options:

- **`rsc.io/qr`** — ~1k LOC, single purpose, by Russ Cox, no transitive
  deps, stable since 2014. Generates the matrix; we render to SVG/PNG
  ourselves.
- **`github.com/skip2/go-qrcode`** — more popular, pulls no runtime deps,
  also generates PNG directly.
- **Hand-roll** — Reed-Solomon + bit-matrix placement is ~600 LOC of
  fiddly, well-specified but error-prone code. Not recommended for a
  security-adjacent artifact.
- **Client-side only** — server returns the payload string + host address;
  the React client renders the QR with a JS lib (`web/` already carries a
  frontend bundle budget — a QR lib is ~10 KB). Server never encodes.

**Recommendation: client-side rendering of a server-provided payload
string.** Keeps the Go dependency surface unchanged, keeps the QR a pure
presentation concern, and the payload (a URL + short code) is trivial to
contract-test as a string. `GET /api/v1/network/pair/qr` then returns
`{ payload, address, expiresAt }` JSON, not an image. If the maintainer
wants a server-rendered image (e.g. for the Electron first-run screen
before the SPA loads), `rsc.io/qr` is the pick and needs sign-off.

### C-5 — CORS: is it actually in scope?

`architecture-system.md` FR-6 and `backend-http-transport.md` establish
**same build, same origin**: the Go server serves the SPA, so browser
clients are same-origin and CORS is not exercised. The roadmap keeps "CORS
hardening … allowed origin tied to the deployment's actual configured
address/domain" in scope. The only real cross-origin case is a
user-operated reverse proxy terminating TLS on a different hostname than
the app knows itself by.

**Recommendation:** ship a CORS middleware that is **deny-by-default** and
allows exactly the configured origin set (`CORS_ALLOWED_ORIGINS`, empty by
default ⇒ no cross-origin request is ever honoured; the same-origin SPA
keeps working because same-origin requests don't need CORS). This satisfies
the exit criterion ("CORS origin configuration verified to track the
deployment's actual configured address") without inventing a cross-origin
feature. State plainly in the spec that with an empty allowlist CORS is
effectively off and that is the default and expected posture.

### C-6 — CSRF: what does it mean under header-Bearer auth?

Phase 12 auth is `Authorization: Bearer` from `localStorage` — **not a
cookie**. Browsers do not attach `Authorization` headers to cross-site
requests automatically, so classic CSRF (forged state-changing request
riding an ambient cookie) is **not reachable** for the authenticated API.
Double-submit-cookie / synchroniser-token machinery would be defending a
vector that doesn't exist.

**Recommendation:** no token-based CSRF middleware. Instead: (1) require
`Authorization` on every state-changing route (already true); (2) add
`Origin`/`Referer` allow-list validation on unauthenticated *pairing*
endpoints (the one place a forged cross-origin POST could matter — e.g.
`pair/verify`), rejecting requests whose `Origin` isn't the configured
host; (3) document in the security spec why full CSRF tokens are not
warranted, so a future reviewer doesn't read the absence as an oversight
(constitution §12). If the maintainer chooses C-1 option (b)/(c) with a
cookie-held device token, CSRF is back on the table and this recommendation
flips — hence it's a Gate 1 decision, downstream of C-1.

### C-7 — ACME "real certificate lifecycle" testing (exit criterion)

Exit criterion: "ACME issuance and renewal tested against a real
certificate lifecycle, not just a fixture." That needs an ACME server in
CI. Options: **Pebble** (Let's Encrypt's test ACME server, a small Go
binary / container) exercised in an integration test with `-tags=integration`;
or Let's Encrypt **staging** (real, rate-limited, needs a real
reachable domain — not viable in CI). **Recommendation: Pebble in the
integration suite**, HTTP-01 challenge served by our own ACME listener,
asserting issuance + a forced renewal. Pebble is a test-only tool (not a
`go.mod` dependency — run as a service container), analogous to how
Postgres is provided to integration tests.

---

## 3. Proposed ADR

### ADR 0028 — Phase 13 network transport: TLS mode selection, ACME provisioning, CORS/CSRF posture, device pairing model

One ADR, because these decisions interlock (C-0, C-1, C-5, C-6 especially).
Scope of what it will decide:

1. **TLS mode is derived, not flagged** (C-0) — restates ADR 0017's
   fail-closed derivation as concrete server behaviour: address class →
   `ServeTLS` vs plain listener; Mode A → ACME (`autocert`) or static cert;
   `TLS_ENABLED` rejected as a key.
2. **ACME via `golang.org/x/crypto/acme/autocert`** — filesystem cache
   (`ACME_CACHE_DIR`), HTTP-01 only, `HostPolicy` pinned to `ACME_DOMAIN`,
   no new module dependency; Pebble for lifecycle tests (C-7).
3. **Cipher/version floor** — TLS 1.2 minimum, 1.3 preferred; explicit
   cipher suite list for 1.2; ALPN `h2`, `http/1.1`.
4. **CORS deny-by-default, allowlist = configured origin(s)** (C-5).
5. **No CSRF token machinery; Origin-validation on pairing endpoints;
   rationale recorded** (C-6).
6. **Device pairing model** (C-1) — whichever option the maintainer picks;
   the ADR records it and why, and its relationship to ADR 0025.
7. **HTTP→HTTPS redirect listener** on Mode A (port 80 → 443) purely to
   serve ACME HTTP-01 + redirect; never serves app content.

If the maintainer prefers finer granularity, this can split into 0028
(transport/TLS/ACME) + 0029 (pairing model). Recommend one.

---

## 4. Proposed spec breakdown

> **Superseded by the spec files (2026-09-02).** Sections 4, 6, and 7
> below are the Gate-1 sketch. The four specs were drafted, independently
> reviewed (review `0050`), reworked, and are the authoritative source now
> — see Gate 1.5 above for the finding→resolution map. Where this sketch
> and a spec disagree (e.g. this section still shows a `claimed` pairing
> state and hash-only code storage), **the spec is right.** Kept for the
> Gate-1 audit trail, not maintained.

Four specs. Each gets its own scope-approval sub-gate per CLAUDE.md —
this plan states scope; drafting waits for the go-ahead.

### Spec A — `backend-network-transport.md` (phase 13)

**Scope:** the fail-closed startup wiring and transport middleware. Bind
classification → listener selection (`ListenAndServe` vs `ServeTLS` vs
`autocert` manager); certificate load/validate at startup (Mode A static);
ACME issuance + renewal + HTTP-01 listener + `ACME_CACHE_DIR` permissions;
TLS version/cipher/ALPN policy; HTTP→HTTPS redirect; graceful shutdown
across both listeners; CORS middleware (deny-by-default, `CORS_ALLOWED_ORIGINS`);
Origin-validation for unauthenticated pairing routes; global unauthenticated
rate-limit middleware (health, static, pairing-initiate) reusing
`auth.IPRateLimiter`; the config keys (`PORT` question C-8 below,
`TLS_ENABLED` dropped per C-0, `ACME_*`, `CORS_ALLOWED_ORIGINS`).
**Key decisions:** C-0, C-5, C-6, C-7; cipher list; redirect-listener
behaviour; rate-limit buckets/limits for public endpoints.
**Design reference:** `N/A` (transport layer, no UI) — the config surface
it feeds is `sgNetwork` "Advanced", covered by Spec D.
**Amends:** `backend-configuration.md` FR-4 (new keys) + FR-8 interim note
(remove once `ServeTLS` wired); `backend-http-transport.md` Non-goals
(CORS un-defers).

### Spec B — `domain-device-pairing.md` (phase 13)

**Scope:** pure domain — `PairingSession` (aggregate: host-initiated, holds
a one-time `PairingCode`, TTL, state machine `pending → claimed → verified
→ consumed | expired`), `PairedDevice` (value/entity: `DeviceID`, owning
`UserID`, label, user-agent class, created/last-seen, enrolment method),
`PairingCode` (value object: format, entropy, comparison rules). Invariants:
code single-use, TTL enforced by an injected clock, a session binds to
exactly one device on verify, no PII beyond a coarse UA class. Zero
networking/crypto imports (crypto-random generation lives in an adapter;
domain takes the generated value).
**Key decisions:** C-1 (shapes the aggregate — device-scoped token vs
enrolment-code-then-login), C-3 (does `PairedDevice` carry revocation state
in phase 13 or phase 14?).
**Design reference:** `Alexandryn-Web.dc.html` `atConnect`/`atAccess` +
`Alexandryn-Electron-Admin.dc.html` `sgDevices`, synced 2026-08-13,
classification Binding (2026-08-17). Contradictions D-1/D-4 recorded above.
**Domain boundary (§3):** devices belong to the **Alexandryn** domain
(what the user has/does), not metadata/source. Transport concerns
(TLS, IP) never enter this package.

### Spec C — `backend-network-api.md` (phase 13)

**Scope:** the HTTP surface + persistence.
Endpoints: `POST /api/v1/network/pair/initiate` (host/admin only — mints a
`PairingSession` + code), `POST /api/v1/network/pair/verify` (device
submits code → gets its session/token per C-1), `GET /api/v1/network/pair/qr`
(payload string + address + expiry — client renders, per C-4),
`GET /api/v1/network/status` (server running, bind class, TLS mode, mDNS
name, addresses — the `sgNetwork` status block; must **not** leak the cert
private key path, full home paths §8, or `DATABASE_URL`),
`PATCH /api/v1/network/settings` (the writable toggles that survive C-2 —
likely just device-remember duration + mDNS name; **not** bind address,
which is restart-only config, see C-9).
Persistence: `paired_devices` + `pairing_sessions` tables, repository in
`internal/persistence/postgres/`, parameterised queries only
(`scripts/check-parameterized-queries.sh`), migration `00010`.
OpenAPI: new paths + `example:` blocks for every JSON response
(`npm run mocks:gen-fixtures`). Contract tests in
`internal/testutil/contracttest/`.
**Key decisions:** C-1, C-2 (which settings are writable at runtime vs
restart-only), C-9 (can bind address change without a restart? recommend
no — `backend-configuration.md` Non-goals: "no runtime-mutable
configuration"; a bind change is a restart).
**Design reference:** `sgNetwork` panel + `fr4` first-run, synced
2026-08-13, Binding/contents-uncaptured. D-2/D-3 recorded.

### Spec D — `frontend-network-and-pairing.md` (phase 13)

**Scope:** two surfaces.
(1) **Host — Settings → Network** (`web/src/screens/Settings/NetworkSettings.tsx`,
behind the existing `hostOnly('settings')` gate): server status block
(HOST/PORT/LOCAL/NETWORK from `/network/status`), the surviving toggles
(C-2), the "Advanced" disclosure (bind address / TLS cert / ACME domain /
mDNS name — display + the restart-required copy per C-9), the
"Open on another device" QR + address card.
(2) **Viewer — pairing/connect** (`web/src/screens/Network/DevicePairingModal.tsx`
and the `connect`/`access` routes currently placeholders): the device's
side of `pair/verify`; `atAccess` "This session" permissions view.
Design tokens only (`text-2xs`, `gap-sm`, `bg-bg-surface`), no raw px,
WCAG AA contrast, keyboard-reachable, announced errors (constitution §7;
`frontend-accessibility.md`). QR rendered client-side (C-4).
**Key decisions:** C-1, C-2, C-3 — all three shape this UI directly. This
spec's scope-gate is the one where D-1/D-2/D-4 must be resolved before
drafting (CLAUDE.md: "If a captured screen contradicts the intended scope,
that's a stop-and-ask before drafting continues").
**Design reference:** `atConnect`, `atAccess` (Web); `sgNetwork`, `fr4`
(Electron-Admin), synced 2026-08-13, Binding. Contradictions D-1/D-2/D-3/D-4.

### C-8 / C-9 — smaller config decisions folded into Spec A/C

- **C-8 `PORT` as a separate key:** the design shows HOST and PORT as
  distinct fields. `BIND_ADDRESS` is already `host:port`. **Recommend:**
  keep `BIND_ADDRESS` as the single source of truth; if a bare `PORT` key
  is added it is sugar that must agree with `BIND_ADDRESS` or fail loudly.
  Prefer not adding it.
- **C-9 runtime-mutable settings:** `backend-configuration.md` Non-goals
  explicitly excludes runtime-mutable config. **Recommend:** `PATCH
  /network/settings` only touches values that are genuinely runtime-safe
  (mDNS advertised name, device-remember TTL). Bind address, TLS cert,
  ACME domain are restart-required; the UI shows them and a "changes apply
  after restart" line.

---

## 5. Threat model summary (Four-Attacker + STRIDE)

Full model goes in each spec's Security considerations and consolidated in
`.claude/audits/0013-phase13-network-access.md` at Gate 2. Summary of what
changes when the bind opens up:

| Attacker | New capability this phase grants them | Primary mitigations |
|---|---|---|
| **Malicious LAN client** (unauth) | Can now reach the listener at all | Auth enforced on every non-health route (§6, already true); global rate-limit on health/static/pairing-initiate; pairing code is single-use, TTL-bound, high-entropy; `pair/verify` rate-limited per IP; Origin-validation on pairing POSTs; `/network/status` leaks no secrets/paths |
| **Malicious LAN client** (paired, low-priv reader) | Holds a valid session | RBAC unchanged (phase 12); pairing never elevates role; device token scoped to the enrolling user's libraries only |
| **On-path / MITM attacker** | Can read/modify plaintext LAN traffic | Mode B: private-range only, operator's trust boundary; Mode A: in-process TLS 1.2+/1.3, HSTS on public bind, cert validated at startup, fail-closed (no plaintext fallback) |
| **Malicious source / book file** | Unchanged by this phase | Out of scope — phases 08/10/11 own it |
| **Attacker who phished the user onto a hostile page** | Could try a cross-origin request to the LAN server | CORS deny-by-default; `Authorization` header not auto-attached cross-site; Origin-validation on unauth pairing routes; localStorage token not reachable cross-origin |
| **Attacker with the QR code** (shoulder-surf, screenshot) | Could enrol their own device | Code TTL ≤ 5 min, single-use, invalidated on first successful verify; pairing requires host-side initiation (an admin action); `pair/initiate` is admin-only and rate-limited |

STRIDE mapping (per spec template's built-in mapping):
- **Spoofing** — auth middleware + pairing code entropy/TTL.
- **Tampering** — TLS integrity (Mode A), parameterised queries, input
  shape/size/timeout on every new endpoint (§4).
- **Repudiation** — structured logs + request IDs already; pairing events
  logged with `UserID` + `DeviceID`, never the code or token (§8).
- **Information disclosure** — `/network/status` scrubbed; no cert key
  path, no home paths, no `DATABASE_URL`; ACME cache dir perms `0700`.
- **Denial of service** — global rate limiter; ACME issuance rate-limited
  by `autocert` internal caching; connection/read/write/idle timeouts
  already set; `HostPolicy` prevents ACME issuance amplification.
- **Elevation of privilege** — pairing yields the enrolling user's own
  role and libraries, never more; `pair/initiate` gated to admin.

Explicit residual risk to record: **phase 12 stores tokens in
`localStorage`** (ADR 0025), which is XSS-reachable. Broadening the bind
raises the blast radius. Not this phase's decision to reverse, but the
audit will note it and confirm the CSP / sanitisation posture (ADR 0024,
phase 11) still holds for LAN-served content.

---

## 6. Domain types (proposed — Spec B fixes them)

```
PairingSession   (aggregate root)
  ID            PairingSessionID
  InitiatedBy   UserID          // the admin who started it
  Code          PairingCode     // one-time
  State         pending | claimed | verified | consumed | expired
  CreatedAt     time.Time
  ExpiresAt     time.Time       // CreatedAt + <=5m
  DeviceID      *DeviceID       // set on verify
  invariants: code single-use; no state move after consumed/expired;
              verify only from pending/claimed; clock injected

PairedDevice
  ID            DeviceID
  Owner         UserID
  Label         string          // user-supplied or derived ("Pixel 8 · Chrome")
  UAClass       phone | tablet | desktop | tv | unknown   // coarse, no fingerprint
  EnrolledVia   pairing-code | password-login
  CreatedAt     time.Time
  LastSeenAt    time.Time
  RevokedAt     *time.Time       // C-3: phase 13 or phase 14?

PairingCode  (value object)
  format: <N> crypto-random bytes, base32-Crockford, grouped (e.g. ABCD-EFGH)
  compare: constant-time
  entropy target: >= 40 bits (survives 5-minute online guessing behind a rate limit)
```

---

## 7. API routes (proposed — Spec C fixes them)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/v1/network/pair/initiate` | admin | mint `PairingSession` + code; returns `{ code, payload, address, expiresAt }` |
| POST | `/api/v1/network/pair/verify` | none (Origin-checked, rate-limited) | device submits `{ code }` → per C-1 returns a session/token or an enrolment result |
| GET | `/api/v1/network/pair/qr` | admin | `{ payload, address, expiresAt }` for client-side QR render |
| GET | `/api/v1/network/status` | authed | `{ running, bindClass, tlsMode, mdnsName, addresses[] }` — scrubbed |
| PATCH | `/api/v1/network/settings` | admin | runtime-safe settings only (C-9) |

All: request body size/shape/timeout validated before handler logic (§4);
JSON `Content-Type`; shared error helper; `example:` blocks in
`api/openapi.yaml`; contract tests.

---

## 8. Tiered TDD sequence (RED → GREEN per tier)

> **Executable breakdown (2026-09-03):** the section below is the Gate-1
> sketch. The maintainer-reviewed, task-by-task plan with acceptance
> criteria, verification commands, and checkpoints lives in
> [`tasks/plan-phase13-network.md`](../../../tasks/plan-phase13-network.md)
> and [`tasks/todo-phase13-network.md`](../../../tasks/todo-phase13-network.md).
> Locked decisions D-A..D-G (first-run held, `0.0.0.0` fail-closed,
> ACME-as-proof, menu-bar deferred, `qrcode` §9 at Tier 5) are in that
> plan's Architecture Decisions section.

Follows the task's tier plan; every tier writes failing tests first (§2).

- **Tier 0 — config & bind validation.** Add `ACME_*`, `CORS_ALLOWED_ORIGINS`,
  `DEVICE_PAIRING_SECRET` keys. Wire Mode A: `validateBindAddress` calls
  `validatePublicBindCertificate` (or ACME-config check) and *accepts* a
  valid public bind. RED: public bind + valid cert currently rejected →
  make it pass; public bind + no cert + no ACME → still fails closed;
  ACME config present → accepted. Remove `backend-configuration.md` FR-8
  interim note.
- **Tier 1 — pure domain.** `PairingSession` state machine, `PairingCode`
  entropy/format/compare, TTL via injected clock, all invariants. No I/O.
- **Tier 2 — transport services.** TLS config builder (version/cipher/ALPN)
  table tests; `autocert` manager construction + `HostPolicy` (unit, no
  network); CORS middleware (deny-by-default, allowlist match, preflight);
  Origin-validation middleware; global rate-limiter middleware; QR payload
  builder (string). Pebble-backed ACME issuance+renewal in integration
  suite (C-7).
- **Tier 3 — persistence.** `paired_devices` + `pairing_sessions` migration
  `00010`; repository integration tests (`-tags=integration`), parameterised
  queries, atomicity of verify (session consume + device insert in one tx —
  ADR 0021 transaction contract).
- **Tier 4 — HTTP + contract.** Handler tests per endpoint (happy +
  malformed/empty/oversized/expired-code/replayed-code/wrong-role);
  `ServeTLS` + redirect-listener wiring in `server.go` with graceful
  shutdown test; OpenAPI update + `npm run mocks:gen-fixtures`; contract
  tests.
- **Tier 5 — web/desktop UI.** `NetworkSettings.tsx`, `DevicePairingModal.tsx`,
  `connect`/`access` routes; Vitest unit tests; Storybook stories;
  Playwright E2E (pair flow end-to-end against mock); token-styling +
  a11y-tabindex + a11y-hidden-text checks; `npm run build`.

---

## 9. Verification matrix (roadmap exit criteria → evidence)

| Exit criterion | Verified by |
|---|---|
| Loopback default; broader bind requires explicit admin action | Tier 0 config tests; default `127.0.0.1:0` unchanged |
| Auth enforced on every reachable route, both bind modes | Tier 4 handler tests; `IsPublicPath` audit; middleware order test |
| TLS functional local (upstream/in-process) + public (in-process, cert validated at startup) | Tier 0 + Tier 2 + Tier 4 `ServeTLS` integration; fail-closed test (public + bad cert ⇒ no start) |
| ACME issuance + renewal against a real lifecycle | Tier 2 Pebble integration test (C-7) |
| CORS origin tracks configured address | Tier 2 CORS middleware tests; empty-allowlist = deny |
| CSRF tested against phase 12's session mechanism | Tier 2 Origin-validation tests + documented rationale (C-6); Bearer-not-cookie means no ambient-credential CSRF path |
| Session cookie attributes correct for LAN-IP and domain binds | **N/A — no cookies (ADR 0025).** Spec A records why, explicitly, per §12. **Maintainer: confirm this reading or redirect (C-1/C-6).** |
| General API rate limiting, not only login | Tier 2 global rate-limit middleware tests on health/static/pairing |
| Security audit recorded, no open Critical/High | `.claude/audits/0013-phase13-network-access.md` at Gate 2 |
| Maintainer approval recorded | Phase doc header, at close |

---

## 10. What I will NOT do without a decision

- Draft any spec (each needs its scope sub-gate; Spec D needs D-1/D-2/D-4
  resolved first).
- Draft ADR 0028 (needs C-0/C-1/C-5/C-6/C-7 direction).
- Write any implementation code.
- Add a Go module dependency (C-4 — recommendation is *no* new module).
- Add `TLS_ENABLED` (C-0 — contradicts ADR 0017).
- Build any "authentication off" affordance (C-2 — contradicts §6).
- Cross either review gate (spec review; security audit) on my own judgement.

---

## 11. Open questions rolled up for the maintainer

1. **C-0** — drop `TLS_ENABLED`? (recommend yes)
2. **C-1** — LAN client auth model: (a) account login + thin pairing / (b) shared passphrase + device token / (c) both? (recommend a)
3. **C-2** — "Require authentication" toggle: remove / lock-on / relabel? (recommend lock-on + explanatory copy)
4. **C-3** — device revoke/list: minimal in phase 13, full in phase 14? (recommend yes)
5. **C-4** — QR: client-side render of a server payload string? (recommend yes; no new Go dep)
6. **C-5** — CORS: deny-by-default + configured-origin allowlist, effectively off by default? (recommend yes)
7. **C-6** — CSRF: no token machinery, Origin-validation on pairing routes + recorded rationale? (recommend yes — downstream of C-1)
8. **C-7** — ACME lifecycle test via Pebble in the integration suite? (recommend yes)
9. **C-8** — add a separate `PORT` key? (recommend no)
10. **C-9** — `PATCH /network/settings` runtime-safe values only, bind/cert/domain restart-required? (recommend yes)
11. **ADR granularity** — one ADR 0028, or split transport vs pairing? (recommend one)
12. **Branch base** — cut `feat/phase13-network-access` from `feat/phase12-auth`, or wait for phase 11+12 to merge to `main`?
