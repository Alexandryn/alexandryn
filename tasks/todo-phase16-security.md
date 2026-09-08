# Phase 16 — Security hardening: task list

Plan with context and method:
[`tasks/plan-phase16-security.md`](plan-phase16-security.md).
Authoritative scope: [`.claude/roadmap/16-security-hardening/README.md`](../.claude/roadmap/16-security-hardening/README.md).
Audit doc: [`.claude/audits/0016-phase16-security-hardening.md`](../.claude/audits/0016-phase16-security-hardening.md).

Branch: `feat/phase16-security-hardening`. Never `main`.
Stop at the **Gate** (maintainer review of findings) before any remediation.

## Progress

### Tier 0 — Scaffold

- [x] Branch cut from `origin/main` (`1c60608`)
- [x] Roadmap outline expanded to full phase document
- [x] Audit scaffold created
- [x] Plan + task list
- [x] GitHub labels created (`severity:*`, `area:*`, `phase-16`)
- [x] Draft PR opened — #85

### Tier 1 — Review fan-out

- [x] `security-auditor` — three trust boundaries + four-attacker pass (17 findings)
- [x] `code-reviewer` — backend `internal/` + `cmd/` (22 findings)
- [x] `code-reviewer` — `web/src/` + `electron/src/` (34 findings)
- [x] `test-engineer` — coverage + negative-test gaps + coverage threshold (14 findings)
- [x] `web-performance-auditor` — `web/src/` (15 findings)
- [x] CI/CD + dependency + license audit — inline (13 findings)
- [x] `govulncheck` + `npm audit` + `go-licenses` + npm license scan
- [ ] `/code-review ultra` (maintainer-run) findings merged

### Tier 2 — Consolidation

- [x] Issues filed, one per finding — 108 issues, #86–#292 (deduplicated)
- [x] Audit Findings table populated with issue links
- [x] Threat-model consolidation written (per boundary)
- [x] "What was not examined" filled — 6 gap findings filed as issues

### Gate — Maintainer review of findings (STOP — awaiting maintainer)

- [ ] Findings report delivered
- [ ] Remediation order + forward-scheduling decisions recorded
- [ ] `/code-review ultra` launched by maintainer and its findings merged

### Tier 3 — Remediation

- [ ] (populated from the issue list after the gate)

### Exit criteria (from the phase README)

- [ ] Consolidated audit recorded with mandatory handler → repository → SQL traces
- [ ] Every finding filed as an issue with labels; audit table links each
- [ ] Zero open Critical or High findings
- [ ] `govulncheck` + `npm audit --audit-level=high` clean; license audit recorded
- [ ] CSP + Electron fuses reviewed, findings filed
- [ ] CI workflow audited, findings filed
- [ ] Test-coverage analysis recorded
- [ ] `/code-review ultra` cross-check completed and merged
- [ ] Documentation updated
- [ ] Maintainer approval recorded
