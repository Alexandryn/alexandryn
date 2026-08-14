# Review: ADR 0005 — Process model, prototype-backed

| | |
|---|---|
| **Subject** | `.claude/decisions/0005-process-model.md` |
| **Reviewer** | Luann Moreira |
| **Date** | 2026-08-13 |
| **Verdict** | Approved |

## Summary

Same conflict-of-interest caveat as the earlier backfilled reviews: I wrote
the ADR and the prototype it cites, and am reviewing my own work. The core
claim (two processes, Linux orphan-prevention via `prctl`) is backed by an
actual run, not just argument — a real improvement over 0001/0003/0004's
process gap. Two things weren't tested that the ADR's confidence language
could be read as covering.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Minor | Prototype coverage | The prototype's graceful-shutdown test only exercises the fast happy path (child exits in 0.10s). FR-9 also requires a bounded grace period before falling back to a hard kill — that fallback path was never triggered or tested, because the mock server has nothing that could hang | Note explicitly in the ADR or prototype README that the hard-kill fallback is untested; phase 03/05 needs a test with a deliberately slow/hung handler to actually exercise it |
| 2 | Nit | Rigor asymmetry | Option A (chosen) got empirical treatment; Options B and C were rejected by reasoning alone, no prototype attempted for either | Defensible — cgo cross-compilation friction and system-service install friction are both well-documented failure modes, not close calls — but the ADR should say so explicitly rather than let the asymmetry pass silently. (Now said, in this review; consider folding a line into the ADR itself.) |
| 3 | Informational | Mechanism scope | `prctl(PR_SET_PDEATHSIG)` is documented as tracking the calling *thread*, not the process, and Node.js is multi-threaded (libuv thread pool) even though this prototype's parent script is a simple single-purpose script that never exercises that edge. The general mechanism is standard and widely used; this specific prototype didn't stress it under anything resembling Electron's actual runtime characteristics | No required change — flagging so "verified" in the ADR is read as "verified under prototype conditions," not "verified under production load." Phase 05's real integration test is what actually closes this. |

## Dimensions checked

- [x] **Completeness** — decision, three options, consequences, reversal, confidence all present; prototype linked and reproducible
- [x] **Ambiguity** — clear: two processes, named mechanism per platform, explicit about which platforms are unverified
- [x] **Architecture** — consistent with `architecture-system.md`'s existing HTTP-over-loopback transport; doesn't reopen FR-3 through FR-9
- [ ] **Domain correctness** — not applicable
- [x] **Security** — doesn't weaken any trust boundary; orphan-prevention is a reliability property, not a security one, and the ADR doesn't conflate the two
- [x] **Testability** — the claims are testable and were tested (findings above note what wasn't)
- [ ] **Accessibility** — not applicable
- [x] **UX and copy** — not applicable at this artifact's level (no user-facing copy)
- [ ] **Observability** — not applicable
- [x] **Maintainability** — prototype code kept, reproducible, explicitly labeled throwaway/non-shipping
- [x] **Evolution** — reversal cost section correctly distinguishes what would and wouldn't need rework

## Contradictions and gaps

None against `architecture-system.md` or the constitution. The ADR
correctly identifies itself as resolving FR-1/FR-2 and half of FR-10, and
doesn't overclaim resolution of FR-3 through FR-9.

## What I did not review

Whether `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` (Windows) and
`EVFILT_PROC`/`NOTE_EXIT` (macOS) are actually the right named mechanisms —
I have no way to verify either without the respective OS, same limitation
the ADR itself states.
