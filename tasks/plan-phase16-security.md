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

### Tier 0 — Scaffold (this commit)

- [x] Branch cut from `origin/main`
- [x] Roadmap outline expanded to full phase document
- [x] Audit scaffold `0016-phase16-security-hardening.md` created
- [x] This plan + the task list
- [ ] Labels created (`severity:*`, `area:*`, `phase-16`)
- [ ] Draft PR opened

### Tier 1 — Review fan-out

- [ ] `security-auditor` subagent — three trust boundaries, four-attacker pass
- [ ] `code-reviewer` subagent — correctness, readability, architecture across
      `internal/`, `web/src/`, `electron/src/`
- [ ] `test-engineer` subagent — coverage gaps, missing negative/cross-tenant
      tests, skipped/flaky tests, the unset coverage threshold
- [ ] `web-performance-auditor` subagent — `web/src/` structural performance
- [ ] `security-and-hardening` skill inline — input validation, auth, storage,
      third-party integration surfaces
- [ ] `ci-cd-and-automation` skill inline — `.github/workflows/ci.yml` +
      `scripts/check-*`
- [ ] Dependency scan: `govulncheck`, `npm audit`, license pass
- [ ] Maintainer runs `/code-review ultra`; findings merged in

### Tier 2 — Consolidation

- [ ] Every confirmed finding filed as a GitHub issue with severity + area +
      `phase-16` labels
- [ ] Audit doc Findings table populated with issue links
- [ ] Threat-model consolidation section written per boundary
- [ ] "What was not examined" filled honestly

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
