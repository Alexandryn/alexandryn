# Review: `backend-configuration.md`, amendment (container topology)

| | |
|---|---|
| **Subject** | `.claude/specs/backend-configuration.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16 |
| **Verdict** | Approved with changes (finding below fixed before this review) |

## Summary

FR-4's `DATABASE_URL` row and Security considerations both stated,
unqualified, that the key's absence means production and its presence
means dev/CI/test. That was true only of the Electron-hosted target.
Amends both to state the target-dependent meaning explicitly: absence
still means production for the Electron target, but presence is the
*normal* production case for the container target, which has no
spawn-and-own fallback to default to.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | FR-4 explanatory paragraph | The paragraph following the FR-4 table described `DATABASE_URL`'s presence/absence purely as a dev-vs-production split, which is the exact language the Security considerations section separately restated — fixing only one without the other would have left the spec self-contradictory | Fixed — both the table's paragraph and Security considerations amended together, in the same pass, so neither statement contradicts the other post-amendment |

## Dimensions checked

- [x] **Completeness** — checked FR-4's table row, its explanatory
      paragraph, and Security considerations together, since all three
      independently stated the same now-superseded absolute
- [x] **Ambiguity** — a reader can now determine what `DATABASE_URL`'s
      presence means without knowing which target they're building for
      in advance — the spec states both cases explicitly
- [x] **Architecture** — no change to `config.Load`'s own precedence
      mechanism (FR-2) or the "no context-dependent requiredness" rule
      (FR-3) — the third category this key already occupies already
      tolerates this without modification, confirmed while amending
- [ ] **Domain correctness** — not applicable
- [x] **Security** — confirms FR-7's dual-interface redaction
      (`slog.LogValuer` + `json.Marshaler`) is target-independent and
      needed no amendment; the finding fixed here was about which
      statement is true in which target, not about a new leak path
- [x] **Testability** — `backend-persistence.md`'s own amendment (tracked
      separately) is where the actual behavioral test for each target's
      branch lives; this spec's acceptance criteria don't need new entries
      since FR-4's own precedence tests are already target-agnostic
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [x] **Observability** — the "log which source provided each non-default
      value" requirement (Non-functional requirements, Observability) is
      unaffected — it already logs the source, not an interpretation of
      what the source means
- [x] **Maintainability** — this spec has now been amended four times
      post-approval; each amendment note follows the same format, keeping
      the accumulation legible rather than each one inventing its own
      phrasing
- [x] **Evolution** — the target-dependent framing is written so a
      hypothetical third target only needs its own clause added, not a
      restructuring of FR-4

## Contradictions and gaps

Found and fixed (Finding #1) — the two originally-independent absolute
statements (FR-4's paragraph, Security considerations) would have drifted
out of agreement with each other if amended separately. Amending both in
one pass avoided introducing that as a new drift, the same class of defect
the parent audit (`0002-topology-gap.md`) was hunting.

## What I did not review

`backend-persistence.md`'s own matching amendment (FR-5, Security
considerations) — reviewed separately, tracked in its own commit, to keep
this spec's diff coherent on its own terms.
