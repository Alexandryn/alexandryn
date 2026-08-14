# Review: architecture-system.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-system.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-13 |
| **Verdict** | Needs rework |

## Summary

The transport model, trust-boundary mapping, and lifecycle states are solid
and internally consistent with the constitution and ADR 0004. One finding is
Blocking: the spec asserts the one-binary-vs-two-processes decision (FR-1,
FR-2) as settled, but phase 01's own risk table requires prototyping the
packaging path *before* deciding this, specifically because it's expensive to
reverse. No prototype happened. Two more findings matter: a scope gap
(messaging/async conditions, which phase 01's scope assigns to some spec, and
this is the only candidate) and a structural one (two MUST-level requirements
live only in the failure-mode table, not as numbered FRs, undermining the
spec's own traceability claim).

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Blocking | Process | `01-architecture/README.md`'s risk table: *"Electron and Go process model chosen for elegance rather than packaging reality \| Medium \| High — reversal is expensive \| Prototype the packaging path before deciding; record it in the ADR."* FR-1/FR-2 decide this with reasoning only, no prototype attempted | Either: (a) actually prototype packaging a Go binary as an Electron-spawned child on at least macOS+Windows before FR-1/FR-2 stay MUST-level, or (b) explicitly override the roadmap's own risk mitigation with the maintainer's sign-off recorded here, not silently |
| 2 | Major | Scope | Phase 01's scope lists "Messaging architecture — the *conditions* under which work becomes asynchronous" as in-scope, but no spec in the phase's own Specifications table owns it, and this spec (the most system-level one) doesn't mention it at all — not in scope, not in non-goals | Add an explicit non-goal line naming where this lives (a new spec, or a section added to this one), so it isn't silently dropped |
| 3 | Major | Traceability | Two MUST-level requirements exist only inside the Failure modes table, never promoted to numbered FRs: Go server must exit if Electron crashes, and concurrent-instance handling must not silently corrupt state. Acceptance criteria claims "every functional requirement maps to an exit criterion" — these two aren't functional requirements at all right now, so nothing will trace them | Promote both to numbered FRs (or explicitly note why a table-only requirement is intentional, which isn't obviously true here) |
| 4 | Minor | Completeness | FR-9 (shutdown ordering) has no time budget for "let in-flight requests complete" before the later-resort hard kill — "never a hard kill as the first resort" implies a hard kill exists as *some* resort, with no bound given | Either specify a budget here or explicitly hand the number to phase 03, same pattern as the startup-budget placeholder already does |
| 5 | Minor | Security | Control-plane channel (main → Go server) needs to pass configuration, and eventually credentials once phase 12 exists, from Electron to the spawned process. Passing secrets via argv is visible in `ps` to any local user; env vars are visible via `/proc/PID/environ` to same-user processes. Neither is addressed | Add a requirement or explicit open question — likely belongs to `architecture-desktop-host.md` or phase 03's `backend-configuration.md`, but this spec defines the channel and should at least flag it, which it currently doesn't |
| 6 | Nit | Completeness | Non-goals doesn't list `architecture-testing.md` (CI/test tooling layer) even though phase 01's scope assigns it there and every other sibling spec gets a non-goal line | Add the line for consistency |

## Dimensions checked

- [x] **Completeness** — findings 2, 5, 6 are real unstated-requirement gaps
- [x] **Ambiguity** — FR-1 through FR-9 are individually unambiguous; the walkthrough section briefly reads as if LAN access works pre-phase-13 before the hostile-walkthrough bullet clarifies it isn't — worth tightening but not a blocking ambiguity
- [x] **Architecture** — consistent with the host/LAN-client boundary (ADR 0003), consistent with ADR 0004's "Go server is the only PostgreSQL client"
- [ ] **Domain correctness** — not applicable; spec correctly scopes itself out of the metadata/source/library boundary
- [x] **Security** — three trust boundaries correctly mapped onto concrete processes; finding 5 is the one gap
- [x] **Testability** — FR-1 through FR-9 are each testable as written; the two table-only requirements (finding 3) are not currently traceable, which is itself the finding
- [ ] **Accessibility** — not applicable at this layer
- [x] **UX and copy** — the failure-mode user-facing strings follow constitution §11 (plain, specific, no apology)
- [x] **Observability** — correlation-ID crossing the Electron/Go boundary is named, appropriately deferred to phase 03 for the full contract
- [x] **Maintainability** — Non-goals section is genuinely load-bearing, names the owning spec for each excluded concern (finding 2 is the one place it doesn't)
- [x] **Evolution** — Open questions section is honest about what's a placeholder (startup budget) vs. what's a real open mechanism (orphan handling, second-instance)

## Contradictions and gaps

Finding 1 is the substantive one: this spec's Acceptance criteria section
says the one-binary-vs-two-processes ADR only needs to "record why,
referencing this spec" — but that's circular if the spec's own "why" skipped
the prototyping step the roadmap demanded. The ADR can't retroactively supply
rigor the spec didn't do.

No contradiction found against the constitution, ADR 0004, or CLAUDE.md.

## Resolution (2026-08-13)

Findings 2–6 fixed in the same commit as this update, at the maintainer's
direction. Finding 1 (Blocking) held open deliberately — it needs the
maintainer's call (prototype, or explicit override) before the spec's
FR-1/FR-2 can be considered settled, and nothing below fixes or works around
that.

- **#2** — non-goal line added naming the gap; open question added
  recommending phase 01's Specifications table get a decision (new spec vs.
  fold into `architecture-backend.md`)
- **#3** — promoted to FR-10 (Go server must exit if Electron does) and
  FR-11 (second-instance handling); failure-mode table rows now reference
  them
- **#4** — FR-9 now states a bounded grace period (placeholder: 10s,
  unmeasured), tracked in Open questions the same way the startup budget is
- **#5** — new Security considerations bullet on the config/secrets spawn
  channel, with an Open questions entry naming the owning spec
- **#6** — non-goal line added for `architecture-testing.md`

**Still open: Finding 1.** FR-1/FR-2 unchanged.

## What I did not review

Whether FR-1/FR-2's conclusion (two processes) is actually *wrong* — I don't
have a packaging prototype to check it against either, which is exactly
finding 1's point. I also didn't independently verify the `/proc/PID/environ`
claim in finding 5 against Windows/macOS equivalents; the general point
(argv and env vars are both weaker than a restricted-permission file or OS
keychain) holds across platforms even if the exact leak mechanism differs.
