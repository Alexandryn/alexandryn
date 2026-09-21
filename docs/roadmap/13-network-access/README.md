# Phase 13 — Network access

| | |
|---|---|
| **Status** | **Closed** — merged to `main` via [PR #80](https://github.com/Alexandryn/alexandryn/pull/80) on 2026-09-05 (merge commit `369190f`). All exit criteria met; security audit `0013` recorded with no open Critical/High findings and AUDIT-0012-C1/C2 re-verified fixed. Maintainer approval recorded via the PR merge itself. Four specs `APPROVED`; ADR 0028 `Accepted`. |
| **Depends on** | Phase 12 |
| **Blocks** | 14, 15 |

## Governing documents

- **ADR** — [`0028`](../../decisions/0028-phase13-network-transport-and-pairing.md)
  (TLS mode derived not flagged; ACME via `autocert`, no new module;
  CORS deny-by-default; no CSRF tokens under Bearer auth; device pairing
  as a thin bootstrap over phase-12 accounts; authentication never
  disable-able). Also [`0017`](../../decisions/0017-network-exposure-condition-gated.md).
- **Specs** (all `DRAFT`, scope approved at Gate 1):
  - [`domain-device-pairing.md`](../../specs/domain-device-pairing.md)
  - [`backend-network-transport.md`](../../specs/backend-network-transport.md)
  - [`backend-network-api.md`](../../specs/backend-network-api.md)
  - [`frontend-network-and-pairing.md`](../../specs/frontend-network-and-pairing.md)
- **Amends** — `backend-configuration.md` FR-4 (six new keys) / FR-8
  (interim note), `backend-http-transport.md` Non-goals (CORS
  un-deferred), `backend-authentication.md` (login `enrolmentGrant`,
  `X-Library-Id` claim check).
- **Plan + Gate 1 outcome** — [`implementation-plan.md`](implementation-plan.md) (this directory).
- **Spec-package review (two independent agents) + phase-12 authorization
  correction** — `../../reviews/0050-phase13-spec-package-and-phase12-authz-review.md`.

## Gate 1 decisions (2026-09-02)

C-0 no `TLS_ENABLED`; C-1(a) account login + thin pairing; C-2 no
"auth off" path anywhere; C-3 phase 13 = pair + revoke-a-pairing,
phase 14 = device inventory; C-4 client-side QR from a payload string;
C-5 CORS deny-by-default; C-6 no CSRF tokens, `Origin` check on pairing
routes; C-7 ACME lifecycle tested with Pebble; C-8 no `PORT` key; C-9
`PATCH /network/settings` runtime-safe values only. **Close gate:**
`X-Library-Id` validated against the JWT `libraries` claim before the
phase closes (carryover from the phase-12 auth middleware).

## Objective

Controlled network exposure, now that phase 12 makes it safe: binding
configuration, device pairing, transport security, and CORS/CSRF — all off
by default, requiring explicit action to enable, and legal only under
ADR 0017's two fail-closed modes (upstream TLS on a private-only bind, or
in-process TLS with a validated certificate on a public bind).

## Scope

**In**

- Bind-address configuration as an explicit, off-by-default setting —
  loopback (default), LAN/private-range, or a publicly routable address
  with a certificate, per ADR 0017's classification in
  `backend-configuration.md` FR-8
- Local TLS and, for a public bind, in-process TLS with certificate
  provisioning — ACME/Let's Encrypt issuance and automatic renewal, not
  previously named in this outline, added per the 2026-08-18
  security-and-hardening review
- Device pairing (e.g. QR code)
- CORS hardening, with the allowed origin tied to the deployment's actual
  configured address/domain, not a fixed default — deny-by-default
  (ADR 0028 §4)
- CSRF hardening — **resolved by ADR 0028 §5 as no synchroniser-token
  machinery**: phase 12 chose Bearer-token sessions (ADR 0025), not
  cookies, so there is no ambient credential a forged cross-site request
  can ride. The concrete work is `Origin`/`Referer` allow-list validation
  on the unauthenticated pairing routes plus a recorded rationale
  (constitution §12). The line below was written expecting cookie
  sessions and no longer applies:
- ~~Session cookie attributes (`Secure`, `SameSite`, `Domain` scope)~~ —
  **N/A. No cookies exist** (ADR 0025 / ADR 0028 §5). The security value
  these attributes would have provided is delivered instead by the
  security-headers/CSP middleware (ADR 0028, review 0050 finding S-H2)
  and by the Bearer token not being attached cross-site by the browser.
- Security response headers / app-origin CSP — added per review 0050
  (S-H2): the SPA is currently served with no CSP, `X-Frame-Options`, or
  `nosniff`; a security-headers middleware on all binds is in scope
  (`backend-network-transport.md`)
- General API rate limiting, not only the login endpoint — once reachable
  beyond a trusted LAN, unauthenticated surfaces (health checks, static
  assets) are a broader target than a LAN-only threat model needs to
  budget for; not previously named, added per the same review
- Settings → Network → Advanced (design reference, `Electron-Admin.dc.html`)
  is this phase's UI home for bind address, certificate, and domain
  configuration — the panel is captured, its contents are not; see
  `.design-reference/ANALYSIS.md`'s classification ledger

**Out**

- Any Alexandryn-operated relay, tunnel, or traffic-mediating
  infrastructure. Alexandryn operates no infrastructure and mediates no
  user's traffic, under any bind mode — this does not change.
- **A first-run / setup-wizard network step.** The design prototype's
  `fr4` ("Network access" step inside the first-run flow) is
  **deliberately out of scope for phase 13** (maintainer decision,
  2026-09-03). Phase 13 builds the standing **Settings → Network** panel
  and the pairing flow. First run is its own flow with its own scope; a
  network step there — if one is wanted — is designed and specified with
  the rest of the first-run experience, reusing this phase's
  `NetworkSettings` component rather than duplicating it. `.design-reference/ANALYSIS.md`
  classifies `atFirstRun` as **Unclassified**; this decision does not
  promote it — it defers the whole first-run surface.

Previously this section also excluded "any WAN/cloud relay" under the
single claim "this project is LAN-only self-hosting." That conflated two
different things: Alexandryn operating infrastructure on a user's behalf
(declined, above, unchanged) and where a user's own instance is reachable
from (their decision — ADR 0017 makes user-operated remote deployment,
at the user's own domain with the user's own certificate, a supported
configuration, LAN remaining the default). Phase 14's own Scope Out
("Any server-mediated relay beyond the LAN") used the same ambiguous
"relay" wording — read literally it already excluded only a mediated
hop, not a direct connection, so it was never actually in tension with
this; it stays as written and now reads correctly against this phase's
split, without needing its own edit.

## Exit criteria

- [x] Loopback remains the default; broader binding requires explicit admin action
- [x] Authentication enforced on every reachable route, no exceptions, under both bind modes
- [x] TLS functional for local connections (upstream, or opt-in in-process on a private bind) and for a public bind (in-process, certificate validated at startup, ACME or static)
- [x] ACME issuance and renewal tested against a real certificate lifecycle (Pebble in the integration suite), not just a fixture
- [x] CORS origin configuration verified to track the deployment's actual configured address (deny-by-default; exact-match allowlist)
- [x] `Origin`/`Referer` validation on the unauthenticated pairing routes tested against a multi-address (LAN-IP + mDNS name) deployment; the no-synchroniser-token decision recorded with its rationale (ADR 0028 §5)
- [x] Security response headers established: an app-origin `Content-Security-Policy` with `frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy` — on all binds, verified present on every response
- [x] Session cookie attributes — **N/A, no cookies** (ADR 0025 / ADR 0028 §5); recorded in the exit-criteria walk, not implemented
- [x] General API rate limiting functional on unauthenticated public surfaces (health, static assets, pairing), not only on the login endpoint; keyed on `RemoteAddr`, not a client header
- [x] Plaintext-on-a-LAN-bind is disclosed to the operator in the Network panel and logged once at `warn` on startup (review 0050 S-H4)
- [x] **Phase-12 hardening prelude verified** (carried into this phase per review 0050): every reading/reader endpoint enforces per-user + per-library scoping at the query layer, with a per-endpoint IDOR test and an `export` isolation test; `X-Library-Id` validated against the JWT `libraries` claim; access-token verification asserts the token type; the enrolment grant uses a distinct signing subkey
- [x] Security audit `0013` recorded, no open Critical or High findings, and it re-verifies AUDIT-0012-C1/C2 as fixed
- [x] Maintainer approval recorded — [PR #80](https://github.com/Alexandryn/alexandryn/pull/80) merged 2026-09-05
