# Review: architecture-frontend.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-frontend.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (both findings fixed — see Resolution below) |

## Summary

FR-3 (server-told capability gating, never client-inferred) is the
strongest part of this spec — it correctly extends `architecture-system.md`
FR-6's "same build artifact" requirement to its logical security
consequence, something no prior spec had actually worked through. One real
gap found on self-review: the spec never states whether the frontend is
server-rendered or purely client-rendered, which matters unusually much
here because the server is Go, not Node — real SSR would need a Node
process this project's process model (`architecture-system.md` FR-1,
amended to three-or-four processes already) has no room for.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Completeness | Never states client-side-rendered vs. server-rendered. This isn't a routine omission: the Go server can't run React SSR without embedding or spawning a Node runtime, which would mean a *fifth* long-running process family this project's process model has never accounted for. The obvious, almost certainly correct answer (pure client-side rendering, Go serves a static build) is never actually stated as a decision | Add an FR fixing CSR, with the Go-can't-run-Node reasoning stated directly — this is exactly the kind of "why on earth is it like this" question a future contributor would ask, and the answer is a real constraint, not a preference |
| 2 | Minor | Precision | FR-3's "every connection is the host (loopback-only, constitution §6)" is imprecise — pre-phase-13, a user could also open a plain browser tab to `127.0.0.1:<port>` directly, bypassing Electron entirely, and get full host capability. "Host" here means "loopback, therefore trusted at the same level as Electron's own renderer," not "literally came from the Electron app" | Reword for precision so a reader doesn't conflate "the host capability level" with "the Electron process specifically" |

## Dimensions checked

- [x] **Completeness** — finding 1 is real and substantial
- [x] **Ambiguity** — FR-1, FR-2, FR-4, FR-5, FR-6 are each clear; FR-3 has finding 2's precision issue
- [x] **Architecture** — FR-3 is a genuine extension of `architecture-system.md` FR-6's consequence, correctly reasoned; finding 1 is a real gap in the same category
- [ ] **Domain correctness** — not applicable
- [x] **Security** — FR-3's "server-told, never client-inferred" framing is the load-bearing security property of the entire spec, correctly stated as such
- [x] **Testability** — FR-3 has an explicit acceptance criterion (capability value withheld, host content must not flash); FR-5's accessibility requirement is checkable via the Playwright MCP's actual, confirmed capability
- [x] **Accessibility** — FR-5 stated as structural, not a pass; ties to a concretely available tool rather than an aspiration
- [x] **UX and copy** — FR-6 correctly keeps loading/error states visually consistent with the design reference without conflating build targets with the desktop-host splash
- [x] **Observability** — correlation ID visibility in error states (Non-functional) is concrete, not just "log it somewhere"
- [x] **Maintainability** — Non-goals section correctly excludes the component catalog and specific screens, keeping this spec at the right altitude
- [x] **Evolution** — FR-3's reserved-shape-not-built-gate framing means phase 12/13 extends rather than restructures this spec's work

## Contradictions and gaps

Finding 1 is the substantive one, and it's a good example of a gap that's
invisible until you ask "wait, how does the *server* actually render
this" — the spec talked about state, routing, and tokens fluently while
never stating the one fact that constrains all three (no Node runtime
available to render anything server-side).

## Resolution (2026-08-14)

- **#1** — new FR-7: pure client-side rendering, with the Go-can't-run-Node
  reasoning stated directly in the spec
- **#2** — FR-3 reworded: "host" means loopback-trusted, explicitly
  including a plain browser tab hitting `127.0.0.1` directly, not
  specifically "came from Electron"

## What I did not review

Whether TanStack Query and React Router are actually the right defaults
versus alternatives (SWR, a framework router) — named as reasonable
defaults with the actual justification explicitly deferred to phase 04
under constitution §9, not weighed against alternatives here.
