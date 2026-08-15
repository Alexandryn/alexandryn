# 0014. Background job queue is PostgreSQL-backed, not a dedicated message broker

| | |
|---|---|
| **Status** | Proposed (reviewed in [`0036`](../reviews/0036-phase09-cross-spec-review.md), findings fixed; awaiting maintainer approval) |
| **Date** | 2026-08-15 |
| **Deciders** | Claude (Sonnet 5), for review by Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 08 (sources) named source re-checking as work it deliberately
deferred to a background job rather than running synchronously; phase
10 (import) will need to run discovery/extraction/matching without
blocking an HTTP request for however long that takes. Both need a real
place to run asynchronous work — this project's `decisions/README.md`
has carried "whether RabbitMQ is warranted, and for exactly which
work" as an open question since phase 01, explicitly forced by this
phase.

What's known: this is a self-hosted, single-household-scale
application (constitution's own framing throughout) — one Electron
process spawning one Go server on one machine, with a PostgreSQL
instance that process already owns and manages the lifecycle of
(`architecture-persistence.md`). Job volume at this scale is small: a
handful of configured sources re-checking periodically, and imports
triggered by a single user's own actions — not a high-throughput,
multi-tenant, or multi-machine workload. What's not fully known yet:
exactly how CPU/memory-heavy a real import job (EPUB parsing, cover
generation) will turn out to be — flagged in Confidence below, not
assumed away.

## Decision

We run background jobs on a PostgreSQL-backed queue: a `jobs` table in
the same database instance the rest of this application already uses,
with workers claiming rows via `SELECT ... FOR UPDATE SKIP LOCKED` and
no separate broker process, network protocol, or infrastructure
dependency.

## Options considered

### Option A — PostgreSQL-backed queue (chosen)

A `jobs` table (`id`, `kind`, `payload jsonb`, `status`, `attempts`,
`available_at timestamptz`, `locked_until timestamptz`, `lease_token`,
`last_error`, timestamps). A worker pool polls for claimable rows
(`status IN ('queued', 'retrying') AND available_at <= $now`) and
claims them transactionally with `SELECT ... FOR UPDATE SKIP LOCKED` —
the standard PostgreSQL pattern for exactly this problem: `FOR UPDATE`'s
row lock is what prevents two workers from claiming the same row,
`SKIP LOCKED` is what keeps a worker that would otherwise block from
stalling behind another worker's in-flight claim. A crashed or stalled
worker's job is recovered by a lease-plus-heartbeat mechanism on top of
this claim query (`backend-job-queue.md` FR-5) — a bespoke design this
project built itself, not an off-the-shelf pattern the way the claim
query itself is.

**Pros**: no new infrastructure to install, configure, back up, or keep
alive — this application's own `pg-supervisor` process
(`architecture-persistence.md` FR-10) already owns PostgreSQL's
lifecycle, so job storage inherits that lifecycle for free. Jobs and
the data they operate on (a `Source`, a `Work`) live in the same
transactional database, so a job's side effects and its own status
transition can commit atomically — a message broker's queue and this
application's own database would otherwise be two systems that can
disagree after a crash between them. No new dependency to justify
under constitution §9 beyond what `backend-persistence.md` already
established (`pgx`/`pgxpool`).

**Cons**: polling-based dequeue has some latency between enqueue and a
worker picking it up (bounded by the poll interval, this spec's own to
fix — a few seconds, not milliseconds) — a real cost against a message
broker's push-based delivery, but not one that matters for this
project's actual job types (a source re-check or an import running a
few seconds later is invisible to a user who isn't watching a
millisecond-level dashboard). `SKIP LOCKED` polling adds load to the
same PostgreSQL instance serving every other request — bounded by
worker pool size (small, this spec's own to fix) and poll interval,
not expected to be significant at this scale, but a real, named cost.

### Option B — RabbitMQ

A dedicated message broker: durable queues, push-based delivery,
mature retry/dead-letter primitives out of the box.

**Cons, decisively**: a second long-running process this application
would need to spawn, supervise, and back up — directly working against
`architecture-system.md`'s whole deployment shape (one Electron
process, one Go server, one self-managed PostgreSQL instance, nothing
else the user has to think about). Constitution §9's dependency bar
("what breaks if abandoned," "why not stdlib/what's already running")
isn't met: RabbitMQ is a substantial new operational dependency for a
job volume this project's own scale doesn't come close to needing. Its
push-based delivery and mature primitives are real advantages at a
scale this project doesn't operate at.

### Option C — Redis Streams / a lighter broker

Same shape of cons as Option B, one level down in weight: still a
second process, still new infrastructure to manage, still nothing this
project's job volume requires that Option A doesn't already provide.
Rejected for the same reason as Option B, not separately re-argued.

### Option D — An existing Postgres-backed Go queue library (e.g. `river`)

A third-party library implementing the same `SKIP LOCKED`-based pattern
Option A hand-rolls, with its own lease/heartbeat/retry machinery
already built and tested.

**Cons**: a new dependency to justify under constitution §9 for a
mechanism this project's own scale doesn't need much beyond the core
claim query and a modest lease/heartbeat layer — the kind of
"tooling investment disproportionate to the current surface size"
reasoning `desktop-host-ipc-surface.md` FR-1 already used for a
similarly-shaped hand-maintained-vs-generated choice. **Pros, real and
weighed**: a mature library's lease/retry edge cases (exactly the kind
of subtlety this ADR's own spec review found a real gap in — see
`backend-job-queue.md` FR-5's fencing mechanism) have already been
hardened by other users' production experience, which this project's
own hand-rolled version has not. Rejected for this phase specifically
because the surface needed (one job kind's worth of infrastructure, no
real handlers yet to stress it) doesn't yet justify the dependency —
but named honestly as the option most likely to be reconsidered later
if this project's own hand-rolled fencing/reaper logic keeps surfacing
correctness gaps in practice, not dismissed as categorically wrong.

### Option E — In-process only (goroutines/channels, no persistence)

No new storage at all — jobs live only in memory, lost on restart.

**Rejected**: a source re-check or an in-progress import silently
disappearing on every app restart (which, for a desktop application a
user closes and reopens routinely, is not a rare event) is a real
reliability regression against Option A's persisted, resumable jobs.
`backend-service-lifecycle.md`'s own graceful-shutdown discipline would
also have nothing to hand off to — an in-flight job would just be
killed, not paused and resumed.

## Consequences

**Good** — no new process, dependency, or infrastructure to operate;
jobs and application data share one transactional store, so a crash
mid-job can't leave the two disagreeing; this application's existing
PostgreSQL lifecycle management (spawn, health, backup — phases 03/13)
covers job storage for free; the *initial claim* mechanism is
well-understood and widely used at comparable scale (`SELECT ... FOR
UPDATE SKIP LOCKED` is a standard, documented PostgreSQL technique, not
a novel design) — this claim is scoped deliberately to the claim query
alone, not the whole crash-recovery design (see Bad, below).

**Bad** — enqueue-to-start latency is bounded by a poll interval, not
instant; polling adds a small, recurring load to the shared database;
if this project's job volume ever grows well beyond household scale
(not expected, not planned for), this decision would need revisiting —
named honestly rather than claimed to scale indefinitely. Unlike the
claim query itself, the lease/heartbeat/reaper crash-recovery layer
(`backend-job-queue.md` FR-5) is this project's own bespoke design, not
an off-the-shelf pattern — a mature broker or an existing Postgres-
queue library (Option D) would have had this correctness worked out
already; this project's own version needed a real fencing gap found
and fixed during that spec's own review (`0036`) before it was sound,
a genuine engineering cost of the hand-rolled choice, not a free
byproduct of "it's just SQL."

**Neutral** — this decision is purely about *where jobs are stored and
claimed*; it says nothing about what any specific job type (source
sync, import) actually does, which remains each of those phases' own
concern.

## Reversal cost

Moderate, not cheap, and not uniform across the design: the public
`Register`/`Enqueue`/`GetJob`/`ListJobs` API (`backend-job-queue.md`
FR-2, FR-9) is genuinely portable — a future broker-backed
implementation could keep that surface unchanged, so job-enqueueing
call sites (phase 10 onward) wouldn't need to change at all. The parts
that would need real replacement are FR-4 through FR-8 — the claim
query, the lease/heartbeat/reaper mechanism, and the backoff logic are
all specific to this poll-based, single-database model and have no
direct analogue in a push-based broker (which needs no lease/heartbeat
at all, since delivery failure is the broker's own concern). The *job
payload shapes* themselves (`kind`/`payload`) are storage-agnostic
either way. This is not a decision made lightly assuming free
reversal, but it's also not architecturally load-bearing the way the
persistence engine choice (ADR 0004) is — nothing outside
`backend-job-queue.md` and its callers depends on PostgreSQL
specifically being the queue backend, unlike ADR 0004's much broader
reach.

## Confidence

Medium-high. The scale argument (household-sized job volume, no
evidence this project will ever need broker-grade throughput) is
strong and consistent with every other infrastructure decision this
project has made. The one real unknown, named honestly: real import
job cost (EPUB parsing, cover generation CPU/memory) hasn't been
measured yet, since phase 10 hasn't been built — if that turns out to
be far heavier than assumed, worker pool sizing (this spec's own
number) might need revisiting sooner than expected, but that's a
tuning question within this ADR's decision, not a reason to reverse it.
