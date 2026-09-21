# 0001. We record architecture decisions in this directory

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-12 |
| **Deciders** | Project maintainers |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Alexandryn is intended to be maintained for years, by people who were not
present for its early decisions — including automated contributors that arrive
with no memory of previous sessions at all.

Decisions made once and never written down get relitigated. Worse, they get
silently reversed: someone sees a constraint that looks arbitrary, removes it,
and reintroduces a problem that was already solved. In a project whose
correctness depends on boundaries being respected — metadata separate from
sources, renderer separate from host, loopback until authenticated — that
failure mode is not cosmetic.

The alternative sources of this reasoning are all unreliable. Commit messages
scatter it. Pull request threads disappear behind a UI. Maintainers forget.

## Decision

We keep architecture decision records in `docs/decisions/`, in the style
described by Michael Nygard, using `../templates/adr.md`.

Each record is immutable once accepted. Changing our minds means writing a new
record that supersedes the old one, and updating both. Nothing is deleted.

An ADR is required when a choice is expensive to reverse, constrains other
work, or would otherwise look arbitrary to someone reading the code later. The
`README.md` in this directory tracks questions known to be open and the phase
that will force each one.

## Options considered

### Option A — ADRs in the repository (chosen)

Versioned with the code they describe, reviewable in the same pull request that
implements them, readable offline, and available to an automated contributor
that has only the working tree.

Cost: they rot if nobody maintains them, and they add friction to every real
decision.

### Option B — A wiki or external document

Easier to write, and easier to reorganise. But it drifts from the code, it
isn't reviewable alongside a diff, and it is invisible to anyone working from a
clone. External documentation about internal constraints tends to describe a
system that no longer exists.

### Option C — Commit messages and PR descriptions only

Zero overhead, and it's where the reasoning naturally appears anyway. But it is
unsearchable in practice: finding why a constraint exists means archaeology
through history, and superseded reasoning looks identical to current reasoning.

### Option D — No deliberate record

Honest about what most projects actually do. Rejected because this project's
correctness leans on constraints that look removable from the outside.

## Consequences

**Good** — the reasoning survives contributor turnover and context loss. Review
has something concrete to argue against. New contributors, human or otherwise,
can reconstruct intent without asking.

**Bad** — friction on every significant decision, and a directory that will
mislead if it's allowed to go stale. Some records will be written for decisions
that turn out not to matter.

**Neutral** — decisions become slower and more visible. For this project that
is the intended trade, but it is a real change in pace.

## Reversal cost

Low. Abandoning the practice costs nothing structurally; the existing records
stay useful as history. The risk isn't reversal, it's quiet neglect — records
that stop being written while the directory implies they still are.

## Confidence

High. This is a well-established practice with a known failure mode
(abandonment), and the failure is visible rather than silent.
