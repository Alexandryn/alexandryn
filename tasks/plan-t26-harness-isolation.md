# T26 — harness FR-3 Variant B: cross-package integration-test DB isolation

## Overview

`tasks/todo.md`'s own T26 line: "harness FR-3 Variant B (still open —
the `-p 1` cross-package DDL race, not yet fixed); real repository
wiring in `run.go` is now done, pulled forward into T24's R10." T22
(Variant A, `backend-test-harness.md` FR-3) already built intra-package
isolation: schema-at-head via `goose`, truncate-based per-test teardown
(`testutil.TruncateTables`), a static check forbidding `t.Parallel()`
inside any `_integration_test.go` file
(`scripts/check-integration-test-parallelism.sh`). What's still open is
cross-package isolation: `internal/testutil`,
`internal/persistence/postgres`, and `cmd/server` each run their own
`TestMain`, each resetting or migrating the *same* `TEST_DATABASE_URL`
database (`DROP SCHEMA public CASCADE` or `goose.Up`). Under Go's default
package-level test parallelism, two packages' DDL can interleave and
collide — reproduced for real during T24's R2/R3 (two concurrent `CREATE
TABLE` statements collided on Postgres's own `pg_type` catalog,
recorded in `tasks/todo.md`'s carry-over section). `-p 1` serializes
package execution in `ci.yml` today as a workaround; this task replaces
it with real per-package isolation, closing audit finding A-03-06
(`.claude/audits/0003-cmd-server-startup-shutdown.md`) and the T26 line
in `tasks/todo.md`.

Three packages, three `TestMain` functions, one shared
`TEST_DATABASE_URL` today:
`internal/testutil/isolation_integration_test.go`,
`internal/persistence/postgres/migrate_integration_test.go`,
`cmd/server/run_integration_test.go`. All three currently call
`os.Exit(testutil.IntegrationTestMain(os.LookupEnv, ..., os.Stderr))`,
and downstream code (`postgres.Migrate`, each package's own
`testDB`/`resetSchema` helpers) reads `TEST_DATABASE_URL` directly via
`os.Getenv` — never through `internal/config`; that boundary is fixed
and not touched by this task.

## Decisions (resolve before/alongside the tasks that need them)

- **T26-D1 — isolation granularity: a distinct physical database per
  package, not a schema.** `ci.yml`'s own comment already frames the real
  fix as "per-package isolation so packages don't share one physical
  database." A freshly created database's `public` schema is already
  empty, so every existing `resetSchema` (`DROP SCHEMA public CASCADE;
  CREATE SCHEMA public;`) and `postgres.Migrate` call keeps working
  completely unchanged once pointed at it — no migration file becomes
  schema-aware, no `search_path` plumbing added anywhere. A
  schema-per-package alternative was weighed and rejected: it would
  require exactly the migration/`search_path` awareness this codebase
  doesn't have today, for no benefit over the database approach `ci.yml`
  already pointed at.
- **T26-D2 — naming: an explicit literal `pkgName` per call site,
  database name = `<TEST_DATABASE_URL`'s existing dbname`>_<pkgName>`.**
  Matches this project's stated preference for explicit over derived (no
  reflection on `t.Name()` or package import path). Creation is `CREATE
  DATABASE "<name>"` via a short-lived admin connection to the
  *original* `TEST_DATABASE_URL` database (already exists, in CI and
  locally). Postgres has no `CREATE DATABASE IF NOT EXISTS`, so a second
  run ignores SQLSTATE `42P04` (`duplicate_database`) — the same
  `errors.As(&pgErr)` pattern `internal/persistence/postgres/errors.go`'s
  `TranslateError` already uses for `23505`. The identifier is quoted via
  `pgx.Identifier{name}.Sanitize()`, not hand-built string concatenation.
  Chosen literals, one per package, matching the package's own short
  name: `testutil`, `postgres`, `cmdserver`.
- **T26-D3 — where the logic lives: `internal/testutil`, called
  explicitly from each `TestMain`; `IntegrationTestMain`'s existing
  signature and its three unit tests in `harness_test.go` stay
  untouched.** New file `internal/testutil/packagedb.go` (untagged,
  alongside `harness.go`/`truncate.go`), two functions: a pure
  `DerivePackageDatabaseURL(base, pkgName string) (dbName, url string,
  err error)` (unit-tested in `packagedb_test.go`, no real Postgres
  needed, runs in the default `go test ./...`), and the real I/O
  `EnsurePackageDatabase(ctx, baseDatabaseURL, pkgName string) (string,
  error)` (creates the database if missing, returns the derived URL).
  Each `TestMain` calls it inside the closure already passed to
  `IntegrationTestMain`, then `os.Setenv("TEST_DATABASE_URL", ...)`
  before calling `m.Run()` — every existing
  `os.Getenv("TEST_DATABASE_URL")` call site (`Migrate`, `testDB`,
  `resetSchema`) picks up the isolated database with zero changes to
  those call sites. Rejected: extending `IntegrationTestMain`'s own
  signature — would force all three of `harness_test.go`'s existing
  tests (proving the fail-loud gate) to change for a concern orthogonal
  to what that function gates. Variant B's own proof belongs beside T22's
  own file, the same way T22 proved Variant A by using it — new tests in
  `internal/testutil`, proving two package names get two independent,
  non-colliding databases and that a repeat call is idempotent.
- **T26-D4 — remove `-p 1` from CI, verify at default parallelism.** Once
  all three packages are isolated, `ci.yml`'s Integration tests step
  drops `-p 1` and its explanatory comment is rewritten to describe the
  real fix (referencing T26) instead of the workaround. Verified locally
  by running `go test -race -tags=integration ./...` at default
  parallelism repeatedly (matching T20/R6's repeated-run precedent for a
  timing-sensitive proof) against a real Postgres, if one is reachable in
  this environment — stated honestly either way, not assumed from the
  design alone.
- **T26-D5 — `TEST_DATABASE_URL`'s contract: unchanged externally,
  redefined internally as a base connection.** Nothing outside the Go
  test binary ever reads `TEST_DATABASE_URL` after CI or a developer sets
  it, so no external tooling contract breaks. Internally it now means
  "an admin-capable connection to an existing database, from which
  per-package databases are derived," not "the one database every
  package's tests run against." `CONTRIBUTING.md`'s `TEST_DATABASE_URL`
  row gets one clarifying clause; `backend-test-harness.md` FR-3 and the
  audit's A-03-06 row get updated once this lands (a docs task, not a new
  FR — Variant B was always implicit in FR-3's cross-package "does not
  affect other tests" language).
  `scripts/check-integration-test-parallelism.sh` is unaffected —
  confirmed, not assumed: it forbids `t.Parallel()` inside a package
  (Variant A's concern, intra-package), orthogonal to which physical
  database a package targets (Variant B, cross-package).
- Per-package databases are **not dropped** after a run — `CREATE
  DATABASE` is idempotent here (duplicate ignored) and
  `goose.Up`/`resetSchema` are idempotent against an existing database,
  so leaving them costs nothing and avoids a second failure-prone DDL
  path for cleanup with no real benefit.

## Task list

- [ ] **T26-1** — `internal/testutil/packagedb.go`: pure
  `DerivePackageDatabaseURL(base, pkgName string) (dbName, url string,
  err error)`. RED first: `packagedb_test.go` (untagged) covering a valid
  URL rewritten correctly (dbname suffixed, other components — user,
  host, port, query params like `sslmode` — preserved), an invalid URL
  returns an error, an empty `pkgName` returns an error.
- [ ] **T26-2** — same file: `EnsurePackageDatabase(ctx
  context.Background(), baseDatabaseURL, pkgName string) (string,
  error)`. Opens a short-lived `sql.Open("pgx", baseDatabaseURL)` admin
  connection (blank-imports `_ "github.com/jackc/pgx/v5/stdlib"` in this
  file), issues `CREATE DATABASE` with `pgx.Identifier{dbName}.Sanitize()`,
  ignores SQLSTATE `42P04` via `errors.As(&pgconn.PgError{})`, wraps any
  other connection-open failure in a fixed generic message (never the raw
  error text — it can embed the DSN, same rule
  `postgres-access-and-migrations` names for migration-connection
  failures), returns the real SQL/DDL failure text unchanged otherwise
  (no DSN risk in that case). RED first, as an integration test (needs
  real Postgres) — see T26-3.

  > **Checkpoint T26-A** — `DerivePackageDatabaseURL` unit-tested and
  > green in the default `go test ./...`; `EnsurePackageDatabase`
  > written, not yet wired into any real `TestMain`.

- [ ] **T26-3** — Variant B's own proof, new integration tests beside
  T22's own file (`internal/testutil/isolation_integration_test.go` or a
  new sibling `internal/testutil/packagedb_integration_test.go` in the
  same package): `EnsurePackageDatabase` called twice with different
  `pkgName` values against the same base URL produces two databases that
  exist independently (query `pg_database`); a second call with the
  *same* `pkgName` is idempotent (no error, same derived URL returned).
- [ ] **T26-4** — wire `internal/testutil/isolation_integration_test.go`'s
  `TestMain`: call `EnsurePackageDatabase(ctx, os.Getenv("TEST_DATABASE_URL"),
  "testutil")` before its existing `postgres.Migrate` call, `os.Setenv`
  the result back.
- [ ] **T26-5** — wire
  `internal/persistence/postgres/migrate_integration_test.go`'s
  `TestMain`: change from the bare `m.Run` form to the closure form,
  calling `EnsurePackageDatabase(ctx, ..., "postgres")` first.
- [ ] **T26-6** — wire `cmd/server/run_integration_test.go`'s `TestMain`:
  same closure-form change, `pkgName` `"cmdserver"`.

  > **Checkpoint T26-B** — all three packages isolated; full integration
  > suite still green with `-p 1` present as a safety net (not yet
  > removed from CI).

- [ ] **T26-7** — `ci.yml`: drop `-p 1` from the Integration tests `run`
  line; rewrite the explanatory comment block above it to describe
  per-package isolation (referencing T26) instead of the `-p 1`
  workaround.
- [ ] **T26-8** — local verification: `go test -race -tags=integration
  ./...` at default parallelism, run repeatedly against a real
  `TEST_DATABASE_URL` if one is reachable in this environment; record the
  outcome honestly either way (matching T25's own precedent for
  environment limits) rather than assuming the design alone proves it.

  > **Checkpoint T26-C** — default-parallelism integration run proven
  > stable (or its environment limitation stated plainly); `-p 1` gone
  > from CI.

- [ ] **T26-9** — docs: `CONTRIBUTING.md`'s `TEST_DATABASE_URL` row (one
  clarifying clause), `.claude/audits/0003-cmd-server-startup-shutdown.md`
  A-03-06 (mark Resolution fixed, correct the "Variant A's job"
  mislabeling the audit currently carries — `tasks/todo.md`'s own T22/T26
  split shows Variant A was schema-at-head/parallelism-static-check and
  Variant B is this cross-package gap), `backend-test-harness.md` FR-3
  (a short implementation note, no new FR).
- [ ] **T26-10** — `tasks/todo.md`: check off T26, close its checkpoint.

  > **Checkpoint T26-D (final)** — full suite green: `go build ./...`,
  > `go vet ./...`, unit `-race`, integration `-race` at default
  > parallelism, `scripts/check-integration-test-parallelism.sh` (still
  > clean, confirmed), `scripts/check-import-boundaries.sh .`,
  > `scripts/check-parameterized-queries.sh .`, `golangci-lint run
  > ./...`, `govulncheck ./...` — done. T26 complete; T27 (CI workflow
  > formalization) next, not started here.

## Critical files

- `.claude/specs/backend-test-harness.md` FR-3 — the isolation
  requirement this task closes the cross-package half of
- `.claude/audits/0003-cmd-server-startup-shutdown.md` A-03-06 — the
  concrete reproduction and the "Open — deferred to T22" line this task
  updates
- `internal/testutil/harness.go` — `IntegrationTestMain`, untouched by
  this task, the existing fail-loud gate T26 composes with rather than
  modifies
- `internal/testutil/truncate.go` — `TruncateTables`, Variant A's own
  mechanism, unrelated to this task but in the same package
- `internal/testutil/isolation_integration_test.go` — T22's own Variant A
  proof and `TestMain`; Variant B's own proof lives beside it
- `internal/persistence/postgres/errors.go` — `TranslateError`'s
  `errors.As(&pgconn.PgError{})` pattern, mirrored by T26-2's
  `42P04`-ignoring logic
- `internal/persistence/postgres/migrate_integration_test.go`,
  `cmd/server/run_integration_test.go` — the two `TestMain`s changing
  from bare `m.Run` to the closure form
- `.github/workflows/ci.yml` — the `-p 1` workaround and its explanatory
  comment, both replaced
- `scripts/check-integration-test-parallelism.sh` — confirmed unaffected,
  not changed
- `CONTRIBUTING.md` — `TEST_DATABASE_URL` row, one clause added

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| No real Postgres reachable in this authoring environment for the default-parallelism repeated-run proof (T26-8) | Medium — the core claim of this task ("`-p 1` is no longer needed") would be unverified here | State plainly if unverified, same honesty precedent as T25's `//go:build spawn` suite; first real proof happens wherever CI or a dev machine with Postgres runs it |
| `CREATE DATABASE` cannot run inside a transaction block; some drivers/pools implicitly wrap statements | Low — would surface immediately as a clear Postgres error, not a silent failure | `EnsurePackageDatabase` uses a plain `*sql.DB` connection via `sql.Open`, no explicit transaction wrapping, matching how `resetSchema`/`Migrate` already issue DDL |
| A leftover per-package database from a previous local run has drifted schema (e.g., a migration file was edited, not just added) | Low — same class of risk any persistent local test database already has, not new to this task | `goose.Up` is itself forward-only and drift-detecting (`backend-persistence.md` FR-6); not a new failure mode T26 introduces |

## Open questions

- Whether a real Postgres instance is reachable in this session's
  environment for T26-8's repeated default-parallelism run — checked at
  build time, not assumed here; if unreachable, stated as an explicit gap
  rather than skipped silently.
- Whether `pgx.Identifier{name}.Sanitize()` is available in the pinned
  `pgx` version (`go.mod`) — checked directly against the vendored API at
  build time, not assumed from general `pgx` documentation.
