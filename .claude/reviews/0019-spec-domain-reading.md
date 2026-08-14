# Review: domain-reading.md, ADR 0009, ADR 0010

| | |
|---|---|
| **Subject** | `.claude/specs/domain-reading.md`, `.claude/decisions/0009-progress-attachment.md`, `.claude/decisions/0010-bibliographic-identity-strategy.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (all three findings fixed — see Resolution below) |

## Summary

ADR 0009's Work-level attachment with a `Percentage` primary and
Edition-tagged precise-position fallback directly answers phase 02's own
"not obvious" question, with real reasoning for why the common case
(re-reading the same edition) shouldn't pay for the rare case (switching
editions) or vice versa. ADR 0010 formalizes what `domain-bibliographic.md`
already implemented, satisfying phase 02's exit criterion that it exist as
its own record. One real gap in `domain-reading.md`: `ReconcileProgress`
(FR-6) is described operationally but never required to have the
mathematical properties phase 14's actual multi-device sync will need —
commutative and associative, so reconciling N devices' reports pairwise in
any order produces the same result. Without that stated, a future
implementation could produce different outcomes depending on message
arrival order, which is exactly the kind of bug that's invisible until two
devices sync in an unlucky sequence.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Missing invariant | `ReconcileProgress` (FR-6) is specified as taking two reports and producing one result, but never states it must be commutative and associative — properties phase 14's actual N-device sync depends on to be correct regardless of message ordering. "Furthest wins" happens to have these properties by construction (it's a max function), but the spec doesn't say so, so a future implementer isn't told this is a requirement, just an incidental fact about the current default | Add explicit requirement: `ReconcileProgress` MUST be commutative and associative, so applying it pairwise across any number of reports in any order yields the same canonical result |
| 2 | Minor | Completeness | No derived reading-status (not-started / in-progress / finished) — a near-free computed property from `Percentage` (0.0 → not started, 1.0 → finished, else in-progress) that's a common, low-cost feature for shelving/filtering. Not named in phase 02's scope, so not clearly required, but cheap enough that leaving it out silently is worth a deliberate call, not an oversight | Add as a computed (never stored) property, same pattern as `domain-library.md` FR-2's "in library" — or explicitly non-goal it if UI-level derivation is preferred instead |
| 3 | Nit | Completeness | `ReadingPreferences` (FR-5) is per-device with no stated behavior for a brand-new device — starting from system defaults vs. inheriting another device's settings is implied but never stated as a deliberate choice | State explicitly: a new `DeviceID` starts with system defaults, no cross-device inheritance — consistent with FR-5's "each device simply keeps its own" |

## Dimensions checked

- [x] **Completeness** — finding 1 is a real gap with real downstream consequences (phase 14 correctness); finding 2 is minor
- [x] **Ambiguity** — FR-1 through FR-7 individually clear
- [x] **Architecture** — correctly keeps device identity/sync transport out (Non-goals), correctly references `Work`/`Edition` by ID only
- [x] **Domain correctness** — ADR 0009's edition-switch reasoning is sound and directly grounded in an already-decided constraint (`domain-library.md` allows multiple owned editions per Work), not invented in isolation
- [x] **Security** — constitution §8 ("never log what someone reads") correctly elevated to this spec's own load-bearing requirement, not just cited in passing; hostile-input bar applied consistently to highlight/bookmark text
- [ ] **Accessibility** — not applicable to the domain layer itself; correctly deferred to phase 11
- [x] **Testability** — FR-6/FR-7's conflict resolution has a concrete test strategy (property-based, adversarial sequences); finding 1's gap is exactly the kind of property a test could catch once stated
- [x] **Maintainability** — the deliberate Edition-vs-Work attachment difference between `Bookmark`/`Highlight` and `ReadingProgress` is explicitly explained (References section) rather than left as something a future reader has to reverse-engineer
- [x] **Evolution** — ADR 0009's reversal-cost section is honest that this gets expensive fast once phase 03/14 build against it — correctly weighted given how central this decision is

## Contradictions and gaps

Finding 1 is the substantive one. Both ADRs hold up under scrutiny — ADR
0010 in particular is a case of formalizing an already-correct decision
rather than making a new one, and no inconsistency was found between it
and `domain-bibliographic.md`'s actual FRs.

## Resolution (2026-08-14)

- **#1** — FR-6 now requires `ReconcileProgress` be commutative and
  associative, with the correctness reasoning (order-independent
  multi-device sync) stated directly
- **#2** — new FR-8: computed (never stored) reading status from
  `Percentage`, same pattern as `domain-library.md` FR-2
- **#3** — FR-5 now states a new device starts from system defaults, no
  cross-device inheritance

## What I did not review

Whether `ReadingPreferences` being purely per-device (FR-5, no
cross-device default/inheritance) is the right call for a user setting up
a *new* device — starting with system defaults rather than "copy my
phone's settings" is implied but not stated as a deliberate choice versus
an oversight.
