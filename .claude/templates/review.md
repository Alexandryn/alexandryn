# Review: <spec or change under review>

| | |
|---|---|
| **Subject** | `.claude/specs/…` or PR #NN |
| **Reviewer** | |
| **Date** | YYYY-MM-DD |
| **Verdict** | Approved / Approved with changes / Rejected / Needs rework |

## Summary

Two or three sentences. What was reviewed and what the verdict rests on.

## Findings

Each finding gets a severity and a concrete ask. "Consider maybe looking at
this" is not a finding.

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Blocking | | | |
| 2 | Major | | | |
| 3 | Minor | | | |
| 4 | Nit | | | |

**Blocking** — must be resolved before this proceeds.
**Major** — must be resolved, but can be a follow-up if tracked.
**Minor** — should be resolved; reviewer won't block on it.
**Nit** — preference. The author may decline without justifying.

## Dimensions checked

Mark each honestly. An unchecked box is information; a falsely checked one is
damage.

- [ ] **Completeness** — are there unstated requirements?
- [ ] **Ambiguity** — could two engineers read this and build different things?
- [ ] **Architecture** — does it respect the layering?
- [ ] **Domain correctness** — are metadata / source / library boundaries intact?
- [ ] **Security** — threats identified, mitigations concrete?
- [ ] **Testability** — can every requirement become a test?
- [ ] **Accessibility** — keyboard, focus, naming, announcements?
- [ ] **UX and copy** — plain, specific, human?
- [ ] **Observability** — will we know when this breaks in the wild?
- [ ] **Maintainability** — will this be legible in two years?
- [ ] **Evolution** — what does this make harder to change later?
- [ ] **Indexes** — does this change move a spec/ADR/phase's status, or resolve
      an open question? If so, are `roadmap/README.md`, `specs/README.md`,
      `decisions/README.md`, and `skills/README.md` all current with it?

## Contradictions and gaps

Places where the subject contradicts itself, the constitution, an ADR, or
another spec. These matter more than style comments and are easy to miss.

## What I did not review

Scope honesty. Say which parts you didn't examine closely.
