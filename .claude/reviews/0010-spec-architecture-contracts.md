# Review: architecture-contracts.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-contracts.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (all four findings fixed — see Resolution below) |

## Summary

Spec-first OpenAPI, a fixed error shape, and a versioning scheme are decided
with real reasoning. One overclaim caught on self-review: FR-4 asserted
client/server skew "structurally cannot happen" — that's wrong for an
already-open LAN browser tab surviving a host restart, just narrower than a
public API's skew problem, not absent. Two gaps: the file location assumes
a monorepo layout phase 01 hasn't actually decided yet, and local dev's
frontend-dev-server-talks-to-backend-dev-server scenario (different ports)
isn't addressed at all.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Correctness | FR-4 claims client/server skew "structurally cannot happen here" because the web UI is fetched fresh each load. That's true for a *new* page load, but an already-open LAN browser tab (phone, tablet) keeps running its already-loaded JS across a host restart or update — if that update includes a breaking API change, the stale tab's old frontend code calling the new server *is* skew, just narrower in scope and duration than a public API's version-skew problem | Correct FR-4: skew is reduced, not eliminated — versioning still matters for the "already-open tab across a host update" case, not only for hypothetical external integrations |
| 2 | Minor | Sequencing | FR-2 fixes `api/openapi.yaml` at the repo root, but phase 01's own open-questions list still has "monorepo layout and tooling" undecided. This spec is asserting a path structure ahead of the decision that actually owns it | Note explicitly that this location is provisional, pending `architecture-backend.md` or a monorepo-layout ADR, rather than presenting it as settled |
| 3 | Minor | Completeness | Local development needs the frontend dev server (a different port, per typical Vite/webpack dev setups) to reach the backend dev server (also a different port) — a CORS or dev-proxy scenario entirely unaddressed. Not the same as the "no skew" production case FR-4 discusses | Add as an explicit open question or non-goal with an owner (`architecture-frontend.md` or phase 03/04's dev-tooling setup) |
| 4 | Minor | Completeness | Constitution §4 requires every request reaching the host to have a size limit, shape check, and timeout — this spec's error shape (FR-5) and contract-test requirement (FR-3) don't mention request-size limits as part of what the contract itself should document or enforce | Add a brief Security considerations note tying request limits to the contract, even though the actual limit values are phase 03's |

## Dimensions checked

- [x] **Completeness** — findings 2, 3, 4 are real gaps
- [x] **Ambiguity** — FR-1 through FR-6 are individually clear
- [x] **Architecture** — correctly layers shape (here) vs. taxonomy (phase 03), correctly excludes the IPC surface as a separate boundary
- [ ] **Domain correctness** — not applicable
- [x] **Security** — error-shape-as-testable-leak-prevention is a genuinely good idea (asserting `message` never contains a stack-trace pattern); finding 4 is the one gap
- [x] **Testability** — FR-3's drift-is-a-test-failure requirement is the spec's strongest property
- [ ] **Accessibility** — not applicable, machine contract
- [x] **UX and copy** — FR-5's message field explicitly inherits constitution §11's bar
- [x] **Observability** — correlation ID tying error responses to log lines is concrete, not hand-waved
- [x] **Maintainability** — FR-1's justification (spec-first matches this project's existing discipline) is the kind of reasoning that survives a "why on earth is it like this" question later
- [x] **Evolution** — FR-4's versioning-as-deprecation-path reasoning holds even after finding 1's correction; the mechanism doesn't change, just its stated justification

## Contradictions and gaps

Finding 1 is the substantive one — not a missing consideration, an actively
wrong claim that needed catching before it became "and this is why we don't
need versioning to matter much," which would have been the wrong lesson to
carry into phase 03.

## Resolution (2026-08-14)

All four findings fixed in the same pass:

- **#1** — FR-4 corrected: skew is bounded (an already-open tab across a
  host update), not absent
- **#2** — FR-2 marked provisional, pending the still-open monorepo-layout
  decision
- **#3** — local dev CORS/proxy scenario added to Open questions with an
  owner
- **#4** — Security considerations gained a request-limits-in-the-contract
  note, cross-referencing constitution §4 and phase 03's actual limit
  values

## What I did not review

Whether path-based versioning (`/api/v1/`) is actually the best choice
versus header-based — asserted with reasonable but not exhaustively argued
justification. Both are common, defensible choices; this wasn't weighed
against alternatives the way, say, ADR 0005's process model was.
