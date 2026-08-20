# Review: Phase 02 domain model — correctness pass over the twice-reviewed specs

| | |
|---|---|
| **Subject** | All five phase 02 specs (`domain-bibliographic.md`, `domain-source.md`, `domain-library.md`, `domain-reading.md`, `domain-events.md`), read in dependency order, against constitution, ADRs 0009/0010, `go-backend-conventions/SKILL.md` and `backend-persistence.md` |
| **Reviewer** | Claude (Opus 5), single pass; review [`0021`](0021-phase02-cross-spec-review.md) read last, after findings were formed |
| **Date** | 2026-08-19 |
| **Verdict** | Approved with changes — four correctness defects and two architectural gaps, all addressed or scheduled; maintainer approved the specs 2026-08-19 with defects recorded in each status cell |

## Summary

Not a structural or completeness audit — review 0021 already did that well, and
its fourteen findings are largely disjoint from these. This pass asked one
question instead: implemented exactly as written, does each spec produce correct
behaviour? `domain-source.md` does. The other four each carry a defect that
survived two reviews because it reads as reasonable. Two architectural gaps sit
underneath three of the four, and were fixed first as ADRs 0020 and 0021.

The core modelling decisions all survive: internal-ID-primary identity,
Edition-level membership against Work-level progress, and
availability-as-observation are correct and consistently applied. Every defect
below is in a mechanism, not in the approach.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Blocking | `domain-reading.md` FR-6 × FR-7 | Furthest-wins with an explicit override is not order-independent, so the two MUSTs cannot both hold. Trace: canonical 0.5, override 0.1, normal report 0.9 — fold `[o,r]` gives 0.9, fold `[r,o]` gives 0.1. FR-6 justifies itself with "it's a max function" (`:119-120`); FR-7 removes that premise. Not fixable within the specified type: neither field list (`:153-160`) carries anything that could order an override against a concurrent report. The user-visible failure is the deliberate re-read being silently clobbered — the exact ADR 0009 failure, re-entering through the door FR-7 opened | **Fixed** — reconciliation becomes max over `(epoch, percentage)`; server-assigned epoch, observed-epoch precondition for the offline case, third `Rejected` outcome |
| 2 | Blocking | `domain-reading.md` acceptance criteria `:251-255` | The two criteria are mutually unsatisfiable: one requires a passing override test, the other a property test that reconciling in any order gives the same result. If the generator emits overrides it fails; if it excludes them nothing says so, and two green tests would mean the interesting input was quietly skipped | **Fixed** in the same pass as finding 1 |
| 3 | Blocking | `domain-bibliographic.md` FR-4 | Merge scope is unspecified — no line in the spec says what merge does to `Edition`s, authors, subjects or external references. Both available readings break. Pointer-only: A's Editions stay parented to A while reads of A return B, so `domain-library.md` FR-2's "at least one of its Editions has a LibraryEntry" computes **false for a book the user demonstrably owns**. Data-moving: undo becomes lossy, since original parentage is stored nowhere, contradicting FR-4's own reversibility requirement | **Fixed** — merge moves nothing; resolution is read-time and transitive, undo is exactly the identity, and every read that walks a Work's relations resolves through first |
| 4 | Major | `domain-events.md` FR-1 × Goals `:42`, user story `:65-67`, Security `:210-215` | The `Sensitive()` barrier is a runtime check presented as a compile-time guarantee. Go cannot dispatch on a method's return value; every concrete event satisfies `Event`, so a `chan Event` carries both classes and a logging consumer over `<-chan Event` compiles cleanly. FR-3 concedes it ("compile time **or** construction time", `:115-116`) while three other sections claim the type system enforces it. The spec's own `:236` calls the per-type test "the actual defense-in-depth layer behind the type system" — inverted; the test was the primary defence | **Fixed** — sealed `Event` with `PublicEvent`/`SensitiveEvent` marker interfaces; sinks typed so misrouting fails to compile |
| 5 | Major | `domain-events.md` FR-2 | "Every event from `domain-bibliographic.md` ... MUST return `Sensitive() == false`" (`:86-88`) is wrong on the import path. `domain-bibliographic.md:63-65`'s own first user story makes creating a Work and Edition *the acquisition* for an imported EPUB, disclosing exactly what `LibraryEntryAdded` is protected for (`:88-91`). It cannot be fixed by reclassifying, because a Discover-browsed Work genuinely is public catalog data and FR-1 forbids computing sensitivity from the event's data (`:79-80`) | **Fixed** — the defect was one event name covering two different facts. Split into `EditionCreated` (public, catalog) and `EditionImported` (sensitive, acquisition), which restores a fixed per-type classification |
| 6 | Major | Three specs × `go-backend-conventions/SKILL.md:29-36` | `domain-bibliographic.md:132-134`, `domain-library.md:185-187`/`:200` and `domain-reading.md:197-200` place graph-reachability checks at construction time in `internal/domain` — cycle detection, Edition existence, Edition-belongs-to-Work. None is decidable from the value being constructed. **This review initially overstated the constraint** as an architectural impossibility; `SKILL.md` forbids *importing* persistence packages, not calling interfaces the domain declares itself, so it is a design gap | **Fixed** — [ADR 0020](../decisions/0020-graph-invariants-in-domain-services.md) draws the line: single-value invariants stay in constructors, graph invariants move to domain services holding domain-defined repository interfaces |
| 7 | Major | Phase 02 × `backend-persistence.md` FR-2/FR-4 | No Transactor, UnitOfWork or outbox exists in any of the five specs — verified by grep, the words occur twice in total, both incidental. `backend-persistence.md` FR-4 (`:131-144`) forbids composing repositories, while `domain-source.md` FR-6's cascade spans two (`:110`) and `domain-events.md` FR-5's "not zero" is the dual-write problem for every mutation. Compounding it, **`domain-source.md` never declares FR-6 atomic**, so FR-4 has no trigger to engage and a non-atomic cascade violates nothing written | **Fixed** — [ADR 0021](../decisions/0021-transaction-contract-and-event-outbox.md); `backend-persistence.md` FR-2/FR-4 amended to match |
| 8 | Major | `domain-library.md` `:182` vs FR-6 | The state-transition line says removing a `LibraryEntry` is "removed from any Collection it was part of (FR-6)" — citing the FR that forbids it. FR-6 (`:116-118`), the failure-mode row (`:202`), the acceptance criterion (`:241-242`) and the explicitly-legal-state paragraph (`:188-194`) all disagree with it. An implementer reading the state machine builds a cascade that silently loses "want to read" shelf entries | **Fixed** — `:182` corrected; FR-6 was authoritative |
| 9 | Minor | `domain-bibliographic.md` FR-8 | The normative sentence forbids only a merge pointing at itself, while `:206-207` and `:215` both say "cycle" unqualified and FR-9 says "transitively" (`:141`). Implemented literally, `A→B` then `B→A` is legal and FR-4's resolve-through (`:102-104`) has no fixed point — every read of either Work does not terminate | **Fixed** — FR-8 now forbids reachability, not self-reference |
| 10 | Minor | `domain-source.md` FR-2 | Re-observation "updates that row's timestamp" (`:94-95`) and says nothing about the `FileReference`'s opaque identifier. If a source re-catalogues, the row's freshness refreshes while its identifier goes stale — a freshly-observed offering pointing at an identifier that no longer resolves, inverting the spec's own central guarantee | **Open** — recorded in `domain-source.md`'s status cell, scheduled |
| 11 | Minor | `domain-reading.md` `:208` vs `:269-271` | The failure-mode row asserts `ReconcileProgress` "is deterministic" while the open question admits the equal-percentage tie is undecided. A function with an unspecified branch is not deterministic | **Fixed** with finding 1 — the tie is closed by the epoch ordering plus an explicit tiebreak |
| 12 | Minor | `domain-reading.md` FR-2 | The cross-edition fallback is defined for *reading* (`:83-86`) but not for *reconciling*: nothing says what the canonical record's `PrecisePosition` becomes when the winning report has none. Keeping it leaves two fields of one record disagreeing; clearing it loses exact resume permanently because of one report from another device | **Fixed** with finding 1 |
| 13 | Minor | `domain-library.md` FR-7 → `domain-events.md` | FR-7 (`:128-130`) defers the format/cache question to "`domain-events.md`'s reasoning about the design reference's 'offline copies' concept". That reasoning is not in `domain-events.md` — the string does not occur there. It lives in review 0021 (`:39`, `:43`), so the spec is unreadable on that point without the review record | **Open** — scheduled with the remaining `domain-library.md` work |
| 14 | FYI | `domain-reading.md` FR-8 | `Finished` requires `Percentage` exactly 1.0 (`:129-131`). Whether that is reachable depends on how phase 11 computes a fraction over back matter and indices. Risk is a shelf filter where nothing is ever `Finished`, not data corruption | No change — recorded for phase 11 |

## Dimensions checked

- [x] **Completeness** — not the focus; review 0021 covered it. Finding 3 is a completeness gap only incidentally; it is here because both ways of completing it produce a bug.
- [x] **Ambiguity** — findings 3, 8, 9 and 12 are all "two engineers build different things", and finding 3's two readings were traced to their separate consequences.
- [x] **Architecture** — findings 6 and 7. Review 0021 concluded "No domain/metadata/source boundary violations found anywhere" (`:60`), which is true for which types reference which; this pass asked the different question of whether the checks placed inside the boundary can be performed there.
- [x] **Domain correctness** — boundaries intact. Finding 3's consequence crosses bibliographic into library, but by omission rather than by a boundary breach.
- [x] **Security** — findings 4 and 5, both constitution §8. ADR 0021's outbox introduced a new one during this pass (the drain is an unconstrained consumer) and was amended before filing.
- [x] **Testability** — finding 2 is a testability defect specifically: the acceptance criteria could not both pass.
- [ ] **Accessibility** — not applicable to a pure domain model.
- [ ] **UX and copy** — not applicable.
- [x] **Observability** — finding 7's dual-write is what makes FR-5's event guarantee real or hollow.
- [x] **Maintainability** — finding 13.
- [x] **Evolution** — finding 5 is squarely this: FR-1's fixed-per-type rule forecloses expressing context-dependent sensitivity, so the fix had to change the event catalog rather than the classification.
- [x] **Mechanism properties** — the new template line, applied to this pass: findings 1, 3 and 4 each changed a mechanism, and each spec's amendment restates the mechanism's properties and shows they hold.
- [x] **Indexes** — `specs/README.md`, `decisions/README.md` and `roadmap/02-domain/README.md` updated.

## Contradictions and gaps

Recorded above as findings 1, 2, 3, 8, 9, 11 and 13 — every one is a spec
contradicting itself or a sibling, which is why they survived two prior passes
that read each document as internally coherent.

One contradiction was introduced *by this pass* and caught in its own
self-review: ADR 0021 was filed `Accepted` while `backend-persistence.md`
FR-4 (`APPROVED`) still forbade the composition the ADR requires. Fixed by
amending FR-4 and FR-2 the same day, before any other work built on it.

## Disagreements with review 0021

Its fourteen findings are sound and its fixes hold. Four qualifications:

1. **Finding 3's fix introduced finding 1 above.** Splitting `ProgressReport`
   from `ReadingProgress` was right in direction and produced FR-6's
   `(ReadingProgress, ProgressReport)` signature — type-ill-formed against the
   commutativity MUST in the same FR, and contradicting `:174-175`, which the
   pass did not update. A fix to a central mechanism should have re-derived
   that mechanism's properties. This is now the review template's own
   Mechanism-properties checkbox.
2. **Finding 4 was treated as a documentation correction**; it is evidence of a
   lossy operation. Because the superseded report is captured nowhere, a wrong
   furthest-wins outcome is unrecoverable — and the override, the only escape,
   is the operation that did not survive reordering.
3. **Finding 11's resolution left a dangling reference** — see finding 13
   above.
4. **"No domain/metadata/source boundary violations found anywhere" (`:60`)**
   is a correct answer to a narrower question than the sentence implies. See
   finding 6.

## What I did not review

- `architecture-backend.md`, `architecture-persistence.md`,
  `architecture-contracts.md`, `backend-errors-and-logging.md`,
  `backend-service-lifecycle.md`, `backend-configuration.md` — read only
  through `backend-persistence.md`'s quotations of them. Citations of
  `architecture-backend.md` FR-2 are second-hand and UNVERIFIED at first hand.
- Reviews 0016–0020 and 0022. Only 0021, deliberately and last.
- The design-reference canvases. Review 0021's "offline copies" finding is
  taken as given; only the absence of any phase 02 modelling of it was checked.
- Downstream `APPROVED` specs built on these mechanisms. Sized, not reviewed —
  `backend-reading-api.md` alone cites `ReconcileProgress` at thirteen lines
  including an integration test (`:292`) specified to prove the property
  finding 1 shows does not hold. Tracked as a follow-up list, not fixed here.
- Any code. None exists for phase 02.
