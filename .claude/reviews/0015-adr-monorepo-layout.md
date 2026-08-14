# Review: ADR 0008 — Monorepo layout (commit `8d5278f`)

| | |
|---|---|
| **Subject** | `.claude/decisions/0008-monorepo-layout.md` and its ripple edits to `architecture-contracts.md`, `architecture-backend.md` |
| **Reviewer** | Two independent subagents (fresh context each, no coordination) — genuinely independent, not self-review |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at the time of review (Blocking finding present); **fixed in this pass** |

## Summary

Both reviewers independently flagged the same Blocking issue, with the
same reasoning: ADR 0008 was committed as `Accepted` with no
`.claude/reviews/` record — the exact enforcement gap review 0014 was
written *this same session* to close, recurring on the very next ADR
commit. Both also independently caught that `architecture-desktop-host.md`
FR-6/FR-7's bundled splash asset contradicted ADR 0008's "nothing for
`electron/` to bundle" claim. Beyond that, each surfaced things the other
didn't: an overclaim ("last open phase 01 question" — false against the
same commit's own edited open-questions tables), an unverified
cross-spec dependency (`architecture-testing.md` never accounted for the
`web/`-before-`go build` ordering ADR 0008 itself named as a risk), an
unnamed consequential tradeoff (embedded frontend can't be hot-patched
without a full binary rebuild), and two citation errors (ADR 0006 cited
where ADR 0001 applied; `architecture-frontend.md` FR-7 cited for a claim
FR-7 doesn't make).

## Findings

| # | Severity | Area | Finding | Resolution |
|---|---|---|---|---|
| 1 | Blocking | Governance | No review record existed for ADR 0008 before it was marked `Accepted` — the exact gap review 0014 closed, reopened on the very next commit | **Fixed** — this file. Going forward: write the review before or in the same commit as the ADR, not after, matching how ADR 0007 did it correctly the first time |
| 2 | Major | Consistency | ADR 0008 claimed `electron/` has "nothing to bundle," contradicting `architecture-desktop-host.md` FR-6/FR-7's disk-loaded splash asset, and leaving that spec's own open question about which package owns it unresolved despite ADR 0008 being positioned to answer exactly that | **Fixed** — ADR 0008 now names the splash asset as the one thing `electron/` does bundle; `architecture-desktop-host.md`'s open question updated to point at ADR 0008's answer, with the design-token-consistency risk correctly left as still open |
| 3 | Major | Overclaim | Called this "the last open phase 01 question" in chat/commit framing — false against `decisions/README.md`'s own open-questions table (still lists frontend data-fetching, API contract ownership/versioning) edited in the same commit | Not a file fix (the overclaim was verbal/commit-message only, not written into ADR 0008 or the roadmap doc itself — checked, neither makes the claim). Corrected verbally to the maintainer; not repeating it |
| 4 | Major | Cross-spec gap | ADR 0008 named the `web/`-before-`go build` ordering as a CI risk `architecture-testing.md` "needs to get right," but that spec's Testing layers table and FR-1 gate list never mentioned a build step at all | **Fixed** — new FR-8 in `architecture-testing.md` requiring the ordered build step explicitly in CI, plus a new "Build" row in the Testing layers table |
| 5 | Major | Undernamed tradeoff | `go:embed`'s "Medium confidence" rating never said what the uncertainty actually was — an embedded frontend can't be hot-patched without a full binary rebuild, a real phase 99 cost | **Fixed** — named explicitly in Consequences/"Bad" and referenced from Confidence, instead of an unweighted number |
| 6 | Minor | Citation | Option C's rejection credited to ADR 0006; the actual reasoning (single-PR reviewability) is ADR 0001's | **Fixed** |
| 7 | Minor | Citation | "Electron loads the UI at runtime" claim cited `architecture-frontend.md` FR-7 (which is about CSR vs. SSR, not loopback loading) | **Fixed** — recited to `architecture-desktop-host.md`'s "Window ↔ Go server" section and `architecture-system.md` FR-6 |
| 8 | Nit | Precision | "`go build`" in Consequences read as unscoped despite `go:embed` being scoped to `cmd/server` earlier | **Fixed** — "`go build ./cmd/server`" throughout |
| 9 | Nit | Citation inversion | Cited ADR 0005 as establishing a "single-binary-where-possible ethos" — ADR 0005's actual decision *rejected* single-binary for Electron+Go; citing it here inverted its meaning | **Fixed** — recited to ADR 0007's bundle-Postgres reasoning instead, with an explicit note on why ADR 0005 doesn't apply here |

## What checked out (both reviewers agreed)

- The core layout decision itself (Go at root, `web/` + `electron/` as npm
  workspaces, no Turborepo/Nx) is sound and doesn't need reopening
- Every `templates/adr.md` section filled meaningfully, both rejected
  options fairly represented, not strawmanned
- No contradiction with `architecture-system.md`'s process model or
  `architecture-persistence.md`'s `cmd/pg-supervisor` — `go:embed` is
  correctly scoped to `cmd/server` only
- Commit type (`spec:`) and `Refs:` trailer correctly followed
  `commit/SKILL.md`'s conventions, including the lesson from review 0014
  finding #6

## Resolution

All findings fixed in the same session, same pass. ADR 0008's status
stays `Accepted` — nothing here reopened the core decision, only its
completeness and a few citations. This review itself is the audit-trail
artifact `review/SKILL.md` step 5 requires, produced (late, per finding 1)
rather than skipped.
