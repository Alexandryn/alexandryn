# Spec: Frontend generated cover system

| | |
|---|---|
| **Status** | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `04-frontend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0031`](../reviews/0031-phase04-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`roadmap/04-frontend-foundation/README.md` names "the generated
placeholder cover system — a procedural, layered component (texture,
spine, title, author) with a degradation ladder for when a real cover
image never arrives" as in-scope, and names its own risk: "Generated-
cover system becomes a performance sink at library-scale grids." Its own
Test strategy section names this the hardest thing to test in the whole
phase — "it has to look intentional at every step, not just avoid
crashing, and that is closer to a design review than a unit test."
Metadata (a real cover URL, when one exists) is phase 07's concern; this
spec covers what renders when one doesn't.

## Problem

Nothing has fixed: the layer composition (what texture, spine, title,
author actually mean as renderable layers), how a cover is seeded
deterministically (the same book always gets the same generated look),
the degradation ladder's actual steps, or the performance budget at
library-grid scale (hundreds of covers rendered at once).

## Goals

- Fix the layer composition and what data each layer needs
- Fix deterministic seeding — same book, same generated cover, every
  render, every session
- Fix the degradation ladder: what happens as available data shrinks
  (title only, no author; no title either; genuinely nothing)
- Fix a real performance budget for a large grid and how it's met
  (memoization, caching, or a cheaper rendering approach)

## Non-goals

- Real cover images or fetching them — phase 07 (metadata)
- The library-grid layout that arranges many covers — phase 06
- Any specific book's actual generated appearance — this spec fixes the
  *system*, not example output

## User stories

- As **a user with a book that has no cover art**, I want a placeholder
  that looks intentional and consistent, not a broken-image icon or a
  blank rectangle.
- As **a user browsing a large library**, I want the grid to stay smooth
  even when most covers are generated, not fetched.
- As **phase 07**, I want a defined fallback path so a metadata fetch
  failure or a source with no cover art degrades gracefully into this
  system rather than crashing the book's render.

## Functional requirements

- **FR-1** A generated cover is composed of four layers, each a pure
  function of the book's own data: **texture** (a background pattern,
  seeded — FR-2), **spine** (a colored bar/accent, seeded), **title**
  (rendered text, `frontend-design-tokens.md`'s Newsreader token,
  wrapped and truncated to fit), **author** (rendered text, smaller,
  below title — for a `Work` with more than one `Author` reference
  per `domain-bibliographic.md`, rendered as the first author's name
  followed by "et al." when more than one exists, never a concatenated
  list that could overflow the layout; the component receives this
  already-reduced display string as its `author` prop, never a raw
  `Author[]` array it would have to know how to reduce itself — keeping
  this component decoupled from the domain array shape, consistent with
  the API and contracts section below). No layer fetches external data
  or renders anything not derivable from the book's own
  title/author/identifier already in hand.
- **FR-2** Seeding is **deterministic**, derived from **FNV-1a** (a
  small, well-known, non-cryptographic string hash — chosen specifically
  because it's boring and has no external dependency, just a few lines
  of arithmetic) applied to the book's own identifier (`Work.ID` or
  `Edition.ID`, whichever the calling context has —
  `domain-model-and-boundaries` skill's aggregate list) — never
  `Math.random()` or any non-deterministic source, and never a hash
  choice left open to per-implementation variation, since two different
  valid hash functions would defeat FR-2's own "recognizes it again"
  user story by producing two different appearances for the same book
  depending on which one happened to get implemented. The same book
  produces the identical texture pattern and spine color every render,
  every session, every device — a user who's seen a book's generated
  cover once recognizes it again, and two users viewing the same shared
  library see the same generated appearance, not two different random
  ones.
- **FR-3** The degradation ladder, in order of decreasing available
  data:
  1. Title + author present → full four-layer composition (FR-1)
  2. Title present, author absent → texture + spine + title only, title
     layout re-centered to use the space author would have occupied
  3. Neither present (an edge case — `domain-bibliographic.md` doesn't
     state an explicit non-empty-title invariant as its own FR, only
     that title/subtitle are modeled fields with subtitle marked
     optional and title not, an inference by omission rather than a
     written guarantee; this step exists for defensive completeness
     against that gap, not an expected real case) → texture + spine
     only, no text layer, visually still a "cover shape," never a blank
     box
  Each step MUST look like a deliberate design choice, not a broken
  degradation — this is `roadmap/04-frontend-foundation/README.md`'s own
  named hardest-to-test property, addressed here by fixing exactly what
  renders at each step rather than leaving "graceful" undefined.
- **FR-4** Performance: a grid of 500 generated covers renders its
  initial viewport within **100ms** of layout data being available, and
  scrolling stays at 60fps — met via memoization (a cover's layers are
  computed once per book identifier and cached for the session, per
  FR-2's determinism making this cache trivially correct — the same
  input always produces the same output, so caching introduces no
  staleness risk). This spec owns a minimal, self-contained benchmark
  harness (a bare virtualized container — not the real library-grid
  layout, which stays phase 06's own concern per Non-goals — built and
  measured entirely within this spec's own test suite) to prove a
  single cover's render cost is cheap enough that virtualization is
  sufficient at scale, independent of whatever grid component phase 06
  eventually builds around it. **Measurement mechanism**: a Playwright
  test (`frontend-tooling.md` FR-7's Vitest doesn't run in a real
  browser, so a browser-level benchmark needs `@playwright/test`
  directly, the same real CI dependency `frontend-shell-and-routing.md`
  and `frontend-accessibility.md` use) renders the benchmark harness,
  samples `performance.now()` timestamps across
  `requestAnimationFrame` callbacks during a scripted scroll, and
  asserts both the initial-paint duration (via the Performance API's
  `paint` timing entries) and the frame-interval distribution implied
  by the RAF samples — a concrete, CI-runnable mechanism, not an
  asserted number with no way to check it.

## Non-functional requirements

- **Performance** — FR-4 is this spec's own concrete budget, the direct
  answer to phase 04's own named risk.
- **Security** — see Security considerations below.
- **Accessibility** — the generated cover is decorative once real title/
  author text exists elsewhere on the book's card/detail view (e.g. as
  actual text alongside the cover); the cover component itself carries
  `alt=""` or `role="presentation"` plus an `aria-label` on the
  containing element naming the book, never relying on the generated
  text layer (FR-1) as the only accessible name — a screen-reader user
  gets the real title/author from the surrounding component, not by
  the cover system attempting to expose its own rendered pixels as
  meaningful.
- **Reliability** — FR-3's ladder guarantees a cover always renders
  something intentional, never a crash or a blank box, regardless of
  how little data is available.
- **Observability** — not applicable at this layer.

## Domain model

Not applicable directly — consumes `Work`/`Edition` fields
(title, author, identifier) already fixed by
`domain-bibliographic.md`; introduces no new domain concept.

## API and contracts

- **Cover component ↔ domain data**: takes a book's title, author, and
  identifier as props — never fetches its own data, never receives a
  full `Work`/`Edition` object it would have to know how to parse
  (keeps this component decoupled from domain-object shape changes).
- **Cover component ↔ real cover images (future, phase 07)**: this
  system is the fallback path a `<CoverImage>`-shaped component (phase
  07's concern) falls back to on load failure or absence — the contract
  is "renders standalone given title/author/identifier," letting phase
  07 wire the fallback without this spec needing to anticipate phase
  07's own fetch-error handling shape.

## State transitions

Not applicable — a generated cover is a pure render of its inputs
(FR-1/FR-2), no internal state machine.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Title too long for the layout | FR-1's title layer's own wrap/truncate logic | Truncated title, never overflow or a broken layout | Truncation is part of FR-1's requirement, not a bug to fix later |
| A book identifier changes (unlikely, but `domain-bibliographic.md`'s merge operation could reassign one) | FR-2's seed is identifier-derived | The generated cover changes appearance | Accepted consequence of FR-2's determinism — a real identifier change means the previously "same" book is now keyed differently; not treated as a defect |
| Large grid renders slowly | FR-4's performance test | Janky scrolling, slow initial paint | Caught in phase 04's own test suite before merge, not discovered at library-scale in production |

## Security considerations

- **No external data, no network call, no user-supplied styling** — the
  generated cover's only inputs are already-validated domain fields
  (title, author, identifier); there's no path for a hostile source or
  a malicious file to influence anything beyond the text that gets
  rendered, which React's default escaping already covers (same
  reasoning `frontend-component-primitives.md`'s Security considerations
  gives generally).

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Seeding determinism (FR-2) — same identifier always produces the same layer values; degradation ladder (FR-3) — each of the three steps renders the expected layer set given the corresponding input |
| Visual/manual | Each degradation step "looks intentional" — phase 04's own named hardest-to-test property; a design review pass, not a unit test, checking the actual rendered output against `frontend-design-tokens.md`'s visual language |
| Performance | FR-4's self-contained benchmark harness (own virtualized container, not phase 06's real grid), measured via `@playwright/test` and the Performance API as described in FR-4, checked in CI as a regression gate |

## Acceptance criteria

- [ ] Given the same book identifier, the generated cover is pixel-
      identical across repeated renders and across sessions
- [ ] All three degradation steps (FR-3) render without a blank box or
      crash, proven with a test per step
- [ ] A 500-cover grid meets FR-4's performance budget, proven with a
      benchmark test in CI
- [ ] The cover component never renders as the sole accessible name for
      its book — a screen-reader test confirms the real title/author is
      exposed by the surrounding component, not the cover itself

## Open questions

- **FR-4's exact numbers (100ms, 500 covers, 60fps)** — reasoned
  placeholders, not measured against real hardware or a real library
  size; confirm or replace once phase 06 provides a real grid to
  benchmark against.
- **Texture/spine visual design specifics** — this spec fixes the layer
  *system*, not the actual pattern/color-generation algorithm's visual
  output, which is implementation work informed by
  `frontend-design-tokens.md`'s palette once extracted.

## References

- `roadmap/04-frontend-foundation/README.md` — the generated-cover
  requirement, its own named performance risk and hardest-to-test note
- `domain-bibliographic.md` — `Work`/`Edition` fields this system
  consumes, title invariant referenced in FR-3
- `frontend-design-tokens.md` — typography tokens (Newsreader) FR-1's
  title layer uses
- `frontend-component-primitives.md` — Security considerations pattern
  this spec's own section restates for generated content specifically
- Constitution §7 (accessibility — the alt-text/labeling requirement)
