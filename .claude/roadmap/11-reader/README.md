# Phase 11 — Reader

| | |
|---|---|
| **Status** | Specs reviewed, findings fixed, awaiting maintainer approval |
| **Depends on** | Phase 06, Phase 10 |
| **Blocks** | 14 |
| **Opened** | — |
| **Closed** | — |

## Objective

The in-browser reader: EPUB rendering, typography preferences, position
tracking, bookmarks, highlights, table of contents — tested against a
real corpus of imported files, with EPUB content sandboxed as hostile
by default. This phase resolves the standing open question this
project has carried since phase 01 ("how the reader renders EPUB, and
in what sandbox") and gives `domain-reading.md`'s (phase 02)
`PrecisePosition` field — deliberately left "opaque to this spec" —
its first concrete shape.

## Why here

It needs a real, owned `Edition` to read (phase 06's library, phase
10's import — a book has to exist before it can be opened) and no
earlier phase renders any file content at all, only metadata. It sits
before phase 14 (cross-device sync) because sync needs something real
to synchronise; `domain-reading.md` FR-6/FR-7's `ReconcileProgress`
already exists as a pure domain function, but nothing calls it yet.

## Scope

**In**

- EPUB rendering: paginated and continuous-scroll modes, typography
  (font, size, line spacing) and theme preferences
  (`domain-reading.md` FR-5's `ReadingPreferences`)
- Reading position tracking using EPUB CFI (Canonical Fragment
  Identifiers, the real interoperable standard) as `PrecisePosition`'s
  concrete shape for this format
- Bookmarks, highlights, table of contents
- Sandboxed rendering: no script execution from EPUB content, no
  external network requests from within a book (constitution's "what
  someone reads is private" applied to the sandbox boundary itself,
  not just to logging)
- A new backend content-serving path: EPUB spine/resource content
  resolved, sanitised, and served on demand — the second real
  file-content-parsing boundary this project has built (after phase
  10's metadata extraction), reusing that phase's zip-bomb/zip-slip
  defences rather than re-deriving them

**Out**

- PDF or CBZ reading — `domain-reading.md`'s own model is format-
  agnostic enough to support them later (its Failure modes table
  explicitly names "a PDF page number applied to an EPUB" as an
  illegal case, implying the domain always anticipated more than one
  format), but this phase's actual rendering surface is EPUB only,
  matching the roadmap's own original scope; PDF/CBZ reading is real,
  deliberately deferred scope, not an oversight — no phase currently
  named picks it up
- Cross-device sync of position — phase 14; `ReconcileProgress` is
  wired to a single-device write path this phase builds, not a real
  multi-device sync transport yet
- EPUB scripted-content profile (fixed-layout books requiring
  JavaScript) — explicitly unsupported, not merely unsandboxed;
  constitution's "no script execution from EPUB content" is treated
  as a hard requirement, not a configurable trust decision the user
  can opt into per book

## Specifications

| Spec | Covers |
|---|---|
| `backend-reader-content.md` | EPUB content resolution, sanitisation, and serving; sandbox-supporting response headers; reused adversarial-file defences |
| `backend-reading-api.md` | `ReadingProgress`/`Bookmark`/`Highlight`/`ReadingPreferences` persistence, `ReconcileProgress` wiring, `/api/v1/reading*` endpoints |
| `frontend-reader.md` | Rendering engine integration, pagination/scrolling, typography/theme UI, TOC, bookmarks/highlights UI, position reporting |

## Architecture decisions expected

- **Rendering engine**: researched against real options —
  `futurepress/epub.js` (6.9k stars, iframe-based, sandboxes content,
  scripts disabled by default, but its own maintainers' forks cite
  architectural problems with its layout-rebuild model) versus
  `foliate-js` (pure JS, no hard dependencies, real EPUB CFI support
  via its own `epubcfi.js`, explicit reflowable/fixed-layout paginators
  — but its own README admits instability ("expect it to break... at
  any time") and its default blob:-URL rendering approach is
  explicitly documented by its own maintainer as *not* securely
  sandboxable as shipped). `frontend-reader.md`'s own case to make,
  not assumed here — leaning `foliate-js` for its real CFI support and
  lighter footprint, paired with server-side sanitisation
  (`backend-reader-content.md`) specifically to close the gap its own
  maintainer names, rather than accepting that gap as-is.
- **EPUB content sanitisation**: Go-side, via `microcosm-cc/bluemonday`
  (an allowlist HTML sanitiser built on Go's own `net/html` tokenizer)
  — `backend-reader-content.md`'s own case to justify concretely under
  constitution §9, closing the exact security gap `foliate-js`'s own
  documentation names (client-side-only blob-URL rendering "is
  currently impossible to do securely") by never handing the client
  unsanitised content in the first place.
- **`PrecisePosition`'s concrete shape**: EPUB CFI strings — the real,
  standardised (W3C/IDPF) format designed specifically for
  cross-reading-system position interoperability, not an invented
  addressing scheme.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| A malicious EPUB's XHTML/CSS content executes a script, exfiltrates reading activity via an external resource request, or escapes its sandbox | Medium | High | This phase's own sharpest named risk — closed in two independent layers: server-side sanitisation (`backend-reader-content.md`, strips scripts/event handlers/external URLs before the client ever sees content) plus a strictly-sandboxed iframe with no `allow-scripts` client-side, neither treated as sufficient alone |
| The chosen rendering engine's own instability (`foliate-js`'s explicit "expect it to break") causes a real regression | Medium | Medium | Named honestly in Architecture decisions expected, not hidden; `frontend-reader.md`'s own dependency justification must weigh this explicitly against the alternative's architectural concerns, not pick either option by default |
| A malicious EPUB entry requested at *read* time (not import time) is a zip bomb or path-traversal attempt, bypassing phase 10's import-time extraction checks because reading opens a different code path | Medium | High | `backend-reader-content.md` explicitly reuses `backend-file-extractors.md`'s size/entry-count/path-safety defences rather than assuming import-time validation covers a later, independent read-time access |
| EPUB CFI's real complexity (it is a genuinely intricate addressing grammar) is mis-implemented, producing positions that don't survive a re-open or don't match across a re-paginated layout | Medium | Medium | *Generation and resolution* reuse a library's own CFI implementation (`foliate-js`'s `epubcfi.js`) rather than hand-rolling the grammar, consistent with this project's general "don't reinvent a well-specified standard" posture; `backend-reading-api.md`'s own server-side check is a separate, narrower problem (rejecting an obviously-hostile string before storage) and is deliberately scoped as a shallow structural check, not a claim of full grammar validation — the two specs' own FRs state this distinction explicitly so they don't silently disagree about what "CFI validated" means |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | CFI generation/parsing round-trips against real fixture EPUBs; sanitisation stripping scripts/event handlers/external URLs from crafted hostile XHTML/CSS fixtures; `ReconcileProgress` reuse (already tested in `domain-reading.md`, not re-tested here) |
| Integration | Content-serving endpoint against a real imported EPUB and a deliberately hostile one (zip bomb, zip slip, script-laden content), reusing `backend-test-harness.md`'s harness; reading API CRUD against a real PostgreSQL instance |
| Contract | `architecture-contracts.md` FR-3's contract test, extended to `/api/v1/reading*` |
| E2E | Open a real imported EPUB → read across a pagination boundary → close and reopen → position restored; a script-injection fixture confirmed inert (constitution's own exit criterion, made concrete) |
| Accessibility | `frontend-accessibility.md`'s requirements applied to the reader UI itself — keyboard page-turning, screen-reader table-of-contents navigation, respected reduced-motion for page-turn animation — the phase's own named "full keyboard and screen-reader operability" exit criterion |

## Security considerations

The second real file-content-parsing boundary this project has built,
and the first that serves content *back* to a client rather than only
extracting metadata from it server-side:

- **No script execution from EPUB content, verified, not assumed** —
  this phase's own named exit criterion; a concrete test fixture with
  injected script content is part of Test strategy, not left to the
  rendering library's own defaults
- **No external network requests from within a rendered book** — the
  concrete mechanism protecting constitution's "what someone reads is
  private" at the rendering boundary itself: an external image/font/
  stylesheet reference inside a book would otherwise leak reading
  activity (and the reader's IP) to a third party the instant a page
  renders, independent of anything this project logs
- **Read-time content access reuses, not re-derives, phase 10's
  adversarial-file defences** — named explicitly above (Risks) since a
  reader opening a book is a materially different code path from
  import-time extraction, and the same zip-bomb/zip-slip risk exists
  at both points

## Observability

Per constitution §8 and `domain-reading.md`'s own Security
considerations (restated there specifically for this domain): no
`Work`/`Edition` title, highlight text, bookmark label, or reading
position ever appears in a log line, even indirectly (a log entry
naming an ID alongside a reading-related event is exactly the leak
that section already names). Sanitisation and content-serving failures
log at `info`/`warn` per `backend-errors-and-logging.md`'s existing
discipline, with the failure category only, never the offending
content.

## Exit criteria

- [ ] All three specifications `APPROVED` with recorded reviews
- [ ] Renders real imported EPUBs correctly across screen sizes
- [ ] Position reliably saved and restored, using EPUB CFI
- [ ] EPUB sandbox verified against a script-injection test case
- [ ] Full keyboard and screen-reader operability
- [ ] Test coverage across unit, integration, and E2E for this slice
- [ ] All specs in this phase are `VERIFIED`
- [ ] Security audit recorded in `.claude/audits/` with no open Critical
      or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
