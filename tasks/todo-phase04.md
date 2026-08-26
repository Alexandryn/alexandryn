# Phase 04 (Frontend foundation) — task list

Full plan with context/approach/rationale: [`tasks/plan-phase04.md`](plan-phase04.md).
Execute in order; each task is RED → GREEN → Refactor. Stop at every
checkpoint.

## Decisions (resolve once, don't re-derive mid-task)

- [ ] D1 — task-doc naming: fresh phase-04 docs, not appended to phase 03's
- [ ] D2 — CI shape: separate `frontend` job, `needs:` ordering + artifact hand-off to `backend`
- [ ] D3 — `web/` at repo root, `npm` as the package manager

## Tasks

**Tier 0 — Bootstrap**

- [x] F1 — `web/` scaffold: Vite+React+TS, pinned `package.json`, `tsconfig.json` (`strict`+`noUncheckedIndexedAccess`) — oxlint removed (not spec-decided), build verified (60.63 KiB gzipped)
- [ ] F2 — ESLint+Prettier+`jsx-a11y`, one shared config
- [ ] F3 — Vitest+RTL, sentinel test
- [ ] F4 — Storybook, `build-storybook` scripted
- [ ] F5 — Bundle-size check, secrets-grep, MSW-exclusion-grep
- [ ] F6 — `ci.yml`: new `frontend` job, `backend` gets `needs: frontend` + artifact hand-off

**Checkpoint P4-A** — build/lint/typecheck/Vitest/Storybook green locally and in CI; oversized-bundle, High-severity-advisory, and MSW-in-production-build each proven to fail CI on a test branch

**Tier 1 — Design tokens**

- [ ] F7 — `frontend-design` skill extraction: Tailwind theme config + `tokens.css`, one pass
- [ ] F8 — Light palette wired; dark-mode structure stubbed, no values, no toggle
- [ ] F9 — `check-token-contrast.ts`, WCAG AA per pair, recorded

**Checkpoint P4-B** — every token traces to the design reference, zero invented values, contrast script passes

**Tier 2 — Component primitives**

- [ ] F10 — Radix-wrapped: `Modal`, `Toggle`, `SegmentedControl`, `Slider`, `Toast`, `VisuallyHidden`
- [ ] F11 — Hand-built: `Button`, `Input`, `ProgressBar`, `StatusPill`, `FormatBadge`, `Chip`, `Spinner`, `Skeleton`, `StatCard`, `EmptyState`
- [ ] F12 — `DataTable` (forced hand-built, no Radix table primitive)
- [ ] F13 — State-matrix + a11y-contract + token-only-styling coverage for all 17 primitives, Storybook stories

**Checkpoint P4-C** — 17/17 primitives classified per FR-1, zero `axe-core` violations, fully keyboard-operable, zero raw hex/px outside tokens

**Tier 3 — Generated cover system**

- [ ] F14 — 4-layer composition, FNV-1a seeding
- [ ] F15 — 3-step degradation ladder
- [ ] F16 — Memoization + Playwright benchmark (500 covers/100ms/60fps), CI gate

**Checkpoint P4-D** — pixel-identical determinism, all 3 degradation steps render, benchmark meets budget, never the sole accessible name

**Tier 4 — Shell and routing**

- [ ] F17 — React Router v7 + TanStack Query v5, full URL list, no direct `fetch`
- [ ] F18 — Shell composition + `MobileTabBar` reflow
- [ ] F19 — `useCapability()` hook, never-instant mock, no optimistic host-only render
- [ ] F20 — `<NotFound>` + MSW two-tier fixture strategy + `TODO(phase-06)` marker test
- [ ] F21 — Correlation-ID error states + `<EmptyState>` wiring

**Checkpoint P4-E** — every named URL resolves; capability-gating never flashes; `MobileTabBar` reflow proven; keyboard-only "open a book" completes; MSW absent from production build

**Tier 5 — Accessibility**

- [ ] F22 — Keyboard map documented+tested; focus-order rule, no positive `tabindex` (grep-checked)
- [ ] F23 — `<VisuallyHidden>` convention enforced (grep-checked)
- [ ] F24 — `axe-core`+`@playwright/test` CI stage
- [ ] F25 — Reduced-motion/high-contrast composed check on the E2E smoke test

**Checkpoint P4-F** — zero `axe-core` violations app-wide; keyboard map matches behavior; no positive `tabindex`; every hidden label uses `VisuallyHidden`; reduced-motion/high-contrast passes

**Tier 6 — Closure**

- [ ] F26 — Security audit (four-attacker pass), recorded in `.claude/audits/`
- [ ] F27 — All specs → `VERIFIED`; roadmap exit criteria walked with evidence; docs updated

**Checkpoint P4-G (final)** — full suite green, security audit clean (no open Critical/High), all specs `VERIFIED`, exit criteria cited, maintainer approval — phase 04 complete
