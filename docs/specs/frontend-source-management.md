# Spec: Frontend source management

| | |
|---|---|
| **Status** | `IMPLEMENTED` (all Tiers 0–6 implemented and verified, security audit Clear) |
| **Phase** | `08-sources` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | `0035` (two independent agents, cross-spec) — Needs rework at review time (2 Blocking, 8 Major, 5 Minor), all findings fixed; approved by maintainer 2026-08-15 |

## Context

`frontend-shell-and-routing.md` (phase 04) built the shell and the
router; `backend-source-adapter.md` (phase 08) now provides a real
`/api/v1/sources*` API — create/edit/remove a source, see its health
and detected capabilities, browse or search what it offers. This is the
project's first screen with a credential-entry form, and the first
screen where "add a folder path" needs to work differently depending on
whether the renderer is running inside Electron (a native folder
picker is available) or in a plain LAN browser tab (it is not) — a
platform-detection need distinct from `frontend-shell-and-routing.md`
FR-4's `useCapability()` hook, which gates *authorization*
(host-only routes), not *platform* (Electron vs. browser).

## Problem

Nothing renders `/api/v1/sources*`. There is no way to add, inspect, or
remove a source from the UI, and no design-reference capture exists for
this screen (a gap, same as phase 07's Discover screen — see Open
questions).

## Goals

- A source management screen: list configured sources with health
  status and capabilities, add a new source, edit or remove an existing
  one
- A local-folder path field that uses Electron's native folder picker
  when available, and a plain text field otherwise — both paths landing
  in the same form state
- A credential-entry form for OPDS sources that never round-trips a
  previously-entered password back to the user
- Browse/search UI for a source's contents, reusing this project's
  established grid/list rendering patterns where they fit
- Clear, distinct feedback for each of `backend-source-adapter.md`
  FR-6's health-check `detail` categories — not a single generic
  "error" state

## Non-goals

- Anything past viewing a source's browsed/searched contents — no
  "add this to my library" action exists yet; that's phase 10's import
  flow, the same deliberate dead-end `frontend-discover-screen.md`
  already accepted for Discover results, for the same reason
- OAuth/Authentication-Document credential flows — `backend-source-adapter.md`'s
  Basic-Auth-only scope, reflected here as a plain username/password
  form, nothing more
- Recursive folder browsing UI — `backend-source-adapter.md` FR-7's own
  scope cut, nothing for this spec to build UI for yet

## User stories

- As **someone setting up their library from inside the desktop app**,
  I want to click "Browse..." and pick a folder with a native dialog,
  so I don't have to type or paste a filesystem path by hand.
- As **someone configuring a source from a laptop on the LAN, not the
  desktop app**, I want a plain text field that works the same way any
  other web form does, so I'm not blocked by a feature that only exists
  inside Electron.
- As **someone whose OPDS source stopped working**, I want to see
  *why* — wrong password, unreachable, malformed response — not a
  single generic "error," so I know what to actually go fix.

## Functional requirements

- **FR-1** The source management screen (`/sources`) renders a
  `<Sidebar>`-framed `<ContentPane>` (`frontend-shell-and-routing.md`
  FR-3) listing every configured source as a card: `label`, `kind`
  (with a small icon per `domain-source.md` FR-1's own "category tag,
  for display purposes" framing), health status (a colored indicator
  plus the FR-3-below text), and detected capabilities (small badges —
  "Search" shown only when `CanSearch: true`). Each card links to that
  source's browse view (FR-6).
- **FR-2** "Add source" opens a form: `label` (text input), `kind`
  (a `<SegmentedControl>`, `frontend-component-primitives.md` FR-1's
  primitive, matching `backend-source-adapter.md` FR-1's closed
  `local-folder`/`opds` vocabulary exactly), then kind-specific fields
  — a path field (FR-4) for `local-folder`, a URL field plus an
  optional "Requires a username and password" toggle revealing a
  credential sub-form (FR-5) for `opds`. Submitting calls `POST
  /api/v1/sources`; on success, the new source's card appears with
  whatever health status the create-time check (`backend-source-adapter.md`
  FR-1) returned — not assumed healthy just because creation succeeded.
- **FR-3** Health status renders one of eleven distinct, plainly-worded
  states: two non-`detail` states ("Checking..." while in flight,
  "Reachable" on success) plus one dedicated state per
  `backend-source-adapter.md` FR-6's full closed `detail` vocabulary —
  all **nine** values, not a partial subset (constitution §11: specific,
  no apology, no exclamation marks) — when unreachable: "Can't connect
  — timed out" (`timeout`), "Can't connect — connection refused"
  (`connection-refused`), "Wrong username or password" (`auth-rejected`,
  covering both a bad credential and the decrypt-failure case FR-13
  deliberately makes indistinguishable), "This source returned an
  error" (`http-4xx`, excluding the auth case above), "This source is
  having a problem right now" (`http-5xx`), "This source tried to
  redirect, which isn't supported" (`http-3xx-unsupported`), "Received
  an unexpected response" (`unparseable-response`), "Folder not found"
  (`path-not-found`), "Folder isn't readable" (`path-not-readable`).
  Each unreachable state offers a "Check again" action calling
  `POST /api/v1/sources/:id/health-check` directly, without requiring
  the user to re-open the edit form.
- **FR-4** The local-folder path field is a text input with an adjacent
  "Browse..." button, **shown only when `window.alexandryn` is defined**
  — a plain `typeof window !== 'undefined' && 'alexandryn' in window`
  presence check, deliberately not `frontend-shell-and-routing.md`
  FR-4's `useCapability()` hook, which answers a different question
  (is this connection authorized for host-only content) than the one
  this field asks (is a native OS dialog available at all). Clicking
  "Browse..." calls `window.alexandryn.source.pickLocalFolder()`
  (`desktop-host-ipc-surface.md` FR-6, amended for this spec), which
  resolves to `{ path: string } | null`, and fills the text field with
  `result.path` when non-`null`, or leaves the field unchanged on
  cancel. When `window.alexandryn` is undefined (a LAN
  browser tab), the "Browse..." button simply isn't rendered — the text
  field alone remains, fully functional, no degraded messaging needed
  since typing a path is the field's normal, always-available mode, not
  a fallback for a broken feature.
- **FR-5** The credential sub-form (`username`, `password`, both text
  inputs, `password` using `type="password"`) is submitted once, on
  source creation or an explicit credential *replacement* — never
  pre-filled or partially editable afterward. Editing an existing
  authenticated source (FR-2's edit path) shows "Password set" as
  static text with a "Replace" action that reveals a fresh, empty
  credential sub-form — matching `backend-source-adapter.md` FR-2's own
  "never echoing the username or password back" rule; there is no code
  path in this screen that could display a previously-entered
  credential even if a future bug tried to, since the API response this
  screen consumes (`hasCredential: boolean`) structurally cannot carry
  one. When the credential sub-form is revealed for a source whose
  `baseUrl` (FR-2) doesn't start with `https://`, a visible,
  non-blocking inline warning is shown — "This source doesn't use
  HTTPS. Your password will be sent unencrypted." — surfacing
  `backend-source-adapter.md` FR-4's accepted-risk decision concretely
  at the one point a user is about to act on it, rather than leaving it
  silent; the form remains submittable, matching that spec's own choice
  not to block an `http://` source with a credential outright.
- **FR-6** A source's browse view (`/sources/:id`) shows its contents as
  a list of `SourceCandidate` items (`backend-source-adapter.md` FR-9:
  `title`, `author`, `fileReference` — itself carrying format and size
  — and `coverUrl`) with infinite scroll driven by `useInfiniteQuery`
  against `GET /api/v1/sources/:id/browse`'s cursor
  (`frontend-library-screens.md` FR-3's same TanStack Query primitive
  and justification — this endpoint has a real server-provided cursor
  to page with, the same reasoning that made offset/limit the right
  choice for `frontend-discover-screen.md` FR-3 wrong here). A search
  input is shown only when the source's `CanSearch` is `true`
  (`backend-source-adapter.md` FR-5); typing into it switches the same
  list to `GET /api/v1/sources/:id/search`'s results, debounced 300ms
  matching this project's established convention
  (`frontend-library-screens.md` FR-1, `frontend-discover-screen.md`
  FR-1). This introduces a new, small **`<SourceCandidateList>`**
  component — deliberately not a reuse of `<DiscoverResultGrid>`:
  `<DiscoverResultGrid>` (`frontend-discover-screen.md` FR-1) is typed
  to `NormalisedSearchResult`, and that same FR's own reasoning for
  *not* generalising `<WorkGrid>` to fit a second DTO applies here
  identically to a third — `SourceCandidate` has no `openLibraryWorkKey`
  and carries a `fileReference` neither of the other two DTOs has;
  forcing it through `<DiscoverResultGrid>` would repeat the exact
  distortion that spec already rejected one level up.
  `<SourceCandidateList>` instead reuses the same lower-level cover-tile
  and title/author-text primitives (`frontend-component-primitives.md`,
  `frontend-generated-covers.md`) that `<DiscoverResultGrid>` itself was
  built from, so the two views stay visually consistent without sharing
  a data contract — the same relationship `<DiscoverResultGrid>` has to
  `<WorkGrid>`, applied one level further down the same chain. This view
  does not reuse `<WorkGrid>` either, for the same reason
  `frontend-discover-screen.md` FR-1 already gave (no
  ownership/collection-membership fields exist for an unmatched
  candidate).
- **FR-7** Removing a source (FR-1's card, a "Remove" action) shows a
  confirmation step naming the source's `label` before calling `DELETE
  /api/v1/sources/:id` — an irreversible action from the UI's own point
  of view (no undo), warranting the same confirm-before-destructive
  pattern this project's design system uses elsewhere
  (`frontend-component-primitives.md`'s dialog primitive).

## Non-functional requirements

- **Performance** — no new performance budget beyond what
  `frontend-library-screens.md` FR-4's virtualization and
  `frontend-discover-screen.md`'s lazy-loading already established;
  this screen's own data volumes (a handful of configured sources, a
  browsable list bounded by `backend-source-adapter.md` FR-7's own
  50-item page cap) are smaller than either of those screens' worst
  case.
- **Security** — the credential form's `password` input uses
  `autocomplete="new-password"` (never `"current-password"`, which
  would invite a password manager to suggest an unrelated stored
  credential) and the field's value is never logged or included in any
  client-side error report — `frontend-shell-and-routing.md` FR-7's
  correlation-ID error slot carries only the correlation ID and a
  generic message, never form field values, the same discipline that
  slot already has for every other form in this project.
- **Accessibility** — `frontend-accessibility.md`'s existing keyboard
  map and screen-reader conventions apply: the credential form's fields
  have accessible labels distinguishing `username`/`password`, health
  status changes are announced via a live region (matching
  `frontend-discover-screen.md`'s degraded-state announcement pattern),
  the "Browse..."/text-field platform split (FR-4) is transparent to a
  screen-reader user either way (one clearly-labelled control, not two
  competing ones visible simultaneously).
- **Reliability** — a failed health check (FR-3) never blocks the rest
  of the screen; a source's card renders fully regardless of its
  current health status, matching `backend-source-adapter.md`'s own
  "create succeeds even if the health check fails" design (FR-1).
- **Observability** — no new client-side logging; reuses
  `frontend-shell-and-routing.md` FR-7's existing slot unchanged.

## Domain model

No new domain types on the frontend. This screen renders
`backend-source-adapter.md`'s wire shapes (`Source`, `SourceCandidate`)
directly. It has no concept of `domain-source.md`'s `SourceOffering` at
all — nothing in this screen's data ever reaches that far, per
`backend-source-adapter.md` FR-10's own boundary.

## API and contracts

Consumes `backend-source-adapter.md`'s full `/api/v1/sources*` surface
(FR-1 through FR-9 there) and `desktop-host-ipc-surface.md`'s amended
`source.pickLocalFolder()` (FR-6 there). No new backend or IPC surface
of its own.

## State transitions

Per source card: `creating → created (health: checking) → reachable |
unreachable(detail)`, with `reachable ⇄ unreachable(detail)` freely
transitioning on any subsequent check (FR-3), matching
`backend-source-adapter.md`'s own State transitions section exactly —
this screen adds no state of its own beyond what that API already
expresses. The credential sub-form: `hidden → editing → submitted
(collapses back to "Password set" static text)` — never
`editing → pre-filled`, since pre-filling is structurally impossible
(Domain model above).

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `POST /api/v1/sources` fails validation (bad `kind`, malformed URL) | `400 InvalidInput` | Field-level error text next to the offending field, not a generic banner | No source created |
| Health check reports any `detail` category | `backend-source-adapter.md` FR-6's response | The matching FR-3 state, with a "Check again" action | No automatic retry |
| `window.alexandryn.source.pickLocalFolder()` rejects (Electron-side error, not a normal cancel) | IPC call's own rejection | Text field remains editable, a small inline note that the picker failed — never a full-screen error for a non-fatal, optional convenience feature | User can still type the path directly |
| Removing a source fails server-side | Non-2xx response | Confirmation dialog stays open with an inline error, not silently dismissed | Source remains in the list |
| Browse/search page fails (`503 Unavailable`) | TanStack Query error state | "This source is unavailable right now" — distinct wording from `frontend-discover-screen.md` FR-5's Open-Library-specific copy, since this is a different, user's-own source, not a shared third-party service | No automatic retry beyond TanStack Query's own bounded default |

## Security considerations

- **Credentials never round-trip** — FR-5's structural guarantee, not
  a UI convention that could be bypassed by a future change: the API
  response shape itself (`hasCredential: boolean`) makes displaying a
  stored password impossible from this screen's own data, not merely
  discouraged.
- **`autocomplete="new-password"`** — prevents an unrelated saved
  credential from being suggested into a source's password field.
- **The native folder picker crosses no new trust boundary beyond
  `desktop-host-ipc-surface.md`'s existing one** — this screen only
  ever calls the one enumerated operation (FR-4); it does not read or
  write files itself, and the returned path is sent to the backend the
  same way a typed path would be, validated there
  (`backend-source-adapter.md` FR-3) before anything trusts it.
- **No rendering of raw source content beyond validated fields** — every
  string this screen renders (`SourceCandidate.title`/`author`) has
  already passed `backend-source-adapter.md` FR-9's normalisation and
  validation; same reasoning `frontend-discover-screen.md`'s Security
  considerations already gave for Open Library data, applied here to a
  second external data source.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Health-status text mapping for each of FR-3's nine `detail` values plus the two non-`detail` states; credential sub-form's collapsed/expanded states; the `window.alexandryn` presence check rendering or hiding "Browse..."; the HTTPS warning (FR-5) rendering only for a non-`https` `baseUrl` |
| Integration | Add/edit/remove flow against MSW fixtures shaped like `backend-source-adapter.md`'s real response types (tier-(a) contract-generated, `frontend-shell-and-routing.md`'s existing two-tier strategy); a mocked `window.alexandryn.source.pickLocalFolder` exercising both the successful-pick and cancel (`null`) paths |
| Contract | Reuses `backend-source-adapter.md`'s `kin-openapi` contract test — no separate frontend contract test |
| E2E | Add a local-folder source (fake backend, fake `window.alexandryn`) → see it reachable → browse it; add an OPDS source with a credential → replace the credential → confirm the old one is never displayed; remove a source with confirmation |
| Accessibility | `frontend-accessibility.md`'s `axe`/keyboard-map requirements applied to the add-source form, credential sub-form, and each health-status state |

Tests that must fail before implementation begins: a test asserting no
DOM node, at any point in the edit flow, ever contains a previously-
entered password value; a test asserting the "Browse..." button is
absent when `window.alexandryn` is undefined; a test asserting each of
FR-3's nine `detail` values renders distinct, non-generic copy.

## Acceptance criteria

- [ ] A local-folder source can be added via native picker (inside
      Electron) or typed path (LAN browser), both landing correctly
- [ ] An OPDS source can be added with or without a credential
- [ ] Health status shows one of FR-3's eleven distinct states, never a
      single generic "error"
- [ ] A stored credential is never displayed or echoed back anywhere in
      this screen
- [ ] Browsing and searching a source's contents work, with search
      hidden entirely when `CanSearch` is `false`
- [ ] Removing a source requires confirmation naming its label
- [ ] Accessibility checks pass for the add-source form, credential
      sub-form, and every health-status state

## Open questions

- **No design-reference screen for source management** — same gap
  `frontend-discover-screen.md` already named for Discover; this
  spec's layout choices (card list, inline health status, kind-specific
  form fields) are reasoned defaults, not derived from a capture.
  Escalate for a design-project-owner decision if a real capture later
  contradicts these choices.
- **No "add to library" action from a browsed/searched source item** —
  acceptable for this phase (phase 10 owns it), flagged so it isn't
  mistaken for an oversight, same framing `frontend-discover-screen.md`
  already used for its own equivalent gap.

## References

- `backend-source-adapter.md` (phase 08) — `/api/v1/sources*` shapes
  this screen renders
- `desktop-host-ipc-surface.md` (phase 05, amended FR-6) —
  `source.pickLocalFolder()`
- `domain-source.md` (phase 02) — `Source`/capability model, `Kind`'s
  display-only framing
- `frontend-discover-screen.md` (phase 07) — `<DiscoverResultGrid>`
  reuse, degraded-state announcement pattern, unmatched-candidate
  framing precedent
- `frontend-library-screens.md` (phase 06) — cursor-based
  `useInfiniteQuery` precedent, debounce value
- `frontend-shell-and-routing.md` (phase 04) — shell composition,
  MSW two-tier mock strategy, correlation-ID error slot,
  `useCapability()` (explicitly distinguished from this spec's own
  platform-detection check, FR-4)
- `frontend-component-primitives.md` (phase 04) — `<SegmentedControl>`,
  dialog/confirmation primitive
- `frontend-accessibility.md` (phase 04) — keyboard/screen-reader
  requirements applied here
- Constitution §11 (interface copy), §8 (credential handling, applied
  to this screen's own form)
