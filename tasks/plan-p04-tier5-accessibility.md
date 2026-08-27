# Phase 04, Tier 5 — Accessibility — implementation plan

Full phase context: [`tasks/plan-phase04.md`](plan-phase04.md). Task list:
[`tasks/todo-p04-tier5-accessibility.md`](todo-p04-tier5-accessibility.md).
Spec: [`frontend-accessibility.md`](../.claude/specs/frontend-accessibility.md)
(`APPROVED`).

## Context

Tier 0–4 are merged to `main` (PR #59–#63). Tier 5 fixes the
cross-cutting accessibility rules the rest of phase 04 already assumes:
a project-wide keyboard map, a focus-order rule, the screen-reader-text
convention, a real-browser `axe-core` stage, and a composed
reduced-motion / high-contrast proof.

Much of the substance already exists from Tiers 2–4 and passes:

- No positive `tabindex` anywhere (`grep` — zero hits today).
- Every primitive has a jsdom `axe` test (Tier 2, `runAxe`).
- The shell has a real-browser `@axe-core/playwright` pass (Tier 4,
  `e2e/a11y.app.spec.ts`).
- `<VisuallyHidden>` (Radix-wrapped) exists and is used; the skip link
  uses Tailwind's own `sr-only`.
- Six primitives use `motion-reduce:` for their animations
  (`Button`, `Modal`, `Toggle`, `Toast`, `Spinner`, `Skeleton`,
  `ProgressBar`).
- Radix supplies roving-tabindex / `Escape` for `Modal`, `Toast`,
  `SegmentedControl`, `Slider`, `Toggle`; `DataTable` has a hand-rolled
  roving pattern.

Tier 5 is therefore mostly *proving and enforcing* what's there, plus
two genuine gaps (below).

### Design-conformance check (2026-08-27)

`DesignSync list_files` against project `78075626-…`: the same four
`.dc.html` canvases, no new files; the byte-identical diff performed
earlier today (Tier 4 plan) still holds. **No captured screen governs
keyboard behaviour, focus order, screen-reader text, or motion/contrast
preferences** — `ANALYSIS.md` classifies none of this, and it isn't
screen content: it's cross-cutting behaviour the spec fixes directly
(constitution §7, `architecture-frontend.md` FR-5). The canvases'
`@keyframes` (`shim`/`spin`/`pulse`/`fu`) carry no
`@media (prefers-reduced-motion)` rule — reduced-motion is an
implementation addition, already made per-primitive in Tier 2. No
conflict, no stop-and-ask on the design boundary.

### Gaps found during planning — decisions (resolved 2026-08-27)

1. **`prefers-contrast`** — implemented in T5 (`src/a11y.css`).
2. **FR-5 contrast proof** — CSS-parse test only; reduced-motion half via
   real `emulateMedia`; no `forced-colors` this tier.
3. **Test-plan cadence** — continue Tier 0–4 convention; backfill flagged
   for Tier 6 / F27.

Full reasoning, as flagged:

### Gaps found during planning

1. **`prefers-contrast` is unimplemented.** Zero occurrences of
   `contrast-more:` / `@media (prefers-contrast)` / `forced-colors` in
   `web/src`. `frontend-component-primitives.md` FR-5 and its Acceptance
   criteria require it ("any primitive with a subtle-contrast decorative
   element … swaps to a higher-contrast variant"), and this spec's FR-5
   wants the composed proof. Tier 2's Checkpoint P4-C covered only the
   `motion-reduce` half. Proposed resolution (T5): a hand-authored
   `web/src/a11y.css` with `@media (prefers-contrast: more)` overrides —
   the highest-value one being `--color-text-3 → var(--color-text-2)`,
   which is AA-compliant on every surface, so it **also gives the
   standing `text-3` contrast exception a real mitigation** (a
   high-contrast user gets a passing ratio) rather than leaving it a
   flat exception. Plus contrast bumps for the subtle decorative
   borders/shimmer on `Spinner`/`Skeleton`/`ProgressBar`. **Decision
   needed:** accept this scope, or treat `prefers-contrast` as a Tier 2
   defect to backfill separately.

2. **`@playwright/test` cannot emulate `prefers-contrast`.**
   `page.emulateMedia()` supports `reducedMotion`, `forcedColors`,
   `colorScheme` — not `contrast`. So FR-5's "high-contrast variant
   renders" can't be asserted the way reduced-motion can. Proposed
   resolution (T6): assert the `@media (prefers-contrast: more)` rules
   **ship and are well-formed** via a CSS-parse unit test, and prove the
   reduced-motion half with real `emulateMedia({ reducedMotion })` in
   the E2E. **Decision needed:** is that split acceptable, or should
   Tier 5 also add `forced-colors: active` handling (Windows High
   Contrast Mode — a broader, separate mechanism the spec doesn't name)
   so there's a real-browser assertion for a contrast adaptation?

3. **No `.claude/test-plans/frontend-accessibility.md`.** ADR 0016
   (Proposed) says a spec's test plan is written immediately before its
   phase's RED step. Phase 04 Tiers 0–4 were built with the per-tier
   `tasks/plan-*.md` carrying test strategy instead, and no phase-04
   spec has a formal test plan. **Decision needed:** author the formal
   test plan for this spec now (ADR-0016-compliant), or continue the
   Tier 0–4 convention and note the phase-04 test-plan backfill as a
   Tier 6 / F27 documentation item. Recommend the latter for
   consistency; flag it at closure.

## Decisions

- **D1 — keyboard map lives at `web/docs/keyboard-map.md`**, co-located
  with the code it documents and versioned with it; the F22 test asserts
  behaviour against what it states, so the two can't drift silently.
- **D2 — grep-style checks follow the existing `scripts/checks/`
  pattern** (`tokenStyling.ts` / `mswExclusion.ts` shape): a pure
  `find*` function + a thin `check-*.ts` entry + a `.test.ts` with a
  deliberately-failing fixture, wired as its own `npm run check:*` and
  CI step.
- **D3 — the real-browser `axe` stage renders primitives on one gallery
  harness page** (`e2e/a11y-gallery/`, the same shape as the Tier 3
  benchmark harness), scanned once by `AxeBuilder`. Kept out of the
  app's own entry point. `color-contrast` stays disabled there for the
  documented `text-3` reason, exactly as Tier 4 set it — the CSS-parse
  test (T6) and `tokens:check-contrast` are what cover contrast.
- **D4 — test-plan cadence:** per D2 above's finding 3 — carried to the
  maintainer.

## Dependency graph

```
T1 no-positive-tabindex check ──────────────┐
T2 hidden-text (<VisuallyHidden>) check ─────┤ (independent grep checks)
T3 keyboard map doc + per-category tests ────┤
                                             ├──► T7 Checkpoint P4-F
T4 real-browser axe: primitives gallery ─────┤
T5 prefers-contrast implementation ──────────┤
      └── T6 composed reduced-motion / contrast E2E + CSS-parse test ──┘
```

T1–T4 are independent and can land in any order. T6 depends on T5.

## Task list

Each task: RED → GREEN → Refactor, one commit. Stop at Checkpoint P4-F.

1. **T1 — no-positive-`tabindex` check (FR-2).**
   `web/scripts/checks/positiveTabindex.ts` + `check-positive-tabindex.ts`
   + `.test.ts`. Scans `src/` (and `e2e/`) for `tabIndex={<n>}` /
   `tabindex="<n>"` with n ≥ 1 (a bare `tabIndex={-1}` / `{0}` is fine).
   *RED:* a fixture string with `tabIndex={2}` is flagged; the real tree
   is clean. Wire `npm run check:a11y-tabindex` into CI.
   *Acceptance:* no positive `tabindex` in the codebase, proven by a
   grep-based check (spec Acceptance criterion 3). *Files:* 3 in
   `scripts/checks/` + `package.json` + `ci.yml`. *Scope:* S.

2. **T2 — `<VisuallyHidden>` convention check (FR-3).**
   `web/scripts/checks/hiddenText.ts` — flags `display:none` /
   `display: none` and hand-rolled clip-rect patterns (`clip: rect(`,
   a `w-px h-px overflow-hidden` cluster, `position:absolute` +
   `overflow:hidden` + a 1px size) that wrap **text content**, anywhere
   a component should have used `<VisuallyHidden>` or Tailwind `sr-only`.
   Allow-list: the `<VisuallyHidden>` primitive itself, Tailwind's
   `sr-only`/`not-sr-only` utilities. *RED:* a fixture with
   `style={{ display: 'none' }}` around a label; audit the real tree and
   fix any hit (none expected). Wire `npm run check:a11y-hidden-text`
   into CI. *Acceptance:* spec Acceptance criterion 4. *Files:* 3 in
   `scripts/checks/` + `package.json` + `ci.yml` (+ any fix). *Scope:* M.

3. **T3 — keyboard map doc + per-category tests (FR-1).**
   `web/docs/keyboard-map.md`: Tab / Shift+Tab (DOM order), Enter / Space
   (activate; Space scrolls a scroll region, never overridden), Arrow
   keys (roving within a composite — Radix for `SegmentedControl` /
   `Slider` / `Toggle`, hand-rolled for `DataTable`; never page-level),
   Escape (closes the topmost transient — `Modal` / `Toast`), and the
   explicit "no global single-key shortcut this phase" note.
   `web/src/test/keyboardMap.test.tsx` exercises one representative
   primitive per category against the documented behaviour
   (`user.tab()` / `user.keyboard()` only, never a pointer event —
   Tier 2's discipline). *RED:* the tests assert the doc; any primitive
   that deviates is fixed to match (most already conform via Radix).
   *Acceptance:* spec Acceptance criterion 2. *Files:* `docs/keyboard-map.md`,
   `src/test/keyboardMap.test.tsx`, + any primitive fix. *Scope:* M–L.

4. **T4 — real-browser `axe` over every primitive (FR-4).**
   `web/e2e/a11y-gallery/` (harness page + `index.html` + a minimal
   vite config, mirroring `e2e/benchmark/`) rendering every primitive in
   representative states (default / hover-N/A / focus via script /
   disabled / error). `web/e2e/a11y-gallery.app.spec.ts` runs one
   `AxeBuilder` scan over the page (`color-contrast` disabled, D3).
   Fold the existing shell scan (`e2e/a11y.app.spec.ts`) into the same
   FR-4 narrative; add a CI comment making the "distinct from Tier 0's
   `jsx-a11y`" split explicit, and cross-reference it from
   `frontend-tooling.md` if that's the cleaner home for the note.
   *RED:* a deliberately broken primitive fixture (e.g. an unlabelled
   icon button) fails the scan, then is removed. *Acceptance:* spec
   Acceptance criterion 1 (zero violations across every primitive and
   shell layout). *Files:* `e2e/a11y-gallery/**`, `e2e/a11y-gallery.app.spec.ts`,
   `playwright.config.ts` (a11y-gallery under the `app` project or its
   own), maybe `ci.yml` comment. *Scope:* M.

5. **T5 — `prefers-contrast` support (FR-5, part 1).** *(Gated on
   gap-1's decision.)*
   `web/src/a11y.css` (hand-authored, imported from `main.tsx` after
   `theme.css`): `@media (prefers-contrast: more)` block redefining
   `--color-text-3` to the `text-2` value (AA on every surface) and
   raising the subtle decorative borders/shimmer used by `Spinner` /
   `Skeleton` / `ProgressBar`. A short comment records that this is an
   accessibility adaptation, not a design token, and why `text-3`'s
   standing AA exception is mitigated (not erased) by it.
   *RED:* a unit test parsing the file / computed styles asserting the
   override exists and resolves `text-3` to a ≥ 4.5:1 pair; before the
   file, it fails. *Files:* `src/a11y.css`, `src/main.tsx`, a test.
   *Scope:* M.

6. **T6 — composed reduced-motion / high-contrast proof (FR-5, part 2).**
   Extend `web/e2e/a11y.app.spec.ts`: with
   `page.emulateMedia({ reducedMotion: 'reduce' })`, load a shell view
   showing a `Spinner` and `Skeleton` and assert
   `element.getAnimations()` is empty / `animation-name: none` — the
   cross-cutting proof that the per-primitive `motion-reduce:` rules
   compose. For contrast (not emulatable, gap-2): a unit test parsing
   the built CSS for a well-formed `@media (prefers-contrast: more)`
   block, plus (if gap-2's decision adds it) a real
   `emulateMedia({ forcedColors: 'active' })` assertion.
   *RED:* the reduced-motion composed assertion fails against a shell
   view before it's wired (the primitives conform in isolation; the
   composed check is new). *Acceptance:* spec Acceptance criterion 5.
   *Files:* `e2e/a11y.app.spec.ts`, a CSS-parse test. *Scope:* M.

7. **Checkpoint P4-F.**

## Checkpoint P4-F (exit criteria — from `tasks/todo-phase04.md`)

- [ ] Zero `axe-core` violations across every primitive and shell layout
- [ ] Keyboard map documented and matches actual behaviour, proven per
      category
- [ ] No positive `tabindex` anywhere, grep-checked
- [ ] Every visually-hidden label uses `<VisuallyHidden>` (or `sr-only`),
      grep-checked
- [ ] Reduced-motion / high-contrast composed check passes

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| gap-1 (`prefers-contrast` scope) balloons if many primitives need bespoke high-contrast variants | Medium | Start with the one high-value CSS-variable override (`text-3 → text-2`); only add per-primitive variants where a real subtle-contrast element exists (Spinner/Skeleton/ProgressBar), not speculatively |
| gap-2 — no real-browser assertion for `prefers-contrast` leaves FR-5's contrast half proven only by a CSS-parse test | Low–Medium | Honest about the tool limit; the CSS-parse test is a real regression gate, and `forced-colors` can be added if the maintainer wants a rendered assertion |
| T3's keyboard tests surface a primitive that doesn't actually match the map | Medium | That's the point of writing the doc first and testing against it; the fix is a primitive change with its own RED, not a doc footnote |
| `text-3` AA exception still stands for the default (non-high-contrast) case | Low for this tier | T5 mitigates it under `prefers-contrast`; the base-case decision stays the maintainer's, carried forward |

## Open questions (carried forward, not resolved here)

- **`text-3` base-case WCAG AA contrast** — T5 mitigates it under
  `prefers-contrast`; whether the default palette value changes is still
  a maintainer decision.
- **`atTablet` placement** — Claude Design project owner.
- **FR-4 numeric budgets** — phase 06.
- **Global keyboard shortcuts** — deferred by the spec's own FR-1 to
  phase 06+.
- **Phase-04 formal test plans** — gap-3; reconcile at Tier 6 / F27.
