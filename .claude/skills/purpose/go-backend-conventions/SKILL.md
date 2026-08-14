---
name: "go-backend-conventions"
description: "Alexandryn's Go backend conventions: package layout, dependency direction, no package-level globals, the error taxonomy, and repository naming. Use whenever writing or reviewing Go code under cmd/server, cmd/pg-supervisor, or internal/ in this repository."
---

Assumes `architecture-backend.md`, `backend-service-lifecycle.md`,
`backend-errors-and-logging.md`, and `backend-persistence.md` (all
`APPROVED`) — this skill restates their decisions as a checklist, it
doesn't re-derive them. Read those specs for the *why*; this is the *what
to do* while writing code against them.

## Package layout (`architecture-backend.md` FR-1)

```
cmd/server          — the Go service entry point
cmd/pg-supervisor    — macOS-only Postgres supervisor (backend-persistence.md FR-8)
internal/domain      — aggregates, value types, repository interfaces
internal/transport/http — router, middleware, handlers, wire-shape helpers
internal/persistence/postgres — repository implementations, pool, migrations
internal/config      — config.Load, Config struct
internal/logging     — logger construction, redaction types
internal/testutil     — fixture factories, FakeClock, fake FS/IDGenerator (test-only, never imported by production code)
```

One file per repository (`work_repository.go`, not `repository.go`) —
`backend-persistence.md` FR-2's convention, not a hard rule with a test,
but don't invent a different shape under deadline pressure.

## Dependency direction — never violate this

`internal/domain` MUST NOT import `internal/persistence/postgres`,
`internal/transport/http`, or `pgx`. Repository *interfaces* live in
`internal/domain`; repository *implementations* live in
`internal/persistence/postgres` and satisfy those interfaces. A domain
method signature MUST NOT mention `pgx.Tx`, `*sql.DB`, or any
persistence-specific type (`backend-persistence.md` FR-4).

This is enforced by an import-boundary lint (`architecture-backend.md`
FR-3) — if you're fighting the linter to make a domain type reach into
persistence, the fix is to move logic to the persistence side, not
suppress the lint.

## No package-level globals — ever

Config, logger, database pool, clock, filesystem, ID generator: all
constructed once in `main`/`run`, passed as constructor arguments or
struct fields (`backend-service-lifecycle.md` FR-2). No `var logger
*slog.Logger` at package scope, anywhere, in any package. This is a
structural check (a lint or grep-based CI step per this phase's test
plans), not a matter of taste — a global here is a defect, full stop.

## Errors — the six categories, nowhere else

`domain.Error` carries exactly one of: `NotFound`, `InvalidInput`,
`Unauthorized`, `Conflict`, `Unavailable`, `Internal`
(`backend-errors-and-logging.md` FR-1). Construct it at the point an
operation fails — never let a bare `error` or a raw `pgx` error cross
from `internal/persistence/postgres` into `internal/domain`-typed
territory un-translated (FR-3). An error that doesn't fit one of the six
is `Internal` by default, never an invented seventh category.

## Logging — inject the logger, redact by type

`log/slog`, JSON handler, one logger constructed at startup
(`backend-errors-and-logging.md` FR-6). A type carrying a value that must
never be logged (a connection string, a future credential) implements
**both** `slog.LogValuer` and `json.Marshaler` — one alone leaves a gap
when the containing struct is logged as a single attribute (FR-8). Never
pass a driver or parser error's raw `.Error()` string into a log line or
response without checking whether it could embed a connection string
first — this bit the project once already (`.claude/audits/0001`); when
in doubt, use a fixed generic message and log the specific *kind* of
failure, not the underlying library's own text.

## Config — one call, through `internal/config` only

Nothing outside `internal/config` reads an environment variable or the
config file directly (`backend-configuration.md` FR-1). `config.Load`
returns a fully validated `Config` or an error — never a partial struct.

## Testing hooks

Every place code reads "now," touches the filesystem, or needs
randomness goes through the injected `Clock`/`FS`/`IDGenerator`
interfaces (`backend-test-harness.md` FR-5/FR-6), never `time.Now()`,
`os.*`, or a UUID library called directly inside logic under test.

## Before opening a PR

Run `/review` (this repo's own batched-review skill) against the diff,
then this project's `code-review` skill for line-level findings, before
`/make-pr`.
