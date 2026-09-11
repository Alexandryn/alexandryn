# Accessibility & QA audit: Phase 17 — whole-application conformance and regression sweep

| | |
|---|---|
| **Scope** | All routes listed in `.claude/roadmap/17-accessibility-and-qa/README.md` Scope/In — WCAG 2.1 AA, keyboard operability, cross-browser/viewport matrix, scale performance |
| **Auditor** | Tier 1-3 automated + manual sweep (self + `test-engineer` + `web-performance-auditor` subagents) |
| **Date** | 2026-09-11 |
| **Commit** | `b114d20` |
| **Verdict** | Findings open — 11 findings (1 Critical, 3 Medium, 3 Low, 4 Informational); **A-17-08, A-17-09, A-17-10 fixed** (`1b008ec`, `0f9ccdd`) — 0 open Critical, 0 open Medium (A-17-02 escalated, not fixed here — see its own resolution note). Reader (EPUB/PDF) and Import screen not yet examined. |

## Template deviation, stated per constitution §12

`.claude/templates/audit.md` is written for a security audit — its "Trust
boundaries examined," "Adversarial questions asked," and severity guide (RCE,
auth bypass, sandbox escape) assume an attacker. This phase's findings are
mostly accessibility and QA defects, not attacker-driven, so those sections
are repurposed below rather than left contradicting their own headings. This
is a deliberate adaptation of the existing template per `CLAUDE.md`'s "use
these; don't invent new document shapes" — not a new template — and is
recorded here rather than silently reusing security language for non-security
findings.

## Scope and method

Tier 1 (automated conformance sweep) run 2026-09-11 against commit `0c9e65e`:

- Static: `npm run lint` (clean), `npm run tokens:check-contrast` (clean —
  4 `text-3`-on-light-surface pairs fail raw contrast but are a permanent,
  documented exception per `frontend-accessibility.md` FR-4/FR-5, with a
  `prefers-contrast: more` fallback that itself passes AA; not a new
  finding), `npm run check:token-styling` (clean), `npm run check:a11y-tabindex`
  (clean), `npm run check:a11y-hidden-text` (clean).
- `npx playwright test --project=gallery` — 3/3 pass (`@axe-core/playwright`
  zero violations across every primitive, the modal dialog open state, and
  `prefers-reduced-motion`).
- `npx playwright test --project=app` — 30/30 pass, covering Library/shell,
  Discover, Sources, pairing (functional, not yet axe-audited — see below),
  hostile-input boundary cases, and the not-found view.
- `npx playwright test --project=electron` (electron/ suite) — 14/14 pass:
  preload surface isolation, IPC argument validation, boot lifecycle, window
  security flags.
- Coverage gap closed under a separate `test-engineer` pass: added
  `axe-core` coverage for `/login`, `/setup`, `/forgot-password`,
  `/reset-password`, `/activity`, `/more`, `/settings`, `/settings/devices`,
  `/settings/network`, and the `DevicePairingModal` open state.
  (`WorkDetail`/`CollectionDetail` turned out already covered in
  `library.app.spec.ts` — this audit's initial gap list was stale on that
  point.) Result: **33 passed, 5 failed** — the 5 failures are genuine
  defects (A-17-08, A-17-09), not test-authoring bugs; left red
  deliberately per the Prove-It pattern rather than weakened to pass.
  Reader (`/read/:workId/:editionId`) is a genuine, not merely
  not-yet-done, gap: the shared MSW handler set
  (`src/mocks/handlers.ts`) has no route for the reader-content endpoint
  `epubBook.ts` fetches — only `Reader.test.tsx`'s local, file-scoped
  helper mocks it. Covering it would mean adding a new shared MSW handler,
  a `src/` change outside a test-only pass's scope; recorded as a
  follow-up, not invented.
- Cross-browser matrix (G0-4): `playwright.config.ts` extended with
  `app-firefox`, `app-webkit`, `app-mobile-chrome`, `app-mobile-safari`
  projects. `app-mobile-chrome` verified green (30/30, Chromium engine, no
  extra system deps). Firefox and WebKit browser binaries are installed but
  **blocked** — the host is missing system libraries
  (`libicu74`, `libxml2`, `libflite1`); the fix (`sudo npx playwright
  install-deps`) needs the maintainer's explicit go-ahead, since it's a
  privileged, machine-wide change outside this repo's scope. `app-mobile-safari`
  is WebKit-engine and is blocked the same way. Recorded as a finding below.
- Scale/performance benchmark (Tier 3): running under a separate
  `web-performance-auditor` pass. Results pending.

## Screens/flows examined (in place of "Trust boundaries examined")

| Screen/flow | WCAG axes checked | Method | Assumption being made |
|---|---|---|---|
| Auth/Login (`/login`, `/setup`, `/forgot-password`, `/reset-password`) | Landmarks, headings, form labels, 320px reflow, touch targets, focus visibility | Automated axe (new) + manual 320px/keyboard pass | None of these screens require MSW auth-state setup (public, outside `RequireAuth`) |
| Library Catalog | Axe (existing), 320px reflow, touch targets, scroll-perf scale (10k/20k) | Automated axe (existing) + manual pass + `web-performance-auditor` benchmark | Benchmark used dev-mode Vite, not a production build |
| Book Detail (`WorkDetail`) | Functional coverage; axe coverage confirmed pre-existing | Automated (`library.app.spec.ts`, pre-existing) | Not independently re-verified this session beyond confirming the test exists and passes |
| Collections (`CollectionDetail`) | Same as Book Detail | Automated (`library.app.spec.ts`, pre-existing) | Same as Book Detail |
| Reader (EPUB) | **Not examined** — see "What was not examined" | — | — |
| Reader (PDF) | **Not examined** — see "What was not examined" | — | — |
| Sources | Axe (existing), 320px reflow, touch targets | Automated axe (existing) + manual pass | — |
| Devices/Pairing (`/settings/network`, `/settings/devices`, `DevicePairingModal`) | Landmarks, headings, 320px reflow, touch targets, modal focus | Automated axe (new) + manual pass | `/settings/devices`'s error state (mocked 404, no shared MSW handler) is the state audited, not a populated device list |
| Settings (index) | Axe (new), 320px reflow, touch targets | Automated axe (new) + manual pass | — |
| More/Activity | Axe (new), 320px reflow, touch targets | Automated axe (new) + manual pass | `/activity`'s error state (mocked 404, no shared MSW handler) is the state audited |

## Conformance questions asked (in place of "Adversarial questions asked")

Work through these explicitly for every screen/flow above, per the phase
README's G0-2/G0-4 and constitution §7.

- Can every interactive control be reached and operated with `Tab`,
  `Shift+Tab`, `Enter`, and `Space` alone — no mouse, no positive `tabindex`?
- Is focus visible at every step, with a ring that meets contrast on every
  background it appears on (light, dark, high-contrast)?
- Does every dialog/modal trap focus while open and return it to the
  triggering element on dismissal? Does `Escape` close it?
- Does every input have a programmatically associated label, and does a
  validation error announce via `role="alert"` or `aria-live="assertive"`?
- Does every async event (import job, sync status, background indexing)
  announce via `aria-live="polite"` without stealing focus?
- Does the screen hold up at 320px with no horizontal scroll and no lost
  functionality, with ≥44×44px touch targets?
- Does `prefers-reduced-motion: reduce` neutralize every animation on this
  screen?
- Does the screen render and behave equivalently in Chromium, Firefox,
  WebKit, and the two mobile viewports?
- At 10,000+ catalog items: does the list stay virtualized, hold ≥55 FPS
  scroll, and avoid a DOM-node/memory leak across repeated navigation?

## Findings

| ID | Severity | Title | Issue | Status |
|---|---|---|---|---|
| A-17-01 | Informational | `electron/src/renderer/boot/tokens.css` can silently drift from its generator source; no CI check | #314 | Open |
| A-17-02 | Medium | `WorkGrid` virtualizes only the cover image; wrapper-node mount cost scales linearly with catalog size | #315 | Open |
| A-17-03 | Informational | Scroll frame rate holds ~60fps through 20,000 items — confirmed clean, worth a regression-watch benchmark | #321 | Open |
| A-17-04 | Low | List view never gets the grid view's `content-visibility: auto` treatment — unmeasured, plausible gap | #318 | Open |
| A-17-05 | Informational | `seedCache` module-level `Map` is unbounded — trivial at tested scale | #322 | Open |
| A-17-06 | Informational | Reader shows no memory-leak pattern across pagination/open-close (static analysis only, not live-measured) | #323 | Open |
| A-17-07 | Low | Firefox/WebKit Playwright projects blocked in this dev sandbox by missing host libraries | #319 | Open |
| A-17-08 | **Critical** | `theme.css`'s generated `--spacing-*` scale collides with Tailwind's `max-w-*` key names, collapsing `max-w-md`/`max-w-3xl`/etc. to single-digit pixel widths app-wide | #324 | **Fixed** (`1b008ec`) |
| A-17-09 | Medium | Missing `<main>` landmark on all four public auth screens; `/settings/devices` has no `<h1>` | #316 | **Fixed** (`1b008ec`) |
| A-17-10 | Medium | Numerous interactive controls fall short of the project's own 44×44px touch-target minimum at mobile width, across nearly every screen | #317 | **Fixed** (`0f9ccdd`) |
| A-17-11 | Low | `LoginScreen`/`SetupScreen` hand-roll raw `<input>`s instead of the shared `Input` component, with a border-color-only focus indicator not verified against contrast requirements | #320 | Open |

### A-17-01 — `electron/src/renderer/boot/tokens.css` can silently drift from its generator source; no CI check

**Severity:** Informational

**Component:** `electron/electron.vite.config.ts`'s `syncTokensPlugin` (copies `web/src/tokens.css` → `electron/src/renderer/boot/tokens.css` at build time); `.github/workflows/ci.yml`'s generated-file drift check (only covers `web/src/theme.css`, `web/src/tokens.css`, `web/src/breakpoints.ts`)

**Description** — `web/src/tokens.css` is the generated source of truth (`npm run tokens:generate`, checked by CI's `git diff --exit-code` step). `electron/src/renderer/boot/tokens.css` is a build-time *copy* of it (`copyFileSync` in `syncTokensPlugin`'s `buildStart` hook), committed separately so the Electron boot screen has a disk asset before the renderer bundle loads. Nothing checks that the committed copy matches the current generator output. Running `npm run build` in `electron/` regenerates it correctly (observed directly this session — it picked up the `FONT_INTEGRATION_NOTE` comment that's present in `web/src/tokens.css` but missing from the committed Electron copy), which means the two files are currently out of sync in the repo.

**Impact** — None at runtime: every Electron build overwrites the file fresh before bundling, so a shipped build always gets current tokens regardless of what's committed. The impact is purely a repo-hygiene / future-confusion risk — a `git diff` after any Electron build looks "dirty" for a file nobody touched, and there's no automated signal telling anyone the committed copy is stale.

**Preconditions** — Running an Electron build locally after `web/src/tokens.css` changes upstream, without also committing the resulting `electron/src/renderer/boot/tokens.css` diff.

**Reproduction** — `cd electron && npm run build`, then `git status` shows `electron/src/renderer/boot/tokens.css` modified.

**Recommendation** — Commit the regenerated file, and extend the CI drift-check step in `.github/workflows/ci.yml` to also run the Electron build (or just `copyFileSync`'s equivalent) and `git diff --exit-code` on this path, the same way it already does for the three `web/src/` generated files.

**Resolution** — Open, filed for the findings gate.

### A-17-02 — `WorkGrid` virtualizes only the cover image; wrapper-node mount cost scales linearly with catalog size

**Severity:** Medium

**Component:** `web/src/components/WorkGrid/WorkGrid.tsx:40-112,180` (mounts a wrapper `<li>`/`<div>` + `<Link>` + text for every item in `works`, unconditionally; only `GeneratedCover` is windowed via `coverFor(index)` to a sliding 40-item window); `web/src/screens/Library/Library.tsx:156-159,261` (accumulates every fetched page into one uncapped `allWorks` array)

**Description** — At the phase's own 10,000-item scale target, this measured as a ~1.1-1.2s synchronous mount before the catalog is interactive (~1.9s at 20,000). Marginal cost is ~0.07-0.09 ms/item on top of a ~380-420ms fixed baseline; cold vs. warm navigation differ by only 5-10%, so the cost is DOM-mount-bound, not fetch/parse-bound — a production build would not remove this pattern, only shift the constant. Full measurements:

| N | Cold nav | Warm reload | DOM nodes | JS heap | `/api/v1/library` payload |
|---|---|---|---|---|---|
| 500 | 460 ms | 362 ms | 3,939 | 48.1 MB | 88 KB |
| 2,000 | 595 ms | 504 ms | 14,939 | — | — |
| 10,000 | 1,175 ms | 1,109 ms | 73,606 | 179.3 MB | 1,768 KB |
| 20,000 | 1,884 ms | 1,681 ms | 146,939 | 289.9 MB | 3,546 KB |

Measured dev-mode (unminified Vite serving) via a throwaway Playwright script driving the real `/library` route; not a production-bundle measurement.

**Impact** — Degrades but doesn't block: the page still renders and becomes interactive within ~2s at double the target scale, no reflow/collapse, and heap growth (~13-15 KB/item) is ordinary linear growth, not a leak. Scroll performance after mount is unaffected (see A-17-03).

**Preconditions** — A library with several thousand or more items, scrolled through at least once (so `Library.tsx`'s accumulator has ingested every page).

**Reproduction** — See the benchmark methodology recorded by the `web-performance-auditor` pass; the script is not checked into the repo (scratch-only per this task's constraints).

**Recommendation** — Extend `WorkGrid`'s existing sliding-window tracking (`clampedStart`/`windowEnd`) to also skip mounting the wrapper element outside the window (fixed-height placeholder instead), following the fully-windowed pattern `web/e2e/benchmark/CoverGridHarness.tsx:38-44` already uses for the GeneratedCover benchmark harness. This is a real windowing change (scroll-position-driven, not just intersection-observer-sentinel-driven) and should be scoped as its own change rather than folded into a same-PR mechanical fix — per the roadmap's own risk table, which anticipated exactly this outcome and named escalation as the correct response.

**Resolution** — Open, filed for the findings gate; escalate rather than fix inline per G0-5/risk table.

### A-17-03 — Scroll frame rate holds ~60fps through 20,000 items

**Severity:** Informational

**Component:** `web/src/utilities.css:14-17` (`.cv-auto`, `content-visibility: auto` + `contain-intrinsic-size`), applied at `WorkGrid.tsx:187` (grid view only)

**Description** — Scroll-frame interval median was 16.7ms (~60fps) at every tested N (500 / 2,000 / 10,000 / 20,000) with no degradation as N grows. Isolated single-frame spikes appear (worst frame 49.9ms at N=20,000) but aren't sustained jank and don't scale monotonically with N — consistent with occasional window-slide re-renders landing on a frame, not a scaling problem. This meets the phase's ≥55fps exit criterion with margin.

**Impact** — None — confirmed-clean result, recorded so "no finding here" is an evidenced statement, not an absence of looking.

**Recommendation** — Worth a permanent regression-watch benchmark (median frame-interval assertion, the way `benchmark.spec.ts` already asserts `GeneratedCover`'s timing budget) but not a fix.

**Resolution** — Accepted — no action needed.

### A-17-04 — List view never gets the grid view's `content-visibility` treatment

**Severity:** Low

**Component:** `WorkGrid.tsx:187` (`cv-auto` class on grid-view cell) vs. `WorkGrid.tsx:108-112` (list-view `<li>`, no `cv-auto`)

**Description** — The list-view render path doesn't apply the `.cv-auto` utility its grid-view sibling uses, so list view gets none of A-17-03's paint/layout-skip benefit at scale. Not benchmarked directly — list rows are simpler DOM (no cover SVG/canvas layering) so the practical gap may be small, but this is unverified.

**Impact** — Potential only, unmeasured, one view mode.

**Recommendation** — Run the same benchmark against list view before deciding whether it needs `cv-auto` — don't assume parity either way.

**Resolution** — Open, filed for the findings gate.

### A-17-05 — `seedCache` module-level `Map` is unbounded

**Severity:** Informational

**Component:** `web/src/components/GeneratedCover/seedCache.ts:8-16`

**Description** — Caches a derived seed per work identifier for the session's lifetime (deliberate, documented in-code), cleared only on full reload. At 10,000-20,000 unique identifiers this is 10-20k small numeric-struct entries — negligible at the tested scale.

**Impact** — None at tested scale. Noted only for completeness since the Reader stress-test framing ("global/module-level caches that could accumulate") applies to this pattern generally, even though it sits in the catalog path.

**Resolution** — Accepted — no action needed at current scale.

### A-17-06 — Reader shows no leak pattern across pagination/open-close (static analysis)

**Severity:** Informational

**Component:** `web/src/screens/Reader/Reader.tsx:151-220`, `useDebouncedCallback.ts`, `epubBook.ts:40-62`, vendored `web/src/vendor/foliate/epub.js`

**Description** — Traced explicitly: iframe reuse across chapter navigation (browser discards each chapter's realm on its own), per-chapter listener cleanup (`cleanupIframeListenersRef.current?.()` runs before attaching new listeners and again on unmount), debounce timer cleared on unmount, `loadBlob` explicitly rejected so foliate's blob-URL cache is genuinely unused dead code for this app (not a live leak, since it's never populated), and the reader query cache relies on TanStack Query's default 5-minute inactive-query GC rather than an unbounded override.

**Impact** — None found via static code-path tracing. **Not live-measured** — no heap-snapshot diff was captured across repeated real open/close cycles this session; if the phase wants that as empirical exit-criteria evidence rather than code-reading, that's a follow-up.

**Resolution** — Accepted for the static-analysis question asked; live heap-growth measurement remains a stated gap (see "What was not examined").

### A-17-07 — Firefox/WebKit Playwright projects blocked in this dev sandbox

**Severity:** Low

**Component:** This development environment (missing `libicu74`, `libxml2`, `libflite1`), not the repo

**Description** — `playwright.config.ts` now has `app-firefox` and `app-webkit`/`app-mobile-safari` (WebKit engine) projects per Gate 0 G0-4. The browser binaries downloaded successfully but the sandbox is missing system libraries; `sudo npx playwright install-deps` is the fix but wasn't run without explicit maintainer approval (privileged, machine-wide change). `app-mobile-chrome` (Chromium engine) was verified green, so the config itself is correct — only two engines are blocked from running *in this specific sandbox*. `.github/workflows/ci.yml` was updated to `npx playwright install --with-deps chromium firefox webkit`, which should work cleanly on a real GitHub Actions runner (this is a sandbox-specific limitation, not expected to recur in CI).

**Impact** — Blocks local verification of the Firefox/WebKit legs of the cross-browser matrix in this environment; CI is expected to run them cleanly once this PR's workflow change lands. Rated Low rather than blocking because it's an environment gap with a known, low-risk fix, not a defect in the tests or the app.

**Recommendation** — Run `sudo npx playwright install-deps` (or `sudo apt-get install libicu74 libxml2 libflite1`) if local Firefox/WebKit verification is wanted before the CI run confirms it; otherwise this resolves itself once the updated workflow runs in CI.

**Resolution** — Open, pending either maintainer approval of the sudo install locally, or a green CI run on this branch's PR.

### A-17-08 — `theme.css`'s generated `--spacing-*` scale collides with Tailwind's `max-w-*` key names

**Severity:** Critical

**Component:** `web/src/theme.css` (generated by `scripts/generate-tokens.ts`, lines 39-48: `--spacing-xs: 8px`, `--spacing-md: 10px`, `--spacing-3xl: 20px`, etc.) — no `--container-*`/`--max-width-*` override exists in the same `@theme` block to take precedence

**Description** — Tailwind v4 resolves `max-w-{key}` utilities against a scale keyed by the same names this project's custom spacing scale reuses (`xs`, `sm`, `md`, `lg`, `xl`, `2xl`, `3xl`). With no `--container-*`/`--max-width-*` override defined, Tailwind falls back to the `--spacing-*` namespace for these keys — so `max-w-md` resolves to `--spacing-md` (10px) instead of Tailwind's intended ~28rem, and `max-w-3xl` resolves to `--spacing-3xl` (20px) instead of ~48rem. Confirmed empirically (`test-engineer` pass): a bare `<div class="max-w-md">` computes `maxWidth: 10px` in a real browser.

**Impact** — Confirmed broken on `/login`, `/setup`, `/forgot-password`, `/reset-password` (auth card collapses to ~25px wide, text wrapping letter-by-letter — screenshot captured) and `/settings/devices` (`max-w-3xl` container + its `max-w-md` confirm dialog). Grep confirms 17 files use `max-w-{xs,sm,md,lg,xl,2xl,3xl}` classes and are therefore at risk of the same collapse: `components/Modal/Modal.tsx`, `screens/Activity/Activity.tsx`, `screens/Auth/{AcceptInviteScreen,ForgotPasswordScreen,LoginScreen,MfaPromptModal,MfaSetupModal,ResetPasswordScreen,SetupScreen}.tsx`, `screens/Discover/Discover.tsx`, `screens/Import/Import.tsx`, `screens/Library/Library.tsx`, `screens/Network/{AccessScreen,ConnectScreen,DevicePairingModal}.tsx`, `screens/Settings/{DevicesSettings,NetworkSettings}.tsx`, `screens/Sources/SourceDetail.tsx`. This is rated Critical, not High, because it isn't narrow to one screen or one precondition — it's a repo-wide, every-browser, every-viewport breakage of the base `Modal` component and the primary authentication entry point, silently present since whenever this token collision was introduced (predates Phase 17; not caught earlier because jsdom-based unit tests don't compute real CSS layout, and no existing real-browser Playwright test asserted on these components' actual rendered width).

**Preconditions** — None — reproduces on every load of an affected screen, in every browser.

**Reproduction** — `npx playwright test --project=app e2e/auth-screens.app.spec.ts` (currently red — `toBeVisible()` on the auth card's labeled fields fails because the card is laid out at near-zero width); or manually, `page.goto('/login')` and read `getComputedStyle(document.querySelector('.max-w-md')).maxWidth`.

**Recommendation** — Rename the generated `--spacing-*` scale's keys in `scripts/generate-tokens.ts`/`scripts/tokens/extract.ts` so they don't collide with Tailwind's default named scales (e.g. a `--spacing-` numeric scale instead of named `xs`/`sm`/`md`/…, or prefix them, e.g. `--spacing-space-md`), **or** add an explicit `--container-*`/`--max-width-*` block to `web/src/theme.css`'s `@theme` so Tailwind's intended max-width scale takes precedence regardless of the spacing scale's key names. The former is more correct (removes the ambiguity at the source) but is a generator change requiring re-validation of every consumer of the current `--spacing-*` names; the latter is the smaller, faster fix. This needs a maintainer decision on which, not a default pick, since the generator is shared with the Electron boot CSS (A-17-01) and any consumer already depending on the current `--spacing-{key}` utility class names (e.g. `p-md`, `gap-xs`) would be affected by a rename.

**Resolution** — **Fixed**, commit `1b008ece96cc8c64de83b96f323fa48fb3ee1934`. The originally-recommended `--container-*` override does **not** work — proven, not assumed: Tailwind v4 always prefers a `--spacing-{key}` theme value over `--container-{key}` on a name collision regardless of declaration order (confirmed by inspecting the compiled CSS — `.max-w-md` used `var(--spacing-md)` even with an explicit `--container-md` defined; `.max-w-4xl`, which doesn't collide, correctly used `var(--container-4xl)`). Actual fix: replaced every `max-w-{xs,sm,md,lg,xl,2xl,3xl}` class across the 17 affected files with Tailwind arbitrary-value syntax (`max-w-[28rem]`, etc.), bypassing the named-scale lookup entirely — no token rename, no risk to other `--spacing-{key}` consumers. Verified: full `app` project 38/38, `gallery` 3/3, vitest 539/539, lint and `check:token-styling` clean.

### A-17-09 — Missing `<main>` landmark on public auth screens; `/settings/devices` has no `<h1>` — **Fixed**

**Severity:** Medium

**Component:** `web/src/screens/Auth/{LoginScreen,SetupScreen,ForgotPasswordScreen,ResetPasswordScreen}.tsx` (no `<main>`/`role="main"` wrapping the centered card); `web/src/screens/Settings/DevicesSettings.tsx` (only an `<h2>Devices</h2>`, no route-level `<h1>` the way other screens get from shell chrome)

**Description** — `axe-core` `landmark-one-main` (moderate) fires on all four auth screens: no landmark region contains the sign-in/setup/reset card, the field groups, or the footer link. `page-has-heading-one` (moderate) fires on `/settings/devices`: its only heading is an `<h2>`, with no `<h1>` anywhere on the page.

**Impact** — Screen-reader users lose the "jump to main content" landmark navigation on every public auth screen — the exact entry point where a user with no prior context most needs it. The missing `<h1>` on Devices means AT users navigating by heading level see no page-level heading at all for that screen.

**Preconditions** — None — present on every load, any assistive-tech context.

**Reproduction** — `npx playwright test --project=app e2e/auth-screens.app.spec.ts` and `e2e/settings.app.spec.ts` (both now green).

**Recommendation** — Wrap each auth screen's card in a `<main>` (or add `role="main"`) landmark, matching whatever pattern the authenticated shell already uses for its route content. Add a visually-consistent `<h1>` to `DevicesSettings` (can visually match the existing `<h2>` styling while being the correct semantic level, per `frontend-ui-engineering`'s native-semantics-first rule).

**Resolution** — **Fixed**, commit `1b008ece96cc8c64de83b96f323fa48fb3ee1934`. Auth screens' outer card wrapper (`LoginScreen`, `SetupScreen`, `ForgotPasswordScreen`, `ResetPasswordScreen`, plus `AcceptInviteScreen` — same pattern, not separately tested) promoted from `<div>` to `<main>`. `DevicesSettings`' heading promoted from `<h2>` to `<h1>`, visual styling unchanged.

### A-17-10 — Numerous interactive controls fall short of the 44×44px touch-target minimum at mobile width — **Fixed**

**Severity:** Medium

**Component:** Shell navigation (`web/src/components/NavList/NavList.tsx` and the tab-bar links it renders — 80×33px, height short of 44px), Library's filter radio-buttons (33×19 / 99×19), Sources' Edit/Remove row actions (44×19 / 66×19), Activity's "Retry connection" (89×15), Devices' "Retry" (47×21) — sampled via a manual 320px-viewport keyboard-tab walkthrough across `/library`, `/discover`, `/collections`, `/sources`, `/settings`, `/settings/network`, `/settings/devices`, `/activity`, `/more`, `/login`, `/setup` (11 routes; script not committed, throwaway per this session).

**Description** — The phase's own exit criteria (and `frontend-ui-engineering`'s bar) set 44×44px as the mobile touch-target minimum. Nearly every route sampled has at least one, usually several, controls short of that — predominantly on the height axis (many controls are 15-21px tall). This reads as systemic rather than per-screen: the shell's own nav links (present on every route) are 33px tall everywhere, and several screens reuse the same undersized filter-chip/row-action pattern, so a handful of shared-component fixes would resolve most instances rather than needing a per-screen patch.

**Impact** — Degrades touch usability (harder, more error-prone tapping) but doesn't block — every sampled control remained tappable, just below the ideal size. No route was unusable.

**Preconditions** — Mobile/narrow viewport (~320-480px), touch input.

**Reproduction** — Manual: load any listed route at 320px width, Tab through and inspect `getBoundingClientRect()` on each focusable element (method used this session, not committed as a test).

**Recommendation** — Fix at the shared-component level first (`NavList`'s tab-bar link height, the filter-chip/row-action button pattern used across Library/Sources/Activity/Devices) rather than per-screen; re-audit afterward to see how much this closes automatically.

**Resolution** — **Fixed**, commit `0f9ccdd`. Correction: the tab-bar component is actually `web/src/app/shell/MobileTabBar.tsx`, not `NavList.tsx` (a different, unrelated component used by `/settings` and `/more`'s index lists) — fixed at the actual component. Also fixed `SegmentedControl` (`min-h-11 min-w-11`), `Button`'s shared `SIZE.sm` (`min-h-11` — this alone fixed Devices' Retry and every other `size="sm"` consumer app-wide), Activity's "Retry connection" (was a bare `<button>` bypassing the shared `Button` component entirely, inconsistent with Devices' identical pattern — switched to `Button variant="secondary" size="sm"`), and Sources' Edit/Remove (`h-auto` override opted out of `sm` sizing — switched to `size="sm"` plus a scoped `min-w-11`, since Edit's text alone left it 38px wide even at the corrected height). Manually re-measured all five originally-sampled controls at 320px — all ≥44×44px now.

### A-17-11 — `LoginScreen`/`SetupScreen` hand-roll raw `<input>`s with an unverified focus indicator

**Severity:** Low

**Component:** `web/src/screens/Auth/LoginScreen.tsx:92,107` (and the equivalent in `SetupScreen.tsx`) — `className="... focus:outline-none focus:border-accent"` on a raw `<input>`, versus the shared `web/src/components/Input/Input.tsx` used elsewhere, whose inputs showed a detectable focus outline/box-shadow in the same manual pass.

**Description** — The manual keyboard-tab pass's detector (checks for a non-`none` CSS outline or a box-shadow) found no visible-focus signal on `/login` and `/setup`'s text inputs, while the shared `Input` component's fields elsewhere (`/library`, `/discover` search boxes) did register one. Reading the source shows why: these two screens suppress the default outline and rely on a border-color change to `accent` instead — a legitimate technique in principle, but this session did not verify its actual contrast against WCAG 2.4.7/1.4.11's focus-indicator requirements, so this is reported as an **inconsistency and an unverified claim**, not a confirmed violation. The detector's limitation (outline/box-shadow only) is stated plainly rather than overclaiming a defect from a script gap.

**Impact** — Unverified. If the border-color change doesn't meet contrast requirements, this is a real Focus Visible failure on the auth entry point specifically; if it does, this is purely a consistency/reuse issue (bypassing the shared `Input` component `frontend-ui-engineering` would otherwise steer toward).

**Preconditions** — Keyboard navigation on `/login` or `/setup`.

**Reproduction** — Tab to either input field on `/login`; compare against tabbing to the search field on `/library`.

**Recommendation** — Switch `LoginScreen`/`SetupScreen` to the shared `Input` component (removes the inconsistency regardless of the contrast question), or if there's a reason these screens can't use it, verify the border-color focus indicator's contrast ratio explicitly and record that verification.

**Resolution** — Open, filed for the findings gate.

---

## Severity guide (adapted for accessibility/QA, not security)

Rate the actual impact on a real user of this system, not the textbook worst
case for the class of issue. Inflated ratings train people to ignore ratings
(constitution §10, applied here to a11y/QA findings the same way).

| | |
|---|---|
| **Critical** | A core flow (auth, catalog browse, opening a book) is entirely unusable by keyboard or assistive-tech users, with no workaround |
| **High** | A WCAG 2.1 Level A failure that blocks a flow for some users (missing label on a required control, a real keyboard/focus trap, a modal that doesn't return focus), or a QA defect that crashes/hangs a browser target in the matrix |
| **Medium** | A WCAG 2.1 Level AA failure that degrades but doesn't block a flow (contrast shortfall, non-live async announcement, awkward-but-possible keyboard path), or a performance regression below budget but not user-blocking |
| **Low** | Narrow impact, high preconditions, or a defence-in-depth/robustness gap (e.g., works today but relies on an implicit DOM order) |
| **Informational** | No user impact today, but makes a future regression more likely, or a coverage gap (untested browser/viewport combination) |

## What was not examined

Honest gaps in coverage. Recorded as they happened, not reconstructed at the
end.

- **Production-bundle performance measurement.** The Tier 3 catalog benchmark
  (A-17-02/A-17-03) ran against the dev-mode Vite server, not
  `vite build && vite preview`. The linear-in-N DOM-mount pattern is
  structural (React committing N nodes), so it's expected to hold in
  production too, but the absolute millisecond numbers would shift and
  weren't re-measured against a production build.
- **List-view scroll performance.** Only grid view was benchmarked (A-17-04);
  list view's lack of `content-visibility: auto` is a plausible but
  unmeasured gap.
- **Reader live heap-growth measurement.** A-17-06's "no leak found" is
  static code-path tracing, not a DevTools heap-snapshot diff across
  repeated real open/close cycles.
- **Firefox/WebKit local execution.** Blocked in this sandbox (A-17-07);
  expected to run cleanly in CI once the updated workflow lands, but not
  independently confirmed by a local run as of this writing.
- **Reader (`/read/:workId/:editionId`) automated axe coverage** — genuinely
  blocked, not merely deferred: no shared MSW handler exists for the
  reader-content endpoint. Adding one is a `src/` (well, `src/mocks/`)
  change outside a test-only pass's scope; needs its own small task before
  this route can get automated coverage.
- **Manual keyboard-only walkthrough and 320px reflow/touch-target audit** —
  performed across 11 routes (`/library`, `/discover`, `/collections`,
  `/sources`, `/settings`, `/settings/network`, `/settings/devices`,
  `/activity`, `/more`, `/login`, `/setup`) via a throwaway script measuring
  horizontal-scroll presence, tab-stop reachability, computed focus-indicator
  presence, and touch-target size. Found A-17-10 and A-17-11. **Not**
  covered by this pass: Book Detail, Collection Detail, Import, Reader (see
  below), and any modal/dialog's internal tab order beyond
  `DevicePairingModal` (already axe-audited) and `Sources`'s remove-confirm
  dialog (functionally tested pre-existing, not re-walked manually this
  session).
- **Screen-reader semantics review** was performed structurally (ARIA
  roles/labels/landmarks via axe, confirmed above) but **not** with an
  actual screen reader (NVDA/VoiceOver/JAWS) reading the announced content
  aloud — axe's ARIA-correctness checks are a proxy for, not equivalent to,
  hearing the real announcement, especially for live-region timing and
  reading-order edge cases axe can't evaluate structurally.
- **Reader (EPUB & PDF)** — no automated coverage exists (genuine MSW gap,
  not merely undone — see Scope and method) and no manual walkthrough was
  performed this session. This is the single largest coverage gap in this
  audit and should be prioritized before the phase closes, given the Reader
  is a primary user-facing surface explicitly named in Scope.
- **Import screen** — not examined by either the automated or manual passes
  this session; no stated reason beyond time, recorded honestly rather than
  silently.
