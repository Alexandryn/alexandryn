---
name: "domain-model-and-boundaries"
description: "Alexandryn's domain model: the three boundaries the constitution refuses to merge (metadata, source, library), the eleven persisted aggregates and who owns each, and the invariants worth remembering while writing internal/domain code. Use whenever writing or reviewing domain types, repository interfaces, or anything under internal/domain."
---

Assumes phase 02's five domain specs (`domain-bibliographic.md`,
`domain-library.md`, `domain-source.md`, `domain-reading.md`,
`domain-events.md`) — this skill restates their decisions as a
checklist. Read those specs for the *why* and the full invariant list;
this is the *what to remember* while writing code against them.

## Three boundaries — constitution §3, never merged for convenience

| Boundary | What it is | Never |
|---|---|---|
| **Metadata** | What a book *is* — Open Library work/edition records | Let a provider's response shape leak past its adapter |
| **Source** | Where a file *comes from* — a configured OPDS server, a local folder | Let a source's protocol reach the domain |
| **Alexandryn** | What the user *has and does* — library, collections, progress | Collapse this into metadata or source because a field "basically means the same thing" |

Each integration is normalised at its own boundary. If a change makes it
convenient to skip that normalisation, that's the signal to stop, not to
proceed.

## The eleven persisted aggregates, by owner

- `domain-bibliographic.md` — `Work`, `Edition`, `Author`
- `domain-library.md` — `LibraryEntry`, `Collection`
- `domain-source.md` — `Source`, `SourceOffering`
- `domain-reading.md` — `ReadingProgress`, `Bookmark`, `Highlight`,
  `ReadingPreferences`

Value types embedded within these (`Subject`, `Language`,
`FileReference`) and the ephemeral, never-persisted `ProgressReport`
don't get their own repository — constructed and validated as part of
the aggregate that owns them. This is the authoritative list
(`backend-persistence.md` FR-2) — a later addition extends it, doesn't
invent a parallel one elsewhere.

## Repository interfaces live in `internal/domain`

One interface per aggregate, defined in `internal/domain`, implemented in
`internal/persistence/postgres` (`architecture-backend.md` FR-2). The
interface never mentions a persistence-specific type. See
`go-backend-conventions` for the dependency-direction rule this
implies.

## Events (`domain-events.md`)

The domain emits events for state changes other subsystems might care
about — check that spec's own catalog before adding a new event type
ad hoc; an incomplete event catalog is the exact defect class review
`0021` caught once already in this project's history.

## Before writing a new domain type or field

1. Which of the three boundaries does this belong to? If the answer
   requires a sentence with "and" in it, it's probably two boundaries'
   worth of concern trying to be one type.
2. Does an existing aggregate already own this, or is it a genuinely new
   one? Check the eleven above before adding a twelfth.
3. Does this need its own repository, or is it a value type embedded in
   an aggregate that already has one?
