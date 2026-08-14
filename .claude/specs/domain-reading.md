# Spec: Reading domain

| | |
|---|---|
| **Status** | `REVIEWED` (self + independent, approved with changes) |
| **Phase** | `02-domain` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0019`](../reviews/0019-spec-domain-reading.md) (self) + [`0021`](../reviews/0021-phase02-cross-spec-review.md) (two independent agents, cross-spec — found a Blocking ambiguity in the central mechanism, fixed) — both Approved with changes; maintainer's own read still pending |

## Context

Phase 02's own roadmap doc calls this out specifically: "whether reading
progress attaches to a work, an edition, or a file... is not obvious" and
demands it be "decided deliberately in an ADR, with the cross-edition case
tested." That ADR is [0009](../decisions/0009-progress-attachment.md),
written alongside this spec, not left for later. Conflict resolution
across devices is the other named hard case — also decided there, since
it's the same class of decision (what does "progress" even mean, well
enough to define what a conflict *is*).

## Problem

Nothing has decided: what level progress attaches to, how a percentage
survives switching editions, what a bookmark or highlight is scoped to,
how reading preferences relate to devices, or what happens when two
devices report different progress for the same book.

## Goals

- Implement ADR 0009's progress-attachment decision as a concrete domain
  type
- Bookmarks and highlights scoped correctly (edition-specific position
  data, since exact content addressing is format-specific)
- Reading preferences modelled as device-scoped, not a single global or
  a duplicate-per-book setting
- Conflict resolution (ADR 0009) implemented as an explicit domain
  operation, not left to "whichever write happens last" by accident

## Non-goals

- Device identity/registration itself — phase 14 (devices and sync); this
  spec assumes a `DeviceID` exists to attach preferences and progress
  reports to, without designing device management
- The actual EPUB/PDF position format (CFI, page number, etc.) — phase 11
  (reader) picks the concrete representation; this spec treats a position
  as an opaque, edition-scoped value plus a percentage
- Sync transport/timing — phase 14; this spec defines what a conflict *is*
  and how it resolves once two reports exist, not how they get transmitted

## User stories

- As **a user who upgrades from an EPUB to a better-scanned PDF of the
  same book**, I want my rough progress to carry over, not reset to zero.
- As **a user reading on both a phone and a tablet**, I want the further
  of the two positions to win when they sync, not to have my tablet's
  progress overwritten by a stale phone read from yesterday.
- As **a user who deliberately re-reads a chapter**, I want going
  backwards to be possible without the system treating it as a stale
  conflict to override.

## Functional requirements

- **FR-1** `ReadingProgress` MUST attach to `Work`, not `Edition` or
  `File` (ADR 0009) — the record survives switching which `Edition` the
  user reads. **There MUST be at most one `ReadingProgress` per `Work` —
  a true singleton, not one row per reporting device.** This is stated
  explicitly because the type also carries a `DeviceID` and an
  `observed-at` timestamp (FR-2), which could otherwise be misread as
  "one row per (Work, Device)," reconciled only lazily at read/sync
  time — that reading would silently reproduce the exact stale-device-
  wins bug ADR 0009 exists to prevent, without violating any other FR's
  literal text. It doesn't, because this FR forbids it directly.
- **FR-2** The canonical, singleton `ReadingProgress` (FR-1) carries a
  `Percentage` (0.0–1.0, edition-independent, always meaningful) as its
  primary, portable value; an optional `PrecisePosition` tagged with the
  `Edition` ID it was recorded against; and `DeviceID`/`observed-at`
  recording *provenance* — which device's report is currently reflected,
  and when. These provenance fields describe the canonical record's
  history, not a second, independently-stored copy. A `PrecisePosition`
  is used only when the user is reading that *same* `Edition` again;
  reading a *different* `Edition` of the same `Work` falls back to
  `Percentage` as the best available estimate, per ADR 0009. An incoming
  update from a device (a **`ProgressReport`** — `Work` ID, `Percentage`,
  optional `PrecisePosition`, `DeviceID`, reported-at) is a distinct,
  ephemeral type, never itself persisted — it exists only as
  `ReconcileProgress`'s input (FR-6), which produces the next value of
  the one canonical `ReadingProgress` row.
- **FR-3** `Bookmark` and `Highlight` MUST attach to `Edition` (not
  `Work`), since their position data is inherently content/format-specific
  — a bookmark at "location 4210" is only meaningful for the exact
  `Edition` (and its content addressing) it was created against. Switching
  editions does not carry bookmarks/highlights over; they're specific to
  the artifact the user marked.
- **FR-4** A `Highlight` MUST record a start and end position (both
  `Edition`-scoped, FR-3) and MAY carry a note and a category/color. A
  `Bookmark` MUST record one position and MAY carry a label.
- **FR-5** `ReadingPreferences` (font, theme, line spacing, etc.) MUST be
  scoped per `DeviceID`, not global and not per-`Work` — a phone and a
  tablet reasonably want different settings; a preference is not
  "progress" and MUST NOT be conflict-resolved the way `ReadingProgress`
  is (ADR 0009) — each device simply keeps its own. A new `DeviceID`'s
  first `ReadingPreferences` MUST start from system defaults — there is
  no cross-device inheritance; a second device does not silently copy a
  first device's settings.
- **FR-6** Conflict resolution (ADR 0009: furthest-`Percentage`-wins, with
  an explicit override) MUST be a named domain operation —
  `ReconcileProgress(canonical ReadingProgress, incoming ProgressReport)
  -> ReadingProgress` (FR-2's types) — taking the current singleton
  (FR-1) and one incoming report for the same `Work` and producing the
  next canonical value. Never implicit in whichever write happens to land
  last in storage. `ReconcileProgress` MUST be commutative and
  associative: folding any number of devices' reports into the canonical
  value, in any order, MUST produce the same result. This is what makes
  multi-device sync (phase 14) correct regardless of message arrival
  order — without it, two devices syncing in a different order than two
  others could disagree about the canonical progress. "Furthest wins"
  satisfies this by construction (it's a max function); stated here as a
  requirement so a future change to the reconciliation rule can't
  silently drop the property.
- **FR-7** `ReconcileProgress` (FR-6) MUST accept an explicit override
  (the user deliberately choosing the "earlier" position, e.g. after a
  real re-read) — furthest-wins is the *default*, not the *only* legal
  outcome, or FR-6's own "deliberately re-reading a chapter" user story
  becomes impossible to satisfy.
- **FR-8** A reading status (`NotStarted`, `InProgress`, `Finished`) MUST
  be computed from `Percentage` (0.0 → `NotStarted`, 1.0 → `Finished`,
  otherwise `InProgress`) — never stored separately, same
  computed-not-duplicated pattern as `domain-library.md` FR-2's "in
  library." A cheap, common shelving/filtering need not worth a second
  source of truth.

## Non-functional requirements

- **Performance** — not applicable; no I/O in this layer.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable to a pure domain model; reader-level
  accessibility (font size, reduced motion) is FR-5's `ReadingPreferences`
  concern at the UI layer, phase 11's to implement.
- **Reliability** — FR-6/FR-7 are the reliability property: progress is
  never silently lost to a race between two devices, and a legitimate
  backward move is always possible, never mistaken for corruption.
- **Observability** — progress/bookmark/highlight changes are domain
  events (`domain-events.md`), not logged directly — and per constitution
  §8, what someone reads and how far MUST NOT appear in logs even
  indirectly (a log line naming a `Work` ID alongside a reading event
  would be exactly this leak).

## Domain model

- **`ReadingProgress`** — the canonical, singleton-per-`Work` record
  (FR-1): internal ID, `Work` ID (required), `Percentage`,
  `PrecisePosition` (optional, `Edition`-tagged — MUST belong to the same
  `Work`), `DeviceID` and observed-at timestamp recording *provenance*
  (which report is currently reflected), not a second stored copy.
- **`ProgressReport`** — ephemeral, never persisted, `ReconcileProgress`'s
  input only: `Work` ID, `Percentage`, `PrecisePosition` (optional),
  `DeviceID`, reported-at (FR-2).
- **`Bookmark`** — internal ID, `Edition` ID (required, FR-3), position,
  optional label.
- **`Highlight`** — internal ID, `Edition` ID (required, FR-3), start
  position, end position, optional note, optional category.
- **`ReadingPreferences`** — `DeviceID` (required, FR-5), font/theme/
  spacing fields (opaque to this spec beyond validation — phase 11 defines
  the actual set). New device starts from system defaults, no
  cross-device inheritance.
- **Computed, not stored**: reading status (`NotStarted`/`InProgress`/
  `Finished`, FR-8) from `ReadingProgress.Percentage`.

## API and contracts

Not applicable — pure Go model. `FR-6`'s `ReconcileProgress` is a domain
function signature (two `ReadingProgress` in, one out), not a transport
concern; phase 03/14 wire it to whatever sync mechanism reports progress.

## State transitions

```
No ReadingProgress exists for a Work -> first ProgressReport becomes the
    canonical ReadingProgress directly (no reconciliation needed, nothing to reconcile against)
ReadingProgress already exists (FR-1's singleton) -> a new ProgressReport arrives, any Device
    -> ReconcileProgress(canonical, report) -> next canonical value (FR-6)
    -> furthest Percentage wins by default (ADR 0009)
    -> explicit override accepted (FR-7), e.g. user chooses to go back
Edition switched (same Work) -> Percentage carries over; PrecisePosition
    from the old Edition is not reused for the new one (FR-2)
Bookmark/Highlight created -> scoped to one Edition; not carried over on
    Edition switch (FR-3)
```

Illegal: `ReadingProgress` constructed without a `Work` reference (FR-1,
type-level, same pattern as `domain-bibliographic.md` FR-8); a
`Percentage` outside `[0.0, 1.0]` (phase 02's own risk table: "progress
never exceeds its bounds"); a `Highlight` with an end position before its
start position within the same `Edition`; a `PrecisePosition` whose
tagged `Edition` does not belong to the `ReadingProgress`'s own `Work` —
checked at construction (same construction-time-invariant pattern as
`domain-bibliographic.md` FR-8), since nothing else stops a
`PrecisePosition` for an unrelated `Work`'s `Edition` from being attached
otherwise.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Two devices report conflicting progress for the same `Work` | `ReconcileProgress` invoked (FR-6) | The reconciled result (furthest by default) | Never silently drops a report at the *function* level — `ReconcileProgress` is deterministic and both inputs are known to it. **Correction**: the superseded report's value is not currently captured by any event (`domain-events.md` FR-5 requires exactly one `ReadingProgressUpdated` per reconciliation) — it is not "auditable via events" as an earlier draft of this spec claimed. Only the winning, canonical value is observable after the fact. |
| `Percentage` reported outside `[0.0, 1.0]` | Construction-time validation | A domain-level error | Refused; no `ReadingProgress` constructed with an out-of-bounds value |
| `PrecisePosition` exists but the user is now reading a different `Edition` | FR-2's fallback rule | `Percentage`-based position only | `PrecisePosition` from the prior `Edition` is not applied to the new one, avoiding a nonsensical position (e.g. a PDF page number applied to an EPUB) |

## Security considerations

- **What someone reads never appears in logs, even indirectly
  (constitution §8, restated here as this spec's own load-bearing
  requirement)** — this includes `Work` titles, `Highlight` text, and
  `Bookmark` labels, none of which are safe to log even at debug level.
  Phase 03's logging contract (`architecture-backend.md`) must treat every
  type in this spec as sensitive by default, not opt-in.
- **`Highlight` notes and `Bookmark` labels are hostile input** — same
  validation bar as every other user-typed string in this domain
  (`domain-bibliographic.md` FR-5's pattern): length bound, control
  characters rejected.
- **Progress/bookmark data reveals reading habits** — a device-scoped
  read of another user's `ReadingPreferences` or `ReadingProgress` isn't
  addressed by this spec (no multi-user model yet, `domain-library.md`'s
  same Non-goal) — flagged so phase 12 doesn't discover this domain
  assumed single-user access control implicitly.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | FR-1 through FR-7, table-driven |
| Integration | Not applicable — no I/O |
| Property-based | "Progress never exceeds its bounds and never moves backwards without an explicit reset" (phase 02's own test strategy, verbatim) — generate adversarial sequences of reports and reconciliations, assert the invariant holds throughout |
| Table-driven | The cross-edition case phase 02's roadmap explicitly demands tested: progress on Edition A, switch to Edition B of the same Work, `Percentage` carries over, `PrecisePosition` does not |

## Acceptance criteria

- [ ] `ReadingProgress` references `Work`, never `Edition` directly (ADR
      0009, FR-1)
- [ ] A test proves at most one `ReadingProgress` row exists per `Work`
      after any number of `ProgressReport`s from any number of devices
      (FR-1's singleton requirement)
- [ ] A test proves constructing a `ReadingProgress` with a
      `PrecisePosition` tagged to an `Edition` of a *different* `Work`
      is rejected
- [ ] A test proves switching editions preserves `Percentage` and drops
      the now-inapplicable `PrecisePosition`
- [ ] A test proves `ReconcileProgress` picks the furthest `Percentage` by
      default and accepts an explicit override for the backward case
- [ ] A property-based test proves `ReconcileProgress` is commutative and
      associative — reconciling three or more reports in different orders
      always produces the same canonical result (FR-6)
- [ ] `Bookmark`/`Highlight` cannot be constructed without a valid
      `Edition` reference
- [ ] A test proves no code path in this package can produce a log-shaped
      string containing a `Work` title, `Highlight` text, or `Bookmark`
      label (a static check or a targeted test, per constitution §8)

## Open questions

- **Multi-user access to progress/preferences** — flagged in Security
  considerations; genuinely unaddressed pending phase 12.
- **`PrecisePosition` format** — deliberately opaque here; phase 11
  decides whether it's a CFI, a page number, or something else per
  format.
- **Reconciliation when both reports have identical `Percentage`** — ADR
  0009 doesn't cover an exact tie; likely "most recent observed-at wins"
  as a tiebreaker, not decided here.

## References

- ADR 0009 — progress attachment and conflict resolution
- `.claude/roadmap/02-domain/README.md` — the named hard cases this spec
  and its ADR resolve
- `domain-bibliographic.md` — `Work`/`Edition`, referenced by ID only
- `domain-library.md` — the same "attaches to which level" reasoning
  pattern, applied differently here on purpose (library membership is
  Edition-level, progress is Work-level — not an inconsistency, a
  deliberate difference explained by what each concept actually tracks)
- Constitution §4 (hostile input), §8 (never log what someone reads)
