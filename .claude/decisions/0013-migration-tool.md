# 0013. Schema migrations use `goose`, embedded via `go:embed`

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-14 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

`architecture-persistence.md` FR-4 through FR-6 already fixed the
*policy*: forward-only, applied automatically at startup before readiness
(`architecture-system.md` FR-7), the only legitimate path a schema
changes, and a failed partial migration must leave a detectable, blocking
state rather than silently continuing. Phase 01's own "architecture
decisions expected" list left the *tool* to phase 03. ADR 0008 already
committed this project to `go:embed` for the frontend build; the same
mechanism is available for migration files, which matters for this
decision specifically — a shipped binary (ADR 0008: one artifact, no
separate asset-copying step) needs its migrations to travel inside it, not
as loose files next to the binary that an install step could separate from
it.

## Decision

Migrations use `github.com/pressly/goose/v3`, with SQL migration files
embedded into the `cmd/server` binary via `go:embed` (the same mechanism
ADR 0008 already applies to `web/dist/`), applied programmatically at
startup — not `goose`'s own CLI invoked as a build or release step.

Concretely: `internal/persistence/postgres/migrations/*.sql`, embedded
through an `embed.FS`, run via `goose.Up(db, "migrations")` (or the
equivalent embedded-source call) from inside `backend-service-lifecycle.md`'s
startup sequence, before the readiness check `architecture-system.md` FR-7
requires. Migration files are plain SQL (`-- +goose Up` / `-- +goose Down`
directive comments), not Go-code migrations — SQL is what's actually being
changed, and plain SQL keeps a migration reviewable without reading Go.

`goose Down` exists in the tool but is not part of this project's supported
path — `architecture-persistence.md` FR-4's forward-only policy means a
schema mistake gets fixed by a new forward migration, never a rollback
applied against a database that may already hold real user data. Down
scripts may still be written (`goose` requires the directive to exist for
some commands) but are never invoked outside a developer's own local,
disposable database.

## Options considered

### Option A — `goose`, embedded, applied at startup (chosen)

*For* — plain-SQL migrations, first-class `embed.FS` support (added
specifically for exactly this "ship migrations inside the binary" use
case), a small dependency surface (one library, no separate CLI required
at runtime), and a programmatic `Up`-only call that fits directly into
`backend-service-lifecycle.md`'s startup sequence without shelling out to
an external binary the way some migration tools require.

*Against* — smaller ecosystem/mindshare than `golang-migrate`; fewer
third-party integrations. Not a real cost here — this project needs one
thing (apply forward migrations from an embedded source at startup) and
`goose` does that directly.

### Option B — `golang-migrate/migrate`

*For* — the most widely used Go migration tool, supports many source
drivers including embedded filesystems, large community.

*Against* — its embedded-source support exists but is less central to how
the project is typically used (it's most commonly driven by its own CLI
against a file-path or URL source); using it purely programmatically
against an `embed.FS` is a supported but secondary path. `goose`'s
programmatic embedded-source usage is more directly the tool's own primary
documented use case for exactly this scenario. Marginal difference, not a
disqualifying one — recorded because it's the real alternative, not to
overstate the gap.

### Option C — Go-code migrations (no SQL files; e.g., write raw
`ALTER TABLE` calls directly in Go, versioned by hand)

*Against* — reinvents version tracking, ordering, and the applied-migrations
table that both `goose` and `golang-migrate` already provide correctly;
loses the plain-SQL reviewability a schema change should have (a
`.sql` file is the natural artifact to read in a PR diff, not a Go function
that happens to execute DDL).

### Option D — `sqlc`'s bundled migration support / an ORM's built-in migrator

*Against* — would mean adopting a query-generation tool (`sqlc`) or an ORM
(rejected already, ADR 0012 Option C) for its migration feature alone;
disproportionate to the actual need, same §9 reasoning ADR 0012 already
applied.

## Consequences

**Good** — migrations ship as part of the one binary ADR 0008 already
commits to (no separate migrations directory to lose track of on a user's
machine); plain SQL keeps `architecture-persistence.md` FR-6's "one
reviewable path" concrete and readable; `goose`'s applied-migrations
tracking table gives FR-5's "detect and refuse to proceed past a partial
migration" a real mechanism to build on rather than inventing one.

**Bad** — a new dependency to justify under §9 (justified above: no
stdlib equivalent exists, and hand-rolling version tracking would
reproduce what `goose` already does correctly) and to track through
`architecture-testing.md` FR-5's dependency-vulnerability scanning like
any other dependency.

**Bad, and missed in this ADR's first draft** (caught by independent
review, `.claude/reviews/0022-phase03-cross-spec-review.md`): `goose`'s
public API takes a `*sql.DB`, not the `pgx`-native connection type ADR
0012 chose for everything else in this codebase — this ADR's first draft
didn't check that against ADR 0012's own explicit rejection of
`database/sql`, and the two didn't compose without a fix. Resolved in
`backend-persistence.md` FR-6 (a narrowly-scoped `pgx/v5/stdlib`
`*sql.DB`, opened only for the `goose.Up` call, closed immediately after)
and named in ADR 0012's own Bad section too, since the cost belongs to
both decisions equally.

**Neutral** — doesn't change `architecture-persistence.md`'s already-
decided policy (FR-4 through FR-6); this ADR only fixes the tool that
implements it.

## Reversal cost

Medium. `goose`'s migration file format (plain SQL with directive
comments) is close enough to `golang-migrate`'s that switching tools later
would mostly mean rewriting the applied-migrations bookkeeping, not the
SQL itself. Expensive only in the sense that any already-applied
migration history on a real user's data directory would need to be
reconciled with whatever tracking table the new tool expects.

## Confidence

High on `goose` over hand-rolling migration tracking; medium on `goose`
specifically over `golang-migrate` — both are credible, well-maintained
choices for this exact use case, and the deciding factor (embedded-source
usage being more central to `goose`'s own primary documented path) is a
real but narrow distinction, not a wide gap.
