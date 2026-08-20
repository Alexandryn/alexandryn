# Phase 02 (Domain) — task list

Full plan with context/decisions/rationale: [`tasks/plan-phase02-domain.md`](plan-phase02-domain.md).
Execute in order; each task is RED (see the plan's own RED-step description) →
GREEN → Refactor. Stop at every lettered checkpoint.

## Decisions (resolve once, don't re-derive mid-task)

- [x] E0 — ID types: distinct named string type per aggregate
- [x] E1 — `IDGenerator` interface in `internal/domain`, matching `testutil.FakeIDGenerator`
- [x] E2 — shared `internal/domain/validate.go` (bounded text, BCP-47, ISBN)
- [ ] E3 — domain services live beside their aggregate, one file per operation-cluster
- [x] E4 — repository interfaces here are minimal, not phase 03 T24's full CRUD shape (confirmed by maintainer 2026-08-20; flag for T24)
- [x] E5 — event emission returns the event; caller persists it (confirmed by maintainer 2026-08-20)
- [x] E6 — `internal/domain/event.go`, built in Tier 1 before any emitting service

## Tasks

**Tier 0 — shared foundations**
- [x] P0 — ID types (E0) + `IDGenerator` interface (E1) — merged, `internal/domain/id.go`
- [x] P1 — shared validators (E2): bounded text, BCP-47, ISBN — `internal/domain/validate.go`

**Checkpoint P-A** — P0+P1 green, `go vet ./...` clean, import-boundary lint clean

- [x] P2 — `internal/domain/event.go`: sealed `Event`/`PublicEvent`/`SensitiveEvent` + full catalog (23 types, recounted against the spec directly), negative-compile fixture confirmed to fail under `go vet -tags negativecompile`

**Checkpoint P-B** — event catalog complete and marker-correct before Tier 2

**Tier 1 — `domain-bibliographic.md` construction-only types**
- [ ] P3 — `Language`, `Subject` value types
- [ ] P4 — `Author` (construction only)
- [ ] P5 — `Work` (construction only, `MergedInto`/`Contains` nil/empty)
- [ ] P6 — `Edition` (type-level parent-`Work` requirement, translation-vs-reissue fixture)

**Checkpoint P-C** — bibliographic value/aggregate types green

**Tier 2 — `domain-bibliographic.md` graph invariants (ADR 0020 services)**
- [ ] P7 — `WorkRepository`/`AuthorRepository` interfaces (minimal)
- [ ] P8 — `WorkMergeService` (cycle-by-reachability, undo-is-identity)
- [ ] P9 — `AuthorMergeService` (symmetric with P8)
- [ ] P10 — `WorkContainmentService` (cycle check over merge-resolved graph)
- [ ] P11 — `ResolveWork` (transitive union read)

**Checkpoint P-D (highest risk)** — full review of P7–P11 before anything downstream depends on `ResolveWork`

**Tier 3 — `domain-library.md`**
- [ ] P12 — `LibraryEntry`, `Collection` construction types
- [ ] P13 — `EditionRepository`/`LibraryEntryRepository` interfaces (minimal)
- [ ] P14 — `LibraryService` (add/remove entry, no cascade, uniqueness)
- [ ] P15 — `IsInLibrary` over the merge-resolved Edition set (review 0048 finding 3's fix, proven directly)
- [ ] P16 — `CollectionService` (create/delete/add-member/remove-member)

**Checkpoint P-E** — library/collection service layer green, property-based computed-not-stored proof

**Tier 4 — `domain-source.md`**
- [ ] P17 — `FileReference`, `Source`, `SourceOffering` construction types
- [ ] P18 — `SourceRemovalService` (cascade offerings, `LibraryEntry` untouched)

**Checkpoint P-F** — source domain green, cross-checked against P14's independence fixture

**Tier 5 — `domain-reading.md` (excluding FR-6/FR-7)**
- [ ] P19 — `Percentage`, `ReadingProgress`/`ProgressReport`, `Bookmark`, `Highlight`, `ReadingPreferences`, computed status
- [ ] P20 — `AttachPrecisePosition` (Edition-belongs-to-Work service check)
- [ ] P21 — **explicit non-task**: confirm `ReconcileProgress` (FR-6/FR-7) was not implemented, recorded not omitted

**Checkpoint P-G** — reading-domain types green (minus FR-6/FR-7); `ReadingProgressUpdated` has no real emitter yet, flagged not hidden

**Tier 6 — close the loop**
- [ ] P22 — full-suite verification (`go build`, `go vet`, import-boundary lint, `go test ./...`, adversarial-construction sweep)
- [ ] P23 — re-check phase 03's Checkpoint G for real; report back into `tasks/todo.md` whether T24–T26 can proceed

**Checkpoint P-H (final)** — walk `.claude/roadmap/02-domain/README.md`'s exit criteria item by item
