# Phase 04, Tier 3 — Generated cover system — task list

Full plan: [`tasks/plan-p04-tier3-generated-covers.md`](plan-p04-tier3-generated-covers.md).
Each task: RED → GREEN → Refactor. Stop at Checkpoint P4-D.

## Decisions (resolve once)

- [x] D1 — component name/location: `GeneratedCover`, `web/src/components/GeneratedCover/`
- [x] D2 — texture/spine: seeded HSL hue + small fixed set of CSS-only pattern variants, no canvas/SVG
- [x] D3 — `@playwright/test` added now, scoped to a separate `e2e/` directory, own `playwright.config.ts`

## Tasks

- [x] T1 — `fnv1a.ts` hash + `deriveSeed()` helper
- [x] T2 — `TextureLayer` + `SpineLayer`
- [x] T3 — `TitleLayer` + `AuthorLayer`
- [x] T4 — `GeneratedCover` — full composition + degradation ladder (FR-3)
- [x] T5 — Determinism proof (whole render pipeline, not just the hash)
- [x] T6 — Accessibility contract (never sole accessible name)
- [x] T7 — Memoization (session-scoped cache, spy-proven)
- [x] T8 — Playwright benchmark harness (FR-4), wired into CI

**Checkpoint P4-D** — done. Same identifier renders byte-identical
`innerHTML` across independent mounts, a full unmount+remount, and every
step of the degradation ladder (`GeneratedCover.determinism.test.tsx`).
All 3 ladder steps render without a blank box or crash — texture+spine
always present (`GeneratedCover.test.tsx`). Benchmark meets FR-4's budget
against the real harness: initial-viewport paint ≤100ms, scroll median
frame interval ≤20ms (60fps + CI-runner slack), both passing locally
(Chromium, `npx playwright test`) and wired into CI. Cover is never the
sole accessible name — RED proved the gap concretely (a wrapping `<a>` +
real `<h3>` computed "Dune Frank Herbert Dune" before the fix),
`aria-hidden="true"` on the root closes it. Storybook story added as the
spec's own named visual/manual design-review surface. Full local
regression green: build, lint, 165 Vitest tests, 2 Playwright tests,
Storybook build, format, `check:token-styling`, `npm audit` (0
vulnerabilities).
