# Phase 14 — Devices and sync

*Outline — expanded to a full phase document when phases 11, 13 close.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 11, Phase 13 |
| **Blocks** | 16 |

## Objective

Reading progress, bookmarks, and highlights synchronised across a
household's authorized devices, with explicit conflict resolution for
concurrent offline updates.

## Scope

**In**

- Device registry and session management
- Sync protocol and conflict resolution for progress/bookmarks/highlights

**Out**

- Any server-mediated relay beyond the LAN.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Two devices can disagree about whether they're reporting progress against the same file — one direction of sync agreeing, the other silently not, for what should be identical content | Medium | Progress appears to sync but doesn't; a user sees stale progress on one device with no visible error | Not yet designed — `domain-reading.md`'s Open questions records the gap ([`0049`](../../reviews/0049-real-world-edge-case-conformity-review.md), finding 8); this phase's own sync-protocol spec needs to state what "the same file" means to two independently-reporting devices before FR-6's reconciliation math is asked to run on it |

## Exit criteria

- [ ] Progress syncs correctly across two or more devices on LAN
- [ ] Conflict resolution tested against concurrent offline edits
- [ ] Device management UI supports revocation
- [ ] Maintainer approval recorded
