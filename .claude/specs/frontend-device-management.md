# Spec: Frontend device management

| | |
|---|---|
| **Status** | `DRAFT` |
| **Phase** | `14-devices-and-sync` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-09-05 |
| **Last updated** | 2026-09-06 |
| **Supersedes** | — |
| **Reviewed in** | — |
| **Design reference** | ANALYSIS.md sync date consulted: 2026-09-05 (re-pulled from the design project; content is byte-identical to the 2026-08-13 sync — nothing changed on the design side). Canvas files consulted: `Alexandryn-Electron-Admin.dc.html`, `Alexandryn-Web.dc.html`, `Alexandryn-Mobile.dc.html`. A device-management screen exists in `Alexandryn-Electron-Admin.dc.html` — see "Design reference" below. An earlier version of this file claimed no canvas existed anywhere; that was wrong (see Correction). |

## Correction (2026-09-06)

This spec originally recorded a stop-and-ask: no device-management canvas found, blocked
pending a DesignSync capture. That was incorrect. A re-pull of the design project returned
content byte-identical to what was already committed — the screen had been captured the whole
time, in `Alexandryn-Electron-Admin.dc.html`, as a **Settings sub-tab** (`sgDevices`,
gated on `settingsTab==='devices'`), not a separate top-level route. The original check searched
for a literal `atDevices`/`atDeviceManagement` route and found none, which is true but was the
wrong question — `.design-reference/ANALYSIS.md` had already filed the sub-tab correctly under
a wide "`atSettings` (remaining tabs) — Unclassified" bucket, which this spec's check read past
without opening. `ANALYSIS.md` has since been corrected to give the Devices tab its own row,
classified Binding for this phase (`roadmap/14-devices-and-sync/README.md`, "Design reference
correction", 2026-09-05).

This file is rewritten below against what the canvas actually shows, and is `DRAFT` — self-review
and `APPROVED` still need to happen, they were never blocked on a missing canvas.

## Design reference

**Electron Admin — `atSettings` → Devices tab** (`sgDevices`), one of six Settings tabs
(metadata/network/security/storage/devices/advanced). Content:

```
Devices that have connected to this host. Revoking a device ends its session immediately.

[dot] MacBook Pro          192.168.1.24   Active now              Owner              
      This device · Host

[dot] Pixel 8              192.168.1.41   Active 4 minutes ago    Remembered device   Revoke
      Phone · Chrome

[dot] iPad Air             192.168.1.52   Yesterday, 22:10        Expires in 26 days   Revoke
      Tablet · Safari

[dot] Study iMac           192.168.1.18   3 days ago              Remembered device    Revoke
      Desktop · Safari
```

Per row: a status dot (green = active, grey = idle), device name, device kind + client
("Phone · Chrome", "This device · Host"), IP address, a last-seen text, a session descriptor
("Owner" / "Remembered device" / "Expires in *N* days"), and an inline "Revoke" text action
(styled in the error color, not a button). The current device ("This device · Host") has no
Revoke action — a device cannot revoke itself from this list.

**No confirmation dialog is captured for Revoke.** Left as an open question below.

**Web (`atAccess`, "This session") and Mobile ("THIS DEVICE")** — a *different*, self-service
screen: the current browser/device's own session only (hosting mode, permissions list, "Sign out
of this browser" / "Sign out of this device"), not a list of other devices and no revoke control
over them. This spec's device *list* is host-only; the self-service sign-out screen is a
separate, smaller piece — see Scope, In, below.

**Two things the canvas also surfaces, not resolved by this spec:**

- `atConnect` (Web canvas's join screen) shows a "Library passphrase" + "Remember this browser
  for 30 days" flow with a fallback pointing to "Settings → Devices → Pending" for host approval.
  This does not match phase 13's actual shipped join mechanism (account login + QR pairing, no
  passphrase, no pending-approval state — Gate 1 decision C-1(a)). The maintainer has not
  confirmed whether this is stale content predating that decision or a still-wanted second join
  path. **This spec does not build the passphrase/pending-approval flow.** Whoever implements
  the Web-side session screen must ask before assuming either way.
- `atAccess` also carries an "OFFLINE COPIES" card (cached-book count/size). That's the
  offline-file-cache element `.design-reference/ANALYSIS.md` already excludes from this phase
  (coupled to the unresolved cloud-relay question). This spec's self-service session view
  (hosting mode, permissions, sign-out) does not include it.

## Context

`backend-device-sync.md` (`APPROVED`) builds `GET /api/v1/devices` and
`DELETE /api/v1/devices/{id}`, both user-scoped, but no UI calls them yet — `FindByOwner` is
wired to a handler for the first time in this phase and has no frontend consumer. This spec is
that consumer, for the host-side device list. The Web/Mobile self-service session view is a
separate, smaller slice against the same underlying session/device state, already partially
built (`atAccess`/"THIS DEVICE" exist as canvases; whether their "Sign out" action already has a
backend endpoint or needs one is Open questions, below).

## Scope

**In**

- A device list view in Settings (Electron Admin), rendering `GET /api/v1/devices`'s response:
  label, device class, enrolled-via, created-at, last-seen-at, last-synced-at, revoked status.
  Matches the canvas's row shape (status indicator, name + kind, last-seen, a session-state
  label) using the fields the backend actually returns — see "Field mapping" below for what the
  canvas shows that the API does not.
- Revocation: an inline "Revoke" action per non-current device, calling
  `DELETE /api/v1/devices/{id}` (`backend-device-sync.md` FR-2), with a confirmation step before
  the destructive call fires (not captured in the canvas — this spec decides the confirmation UI,
  see Open questions).
- The current device is visually distinguished ("This device · Host" in the canvas) and has no
  Revoke action — a device cannot revoke its own active session from this list.
- Keyboard navigability: the list is traversable by keyboard; Revoke is reachable without a
  mouse; the confirmation step (however it's built) traps focus and is dismissable via Escape.
- Screen-reader announcement of the revocation confirmation prompt and its result.
- Empty/single-device state: a user with only their current device sees it listed, not a blank
  list or an empty-state illustration the canvas doesn't show.
- The Web/Mobile self-service "This session" / "THIS DEVICE" sign-out action, scoped to exactly
  what those canvases show: hosting mode, a permissions list, "Sign out of this browser/device."
  Excludes the OFFLINE COPIES card (see Design reference, above) and the passphrase/pending-
  approval join flow (same).

**Out**

- Device renaming/relabeling — not shown in the canvas; the label is set at pairing time.
- Per-device reading history or activity — phase 15.
- Mobile native app device management beyond the captured "Sign out of this device" — no native
  mobile client in this roadmap.
- The `atConnect` passphrase/pending-approval flow (Design reference, above) — not built until
  the maintainer confirms it's still wanted.
- The OFFLINE COPIES card on `atAccess` (Design reference, above) — out of this phase's sync
  scope per `.design-reference/ANALYSIS.md`.

## Field mapping — canvas vs. API (open question, needs a decision before FRs are finalized)

The canvas's device rows show an IP address and a session descriptor ("Owner" / "Remembered
device" / "Expires in *N* days") that `backend-device-sync.md`'s `GET /api/v1/devices` response
does not return (its fields: `id`, `label`, `deviceClass`, `enrolledVia`, `createdAt`,
`lastSeenAt`, `lastSyncedAt`, `revokedAt`). Two ways to close this, not decided here:

1. Render only what the API provides — drop the IP and session-expiry columns, keep the rest.
   Simplest, no backend change, but visually thinner than the canvas.
2. Amend `backend-device-sync.md` FR-1 to add an IP and/or session-expiry field. `backend-
   device-sync.md` is already `APPROVED`; amending it is a real re-review, not a rename.
   An IP-per-device field also needs a constitution §8 check (this isn't reading content, but a
   device's network address is still worth a deliberate call, not a silent addition) before it's
   added to a persisted, user-visible field.

Recommendation for whoever finalizes this spec's FRs: option 1 for the first cut — it needs no
backend change and the list is still fully functional (name, kind, last-seen, last-synced,
revoke) without the IP/session-expiry columns. Revisit option 2 if the maintainer wants the
canvas matched exactly.

## Open questions

- Confirmation dialog UX for Revoke — not captured in the canvas. A destructive-action modal
  consistent with this app's existing conventions (check `ConnectScreen`/`DevicePairingModal`
  from phase 13 for the established pattern) is the likely answer, not a novel one.
- Field mapping (canvas IP/session-expiry vs. the approved API response) — see above.
- Whether the Web/Mobile "Sign out of this browser/device" action needs a new endpoint or reuses
  an existing session/logout mechanism from phase 12 — not traced in this pass.
- The `atConnect` passphrase/pending-approval flow — stale or still wanted (Design reference,
  above). Not this spec's call.
- Which surface (host vs. Web) owns device *revocation* specifically (as opposed to self-service
  sign-out) — resolved above: host-only, per the canvas. Recorded here so it isn't re-litigated.

## References

- `backend-device-sync.md` FR-1/FR-2 — `GET /api/v1/devices`, `DELETE /api/v1/devices/{id}`
- ADR 0029 — cross-device identity agreement (accepted); this spec's UI doesn't surface sync
  cursors directly, but `lastSyncedAt` in the device list comes from the same `PairedDevice`
  extension ADR 0029 Part 2 defines
- CLAUDE.md — design-conformance rule
- ADR 0003 — design canvas split
- `.design-reference/ANALYSIS.md` — classification table (re-verified 2026-09-05/06)
- `roadmap/14-devices-and-sync/README.md` — "Design reference correction" section, the same
  finding this spec's Correction section above summarizes
