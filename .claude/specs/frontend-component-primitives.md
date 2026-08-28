# Spec: Frontend component primitives

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-28, phase 04 Tier 6 / F27 — implemented Tier 2 (PR #61), audited [`0004`](../audits/0004-phase04-frontend-foundation.md), acceptance criteria walked in [`roadmap/04`](../roadmap/04-frontend-foundation/README.md#spec-verification)) — was `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `04-frontend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0031`](../reviews/0031-phase04-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`roadmap/04-frontend-foundation/README.md` names the full primitive list
(buttons, inputs, segmented controls, status pills, format badges, chips,
toggles, sliders, progress bars, stat cards, data tables, modals,
skeletons, spinners, toasts) and names "hand-built primitives versus a
headless library (Radix or similar) underneath the design tokens" as an
open architecture decision. `architecture-frontend.md` FR-5 fixed that
every interactive element is keyboard-reachable with a visible focus
state and an accessible name, as a structural requirement per component.
`frontend-design-tokens.md` fixes what tokens exist; this spec fixes how
components consume them and what "done" means per primitive.

## Problem

Nothing has fixed: whether primitives are hand-built or wrap a headless
library, what states every primitive must support (default, hover,
focus, disabled, error — phase 04's own Test strategy table names these
four), the accessibility contract each primitive carries, or how a
primitive is composed from tokens without ad hoc per-screen overrides.

## Goals

- Decide hand-built vs. headless-library-underneath, justified under
  constitution §9
- Fix the state matrix every primitive MUST support
- Fix the accessibility contract per primitive category (what "keyboard
  operable, visible focus, accessible name" concretely means for a
  toggle vs. a data table vs. a modal)
- Fix how a primitive consumes `frontend-design-tokens.md`'s tokens
  exclusively, never a raw value

## Non-goals

- Design tokens' own values — `frontend-design-tokens.md`
- Any screen-specific composition of primitives — phase 06 onward
- The generated-cover component — `frontend-generated-covers.md`, a
  distinct, more complex primitive with its own spec
- Storybook's actual story content per primitive — `frontend-tooling.md`
  fixes that the preview environment exists; populating it is
  implementation work, not a requirement this spec enumerates per
  primitive

## User stories

- As **a screen author** (phase 06 onward), I want every primitive
  already accessible and token-driven, so building a screen means
  composing primitives, not re-solving focus management per component.
- As **a keyboard or screen-reader user**, I want every primitive usable
  without a mouse or sight from the moment it exists, not retrofitted.
- As **a contributor extending the primitive set later**, I want a fixed
  contract (states, a11y requirements, token-only styling) to follow, so
  a new primitive doesn't quietly skip a requirement an earlier one met.

## Functional requirements

- **FR-1** Primitives are **hand-built on top of Radix UI primitives**
  for the interaction-logic-heavy ones specifically — real focus-trap,
  roving-tabindex, or ARIA-state-machine complexity is the criterion,
  applied to all fifteen primitives this phase names plus one net-new
  one, with no primitive left unclassified:
  - **Radix-wrapped**: `Modal` (Radix `Dialog` — focus trap), `Toggle`
    (Radix `Switch` — ARIA state), `SegmentedControl` (Radix
    `RadioGroup`, styled as segments — roving-tabindex), `Slider`
    (Radix `Slider` — pointer/keyboard value state machine), `Toast`
    (Radix `Toast` — auto-dismiss timing plus `aria-live` region
    management is exactly the hard-to-get-right-by-hand complexity this
    criterion selects for), **`VisuallyHidden`** (Radix's own
    `VisuallyHidden` primitive — not chosen for interaction complexity,
    which it has none of, but because it's a well-known, easy-to-get-
    subtly-wrong CSS pattern Radix already ships correctly; trivial
    dependency cost, used by `frontend-accessibility.md` FR-3 as this
    project's one shared screen-reader-only-text mechanism).
  - **Hand-built, no headless dependency**: `Button`, `Input`,
    `ProgressBar` (native `<input>`/`<progress>`-shaped semantics need
    no ARIA state machine beyond what the element itself provides),
    `StatusPill`, `FormatBadge`, `Chip`, `Spinner`, `Skeleton`,
    `StatCard`, and **`EmptyState`** (a net-new primitive, composed
    entirely from an icon/illustration slot, `StatusPill`-adjacent
    message text, and an optional `Button` — no interaction-logic
    complexity of its own, so it belongs here rather than in the Radix
    bucket; required by `frontend-shell-and-routing.md` FR-8 for
    `architecture-frontend.md` FR-6's empty-state requirement).
  - **`DataTable`**: hand-built, **not** Radix-wrapped, despite its
    real complexity — Radix ships no table primitive, so this is a
    forced choice, not a judgment call between two options. Correct
    `<table>`/`<th scope>` semantics, keyboard-operable column sorting,
    and `aria-selected` row state (FR-3) are implemented directly
    against the semantic HTML table elements themselves.
  All Tailwind-styled per `frontend-design-tokens.md`'s tokens (FR-4).
  Justified under constitution §9: Radix is a well-maintained,
  widely-used, unstyled-by-design library — using it only where its
  actual value (correct ARIA/focus behavior, hard to get right by hand)
  applies, not as a blanket dependency for every primitive, keeps the
  dependency surface proportionate to what it actually buys. If Radix
  were abandoned, exit cost is bounded to the six Radix-wrapped
  primitives above (DataTable and the nine hand-built ones are
  unaffected) — each wraps a narrow slice of Radix's API behind this
  project's own component boundary (API and contracts, below), so a
  replacement would mean reimplementing six components' interaction
  logic, not rewriting every call site across the app.
- **FR-2** Every primitive supports, at minimum, the four states phase
  04's own Test strategy table names: **default**, **hover**,
  **focus**, **disabled** — plus **error** for any primitive that
  accepts user input (`Input`, `Toggle`, `Slider`, form-shaped
  controls). A primitive missing a required state for its category is
  incomplete, not "good enough for now."
- **FR-3** Accessibility contract, per primitive category:
  - **Buttons, toggles, sliders, segmented controls** (actionable):
    keyboard-operable via Tab/Enter/Space/Arrow keys as appropriate to
    the control type, visible focus ring using
    `frontend-design-tokens.md`'s focus-ring token (never the browser
    default, which the design reference's own visual language doesn't
    match, but never *removed* without a replacement either), an
    accessible name via visible label, `aria-label`, or
    `aria-labelledby` — never an icon-only control with no name.
  - **Data tables**: proper `<table>`/`<th scope>` semantics (never a
    div-grid styled to look like a table), sortable columns
    keyboard-operable, row selection (if present) announced via
    `aria-selected`.
  - **Modals**: focus trapped inside while open (Radix's `Dialog`
    handles this per FR-1), focus returned to the triggering element on
    close, `Escape` closes, background content marked `aria-hidden`
    while open.
  - **Skeletons, spinners, toasts** (non-interactive/transient):
    `aria-live` regions where the state change itself is the
    information (a toast appearing, a spinner replacing content) so a
    screen-reader user isn't silently missing a state transition a
    sighted user sees.
- **FR-4** Every primitive's styling comes exclusively from
  `frontend-design-tokens.md`'s Tailwind token classes — no inline
  style, no raw hex/px value, no primitive-local CSS file with its own
  color/spacing constants. This is checked in review against
  `roadmap/04-frontend-foundation/README.md`'s own named "styled ad hoc
  instead of tokenised" risk until a lint rule exists
  (`frontend-design-tokens.md`'s Open questions) — not yet one of
  `general/code-review`'s own listed dimensions.
- **FR-5** `prefers-reduced-motion` and `prefers-contrast` are respected
  at the primitive level, not per screen — any primitive with a
  transition/animation (toasts, modals, skeletons) reads
  `prefers-reduced-motion` and either skips or shortens its animation;
  any primitive with a subtle-contrast decorative element reads
  `prefers-contrast` and swaps to a higher-contrast variant.
  `roadmap/04-frontend-foundation/README.md`'s own accessibility-baseline
  scope names both explicitly.

## Non-functional requirements

- **Performance** — primitives contribute to `frontend-tooling.md`'s
  bundle budget; no per-primitive budget is set here, since the whole-
  bundle number is the actual constraint that matters.
- **Security** — see Security considerations below.
- **Accessibility** — FR-3/FR-5 are this spec's own core requirements;
  constitution §7 is the bar every primitive is checked against before
  merge, not after.
- **Reliability** — FR-2's state matrix is what keeps a primitive from
  shipping half-finished (a button with no visible disabled state,
  discovered only once a screen tries to use one).
- **Observability** — not applicable at this layer.

## Domain model

Not applicable — component primitives, not the Alexandryn domain. FR-4's
token-only-styling rule is this spec's own version of a boundary rule,
applied to styling rather than data.

## API and contracts

- **Primitive props ↔ token classes**: every visual prop (`variant`,
  `size`, `tone`) maps to a fixed set of token-class combinations, never
  an open-ended `className` override that lets a caller bypass FR-4
  (a narrow, explicit escape hatch for genuinely one-off cases may exist,
  but isn't the default path).
- **Primitive ↔ Radix (FR-1)**: Radix's own unstyled primitive
  components are wrapped, never re-exported directly — callers import
  Alexandryn's own primitive, not Radix's, so the styling/token
  boundary stays enforced at one point.

## State transitions

Not applicable at the primitive level generally; **Modal**'s open/close
transition and **Toast**'s appear/auto-dismiss/manually-dismissed
transition (FR-3) are the two real state machines this spec fixes
directly, via Radix's `Dialog` and `Toast` respectively (FR-1).

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| A primitive ships missing a required state (FR-2) | Code review, Storybook preview inspection | An unstyled or broken-looking disabled/error state on first real use | Caught before merge — not yet one of `general/code-review`'s own listed dimensions (`skills/README.md`'s bar for adding one is the same pattern recurring in review twice); until then, this table is the enforcement mechanism |
| A primitive uses a raw color/spacing value (FR-4) | Code review, eventual lint rule | Visual drift from the token set, invisible until compared side-by-side | Same review discipline as above |
| An icon-only button ships with no accessible name (FR-3) | `eslint-plugin-jsx-a11y` (`frontend-tooling.md` FR-5), or manual/automated `axe` check (`frontend-accessibility.md`) | A screen-reader user hears nothing meaningful for that control | Caught at lint or accessibility-test time, before merge |

## Security considerations

- **No component renders unescaped external content by default** —
  restates `architecture-frontend.md`'s own Security considerations
  (XSS via metadata or book content): any primitive that will eventually
  display source- or file-derived text (a future `BookTitle` or
  `FilenameLabel`-shaped primitive, once phase 06/07 need one) treats
  that text as untrusted from the start — React's own default escaping
  covers this as long as no primitive uses `dangerouslySetInnerHTML`,
  which none of the primitives named in this spec need.

## Test strategy

| Layer | What it covers |
|---|---|
| Component | Every primitive's state matrix (FR-2), rendered and asserted per state |
| Accessibility | Automated `axe` checks per primitive (`frontend-accessibility.md`'s tooling), keyboard-operability assertions per FR-3's category-specific contract |
| Visual | Storybook preview (`frontend-tooling.md`) as the manual-inspection surface for "does this look intentional," not a substitute for the automated checks above |

## Acceptance criteria

- [ ] Every primitive named in `roadmap/04-frontend-foundation/README.md`,
      plus `VisuallyHidden` (required by `frontend-accessibility.md`
      FR-3) and `EmptyState` (required by `frontend-shell-and-routing.md`
      FR-8), FR-1's two net-new additions, exists, implemented per
      FR-1's complete classification with no primitive left
      unclassified
- [ ] Every primitive passes an automated `axe` check with zero
      violations
- [ ] Every interactive primitive is fully keyboard-operable, proven by
      a test that never uses a pointer event
- [ ] No primitive's source contains a raw hex/px value outside the
      token set (FR-4), proven by a grep-based check as an interim
      measure until a lint rule exists
- [ ] `prefers-reduced-motion` and `prefers-contrast` are respected,
      proven by rendering under both media-query states in tests

## Open questions

- **The explicit `className` escape hatch (API and contracts)** — how
  narrow, and whether it should exist at all versus forcing every
  visual variation through a defined prop — not fixed here; a real
  tension between flexibility and FR-4's enforcement worth revisiting
  once real screens (phase 06+) reveal how often an escape hatch is
  actually needed.
- **A lint rule enforcing FR-4** — same status as
  `frontend-design-tokens.md`'s equivalent open question; interim
  enforcement is code review plus a grep-based CI check.

## References

- `roadmap/04-frontend-foundation/README.md` — the full primitive list,
  the four-state Test strategy requirement, the hand-built-vs-headless
  open decision this spec resolves
- `architecture-frontend.md` FR-5 — the accessibility structural
  requirement this spec's FR-3 makes concrete per category
- `frontend-design-tokens.md` — the token set FR-4 requires exclusive
  use of
- `.claude/skills/general/code-review` — accessibility is one of its
  listed dimensions (FR-3's enforcement point); "styled ad hoc instead
  of tokenised" is not yet one, tracked as `roadmap/04-frontend-foundation/README.md`'s
  own named risk instead
- Constitution §7 (accessibility), §9 (dependencies — Radix
  justification)
