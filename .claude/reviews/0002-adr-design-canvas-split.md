# Review: ADR 0003 — Design reference split into per-surface canvases

| | |
|---|---|
| **Subject** | `.claude/decisions/0003-design-canvas-split.md` |
| **Reviewer** | Luann Moreira |
| **Date** | 2026-08-13 |
| **Verdict** | Approved |

## Summary

Backfilled review — same gap as 0001: marked `Accepted` with nothing recorded
here. Content-wise the decision is well-grounded: it solves the 256 KiB
capture-tool limit as a side effect of aligning the canvas split to a trust
boundary the codebase must enforce anyway (host vs. LAN client, constitution
§6), rather than splitting arbitrarily to dodge a size cap.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Minor | Follow-through | The "Neutral" consequence says `ANALYSIS.md` "gets rewritten once the new canvases are captured" — that rewrite isn't tracked as a checkable item anywhere (not in phase 00's exit criteria, not as a TODO) | Either add it to phase 00's exit criteria alongside "Complete design export obtained," or explicitly note it's covered by that existing line |

## Dimensions checked

- [x] **Completeness** — context (tooling cap + trust-boundary mismatch), decision, three options, consequences, reversal, confidence
- [x] **Ambiguity** — the four-canvas split and what belongs in each is spelled out by screen name
- [x] **Architecture** — explicitly aligns the design-authoring split to the host/LAN-client boundary constitution §6 already requires; doesn't invent a new boundary
- [ ] **Domain correctness** — not applicable (metadata/source/library boundary untouched)
- [x] **Security** — doesn't move or weaken a trust boundary; correctly treats "host has capabilities the LAN client must not have" as a constraint on the design work, not just the eventual code
- [ ] **Testability** — not applicable; design-authoring workflow, not behavior
- [ ] **Accessibility** — not applicable at this decision's level
- [x] **UX and copy** — clear
- [ ] **Observability** — not applicable
- [x] **Maintainability** — "Bad" section is honest that four files can drift without something enforcing consistency; no such enforcement exists yet, correctly left as a known cost rather than papered over
- [x] **Evolution** — reversal cost correctly rated low, with the actual boundary (finished screens' content) named as what would survive a re-split

## Contradictions and gaps

None against the constitution. Cross-references CLAUDE.md's note on the
design reference being truncated and incomplete — consistent, not
contradictory; this ADR is what explains *why* the capture is split going
forward, CLAUDE.md's note is about the *current* incomplete state.

## What I did not review

Whether the specific screen-to-canvas grouping (which screens go in "Desktop/
Host" vs "Web/Remote viewer") is the right cut — the ADR itself flags this as
the one open, cheaply-revisited question, and I have no independent basis to
second-guess a product/design call.
