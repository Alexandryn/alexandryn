# Phase 11 — Reader: Implementation Plan

**Status (2026-09-01):** Gate 1 scope approved. Maintainer decisions:
**A1** (amend `domain-reading.md` FR-6/FR-7 to the `(epoch, percentage)`
model, realign `backend-reading-api.md`, then build the whole phase),
**B** (keep `columnWidth` + `lineSpacing`), **C** (both reading modes,
paginated default), **D** (reading-data export enters this phase as a new
sibling spec — read-only JSON + web download; import + native save
deferred), **E** (fix the stale `frontend-reader.md` open-question).
The amendment package (spec edits + ADRs 0023/0024 + new spec
`reading-data-export.md` + roadmap/index updates) is drafted and awaiting
a final sign-off on the diff before implementation code begins.

**Branch:** `feat/phase11-reader` from clean `main` (commit `cc522d1`).

**Governing documents:**

- `CLAUDE.md`, `.claude/constitution.md` (§1, §2, §3, §4, §5, §7, §8, §9, §10, §11, §12)
- `.claude/roadmap/11-reader/README.md`
- `.claude/specs/domain-reading.md` (`APPROVED` — **FR-6/FR-7 not implementable until amended**)
- `.claude/specs/backend-reader-content.md` (`APPROVED`)
- `.claude/specs/backend-reading-api.md` (`APPROVED`)
- `.claude/specs/frontend-reader.md` (`APPROVED`)
- `.claude/reviews/0038-phase11-cross-spec-review.md`, `.claude/reviews/0048-phase02-correctness-review.md`, `.claude/reviews/0049-real-world-edge-case-conformity-review.md`
- `.claude/decisions/0009-progress-attachment.md`, `.claude/decisions/0020-graph-invariants-in-domain-services.md`, `.claude/decisions/0021-transaction-contract-and-event-outbox.md`

---

## 0. Design reference conformance (ADR 0003 check)

- **Canvas consulted:** `Alexandryn-Web.dc.html`, `atReader` block
  (lines 428–530) plus its `rd:` state and `RTHEMES`/`SAMPLES`/`MARKS`
  mock data (lines 719–890). Read fresh at drafting time, not from memory.
- **`ANALYSIS.md` sync date as read at drafting:** last synced **2026-08-13**
  (second sync, byte-identical re-pull noted). Scope-classification pass
  dated **2026-08-17**.
- **Classification:** `atReader` (Web) = **Binding** — "Phase 11, approved
  spec; confirmed intentionally single-captured (`architecture-system.md`
  FR-6, `frontend-reader.md` NFRs)". Not unclassified, not exploratory.
- **Contradiction found — flagged for Gate 1, not reasoned around:**
  `frontend-reader.md`'s Open questions still say *"No design-reference
  screen for the reader"*. That is **stale**. The spec was approved
  2026-08-15; the classification pass that recorded `atReader` as a
  captured, binding screen is dated 2026-08-17 — two days later. A binding
  captured reader screen exists and the approved spec believes it does not.
- **Divergences between `atReader` and `frontend-reader.md` scope:**
  1. The canvas exposes a **column-width** control (Narrow / Default /
     Wide) that no spec FR names. Spec FR-4's preference set is
     `font, fontSize, lineSpacing, theme`.
  2. The canvas shows **no paginated/continuous-scroll toggle**; its
     content region is a single scrolling column with a page counter.
     Spec FR-2 requires both modes with a persisted toggle, paginated
     default.
  3. The canvas has **no visible line-spacing control**; the spec has one
     and no column-width one.
  4. The canvas's Marks panel carries an **Export** action. No spec FR
     names export.
  These are surface-vs-scope tensions (design binding for look, roadmap
  binding for scope, ADR 0003 addendum). Proposed reconciliation is in
  "Open questions for Gate 1" below — not resolved unilaterally.
- **DesignSync re-pull:** tool not reachable in this environment
  (`plugin:github` MCP failed to connect; `DesignSync` deferred-tool not
  exercised). Proceeding against local `.design-reference/` per the
  standing task convention, same as phase 10's plan recorded.

---

## 1. Current state — what already exists

Phase 02 and the phase-10 groundwork already landed a large share of what a
naive reading of the task's Tier 0 would re-create. **No duplicate tables,
types, or repositories will be created.**

| Area | State on `main` | Source |
|---|---|---|
| `reading_progress`, `bookmarks`, `highlights`, `reading_preferences`, `outbox` tables | **Exist** in migration `00002_phase02_schema.sql` | phase 02 |
| `domain.ReadingProgress`, `ProgressReport`, `Bookmark`, `Highlight`, `ReadingPreferences`, `PrecisePosition` types | **Exist** (`internal/domain/reading_progress.go`, `bookmark_and_highlight.go`, `reading_preferences.go`) | commit `623a0c9` (P19–P21) |
| `domain.ReadingProgressService.AttachPrecisePosition` (graph invariant) | **Exists** | commit `623a0c9` |
| `ReconcileProgress` domain function | **Does not exist — deliberately blocked** | roadmap 02 exit criteria; review `0048` finding 1 |
| Domain repo interfaces (`ReadingProgressRepository`, `BookmarkRepository`, `HighlightRepository`, `ReadingPreferencesRepository`) | **Exist** (`internal/domain/repository.go`) | commit `2489017` (R8) |
| Postgres repos for all four + integration tests | **Exist** (`internal/persistence/postgres/*_repository.go`) | commit `2489017` (R8) |
| `internal/reader/*` (content serving + reading API transport) | **Does not exist** | — this phase |
| `extract.Materialize`, decompressed/entry-count caps, `BoundedReader` | **Exist**, reusable as-is | phase 10 |
| `sources.Provider.Resolve(ctx, FileReference) (io.ReadCloser, error)` | **Exists** | phase 08 |
| `SourceOfferingRepository.FindByEdition` | **Missing** — only `FindByID`, `FindBySource` | — this phase adds it |
| OpenAPI `/api/v1/reading*` + `/api/v1/library/editions/{id}/reader/*` paths | **Missing** | — this phase |
| `web/` reader screen, `web/src/data/reading.ts`, foliate-js dep | **Missing** | — this phase |
| ADRs 0023 (rendering engine), 0024 (HTML sanitiser) | **Missing** | — this phase |

---

## 2. The `ReconcileProgress` blocker

`backend-reading-api.md` FR-2 (the `POST .../progress` endpoint) is built
entirely on `domain.ReconcileProgress`. That function **must not be
implemented** until `domain-reading.md` FR-6/FR-7 are amended
(`domain-reading.md` status line; roadmap 02 exit criteria; review `0048`
findings 1, 2, 11).

Review `0048` already decided the fix in principle:

> reconciliation becomes max over `(epoch, percentage)`; server-assigned
> epoch, observed-epoch precondition for the offline case, third
> `Rejected` outcome

But **no spec text has been amended** to match. `domain-reading.md` FR-2/
FR-6/FR-7 still describe furthest-`Percentage`-wins with a free-form
"explicit override", and `backend-reading-api.md` FR-2 still describes
`ReconcileProgress(canonical, report) -> ReadingProgress` with that same
override model and no epoch. The `reading_progress` table has no `epoch`
column.

This is a spec-vs-spec conflict that blocks part of the phase. Per the
constitution's Review-gates clause and CLAUDE.md, resolving it is a
stop-and-ask, not something to reason around. Options are in "Open
questions for Gate 1" below (item A).

Everything else in the phase — content serving, bookmarks, highlights,
preferences, `GET` progress, the whole frontend reader — is unblocked and
does not depend on `ReconcileProgress`.

---

## 3. ADRs to author (Gate 1 scope statement)

### ADR 0023 — Reader rendering engine: `foliate-js`

- **Decision:** vendor `foliate-js` as the client rendering engine — EPUB
  pagination (reflowable CSS-multi-column + scrolled) and CFI
  generation/resolution via its own `epubcfi.js`.
- **Options weighed:** `futurepress/epub.js` (larger, maintainers' own
  forks cite layout-rebuild problems), hand-rolled pagination + CFI
  (rejected — CFI grammar and multi-column pagination are both large,
  well-trodden correctness problems, unlike phase 10's stdlib-only
  zip+XML), `foliate-js` (chosen — real CFI support, no hard deps, both
  modes built in; its own README warns it is unstable and its default
  `blob:`-URL approach is not securely sandboxable — closed here by
  never using that default, see ADR 0024 + `backend-reader-content.md`).
- **§9:** what it does / why not hand-rolled / what breaks if abandoned
  (CFI data is a W3C standard, not foliate's invention — a future swap
  keeps the stored positions).
- **Vendoring mechanism:** pinned exact version. `foliate-js` is
  distributed as ES modules; decision records whether it enters as an
  `npm` dependency (exact pin, no `^`) or a checked-in vendored copy
  under `web/vendor/` — leaning `npm` exact-pin for lockfile visibility
  and `CODEOWNERS` review, matching every other web dep.

### ADR 0024 — Server-side EPUB HTML/SVG sanitisation: `microcosm-cc/bluemonday`

- **Decision:** sanitise all served HTML/XHTML server-side with a
  `bluemonday` policy built from `UGCPolicy()`, further restricted to
  reject non-relative / non-`data:` `href`/`src`, strip
  `<iframe>/<object>/<embed>/<link>`, strip inline `<svg>` wholesale,
  strip `style` attributes, route `<style>` block text through the
  hand-written CSS scheme scanner (`backend-reader-content.md` FR-6/FR-7).
- **Options weighed:** hand-rolled HTML sanitiser (rejected — the classic
  place to smuggle XSS past a naive filter), `bluemonday` (chosen —
  allowlist model on Go's own `net/html` tokenizer, OWASP-Java-Sanitizer
  lineage), strip-all-to-text (rejected — destroys legitimate book
  markup).
- **§9:** built on stdlib tokenizer, self-contained policy object, a
  replacement is one function call's implementation.
- CSS sanitisation stays hand-written (bounded `url(` / `@import` scheme
  scan) — justified separately in the spec as a narrow problem.

---

## 4. Migration `00008_phase11_reader.sql`

Confirmed scope (A1 + D):

```sql
-- +goose Up
ALTER TABLE reading_progress ADD COLUMN epoch BIGINT NOT NULL DEFAULT 0;
ALTER TABLE bookmarks  ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE highlights ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- +goose Down
ALTER TABLE highlights DROP COLUMN created_at;
ALTER TABLE bookmarks  DROP COLUMN created_at;
ALTER TABLE reading_progress DROP COLUMN epoch;
```

- `epoch` — `domain-reading.md` FR-2/FR-6 as amended; `NOT NULL DEFAULT 0`
  so existing rows (none in production yet) get epoch 0.
- `created_at` on `bookmarks`/`highlights` — `reading-data-export.md`
  FR-4; the domain types gain a `CreatedAt` field, set at construction.
- No other schema change: `reading_preferences.settings` is `JSONB` and
  absorbs `{font, fontSize, lineSpacing, theme, layoutMode, columnWidth}`;
  `reading_progress.precise_position_value` already carries the CFI
  string; `bookmarks.position` / `highlights.start_position`/`end_position`
  already carry CFI strings.
- `schema_integration_test.go` gets assertions for the three new columns.

---

## 5. Tiered implementation (Red → Green → Refactor per tier)

### Tier 0 — spec amendments, ADRs, migration  *(amendments done; awaiting diff sign-off)*

- **Done in the amendment package:** ADR 0023, ADR 0024 (`Accepted`);
  `domain-reading.md` FR-2/FR-6/FR-7 + Domain model + State transitions +
  Failure modes + acceptance criteria amended to the `(epoch, percentage)`
  model; `backend-reading-api.md` FR-2/FR-3 + contract + failure modes +
  acceptance criteria realigned; `frontend-reader.md` FR-2/FR-4/FR-5/FR-6
  + Open questions amended; ADR 0009 addendum; new spec
  `reading-data-export.md`; `roadmap/11-reader/README.md`,
  `specs/README.md`, `decisions/README.md` updated.
- **On sign-off, then code:** `go get github.com/microcosm-cc/bluemonday@<pin>`;
  add `foliate-js` per ADR 0023 (both recorded in the PR body, §9);
  migration `00008` (epoch + 2× created_at) + `schema_integration_test.go`
  assertions.

### Tier 1 — `ReconcileProgress` / `OverrideProgress` domain functions

- RED: table-driven — lexical `max` over `(epoch, percentage)`;
  `Advanced` / `Unchanged` / `Rejected` outcomes; below-stored-epoch →
  `Rejected`; clamp of an over-reported `ObservedEpoch`. Property-based:
  (a) fold over same-epoch reports (generator emits **no** overrides) is
  order-independent; (b) `OverrideProgress` then any stale reports keeps
  the override's percentage.
- GREEN: pure functions in `internal/domain/reconcile_progress.go`;
  `ProgressReport` gains `ObservedEpoch`, `ReadingProgress` gains `Epoch`,
  new `ReconcileResult` type.
- The caller (Tier 3) emits one `ReadingProgressUpdated` on `Advanced`/
  `overridden`, none on `Unchanged`/`Rejected` (`domain-events.md` FR-5 —
  no mutation, no event; consistent, no amendment to that spec needed).
- `Bookmark`/`Highlight` gain `CreatedAt` (`reading-data-export.md` FR-4).

### Tier 2 — `internal/reader/content` (backend content serving + sanitisation)

- `SourceOfferingRepository.FindByEdition(editionID) []*SourceOffering`
  ordered `observed_at DESC` — RED integration test first.
- Content resolver: ownership check (`LibraryEntry` join) → resolve bytes
  via each offering's `Provider.Resolve` in turn → `extract.Materialize`
  (reused) → in-process LRU cache (5 editions, 10-min idle, injected
  `Clock`, reference-counted, cache-owned context independent of request
  context) holding the temp file + open `zip.Reader`.
- `*path` validation (`..`/absolute rejected before lookup).
- Per-entry: 200 MiB streaming cap + 10k entry cap (reused constants),
  30 s per-request `ctx` timeout.
- Content-type dispatch by `http.DetectContentType`: HTML/XHTML →
  bluemonday policy; CSS → scheme scanner; image/font (not SVG) → as-is;
  standalone SVG + unrecognised → `400`.
- `Content-Security-Policy: default-src 'self'; script-src 'none';
  object-src 'none'` on every response.
- `internal/reader/content/adversarial_test.go`: `<script>`, event-handler
  attrs, inline `<svg>`, `style` attr, `javascript:` URI in href/src/CSS
  `url()`, absolute external URL, `url(javascript:...)`, zip-bomb entry,
  zip-slip `*path`, zip-slip reference inside content, log-redaction
  assertion. All RED first.

### Tier 3 — `internal/reader/api` (reading API transport)

- `internal/reader/api` handlers:
  - `GET /api/v1/reading/works/{workId}/progress` → `{progress|null}`
  - `POST /api/v1/reading/works/{workId}/progress` → construct
    `ProgressReport`, transactional `SELECT ... FOR UPDATE` on the
    singleton row, `ReconcileProgress`, persist, return canonical
    *(conditional on item A; otherwise this endpoint is deferred)*
  - `GET/POST /api/v1/reading/editions/{editionId}/bookmarks`,
    `DELETE /api/v1/reading/bookmarks/{bookmarkId}`
  - `GET/POST /api/v1/reading/editions/{editionId}/highlights`,
    `PATCH/DELETE /api/v1/reading/highlights/{highlightId}`
  - `GET/PUT /api/v1/reading/preferences` (scoped by `X-Device-Id`)
  - `GET /api/v1/reading/export` (+ `?workId=`) — versioned JSON,
    `Content-Disposition: attachment`, read-only, counts-only logging
    (`reading-data-export.md`)
- `X-Device-Id` UUID-v4 shape check on progress + preferences endpoints
  only (`400` otherwise); **not** on bookmarks/highlights.
- CFI shallow structural check (`internal/reader/api/cfi.go`): begins
  `epubcfi(`, ends `)`, balanced brackets/parens, charset allowlist,
  length cap. Reused for bookmark position, highlight start/end,
  `precisePosition.cfi`. `endCfi` not sorting before `startCfi`.
- `percentage` `[0.0,1.0]`; `precisePosition.editionId` belongs to
  `:workId`'s Work.
- Log-redaction: no title / CFI / note / label / percentage in any log
  line — targeted test across every endpoint.

### Tier 4 — OpenAPI contract + server wiring

- Add all paths to `api/openapi.yaml`.
- `internal/testutil/contracttest` extended — route-completeness +
  request/response validation.
- Wire routes in `cmd/server/main.go`; construct the content cache once
  at startup and inject (`cmd/server/repositories.go` / `run.go`).

### Tier 5 — Frontend reader (`web/`)

- `npm run mocks:gen-fixtures` from the extended OpenAPI.
- `web/src/data/reading.ts` — TanStack Query hooks (progress, bookmarks,
  highlights, preferences), `X-Device-Id` from `localStorage`
  (`crypto.randomUUID()` on first use).
- `web/src/screens/Reader/` — route `/read/:workId/:editionId`
  (`frontend-library-screens.md` FR-5's "Read" action already carries
  both IDs):
  - `<iframe sandbox="allow-same-origin">` — **never** `allow-scripts`,
    asserted by a test under every code path incl. theme changes.
  - foliate-js renderer pointed at
    `/api/v1/library/editions/:editionId/reader/content/*path` directly,
    no `blob:`.
  - Chrome per `atReader`: top bar (← Library, title·chapter, tools),
    tap-content toggles chrome, bottom progress bar.
  - Contents panel (left), Marks panel (right), Reading/Aa panel — theme
    light/sepia/dark (`RTHEMES` hex values), type-size stepper,
    line-spacing, layout toggle (paginated default), column width
    (pending item B).
  - Debounced writes: preferences 500 ms, position report 3 s + unload
    best-effort.
  - Position restore: open at last CFI, `Percentage` fallback.
  - Bookmark-this-page + selection→highlight with note + category swatch.
  - Marks panel "Export" button → `GET /api/v1/reading/export` → `Blob`
    + `<a download>` (web/LAN only, no Electron). Accessible name, in
    focus order.
- `web/src/screens/Reader/Reader.test.tsx` — Vitest unit/component +
  `axe` a11y + responsive (desktop + narrow LAN width). Sandbox-attr and
  script-inert fixtures RED first.
- Wire the "Read" entry point / route into the shell + router.

### Tier 6 — Full verification suite

Backend: `go test -race ./...`; integration with `TEST_DATABASE_URL`;
`golangci-lint run ./...`; `govulncheck ./...`.
Frontend: `npm --prefix web test`; `lint`; `build`;
`check:token-styling`; `check:a11y-tabindex`.
Boundary scripts: `check-import-boundaries.sh`,
`check-parameterized-queries.sh`, `check-compose-published-port.sh`.

### Tier 7 — Security audit `0011` + Gate 2

- `.claude/audits/0011-phase11-reader.md` — Four-Attacker + STRIDE over
  each trust boundary (client `*path`, EPUB bytes, EPUB content →
  browser, `X-Device-Id`, highlight/bookmark strings, CFI strings, the
  content cache). Same shape as `0010`.
- Present findings for **Gate 2 sign-off**. No PR before sign-off.
- On sign-off: specs → `IMPLEMENTED`; roadmap exit criteria walked;
  `domain-reading.md` re-confirmation recorded if item A amended it.

### Tier 8 — PR + CI

- Atomic conventional commits throughout (no `Co-Authored-By` / AI
  trailers). Push `feat/phase11-reader`.
- PR vs `main`: summary, test matrix, verification proof, audit link.
- `gh pr checks --watch` until `CI/Backend`, `CI/Frontend`, `CI/Desktop`,
  `CI/Contracts`, `CI/Repo` all green.

---

## 6. Gate 1 questions — resolved

**A → A1.** Amend `domain-reading.md` FR-6/FR-7 to review 0048's
`(epoch, percentage)` model; realign `backend-reading-api.md`. Done in
the amendment package; full phase (incl. position save/restore) proceeds.

**B → keep both.** `ReadingPreferences` carries `lineSpacing` **and**
`columnWidth` (JSONB, no migration cost). `frontend-reader.md` FR-4
amended.

**C → follow the spec.** Both paginated + continuous-scroll,
paginated default; the canvas's single-scroll depiction is one toggle
state. `frontend-reader.md` FR-2 amended.

**D → in scope, as a new sibling spec.** `reading-data-export.md`:
read-only `GET /api/v1/reading/export`, versioned JSON, web download.
Import, Electron-native save, and preferences export explicitly deferred.
Migration `00008` adds `created_at` to `bookmarks`/`highlights`.

**E → corrected.** `frontend-reader.md` Open questions now point at the
binding `atReader` canvas and the 2026-08-17 classification.

## 7. Superseded — original Gate 1 questions

**A. `ReconcileProgress` / `domain-reading.md` FR-6-7 amendment (blocker).**
The progress-report write path can't be built against the current spec
text. Which:

- **A1 (recommended):** I amend `domain-reading.md` FR-2/FR-6/FR-7 and
  realign `backend-reading-api.md` FR-2 to review `0048`'s already-decided
  `(epoch, percentage)` model — server-assigned epoch, observed-epoch
  precondition, third `Rejected` outcome — present the amendment diff for
  your approval, then implement the full phase including position
  save/restore. Adds spec work; keeps the phase whole (position
  save/restore is a roadmap exit criterion).
- **A2:** Defer the `POST .../progress` endpoint and `ReconcileProgress`
  to a follow-up phase-11 task once phase 02's own amendment lands.
  This session ships content serving + bookmarks + highlights +
  preferences + `GET progress` + the full reader UI with position
  *reporting* wired but the reconcile/persist step stubbed. Phase 11
  can't close (exit criterion "position reliably saved and restored")
  until the follow-up.
- **A3:** Implement `ReconcileProgress` now to review `0048`'s spec, record
  it as an ADR, and flag the `domain-reading.md` text amendment as an
  immediate fast-follow rather than a Gate 1 pre-condition. Faster; softer
  on gate discipline.

**B. `atReader` column-width vs spec line-spacing.** The binding canvas has
a column-width control and no line-spacing one; the spec is the reverse.
Proposed: keep **both** in `ReadingPreferences` (`lineSpacing` +
`columnWidth`), since the `JSONB` column costs nothing and both are real
reading affordances — the spec's FR-4 explicitly says phase 11 fixes the
field set. Confirm, or drop one.

**C. `atReader` shows no paginated/scroll toggle; spec FR-2 requires
both.** Proposed: follow the spec (build both, paginated default), treat
the canvas's single-scroll depiction as one captured state of the toggle,
not a scope cut. Confirm.

**D. Marks-panel "Export" action in the canvas.** No spec FR. Proposed:
out of scope this phase, tracked as a follow-up — not built, not removed
from the design record. Confirm.

**E. Stale `frontend-reader.md` Open-questions line** ("no design-reference
screen for the reader"). Proposed: correct it to point at `atReader` +
the 2026-08-17 classification as part of this phase's doc updates.
Confirm.
