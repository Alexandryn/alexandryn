# 0009. Reading progress attaches to Work, with an Edition-scoped precise position as a fallback-capable secondary value

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-14 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 02's roadmap doc names this explicitly as needing a deliberate ADR:
"whether reading progress attaches to a work, an edition, or a file...
determines whether progress survives switching editions, and it is not
obvious." A second, related question: what happens when two devices
report different progress for the same book — is decided here too, since
defining "progress" is a prerequisite to defining what a conflict between
two reports even means.

The tension: attaching progress to `Edition` (or `File`) gives an exact,
format-native position (a PDF page number, an EPUB CFI) but loses it the
moment the user switches to a different edition of the same book — a
real scenario (`domain-library.md`'s `LibraryEntry` is Edition-scoped
precisely because a user can own multiple editions of one `Work`).
Attaching to `Work` survives edition switches but a raw position (a page
number, a byte offset) doesn't mean the same thing across two different
editions' pagination or internal structure.

## Decision

`ReadingProgress` attaches to `Work`. Its primary, always-meaningful value
is a `Percentage` (0.0–1.0) — edition-independent by construction, since
it's a fraction of the whole rather than a position within one specific
artifact. It may also carry a `PrecisePosition`, explicitly tagged with
the `Edition` it was recorded against, used only when the user resumes
reading that same `Edition`. Switching to a different `Edition` of the
same `Work` falls back to `Percentage`-derived positioning; the old
`PrecisePosition` is not applied to a different edition's content.

Conflict resolution, for when two devices report progress for the same
`Work`: **furthest `Percentage` wins by default**, with an explicit
override path for a deliberate backward move (a real re-read, not
staleness). This is a reconciliation *function*
(`ReconcileProgress(existing, new) -> result`), never an implicit
side effect of whichever write reaches storage last.

## Addendum (2026-09-01) — reconciliation mechanism refined

The conflict-resolution *intent* below stands: furthest reading progress
wins, deliberate backward moves stay possible. The *mechanism* is
refined per review `0048`
finding 1, which showed "furthest-wins plus a free-form override" is not
order-independent. `domain-reading.md` FR-6/FR-7 (amended the same day)
now specify: reconciliation is lexical `max` over `(epoch, percentage)`
with a server-assigned monotonic `epoch`; a stale report (observed epoch
below the stored epoch) is `Rejected`; a deliberate backward move is a
server epoch bump, serialised, explicitly outside the commutativity
guarantee. This ADR's "Confidence" note — medium on the exact mechanism,
tie-break open — is now resolved there.

## Options considered

### Option A — Attach to Work, Percentage primary, Edition-tagged precise position as fallback-aware secondary (chosen)

*For* — survives edition switches (the common real case
`domain-library.md` already assumes is possible), gives an always-valid
coarse position with no cross-format translation problem, and still gives
exact resume-where-you-left-off behavior for the common case of
re-opening the *same* edition, which is most of the time.

*Against* — a `Percentage`-only fallback after switching editions is an
estimate, not exact — resuming on a new edition might land the reader a
paragraph or two off from the literal old position. Judged acceptable:
better than losing all progress, and the alternative (Option B) makes the
common case — reading one edition all the way through — no better while
making the switching case much worse.

### Option B — Attach to Edition (or File)

*For* — always exact, no estimation, simplest to implement per-edition.

*Against* — progress resets to zero (or becomes orphaned) the moment a
user switches editions, which is a real, not hypothetical, case this
project's own domain already supports (`domain-library.md` allows
multiple `Edition`s of one `Work` to be owned simultaneously). Rejected:
optimizes the rare case (never switching editions) at the cost of the
real one.

### Option C — Attach to Work only, no Edition-scoped precise position at all

*For* — simplest model, `Percentage` only, no tagging complexity.

*Against* — throws away exact resume position even for the overwhelmingly
common case of re-opening the same edition repeatedly. Rejected: the
common case shouldn't pay for the rare case's problem.

### Conflict resolution — furthest-wins with override (chosen) vs. last-write-wins

*Furthest-wins, for* — matches user expectation: syncing should never
make you lose read progress by having an older device's stale report
clobber a newer, further-along one, regardless of which one happened to
sync last.

*Furthest-wins, against* — needs an explicit override path, or genuine
re-reading (deliberately going backward) becomes impossible to represent,
which is why FR-7 (the spec) requires one.

*Last-write-wins, against, rejected* — simpler, but directly produces the
failure this ADR's own user story names: a stale phone sync overwriting a
tablet's further-along progress just because it happened to write last.

## Consequences

**Good** — progress survives the real, supported case of owning multiple
editions of one book; conflict resolution has one deterministic rule
instead of depending on write ordering; re-reading is representable, not
accidentally forbidden by the conflict-resolution default.

**Bad** — `Percentage`-based repositioning after an edition switch is an
estimate; `ReconcileProgress` is a real function that must be called
correctly by whatever wires devices together (phase 14) — an
implementation that forgets to call it and just overwrites reverts to the
last-write-wins failure mode this ADR explicitly rejected.

**Neutral** — doesn't decide `PrecisePosition`'s actual format (CFI, page
number) — phase 11's job; doesn't decide the exact tiebreak when two
`Percentage`s are equal — flagged as an open question in
`domain-reading.md`, not resolved here.

## Reversal cost

Medium. Changing the attachment level after phase 03/14 build storage and
sync around it means migrating every stored `ReadingProgress` record and
re-deriving what level to attach to for data that no longer has a clean
mapping (a `Percentage` can't be un-derived back into a specific position
without the original precise data, which may not exist for old records).
Cheap right now — nothing built yet.

## Confidence

High on Work-level attachment with a `Percentage` primary — directly
derived from a real, already-decided constraint (`domain-library.md`
allows multiple owned editions per Work) rather than a preference.
Medium on furthest-Percentage-wins specifically as the conflict default —
reasonable and matches common reader expectations (Kindle-style "furthest
location" sync is a known, validated pattern elsewhere), but not
user-tested for this specific project, and the exact-tie tiebreaker is
still open.
