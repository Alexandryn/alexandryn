# Review: Phase 02 domain model — cross-spec, two independent agents

| | |
|---|---|
| **Subject** | All five Phase 02 specs (`domain-bibliographic.md`, `domain-library.md`, `domain-source.md`, `domain-reading.md`, `domain-events.md`) and ADRs 0009, 0010, reviewed together |
| **Reviewer** | Two independent subagents (fresh context each, no coordination) — genuinely independent, not self-review |
| **Date** | 2026-08-14 |
| **Verdict** | Needs rework at review time (Blocking findings present); addressed in this pass |

## Summary

Both reviewers agreed the core modelling decisions hold up under cross-spec
scrutiny — internal-ID-primary identity, Edition-level library membership
vs. Work-level progress, availability-as-observation — and neither found
the *approach* to be wrong anywhere. What a same-author pass structurally
missed: three real Blocking-level ambiguities/contradictions, a
concentrated cluster of gaps in `domain-events.md`'s event catalog, and —
the most consequential single finding — phase 02's own exit criterion
requiring every design-reference entity be mapped or explicitly excluded
was never actually performed. Checking it directly surfaced a genuine
unmodelled concept: client-side "offline copies" (`Alexandryn-Web.dc.html`:
`b.offline`, `offlineCount`, `offlineSize` — "7 books cached in this
browser · 86 MB of 220 MB"), present across three of four canvases.

One claim from Review A was checked and found incorrect: CLAUDE.md's
design-reference section is not stale — it was already fixed earlier this
session. Disregarded, not applied.

## Findings

| # | Severity | Area | Finding | Resolution |
|---|---|---|---|---|
| 1 | Blocking | `domain-bibliographic.md` FR-2 | Self-contradicts on whether ISBN is part of "external references" (FR-2's prose) or a separate field (Domain model section) — two engineers would build different types | **Fixed** — ISBN is inherent edition data with its own format validation, explicitly *not* an "external reference" in ADR 0010's sense |
| 2 | Blocking | `domain-events.md` FR-2 | `CollectionCreated` classified non-sensitive, but a collection's user-chosen name can itself reveal sensitive interest with zero members — the same "owns is reads" reasoning FR-2 already applies to membership events was not applied to the name | **Fixed** — `CollectionCreated` reclassified `Sensitive() == true` |
| 3 | Blocking | `domain-reading.md` FR-1/FR-2 × ADR 0009 | `ReadingProgress`'s shape (carrying both `DeviceID` and `observed-at`) is ambiguous between "one singleton row per Work, continuously reconciled" and "one row per (Work, Device), reconciled only at read time" — the second reading silently reproduces the exact stale-device-wins bug ADR 0009 exists to prevent, without violating any FR's literal text | **Fixed** — explicit singleton-per-Work requirement added, with `DeviceID`/`observed-at` reinterpreted as provenance of the last contributing report, distinct from an ephemeral incoming report fed to `ReconcileProgress` |
| 4 | Major | `domain-reading.md` vs `domain-events.md` | `domain-reading.md`'s Failure modes claims a losing report in a conflict is "auditable via events," but `domain-events.md` FR-5 requires exactly one event per reconciliation — the superseded report's value is never captured by any event. Direct contradiction between two documents | **Fixed** — `domain-reading.md`'s claim corrected; superseded reports are not currently auditable via the event stream, stated plainly rather than overclaimed |
| 5 | Major | `domain-events.md` — catalog completeness | Missing `AuthorMergeUndone` (asymmetric with `WorkMergeUndone`, despite `domain-bibliographic.md` FR-7 requiring the same reversibility); missing an event for FR-9's omnibus "contains" mutation; missing `EditionCreated`/`AuthorCreated` (only `WorkCreated` exists, reproducing the exact Work/Edition asymmetry phase 02's own risk table warns against); missing a `SourceOffering` removal event despite `domain-source.md` FR-6's cascade-delete requirement | **Fixed** — all five added to the non-sensitive event list |
| 6 | Major | `domain-events.md` — provenance framing | Claims events are "defined by" the four sibling specs; none of them actually name a single concrete event type — every name is `domain-events.md`'s own invention | **Fixed** — reworded to state plainly that this spec is the sole author of the event catalog |
| 7 | Major | Exit criterion — design-reference mapping | Phase 02's exit criterion ("every entity in the design prototype's data bindings maps to something in the model, or is explicitly recorded as presentation-only") was never actually performed by any of the five specs — checking it directly surfaced a real gap: client-side "offline copies" (`b.offline`, `offlineCount`, `offlineSize`), present in the Web, Electron-Admin, and Mobile canvases, has no domain concept anywhere | **Addressed** — reasoned explicitly rather than modelled: this is judged a frontend/client-storage concern (Service Worker + browser cache), not a backend domain concept — the Go domain doesn't need to track which devices have cached which books locally, since `SourceOffering`/`FileReference` (`domain-source.md`) already provide what a client needs to fetch and cache on its own. Recorded as an explicit non-goal with reasoning, not a silent gap. A brief pass across all four canvases for other major unmodelled concepts found nothing else load-bearing — "duplicates skipped" language ties correctly to phase 10's already-scoped import matching |
| 8 | Minor | `domain-reading.md` | `PrecisePosition`'s `Edition` isn't required to belong to the same `Work` the `ReadingProgress` attaches to — representable as written, unlike `domain-bibliographic.md` FR-8's construction-time invariants | **Fixed** — cross-reference invariant added |
| 9 | Minor | `domain-library.md` FR-7 | Bundles three unrelated requirements (uniqueness, empty-collection legality, membership timestamp) under one FR number, weakening FR-to-test traceability | **Fixed** — split into FR-7/FR-8/FR-9 |
| 10 | Minor | `domain-library.md` | No FR addresses deleting a `Collection` itself, and it isn't in Non-goals either — reads as an unstated gap | **Fixed** — added |
| 11 | Minor | `domain-library.md` / `domain-source.md` | Format-level ownership ambiguity: `LibraryEntry` uniqueness is one-per-`Edition` regardless of format, but `SourceOffering` tracks format separately — no way to represent "I have both the EPUB and the PDF" | **Addressed** — resolved by finding 7's reasoning: which format was actually fetched/cached is a client-side concern, not `LibraryEntry`'s job; `LibraryEntry` stays Edition-level, format selection happens at fetch/cache time |
| 12 | Minor | `domain-events.md` FR-4 | Uses "the same user's own data" — no `User` entity exists in this phase's single-instance model | **Fixed** — reworded to "the instance's own data" |
| 13 | Nit | `domain-bibliographic.md` | FR-10 calls the field `Work.language`; Domain model section calls it "original language" — same field, two names | **Fixed** — named consistently |
| 14 | Nit | `domain-bibliographic.md` | FR-9 (containment) and FR-4/FR-8 (merge cycles) validate two separate graphs; their interaction (a contained Work later merged into its container) isn't addressed | **Fixed** — recorded as an explicit open question |

## What checked out (both reviewers agreed)

- Internal-ID-primary identity (ADR 0010) applied consistently everywhere
  an entity is referenced across all five specs
- Edition-level library membership vs. Work-level progress: a deliberate,
  correctly-justified difference, not an inconsistency
- Availability-as-observation (`SourceOffering`) consistent against every
  sibling spec that touches it
- `LibraryEntry`'s independence from `Source` (cited in both directions)
  accurate
- ADRs 0009 and 0010 correctly use full new-ADR treatment (not the
  Addendum convention) since neither extends a pre-existing decision
- No domain/metadata/source boundary violations found anywhere

## Resolution

All fourteen findings addressed in this pass — twelve fixed as stated
requirement changes, two (7, 11) resolved by explicit reasoning rather
than new domain modelling, judged the better fit for what these actually
are (client-side concerns, not backend domain gaps). Detailed edits in the
same commit as this record.
