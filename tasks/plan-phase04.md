# Phase 04 (Frontend foundation) — implementation plan

## Context

Alexandryn is a self-hosted digital library. Phase 03 (backend) closed
2026-08-26. Phase 04 builds the React/TypeScript/Tailwind frontend shell:
build tooling, design tokens, component primitives, the generated-cover
system, shell/routing, and accessibility — fully mocked against phase
03's contract, zero real backend wiring (phase 06's job). Phase 04
depends only on Phase 01 (done) and runs independently of phase 03/05's
own work.

All six phase-04 specs are `APPROVED` (2026-08-14, cross-spec reviewed in
review `0031`, all findings fixed): `frontend-tooling.md`,
`frontend-design-tokens.md`, `frontend-component-primitives.md`,
`frontend-generated-covers.md`, `frontend-shell-and-routing.md`,
`frontend-accessibility.md`, governed by `architecture-frontend.md`
(phase 01, `APPROVED` 2026-08-20). Unusually for this project, these
specs are near-exhaustively concrete — nearly every tool, library, and
number is already fixed. This plan is sequencing and TDD mechanics, not
new decisions, with one real exception (D2 below).

Design-conformance check performed 2026-08-26: all 4
`.design-reference/*.dc.html` canvases re-synced against the live Claude
Design project (`78075626-e444-438f-8437-205d57129a37`) via `DesignSync`
— byte-identical to the local cache, no drift since 2026-08-13. One real
scope conflict found: phase 04's own roadmap README still described
Mobile/`atStates` as undocumented, contradicting `architecture-frontend.md`
which had already superseded that (Mobile is responsive web, not native;
`atStates` is wired into FR-6) — fixed in a doc commit before this plan
was written. `atTablet`'s file placement stays the one genuinely open
question, already tracked in two approved specs — deferred, not built
assuming a settled surface, same as those specs' own precedent.

This project runs strict TDD (RED → GREEN → Refactor, test before code)
and requires: token-only styling (no raw hex/px in component source),
server-told capability gating (never client-inferred), a documented
keyboard map applied consistently, and no primitive shipped missing a
required state — all fixed by the six specs.

## Decisions (resolve before/alongside the tasks that need them)

- **D1 — task-doc naming: fresh phase-04 docs, not appended to phase
  03's.** `tasks/plan.md`/`tasks/todo.md` are explicitly titled "Phase
  03" and now closed. Phase 04 gets its own pair (this file and
  `tasks/todo-phase04.md`), with per-tier sub-docs
  (`tasks/plan-p04-<tier>.md`) once a tier is large enough to warrant
  one, mirroring T24-T27's own precedent within phase 03.
- **D2 — CI shape: extend the existing `Backend` job's workflow file
  with a new, separate `frontend` job, sequenced before `backend` via
  `needs:`.** `architecture-testing.md` FR-8 fixes web-before-go-build
  *ordering*, not that both must be one job. A separate job lets the
  frontend toolchain (Node) and backend toolchain (Go) each use their
  own natural GitHub Actions setup action without cross-contaminating
  caches or install steps, while `needs: frontend` on the `backend` job
  (plus an `actions/upload-artifact`/`download-artifact` pair moving
  `web/dist` between jobs) enforces the real ordering ADR 0008 needs —
  `go build` must see `web/dist` already present for `go:embed` to
  succeed. Rejected: cramming frontend steps into the existing `backend`
  job — would force every backend-only PR to pay Node setup cost, and
  vice versa, and mixes two independent toolchains' failure surfaces
  into one job's red/green signal.
- **D3 — `web/` directory location and package manager: `web/` at
  repository root, `npm`** (not pnpm/yarn) — matches `frontend-tooling.md`
  FR-1's exact `web/dist` path, and `npm` is what's already invoked
  throughout the approved specs' own Test strategy/Acceptance criteria
  text (`npm run build`, `npm audit`) — not re-litigated, just the
  literal tool the specs already name.
- **Everything else tool/library-wise is fixed by the specs, not
  re-decided here**: Vite, TypeScript `strict`+`noUncheckedIndexedAccess`,
  ESLint+Prettier+`jsx-a11y`, Vitest+RTL, Storybook, React Router v7,
  TanStack Query v5, MSW, Radix UI (six named primitives), `axe-core`+
  `@playwright/test`, FNV-1a seeding, the 250 KiB bundle budget.

## Task list

**Tier 0 — Bootstrap (`frontend-tooling.md`)**

- **F1.** `web/` scaffold: Vite+React+TS via `npm create vite@latest`,
  `package.json` with every FR-1/FR-3/FR-7/FR-8 dependency pinned,
  `tsconfig.json` (`strict: true`, `noUncheckedIndexedAccess: true`,
  FR-2).
- **F2.** ESLint (`typescript-eslint`, `eslint-plugin-jsx-a11y`) +
  Prettier, one shared committed config, zero-warnings-tolerated (FR-3/
  FR-5).
- **F3.** Vitest + React Testing Library config, one trivial sentinel
  test proving the runner works (FR-7).
- **F4.** Storybook, wired to the same Vite config, `build-storybook`
  scripted (FR-8).
- **F5.** Bundle-size check (FR-4, 250 KiB gzipped budget) as a CI-
  runnable script; secrets-grep and MSW-exclusion-grep checks against
  `web/dist` (Security considerations).
- **F6.** `ci.yml`: new `frontend` job (D2) — `npm run build`, ESLint,
  `tsc --noEmit`, Vitest, `build-storybook`, bundle-size check,
  `npm audit --audit-level=high`, secrets/MSW-exclusion greps;
  `backend` job gets `needs: frontend` plus the artifact hand-off for
  `web/dist`.

  > **Checkpoint P4-A** — `npm run build`/lint/typecheck/Vitest/Storybook
  > all green locally and in CI; a deliberately oversized bundle, a
  > deliberately introduced High-severity `npm audit` advisory, and a
  > deliberately included MSW reference in a production build each fail
  > CI, proven on a test branch (FR-4/Security Acceptance criteria).

**Tier 1 — Design tokens (`frontend-design-tokens.md`)**

- **F7.** Run the `frontend-design` skill against all 4 `.dc.html`
  canvases; extraction script produces Tailwind theme config +
  `tokens.css` from one pass (FR-1/FR-2) — color, spacing, radius,
  shadow, typography (Geist/Newsreader/IBM Plex Mono, FR-5), breakpoint.
- **F8.** Light palette only; dark-mode CSS-variable structure exists
  with no values populated, no toggle (FR-3).
- **F9.** `web/scripts/check-token-contrast.ts` — every color token pair
  checked against WCAG AA, results recorded at extraction time (Test
  strategy).

  > **Checkpoint P4-B** — every extracted token traces to the design
  > reference, zero invented values (FR-4), contrast script passes,
  > `tokens.css` and Tailwind config both generated and consistent.

**Tier 2 — Component primitives (`frontend-component-primitives.md`)**

- **F10.** Radix-wrapped: `Modal`, `Toggle`, `SegmentedControl`,
  `Slider`, `Toast`, `VisuallyHidden` (FR-1).
- **F11.** Hand-built: `Button`, `Input`, `ProgressBar`, `StatusPill`,
  `FormatBadge`, `Chip`, `Spinner`, `Skeleton`, `StatCard`, `EmptyState`
  (FR-1).
- **F12.** `DataTable` — hand-built, no Radix table primitive exists
  (FR-1), correct `<table>`/`<th scope>` semantics, keyboard-operable
  sort, `aria-selected` row state (FR-3).
- **F13.** State-matrix coverage (default/hover/focus/disabled/error,
  FR-2) and per-category a11y contract (FR-3) for all 17 primitives, one
  task per primitive or small related group — token-only styling (FR-4),
  `prefers-reduced-motion`/`prefers-contrast` respected (FR-5), Storybook
  story each.

  > **Checkpoint P4-C** — all 17 primitives exist, fully classified per
  > FR-1, zero `axe-core` violations, fully keyboard-operable (test never
  > uses a pointer event), zero raw hex/px values outside the token set
  > (grep-checked).

**Tier 3 — Generated cover system (`frontend-generated-covers.md`)**

- **F14.** 4-layer composition (texture/spine/title/author, FR-1),
  FNV-1a deterministic seeding from `Work.ID`/`Edition.ID` (FR-2).
- **F15.** 3-step degradation ladder (FR-3): full → no-author-recenter →
  texture+spine-only.
- **F16.** Memoization (session-cached per identifier); Playwright-based
  benchmark harness — 500 covers, ≤100ms initial paint, 60fps scroll
  (FR-4), CI regression gate.

  > **Checkpoint P4-D** — same identifier produces pixel-identical output
  > across renders/sessions; all 3 degradation steps render without a
  > blank box or crash; benchmark meets budget in CI; cover never the
  > sole accessible name for its book (screen-reader test).

**Tier 4 — Shell and routing (`frontend-shell-and-routing.md`)**

- **F17.** React Router v7 (data-router mode) with the full URL list
  (FR-1); TanStack Query v5, no component calls `fetch` directly (FR-2).
- **F18.** Shell composition: `Sidebar`/`Titlebar`/`ContentPane`,
  `MobileTabBar` reflow below Tier 1's breakpoint token (FR-3).
- **F19.** `useCapability()` hook (FR-4) — mock returns all-granted,
  never instant; every host-only route waits for it, never renders
  optimistically.
- **F20.** `<NotFound>` (FR-5); MSW two-tier fixture strategy — contract-
  generated where `api/openapi.yaml` covers it, hand-written with a
  `TODO(phase-06)` marker otherwise (FR-6), enforced by a test scanning
  fixture files.
- **F21.** Correlation-ID-visible error states (FR-7); `<EmptyState>`
  wiring for genuinely-empty successful responses (FR-8).

  > **Checkpoint P4-E** — every named URL resolves to a real component or
  > `<NotFound>`; capability-gating test proves host-only content never
  > flashes before disappearing; `MobileTabBar` reflow proven by a
  > viewport-resize test; keyboard-only "open a book" walkthrough
  > completes; MSW verifiably absent from a production build.

**Tier 5 — Accessibility (`frontend-accessibility.md`)**

- **F22.** Keyboard map (FR-1) documented and tested per category;
  focus-order rule (FR-2) — no positive `tabindex`, grep-checked.
- **F23.** `<VisuallyHidden>` convention enforced (FR-3) — grep-checked
  for `display:none`/manual clip-rect misuse on text content.
- **F24.** `axe-core` + `@axe-core/playwright` CI stage (FR-4), distinct
  from Tier 0's lint-time `jsx-a11y` check.
- **F25.** Reduced-motion/high-contrast composed check (FR-5), riding on
  Tier 4's E2E smoke test.

  > **Checkpoint P4-F** — zero `axe-core` violations across every
  > primitive and shell layout; keyboard map matches actual behavior; no
  > positive `tabindex` anywhere; every visually-hidden label uses
  > `<VisuallyHidden>`; reduced-motion/high-contrast check passes.

**Tier 6 — Closure**

- **F26.** Security audit (constitution §10's four-attacker pass), scoped
  to this phase's real trust boundaries: MSW/mock-boundary production
  leak, XSS via future source/file-derived text (React's default
  escaping, no `dangerouslySetInnerHTML` anywhere), no secrets in the
  bundle. Recorded in `.claude/audits/`.
- **F27.** All six spec statuses moved to `VERIFIED`; roadmap exit
  criteria walked item by item, each citing real evidence (same
  discipline phase 03's Checkpoint H used); documentation updated.

  > **Checkpoint P4-G (final)** — full suite green (build/lint/typecheck/
  > Vitest/Storybook/`axe-core`/Playwright E2E/bundle-size/`npm audit`),
  > security audit recorded with no open Critical/High findings, all
  > specs `VERIFIED`, roadmap exit criteria checked with evidence,
  > maintainer approval recorded — phase 04 complete.

## Critical files

- `.claude/specs/architecture-frontend.md` — governing spec, FR-1
  through FR-7, every phase-04 spec's own FRs make one of these concrete
- `.claude/specs/frontend-tooling.md` — Tier 0's source
- `.claude/specs/frontend-design-tokens.md` — Tier 1's source
- `.claude/specs/frontend-component-primitives.md` — Tier 2's source
- `.claude/specs/frontend-generated-covers.md` — Tier 3's source
- `.claude/specs/frontend-shell-and-routing.md` — Tier 4's source
- `.claude/specs/frontend-accessibility.md` — Tier 5's source
- `.design-reference/*.dc.html`, `.design-reference/ANALYSIS.md` — the
  visual source of truth every token/screen extraction traces back to
- `.claude/skills/README.md` — `frontend-design` skill (Tier 1)
- ADR 0008 — monorepo layout, `web/dist` → `go:embed` direction
- `.github/workflows/ci.yml` — D2's new `frontend` job

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| `atTablet`'s file placement stays unresolved through this whole phase | Low for phase 04 itself (no tablet-specific screen is built assuming a settled surface) — real impact lands whenever phase 05/13 needs it | Deferred per two approved specs' own precedent, not resolved here; flagged again if phase 05 needs it |
| FR-4's numeric budgets (250 KiB bundle, 100ms/500-cover/60fps benchmark) are reasoned placeholders, not measured against a real component library | Medium — could need revision once phase 06 adds real screens | Both specs' own Open questions already name this; confirm-or-replace explicitly deferred to phase 06, not silently assumed final |
| A large FR-6 tier-(b) hand-written-fixture surface if phase 04's screens need many endpoints `api/openapi.yaml` doesn't have yet | Medium | `TODO(phase-06)` marker + a test enforcing its presence, per spec; if the count turns out large, flagged for `architecture-contracts.md` amendment rather than silently accepted |
| D2's two-job CI split is new territory for this repo (phase 03's `ci.yml` has one job) | Low | Proven with a real PR before Tier 1 starts — `needs:`/artifact hand-off tested for real, not assumed to work from reading GitHub Actions docs alone |

## Open questions

- `atTablet`'s actual surface (inherited from `architecture-frontend.md`
  and `frontend-shell-and-routing.md`'s own Open questions) — needs the
  Claude Design project owner's confirmation before either spec treats
  it as settled; not blocking this plan.
- The exact responsive breakpoint value, FR-4's numeric budgets, and
  whether an `explicit className` escape hatch exists on primitives —
  all explicitly left to phase 04 implementation by the specs
  themselves, resolved as each tier is actually built, not guessed here
  in advance.
