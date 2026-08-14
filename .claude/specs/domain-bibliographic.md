# Spec: Bibliographic domain

| | |
|---|---|
| **Status** | `REVIEWED` (self + independent, approved with changes) |
| **Phase** | `02-domain` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0016`](../reviews/0016-spec-domain-bibliographic.md) (self) + [`0021`](../reviews/0021-phase02-cross-spec-review.md) (two independent agents, cross-spec) — both Approved with changes, all findings fixed; maintainer's own read still pending |

## Context

Phase 01 fixed how this model is protected (`internal/domain`, zero
outward dependencies, import-boundary lint — `architecture-backend.md`
FR-1/FR-2/FR-3). Phase 02's own risk table names the failure this spec
exists to prevent: collapsing Work and Edition because the interface
usually shows one book, and modelling only books Open Library knows about.
The design reference already implies the shape (`WORK → 12 EDITIONS`,
`.design-reference` `atBook` screens) but the prototype's runtime state
shortcuts don't fix identity, dedup, or what a book with no Open Library
record looks like — those are this spec's job.

## Problem

Nothing has decided: what identifies a Work or Edition (internal ID vs.
external identifier, and what happens with neither), when two records
count as the same Work, what an Author/Subject/Language actually is as a
type, or how a string arriving from a hostile source becomes a value this
model can hold without downstream code having to re-check it.

## Goals

- Work and Edition as distinct types, never conflated, with the
  containment direction fixed (an Edition belongs to exactly one Work)
- Identity strategy: internal IDs are primary, external identifiers are
  optional attached references, never the primary key
- A Work or Edition with zero external references is a fully valid,
  permanent, first-class state — not an error, not "pending"
- Dedup/matching as something the domain *supports recording*, not
  something it *computes* — matching algorithms are phase 07/10's job
- Author, Subject, Language as constrained value types, not free-form
  strings, closing the "domain is where the trust boundary ends" bar
  phase 02's own security section sets

## Non-goals

- The matching *algorithm* (fuzzy title/author comparison, confidence
  scoring) — phase 07 (Open Library normalisation) and phase 10 (import
  matching) implement it; this spec only defines what a confirmed match
  looks like once one exists
- Persistence, indexes, query shape — phase 03
- Open Library's actual response shapes — phase 07's adapter normalises
  into this model; nothing here should look like an Open Library JSON
  document
- Library membership (owning a work), collections — `domain-library.md`
- Availability and source offerings — `domain-source.md`
- Reading progress — `domain-reading.md`

## User stories

- As **an imported EPUB with no metadata match**, I want to become a
  fully valid Work and Edition with no external references, not an error
  state or a fallback type — this is phase 02's own stated exit criterion.
- As **phase 07's Open Library adapter**, I want a clear target shape to
  normalise into, so vendor response quirks never leak past the adapter
  boundary (CLAUDE.md's own standing warning).
- As **phase 10's import matcher**, I want a place to record "these two
  Works are the same" once matching decides it, without the domain itself
  having opinions about how that decision gets made.

## Functional requirements

- **FR-1** A `Work` MUST have an internal identifier (generated at
  creation, never derived from external data) as its only primary
  identity. External references (an Open Library work key, e.g.
  `OL27448W`) are an optional, attached field — present or absent, never
  required, never the primary key.
- **FR-2** An `Edition` MUST belong to exactly one `Work` (a required
  reference, not optional) and MUST have its own internal identifier,
  independent of its parent Work's. An Edition's *external references*
  (Open Library edition key — a link to another catalog's record of this
  edition) are optional and independent of whether the parent Work has
  any. **ISBN is not an external reference** — unlike an Open Library
  key, an ISBN is inherent data *about* the edition itself, not a pointer
  to another system's record of it. It gets its own optional field,
  validated by its own format rule (ISBN-10/13), not folded into the
  external-references collection ADR 0010 describes.
- **FR-3** A `Work` with zero Editions MUST be a legal, constructible
  state (a book known about but not yet catalogued at edition level, or
  metadata-only from Discover browsing before any edition enters a
  library). A `Work` or `Edition` with zero external references MUST be
  equally legal and MUST NOT be distinguished in type from one with
  references — same type, optional field empty, not a different case to
  handle downstream.
- **FR-4** The domain MUST NOT compute whether two Works are "the same
  work" from string similarity (title, author name) at construction or
  query time — that is a matching decision made elsewhere (phase 07/10).
  The domain MUST provide a way to *record* a confirmed match (e.g. a
  `MergedInto` or equivalent reference from one Work to another) once
  something else decides it, and once recorded, every read of the merged
  Work MUST resolve through to the canonical one — no caller has to know
  to follow the link manually. A recorded merge MUST be reversible — an
  automated or semi-automated match (phase 10) can be wrong, and there
  MUST be a way to undo it that doesn't require deleting and recreating
  either Work.
- **FR-5** `Author`, `Subject`, and `Language` MUST be constrained value
  types, not bare strings, each validated at construction: a maximum
  length, a restricted character class appropriate to the field (a
  `Language` is a BCP-47 tag, not arbitrary text; an `Author` name and a
  `Subject` are bounded, printable text with control characters
  rejected). Construction failure is a domain-level error the caller must
  handle, not a panic and not a silent truncation.
- **FR-6** Every free-form string field on `Work` or `Edition` (title,
  subtitle, publisher) MUST go through the same construction-time
  validation as FR-5 — length bound, control-character rejection. This is
  where "every string arriving from a source is attacker-controlled"
  (phase 02's own security section) actually becomes enforced, not just
  stated.
- **FR-7** `Author` MUST support the same identity pattern as `Work`
  (FR-1): internal ID primary, optional external reference (Open Library
  author key). A Work may reference zero authors (anonymous or unknown)
  — not an error state, same reasoning as FR-3. `Author` MUST also support
  the same merge pattern as `Work` (FR-4), reversible the same way —
  duplicate author records (a name spelling variant, a pen name recorded
  separately) are a real Open Library data-quality issue, not a
  hypothetical one.
- **FR-8** Illegal states MUST be unrepresentable by construction, not
  merely rejected at runtime where avoidable: an `Edition` cannot be
  constructed without a parent `Work` reference (Go's type system, not a
  nil-check convention); a `MergedInto` reference (FR-4) cannot point to
  itself (a Work merged into itself is a cycle, checked at the point the
  merge is recorded, not discovered later by a caller walking the chain).
- **FR-9** An omnibus (one `Edition` containing several distinct creative
  works, e.g. a trilogy bound as one volume) is modelled as its own
  `Work`, with an optional set of "contains" references to the
  constituent `Work`s it collects — **not** as an `Edition` belonging to
  more than one `Work` (FR-2's one-`Work`-per-`Edition` containment stays
  exactly as strict as written). A "contains" reference MUST NOT create a
  cycle (a Work cannot, transitively, contain itself), checked the same
  way FR-8 checks merge cycles.
- **FR-10** `Edition` MUST have its own `Language` field (FR-5's
  constrained type), independent of `Work.OriginalLanguage`. A translation is a
  same-`Work`, different-`Edition`, different-`Edition.Language` case —
  this is the field that actually varies per translation. `Work.OriginalLanguage`
  is reinterpreted as the work's original/first-written language, a
  distinct and equally legal concept, not a duplicate of the same fact.

## Non-functional requirements

- **Performance** — not applicable; no I/O in this layer.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable to a pure domain model.
- **Reliability** — FR-8's construction-time invariants are the
  reliability property: a Work/Edition graph that type-checks is a graph
  with no dangling or self-referential merge chains to defend against
  later.
- **Observability** — this spec doesn't emit events itself
  (`domain-events.md` does); it's the thing events describe changes to.

## Domain model

This spec *is* the domain model for this slice. Core types:

- **`Work`** — internal ID, title, subtitle (optional), authors (zero or
  more `Author` references, FR-7), subjects (zero or more `Subject`
  values), original language (`Language`, optional — unknown is legal,
  FR-10), external references (zero or more, keyed by source:
  `openlibrary` today, extensible), merge state (FR-4), "contains"
  references to constituent Works for the omnibus case (FR-9, empty for
  an ordinary Work).
- **`Edition`** — internal ID, parent `Work` ID (required), language
  (`Language`, FR-10 — this edition's actual language, independent of the
  Work's original language), ISBN (optional, validated format when
  present), publisher (optional), publication year (optional), external
  references.
- **`Author`** — internal ID, name (FR-5 constrained), external
  references, merge state (FR-7, same pattern as `Work`).
- **`Subject`, `Language`** — value types (FR-5), not entities: no
  identity of their own, compared by value, safe to construct freely once
  validated.

Explicitly not modelled here: which Editions are "available," which Work
is "in the library," what a Source is. Those are the sibling specs'
aggregates, referencing `Work`/`Edition` by ID, never embedding them.

## API and contracts

Not applicable — this is the pure Go model
`architecture-contracts.md`'s HTTP layer and `architecture-backend.md`'s
repository interfaces will eventually wrap. No transport shape decided
here.

## State transitions

```
Work created (no editions, no external refs) -> legal, permanent state
Work created -> Edition(s) added over time -> still the same Work identity
Work A merged into Work B (FR-4) -> reads of A resolve to B
Work merged into itself -> rejected at merge-recording time (FR-8)
```

Illegal: an `Edition` existing with no parent `Work` reference (prevented
by FR-2's type-level requirement, not a runtime check); a merge cycle
(FR-8); a free-form string field bypassing FR-5/FR-6 validation (prevented
by construction being the only way to produce a valid value — no exported
way to build one that skips it).

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Title exceeds length bound or contains control characters | FR-6 construction-time validation | A domain-level construction error, naming which field and why | Refuses to construct the `Work`/`Edition`; no partially-constructed value escapes |
| Attempted merge would create a cycle | FR-8 check at merge-recording time | A domain-level error | Merge not recorded; existing state unchanged |
| Edition constructed with no parent Work | Doesn't compile — Go's type system, not a runtime path | N/A | N/A — this is the point of FR-8 |
| Read of a Work that was later merged away | FR-4's resolve-through requirement | The canonical Work's current data, transparently | No caller-visible indirection; the merge chain is an implementation detail |

## Security considerations

- **Every source-derived string is attacker-controlled (constitution §4,
  phase 02's own security section)** — FR-5/FR-6 are where this is
  actually enforced: length bounds and character-class restrictions at
  construction, not a convention every call site has to remember. A
  malicious source setting a title to a 10MB string or embedding control
  characters (terminal escape sequences, null bytes) never produces a
  constructible `Work`.
- **No path-shaped strings in this spec** — file references belong to
  `domain-source.md`, not here; this spec's job is limited to
  bibliographic metadata, deliberately, so the path-traversal risk phase
  02's security section names stays scoped to the spec that actually
  introduces file references.
- **Identifiers used in lookups (FR-1, FR-2, FR-7's internal IDs) are
  generated, not user-supplied** — closes the "identifiers must not be
  free-form strings, or they become an injection surface" concern at the
  type level: an internal ID's type constructor is the only way to
  produce one, never parsed from arbitrary external input as a lookup key
  directly.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Every FR above — this phase is "almost entirely unit-tested" (phase 02's own test strategy) |
| Integration | Not applicable — no I/O |
| Contract | Not applicable |
| Property-based | FR-8's invariants: no construction path produces a cycle, an orphaned Edition, or an unvalidated string field, tested by generating adversarial inputs, not just hand-picked examples |
| Table-driven | The hard dedup cases phase 02's risk table names explicitly: same title different author, reissues, translations, omnibus editions — as fixtures proving the domain does *not* silently merge any of them absent an explicit FR-4 merge record |

## Acceptance criteria

- [ ] `Work` and `Edition` are distinct types; no shortcut type or
      shared base collapses them
- [ ] A `Work` with zero editions and zero external references compiles,
      constructs, and has a passing test — not just a theoretical
      allowance
- [ ] Every FR-5/FR-6 validation has a test proving the *rejection*, not
      just the happy path
- [ ] FR-4's merge-and-resolve-through behavior has a test reading
      through two levels of merge (A merged into B, B merged into C),
      plus a test proving a merge can be undone
- [ ] An omnibus `Work` containing three constituent `Work`s (FR-9) has a
      passing test, including a rejected self-containment cycle attempt
- [ ] A translation fixture (same `Work`, two `Edition`s with different
      `Edition.Language`) has a passing test distinguishing it from a
      reissue (same `Work`, same language, different year/publisher)
- [ ] Every entity named in the design prototype's data bindings maps to
      something in this model or `domain-library.md`/`domain-source.md`,
      or is explicitly recorded here as presentation-only

## Open questions

- **External reference extensibility (`openlibrary` today)** — the shape
  assumes future sources of authoritative metadata might exist beyond
  Open Library. Not designed in detail, just not foreclosed by a
  single-vendor-shaped field.
- **ISBN validation strictness** — ISBN-10 vs. ISBN-13 checksum
  validation, or just format/length. Not decided here; a phase 03/07
  implementation detail once there's real data to validate against.
- **Ordering of merge chains longer than 2–3 hops** — FR-4 requires
  resolve-through and FR-8 requires no cycles, but performance/complexity
  of a long merge chain isn't addressed. Unlikely at this project's scale
  (phase 01's own "one household" non-goal) but not proven, just assumed.
- **Interaction between FR-9's containment graph and FR-4/FR-8's merge
  graph** — each is checked for cycles independently, but nothing
  addresses what happens if Work A contains Work B (FR-9), and B is
  later merged into A (FR-4) — after merge resolution, A would contain
  something that now resolves back to itself, a cycle FR-9's
  containment-time check couldn't have caught since it happened
  afterward. Not resolved here; flagged for whoever implements both
  graphs in phase 03.

## References

- `.claude/roadmap/02-domain/README.md` — this phase's objective, risk
  table, exit criteria
- `architecture-backend.md` FR-1/FR-2/FR-3 — the `internal/domain`
  boundary this model lives inside, and its lint enforcement
- `.design-reference/` — `atBook` screens, the `WORK → 12 EDITIONS`
  hierarchy this spec's Work/Edition split is grounded in
- CLAUDE.md — "Letting an Open Library response shape leak past its
  adapter" standing warning
- Constitution §3 (domain boundaries), §4 (hostile input)
