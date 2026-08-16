# 0007. Production PostgreSQL is bundled and managed by the Go server, never user-configured

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-14 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Review 0007 (on `architecture-desktop-host.md`) found this while checking
the design reference for an unrelated reason: no screen, anywhere in the
captured design, configures a database connection. `atFirstRun`'s "Storage
location" and Settings' "Storage" tab are both file storage (book files on
disk), not PostgreSQL. ADR 0004 decided production runs self-hosted
PostgreSQL but never decided how it reaches an end user's machine — the
Supabase CLI stack (ADR 0004's addendum) is a developer answer.

Drafting `architecture-persistence.md` forced the actual question: does
Alexandryn ship instructions for connecting to a Postgres the user already
has, or does it bring its own? The design reference's silence is itself
evidence — a product aimed at "opens on any device in your home" (README)
does not put a connection-string field in front of someone setting up a
personal library.

## Decision

Alexandryn bundles PostgreSQL and manages its lifecycle itself. The Go
server spawns and owns a PostgreSQL process the same way Electron spawns and
owns the Go server (`architecture-system.md` FR-2, ADR 0005) — on first run,
it initializes a data directory under the app's own data path, starts
`postgres` bound to loopback on an OS-assigned port, waits for it to accept
connections, then proceeds with its own migrations and readiness check. No
user-visible connection string, no setup screen, matching what the design
reference actually shows (nothing).

Pointing Alexandryn at an external, user-managed Postgres instance is
explicitly **not** a v1 goal. It may be worth a future "Advanced" option —
the design reference does have an Advanced settings tab, unconfirmed to be
for this — but deciding that now would be guessing at a requirement nobody
has asked for yet, the same reasoning `architecture-desktop-host.md`
already used to defer tray/background mode.

**Consequence acknowledged, not smoothed over:** this makes PostgreSQL a
*third* long-running process in a running instance. `architecture-system.md`
FR-1 currently says "exactly two." That spec is `APPROVED` — this ADR does
not get to silently make an approved spec wrong. See the companion
amendment recorded in `architecture-persistence.md`'s references and applied
directly to `architecture-system.md`, with its status moved back for
re-review rather than left falsely marked settled.

## Options considered

### Option A — Bundled and managed, invisible to the user (chosen)

*For* — matches the design reference (which shows nothing, implying nothing
to show), matches the product's own framing (a household library app, not
infrastructure software), removes an entire category of support burden
("how do I configure Postgres") that would otherwise land on a project with
one maintainer.

*Against* — a real, not-yet-prototyped packaging problem: bundling
`postgres` binaries per target OS/architecture inside an Electron app,
managing `initdb`, data directory placement, and version upgrades across
app updates. Adds a third process to a system that had just settled on two.
`embedded-postgres`-style tools (real `postgres` binaries, downloaded or
bundled, managed as a subprocess) are an established pattern elsewhere
(commonly used for Go integration tests) — the mechanism is real, but using
it for a production desktop app's actual data, not disposable test fixtures,
is not the same maturity bar. Named here, not verified — same honesty
standard as ADR 0005's macOS/Windows mechanisms.

### Option B — User brings their own Postgres

*For* — no bundling problem, no version-upgrade story to design, fits
self-hosters who already run Postgres for other things (a home server,
a NAS).

*Against* — requires a setup screen the design reference doesn't have,
requires the target user (README: "opens on any device in your home,"
not "requires a home server admin") to already run or stand up Postgres
themselves. Wrong default for this product, though possibly a good
*option* for the subset of users who'd want it — see Option A's deferred
Advanced-tab possibility.

### Option C — Embedded engine instead (reopen ADR 0004)

Use something that genuinely runs in-process (SQLite, or a Postgres-wire-
compatible embeddable engine) instead of managing a separate `postgres`
process at all.

*Against* — reopens ADR 0004, which already weighed this and chose
PostgreSQL specifically for concurrent multi-device access
(`architecture-system.md`'s whole trust-boundary and LAN-serving design
assumes it). Nothing learned here changes that reasoning — the provisioning
problem is about *how Postgres gets there*, not evidence that Postgres was
the wrong engine. Rejected as out of scope for this ADR.

## Consequences

**Good** — a real answer to a real gap, one that matches what the design
already implies rather than inventing a new screen to match an
assumption. No user-facing database configuration to design, document, or
support.

**Bad** — packaging must now bundle `postgres` binaries per platform (real
new scope for phase 99, not investigated here); the process model gains a
third long-running process, forcing an amendment to an `APPROVED` spec;
version-upgrade story (what happens to a user's data directory when
Alexandryn ships a new bundled Postgres major version) is completely
undesigned.

**Neutral** — doesn't change ADR 0004's engine choice, doesn't change who
talks to Postgres (`architecture-system.md` FR-5: the Go server only, still
true — it now also happens to be the process that spawns it).

## Reversal cost

High once phase 03/05 build against it — packaging, data directory layout,
and upgrade tooling would all need to change if this is reversed later.
Cheap right now: nothing has been built.

## Confidence

Medium. High confidence that *some* zero-configuration answer is correct —
the design reference's silence is real evidence, not an assumption. Lower
confidence on *bundled-and-spawned* specifically over other zero-config
shapes (e.g., a background-installed system service instead of a spawned
child) — that comparison wasn't weighed as rigorously as ADR 0005's process
model was, because it wasn't prototyped. Flagged, not hidden.

## Addendum — scoped to the Electron-hosted target (2026-08-16)

ADR 0015 adds a second, container-hosted deployment target whose Postgres
is a separate, sibling container the Go server connects to over the
network — never spawned or owned by it. Read literally, this ADR's Decision
text ("Pointing Alexandryn at an external, user-managed Postgres instance
is explicitly **not** a v1 goal") forbids exactly this. It doesn't, for the
container target, and this addendum states the reasoning rather than
asserting the exemption.

Every reason this ADR actually gives for rejecting Option B ("User brings
their own Postgres") is scoped to the Electron-hosted product's default
experience specifically: *"requires a setup screen the design reference
doesn't have, requires the target user... to already run or stand up
Postgres themselves. Wrong default for this product."* None of that
reasoning argues an externally managed Postgres is unsafe or unworkable in
general — only that it's the wrong default for someone who wants a
double-click desktop app with no setup screen. This ADR itself named a
"future 'Advanced' option" as plausible, deliberately undecided for lack of
demand: *"deciding that now would be guessing at a requirement nobody has
asked for yet."* The container target isn't that Advanced-tab option
specifically — it's a separate deployment surface with no Electron and no
settings UI to gate anything — but it's the same underlying shape
(operator-supplied Postgres, not spawned by the app), now with the demand
this ADR said was missing: the maintainer asked for it directly, for that
target, when ADR 0015 was drafted.

This is an addendum, not a supersession, because the finding this ADR
actually records — the Electron-hosted target bundles and spawns its own
Postgres, invisible to the user, no setup screen — is unchanged and still
the right default for that target. What's added is a second target where
the opposite is the right default. As with ADR 0005's parallel addendum:
if "explicitly not a v1 goal" was meant as a permanent, general
prohibition rather than a statement about this product's *default*, this
addendum is the wrong instrument and ADR 0015 should have superseded this
ADR instead. Not resolved unilaterally here — named in ADR 0015 as an open
question about original intent.
