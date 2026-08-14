# Spec: Testing architecture

| | |
|---|---|
| **Status** | `REVIEWED` (self, approved with changes) |
| **Phase** | `01-architecture` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`.claude/reviews/0013-spec-architecture-testing.md`](../reviews/0013-spec-architecture-testing.md) — Approved with changes, both findings fixed; self-reviewed, independent read still pending |

## Context

Phase 01's own text calls this the highest-leverage document in the phase —
"defines how everything after this phase is tested." Every sibling spec
this phase produced deferred at least one testing or CI decision here
rather than re-deciding it locally: `architecture-desktop-host.md` deferred
the CI-side Electron E2E tool (twice, across its own self-review passes);
`architecture-backend.md` deferred the import-boundary lint's actual CI
mechanics; `architecture-contracts.md` deferred the contract-test tool;
`architecture-persistence.md` deferred the integration-test harness against
real PostgreSQL. This spec is where those all get resolved, not
re-deferred again.

Separately, the maintainer has stated directly (not inferred) that GitHub
Actions is the CI platform, and that its scope grows well beyond tests:
lint, QA automation, backend and PostgreSQL testing, Docker image
vulnerability scanning, and dependency/version vulnerability scanning.
This spec incorporates that as fixed direction, not an assumption.

## Problem

Nothing has fixed: the CI platform (now stated directly — GitHub Actions),
the distinction between an interactive AI-assistant tool (the Playwright
MCP available to Claude Code sessions) and a real, headless, CI-runnable
test framework, how integration tests reach a real PostgreSQL, or where
Docker/dependency vulnerability scanning fits in the pipeline.

## Goals

- Fix GitHub Actions as the CI platform, stated as direction, not
  discovered per sibling spec
- Resolve the Electron E2E tool question `architecture-desktop-host.md`
  deferred twice: a real headless test framework, distinct from the
  interactive Playwright MCP
- Fix how integration tests reach real PostgreSQL in CI, distinguishing
  "any Postgres to test repository code against" from "the actual bundled-
  spawn mechanism `architecture-persistence.md` specifies," which are two
  different testing needs
- Fix where Docker image scanning and dependency/vulnerability scanning sit
  in the pipeline, with a severity bar reusing this project's own existing
  Critical/High/Medium/Low/Informational taxonomy (`audits/README.md`)
  rather than inventing a new one
- Fix determinism and fixture conventions

## Non-goals

- The actual GitHub Actions YAML — phase 03 writes the first real workflow
  file, against this spec's decisions, not before there's a first thing to
  build and lint
- Specific lint rule configuration (which `golangci-lint` rules, which
  `eslint` config) — phase 03/04's to tune
- Release/versioning automation specifics (semantic-release or similar) —
  phase 99's territory
- Load/performance testing — not raised by any prior spec, not invented
  here; flagged as an open question with no owner

## User stories

- As **phase 03**, I want the first CI workflow to implement a fixed
  pipeline shape, not invent one under the pressure of "we need SOME CI
  running."
- As **the maintainer**, I want Docker and dependency vulnerability
  scanning to actually block a release, not exist as a report nobody reads.
- As **a future contributor**, I want to know that the Playwright MCP
  Claude Code uses during development and the E2E suite that runs in CI
  are two different things on purpose, not a confusing overlap.

## Testing layers

Phase 01 names "Layers" specifically as this spec's job. One table, the
whole system, not scattered across sibling specs' individual Test strategy
sections:

| Layer | Runs against | Tool | Owning spec |
|---|---|---|---|
| Build (ordered, before anything else) | `web/` first, then `go build ./cmd/server` (FR-8) | npm build, then `go build` | ADR 0008 |
| Unit | Nothing external, pure functions | Go's `testing`, a JS test runner (phase 04's pick) | Every spec, own code |
| Integration (backend) | Service-container Postgres (FR-3) | Go's `testing` + `database/sql` against real Postgres | `architecture-backend.md`, `architecture-persistence.md` |
| Integration (persistence spawn) | The application's own bundled-Postgres mechanism (FR-3, kept separate from the above) | A dedicated suite, not the fast per-PR one | `architecture-persistence.md` FR-1/FR-8/FR-9/FR-10 |
| Contract | Real Go handlers vs. the OpenAPI spec | Tool TBD, phase 03 | `architecture-contracts.md` FR-3 |
| E2E (web) | A real browser | Playwright MCP (dev-time), `@playwright/test` (CI) | `architecture-frontend.md` |
| E2E (Electron) | A real Electron app, `xvfb`-backed in CI | `@playwright/test`'s `_electron` (FR-2) — never the MCP | `architecture-desktop-host.md` |
| Accessibility | Rendered content | Playwright MCP's accessibility snapshot | `architecture-frontend.md` FR-5 |
| Lint / static | Source code, no execution | Go linter + import-boundary check (`architecture-backend.md` FR-3), JS linter (phase 04) | `architecture-backend.md` |
| Security scanning | Built artifacts and dependency manifests | Dependency scanner (FR-5), Docker scanner (FR-4) | This spec |

## Functional requirements

- **FR-1** GitHub Actions MUST be the CI platform. Every PR MUST run, at
  minimum: lint, unit tests, integration tests, contract tests
  (`architecture-contracts.md` FR-3), before merge is possible — this is
  the concrete mechanism behind phase 00's already-recommended branch
  protection ("require status checks to pass once CI exists") and
  `architecture-backend.md` FR-3's import-boundary lint requirement.
- **FR-2** Electron end-to-end tests (resolving `architecture-desktop-host.md`'s
  twice-deferred question) MUST use `@playwright/test`'s own Electron
  support (`_electron`), run in GitHub Actions on a Linux runner with a
  virtual display server (`xvfb` or equivalent) — **not** the Playwright
  MCP, and not assumed to run "headless" the simple way a browser does:
  Electron is a GUI application and needs a display to launch at all on
  Linux CI, whether or not a window is ever actually shown on screen.
  Exact setup step (a GitHub Action like `xvfb-action`, or a manually
  wrapped `xvfb-run`) is phase 03's to pick; the requirement that a
  display server exists in CI is fixed here so it isn't discovered as a
  surprise mid-implementation. These are different tools for different
  purposes: the
  MCP is interactive, scoped to a Claude Code session, used for
  development-time exploration and the accessibility-snapshot checks
  `architecture-frontend.md`'s Test strategy already relies on correctly;
  `@playwright/test` is a real npm dependency, runs unattended in CI, and
  is what actually gates a merge. Conflating the two was the exact mistake
  `architecture-desktop-host.md`'s self-review caught and corrected once
  already — this FR exists so it doesn't need correcting a second time.
- **FR-3** Integration tests against PostgreSQL in CI MUST use a plain
  Postgres service container (GitHub Actions' native `services:` support),
  not the application's own bundled-Postgres-spawn mechanism
  (`architecture-persistence.md` FR-1/FR-8/FR-9/FR-10) — those are two
  different things under test. Ordinary repository/handler integration
  tests need *a* Postgres to run against, fast and disposable; testing the
  spawn mechanism itself (does the Go server actually initialize, spawn,
  and orphan-prevent a bundled Postgres correctly) is its own, separate
  test target, closer to `architecture-system.md`'s own E2E walkthrough
  than to routine backend integration tests, and MUST NOT be conflated
  with the fast, service-container-backed suite that runs on every PR.
- **FR-4** Docker images (the self-hosting stack, README's stated packaging)
  MUST be scanned for known vulnerabilities before a release is tagged
  (phase 99). Findings MUST be rated using this project's existing
  Critical/High/Medium/Low/Informational taxonomy (`audits/README.md`),
  not a scanner-tool-specific scale — a Critical or High finding blocks
  release, same bar `audits/README.md` already sets for security audit
  findings generally. Specific scanner: phase 99's to pick.
- **FR-5** Dependency vulnerability scanning MUST run in CI for both the Go
  module (`govulncheck` — already implied by phase 03's own risk
  mitigation, "dependency audit in CI from the first dependency," now made
  concrete) and the JS/TypeScript dependencies (an SCA tool — specific
  choice deferred to phase 04). Same severity bar and blocking behavior as
  FR-4.
- **FR-6** Tests MUST be deterministic — a flaky test is a defect to fix,
  not a step to retry until green. This restates phase 03's own "a clock
  that tests control" requirement as a project-wide rule, not
  backend-specific.
- **FR-7** Test fixtures MUST be generated programmatically (seed
  functions, factories) rather than checked-in binary or hand-maintained
  snapshot files that can drift silently from what they claim to
  represent.
- **FR-8** (Added 2026-08-14, ADR 0008) CI MUST build `web/` before
  `go build ./cmd/server` — ADR 0008's `go:embed` decision means `web/`'s
  output is embedded into the Go binary, so a build that skips this step
  doesn't fail, it silently ships a server with a stale or missing
  frontend. This MUST be an explicit, ordered step in the pipeline, not an
  assumption that whoever writes the workflow file remembers the
  dependency.

## Non-functional requirements

- **Performance** — CI pipeline duration isn't budgeted here (no pipeline
  exists yet to measure); phase 03 sets a real number once there's
  something to time, same placeholder pattern used everywhere else in
  phase 01.
- **Security** — FR-4/FR-5 *are* the security requirement at this spec's
  level: vulnerabilities caught before release, not after.
- **Accessibility** — `architecture-frontend.md` FR-5 is the requirement;
  this spec's job is only to confirm the Playwright MCP's accessibility
  snapshot is a real, available mechanism for checking it during
  development (confirmed, not assumed — `architecture-desktop-host.md`'s
  self-review already verified the MCP's actual tool surface).
- **Reliability** — FR-6's determinism requirement is what keeps a red CI
  check meaningful; a pipeline anyone's learned to ignore because it's
  flaky is worse than no pipeline.
- **Observability** — not applicable at this spec's level beyond what CI
  itself reports (pass/fail, which check).

## Domain model

Not applicable — this spec is process/tooling, not the Alexandryn domain.

## API and contracts

Not applicable in the usual sense — this spec's "contract" is the CI
pipeline's own stages and what gates merge, fixed in FR-1.

## State transitions

Not applicable.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| A PR's tests fail | GitHub Actions run | Red status check, blocking merge | Nothing merges until fixed — no override path assumed, branch protection (phase 00's recommendation) is what enforces this once applied |
| A dependency scan finds a Critical/High CVE | FR-5's scan step | A failed check naming the vulnerable package and severity | Blocks merge/release, same as any other Critical/High finding under this project's existing taxonomy |
| A test is flaky (passes/fails nondeterministically) | Repeated CI runs disagree on the same commit | Confusing, erodes trust in CI | Per FR-6, this is a defect to fix, not something to route around with retries — no auto-retry-on-failure mechanism is assumed or recommended |

## Security considerations

- **FR-4/FR-5 are the whole point** — this spec's main security
  contribution is making vulnerability scanning a blocking CI gate, not an
  advisory report. Restating constitution §9 ("dependencies are
  liabilities") as an enforced check rather than a reviewer's memory.
- **CI secrets** — not designed in detail here (no CI workflow exists
  yet), but flagged: any credential a future GitHub Actions workflow needs
  (registry push tokens, signing keys for phase 99) must follow the same
  "never logged, never in an error" bar constitution §8 already sets for
  the application itself. Real design work for phase 03/99 when there's an
  actual secret to handle.

## Test strategy

This spec doesn't have a separate "what tests this" layer table the way
feature specs do — it *is* the test-strategy document for every layer
beneath it. Its own verification is the same walkthrough method phase 01
uses elsewhere: trace one real CI run (once phase 03 has something to
build) through lint → unit → integration → contract → merge, and confirm
every gate in FR-1 actually exists and actually blocks.

## Acceptance criteria

- [ ] A GitHub Actions workflow exists implementing FR-1's minimum gate set
- [ ] Electron E2E proven via `@playwright/test`, not the MCP — a
      deliberately broken lifecycle state (per `architecture-desktop-host.md`)
      caught by this suite, not just described as possible
- [ ] Integration tests run against a service-container Postgres, separate
      from a dedicated test proving the bundled-spawn mechanism itself
      (FR-3's distinction, not collapsed into one suite)
- [ ] Docker and dependency scans block on Critical/High, proven with a
      deliberately vulnerable dependency in a test branch, not just
      configured and assumed working

## Open questions

- **Load/performance testing** — not raised by any prior spec, no owner
  named here. Flagged, not invented.
- **CI pipeline duration budget** — placeholder-free, genuinely no number
  yet, phase 03's to set once there's something to time.
- **Specific scanner tools (FR-4, FR-5's JS side)** — deferred to phase
  99/04 respectively, per the established pattern-here/tool-there layering
  this entire phase has used throughout.

## References

- `architecture-desktop-host.md` — the Electron E2E question this spec
  resolves (FR-2), self-review 0007's finding 3
- `architecture-backend.md` FR-3 — import-boundary lint, the CI mechanics
  this spec fixes
- `architecture-contracts.md` FR-3 — contract test, same pattern
- `architecture-persistence.md` — FR-1/FR-8/FR-9/FR-10, the bundled-spawn
  mechanism FR-3 here distinguishes from routine integration tests
- `architecture-frontend.md` — Playwright MCP's confirmed real capability
  for development-time accessibility checks, distinct from this spec's
  CI-side E2E tool
- `.claude/roadmap/03-backend-foundation/README.md` — "dependency audit in
  CI from the first dependency," made concrete by FR-5
- `.claude/audits/README.md` — the severity taxonomy FR-4/FR-5 reuse
  rather than inventing a new one
- `.claude/roadmap/00-foundation/README.md` — branch protection
  recommendation this spec's FR-1 gives a concrete mechanism to enforce
- Constitution §8 (no logging secrets), §9 (dependencies are liabilities)
