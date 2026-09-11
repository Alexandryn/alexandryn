# Accessibility & QA audit: Phase 17 — whole-application conformance and regression sweep

| | |
|---|---|
| **Scope** | All routes listed in `.claude/roadmap/17-accessibility-and-qa/README.md` Scope/In — WCAG 2.1 AA, keyboard operability, cross-browser/viewport matrix, scale performance |
| **Auditor** | Scaffold — sweep not yet run |
| **Date** | — |
| **Commit** | — |
| **Verdict** | Not started |

## Template deviation, stated per constitution §12

`.claude/templates/audit.md` is written for a security audit — its "Trust
boundaries examined," "Adversarial questions asked," and severity guide (RCE,
auth bypass, sandbox escape) assume an attacker. This phase's findings are
mostly accessibility and QA defects, not attacker-driven, so those sections
are repurposed below rather than left contradicting their own headings. This
is a deliberate adaptation of the existing template per `CLAUDE.md`'s "use
these; don't invent new document shapes" — not a new template — and is
recorded here rather than silently reusing security language for non-security
findings.

## Scope and method

To be filled once Tier 1 runs. Expected: automated (`axe-core`, static
scripts) + manual (keyboard walkthrough, screen-reader semantics review,
viewport reflow) + performance benchmarking, per the phase README's Test
strategy table. Say which method found which finding, so a later reader knows
whether "clear" means "no human checked" or "a human checked and it's fine."

## Screens/flows examined (in place of "Trust boundaries examined")

| Screen/flow | WCAG axes checked | Method | Assumption being made |
|---|---|---|---|
| Auth/Login | | | |
| Library Catalog | | | |
| Book Detail | | | |
| Collections | | | |
| Reader (EPUB) | | | |
| Reader (PDF) | | | |
| Sources | | | |
| Devices/Pairing | | | |
| Settings | | | |
| More/Activity | | | |

## Conformance questions asked (in place of "Adversarial questions asked")

Work through these explicitly for every screen/flow above, per the phase
README's G0-2/G0-4 and constitution §7.

- Can every interactive control be reached and operated with `Tab`,
  `Shift+Tab`, `Enter`, and `Space` alone — no mouse, no positive `tabindex`?
- Is focus visible at every step, with a ring that meets contrast on every
  background it appears on (light, dark, high-contrast)?
- Does every dialog/modal trap focus while open and return it to the
  triggering element on dismissal? Does `Escape` close it?
- Does every input have a programmatically associated label, and does a
  validation error announce via `role="alert"` or `aria-live="assertive"`?
- Does every async event (import job, sync status, background indexing)
  announce via `aria-live="polite"` without stealing focus?
- Does the screen hold up at 320px with no horizontal scroll and no lost
  functionality, with ≥44×44px touch targets?
- Does `prefers-reduced-motion: reduce` neutralize every animation on this
  screen?
- Does the screen render and behave equivalently in Chromium, Firefox,
  WebKit, and the two mobile viewports?
- At 10,000+ catalog items: does the list stay virtualized, hold ≥55 FPS
  scroll, and avoid a DOM-node/memory leak across repeated navigation?

## Findings

| ID | Severity | Title | Status |
|---|---|---|---|
| A-17-01 | | | Open / Fixed / Accepted / Won't fix |

### A-17-01 — \<title\>

**Severity:** Critical / High / Medium / Low / Informational

**Component:** `path/to/thing`

**Description** — what the defect is, mechanically (e.g., which WCAG success
criterion fails and how).

**Impact** — who is blocked and how badly. If the honest answer is "narrow,
one keyboard-user edge case," say that and rate it accordingly.

**Preconditions** — what triggers it (specific browser, viewport, assistive
tech, data state).

**Reproduction** — steps, ideally a Playwright test reference.

**Recommendation** — the specific fix, not a category of fix. Prefer native
semantic HTML per `frontend-ui-engineering`'s push-back rule over an ARIA
patch.

**Resolution** — filled in when addressed, with the commit and issue number.

---

## Severity guide (adapted for accessibility/QA, not security)

Rate the actual impact on a real user of this system, not the textbook worst
case for the class of issue. Inflated ratings train people to ignore ratings
(constitution §10, applied here to a11y/QA findings the same way).

| | |
|---|---|
| **Critical** | A core flow (auth, catalog browse, opening a book) is entirely unusable by keyboard or assistive-tech users, with no workaround |
| **High** | A WCAG 2.1 Level A failure that blocks a flow for some users (missing label on a required control, a real keyboard/focus trap, a modal that doesn't return focus), or a QA defect that crashes/hangs a browser target in the matrix |
| **Medium** | A WCAG 2.1 Level AA failure that degrades but doesn't block a flow (contrast shortfall, non-live async announcement, awkward-but-possible keyboard path), or a performance regression below budget but not user-blocking |
| **Low** | Narrow impact, high preconditions, or a defence-in-depth/robustness gap (e.g., works today but relies on an implicit DOM order) |
| **Informational** | No user impact today, but makes a future regression more likely, or a coverage gap (untested browser/viewport combination) |

## What was not examined

Honest gaps in coverage. To be filled as the sweep runs — recorded as it
happens, not reconstructed at the end.
