# T24 — 11 repository implementations — task list

Full plan: [`tasks/plan-t24-repositories.md`](plan-t24-repositories.md).

## Decisions

- [x] T24-D1 — production IDGenerator: crypto/rand UUID v4, stdlib only
- [x] T24-D2 — complete 4 incomplete interfaces + add 4 missing ones (domain layer)
- [ ] T24-D3 — schema: 11 tables, TEXT PKs, snake_case, one migration file
- [ ] T24-D4 — outbox table built now, writer deferred (no real caller exists yet)
- [ ] T24-D5 — repository shape: one file per aggregate, no query builder
- [ ] T24-D6 — shared transaction-executor helper, reused by all 11

## Tasks

**Tier 0 — foundations**
- [x] R0 — production IDGenerator (`internal/idgen`)
- [x] R1 — complete/add domain repository interfaces — extended existing fakes and every dependent phase 02 test still compiles and passes (218+ subtests), plus new round-trip tests for every new/extended method
- [ ] R2 — transaction-executor helper + Transactor implementation

**Checkpoint R-A** — foundations green

**Tier 1 — schema**
- [ ] R3 — 00002_phase02_schema.sql (11 tables + outbox)

**Checkpoint R-B** — schema applies to empty and populated databases

**Tier 2 — repositories**
- [ ] R4 — WorkRepository, AuthorRepository (+ SQL-injection proof, built once)
- [ ] R5 — EditionRepository
- [ ] R6 — LibraryEntryRepository, CollectionRepository (+ concurrent unique-constraint race)

**Checkpoint R-C** — bibliographic + library green

- [ ] R7 — SourceRepository, SourceOfferingRepository (+ real uniqueness-key constraint proof)
- [ ] R8 — ReadingProgressRepository, BookmarkRepository, HighlightRepository, ReadingPreferencesRepository (+ singleton-per-Work constraint proof)

**Checkpoint R-D** — all 11 repositories green

**Tier 3 — full-set proofs**
- [ ] R9 — cross-repository transaction-atomicity proof (SourceRemovalService's real cascade)
- [ ] R10 — wire real repositories into cmd/server's run.go (T26's repository half, pulled forward)

**Checkpoint R-E (final)** — full suite green against real Postgres, cmd/server reaches Ready with real repositories, report back into tasks/todo.md
