# Phase 04 — Frontend foundation

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 01 |
| **Blocks** | 05, 06 |
| **Opened** | — |
| **Closed** | — |

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
- Any screen the design reference doesn't cover: Tablet, Mobile, Remote,
  States, Design system are undocumented per
  `.design-reference/ANALYSIS.md` and are not built from guesswork here

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

- [ ] All six specifications `APPROVED` with recorded reviews
- [ ] Component library and design tokens implemented and documented
- [ ] Generated cover component tested at every degradation step
- [ ] Every component primitive keyboard-operable with a visible focus state
- [ ] Automated accessibility tests passing on all shell layouts
- [ ] Component preview environment (Storybook or equivalent) functional
- [ ] All specs in this phase are `VERIFIED`
- [ ] Security audit recorded in `.claude/audits/` with no open Critical or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
