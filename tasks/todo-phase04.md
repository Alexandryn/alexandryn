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
- [x] F2 — ESLint+Prettier+`jsx-a11y`, one shared config — pinned `eslint@^9` (jsx-a11y's peer range doesn't yet cover ESLint 10)
- [x] F3 — Vitest+RTL, sentinel test — jsdom + jest-dom matchers, passes
- [x] F4 — Storybook, `build-storybook` scripted — trimmed installer's default addons to just `addon-docs`+`eslint-plugin-storybook`, removed 4 undecided ones (Chromatic, addon-vitest/Playwright+Chromium, addon-a11y, addon-mcp)
- [x] F5 — Bundle-size check, secrets-grep, MSW-exclusion-grep — all three failure paths proven against real fixtures (nonzero exit), not just unit-level logic
- [x] F6 — `ci.yml`: new `frontend` job, `backend` gets `needs: frontend` + artifact hand-off — all steps verified locally, real GH Actions `needs:`/artifact proof pending this tier's own PR

**Checkpoint P4-A** — done, merged (PR #59). Real GitHub Actions proof of the new two-job `needs:`/artifact-hand-off structure: `Frontend` passed in 31s, `Backend` (waiting on `needs: frontend` + the real artifact download) passed in 5m29s — confirmed working, not just locally plausible.

**Tier 1 — Design tokens**

- [x] F7 — real extraction pipeline (`web/scripts/tokens/`), Tailwind theme config + `tokens.css`, one pass, `npm run tokens:generate` — proven against the real 4 canvases, 2 real bugs caught by tests before generation ran for real (`--cov` miscategorized as a color, letter-spacing double-prefixing)
- [x] F8 — light palette wired; dark satisfied by construction (every token is a plain CSS custom property via `var()`, no dark values populated, no toggle) — no separate code needed
- [x] F9 — `check-token-contrast.ts`, WCAG AA per meaningful text/surface pair — real finding: `text-3` fails AA against every surface (2.90:1 max), recorded as a documented exception pending a maintainer decision, not silently fixed or hidden

**Checkpoint P4-B** — done. Every token traces to the design reference (verified: `git diff --exit-code` on regenerated output is clean); zero invented values; contrast script passes with one flagged, documented exception (`text-3`). CI wired: staleness check + contrast check, both proven locally.

**Tier 2 — Component primitives**

- [x] F10 — Radix-wrapped: `Modal`, `Toggle`, `SegmentedControl`, `Slider`, `Toast`, `VisuallyHidden`
- [x] F11 — Hand-built: `Button`, `Input`, `ProgressBar`, `StatusPill`, `FormatBadge`, `Chip`, `Spinner`, `Skeleton`, `StatCard`, `EmptyState`
- [x] F12 — `DataTable` (forced hand-built, no Radix table primitive)
- [x] F13 — State-matrix + a11y-contract + token-only-styling coverage for all 17 primitives, Storybook stories

**Checkpoint P4-C** — done, merged (PR #61). 17/17 primitives classified per FR-1, zero `axe-core` violations, fully keyboard-operable, zero raw hex/px outside tokens (`check:token-styling`, new). Detail: `tasks/todo-p04-tier2-component-primitives.md`.

**Tier 3 — Generated cover system**

- [x] F14 — 4-layer composition, FNV-1a seeding
- [x] F15 — 3-step degradation ladder
- [x] F16 — Memoization + Playwright benchmark (500 covers/100ms/60fps), CI gate

**Checkpoint P4-D** — done. Pixel-identical determinism, all 3 degradation steps render, benchmark meets budget (Playwright, CI-wired), never the sole accessible name. Detail: `tasks/todo-p04-tier3-generated-covers.md`.

**Tier 4 — Shell and routing**

- [x] F17 — React Router v7 + TanStack Query v5, full URL list, no direct `fetch`
- [x] F18 — Shell composition + `MobileTabBar` reflow
- [x] F19 — `useCapability()` hook, never-instant mock, no optimistic host-only render
- [x] F20 — `<NotFound>` + MSW two-tier fixture strategy + `TODO(phase-06)` marker test
- [x] F21 — Correlation-ID error states + `<EmptyState>` wiring

**Checkpoint P4-E** — done, merged (PR #63). Detail: `tasks/todo-p04-tier4-shell-and-routing.md`.

**Tier 5 — Accessibility**

- [x] F22 — Keyboard map documented+tested; focus-order rule, no positive `tabindex` (grep-checked)
- [x] F23 — `<VisuallyHidden>` convention enforced (grep-checked)
- [x] F24 — `axe-core`+`@playwright/test` CI stage
- [x] F25 — Reduced-motion/high-contrast composed check on the E2E smoke test

**Checkpoint P4-F** — done, merged (PR #64). Detail: `tasks/todo-p04-tier5-accessibility.md`.

**Tier 6 — Closure** — detail: `tasks/todo-p04-tier6-closure.md`

- [x] F26 — Security audit (four-attacker pass) — `.claude/audits/0004-phase04-frontend-foundation.md`, verdict Clear, 0 Critical/High; three reconciled passes + manual T2–T6; maintainer sign-off 2026-08-28 (Gate 1). PR #65.
- [x] F27 — six specs → `VERIFIED`; `roadmap/04-frontend-foundation/README.md` exit criteria walked with evidence; carried-item decisions recorded (D2 text-3, D3 ADR 0016, items 3–5); docs swept.

**Checkpoint P4-G (final)** — full suite green, security audit clean (no open Critical/High), all six specs `VERIFIED`, roadmap exit criteria cited with evidence — **pending final maintainer approval** (Gate 2). On approval: roadmap `Closed` is set, phase 04 complete.
