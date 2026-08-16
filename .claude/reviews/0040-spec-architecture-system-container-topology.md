# Review: `architecture-system.md`, second amendment (container topology)

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-system.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16 |
| **Verdict** | Approved with changes (finding below fixed before this review) |

## Summary

Amends FR-1, FR-2, and Security considerations to state two legal
topologies (Electron-hosted and container-hosted, ADR 0015) instead of one
described as exhaustive, and corrects the Non-goals section's "cloud
deployment... out of scope" line, which ADR 0015 directly contradicts.
Adds two Open questions the container target introduces and doesn't
resolve. This is this spec's second post-approval amendment; the first
(ADR 0007, 2026-08-14) is still itself unconfirmed by the maintainer.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Non-goals | The first drafting pass amended FR-1/FR-2/Security considerations for the container topology but left the Non-goals section's "Multi-host or cloud deployment — explicitly out of scope for v1" unchanged, directly contradicting the new FR-1 | Fixed — split into "multi-host remains out of scope" / "cloud deployment is no longer a non-goal," with the reasoning stated |

## Dimensions checked

- [x] **Completeness** — checked every section that names process count or
      topology (FR-1, FR-2, Security considerations, Non-goals, Open
      questions, References) rather than only the two FRs named in the
      amendment plan
- [x] **Ambiguity** — a reader building against FR-1 alone can now tell
      which target they're building for, since each clause is labeled
- [x] **Architecture** — the container target's boundary (two containers,
      no spawn relationship) is stated precisely enough that
      `architecture-backend.md` and `backend-service-lifecycle.md` don't
      need to guess at it
- [ ] **Domain correctness** — not applicable, this spec doesn't touch the
      Alexandryn domain
- [x] **Security** — confirms the loopback/authentication gate is
      unaffected; flags the container-network-namespace `BIND_ADDRESS`
      nuance as a real, unresolved gap rather than assuming it away
- [x] **Testability** — nothing in this amendment is itself testable (no
      code); it correctly defers test coverage to the new deployment spec
      and `backend-test-harness.md`'s own amendment
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [x] **Observability** — the Non-goals/Open-questions gap this amendment
      does *not* close (the Electron target's correlation-ID-across-
      process-boundaries reasoning has no container-target equivalent
      written) is left named in Open questions rather than silently
      unaddressed — genuinely still open, not fixed in this pass
- [x] **Maintainability** — the second-amendment note follows the exact
      pattern the first one set, so a future reader sees one consistent
      convention, not two different styles of "this spec changed"
- [x] **Evolution** — states plainly that a third target, if one is ever
      proposed, should follow this same "amend, don't silently narrow"
      pattern

## Contradictions and gaps

Found and fixed (Finding #1). No other contradiction found between this
amendment and the rest of the spec, ADR 0015, or the constitution.

## What I did not review

Whether `architecture-desktop-host.md`, which also inherits process-model
language from this spec, needs its own amendment — out of scope for this
review; not touched by ADR 0015's amendment plan, and the desktop-host spec
only ever describes the Electron target, which is unchanged. Not
re-verified line by line here.
