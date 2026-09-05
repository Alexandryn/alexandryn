# Phase 14 — Devices and sync

| | |
|---|---|
| **Status** | In progress — phase document opened 2026-09-05; planning in progress on `feat/phase14-devices-and-sync` |
| **Depends on** | Phase 11 (implemented, Gate 2 cleared), Phase 13 (closed, merged PR #80) |
| **Blocks** | 16 |
| **Opened** | 2026-09-05 |
| **Closed** | — |

## Dependency status at open (verified, not trusted from roadmap)

**Phase 11 (Reader):** The roadmap status line read "Specs approved, implementation not started"
— stale. `.claude/specs/README.md` shows all four phase-11 specs (`backend-reader-content.md`,
`backend-reading-api.md`, `frontend-reader.md`, `reading-data-export.md`) as `IMPLEMENTED`;
security audit `0011` cleared Gate 2 on 2026-09-02. Phase 11 is awaiting final maintainer PR
approval for `VERIFIED`. The stale status line was corrected in the phase 11 README as part of
opening this phase.

**Phase 12 (Authentication):** The phase-12 README documents a "Correction (2026-09-02)" listing
three closure conditions. Conditions 1 (C1/C2 fixes) and 2 (X-Library-Id claim validation) were
addressed on `feat/phase13-network-access` and re-verified by audit `0013`. Condition 3 — "a
fuller independent re-audit of the phase-12 authorization surface" (RBAC, membership, refresh
rotation) beyond the C1–C2 spot-fixes — has **not** been satisfied anywhere in the record.
Audit `0013` was explicitly scoped to phase-13 files and does not constitute the independent
phase-12 surface re-audit. Phase 12 is formally open. Phase 14 does not depend on phase 12
directly (it depends on phase 11 and phase 13); this flag is recorded here for the maintainer,
not as a blocker. Resolving condition 3 is out of phase 14's scope.

## Objective

Reading progress is synchronized across a household's authorized devices. When two devices
update it while offline, the server reconciles the conflict explicitly and deterministically —
no silent last-write-wins, no data loss. A user can see which devices are paired to their account
and revoke any of them.

Bookmarks and highlights are pulled read-only in the same sync payload (so a new or reconnecting
device sees them), but this phase does not build a write path or conflict resolution for them —
`backend-device-sync.md`'s own Non-goals name this explicitly: "the merge semantics for bookmarks
are more complex than progress reconciliation and warrant their own spec." A device that creates
a bookmark or highlight offline has no way to sync it yet; that gap is named, not fixed, here.
The original phase outline's "synchronised... with explicit conflict resolution" language covered
all three; this correction narrows it to what is actually specified so the exit criteria below
don't imply more than what's being built.

## Why here

Phase 11 built what is synced (reading progress, bookmarks, highlights, `ReconcileProgress`).
Phase 13 built what devices are (the `PairedDevice` aggregate, the pairing flow, the trust
boundary between a host and LAN readers). Phase 14 is where those two surfaces are joined: the
first time a LAN reader's progress state and the server's canonical record are reconciled across
a network.

Running earlier would have had no `ReconcileProgress` to call and no `PairedDevice` concept to
attach sync cursors to. Running later risks phase 16 (security hardening) having to harden a sync
surface it has never seen designed.

## Scope

**In**

- A "list my devices" endpoint and revocation endpoint, surfacing phase 13's existing
  `FindByOwner` and `Revoke` repository methods through a handler for the first time.
  No new repository logic for this — the repository exists and is tested; the gap is transport.
- A sync transport: a pull-based, LAN-local mechanism by which a remote device fetches the
  current canonical state of progress, bookmarks, and highlights for works in its library.
  Progress is a full read/write path — a device also posts its own progress updates, and the
  server runs `ReconcileProgress` on every incoming report (already implemented in the domain
  and wired in phase 11 for single-device writes; this phase extends that to multi-device
  contention). Bookmarks and highlights are read-only in this pull — see Out, below.
- Conflict resolution for concurrent offline updates — the `(epoch, percentage)` reconciliation
  math is already correct; this phase provides the transport through which it is invoked by
  multiple devices.
- Per-device sync cursors: a mechanism by which a device fetches only changes it has not yet
  seen, rather than the full dataset on every poll. Whether this cursor extends the existing
  `PairedDevice` aggregate or lives in a new model is decided in ADR 0029.
- Cross-device identity agreement: the unresolved question from `domain-reading.md` Open
  Questions (review `0049`, finding 8). Before the sync protocol runs, both sides must agree
  which Work/Edition a ProgressReport is about. ADR 0029 decides this first.
- A device management UI: a screen or panel from which the authenticated user can see their
  paired devices and revoke one. **A canvas for this exists and always has** — see "Design
  reference correction" below; the earlier claim in this document that no canvas existed was
  wrong.

**Out**

- Bookmark/highlight write-path sync and conflict resolution. A device creating a bookmark or
  highlight while offline has no way to post it through the sync transport in this phase — the
  merge semantics are more complex than progress reconciliation (deletion vs. edit, no natural
  total order the way epoch/percentage gives progress) and warrant their own spec and ADR.
  Named in `backend-device-sync.md`'s Open questions so it is not later rediscovered as a "sync
  bug." A future spec, still within or after this phase depending on scheduling, closes it.
- Any server-mediated relay beyond the LAN. Alexandryn operates no relay, tunnel, or
  traffic-mediating infrastructure on any user's behalf, under any bind mode (constitution §6,
  ADR 0017, ADR 0028).
- Offline file cache (`off`/`offlineCount`/`offlineSize`/`Save offline` elements in Web and
  Mobile canvases). Per `.design-reference/ANALYSIS.md`, this element is **Unclassified** and
  coupled to the already-declined cloud-relay question. Out of this phase's sync scope.
- Library-visible finished/leaderboard view — `backend-reading-api.md`'s status line defers
  this explicitly to phase 15.
- PDF or CBZ reading — phase 11 is EPUB-only; this phase does not extend the format surface.
- Phase 12's outstanding "fuller independent re-audit" — out of this phase's scope, flagged above.

## Design reference correction (2026-09-05)

This document originally claimed no device-management canvas existed, and blocked
`frontend-device-management.md` on a DesignSync capture. **That claim was wrong.** A re-pull of
the design project (same content as already committed — `Alexandryn-Electron-Admin.dc.html` is
byte-identical to its 2026-08-13 version, nothing changed on the design side) found the screen
had been there the whole time: a "Devices" tab within Settings (`sgDevices`,
`settingsTab==='devices'`), alongside metadata/network/security/storage/advanced. It was missed
because it's a Settings sub-tab, not a separate top-level route — a search for a literal
`atDevices` route found nothing, but the sub-tab was drawn in full. `ANALYSIS.md` had already
filed it correctly under a wide "`atSettings` (remaining tabs) — Unclassified" bucket; that entry
should now be split out and explicitly claimed for this phase rather than left unclassified.

What the canvas actually shows, answering `frontend-device-management.md`'s original capture
questions directly:

- Sub-tab within Settings, not a separate route (above).
- **Both surfaces exist, scoped differently.** Electron Admin's Devices tab is the admin-facing
  list of every device, each with a "Revoke" action. The Web canvas (`atAccess`, "This session")
  and Mobile ("THIS DEVICE") each show only the *current* browser/device's own session — a
  "Sign out" action, not a list of other devices or a revoke control over them.
- Per device, the host list shows: a status dot, name (e.g. "Pixel 8"), kind (e.g.
  "Phone · Chrome"), IP address, last-seen text, and a session descriptor ("Owner" /
  "Remembered device" / "Expires in *N* days").
- Revocation is an inline text action ("Revoke", in the error color), not a button or swipe.
- **No confirmation dialog is captured for revocation.** Still a genuine open question the spec
  must decide.

Two things the re-pull surfaced that need a decision before the Web-side session screen is built,
not before the host-side Devices tab:

- The Web canvas's join screen is a "Library passphrase" + "Remember this browser for 30 days"
  flow, with a fallback pointing to "Settings → Devices → Pending" for host approval. That does
  not match phase 13's actual shipped join mechanism (Gate 1 decision C-1(a): account login +
  QR pairing, no passphrase, no pending-approval state). **Maintainer has not decided whether
  this is stale content predating that decision or a second join path still wanted** — the
  implementation session must stop and ask before assuming either way when it reaches the
  Web-side session/join screens. Do not build the passphrase flow, and do not silently drop it
  from the canvas either — ask.
- The same Web "This session" screen also carries an "OFFLINE COPIES" card. That is the
  offline-file-cache element this phase's own Out section excludes (coupled to the unresolved
  cloud-relay question). Building the session/sign-out part of that screen must not carry the
  offline-copies card along with it.

## What phase 13 built that this phase extends

- `domain.PairedDevice` (`internal/domain/device_pairing.go`): `Owner`, `DeviceClass`,
  `EnrolledVia`, `RevokedAt`, `LastSeenAt`, `Touch()`. `Touch()` has no phase-13 caller;
  comment at line 479 explicitly defers it to phase 14.
- `PairedDeviceRepository`: `FindByOwner`, `Revoke`, `RevokeByPairingSessionID`,
  `AssignOwnerByPairingSession`, `FindByID`, `FindByPairingSessionID`, `Save`, `InsertProvisional`.
- `FindByOwner` is called nowhere in any handler or frontend code (confirmed by grep). The
  endpoint that would call it does not exist yet.
- Audit finding A-13-06 (Informational, Accepted, deferred to phase 14): device revocation does
  not invalidate active refresh tokens. This phase must explicitly resolve or carry this forward.

## Architecture decisions expected

**ADR 0029 — Cross-device identity agreement for progress reconciliation** (load-bearing; must
precede all sync-protocol spec work):

`ReconcileProgress` (FR-6, `domain-reading.md`) assumes both sides agree which Work/Edition a
ProgressReport is about. Nothing built so far addresses how that agreement is reached, or what
happens when it is asymmetric — one device's sync agreeing, another silently not, for what should
be the same file. ADR 0029 states options, tradeoffs, and a recommendation.

Secondary decisions (may be ADR addenda or spec FRs depending on weight):

- Whether per-device sync state (cursor, last-synced-at) extends `PairedDevice` or lives in a
  new model — decided in ADR 0029 alongside the identity question, since both concern what a
  device "knows" about a Work.
- Sync transport mechanism: pull-based polling cadence, push notification option (server-sent
  events or WebSocket), or hybrid. Must stay LAN-local.
- Whether to address A-13-06 (refresh token revocation on device revoke) in this phase.

## Specifications

| Spec | Status |
|---|---|
| `backend-device-sync.md` | Not yet scoped — blocked on ADR 0029 |
| `frontend-device-management.md` | Canvas exists (see "Design reference correction" above); spec content being corrected to match, not yet APPROVED |

Working decomposition (planning session's decomposition is authoritative if it diverges):
1. ADR 0029 — drafted and approved first; everything else depends on it.
2. `backend-device-sync.md` — sync transport, conflict resolution, device list/revocation
   endpoints, per-device cursors, PairedDevice extension decision. Depends on ADR 0029.
3. `frontend-device-management.md` — device list, revocation, sync status indicators, against
   the canvas identified in "Design reference correction" above. Depends on ADR 0029. Does not
   build the Web-side passphrase/pending-approval join screen without the maintainer's
   confirmation (see the same section).

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Two devices disagree about which Work/Edition a ProgressReport refers to — sync appears to work but reports are against different canonical records | Medium | High — silent data divergence with no visible error | ADR 0029 must resolve this before any sync spec is written. Leaving it as a footnote inside the sync spec reproduces the exact failure `0049` finding 8 documents. |
| Refresh token revocation not applied on device revoke (A-13-06, Accepted, deferred from phase 13) | Low | Medium | ADR 0028 §6 deferred this to phase 14. This phase must state the decision explicitly — carry forward or resolve. Cannot be silently inherited from the closed audit finding. |
| Web-side passphrase/pending-approval join screen gets built or silently dropped without the maintainer's input | Medium | Either builds a join mechanism phase 13 never shipped, or loses design intent that was actually still wanted | Named as a stop-and-ask in "Design reference correction" above; the implementation session must ask before assuming either way when it reaches the Web-side session/join screens. |
| Sync cursor design introduces IDOR (device reading another user's sync state) | Low | High | Per-user + per-library scoping discipline applied from the start. `scripts/check-user-scoped-reading.sh` extended to cover sync endpoints. |
| `ReconcileProgress` epoch assumption broken by a clock-skewed or replay-attacking LAN client | Low | Medium | Epoch is server-assigned, never client-supplied (domain-reading.md FR-2). Sync spec must state this invariant and the transport must validate it at the boundary. |

## Test strategy

| Layer | Weight |
|---|---|
| Unit | ADR 0029's identity-agreement rule table-driven; per-device cursor advancement; `Touch()` with sync path |
| Integration | Sync endpoint with two concurrent progress reports from different devices; IDOR test (device A cannot read device B's sync state); revocation test (device listed before revoke, gone after) |
| Property-based | Extend the existing `ReconcileProgress` commutativity test to the multi-device transport path: N devices posting progress in any order, canonical result invariant to arrival order |
| E2E | Two browser sessions syncing progress for the same Work; one session goes offline, both advance, reconnect, verify the higher-epoch report wins |
| Accessibility | Device management UI: keyboard-navigable list, revoke reachable by keyboard, destructive action labelled correctly, screen-reader announcement of revocation result |

## Security considerations

Three trust boundaries the audit must examine:

1. **Sync endpoint scoping.** Every sync endpoint must enforce per-user + per-library scoping at
   the handler call, SQL-traced (same discipline AUDIT-0012-C1 found missing). `scripts/check-user-scoped-reading.sh` must be extended to sync endpoints.
2. **Device identity.** The DeviceID on a sync request must be validated against the authenticated
   user's device list. A device presenting another user's DeviceID must be rejected. ADR 0029's
   identity design must not introduce a spoofable identifier.
3. **Revocation propagation.** When a device is revoked, the sync endpoint must refuse subsequent
   requests from that device's tokens. A-13-06's deferred question is directly relevant.

Constitution §8: sync log lines must never include Work titles, highlight text, bookmark labels,
or precise positions — only IDs and outcome categories.

## Observability

- Sync events (device synced, conflict resolved, stale report rejected) logged at `info` with
  device ID and outcome tag. No Work titles, content, or positions.
- Device last-synced-at surfaced in device list endpoint and device management UI.
- `ReconcileProgress` outcomes logged at `debug` with epoch delta but not percentage values.
- Revocation events logged at `info` (device ID, revoked-at, revoking user). No session content.

## Exit criteria

- [ ] ADR 0029 approved; sync-protocol spec built on top of it
- [ ] "List my devices" endpoint exists, user-scoped, `FindByOwner` wired to a handler
- [ ] Device revocation endpoint exists, user-scoped only — cross-user revocation refused and tested
- [ ] Progress syncs correctly across two or more devices on LAN
- [ ] Conflict resolution tested against concurrent offline edits
- [ ] Per-device sync cursor advances correctly; incremental fetch tested
- [ ] IDOR test: a device cannot read or write another user's sync state
- [ ] Device management UI supports revocation, built against the existing Settings → Devices
      canvas; revocation confirmation UX decided in the spec (not captured in the canvas)
- [ ] A-13-06 explicitly resolved or deferred with a recorded reason — not silently inherited
- [ ] `scripts/check-user-scoped-reading.sh` extended to sync endpoints and passes
- [ ] All specs in this phase are `VERIFIED`
- [ ] Test plans exist for every spec in this phase, written before this phase's own RED step (ADR 0016)
- [ ] Security audit recorded in `.claude/audits/` with no open Critical or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
