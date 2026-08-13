# Design reference analysis

Per [ADR 0003](../.claude/decisions/0003-design-canvas-split.md), the design
reference is authored as multiple per-surface canvases rather than one
canvas for the whole product. Source: Claude Design project
`78075626-e444-438f-8437-205d57129a37` ("Alexandryn interactive prototype").
Last synced 2026-08-13 via the `claude_design` MCP (`DesignSync`).

## Status per canvas

| File | Size | Truncated | Surface |
|---|---|---|---|
| `Alexandryn-Electron.dc.html` | 261,867 B | **Yes**, at the 256 KiB `get_file` cap | Desktop/Host |
| `Alexandryn-Web.dc.html` | 74,623 B | No — complete, closing tags and `data-dc-script` block intact | Web/Remote viewer |
| `Alexandryn-Mobile.dc.html` | 55,344 B | No — complete | Mobile |
| `support.js` | 69,150 B | No — complete | Shared `dc-runtime` renderer, not project-specific |

Splitting by surface fixed the cap problem for Web and Mobile: both now
carry their full state logic and mock data, not just static markup. The
Electron canvas is still too large for one file and remains truncated in
the same place as before the split — it needs a further split of its own
(see below) before it is complete.

## Electron (Desktop/Host) — still truncated

Cuts off mid-element, before the trailing `<script data-dc-script>` block,
so this canvas still has no state/mock data at all — everything captured is
structure with no bindings resolved. Screens whose markup falls after the
cutoff are not captured. Do not infer them.

Per Phase 00's original capture, the host canvas covers (or should cover):
`atFirstRun`, `atLibrary`, `atBook`, `atCollections`, `atCollection`,
`atDiscover`, `atSources`, `atSourceDetail`, `atImport`, `atActivity`,
`atSettings`, `atSystem`. `atStates` and `atSystem`'s design-system sibling
screen were referenced by nav label only, never confirmed present as
markup — treat both as unverified until the next split.

**Recommended next step:** split the Electron canvas the same way Web and
Mobile were split out — by screen group, e.g. `Alexandryn-Electron-Library.
dc.html` (Library/Book/Collections/Discover) and `Alexandryn-Electron-
Admin.dc.html` (Sources/Import/Activity/Settings/System), each small enough
to clear the cap with its `data-dc-script` block intact. States and Design
system should each get their own canvas too, per ADR 0003 — neither exists
as a separate file yet.

## Web (Remote viewer) — complete

Screens present, with full state and mock data: `atAccess`, `atBook`,
`atCollection`, `atCollections`, `atConnect`, `atDiscover`, `atLibrary`,
`atReader`, `atSources`.

Notable: `atAccess` and `atConnect` are new relative to the original
single-canvas capture — this is the pairing/login flow for a LAN client,
which lines up with Phase 12 (authentication) and Phase 13 (network
access) needing exactly that before any device can reach this surface.
`atSources` here appears to be a read view (what's available), not the
Electron canvas's configuration screen — confirm this distinction when
Phase 08/13 specs are written, since the two must not share a component
that silently exposes host-only controls to a LAN client.

No `atSettings`, `atImport`, `atSystem` — consistent with ADR 0003's
boundary: the viewer surface has no source configuration, import, or system
controls.

## Mobile — complete

No named `atXxx` screens; this canvas is a single responsive layout driven
by a tab bar (`library` / `discover` / `collections` / `more`), not a
multi-screen flow like Electron or Web. Full state and mock data present.

## Conclusion

Extract design tokens, typography, and component shapes from all three
files now — Web and Mobile are authoritative end to end, Electron is
authoritative only up to its cutoff. Do not infer anything past the
Electron cutoff, and do not build States or Design system screens from
guesswork; neither has a captured canvas yet.
