# Spec: Frontend tooling

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-28, phase 04 Tier 6 / F27 — implemented Tier 0 (PR #59), audited `0004`, acceptance criteria walked in [`roadmap/04`](../roadmap/04-frontend-foundation/README.md#spec-verification)) — was `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `04-frontend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | `0031` (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`architecture-frontend.md` fixed CSR-only (FR-7), real routing (FR-1),
and a dedicated data-fetching layer (FR-2), but left every build-tooling
choice — bundler, linter, formatter, typechecker config, bundle budget —
to phase 04, same "fix shape, phase decides tools" pattern
`architecture-backend.md` used for phase 03's router/driver/migration
choices. `architecture-testing.md` fixed the system-wide CI shape
(GitHub Actions, ordered stages) and named a frontend linter as
phase 04's own pick; this spec is where that becomes concrete.

## Problem

Nothing has fixed: the bundler, the lint/format tool chain, TypeScript's
strictness config, the bundle-size budget and how it's enforced, or how
`web/`'s build output reaches `cmd/server`'s `go:embed` (ADR 0008 already
decided the embedding *direction*, not the build command that produces
what gets embedded).

## Goals

- Pick a bundler/dev-server, justified under constitution §9
- Pick a lint/format toolchain producing one canonical style, no
  per-contributor formatting debate
- Fix TypeScript's strictness: `strict: true` baseline, and whether
  anything stricter (`noUncheckedIndexedAccess`, etc.) is worth the
  friction this early
- Fix a real bundle-size budget and the CI mechanism that enforces it
- Fix the exact build command `architecture-testing.md` FR-8's
  `web/`-before-`go build` CI ordering invokes

## Non-goals

- The component library itself — `frontend-component-primitives.md`
- Design tokens — `frontend-design-tokens.md`
- Individual component/screen test *content* — this spec fixes the test
  *runner* (FR-7) every other phase 04 spec's Test strategy assumes;
  what each spec actually tests with it is that spec's own concern
- Storybook's actual per-primitive story content — FR-8 fixes that the
  tool exists and runs in CI; populating it with real stories is
  `frontend-component-primitives.md`'s implementation work, not
  enumerated here

## User stories

- As **a contributor**, I want one command that builds, lints, and
  typechecks, so CI and my local machine run the identical check.
- As **`architecture-testing.md` FR-8's CI workflow**, I want a single,
  fixed build command to invoke before `go build ./cmd/server`, so
  ADR 0008's embedding contract has something deterministic to embed.
- As **a reviewer**, I want a bundle-size regression to fail CI
  automatically, not be caught by someone eyeballing a deploy.

## Functional requirements

- **FR-1** The bundler/dev-server is **Vite** (pinned to its current
  major version in `package.json`, exact version in the lockfile,
  matching every other dependency this batch pins per constitution §9)
  — fast HMR for component iteration, native TypeScript support, and
  the de facto standard for a CSR-only React app in 2026; justified
  under constitution §9: small, well-maintained, replacing what would
  otherwise be a hand-rolled esbuild/Rollup pipeline this project
  doesn't need to own. If abandoned, the exit cost is bounded — Vite's
  config format is a thin layer over esbuild/Rollup directly, so a
  migration to a successor tool or to hand-rolled esbuild config
  wouldn't require rewriting application code, only build config.
  Build output goes to `web/dist`, the exact path `architecture-testing.md`
  FR-8's CI ordering and ADR 0008's `go:embed` directive both reference.
- **FR-2** TypeScript runs in `strict: true` mode plus
  `noUncheckedIndexedAccess` — the one additional check worth the early
  friction, since it catches a real class of bug (assuming an array/map
  lookup succeeded) that `strict` alone doesn't, and is far cheaper to
  adopt now than retrofit once hundreds of components exist. No other
  non-default strictness flag is enabled without a specific, named
  reason.
- **FR-3** Lint/format is **ESLint** (with `typescript-eslint`,
  `eslint-plugin-jsx-a11y` for FR-5's accessibility baseline hooks) plus
  **Prettier** for formatting, the standard, boring pairing — boring is
  the point, since a novel toolchain here buys nothing constitution §9
  would accept as justification. One shared config, committed, no
  per-contributor override.
- **FR-4** The bundle-size budget is **250 KiB** gzipped for the initial
  JS payload (the shell + router + data-fetching layer, before any
  route-level code-splitting reduces a specific screen's own chunk) — a
  placeholder-but-reasoned number for a desktop-adjacent app served over
  loopback/LAN (not a public web page competing for mobile data budgets),
  generous enough for a real component library, tight enough that it
  isn't a rubber stamp. Enforced in CI via a build-output size check
  that fails the pipeline on regression past this number — flagged in
  Open questions as confirm-or-replace once phase 06 adds real screens
  to measure against, the same placeholder pattern phase 03 used
  throughout.
- **FR-5** `eslint-plugin-jsx-a11y` runs as part of the standard lint
  step (FR-3), not a separate accessibility-specific CI stage — catching
  missing `alt` text, invalid ARIA usage, and similar structural issues
  at the same point every other lint violation is caught, before
  `frontend-accessibility.md`'s runtime `axe` checks ever run.
- **FR-6** `architecture-testing.md` FR-8's CI ordering invokes exactly
  `npm run build` (or the package manager's equivalent) inside `web/`,
  producing `web/dist`, before `go build ./cmd/server` runs — this spec
  fixes that command's existence and output path as the literal contract
  ADR 0008's `go:embed` directive depends on; not a new requirement,
  making an existing one concrete.
- **FR-7** The unit/component test runner is **Vitest** (with React
  Testing Library for component rendering) — Vite-native (FR-1), so no
  second bundler/config pipeline is needed just to run tests, and the
  de facto pairing for a Vite-based React app. Justified under
  constitution §9: avoids maintaining a separate Jest/Babel
  configuration that would duplicate what Vite's own transform pipeline
  already does. Every other phase 04 spec's Unit/Component Test strategy
  row (`frontend-component-primitives.md`, `frontend-shell-and-routing.md`,
  `frontend-generated-covers.md`) runs against this tool — it did not
  exist as a named FR before this amendment, an omission review `0031`
  caught.
- **FR-8** The component-preview environment is **Storybook**, wired to
  the same Vite config (FR-1) and token/component source
  `frontend-component-primitives.md` produces — a build step
  (`npm run build-storybook` or equivalent) runs in CI, failing the
  pipeline if Storybook itself fails to build (proving every story
  compiles against current component code), separate from FR-6's
  application build. Justified under constitution §9: the roadmap's own
  exit criteria require a "component preview environment... functional,"
  Storybook is the standard tool for exactly this, and hand-rolling one
  would be reinventing a solved problem for no project-specific reason.

## Non-functional requirements

- **Performance** — FR-4's bundle budget is this spec's own performance
  property; no runtime performance budget exists yet, since no component
  exists to profile (same status `architecture-frontend.md` already
  named).
- **Security** — see Security considerations below.
- **Accessibility** — FR-5 wires the structural lint check; the runtime
  requirement itself belongs to `frontend-accessibility.md`.
- **Reliability** — one fixed toolchain (FR-1/FR-3) is what keeps "works
  on my machine" from being a real failure mode; no per-contributor
  config drift is possible if the shared config is the only one that
  exists.
- **Observability** — the bundle-size check's own output (FR-4) is
  logged/reported in CI the same way phase 03's coverage stage is —
  captured, not just pass/fail.

## Domain model

Not applicable — build tooling, not the Alexandryn domain.

## API and contracts

- **`web/` build output ↔ `cmd/server`'s `go:embed`**: `web/dist`, the
  exact path (FR-1/FR-6), ADR 0008's own contract made concrete here.
- **Lint/format config ↔ every contributor's editor**: one shared,
  committed config (FR-3) — no contributor-local override changes what
  CI enforces.

## State transitions

Not applicable.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Bundle size regresses past FR-4's budget | CI's size check | CI red at the build-size stage | Merge blocked, same `architecture-testing.md` FR-1 gating every other stage uses |
| `web/dist` missing or stale when `go build` runs | ADR 0008's own embed directive fails to compile | CI red at the Go build stage, before any test runs | `architecture-testing.md` FR-8's ordering — build failure caught before wasting time on tests against a stale bundle |
| A contributor's local lint/format disagrees with CI's | FR-3's shared config, run identically in both places | Local run matches CI — no surprise failures on push | One config file, no environment-specific override |

## Security considerations

- **No secrets in the bundle** — restates `architecture-frontend.md`'s
  own Security considerations; this spec's concrete mechanism is a
  build-time check (grep for common secret-shaped patterns in `web/dist`
  output) as a CI step, closing the gap between "flagged for phase 12"
  and "actually checked for" now, since a build-time leak is checkable
  today even before phase 12 has a real secret to leak.
- **Dependency audit from the first dependency** — constitution §9,
  `architecture-testing.md` FR-5's SCA-tool requirement made concrete:
  **`npm audit --audit-level=high`** runs as its own CI step, failing
  the pipeline on any High or Critical advisory — the same
  Critical/High/Medium/Low taxonomy `architecture-testing.md` FR-4
  already established for Docker image scanning, reused here rather
  than inventing a second severity scale. `npm audit` specifically
  (not a third-party SCA service) because it's already present with no
  new dependency or account to provision, and this project's own
  dependency surface is small enough that a hosted SCA product's extra
  features (license scanning, PR-bot auto-fixes) aren't worth the
  added account/secret surface yet — revisit if the dependency count
  grows enough to justify it.
- **MSW's bundle excluded from production builds** — restates
  `roadmap/04-frontend-foundation/README.md`'s own named risk ("the
  mock/stub boundary must not leak into production builds — a build
  check, not a review habit") as an enforced CI step: the same
  build-time grep-style check as the secrets check above, applied to
  confirm no MSW service-worker registration or `msw` package code
  appears in `web/dist`. `frontend-shell-and-routing.md` FR-6 is the
  spec that requires MSW exist for development/testing; this spec is
  where the production-exclusion check that requirement depends on
  actually lives, since it's this spec's build pipeline that produces
  the artifact being checked.

## Test strategy

| Layer | What it covers |
|---|---|
| Build | `npm run build` succeeds, produces `web/dist`, stays under FR-4's budget |
| Lint | ESLint + Prettier + `jsx-a11y` (FR-3/FR-5), zero warnings tolerated in CI |
| Typecheck | `tsc --noEmit` under FR-2's strictness config |
| Unit/Component | Vitest + React Testing Library (FR-7) run and report in CI, zero test content required by this spec itself — proving the runner works, not testing any component |
| Storybook build | `build-storybook` (FR-8) succeeds against current component source |
| Dependency audit | `npm audit --audit-level=high` (Security considerations), zero High/Critical advisories tolerated |
| Production-bundle checks | Secrets grep and MSW-exclusion grep (Security considerations), both against `web/dist` |

This spec's own test content is the build pipeline itself — proving each
tool runs and gates correctly, not testing any component or screen,
which is every other phase 04 spec's job.

## Acceptance criteria

- [ ] `npm run build` produces `web/dist` under FR-4's 250 KiB budget,
      proven with a CI run
- [ ] A deliberately oversized bundle fails CI, proven with a test
      branch
- [ ] `tsc --noEmit` and ESLint both pass with zero warnings on a clean
      checkout
- [ ] `architecture-testing.md` FR-8's CI ordering actually invokes this
      spec's build command before `go build`, proven by the workflow
      file
- [ ] Vitest runs in CI and reports results, proven by a trivial
      sentinel test (FR-7)
- [ ] Storybook builds successfully in CI (FR-8)
- [ ] `npm audit --audit-level=high` runs in CI and fails on a
      deliberately introduced High-severity advisory, proven with a
      test branch
- [ ] A deliberately included MSW reference in a production build fails
      the MSW-exclusion check, proven with a test branch

## Open questions

- **FR-4's 250 KiB number** — reasoned placeholder, not measured against
  a real component library; confirm or replace once phase 06 adds real
  screens. *(Phase 04 Tier 6 / F27 baseline, 2026-08-28: the built
  initial JS payload is **~103.9 KiB gzipped** — 18 primitives + the
  generated-cover system + shell + router + TanStack Query + MSW-free
  production build. That is the number phase 06 confirms or replaces
  against; the 250 KiB budget held with ~58% headroom through all of
  phase 04. `check:bundle-size` gates it in CI.)*
- **Route-level code-splitting** — FR-4's budget covers the initial
  payload; whether/how per-route chunks are split is an implementation
  detail this spec doesn't fix, since no routes exist yet
  (`frontend-shell-and-routing.md`'s concern once they do).

## References

- `architecture-frontend.md` FR-7 (CSR-only), FR-1/FR-2 (routing,
  data-fetching — tool choices this spec's FR-1/FR-3 make concrete)
- `architecture-testing.md` FR-1 (CI gating), FR-5 (dependency scanning),
  FR-8 (`web/`-before-`go build` ordering, the exact contract FR-6 fixes)
- ADR 0008 — monorepo layout, `go:embed` direction
- `.claude/skills/README.md` — `frontend-design` skill, used during
  actual token/component extraction, not this spec's own concern
- Constitution §9 (dependencies)
