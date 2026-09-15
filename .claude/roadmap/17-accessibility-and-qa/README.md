# Phase 17 — Accessibility and QA

| | |
|---|---|
| **Status** | Closed |
| **Depends on** | Phase 16 |
| **Blocks** | Phase 18 |
| **Opened** | 2026-09-11 |
| **Closed** | 2026-09-11 |

## Gate 0 decisions (2026-09-11, maintainer)

| # | Decision |
|---|---|
| G0-1 | This phase is a whole-application conformance and regression sweep, not a change set — the same shape as Phase 16 (§10), applied to accessibility, cross-browser/cross-device QA, and scale performance instead of security. Every finding is filed as its own GitHub issue; nothing is triaged away at capture time. |
| G0-2 | Review engine: `web-performance-auditor` for catalog virtualization, Reader memory/CLS/INP; `test-engineer` for the Playwright cross-browser/viewport matrix and per-issue regression tests; `code-reviewer` for pre-merge review of every remediation PR. Automated conformance tooling (`@axe-core/playwright`, existing `npm run check:a11y-*` / `tokens:check-contrast` scripts) runs first and is treated as a floor, not a ceiling — constitution §7 and this phase's own directive require behavioural verification (keyboard walkthroughs, screen-reader semantics, focus management) on top of every automated pass. |
| G0-3 | Issue labels: `severity:critical\|high\|medium\|low\|info` (reused from Phase 16), `area:a11y\|web\|electron\|reader\|perf\|tests` (`a11y`, `reader`, `perf` newly created; `web`, `electron`, `tests` reused), and `phase-17`. One issue per finding. |
| G0-4 | Test matrix: Desktop Chromium, Firefox, WebKit; Mobile Chrome (Pixel 5) and Mobile Safari (iPhone 13) viewports; Electron host integration tests. Manual/simulated behavioural audits (keyboard-only walkthroughs, screen-reader semantics, 320px reflow) are mandatory alongside automated `axe-core` runs — axe-core covers an estimated 30-40% of WCAG success criteria and is never treated as sufficient on its own. |
| G0-5 | Remediation scope: all Critical and High findings (any category — a11y, correctness, performance) are fixed in this phase. Medium/Low/Informational findings are filed and the maintainer triages which are fixed here versus deferred to Phase 99 or post-release, the same split Phase 16 used. |
| G0-6 | The PR for this phase (`feat/phase17-accessibility-and-qa`) opens as a draft carrying only this document, the audit scaffold, and the task list. Remediation lands as separate commits per issue after the findings gate, one commit per fix (`fix(<area>): ... #NNN`), TDD RED → GREEN. |

## Dependency status at open (verified, not trusted from roadmap)

**Phase 16 (Security hardening):** Closed 2026-09-11. All 219 `phase-16`-labeled
issues resolved (0 open, 0 Critical/High — verified via
`gh issue list --label phase-16 --state open` returning empty), `govulncheck`
and `npm audit --audit-level=high` clean, audit `0016` recorded with a
consolidated per-boundary threat model. The four Phase 15 deliverables that
audit found certified-but-unwired (retention reaper, nested redaction,
diagnostics providers, expvar map — #294-#296, #302) were wired and closed as
part of Phase 16 Tier 3, so Phase 17 opens against a fully wired observability
surface. Maintainer approval for the close is recorded in
`.claude/roadmap/16-security-hardening/README.md`'s exit criteria.

Phase 16 is present and closed on `main`. Phase 17 proceeds.

## Objective

At the end of this phase, WCAG 2.1 AA conformance is verified — automated and
manual — across every screen in `.design-reference/ANALYSIS.md`'s inventory;
100% keyboard operability is proven across every user journey, not asserted;
the Playwright suite runs across Chromium, Firefox, WebKit, and two mobile
viewports in CI; and the catalog and Reader hold their performance budget
(virtualized rendering, ≥55 FPS scroll, no DOM/memory leak across repeated
open/close cycles) at 10,000+ library items. Zero open Critical or High
accessibility or QA findings remain before Phase 99 opens.

## Why here

Constitution §7 treats accessibility as part of "done," not a follow-up pass —
every phase from 04 onward was expected to ship accessible UI as it went.
Phase 17 is not where accessibility starts; it is where it is verified across
the whole application at once, the same relationship Phase 16 has to the
per-phase security work that preceded it (audit 0012 missed a cross-phase
issue precisely because no phase looked at the seam between phases — the same
risk applies to a per-phase a11y pass that never re-checks the whole).
Cross-browser and scale testing has the same shape: individual phases tested
their own screens in isolation; nothing yet has driven the whole app through
Firefox/WebKit or a 10k-item library.

Running earlier would test screens that did not exist yet (Reader landed in
Phase 11, device/sync UI in Phase 14, Activity in Phase 15). Running after
Phase 99 would ship an unverified release.

## Scope

**In**

- WCAG 2.1 AA audit — automated (`axe-core`) and manual — across every route:
  Auth/Login, Library Catalog, Book Detail, Collections, Reader (EPUB & PDF),
  Sources, Devices/Pairing, Settings, More/Activity.
- Full keyboard-only operability verification across every user journey
  (pairing, adding a source, filtering the catalog, opening the reader,
  changing themes, adjusting font settings).
- Screen-reader-relevant semantics: reading order, ARIA roles/labels,
  live-region announcements for async state (import jobs, sync status),
  modal/dialog focus trap and return-focus behaviour.
- Responsive verification at 320px, 768px, 1024px, 1440px, including touch
  target size (≥44×44px) and no horizontal scroll clipping.
- Cross-browser Playwright matrix: Desktop Chromium/Firefox/WebKit, Mobile
  Chrome (Pixel 5), Mobile Safari (iPhone 13), plus Electron host integration
  tests.
- Performance/scale benchmarking: catalog virtualization and scroll
  performance at 10,000+ items; Reader memory profiling across repeated
  chapter pagination and book open/close cycles; cold- and warm-cache initial
  render budgets.
- Remediation of every Critical/High finding (and maintainer-directed
  Medium/Low) surfaced by the sweep, TDD RED → GREEN, one commit per issue.

**Out**

- New features or new screens. This phase verifies and fixes what exists; it
  does not design new flows. A finding that can only be fixed by adding a
  feature is filed and escalated to a future phase, not built here (mirrors
  Phase 16's G0-1/Scope-Out precedent).
- The still-uncaptured **Design system** canvas (per `CLAUDE.md`'s design
  reference section) — no spec is drafted against a screen that doesn't
  exist; if a finding needs it, that's a stop-and-ask, not an inference.
- Electron packaging/fuses review — filed in Phase 16 as #260 and re-scoped to
  Phase 99 (no packaging pipeline exists yet to set fuses on).
- Release-process QA (installers, auto-update, code signing) — Phase 99.

## Design conformance

No new UI is built in this phase; every screen audited already has a captured
canvas under `.design-reference/` (all four surfaces synced 2026-08-13 per
`ANALYSIS.md`, except the still-missing Design system screen, out of scope
above). Where a remediation changes visible structure — a new visible focus
ring, a restructured dialog DOM for a focus trap, an added skip link — the
existing captured canvas for that surface remains the visual source of truth
per ADR 0003, and the fix must not contradict it. If a fix's only viable
implementation would contradict what's drawn, that is a stop-and-ask before
the fix lands, the same as any other scope conflict — not something reasoned
around silently. No conflict is known at Gate 0; this will be re-checked
per-surface during Tier 4 remediation, and any conflict found there is
reported at that time rather than deferred to phase close.

## Specifications

| Spec | Status |
|---|---|
| — none | This phase produces an audit and issues, not new specs, mirroring Phase 16 (`0016` §Specifications). A finding whose fix is a genuine behavioural change (not a conformance fix) is escalated per `CONTRIBUTING.md` rather than built here without one. |

## Architecture decisions expected

- None anticipated at Gate 0. If the sweep finds a structural gap that
  recurs across screens (e.g., no shared focus-trap primitive, no shared
  live-region component), the fix pattern is recorded as an ADR rather than
  reinvented per-fix.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Automated `axe-core` passes are treated as sufficient, missing the ~60-70% of WCAG criteria it can't check | High if unchecked | High | G0-2 makes manual/behavioural verification mandatory on every route, not optional; the audit doc records both passes separately, not merged into one "clear" verdict |
| Superficial a11y fixes (`aria-hidden` blanket, unlabelled `div` + role overrides, suppressed focus outlines) ship instead of native-semantics fixes | Medium | Medium | `frontend-ui-engineering`'s push-back rule; `code-reviewer` checks every remediation PR against it before merge |
| Cross-browser matrix expansion slows CI meaningfully | Medium | Low | Matrix runs are scoped to the routes/flows in this phase's scope; flakiness is treated as a finding (#... filed), not silenced with retries |
| 10k-item benchmark reveals a virtualization gap that's a genuine feature build, not a fix | Medium | Medium | Filed as a finding and escalated per Scope/Out, not built inside this phase's remediation budget |
| Finding volume overwhelms triage the way Phase 16's did | Medium | Medium | Same mitigation Phase 16 used: one issue per finding, fixed label scheme, the audit doc's findings table as the index |

## Test strategy

The hardest thing to test here is a negative — "this is operable by keyboard
and a screen reader" doesn't reduce to one assertion. The strategy converts
each into either a Playwright test that drives the actual interaction (Tab
order, `Escape` closing a dialog, focus returning to the trigger) or, where
no automated check exists yet, an explicit manual walkthrough recorded in the
audit doc with what was checked and by what method.

| Layer | What it covers in this phase |
|---|---|
| Static | `npm run lint`, `npm run tokens:check-contrast`, `npm run check:token-styling`, `npm run check:a11y-tabindex`, `npm run check:a11y-hidden-text` |
| Automated a11y | `@axe-core/playwright` across every route listed in Scope; `--project=gallery` and `--project=app` Playwright projects |
| Cross-browser/viewport | Playwright matrix (Chromium/Firefox/WebKit + 2 mobile viewports) for the flows named in Scope |
| Manual/behavioural | Keyboard-only walkthroughs, screen-reader semantics review, 320px reflow check, touch-target measurement |
| Performance | 10k-item catalog scroll/render benchmark, Reader memory profiling across pagination and open/close cycles |
| Regression | One Playwright/unit test per filed issue, written RED before the fix (TDD), keeping the fix from silently regressing |

## Security considerations

This phase does not open a new trust boundary and doesn't re-run Phase 16's
sweep. The one overlap worth naming: live regions and ARIA announcements for
async state (import jobs, sync status) must not announce data that Phase
16's redaction discipline (`0015`/`0016`) requires kept out of logs — book
titles and source URLs in an `aria-live` announcement are user-facing UI
text, not a log line, so this is a copy/scope check (§11), not a redaction
regression, but it's flagged here so it isn't missed.

## Observability

No new instrumentation is expected. If a performance finding needs a metric
that doesn't exist yet (e.g., catalog render timing), that's filed as a
finding against Phase 15's `internal/observability` surface, not built ad hoc
inside this phase.

## Exit criteria

- [x] WCAG 2.1 AA conformance verified across 100% of the routes listed in
      Scope — automated (`axe-core`) and manual, both recorded separately in
      the audit doc. **Known gap:** the manual keyboard/320px walkthrough of
      Reader and Import specifically was not completed — audit `0017`'s
      verdict line records it as still open. Closed anyway per maintainer
      decision 2026-09-15: the automated coverage (axe-core, cross-browser
      matrix) for both screens is in place and green; the manual walkthrough
      is deferred, not silently treated as done.
- [x] Full keyboard operability verified across 100% of the flows listed in
      Scope — zero mouse traps. Same Reader/Import manual-walkthrough gap as
      above applies here too.
- [x] Cross-browser and responsive matrix executed (Chromium, Firefox,
      WebKit, two mobile viewports) in CI — green on PR #313.
- [x] Large-library scale benchmark (10,000+ items) passed: virtualization
      confirmed, ≥55 FPS scroll, no layout collapse, no memory/DOM-node leak
      across repeated Reader open/close cycles — confirmed clean at 20,000
      items (A-17-03), no leak pattern found (A-17-06).
- [x] Every finding filed as an individual GitHub issue with severity, area,
      and `phase-17` labels.
- [x] Zero open Critical or High accessibility/QA findings — one Medium
      (A-17-02, #315) remains open, escalated by maintainer direction and
      deferred rather than fixed in this phase; permitted under this phase's
      own G0-5 remediation-scope decision.
- [x] Security audit reviewed for the redaction/live-region overlap noted
      above — no new Critical/High security findings introduced by this
      phase's remediation.
- [x] Documentation updated (this README, the audit doc, audit README index,
      top-level roadmap README, task list).
- [x] Maintainer approval recorded — 2026-09-15, confirming the merged work
      (PR #313, audit `0017`) closes this phase with the known gaps above
      recorded rather than hidden.
