# Spec: Library domain

| | |
|---|---|
| **Status** | `APPROVED` (maintainer read 2026-08-19; amended same day for [`0048`](../reviews/0048-phase02-correctness-review.md) findings 8, 3 and 6 — the state-transition cascade line corrected to match FR-6, FR-2's computed membership now runs over the merge-resolved `Edition` set, and the Edition-existence check moved from construction to the owning domain service per [ADR 0020](../decisions/0020-graph-invariants-in-domain-services.md)). One item open: FR-7's deferral to `domain-events.md` for the "offline copies" reasoning still points at a passage that does not exist there ([`0048`](../reviews/0048-phase02-correctness-review.md) finding 13) |
| **Phase** | `02-domain` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0017`](../reviews/0017-spec-domain-library.md) (self) + [`0021`](../reviews/0021-phase02-cross-spec-review.md) (two independent agents, cross-spec) — both Approved with changes, all findings fixed; maintainer read and approved 2026-08-19; correctness pass [`0048`](../reviews/0048-phase02-correctness-review.md) |

## Context

`domain-bibliographic.md` modelled what a book *is* (Work, Edition).
This spec models what it means for one to be *in the library* — a
different question, deliberately kept separate (constitution §3: the
Alexandryn domain is what the user has and does, distinct from what a
book is). Phase 02's own scope names the hard part directly: "what it
means for a work to be 'in your library' when the file lives somewhere
else and might vanish."

Worth resolving before writing anything else: README.md says a second
device reaches "the same library... same books, same collections, same
page you left off on" — collections shared across devices. Phase 02's own
roadmap scope calls collections "explicitly local and unsynchronised."
These read as contradictory. They aren't, once precise: collections live
in one shared PostgreSQL database (`architecture-persistence.md`), so
every device queries the same source of truth — there's nothing to
*sync* in the offline-first, eventually-consistent sense, because there's
only ever one copy. "Local and unsynchronised" means no CRDT-style
multi-writer conflict resolution is needed for collections, unlike reading
progress (`domain-reading.md`), which genuinely can be recorded
independently by two devices before reconciliation. Both statements are
true; they're about different things.

## Problem

Nothing has decided: what level (Work or Edition) library membership
attaches to, whether membership requires current availability, what a
collection may contain, or how a vanished source/file affects a
membership record that already exists.

## Goals

- Fix library membership at the `Edition` level, with `Work`-level "is
  this in my library" as a computed view, not a duplicated fact
- Fix that membership is independent of current availability — a vanished
  file doesn't retroactively un-own a book
- Fix collections as containing `Work`s, closing phase 02's explicit open
  question
- Resolve the README/roadmap "local and unsynchronised" apparent
  contradiction explicitly, so it doesn't get rediscovered as a bug later

## Non-goals

- Availability itself (whether a source currently has a file) —
  `domain-source.md`; this spec only says membership doesn't depend on it
- Reading progress, position — `domain-reading.md`
- Multi-user / per-account library scoping — phase 12 doesn't exist yet;
  this spec models a single shared library per instance, matching
  constitution's existing framing (a household's library, not a
  multi-tenant one) and the roadmap's own anti-overengineering stance.
  If phase 12 needs per-account scoping later, that's an extension, not
  assumed here
- Import mechanics (how a file becomes a `LibraryEntry` in the first
  place) — phase 10

## User stories

- As **a user whose source server went offline**, I want the books I
  already added to stay in my library, clearly marked unavailable, not
  silently removed.
- As **a user organizing a "want to read" shelf**, I want to add the
  *book*, not have to pick which specific file/edition represents it in
  the collection.
- As **phase 08/10**, I want a clear place to record "this file is now
  part of the user's library" without that record depending on the
  source staying reachable forever.

## Functional requirements

- **FR-1** A `LibraryEntry` MUST reference exactly one `Edition` (not a
  `Work` directly) and MUST record when it was added. This is the unit of
  "the user has this."
- **FR-2** A `Work` MUST NOT have its own stored "in library" flag —
  whether a `Work` is in the library is computed as "at least one of its
  `Edition`s has a `LibraryEntry`," derived at query time, never persisted
  as a separate fact that could drift out of sync with the `LibraryEntry`
  records it's derived from.

  **"Its `Edition`s" means the merge-resolved set.** A `Work` merged into
  another keeps its own `Edition`s — `domain-bibliographic.md` FR-4's
  merge moves nothing — so this computation MUST run over the union of the
  `Work`'s `Edition`s and those of every `Work` merged into it,
  transitively, exactly as that FR requires of any read that walks a
  `Work`'s relations. Computed over raw parentage instead, it answers
  *false* for a book the user demonstrably owns the moment a phase-10
  automated match merges its `Work` — the `LibraryEntry` is untouched and
  correct, and the derived answer is wrong
  ([`0048`](../reviews/0048-phase02-correctness-review.md), finding 3).
  This is the one place where a bibliographic operation changes a library
  answer, and neither spec previously mentioned the other.
- **FR-3** A `LibraryEntry`'s existence MUST NOT depend on current
  availability (`domain-source.md`'s concern) — creating one doesn't
  require the source to be reachable at that instant (a user might add a
  Work while their source is briefly down, if the metadata is already
  known), and a source becoming unreachable or a file vanishing MUST NOT
  delete or invalidate an existing `LibraryEntry`. The entry persists;
  only its *availability* (read separately, `domain-source.md`) changes.
- **FR-4** A `Collection` MUST contain `Work`s, not `Edition`s or files —
  closing phase 02's explicit open question. Organizing by the abstract
  book, not by which specific file represents it, matches how a shelf or
  a reading list is actually used; if a `Work` has multiple `Edition`s
  and only one is in the library, the `Work` still shows in the
  collection, not a specific edition.
- **FR-5** Collections MUST have exactly one copy of state (the shared
  database, `architecture-persistence.md`) — no per-device local storage,
  no CRDT merge logic, no conflict resolution, because there is
  structurally nothing to conflict: every device reads and writes the
  same rows through the same API. This is the precise meaning of "local
  and unsynchronised" (phase 02's roadmap scope) reconciled with "same
  collections" (README): one copy, reached identically by every device,
  not a copy-per-device that happens to match.
- **FR-6** Removing a `LibraryEntry` (the user deliberately removes a
  book) MUST NOT cascade-delete the `Work` or `Edition` themselves — the
  bibliographic record can outlive the user's ownership of it (e.g. it
  stays visible in Discover, or another still-owned `Edition` of the same
  `Work` keeps the `Work` in the library per FR-2). It also MUST NOT touch
  `Collection` membership at all: `Collection`s hold `Work`s (FR-4), not
  `LibraryEntry`s or `Edition`s, so removing a `LibraryEntry` has no
  collection-level cascade to perform — a `Work` can be, and often is
  (the "want to read" case), a `Collection` member with zero
  `LibraryEntry`s referencing any of its `Edition`s.
- **FR-7** At most one `LibraryEntry` MUST exist per `Edition` — adding an
  already-owned `Edition` again is a no-op, not a second row. This is
  deliberately Edition-level, not Format-level: `domain-source.md`
  tracks which `Format`s a `SourceOffering` provides separately, but
  *which format was actually fetched or is cached* for a given owned
  `Edition` is not this type's concern — it's a client-side question
  (what a device chose to download or cache locally), addressed in
  `domain-events.md`'s reasoning about the design reference's "offline
  copies" concept, not modelled as a second dimension on `LibraryEntry`.
- **FR-8** An empty `Collection` (zero `Work`s) is a legal, ordinary
  state, same reasoning as `domain-bibliographic.md` FR-3's "zero is not
  an error."
- **FR-9** `Collection` membership carries its own added-at timestamp
  per `Work`, independent of any `LibraryEntry`'s — you can want a book
  (in a collection) before or after you own it (a `LibraryEntry`), and
  the two timestamps track different events.
- **FR-10** A `Collection` MUST be deletable. Deleting one removes its
  membership records only — it MUST NOT cascade to `Work`, `Edition`, or
  any `LibraryEntry`, the same non-cascading reasoning FR-6 already
  applies to removing a `LibraryEntry`.

## Non-functional requirements

- **Performance** — not applicable; no I/O in this layer.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable to a pure domain model.
- **Reliability** — FR-3's independence from availability is the
  reliability property: a flaky source (constitution §4 — every source
  response is hostile, including "briefly unreachable") never costs the
  user their library.
- **Observability** — this spec emits events (added, removed, collection
  changed) via `domain-events.md`'s pattern, not logging directly.

## Domain model

- **`LibraryEntry`** — internal ID, `Edition` ID (required, FR-1, at most
  one entry per `Edition`, FR-7), added-at timestamp. No availability
  field — that's read from `domain-source.md`'s model, not duplicated
  here.
- **`Collection`** — internal ID, name (constrained per
  `domain-bibliographic.md` FR-5's validation pattern — a collection name
  is exactly the same class of hostile-input-adjacent string a title is),
  member `Work` IDs each with their own added-at timestamp (FR-4, FR-9) —
  legally empty (FR-8), deletable (FR-10).
- **Computed, not stored**: "is this `Work` in the library" (FR-2), "which
  `Collection`s is this `Work` in."

## API and contracts

Not applicable — pure Go model. `architecture-backend.md`'s repository
interfaces will define how `LibraryEntry`/`Collection` are queried and
persisted; that's phase 03's job, not this spec's.

## State transitions

```
Edition has no LibraryEntry -> LibraryEntry created (FR-1) -> Work "in library" (FR-2, computed)
LibraryEntry exists, source becomes unreachable -> LibraryEntry unchanged (FR-3); availability (elsewhere) changes
LibraryEntry removed -> Work/Edition records unchanged (FR-6); Work "in library" recomputes,
                         possibly still true if another Edition's entry exists
LibraryEntry removed -> every Collection is untouched (FR-6): Collections hold
                         Works, never LibraryEntrys, so there is no cascade to
                         perform. A Work stays a Collection member with zero
                         LibraryEntrys — the "want to read" case
```

Illegal: a `LibraryEntry` for an `Edition` that doesn't exist — rejected by
the domain service that owns entry creation, **not** at construction, since
whether an `Edition` exists is a property of the database rather than of the
ID value and no value constructor can decide it
([ADR 0020](../decisions/0020-graph-invariants-in-domain-services.md); an
earlier draft claimed a construction-time check,
[`0048`](../reviews/0048-phase02-correctness-review.md) finding 6). A foreign
key backs the same rule in the schema as defence in depth; a `Collection` containing a dangling
`Work` reference after that `Work`'s last `LibraryEntry` is removed —
membership in a collection is about the `Work` existing and being wanted,
not about current ownership, so this is actually legal: **a `Work` can be
in a `Collection` without currently being in the library** (a "want to
read" shelf listing a book you don't own yet is exactly this case) —
stated explicitly since FR-4/FR-6 could otherwise be misread as requiring
ownership for collection membership.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Attempt to create a `LibraryEntry` for a nonexistent `Edition` | The entry-creation domain service's reference check (ADR 0020), backed by a foreign key | A domain-level error | Refused; no entry created |
| Source goes offline after a `LibraryEntry` exists | Not this spec's concern to detect (`domain-source.md`) | The `LibraryEntry` unaffected; availability reads separately reflect the outage | Nothing — this is FR-3's whole point |
| `LibraryEntry` removed while its `Work` is in two `Collection`s | FR-6 | Both `Collection`s unaffected — they reference the `Work`, never the `LibraryEntry` (FR-4) | Only the `LibraryEntry` is deleted; no `Collection`-level cascade exists to perform (FR-6) |
| Same `Edition` added to the library twice | FR-7's uniqueness rule | No visible change the second time | No-op; no second `LibraryEntry` row created |

## Security considerations

- **Collection names are hostile input** — a user-typed string, same
  validation bar as `domain-bibliographic.md` FR-5/FR-6 (length bound,
  control-character rejection). Named explicitly here so it isn't assumed
  "user's own data, therefore safe" (constitution §4: "the user configured
  it, so it's fine" is not a threat model).
- **No availability-based information leak** — FR-3 keeps `LibraryEntry`
  and availability separate, which also means a source outage can't be
  used to infer anything about library contents by an attacker probing
  availability endpoints; ownership records don't disappear or appear
  based on external, unauthenticated signals.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | FR-1 through FR-6, table-driven |
| Integration | Not applicable — no I/O |
| Property-based | FR-2's computed-not-stored invariant: for any set of `LibraryEntry` creations/removals, "Work in library" always matches "at least one Edition has an entry," generated adversarially |
| Table-driven | The FR-6/State-transitions distinction: Work in library only, Work in collection only, Work in both, Work in neither — all four combinations as explicit fixtures |

## Acceptance criteria

- [ ] `LibraryEntry` references `Edition`, never `Work`, directly
- [ ] "Work in library" has zero stored representation — a test asserting
      no such field/column-shaped concept exists in the type, only a
      derived query
- [ ] A test proves a `LibraryEntry` survives its `Edition`'s source
      reporting unavailable (mocked/faked availability signal, since
      `domain-source.md` owns the real thing)
- [ ] A test proves a `Work` can be a `Collection` member with zero
      `LibraryEntry`s referencing any of its `Edition`s (the "want to
      read" case)
- [ ] A test proves adding the same `Edition` twice produces one
      `LibraryEntry`, not two (FR-7)
- [ ] A test proves removing a `LibraryEntry` never changes any
      `Collection`'s membership (FR-6)
- [ ] A test proves deleting a `Collection` removes its membership rows
      but leaves every `Work`, `Edition`, and `LibraryEntry` untouched
      (FR-10)

## Open questions

- **Collection ordering/nesting** — flat collections assumed (a `Work` is
  either in a `Collection` or not); nothing here addresses ordering within
  a collection or nested collections. Not raised by phase 02's scope,
  not invented here.
- **Collection sharing beyond "the whole instance sees the same ones"
  (FR-5)** — whether a future multi-account phase 12 needs private,
  per-account collections is explicitly out of scope (Non-goals); FR-5's
  "one copy" model would need revisiting if so, not assumed compatible.

## References

- `.claude/roadmap/02-domain/README.md` — phase objective, the
  local-and-unsynchronised phrasing this spec reconciles with README.md
- `domain-bibliographic.md` — `Work`/`Edition` this spec references by ID
  only, never embeds
- `domain-source.md` — availability, kept explicitly separate (FR-3)
- README.md — "same books, same collections, same page you left off on"
- `architecture-persistence.md` — the single shared database FR-5's "one
  copy" reasoning depends on
- Constitution §3 (domain boundaries), §4 (hostile input)
