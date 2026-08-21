# T24 — 11 repository implementations

## Overview

`tasks/todo.md`'s T24: `backend-persistence.md` FR-2's 11 repository
implementations against real PostgreSQL, satisfying phase 02's domain
interfaces. Checkpoint G confirmed the domain types exist; this plan is
what T24 itself actually requires, which turned out larger than the
one-line task suggested once the interfaces were checked for
completeness against real persistence needs.

## What's actually needed, beyond "implement the 11 interfaces"

- **Four of phase 02's repository interfaces are incomplete for real
  persistence** — built only with the methods each domain *service*
  needed, per E4's own minimal scope, not full CRUD:
  `EditionRepository` has no `Save`; `SourceRepository` has no `Save`;
  `SourceOfferingRepository` has neither `Save` nor `FindByID`.
- **Four aggregates have no repository interface at all yet**:
  `ReadingProgress`, `Bookmark`, `Highlight`, `ReadingPreferences` — P20
  only needed `EditionRepository`, nothing needed a repository for these
  four directly.
- **No production `IDGenerator` exists anywhere** — only the interface
  (`internal/domain/id.go`) and `testutil.FakeIDGenerator`. `CollectionService.Create`
  is the only phase 02 code that calls one, and nothing outside tests has
  ever run it for real.
- **The outbox table** (ADR 0021, a 12th persisted thing, no domain type)
  needs a migration, but *writing* to it per-mutation needs a real
  caller composing a domain service's returned event with persistence —
  no such caller exists yet (that's phase 06+ feature work, HTTP
  handlers calling domain services). Scoped out of this plan explicitly,
  not silently skipped — see Decisions below.

## Architecture decisions (resolve once)

- **T24-D1 — Production `IDGenerator`**: a small `internal/persistence/postgres`
  (or a new `internal/idgen`) type generating UUID v4 via `crypto/rand`
  only — stdlib, no new dependency. `github.com/google/uuid` is already
  in `go.sum` (pulled in transitively) but promoting it to a direct
  dependency isn't warranted for ~15 lines of well-defined, stdlib-only
  logic (constitution §9: record a reason for every new dependency; "it's
  already transitively present" isn't one).
- **T24-D2 — Complete the four incomplete interfaces, add the four
  missing ones, in `internal/domain`** before writing any repository
  against them. This is domain-layer work (TDD, same as phase 02), not
  persistence-layer work — the interfaces belong to `internal/domain`
  regardless of who's about to implement them.
- **T24-D3 — Schema**: one table per aggregate (11), `TEXT` primary keys
  matching the domain's own string-based ID types (no UUID column type
  needed at the DB level — the ID *value* is a UUID string, stored as
  text, matching this project's existing convention of not over-typing
  columns beyond what's needed). `snake_case` column names
  (`backend-persistence.md`'s own stated convention for DB columns,
  distinct from the wire's `camelCase`). One migration file,
  `00002_phase02_schema.sql` — not eleven separate ones, since they're
  being designed and reviewed together and splitting them buys nothing
  at this stage.
- **T24-D4 — Outbox table, schema only, no writer wired yet**: the
  table itself is built (FR-2's own list needs it), but no repository
  method writes to it. Writing an event atomically with its mutation
  needs a real orchestrating caller (a domain service *and* a
  transaction spanning both the aggregate row and the outbox row) — no
  such caller exists yet in this codebase; the first real feature phase
  (06 onward) that calls a domain service and persists its result is
  where this actually gets exercised, not invented speculatively here.
  Flagged explicitly, matching this project's "don't design for
  hypothetical future requirements" discipline.
- **T24-D5 — Repository shape**: `type workRepository struct { pool
  *pgxpool.Pool }`-style, one file per aggregate
  (`work_repository.go`, ...), matching `backend-persistence.md` FR-2's
  named convention. No query builder (ADR 0012), parameterized queries
  only (FR-3).
- **T24-D6 — Transaction executor**: per ADR 0021, every repository
  method reads its executor (pool or an in-flight transaction) through
  one shared helper checking `context.Context` for a `Transactor`-opened
  transaction, falling back to the pool. Built once, reused by all 11.

## Task list

**Tier 0 — foundations**

- **R0.** Production `IDGenerator` (T24-D1): `crypto/rand`-based UUID v4,
  RFC 4122 variant/version bits set correctly. RED: format-shape
  assertions (36 chars, correct dashes, version nibble), uniqueness
  across many calls, no two calls ever collide in a large sample.
- **R1.** Complete `EditionRepository`/`SourceRepository`/
  `SourceOfferingRepository` (add `Save`, `FindByID` where missing) and
  add `ReadingProgressRepository`/`BookmarkRepository`/
  `HighlightRepository`/`ReadingPreferencesRepository` (T24-D2), in
  `internal/domain`. RED/GREEN in the domain package, same discipline as
  phase 02 — update the in-memory fakes (`fake_repository_test.go`) to
  match, since existing phase 02 tests depend on them still compiling.
- **R2.** Shared transaction-executor helper (T24-D6) +
  `internal/persistence/postgres`'s own `Transactor` implementation
  (ADR 0021 — begins a transaction, places it in context, commits/rolls
  back on return). RED/GREEN against real Postgres: a function that
  writes inside `InTx` and returns an error rolls back; one that returns
  nil commits; a nested call (executor already in context) reuses the
  same transaction rather than starting a second one.

  > **Checkpoint R-A** — foundations green before any repository is
  > written against them.

**Tier 1 — schema**

- **R3.** `00002_phase02_schema.sql` (T24-D3/D4): 11 aggregate tables +
  `outbox` table. RED: `TestMigrate_AppliesToAnEmptyDatabase` (already
  exists) extended to assert the new tables exist after migrating;
  `backend-persistence.md`'s own required fixture — a migration applied
  to a database *with existing rows* (seed via T3's placeholder
  migration having already run, then seed a row, then migrate again — or
  more precisely: this migration applied fresh should leave any
  already-migrated, unrelated data untouched, tested against a seeded
  fixture per `backend-test-harness.md` FR-4).

  > **Checkpoint R-B** — schema applies cleanly to both an empty and a
  > populated database before any repository is written against it.

**Tier 2 — repositories, grouped by owning spec (matches phase 02's own
tiers, so each repository's tests can cite the same fixtures the domain
layer already proved)**

- **R4.** `WorkRepository`, `AuthorRepository` (bibliographic). RED:
  `FindByID`/`Save`/`FindMergedInto` round-trip; the SQL-injection proof
  (a hostile string in every text field, via `pgx.QueryTracer` literal-
  SQL capture per the spec's own Test strategy) — built once here,
  reused by every later repository rather than repeated per file.
- **R5.** `EditionRepository` (bibliographic) — `FindByID`, `FindByWork`,
  `Save`. RED: `FindByWork` returns every Edition for a WorkID, proven
  against multiple Editions of one Work and Editions of an unrelated
  Work not leaking in.
- **R6.** `LibraryEntryRepository`, `CollectionRepository` (library).
  RED: `FindByEdition`'s NotFound-vs-found distinction against real rows;
  a concurrent unique-constraint race (`backend-persistence.md`'s own
  acceptance criterion) — two goroutines both trying to insert a
  `LibraryEntry` for the same Edition, exactly one succeeds, proven
  against the real DB's own unique constraint, not application-level
  locking.

  > **Checkpoint R-C** — bibliographic + library repositories green,
  > including the concurrency proof, before the remaining two groups
  > (source, reading) which don't need anything new architecturally.

- **R7.** `SourceRepository`, `SourceOfferingRepository` (source). RED:
  `SourceOffering`'s real `UniquenessKey()` behavior against a real
  unique constraint — re-observing the same (Source, Edition, Format)
  updates the row, a different Format inserts a new one, proven against
  Postgres's own constraint, not just the Go-level struct equality
  Tier 4 already proved.
- **R8.** `ReadingProgressRepository`, `BookmarkRepository`,
  `HighlightRepository`, `ReadingPreferencesRepository` (reading). RED:
  `ReadingProgress`'s FR-1 singleton-per-Work invariant — proven against
  a real unique constraint on `work_id`, the same pattern R6/R7 already
  established for their own uniqueness rules.

  > **Checkpoint R-D** — all 11 repositories green.

**Tier 3 — the two acceptance criteria that need the full set**

- **R9.** The cross-connection transaction-atomicity proof
  (`backend-persistence.md`'s own required acceptance criterion): a
  multi-step operation's transaction rolls back completely on a
  mid-operation failure — `SourceRemovalService`'s real cascade (R7's
  repositories, wired to the real `Transactor` from R2) is the concrete
  case, since it's the one phase 02 operation that actually spans two
  repositories. Proven against real Postgres: kill the operation
  partway (a deliberately-failing second delete), assert zero rows
  changed, not a partial cascade.
- **R10.** Wire real repositories into `cmd/server`'s `run.go`, replacing
  T18's stub (the `// TODO(D1)` comment left there specifically for
  this). This is T26's own task per `tasks/todo.md`, pulled forward here
  since it's the natural place to prove the full set actually
  constructs and the server still starts cleanly against real
  repositories, not stubs.

  > **Checkpoint R-E (final)** — full suite green (`go build`, `go vet`,
  > `go test ./... -race`, `go test -tags integration ./...` against
  > real Postgres), `cmd/server` starts and reaches `Ready` against the
  > real repository set, `scripts/check-import-boundaries.sh` and
  > `scripts/check-parameterized-queries.sh` both clean. Report back
  > into `tasks/todo.md`: T24 (and T26's repository-wiring half) done,
  > T25 (remaining persistence E2E) and the rest of T26/T27 still open.

## Critical files

- `.claude/specs/backend-persistence.md` — FR-1 through FR-8, this
  plan's own source
- `.claude/decisions/0012-postgres-driver.md`, `0013-migration-tool.md`,
  `0021-transaction-contract-and-event-outbox.md` — driver, migration
  tool, Transactor/outbox
- `tasks/plan-phase02-domain.md`, its 11 aggregate types and their
  domain-defined repository interfaces — what this plan implements
  against
- `internal/persistence/postgres/{pool,migrate,errors}.go` — existing
  phase 03 foundations this plan builds on, unchanged
- `internal/testutil` — `Clock`/`FS`/`IDGenerator` fakes,
  `TEST_DATABASE_URL` convention, truncate-based integration isolation —
  all reused, none reinvented

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Completing 4 interfaces + adding 4 more (R1) touches files phase 02's own PRs already merged — a regression here breaks already-shipped domain code | Medium | Full phase 02 domain test suite (218 subtests) re-run after every interface change, not just the new repository tests |
| The outbox table exists with no writer (T24-D4) — a future phase might build the writer inconsistently with what this schema assumes | Low-medium | Table shape documented in the migration file's own comment, referencing ADR 0021 directly, so a future implementer isn't guessing |
| Concurrent unique-constraint tests (R6/R8) are exactly the kind of test that's flaky under load | Medium | Run each such test repeatedly before trusting it, matching `tasks/plan.md`'s own T20 precedent for timing-sensitive tests |

## Open questions

- **Whether `SourceRepository`/`EditionRepository`'s newly-added `Save`
  should be insert-only or upsert** — no phase 02 service currently
  calls it (nothing constructs-then-persists a Source or Edition yet;
  that's phase 07/08/10 territory), so this plan picks upsert-by-ID
  (matching `WorkRepository.Save`'s own established pattern) without a
  real caller to validate the choice against yet.
