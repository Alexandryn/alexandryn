# Phase 18 — Human Codebase Cleanup and Release Hygiene

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 17 |
| **Blocks** | Phase 99 |
| **Opened** | — |
| **Closed** | — |

## Gate 0 decisions (maintainer approved)

| # | Decision |
|---|---|
| G0-1 | **Standard & Purpose**: The goal is not to add features or overhaul architecture. The goal is to make the codebase read like an intentionally maintained, human-crafted software project. Code communicates intent through structure, naming, and types; comments remain only when they explain non-obvious *why*, security constraints, or domain rules the code cannot express. Zero unnecessary comments, not zero comments. |
| G0-2 | **Behavioral Preservation Invariant**: Under Constitution §1, comment pruning and pure readability refactors are exempt from specs, provided *no behavior changes*. Security controls, size limits, shape checks, timeouts (§4), Electron privilege boundaries (§5), tenant-scoped queries (§3, §6), and accessibility semantics (§7) must never be relaxed or deleted. |
| G0-3 | **Tooling & Execution Engine**: Guided by `/caveman:caveman` for token-efficient session dialogue (chat terse like smart caveman; all code comments, commits, and user-facing copy written in polished, standard human prose). Leverages `using-agent-skills`, `code-simplification`, `safe-refactor`, and `code-review-and-quality`. Uses `cavecrew-builder` for bounded surgical file edits and `code-reviewer` for human readability and regression reviews. |
| G0-4 | **Process Residue & De-AI Sweep**: Systematic purge of AI conversational residue, prompt echoes, development-phase narration (`Phase XX`), functional requirement references (`FR-X`), spec tracking tokens (`P0`, `T24`, `P4-G`), bug tracker / audit breadcrumbs (`audit 0016 #NNN`, `review 0050`), obvious code-paraphrasing comments, and stuttering docstrings across Go, TypeScript, React, Electron, scripts, and SQL. |
| G0-5 | **Verification Floor**: All CI quality gates must remain 100% green without regressions: Go unit, integration, and contract tests with `-race`, coverage floor (`check-coverage.sh`), all shell guard scripts, linters, frontend TypeScript/Vite builds, Storybook, bundle checks, Playwright cross-browser matrix, Electron host lifecycle tests, and the Docker container target. |

## Dependency status at open (verified, not trusted from roadmap)

**Phase 17 (Accessibility and QA):** Must be closed on `main`. All WCAG 2.1 AA audits
complete, keyboard walkthroughs verified, cross-browser Playwright matrix passing,
and scale benchmarks (10k items) holding performance budgets.

Phase 17 closes before Phase 18 begins.

## Objective

At the end of this phase, the Alexandryn repository is free of development-process
residue and conversational AI artifacts. All code expresses its intent clearly
through structure and naming, unnecessary abstraction is removed, and every
remaining comment conveys critical, non-obvious engineering rationale. The repository
reads as a cohesive, mature, production-grade codebase ready for public release.

## Why here

Alexandryn evolved across 17 intensive phases, with dozens of automated and
human sessions leaving developmental markers: phase numbers, functional requirement
tags, audit citations, and conversational notes.

Running this cleanup earlier would have been premature: subsequent implementation,
security hardening (Phase 16), and accessibility remediation (Phase 17) would have
continued to inject new developmental scaffolding. Running it after release (Phase 99)
means publishing a codebase cluttered with internal process residue. Phase 18 is the
natural hygiene boundary before packaging, versioning, and publishing.

## Scope

**In**

- **Go Backend (`cmd/`, `internal/`)**:
  - Pruning phase comments (`// Phase 08's source-adapter layer`, `// Phase 10: Import pipeline...`).
  - Removing requirement and audit tags (`(FR-4 amendment)`, `(audit 0016 #205)`, `(review 0050)`). Where a comment preserves a vital security rationale (e.g., token type verification or tenant-scoped queries), rewrite it as timeless engineering rationale without bug-tracker citations.
  - Removing code-paraphrasing comments and stuttering docstrings (`// User represents a user`).
  - Simplifying convoluted local logic without altering behavior.
- **Web Frontend (`web/src/`)**:
  - Pruning redundant JSDoc comments that merely restate TypeScript types.
  - Removing framework-explaining comments and development milestone annotations (`D2, tasks/plan-phase04.md`).
  - Simplifying unwieldy component and hook abstractions, removing redundant state where direct derived values suffice.
- **Desktop Host (`electron/src/`)**:
  - Cleaning preload, main process, and boot scripts of developmental notes.
  - Verifying comments around Electron privilege boundaries focus strictly on security guarantees.
- **Migrations & Infrastructure (`internal/persistence/postgres/migrations/`, `scripts/`, `Dockerfile`, `docker-compose.yml`, `.github/workflows/ci.yml`)**:
  - Cleaning operational comments and CI explanations of phase-by-phase narration.
  - Keeping technical explanations of complex CI caching, xvfb requirements, or compose profiles while stripping temporary audit pointers.
- **Documentation & Root Files (`README.md`, `CONTRIBUTING.md`, `SECURITY.md`)**:
  - Ensuring documentation speaks directly to human users, operators, and contributors.
  - Removing temporary roadmap notes or instructions written for AI agents.

**Out**

- **New features or schema changes**: Zero behavioral changes or new capabilities.
- **Architectural redesign**: Domain boundaries (Metadata, Source, Alexandryn) remain strictly isolated (Constitution §3).
- **Removing engineering safeguards**: Size limits, shape checks, timeouts, context cancellation, and validation logic remain inviolable (Constitution §4).
- **Collapsing security boundaries**: Electron preload restrictions, loopback binding, and fail-closed TLS invariants are immutable (Constitution §5, §6).
- **Modifying user-facing copy**: Interface copy adheres to Constitution §11 and remains unchanged unless correcting an error.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Simplification inadvertently alters behavior or weakens input validation/timeouts | Low | High | Strict adherence to Constitution §1 & §4; zero tolerance for behavior modification; every edit bracketed by unit/integration tests with `-race`. |
| Pruning comments strips critical, non-obvious domain or security rationale | Medium | Medium | Comment quality standard: retain the *why*; rewrite raw issue/phase citations (`audit 0016 #205`, `Phase 12`) into durable architectural explanations rather than deleting blindly. |
| Removing comments in SQL migrations breaks migrations checksum or history | Low | High | In migrations (`internal/persistence/postgres/migrations/`), leave applied migration headers intact if tooling or checksums depend on them; focus comment pruning on active application source files. |
| Redundant state refactoring in React breaks edge-case UI rendering or a11y | Low | Medium | Run component tests (`npm test`), token styling checks, Storybook builds, and Playwright cross-browser tests after any frontend refactor. |

## Test strategy

This phase modifies comments and refactors local complexity without adding features. The test strategy is **bracketed regression verification**:
- Run `go test -race ./...` and `npm test` before touching a subsystem to confirm a baseline green state.
- Make targeted, bounded edits (leveraging `cavecrew-builder` for surgical single-file passes).
- Re-run local package tests immediately after each edit.
- Run the full CI gate (shell guards, integration tests, Playwright cross-browser suite, Docker target) before marking the phase complete.

## Security considerations

Code simplification must never be used as a pretext to relax security boundaries:
- **Tenant isolation**: Scoped queries (`user_id`, `library_id`) must remain untouched in repositories and handlers (§3, §6).
- **Host exposure & TLS**: Loopback binding and ADR 0017 fail-closed TLS logic must not be altered (§6).
- **Electron privileges**: Context isolation, sandboxing, disabled Node integration, and strict preload IPC validation are non-negotiable (§5).
- **Observability**: Redaction and private data protections (§8) must be preserved.

## Exit criteria

- [ ] Repository-wide search confirms zero unnecessary "Phase XX", "FR-XX", "audit 00XX", or AI agent references in source files
- [ ] Every remaining comment passes the Comment Quality Standard (explains non-obvious *why*, not *what*)
- [ ] No behavioral regressions introduced; code simplifications are local and verified
- [ ] All shell guard scripts pass:
  - `bash scripts/check-gofmt.sh .`
  - `bash scripts/check-import-boundaries.sh .`
  - `bash scripts/check-parameterized-queries.sh .`
  - `bash scripts/check-compose-published-port.sh .`
  - `bash scripts/check-user-scoped-reading.sh .`
  - `bash scripts/check-license.sh .`
  - `bash scripts/check-coverage.sh .`
- [ ] Backend tests and linters pass:
  - `go vet ./...`
  - `golangci-lint run ./...`
  - `go test -race -coverprofile=coverage.out ./...`
  - `go test -race -tags=integration ./...`
  - `go test -race -v ./internal/testutil/contracttest/...`
  - `govulncheck ./...`
  - `gosec -quiet -severity high -confidence high ./...`
- [ ] Frontend tests, checks, and builds pass:
  - `npm run -w web build` (`tsc -b && vite build`)
  - `npm run -w web lint`
  - `npm run -w web test`
  - `npm run -w web check:token-styling`
  - `npm run -w web check:a11y-tabindex`
  - `npm run -w web check:a11y-hidden-text`
  - `npm run -w web check:bundle-size`
  - `npm run -w web check:radix-dedup`
  - `npm run -w web check:dist-secrets`
  - `npm run -w web check:dist-msw`
  - `npm run -w web build-storybook`
  - Playwright test suite (`npx playwright test`)
- [ ] Desktop tests and builds pass:
  - `npm run -w @alexandryn/desktop build`
  - `npm run -w @alexandryn/desktop typecheck`
  - `npm run -w @alexandryn/desktop lint`
  - `npm run -w @alexandryn/desktop test`
  - `npm run -w @alexandryn/desktop test:e2e`
- [ ] Container-target test passes:
  - `docker compose --profile bundled-db up --build --wait --wait-timeout 120`
- [ ] Final human-readability review completed and report generated
- [ ] Maintainer approval recorded
