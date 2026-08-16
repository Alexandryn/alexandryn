# Review: `architecture-backend.md`, amendment (container topology)

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-backend.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-16 |
| **Verdict** | Approved with changes (no findings — clean on first pass) |

## Summary

FR-5 stated the Electron-spawn-delivered config file as "how a packaged
instance is always configured," treating environment-variable-driven
configuration as a dev-only override. Amends it to name the
container-hosted target's Compose-delivered environment configuration as
an equally legitimate second normal source, with no change to
`config.Load`'s actual precedence algorithm — the fix is in what counts as
"packaged," not in any mechanism.

## Findings

None. The amendment is additive and doesn't touch FR-1 through FR-4 or
FR-6, none of which reference a specific deployment target.

## Dimensions checked

- [x] **Completeness** — checked the whole spec for other topology-coupled
      language (FR-1 through FR-6, References) rather than only FR-5
- [x] **Ambiguity** — a reader can now tell the config-precedence algorithm
      is target-independent by construction, not just told the answer
- [x] **Architecture** — no change to `internal/config`'s ownership or the
      no-globals rule (FR-2's territory, untouched)
- [ ] **Domain correctness** — not applicable
- [x] **Security** — no new secret-handling path introduced; this
      amendment is about config *delivery*, not what's in the config
- [x] **Testability** — `backend-configuration.md`'s own FR-2 precedence
      test already covers per-key resolution regardless of source; nothing
      new to test here specifically
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [ ] **Observability** — not applicable, no new logging surface
- [x] **Maintainability** — the amendment note follows the same pattern
      established in `architecture-system.md`'s two amendments
- [x] **Evolution** — states the precedence algorithm is source-agnostic,
      which is what makes a hypothetical third target free to add later

## Contradictions and gaps

None found.

## What I did not review

Whether `backend-configuration.md` itself (a separate, `APPROVED` spec)
needs a matching amendment — it does, tracked separately in the amendment
plan and reviewed in its own pass, not folded into this one to keep each
commit's diff coherent.
