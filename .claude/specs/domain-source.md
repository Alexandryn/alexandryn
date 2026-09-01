# Spec: Source domain

| | |
|---|---|
| **Status** | `IMPLEMENTED` (all Tiers 0–6 implemented and verified, security audit Clear) |
| **Phase** | `02-domain` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-14 |
| **Supersedes** | — |
| **Reviewed in** | [`0018`](../reviews/0018-spec-domain-source.md) (self) + [`0021`](../reviews/0021-phase02-cross-spec-review.md) (two independent agents, cross-spec) — both Approved with changes, all findings fixed; maintainer read and approved 2026-08-19; correctness pass [`0048`](../reviews/0048-phase02-correctness-review.md) |

## Context

Constitution §3 keeps Source ("where a file comes from") as its own
domain, separate from Alexandryn (library) and metadata (bibliographic).
This spec models Source *abstractly* — identity, declared capabilities,
availability as an observation — never a protocol. OPDS, a local folder,
and anything phase 08 adds later all implement the same abstract shape;
none of their specifics belong here (CLAUDE.md: normalise at the
boundary, and a source's protocol must never reach the domain).

Phase 02's own risk table names the exact failure this spec exists to
prevent: "source availability treated as fact rather than as a cached
observation." `domain-library.md` already assumes this spec provides
availability as something read separately from library membership,
carrying its own timestamp — this is where that gets defined for real.

## Problem

Nothing has decided: what a `Source` is as a domain type independent of
protocol, what "available" actually means (a fact or an observation),
how staleness is expressed, or what a file reference is if it must not be
a bare path or URL (phase 02's own security section: "a file reference is
not a path").

## Goals

- `Source` as identity + declared capabilities, zero protocol knowledge
- Availability modelled as a **timestamped observation**, never a stored
  fact with no expiry — closes phase 02's named risk directly
- A `FileReference` as a constrained type, not a string — closes the
  path-traversal risk phase 02's security section names
- Support multiple `Source`s offering the same `Edition` (a user with two
  configured OPDS servers that both happen to carry the same book)
- Define what happens to availability records when a `Source` is removed,
  without touching `domain-library.md`'s `LibraryEntry`s (already
  Source-independent by that spec's FR-3)

## Non-goals

- OPDS or any other protocol specifics — phase 08 adapts into this model,
  never the reverse
- Actual connection configuration (URLs, credentials) — this spec models
  the domain concept of "a source exists and can do X"; where connection
  details live (likely owned by the phase 08 adapter, referenced by
  `Source` ID) isn't decided here
- Downloading/fetching bytes — phase 08/10; this spec stops at "here's a
  reference to a file this source claims to have"
- Bibliographic metadata a source might also carry (its own title/author
  strings) — phase 07 normalises that into `domain-bibliographic.md`;
  this spec doesn't duplicate those fields

## User stories

- As **phase 08's OPDS adapter**, I want a fixed shape to report
  availability into, without the domain caring that it's OPDS.
- As **a user whose source server is slow to respond**, I want the system
  to know its last observation is aging, not treat a five-day-old
  "available" as still true.
- As **phase 10's import matcher**, I want a `FileReference` type that
  can't be constructed from an arbitrary attacker-influenced string.

## Functional requirements

- **FR-1** A `Source` MUST have an internal identifier and a
  user-assigned label. It MUST declare its capabilities from a fixed,
  small set (`CanList`, `CanSearch`, `CanDownload` — extensible, not
  protocol-specific) rather than the domain inferring what a source can
  do from its type. A `Source` MAY carry a coarse `Kind` (e.g. `opds`,
  `local-folder`) for display purposes (an icon, a grouping label) — this
  is a category tag, not protocol configuration; it doesn't imply
  anything about capabilities (declared separately) and phase 08 owns
  what values exist.
- **FR-2** Availability MUST be modelled as `SourceOffering` — a record
  of "this `Source` claims to offer this `Edition`, as this
  `FileReference`, observed at this timestamp." It is **never** a boolean
  stored on the `Edition` or `Source`. Every read of availability MUST
  come with its observation timestamp; there is no way to ask "is this
  available" and get an answer without also getting "as of when." A
  `SourceOffering` is uniquely identified by `Source` + `Edition` +
  `Format` (from `FileReference`, FR-4) — the same source offering the
  same edition in two formats (EPUB and PDF) is two `SourceOffering`
  rows, not one; re-observing the *same* format updates that row's
  timestamp rather than creating a new one.
- **FR-3** The domain MUST NOT express "definitely available" — only
  "observed available as of T." How stale is too stale (a re-check
  policy, a TTL) is NOT decided here; this spec only makes staleness
  *expressible*, phase 09 (async jobs, if that's where re-checking lands)
  decides the policy.
- **FR-4** A `FileReference` MUST be a constrained type, not a bare
  string: an opaque, `Source`-scoped identifier plus a declared format
  (EPUB, PDF, etc.) and a size in bytes if known. It MUST NOT be
  interpretable as a filesystem path or URL by anything in this domain —
  resolving a `FileReference` into actual bytes is phase 08's adapter's
  job, with its own hostile-input handling (constitution §4); this domain
  never parses or constructs a path from one.
- **FR-5** The same `Edition` MUST be offerable by more than one
  `Source` simultaneously — `SourceOffering` is many-to-many between
  `Source` and `Edition`, not a single "the" source per edition.
- **FR-6** Removing a `Source` MUST remove every `SourceOffering`
  referencing it (those observations are meaningless without the source
  that made them) and MUST NOT touch `domain-library.md`'s
  `LibraryEntry` records — a `LibraryEntry` doesn't reference a `Source`
  at all (only an `Edition`, per that spec's FR-1), so there's nothing
  there to cascade.

  **This cascade MUST apply as a single atomic unit** (amended per
  [ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md)):
  a `Source` removed with only some of its `SourceOffering`s also removed
  leaves offerings whose required `Source` reference points at nothing,
  which is worse than not starting the removal at all. Because `Source`
  and `SourceOffering` are separate repositories, this domain spec's own
  statement that the operation is atomic is what gives
  `backend-persistence.md` FR-4's transaction mechanism something to
  engage with — without it, a non-atomic implementation would violate
  nothing written down (the gap ADR 0021 itself identifies). The domain
  service owning this operation ([ADR 0020](../decisions/0020-graph-invariants-in-domain-services.md))
  composes both repositories through the `Transactor` interface ADR 0021
  declares, rather than sequencing two independent calls a caller could
  observe half-applied.

## Non-functional requirements

- **Performance** — not applicable; no I/O in this layer.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable to a pure domain model.
- **Reliability** — FR-3's "no definitely available" rule is the
  reliability property: nothing downstream can be built on an assumption
  this domain doesn't actually support.
- **Observability** — `SourceOffering` creation/update/removal are domain
  events (`domain-events.md`), not logged directly here.

## Domain model

- **`Source`** — internal ID, label (constrained per
  `domain-bibliographic.md` FR-5's validation pattern), capability set
  (FR-1), optional `Kind` tag (FR-1).
- **`SourceOffering`** — internal ID, `Source` ID (required), `Edition`
  ID (required), `FileReference` (FR-4), observed-at timestamp (FR-2,
  FR-3). Unique by `Source` + `Edition` + `Format` (FR-2). Many-to-many
  between `Source` and `Edition` (FR-5).
- **`FileReference`** — opaque `Source`-scoped identifier, declared
  format, size in bytes (optional, unknown is legal).

Explicitly not modelled here: which offerings the user actually owns
(`domain-library.md`'s `LibraryEntry`, which references `Edition`
directly and doesn't care which `Source` — or whether any `Source` still
— offers it).

## API and contracts

Not applicable — pure Go model. Phase 08's adapters are the only thing
that ever constructs a `SourceOffering`; phase 03's repository interfaces
persist it.

## State transitions

```
Source created (capabilities declared, FR-1) -> can have SourceOfferings
SourceOffering created -> observed-at set to construction time (FR-2)
SourceOffering re-observed (same Source, same Edition, same Format, new check)
    -> observed-at updated, not a new row (FR-2's uniqueness key)
SourceOffering observed in a new Format for an already-offered Edition
    -> a new SourceOffering row (different uniqueness key), not an update
Source removed -> every SourceOffering referencing it removed (FR-6)
Source removed -> LibraryEntry records referencing Editions that Source offered: untouched (FR-6)
```

Illegal: a `SourceOffering` referencing a `Source` that doesn't declare
`CanList` or `CanDownload` capability sufficient to have produced it in
the first place — not strictly enforced at the type level (a capability
mismatch is more of an application-logic concern than an invariant this
domain can check in isolation), named here as a genuine open question
rather than silently assumed fine.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Availability queried for an `Edition` with no `SourceOffering` at all | Empty result set | "Not currently available from any source" — distinct from "available but stale" | No offering exists; nothing to report as stale either |
| Availability queried, only stale `SourceOffering`s exist | Caller reads the observed-at timestamp itself | The offering, with its age — the *caller* (a later phase's UI/logic) decides what "too old" means, this domain doesn't hide the age | Returns the offering as-is, timestamp included, no filtering by this domain |
| `Source` removed while `SourceOffering`s reference it | FR-6's cascade rule | Those offerings simply stop appearing in availability queries | `SourceOffering` rows removed; `LibraryEntry`s (if any exist for the same `Edition`) are completely unaffected |

## Security considerations

- **`FileReference` is not a path (FR-4)** — this is the concrete
  enforcement of phase 02's own named risk. A malicious source returning
  a filename like `../../etc/passwd` or an absolute path never becomes a
  filesystem-interpretable value in this domain; it's an opaque
  identifier the adapter that issued it (phase 08) is responsible for
  resolving safely, with its own validation, not this domain's.
- **`Source` labels are hostile input** — user-typed, same validation bar
  as `domain-bibliographic.md`/`domain-library.md`'s constrained strings.
- **Availability as observation, not fact (FR-2/FR-3), is itself a
  security property**, not just a data-freshness one: a compromised or
  malicious source cannot cause the system to treat a stale or fabricated
  claim as current truth, because "current truth" isn't a concept this
  domain expresses at all — only "observed at T," which a caller can
  always discount.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | FR-1 through FR-6, table-driven |
| Integration | Not applicable — no I/O |
| Property-based | FR-5's many-to-many invariant: for any set of `Source`/`Edition`/`SourceOffering` combinations, querying by either side returns exactly the offerings that reference it, generated adversarially |
| Table-driven | FR-6's cascade: `Source` with offerings for an `Edition` that also has a `LibraryEntry` — removing the `Source` removes the offerings, the `LibraryEntry` survives untouched |

## Acceptance criteria

- [ ] `SourceOffering` always carries an observation timestamp; no
      code path constructs one without it
- [ ] A test proves availability can be read with its age, and that no
      API in this domain answers "is it available" as a bare boolean
- [ ] `FileReference` cannot be constructed from a raw path or URL string
      directly — a test attempting it fails to compile or is rejected at
      construction
- [ ] A test proves removing a `Source` removes its `SourceOffering`s but
      leaves an existing `LibraryEntry` for the same `Edition` untouched
- [ ] A test proves the same `Source` offering the same `Edition` in two
      `Format`s produces two `SourceOffering` rows, and re-observing one
      `Format` updates only that row's timestamp (FR-2's uniqueness key)

## Open questions

- **Capability-vs-offering consistency** — noted in State transitions as
  an unenforced illegal state. Whether this needs a real invariant or is
  fine as an application-logic concern (phase 08's adapters are trusted
  to only produce offerings consistent with their own declared
  capabilities) isn't decided here.
- **Re-check/staleness policy** — FR-3 makes staleness expressible but
  explicitly doesn't decide how stale is too stale, or what triggers a
  re-observation. Likely phase 09 (async jobs), not assumed here.
- **Multi-format offerings** — resolved (FR-2): one `FileReference`/
  `Format` per `SourceOffering` row, uniqueness key includes `Format`, so
  a source offering EPUB and PDF of the same `Edition` is two rows. Not
  stress-tested against a real source's actual behavior; phase 08 may
  still surface a case this doesn't cleanly cover.
- **A source item that is one logical reading unit split across
  multiple files** — not decided here. `FileReference` (FR-4) models
  one opaque identifier per file, and nothing in this domain represents
  several `FileReference`s that jointly constitute a single reading
  unit rather than independent alternatives. Whether that shape is out
  of scope entirely (each file is its own, independently-imported item
  — a known-degraded outcome for a book stored that way) or needs a
  real modelling extension is unresolved; `backend-file-extractors.md`
  and `backend-import-pipeline.md` (phase 10) each note the same gap
  from their own side
  ([`0049`](../reviews/0049-real-world-edge-case-conformity-review.md),
  finding 2).

## References

- Constitution §3 (source is its own domain, never merged with
  metadata or Alexandryn), §4 (hostile input — file references
  specifically)
- `.claude/roadmap/02-domain/README.md` — "source availability treated
  as fact rather than as a cached observation," this spec's central risk
- `domain-library.md` — `LibraryEntry`'s explicit independence from
  `Source`, which this spec's FR-6 preserves
- `domain-bibliographic.md` — `Edition`, referenced by ID only
- CLAUDE.md — "assuming a filename from a source is safe... it is a
  string an attacker chose"
