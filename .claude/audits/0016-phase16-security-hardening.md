# Security audit: Phase 16 — Whole-application security hardening

| | |
|---|---|
| **Scope** | The entire application at branch tip `feat/phase16-security-hardening` (`origin/main` = `1c60608`): `cmd/`, `internal/`, `web/src/`, `electron/src/`, `scripts/`, `.github/workflows/`, `api/`, `docker-compose.yml`, `Dockerfile`, `supabase/`, dependency manifests (`go.mod`, `go.sum`, `package.json`, `package-lock.json`). |
| **Auditor** | Claude Sonnet 5 (Claude Code) — specialist fan-out: `agent-skills:security-auditor`, `agent-skills:code-reviewer`, `agent-skills:test-engineer`, `agent-skills:web-performance-auditor`; plus `security-and-hardening` and `ci-cd-and-automation` skills inline; plus a separate maintainer-run `/code-review ultra` cross-check. |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE, applied to the application as a whole across the three phase-01 trust boundaries: Renderer/Main, Host/LAN, System/Source. |
| **Date** | 2026-09-07 |
| **Commit** | `1c60608` |
| **Verdict** | **In progress** — sweep running; findings filed as GitHub issues as they are confirmed. |

---

## Scope and method

This is a consolidation audit (Phase 16, roadmap). Method:

1. **Code reading**, every wired path across the three trust boundaries. Not a
   sampling — the preload surface is enumerated, every HTTP handler is listed and
   traced, every source-facing parser is read.
2. **Mandatory authorization trace.** For every endpoint that reads or writes
   data scoped to a user, a library, a device, or any other tenant boundary:
   the *actual wired* handler → repository → SQL is traced and the tenant/user
   predicate is quoted from the query text (or the middleware check). A scoped
   repository method existing, a migration column, or spec text is **not**
   accepted as evidence. This is the control that audit 0012 got wrong (review
   0050): per-user reading scoping and token-type assertion were certified from
   the repository and migration layers without tracing one request to its SQL.
3. **Dependency review** — `govulncheck`, `npm audit`, and a license pass over
   direct and transitive dependencies, cross-checked against §9's recorded
   reasons.
4. **CI/CD review** — the GitHub Actions workflow's supply-chain surface (action
   pinning, `GITHUB_TOKEN` permissions, cross-job artifact trust, secret
   exposure, `pull_request_target`), and the `scripts/check-*` guard scripts.
5. **Automated-testing review** — coverage on security-relevant paths, presence
   of cross-tenant/negative tests, skipped or flaky tests, the unset coverage
   threshold.
6. **Manual probing** where a static read is inconclusive.

Every confirmed finding is filed as its own GitHub issue (label scheme in the
phase README, G0-3) and recorded in the Findings table below with a link. The
sweep does not triage findings away at capture time (G0-1).

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| Renderer → Main (IPC) | The renderer process and anything it loads | The preload exposes an enumerated operation list; every argument is validated in the main process |
| Main → OS | Book files, paths, spawned processes | Paths from sources are attacker-chosen strings; the pg supervisor is spawned with a fixed argv |
| LAN client → Host (HTTP API) | Any device on the network, and a hostile page in the local user's browser | Loopback-default bind; broader bind requires auth + ADR 0017 TLS; every data handler is user-and-library scoped |
| Authenticated user → API | A credentialed user wanting more than their share | Every tenant-scoped query carries a `user_id` / `library_id` predicate; token type is asserted, not just signature |
| Source → System | A user-configured source: its metadata, filenames, redirects, response bytes | Size limit, shape check, timeout on every response; filenames sanitised; redirects bounded |
| Book file → Parser | A file that is legal to possess but hostile to parse | Zip-bomb / path-traversal / malformed-container defence in the EPUB/format path |
| CI job → CI job | Artifacts and state passed between GitHub Actions jobs | `web/dist` and the server binary handed between jobs are trusted; the workflow token is minimally scoped |
| Dependency tree | Every direct and transitive package | No known CVE at or above `high`; each direct dependency has a recorded reason (§9) |

## Adversarial questions asked

Worked explicitly per Constitution §10, against the whole application:

- **Malicious user of this instance** — what cross-tenant read/write/delete is
  reachable? Token confusion between access, MFA, and pairing grants? Role
  escalation through an unguarded handler?
- **Malicious source** — what do its metadata, filenames, redirects, and
  response bytes achieve? Oversized/slow/never-arriving responses? SSRF via a
  source URL or redirect?
- **Unauthenticated LAN device** — what is reachable without a credential? What
  does a hostile page in the local user's browser reach via CSRF / DNS
  rebinding? Do the security headers and CSP hold?
- **Malformed / empty / enormous / duplicated / slow / absent input** — on every
  external surface: IPC, HTTP body, source response, book file, sync payload.
- **What is logged that shouldn't be** — credentials, tokens, home paths, book
  content, reading position — re-verified against every log call, including
  those added since the phase 15 redaction test.
- **What happens if two run at once** — concurrent sync, concurrent import,
  concurrent job cancellation, reaper vs. writer.

## Findings

Filed as GitHub issues; this table is the index. Severity rated per the guide
below — impact in this system, not the textbook worst case.

| ID | Severity | Title | Issue | Status |
|---|---|---|---|---|
| _(populated as the sweep confirms findings)_ | | | | |

## Severity guide

| | |
|---|---|
| **Critical** | Remote code execution, or unauthenticated access to the library from the network |
| **High** | Authentication bypass, arbitrary file read/write, renderer sandbox escape, credential leakage |
| **Medium** | Requires user interaction or an unusual configuration; limited data exposure; persistent DoS |
| **Low** | Narrow impact, high preconditions, or defence-in-depth that's missing |
| **Informational** | No impact today, but it makes a future mistake more likely |

Non-security findings (code quality, user-flow, workflow, test-coverage) are
filed with the same `severity:` labels used as a priority proxy and an
`area:` label, per G0-1/G0-3.

## What was not examined

_(Recorded honestly at the end of the sweep — boundaries, files, or scenarios
that time or tooling did not reach.)_
