# 0012. PostgreSQL access uses `pgx` natively, not `database/sql`

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-14 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

ADR 0004 decided the engine (self-hosted PostgreSQL). `architecture-
persistence.md` FR-3 requires a bounded connection pool without picking an
implementation, and left "driver/access layer (e.g. `database/sql` plus
driver, or a query builder)" to phase 03 explicitly. `architecture-
backend.md` FR-2 already fixed that repository interfaces live in
`internal/domain`, implemented in `internal/persistence/postgres` — this
ADR only fixes what that implementation is built on.

Two real shapes exist in the Go ecosystem: `database/sql` (the standard
library's generic SQL interface) plus a driver registered under it (e.g.
`jackc/pgx`'s `stdlib` compatibility shim), or `pgx` used natively — its own
connection, pooling (`pgxpool`), and query interfaces, bypassing
`database/sql` entirely.

## Decision

Persistence code uses `pgx` (`github.com/jackc/pgx/v5`) natively, via
`pgxpool.Pool` for connection pooling — not `database/sql` with a
registered driver.

`internal/persistence/postgres` is the only package that imports `pgx`
directly (`architecture-backend.md` FR-2's boundary: repository
*implementations* live here; `internal/domain` never imports it).
Repository implementations take a `*pgxpool.Pool` (or a narrower interface
covering just `Query`/`Exec`/`Begin`, sized once real repositories exist),
constructed once at startup (`backend-service-lifecycle.md`) and passed in,
never a package-level global (constitution §17-style prohibition phase 03's
own risk table already names).

No query builder (`squirrel`, `sqlc`-generated code, or similar) — SQL is
hand-written in each repository method. With the current, small set of
domain repositories this phase and phase 06 onward will produce, hand-
written SQL is fewer moving parts than a code-generation step, and keeps
`architecture-persistence.md` FR-6 ("migration files are the only
legitimate way the schema changes") from also needing a second tool kept in
sync with the schema. Revisit if the query surface grows enough that
hand-written SQL becomes the actual maintenance cost — not a decision this
ADR forecloses, just doesn't choose today without a real query count to
weigh it against.

## Options considered

### Option A — `pgx` native, `pgxpool`, hand-written SQL (chosen)

*For* — `pgx` is the de facto standard native Postgres driver for Go
(used directly by, among others, CockroachDB's own Go tooling and a large
share of the Go ecosystem's Postgres-specific libraries); it speaks
Postgres's binary wire protocol directly rather than through
`database/sql`'s generic `driver.Value` abstraction, which matters
concretely for this project's own domain types — `pgx` supports native
scanning of arrays, JSONB, and composite types without the manual
byte-shuffling `database/sql`'s narrower `driver.Value` set would otherwise
require for, e.g., `domain-bibliographic.md`'s subject lists or `domain-
events.md`'s event payloads. `pgxpool` is a connection pool built
specifically for Postgres's actual connection lifecycle (not
`database/sql`'s generic, driver-agnostic pool, which was designed around
assumptions — like every driver looking similar — that don't always fit
Postgres's specifics well), which is exactly what `architecture-
persistence.md` FR-3 requires exist and be enforced.

*Against* — code written against `pgx`'s native interfaces cannot swap to a
different `database/sql`-compatible engine later without a rewrite of every
repository. Named, not treated as a real risk: ADR 0004 already closed the
"which engine" question specifically because this system's LAN-serving,
multi-device design assumes Postgres; there is no plausible future engine
swap this ADR is trading away.

### Option B — `database/sql` + `pgx`'s `stdlib` shim

*For* — keeps repository code portable to any `database/sql`-compatible
driver in principle, and interoperates directly with any tooling that
expects a `database/sql`-shaped `*sql.DB` (some migration tools, some ORMs).

*Against* — pays `database/sql`'s abstraction cost (the generic
`driver.Value` interface, no native array/JSONB scanning, an extra
translation layer between Postgres's actual wire types and Go values) for a
portability benefit this project has no use for — ADR 0004 already fixed
the engine. Using the `stdlib` shim over `pgx` natively would mean carrying
both APIs' concepts (`database/sql`'s `*sql.DB` *and* `pgx`'s own
Postgres-specific types wherever the shim's abstraction leaks) for no
benefit over choosing one directly.

### Option C — An ORM (`ent`, `gorm`)

*Against* — a bigger dependency than this project's endpoint/table count
justifies under constitution §9, and the same "we might grow into needing
it" reasoning ADR 0008 already rejected for build-orchestration tooling.
Repository interfaces (`architecture-backend.md` FR-2) already give this
project the abstraction boundary an ORM would otherwise be reached for to
provide.

## Consequences

**Good** — direct, efficient scanning of Postgres-specific types the
domain model will actually use (arrays, JSONB for anything schema-flexible,
composite types if the schema ever wants them); `pgxpool`'s pool matches
`architecture-persistence.md` FR-3's bounded-pool requirement without extra
wiring; one fewer abstraction layer between repository code and the actual
protocol on the wire, which makes SQL errors easier to map correctly into
`architecture-backend.md` FR-4's fixed error categories (the raw Postgres
error code arrives un-translated by any intermediate driver).

**Bad** — `pgx`-native code is Postgres-specific by construction — accepted
directly above, not a hidden cost. Hand-written SQL (no query builder,
no generated code) means each new repository method is written by hand,
including its own SQL injection discipline (parameterized queries only,
never string-built SQL — a security requirement independent of which
driver library is chosen, but worth restating here since it's this ADR
that puts hand-written SQL on the table at all).

**Bad, and missed in this ADR's first draft** (caught by independent
review, `.claude/reviews/0022-phase03-cross-spec-review.md`): this
project's chosen migration tool (ADR 0013, `goose`) has a public API that
takes a `*sql.DB`, not a `pgx` native connection or `*pgxpool.Pool` — the
exact `database/sql` layer this ADR otherwise avoids. Neither ADR named
this when each was written independently; they simply didn't compose
without a fix. Resolved in `backend-persistence.md` FR-6: the migration
runner opens one short-lived `*sql.DB` via `pgx/v5/stdlib` (the
`database/sql`-compatibility shim `pgx` itself provides), scoped narrowly
to the one `goose.Up` call and closed immediately after, never reused by
any repository. A real, if small, exception to "pgx native, never
`database/sql`" — worth stating plainly here rather than leaving this
ADR's Bad section silent on the one place its own choice doesn't fully
hold.

**Neutral** — doesn't change ADR 0004's engine choice or `architecture-
backend.md` FR-2's repository-interfaces-in-`internal/domain` boundary;
this ADR only fixes what's on the implementation side of that boundary.

## Reversal cost

Medium once phase 03/06 write real repositories against `pgx`'s native
interfaces — every repository method's scanning code would need rewriting
to move to `database/sql`, though the repository *interfaces* themselves
(`internal/domain`) wouldn't change, since they're already engine-agnostic
by `architecture-backend.md` FR-2's design.

## Confidence

High. `pgx` over `database/sql`+driver is close to consensus guidance in
the current Go ecosystem for a Postgres-only project with no portability
requirement, and ADR 0004 already removed the one reason (engine
portability) that would argue the other way.
