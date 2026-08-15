# Spec: Frontend reader

| | |
|---|---|
| **Status** | `REVIEWED` |
| **Phase** | `11-reader` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0038`](../reviews/0038-phase11-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (7 Blocking across the batch, 3 confirmed independently by both passes; 9 Major, 6 Minor), all findings fixed; awaiting maintainer approval |

## Context

`backend-reader-content.md` serves sanitised EPUB content on demand;
`backend-reading-api.md` persists position/bookmarks/highlights/
preferences. This spec is the screen that ties both together — the
reader itself. `frontend-library-screens.md` FR-5 was amended to add
the "Read" entry point this spec's own route is the target of.

Grounded against real research (also informing the roadmap's own
Architecture decisions expected): **`foliate-js`** is this spec's
chosen rendering engine — pure JS, no hard dependencies, real EPUB CFI
support (`epubcfi.js`), explicit reflowable (CSS-multi-column
paginator) and fixed-layout renderers, and both paginated and scrolled
modes built in. Its own documentation is explicit that its *default*
approach (unzip client-side, serve via `blob:` URLs) "is currently
impossible to do securely" — this project doesn't take that default:
`backend-reader-content.md` serves already-sanitised content from this
system's own API origin instead, so this spec's `<iframe>` points
directly at real URLs, never a client-reconstructed `blob:` one.

## Problem

Nothing renders EPUB content, tracks reading position, or lets a user
set typography/theme preferences, create bookmarks/highlights, or
navigate a table of contents.

## Goals

- Render an owned EPUB's content via `foliate-js`, inside a strictly
  sandboxed `<iframe>`, pointed at `backend-reader-content.md`'s
  content endpoint
- Paginated and continuous-scroll reading modes
- Typography (font, size, line spacing) and theme preferences, synced
  to `backend-reading-api.md`
- Position tracking (EPUB CFI) reported on navigation, restored on
  reopen
- Bookmarks, highlights, table of contents
- Full keyboard and screen-reader operability (this phase's own named
  exit criterion)

## Non-goals

- PDF/CBZ rendering — `backend-reader-content.md`'s own scope cut,
  mirrored here; a non-EPUB owned edition has no "Read" action at all
  (`frontend-library-screens.md` FR-5's amendment)
- Cross-device sync UI (seeing another device's live position update
  in real time) — phase 14; this screen reports and restores its own
  device's position via ordinary request/response, no live transport
- Any script-execution affordance for EPUB content, even opt-in — a
  hard requirement (constitution, this phase's own named exit
  criterion), not a per-book trust decision this UI exposes

## User stories

- As **someone opening a book from their library**, I want it to open
  right where I left off, so I don't have to hunt for my place.
- As **someone reading at night**, I want a dark theme and adjustable
  text size that stick to this specific device, so my phone and my
  desktop can each be set up the way I like.
- As **someone using a screen reader**, I want the table of contents
  and page navigation to be fully operable without a mouse, so the
  reader is actually usable, not just visually present.

## Functional requirements

- **FR-1** `/read/:workId/:editionId` renders the reader — both IDs
  carried in the route (`frontend-library-screens.md` FR-5's amended
  "Read" action already has both from its own `/book/:id` URL, so no
  separate edition-to-work lookup is needed; `:workId` is what FR-5/
  FR-6's progress-reporting calls require, per `domain-reading.md`
  FR-1's Work-scoped `ReadingProgress` singleton, while `:editionId`
  is what `backend-reader-content.md`'s content endpoint and
  FR-7's Edition-scoped bookmarks/highlights require): a strictly
  sandboxed
  `<iframe sandbox="allow-same-origin">` (**no** `allow-scripts` —
  EPUB content never executes, full stop, `backend-reader-content.md`
  FR-9's CSP is this UI's second, independent layer, not a substitute
  for this attribute) whose content documents are fetched through
  `foliate-js`'s renderer, each resource request pointed at
  `GET /api/v1/library/editions/:editionId/reader/content/*path`
  directly — never a `blob:` URL constructed from client-side-unzipped
  bytes, the specific choice that closes `foliate-js`'s own documented
  security gap (Context).
- **FR-2** A mode toggle — **paginated** (`foliate-js`'s
  CSS-multi-column paginator, page-turn navigation) or **continuous
  scroll** — persisted per `DeviceID` alongside FR-4's typography
  preferences (`backend-reading-api.md`'s `ReadingPreferences`, this
  spec's own addition to that type's "opaque to this spec, phase 11
  defines the actual set" field list). Paginated is the default,
  matching the design reference's own book-reading visual intent where
  captured (`frontend-discover-screen.md`'s and
  `frontend-source-management.md`'s own precedent: when no reader
  screen exists in `.design-reference/` — flagged in Open questions,
  same gap those two specs already named for their own screens — a
  reasoned default stands in, not a guess presented as certain).
- **FR-3** Table of contents (from the EPUB's own navigation document,
  parsed by `foliate-js`) renders as an accessible, keyboard-navigable
  list in a collapsible panel; selecting an entry navigates the
  `<iframe>`'s content to that position and reports it via FR-6.
- **FR-4** Typography/theme preferences (`font`, `fontSize`,
  `lineSpacing`, `theme`) are read from `GET /api/v1/reading/preferences`
  on mount and written via `PUT` on change (`backend-reading-api.md`
  FR-8), debounced 500ms (a slower debounce than
  `frontend-library-screens.md`'s 300ms search debounce, since a
  typography change is a deliberate, occasional adjustment, not
  keystroke-by-keystroke input — a different UX case warranting its
  own number, not a citation of that spec's unrelated value). Applied
  to the `<iframe>`'s content via `foliate-js`'s own CSS-injection
  API, not by modifying `backend-reader-content.md`'s served content
  server-side — a client-local rendering concern, the same
  UI-local-vs-server-data split `architecture-frontend.md` FR-2
  already established, applied here to "how content looks" versus
  "what content is."
- **FR-5** Reading position is tracked continuously as the user
  navigates (a page turn, a scroll-position change past a debounce
  threshold) via `foliate-js`'s own CFI-generation API for the
  currently-visible content — never hand-derived from scroll offset
  or page number, which would produce a CFI-shaped string without the
  library's own correctness guarantees for one. On navigation, a CFI
  is generated and queued for FR-6's reporting; on mount, the reader
  opens directly at the last-reported `PrecisePosition.cfi`
  (`GET /api/v1/reading/works/:workId/progress`), falling back to
  `Percentage`-derived position (an approximate location `foliate-js`
  can compute from a fractional value) when no `PrecisePosition`
  exists for this specific `Edition` — `domain-reading.md` FR-2's own
  fallback rule, restated here as this screen's concrete behaviour.
- **FR-6** Position reports are sent via `POST
  /api/v1/reading/works/:workId/progress` (`backend-reading-api.md`
  FR-2), debounced **3 seconds** after navigation settles (not on
  every intermediate scroll event) and additionally on page
  unload/navigation-away (a synchronous best-effort report, accepting
  it may not always land — `backend-reading-api.md`'s own reconcile
  step means a missed final report simply isn't reflected until the
  next successful one, not a correctness failure, since
  `ReconcileProgress` never assumes every report arrives).
- **FR-7** Bookmarks and highlights: a "Bookmark this page" action
  (`POST /api/v1/reading/editions/:editionId/bookmarks`, current CFI,
  FR-5's same generation mechanism) and a text-selection-triggered
  "Highlight" action inside the sandboxed `<iframe>` (`POST
  .../highlights`, start/end CFI from the selection range, an optional
  note and an optional category/colour picked from a small, fixed set
  of swatches — `backend-reading-api.md` FR-7's `category` field, kept
  in parity with the domain model rather than silently always
  submitted as `null`, both entered via a small inline form). Both
  render in a side panel alongside the table of contents (FR-3), each
  entry navigable — clicking one moves the reader to that position,
  the same navigation path FR-3's TOC entries use.
- **FR-8** `DeviceID` (`backend-reading-api.md` FR-1's required
  header) is generated once, client-side, on first use of this screen
  — a `crypto.randomUUID()` value — and persisted in `localStorage`
  (the same zero-dependency justification `frontend-library-screens.md`
  FR-1 already established for its own `localStorage` use), sent as
  `X-Device-Id` on every `backend-reading-api.md` request this screen
  makes. This value is never displayed to the user or treated as an
  identity — purely a mechanism this spec needs to satisfy
  `backend-reading-api.md`'s own required header.

## Non-functional requirements

- **Performance** — `foliate-js`'s own pagination/CFI machinery
  governs rendering performance; no additional budget fixed here
  beyond what that library provides, consistent with this spec's
  choice to lean on a real rendering engine rather than reinvent one.
- **Responsive layout** — this phase's own named exit criterion
  ("renders real imported EPUBs correctly across screen sizes"): the
  reader's own chrome (TOC panel, bookmarks/highlights panel,
  typography controls) collapses to an overlay rather than a
  persistent side panel below `frontend-shell-and-routing.md`'s
  existing tablet/mobile breakpoint, and `foliate-js`'s own reflowable
  pagination is what keeps the *content* itself correctly laid out at
  any viewport width — this spec adds no separate content-reflow logic
  of its own, only chrome-layout responsiveness on top of it.
- **Security** — see dedicated section below.
- **Accessibility** — `frontend-accessibility.md`'s existing keyboard
  map extended with reader-specific bindings (page-forward/back,
  TOC-panel toggle, bookmark-current-page); the table of contents and
  bookmarks/highlights panels use semantic list/nav landmarks, fully
  operable by keyboard and screen reader (this phase's own named exit
  criterion); page-turn transitions respect `prefers-reduced-motion`
  (an instant cut, no animation, when set), the same convention
  `frontend-accessibility.md` already established generally, applied
  here to this screen's own new transition.
- **Reliability** — a `backend-reader-content.md` `503` (every source
  unreachable) surfaces as "This book's source isn't reachable right
  now" — distinct wording from `frontend-discover-screen.md`'s
  Open-Library-specific and `frontend-source-management.md`'s
  generic-source-unavailable copy, since this is neither (it's the
  user's own already-owned book, temporarily unreachable) — with a
  retry action, not a dead end.
- **Observability** — no new client-side logging beyond
  `frontend-shell-and-routing.md` FR-7's existing correlation-ID slot;
  explicitly, no reading position, book title, or highlight content
  ever included in any client-side error report this screen produces
  (constitution §8, restated at the UI layer).

**Dependency justification (constitution §9)**: `foliate-js` — what it
does: EPUB parsing, pagination (reflowable and fixed-layout), and CFI
generation/parsing in pure JS with no hard dependencies. Why not hand-
rolled: correct CFI generation against arbitrary EPUB structure and
correct CSS-multi-column pagination are both substantial, well-trodden
problems (unlike phase 10's simple zip+XML metadata parsing, which
stayed stdlib-only) — reinventing either risks subtle correctness bugs
a maintained library has already worked through. What breaks if
abandoned: the library's own README warns it is "not stable" and its
API may change — a real, named risk (roadmap Risks table), not hidden;
its modular design (`epubcfi.js`, `paginator.js` as separable pieces)
means a future replacement could plausibly swap the rendering layer
while keeping this spec's own CFI-shaped position data intact, since
CFI itself is a standard, not `foliate-js`'s own invention.

## Domain model

No new domain types. This screen renders `backend-reader-content.md`'s
served content and `backend-reading-api.md`'s wire shapes
(`ReadingProgress`, `Bookmark`, `Highlight`, `ReadingPreferences`)
directly.

## API and contracts

Consumes `backend-reader-content.md`'s content endpoint (as `<iframe>`/
resource-fetch targets, not JSON) and `backend-reading-api.md`'s full
`/api/v1/reading*` surface. No new backend surface of its own.

## State transitions

`closed → opening (fetching last position) → open (rendering) →
navigating (a page turn/scroll, CFI regenerated) → open`. Bookmark/
highlight creation and TOC navigation are sub-transitions within
`open`, not separate top-level states. `open → closed` on navigating
away, triggering FR-6's best-effort final position report.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `backend-reader-content.md` returns `503` (source unreachable) | Content fetch failure | "This book's source isn't reachable right now," retry action | No partial/broken render |
| A content resource returns `404` (a manifest reference to a missing entry — a malformed EPUB) | Content fetch failure | "This book's file appears to be incomplete or corrupted" | The specific missing resource is skipped where possible (a missing image doesn't block surrounding text), the whole page fails only if the spine document itself is missing |
| Position report (FR-6) fails | Mutation error | No user-visible error for a background report — silent, logged via the correlation-ID slot only | Retried on the next debounce cycle or navigation event, not immediately re-attempted in a tight loop |
| Preferences load/save fails | Query/mutation error | Preferences UI shows an inline error, reader itself still renders with defaults | Reading is never blocked by a preferences failure |
| Malformed/nonsensical stored CFI (`backend-reading-api.md`'s own named Open question) | `foliate-js` fails to resolve the position | Falls back to `Percentage`-derived position, same as no `PrecisePosition` existing at all | No error surfaced — treated as the fallback case, not a failure |
| A content resource returns `400` (`backend-reader-content.md` FR-5, e.g. a standalone SVG cover) | Content fetch failure | Same as the `404` row above — a corrupted/unsupported-content state, not distinguished further at this UI layer | The specific resource is skipped where possible |

## Security considerations

- **No script execution, two independent layers** — the `<iframe>`'s
  own `sandbox` attribute (no `allow-scripts`, FR-1) and
  `backend-reader-content.md` FR-9's CSP header; this screen never
  weakens either for any reason, including a future "advanced" EPUB
  feature request — restated as a hard line, not a default that could
  be configured away.
- **`allow-same-origin` without `allow-scripts` is the deliberate
  choice, considered against the alternative** — omitting
  `allow-same-origin` entirely (an opaque-origin iframe) was
  considered and rejected: it would break `foliate-js`'s own resource-
  fetch and text-selection behaviour against content served from this
  system's own API origin (FR-1), and — the reasoning that actually
  matters here — `allow-same-origin` alone, **without**
  `allow-scripts`, carries none of the classic sandbox-escape risk
  that combination is known for (that risk specifically requires
  *both* tokens together, letting scripted content remove its own
  sandbox restrictions from within); with no script execution possible
  at all, `allow-same-origin`'s only practical effect here is letting
  resource loads and selection work against the content origin, which
  is this system's own origin anyway.
- **No `blob:`-URL rendering** — the specific architectural choice
  (FR-1) that avoids `foliate-js`'s own documented insecure default;
  every resource request goes to this system's own API origin.
- **`DeviceID` (FR-8) carries no sensitive meaning** — a bare
  correlation value, never displayed, never treated as identity.
- **Rendered content is already sanitised** — every string/resource
  this screen displays passed `backend-reader-content.md` FR-6/FR-7's
  sanitisation before this screen ever received it; this screen adds
  no further trust decision on top.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Preference debounce/apply logic; CFI-based position restore including the `Percentage`-fallback case; bookmark/highlight panel rendering |
| Integration | Reader flow against MSW fixtures shaped like `backend-reading-api.md`'s real response types and a fixture EPUB served through a fake content endpoint (tier-(a) contract-generated, `frontend-shell-and-routing.md`'s existing two-tier strategy) |
| Contract | Reuses `backend-reading-api.md`'s `kin-openapi` contract test — no separate frontend contract test |
| E2E | Open a real imported EPUB from its work-detail "Read" action → navigate several pages → close → reopen → position restored; a script-injection fixture confirmed inert inside the rendered `<iframe>` (this phase's own named exit criterion, exercised end to end at the UI layer, not only `backend-reader-content.md`'s own server-side test) |
| Accessibility | `frontend-accessibility.md`'s `axe`/keyboard-map requirements applied to the reader controls, TOC panel, and bookmarks/highlights panel — this phase's own named "full keyboard and screen-reader operability" exit criterion |
| Responsive | The reader's chrome (TOC/bookmarks panels, typography controls) tested at both a desktop window width and a narrow LAN-client viewport width — this phase's own named "across screen sizes" exit criterion, exercised explicitly rather than left implicit in the E2E row above |

Tests that must fail before implementation begins: a test asserting
the `<iframe>`'s `sandbox` attribute never includes `allow-scripts`
under any code path, including preference/theme changes; a test
asserting a script-injection fixture's script never executes when
rendered; a test asserting reopening a book restores the exact last-
reported CFI position, not merely an approximate one, when a
`PrecisePosition` exists for that Edition.

## Acceptance criteria

- [ ] A real imported EPUB renders correctly, paginated and scrolled
      modes both functional
- [ ] Typography/theme preferences persist per device and apply
      correctly
- [ ] Position is saved and restored using CFI, with correct fallback
      to `Percentage` when no precise position exists for the current
      Edition
- [ ] Bookmarks and highlights can be created, viewed, and navigated to
- [ ] The `<iframe>` sandbox never permits script execution, proven by
      a test with real injected script content
- [ ] Reader chrome remains usable at both a desktop and a narrow
      LAN-client viewport width, proven by a test, not visual
      inspection alone
- [ ] Full keyboard and screen-reader operability, proven by
      accessibility tests, not just visual inspection

## Open questions

- **No design-reference screen for the reader** — the same gap
  `frontend-discover-screen.md` and `frontend-source-management.md`
  already named for their own screens; FR-2's paginated-default choice
  and this screen's overall layout are reasoned defaults, not derived
  from a capture.
- **`foliate-js`'s own stated instability** — named honestly in the
  roadmap's Risks table and this spec's own dependency justification;
  no mitigation beyond "watch it closely" is proposed here, since a
  concrete fallback plan would be premature before real usage surfaces
  whether the risk materialises.
- **Highlight creation UX inside a sandboxed iframe** — text selection
  across cross-origin-*like* sandboxed content has known browser
  quirks (`sandbox="allow-same-origin"` without `allow-scripts` still
  permits selection, but the exact event-wiring between the iframe's
  selection and this screen's own "Highlight" button needs
  implementation-time verification against real browsers, not assumed
  to just work identically everywhere).

## References

- `backend-reader-content.md` (phase 11, sibling) — content this
  screen renders; the specific `blob:`-avoidance architecture this
  spec's FR-1 depends on
- `backend-reading-api.md` (phase 11, sibling) — position/bookmarks/
  highlights/preferences this screen reads and writes
- `frontend-library-screens.md` (phase 06, amended FR-5) — the "Read"
  entry point this screen's route is the target of, and the source of
  both `:workId`/`:editionId` this spec's FR-1 route carries
- `backend-library-api.md` (phase 06, amended FR-5) — the `formats`
  field `frontend-library-screens.md`'s amended "Read" gate is
  computed from
- `frontend-shell-and-routing.md` (phase 04) — shell composition, MSW
  two-tier mock strategy, correlation-ID error slot
- `frontend-accessibility.md` (phase 04) — keyboard/screen-reader
  requirements, `prefers-reduced-motion` convention
- `frontend-library-screens.md` (phase 06) FR-1 — `localStorage`
  justification precedent reused for `DeviceID` (FR-8)
- `architecture-frontend.md` (phase 01) FR-2 — UI-local-vs-server-data
  split, applied to typography rendering (FR-4)
- `domain-reading.md` (phase 02) FR-2 — `Percentage`-fallback rule
  this screen's FR-5 implements concretely
- Constitution §7 (accessibility as part of done), §8 (reading
  privacy), §9 (dependency justification for `foliate-js`)
