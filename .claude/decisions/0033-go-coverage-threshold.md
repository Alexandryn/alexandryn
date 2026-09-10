# 0033. Go unit-test coverage is reported in CI with a non-regression floor, not a fixed target

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-08 |
| **Deciders** | Maintainer, via Phase 16 (audit 0016 #131) |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 03's CI work (`backend-test-harness.md` FR-8, stage 7) added a
coverage step — `go test -race -coverprofile` plus `go tool cover -func`
— but deliberately left the tool and any threshold as an open question:
"No tool or threshold is fixed yet." The frontend runs no coverage at
all. Audit 0016 (#131) recorded this as a gap: a coverage number is
computed and uploaded every CI run but nothing acts on it, so a change
that deletes tests or adds a large untested code path is invisible.

At the time of this decision the backend unit suite (no build tag) sits
at **54.5%** statement coverage. That figure is low mostly because large
generated/wire and adapter surfaces are exercised only by the
integration-tagged suite, which the `-func` total does not include, and
because `cmd/` wiring is covered by integration tests. A hard target set
above the current number would fail CI immediately; a target set at the
current number would ratchet noise (coverage moves ±1–2% run to run as
table tests are added and removed).

What we know: the constitution's real requirement is §2 — "a test that
has never failed has not been shown to test anything", and the mandatory
RED→GREEN loop. Coverage percentage is a proxy, and a weak one. What we
do not know: whether a meaningful per-package target can be set without
first splitting the profile to include the integration suite.

## Decision

CI enforces a **non-regression floor**, not a fixed target:

- The coverage step computes the combined statement coverage of the
  unit suite and fails the build if it drops **more than 1.0 percentage
  point below the recorded baseline** in `scripts/coverage-baseline.txt`.
- Raising the baseline is a deliberate commit that edits that file, made
  when a phase's work lands with tests. Lowering it requires a recorded
  reason in the commit message and is expected to be rare.

**Baseline calibration (2026-09-09).** The initial `54.0` was taken from
a local `go test -coverprofile ./...` run while this ADR was drafted. The
gate's first real execution on a clean CI runner (Backend job, run
34417101994) measured **51.1%**, and the last pre-gate green run
measured 50.8% — the gate had never actually executed against CI's
environment when the number was chosen. A clean checkout has no
`web/node_modules`, so the two vendored `flatted/golang` packages that a
local `./...` compiles at 0% are absent, and `cmd/` wiring is exercised
only by the integration-tagged suite the `-func` total excludes. The
baseline is corrected to **`51.0`** (floor 50.0) to match the
environment that enforces it; Phase 16's Go changes were coverage-neutral
to slightly positive (`internal/transport/http` rose 54.7% → 56.4%).
- The absolute number is not a quality gate on its own. A reviewer still
  checks that new behaviour arrived with a failing-first test (§2); the
  floor only catches the case where that review missed a net deletion of
  test coverage.
- The frontend keeps `vitest run` without a coverage gate for now; its
  equivalent is the `check:*` guard scripts plus the Playwright/axe
  suites, which assert behaviour directly rather than line execution.

## Options considered

### Option A — a fixed per-package target (e.g. 80%)

Pros: an unambiguous number; common practice. Cons: the current tree is
nowhere near it and much of the gap is structural (integration-only
surfaces, `cmd/` wiring), so it would either fail CI on day one or force
a batch of low-value "coverage tests" written to hit lines rather than
assert behaviour — the exact anti-pattern §2 warns about. Rejected.

### Option B — the current per-run number as a hard floor

Pros: no immediate failure. Cons: run-to-run variance of ±1–2% would
make CI flaky; every PR that happened to remove a redundant table case
would go red. Rejected in favour of the 1-point tolerance band.

### Option C — report only, never fail (the status quo)

Pros: zero friction. Cons: this is what #131 filed as the gap — a number
nobody acts on. Rejected.

### Option D — split the coverage profile to include the integration suite, then set a target

The honest long-term answer. Deferred: it needs the two suites' profiles
merged (`go test` writes one profile per invocation) and is more work
than Phase 16's remediation scope. Recorded here so the next person knows
the floor is an interim measure, not the intended end state.

## Consequences

- A new `scripts/check-coverage.sh` reads `coverage.out` and
  `scripts/coverage-baseline.txt` and enforces the band; it has a
  `_test.sh` self-test like the other guard scripts.
- CI stage 7 gains a "Coverage floor" step after the existing summary.
- The baseline file is under `/scripts` CODEOWNERS coverage already
  (`.github/` and root manifests are called out; `scripts/` is covered by
  the catch-all `* @luannmoreira`).
- When Option D is done, this ADR is superseded rather than edited.
