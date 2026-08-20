# 0021. Operations spanning repositories compose through a domain-declared Transactor carrying its handle in context; events are written to a transactional outbox

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-19 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 02 defines operations that write more than one aggregate. Phase 03
forbids the only mechanisms that would let them do so atomically. Neither
document notices.

`backend-persistence.md` FR-4 (`:131-144`) requires that an operation needing
atomicity "MUST run inside one `pgx` transaction ... and the transaction
boundary MUST be decided by the repository method implementing that specific
domain operation", then closes the escape hatch: "`internal/domain`'s
interfaces MUST NOT expose a `pgx.Tx` type or any persistence-specific
transaction handle in their method signatures ... a multi-step domain operation
is exposed as one repository method that internally manages its own
transaction, not as several methods the caller must sequence correctly."

FR-2 (`:104-111`) puts each aggregate in its own repository type, and the
Domain model section adds "never one repository spanning multiple domains'
aggregates" (`:268-270`). Together: an atomic operation may only touch rows one
repository owns. Three phase 02 requirements do not fit inside that.

**The source cascade.** `domain-source.md` FR-6 (`:111-113`) requires that
removing a `Source` "MUST remove every `SourceOffering` referencing it (those
observations are meaningless without the source that made them)". `Source` and
`SourceOffering` are separate repositories (`backend-persistence.md:110`). A
partial cascade leaves offerings whose required `Source` ID
(`domain-source.md:134-135`) points at nothing. There is no legal home for
this operation: `SourceRepository.Remove` writing `source_offering` rows
crosses FR-2's boundary, and composing the two repositories needs the handle
FR-4 forbids.

Compounding it: **`domain-source.md` never declares FR-6 atomic.**
`backend-persistence.md` FR-4 only engages for an operation that "must apply as
a single atomic unit". As the two specs stand, phase 03 has no obligation to
wrap the cascade at all, and a non-atomic implementation violates nothing
written down. The gap is invisible from either document alone.

**The dual write.** `domain-events.md` FR-5 (`:128-134`) requires "exactly one
event per logical change — not zero". An event not written in the same
transaction as the mutation it describes is the classic dual-write problem: the
row commits and the event is lost, or the event is emitted and the row rolls
back. Phase 02 lists the outbox as a candidate and defers it —
`domain-events.md:53-56` names "in-process channel, DB outbox table, message
queue" as "phase 03/09's implementation choice" — and phase 03's eleven
repositories (`:108-111`) contain no event or outbox aggregate. So FR-5's "not
zero" is unachievable atomically for *any* mutation in the system as currently
specified, not merely for the hard ones.

**The merge, conditionally.** `backend-persistence.md` cites the reversible
merge by name as FR-4's motivating example (`:131-133`, `:428-429`) and its
Reliability line claims FR-4 "is what keeps a reversible merge ... actually
atomic at the storage layer" (`:254-257`). Whether it actually spans
repositories depends on what merge does to `Edition`s, which
`domain-bibliographic.md` never states — the defect that spec's own amendment
will settle. If merge re-parents `Edition`s it writes two repositories
(`:108-109`) and lands in exactly this problem; if it writes only the merge
pointer it does not. This ADR must work either way.

What we did not know at the time: whether phase 09's job queue (ADR 0014,
PostgreSQL-backed) would want to consume the same outbox, or keep its own
table. Left open below.

## Decision

**A `Transactor`, declared in `internal/domain`, carrying its handle in
`context.Context`.**

```go
// internal/domain
type Transactor interface {
    InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

The implementation in `internal/persistence/postgres` begins a transaction,
places it in the context it passes to `fn`, and commits or rolls back on
return. Repository methods take a transaction from the context when one is
present and use the pool when it is not, so the same method serves both
transactional and standalone callers with no signature change.

This satisfies `backend-persistence.md` FR-4's actual prohibition exactly: no
`pgx.Tx`, no `*sql.DB`, and no persistence-specific type appears in any
`internal/domain` signature. `context.Context` is stdlib and already crosses
every layer.

It does **amend** FR-4's stated mechanism, and that is the point. FR-4's "one
repository method that internally manages its own transaction" is correct for
an operation within one aggregate and impossible for one across two. The
composing caller is a domain service (ADR 0020) holding both repository
interfaces and the `Transactor` — not a transport handler, and not "several
methods the caller must sequence correctly", which FR-4 was right to forbid.
`backend-persistence.md` FR-4 requires a post-approval amendment to cite this
ADR; it is listed under Consequences.

**A transactional outbox for events.**

Every event `domain-events.md` FR-5 requires is written to an `outbox` table
inside the same transaction as the mutation that produced it. Commit makes the
row and its event durable together or neither. A separate reader drains the
outbox and delivers to consumers. Delivery is therefore at-least-once, and
consumers must tolerate a repeat — which is a weaker guarantee than FR-5's
"exactly one event per logical change" reads as promising, so the distinction
is stated plainly here: **FR-5 is a rule about emission, not about delivery.**
Exactly one event is *written* per logical change. A consumer may see it more
than once. `domain-events.md` FR-5 and its Reliability line (`:143-146`) need
that distinction spelled out; it is not what the current wording says.

The outbox table is a twelfth persisted thing, absent from
`backend-persistence.md` FR-2's list (`:108-111`). That list needs amending.
It is deliberately *not* a phase 02 aggregate: no domain type corresponds to
it, `domain-events.md` states no persistence requirement, and adding a domain
aggregate for a delivery mechanism would give that spec scope it cannot defend.

**`domain-source.md` FR-6's cascade is declared atomic.** The declaration
belongs in `domain-source.md` — it is a domain statement about what the
operation means, and without it phase 03 has nothing to engage with. That
amendment is `domain-source.md`'s to make.

**The outbox is inside the `Sensitive` barrier, not outside it.** An outbox
adds a hop `domain-events.md` FR-3 (`:111-117`) does not currently cover: that
FR constrains which *consumers* may receive a `Sensitive` event, and an outbox
introduces a durable table and a drain process sitting between the mutation and
any consumer. Two consequences, both binding on whoever implements this:

- The drain reads every event, sensitive ones included, and dispatches them. It
  is therefore itself a consumer for FR-3's purposes and MUST NOT log the
  events it handles. A drain that logs "delivered `ReadingProgressUpdated` for
  aggregate X" is the constitution §8 leak the barrier exists to prevent,
  arriving through the transport rather than through a registered consumer.
  `domain-reading.md:145-149` already sets this bar: a log line naming a `Work`
  ID alongside a reading event is the leak.
- The outbox table durably stores reading and ownership activity. That is
  legitimate — it is the same instance's own data, and FR-4 (`:118-127`)
  permits a first-party feature to read it — but it means the retention
  question `domain-events.md:261-265` files as open now has a second home, and
  a table nobody prunes accumulates reading history indefinitely.

**Not decided here:** whether phase 09's job queue (ADR 0014) shares this table
or keeps its own. Both are PostgreSQL-backed and the shapes are similar enough
that someone will ask. Deferred to phase 09 with no assumption of
compatibility.

## Options considered

### Option A — Transactor with the handle in context (chosen)

*For* — satisfies FR-4's literal prohibition without contortion; no signature
in `internal/domain` changes; repository methods work identically inside and
outside a transaction; composes naturally with ADR 0020's domain services,
which already hold the repository interfaces this needs.

*Against* — the transaction is implicit. A repository method that forgets to
check the context for a transaction silently writes outside it, commits
independently, and no test of that method alone catches it. This is the real
cost and it is not small: the failure is silent, and it produces exactly the
partial-write state the mechanism exists to prevent. Mitigation is a
convention plus a test, not a type — every repository method reads its
executor through one shared helper, and an integration test proves a rolled-
back service operation leaves no row from any participating repository. Named
as a residual risk, in the manner `backend-persistence.md:314` already uses for
the macOS supervisor.

### Option B — an explicit domain-declared transaction token

`internal/domain` declares its own `Tx` marker interface, passed explicitly
into every repository method that participates.

*For* — no implicitness. The type system shows which calls are transactional.

*Against, and why it lost* — it puts a transaction parameter in every
repository method signature in `internal/domain`, including the majority that
never need one. `backend-persistence.md` FR-4's prohibition is written against
"any persistence-specific transaction handle", and while a domain-declared
marker is arguably not persistence-specific, threading it everywhere
re-introduces precisely the coupling FR-4's Security note (`:322-327`) says the
ban exists to prevent. Rejected on ergonomics, not on correctness — this is the
option to revisit if Option A's implicitness bites.

### Option C — keep FR-4 as written; forbid cross-aggregate operations

Redraw aggregate boundaries so nothing spans repositories: fold
`SourceOffering` into `Source`'s repository, and accept a larger aggregate.

*For* — no new mechanism at all. FR-4 stands unamended.

*Against, and why it lost* — it fixes the cascade and does nothing for the
dual write, which affects every mutation. It also contradicts
`backend-persistence.md` FR-2's own list (`:108-111`), which names
`SourceOffering` as a separate aggregate for good reason: `domain-source.md`
FR-5 (`:108-110`) makes offerings many-to-many across `Source` and `Edition`,
so they are not owned by a `Source` in the aggregate sense. Rejected.

### Option D — event bus in process, no outbox

Emit events on an in-process channel after commit.

*For* — simplest; `domain-events.md:53` lists it first among candidates.

*Against, and why it lost* — it is the dual write, not a solution to it. A
crash between commit and emit loses the event with no trace, silently
violating FR-5's "not zero" in the one circumstance where an Activity feed or a
metrics count most needs to be trustworthy (`domain-events.md:143-146`).
Rejected.

## Consequences

**Good** — the source cascade becomes implementable atomically. FR-5's "not
zero" becomes achievable for every mutation rather than none. The merge works
under either resolution of `domain-bibliographic.md`'s unstated scope, so that
spec's amendment is unblocked. ADR 0020's domain services get a coherent home
for multi-aggregate operations, so the two ADRs compose rather than compete.

**Bad** — an implicit transaction context is a silent-failure mode, mitigated
by convention and one integration test rather than by the type system. Three
approved documents need post-approval amendments:
`backend-persistence.md` FR-4 (mechanism) and FR-2 (the outbox table),
`domain-events.md` FR-5 (emission versus delivery, and at-least-once), and
`domain-source.md` FR-6 (declare the cascade atomic). An outbox also adds a
drain process, its own failure modes, and a latency floor between mutation and
Activity-feed visibility that nothing currently specifies.

**Neutral** — delivery semantics move from unstated to at-least-once. That is
not worse than today, since today's is undefined, but it is a real constraint
consumers now inherit, and the Activity feed `domain-events.md` FR-4
(`:118-127`) licenses will need to deduplicate.

## Reversal cost

Medium. Cheap today — nothing implements either mechanism. Once phase 06 writes
repositories against the context-carried executor, changing to Option B means
touching every repository method signature; removing the outbox means
re-answering the dual-write question with live consumers attached. The
`Transactor` interface itself is one small declaration and would survive most
changes of implementation.

## Confidence

High on the outbox. It is the standard answer to the dual write, ADR 0014
already commits this project to PostgreSQL for exactly this class of work, and
`domain-events.md:53` had already named it as a candidate.

Medium on the context-carried `Transactor`. It is a common Go pattern and it
satisfies FR-4's constraint cleanly, but its implicitness is a genuine defect
and Option B is the more honest design in isolation. The judgement is that
threading a token through every signature costs more, across far more code,
than the one convention this asks repositories to follow. That is a trade made
without measurement, and Option B is written up specifically so the next person
can take it without re-deriving the argument.

Low on the phase 09 interaction. Whether the job queue and the outbox want to
be one table is genuinely unknown, and the answer may make one of them
awkward.
