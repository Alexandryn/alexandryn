# Spec: Domain events

| | |
|---|---|
| **Status** | `APPROVED` (maintainer read 2026-08-19; amended same day for [`0048`](../reviews/0048-phase02-correctness-review.md) findings 4 and 5 — the `Sensitive() bool` method is replaced by a sealed `Event` with `PublicEvent`/`SensitiveEvent` markers and sinks typed against them, making the barrier genuinely compile-time; the acquisition path is split into `…Imported` event types. FR-5 restated as emission-not-delivery per [ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md)) |
| **Phase** | `02-domain` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0020`](../reviews/0020-spec-domain-events.md) (self) + [`0021`](../reviews/0021-phase02-cross-spec-review.md) (two independent agents, cross-spec — the largest finding cluster of the phase) — both Approved with changes, all findings fixed; maintainer read and approved 2026-08-19; correctness pass [`0048`](../reviews/0048-phase02-correctness-review.md) |

## Context

Every sibling domain spec this phase deferred event emission here rather
than inventing its own pattern: `domain-bibliographic.md`,
`domain-library.md`, `domain-source.md`, and `domain-reading.md` all say
"emits events via `domain-events.md`'s pattern, not logging directly."
Phase 02's own Observability section is explicit about why: "the domain
emits events rather than logging... keeping logging out of the domain is
what keeps it testable — and stops reading history from leaking into logs
by accident." That last clause is this spec's sharpest edge:
`domain-reading.md`'s events (progress, bookmarks, highlights) are exactly
the constitution §8 case — "never logged... what a person reads is
private, including from their own log files" — and this is the spec that
has to make that structural, not a convention every future consumer has
to remember.

## Problem

Nothing has decided: what a domain event actually is as a type, what
events exist across the four sibling specs, how a consumer knows an event
is too sensitive to log, or what "consuming" an event even means (phase
15's observability pipeline is one consumer; an in-app Activity feed,
implied by the design reference's `atActivity` screen, is a very
different one with a very different trust level).

## Goals

- One event envelope shape, implemented by every event type across all
  four sibling domains
- A structural `Sensitive` classification on every event type — not a
  runtime decision, not a consumer's judgment call
- Every event from `domain-reading.md` classified `Sensitive`, with a
  concrete rule for what that classification actually restricts
- Distinguish two consumer trust levels: first-party, user-facing (an
  Activity feed showing the user their own history) versus system-level
  observability (phase 15's logs/metrics), since constitution §8 applies
  to the second, not the first

## Non-goals

- The event bus/transport mechanism (in-process channel, DB outbox table,
  message queue) — phase 03/09's implementation choice; this spec fixes
  the event *shape* and classification, not how it's delivered
- The Activity feature's actual UI or query shape — phase 15 / whichever
  phase owns `atActivity`; this spec only says such a consumer is allowed
  to exist and what it may see
- Retry/delivery guarantees, ordering across event types — phase 09 if
  async consumption needs them

## User stories

- As **phase 15's observability pipeline**, I want a compile-time
  guarantee that a `ReadingProgressUpdated` event can't accidentally end
  up in a structured log line, not a code-review checklist item.
- As **a future Activity feed**, I want to read the user's own event
  history, including sensitive events, because showing the user their own
  reading activity is the feature, not a leak.
- As **a contributor adding a new event type to any sibling domain**, I
  want the type system to force me to declare sensitivity, not let me
  forget.

## Functional requirements

- **FR-1** Sensitivity is carried by the **type**, never by a method's
  return value. `internal/domain` declares one sealed base interface and
  two mutually exclusive markers extending it:

  ```go
  type Event interface {
      Type() string          // fixed discriminator, e.g. ReadingProgressUpdated
      AggregateID() ID       // which entity changed
      OccurredAt() time.Time
      isDomainEvent()        // unexported: sealed to this package
  }

  type PublicEvent    interface { Event; isPublicEvent() }
  type SensitiveEvent interface { Event; isSensitiveEvent() }
  ```

  Every concrete event type implements exactly one of `PublicEvent` or
  `SensitiveEvent`, fixed at the type and not computed from the event's
  own data. **A `Sensitive() bool` method is not the mechanism and MUST
  NOT exist**: Go cannot dispatch on a method's return value, so a
  bool-returning classification cannot stop a logging sink from accepting
  a stream carrying sensitive events — it can only be consulted once the
  event has already reached the code meant to be prevented from seeing
  it. That is a runtime convention a contributor breaks by editing one
  `return`, which is what an earlier draft of this FR specified
  ([`0048`](../reviews/0048-phase02-correctness-review.md), finding 4).
  The unexported `isDomainEvent()` is what makes the two markers
  exhaustive: no type outside `internal/domain` can satisfy `Event`, so
  no event can exist that is neither public nor sensitive.
- **FR-2** Every event `domain-reading.md` describes
  (`ReadingProgressUpdated`, `BookmarkCreated`, `HighlightCreated`, and
  any future reading-domain event — see the note on provenance below)
  MUST be `SensitiveEvent`. Events from `domain-source.md`, and events
  recording a *catalog* fact from `domain-bibliographic.md`, are
  `PublicEvent` — **with the acquisition exception below, which is not
  optional**. From `domain-library.md`,
  `LibraryEntryAdded`/`LibraryEntryRemoved` MUST be
  `SensitiveEvent` — they reveal what books a user owns, "what
  someone reads" broadly read (constitution §8 doesn't distinguish
  "owns" from "reads"). The same reasoning applies to
  `CollectionMemberAdded`/`CollectionMemberRemoved` — a curated "want to
  read" list reveals book-level interest exactly as directly as ownership
  does — so those are `SensitiveEvent` too. **`CollectionCreated`
  is also `SensitiveEvent`**: a collection's user-chosen *name* is
  free-form text that can itself disclose sensitive interest (a health
  condition, an identity, a topic) with zero members required — the same
  protective reasoning this FR already applies to membership applies at
  least as strongly to the name. No event in this catalog is
  non-sensitive purely because it references no `Work` — the content of
  the event itself matters too.

  **The acquisition exception, and why it is a catalog split rather than
  a classification.** "Bibliographic events are public" is wrong on the
  import path. `domain-bibliographic.md:63-65`'s own first user story
  makes an imported EPUB "become a fully valid Work and Edition" — so on
  that path, creating the records *is* the acquisition, disclosing
  exactly what `LibraryEntryAdded` is protected for. A Work created by
  Discover browsing (`domain-bibliographic.md:90-93`) genuinely is public
  catalog data. Same act, two different facts, and FR-1's fixed-per-type
  rule cannot express both under one name.

  The resolution is that one event name was covering two events, not that
  classification must become dynamic. They are split by origin:
  `WorkCreated`/`EditionCreated`/`AuthorCreated` are `PublicEvent` and
  are emitted only when a record enters the catalog without entering
  anyone's possession; `WorkImported`/`EditionImported` are
  `SensitiveEvent` and are emitted by the phase-10 import path. A phase
  07/10 caller MUST emit the import variant when the record is created as
  part of acquiring a file, and the catalog variant otherwise. This keeps
  the classification fixed per type, which FR-1 requires, and keeps the
  compile-time barrier intact for the disclosing case
  ([`0048`](../reviews/0048-phase02-correctness-review.md), finding 5).
  
  **Provenance, stated plainly**: none of the four sibling specs
  actually names a concrete event type anywhere in their own text — each
  only says generically "emits events via `domain-events.md`'s pattern."
  Every event name in this spec's catalog is this spec's own invention,
  not a citation of something the sibling specs already defined. Read
  "domain-reading.md defines" and similar phrasing throughout this
  document as "domain-reading.md's mutations are what this event
  corresponds to," not as a citation of a name that exists elsewhere.
- **FR-3** A `SensitiveEvent` MUST NOT reach any consumer whose output
  can become a system-level log line, metric label, or crash report
  (phase 15's observability pipeline, or any future one). This is
  enforced by the signature, not by a filter the logging code must
  remember to apply: such a sink is declared over `PublicEvent`, so
  handing it a `SensitiveEvent` **does not compile**. There is no runtime
  check to forget and no registration-time error to handle, because the
  call cannot be written.

  Two consequences follow, and both are requirements:
  - A sink that legitimately needs both classes (the Activity feed, FR-4)
    is declared over `Event` and is a deliberate, reviewable act — the
    wide type is the signal that it has been thought about.
  - **ADR 0021's outbox drain is itself a consumer for this FR's
    purposes.** The transactional outbox
    ([ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md))
    puts a durable table and a drain process between a mutation and every
    consumer. The drain reads every event, sensitive ones included, so it
    MUST NOT log what it dispatches — a line reading "delivered
    `ReadingProgressUpdated` for aggregate X" is the constitution §8 leak
    arriving through the transport rather than through a registered
    consumer. `domain-reading.md:145-149` already sets that bar.
- **FR-4** A `Sensitive` event MAY be consumed by a first-party,
  user-facing feature reading the instance's own data back to whoever is
  using it (an Activity feed) — this is not a logging destination and
  constitution §8 does not restrict it. (Worded as "the instance's own
  data," not "the same user's own data": no `User` entity exists in this
  phase's single-shared-library model, `domain-library.md`'s own
  Non-goal. If phase 12 introduces accounts, this FR needs revisiting
  alongside that phase's access-control design — not assumed compatible
  now.) Seeing the instance's own reading history is the feature, not the
  leak this spec's other requirements exist to prevent.
- **FR-5** Every domain mutation that has an event type defined (FR-2's
  list, extensible) MUST emit exactly one event per logical change — not
  zero (silently mutating with no event, defeating phase 15/Activity's
  ability to observe anything), not more than one for a single logical
  operation (e.g. `ReconcileProgress`, `domain-reading.md` FR-6, emits
  one `ReadingProgressUpdated` for the reconciled result, not one per
  input report).

  **This is a rule about emission, not delivery.** Exactly one event is
  *written*, in the same transaction as the mutation it describes
  ([ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md)),
  so the pair commits together or not at all — which is what makes "not
  zero" achievable rather than aspirational. Delivery from the outbox is
  **at-least-once**: a consumer may see the same event more than once,
  and any consumer that counts, aggregates, or displays events — the
  Activity feed FR-4 licenses, phase 15's metrics — MUST deduplicate on
  the event's identity. Nothing here promises exactly-once delivery,
  which is not achievable across a process boundary and was never what
  this FR meant.

## Non-functional requirements

- **Performance** — not applicable; no I/O or transport in this layer.
- **Security** — see Security considerations below; this spec's entire
  purpose is a security/privacy property (constitution §8) made
  structural.
- **Accessibility** — not applicable to a pure domain model.
- **Reliability** — FR-5's exactly-once-per-logical-change rule is what
  keeps an Activity feed or metrics count trustworthy — a missed or
  duplicated event corrupts whatever's built on top without an obvious
  symptom.
- **Observability** — this spec *is* the observability contract at the
  domain layer; phase 15 consumes what this spec produces.

## Domain model

- **`Event`** (sealed base, FR-1) — `Type() string`, `AggregateID()`
  (opaque ID of whichever entity changed), `OccurredAt() time.Time`,
  and an unexported `isDomainEvent()` that seals it to this package.
- **`PublicEvent`** and **`SensitiveEvent`** (FR-1) — `Event` plus one
  unexported marker each. Every concrete type below implements exactly
  one of them; the seal makes the pair exhaustive.

**`PublicEvent` types** — `WorkCreated`, `EditionCreated`,
`AuthorCreated` (catalog entry only; see the acquisition split in FR-2 —
records created by the phase-10 import path emit the `…Imported`
variants below instead), `WorkMerged`, `WorkMergeUndone` (both undo-event
names invented by this spec, not cited from `domain-bibliographic.md`,
per the provenance note above), `AuthorMerged`, `AuthorMergeUndone`
(symmetric with `WorkMergeUndone` — FR-7 of `domain-bibliographic.md`
requires the same reversibility for Author merges, so its event coverage
must match), `WorkContainsAdded`, `WorkContainsRemoved` (FR-9's omnibus
containment mutation — cycle-checked the same way merges are, and just as
much a real domain mutation) (`domain-bibliographic.md`);
`SourceCreated`, `SourceRemoved`, `SourceOfferingObserved`,
`SourceOfferingRemoved` (the removal counterpart `domain-source.md` FR-6's
cascade-delete requirement needs) (`domain-source.md`).

**`SensitiveEvent` types** (FR-2) — `WorkImported`, `EditionImported`
(the acquisition path's counterparts to `WorkCreated`/`EditionCreated`,
FR-2's acquisition split) (`domain-bibliographic.md`);
`LibraryEntryAdded`, `LibraryEntryRemoved`, `CollectionCreated`,
`CollectionMemberAdded`, `CollectionMemberRemoved`
(`domain-library.md`); `ReadingProgressUpdated`, `BookmarkCreated`,
`HighlightCreated` (`domain-reading.md`).

This list was built the first time by naming each sibling spec's headline
mutation rather than systematically walking every state-changing FR —
worth naming as the actual failure mode, not just listing the specific
gaps it produced: `Edition`/`Author` creation, containment mutations, and
a removal counterpart were all missed the same way. Anyone adding a new
FR to a sibling spec should check it against this list directly, not
assume a "matching" event already exists.

## API and contracts

Not applicable as a wire contract — these are pure Go types. The one thing
this spec does fix concretely is the sink signature: a logging-shaped
consumer is declared over `PublicEvent` and a both-classes consumer over
`Event` (FR-3), so the barrier is a property of the type declarations
here, not a phase 03 registration mechanism to be designed later. What
phase 03 still owns is delivery — the outbox table, its drain, and its
at-least-once semantics
([ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md)).

## State transitions

Not applicable in the usual sense — events *are* the record of state
transitions in the four sibling specs. This spec doesn't add new domain
state of its own.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| A new event type implementing neither `PublicEvent` nor `SensitiveEvent` | Doesn't compile — `Event` is sealed by an unexported method, so a type that satisfies neither marker cannot be constructed or passed anywhere events flow (FR-1) | A compiler error | N/A — this is the point |
| A logging sink handed a `SensitiveEvent` | Doesn't compile — the sink is declared over `PublicEvent` (FR-3) | A compiler error at the call site | N/A; there is no runtime path to reach |
| A logging consumer attempts to register for a `Sensitive` event stream | FR-3's registration-time rejection | An error at wiring time, not a runtime leak discovered later | Refused; the logging pipeline never receives the event |
| `ReconcileProgress` (a single logical operation) accidentally emits two events | FR-5 violation, caught by a test, not by this domain at runtime | N/A — this is a test-time correctness check, not a runtime-enforced invariant (no cheap way to detect "should have been one event" generically) | Named as a testable requirement, not something the type system can catch, unlike FR-1/FR-3 |

## Security considerations

This entire spec is a security/privacy control. Restated concretely:

- **Constitution §8, made structural, not conventional (FR-1/FR-2/FR-3)**
  — "never logged... what a person reads is private, including from
  their own log files" now has a compile-time-checked home: the
  `PublicEvent`/`SensitiveEvent` split and the sink signatures typed
  against it. Nobody has to remember this in a code review; the type
  system enforces it, and a violation is a build failure rather than a
  leak found later. This claim was **not** true of the `Sensitive() bool`
  method an earlier draft specified — Go cannot dispatch on a return
  value, so that version was a runtime convention wearing a type
  system's clothes ([`0048`](../reviews/0048-phase02-correctness-review.md),
  finding 4). It is true of FR-1 as it now stands.
- **"Owns" is "reads" for this purpose (FR-2)** — a narrower reading of
  constitution §8 might exempt library membership (you own a book, but
  maybe you haven't read it) from the reading-privacy bar. This spec
  doesn't take that narrower reading — a list of what someone owns is
  still revealing, and constitution §8's own phrasing ("what a person
  reads") is naturally read to include their library, not just their
  in-progress position. Treated as sensitive by the more protective
  reading, deliberately.
- **The Activity-feed exemption (FR-4) is narrow and first-party only**
  — it does not extend to any future feature that aggregates across
  users, exports data, or sends it anywhere off-device. Those are new
  questions for whichever phase introduces them, not pre-approved by this
  FR.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | FR-2 through FR-5, table-driven. FR-1 needs no runtime test — it is a compile-time property, and the compiler is its test |
| Compile-time | The barrier itself: a `SensitiveEvent` passed to a `PublicEvent`-typed sink must fail to build. Proven with a `go vet`-style negative-compilation fixture (a file behind a build tag, asserted not to compile), since a passing runtime test cannot demonstrate an absence of runtime path |
| Integration | Not applicable — no I/O at this layer |
| Property-based | Not applicable — the split is a fixed enumeration over a sealed interface, not a universal invariant over generated inputs |
| Table-driven | Every event type in the catalog, asserted to implement the marker FR-2 assigns it. This is no longer the primary defence — the type system is — but it is what catches a *new* type being given the wrong marker, which compiles fine and is exactly the mistake the type system cannot see |

## Acceptance criteria

- [ ] Every state-changing FR across all four sibling domain specs is
      checked against this spec's event catalog, not just each spec's
      headline mutation — every entry implements exactly one marker, with
      a test row asserting which
- [ ] A negative-compilation fixture proves a `PublicEvent`-typed sink
      cannot be handed a `SensitiveEvent` — it must fail to build, not
      fail an assertion
- [ ] A test proves an Activity-feed-shaped consumer, declared over
      `Event`, *can* receive both classes (FR-4's exemption actually
      works, not just FR-3's restriction)
- [ ] A test proves the phase-10 import path emits `EditionImported`
      (sensitive) and the Discover path emits `EditionCreated` (public)
      for what is otherwise the same record creation — FR-2's acquisition
      split, which is the one classification a compiler cannot check
- [ ] `ReconcileProgress` (`domain-reading.md` FR-6) writes exactly one
      `ReadingProgressUpdated` per call, tested directly — emission, not
      delivery (FR-5)

## Open questions

- **Event bus/transport implementation** — explicitly deferred to phase
  03/09 (Non-goals).
- **Does every sibling-domain mutation need an event, or only the ones
  listed?** FR-2's list is what's named across the four specs as
  written; a future addition to any sibling spec needs a corresponding
  event type and sensitivity classification added here — not automatic,
  needs to be deliberate per FR-1's compile-time requirement anyway.
- **Retention/expiry of event history** — if an Activity feed stores
  events long-term, does old reading-activity history need its own
  deletion/retention policy (arguably still constitution §8's concern,
  extended over time)? Not addressed; flagged for whichever phase
  actually builds the Activity feed.

## References

- `.claude/roadmap/02-domain/README.md` — "the domain emits events rather
  than logging," this spec's founding requirement
- `domain-bibliographic.md`, `domain-library.md`, `domain-source.md`,
  `domain-reading.md` — every event type this spec enumerates
- `.design-reference/` — `atActivity` screen, the likely first consumer
  of non-logging, first-party event consumption (FR-4)
- Constitution §8 (never log what someone reads) — this spec's central
  requirement, made structural
