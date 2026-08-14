# Design reference analysis

Per [ADR 0003](../.claude/decisions/0003-design-canvas-split.md), the design
reference is authored as multiple per-surface canvases rather than one
canvas for the whole product. Source: Claude Design project
`78075626-e444-438f-8437-205d57129a37` ("Alexandryn interactive prototype").
Last synced 2026-08-13 via the `claude_design` MCP (`DesignSync`) — this is
the second sync; the first (same day, earlier) is what the previous version
of this file described.

## Status per canvas

| File | Size | Truncated | Surface |
|---|---|---|---|
| `Alexandryn-Electron.dc.html` | 144,397 B | No — complete | Desktop/Host: Library, Discover, Collections, Book, Sources, Import, Activity |
| `Alexandryn-Electron-Admin.dc.html` | 124,800 B | No — complete | Desktop/Host: Settings, System, First run, **States**, **Tablet** |
| `Alexandryn-Web.dc.html` | 74,775 B | No — complete | Web/Remote viewer |
| `Alexandryn-Mobile.dc.html` | 55,424 B | No — complete | Mobile |
| `support.js` | 69,150 B | No — complete | Shared `dc-runtime` renderer, not project-specific |

**What changed since the first sync:** the Electron canvas was too large for
one file (truncated at the 256 KiB cap, no `data-dc-script` block, so no
state/mock data at all). It has since been split by whoever authors the
Claude Design project — `Alexandryn-Electron.dc.html` shrank to the
library-facing screens, and a new `Alexandryn-Electron-Admin.dc.html`
carries settings/system/admin. Both are now under the cap and both have
their `data-dc-script` block intact — the Electron canvas is complete for
the first time since capture began.

**Two screens previously unverified are now confirmed with real markup**:
`atStates` and `atTablet` were nav-label-only in the first sync (referenced
by name, no content, explicitly flagged "treat as unverified" in the
previous version of this file). Both now have full `<sc-if>` blocks with
real content in `Alexandryn-Electron-Admin.dc.html`:

- `atStates` — "Empty, loading and error states. Plain language first,
  technical detail stays behind Advanced [...]" — this is the shared
  empty/loading/error/skeleton reference ADR 0003 called out as
  cross-cutting both surfaces (its own canvas, category 3 in ADR 0003's
  split). It didn't get a separate file; it lives inside the Electron/Admin
  canvas as a reference screen instead. Worth a maintainer decision on
  whether that's the final home or it should be pulled into its own file
  later, matching ADR 0003's original four-canvas plan more literally.
- `atTablet` — a full screen ("Tablet" heading, real layout), living inside
  the *Electron/Admin* canvas, not the Web canvas. This is worth flagging
  explicitly: ADR 0003 grouped Tablet under "Web/Remote viewer"
  (`atRemote`, `atMobile`, `atTablet`) as the LAN-served, access-only
  surface. Finding it captured inside the Host canvas instead could mean
  either (a) it's a host-side reference/preview of how the Web surface
  renders at tablet width, authored here for convenience, or (b) the
  surface boundary drifted from ADR 0003's plan during design work. Confirm
  which before `architecture-frontend.md` or `architecture-desktop-host.md`
  treats this as settled — do not assume either reading.

**Design system screen — still not captured.** A "Design system" link
appears in the nav in both Electron canvases, but there is no `atDesignSystem`
route and no content block behind it — same status as the first sync. No
separate canvas file for it exists in the project yet either
(`list_files` on the project shows only the five files in the table above).

## Electron (Desktop/Host + Admin) — complete

Screens confirmed present, with full state and mock data, across the two
files: `atLibrary`, `atBook`, `atCollection`, `atCollections`, `atDiscover`,
`atSources`, `atSourceDetail`, `atImport`, `atActivity` (in
`Alexandryn-Electron.dc.html`); `atSettings`, `atSystem`, `atFirstRun`,
`atStates`, `atTablet` (in `Alexandryn-Electron-Admin.dc.html`).

## Web (Remote viewer) — complete

Unchanged from the first sync — byte-identical on re-pull. Screens present:
`atAccess`, `atBook`, `atCollection`, `atCollections`, `atConnect`,
`atDiscover`, `atLibrary`, `atReader`, `atSources`.

`atAccess` and `atConnect` are the pairing/login flow for a LAN client —
lines up with Phase 12 (authentication) and Phase 13 (network access).
`atSources` here appears to be a read view, not the Electron canvas's
configuration screen — confirm this distinction when Phase 08/13 specs are
written, since the two must not share a component that silently exposes
host-only controls to a LAN client.

No `atSettings`, `atImport`, `atSystem` — consistent with ADR 0003's
boundary: the viewer surface has no source configuration, import, or system
controls. (`atTablet` is also absent here — see the note above about where
it actually lives.)

## Mobile — complete

Unchanged from the first sync — byte-identical on re-pull. Single
responsive layout driven by a tab bar (`library` / `discover` /
`collections` / `more`), not a multi-screen flow. Full state and mock data
present.

## Conclusion

All five files are now complete (no truncation) and extractable. The one
real gap left is the Design system screen — still nav-label-only, no
canvas, no content. Do not infer it. The `atTablet` placement question above
should be resolved with whoever owns the Claude Design project before
`architecture-frontend.md` treats the host/LAN-client screen boundary as
settled.
