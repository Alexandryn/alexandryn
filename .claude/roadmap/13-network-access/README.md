# Phase 13 — Network access

*Outline — expanded to a full phase document when phase 12 closes.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 12 |
| **Blocks** | 14, 15 |

## Objective

Controlled LAN exposure, now that phase 12 makes it safe: binding
configuration, device pairing, transport security, and CORS — all off by
default, requiring explicit action to enable.

## Scope

**In**

- LAN interface binding as an explicit, off-by-default setting
- Local TLS, device pairing (e.g. QR code), CORS/CSRF hardening

**Out**

- Any WAN/cloud relay — this project is LAN-only self-hosting.

## Exit criteria

- [ ] Loopback remains the default; LAN binding requires explicit admin action
- [ ] Authentication enforced on every LAN-reachable route, no exceptions
- [ ] TLS functional for local connections
- [ ] Security audit recorded, no open Critical or High findings
- [ ] Maintainer approval recorded
