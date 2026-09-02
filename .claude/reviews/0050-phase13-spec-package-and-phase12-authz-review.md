# Review: Phase 13 — network-access spec package (four specs + ADR 0028), two independent agents; plus two confirmed phase-12 authorization defects

| | |
|---|---|
| **Subject** | `.claude/decisions/0028-phase13-network-transport-and-pairing.md`; `.claude/specs/domain-device-pairing.md`, `backend-network-transport.md`, `backend-network-api.md`, `frontend-network-and-pairing.md` (all `DRAFT`); amendments to `.claude/specs/backend-configuration.md`, `backend-http-transport.md`, `.claude/specs/README.md`, `.claude/decisions/README.md`, `.claude/roadmap/13-network-access/README.md`; `.claude/roadmap/13-network-access/implementation-plan.md`. **Also, discovered during the review:** two shipped defects in phase-12 code (`internal/transport/http/reading.go`, `reading_export.go`, `reader_content.go`, `internal/auth/jwt.go`, `internal/transport/http/auth_middleware.go`). |
| **Reviewer** | Two independent agents run in parallel with no shared context: one `agent-skills:code-reviewer` (quality / cross-spec / architecture), one `agent-skills:security-auditor` (four-attacker + STRIDE). Code-level claims re-verified by the author before this record was written. |
| **Date** | 2026-09-02 |
| **Verdict** | **Needs rework** — spec package (3 Blocking, 8 Major, ~13 Minor) — plus **2 confirmed phase-12 High-severity authorization defects** that block phase 13 and require correcting audit `0012`. Maintainer response 2026-09-02: fix everything; encode the fixes as forward directives in specs / audits / phase docs, not as incident notes; proceed on the author's recommended path (phase-12 hardening prelude on the phase-13 branch). Resolution tracked in `implementation-plan.md` "Gate 1.5". |

## Summary

Phase 13 opens the host beyond loopback, and its safety argument rests
entirely on "phase 12 already made this safe — authentication on every
route, per-user data scoping, RBAC." The security pass found that **two of
those three are real and one is not**: the reading API is not user- or
library-scoped (any account reads, overwrites, deletes, and exports every
other account's private reading position, bookmarks, and highlight notes
by object ID), and the access-token verification path does not check the
token type (an MFA ticket is accepted as a bearer token). Audit `0012`
certifies both controls as present. Separately, the quality pass found the
new pairing-code persistence model self-contradictory and incompletely
plumbed, the roadmap exit-criteria not amended for ADR 0028, three
APPROVED specs changed in substance without amendment text, and the
design-conformance evidence over-claiming two Unclassified/absent design
surfaces as binding. The transport core of ADR 0028 (TLS derivation,
fail-closed public bind, ACME `HostPolicy` pinning, CORS deny-by-default,
the no-CSRF-under-Bearer reasoning) was found sound by both reviewers and
survives largely intact.

## Findings — phase-12 defects (confirmed in code)

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| P12-1 | **High** (Critical for a Mode-A public bind) | Authorization / IDOR | The reading API is not user- or library-scoped. `internal/transport/http/reading.go` handlers call `deps.Progress.FindByWork`, `deps.Bookmarks.FindByEdition`, `deps.Bookmarks.FindByID`, `.Save`, `.Delete` — the bare-ID / bare-work repository methods — never the `…AndUser` variants that exist alongside them. `UserFromContext` is never called in `reading.go`. `bookmark_repository.go:35` is `SELECT … WHERE id = $1`; `:114` is `DELETE … WHERE id = $1`. `reading_progress_repository.go:63` resolves, on the wired path, to `WHERE work_id = $1 AND COALESCE(user_id,'') = COALESCE('','')` — all progress collapses onto one NULL-user row. `reading_export_repository.go:55` filters `WHERE work_id = $1` only, so `GET /api/v1/reading/export` returns the whole instance's annotations. | Every reading/reader handler resolves `UserFromContext` + `ActiveLibraryFromContext` and calls a user+library-scoped repository method. `FindByID`/`Delete` for bookmarks and highlights verify row ownership (add `…AndUser` variants). A CI guard fails the build when a reading/reader handler calls a bare-ID method. Per-endpoint IDOR tests + an `export` isolation integration test. Directive added to `backend-reading-api.md`, `backend-reader-content.md`, `CLAUDE.md`, `templates/audit.md`. Folded into the phase-13 close gate. |
| P12-2 | **High** | Authentication / token confusion | `internal/auth/jwt.go:83` `JWTSigner.Verify` (called directly by `AuthMiddleware`) validates `alg`, signature, `exp`, `iss` but not `claims.Type`. `VerifyMFATicket` checks the type only in its own wrapper, which the middleware never calls. An MFA ticket (`Type:"mfa_ticket"`, real `Subject`, same signing key, ~5 min TTL) passes `Verify` and authenticates any route that is authenticated-but-not-role-gated. | `Verify` (or a new `VerifyAccessToken` used by the middleware) positively requires the access-token type and rejects `mfa_ticket` / `enrol` / any other `typ`. The phase-13 enrolment grant uses a separate HKDF subkey (`DeriveSubkey("enrolment-grant-v1")`). Middleware test per token type. Directive added to `backend-authentication.md`, `CLAUDE.md`, `templates/audit.md`. |
| P12-3 | Major | Audit integrity | Audit `0012` states JWT verification does "`nbf`/`exp` boundary checks" (no `nbf` field exists on `Claims`) and "explicit `typ: JWT`" validation (only `alg` is checked), and that reading data is "scoped by `user_id`" (P12-1). | Audit `0012` gets a "Post-audit correction (2026-09-02)" section: what was actually wired, the two missed findings at honest severity (§10), the corrective directives, a pointer to where each is now enforced. Not a rewrite. `roadmap/12-authentication/README.md` gets a post-close-correction note; phase 12 cannot be `Closed` until P12-1/P12-2 are fixed and re-verified. |
| P12-4 | Major | Authorization | `AuthMiddleware` (`auth_middleware.go:104`) takes `X-Library-Id` from the client header and never checks it against `claims.Libraries`. | `AuthMiddleware` rejects (`403`) a request whose `X-Library-Id` is not in the token's `libraries` claim. Directive in `backend-library-namespaces.md`. Handler test. (Already the phase-13 close gate; now stated as an owned FR.) |

## Findings — phase-13 spec package

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Blocking | Implementability | `backend-network-api.md` FR-7 stores SHA-256(code) only, but `domain-device-pairing.md` FR-6 runs the constant-time `Equal` **inside** the aggregate (needs the `PairingCode` value on a rehydrated session), and `backend-network-api.md` FR-3 (`/qr`) returns `code` + a code-bearing `payload`. A hash cannot satisfy any of these. | **Resolve as B1 option (a):** store the code **encrypted at rest** (AES-GCM, dedicated HKDF subkey `pairing-code-enc-v1`). Rehydration decrypts → domain `Equal` works; `/qr` decrypts → payload re-renders after a reload; a DB-read attacker in the ≤5-min window still faces the rate limiter + `Origin` check + single-use. |
| 2 | Blocking | Contradiction | `backend-network-api.md` FR-1 ("the `code` is returned once, here — never retrievable again from any endpoint") directly contradicts FR-3 (`/qr` returns `code` + `payload` repeatedly to the initiating admin until terminal). | Reword FR-1: the code is returned in the `initiate` response **and** re-fetchable via `/qr` by the initiating admin while the session is non-terminal. Consistent with finding 1's encrypted-at-rest model. |
| 3 | Blocking | Index / process | `roadmap/13-network-access/README.md` Scope In still lists "Session cookie attributes (`Secure`, `SameSite`, `Domain`)" and Exit criteria still list "CSRF protection tested against phase 12's chosen session mechanism" and "Session cookie attributes verified correct" — all resolved by ADR 0028 §5 as *no token machinery* / *N/A, no cookies*. Every spec's "each FR maps to an exit criterion" is unsatisfiable while the checklist contradicts the governing ADR. | Amend the phase-13 roadmap Scope In + Exit criteria: cookie-attribute items → `N/A` with a pointer to ADR 0028 §5; CSRF criterion → the `Origin`-validation-plus-recorded-rationale form. |
| 4 | Major | Spec ownership (§1) | `backend-network-api.md` FR-9 changes `POST /api/v1/auth/login`'s contract (owned by `backend-authentication.md`); FR-2 / Security mandate an `X-Library-Id` check in `AuthMiddleware` (owned by `backend-authentication.md` FR-7 / `backend-library-namespaces.md`); `frontend-network-and-pairing.md` extends the phase-12 `LoginScreen` and adds a `'network'` capability (`frontend-shell-and-routing.md` FR-4). The specs *claim* "Amends …" but the owned files are untouched. | Write the actual amendment text into `backend-authentication.md`, `backend-library-namespaces.md`, `frontend-shell-and-routing.md` / `frontend-auth-and-tenancy.md` before those specs reach `APPROVED`. |
| 5 | Major | Architecture | The middleware-order change (HSTS + CORS before limits/logging; new global rate-limit and Origin-validation layers) is narrated only in `backend-network-transport.md`; `backend-http-transport.md` FR-1 (the spec that *fixes* the chain order, citing `architecture-backend.md` FR-6) is untouched apart from a CORS Non-goals bullet. | Amend `backend-http-transport.md` FR-1 with the full new ordered chain + rationale; confirm `architecture-backend.md` FR-6 permits it. |
| 6 | Major | ADR 0017 conformance | `backend-network-transport.md` FR-2 requires behaviour "when the host portion of `BIND_ADDRESS` is a DNS name" but the live classifier (`internal/config/bindaddress.go` `isLoopbackOrPrivate`) rejects every non-`localhost` hostname outright and never resolves DNS. The actual classification rule for a named bind — the one security-load-bearing function for the whole gate — is unstated. | FR-2 states it explicitly: a host that is neither an IP literal nor `localhost` is classified **publicly routable without resolution** (Mode A required, fail-closed); never resolved. Add classifier test rows (`library.example.com:443` → public; `foo.local:8080` → public). |
| 7 | Major | Scope / substance | `backend-network-api.md` FR-5 accepts and persists `rememberDeviceDays` (1..90); nothing consumes it. If it overrides ADR 0025's fixed 30-day refresh-token lifetime, that is an unstated change to `backend-authentication.md` / ADR 0025. | Wire it: refresh-token issuance reads the setting. Amend `backend-authentication.md` + ADR 0025; a test proves a changed value changes the issued token's `expires_at`. |
| 8 | Major | Design-conformance gate | The specs cite `fr4` / `atFirstRun` (classified **Unclassified** in `ANALYSIS.md`'s 2026-08-17 ledger) and `sgDevices` (**no ledger row**) as "Binding". CLAUDE.md: an unclassified surface is a stop-and-ask, not a default. | Drop `fr4` and `sgDevices` as cited authority. Rest D-2/D-4 on `sgNetwork`'s "Advanced panel" row (Binding, contents uncaptured) and the roadmap text. Flag the Unclassified status in the specs as an open maintainer item. |
| 9 | Major | Design-conformance | A fifth captured contradiction was unrecorded: the "Allow access from this network" toggle (`Alexandryn-Electron-Admin.dc.html:223`) contradicts C-9 (bind is restart-only config) the same way D-2's "Require authentication" toggle does. | Record as **D-5** in the plan and every relevant spec's contradiction list; resolve identically — toggle → read-only reachability statement. |
| 10 | Major | Domain / logic | `backend-network-api.md` FR-9's device-ownership logic has an unreachable `else` (`owner_id` is set to `InitiatedBy` at verify, then "if it still matches `InitiatedBy`" is always true) and lets a `reader` who logs into an admin-paired device silently take ownership. | Define ownership: the device belongs to whoever completes login with the grant; the initiating admin is not an owner. Remove the unreachable branch. A grant whose session was `InitiatedBy` a different user is ignored (I-4). Specify a second user's login from a shared device. |
| 11 | Major | Testability | `domain-device-pairing.md` FR-2 treats decoded entropy as something the `PairingCode` constructor validates; a constructor over an untrusted string can only check shape/length. | FR-2 → a fixed 8-char Crockford shape/length invariant on the constructor; the ≥40-bit entropy assertion moves to `GeneratePairingCode`'s acceptance criteria (`backend-network-transport.md` FR-9). |
| S-H2 | **High** (security pass) | Missing hardening / clickjacking / XSS | The SPA and every route are served with **no** `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`. The only CSP is on the reader iframe. Opening the bind makes clickjacking of authenticated actions possible and leaves XSS→`localStorage`-token theft with no backstop; `backend-network-transport.md` tells the audit to "re-confirm" a CSP that does not exist. | New FR in `backend-network-transport.md`: a security-headers middleware on **all** binds — app-origin CSP (`frame-ancestors 'none'`, `default-src 'self'`, `object-src 'none'`, `base-uri 'none'`, no `unsafe-inline` script-src), `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, minimal `Permissions-Policy`. Audit checklist item reworded "establish", not "re-confirm". |
| S-H4 | **High** (security pass) | Cleartext on a LAN bind | A private-range bind (`192.168.x.y:port`) serves everything cleartext — bearer tokens, `DEVICE_PAIRING_SECRET` in request bodies, book bytes. ADR 0017 permits it (proxy "may" terminate TLS), but the panel copy doesn't disclose it and no in-process-TLS option exists for a private bind. | (1) `frontend-network-and-pairing.md` FR-1: when `tlsMode == "none"` and `reachability != "loopback"`, state plainly that LAN traffic is unencrypted unless a reverse proxy terminates TLS. (2) startup `warn` log. (3) **opt-in in-process TLS for a private bind** — `TLS_CERT_FILE`/`TLS_KEY_FILE` present ⇒ `ServeTLS` (no ACME). Recorded as an ADR 0017 Mode-B sub-case in ADR 0028 §1. |
| S-M1 | Medium (security pass) | Race condition | `backend-network-api.md` FR-2's "two devices submit the same code → second gets 404" only holds if the verify transaction serialises on the session row; FR-2 does not require a row lock. | FR-2 specifies `SELECT … FOR UPDATE` on the `pairing_sessions` row before the domain `Verify`→`Consume`; a concurrent double-submit integration test joins the must-fail-first list. |
| S-M3 | Medium (security pass) | Origin allow-set | `backend-network-transport.md` FR-7 derives the server origin from "bind scheme + `BIND_ADDRESS`"; a LAN SPA is reached at the interface IP or `alexandryn.local`, neither of which equals a `0.0.0.0` bind literal, and `CORS_ALLOWED_ORIGINS` is empty by default — so legitimate same-origin `pair/verify` is rejected, or the check gets loosened. | FR-7 enumerates the accepted origin set: every non-loopback interface address at the bound port, the configured mDNS `hostName`, any `CORS_ALLOWED_ORIGINS` entry. Multi-address test matrix. |
| S-M4 | Medium (security pass) | Over-disclosure | `backend-network-api.md` FR-4 `/network/status` returns the full interface address list + `acmeDomain` to any authenticated account, including a `reader` invited to one library — the host's whole network topology. | Split the response by role: `reader` gets `reachability` + `tlsMode` + the one address they connected on; `admin` gets `addresses[]` + `acmeDomain`. `ACME_EMAIL` added to the never-include list. |

**Minor / Informational** (folded into the rework, tracked in
`implementation-plan.md` Gate 1.5): stale `claimed` state still in the
plan; static-cert public bind has no `:80` redirect; enrolment grant not
single-use; distributed-guesser note for `pair/verify`; `tlsMode` UI copy
untrue for `private` + `none`; `EnrolledVia = password_login` / `Touch()`
/ `LastSeenAt` have no phase-13 producer (defer to phase 14 or name the
caller); Pebble §9 record missing "what breaks if abandoned"; npm
`qrcode` §9 record + `check:bundle-size` headroom not yet done; Let's
Encrypt TOS auto-accepted for the operator without surfacing it;
`pairing_sessions.initiator_ip` retention unbounded; `implementation_plan.md`
at repo root (moved to `.claude/roadmap/13-network-access/`); ADR 0028
title is a paragraph; `paired_devices.pairing_session_id` FK needs
`ON DELETE SET NULL`; `hostName` regex allows leading/trailing hyphen;
`/connect?c=` code should be stripped via `history.replaceState`; ACME
`:80` listener not rate-limited / not path-restricted; `IsPublicPath`
default-open for any non-`/api/v1/` prefix; HTTP/2 rapid-reset toolchain
pin + stream-limit note; `DEVICE_PAIRING_SECRET` value proposition
overstated in ADR 0028 §6.

## Dimensions checked

- [x] **Completeness** — gaps found (findings 1, 4, 5, 7, S-H2, S-M3; several "no producer" fields).
- [x] **Ambiguity** — findings 6, 11, S-M1, S-M3 (two engineers would build different things).
- [x] **Architecture** — finding 5 (middleware order not amended into its owning spec); domain boundary of `domain-device-pairing.md` confirmed clean.
- [x] **Domain correctness** — metadata/source/library boundaries intact in the new specs; the phase-12 defect P12-1 is a library/user-scoping failure in *existing* code.
- [x] **Security** — full four-attacker + STRIDE pass; transport core sound; P12-1/P12-2/S-H2/S-H4 are the material gaps.
- [x] **Testability** — finding 11; several specs now name concurrent/IDOR tests that must fail first.
- [x] **Accessibility** — `frontend-network-and-pairing.md` FR-7 reviewed; axe gate present; copy findings folded in.
- [x] **UX and copy** — `tlsMode` honesty (S-H4), reachability copy, Let's Encrypt TOS surfacing.
- [x] **Observability** — startup `warn` for plaintext-private-bind added (S-H4); never-log lists reviewed.
- [x] **Maintainability** — directives added to `CLAUDE.md` + templates so the P12-1 class of miss is caught next time.
- [x] **Evolution** — the H-1 fix explicitly preserves the future library-visible "completed by" / most-read read model (phase 15).
- [x] **Mechanism properties** — finding 1's fix changes the code-storage mechanism; `domain-device-pairing.md`'s constant-time-compare and single-use properties are restated against the encrypted-at-rest model and shown to still hold.
- [x] **Indexes** — `specs/README.md`, `decisions/README.md`, `roadmap/README.md`, `roadmap/13-*/README.md` updated; phase-13 Exit criteria amendment (finding 3) is part of the rework; `roadmap/12-*/README.md` gets the correction note.

## Contradictions and gaps

- Findings 1, 2, 3, 9 are contradictions (spec-internal, spec-vs-ADR, spec-vs-roadmap, design-vs-decision).
- Audit `0012` vs. the wired code (P12-1, P12-2, P12-3) — the most serious gap: a security audit certified controls that are not in the code.
- `ANALYSIS.md` ledger vs. the specs' design-reference headers (finding 8).

## What I did not review

- The phase-12 code beyond the reading/auth surface the two defects touch — RBAC on the library-management endpoints, the refresh-token rotation logic, Argon2id parameters, TOTP — were spot-checked against audit `0012`'s claims but not independently re-audited. **Given P12-1/P12-2/P12-3, a fuller re-audit of the phase-12 authorization surface is warranted before phase 12 closes** — recorded as a directive in `roadmap/12-authentication/README.md`, out of scope for this review.
- The `.design-reference` canvases beyond the four network/pairing surfaces.
- Test-plan-level detail (phase 13 test plans are not yet written — ADR 0016 cadence).

## Resolution (2026-09-02)

The full finding→resolution map is in
`roadmap/13-network-access/implementation-plan.md` "Gate 1.5". Summary:

**Spec-package findings (Blocking + Major + Minor):** all addressed in a
rework pass over ADR 0028 and the four specs.
- **B1/B2** — pairing codes now stored **encrypted at rest** (AES-256-GCM,
  `pairing-code-enc-v1` subkey) with a blind HMAC index; FR-1/FR-3
  reconciled (code re-fetchable via `/qr` by the initiating admin until
  terminal, by nobody else). ADR 0028 gains Option F recording why
  hash-only was rejected.
- **B3** — `roadmap/13-network-access/README.md` Scope In + Exit criteria
  amended: cookie-attributes → N/A (ADR 0028 §5), CSRF criterion reworded,
  CSP criterion + phase-12 hardening-prelude criterion added.
- **M1** — amendment text written into `backend-authentication.md`
  (FR-3/FR-4/FR-7), `backend-library-namespaces.md` (FR-3/FR-4),
  `backend-reading-api.md` (new FR-9), `backend-reader-content.md`
  (FR-1), `architecture-backend.md` FR-6, `backend-http-transport.md`
  FR-1, ADR 0025.
- **M2** — middleware chain reconciled with `architecture-backend.md`
  FR-6 (the single reserved auth slot expands into an ordered group;
  recovery stays outermost, limits/logging keep their phase-03
  positions).
- **M3 / S-M2** — `backend-network-transport.md` FR-2 states the
  DNS-name classification rule explicitly (name ⇒ public, no resolution).
- **M4** — `rememberDeviceDays` wired to refresh-token lifetime `[1,90]`
  (ADR 0028 §10, ADR 0025 amendment, `backend-authentication.md` FR-3/4).
- **M5** — `fr4`/`sgDevices` dropped as design authority across all
  specs' headers; `fr4`'s Unclassified status flagged as a stop-and-ask
  in `frontend-network-and-pairing.md` Open questions.
- **M6** — D-5 (the "Allow access from this network" toggle) recorded and
  resolved (read-only statement).
- **M7** — `PairedDevice` ownership assigned at login to the
  authenticating user; the unreachable branch removed.
- **M8** — `PairingCode` FR-2 reworded to a shape/length invariant;
  entropy assertion moved to `GeneratePairingCode`.
- **S-H2** — `backend-network-transport.md` FR-5a: security-headers /
  app-origin CSP middleware on every bind; ADR 0028 §9; audit item
  reworded "establish".
- **S-H4** — opt-in in-process TLS for a private bind (ADR 0017 Mode-B
  sub-case, ADR 0028 §1); honest `tlsMode` copy in the panel
  (`frontend-network-and-pairing.md` FR-1); startup `warn` on a
  plaintext non-loopback bind.
- **S-M1** — `backend-network-api.md` FR-2: `SELECT … FOR UPDATE` row
  lock on the verify transaction + a concurrent-double-submit
  integration test in the must-fail-first list.
- **S-M3** — `backend-network-transport.md` FR-7: Origin allowed-set
  enumerated (interface addrs + mDNS name + CORS origins); absent Origin
  allowed.
- **S-M4** — `/network/status` split by role; `ACME_EMAIL` on the
  never-include list.
- **Minors** — all folded (Gate 1.5 table): `:80` redirect on every
  public bind, single-use grant, `ON DELETE SET NULL`, tightened
  `hostName` regex, strip `?c=` via `replaceState`, rate-limited ACME
  `:80`, `initiator_ip` retention bound, `qrcode` §9 record, HTTP/2
  rapid-reset note, Let's Encrypt TOS surfaced, ADR title trimmed, plan
  moved to `roadmap/13-*/`.

**Phase-12 defects (P12-1/C1, P12-2/C2, P12-3/C3, P12-4):** audit `0012`
carries a "Post-audit correction" section (honest severity, corrective
directives, where each is enforced). Forward directives added to
`CLAUDE.md` (2 Reflexes + 1 "easy to get wrong"),
`.claude/templates/audit.md` (mandatory call-path-tracing), and
`.claude/templates/spec.md` (name the enforcement point + the
cross-tenant refusal test). `roadmap/12-authentication/README.md` gets a
Correction section — phase 12 cannot close until C1/C2 are fixed +
re-verified, `X-Library-Id` is claim-checked, and the phase-12
authorization surface gets a fuller re-audit. The C1/C2 fixes ship on
`feat/phase13-network-access` as a hardening prelude and are part of the
phase-13 close gate; audit `0013` re-verifies them.

**Post-rework status:** ADR 0028 `Accepted`; the four phase-13 specs move
`DRAFT → APPROVED` (scope maintainer-approved at Gate 1; independent
review + rework complete). Phase-12 spec amendments are `DRAFT`, pending
maintainer re-confirmation, and are non-blocking for phase-13
implementation start (the amendments describe what the hardening prelude
builds). A second independent review pass on the reworked package was not
run — the rework is finding-directed and traceable; the maintainer may
call for one.
