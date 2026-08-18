# Phase 13 — Network access

*Outline — expanded to a full phase document when phase 12 closes.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 12 |
| **Blocks** | 14, 15 |

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
  configured address/domain, not a fixed default
- CSRF hardening, dependent on phase 12's session-mechanism ADR
- Session cookie attributes (`Secure`, `SameSite`, `Domain` scope), which
  differ between a bare LAN IP and a real domain — not previously named,
  added per the same review
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

- [ ] Loopback remains the default; broader binding requires explicit admin action
- [ ] Authentication enforced on every reachable route, no exceptions, under both bind modes
- [ ] TLS functional for local connections (upstream or in-process) and for a public bind (in-process, certificate validated at startup)
- [ ] ACME issuance and renewal tested against a real certificate lifecycle, not just a fixture
- [ ] CORS origin configuration verified to track the deployment's actual configured address
- [ ] CSRF protection tested against phase 12's chosen session mechanism
- [ ] Session cookie attributes verified correct for both a LAN-IP bind and a domain bind
- [ ] General API rate limiting functional, not only on the login endpoint
- [ ] Security audit recorded, no open Critical or High findings
- [ ] Maintainer approval recorded
