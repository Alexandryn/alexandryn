# T26 — harness FR-3 Variant B: cross-package integration-test DB isolation

## Decisions

- [ ] T26-D1 — isolation granularity: distinct physical database per package, not a schema
- [ ] T26-D2 — naming: explicit literal `pkgName` per call site (`testutil`/`postgres`/`cmdserver`), `CREATE DATABASE` idempotent via ignoring `42P04`
- [ ] T26-D3 — logic lives in `internal/testutil/packagedb.go`, called explicitly from each `TestMain`; `IntegrationTestMain`'s own signature/tests untouched
- [ ] T26-D4 — remove `-p 1` from CI, verify at default parallelism
- [ ] T26-D5 — `TEST_DATABASE_URL`'s external contract unchanged, redefined internally as a base connection; docs updated accordingly

## Tasks

- [x] T26-1 — `internal/testutil/packagedb.go`: pure `DerivePackageDatabaseURL`, unit-tested (RED first)
- [x] T26-2 — same file: `EnsurePackageDatabase` (real I/O, `CREATE DATABASE` + idempotent `42P04` handling)

**Checkpoint T26-A** — `DerivePackageDatabaseURL` unit-tested and green; `EnsurePackageDatabase` written and proven against a real local Postgres (podman/supabase stack, `127.0.0.1:54322`), not yet wired into any real package `TestMain`

- [x] T26-3 — Variant B's own proof: two `pkgName`s produce two independent databases; repeat call is idempotent — `internal/testutil/packagedb_integration_test.go`, green against real Postgres
- [x] T26-4 — wire `internal/testutil/isolation_integration_test.go`'s `TestMain` (`pkgName` `testutil`)
- [x] T26-5 — wire `internal/persistence/postgres/migrate_integration_test.go`'s `TestMain` (`pkgName` `postgres`)
- [x] T26-6 — wire `cmd/server/run_integration_test.go`'s `TestMain` (`pkgName` `cmdserver`)

**Checkpoint T26-B** — all three packages isolated; full integration suite green (`-race -tags=integration -p 1`) against real local Postgres, `-p 1` still present as a safety net

- [x] T26-7 — `ci.yml`: drop `-p 1`, rewrite the explanatory comment
- [x] T26-8 — local verification: a real Postgres *was* reachable in this environment this session (podman/supabase stack, `127.0.0.1:54322`) — `go test -race -tags=integration -count=5 ./...` at default parallelism, all 5 real re-executions green, no collision

**Checkpoint T26-C** — default-parallelism run proven stable against a real local Postgres; `-p 1` gone from CI

- [x] T26-9 — docs: `CONTRIBUTING.md`, audit A-03-06 resolution + mislabeling fix, `backend-test-harness.md` FR-3 note
- [x] T26-10 — `tasks/todo.md`: check off T26

**Checkpoint T26-D (final)** — full suite green: `go build ./...`, `go vet ./...` (incl. `-tags=integration`), unit `-race`, integration `-race -tags=integration -count=5` at default parallelism (real Postgres, 5/5 green), `scripts/check-integration-test-parallelism.sh`, `scripts/check-import-boundaries.sh .`, `scripts/check-parameterized-queries.sh .`, `golangci-lint run ./...` (0 issues), `govulncheck ./...` (no vulnerabilities) — done. T26 complete; T27 (CI workflow formalization) next, not started here.
