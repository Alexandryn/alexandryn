# Phase 02 (Domain) — implementation plan

## Context

Alexandryn's phase 02 domain model has five `APPROVED` specs
(`domain-bibliographic.md`, `domain-library.md`, `domain-source.md`,
`domain-reading.md`, `domain-events.md`) and zero real Go implementation —
`internal/domain` currently holds only `error.go` (`domain.Error`, built in
phase 03's T7). Phase 03's own plan (`tasks/plan.md`) stopped at Checkpoint G
for exactly this reason: `backend-persistence.md` FR-2's 11 repository
implementations need real domain types to satisfy interfaces against, and
none exist yet. This plan is what Checkpoint G asked for.

This project runs strict TDD (RED → GREEN → Refactor, per FR, test before
code) and the same conventions phase 03 already established:
`internal/domain` never imports `internal/transport`/`internal/persistence`
(enforced by `scripts/check-import-boundaries.sh`, reused unchanged — no new
lint mechanism needed), no package-level globals, the six-category
`domain.Error` taxonomy (already built), and Clock/FS/IDGenerator injected
instead of `time.Now()`/`os.*`/direct randomness
(`.claude/skills/purpose/go-backend-conventions/SKILL.md`).

**Named exclusion, not a silent omission**: `domain-reading.md`'s FR-6/FR-7
(`ReconcileProgress`) are separately blocked — the spec's own status header
says "must not be implemented until amended" (furthest-wins-with-override is
not commutative as currently written; a pre-existing defect, unrelated to
today's work). This plan builds every other part of `domain-reading.md`
(FR-1/2/3/4/5/8 — `ReadingProgress`, `ProgressReport`, `Bookmark`,
`Highlight`, `ReadingPreferences`, computed status) and explicitly does not
touch FR-6/FR-7.

## Architecture decisions (resolve once, don't re-derive per aggregate)

- **E0 — ID types.** Distinct named string types per aggregate
  (`WorkID`, `EditionID`, `AuthorID`, `LibraryEntryID`, `CollectionID`,
  `SourceID`, `SourceOfferingID`, `ReadingProgressID`, `BookmarkID`,
  `HighlightID`) rather than one bare `domain.ID` — no spec dictates the Go
  shape, and distinct types catch "passed an `AuthorID` where a `WorkID`
  was expected" at compile time, for free, at zero runtime cost. `Subject`
  and `Language` stay identity-free value types per their own spec text (no
  ID at all).
- **E1 — `IDGenerator` interface, declared in `internal/domain`.**
  `type IDGenerator interface { NewID() string }` — this is the exact shape
  `internal/testutil.FakeIDGenerator` (built in phase 03's T1) already
  implements; zero changes needed there. Every aggregate's ID is generated
  at construction time via an injected `IDGenerator`, never derived from
  external data (`domain-bibliographic.md` FR-1), matching this project's
  existing Clock/FS injection pattern exactly.
- **E2 — one shared validation file, `internal/domain/validate.go`.**
  `validateBoundedText(s string, maxLen int, field string) error` (length
  bound + control-character rejection, `domain-bibliographic.md` FR-5/FR-6's
  rule, reused for every title/subtitle/publisher/author-name/subject/
  collection-name/source-label/bookmark-label/highlight-note field across
  all five specs — one implementation, not eleven copies), plus
  `validateLanguageTag(s string) error` (BCP-47 shape) and
  `validateISBN(s string) error` (ISBN-10/13 shape, per
  `domain-bibliographic.md` FR-2). Every validator returns a `*domain.Error`
  with category `InvalidInput`.
- **E3 — domain services live beside their aggregate, not in one shared
  file.** `internal/domain/work_merge_service.go`,
  `library_service.go`, etc. — one service type per operation-cluster,
  matching `architecture-backend.md`'s "one file per repository" convention
  applied to services. Each service is constructed with only the domain-
  declared repository interfaces it actually needs (ADR 0020, Option B).
- **E4 — repository interfaces defined in this plan are minimal, not
  `backend-persistence.md` FR-2's full CRUD shape.** This plan declares only
  the read methods a domain service needs to enforce its own invariant
  (e.g. `WorkRepository` needs `FindByID` and whatever the merge-chain walk
  requires; it does not need `Delete` or `List`). Phase 03's T24 (11 full
  repository implementations) will need to satisfy a broader interface —
  **flagged explicitly so T24 doesn't assume this plan already produced its
  final shape**; reconciling the two is T24's own job, not this plan's.
- **E5 — event emission returns the event, doesn't send it anywhere.**
  No outbox exists in domain scope (`domain-events.md`'s own Non-goals defer
  transport to phase 03/09, and ADR 0021's outbox is phase 03 persistence
  work). Every domain service method that mutates state and has a
  catalogued event type returns `(result, domain.Event, error)` (or an
  `Event` slice, for the rare multi-emission case — none currently expected,
  per `domain-events.md` FR-5's "exactly one" rule). The caller (eventually
  phase 03's transactional-outbox write) is responsible for persisting it in
  the same transaction as the mutation. **This is this plan's own call, not
  dictated by any spec** — flagged for confirmation alongside the rest of
  this document, not picked silently.
- **E6 — events package location.** `internal/domain/event.go` for the
  sealed `Event`/`PublicEvent`/`SensitiveEvent` interfaces plus every
  concrete event type from `domain-events.md`'s catalog (18 types across
  four sibling domains). Built early (Tier 1), before any domain service
  that needs to emit one.

## Task list

**Tier 0 — shared foundations**

- **P0.** ID types (E0) + `IDGenerator` interface (E1). RED: a test proving
  two aggregates' ID types are not mutually assignable (a compile-time
  proof, same pattern as phase 03's negative-compilation fixtures) and that
  `testutil.FakeIDGenerator` satisfies `domain.IDGenerator` without
  modification.
- **P1.** Shared validators (E2): `validateBoundedText`,
  `validateLanguageTag`, `validateISBN`. RED: table-driven cases per
  validator — empty string, whitespace-only, over-length, control
  characters (null byte, terminal escape sequence), valid boundary cases,
  a valid ISBN-10 and ISBN-13, an invalid checksum-shaped-but-wrong string,
  a malformed BCP-47 tag.

  > **Checkpoint P-A** — P0+P1 green, `go vet ./...` clean,
  > `scripts/check-import-boundaries.sh` clean, before any aggregate type
  > is written.

- **P2.** `internal/domain/event.go` (E6): sealed `Event` base,
  `PublicEvent`/`SensitiveEvent` markers, and all 18 concrete event types
  from `domain-events.md`'s catalog (`WorkCreated`, `EditionCreated`,
  `AuthorCreated`, `WorkImported`, `EditionImported`, `WorkMerged`,
  `WorkMergeUndone`, `AuthorMerged`, `AuthorMergeUndone`,
  `WorkContainsAdded`, `WorkContainsRemoved`, `SourceCreated`,
  `SourceRemoved`, `SourceOfferingObserved`, `SourceOfferingRemoved`,
  `LibraryEntryAdded`, `LibraryEntryRemoved`, `CollectionCreated`,
  `CollectionMemberAdded`, `CollectionMemberRemoved`,
  `ReadingProgressUpdated`, `BookmarkCreated`, `HighlightCreated` — 23 in
  total, not 18; recount against the spec's Domain model section before
  writing the RED step, don't trust this list over the source). RED:
  table-driven, every type asserted to implement exactly the marker
  `domain-events.md` FR-2 assigns it; a negative-compilation fixture
  (build-tag-gated file, per `domain-events.md`'s own Test strategy)
  proving a `SensitiveEvent` cannot be passed to a `PublicEvent`-typed
  sink.

  > **Checkpoint P-B** — event catalog complete and marker-correct before
  > any service that emits one is written. This is deliberately before
  > Tier 2, not after — a service written against an incomplete catalog is
  > exactly how `domain-events.md`'s own retrospective note ("this list was
  > built the first time by naming each sibling spec's headline mutation,"
  > missing `Edition`/`Author` creation and containment events) happens
  > again.

**Tier 1 — `domain-bibliographic.md` construction-only types**

- **P3.** `Language`, `Subject` value types (FR-5). RED: valid/invalid
  construction per E2's validators; value equality (no identity).
- **P4.** `Author` (FR-7, construction only — merge fields present as an
  optional `MergedInto *AuthorID`, but the cycle-checking *operation* is
  Tier 2). RED: valid/invalid name construction; zero-external-references
  is legal (FR-3's reasoning, restated for Author).
- **P5.** `Work` (FR-1, FR-3, FR-6): internal ID (P0), title, subtitle
  (optional), authors (`[]AuthorID`), subjects (`[]Subject`), original
  language (optional `Language`), external references, `MergedInto
  *WorkID` (nil-valued at construction — recording a merge is Tier 2),
  `Contains []WorkID` (empty at construction — Tier 2). RED: zero-Editions-
  and-zero-external-refs is legal and has a passing test, not just a
  theoretical allowance (spec's own acceptance criterion, quoted almost
  verbatim); every free-form string field routes through P1's validator,
  proven by a rejection test per field, not just the happy path.
- **P6.** `Edition` (FR-2, FR-8's type-level "cannot exist without a
  parent Work," FR-10's own `Language`). RED: `Edition` literally cannot be
  constructed without a `WorkID` argument — a compile-time proof (no
  constructor overload omits it), matching FR-8's "not a nil-check
  convention" language exactly; ISBN optional-but-validated-when-present;
  a translation fixture (same `Work`, two `Edition`s, different
  `Edition.Language`) distinguished from a reissue (same `Work`, same
  language, different year/publisher) — both named explicitly in
  `domain-bibliographic.md`'s own Test strategy section, required fixtures,
  not generic placeholders.

  > **Checkpoint P-C** — all four bibliographic value/aggregate types green
  > before any domain service touches them.

**Tier 2 — `domain-bibliographic.md` graph invariants (ADR 0020 services)**

- **P7.** `WorkRepository` interface (E4, minimal: `FindByID(WorkID)
  (*Work, error)`, plus whatever the merge-chain walk in P8 needs — decide
  the exact minimal signature while writing P8's RED step, not
  speculatively here) and `AuthorRepository` interface, symmetric.
- **P8.** `WorkMergeService.RecordMerge`/`UndoMerge` (FR-4/FR-8): cycle-by-
  reachability check (walks the merge chain via `WorkRepository`, rejects
  `A→B` when `A` is already reachable from `B`), emits `WorkMerged`/
  `WorkMergeUndone` (P2). RED: the exact case ADR 0020/review 0048 finding
  9 fixed — `A→B` then `B→A` must be rejected, not just direct
  self-reference; undo is exactly the identity (clears one field, nothing
  else changes — a test reading through two levels of merge, per the
  spec's own acceptance criterion).
- **P9.** `AuthorMergeService`, same shape as P8, symmetric
  (`domain-bibliographic.md` FR-7 requires the identical reversibility
  guarantee).
- **P10.** `WorkContainmentService.AddContains`/`RemoveContains` (FR-9):
  cycle check run over the **merge-resolved** graph, not the raw one — the
  specific case the spec's own Open questions section resolved (`A`
  contains `B`, `B` merged into `A`, resolving first turns an invisible
  cycle into one the merge is rejected for). RED: exactly that fixture,
  plus a plain three-`Work` self-containment rejection. Emits
  `WorkContainsAdded`/`WorkContainsRemoved`.
- **P11.** `ResolveWork(WorkID) (*ResolvedWork, error)` — the read-time
  transitive union FR-4 requires (Editions/authors/subjects/external-refs/
  containment across a `Work` and everything merged into it). This is the
  function `domain-library.md` FR-2 depends on directly (Tier 3). RED: a
  three-level merge chain, reading the canonical `Work`'s resolved view
  returns the union, not just the canonical row's own fields.

  > **Checkpoint P-D (highest risk in this plan)** — merge/containment
  > cycle logic is genuinely tricky (two interacting graphs, per ADR 0020's
  > own "Medium confidence" note on this exact sub-choice). Full review of
  > P7–P11 before anything downstream depends on `ResolveWork`.

**Tier 3 — `domain-library.md`**

- **P12.** `LibraryEntry`, `Collection` construction types (FR-1, FR-4,
  FR-8, FR-9). RED: empty `Collection` is legal (FR-8); `Collection`
  membership carries its own added-at timestamp independent of any
  `LibraryEntry`'s (FR-9).
- **P13.** `EditionRepository` (E4, minimal: `FindByID` — needed both here
  for LibraryEntry's existence check and by P6's Edition/Work relationship
  reasoning) and `LibraryEntryRepository` (minimal: `FindByEdition` for
  FR-7's uniqueness check).
- **P14.** `LibraryService.AddEntry`/`RemoveEntry` (FR-1/FR-3/FR-6/FR-7):
  Edition-existence check via `EditionRepository` (ADR 0020's own named
  example), at-most-one-per-Edition via `LibraryEntryRepository`, no
  cascade on removal. Emits `LibraryEntryAdded`/`LibraryEntryRemoved`.
  RED: adding an already-owned Edition twice produces one entry, not two;
  removing an entry never touches a `Collection` (FR-6, table-driven across
  all four Work-in-library/Work-in-collection combinations, per the spec's
  own Test strategy).
- **P15.** `IsInLibrary(WorkID) (bool, error)` — computed via P11's
  `ResolveWork` plus `LibraryEntryRepository`, over the **merge-resolved**
  Edition set (FR-2's 2026-08-20 amendment, review 0048 finding 3's fix).
  RED: a `Work` merged into another still answers `true` for "in library"
  through the resolved view — the exact bug this amendment fixed.
- **P16.** `CollectionService.Create`/`Delete`/`AddMember`/`RemoveMember`
  (FR-4/FR-9/FR-10). Emits `CollectionCreated`/`CollectionMemberAdded`/
  `CollectionMemberRemoved`. RED: a `Work` can be a `Collection` member
  with zero `LibraryEntry`s (the "want to read" case, spec's own required
  fixture); deleting a `Collection` removes only its membership rows.

  > **Checkpoint P-E** — library/collection service layer green,
  > property-based test proving FR-2's computed-not-stored invariant holds
  > for any generated sequence of entry creations/removals/merges.

**Tier 4 — `domain-source.md`**

- **P17.** `FileReference` (FR-4, opaque constrained type — a test proving
  it cannot be constructed from a raw path/URL string, or from anything
  that looks like one, matching the spec's own acceptance criterion
  literally), `Source` (FR-1), `SourceOffering` (FR-2/FR-5, uniqueness by
  `Source`+`Edition`+`Format`). RED: same `Source` offering the same
  `Edition` in two `Format`s is two rows; re-observing one `Format` updates
  only that row's timestamp, not a new row.
- **P18.** `SourceRemovalService.Remove` (FR-6): cascades every
  `SourceOffering` referencing the removed `Source`, MUST NOT touch any
  `LibraryEntry`. Needs a minimal `SourceOfferingRepository`
  (`FindBySource`, per E4). Emits `SourceRemoved`/`SourceOfferingRemoved`;
  `SourceCreated`/`SourceOfferingObserved` emitted directly from
  construction, no service needed for those two (no graph invariant to
  enforce). RED: the spec's own required fixture — a `Source` with
  offerings for an `Edition` that also has a `LibraryEntry`: offerings
  removed, `LibraryEntry` untouched.

  > **Checkpoint P-F** — source domain green, cross-checked against P14's
  > `LibraryEntry` independence (same fixture, both directions).

**Tier 5 — `domain-reading.md` (excluding FR-6/FR-7)**

- **P19.** `Percentage` value type (`[0.0, 1.0]` bound, phase 02's own
  risk-table invariant), `ReadingProgress`/`ProgressReport` construction
  (FR-1/FR-2 — singleton-per-Work is a repository-level invariant, not
  constructible here without a repository; construct the *type* only in
  this task), `Bookmark`/`Highlight` (FR-3/FR-4 — a `Highlight`'s end
  position not preceding its start, construction-time per ADR 0020's
  single-value carve-out), `ReadingPreferences` (FR-5, new-device-starts-
  from-defaults). Computed reading status (FR-8) as a pure function over
  `Percentage`.
- **P20.** `PrecisePosition`'s Edition-belongs-to-Work construction check
  (per ADR 0020, this is graph-shaped — needs `EditionRepository` to read
  the `Edition`'s parent `WorkID` — so it's a service method,
  `AttachPrecisePosition`, not the bare constructor, despite the spec's
  original "checked at construction" wording that ADR 0020 corrected).
  RED: a `PrecisePosition` tagged to an `Edition` of a *different* `Work`
  is rejected (spec's own required acceptance-criterion test).
- **P21.** **Explicit non-task**: `ReconcileProgress` (FR-6/FR-7) is not
  implemented in this plan. `ReadingProgress`/`ProgressReport` types exist
  (P19) but no reconciliation function is written against them. Recorded
  here so a future session doesn't mistake the types' existence for the
  function's absence being an oversight.

  > **Checkpoint P-G** — reading-domain types green (minus FR-6/FR-7),
  > `domain.Event` emission proven for `ReadingProgressUpdated`/
  > `BookmarkCreated`/`HighlightCreated` at the construction sites that
  > have one (P19/P20) — note `ReadingProgressUpdated`'s real emission site
  > is `ReconcileProgress` itself (P21, not built), so this event type has
  > no real emitter yet in this plan; flagged, not silently assumed
  > covered.

**Tier 6 — close the loop**

- **P22.** Full-suite verification: `go build ./...`, `go vet ./...`,
  `scripts/check-import-boundaries.sh`, `go test ./...` (all packages),
  a property-based adversarial-input pass over every construction path
  (FR-8-style "no cycle, no orphan, no unvalidated string" sweep,
  `domain-bibliographic.md`'s own Test strategy requirement).
- **P23.** Re-run phase 03's Checkpoint G check for real: confirm
  `backend-persistence.md` FR-2's 11 aggregate types compile and satisfy
  this plan's repository interfaces (E4's minimal shape) — report back
  whether phase 03's T24–T26 can now proceed, and flag E4's
  minimal-vs-full-CRUD gap explicitly for whoever picks up T24.

  > **Checkpoint P-H (final)** — walk `.claude/roadmap/02-domain/README.md`'s
  > exit criteria item by item against what's actually built, same
  > discipline phase 03's plan closed with.

## Critical files

- `.claude/specs/domain-{bibliographic,library,source,reading,events}.md` —
  all five, `APPROVED`
- `.claude/decisions/0020-graph-invariants-in-domain-services.md` — the
  construction-vs-service split this entire plan is organized around
- `.claude/reviews/0048-phase02-correctness-review.md` — findings 3, 4, 5,
  6, 8, 9 are each a specific fixture this plan's RED steps must reproduce
  (P8's merge-cycle case, P15's merge-resolved-library case, P2's event
  marker case)
- `.claude/roadmap/02-domain/README.md` — phase exit criteria (P23)
- `.claude/skills/purpose/go-backend-conventions/SKILL.md` — package
  layout, naming, import-boundary/no-globals rules, reused unchanged
- `internal/domain/error.go` — existing `domain.Error`/`Category`, reused
  by every validator (E2) and service (Tier 2+)
- `internal/testutil/idgen.go` — `FakeIDGenerator`, the shape E1's
  `IDGenerator` interface is written to match
- `tasks/plan.md`, `tasks/todo.md` — phase 03's plan, whose Checkpoint G is
  what this plan resolves; P23 reports back into it

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Merge/containment cycle logic (P8–P11) is genuinely the hardest code in this plan — two interacting graphs, ADR 0020 itself rates its own chosen approach only Medium confidence here | High — a wrong fixed point means a non-terminating read, not just a wrong value | Checkpoint P-D is a dedicated review gate before anything downstream depends on `ResolveWork`; both review-0048-flagged cycle fixtures (self-merge-via-indirection, merge-after-containment) are named RED steps, not left implicit |
| E4's minimal repository interfaces may not match what phase 03's T24 needs for full CRUD | Medium — rework when T24 starts | Flagged explicitly in E4 and again at P23; not silently assumed compatible |
| E5 (event-emission-returns-the-event) is this plan's own invention, not spec-dictated | Low-medium — could need revisiting once phase 03/09 designs the real outbox wiring | Named as a flagged decision, not picked silently; cheap to change since nothing outside `internal/domain` consumes it yet |
| `ReadingProgressUpdated` (P19/P20) has no real emitter until `ReconcileProgress` exists | Low — known gap, not a defect | P21 records it explicitly; Checkpoint P-G restates it so it isn't rediscovered as a surprise |

## Open questions (for the maintainer, not decided here)

- **E5's event-emission shape** — return-the-event vs. some other mechanism
  (an injected recorder, an in-memory event log on the service) — flagged
  above, this plan's own call, cheap to revisit.
- **E4's interface scope** — whether to write phase 03's fuller CRUD
  repository interfaces now (bigger, but avoids a second pass at T24) or
  keep them minimal as planned (smaller, more likely to need extension) —
  this plan defaults to minimal; say if you'd rather go broader now.
