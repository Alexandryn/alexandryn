# Phase 04 — Frontend foundation

| | |
|---|---|
| **Status** | Implementation complete; in closure — Tier 6 F26 (audit) signed off, Checkpoint P4-G pending final maintainer approval |
| **Depends on** | Phase 01 |
| **Blocks** | 05, 06 |
| **Opened** | 2026-08-26 (Tier 0, PR #59) |
| **Closed** | — (Checkpoint P4-G) |

## Objective

A React, TypeScript and Tailwind application that renders shell chrome,
design-system primitives, and mock screen states — fully accessible, fully
tested, with zero domain logic wired to a real backend.

## Why here

Component work and backend work don't share a dependency in either
direction, so this runs alongside phase 03 rather than after it. It comes
after phase 01 because `architecture-frontend.md` decides state ownership,
data-fetching strategy, and routing — building the shell before that exists
means building it twice. It comes before phase 05 because the desktop host
serves this shell; there must be something to serve.

## Scope

**In**

- Build tooling: bundler, linting, formatting, typechecking, bundle size
  budget
- Design tokens as CSS custom properties, extracted from
  `.design-reference/Alexandryn.dc.html` — palette, spacing, radii, shadow
  scale
- Typography: Geist, Newsreader, IBM Plex Mono, with a defined scale and
  letter-spacing rules
- The generated placeholder cover system — a procedural, layered component
  (texture, spine, title, author) with a degradation ladder for when a real
  cover image never arrives
- Component primitives: buttons, inputs, segmented controls, status pills,
  format badges, chips, toggles, sliders, progress bars, stat cards, data
  tables, modals, skeletons, spinners, toasts
- Layout shell: sidebar, titlebar, content pane, mobile tab bar, responsive
  breakpoints
- The data-fetching and state layer's shape, per `architecture-frontend.md`
  — wired to a mock or contract-stub backend, not phase 03's real one
- Accessibility baseline: semantic elements, visible focus rings, full
  keyboard navigation, screen-reader text, prefers-contrast and
  prefers-reduced-motion support
- Frontend test stack: unit, component, accessibility (axe or equivalent),
  and the E2E harness's first smoke test

**Out**

- Wiring to the real phase 03 backend — phase 06 does the first real
  integration
- Desktop host IPC — phase 05
- Any screen the design reference doesn't cover: Design system has no
  captured canvas at all (`.design-reference/ANALYSIS.md`) and is not built
  from guesswork here. `atTablet` is captured but its file placement is
  still an open question (`architecture-frontend.md`'s own Open
  questions) — treated as deferred, not built assuming a settled surface,
  until that's resolved. Mobile and `atStates` are fully captured and
  binding (`architecture-frontend.md`'s Non-goals and FR-6) — this line
  previously listed them as undocumented, which predated that spec;
  corrected 2026-08-26

## Specifications

| Spec | Covers |
|---|---|
| `frontend-tooling.md` | Bundler, lint/format/typecheck config, bundle budget, CI hooks |
| `frontend-design-tokens.md` | Palette (light, structure for dark), type scale, spacing, radii, shadows |
| `frontend-component-primitives.md` | Every primitive listed above: props, states, a11y contract |
| `frontend-generated-covers.md` | Layer composition, seeding, degradation ladder, performance budget |
| `frontend-shell-and-routing.md` | Sidebar/titlebar/content/tab-bar shell, routing, data-fetching layer shape |
| `frontend-accessibility.md` | Keyboard map, focus order, screen-reader text conventions, motion/contrast rules |

## Architecture decisions expected

- Component library approach: hand-built primitives versus a headless
  library (Radix or similar) underneath the design tokens
- Where generated-cover output is cached — computed per render, memoised, or
  persisted
- Mock/contract-stub strategy for the data layer until phase 06: a fixture
  server, MSW-style interception, or hand-written stubs
- Dark palette: deferred entirely, or stubbed as unstyled tokens now so
  nothing has to be revisited structurally later

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Building Tablet, Mobile, Remote, States, or Design system screens from guesswork because they're referenced in nav strings | Medium | High — invents UI the design owner never approved | `.design-reference/ANALYSIS.md` names these explicitly as unbuilt; the shell renders only what's captured |
| Component primitives styled ad hoc per screen instead of tokenised | High | Medium — drift makes the design system fiction | Every primitive's spec is reviewed against the token set before merge |
| Accessibility treated as a pass at the end | Medium | High — retrofitting focus order and screen-reader text is expensive | Constitution §7: each primitive's spec includes its a11y contract before implementation |
| Generated-cover system becomes a performance sink at library-scale grids | Medium | Medium | Budget stated in `frontend-generated-covers.md`, tested against a large mock library |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Token resolution, generated-cover seeding and degradation logic |
| Component | Every primitive's states (default, hover, focus, disabled, error) |
| Accessibility | Automated axe checks on every shell layout and primitive |
| E2E | One smoke path through the shell with mock data, proving routing and shell composition work end to end |

The hardest thing to test here is the generated-cover degradation ladder —
it has to look intentional at every step, not just avoid crashing, and that
is closer to a design review than a unit test.

## Security considerations

Small at this phase, but two things carry forward to every screen built on
this shell:

- The data-fetching layer's mock/stub boundary must not leak into
  production builds — a build check, not a review habit
- No component renders unescaped external content by default; anything that
  will eventually hold source- or file-derived text (titles, filenames) uses
  a primitive that treats it as untrusted from the start, so phase 06
  inherits safety rather than retrofitting it

## Observability

Nothing runtime-observable yet — there's no server to report on. The bundle
size budget from `frontend-tooling.md` is the one number this phase tracks
over time, checked in CI.

## Exit criteria

- [x] **All six specifications `APPROVED` with recorded reviews** — cross-spec
      review [`0031`](../../reviews/0031-phase04-cross-spec-review.md) (two
      independent agents; 1 Blocking + 15 Major + 9 Minor, all fixed);
      post-approval amendments [`0032`](../../reviews/0032-spec-amendments-phase05-cross-phase-findings.md).
      Maintainer sign-off 2026-08-14 (`.claude/specs/README.md`).
- [x] **Component library and design tokens implemented and documented** —
      19 component directories under `web/src/components/` (the 17
      primitives `frontend-component-primitives.md` FR-1 classifies, plus
      `GeneratedCover` and `ErrorState`); `web/src/theme.css` +
      `web/src/tokens.css` + `web/src/breakpoints.ts` generated by
      `npm run tokens:generate` from `.design-reference/*.dc.html`, with a
      CI staleness gate (`git diff --exit-code` after regeneration);
      `check:token-styling` (CI) proves no raw hex/px outside the token
      set. PRs #60, #61. Detail: `tasks/todo-p04-tier2-component-primitives.md`.
- [x] **Generated cover component tested at every degradation step** —
      `web/src/components/GeneratedCover/GeneratedCover.test.tsx` covers
      all three FR-3 ladder steps (title+author / title only / neither);
      `.determinism.test.tsx` proves pixel-identical output across
      renders/sessions; `.a11y.test.tsx` proves the cover is never the
      sole accessible name. PR #62. Detail:
      `tasks/todo-p04-tier3-generated-covers.md`.
- [x] **Every component primitive keyboard-operable with a visible focus
      state** — `web/src/test/keyboardMap.test.tsx` (one representative
      primitive per FR-1 category, keyboard-only, no pointer events);
      `web/src/lib/focusRing.ts` is the shared visible-focus token
      applied by every primitive; `check:a11y-tabindex` (CI) proves no
      positive `tabindex`. `web/docs/keyboard-map.md` is the documented
      contract. PRs #61, #64.
- [x] **Automated accessibility tests passing on all shell layouts** —
      `web/e2e/a11y.app.spec.ts` (`@axe-core/playwright` on the shell at
      desktop and mobile width, and `<NotFound>`); the `gallery`
      Playwright project (`web/e2e/a11y-gallery.gallery.spec.ts`) scans
      every primitive; `web/src/test/axe.test.tsx` is the per-primitive
      jsdom pass. All three run in CI (the "Playwright — E2E,
      accessibility and cover benchmark" step and `npm test`). Zero
      violations. PRs #63, #64.
- [x] **Component preview environment functional** — Storybook, 19
      `*.stories.tsx` (one per component directory), `build-storybook`
      is a CI step that fails the pipeline if any story stops compiling.
      PR #59 (setup), populated across #61–#64.
- [x] **All specs in this phase are `VERIFIED`** — see the "Spec
      verification" section below; `.claude/specs/README.md` status column
      updated. (Tier 6 / F27, this closure.)
- [x] **Security audit recorded with no open Critical or High findings** —
      [`.claude/audits/0004-phase04-frontend-foundation.md`](../../audits/0004-phase04-frontend-foundation.md),
      verdict **Clear**. Three reconciled passes (`/security-review`, the
      `agent-skills:security-auditor` subagent, the
      `agent-skills:security-and-hardening` checklist) + manual
      verification T2–T6. 8 findings, 0 Critical/High: 1 fixed this tier
      (A-0004-07), 1 accepted phase-gated (A-0004-03), 6 carried to
      phases 05/06/12. Maintainer sign-off 2026-08-28. PR #65.
- [x] **Documentation updated** — this README's closure walk; the six
      specs → `VERIFIED`; `.claude/specs/README.md`; ADR 0016 → Accepted
      and `test-plans/README.md` amended (D3); the carried-item decisions
      recorded in `frontend-design-tokens.md` / `frontend-tooling.md` /
      `frontend-generated-covers.md` / `check-token-contrast.ts` (D2,
      items 3–5); `web/README.md`; `tasks/todo-phase04.md`.
- [x] **A test plan exists for every spec** (ADR 0016, Accepted 2026-08-28)
      — carried per-tier in `tasks/plan-phase04.md` and
      `tasks/plan-p04-<tier>.md`'s Test-strategy sections, written before
      each tier's RED step. This is the ratified form for a tiered phase
      (`test-plans/README.md`).
- [ ] **Maintainer approval recorded** — Checkpoint P4-G (below); the
      final Tier 6 gate, not yet crossed.

## Closure

Phase 04 was implemented in seven tiers (0–6), each its own PR, each
gated at a checkpoint:

| Tier | Scope | PR | Checkpoint |
|---|---|---|---|
| 0 | Build tooling (Vite, ESLint/Prettier, Vitest, Storybook, bundle/secrets/MSW checks, the two-job CI) | #59 | P4-A |
| 1 | Design-token extraction pipeline → Tailwind theme + `tokens.css` + `breakpoints.ts`; contrast script | #60 | P4-B |
| 2 | 17 component primitives (6 Radix-wrapped, 11 hand-built, `DataTable`), state matrix, a11y contract, token-only styling | #61 | P4-C |
| 3 | Generated cover system — 4-layer composition, FNV-1a seeding, 3-step degradation ladder, Playwright benchmark | #62 | P4-D |
| 4 | Shell + React Router v7 + TanStack Query v5, `useCapability()`, `<NotFound>`, MSW two-tier fixtures, correlation-ID error states | #63 | P4-E |
| 5 | Accessibility — keyboard map, no-positive-tabindex + hidden-text grep checks, real-browser axe gallery, `prefers-contrast` / reduced-motion | #64 | P4-F |
| 6 | Closure — security audit (F26), spec verification + this walk (F27) | #65 | **P4-G** |

**Carried past phase 04** (recorded, not resolved here):

- **`atTablet` file placement** — captured in the Electron/Admin (host)
  canvas, not the Web canvas ADR 0003 assigned it to. Needs the Claude
  Design project owner's confirmation. **Phase 04 closes without the
  host / LAN-client screen boundary settled** — `architecture-frontend.md`
  and `frontend-shell-and-routing.md` both carry it as an Open question,
  and `.design-reference/ANALYSIS.md` classifies it "Binding, location
  TBD". Phase 05/13 must re-read this rather than assume it settled.
- **FR-4 numeric budgets** — the 250 KiB bundle budget and the
  100 ms / 500-cover / 60 fps cover-render budget are reasoned
  placeholders. Phase-04 measured baseline: bundle ~103.9 KiB gzipped,
  benchmark ~184 ms. Confirm-or-replace deferred to phase 06 against
  real screens / a real grid (both specs' Open questions).
- **Benchmark-harness Tailwind scope** — `e2e/benchmark/` doesn't scan
  `src/` for Tailwind classes, so the cover renders there with fewer
  styles and its paint budget is measured lenient. Phase-06 follow-up,
  tied to the FR-4 re-baseline (`frontend-generated-covers.md` Open
  questions).
- **`getJson` boundary controls** (audit A-0004-05) and **CSP / security
  headers** (A-0004-04) — phase 05/06 serving layer.

## Spec verification

All six phase-04 specs move `APPROVED → IMPLEMENTED → VERIFIED` at this
closure. Per spec, the acceptance criteria and the evidence each is met:

- **`frontend-tooling.md`** — `npm run build` → `web/dist` at ~103.9 KiB
  gzipped (budget 250; `check:bundle-size` CI); oversized-bundle /
  High-advisory / planted-MSW all proven to fail CI on a test branch
  (Checkpoint P4-A, `tasks/todo-phase04.md`); `tsc -b` + ESLint zero
  warnings; the CI workflow invokes `npm run build` before
  `go build ./cmd/server` with the `web-dist` artifact hand-off; Vitest
  sentinel + 257 tests run in CI; `build-storybook` in CI;
  `npm audit --audit-level=high` a CI gate; `check:dist-secrets` +
  `check:dist-msw` against a real build.
- **`frontend-design-tokens.md`** — every token traces to
  `.design-reference/*.dc.html` (CI staleness gate: `tokens:generate`
  then `git diff --exit-code`); light palette wired, dark structurally
  stubbed (CSS custom properties, no values, no toggle);
  `check-token-contrast.ts` passes with the one recorded, maintainer-
  accepted `text-3` exception (D2); no invented values.
- **`frontend-component-primitives.md`** — all 17 primitives exist and
  are classified per FR-1 (no primitive unclassified); zero `axe-core`
  violations (`src/test/axe.test.tsx` + the `gallery` project); every
  interactive primitive keyboard-operable with a pointer-free test
  (`keyboardMap.test.tsx`); no raw hex/px outside tokens
  (`check:token-styling` CI); `prefers-reduced-motion` +
  `prefers-contrast` respected (media-query tests +
  `a11y-gallery.gallery.spec.ts`).
- **`frontend-generated-covers.md`** — same identifier → pixel-identical
  cover (`GeneratedCover.determinism.test.tsx`); all three degradation
  steps render without a blank box (`GeneratedCover.test.tsx`); the
  500-cover benchmark meets budget in CI (`benchmark.spec.ts`, ~184 ms);
  the cover is never the sole accessible name (`GeneratedCover.a11y.test.tsx`).
- **`frontend-shell-and-routing.md`** — every URL in
  `architecture-frontend.md` FR-1's list resolves to a real component or
  `<NotFound>` (`routes.test.tsx`); capability gating proven with a
  withhold-then-grant test, no host-only flash (`capability.test.tsx`,
  incl. the fail-closed case added in Tier 6); `<MobileTabBar>` reflow
  proven by a viewport-resize test (`shell.app.spec.ts`); keyboard-only
  "open a book" walkthrough completes (`shell.app.spec.ts`); MSW absent
  from the production build (`check:dist-msw` + the audit's T2); a mock
  error fixture's `correlationId` renders (`QueryResult.test.tsx`);
  empty response → `<EmptyState>` (`Library.test.tsx`); every tier-(b)
  fixture carries the `TODO(phase-06)` marker (`fixtures.test.ts`).
- **`frontend-accessibility.md`** — zero `axe-core` violations across
  every primitive and shell layout; the keyboard map is documented
  (`web/docs/keyboard-map.md`) and matches behaviour per category
  (`keyboardMap.test.tsx`); no positive `tabindex` (`check:a11y-tabindex`
  CI); every visually-hidden label uses `<VisuallyHidden>` / `sr-only`
  (`check:a11y-hidden-text` CI); the reduced-motion / high-contrast
  composed check passes (`a11y-gallery.gallery.spec.ts` +
  `check-token-contrast.ts` fallback assertion).

## Checkpoint P4-G

Pending. Presented to the maintainer when: the full suite is green
(build / lint / typecheck / Vitest / Storybook / `axe-core` / all three
Playwright projects / bundle-size / `npm audit` / the two a11y grep
checks / `token-styling` / `token-contrast` / staleness); the security
audit is recorded with no open Critical/High; all six specs are
`VERIFIED`; this exit-criteria walk is complete with evidence. On the
maintainer's approval, **Closed** is set and phase 04 is complete.
