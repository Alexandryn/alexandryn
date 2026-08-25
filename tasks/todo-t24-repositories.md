# T24 — 11 repository implementations — task list

Full plan: [`tasks/plan-t24-repositories.md`](plan-t24-repositories.md).

## Decisions

- [x] T24-D1 — production IDGenerator: crypto/rand UUID v4, stdlib only
- [x] T24-D2 — complete 4 incomplete interfaces + add 4 missing ones (domain layer)
- [x] T24-D3 — schema: 11 tables, TEXT PKs, snake_case, one migration file
- [x] T24-D4 — outbox table built now, writer deferred (no real caller exists yet)
- [ ] T24-D5 — repository shape: one file per aggregate, no query builder
- [x] T24-D6 — shared transaction-executor helper, reused by all 11

## Tasks

**Tier 0 — foundations**
- [x] R0 — production IDGenerator (`internal/idgen`)
- [x] R1 — complete/add domain repository interfaces — extended existing fakes and every dependent phase 02 test still compiles and passes (218+ subtests), plus new round-trip tests for every new/extended method
- [x] R2 — transaction-executor helper + Transactor implementation — found (not fixed, out of scope, already tracked as T26's harness Variant B) a real cross-package composability hazard: `go test ./...` runs packages in parallel by default, so concurrent integration tests across packages can wipe each other's fixture tables against the shared TEST_DATABASE_URL; `-p 1` serializes and fixes it

**Checkpoint R-A** — foundations green

**Tier 1 — schema**
- [x] R3 — 00002_phase02_schema.sql (11 tables + outbox), 19 physical tables (child/join tables for owned collections), real UNIQUE/FK constraints proven directly against Postgres

**Checkpoint R-B** — schema applies to empty and populated databases — done

**Tier 2 — repositories**
- [x] R4 — WorkRepository, AuthorRepository (+ SQL-injection proof, built once) — RehydrateWork/RehydrateAuthor added to internal/domain for the repository "read from storage" reconstruction path; SQL-injection proof via a real pgx.QueryTracer (internal/persistence/postgres/sql_injection_tracer_integration_test.go), reused as-is by AuthorRepository, meant to be reused by R5-R8 too
- [x] R5 — EditionRepository — no Rehydrate needed (unlike Work/Author, every Edition field is already a NewEdition constructor parameter); reused R4's SQL-injection queryTracer as-is
- [x] R6 — LibraryEntryRepository, CollectionRepository (+ concurrent unique-constraint race) — LibraryEntryRepository.Save is insert-only (ON CONFLICT DO NOTHING), not update-in-place like Work/Author/Edition's Save, since FR-7's at-most-one-per-Edition invariant means a second Save for an existing edition_id must lose the race, not overwrite; two real goroutines racing Save for the same EditionID proved exactly one succeeds and the other translates to Conflict, stable across 5 repeated runs

**Checkpoint R-C** — bibliographic + library green

- [x] R7 — SourceRepository, SourceOfferingRepository (+ real uniqueness-key constraint proof) — SourceOfferingRepository.Save upserts on the real UNIQUE constraint on (source_id, edition_id, file_reference_format), not on id, since FR-2/FR-3's real identity is that tuple: re-observing it updates FileReference/ObservedAt in place (proven against 1 row staying 1 row across two Saves with the same tuple), a different Format inserts a second row (proven at 2 rows); SourceRepository.Save is upsert-by-id, matching Work/Edition's own Save shape; reused R4's queryTracer SQL-injection proof for both
- [x] R8 — ReadingProgressRepository, BookmarkRepository, HighlightRepository, ReadingPreferencesRepository (+ singleton-per-Work constraint proof) — RehydrateReadingProgress added to internal/domain for the repository read path (PrecisePosition is only settable via ReadingProgressService.AttachPrecisePosition otherwise, which needs an EditionRepository); ReadingProgressRepository.Save is upsert-by-id like Work/Edition/Source, so FR-1's singleton-per-Work invariant is proven by a different id for an already-recorded WorkID surfacing as a real unique_violation on reading_progress.work_id's own UNIQUE constraint (translated to Conflict), not by the id-conflict clause; Bookmark/Highlight Save is upsert-by-id with no other uniqueness invariant; ReadingPreferences Save upserts by device_id (its natural PK) with settings marshalled to/from JSONB at the repository boundary; reused R4's queryTracer SQL-injection proof for all four

**Checkpoint R-D** — all 11 repositories green

**Tier 3 — full-set proofs**
- [x] R9 — cross-repository transaction-atomicity proof (SourceRemovalService's real cascade) — real SourceRepository/SourceOfferingRepository/Transactor wired through SourceRemovalService.Remove; the mid-operation failure is injected via a thin test-only decorator around SourceOfferingRepository that lets the first Delete call through to the real repository (a genuine row removal, inside the transaction) then fails the second — same "deliberately fail after a real write" idiom transactor_integration_test.go already established, since nothing in this schema naturally rejects a second DELETE at the constraint level; post-failure state verified through a second, independent *pgxpool.Pool connection (not the same tx handle), proving the already-executed first delete rolled back along with everything else, not just the second, failed one; a companion success-path test proves the same cascade actually removes everything when nothing fails
- [x] R10 — wire real repositories into cmd/server's run.go (T26's repository half, pulled forward) — new cmd/server/repositories.go holds a `repositories` struct (all 11 domain repository interfaces + Transactor) and `newRepositories(pool *pgxpool.Pool)`, constructed as part of FR-1 step 6 alongside pool construction (backend-service-lifecycle.md's own State transitions describes this as one step, not two — the log line stays "pool", no new "repositories" step); `runDeps.newPool`'s signature grew a `*repositories` second return value, updated at all six call sites across run.go/main.go/run_test.go/run_realserver_test.go/run_integration_test.go; nothing consumes the constructed repositories yet (phase 03 registers no /api/v1 routes) so run() holds them via `_ = repos` with a comment pointing at the future consumer, per FR-2's constructor-injection requirement (a struct field/local, never a package-level global); Checkpoint R-E's own criterion is proven by extending TestIntegration_ColdStartToReadyAgainstRealPostgres to capture what newRepositories actually built against a real pool and assert every field is non-nil, not a leftover stub

**Checkpoint R-E (final)** — full suite green against real Postgres, cmd/server reaches Ready with real repositories, report back into tasks/todo.md — done, see tasks/todo.md's own T24 entry
