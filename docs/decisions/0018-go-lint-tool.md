# 0018. General Go static analysis uses `golangci-lint`

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-18 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

`architecture-backend.md`'s Open questions left "lint tool choice" for
phase 03 to pick (its own FR-3 requires *some* automated lint, but named
no tool). Separately, `backend-test-harness.md`'s Non-goals section
already referred to "the specific `golangci-lint` rule configuration"
by name, written before this ADR existed — an assumption with no formal
decision behind it, found while validating phase 03's CI readiness
before any workflow file was written. This ADR is that decision, made
formal; `architecture-backend.md` FR-7 and `backend-test-harness.md`'s
Non-goals are both corrected to cite it instead of assuming it.

This is one of two lint concerns phase 03 needs, and they are
deliberately not the same mechanism: *general* Go static analysis
(unused code, staticcheck-class issues, common mistakes) is this ADR's
scope. The *import-boundary* rule specifically (`architecture-backend.md`
FR-2/FR-3 — `internal/domain` must not import `internal/transport` or
`internal/persistence`) is not a stock lint rule any general-purpose Go
linter ships with opinions about; `tasks/plan.md`'s D0 already commits
that check to an interim grep/file-walk script
(`scripts/check-import-boundaries.sh`), revisited as a `golangci-lint`
custom rule or `go/analysis` pass once CI exists to observe it running.
This ADR does not reopen that decision.

## Decision

General Go lint in CI runs through `golangci-lint`, configured by a
project-root `.golangci.yml` committed alongside the first CI workflow
(phase 03's own T0/T27 tasks). The specific rule set and its tuning are
explicitly out of scope here — `backend-test-harness.md`'s own Non-goals
already deferred that to phase 03/04's tuning, and this ADR doesn't
re-decide it, only which tool runs.

## Options considered

### Option A — `golangci-lint` (chosen)

*For* — a single meta-linter bundling `go vet`, `staticcheck`,
`errcheck`, `unused`, and dozens of other analyzers behind one config
file and one CI invocation, rather than wiring each analyzer as its own
step. Already the tool this project's own prose assumed by name in two
places (`CONTRIBUTING.md`'s tooling table, `backend-test-harness.md`'s
Non-goals) before any decision selected it — adopting it formally makes
that existing prose accurate instead of introducing something new.
Widely used in the Go ecosystem, actively maintained, config format is
stable enough to commit without expecting frequent breaking changes.

*Against* — one more dependency to track through
`architecture-testing.md` FR-5's vulnerability scanning and this
project's own dependency audit, same as any other tool. Its own bundled
analyzer set changes across `golangci-lint` releases, which means a
version bump can change what fails CI without any code changing — pin
the version in CI, same discipline this project already applies to
other dependencies (constitution §9).

### Option B — `go vet` alone, no meta-linter

*For* — zero new dependency, already required regardless (`go vet` is
part of the standard toolchain, and `backend-test-harness.md` FR-8 stage
3 already names it separately from "lint").

*Against* — catches a materially smaller class of defects than
`staticcheck`-class analysis (unused code, some categories of
correctness bugs `go vet` doesn't check). Phase 03's own risk table
already worries about global-state and convention drift creeping in
early; a meta-linter catching more of that class in CI, automatically,
is exactly the kind of tooling this phase is meant to establish once
rather than improvise per contributor.

### Option C — Individual analyzers wired separately (`staticcheck`,
`errcheck`, etc., each its own CI step)

*Against* — reproduces what `golangci-lint` already does as
orchestration, with more CI configuration to maintain and no offsetting
benefit; `golangci-lint` is itself a thin wrapper over these same tools,
not a replacement for them, so this option buys nothing `golangci-lint`
doesn't already provide with less config.

## Consequences

**Good** — one config file, one CI step, and one local command
(`golangci-lint run`) covering the general-lint surface FR-3 requires;
closes the gap between what `backend-test-harness.md` already assumed
by name and what had actually been decided.

**Bad** — a new dependency to justify under constitution §9. What it
does: runs a configurable set of static-analysis checks against the Go
module. Why not the standard library alone: `go vet` alone catches a
narrower class of defects, named above (Option B). What breaks if it's
abandoned: low risk — `golangci-lint` is an orchestrator over
independently-maintained analyzers (`staticcheck`, `errcheck`, and
others), each of which keeps working standalone if the orchestrator
itself stalls; the fallback is running the underlying analyzers
directly, not a rewrite. Pin the version in CI (`.golangci.yml` and the
CI step both) so a new release can't silently change what fails a PR.

**Neutral** — doesn't change FR-2/FR-3's import-boundary rule, which
stays D0's interim script per `tasks/plan.md`, untouched by this ADR.

## Reversal cost

Low. `golangci-lint` wraps independently-usable analyzers; dropping it
means running `go vet` plus whichever of its wrapped tools still matter
directly, not rewriting any application code — nothing in the codebase
depends on `golangci-lint`'s own presence at runtime, only in CI.

## Confidence

High. This is the least contested of phase 03's tooling decisions — the
project's own prose had already assumed this tool by name in two places
before any ADR existed to back it; this decision brings the paper trail
in line with what was already, in practice, the working assumption.
