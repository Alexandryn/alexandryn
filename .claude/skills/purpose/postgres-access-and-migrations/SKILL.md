---
name: "postgres-access-and-migrations"
description: "Alexandryn's PostgreSQL access pattern: pgx native (never database/sql for application queries), parameterized queries only, transaction boundaries owned by the repository method, and goose migrations embedded via go:embed. Use whenever writing repository code in internal/persistence/postgres or a migration file."
---

Assumes ADR 0012 (`pgx`), ADR 0013 (`goose`), and `backend-persistence.md`
(all `APPROVED`/`Accepted`) — this skill restates their decisions as a
checklist. Read those for the *why*.

## Driver — `pgx` native, not `database/sql`

`internal/persistence/postgres` uses `pgx`'s own API (`pgxpool.Pool`,
`pgx.Row`, `pgx.Rows`) directly — never wrap it behind `database/sql`.
The **one** exception: the migration runner opens a short-lived `*sql.DB`
via the `pgx/v5/stdlib` compatibility shim, solely because `goose`'s
public API requires it, scoped to the single function that calls
`goose.Up`, closed immediately after it returns, never stored or reused
(`backend-persistence.md` FR-6). If you find yourself reaching for
`database/sql` anywhere else, stop — that's ADR 0012's reasoning being
quietly walked back.

## Pool — one per process, constructed once

`MinConns: 2`, `MaxConns: 10` by default (`DB_POOL_MAX_CONNS`
overridable), constructed in `main`/`run`, passed to repository
constructors — never a package-level pool variable
(`backend-persistence.md` FR-1). `MaxConnLifetime`/`MaxConnIdleTime` stay
at `pgxpool`'s own defaults unless a future spec argues otherwise.

## Queries — parameterized, always, no exceptions

Every query uses `pgx`'s own argument placeholders (`$1`, `$2`, ...).
String concatenation or `fmt.Sprintf` building SQL around any value that
traces back to a source, a filename, or any other hostile input
(constitution §4) must not exist anywhere in this package
(`backend-persistence.md` FR-3) — there is no query builder here to make
this structurally hard to get wrong; it's discipline, checked by a
static scan in CI and by a live adversarial test (a hostile string
round-tripped through a real query, with the actual SQL text inspected
via a `pgx.QueryTracer` to confirm a placeholder was used, not just that
the payload happened not to break anything).

## Transactions — one per domain operation, owned by the repository method

A multi-step operation that must be atomic runs inside one
`pool.Begin(ctx)` / `tx.Commit(ctx)` / `tx.Rollback(ctx)` — decided
entirely inside the repository method implementing that operation, never
left implicit across separate pool calls a caller has to sequence
correctly (`backend-persistence.md` FR-4). `internal/domain` never sees a
`pgx.Tx`.

Because the pool hands separate connections to separate `Begin()` calls,
an outer test-level transaction does not compose with a repository
method's own internal transaction — integration tests use truncate-based
teardown between tests, not an outer rollback (`backend-test-harness.md`
FR-3, the gap review `0022` caught once already).

## Migrations — `goose`, embedded, forward-only

Migration files live in `internal/persistence/postgres/migrations`,
embedded via `go:embed`, applied by `goose.Up` immediately after a
successful connection and before repository construction
(`backend-persistence.md` FR-6). Forward-only — no down-migration path
is designed or expected (`architecture-persistence.md` FR-4/FR-5). A
failed migration is a startup failure, full stop; the next startup
attempt detects the partial state via `goose`'s own tracking table and
also refuses, rather than silently continuing past it.

Never log a migration-connection failure's raw underlying error text —
it can embed the DSN. Use a fixed, generic message for the
connection-open case specifically; the migration filename and `goose`'s
own error text are still fine to log for a genuine SQL/schema failure,
which carries no connection-string risk.

## `DATABASE_URL` vs `TEST_DATABASE_URL`

Two different things, never conflated: `DATABASE_URL` (optional, no
default, read only through `internal/config`) is the dev/CI bypass that
skips the production spawn sequence entirely when present
(`backend-configuration.md` FR-4, `backend-persistence.md` FR-5).
`TEST_DATABASE_URL` (`backend-test-harness.md` FR-2) is test-only
plumbing, read directly by the integration-test harness, never through
`internal/config`, and never reachable by production code at all.

## Before opening a PR

Run this project's `postgres` MCP server to sanity-check a migration or
query against the real dev database before committing it.
