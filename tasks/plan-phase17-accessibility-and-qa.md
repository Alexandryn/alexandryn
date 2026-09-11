# Implementation Plan: Phase 17 — Accessibility and QA

## Overview

A whole-application conformance and regression sweep: WCAG 2.1 AA across every
screen, full keyboard operability, a cross-browser/cross-viewport Playwright
matrix, and a 10,000+ item scale/performance benchmark. Mirrors Phase 16's
shape (audit doc + one GitHub issue per finding + a maintainer gate before
remediation) applied to accessibility/QA instead of security.

Authoritative scope: `.claude/roadmap/17-accessibility-and-qa/README.md`.
Gate 0 decisions recorded there (G0-1..G0-6). Audit doc:
`.claude/audits/0017-phase17-accessibility-and-qa.md`. Task checklist:
`tasks/todo-phase17-accessibility-and-qa.md`.

Branch: `feat/phase17-accessibility-and-qa`, cut from `origin/main` (`7516581`,
which includes the Phase 16 close). Never `main`.

## Method

Per the audit doc's "Scope and method." The non-negotiable rule: every
automated `axe-core`/lint pass is paired with a manual behavioural check before
a screen is called conformant — an automated pass alone proves absence of the
~30-40% of criteria a tool can check, not absence of the rest (G0-2).

## Tiers

### Tier 0 — Scaffold

- [x] Branch cut from `origin/main`
- [x] Roadmap outline expanded to full phase document
- [x] Audit scaffold `0017-phase17-accessibility-and-qa.md` created
- [x] This plan + the task list
- [x] Labels created (`area:a11y`, `area:reader`, `area:perf`, `phase-17`;
      `severity:*` and other `area:*` reused from Phase 16)
- [ ] Draft PR opened
- [ ] **Gate 0 — STOP for maintainer approval of this scope before Tier 1**

### Tier 1 — Automated conformance sweep

- [ ] `npm run lint`, `npm run tokens:check-contrast`,
      `npm run check:token-styling`, `npm run check:a11y-tabindex`,
      `npm run check:a11y-hidden-text`
- [ ] `npx playwright test --project=gallery`
- [ ] `npx playwright test --project=app`
- [ ] `@axe-core/playwright` run against every route in Scope

### Tier 2 — Cross-device, cross-browser & manual QA matrix

- [ ] Extend `web/playwright.config.ts` — Desktop Chromium/Firefox/WebKit,
      Mobile Chrome (Pixel 5), Mobile Safari (iPhone 13)
- [ ] Extend `electron/playwright.config.ts` for host integration tests
- [ ] Manual keyboard-only walkthrough of every flow in Scope
- [ ] Manual screen-reader-semantics review (reading order, ARIA, modal
      announcements)
- [ ] 320px reflow + touch-target check

### Tier 3 — Scale & performance benchmarking

- [ ] `web-performance-auditor` — 10k+ item catalog: virtualization, scroll
      FPS, cold/warm-cache render budget
- [ ] Reader stress test — memory profiling across pagination, DOM-leak check
      across open/close cycles

### Gate — Maintainer review and triage (STOP)

- [ ] Audit report delivered (finding count by severity, Critical/High list,
      what wasn't reached)
- [ ] Every finding filed as an individual issue (`severity:*`, `area:*`,
      `phase-17`)
- [ ] Maintainer decides remediation order and which Medium/Low/Info issues
      schedule into Phase 99 vs. block this phase

### Tier 4 — Remediation (per issue, after the gate)

- [ ] Each fix is its own commit referencing its issue, RED → GREEN
      (`test-driven-development`)
- [ ] `code-reviewer` pre-merge review on every remediation PR
- [ ] Phase closes when zero Critical/High issues are open

## Risks

Recorded in the phase README's risk table. The dominant one: treating an
automated `axe-core` pass as sufficient conformance evidence. Mitigated by
G0-2/G0-4 making manual verification mandatory, not optional.
