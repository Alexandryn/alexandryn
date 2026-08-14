# Review: architecture-persistence.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-persistence.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (all four findings fixed — see Resolution below) |

## Summary

Covers the territory phase 01 assigned it (engine, schema ownership,
migrations, corruption, backup) and resolves `architecture-system.md`
FR-12 with a genuinely non-obvious technical distinction: Linux and Windows
have parent-side, spawn-time orphan-prevention mechanisms requiring no
change to PostgreSQL's own source; macOS doesn't, and needs a supervisor
process instead — which itself became a fourth-process consequence that
required a second amendment to `architecture-system.md`'s FR-1, caught in
this spec's own self-review rather than left for an independent reviewer to
find. One Blocking-adjacent gap remains: connection pool sizing and
migration tooling are correctly deferred to phase 03, but backup/restore is
deferred with *no owner at all*, which is a weaker position than the other
deferrals in this document.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Completeness | Backup and restore is listed as a Non-goal and an Open question, but unlike every other deferred item in this spec (connection pool → phase 03, migration tool → phase 03, encryption at rest → unnamed but explicitly "needs its own consideration"), it has no plausible next owner named at all. For a self-hosted app where the maintainer is the only support channel, "your library has no backup story" is a real product gap, not a paperwork one | Name a plausible owner (a future phase, or explicitly flag that no phase in the current 19-phase roadmap owns it — which would itself be a finding worth surfacing to the maintainer) rather than leaving it as pure absence |
| 2 | Minor | Consistency | FR-8 (Linux) and FR-9 (Windows) are stated as flatly required; FR-10 (macOS) is stated with the same MUST-level confidence but is honestly the least-verified requirement in this spec (no macOS to test against, and it's a novel-to-this-project supervisor-process pattern, not a well-known stdlib feature like FR-8's `Pdeathsig`). The confidence gap between FR-8/9 and FR-10 isn't reflected in how they're worded — all three read equally certain | Consider downgrading FR-10's certainty in wording (e.g., "the following approach is specified, pending validation" rather than a bare MUST) so a reader doesn't inherit false confidence from FR-numbering parity alone |
| 3 | Minor | Scope | This spec's FR-1 says the data directory goes "under the application's own persistent data path" without naming it — is that the same path family `.gitignore`'s `.data/` comment already references (root README's gitignore, phase 00 era), a platform-specific app-data directory (`~/.config`, `%APPDATA%`, etc.), or something else? Not decided, and it's a concrete detail phase 03/05 will need | Either pick the actual path convention (platform-standard app-data directories is the obvious default) or explicitly flag it as phase 03/05's to decide, the same way connection pool size is handled |
| 4 | Nit | Cross-reference | References section doesn't link back to review 0007 (the desktop-host review whose finding 1 this entire spec traces back to), even though the Context section narrates it in prose | Add the cross-reference for anyone jumping straight to References |

## Dimensions checked

- [x] **Completeness** — finding 1 is a real unnamed-owner gap; finding 3 is a concrete missing detail
- [x] **Ambiguity** — FR-1 through FR-10 are each individually clear about *what* MUST happen; finding 3 is about *where*, not clarity of the requirement itself
- [x] **Architecture** — correctly distinguishes parent-side vs. child-side orphan-prevention mechanisms across three platforms; this is the spec's strongest section
- [ ] **Domain correctness** — not applicable
- [x] **Security** — data-directory encryption-at-rest correctly left as an explicit open question rather than a silent default either way; the macOS supervisor's own trust boundary is named, not treated as a free extension of the Go server's
- [x] **Testability** — FR-1 through FR-9 are each testable; FR-10 is testable in principle but only on hardware this environment doesn't have, disclosed accurately
- [ ] **Accessibility** — not applicable
- [x] **UX and copy** — failure-mode messages follow constitution §11 (named failures, no raw errors)
- [x] **Observability** — migration event logging requirement stated, correctly deferred to phase 03 for the full contract
- [x] **Maintainability** — FR-7's explicit prohibition on automatic data-directory "recovery" is exactly the kind of thing worth stating as a MUST NOT rather than assuming a future implementer would know better
- [x] **Evolution** — Open questions honestly separates "real gap, no owner" (backup) from "placeholder pending a number" (pool size) from "unverified but specified" (macOS supervisor) — three different kinds of unfinished, correctly not flattened into one list

## Contradictions and gaps

Finding 1 is the substantive one. Beyond that: this spec's FR-1/FR-2/FR-12
amendment to `architecture-system.md` was itself caught mid-draft (the
four-process-on-macOS consequence) and fixed in the same pass — noting for
the record that this spec required *two* rounds of consistency-checking
against `architecture-system.md` before it stopped contradicting it, which
is worth the maintainer's attention as a pattern: cross-spec consequences
in this project keep surfacing one layer deeper than the first check finds.

## Resolution (2026-08-14)

All four findings fixed in the same pass:

- **#1** — backup/restore's Open questions entry now states plainly that no
  phase in the current roadmap owns it, names phase 99 as the most likely
  home without deciding it, and flags it as needing the maintainer's
  attention rather than a future spec's alone
- **#2** — FR-10 now opens with an explicit lower-confidence note
  distinguishing it from FR-8/FR-9's stdlib-feature certainty
- **#3** — FR-1 now names the actual convention (OS-standard per-user
  app-data directories), not a placeholder
- **#4** — References now links back to review 0007's finding 1

## What I did not review

Whether `os/exec`'s `SysProcAttr.Pdeathsig` behaves identically for a
long-running, multi-goroutine Go server as it would for a simple test
program — the same class of caveat review 0005 raised (informational, not
blocking) about Node's analogous behavior. Not re-litigated in depth here;
same low-concern status assumed by analogy, not independently re-verified.
