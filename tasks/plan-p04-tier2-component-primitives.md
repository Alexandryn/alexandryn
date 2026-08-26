# Phase 04, Tier 2 — Component primitives — implementation plan

Full phase context: [`tasks/plan-phase04.md`](plan-phase04.md). Task list:
[`tasks/todo-p04-tier2-component-primitives.md`](todo-p04-tier2-component-primitives.md).
Spec: [`frontend-component-primitives.md`](../.claude/specs/frontend-component-primitives.md)
(`APPROVED`). Governing: `architecture-frontend.md` FR-5.

## Context

Tier 0 (bootstrap) and Tier 1 (design tokens) are merged to `main`. Tier 2
builds the 17 primitives `frontend-component-primitives.md` FR-1 classifies:
6 Radix-wrapped, 10 hand-built, 1 forced-hand-built (`DataTable`).
`web/src/tokens.css`/`theme.css` (Tier 1) already expose the full Tailwind
`@theme` token set (`bg-background`, `text-text`/`text-2`/`text-3`,
`border-border`, `bg-accent`/`text-accent`, `bg-success`/`bg-error`,
`rounded-*`, `text-*` sizes, `shadow-sm`/`shadow-lg`). No focus-ring-specific
token exists separately — the design reference's own
`:focus-visible{outline:2px solid var(--ac)}` rule means `--color-accent`
*is* the focus-ring color (FR-3 "focus-ring token" resolves to this).

Design-conformance re-check (2026-08-26, this session): all 4
`.dc.html` canvases re-pulled via `DesignSync` against project
`78075626-e444-438f-8437-205d57129a37`, byte-identical to
`.design-reference/` local cache. No new "Design system" canvas surfaced.
No drift since the 2026-08-13/2026-08-26 syncs `plan-phase04.md` already
recorded — this tier's scope is unaffected.

`web/package.json` currently has no Radix packages, no `axe`-family
package, and no class-list helper (`clsx`/`cva` or equivalent). Three
implementation decisions follow from that gap — none reopen anything the
spec already fixed; each is either a spec-approved dependency's concrete
package name, or a choice to *avoid* a new dependency in favor of a few
owned lines, per constitution §9's "small amount of code you own over a
large amount you don't."

## Decisions

- **P1 — Radix package names.** FR-1 already decides *that* Radix is
  used for six named primitives; the concrete npm packages are
  `@radix-ui/react-dialog` (Modal), `@radix-ui/react-switch` (Toggle),
  `@radix-ui/react-radio-group` (SegmentedControl), `@radix-ui/react-slider`
  (Slider), `@radix-ui/react-toast` (Toast), `@radix-ui/react-visually-hidden`
  (VisuallyHidden) — the standard per-primitive scoped packages, not a
  bundled meta-package, so each primitive's dependency footprint is
  independently visible (matches FR-1's own "exit cost is bounded to the
  six Radix-wrapped primitives" argument — a bundled package would muddy
  that boundary). Recorded in the PR per constitution §9.
- **P2 — no `clsx`/`cva`.** Conditional class composition is a ~10-line
  hand-written `cx()` helper (`web/src/lib/cx.ts`) and a plain
  `Record<Variant, string>` lookup per primitive, not a new dependency —
  both `clsx` and `class-variance-authority` solve a problem this project's
  own token/variant surface is small enough not to need. Avoids two new
  dependencies neither spec names.
- **P3 — component-level `axe` checks now, without pulling in a browser
  driver.** `frontend-accessibility.md` FR-4 fixes `axe-core` +
  `@axe-core/playwright` + `@playwright/test` as Tier 5's own *CI-stage*
  dependency set, run against real rendered pages — that's Tier 5's job
  (F24), not built here. But this spec's own Test strategy table requires
  "automated `axe` checks per primitive" *now*, and Checkpoint P4-C's exit
  criterion is "zero `axe-core` violations" for this tier specifically —
  waiting for Tier 5 would leave the checkpoint unverifiable. Resolution:
  add `axe-core` itself (already a spec-named, approved dependency — only
  its *driver* differs by layer) plus a ~15-line Vitest helper
  (`web/src/test/axe.ts`) that calls `axe.run()` directly against an RTL-
  rendered container in jsdom, rather than adding `jest-axe`/`vitest-axe`
  as a second wrapper dependency. Tier 5 layers real-browser E2E scanning
  on top of this later; this doesn't preempt or duplicate that stage, it
  covers the per-component layer the spec's Test strategy table names
  separately from Tier 5's dedicated CI stage.

All three recorded in the PR body per constitution §9 (what it does, why
not stdlib/hand-rolled, what breaks if abandoned).

## Dependency graph

`VisuallyHidden` and the `cx()`/token-class-lookup helper have no
dependents among the 17 and are needed by several others (`Chip`,
`StatusPill`, icon-only `Button` variants, `FormatBadge` all use visually-
hidden text for icon-only or abbreviated labels) — built first.
`StatusPill` before `EmptyState` (EmptyState composes a `StatusPill`-
adjacent message per FR-1) and before `Button` is available for
`EmptyState`'s optional action slot — `EmptyState` goes last among the
hand-built group. `DataTable` has no dependents and the most surface area
— goes last overall. Everything else has no cross-primitive dependency and
can build in any order; grouped below by spec category to match F10-F13's
own task numbering, not because of a hard dependency.

```
cx() helper + axe test helper (infra)
  └── VisuallyHidden (Radix-wrapped)
        ├── Chip, StatusPill, FormatBadge, Button, Input, ProgressBar,
        │   Spinner, Skeleton, StatCard   (hand-built, independent of each other)
        │     └── EmptyState (composes StatusPill-shaped message + Button)
        ├── Toggle, SegmentedControl, Slider, Toast, Modal   (Radix-wrapped, independent of each other)
        └── DataTable (hand-built, largest surface — last)
```

## Task list (one task = one primitive or the stated small group)

Each task is RED → GREEN → Refactor: write the state-matrix + a11y-contract
+ token-only-styling tests and Storybook story first (failing/absent
component), then implement. Per task, "done" means:

- Every required state renders and is asserted (FR-2: default/hover/focus/
  disabled, +error if it accepts input)
- Category a11y contract met (FR-3) and proven by a keyboard-only test
  (`user-event`'s `.tab()`/`.keyboard()`, never `.click()`/`.hover()` for
  the operability assertion) plus zero violations from the `axe.ts` helper
- Zero raw hex/px in the component's source (spot-checked per task; the
  grep-based sweep is Checkpoint P4-C's own final gate, not repeated per
  task)
- `prefers-reduced-motion`/`prefers-contrast` respected where the
  primitive has any transition or subtle-contrast decorative element
  (FR-5) — most hand-built primitives have neither and are exempt by
  inspection; Modal/Toast/Skeleton/Spinner do and are tested under both
  media-query states
- Storybook story covering the state matrix

1. **Infra** — `cx()` helper, `axe.ts` Vitest helper, Radix + nothing-else
   installed (P1/P2/P3). No component yet; a trivial smoke test proves
   `axe.ts` catches a deliberately-broken fixture (an unlabelled `<input>`)
   before any real primitive relies on it — this is the RED for the
   helper itself.
2. **VisuallyHidden** (Radix-wrapped) — thin wrap, no visual states beyond
   presence; a11y contract: content is in the accessibility tree, never
   visually rendered, never `display:none`/manual clip-rect (grep-checked
   against this one file as the canonical usage the rest of Tier 5's F23
   convention check relies on).
3. **Button** — variant × size × tone state matrix incl. icon-only (uses
   `VisuallyHidden` for the accessible name), disabled, focus-visible ring.
4. **Input** — default/hover/focus/disabled/error, associated `<label>`,
   `aria-invalid`+`aria-describedby` on error.
5. **ProgressBar** — native `<progress>`-shaped semantics, indeterminate
   state, `aria-valuenow`/`aria-valuemin`/`aria-valuemax`.
6. **StatusPill** — tone variants (success/warning/error/neutral via the
   token set), text alternative for color-only meaning (never color-only).
7. **FormatBadge** — small fixed-vocabulary badge (EPUB/PDF/etc.), same
   token-tone pattern as StatusPill, no color-only meaning.
8. **Chip** — default/hover/focus/disabled + optional remove action
   (keyboard-operable, accessible name for the remove control).
9. **Spinner** — `aria-live`/`role="status"` region (FR-3), respects
   `prefers-reduced-motion` (freezes/simplifies the spin animation).
10. **Skeleton** — `aria-live`/`role="status"` announcing the loading
    state once, respects `prefers-reduced-motion` (shimmer animation).
11. **StatCard** — presentational composition, no interactive state beyond
    default/hover if it's ever a link/button variant; accessible name via
    heading/label structure.
12. **Toggle** (Radix `Switch`) — checked/unchecked/disabled, keyboard
    Space/Enter, `aria-checked`.
13. **SegmentedControl** (Radix `RadioGroup`, styled as segments) —
    roving-tabindex arrow-key navigation, `aria-checked`/`role="radio"`
    per segment.
14. **Slider** (Radix `Slider`) — keyboard arrow-key value adjustment,
    `aria-valuenow`/min/max, disabled state.
15. **Toast** (Radix `Toast`) — appear/auto-dismiss/manually-dismissed
    state machine, `aria-live` region management (Radix-provided),
    `prefers-reduced-motion` on the enter/exit transition.
16. **Modal** (Radix `Dialog`) — focus trap, focus-return-on-close,
    `Escape` closes, background `aria-hidden` while open,
    `prefers-reduced-motion` on the open/close transition.
17. **EmptyState** — composes an icon/illustration slot + `StatusPill`-
    adjacent message + optional `Button`; depends on task 3 and 6.
18. **DataTable** — `<table>`/`<th scope>` semantics, keyboard-operable
    column sort (`Enter`/`Space` on the sort header, not a div-grid),
    `aria-selected` row state if selection is present. Largest task;
    split into its own RED/GREEN pass per sub-behavior (render → sort →
    select) rather than one monolithic test file.

## Checkpoint P4-C (exit criteria, unchanged from `tasks/todo-phase04.md`)

- 17/17 primitives exist, classified per FR-1, none left unclassified
- Zero `axe-core` violations (via task 1's helper, run per primitive)
- Fully keyboard-operable — proven by tests that never use a pointer event
  for the operability assertion
- Zero raw hex/px outside the token set — grep-checked sweep across
  `web/src/components/**` as the final gate (interim measure per FR-4/spec
  Open questions, until a lint rule exists)
- Storybook story per primitive, covering its state matrix

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Radix's own default styling/CSS resets could leak an unstyled visual flash before Tailwind classes apply | Low — Radix ships unstyled by design | Each wrapper applies token classes directly to Radix's own components via `asChild`/class props, verified visually in Storybook per task |
| `axe-core`-in-jsdom (task 1's helper) catches less than a real-browser scan (no real layout, no real computed contrast in some cases) | Medium — Tier 5's `@axe-core/playwright` E2E stage is the real backstop | Documented in P3 above; Checkpoint P4-C's "zero violations" claim is scoped to what jsdom-driven `axe-core` can check, not a substitute for Tier 5's later real-browser pass |
| `DataTable`'s scope (task 18) is large enough to slip a tier-2 estimate | Medium | Split into render/sort/select sub-passes per the task's own note; flagged here rather than discovered mid-build |

## Open items carried forward (not this tier's to resolve)

- `text-3` WCAG AA contrast exception — documented in
  `check-token-contrast.ts`'s output, needs a maintainer decision (Tier 1
  leftover).
- No dark-mode toggle — tokens are `var()`-based and dark-ready by
  construction, but no `@media (prefers-color-scheme: dark)` values exist
  yet (Tier 4's problem per the kickoff note, not this tier's).
