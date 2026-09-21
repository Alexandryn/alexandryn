# Spec: Backend persistence implementation

| | |
|---|---|
| **Status** | `APPROVED` (amended post-approval three times — DSN redaction in migration failure logging (`0028`, still needs maintainer re-confirmation), container-target Postgres connection (`0043`, still needs maintainer re-confirmation), and FR-2/FR-4 for the transaction contract and outbox ([ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md), 2026-08-19 — confirmed 2026-08-21, alongside domain-source.md's matching FR-6 amendment)) |
| **Phase** | `03-backend-foundation` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-16 |
| **Supersedes** | — |
| **Reviewed in** | `0022` (two independent agents, cross-spec) — Needs rework at review time, fixed; approved by maintainer 2026-08-14. Amended post-approval, `0028` — DSN redaction gap found by security review, self-reviewed, needs maintainer re-confirmation. Amended again, `0043` — FR-5 and Security considerations made target-dependent for ADR 0015, self-reviewed, needs maintainer re-confirmation |

## Context

`architecture-persistence.md` fixed the policy: data directory location,
loopback binding, bounded pool, forward-only migrations run at startup
before readiness, migrations as the only schema-change path, no automatic
recovery from corruption, and per-platform orphan-prevention for the
spawned Postgres process. ADR 0012 fixed the driver (`pgx` native,
`pgxpool`). ADR 0013 fixed the migration tool (`goose`, embedded via
`go:embed`). This spec is where those become actual Go code: the pool
construction, the migration runner invocation, the repository pattern
phase 02's domain interfaces get implemented against, and transaction
boundaries.

## Problem

Nothing has fixed: the connection pool's actual size bounds
(`architecture-persistence.md` FR-3 required a bound exist, didn't size
it), how `internal/persistence/postgres` structures repository
implementations against phase 02's domain interfaces
(`architecture-backend.md` FR-2), how a multi-step domain operation gets a
transaction boundary, or how the spawn/orphan-prevention mechanisms
(`architecture-persistence.md` FR-8/FR-9/FR-10) actually get invoked from
`cmd/server`.

## Goals

- Size the connection pool with a real number and justification
- Fix the repository implementation pattern: one Go type per phase 02
  aggregate root, implementing that aggregate's domain-defined interface
- Fix transaction boundaries: what unit of work gets one transaction, and
  how a repository method exposes that without leaking `pgx`-specific
  types into `internal/domain`
- Fix the concrete invocation of `architecture-persistence.md`'s spawn
  sequence (FR-1, FR-8/FR-9/FR-10) from `cmd/server`, in production
- Fix the migration runner's concrete invocation (ADR 0013) inside
  `backend-service-lifecycle.md`'s startup sequence

## Non-goals

- The domain interfaces themselves — already fixed by phase 02's specs;
  this spec implements them, doesn't redesign them
- Individual migration files' actual schema (the first real tables) —
  arrives with phase 06's first feature, built on this spec's runner and
  pattern, not invented here
- Query builder / codegen tooling — ADR 0012 already rejected this

An earlier draft of this section deferred the macOS supervisor's actual
code (`architecture-persistence.md` FR-10) to phase 05, reasoning it was
a desktop-host concern. That was wrong: `architecture-persistence.md`
FR-10 itself says the Go server spawns the supervisor directly (the
supervisor then spawns Postgres, becoming its real parent), and
`docs/roadmap/03-backend-foundation/README.md`'s own Scope/In line
explicitly lists FR-10 alongside FR-8/FR-9 as in this phase's scope —
review 0022 caught the mismatch. FR-8 below now covers it; it is not a
Non-goal.

## User stories

- As **`backend-service-lifecycle.md`'s startup sequence**, I want one
  function to obtain a ready-to-use pool and run pending migrations, so
  steps 5–6 of the startup sequence are a single call each.
- As **phase 06's first repository**, I want a fixed pattern (a
  Go type wrapping `*pgxpool.Pool`, implementing a phase 02 interface, one
  file per aggregate) already established, so writing it is filling in a
  template, not inventing conventions.
- As **a contributor debugging a slow query**, I want the pool's actual
  size bound and timeout behavior written down, so "is the pool exhausted"
  is a checkable hypothesis, not a guess.

## Functional requirements

- **FR-1** `internal/persistence/postgres` constructs one
  `*pgxpool.Pool` per process (`backend-service-lifecycle.md` FR-2: no
  package-level global — the pool is constructed in `main`/`run` and
  passed to repository constructors), sized: `MinConns: 2`, `MaxConns: 10`
  (`backend-configuration.md`'s `DB_POOL_MAX_CONNS`, default 10). `MaxConns`
  is a placeholder-but-reasoned number for a single-process, single-database,
  local-loopback-latency service with no measured load yet, generous
  enough to cover concurrent LAN requests once phase 13 exists without
  being large enough to overwhelm a bundled Postgres instance sized for
  one household's hardware. `MinConns: 2` has no comparable capacity
  reasoning behind it — unlike `MaxConns`, which is a real ceiling this
  spec argues for, `MinConns` only controls how many connections are kept
  warm at idle, a minor startup-latency optimization, not a correctness or
  capacity property; 2 is a round-number floor (enough that two
  near-simultaneous requests at startup don't both pay a fresh-connection
  cost) rather than a derived value, and is not exposed as a separate
  config key for that reason — it doesn't need per-deployment tuning the
  way a hard capacity ceiling does. `MaxConnLifetime` and `MaxConnIdleTime`
  are left at `pgxpool`'s own defaults — no requirement here argues for
  overriding them.
- **FR-2** Each phase 02 domain aggregate that needs persistence gets
  exactly one repository implementation type in
  `internal/persistence/postgres`, implementing the corresponding
  interface `internal/domain` defines (`architecture-backend.md` FR-2).
  The complete list, by owning spec: `domain-bibliographic.md` — `Work`,
  `Edition`, `Author`; `domain-library.md` — `LibraryEntry`, `Collection`;
  `domain-source.md` — `Source`, `SourceOffering`; `domain-reading.md` —
  `ReadingProgress`, `Bookmark`, `Highlight`, `ReadingPreferences`. Value
  types embedded within these aggregates (`Subject`, `Language`,
  `FileReference`) and the ephemeral, never-persisted `ProgressReport`
  (`domain-reading.md` FR-2) do not get their own repository — they are
  constructed and validated as part of the aggregate that owns them, not
  queried independently. **Amended 2026-08-19, [ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md):**
  the transactional outbox that ADR carries is a twelfth persisted table and
  is deliberately absent from the eleven above — no phase 02 domain aggregate
  corresponds to it, and it is a delivery mechanism rather than a domain
  concept, so it gets a store of its own without a domain type.
  One file per aggregate (`work_repository.go`,
  not one monolithic `repository.go`) — this is a naming/organization
  convention, not a functional requirement with a test, but stated here
  so phase 06 doesn't need to invent it under deadline pressure, matching
  this spec's own stated purpose.
- **FR-3** A repository method's SQL MUST use parameterized queries
  exclusively (`pgx`'s own query-argument mechanism) — string-built SQL
  incorporating any value that ultimately traces back to a source, a
  filename, or any other constitution §4 "hostile input" MUST NOT exist
  anywhere in `internal/persistence/postgres`. This is standard SQL-
  injection discipline, restated as a functional requirement because ADR
  0012's "hand-written SQL, no query builder" choice makes it a
  requirement this project's own tooling doesn't enforce automatically —
  a query builder would have made this structurally hard to violate; hand-
  written SQL makes it a discipline this spec states explicitly instead.
- **FR-4** A domain operation that must apply as a single atomic unit
  (e.g. `domain-bibliographic.md`'s reversible Work merge, which touches
  multiple rows) MUST run inside one `pgx` transaction
  (`pool.Begin(ctx)` / `tx.Commit(ctx)` / `tx.Rollback(ctx)`), and the
  transaction boundary MUST be decided by the repository method
  implementing that specific domain operation — never left implicit
  across multiple separate pool calls that could interleave with another
  request's writes. `internal/domain`'s interfaces MUST NOT expose a
  `pgx.Tx` type or any persistence-specific transaction handle in their
  method signatures (`architecture-backend.md` FR-2's boundary: the
  domain doesn't know its own persistence mechanism) — a multi-step
  domain operation is exposed as one repository method that internally
  manages its own transaction, not as several methods the caller must
  sequence correctly. **Amended 2026-08-19, [ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md):**
  the preceding sentence holds for an operation confined to one aggregate,
  and is impossible for one that spans two — `domain-source.md` FR-6's
  cascade writes `Source` and `SourceOffering`, which FR-2 below places in
  separate repositories. For the spanning case, composition goes through the
  `Transactor` interface ADR 0021 declares in `internal/domain`, whose handle
  travels in `context.Context` so no persistence-specific type enters a
  domain signature and this FR's prohibition above is preserved exactly. The
  composing caller is a domain service ([ADR 0020](../decisions/0020-graph-invariants-in-domain-services.md))
  holding the repository interfaces and the `Transactor` — not a transport
  handler, and not an unsequenced set of calls, which this FR was right to
  forbid.
- **FR-5** Which of two paths `cmd/server` takes at this step depends on
  `DATABASE_URL`'s presence, and what that presence *means* is
  target-dependent (amended 2026-08-16, ADR 0015; this FR previously
  described the presence branch as a dev/CI/test-only path, before a
  second target existed). **In the Electron-hosted target's production
  use** (spawned by Electron via `architecture-system.md`'s process
  model, `DATABASE_URL` absent per `backend-configuration.md` FR-4's
  third category), `cmd/server` MUST perform `architecture-persistence.md`
  FR-1 (data directory init if absent) and FR-8/FR-9 (platform-specific
  spawn with orphan-prevention on Linux/Windows) or FR-10 (on macOS, via
  FR-8 below) before attempting to connect — implemented as a small
  `internal/persistence/postgres/supervisor` (naming placeholder) package
  invoked from `backend-service-lifecycle.md` FR-1 step 5, on Linux and
  Windows directly (`os/exec` with the platform `SysProcAttr`,
  `architecture-persistence.md` FR-8/FR-9), and via a call to the
  separate `cmd/pg-supervisor` binary on macOS (FR-8 below,
  `architecture-backend.md` FR-1's package layout). **When a
  `DATABASE_URL` value is present**, this spawn step MUST be skipped
  entirely — the process connects directly to whatever `DATABASE_URL`
  points at, treating it as already running, unmanaged by this process.
  This is the Electron target's developer/CI/test override (`go run`,
  tests, CI, per `backend-configuration.md` FR-4's table) **and** the
  container-hosted target's normal production path (ADR 0015) — the code
  path is identical either way; only which target is running determines
  whether taking it is the exception or the rule. The container-hosted
  target has no spawn-and-own alternative to fall back to at all — it is
  not that its spawn step is skipped, it is that no such step exists for
  that target to take.
- **FR-6** Migrations (ADR 0013) run via `goose.Up`, invoked immediately
  after a successful connection (FR-5, or the direct `DATABASE_URL` path)
  and before `backend-service-lifecycle.md` FR-1 step 6's repository
  construction begins, against the embedded
  `internal/persistence/postgres/migrations` filesystem. `goose`'s public
  API takes a `*sql.DB`, not a `pgx` native connection or `*pgxpool.Pool`
  — a real integration point ADR 0012 (which rejected `database/sql` for
  the application's own query code) and ADR 0013 (which chose `goose`)
  didn't reconcile between themselves when each was written. Resolved
  here: the migration runner opens one short-lived `*sql.DB` via
  `pgx/v5/stdlib` (the `database/sql`-compatibility shim `pgx` itself
  provides), scoped narrowly to this one call, closed immediately after
  `goose.Up` returns — never passed to, or reused by, any repository or
  any other code in `internal/persistence/postgres`. This is the one
  deliberate, narrow use of `database/sql` in an otherwise
  `pgx`-native codebase, and it exists because `goose`'s API requires it,
  not because ADR 0012's reasoning against `database/sql` for
  application queries has changed. A migration failure MUST return an
  error that `backend-service-lifecycle.md` FR-3 treats as a startup
  failure (`architecture-persistence.md` FR-5: does not proceed to
  `Ready`), carrying enough detail (which migration, `goose`'s own error)
  for `backend-errors-and-logging.md` FR-1's `Internal` category and a
  specific log line, without exposing raw SQL to any client-facing
  surface (nothing client-facing exists at this point in startup anyway).
  When `goose`'s own error wraps a connection failure from opening the
  narrowly-scoped `*sql.DB` above (rather than a SQL-execution failure
  against an already-open connection), the logged detail MUST NOT include
  that underlying connection error's `Error()` string verbatim — it can
  embed the DSN, the same failure mode `backend-http-transport.md` FR-5
  and `backend-service-lifecycle.md` FR-3 already redact for their own
  call sites; this spec's migration-failure logging uses the same fixed,
  generic phrasing for the connection-failure case specifically, while
  still naming the failing migration file and `goose`'s own error text
  for a genuine SQL/schema failure, which carries no connection-string
  risk (security review finding, 2026-08-14).
- **FR-7** The next startup after a failed partial migration
  (`architecture-persistence.md` FR-5's "next startup attempt can detect
  and refuse to proceed past") MUST also fail at the same FR-6 step,
  since `goose`'s own applied-migrations tracking table will show the
  failed migration as not fully applied — this spec doesn't need
  additional bespoke detection logic beyond calling `goose.Up` again and
  trusting its own bookkeeping, which is exactly the mechanism ADR 0013
  cited as the reason to use a real migration tool rather than hand-
  rolled version tracking.
- **FR-8** (Lower confidence — same caveat `architecture-persistence.md`
  FR-10 itself carries: a novel supervisor-process pattern, not a
  well-known stdlib feature, unverified in this environment with no
  macOS available) On macOS, `cmd/server`'s FR-5 spawn step invokes
  `cmd/pg-supervisor` (`architecture-backend.md` FR-1's package layout) —
  a separate binary, built from the same Go module, that becomes
  PostgreSQL's actual direct parent. `cmd/pg-supervisor` MUST: (1) accept
  the same PostgreSQL data-directory path and startup arguments
  `cmd/server` would otherwise pass directly to `postgres` on
  Linux/Windows; (2) spawn `postgres` itself as its own child, once
  invoked; (3) monitor the Go server's PID via `kqueue`
  (`EVFILT_PROC`/`NOTE_EXIT`, `architecture-persistence.md` FR-10); (4) on
  detecting the Go server's death, kill the `postgres` child and then
  exit itself — it MUST NOT continue running after its target process is
  gone, since that would make it exactly the kind of orphan it exists to
  prevent. `cmd/server` on macOS therefore does not connect to Postgres
  directly at spawn time the way FR-5 describes for Linux/Windows — it
  spawns `cmd/pg-supervisor` and waits for the same "Postgres accepting
  connections" signal FR-5's other platforms wait for, since
  `cmd/pg-supervisor` is a thin process-lifecycle wrapper, not a proxy in
  the connection path itself (the Go server still connects to Postgres's
  own listening port directly, once it's up). The spawn-argument
  construction logic (data directory path, `postgres` binary location) MAY
  be shared between `cmd/server`'s Linux/Windows path and
  `cmd/pg-supervisor`'s macOS path via a common internal package, since
  both need to produce the same arguments — not required, but the
  natural implementation choice given `architecture-backend.md`'s own
  Open questions already raised this possibility.

## Non-functional requirements

- **Performance** — FR-1's pool bounds are this spec's concrete answer to
  `architecture-persistence.md` FR-3's "a real number from phase 03" —
  placeholder-but-reasoned, flagged in Open questions for confirmation
  once real load exists.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; no UI.
- **Reliability** — FR-4's explicit transaction-boundary-per-operation
  requirement is what keeps a reversible merge (`domain-bibliographic.md`
  FR-4/FR-7) actually atomic at the storage layer, not just atomic in
  intent.
- **Observability** — migration application (FR-6) MUST log success/
  failure at the levels `backend-errors-and-logging.md` FR-9 defines
  (`info` for success, `error` for failure), distinguishable from a
  routine connection failure per `architecture-persistence.md`'s own
  Observability requirement.

## Domain model

Not applicable directly — this spec implements phase 02's already-fixed
domain interfaces; it introduces no new domain concepts. It does fix,
concretely, where the metadata/source/library boundary (constitution §3)
becomes physical: one repository type per aggregate, never one repository
spanning multiple domains' aggregates.

## API and contracts

- **`internal/domain` ↔ `internal/persistence/postgres`**: Go interfaces
  (phase 02, `architecture-backend.md` FR-2) implemented by FR-2's
  repository types — the only contract this spec fixes the implementation
  side of.
- **`internal/persistence/postgres` ↔ PostgreSQL**: `pgx` native (ADR
  0012), parameterized queries only (FR-3), over the pool FR-1
  constructs.
- **`internal/persistence/postgres` ↔ the bundled Postgres process**: the
  spawn/orphan-prevention mechanism FR-5 invokes, per
  `architecture-persistence.md` FR-8/FR-9 (Linux/Windows) or FR-8 above
  via `cmd/pg-supervisor` (macOS, `architecture-persistence.md` FR-10).
- **`internal/persistence/postgres` ↔ migration files**: `goose`,
  `embed.FS` (ADR 0013), invoked via FR-6, against a `pgx/v5/stdlib`
  `*sql.DB` scoped narrowly to the migration call.

## State transitions

Extends `backend-service-lifecycle.md`'s FR-1 step 5 with this spec's own
detail:

```
FR-1.step5 -> (production: FR-5 init data dir + spawn Postgres per
               platform, FR-8 on macOS)
           -> (dev/test: FR-5 skip spawn, connect to DATABASE_URL directly)
           -> (either path: wait for a successful connection)
           -> (FR-6: goose.Up, via a narrowly-scoped pgx/stdlib *sql.DB,
               against embedded migrations)
           -> success: proceed to step 6
           -> failure (connection or migration): Failed, per
              backend-service-lifecycle.md FR-3
```

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Pool exhausted (all `MaxConns` in use) | A request blocks waiting for a connection, or `pgx`'s own context-deadline error if the request's context times out first | A slow response, or (if the request's own timeout — `backend-http-transport.md` FR-2 — fires first) an `Unavailable` error | `pgxpool` queues the waiter per its own default behavior; no custom queueing logic added by this spec |
| A repository method's transaction fails partway (FR-4) | `tx.Commit(ctx)` returns an error | The domain operation as a whole fails with the appropriate category (`backend-errors-and-logging.md`), no partial write visible to any other reader | `tx.Rollback(ctx)` called (deferred, standard Go `pgx` pattern), nothing partially committed |
| Migration fails partway (FR-6) | `goose.Up`'s own error | Startup failure (`backend-service-lifecycle.md` FR-3) | Does not proceed to `Ready`; `architecture-persistence.md` FR-5's prohibition on silent continuation holds |
| Production spawn step fails (`architecture-persistence.md` FR-7: corrupted data directory, disk full) | FR-5's spawn call returns an error | Startup `Failed`, naming what happened, not raw Postgres output | `architecture-persistence.md` FR-7 already prohibits automatic data-directory modification; this spec's FR-5 surfaces the failure, doesn't attempt recovery |
| macOS: `cmd/pg-supervisor` dies without the Go server having exited | Not directly detectable by `cmd/server` — the supervisor's own FR-8 obligation is the only thing that would have caught the Go server's death; a dead supervisor with a live Go server has no dedicated detection here | Postgres orphaned if this happens (the failure mode FR-8 exists to prevent, occurring in the mechanism meant to prevent it) | Named as a residual risk, not solved — the same honesty `architecture-persistence.md` FR-10 already carries for this whole platform's mechanism |

## Security considerations

- **Parameterized queries only (FR-3)** is this spec's primary security
  requirement — SQL injection defense, made explicit because ADR 0012's
  hand-written-SQL choice removes the structural protection a query
  builder would have given for free.
- **Transaction boundaries never leak persistence types into the domain
  (FR-4)** — keeps `internal/domain` from ever needing to know about
  `pgx.Tx`, which is also what keeps a future alternative persistence
  implementation (however unlikely, given ADR 0012's Postgres-specific
  choice) from being blocked by a domain-level API that assumes one
  driver's transaction type.
- **`DATABASE_URL` bypass (FR-5) is target-dependent, not universally
  dev-only (amended 2026-08-16, ADR 0015)** — in the Electron-hosted
  target, the spawn-skip path exists for developer/test convenience only
  and MUST NOT be reachable when `cmd/server` is running as Electron's
  spawned child (there is no Electron-supplied `DATABASE_URL` in that
  path at all, per `backend-configuration.md` FR-4's table, so there's
  nothing to accidentally trust). In the container-hosted target, the
  same path is the *only* path — there is no spawn-and-own alternative to
  fall back to, and reaching it is correct, not a bypass. Both statements
  hold simultaneously because the two targets are mutually exclusive at
  runtime (a given `cmd/server` process is started by exactly one of
  them): the distinguishing signal is still which environment variable
  exists, never a runtime flag someone could get wrong, exactly as
  before — what changed is only that a second, legitimate reason for that
  variable to exist now exists alongside the original one.
- **The migration runner's narrow `database/sql` exception (FR-6) is
  scoped tightly on purpose** — the `pgx/v5/stdlib`-backed `*sql.DB` lives
  only inside the function that calls `goose.Up`, opened and closed
  around that one call, never stored, never passed to a repository, never
  reachable from `internal/domain`. This keeps ADR 0012's reasoning
  (avoid `database/sql`'s abstraction cost for application queries) intact
  everywhere except the one place an external tool's API leaves no
  alternative.
- **The macOS supervisor (FR-8) is new attack surface, restated from
  `architecture-persistence.md`'s own Security considerations** — it's a
  small process Alexandryn writes and spawns itself, so the same
  "our own binary, never a path influenced by external input" requirement
  applies to it as to every other spawned process in this system.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Pool construction with FR-1's bounds (config-driven), transaction rollback-on-error behavior (FR-4) against a mock/fake |
| Integration | Every repository method against a real (service-container) PostgreSQL (`architecture-testing.md`'s "Integration (backend)" layer); a deliberately failing transaction proven to leave no partial write; a migration applied to a database seeded with existing rows (via `backend-test-harness.md` FR-4's fixtures), proving the rows survive and the new schema applies cleanly — phase 03's own named exit criterion ("migrations apply to... a populated one"), not previously covered by any walkthrough in this batch |
| Contract | N/A directly — repository behavior isn't part of the wire contract |
| E2E | The dedicated bundled-spawn suite `architecture-testing.md` FR-3 already separates from routine integration tests — this spec's FR-5/FR-6/FR-8 is exactly what that suite exercises: full init → spawn (including the macOS supervisor path) → migrate → ready, against the real mechanism, not a service container |

## Acceptance criteria

- [ ] Every phase 02 aggregate needing persistence has a repository
      implementation satisfying its domain interface, proven by the
      interface's own compile-time check (a Go interface satisfaction, no
      runtime test needed for this part)
- [ ] A migration applied to a database with existing rows (not just an
      empty one) succeeds and leaves those rows intact, proven with a
      test — phase 03's own named exit criterion
- [ ] A test proves a multi-step operation's transaction rolls back
      completely on a mid-operation failure (FR-4)
- [ ] A test proves no repository method builds SQL by string
      concatenation with any external input (FR-3) — a grep-based or
      static-analysis check may suffice, given the requirement is about
      absence of a pattern
- [ ] The dedicated bundled-spawn E2E suite (`architecture-testing.md`
      FR-3) exercises FR-5/FR-6/FR-8's full production path end to end,
      including the macOS supervisor
- [ ] Every FR maps to a line in phase 03's own exit criteria

## Open questions

- **Pool size (FR-1: `MinConns: 2`, `MaxConns: 10`)** — `MaxConns` is a
  reasoned placeholder, not measured; phase 06 onward, once real
  concurrent load exists (especially once phase 13 allows LAN clients),
  should confirm or replace. `MinConns` is a round-number floor with no
  comparable derivation (FR-1) and isn't expected to need the same
  revisiting.
- **Whether spawn-argument-construction logic is actually shared between
  `cmd/server`'s Linux/Windows path and `cmd/pg-supervisor`'s macOS path
  (FR-8)** — leaning toward sharing it via a common internal package,
  named as the natural choice in FR-8, but not fixed as a requirement;
  an implementation detail phase 03/05 can settle either way without
  contradicting anything decided here.
- **Should `/readyz` (`backend-http-transport.md`'s Open questions)
  check migration completeness, not just connectivity?** Raised there,
  answerable here: `goose`'s own tracking table makes this a cheap
  additional check (query whether any migration is marked started-but-
  not-completed) if `backend-http-transport.md` decides to add it — not
  decided in either spec yet.

## References

- `architecture-persistence.md` — FR-1 through FR-10, the policy this
  spec implements
- ADR 0012 — `pgx`/`pgxpool`, this spec's driver and pool mechanism
- ADR 0013 — `goose`, this spec's migration mechanism
- `architecture-backend.md` FR-2 — repository interfaces live in
  `internal/domain`; FR-1 — `cmd/pg-supervisor`'s place in the module
  layout
- `backend-service-lifecycle.md` FR-1 steps 5–6, FR-2 — where this spec's
  mechanics sit in the startup sequence and the no-globals rule FR-1 here
  follows
- `backend-configuration.md` FR-4 — `DATABASE_URL`,
  `DB_POOL_MAX_CONNS` keys this spec consumes
- `backend-errors-and-logging.md` FR-3 — Postgres-error-to-category
  translation, the boundary this spec's repository methods sit on one
  side of
- `architecture-testing.md` FR-3 — the dedicated bundled-spawn test suite
  distinct from routine integration tests
- ADR 0015 — the container-hosted target FR-5 and Security considerations
  were amended 2026-08-16 to cover, alongside the Electron-hosted target
- `domain-bibliographic.md` FR-4/FR-7 — the reversible-merge operation
  cited as FR-4's motivating transactional example
- Constitution §3 (domain boundaries), §4 (hostile input — FR-3's SQL
  injection defense)
