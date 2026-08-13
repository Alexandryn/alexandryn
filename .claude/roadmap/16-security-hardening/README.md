# Phase 16 — Security hardening

*Outline — expanded to a full phase document when phases 14, 15 close.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 14, Phase 15 |
| **Blocks** | 17 |

## Objective

Consolidated adversarial review across all three trust boundaries named in
phase 01, ahead of external-review readiness. Not the only security gate —
Constitution §10 makes every phase carry its own audit — but the point where
findings get looked at together instead of in isolation.

## Scope

**In**

- Threat model consolidation against Constitution §4–§8
- Adversarial review of Renderer/Main, Host/LAN, System/Source boundaries
- Dependency vulnerability and license audit, CSP review

**Out**

- Ongoing post-release maintenance audits.

## Exit criteria

- [ ] Consolidated audit recorded, zero open Critical or High findings
- [ ] Dependency scan clean
- [ ] Full compliance verified against constitutional security requirements
- [ ] Maintainer approval recorded
