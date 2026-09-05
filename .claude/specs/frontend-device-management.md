# Spec: Frontend device management

| | |
|---|---|
| **Status** | `DRAFT` — **blocked on missing design canvas; cannot advance to APPROVED** |
| **Phase** | `14-devices-and-sync` |
| **Author** | Claude (Sonnet 4.6 Thinking), for review by Luann Moreira |
| **Created** | 2026-09-05 |
| **Last updated** | 2026-09-05 |
| **Supersedes** | — |
| **Reviewed in** | — |
| **Design reference** | ANALYSIS.md sync date consulted: 2026-08-13. Canvas files consulted: `Alexandryn-Electron-Admin.dc.html`, `Alexandryn-Electron.dc.html`, `Alexandryn-Web.dc.html`, `Alexandryn-Mobile.dc.html`. **No device-management screen exists in any canvas.** `atSettings` is classified Unclassified (no phase claims it beyond the Network panel). Per CLAUDE.md design-conformance rule and constitution's gate: this spec cannot be drafted, self-reviewed, or marked APPROVED until a canvas is captured. This file records the stop-and-ask and the intended scope so the capture request is precise. |

## Design-conformance stop — action required

CLAUDE.md states: "Before drafting a UI-facing spec, re-read `.design-reference/`'s canvas(es)
for that spec's surface... and check whether a captured screen exists for it."

Result of that check (2026-08-13 sync, all four canvases):

- No `atDevices`, `atDeviceManagement`, or equivalent route in any canvas.
- `atSettings` exists in `Alexandryn-Electron-Admin.dc.html` but is classified **Unclassified**
  — no phase claims its content beyond the Network panel (which belongs to phase 13). No
  device-management sub-panel is visible behind `atSettings`.
- The Web canvas (`Alexandryn-Web.dc.html`) has no settings or management surfaces at all,
  consistent with ADR 0003's boundary (viewer surface has no management controls).

**Required action before this spec can be drafted:**

Capture the device management screen via the DesignSync tool:
- `get_project` with project ID `78075626-e444-438f-8437-205d57129a37`
- `list_files` to verify current file set
- `get_file` for the relevant canvas(es)

Questions the capture should answer:
1. Is device management a tab/sub-panel within `atSettings`? Or a separate `atDevices` route?
2. Does the host (Electron) surface show the device list, or is it the Web/remote surface that
   shows it (since LAN readers are the ones with devices), or both?
3. What information is shown per device (label, class, last-seen, last-synced, enrolled-via)?
4. Is revocation a button per device, a swipe action, or something else?
5. Is there a confirmation dialog for revocation?

If no canvas exists after a fresh capture, the maintainer must create one in the design project
before this spec can be drafted, per the same precedent as the "First-run Setup" canvas
handling in earlier phases.

## Intended scope (once canvas is captured)

This placeholder records the intended scope so the capture request is precise:

**In**
- A device list screen or panel accessible to the authenticated user, showing their paired devices.
- Per-device: label, class icon, enrolled-via, created-at, last-seen-at, last-synced-at,
  revoked status.
- Revocation action: a per-device revoke button/control, with a confirmation step for the
  destructive action, calling `DELETE /api/v1/devices/{id}` from `backend-device-sync.md` FR-2.
- Keyboard navigability: the list must be traversable by keyboard; revoke must be reachable
  without a mouse; the confirmation dialog must trap focus and be dismissable via Escape.
- Screen-reader announcement: revocation confirmation and result must be announced.
- Empty state: user has only one device (the current one) — show it, not a blank list.

**Out**
- Device renaming/relabeling — not in scope; the label is set at pairing time.
- Per-device reading history or activity — phase 15.
- Mobile native app device management — no native mobile client in this roadmap.

## Open questions

- All of the "questions the capture should answer" above.
- Which surface (host vs. Web) shows the device list — this is an ADR 0003 boundary question.
  A host-only surface makes sense for admin control; a viewer-side surface makes sense for a
  user managing devices they read on. Both may be needed with different scopes.

## References

- `backend-device-sync.md` FR-1/FR-2 — the API endpoints this UI calls
- CLAUDE.md — design-conformance rule (stop-and-ask when no canvas exists)
- ADR 0003 — design canvas split, determines which canvas this spec belongs to
- `.design-reference/ANALYSIS.md` — classification table consulted (2026-08-13)
