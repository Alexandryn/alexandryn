# Phase 04, Tier 3 — Generated cover system — implementation plan

Full phase context: [`tasks/plan-phase04.md`](plan-phase04.md). Task list:
[`tasks/todo-p04-tier3-generated-covers.md`](todo-p04-tier3-generated-covers.md).
Spec: [`frontend-generated-covers.md`](../.claude/specs/frontend-generated-covers.md)
(`APPROVED`).

## Context

Tier 0-2 are merged to `main` (PR #59/#60/#61). Tier 3 builds the
generated placeholder cover: four pure-function layers (texture, spine,
title, author), FNV-1a deterministic seeding, a three-step degradation
ladder, session memoization, and a Playwright-based performance
benchmark as a CI gate (FR-4).

Design-conformance re-check: this session already re-pulled all 4
`.dc.html` canvases via `DesignSync` against project
`78075626-e444-438f-8437-205d57129a37` earlier today (Tier 2 kickoff),
byte-identical to `.design-reference/`, no new files. This spec's own
Non-goals exclude "any specific book's actual generated appearance"
and the library-grid layout — it fixes the *system*, not a captured
screen, so there is no canvas this component's visual output is
checked against; the design reference's role here is limited to
`frontend-design-tokens.md`'s Newsreader typography token (FR-1's
title layer) and the token palette generally, already extracted in
Tier 1.

`web/src/lib/` already has `cx.ts`, `focusRing.ts`,
`useAnnouncedText.ts` — this tier adds `fnv1a.ts` alongside them.
`web/package.json` has no `@playwright/test` yet — this is the first
tier to add it, though it's not a fresh ad hoc choice: FR-4 itself
names `@playwright/test` as the required measurement mechanism (Vitest
doesn't run in a real browser), and `frontend-shell-and-routing.md`/
`frontend-accessibility.md` reference the same real CI dependency for
their own later stages — this tier is just the first to actually
install it.

## Decisions

- **D1 — component name/location: `GeneratedCover`, `web/src/components/GeneratedCover/`.**
  Matches the existing per-primitive folder convention Tier 2 established
  (`ComponentName/ComponentName.tsx` + `.test.tsx` + `.stories.tsx` +
  `index.ts`). Not itself one of Tier 2's 17 primitives (this spec is
  explicitly distinct per `frontend-component-primitives.md`'s own
  Non-goals), but shares the same file-shape convention since nothing
  about that convention is primitive-specific.
- **D2 — texture/spine visual algorithm: seeded HSL color band + a
  small fixed set of CSS-only pattern variants, not a canvas/SVG
  generative-art approach.** The spec's own Open questions defer the
  "actual pattern/color-generation algorithm's visual output" as
  implementation work informed by the token palette — resolved here as
  the simplest approach that still produces visually distinct,
  book-to-book-recognizable covers: the FNV-1a hash picks (a) a hue
  from a range that stays legible against `frontend-design-tokens.md`'s
  light palette, and (b) one of a small fixed set of background-pattern
  Tailwind-composable treatments (flat, diagonal-stripe, dot-grid) for
  the spine/texture layers. Kept CSS-only (no `<canvas>`, no SVG
  generation) so FR-4's performance budget is cheap by construction —
  a canvas-per-cover approach would need its own rasterization cost
  analysis this spec doesn't ask for.
- **D3 — `@playwright/test` added now, config scoped to this tier's
  own benchmark harness only.** A minimal `playwright.config.ts` at
  `web/` root pointed at a dedicated `e2e/` test directory (separate
  from Vitest's `src/**/*.test.tsx` glob, so the two runners never
  collide over the same files) — just enough to run FR-4's benchmark
  test in CI. Tier 4/5's own E2E smoke test and `@axe-core/playwright`
  stage extend this same config later; not duplicated here.

Recorded in the PR per constitution §9 for D3 (`@playwright/test`).
D1/D2 are implementation choices within the approved spec, not new
dependencies.

## Dependency graph

```
fnv1a.ts (hash) + seed derivation (hue/pattern-variant selection)
  └── TextureLayer, SpineLayer  (pure, seeded, independent of each other)
  └── TitleLayer, AuthorLayer   (pure, given already-reduced strings)
        └── GeneratedCover (composes all 4, implements the FR-3 ladder
            by choosing which layers to render based on title/author
            presence)
              └── memoization (wraps GeneratedCover's layer computation
                  in a session-scoped cache keyed by identifier)
                    └── Playwright benchmark harness (renders 500
                        memoized covers in a virtualized container,
                        last — depends on everything above existing)
```

## Task list

1. **`fnv1a.ts`** — pure FNV-1a hash function (`web/src/lib/fnv1a.ts`),
   plus a `deriveSeed(identifier)` helper producing the hue/pattern-
   variant values D2 needs. RED: a test asserting the same string
   input always produces the same numeric output, and two different
   inputs produce different outputs (no collision on the small fixed
   test set).
2. **`TextureLayer` + `SpineLayer`** — pure, presentational, seeded via
   `deriveSeed`'s output (a hue number + a pattern-variant enum), token-
   styled (Tailwind classes only, no raw hex/px — same FR-4 discipline
   Tier 2's `check-token-styling` already enforces and will catch here
   too). Test: same seed → identical rendered output (class list/style
   props), different seed → visibly different output.
3. **`TitleLayer` + `AuthorLayer`** — pure, presentational, Newsreader
   token font (title) per FR-1; wrap/truncate for overflow (title).
   Test: long title truncates without overflow; author layer renders
   the already-reduced display string as-is (no "et al." logic in this
   component — that reduction happens at the caller per the spec's own
   API contract).
4. **`GeneratedCover` — full composition + degradation ladder (FR-3)**.
   Props: `title?`, `author?`, `identifier` (required — the only thing
   always available), `className?`. Implements all three ladder steps
   by presence of `title`/`author`. RED: one test per ladder step
   asserting the expected layer set renders (and that steps 2/3 never
   render a blank box — always at least texture+spine).
5. **Determinism proof** — same `identifier` renders pixel-identical
   output across two independent renders and (via a snapshot-style
   comparison, not literal cross-session storage) simulated fresh
   mounts, proving FR-2 end to end through the composed component, not
   just the hash function in isolation (task 1 already proves the hash
   itself is deterministic; this proves the whole render pipeline is).
6. **Accessibility contract** — `alt=""`/`role="presentation"` on the
   generated visual, `aria-hidden` where appropriate; a test proving
   the component is never the sole accessible name for its book (e.g.
   rendered inside a card alongside a real heading, the card's
   accessible name comes from the heading, not the cover).
7. **Memoization** — session-scoped cache (a module-level `Map` keyed
   by identifier, cleared only on a full page reload — "session-cached"
   per FR-4, not `localStorage`/`sessionStorage`, since correctness
   depends on nothing outside the current page load and persistence
   across reloads isn't asked for). Test: computing the same
   identifier twice reuses the cached layer values (spy/count-based
   proof, not just output equality — output equality alone doesn't
   prove caching happened).
8. **Playwright benchmark harness (FR-4, last)** — `playwright.config.ts`
   (D3), a minimal virtualized-container test harness page/component
   rendering 500 `GeneratedCover`s, and a `@playwright/test` spec that
   samples `performance.now()`/paint timing during a scripted scroll,
   asserting the 100ms initial-paint and 60fps-implied frame-interval
   budgets. Wired into `ci.yml`'s `frontend` job as its own step.

## Checkpoint P4-D (exit criteria, unchanged from `tasks/todo-phase04.md`)

- Same identifier produces pixel-identical output across renders/
  sessions
- All 3 degradation steps render without a blank box or crash
- Benchmark meets FR-4's budget, proven in CI
- Cover is never the sole accessible name for its book

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| FR-4's exact numbers (100ms/500 covers/60fps) are reasoned placeholders per the spec's own Open questions | Medium — may need revision once phase 06 has a real grid | Spec already defers confirm-or-replace to phase 06; not re-litigated here |
| D2's CSS-only texture/spine approach might read as visually thin once compared against `.design-reference`'s own aesthetic | Low-medium | Flagged for a design-review pass (spec's own Test strategy names this as a manual/visual check, not a unit test) before Checkpoint P4-D closes |
| First tier to add `@playwright/test` — config could conflict with Vitest's own file discovery | Low | D3 scopes Playwright to a separate `e2e/` directory explicitly to avoid overlap; proven by running both `npm test` and the new Playwright script in the same CI job without collision |

## Open items carried forward (not this tier's to resolve)

- FR-4's numeric budgets — confirm-or-replace once phase 06 builds a
  real library grid (spec's own Open questions).
- `text-3` WCAG AA contrast exception (Tier 1 leftover, unrelated to
  this tier's own texture/spine hue choices — D2's hue range is picked
  to stay legible independent of that exception).
