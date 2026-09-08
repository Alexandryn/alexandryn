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
- [ ] GitHub labels created
- [ ] Draft PR opened

### Tier 1 — Review fan-out

- [ ] `security-auditor` — Renderer/Main boundary
- [ ] `security-auditor` — Host/LAN boundary (auth, tenant traces, headers, TLS)
- [ ] `security-auditor` — System/Source boundary (parsers, filenames, redirects, SSRF)
- [ ] `code-reviewer` — backend `internal/`
- [ ] `code-reviewer` — `web/src/` + `electron/src/`
- [ ] `test-engineer` — coverage + negative-test gaps + coverage threshold
- [ ] `web-performance-auditor` — `web/src/`
- [ ] `security-and-hardening` skill — inline
- [ ] `ci-cd-and-automation` skill — workflow + guard scripts
- [ ] `govulncheck` + `npm audit` + license pass
- [ ] `/code-review ultra` (maintainer-run) findings merged

### Tier 2 — Consolidation

- [ ] Issues filed, one per finding
- [ ] Audit Findings table populated
- [ ] Threat-model consolidation written
- [ ] "What was not examined" filled

### Gate — Maintainer review of findings (STOP)

- [ ] Findings report delivered
- [ ] Remediation order + forward-scheduling decisions recorded

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
