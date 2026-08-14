# 0005. The Go server runs as a spawned child process, never embedded into one binary

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-13 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

`architecture-system.md` (spec) asserted FR-1/FR-2 — Electron spawns the Go
server as a child process, two OS processes, never a single binary — without
the prototype phase 01's own risk table required first: *"Electron and Go
process model chosen for elegance rather than packaging reality \| Medium \|
High — reversal is expensive \| Prototype the packaging path before
deciding."* The self-review of that spec
([`reviews/0004-spec-architecture-system.md`](../reviews/0004-spec-architecture-system.md))
flagged this as Blocking. This ADR is that prototype's result, and the
decision it now actually supports.

A second, related question needed an answer alongside the first: if the Go
server is a spawned child, what stops it from surviving as an orphan if
Electron's main process crashes (`SIGKILL`, segfault — anything that skips
Electron's own cleanup code)? `architecture-system.md`'s FR-10 required this
but left the mechanism as an open question.

## Decision

Two OS processes: the Electron application (main + renderer) and a
separately-compiled Go binary, bundled inside the Electron app package and
spawned as a child process by Electron's main process on startup. Not a
single binary, not a Go library embedded via cgo, not a system service
launched independently of the app.

Orphan prevention: the Go binary sets `prctl(PR_SET_PDEATHSIG)` on Linux —
verified in the prototype to reliably terminate the child within the
kernel's signal-delivery time after the parent is `SIGKILL`ed, no polling
delay, no dependency (stdlib `syscall` package only). macOS and Windows need
different, platform-native mechanisms — named below, not yet built or
tested, remaining open for phase 05.

## Options considered

### Option A — Two processes, Electron spawns the Go binary as a child (chosen)

*For* — matches the transport model `architecture-system.md` already
specified (HTTP over loopback between renderer and Go server, so nothing is
saved by collapsing the process boundary); each runtime keeps its own normal
toolchain and test suite; the prototype
([`0005-process-model-prototype/`](0005-process-model-prototype/)) empirically
confirms spawn, health check, graceful shutdown, and — with the pdeathsig fix
— orphan prevention, all work on Linux with off-the-shelf mechanisms.

*Against* — packaging must bundle a compiled Go binary per target OS/architecture
inside the Electron artifact (more build-matrix surface than a pure-JS
Electron app); orphan prevention needs three different platform-specific
mechanisms instead of one, and two of the three are unverified here.

### Option B — Single binary, Go embedded via cgo/native addon

Rejected without a separate prototype: cgo cross-compilation is fragile and
well-documented as a packaging headache (disables trivial cross-compiling,
requires a C toolchain per target platform), and it doesn't remove the
orphan-prevention problem — an in-process crash still needs the same care an
out-of-process crash does. The presumed benefit (avoiding IPC) doesn't
materialize, since `architecture-system.md` already committed to an HTTP
contract between renderer and server regardless of process boundary.

### Option C — Go server as an independently launched background service (systemd/launchd unit), Electron as just a client

Rejected: a system-service install step is heavier and more invasive than a
normal desktop app install, and it reopens a question `architecture-system.md`
deliberately declined to answer for v1 — whether the server should keep
serving LAN clients with no desktop app running. That's a real product
question, but not one this ADR is deciding by accident via a packaging choice.

## Consequences

**Good** — the process-model half of this decision is now backed by
something measured, not just argued. The Linux orphan-prevention mechanism
is three lines of stdlib, no new dependency to justify under constitution §9.

**Bad** — macOS and Windows orphan prevention remain unverified; phase 05
inherits a concrete plan (kqueue `EVFILT_PROC`/`NOTE_EXIT` on macOS, a Job
Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` on Windows) but not proof.
Packaging (phase 99) now has a real requirement to bundle and codesign a
second binary alongside the Electron app on every target platform — not
investigated here at all.

**Neutral** — doesn't change `architecture-system.md`'s FR-3 through FR-9;
this ADR resolves FR-1/FR-2 (with evidence) and half of FR-10 (Linux, with
evidence; macOS/Windows, named but not evidenced).

## Reversal cost

Medium. The HTTP contract between renderer and Go server stays valid
regardless of process boundary, so this isn't a full rewrite if reversed —
but phase 03/05 code built against child-process spawn/lifecycle semantics
would need re-plumbing, and every platform's orphan-prevention code would be
thrown out. Cheaper to reverse now than after phase 05 ships it, same as
`architecture-system.md`'s own reversal-cost note already said.

## Confidence

High on "two processes, not embedded, Linux orphan-prevention via pdeathsig"
— measured, not reasoned. Medium on the overall orphan-prevention plan,
because two of its three platform-specific legs (macOS, Windows) are
well-known patterns asserted here, not verified. Constitution §12: that gap
is a guess, recorded as one, owned by phase 05.

## Addendum — process count extended (2026-08-14)

This ADR's "two processes" finding is still correct as far as it goes —
Electron spawns the Go server as a child, never one combined binary, exactly
as decided here. It's no longer the complete picture: ADR 0007 (production
PostgreSQL is bundled and managed by the Go server) adds a third process the
Go server itself spawns and owns, a fourth on macOS specifically
(`architecture-persistence.md` FR-10, a supervisor process for Postgres
orphan-prevention where no parent-side mechanism exists).  Not a
supersession — the pdeathsig mechanism and the one-binary rejection this ADR
records are unchanged and still apply at the Electron↔Go-server level. See
ADR 0007 for the extension.
