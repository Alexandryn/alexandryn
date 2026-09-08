# Implementation Plan: Phase 16 — Security hardening

## Overview

A whole-application adversarial sweep across the three phase-01 trust boundaries
(Renderer/Main, Host/LAN, System/Source), a dependency vulnerability and license
audit, a CSP / Electron-fuse review, a CI/CD supply-chain audit, and an
automated-testing coverage audit. The deliverable is one consolidated audit
document plus one GitHub issue per finding, at every severity. Remediation is
follow-up work, one issue at a time.

Authoritative scope: `.claude/roadmap/16-security-hardening/README.md`. Gate 0
decisions recorded there (G0-1..G0-5). Audit doc:
`.claude/audits/0016-phase16-security-hardening.md`. Task checklist:
`tasks/todo-phase16-security.md`.

Branch: `feat/phase16-security-hardening`, cut from `origin/main` (`1c60608`).
Never `main`.

## Method

Per the audit doc's "Scope and method". The non-negotiable rule: every
authorization control is certified from the wired call path — handler →
repository → SQL, predicate quoted from query text — never from the layer it
could live in (audit 0012 / review 0050).

## Tiers

### Tier 0 — Scaffold  ✅

- [x] Branch cut from `origin/main`
- [x] Roadmap outline expanded to full phase document
- [x] Audit scaffold `0016-phase16-security-hardening.md` created
- [x] This plan + the task list
- [x] Labels created (`severity:*`, `area:*`, `phase-16`)
- [x] Draft PR opened — #85

### Tier 1 — Review fan-out  ✅ (ultra pass still owed)

- [x] `security-auditor` subagent — three trust boundaries, four-attacker pass
- [x] `code-reviewer` subagent ×2 — backend `internal/`+`cmd/`; `web/src/`+`electron/src/`
- [x] `test-engineer` subagent — coverage gaps, missing negative/cross-tenant tests
- [x] `web-performance-auditor` subagent — `web/src/` structural performance
- [x] CI/CD + dependency + license audit — inline
- [x] Dependency scan: `govulncheck` (0 called), `npm audit` (0), `go-licenses`, npm license scan
- [ ] Maintainer runs `/code-review ultra`; findings merged in

### Tier 2 — Consolidation  ✅

- [x] Every finding filed as a GitHub issue with severity + area + `phase-16`
      labels — 108 issues, #86–#292, deduplicated to one per finding
- [x] Audit doc Findings table populated with issue links
- [x] Threat-model consolidation section written per boundary
- [x] "What was not examined" filled honestly — 6 coverage gaps filed as issues

### Gate — Maintainer review (STOP)

- [ ] Report: finding count by severity, the Critical/High list, the CI/CD and
      test-coverage findings, what was not reached
- [ ] Maintainer decides remediation order and which Medium/Low/Info issues
      schedule forward vs. block this phase

### Tier 3 — Remediation (per issue, after the gate)

- [ ] Each fix is its own branch/commit referencing its issue, RED → GREEN
- [ ] Phase closes when zero Critical/High issues are open

## Risks

Recorded in the phase README's risk table. The dominant one: the sweep
certifies a control from the layer it could live in rather than the wired path.
Mitigation is procedural and enforced by the audit template's mandatory trace.
