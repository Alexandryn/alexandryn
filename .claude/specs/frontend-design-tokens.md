# Spec: Frontend design tokens

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-28, phase 04 Tier 6 / F27 — implemented Tier 1 (PR #60), audited [`0004`](../audits/0004-phase04-frontend-foundation.md), acceptance criteria walked in [`roadmap/04`](../roadmap/04-frontend-foundation/README.md#spec-verification); one maintainer-accepted `text-3` AA exception, D2) — was `APPROVED` (amended post-approval — second extraction output `tokens.css`, cross-phase review finding, self-reviewed, re-confirmed by maintainer 2026-08-14, see [`0032`](../reviews/0032-spec-amendments-phase05-cross-phase-findings.md)) |
| **Phase** | `04-frontend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0031`](../reviews/0031-phase04-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14. Amended post-approval, [`0032`](../reviews/0032-spec-amendments-phase05-cross-phase-findings.md) — added `tokens.css` as a second FR-2 output, closing a citation gap phase 05's cross-spec review found (both independent reviewers, same finding), self-reviewed, re-confirmed by maintainer 2026-08-14 |

## Context

`architecture-frontend.md` FR-4 fixed that design tokens (the CSS custom
properties already throughout the design reference's `.dc.html` files —
`--bg`, `--sf`, `--tx`, `--ac`, and others) get extracted once into
Tailwind's theme config, via the `frontend-design` skill, rather than
re-derived per component from raw hex values. It didn't fix the token
taxonomy (what categories exist, what each is for), the extraction
process itself, or what happens where the design reference is
incomplete (dark palette, the unresolved `atTablet` placement, the
missing Design system screen).

## Problem

Nothing has fixed: the token categories and naming convention, how
`frontend-tooling.md`'s Tailwind config actually consumes them, whether
dark mode is built now or deferred, or how a token gap (a value the
design reference doesn't specify for a state this app needs) gets
resolved without inventing a color that was never approved.

## Goals

- Fix the token taxonomy: color, spacing, radii, shadow, typography —
  what belongs in each category and how it's named
- Fix the extraction process: source (`.design-reference/*.dc.html`'s
  CSS custom properties) → destination (Tailwind theme config), and who
  runs it (the `frontend-design` skill, per `architecture-frontend.md`
  FR-4)
- Decide dark palette's fate for this phase: built, stubbed, or deferred
  — `roadmap/04-frontend-foundation/README.md`'s own "Architecture
  decisions expected" names this as open
- Fix what happens when a token is needed but the design reference
  doesn't specify it

## Non-goals

- The actual extracted values (specific hex codes, spacing scale
  numbers) — this spec fixes the *process and taxonomy*; the values
  themselves come out of running that process against the real design
  reference during phase 04 implementation, not invented in this
  document
- Component-level application of tokens — `frontend-component-primitives.md`
- The generated-cover system's own color/texture treatment —
  `frontend-generated-covers.md`, which may need tokens this spec
  defines but isn't this spec's own scope to design

## User stories

- As **a component author**, I want every color, spacing, and radius
  value to come from a token, never a raw hex/px value typed into a
  component, so the design system stays traceable to its source.
- As **`frontend-tooling.md`'s Tailwind config**, I want tokens in a
  shape Tailwind's theme extension mechanism consumes directly, not a
  parallel system I have to bridge by hand.
- As **a future contributor building a screen the design reference
  doesn't cover** (Design system, the unresolved `atTablet` question),
  I want a documented fallback rule instead of guessing a value that
  looks plausible.

## Functional requirements

- **FR-1** Token categories, each a distinct Tailwind theme extension
  namespace: `color` (semantic names — `background`, `surface`, `text`,
  `accent`, etc. — never raw color names like `blue-500`, since the
  design reference's own CSS custom properties are already semantic:
  `--bg`, `--sf`, `--tx`, `--ac`), `spacing`, `radius`, `shadow`,
  `typography` (font family, size scale, letter-spacing per
  `roadmap/04-frontend-foundation/README.md`'s named typefaces: Geist,
  Newsreader, IBM Plex Mono), **`breakpoint`** (the responsive-width
  values Tailwind's own `screens` theme key consumes — the category
  `frontend-shell-and-routing.md` FR-3 depends on for its
  sidebar-to-tab-bar reflow, extracted the same way as every other
  category rather than left as a number that spec would otherwise have
  to invent unilaterally). Each category's values are extracted
  exclusively from `.design-reference/*.dc.html`'s CSS custom
  properties and layout rules, never invented.
  *(Implementation note, phase 04 Tier 4: Tier 1's extraction shipped
  every category except `breakpoint`. The canvases carry no `@media`
  rules — they are fixed-width artboards — so the one FR-3 reflow value
  is read from the `atTablet` screen's own layout-rule prose
  ("768–1023px. The sidebar becomes a 60px icon rail …"), via
  `web/scripts/tokens/extractBreakpoint.ts`, which throws if that prose
  ever disappears. Emitted as `--breakpoint-reflow: 768px`. Maintainer
  decision F1, `tasks/todo-p04-tier4-shell-and-routing.md`.)*
- **FR-2** Extraction is a documented, repeatable process, not a one-time
  hand-copy: the `frontend-design` skill (`architecture-frontend.md`
  FR-4) reads each `.dc.html` file's `:root` custom-property
  declarations, maps each to FR-1's taxonomy, and writes **two** output
  artifacts from the same single pass — Tailwind theme config (`web/`'s
  consumer) and a plain CSS custom-properties file, `tokens.css`
  (`:root { --bg: ...; --sf: ...; }`, one variable per FR-1 color/
  spacing/radius/shadow/typography value, no Tailwind-specific syntax) —
  for any consumer that isn't a Tailwind build, currently
  `desktop-host-window-and-serving.md` FR-3's Electron-bundled loading/
  error asset (added 2026-08-14: a cross-phase review found phase 05
  citing this second artifact before this spec actually produced it —
  fixed here rather than left as a dangling citation). Both files are
  generated by the same script, from the same parsed source, in the
  same run — never two independently-maintained outputs that could
  drift from each other. Run again whenever the design reference
  changes (CLAUDE.md's own "re-pull before trusting this section is
  current" discipline applied to tokens specifically), not treated as a
  single import frozen at phase 04's start.
- **FR-3** Only the **light palette** is extracted and wired this phase.
  Dark mode is stubbed structurally — every color token has a defined
  Tailwind CSS-variable-backed shape (so a component never hardcodes a
  light-only value it would need to revisit) but no dark values are
  populated, and no dark-mode toggle exists in the UI yet. Reasoning:
  the design reference itself doesn't have a captured dark canvas to
  extract from (`.design-reference/ANALYSIS.md` doesn't list one), so
  populating dark values now would mean inventing them, the exact
  practice this spec's FR-4 forbids for missing tokens generally. The
  CSS-variable structure being ready means a future dark palette is a
  values-only change, not a component-level rewrite.
- **FR-4** A token gap (a value phase 06+ needs that the design
  reference doesn't specify for the relevant state — e.g. a color for a
  screen state the reference never captured) MUST NOT be resolved by
  inventing a plausible-looking value. The blocking screens named in
  `.design-reference/ANALYSIS.md` (Design system, the unresolved
  `atTablet` placement) are the concrete instances of this rule right
  now — CLAUDE.md's own "do not infer it" directive, restated here as an
  enforceable rule for tokens specifically: flag the gap, ask for the
  design reference to be extended, don't guess.
- **FR-5** Typography tokens fix three font families (Geist —
  UI/interface text; Newsreader — book titles/long-form reading-adjacent
  text; IBM Plex Mono — code/technical/correlation-ID-shaped text) each
  with a defined size scale and letter-spacing rule, extracted from the
  design reference the same way color tokens are (FR-2) — not a
  separate, ad hoc process.

## Non-functional requirements

- **Performance** — token extraction is a build-time process (FR-2), not
  a runtime cost; no budget needed beyond `frontend-tooling.md`'s
  overall bundle-size number, which tokens contribute negligibly to
  (CSS custom properties, not JS).
- **Security** — not applicable; tokens carry no user data or secrets.
- **Accessibility** — color tokens MUST be checked for contrast
  compliance (WCAG AA minimum) as part of extraction (FR-2), not
  assumed correct because the design reference "looks fine" — a
  design-tool canvas isn't itself contrast-audited. This is
  `frontend-accessibility.md`'s requirement, satisfied at the token
  layer here rather than retrofitted per component.
  *(Amended 2026-08-28, phase 04 Tier 6 / F27, maintainer decision D2:
  one accepted permanent exception — `text-3` (`#9C978F`, from every
  canvas's `--tx3`) does not meet AA against any surface (2.90:1 at
  best). It is accepted as a decorative / non-essential tertiary label
  colour only — muted captions, the correlation-ID line — never body or
  load-bearing text. `web/scripts/check-token-contrast.ts` records it in
  `KNOWN_EXCEPTIONS` and continues to fail the build if any **other**
  pair regresses. Tier 5 additionally lifts `text-3` to the `text-2`
  value under `@media (prefers-contrast: more)` (`web/src/a11y.css`), so
  a high-contrast user gets an AA-passing value. The default palette is
  not changed — darkening `--tx3` unilaterally would violate FR-4.)*
- **Reliability** — FR-2's repeatable-process requirement is what keeps
  tokens from drifting out of sync with the design reference silently.
- **Observability** — not applicable; build-time only.

## Domain model

Not applicable — design tokens, not the Alexandryn domain.

## API and contracts

- **Design reference (`.dc.html` CSS custom properties) → two outputs**:
  Tailwind theme config and `tokens.css` (FR-2), both from one extraction
  pass.
- **Tailwind theme config → components**: Tailwind's own utility-class
  mechanism — a component uses `bg-surface`, never `bg-[#1a1a1a]`
  (`frontend-component-primitives.md`'s consuming side).
- **`tokens.css` → non-Tailwind consumers**: a plain CSS file, imported
  via a normal `<link>`/`@import`, no build step of its own required to
  consume it (`desktop-host-window-and-serving.md` FR-3's consuming
  side).

## State transitions

Not applicable.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| A component hardcodes a raw color/spacing value instead of a token | Code review, or a lint rule if one is added (Open questions) | Nothing at runtime — this is a maintainability defect, not a user-facing bug | Flagged in review against `roadmap/04-frontend-foundation/README.md`'s own named "styled ad hoc instead of tokenised" risk — not yet one of `general/code-review`'s own listed dimensions; worth adding there once this pattern shows up in review twice, per that skill category's own stated bar (`skills/README.md`) |
| Design reference changes and tokens aren't re-extracted | Manual — no automated staleness check exists yet | A visual drift between the design reference and the shipped app, unnoticed until someone compares them | Named as a residual risk, not solved (Open questions) |
| A screen needs a token the design reference doesn't specify | FR-4's rule | The screen isn't built with an invented value | Flagged for the design reference to be extended, not guessed past |

## Security considerations

Not applicable — no trust boundary, no user input, no credential
involved in a design token.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Token resolution — a component using a token class actually resolves to the extracted value, proven against the generated Tailwind config |
| Automated (partial) | `frontend-accessibility.md` FR-4's `axe-core` CI run checks contrast for every color token pair that appears in a *rendered* phase 04 component — real, automated, but only as complete as phase 04's actual component coverage, not every combinatorially possible pair |
| Manual (full matrix) | Every token pair, including ones no phase 04 component yet renders: checked with a standalone contrast-calculation script (`web/scripts/check-token-contrast.ts`, run against the generated Tailwind config, not the rendered DOM) as part of FR-2's extraction process, with results recorded in this spec's own PR description at extraction time — a concrete, repeatable step, not an unspecified "someone looked at it" |

## Acceptance criteria

- [ ] Every color, spacing, radius, and typography value used by any
      phase 04 component traces back to an extracted token, none
      hardcoded
- [ ] Light palette fully extracted and wired into Tailwind config
- [ ] Dark-mode CSS-variable structure exists (FR-3) with no dark values
      populated and no toggle exposed in the UI
- [ ] Every color token pair meets WCAG AA contrast, checked via the
      standalone contrast-calculation script and recorded at extraction
      time (Test strategy), with rendered-component pairs additionally
      covered by `frontend-accessibility.md`'s automated `axe-core` run
- [ ] No token value was invented to fill a design-reference gap (FR-4)
      — any gap encountered is named in this spec's Open questions or a
      follow-up, not silently resolved

## Open questions

- **Dark palette's actual values and toggle mechanism** — deliberately
  deferred (FR-3); revisit once the design reference captures a dark
  canvas, or a product decision is made to design one independently of
  the current reference.
- **Design system screen, still uncaptured** — `.design-reference/ANALYSIS.md`'s
  own named gap; any token this screen would need stays unresolved until
  it's captured.
- **`atTablet`'s placement** — inherited from `architecture-frontend.md`'s
  own Open questions; affects whether tablet-specific token overrides
  (if any) belong to the host or viewer surface.
- **A lint rule catching hardcoded raw values instead of token classes**
  — not built here; flagged as the concrete mechanism that would close
  the "styled ad hoc" risk phase 04's own roadmap risk table names,
  worth adding once the token set is stable enough to lint against.

## References

- `architecture-frontend.md` FR-4 — the extraction requirement this spec
  makes concrete
- `.design-reference/ANALYSIS.md` — canvas completeness, the Design
  system and `atTablet` gaps FR-4 and Open questions both reference
- `roadmap/04-frontend-foundation/README.md` — typography families,
  dark-palette open decision, the "styled ad hoc" risk
- `.claude/skills/README.md` — `frontend-design` skill, the extraction
  mechanism FR-2 names
- CLAUDE.md — "do not infer it," the rule FR-4 restates for tokens
- Constitution §7 (accessibility — contrast requirement)
