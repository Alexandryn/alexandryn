# 0020. Invariants that depend on reading other records live in domain services holding domain-defined repository interfaces, never in value constructors

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-19 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Three phase 02 specs place an invariant at construction time that cannot be
decided from the value being constructed. Each requires reading a record the
constructor does not hold:

- `domain-bibliographic.md` FR-8 (`:129-134`) — a `MergedInto` reference
  "cannot point to itself (a Work merged into itself is a cycle, checked at
  the point the merge is recorded, not discovered later by a caller walking
  the chain)". Detecting a cycle of any length requires walking the merge
  chain, which means loading other `Work`s. FR-9 (`:140-142`) states the
  containment version explicitly transitively: "A 'contains' reference MUST
  NOT create a cycle (a Work cannot, transitively, contain itself)".
- `domain-library.md` (`:185-187`) — "a `LibraryEntry` for an `Edition` that
  doesn't exist (prevented by requiring a valid `Edition` reference at
  construction)", with the failure-mode row (`:200`) naming the mechanism as a
  "Construction-time reference check". Existence is not a property of the ID
  value; it is a property of the database.
- `domain-reading.md` (`:197-200`) — "a `PrecisePosition` whose tagged
  `Edition` does not belong to the `ReadingProgress`'s own `Work` — checked at
  construction". An `Edition`'s parent `Work` is stored on the `Edition`
  (`domain-bibliographic.md:80-81`), so the check requires loading it.

All three are written as construction-time guarantees under the heading that
`domain-bibliographic.md` FR-8 gives them: "Illegal states MUST be
unrepresentable by construction, not merely rejected at runtime where
avoidable" (`:129-131`). For a single value — a `Percentage` in `[0.0, 1.0]`,
a BCP-47 `Language`, a length-bounded title — that is achievable and already
correct. For an invariant over a graph it is not, and no spec says which kind
it is talking about.

**A correction to the review that raised this**, recorded per constitution §12
because the ADR would otherwise inherit a false premise. The phase 02
correctness review stated that `internal/domain` cannot perform these checks at
all, citing `go-backend-conventions/SKILL.md:29-36`. Re-reading that text, it
forbids something narrower: "`internal/domain` MUST NOT import
`internal/persistence/postgres`, `internal/transport/http`, or `pgx` ... A
domain method signature MUST NOT mention `pgx.Tx`, `*sql.DB`, or any
persistence-specific type". Repository *interfaces* live in `internal/domain`
(`architecture-backend.md` FR-2, cited at `backend-persistence.md:106-107`),
and calling an interface the domain itself declares imports nothing forbidden.
The constraint is a package-dependency rule, not a prohibition on the domain
performing IO through its own abstractions. What phase 02 has is a design gap,
not an architectural impossibility. The review overstated it.

What we knew at the time: the three FRs, the layering rule, and that phase 03
had already fixed one repository type per aggregate
(`backend-persistence.md:104-111`). What we did not know: whether a fourth
option — pushing these into the database as constraints — would turn out
cheaper once real schema exists in phase 06. It might for one of the three.

## Decision

We draw the line at whether an invariant is decidable from the value alone.

**Invariants over a single value stay in the constructor.** Length bounds,
character classes, BCP-47 shape, `Percentage` range, a `Highlight`'s end
position not preceding its start within the same `Edition` — all of these are
decidable from the value being built, all of them stay exactly where the specs
already put them, and "unrepresentable by construction" remains literally true
for them.

**Invariants over a graph move to a named domain service.** A domain service
lives in `internal/domain`, is constructed with the repository interfaces
`internal/domain` itself declares, and owns the operation end to end. The
operation, not the constructor, is where the invariant is enforced. Concretely:
recording or undoing a merge, adding a containment reference, creating a
`LibraryEntry`, and constructing a `ReadingProgress` carrying a
`PrecisePosition` each become a service method rather than a bare constructor
call.

The three specs' language changes accordingly: these invariants are enforced
**by the domain service that owns the operation**, not "by construction". The
guarantee is real — no caller can reach the aggregate's mutating path except
through the service — but it is enforced at a different place than the current
wording claims, and the wording is what misleads.

**One database constraint is adopted as defence in depth, not as the primary
mechanism.** `LibraryEntry`'s `Edition` reference gets a real foreign key when
phase 06 writes the schema. It costs nothing, it is unforgeable in a way no Go
code is, and it catches the case where a future code path bypasses the service.
It does not replace the service check, because a foreign-key violation surfaces
as a driver error at commit time rather than as a `domain.Error` with the
`NotFound`/`InvalidInput` category the error taxonomy requires
(`go-backend-conventions/SKILL.md:52-60`).

Cycle detection is **not** pushed into the database. See Option C.

## Options considered

### Option A — pure domain services taking the needed data as arguments

A service function stays pure by requiring the caller to supply what the check
needs: `RecordMerge(source, target *Work, chain []WorkID) error`.

*For* — the domain stays free of IO entirely, tests need no fakes at all, and
the layering question never arises.

*Against, and why it lost* — the guarantee is forgeable. A caller that passes
an incomplete or stale `chain` gets a passing check, and nothing anywhere
detects that it was wrong. That is strictly worse than an honest application-
layer check, because it *reads* as enforced: the signature advertises a cycle
check that the callee cannot actually perform. Constitution §12's standard —
record a guess as a guess — applies to guarantees too. Rejected.

The variant where the argument is an unforgeable token only the repository can
mint (a `MergeChain` value with an unexported field) closes the forgery hole,
but at that point the repository is already involved and Option B is the same
thing with less ceremony.

### Option B — domain services holding domain-defined repository interfaces (chosen)

The service is constructed with the interfaces `internal/domain` already
declares, and performs the reads it needs itself.

*For* — the guarantee is real rather than advertised; it imports nothing
`go-backend-conventions/SKILL.md:29-36` forbids; it uses the abstraction
`architecture-backend.md` FR-2 already established rather than inventing a
mechanism; and it puts the invariant in the one place that can both enforce it
and return a properly categorised `domain.Error`.

*Against* — the domain becomes IO-performing where it was pure. Unit tests for
these operations need fakes for the repository interfaces, which
`internal/testutil` (`go-backend-conventions/SKILL.md:22`) is already the home
for. It also opens a path to N+1 query patterns inside domain logic: a naive
transitive cycle check walks the chain one `Work` at a time. That cost is
accepted and named here so it is a known trade rather than a discovery.

### Option C — push the checks into the database

Foreign keys for existence; a recursive CTE or a trigger for cycles.

*For* — genuinely unforgeable. A foreign key cannot be bypassed by any code
path, present or future.

*Against for cycles, and why it lost there* — a cycle rule expressed as a
trigger is a domain invariant living somewhere the domain cannot read, test, or
explain. `domain-bibliographic.md` FR-9's rule is a modelling decision about
what an omnibus is; encoding it in PL/pgSQL hides that from every reader of the
domain package and splits one rule across two languages. It also interacts
badly with the merge/containment interaction still open at
`domain-bibliographic.md:284-291` — a trigger would fire on a state the
domain considers legal at write time.

*Adopted narrowly* — for `LibraryEntry`'s `Edition` reference only, as defence
in depth behind Option B's service check, per the Decision above.

### Option D — accept them as application-layer checks and downgrade the specs' language

Change the three FRs to say the checks happen wherever the caller happens to do
them.

*For* — cheapest, and honest about the current state.

*Against, and why it lost* — it gives up the property rather than placing it.
`domain-bibliographic.md` FR-8's intent is right; it is only the location that
is wrong. Downgrading it would mean a merge cycle becomes possible whenever a
caller forgets, and `:102-104`'s resolve-through requirement has no fixed point
on a cyclic graph, so the failure is a non-terminating read rather than a bad
value. Rejected.

## Consequences

**Good** — one rule resolves three FRs across three specs instead of three
independent fixes that could drift. The domain keeps its invariants and keeps
being able to explain them. `internal/domain` gains no forbidden import, so the
import-boundary lint (`architecture-backend.md` FR-3, ADR 0018) needs no
exception. Value-level validation — the constitution §4 boundary that
`domain-bibliographic.md` FR-5/FR-6 actually enforces — is untouched and stays
literally true.

**Bad** — the domain now performs IO in these paths, so a class of unit test
that used to need nothing now needs fakes. Three specs need their
"unrepresentable by construction" language amended, and one of them
(`domain-bibliographic.md` FR-8) uses that phrase to cover two things at once —
the `Edition`-needs-a-parent case, which stays type-level and true, and the
cycle case, which moves. Splitting that sentence is fiddly and easy to get half
right. A naive transitive check is N+1; nothing here forces a better one.

**Neutral** — the merge/containment interaction open at
`domain-bibliographic.md:284-291` is unaffected. This ADR fixes *where* a cycle
check runs, not *what* counts as a cycle across two graphs, which stays open
for `domain-bibliographic.md`'s own amendment to settle.

## Reversal cost

Low now, medium after phase 06. Today this is a wording change in three specs
and a shape decision no code depends on. Once repositories and services exist,
reversing to Option C means moving live invariants into SQL and re-testing
each; reversing to Option D means deciding which guarantees to abandon. Nothing
in phases 03–05 builds on this directly — phase 06 is the first code that will.

## Confidence

High that the line is drawn in the right place — single-value versus graph is a
distinction the specs already make implicitly, and every case sorts cleanly to
one side.

Medium on Option B over Option C for cycle detection specifically. The argument
against a trigger is about legibility and testability, not correctness, and a
recursive CTE is genuinely stronger. If phase 06 finds the Go-side transitive
walk is a performance problem on real merge chains — unlikely at one
household's scale, per `domain-bibliographic.md:280-283`'s own reasoning, but
unmeasured — that is the trigger to re-open this specific sub-choice, not the
whole ADR.

Low-to-medium on the N+1 cost being tolerable, because no merge chain length
has ever been measured and `domain-bibliographic.md:280-283` records the
short-chain assumption as "not proven, just assumed".
