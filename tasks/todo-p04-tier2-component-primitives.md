# Phase 04, Tier 2 — Component primitives — task list

Full plan: [`tasks/plan-p04-tier2-component-primitives.md`](plan-p04-tier2-component-primitives.md).
Each task: RED → GREEN → Refactor. Stop at Checkpoint P4-C.

## Decisions (resolve once)

- [ ] P1 — Radix package names: `@radix-ui/react-{dialog,switch,radio-group,slider,toast,visually-hidden}`
- [ ] P2 — no `clsx`/`cva`; hand-written `cx()` helper + plain variant lookups
- [ ] P3 — `axe-core` + hand-written jsdom test helper now; `@axe-core/playwright` real-browser stage stays Tier 5's (F24)

## Tasks

- [ ] T1 — Infra: `cx()` helper, `axe.ts` Vitest helper (RED: catches a deliberately unlabelled `<input>` fixture), Radix packages installed
- [ ] T2 — `VisuallyHidden` (Radix-wrapped)
- [ ] T3 — `Button`
- [ ] T4 — `Input`
- [ ] T5 — `ProgressBar`
- [ ] T6 — `StatusPill`
- [ ] T7 — `FormatBadge`
- [ ] T8 — `Chip`
- [ ] T9 — `Spinner`
- [ ] T10 — `Skeleton`
- [ ] T11 — `StatCard`
- [ ] T12 — `Toggle` (Radix `Switch`)
- [ ] T13 — `SegmentedControl` (Radix `RadioGroup`)
- [ ] T14 — `Slider` (Radix `Slider`)
- [ ] T15 — `Toast` (Radix `Toast`)
- [ ] T16 — `Modal` (Radix `Dialog`)
- [ ] T17 — `EmptyState` (depends on T3, T6)
- [ ] T18 — `DataTable` (render → sort → select sub-passes)

**Checkpoint P4-C** — 17/17 primitives classified per FR-1, zero `axe-core`
violations, fully keyboard-operable (no pointer events in operability
tests), zero raw hex/px outside tokens (grep-checked sweep), Storybook
story per primitive.
