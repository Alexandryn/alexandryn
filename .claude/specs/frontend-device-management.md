# Spec: Frontend device management

| | |
|---|---|
| **Status** | `APPROVED` |
| **Phase** | `14-devices-and-sync` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-09-05 |
| **Last updated** | 2026-09-06 |
| **Supersedes** | — |
| **Reviewed in** | Self-reviewed against `backend-device-sync.md` (`APPROVED`) and ADR 0029 (`Accepted`); self-review caught and fixed a real inconsistency between FR-1's re-fetch requirement and FR-5's original "remove the row" language (backend returns revoked devices too — see FR-1's `revokedAt == null` filter) |
| **Design reference** | ANALYSIS.md sync date consulted: 2026-09-05 (re-pulled from the design project via the DesignSync MCP tool; content is byte-identical to the 2026-08-13 sync — nothing changed on the design side). Canvas files consulted: `Alexandryn-Electron-Admin.dc.html`, `Alexandryn-Web.dc.html`, `Alexandryn-Mobile.dc.html`. A device-management screen exists in `Alexandryn-Electron-Admin.dc.html`. An earlier version of this file claimed no canvas existed anywhere; that was wrong — see Correction. **Neither the Antigravity/Sonnet-4.6 planning session nor the build session has access to the DesignSync MCP tool** — this file and `.design-reference/ANALYSIS.md` are the only source either of them has for the canvas content; both are committed in full below rather than referenced by pointer. |

## Correction (2026-09-06)

This spec originally recorded a stop-and-ask: no device-management canvas found, blocked
pending a DesignSync capture. That was incorrect. A re-pull of the design project (from a
session with DesignSync access) returned content byte-identical to what was already
committed — the screen had been captured the whole time, in `Alexandryn-Electron-Admin.dc.html`,
as a **Settings sub-tab** (`sgDevices`, gated on `settingsTab==='devices'`), not a separate
top-level route. The original check searched for a literal `atDevices`/`atDeviceManagement`
route and found none, which is true but was the wrong question — `.design-reference/ANALYSIS.md`
had already filed the sub-tab correctly under a wide "`atSettings` (remaining tabs) —
Unclassified" bucket, which this spec's check read past without opening. `ANALYSIS.md` has
since been corrected to give the Devices tab its own row, classified Binding for this phase.

## Context

`backend-device-sync.md` (`APPROVED`) builds `GET /api/v1/devices` and
`DELETE /api/v1/devices/{id}`, both user-scoped, but no UI calls them yet — `FindByOwner` is
wired to a handler for the first time in this phase and has no frontend consumer. This spec is
that consumer.

## Problem

A user cannot see which devices are paired to their account, or revoke one, through any screen.
The `PairedDevice` aggregate and its repository (phase 13) and the list/revoke endpoints
(`backend-device-sync.md`, this phase) exist with no UI in front of them.

## Goals

- An authenticated user (admin, on the host) can see every device paired to their account and
  revoke any of them.
- A user on any device (host, Web, Mobile) can see and end their own current session without
  needing the host-side device list.
- Both views match the captured canvas content exactly where a canvas exists, and name every
  place they don't (see Design reference and Open questions).

## Non-goals

- Device renaming/relabeling — not shown in the canvas; the label is set at pairing time
  (`backend-network-api.md`).
- Per-device reading history or activity — phase 15.
- Mobile native app device management beyond the captured "Sign out of this device" — no native
  mobile client in this roadmap.
- The `atConnect` passphrase/pending-approval join flow (Design reference, below) — not built
  until the maintainer confirms it's still wanted, independent of phase 13's shipped mechanism.
- The OFFLINE COPIES card on `atAccess` — out of this phase's sync scope
  (`.design-reference/ANALYSIS.md`).
- Identifying "which row in the list is the device I'm using right now" — see Open questions.
  Nothing in the current session/token carries a device identifier the frontend could match
  against the list; this spec does not invent one.

## User stories

- As **a user**, I want to see every device paired to my account, so I know what has access.
- As **a user**, I want to revoke a device I no longer use, so it can no longer read my library
  or sync my progress.
- As **a user on a shared or borrowed device**, I want to sign out of just this session, without
  needing to find and revoke it from the host's device list.

## Design reference

**Electron Admin — `atSettings` → Devices tab** (`sgDevices`), one of six Settings tabs
(metadata/network/security/storage/devices/advanced). Captured content:

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
(styled in the error color, not a button). **No confirmation dialog is captured for Revoke.**

**Web (`atAccess`, "This session") and Mobile ("THIS DEVICE")** — a *different*, self-service
screen: the current browser/device's own session only (hosting mode, a permissions list,
"Sign out of this browser" / "Sign out of this device"), not a list of other devices and no
revoke control over them.

**Two things the canvas also surfaces, not resolved by this spec:**

- `atConnect` (Web canvas's join screen) shows a "Library passphrase" + "Remember this browser
  for 30 days" flow with a fallback pointing to "Settings → Devices → Pending" for host approval.
  This does not match phase 13's actual shipped join mechanism (account login + QR pairing, no
  passphrase, no pending-approval state — Gate 1 decision C-1(a)). The maintainer has not
  confirmed whether this is stale content predating that decision or a still-wanted second join
  path. **This spec does not build the passphrase/pending-approval flow.**
- `atAccess` also carries an "OFFLINE COPIES" card (cached-book count/size) — the offline-file-
  cache element `.design-reference/ANALYSIS.md` already excludes from this phase.

## Functional requirements

### Host device list (Electron Admin, Settings → Devices)

- **FR-1** The Devices tab MUST call `GET /api/v1/devices` on tab activation and render every
  returned device whose `revokedAt` is `null` as a row — `backend-device-sync.md` FR-1 returns
  both active and revoked devices (for audit/history purposes on the server side); this screen
  renders only the active ones, matching the canvas (which shows no revoked-devices section).
  The list MUST NOT be fetched or cached client-side beyond the active session — every tab
  activation re-fetches, since revocation from another session must be reflected without a
  manual refresh workaround.
- **FR-2** Each row MUST display, from the API response: `label` (name), a kind line combining
  `deviceClass` and `enrolledVia` (e.g. "Phone · paired by code", "Tablet · paired by code" —
  the canvas's "· Chrome"/"· Safari" client suffix is not available from the API and MUST NOT be
  fabricated), `lastSeenAt` as a relative time string ("Active now" for < 5 minutes, "Active *N*
  minutes/hours ago", otherwise a date), and `lastSyncedAt` as a relative time string or "Never
  synced" if null. The status dot MUST reflect whether `lastSeenAt` is within the last 5 minutes
  (active/green) or not (idle/grey) — an approximation of the canvas's dot, computed client-side
  from data the API already returns, not a new backend field.
- **FR-3** The canvas's IP-address column and session-descriptor text ("Owner" / "Remembered
  device" / "Expires in *N* days") MUST NOT be rendered — `GET /api/v1/devices` does not return
  either. See Open questions for the option to add them later.
- **FR-4** Each row MUST have a "Revoke" action (inline text, matching the canvas's styling,
  not a button) that opens a confirmation step before calling
  `DELETE /api/v1/devices/{id}` (FR-5). Revoke MUST NOT fire on a single click with no
  confirmation — the canvas doesn't show one, but a destructive, irreversible, other-visible
  action (constitution's general caution around irreversible actions) still needs one; this
  spec's confirmation UI follows the same pattern already established in this codebase for a
  comparable action (`DevicePairingModal`'s Revoke, phase 13) rather than inventing a new one.
- **FR-5** Confirming revocation MUST call `DELETE /api/v1/devices/{id}`. On `204`, the row MUST
  be removed from the rendered list immediately (consistent with FR-1's filter — a revoked
  device never renders, whether removed locally now or picked up as already-revoked on the next
  fetch). On `404` (already revoked, or a race with another session revoking it first) or `409`,
  the UI MUST show an inline error and re-fetch the list (FR-1) rather than silently retry the
  same request.
- **FR-6** The confirmation dialog MUST trap focus while open, be dismissable via Escape, and
  return focus to the row's Revoke control on cancel. Its heading and body text MUST name the
  device label being revoked (e.g. "Revoke Pixel 8?") — not a generic "Are you sure?".
- **FR-7** The result of a revocation (success or failure) MUST be announced to screen readers
  via a polite live region, matching the pattern already used for pairing-code expiry
  announcements in `DevicePairingModal` (phase 13).
- **FR-8** The list MUST be fully keyboard-navigable: each row's Revoke control reachable via
  Tab, activatable via Enter/Space, with no mouse-only interaction.
- **FR-9** A user with exactly one device (their current one) MUST see it listed, not an empty
  state — there is no "you have no devices" case for an authenticated user, since a session
  always corresponds to at least one login. An `GET /api/v1/devices` response with zero rows is
  treated as a loading/error state to investigate, not a valid empty state to design for.

### Self-service session view (Web `atAccess`, Mobile "THIS DEVICE")

- **FR-10** The Web canvas's "This session" screen and Mobile's "THIS DEVICE" section MUST
  render: the current hosting mode (already existing data, per `atAccess`'s `hostModes`), a
  permissions list (already existing data, per `atAccess`'s `perms`), and a "Sign out of this
  browser" / "Sign out of this device" action. This spec does not add the OFFLINE COPIES card
  (Non-goals) or the passphrase/pending-approval flow (Non-goals) to this screen.
- **FR-11** "Sign out" MUST end the current session (existing logout mechanism — `POST
  /api/v1/auth/logout`, phase 12) and MUST NOT call `DELETE /api/v1/devices/{id}` — this is a
  self-logout, not a device revocation, and (per Open questions) there is no reliable way for
  the frontend to know its own device ID to revoke even if it wanted to.

## Non-functional requirements

- **Accessibility** — FR-6/FR-7/FR-8 above. Confirmation dialog and live-region conventions
  MUST match the existing `DevicePairingModal` pattern (phase 13), not invent a second one.
- **Performance** — the device list is bounded by how many devices a household plausibly pairs
  (single digits to low tens); no pagination or virtualization needed.
- **Security** — see Security considerations, below.
- **Reliability** — FR-5's re-fetch-on-error behavior; the list never shows a device as revoked
  or active based on stale client state after a failed action.
- **Observability** — not applicable; no new frontend logging beyond what already exists
  (constitution §8 already forbids logging device identifiers or session content from the
  client console in production builds — this spec introduces nothing that would need a new
  logging decision).

## Domain model

No new frontend-side types beyond a thin response-shape type mirroring
`GET /api/v1/devices`'s JSON (already defined server-side, `backend-device-sync.md` FR-1). No
domain logic on the client — revocation and reconciliation are entirely server-side.

## API and contracts

Consumes `backend-device-sync.md`'s existing, approved contracts unchanged:
- `GET /api/v1/devices` (FR-1 of that spec)
- `DELETE /api/v1/devices/{id}` (FR-2 of that spec)
- `POST /api/v1/auth/logout` (phase 12, existing) for FR-11's sign-out

This spec adds no new backend endpoints and no `api/openapi.yaml` changes.

## State transitions

```
Devices tab activated → GET /api/v1/devices → list rendered
Row's Revoke clicked → confirmation dialog opens (focus trapped)
  → Escape / Cancel → dialog closes, focus returns to Revoke control, no request sent
  → Confirm → DELETE /api/v1/devices/{id}
    → 204 → row removed, screen-reader announcement of success
    → 404/409 → inline error shown, screen-reader announcement of failure, list re-fetched
```

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `GET /api/v1/devices` fails (network, 5xx) | Fetch error | An inline error state with a retry action, not a blank list | No silent empty-list fallback — FR-9 treats zero rows as suspicious |
| Revoke confirmed, but device already revoked by another session (race) | `404`/`409` from `DELETE` | Inline error; list re-fetches and shows current state | No retry of the same request; re-fetch is the recovery path (FR-5) |
| Revoke confirmed, network drops before response | Request error/timeout | Inline error, re-fetch triggered | Re-fetch (FR-1) shows whether the revoke actually landed server-side before the response was lost — the UI never assumes success without a `204` |
| User has JS disabled / screen reader with unusual focus handling | N/A (not a server failure) | Confirmation dialog must still be reachable and operable — FR-6/FR-8 | No separate fallback path; the one implementation must satisfy this, not a degraded second path |

## Security considerations

**Trust boundary: this UI vs. the server.** All authorization is server-side
(`backend-device-sync.md`'s FR-2 ownership check, FR-9 revocation enforcement) — this spec adds
no client-side authorization logic and must not be read as a control. Specifically:

- The frontend does not decide whether a device belongs to the current user — it renders
  exactly what `GET /api/v1/devices` returns and calls `DELETE /api/v1/devices/{id}` with the ID
  from that same response. It never accepts or constructs a device ID from any other source
  (e.g. a URL parameter), which would create an IDOR attempt surface even though the server
  would still refuse it (404 per FR-2 of the backend spec) — defense in depth, not the actual
  control.
- No device identifier, IP address, or session descriptor is logged to the browser console or
  any client-side error-reporting mechanism beyond what the existing app-wide error handling
  already does for any API failure (constitution §8).
- The confirmation dialog (FR-4/FR-6) is a UX safeguard against an accidental irreversible
  action, not a security control — the actual authorization is FR-2's server-side ownership
  check, already specified and already `APPROVED` in `backend-device-sync.md`.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Relative-time formatting (FR-2) for last-seen/last-synced, including the "Never synced" case; active/idle dot computation |
| Component | Device list renders only devices with `revokedAt == null` (a fetch response containing an already-revoked device does not render it, FR-1); Revoke opens confirmation; confirm calls `DELETE` with the correct ID; cancel sends no request; `404`/`409` shows inline error and triggers re-fetch; empty response is *not* rendered as a normal empty state (FR-9) |
| Accessibility | Confirmation dialog traps focus, closes on Escape, returns focus on cancel (existing `axe` + manual keyboard-path convention from phase 13's `DevicePairingModal.test.tsx`/`networkA11y.test.tsx`); revocation result announced via live region |
| E2E | Full flow: open Devices tab, see seeded devices, revoke one, see it disappear, confirm via a direct API call that it's actually revoked server-side |

## Acceptance criteria

- [ ] Devices tab lists every device from `GET /api/v1/devices`, re-fetched on each tab activation
- [ ] Each row shows label, kind (class + enrolled-via), last-seen (relative), last-synced (relative or "Never synced") — IP and session-expiry text are not shown (FR-3)
- [ ] A device with a non-null `revokedAt` in the API response is never rendered as a row (FR-1)
- [ ] Revoke requires confirmation naming the specific device; Escape/Cancel sends no request
- [ ] Confirmed revoke calls `DELETE /api/v1/devices/{id}`; success removes the row and announces it to screen readers
- [ ] `404`/`409` on revoke shows an inline error and re-fetches the list, without retrying the same request automatically
- [ ] Full flow is keyboard-operable with no mouse
- [ ] Web/Mobile self-service session view shows hosting mode, permissions, and "Sign out" only — no offline-copies card, no passphrase/pending-approval content
- [ ] Sign-out calls the existing logout endpoint, never `DELETE /api/v1/devices/{id}`

## Open questions

- **Field mapping: canvas IP/session-expiry vs. the approved API response.** The canvas shows
  an IP address and a session descriptor ("Owner"/"Remembered device"/"Expires in *N* days")
  that `GET /api/v1/devices` doesn't return. FR-3 resolves this for now by not rendering them.
  If the maintainer wants exact canvas parity, `backend-device-sync.md` FR-1 needs an amendment
  to add one or both fields — that spec is already `APPROVED`, so this is a real re-review, not
  a rename. An IP-per-device field also deserves a deliberate constitution §8-style check before
  it's added to a persisted, user-visible field, not a silent addition.
- **No reliable "this is the device I'm using right now" signal.** Neither the JWT nor any
  session data available to the frontend carries a device identifier matching `PairedDevice.ID`.
  The canvas's "This device · Host, no revoke button, Owner" row assumes this is knowable; today
  it isn't for two reasons: (1) no `deviceId` claim exists in the access token or is exposed to
  the frontend anywhere in this codebase (confirmed by grep — phase 13 never added one), and
  (2) a host admin who logs in directly (not via QR pairing) has no `PairedDevice` row at all,
  so even with a claim there may be nothing in the list to match against. This spec's FR-1/FR-2
  therefore list every device with no self-exclusion and no "Owner" label — a user could revoke
  their own current session from this screen with no special warning beyond FR-6's generic
  confirmation. Fixing this properly (a `deviceId` claim, and/or a synthetic `PairedDevice` row
  for a directly-logged-in host) is backend work outside this spec's scope; named here so it
  isn't rediscovered as a UI bug when someone asks "why can I revoke myself?"
- **The `atConnect` passphrase/pending-approval flow** — stale or still wanted (Design reference,
  above). Not this spec's call; flagged for the maintainer.
- **Confirmation dialog exact copy/shape** — FR-4/FR-6 specify behavior, not final visual design;
  implementation should reuse `DevicePairingModal`'s existing confirmation pattern rather than
  design a new component.

## References

- `backend-device-sync.md` FR-1/FR-2 — `GET /api/v1/devices`, `DELETE /api/v1/devices/{id}`
- ADR 0029 — cross-device identity agreement (`Accepted`); `lastSyncedAt` rendered here comes
  from the same `PairedDevice` extension ADR 0029 Part 2 defines
- `web/src/screens/Network/DevicePairingModal.tsx` (phase 13) — the confirmation/live-region
  pattern this spec's FR-4/FR-6/FR-7 reuse rather than reinvent
- CLAUDE.md — design-conformance rule
- ADR 0003 — design canvas split
- `.design-reference/ANALYSIS.md` — classification table (re-verified 2026-09-05/06)
- `roadmap/14-devices-and-sync/README.md` — "Design reference correction" section
- Constitution §7 (accessibility is part of done), §8 (no reading/device content in logs)
