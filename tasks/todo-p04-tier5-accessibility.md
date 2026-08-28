# Phase 04, Tier 5 — Accessibility — task list

Full plan: [`tasks/plan-p04-tier5-accessibility.md`](plan-p04-tier5-accessibility.md).
Each task: RED → GREEN → Refactor, one commit. Stop at Checkpoint P4-F.

## Decisions (resolve once)

- [ ] D1 — keyboard map at `web/docs/keyboard-map.md` (co-located, versioned with code)
- [ ] D2 — grep checks follow the `scripts/checks/` pattern (find-fn + entry + fixture test + CI step)
- [ ] D3 — real-browser axe renders primitives on one `e2e/a11y-gallery/` harness page; `color-contrast` stays disabled there (documented `text-3` reason)

## Maintainer decisions (resolved 2026-08-27, plan §Gaps)

- [x] G1 — implement `prefers-contrast` in T5 (`src/a11y.css` — `--color-text-3 → text-2` override, also mitigates the `text-3` AA exception under high-contrast; + Spinner/Skeleton/ProgressBar subtle-decoration bumps)
- [x] G2 — FR-5 contrast proof = CSS-parse test only; reduced-motion half gets a real `emulateMedia` assertion; no `forced-colors` work this tier
- [x] G3 — continue Tier 0–4 convention (tier plan doc carries test strategy); note the phase-04 formal-test-plan backfill as a Tier 6 / F27 item

## Tasks

- [x] T1 — no-positive-`tabindex` grep check + CI (RED: `tabIndex={2}` fixture flagged; real tree clean)
- [x] T2 — `<VisuallyHidden>` convention grep check (`display:none` / clip-rect on text) + CI + fix any real hit
- [x] T3 — `web/docs/keyboard-map.md` + `keyboardMap.test.tsx` per category (Tab/Shift+Tab, Enter/Space, Arrows-roving, Escape) — keyboard-only assertions, fix any deviating primitive
- [x] T4 — real-browser `@axe-core/playwright` over an `e2e/a11y-gallery/` page (every primitive, representative states); fold in the shell scan; CI note distinct from `jsx-a11y`
- [x] T5 — `prefers-contrast` (gated on G1): `src/a11y.css` `@media (prefers-contrast: more)` — `--color-text-3 → text-2`, Spinner/Skeleton/ProgressBar decoration; imported from `main.tsx`
- [x] T6 — composed E2E: `emulateMedia({ reducedMotion })` → no animations on a shell view with Spinner+Skeleton; CSS-parse test for the `@media (prefers-contrast: more)` block (+ `forcedColors` if G2)

## Checkpoint P4-F

- [x] Zero `axe-core` violations across every primitive and shell layout
- [x] Keyboard map documented and matches actual behaviour, proven per category
- [x] No positive `tabindex` anywhere, grep-checked
- [x] Every visually-hidden label uses `<VisuallyHidden>` / `sr-only`, grep-checked
- [x] Reduced-motion / high-contrast composed check passes

Then: full regression → `/code-review high` fork → PR (`/make-pr` conventions) → `gh pr checks --watch`.

## Carried forward (not this tier's to resolve)

- `text-3` base-case WCAG AA contrast — T5 mitigates under `prefers-contrast`; default palette value stays a maintainer decision
- `atTablet` placement — Claude Design project owner
- FR-4 numeric budgets — phase 06
- Global keyboard shortcuts — deferred by spec FR-1 to phase 06+
- Phase-04 formal test plans — reconcile at Tier 6 / F27
