# Spec: Reading domain

| | |
|---|---|
| **Status** | `APPROVED` — FR-6/FR-7 amended 2026-09-01 for `0048` finding 1 (the Group 2a change): reconciliation is now `max` over the total order `(epoch, percentage)`, with a server-assigned monotonic `epoch`, an observed-epoch precondition that yields a third `Rejected` outcome for a stale report, and an explicit backward move expressed as a server epoch bump rather than a free-form override. `ProgressReport` gains `ObservedEpoch`; `ReadingProgress` gains `Epoch`. Acceptance criteria `:251-255` rewritten to be jointly satisfiable. Implementable as of this amendment (phase 11). Separately amended 2026-08-20 for `0049` findings 7 and 8 — Open questions sharpens what phase 11's `PrecisePosition` format must survive and records a phase-14 cross-device identity-agreement gap. Maintainer re-confirmation of both amendments pending. |
| **Phase** | `02-domain` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | `0019` (self) + `0021` (two independent agents, cross-spec — found a Blocking ambiguity in the central mechanism, fixed) — both Approved with changes; maintainer read and approved 2026-08-19; correctness pass `0048` |

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
  primary, portable value; an **`Epoch`** (a non-negative, monotonically
  non-decreasing integer, server-assigned — see FR-6, and never set or
  advanced by a reporting device directly); an optional `PrecisePosition`
  tagged with the `Edition` ID it was recorded against; and
  `DeviceID`/`observed-at` recording *provenance* — which device's report
  is currently reflected, and when. These provenance fields describe the
  canonical record's history, not a second, independently-stored copy. A
  `PrecisePosition` is used only when the user is reading that *same*
  `Edition` again; reading a *different* `Edition` of the same `Work`
  falls back to `Percentage` as the best available estimate, per ADR
  0009. An incoming update from a device (a **`ProgressReport`** — `Work`
  ID, `Percentage`, optional `PrecisePosition`, `DeviceID`, reported-at,
  and an **`ObservedEpoch`**: the `Epoch` value the reporting device last
  received from the server for this `Work`, or `0` if it has never
  synced) is a distinct, ephemeral type, never itself persisted — it
  exists only as `ReconcileProgress`'s input (FR-6), which produces the
  next value of the one canonical `ReadingProgress` row.
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
- **FR-6** Conflict resolution (ADR 0009) MUST be a named domain
  operation — `ReconcileProgress(canonical ReadingProgress, incoming
  ProgressReport) -> ReconcileResult` (FR-2's types) — taking the current
  singleton (FR-1) and one incoming report for the same `Work` and
  producing the next canonical value plus an outcome tag. Never implicit
  in whichever write happens to land last in storage.

  The rule is **`max` over the total order `(epoch, percentage)`**,
  compared lexicographically (`epoch` first, `percentage` as tiebreak).
  The report's ordering key is a fixed function of the report alone:
  `(report.ObservedEpoch, report.Percentage)`. Reconciliation compares
  that key against the canonical `(Epoch, Percentage)`:

  - report key **strictly greater** — the report advances the canonical
    value; outcome `Advanced`. The new canonical takes the report's
    `Percentage`, `PrecisePosition`, `DeviceID`, and reported-at as
    provenance, and keeps `Epoch` equal to the report's `ObservedEpoch`
    (which, for a normal forward report, equals the canonical `Epoch`
    already — a device reporting `ObservedEpoch` *higher* than the
    stored `Epoch` is clamped down to the stored value before comparison,
    since only the server assigns `Epoch`, so such a report can only ever
    compete on `Percentage` at the current epoch).
  - report key **equal** — no mutation; outcome `Unchanged`. Provenance
    is not rewritten for an exact tie (this is what makes the operation
    deterministic without a separate tiebreak — a tie is a no-op in a
    `max` fold; see Open questions, now closed).
  - report key **strictly less** — the report is behind the canonical
    value; outcome `Rejected`. The canonical value is returned unchanged.
    A report with `ObservedEpoch` below the stored `Epoch` (a device that
    was offline across an epoch bump — FR-7) always lands here: it was
    formed against a superseded view and must re-sync before it can
    compete again.

  `ReconcileProgress` MUST be commutative and associative **over the set
  of well-formed `ProgressReport`s at the current epoch** (each carrying
  `ObservedEpoch` equal to the stored `Epoch` — the honest steady-state
  case): folding any number of such reports into the canonical value, in
  any order, MUST produce the same result. This holds by construction —
  within that set the report key is exactly `(Epoch, report.Percentage)`,
  a fixed function of the report, so the final canonical value is `max`
  over a fixed multiset under a total order. The `Advanced`/`Unchanged`/
  `Rejected` tag is a per-step label; it does not affect the fold's final
  value. The clamp (for an `ObservedEpoch` above the stored `Epoch`) and
  the stale-report `Rejected` path (for one below it) are boundary
  defences for malformed or out-of-date input, outside this set — they
  do not need to preserve the fold property, only to be deterministic,
  which they are. This is what makes multi-device sync (phase 14) correct
  regardless of message arrival order.
- **FR-7** A deliberate backward move (the user choosing an "earlier"
  position after a real re-read, ADR 0009's own user story) is expressed
  as a **server epoch bump**, not a free-form flag on a concurrent
  report. A distinct operation — `OverrideProgress(canonical
  ReadingProgress, target Percentage, at PrecisePosition, by DeviceID) ->
  ReadingProgress` — produces a new canonical value with `Epoch =
  canonical.Epoch + 1` and `Percentage = target`, unconditionally (the
  bumped epoch makes `(canonical.Epoch + 1, target)` strictly greater
  than the old `(canonical.Epoch, anything)` under the FR-6 order, so the
  override always wins without special-casing the comparison). This is a
  deliberate, server-serialised act — two overrides racing are ordered by
  the server one at a time, each seeing the other's committed epoch,
  never folded concurrently — so it is explicitly **outside** FR-6's
  commutativity guarantee, which covers automatic background progress
  reports only. After an override, every device still reporting the old
  epoch is `Rejected` by FR-6 until it re-syncs and observes the new
  `Epoch` — which is the intended effect: the re-read sticks, and a stale
  device cannot silently clobber it.
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
  (FR-1): internal ID, `Work` ID (required), `Percentage`, `Epoch`
  (non-negative, monotonically non-decreasing, server-assigned — FR-2/
  FR-6),
  `PrecisePosition` (optional, `Edition`-tagged — MUST belong to the same
  `Work`), `DeviceID` and observed-at timestamp recording *provenance*
  (which report is currently reflected), not a second stored copy.
- **`ProgressReport`** — ephemeral, never persisted, `ReconcileProgress`'s
  input only: `Work` ID, `Percentage`, `ObservedEpoch` (the `Epoch` the
  device last saw for this `Work`, `0` if never synced — FR-2/FR-6),
  `PrecisePosition` (optional), `DeviceID`, reported-at (FR-2).
- **`ReconcileResult`** — `ReconcileProgress`'s return (FR-6): the next
  canonical `ReadingProgress` and an outcome tag (`Advanced`,
  `Unchanged`, `Rejected`). A pure value; the caller (phase 03/11's
  persistence wiring) decides what to write and whether to emit
  `ReadingProgressUpdated` (`domain-events.md` FR-5 — one event on
  `Advanced`, none on `Unchanged`/`Rejected`, since neither mutates).
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
function signature (a canonical `ReadingProgress` plus one
`ProgressReport` in, a `ReconcileResult` out) and `FR-7`'s
`OverrideProgress` is a second one; neither is a transport concern.
Phase 11 (`backend-reading-api.md`) and phase 14 wire them to whatever
mechanism reports progress, inside a row-locked transaction so the
read-then-write is atomic.

## State transitions

```
No ReadingProgress exists for a Work -> first ProgressReport becomes the
    canonical ReadingProgress directly, Epoch := 0 (nothing to reconcile
    against; the report's ObservedEpoch is ignored, there was nothing to observe)
ReadingProgress already exists (FR-1's singleton) -> a new ProgressReport arrives, any Device
    -> ReconcileProgress(canonical, report) -> ReconcileResult (FR-6)
    -> report key (ObservedEpoch, Percentage) vs canonical (Epoch, Percentage), lexical max
    -> strictly greater: Advanced (canonical takes report's values, Epoch unchanged)
    -> equal: Unchanged (no mutation, provenance not rewritten)
    -> strictly less, or ObservedEpoch below stored Epoch: Rejected (canonical unchanged)
Deliberate backward move -> OverrideProgress (FR-7), server-serialised
    -> new canonical: Epoch := Epoch + 1, Percentage := target, unconditionally
    -> every device still on the old Epoch is Rejected by FR-6 until it re-syncs
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
| Two devices report conflicting progress for the same `Work` at the same epoch | `ReconcileProgress` invoked (FR-6) | The reconciled result — lexical `max` over `(epoch, percentage)` | Never silently drops a report at the *function* level — `ReconcileProgress` is deterministic and both inputs are known to it. The superseded report's value is not captured by any event (`domain-events.md` FR-5 emits one `ReadingProgressUpdated` on `Advanced` only) — only the winning canonical value is observable after the fact. |
| A device offline across an epoch bump reports its old-epoch progress | `ReconcileProgress`: `report.ObservedEpoch` below stored `Epoch` (FR-6) | Outcome `Rejected`; canonical returned unchanged | The device must re-sync (observe the new `Epoch`) and re-report before it can compete again — its offline progress is not lost by the device, just not accepted until it is reconciled against the current epoch |
| Two deliberate overrides race | `OverrideProgress` serialised by the server (FR-7) | Each override sees the other's committed `Epoch`; the later one produces `Epoch + 1` again | Deterministic given server arrival order — an override is a deliberate user action, not a background report, so this is not a case FR-6's commutativity needs to cover |
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
| Property-based | Two disjoint properties, so the override and the commutativity criteria are jointly satisfiable (`0048` finding 2): (a) over generated sequences of **same-epoch `ProgressReport`s only** — no overrides in the generator — `ReconcileProgress` folded in any order yields the same canonical value, and `Percentage` never exceeds `[0.0, 1.0]`; (b) `OverrideProgress` followed by any sequence of old-epoch reports leaves the override's `Percentage` canonical (every stale report `Rejected`) |
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
- [ ] A test proves `ReconcileProgress` picks the lexical `max` over
      `(epoch, percentage)` — a further same-epoch `Percentage` advances,
      an equal key is `Unchanged`, a lower key or a below-stored-epoch
      `ObservedEpoch` is `Rejected` (FR-6)
- [ ] A test proves `OverrideProgress` sets the canonical value to the
      target `Percentage` at `Epoch + 1` unconditionally, and that every
      subsequent old-epoch report is `Rejected` (FR-7)
- [ ] A property-based test proves `ReconcileProgress` folded over any
      number of **same-epoch** reports in any order always produces the
      same canonical result — the generator emits no overrides, and a
      second property (FR-7) covers the override case separately, so the
      two criteria above are jointly satisfiable (`0048` findings 1, 2)
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
  format. Two properties whatever concrete format phase 11 picks needs
  to survive, named here so they aren't rediscovered later: the same
  addressed text or location occurring more than once within one
  rendered unit MUST resolve to a single, stable target, not whichever
  occurrence a highlight or bookmark happens to land on; and the format
  MUST tolerate the renderer's own output for "the same" content
  shifting between reads (a layout or DOM change that reflects no edit
  to the book itself) without silently drifting to the wrong location
  or duplicating an existing mark
  (`0049`,
  finding 7).
- **Reconciliation when two reports have an identical `(epoch,
  percentage)` key** — **closed** by the 2026-09-01 amendment: an exact
  tie is `Unchanged`, a no-op in the `max` fold, provenance not
  rewritten. No timestamp tiebreak is needed or wanted — one would
  reintroduce an ordering dependence the fold is designed not to have.
- **Cross-device identity agreement, ahead of phase 14** —
  `ReconcileProgress` (FR-6) assumes the system and every reporting
  device already agree on which `Work`/`Edition` a `ProgressReport` is
  about; nothing here addresses how that agreement is reached, or what
  happens when it's asymmetric — one direction of sync agreeing, the
  other not, for what should be the same file. Recorded here, unresolved,
  for phase 14 to pick up; not a gap this spec's conflict-resolution math
  can address, since reconciliation only runs once both sides already
  agree what they're reconciling
  (`0049`,
  finding 8).

## References

- ADR 0009 — progress attachment and conflict resolution
- `docs/roadmap/02-domain/README.md` — the named hard cases this spec
  and its ADR resolve
- `domain-bibliographic.md` — `Work`/`Edition`, referenced by ID only
- `domain-library.md` — the same "attaches to which level" reasoning
  pattern, applied differently here on purpose (library membership is
  Edition-level, progress is Work-level — not an inconsistency, a
  deliberate difference explained by what each concept actually tracks)
- Constitution §4 (hostile input), §8 (never log what someone reads)
