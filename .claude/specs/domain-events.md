# Spec: Domain events

| | |
|---|---|
| **Status** | `REVIEWED` (self, approved with changes) |
| **Phase** | `02-domain` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`.claude/reviews/0020-spec-domain-events.md`](../reviews/0020-spec-domain-events.md) — Approved with changes, both findings fixed; self-reviewed, independent read still pending |

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

- **FR-1** Every domain event MUST implement a common shape: event type
  (a fixed discriminator, e.g. `ReadingProgressUpdated`), aggregate ID
  (which entity changed), timestamp, and a `Sensitive() bool` method
  returning a compile-time-fixed value per event type (not computed from
  the event's own data) — enforced by the interface itself, not
  convention.
- **FR-2** Every event `domain-reading.md` defines
  (`ReadingProgressUpdated`, `BookmarkCreated`, `HighlightCreated`, and
  any future reading-domain event) MUST return `Sensitive() == true`.
  Every event from `domain-bibliographic.md` and `domain-source.md` MUST
  return `Sensitive() == false`. From `domain-library.md`,
  `LibraryEntryAdded`/`LibraryEntryRemoved` MUST return
  `Sensitive() == true` — they reveal what books a user owns, "what
  someone reads" broadly read (constitution §8 doesn't distinguish
  "owns" from "reads"). The same reasoning applies to
  `CollectionMemberAdded`/`CollectionMemberRemoved` — a curated "want to
  read" list reveals book-level interest exactly as directly as ownership
  does — so those are `Sensitive() == true` too. `CollectionCreated`
  (creating an empty collection, no book referenced yet) stays
  `Sensitive() == false`: it reveals nothing about interest in any
  specific book.
- **FR-3** A `Sensitive` event MUST NOT be passed to any consumer whose
  output can become a system-level log line, metric label, or crash
  report (phase 15's observability pipeline, or any future one) — this
  MUST be enforced by the consumer registration mechanism itself (e.g. a
  logging-consumer's registration function refuses to accept a channel of
  `Sensitive` events at compile time or construction time), not left to
  the logging code to remember to filter.
- **FR-4** A `Sensitive` event MAY be consumed by a first-party,
  user-facing feature reading the *same user's own* data back to them
  (an Activity feed) — this is not a logging destination and constitution
  §8 does not restrict it; a user seeing their own reading history is the
  feature, not the leak this spec's other requirements exist to prevent.
- **FR-5** Every domain mutation that has an event type defined (FR-2's
  list, extensible) MUST emit exactly one event per logical change — not
  zero (silently mutating with no event, defeating phase 15/Activity's
  ability to observe anything), not more than one for a single logical
  operation (e.g. `ReconcileProgress`, `domain-reading.md` FR-6, emits
  one `ReadingProgressUpdated` for the reconciled result, not one per
  input report).

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

- **`Event`** (interface/shape, FR-1) — `Type() string`, `AggregateID()`
  (opaque ID of whichever entity changed), `OccurredAt() time.Time`,
  `Sensitive() bool` (FR-1, fixed per concrete type).
- **Non-sensitive event types**: `WorkCreated`, `WorkMerged`,
  `WorkMergeUndone` (named here on `domain-bibliographic.md` FR-4's
  behalf — that spec requires merges be reversible but doesn't name the
  undo event itself; confirm this name against that spec before
  implementation), `AuthorMerged` (`domain-bibliographic.md`);
  `CollectionCreated` (`domain-library.md`); `SourceCreated`,
  `SourceRemoved`, `SourceOfferingObserved` (`domain-source.md`).
- **Sensitive event types** (FR-2): `LibraryEntryAdded`,
  `LibraryEntryRemoved`, `CollectionMemberAdded`,
  `CollectionMemberRemoved` (`domain-library.md`);
  `ReadingProgressUpdated`, `BookmarkCreated`, `HighlightCreated`
  (`domain-reading.md`).

## API and contracts

Not applicable — pure Go types and an interface. The consumer-registration
mechanism FR-3 requires (compile-time or construction-time rejection of
`Sensitive` events by a logging-shaped consumer) is a phase 03 design
detail building on this spec's `Sensitive()` method, not a transport
contract this spec defines further.

## State transitions

Not applicable in the usual sense — events *are* the record of state
transitions in the four sibling specs. This spec doesn't add new domain
state of its own.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| A new event type added without implementing `Sensitive()` | Doesn't compile (FR-1's interface requirement) | A compiler error | N/A — this is the point |
| A logging consumer attempts to register for a `Sensitive` event stream | FR-3's registration-time rejection | An error at wiring time, not a runtime leak discovered later | Refused; the logging pipeline never receives the event |
| `ReconcileProgress` (a single logical operation) accidentally emits two events | FR-5 violation, caught by a test, not by this domain at runtime | N/A — this is a test-time correctness check, not a runtime-enforced invariant (no cheap way to detect "should have been one event" generically) | Named as a testable requirement, not something the type system can catch, unlike FR-1/FR-3 |

## Security considerations

This entire spec is a security/privacy control. Restated concretely:

- **Constitution §8, made structural, not conventional (FR-1/FR-2/FR-3)**
  — "never logged... what a person reads is private, including from
  their own log files" now has a compile-time-checked home: the
  `Sensitive()` classification and the consumer-registration barrier.
  Nobody has to remember this in a code review; the type system enforces
  it.
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
| Unit | FR-1 through FR-5, table-driven — every event type's `Sensitive()` value asserted explicitly, not just spot-checked |
| Integration | Not applicable — no I/O at this layer |
| Property-based | Not obviously applicable — the sensitive/non-sensitive split is a fixed enumeration, not a universal invariant over generated inputs |
| Table-driven | Every event type in FR-2's list, both categories, with an explicit test asserting each one's `Sensitive()` value — a new event type added later without a corresponding test row is the actual defense-in-depth layer behind the type system |

## Acceptance criteria

- [ ] Every event type from all four sibling domain specs is enumerated
      here or added with a corresponding `Sensitive()` test
- [ ] A test proves a logging-shaped consumer cannot be constructed
      against a `Sensitive` event stream
- [ ] A test proves an Activity-feed-shaped consumer *can* be constructed
      against a `Sensitive` event stream (FR-4's exemption actually works,
      not just FR-3's restriction)
- [ ] `ReconcileProgress` (`domain-reading.md` FR-6) emits exactly one
      `ReadingProgressUpdated` per call, tested directly

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
