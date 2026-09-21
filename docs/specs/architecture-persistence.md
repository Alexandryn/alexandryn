# Spec: Persistence architecture

| | |
|---|---|
| **Status** | `APPROVED` (maintainer confirmed 2026-08-20; backup/restore ownership and encryption-at-rest remain unresolved, flagged for separate attention, not blockers on this spec) |
| **Phase** | `01-architecture` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | `.claude/reviews/0009-spec-architecture-persistence.md` — Approved with changes, all findings fixed; self-reviewed, independent read still pending |

## Context

ADR 0004 decided the engine (self-hosted PostgreSQL) and that Supabase is
dev/test tooling only. Drafting this spec forced the question ADR 0004 left
open: how does a packaged, shipped instance actually get a running
PostgreSQL, given the design reference has no screen for configuring one?
That's now ADR 0007 — bundled, spawned and owned by the Go server, invisible
to the user — with a direct consequence: PostgreSQL becomes a third
long-running process, which required amending `architecture-system.md`'s
FR-1 (originally "exactly two processes"), now `REVIEWED` pending
re-confirmation rather than left falsely `APPROVED`.

This spec covers what `architecture-system.md`'s amendment didn't: the
actual spawn/init mechanism, connection lifecycle, migration policy, schema
ownership, corruption handling, and orphan-prevention for a process whose
source code Alexandryn does not control (unlike the Go server, which is
Alexandryn's own binary).

## Problem

Nothing has decided: where the bundled Postgres's data lives on disk, how
it's initialized on first run, how the Go server connects to and pools
against it, what "the schema" means as a single source of truth, what
happens when the data directory won't start, and — the part
`architecture-system.md`'s FR-12 explicitly deferred here — how a
third-party binary Alexandryn doesn't control the source of gets the same
orphan-prevention guarantee ADR 0005 gave the Go server.

## Goals

- Define the bundled Postgres's data directory location, initialization,
  and binding (loopback only, constitution §6)
- Define connection pooling bounds — no unbounded per-request connections
- State the migration policy (forward-only) as this project's actual rule,
  not just a risk-table aspiration, and name what "a failed partial
  migration" must never do
- Define schema ownership: migrations are the only legitimate way the
  schema changes, ever
- Define corruption handling: what the Go server does when Postgres won't
  start, with an explicit prohibition on the failure mode that would
  actually lose a user's library
- Resolve `architecture-system.md` FR-12: orphan-prevention for Postgres,
  a binary Alexandryn doesn't control the source of — unlike ADR 0005's
  Linux fix, which modified the Go server's own source

## Non-goals

- The specific migration tool/library (goose, golang-migrate, sqlc, etc.) —
  phase 03's `backend-persistence.md` picks it; this spec fixes the policy
  it must implement, not the tool
- Recurring scheduled backups, retention policy, restore UX — a real
  feature, not an architecture decision this spec is positioned to make
  well. Flagged as an open question with no owner yet, not invented here
- Repository interfaces, domain-type mapping, transaction boundaries in Go
  code — `backend-persistence.md` (phase 03)
- An external/user-configured Postgres option — ADR 0007 already deferred
  this; not reopened here
- Postgres major-version upgrade story across Alexandryn app updates — real
  and undesigned, named in Open questions, not solved here

## User stories

- As **phase 03**, I want the data directory layout and connection-pool
  bounds decided, so `backend-persistence.md` implements against a fixed
  target instead of guessing.
- As **a user who force-quits Alexandryn**, I want my data directory intact
  and Postgres actually stopped, not orphaned and not corrupted.
- As **a contributor debugging "the app won't start"**, I want corruption
  and unreachable-database failure modes to be distinguishable from each
  other in what the system does, not just in prose.

## Functional requirements

- **FR-1** On first run, the Go server MUST initialize a PostgreSQL data
  directory under the OS's standard per-user application-data directory
  (`~/.config`/XDG on Linux, `%APPDATA%` on Windows, `~/Library/Application
  Support` on macOS — platform convention, not invented), never a temp
  directory and never inside the Electron app bundle itself, which is
  overwritten on update. Applies if the directory does not already exist.
  Initialization failure MUST be a `Failed` state
  (`architecture-system.md`'s state diagram), not a silent retry loop.
- **FR-2** The bundled PostgreSQL instance MUST bind to `127.0.0.1`
  (loopback) only, on a port not exposed to the LAN under any
  configuration before phase 12/13 — consistent with constitution §6 and
  `architecture-system.md` FR-3's binding requirement for the Go server
  itself.
- **FR-3** The Go server MUST use a bounded connection pool against
  PostgreSQL. It MUST NOT open a new connection per request with no upper
  limit — the specific bound is phase 03's to size, this spec only
  requires that a bound exists and is enforced.
- **FR-4** Migrations MUST be forward-only. A migration MUST NOT be
  editable once it has shipped in a release — a schema change is a new
  migration, never a rewrite of history (this is the same rule
  `docs/decisions/README.md`'s ADR philosophy already applies to
  decisions; it applies to schema for the identical reason).
- **FR-5** Migrations MUST run automatically at Go server startup, before
  the server reports itself ready (`architecture-system.md` FR-7) and
  before any request that touches storage is served. A migration that
  fails partway MUST leave the system in a state the next startup attempt
  can detect and refuse to proceed past — silently continuing with a
  half-migrated schema is prohibited outright, not just discouraged.
- **FR-6** Migration files are the *only* legitimate way the schema
  changes. No other tool, script, or manual `psql` session is a supported
  path to a schema change, in development or production — this is what
  "schema ownership" means concretely: one path, always reviewable, always
  reproducible from history.
- **FR-7** If PostgreSQL fails to start (corrupted data directory,
  disk full, permissions), the Go server MUST surface a `Failed` state
  naming what failed, and MUST NOT delete, reinitialize, or otherwise
  modify the data directory to "recover" automatically. An automatic fix
  that risks data loss is a worse failure mode than a clear error asking
  the user (or a future recovery tool) to intervene deliberately.
- **FR-8** (Resolves `architecture-system.md` FR-12) On Linux, the Go
  server MUST spawn PostgreSQL via `os/exec` with `SysProcAttr.Pdeathsig`
  set — Go's standard library supports this as a spawn-time attribute of
  the *parent* process. This requires no modification of the PostgreSQL
  binary's own source, unlike ADR 0005's Linux fix for the Go server
  itself (which added a `prctl` call inside code Alexandryn controls) —
  Postgres is third-party, its source isn't Alexandryn's to change.
- **FR-9** On Windows, the Go server MUST create a Job Object with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and assign the spawned PostgreSQL
  process to it — the same parent-side, spawn-time mechanism
  `architecture-desktop-host.md` FR-9 specifies for Electron→Go-server,
  applied one level down. Also requires no changes to PostgreSQL's source.
- **FR-10** (Lower confidence than FR-8/FR-9 — a novel supervisor-process
  pattern this project is specifying, not a well-known stdlib feature like
  FR-8's `Pdeathsig`; stated as the specified approach pending validation,
  not asserted with the same certainty) On macOS, unlike Linux and Windows,
  there is no parent-side,
  spawn-time mechanism available — orphan-prevention requires the *child*
  to monitor its parent (the pattern `architecture-desktop-host.md` FR-8
  uses for the Go server, implemented in code Alexandryn controls). Since
  PostgreSQL's source isn't Alexandryn's to modify, this MUST be
  implemented as a small supervisor process Alexandryn writes, which
  becomes PostgreSQL's actual direct parent (spawns Postgres itself after
  being spawned by the Go server) and monitors the Go server's PID via
  `kqueue`. On detecting the Go server's death, the supervisor MUST kill
  Postgres *and then exit itself* — it does not get to become the very
  kind of orphan it exists to prevent. This makes the supervisor a fourth
  long-running process, macOS-only (`architecture-system.md` FR-1,
  amended to say so). Unverified in this environment (no macOS available)
  — named and specified, not built.

## Non-functional requirements

- **Performance** — connection pool sizing (FR-3) needs a real number from
  phase 03, not invented here; same placeholder status as
  `architecture-system.md`'s other unmeasured budgets.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; no user-facing surface in this spec.
- **Reliability** — FR-7's prohibition on automatic data-directory
  "recovery" is the load-bearing reliability requirement here: PostgreSQL's
  own WAL already handles crash recovery for in-flight transactions; what
  this spec adds is refusing to compound a real problem (corruption, full
  disk) with an automated action that could destroy what's recoverable.
- **Observability** — migration application (success, failure, partial)
  MUST be logged with enough detail to diagnose without a stack trace to
  the user (constitution §11); this is phase 03's
  `backend-errors-and-logging.md` contract to design in full, this spec
  only requires that the event exists and is distinguishable from a normal
  connection failure.

## Domain model

Not applicable — this spec is engine/process/schema-ownership, not the
Alexandryn/metadata/source domain (constitution §3). It does establish that
domain migrations (phase 02 onward) have exactly one legitimate mechanism
(FR-6), which every later phase's schema changes must go through.

## API and contracts

- **Go server ↔ PostgreSQL**: standard PostgreSQL wire protocol, over
  loopback (FR-2). Driver/access-layer choice (`database/sql` + driver vs.
  a query builder): phase 03's `backend-persistence.md`, per its own
  "Architecture decisions expected" list.
- **Go server → PostgreSQL (spawn/control)**: `os/exec` with platform-
  specific `SysProcAttr` (FR-8, FR-9), or via the macOS supervisor process
  (FR-10). Not an HTTP or IPC boundary — direct process management,
  same category as `architecture-system.md`'s Electron→Go-server
  control-plane channel, one level down.

## State transitions

Extends `architecture-system.md`'s application states with what "Starting"
now actually contains:

```
Starting -> (init data directory if absent, FR-1)
         -> (spawn PostgreSQL, FR-8/FR-9/FR-10)
         -> (wait for Postgres to accept connections)
         -> (run migrations, FR-4/FR-5)
         -> Ready
Starting -> Failed (data directory init failed, or Postgres won't start, FR-7)
Starting -> Failed (migration failed partway, FR-5 — does not proceed to Ready)
Ready -> Degraded (PostgreSQL connection lost after startup — unchanged from
         architecture-system.md)
```

Illegal transitions, restated for this layer:

- `Ready` reached with a migration that didn't fully apply (violates FR-5)
- Any automatic data-directory modification following a failed start
  (violates FR-7) — the only legal transition from a failed start is
  `Failed`, pending manual intervention

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Data directory doesn't exist (true first run) | `FR-1` check at startup | Normal startup, no error — this is the expected first-run path, not a failure | Initializes it, proceeds |
| Data directory exists but PostgreSQL won't start (corruption, permissions) | Postgres process exits non-zero | `Failed` state naming what happened, not a raw Postgres error dump (constitution §11) | Does not touch the data directory; surfaces the failure for manual intervention (FR-7) |
| Disk full during migration | Migration write fails | `Failed` state, distinct message from "won't start" | Does not attempt to roll back automatically past what PostgreSQL's own transaction guarantees provide; does not proceed to `Ready` |
| PostgreSQL orphaned (Go server killed without triggering FR-8/9/10) | The mechanism itself failing is the failure — no higher-level detection exists | Nothing directly; the practical symptom is a held loopback port on next start | Next startup attempt's port-bind step (`architecture-system.md` FR-3-adjacent, for Postgres's own port) fails clearly rather than silently connecting to a stale, unmanaged instance |

## Security considerations

- **Data directory is the whole library** — no encryption at rest is
  decided here (not raised by any prior ADR); flagged as an open question
  rather than assumed either way, since it's a real decision with real
  tradeoffs (recovery complexity, performance) that deserves its own
  consideration, not a default backed into a persistence spec.
- **PostgreSQL as a spawned process Alexandryn doesn't own the source of**
  — FR-8/FR-9/FR-10 exist specifically because the orphan-prevention
  approach ADR 0005 used (modify the child's own source) isn't available.
  The macOS supervisor (FR-10) is new attack surface worth naming: it's a
  small process Alexandryn does write, so the same "our own binary, never
  a path influenced by external input" requirement
  (`architecture-system.md`'s Security considerations) applies to it too.
- **Migrations as the only schema-change path (FR-6)** is itself a security
  property, not just hygiene: an attacker who could trigger an out-of-band
  schema change (a debug endpoint left in, say) would have a much larger
  blast radius than one confined to data within the existing schema.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Migration forward-only enforcement (FR-4), connection pool bound configuration (FR-3) |
| Integration | Full startup sequence against a real bundled Postgres — init, spawn, migrate, ready (phase 03's actual test harness, `backend-test-harness.md`) |
| Contract | N/A |
| E2E | Corruption/failure walkthrough (below) |
| Accessibility | N/A |

- **Walkthrough** — fresh install: no data directory exists, Go server
  initializes one, spawns Postgres, migrates, reaches `Ready`. Second
  walkthrough: existing data directory, Postgres spawned, no migrations
  pending, reaches `Ready` faster (no init step).
- **Hostile walkthrough** — data directory deliberately corrupted between
  runs (simulating a crash mid-write): system MUST reach `Failed`, MUST NOT
  attempt automatic deletion/reinitialization (FR-7).

## Acceptance criteria

- [ ] FR-1 through FR-10 each map to an exit criterion in phase 03's
      `backend-persistence.md` or phase 05's desktop-host specs, whichever
      owns the actual implementation
- [ ] `architecture-system.md`'s amendment (FR-1, FR-2, FR-12) is
      consistent with this spec — cross-checked, not just cross-referenced
- [ ] The macOS supervisor process (FR-10) is named as new surface in
      Security considerations, not treated as a free extension of the Go
      server's own trust boundary

## Open questions

- **Encryption at rest** — not decided, not assumed. Real tradeoff, needs
  its own consideration before phase 03 builds against an assumption
  either way.
- **Backup and restore has no owner in the current 19-phase roadmap.**
  Checked: none of phases 00–17 or 99 name it as a deliverable. For a
  self-hosted app where the maintainer is the only support channel, "no
  backup story" is a real product gap, not paperwork — surfacing this
  explicitly rather than letting the absence stay quiet. Most likely home
  is phase 99 (release, alongside the self-hosting story) or a new phase;
  not decided here, but it needs the maintainer's attention, not just a
  future spec's.
- **Postgres major-version upgrade across app updates** — completely
  undesigned. What happens to an existing data directory when a new
  Alexandryn version bundles a newer Postgres major version is a real
  question with no answer here.
- **macOS supervisor process (FR-10) is unverified** — same caveat as ADR
  0005 and `architecture-desktop-host.md` FR-8: named and specified, not
  built or tested, no macOS available in this environment.
- **Connection pool size** — placeholder-free this time; genuinely no
  number proposed, since there's no load data yet. Phase 03's to set.

## References

- ADR 0004 — persistence engine is self-hosted PostgreSQL
- ADR 0007 — production PostgreSQL is bundled and managed by the Go server
- `.claude/reviews/0007-spec-architecture-desktop-host.md` finding 1 — where
  this entire spec traces back to (no design screen configures a database
  connection)
- `architecture-system.md` — amended FR-1, FR-2, FR-12 as a direct
  consequence of this spec's existence
- `architecture-desktop-host.md` FR-8/FR-9 — the Electron→Go-server pattern
  this spec's FR-8/FR-9/FR-10 apply one level down
- `docs/roadmap/03-backend-foundation/README.md` — owns the actual
  driver, migration tool, and connection-pool implementation
- Constitution §6 (network exposure), §11 (copy)
