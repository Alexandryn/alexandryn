# Phase 04, Tier 3 — Generated cover system — task list

Full plan: [`tasks/plan-p04-tier3-generated-covers.md`](plan-p04-tier3-generated-covers.md).
Each task: RED → GREEN → Refactor. Stop at Checkpoint P4-D.

## Decisions (resolve once)

- [ ] D1 — component name/location: `GeneratedCover`, `web/src/components/GeneratedCover/`
- [ ] D2 — texture/spine: seeded HSL hue + small fixed set of CSS-only pattern variants, no canvas/SVG
- [ ] D3 — `@playwright/test` added now, scoped to a separate `e2e/` directory, own `playwright.config.ts`

## Tasks

- [ ] T1 — `fnv1a.ts` hash + `deriveSeed()` helper
- [ ] T2 — `TextureLayer` + `SpineLayer`
- [ ] T3 — `TitleLayer` + `AuthorLayer`
- [ ] T4 — `GeneratedCover` — full composition + degradation ladder (FR-3)
- [ ] T5 — Determinism proof (whole render pipeline, not just the hash)
- [ ] T6 — Accessibility contract (never sole accessible name)
- [ ] T7 — Memoization (session-scoped cache, spy-proven)
- [ ] T8 — Playwright benchmark harness (FR-4), wired into CI

**Checkpoint P4-D** — pixel-identical output for same identifier across
renders/sessions, all 3 degradation steps render without a blank box or
crash, benchmark meets budget in CI, cover never the sole accessible
name for its book.
