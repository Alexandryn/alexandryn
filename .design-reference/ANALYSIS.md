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

## Scope classification (ADR 0003 addendum, 2026-08-17)

Binding / exploratory / unclassified, per canvas or per distinct piece of
captured content within one where a single canvas mixes claimed and
unclaimed material. Unclassified is not exploratory — it is a flagged,
unresolved question the design-conformance gate must surface, not a quiet
default.

| Screen / state | Canvas | Classification | Basis |
|---|---|---|---|
| `atLibrary`, `atBook`, `atCollection(s)`, `atDiscover` | Electron, Web | Binding | Phases 06/07, approved specs |
| `atSources`, `atSourceDetail` | Electron, Web | Binding | Phase 08, approved spec — content partially contradicted (5 assumed source kinds vs. the spec's 2), tracked separately, not a classification problem |
| `atImport` | Electron | Binding | Phase 10 names it — premise contradicted by `frontend-import-confirmation.md`'s Source-gated design, tracked separately |
| `atReader` | Web | Binding | Phase 11, approved spec; confirmed intentionally single-captured (`architecture-system.md` FR-6, `frontend-reader.md` NFRs) |
| `atConnect`, `atAccess` | Web | Binding | Phases 12/13 name pairing/access; `atAccess`'s session/sign-out content ("This session" — hosting mode, permissions, "Sign out of this browser") also serves phase 14's self-service device-session view — content partially contradicted, tracked separately, not a classification problem: `atConnect`'s "Library passphrase" + "Remember this browser" + "approve from Settings → Devices → Pending" flow does not match phase 13's actual shipped mechanism (Gate 1 decision C-1(a): account login + QR pairing, no passphrase, no pending-approval state). Maintainer has not confirmed whether this is stale content or a still-wanted second join path (`roadmap/14-devices-and-sync/README.md`, "Design reference correction"). `atAccess`'s own OFFLINE COPIES card is separate — see the offline-file-cache row below |
| `atActivity` | Electron | Binding | Phase 15 names it, correctly deferred (outline only) |
| Acquire queue (`acqActive`/`acqQueued`/`acqDone`/`acqFailed`, per-edition `Acquire`/`Open`/`Find` actions) | Electron | **Unclassified** | No phase claims an async download/acquisition mechanism distinct from Import; not resolved, not exempted — see the ingest-model discussion |
| Cloud relay (`hostModes`'s `'cloud'` option specifically — `stellar.alexandryn.cloud`, an Alexandryn-operated tunnel) | Web | Exploratory | Resolved 2026-08-18: Alexandryn operates no relay/tunnel infrastructure on a user's behalf, declined as a matter of decision, not left open. This specific toggle does not become real; the canvas stays as visual reference only |
| User-operated remote reachability (own domain, own host, own certificate) | Not captured in any canvas | N/A — no UI exists yet | Supported per ADR 0017/`roadmap/13-network-access/README.md`, but distinct from the declined item above: no canvas models "enter your own domain" anywhere, including the Web canvas's own `hostModes` (its only non-local option is the declined cloud-relay toggle). The host-side configuration surface for this is `atSettings` → Network → Advanced, tracked in the row below |
| Offline file cache (`off`, `offlineCount`/`offlineSize`, `Save offline`, Mobile's `downloads` list, and `atAccess`'s own "OFFLINE COPIES" card) | Web, Mobile | **Unclassified** | No phase claims client-side file caching (phase 14 covers reading-progress sync; bookmark/highlight sync is read-only there and write-path is explicitly deferred — see `backend-device-sync.md`); coupled to the cloud-relay question, tracked with it. Whoever builds `atAccess`'s session-management content for phase 14 must not carry this card along with it |
| `atMobile` — library/discover/collections/reader-equivalent tab content | Mobile | Binding | ADR 0003's original grouping (`atRemote`/`atMobile`/`atTablet` as one Web/Remote surface); matches phases 04/12/13's already-claimed scope |
| `atMobile` — "companion app" framing, native packaging | Mobile | **Unclassified** | No phase names a native mobile client; Mobile's own captured state doesn't even model host-selection (only `hostShort`, a fixed LAN IP) — the framing is prose, not built into this canvas's own state the way the Web canvas's cloud-relay toggle is |
| `atTablet` | Electron-Admin | Binding, location TBD | Same responsive-viewer grouping as Mobile; ADR 0003 assigned it to Web/Remote, it's physically captured in the Host canvas instead — file-placement question stays open, unchanged by this classification pass |
| `atStates` | Electron-Admin | Binding | Actively relied on (`architecture-desktop-host.md`, phase 05's Architecture-decisions-expected) for loading/error copy conventions; "final home" file-location question stays open, unchanged |
| `atSettings` → Network tab → Advanced panel (bind address, TLS certificate, mDNS name) | Electron-Admin | Binding, contents uncaptured | `roadmap/13-network-access/README.md` (2026-08-18) names this panel directly as phase 13's UI home for bind/certificate/domain configuration. The panel itself is drawn (`sgNetwork`'s "Advanced" disclosure row); what's behind "Open advanced" is not — no domain field, no cert-upload/ACME UI exists in any canvas. The surface is committed, the detail is not; phase 13 fills it in, doesn't invent it from nothing |
| `atSettings` → Devices tab (`sgDevices`, `settingsTab==='devices'`) | Electron-Admin | Binding | Claimed by phase 14 (`roadmap/14-devices-and-sync/README.md`, "Design reference correction", 2026-09-05) — was filed under the row below as part of "remaining tabs," missed by an earlier search for a top-level `atDevices` route rather than a Settings sub-tab. Shows: per-row status dot, device name + kind, IP, last-seen text, session descriptor ("Owner"/"Remembered device"/"Expires in *N* days"), inline "Revoke" action. No confirmation dialog is captured — the spec must decide that |
| `atSettings` (remaining tabs: metadata/security/storage/advanced-beyond-network), `atSystem`, `atFirstRun` | Electron-Admin | **Unclassified** | No phase, any status, claims these beyond the Network panel (row above) and the Devices tab (row above); not resolved here — see "where they belong" |
| Design system | nav label only, no canvas | N/A | Genuinely uncaptured, not a classification question |

## Conclusion

All five files are now complete (no truncation) and extractable. The one
real gap left is the Design system screen — still nav-label-only, no
canvas, no content. Do not infer it. The `atTablet` placement question above
should be resolved with whoever owns the Claude Design project before
`architecture-frontend.md` treats the host/LAN-client screen boundary as
settled.
