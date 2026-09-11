# Phase 17 — Accessibility and QA: task list

Plan with context and method:
[`tasks/plan-phase17-accessibility-and-qa.md`](plan-phase17-accessibility-and-qa.md).
Authoritative scope: [`.claude/roadmap/17-accessibility-and-qa/README.md`](../.claude/roadmap/17-accessibility-and-qa/README.md).
Audit doc: [`.claude/audits/0017-phase17-accessibility-and-qa.md`](../.claude/audits/0017-phase17-accessibility-and-qa.md).

Branch: `feat/phase17-accessibility-and-qa`. Never `main`.
Stop at **Gate 0** (scope approval) before Tier 1, and at the **findings
Gate** (maintainer review) before Tier 4 remediation.

## Progress

### Tier 0 — Scaffold

- [x] Branch cut from `origin/main` (`7516581` — includes Phase 16 close)
- [x] Roadmap outline expanded to full phase document
- [x] Audit scaffold created
- [x] Plan + task list
- [x] GitHub labels created (`area:a11y`, `area:reader`, `area:perf`,
      `phase-17`)
- [x] Draft PR opened — #313
- [x] **Gate 0 — maintainer approval of scope** (approved 2026-09-11)

### Tier 1 — Automated conformance sweep

- [x] Static checks (lint, contrast, token-styling, tabindex, hidden-text) — all clean
- [x] `axe-core` run per route: Auth/Login, Library Catalog, Sources,
      Devices/Pairing, Settings, More/Activity (new coverage added);
      Book Detail/Collections (pre-existing coverage confirmed).
      **Not covered:** Reader EPUB/PDF (genuine MSW gap), Import screen
      (not attempted this session)

### Tier 2 — Cross-device, cross-browser & manual QA matrix

- [x] Playwright config extended (web: `app-firefox`/`app-webkit`/
      `app-mobile-chrome`/`app-mobile-safari`; electron: existing single
      project, 14/14 passing) — Firefox/WebKit blocked locally (A-17-07,
      sudo-gated), `app-mobile-chrome` verified green
- [x] Keyboard-only walkthrough — 11 routes via throwaway script (tab-stop
      reachability + focus-indicator detection); not extended to Book
      Detail/Collections/Import/Reader
- [ ] Screen-reader semantics review — structural (ARIA via axe) only;
      no real screen reader used
- [x] 320px reflow + touch-target audit — 11 routes, no horizontal scroll
      found anywhere, touch-target gaps found (A-17-10)

### Tier 3 — Scale & performance benchmarking

- [x] 10k+ item catalog benchmark — `web-performance-auditor` pass,
      findings A-17-02/A-17-03/A-17-04/A-17-05
- [x] Reader memory/DOM-leak stress test — static analysis only
      (A-17-06); no live heap-snapshot trace

### Gate — Maintainer review of findings

- [x] Report delivered — 11 findings, 1 Critical (A-17-08), 3 Medium,
      3 Low, 4 Informational; audit doc `0017` commit `3a94a92`
- [x] Issues filed — #314–#324
- [ ] Remediation order + Medium/Low/Info triage decided

### Tier 4 — Remediation

- [ ] (populated per issue once the gate clears)
