# Phase 03 (Backend foundation) — task list

Full plan with context/approach/rationale: [`tasks/plan.md`](plan.md).
Execute in order; each task is RED (existing test-plan case, see
`.claude/test-plans/`) → GREEN → Refactor. Stop at every lettered checkpoint.

## Decisions (resolve once, don't re-derive mid-task)

- [x] D0 — import-boundary lint mechanism: interim grep/file-walk script
      (`scripts/check-import-boundaries.sh`, plus
      `check-integration-test-parallelism.sh` and
      `check-parameterized-queries.sh`, each with its own test script)
- [x] D1 — phase 02 gating: re-checked at Checkpoint G 2026-08-20 (not
      ready) and again 2026-08-21 (ready) — resolved by building phase 02's
      domain types for real, `tasks/plan-phase02-domain.md`, not a
      provisional scaffold
- [x] D2 — `web/dist` placeholder embed for FR-8 (T16) —
      `internal/transport/http/webdist/placeholder/index.html`
- [ ] D3 — CI "contract test" stage as a named no-op (T27, not started)
- [x] D4 — macOS `pg-supervisor` E2E: portable subset built in-phase (T11);
      full E2E still needs a real macOS runner (unchanged, tracked below)
- [x] D5 — `backend-configuration.md` stale FR-8 phrasing cleanup — no
      stale loopback-only language remains outside the legitimate
      `DATABASE_URL`-in-dev mention
- [ ] D6 — branch-protection live-gating verification (infra, not code; Checkpoint H)
- [ ] D7 — `deployment-container-packaging.md` still `REVIEWED`, not
      `APPROVED` — T27 remains blocked on this

## Tasks

**Tier 0**
- [x] T0 — module bootstrap + D0's lint script
- [x] T1 — `internal/testutil`: Clock/FS/IDGenerator fakes + spy handler
- [x] T2 — build-tag / `TEST_DATABASE_URL` convention

**Checkpoint A** — T1+T2 green, `go vet ./...` clean, before anything else — passed

- [x] T3 — `internal/config` core (FR-1/2/3/4/6)
- [x] T4 — `internal/config` FR-5 (file resolution)
- [x] T5 — `internal/config` FR-7 (DATABASE_URL redaction)
- [x] T6 — `internal/config` FR-8 (BIND_ADDRESS, current ADR 0017 rule) + D5

**Checkpoint B** — `internal/config` fully green before `internal/logging` — passed

- [x] T7 — `internal/logging` + `internal/domain/error.go` bootstrap
- [x] T8 — `internal/persistence/postgres` FR-1 (pool)
- [x] T9 — `internal/persistence/postgres` FR-3 (parameterized-query static check)

**Tier 1**
- [x] T10 — FR-6 migration runner
- [x] T11 — FR-5 branch selection + FR-8 spawn-args (portable subset, D4)

**Checkpoint C** — restate D1, skip repositories, continue to transport/lifecycle — passed

- [x] T12 — FR-6/FR-7 Integration (no domain rows needed)

**Tier 2**
- [x] T13 — transport FR-1/FR-3/FR-4 (Unit)
- [x] T14 — transport FR-2/FR-6/FR-7 (limits, Content-Type, 404, wiring)
- [x] T15 — transport FR-5 (healthz/readyz)

**Checkpoint E** — transport Unit layer green; smoke-test all six error categories — passed

- [x] T16 — transport FR-8 (embedded web/dist fallback, D2)

**Tier 3 (highest risk)**
- [x] T17 — `cmd/server` FR-1 steps 1-4 + FR-2 + FR-7
- [x] T18 — `cmd/server` FR-1 steps 5-6 + FR-3 (repo stub per D1, explicit TODO — still stubbed, correctly, pending Checkpoint G)
- [x] T19 — `cmd/server` FR-4/5/6 (graceful shutdown)

**Checkpoint F** — Unit-layer fakes reviewed before real-listener/`-race` layers —
passed; security review recorded as
[`audit 0003`](../.claude/audits/0003-cmd-server-startup-shutdown.md)
(commit `650a40b`); one gap found and fixed same-session (`5726142`, FR-8
Mode A's certificate-validated-but-never-used-for-TLS gap) — Mode A now
refuses a public bind outright until phase 13 wires `ServeTLS`

- [x] T20 — `cmd/server` Integration + Concurrency (`-race`)

**Tier 4**
- [x] T21 — errors-and-logging FR-3/5/7/10/11 against the real chain
- [x] T22 — harness FR-3 Variant A, schema-at-head, parallelism static check

**Checkpoint G — domain-readiness gate.** Re-checked 2026-08-20: `internal/domain`
still has only `error.go` — the 11 real aggregate types don't exist as Go
code. Not silently picked: maintainer chose to build phase 02's domain
types for real now (not a provisional scaffold), starting below.

**Re-checked for real, 2026-08-21.** `tasks/plan-phase02-domain.md`'s
24 tasks are done — all 11 aggregate types `backend-persistence.md` FR-2
names (Work, Edition, Author, LibraryEntry, Collection, Source,
SourceOffering, ReadingProgress, Bookmark, Highlight, ReadingPreferences)
exist as real Go types in `internal/domain`, compile, and pass 218
passing subtests project-wide (77 in `internal/domain` alone), including
a 500-operation randomized fuzz proving the merge/containment cycle
checks hold under adversarial generation, not just hand-picked fixtures.
`go build`/`go vet`/`-race`/the import-boundary lint are all green.
**T24 is unblocked — flagged explicitly, not assumed silently
compatible:** that plan's own E4 declared `WorkRepository`/
`AuthorRepository`/`EditionRepository`/`LibraryEntryRepository`/
`CollectionRepository`/`SourceRepository`/`SourceOfferingRepository`
as *minimal* interfaces — only what each phase 02 domain service's own
invariant needed (e.g. `WorkRepository.FindByID` +
`FindMergedInto` + `Save`, not a full CRUD surface). T24's own 11
repository implementations will need a broader interface
(`backend-persistence.md` FR-2's actual shape — List, Delete, pagination,
etc., none of which phase 02 needed); reconciling the two — extending
each minimal interface to what T24 actually requires, or defining a
wider one the minimal ones embed — is T24's own job now, not solved
here. One further named gap carried over from phase 02, not this
checkpoint's to fix: `domain-reading.md`'s `ReconcileProgress` (FR-6/FR-7)
was deliberately not built (separately blocked, unrelated defect), so
`ReadingProgressRepository`'s eventual shape for T24 has no reconciliation
method to implement yet either — matches reality, not an oversight.

- [x] T24 — `backend-persistence.md` FR-2, all 11 repositories — done;
  full detail in `tasks/plan-t24-repositories.md`/`tasks/todo-t24-repositories.md`
  (Checkpoint R-E, final): all 11 repositories green against real
  Postgres, the cross-repository transaction-atomicity proof
  (`SourceRemovalService`'s cascade) holds, and `cmd/server`'s `run.go`
  now constructs every repository against the real pool at FR-1 step 6
  (T26's repository-wiring half, pulled forward — see T26 below)
- [x] T25 — remaining persistence Integration/E2E (macOS E2E excluded, D4) —
  done; full detail in `tasks/plan-t25-persistence-e2e.md`/
  `tasks/todo-t25-persistence-e2e.md` (Checkpoint E-D, final): a real
  `internal/persistence/postgres/supervisor` package (binary location,
  data directory init, port selection, Linux `Pdeathsig` orphan-prevention
  proven against a real process tree, Windows Job Object orphan-prevention
  written and cross-compiled but unverified — no macOS/Windows runner in
  this environment), wired into `cmd/server`'s real spawn path
  (`spawnPostgres`'s stub replaced), migration observability proven
  against real Postgres (no production code needed changing — already
  correct), and the `//go:build spawn` bundled E2E suite written and
  reviewed for both Linux and Windows but not executed here — no real
  `postgres`/`initdb` binaries in this sandbox, skips cleanly with a
  named reason instead of failing or silently passing
- [x] T26 — harness FR-3 Variant B — done; full detail in
  `tasks/plan-t26-harness-isolation.md`/`tasks/todo-t26-harness-isolation.md`
  (Checkpoint T26-D, final): `internal/testutil/packagedb.go`'s
  `EnsurePackageDatabase` gives each of the three integration-tagged
  packages (`internal/testutil`, `internal/persistence/postgres`,
  `cmd/server`) its own physical database, derived from
  `TEST_DATABASE_URL`, before any test in that package runs; `ci.yml`'s
  `-p 1` workaround removed, verified locally with `go test -race
  -tags=integration -count=5 ./...` at default parallelism against a
  real Postgres, all five re-executions green (real repository wiring in
  `run.go` was already done, pulled forward into T24's R10)
- [ ] T27 — CI workflow (D2/D3/D7 formalized)

**Checkpoint H (final)** — D6 verification, D5 cleanup PR, full-suite run,
roadmap exit-criteria walked item by item (including fixing the exit-criteria
line that still describes the pre-ADR-0017 loopback-only rule).

## Carry-overs — noted, not acted on this pass

- [ ] **FR-8 Mode B**: confirm the authentication condition is asserted
  *independently* of the address classification, not implied by it —
  i.e. a loopback/private bind must not be treated as "therefore
  authenticated" anywhere in the startup path; the two checks are separate
  and both must actually run. Check when T6/T18 are implemented, not now.
- [ ] **Phase 13's CSRF item is blocked on phase 12's session-mechanism
  ADR** (the ADR `roadmap/12-authentication/README.md` now names as
  in-scope, per the 2026-08-18 session). Nothing to do until phase 12
  actually opens and that ADR exists.
- [ ] **Confirmed real, during T24's R2 (`tasks/plan-t24-repositories.md`)**:
  `go test -tags integration ./...` (default parallelism) can run two
  packages' integration tests against the same `TEST_DATABASE_URL`
  concurrently, and one package's schema-wide reset or table drop can
  wipe another's in-flight fixture — exactly the composability hazard
  T26's own harness FR-3 Variant B names, previously suspected, now
  reproduced. `-p 1` serializes packages and avoids it; not a fix, just
  the workaround T24 used for its own verification. Fixed by T26 (see
  above) — `-p 1` is no longer needed or present in `ci.yml`.
