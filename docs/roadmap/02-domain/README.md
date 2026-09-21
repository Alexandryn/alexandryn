# Phase 02 — Domain

| | |
|---|---|
| **Status** | In progress |
| **Depends on** | Phase 01 |
| **Blocks** | 03, 06, 07, 08, 10, 11 |

## Objective

A written, tested model of what Alexandryn is about: works, editions, files,
sources, collections, and reading progress — expressed in pure Go with no
database, no HTTP, and no knowledge of Open Library.

If the model is right, most later phases become plumbing. If it is wrong, every
later phase pays for it.

## Why here

The domain is the one thing that must not be shaped by whatever happened to be
built first. Deriving it from the first API endpoint produces a schema that
describes the interface; deriving it from Open Library's JSON produces a schema
that describes a vendor.

It comes after architecture because the layering rules determine what the
domain is allowed to depend on. It comes before persistence because a schema is
a serialisation of a model, not a substitute for one.

## Scope

**In**

- The work / edition / file hierarchy, which the design prototype already
  implies at four levels: a work (`OL27448W`) has editions (each with an ISBN
  and publisher), an edition is available from sources, and a source offers
  concrete files in specific formats
- Author, subject, language, publisher as modelled values
- Library membership — what it means for a work to be "in your library" when
  the file lives somewhere else and might vanish
- Collections, and why they are explicitly local and unsynchronised
- Reading progress, position, bookmarks, highlights
- Source identity, capabilities, and availability, as domain concepts rather
  than protocol details
- Identity and deduplication: when are two records the same work?
- Invariants, legal state transitions, and domain events

**Out**

- Persistence, schema, and indexes — phase 03 implements against this model
- Open Library's response shapes — phase 07 normalises into this model
- Source protocols — phase 08 adapts into this model
- Anything about how these are displayed

## Specifications

| Spec | Covers |
|---|---|
| `domain-bibliographic.md` | Work, edition, author, subject, language, identity and dedup |
| `domain-library.md` | Library membership, collections, ownership, availability |
| `domain-source.md` | Source identity, capability model, availability, file references |
| `domain-reading.md` | Position, progress, bookmarks, highlights, preferences |
| `domain-events.md` | Events the domain emits, and what may consume them |

## Architecture decisions expected

- Identity strategy decided — ADR 0010: internal IDs primary, external
  identifiers (`OL…`, ISBN) optional and never required, a work with
  neither is fully first-class (`domain-bibliographic.md`)
- Availability decided — `domain-source.md`: a timestamped observation
  (`SourceOffering`), never a stored fact; staleness is always expressible
- Collection contents decided — `domain-library.md` FR-4: `Work`s, not
  `Edition`s or files
- Reading progress attachment and conflict resolution decided — ADR 0009:
  attaches to `Work`, `Percentage` primary with an `Edition`-tagged
  precise position as fallback-capable secondary; furthest-wins conflict
  resolution with an explicit override, required to be commutative and
  associative (`domain-reading.md`)

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Collapsing work and edition because the interface usually shows one book | **High** | **Very high** — pervasive and expensive to unpick | The distinction is explicit in the design (`WORK → 12 EDITIONS`); tests assert both levels from the first commit |
| Modelling only books that Open Library knows about | High | High — imports break | An unmatched work is a normal case with its own tests, not a fallback |
| Progress modelled against a file, then breaking when the user changes edition | Medium | Medium | Decided deliberately in an ADR, with the cross-edition case tested |
| Source availability treated as fact rather than as a cached observation | High | Medium | Availability carries an observation time; the model has no way to express "definitely available" |
| The domain quietly acquiring database or JSON tags | Medium | High | Enforced by import-boundary lint from phase 01 |

## Test strategy

This phase is almost entirely unit-tested, and should reach high coverage
honestly — pure logic with no I/O has no excuse.

- Table-driven tests over identity and deduplication, including the hard cases:
  same title different author, reissues, translations, omnibus editions
- Invariant tests: every illegal state transition has a test proving it is
  rejected
- Property-based tests where an invariant is universal, e.g. progress never
  exceeds its bounds and never moves backwards without an explicit reset
- Round-trip tests for anything that will later be serialised

Fixtures are synthetic. Real Open Library payloads belong to phase 07's tests,
behind the adapter.

## Security considerations

The domain is where the trust boundary *ends* — everything reaching it has
already been validated. That only works if the model makes invalid states
unrepresentable rather than merely unlikely.

Specific concerns:

- Every string arriving from a source is attacker-controlled: titles, author
  names, subjects, filenames. The model constrains length and character class
  at construction, so nothing downstream has to remember to.
- A file reference is not a path. Modelling it as a bare string invites path
  traversal at every use site; it gets a type with its own validation.
- Identifiers used in lookups must not be free-form strings, or they become an
  injection surface in whatever ends up doing the lookup.

## Observability

The domain emits events rather than logging. Phase 15 subscribes to them.
Keeping logging out of the domain is what keeps it testable — and stops reading
history from leaking into logs by accident.

## Exit criteria

- [x] All five specifications `APPROVED` with recorded reviews
- [x] The model compiles as pure Go with no persistence, transport, or vendor
      dependencies, enforced by lint — `internal/domain` built for real,
      2026-08-21 (`tasks/plan-phase02-domain.md`), zero imports beyond
      stdlib, `scripts/check-import-boundaries.sh` clean
- [ ] Every invariant has a test proving its violation is rejected — true
      for everything built, with two named exceptions, not silently
      assumed complete: `domain-reading.md` FR-4's "Highlight end position
      not preceding start" isn't checked (Position's format is itself
      undecided, phase 11's job — a naive string comparison would be
      actively wrong, not just untested); and FR-6/FR-7
      (`ReconcileProgress`) aren't implemented at all, separately blocked
      on that spec's own pending amendment — **amendment landed
      2026-09-01 (`0048` finding 1: `max` over `(epoch, percentage)`);
      phase 11 implements `ReconcileProgress`/`OverrideProgress` with
      tests and closes this exception**
- [x] The work/edition/file distinction holds throughout, with no shortcut type
- [x] Books with no external metadata are a tested normal case —
      `TestNewWork_ZeroEditionsAndZeroExternalReferencesIsLegal`
- [x] ADRs recorded for progress attachment (0009) and identity strategy
      (0010)
- [x] Every entity in the design prototype's data bindings maps to something in
      the model, or is explicitly recorded as presentation-only — done via
      review 0021's cross-check; one real gap found (client-side "offline
      copies") and recorded as an explicit non-goal with reasoning, not
      modelled as a new backend entity
- [ ] Maintainer approval recorded
