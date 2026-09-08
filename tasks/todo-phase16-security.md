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
- [x] `/code-review ultra` — run; 12 new issues #293–#304, 3 cross-validated

### Tier 2 — Consolidation

- [x] Issues filed, one per finding — 120 issues, #86–#304 (deduplicated)
- [x] Audit Findings table populated with issue links
- [x] Threat-model consolidation written (per boundary)
- [x] "What was not examined" filled — 6 gap findings filed as issues
- [x] Post-filing verification — #87 High→Med (latent), #262 Med→High
      (traced: live email disclosure), #260 re-scoped to Phase 99
- [x] Ultra findings merged — 4 Phase 15 deliverables found unwired (#294–#296, #302)

### Gate — Maintainer review of findings

- [x] Findings report delivered
- [x] Remediation order: maintainer directed the 16 HIGH + the named CI
      gaps to be worked directly (session 2026-09-08)

### Tier 3 — Remediation (2026-09-08)

**All 16 actionable HIGH findings resolved, RED → GREEN, one commit each:**

Backend

- [x] #86 — SSRF: `sources.GuardedTransport` dial-time guard + DNS-rebinding
      defence; `SOURCE_ALLOW_PRIVATE_ADDRESSES` opt-in (ADR: backend-configuration.md FR-4 row added)
- [x] #88 — `GET /library` + `/works/{id}` scoped to the active library;
      `work_repository.go` CTE refactor with `library_id` predicate on every join
- [x] #89 — TOTP verify: per-IP + per-user (`MFAUserLimiter`) rate limiting
- [x] #90 — push cursor no longer pull-usable (`seq == cursor+1` guard)
- [x] #262 — `GetLibraryHandler` / `ListMembersHandler` / `CreateInvitationHandler`
      authorization (library-member / library-admin)
- [x] #294 — retention reaper wired into `cmd/server` startup
- [x] #296 — `SanitizedPayload` recurses into nested maps/slices; `note` key added

Web

- [x] #91 — global 401 handling (`queryClient` cache onError → refresh → `/login?next=`); 4xx no-retry (#227)
- [x] #92 — `CapabilityProvider` error state with retry (fail-closed preserved)
- [x] #93 — real `/settings` and `/more` index screens (`NavList`)
- [x] #94 — auth screens + Titlebar + Libraries + DevicesSettings ported to real `@theme` tokens
- [x] #95 — invite token preserved (login `returnTo`, `AcceptInviteScreen` `state.from`)
- [x] #97 — MFA modals rebuilt on the shared Radix `Modal` (dialog role, focus trap, Escape, focus return, OTP autofill)
- [x] #99 — `downloadReadingExport` routes through `http.ts` (`getBlob`) with auth headers; mutation + error UI
- [x] #100 — route-level `React.lazy` code splitting (`lazyScreens.ts`), ~44 KB gzip off first paint
- [x] #102 — adaptive activity-feed polling (`activityPollInterval`)

**CI/CD hardening (maintainer-named):**

- [x] #121 — actions SHA-pinned; Dependabot bumps them. ADR 0034
- [x] #123 — least-privilege `permissions: contents: read` top-level + per-job
- [x] #125 — `gosec` at high/high in the backend job (0 findings today)
- [x] #126 — Dependabot npm ecosystem at workspace root
- [x] #128 — CODEOWNERS: `package-lock.json` + `web/`/`electron/` `package.json`
- [x] #131 — ADR 0033: coverage non-regression floor (`scripts/check-coverage.sh`, baseline 54.0)
- [x] #133 — `check-user-scoped-reading.sh` extended to the catalog surface
- [x] #205 — `server-binary` artifact carries a SHA-256 checksum the desktop job verifies
- [x] #198 — `download-artifact` aligned to v7 (matches `upload-artifact`)

**Medium/Low fixes taken opportunistically (RED → GREEN, committed):**

- [x] #106 — MFA ticket signed with a dedicated HKDF subkey
- [x] #119 — `import.go` / `device_sync.go` errors routed through `writeDomainError`
- [x] #227 — folded into #91

**Deferred to Phase 17 (accessibility & QA) or post-release — recorded per issue:**

- #87 (collections `library_id` — latent, no cross-library collection data today)
- #104 (import-candidate ownership scoping) — same seam, needs its own trace
- #107 (MFA re-enrol/disable step-up), #189 (MFA ticket replay), #187 (login timing oracle), #195 (per-IP limiter behind a proxy)
- #109, #111, #112, #114, #116, #118 (sync/query correctness + index tuning)
- #119 remaining ~40 `err.Error()` sites in the other handler files (same shape, unconfirmed)
- #138–#171 (web UX / a11y / performance Mediums) → Phase 17
- #173, #175, #176, #178, #180, #182, #183, #185 (backend Lows)
- #191 (CSP `style-src 'unsafe-inline'`) — needs nonce plumbing + browser testing
- #193 (Electron banner `executeJavaScript` interpolation)
- #197, #204, #207, #255, #257 (deps / license automation)
- #200, #202, #209, #210, #212 (CI ergonomics, branch protection, flake)
- #214–#245 (web performance/a11y Lows) → Phase 17
- #246, #248, #250, #251, #253, #259 (Informational)
- #264, #265, #267 (unfinished review surfaces — pg-supervisor, contract diff, guard-script bypass audit)
- #293, #295, #298, #299, #300, #302 (Phase 15 correctness/wiring — recorded for the next Phase 15 touch)

### Exit criteria (from the phase README)

- [x] Consolidated audit recorded with mandatory handler → repository → SQL traces
- [x] Every finding filed as an issue with labels; audit table links each
- [x] Zero open Critical or High findings — **0 Critical, 0 High** (16/16 resolved 2026-09-08)
- [x] `govulncheck` + `npm audit --audit-level=high` clean; license audit recorded
- [x] CSP reviewed, findings filed; Electron fuses re-scoped to Phase 99 (#260)
- [x] CI workflow audited + hardened (ADR 0033, ADR 0034)
- [x] Test-coverage analysis recorded (ADR 0033)
- [x] `/code-review ultra` cross-check completed and merged
- [x] Documentation updated
- [ ] Maintainer approval recorded
