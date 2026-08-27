# Phase 04, Tier 2 — Component primitives — task list

Full plan: [`tasks/plan-p04-tier2-component-primitives.md`](plan-p04-tier2-component-primitives.md).
Each task: RED → GREEN → Refactor. Stop at Checkpoint P4-C.

## Decisions (resolve once)

- [x] P1 — Radix package names: `@radix-ui/react-{dialog,switch,radio-group,slider,toast,visually-hidden}`
- [x] P2 — no `clsx`/`cva`; hand-written `cx()` helper + plain variant lookups
- [x] P3 — `axe-core` + hand-written jsdom test helper now; `@axe-core/playwright` real-browser stage stays Tier 5's (F24)

## Tasks

- [x] T1 — Infra: `cx()` helper, `axe.ts` Vitest helper (RED: catches a deliberately unlabelled `<input>` fixture), Radix packages installed
- [x] T2 — `VisuallyHidden` (Radix-wrapped)
- [x] T3 — `Button`
- [x] T4 — `Input`
- [x] T5 — `ProgressBar`
- [x] T6 — `StatusPill`
- [x] T7 — `FormatBadge`
- [x] T8 — `Chip`
- [x] T9 — `Spinner`
- [x] T10 — `Skeleton`
- [x] T11 — `StatCard`
- [x] T12 — `Toggle` (Radix `Switch`)
- [x] T13 — `SegmentedControl` (Radix `RadioGroup`)
- [x] T14 — `Slider` (Radix `Slider`)
- [x] T15 — `Toast` (Radix `Toast`)
- [x] T16 — `Modal` (Radix `Dialog`)
- [x] T17 — `EmptyState` (depends on T3, T6)
- [x] T18 — `DataTable` (render → sort → select sub-passes)

**Checkpoint P4-C** — done. 17/17 primitives exist, classified per FR-1
(6 Radix-wrapped, 10 hand-built, 1 forced-hand-built). Zero `axe-core`
violations (per-primitive jsdom checks via `runAxe`/`expectNoAxeViolations`,
T1's helper). Fully keyboard-operable — every operability assertion uses
`user.tab()`/`user.keyboard()`, never a pointer event. Zero raw hex/px
outside the token set — `check:token-styling` (new grep-based script,
wired into CI) confirms clean. Storybook story per primitive. Full local
regression green: build, lint, 119 Vitest tests, Storybook build, format,
dist secrets/MSW checks, token contrast, `npm audit` (0 vulnerabilities).
