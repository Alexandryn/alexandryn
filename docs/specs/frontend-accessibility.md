# Spec: Frontend accessibility

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-28, phase 04 Tier 6 / F27 — implemented Tier 5 (PR #64), audited `0004`, acceptance criteria walked in [`roadmap/04`](../roadmap/04-frontend-foundation/README.md#spec-verification); phase 17 remains the full WCAG conformance sweep) — was `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-14) |
| **Phase** | `04-frontend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | `0031` (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

Constitution §7 requires accessibility as part of "done," not a later
pass. `architecture-frontend.md` FR-5 already made this structural at
the architecture level; `frontend-component-primitives.md` FR-3/FR-5
already fixed per-primitive contracts and motion/contrast preferences.
This spec is where the cross-cutting rules (keyboard map, focus order,
screen-reader text conventions) get fixed once, so every phase 04 spec
and every screen phase 06+ builds inherits them instead of each
reinventing its own.

## Problem

Nothing has fixed: a project-wide keyboard interaction map (what Tab,
Shift+Tab, Arrow keys, Escape, Enter/Space each do across the app, not
just per primitive), a screen-reader text convention (how a visually-
hidden label is written, when one is needed), the automated testing
tool and where it runs in CI, or how `frontend-generated-covers.md`'s
own accessibility requirement (Non-goals notwithstanding — that spec
references this one) gets verified.

## Goals

- Fix a project-wide keyboard map, consistent across every primitive and
  screen
- Fix focus-order rules: what "logical order" means for this app's
  layouts (shell + content pane composition)
- Fix screen-reader text conventions: visually-hidden label pattern,
  when alt text vs. `aria-label` vs. visible text is the right choice
- Fix the automated accessibility testing tool and its place in CI,
  distinct from `frontend-tooling.md` FR-5's lint-time check

## Non-goals

- Per-primitive accessibility contracts — already fixed in
  `frontend-component-primitives.md` FR-3; this spec is the project-wide
  rules those contracts already assume
- WCAG conformance auditing at scale — that's phase 17's own dedicated
  phase (`roadmap/17-accessibility-and-qa/`); this spec sets the
  baseline every earlier phase builds correctly from, not the final
  conformance sweep
- Screen-reader-specific testing across every AT/browser combination —
  phase 17's matrix; this phase's automated checks (axe) catch
  structural issues, not exhaustive AT compatibility

## User stories

- As **a keyboard-only user**, I want the same key to do the same thing
  everywhere in the app — Escape always closes/cancels, Tab always moves
  forward in a predictable order — not a different convention per
  screen.
- As **a screen-reader user**, I want every visually-hidden label
  written to the same convention, so the experience is consistent
  whether I'm on a button, a data table, or a form.
- As **a developer**, I want an automated check that fails CI on a
  structural accessibility regression, so this doesn't rely on someone
  remembering to test with a keyboard before every merge.

## Functional requirements

- **FR-1** Project-wide keyboard map:
  - **Tab / Shift+Tab** — move focus forward/backward through
    interactive elements in DOM order (FR-2 fixes what DOM order means
    for this app's layouts)
  - **Enter / Space** — activate the focused control (buttons, toggles);
    Space additionally scrolls when focus is on a non-control scrollable
    region, standard browser behavior, never overridden
  - **Arrow keys** — move focus within a composite control: Radix's own
    roving-tabindex behavior for `SegmentedControl`/`Slider`/`Toggle`
    (`frontend-component-primitives.md` FR-1's Radix-wrapped bucket),
    and a hand-implemented roving-tabindex pattern for `DataTable` cell
    navigation specifically, since Radix ships no table primitive for
    that spec's FR-1 to wrap (same source, corrected citation — the
    behavior is consistent across both, the mechanism underneath isn't
    the same library for both); never repurposed for page-level
    navigation, which would conflict with a screen reader's own
    arrow-key browsing mode
  - **Escape** — closes/cancels the topmost open transient UI (a modal,
    a dropdown, a toast) — consistent across every primitive that has
    one, never a per-component override
  - No global keyboard shortcut (a single-letter hotkey, e.g. `g` then
    `l` for "go to library") is introduced in phase 04 — reserved as a
    future enhancement once there's a real feature set to attach
    shortcuts to; premature now and a real risk of colliding with
    assistive technology's own shortcuts if designed without that
    context.
- **FR-2** Focus order follows visual/DOM order matching the shell's own
  composition (`frontend-shell-and-routing.md` FR-3): sidebar, then
  titlebar, then content pane, top to bottom within each — never a
  `tabindex` value used to reorder focus out of DOM order (a
  `tabindex="-1"` to deliberately remove an element from the tab
  sequence, e.g. a decorative element, is fine; a positive `tabindex`
  reordering focus is not, since it creates exactly the inconsistent,
  hard-to-reason-about order this FR exists to prevent).
- **FR-3** Screen-reader text convention: a visually-hidden label uses a
  shared `<VisuallyHidden>` primitive (`frontend-component-primitives.md`
  FR-1 — wrapping Radix's own implementation not for interaction
  complexity, which this primitive has none of, but because it's a
  well-known, easy-to-get-subtly-wrong CSS pattern Radix already ships
  correctly, a trivial-dependency-cost exception to that spec's usual
  interaction-complexity criterion) — never `display: none`
  (removes from the accessibility tree entirely) or a manually-written
  clip-rect CSS hack duplicated per component. Choice of mechanism, per
  case: **visible text** is preferred by default; `aria-label` is used
  only when visible text would be redundant or absent (an icon-only
  button); `alt` text is used only for `<img>` elements specifically
  (the generated-cover system's own `alt=""`/`aria-label` split,
  `frontend-generated-covers.md`'s Accessibility section, follows this
  same convention).
- **FR-4** Automated testing tool is **`axe-core`** via
  `@axe-core/playwright`, pinned version, running atop
  **`@playwright/test`** — a real CI dependency, not the Playwright
  MCP: `architecture-testing.md` FR-2 already fixed that the MCP is
  interactive-only, scoped to a Claude Code session, and cannot run
  unattended in CI (the exact distinction `architecture-desktop-host.md`'s
  own self-review had to correct once already, so this spec doesn't
  repeat the mistake). `axe-core` is a genuinely new dependency, not a
  reuse of anything `architecture-frontend.md` already committed to —
  justified under constitution §9 on its own terms: it's a WCAG
  rule-engine (computed contrast, ARIA-validity rules, violation
  scanning) that Playwright's own accessibility-snapshot capability
  doesn't provide, and hand-rolling contrast computation or ARIA
  validation would be reinventing a well-maintained, industry-standard
  tool. If abandoned, exit cost is low: it's a test-only dependency
  behind one CI step, not embedded in application code, so replacing it
  means swapping the CI step, not touching any component. Runs as its
  own CI stage, distinct from `frontend-tooling.md` FR-5's static
  `eslint-plugin-jsx-a11y` lint check — lint catches structural issues
  at write-time; `axe` catches runtime-rendered issues (computed
  contrast, actual focus order, ARIA state as rendered) lint can't see.
  Both are required, neither substitutes for the other.
- **FR-5** `prefers-reduced-motion` and `prefers-contrast` support
  (`frontend-component-primitives.md` FR-5's per-primitive requirement)
  is verified at this spec's own layer by rendering the full shell
  under both media-query states in the E2E smoke test
  (`frontend-shell-and-routing.md`'s own test) and confirming no
  animation plays / the higher-contrast variant renders — the
  cross-cutting proof that individual primitives' own compliance
  actually composes correctly at the app level, not just in isolation.

## Non-functional requirements

- **Performance** — `axe-core`'s CI run (FR-4) adds to pipeline duration;
  no specific budget set here, since `architecture-testing.md`'s own
  Open questions already leave overall CI duration unbudgeted generally.
- **Security** — not applicable.
- **Accessibility** — this spec's entire subject; constitution §7 is the
  bar every FR above is checked against.
- **Reliability** — FR-4's CI-gated automated check is what keeps
  accessibility from silently regressing as new components/screens are
  added, rather than relying on a periodic manual audit to catch drift.
- **Observability** — `axe` violation reports (FR-4) are captured CI
  output, the same treatment `frontend-tooling.md`'s bundle-size check
  gets.

## Domain model

Not applicable — accessibility conventions, not the Alexandryn domain.

## API and contracts

- **Every primitive/screen ↔ `<VisuallyHidden>` (FR-3)**: the one shared
  mechanism for screen-reader-only text; no component implements its
  own visually-hidden CSS.
- **CI ↔ `axe-core` (FR-4)**: a dedicated stage, output captured, blocks
  merge on any violation — same gating discipline
  `architecture-testing.md` FR-1 requires generally.

## State transitions

Not applicable.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| A new component breaks focus order (FR-2) | `axe-core`'s CI run (FR-4), or manual keyboard testing | Focus jumps unpredictably | CI fails, merge blocked |
| A component uses `display: none` for hidden text instead of `<VisuallyHidden>` (FR-3) | Code review; `axe-core` may not catch this specific pattern reliably, since the text is correctly absent from a sighted user's view either way — the defect is that it's *also* absent from the accessibility tree, which requires checking the rendered accessibility tree specifically, not just visual output | Screen-reader user gets no announcement where one was intended | Caught in review per `general/code-review` skill's accessibility dimension, or a future `axe` rule tuned to catch this specific pattern |
| `axe-core` CI stage flakes or times out | CI's own failure reporting | A red check that isn't a real regression | Treated as a defect in the check itself (`architecture-testing.md` FR-6's determinism requirement), fixed rather than retried until green |

## Security considerations

Not applicable — no trust boundary in accessibility conventions
themselves.

## Test strategy

| Layer | What it covers |
|---|---|
| Automated (axe) | Every primitive and every shell layout, per FR-4 — contrast, ARIA validity, focus order as rendered |
| Manual keyboard pass | The "open a book" reference walkthrough (`architecture-system.md`'s own named slice), done with no pointer device, as a periodic sanity check beyond what automation catches |
| E2E | `frontend-shell-and-routing.md`'s own `@playwright/test` smoke test, extended with `@axe-core/playwright`'s violation-scan assertions (FR-4) |

## Acceptance criteria

- [ ] Zero `axe-core` violations across every phase 04 primitive and
      shell layout
- [ ] The full keyboard map (FR-1) is documented and matches actual
      behavior, proven by a test exercising each key across at least one
      representative primitive per category
- [ ] Focus order matches DOM/visual order (FR-2) with no positive
      `tabindex` anywhere in the codebase, proven by a grep-based check
- [ ] Every visually-hidden label uses `<VisuallyHidden>` (FR-3), proven
      by a grep-based check for `display: none`/manual clip-rect
      patterns applied to text content
- [ ] The reduced-motion/high-contrast composed check (FR-5) passes

## Open questions

- **Global keyboard shortcuts** — deliberately deferred (FR-1); revisit
  once phase 06+ features exist to attach them to, with real assistive-
  technology collision research at that point, not guessed now.
- **The specific `axe-core` rule set/severity threshold** — "zero
  violations" is the acceptance bar (Acceptance criteria), but whether
  every `axe` rule category is enabled at its default severity or tuned
  is an implementation detail not fixed here.

## References

- `architecture-frontend.md` FR-5 — the structural accessibility
  requirement this spec's cross-cutting rules serve
- `frontend-component-primitives.md` FR-3/FR-5 — the per-primitive
  contracts this spec's FR-1/FR-3 provide the shared conventions for
- `frontend-shell-and-routing.md` FR-3 — the shell composition FR-2's
  focus order follows
- `frontend-generated-covers.md` — Accessibility section, the
  alt/aria-label split this spec's FR-3 formalizes as the general rule
- `roadmap/17-accessibility-and-qa/` — the later conformance-audit
  phase this spec is explicitly not substituting for
- `architecture-testing.md` FR-2 — the Playwright MCP's interactive-
  only, not-CI-capable scope, the reason FR-4 uses `@playwright/test`/
  `@axe-core/playwright` as real CI dependencies instead
- `architecture-desktop-host.md` — self-review precedent for the same
  MCP-vs-`@playwright/test` distinction
- Constitution §7 (accessibility), §9 (dependencies — `axe-core`
  justified on its own terms, not as reuse of an existing capability)
