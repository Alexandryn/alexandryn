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

- Identity strategy: internal IDs versus external identifiers (`OL…`, ISBN),
  and what happens when a work has neither
- How a work with no Open Library record is represented — this must be a
  first-class case, not an error state, because imported files often have none
- Whether availability is stored or computed, and how staleness is expressed
- What a collection may contain: works, editions, or files
- Whether reading progress attaches to a work, an edition, or a file — this
  determines whether progress survives switching editions, and it is not
  obvious
- Conflict resolution when two devices report progress for the same book

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

- [ ] All five specifications `APPROVED` with recorded reviews
- [ ] The model compiles as pure Go with no persistence, transport, or vendor
      dependencies, enforced by lint
- [ ] Every invariant has a test proving its violation is rejected
- [ ] The work/edition/file distinction holds throughout, with no shortcut type
- [ ] Books with no external metadata are a tested normal case
- [ ] ADRs recorded for progress attachment and identity strategy
- [ ] Every entity in the design prototype's data bindings maps to something in
      the model, or is explicitly recorded as presentation-only
- [ ] Maintainer approval recorded
