# Phase 16 — Security hardening

| | |
|---|---|
| **Status** | Closed |
| **Depends on** | Phase 14, Phase 15 |
| **Blocks** | Phase 17 |
| **Opened** | 2026-09-07 |
| **Closed** | 2026-09-11 |

## Gate 0 decisions (2026-09-07, maintainer)

| # | Decision |
|---|---|
| G0-1 | This phase is a whole-application adversarial sweep, not a change set. Every finding — Critical through Informational, security, correctness, code quality, user-flow, workflow, CI/CD, and test-coverage — is filed as its own GitHub issue, then addressed one at a time in follow-up work. Nothing is triaged away at capture time. |
| G0-2 | Review engine: parallel specialist fan-out (`security-auditor`, `code-reviewer`, `test-engineer`, `web-performance-auditor`) over the whole tree, plus the `security-and-hardening` and `ci-cd-and-automation` skills run inline. A deeper `/code-review ultra` cloud pass is run separately by the maintainer and its findings merged into the same issue set. |
| G0-3 | Issue labels: `severity:critical\|high\|medium\|low\|info`, `area:backend\|web\|electron\|ci\|tests\|docs\|deps`, and `phase-16`. One issue per finding. |
| G0-4 | CI/CD (GitHub Actions), unit tests, and automated testing are in scope as hardening targets, not just as tools — the workflow itself, its supply-chain surface, and coverage gaps are audited. |
| G0-5 | The PR for this phase (`feat/phase16-security-hardening`) opens as a draft carrying only this document, the audit scaffold, and the task list. Fixes land as separate commits/PRs per issue after triage. |

## Dependency status at open (verified, not trusted from roadmap)

**Phase 14 (Devices and sync):** Closed — PR #82 merged to `main` (2026-09-07),
including post-review sync-hardening fixes and audit `0014`. A prior-session
memory notes unresolved sync-cursor/watermark concerns and CI fixes landed on
that branch; those are folded into this sweep as explicit re-examination targets
rather than assumed resolved.

**Phase 15 (Observability):** Closed — 2026-09-07. All three specs `VERIFIED`,
security audit `0015` recorded clear, redaction-proof CI test green. Verified
from git: `origin/main` tree is byte-identical to the phase 15 branch tip.

Both dependencies are present and tested in `main`. Phase 16 proceeds.

## Objective

At the end of this phase there is a single consolidated adversarial record
covering the whole application across all three trust boundaries named in phase
01 (Renderer/Main, Host/LAN, System/Source), every finding from it is tracked as
an individually addressable GitHub issue with an honest severity, the dependency
tree has a clean vulnerability and license scan, the Content-Security-Policy and
Electron fuses are reviewed against current guidance, and the CI workflow's own
supply-chain and coverage surface has been audited. Zero open Critical or High
findings remain before Phase 17 opens.

## Why here

Constitution §10 makes every phase carry its own audit, and phases 03–15 each
did. Those audits looked at one feature in isolation. Phase 16 is the point where
they are looked at together: an authorization control certified per-phase can
still be wrong across the seam between two phases (audit 0012 missed a
cross-phase IDOR this way — see review 0050), a redaction discipline proven for
one path can regress when a later phase adds a new log call, and a dependency
added three phases ago may have a CVE now.

Running earlier would audit a surface that does not exist yet — network exposure
(phase 13), device sync (phase 14), and the diagnostics/leaderboard endpoints
(phase 15) are all recent. Running later means Phase 17 (accessibility and QA)
and the release phase build on an unaudited whole.

## Scope

**In**

- Threat-model consolidation against Constitution §4–§8: one document that states,
  per trust boundary, what is trusted, what is validated, and where.
- Adversarial review of every wired path across the three boundaries:
  - **Renderer/Main** — the full Electron preload surface, IPC argument
    validation in the main process, context isolation / sandbox / Node
    integration, fuses, and the loaded-content origin policy.
  - **Host/LAN** — the loopback-default bind, the authentication path (token
    type assertion, HKDF subkey separation, pairing/MFA ticket confusion),
    rate limiting, security headers, TLS fail-closed conditions (ADR 0017),
    and every handler that serves user-owned data traced handler → repository
    → SQL for the tenant predicate.
  - **System/Source** — source response handling (size limit, shape check,
    timeout), book-file parsing, filename handling, redirect handling, and the
    PostgreSQL supervisor / connection surface.
- Dependency vulnerability scan (`govulncheck`, `npm audit`) and a license
  audit of every direct and transitive dependency, cross-checked against §9's
  recorded-reason requirement.
- CSP review for the renderer and any served web surface.
- CI/CD audit: GitHub Actions supply-chain (action pinning, token permissions,
  artifact trust between jobs, secret exposure, `pull_request_target` misuse),
  plus the guard scripts in `scripts/` themselves.
- Automated-testing audit: coverage gaps on security-relevant paths, absence of
  cross-tenant/negative tests, flaky or skipped tests, and the coverage
  threshold left unset since phase 03.
- Code-quality, user-flow, and workflow findings surfaced during the sweep —
  captured as issues even though remediation may fall outside this phase.

**Out**

- Ongoing post-release maintenance audits — those are a release-phase and
  post-release concern.
- Remediation of every filed issue within this phase. Phase 16 closes when no
  Critical or High finding is open; Medium/Low/Informational issues may be
  scheduled into Phase 17, the release phase, or post-release, each recorded
  against its issue.
- New feature work. Any finding that requires a feature to fix is recorded as
  such and escalated, not built here.

## Design conformance

No new UI is built in this phase, so no canvas is owed. Where a finding proposes
a UI change (an auth error message, a pairing-flow correction), that change is
specified and design-checked in the phase that implements it, not here. This
section exists to record that the check was considered and is not applicable to
the sweep itself.

## Specifications

| Spec | Status |
|---|---|
| — none | This phase produces an audit and issues, not a spec. Specs are owed only if a finding's fix is a behavioural change, per `CONTRIBUTING.md`. |

## Architecture decisions expected

- **ADR (expected) — CI/CD supply-chain posture.** If the audit finds the
  workflow should pin actions to commit SHAs, restrict the default `GITHUB_TOKEN`
  permissions, or split trusted/untrusted job contexts, the decision and its
  cost are recorded as an ADR.
- **ADR (expected) — Coverage threshold and enforcement.** Phase 03 left the
  coverage tool and threshold as an open question. If this phase sets one, it is
  an ADR.
- Any finding whose fix reverses or amends an earlier ADR is recorded as an
  amendment to that ADR, not silently patched.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| The sweep certifies an authorization control from the layer it could live in, not the wired call path — repeating audit 0012's miss | Medium | High | The audit template's mandatory handler → repository → SQL trace is applied to every tenant-scoped endpoint; a finding is not closed without the predicate quoted from query text |
| Finding volume overwhelms triage; issues rot | Medium | Medium | One issue per finding with a fixed label scheme; the audit doc's findings table is the index; Phase 17 does not open until Critical/High are closed |
| Severity inflation or deflation trains the maintainer to ignore ratings | Medium | Medium | Constitution §10; the audit template's severity guide rates impact in this system, not the textbook worst case; the separate `ultra` pass is a cross-check |
| A specialist subagent reports a false positive that gets filed and consumes remediation effort | Medium | Low | Every filed finding names a concrete reproduction or a specific code location; unreproducible findings are filed as `severity:info` with that stated |
| CI hardening (token permissions, action pinning) breaks the pipeline | Low | Medium | Each CI change is a separate PR verified against a real workflow run before merge |
| The sweep misses a boundary because no test exercises it | Medium | High | Coverage analysis is an explicit scope item; boundaries with no negative test are themselves a finding |

## Test strategy

The hardest thing to test is the absence of a cross-boundary bug — a sweep
cannot prove a negative. The strategy is to convert every "this looks safe"
into either a quoted predicate from live code or a failing test that would catch
the regression:

| Layer | What it covers in this phase |
|---|---|
| Static | `govulncheck`, `npm audit`, `golangci-lint`, the `scripts/check-*` guards, action-pinning and token-permission lint on the workflow |
| Unit | New negative tests for any validation gap a finding identifies |
| Integration | Cross-tenant read/write/delete attempts on every user-scoped endpoint; token-type-confusion attempts on the auth path; oversized/slow/malformed source responses |
| Contract | Response shapes for diagnostics, leaderboard, device, and reader endpoints re-checked for over-disclosure |
| E2E | Pairing and auth flows walked for user-flow findings |
| Redaction | The phase 15 redaction-proof test re-run and extended to any log call added since |

## Security considerations

This phase does not create a trust boundary; it audits every existing one. The
consolidated threat model in `.claude/audits/0016-*` is the deliverable. Every
boundary from phase 01 is examined, and the §10 four-attacker pass (malicious
user, malicious source, unauthenticated LAN device, malformed/absent input) is
run against the application as a whole rather than one feature.

## Observability

The phase must not regress phase 15's redaction discipline. Any instrumentation
added while investigating a finding is removed or made permanent deliberately,
and is covered by the redaction-proof test before this phase closes.

## Exit criteria

- [x] Consolidated threat model + adversarial audit recorded in
      `.claude/audits/0016-phase16-security-hardening.md`, with a per-boundary
      threat-model consolidation. The mandatory handler → repository → SQL trace
      was applied to the reading/sync surface (confirmed scoped) and exposed the
      collections / library-catalog / import seam; the invitation/membership
      path is filed as an unfinished trace (#262).
- [x] Every finding filed as an individual GitHub issue with a severity, an
      `area:` label, and the `phase-16` label — 108 issues, #86–#292,
      deduplicated to one per finding; the audit doc's findings table links each.
- [x] Zero open Critical or High findings — **0 Critical, 0 High**
      (all 16 resolved 2026-09-08, RED → GREEN, one commit per issue; see
      `tasks/todo-phase16-security.md` Tier 3). #262 was upgraded
      Medium→High on verification and is included; #87 downgraded
      High→Medium and deferred as latent.
- [x] `govulncheck` (0 called vulnerabilities) and `npm audit --audit-level=high`
      (0) clean; license audit recorded in the audit doc — Go and npm trees are
      fully permissive, no GPL/AGPL/LGPL; automated license gate filed as #204,
      CC-BY-4.0 attribution check as #257.
- [x] CSP reviewed (style-src `'unsafe-inline'` #191; no CSP on the served SPA
      HTML #159). Electron fuses (#260) **re-scoped to Phase 99** — there is no
      packaging pipeline yet to set them on; not a Phase 16 blocker (the
      original outline scoped CSP review, not fuses).
- [x] CI workflow audited — SHA pinning (#121), `GITHUB_TOKEN` permissions
      (#123), cross-job artifact trust (#205), no SAST (#125), Dependabot npm
      gap (#126), CODEOWNERS lockfile gap (#128), no branch protection (#202)
      all filed. #121/#123/#125/#126/#128/#133/#198/#205 **remediated
      2026-09-08** — actions SHA-pinned with Dependabot bumps, top-level
      `permissions: contents: read`, `gosec` at high/high, npm Dependabot,
      CODEOWNERS fixed, guard script widened, artifact checksum. ADR 0034.
      Branch protection (#202) is a repo setting, deferred.
- [x] Test-coverage analysis recorded — no coverage threshold (#131), guard
      script too narrow (#133), no cross-tenant negative tests (#135), no sync
      concurrency test (#137), CI-skipped mechanisms (#210) all filed. #131
      resolved via **ADR 0033** (non-regression floor); #133 resolved
      (guard widened to the catalog surface); cross-tenant negative tests
      added for the catalog (#88) and library-membership (#262) surfaces.
- [x] `/code-review ultra` cross-check pass completed — 15 findings, 12 filed
      new (#293–#304), 3 cross-validated existing. Surfaced 4 Phase 15
      deliverables certified but never wired (retention reaper #294, nested
      redaction #296, diagnostics providers #295, expvar map #302).
- [x] Documentation updated (this README, the audit doc, audit README index,
      roadmap README, task list). ADR 0033 (coverage threshold) and ADR 0034
      (CI supply-chain posture) written.
- [x] Maintainer approval recorded — 2026-09-11, confirmed all 219
      `phase-16`-labeled issues closed (0 open, 0 Critical/High) and this
      phase ready to close.
