# Review: architecture-backend.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-backend.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (all three findings fixed — see Resolution below) |

## Summary

`internal/` as a real, compiler-enforced boundary plus a CI lint check
directly satisfies phase 01's own stated exit criterion and phase 02's
named risk. Two findings on the same pattern seen twice already this
session: FR-2's dependency-inversion design (repository interfaces defined
in the domain, using domain types) quietly answers a question phase 03's
own roadmap document reserves for itself ("whether repositories return
domain types or their own"), and FR-5's config precedence asserts
"environment variables (development only)" as if already established when
it's actually being decided here for the first time.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Scope | Phase 03's own "Architecture decisions expected" list includes "Whether repositories return domain types or their own, and where mapping lives" as open. FR-2 here (repository interfaces defined in `internal/domain`, implemented by persistence) structurally implies domain types — this substantially answers that question without saying so, the same silent-duplicate pattern review 0007 found between `architecture-desktop-host.md` and phase 05's roadmap doc | Update `03-backend-foundation/README.md`'s decisions-expected list to point at FR-2 as the decided pattern, same fix pattern as before |
| 2 | Minor | Overreach | FR-5 states "environment variables (development only — ADR 0004's addendum)" as if this were already established. ADR 0004's addendum was about the Postgres MCP tool's own `.env` sourcing for Claude Code sessions — a development-tooling concern, not a decision about how the Go server itself loads config in dev. This spec is deciding that now, not citing a prior decision | Reword FR-5 to present the dev-env-var allowance as a fresh decision made here, not implied as already settled elsewhere |
| 3 | Minor | Scope | Test strategy's "Concurrency" row says CI runs the lint check "on every PR" — CI mechanics are `architecture-testing.md`'s territory ("CI shape"), not decided here | Soften to note this requirement is *for* CI, implementation deferred, matching the established pattern from `architecture-contracts.md`'s FR-3 |

## Dimensions checked

- [x] **Completeness** — finding 1 is a real unstated-resolution gap
- [x] **Ambiguity** — FR-1 through FR-6 individually clear
- [x] **Architecture** — FR-1/FR-2/FR-3 together are the spec's strongest section: a real Go-idiomatic enforcement mechanism, not just a documented rule
- [x] **Domain correctness** — correctly stays out of phase 02's actual domain model while fixing the boundary around it
- [x] **Security** — import-boundary-as-security-control framing is genuine, not decorative
- [x] **Testability** — FR-3's "proven not asserted" acceptance criterion (a deliberate violation must fail CI) is the right bar
- [ ] **Accessibility** — not applicable
- [x] **UX and copy** — FR-4/FR-6 correctly route error messages through constitution §11's bar at the transport boundary, not the domain
- [x] **Observability** — correlation ID entry point fixed in the middleware chain (FR-6), consistent with `architecture-system.md`
- [x] **Maintainability** — the `internal/` decision is the kind of thing a future contributor would otherwise ask "why can't I just..." about, and now has a compiler error as the answer
- [x] **Evolution** — reversal cost not explicitly discussed, but package layout is cheap to reorganize before phase 03 writes real code against it, expensive after — worth stating explicitly, minor omission not rising to a separate finding

## Contradictions and gaps

Finding 1 is the substantive one, same category of issue as review 0007's
finding 7 — this project's phase 01 specs keep silently answering questions
their sibling phase documents reserve for themselves. Worth the maintainer
noting as a recurring pattern, not just fixing per instance.

## Resolution (2026-08-14)

All three findings fixed:

- **#1** — `03-backend-foundation/README.md`'s decisions-expected list now
  points at FR-2 as the decided pattern
- **#2** — FR-5 reworded to present the dev-env-var allowance as decided
  here, not implied from ADR 0004's (unrelated) addendum
- **#3** — Test strategy's Concurrency row now defers CI mechanics to
  `architecture-testing.md`, matching the established pattern

Also added, not from a numbered finding but surfaced in this review's own
"what I did not review" section: FR-3's spec text now explains directly why
it isn't redundant with FR-1 (`internal/` blocks external imports only;
nothing stops `internal/domain` importing `internal/persistence` inside the
same module without the lint check). Load-bearing for the spec's own
argument, so it belongs in the spec, not just in this review.

## What I did not review

Whether `internal/` alone (without the lint check) would already prevent
`internal/domain` from importing `internal/persistence` — it would not
(both are inside the same module, so `internal/` only blocks *external*
imports, not imports between internal packages of the same module). FR-3's
lint requirement is therefore not redundant with FR-1 — confirmed this by
reasoning about Go's visibility rules, not by testing an actual violation
in a real module, since no module exists yet.
