# 0003. Design reference is split into per-surface canvases

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-13 |
| **Deciders** | Project owner |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

The design reference (`.design-reference/Alexandryn.dc.html`) was captured as
a single Claude Design canvas. The project's `get_file` API caps a single
file at 256 KiB, and the canvas exceeds that — it cuts off mid-element,
before the trailing `<script data-dc-script>` block that holds all state
logic and mock data. That block missing means the Tablet, Mobile, Remote,
States, and Design system screens have no captured markup, only their nav
labels (see `.design-reference/ANALYSIS.md`). A second `get_file` pull via
the official `claude_design` MCP, made 2026-08-13, returned a byte-identical
truncation — the cap is enforced server-side per file, with no offset or
pagination parameter, so a bigger single canvas cannot be pulled around it.

Separately from the tooling limit, the product itself has two distinct
surfaces that were being modelled as one canvas: the desktop host app (full
local control — sources, import, system, settings) and the web/remote
viewer served to other devices on the LAN (`atRemote`, already named in the
partial capture, plus `atMobile` and `atTablet`). These are not the same
surface with responsive breakpoints; the host has capabilities the LAN
client structurally must not have until Phase 12/13 grants it authentication
(Constitution §6). One canvas was already asking the design tool to describe
two different trust levels as if they were one screen set.

## Decision

The design reference is authored and captured as multiple per-surface
canvases instead of one canvas for the whole product:

1. **Desktop/Host** — `atLibrary`, `atBook`, `atCollections`, `atDiscover`,
   `atSources`, `atSourceDetail`, `atImport`, `atActivity`, `atSettings`,
   `atSystem`, `atFirstRun`. The full-control surface, PC-only.
2. **Web/Remote viewer** — `atRemote`, `atMobile`, `atTablet`. The
   LAN-served, access-only surface: no source configuration, no import, no
   system controls.
3. **States** — shared empty/loading/error/skeleton reference, cross-cutting
   both surfaces.
4. **Design system** — tokens and component reference sheet.

Each canvas is exported as its own `.dc.html` file in the Claude Design
project, each with its own 256 KiB budget.

## Options considered

### Option A — Multiple canvases split by product surface (chosen)

*For* — fixes the size-cap problem as a side effect rather than as the goal:
each file is smaller because it covers less, not because it was cut apart
arbitrarily. It also matches a real structural boundary that the codebase
already has to enforce (host vs. LAN client, Constitution §6), so the
canvas split and the eventual package/route split reinforce each other
instead of drifting apart.

*Against* — more files to keep in sync; shared elements (design tokens,
states) must be authored once and referenced consistently across canvases
rather than copy-pasted per screen, or they will drift.

### Option B — Keep one canvas, request a raw export outside the API

Ask the design tool owner to download the `.dc.html` source directly from
the claude.ai UI, bypassing the capped `get_file` API path entirely.

*For* — no split needed, zero rework of the existing captured screens.

*Against* — doesn't address the surface-boundary problem at all; the Tablet/
Mobile/Remote/States/Design-system screens would still be moulded into the
same file as the host app, which the product doesn't actually want long
term. Also a one-time workaround rather than a durable process — the file
would hit the cap again the moment more screens are added.

### Option C — Keep one canvas, accept the missing screens as a permanent gap

Build only what phases need, when they need it, and never capture Tablet/
Mobile/Remote/States/Design system at all.

*For* — least effort now.

*Against* — Phase 04 (frontend foundation) and Phase 13 (network access)
both need the Remote/Mobile/Tablet surface's visual intent before they can
be built without guessing. Constitution says not to infer missing screens;
this option just defers the same problem to whichever phase needs them.

## Consequences

**Good** — each canvas fits under the API cap without truncation. The split
mirrors the host/LAN-client trust boundary the codebase must enforce
anyway, so Phase 04's frontend architecture and Phase 13's network access
work inherit a design reference that already respects the boundary instead
of fighting it. Design system and States become shared references usable by
both surfaces instead of being duplicated per screen.

**Bad** — four files instead of one to keep current; design tokens and
shared states must be maintained consistently across canvases by whoever
owns the design tool, since nothing here enforces that automatically.

**Neutral** — `.design-reference/ANALYSIS.md` gets rewritten once the new
canvases are captured, since it currently documents the single-canvas
truncation this ADR is replacing as the working method.

## Reversal cost

Low. This is a design-authoring workflow decision, not a code or schema
commitment — recombining the canvases later, or splitting them differently,
costs only the design tool's own editing time. Nothing downstream depends on
the number of files, only on their content once extracted into
`frontend-design-tokens.md` and friends in Phase 04.

## Confidence

High. The surface split (host vs. LAN client) is already a hard constitutional
boundary independent of this ADR; aligning the design canvases to it removes
a mismatch rather than introducing a new judgement call. The only genuinely
open question is whether the specific screen-to-canvas grouping above is
final, which is cheap to revisit given the low reversal cost.
