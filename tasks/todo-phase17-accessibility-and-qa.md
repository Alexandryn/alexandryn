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
- [ ] Draft PR opened
- [ ] **Gate 0 — maintainer approval of scope (this document +
      `.claude/roadmap/17-accessibility-and-qa/README.md`'s Gate 0 table)**

### Tier 1 — Automated conformance sweep

- [ ] Static checks (lint, contrast, token-styling, tabindex, hidden-text)
- [ ] `axe-core` run per route: Auth/Login, Library Catalog, Book Detail,
      Collections, Reader (EPUB), Reader (PDF), Sources, Devices/Pairing,
      Settings, More/Activity

### Tier 2 — Cross-device, cross-browser & manual QA matrix

- [ ] Playwright config extended (web + electron)
- [ ] Keyboard-only walkthrough per flow
- [ ] Screen-reader semantics review
- [ ] 320px reflow + touch-target audit

### Tier 3 — Scale & performance benchmarking

- [ ] 10k-item catalog benchmark
- [ ] Reader memory/DOM-leak stress test

### Gate — Maintainer review of findings

- [ ] Report delivered
- [ ] Issues filed
- [ ] Remediation order + Medium/Low/Info triage decided

### Tier 4 — Remediation

- [ ] (populated per issue once the gate clears)
