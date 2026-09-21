# 0015. A second deployment target ships the backend as a container, composed with Postgres, alongside the Electron-hosted target

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-16 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

<!-- Status: Proposed | Accepted | Rejected | Superseded | Deprecated -->

## Context

Every process-model decision made so far — ADR 0005 (two processes, Electron
spawns the Go server as a child, never a system service), ADR 0007 (the Go
server bundles and spawns PostgreSQL itself, "never user-configured"),
`architecture-system.md` FR-1/FR-2 (a running instance MUST consist of
exactly three processes, four on macOS, all descended from Electron) — was
written against a single target: a desktop app, for one household, that a
user double-clicks. That target is real and stays real. It is not the only
one intended.

The maintainer confirmed directly, via the question this ADR's drafting
raised (recorded verbatim: *"Which topology do you mean by 'backend
orchestrated as microservices'?" → "(b) Containerized monolith"*), that
Alexandryn is also meant to run as a **containerized backend, composed with
PostgreSQL via Docker Compose, for local development and for deployment** —
a second target with no Electron process at all, most plausibly a headless
install on a home server or NAS, reachable the same way any self-hosted
Compose service is: `docker compose up`, not an installer. This has been the
intent throughout, but it is absent from all 34 approved or reviewed specs
and from every ADR that touches process topology.

Two readings of "microservices" were on the table while investigating this
gap: (a) multiple independently-deployable backend services with their own
network boundaries, or (b) one backend service, containerized, composed with
Postgres. The maintainer confirmed (b). This ADR decides only (b). It does
not introduce a second backend service, a message broker, or an internal
service-to-service API — those remain out of scope, consistent with ADR
0014's already-made choice (Postgres-backed job queue, explicitly not a
broker) and with nothing else in the spec set ever proposing a distributed
backend.

## Relationship to ADR 0005 and ADR 0007

Both ADRs considered and rejected an option this ADR now adopts. That
deserves a direct accounting, not a wave at "scope." Two readings compete
for each, and this section states both rather than picking the convenient
one.

### ADR 0005 — Option C ("Go server as an independently launched background service... Electron as just a client")

**What was rejected.** A single-machine desktop install where the Go server
runs as a systemd/launchd unit rather than an Electron-spawned child, with
Electron reduced to a thin client of it — still one machine, still an
Electron installation, just a different startup mechanism for the same
target.

**Why it was rejected then** (`0005-process-model.md:71-77`, quoted in
full): *"a system-service install step is heavier and more invasive than a
normal desktop app install, and it reopens a question `architecture-
system.md` deliberately declined to answer for v1 — whether the server
should keep serving LAN clients with no desktop app running. That's a real
product question, but not one this ADR is deciding by accident via a
packaging choice."*

**What changed.** The first reason — a system-service install step being
heavier than Electron-spawn — hasn't changed, and doesn't apply to what this
ADR decides: the container target isn't a systemd unit installed alongside
Electron on the same desktop machine, it's a separate deployment with no
Electron anywhere. The second reason is the load-bearing one: ADR 0005
explicitly named "should the server keep serving LAN clients with no desktop
app running" as a real, undecided product question and declined to answer
it *by accident, via a packaging choice*. That question has now been
answered directly, on purpose, by the maintainer, in response to this ADR —
not by accident and not via a packaging choice.

**Why it is correct now — with the honest caveat.** Reading ADR 0005's
Decision section alone ("not a system service launched independently of the
app") states an absolute. Reading its Options-considered section shows that
absolute was scoped to a specific alternative install mechanism for the
*same* target, with the adjacent, broader question — no Electron at all —
carved out and named as deliberately unanswered rather than answered "no."
Under that second reading, this ADR isn't reviving Option C; it's answering
the question ADR 0005 explicitly left for someone else to decide, in a shape
(a wholly separate deployment, not a systemd unit next to a still-present
Electron install) that ADR 0005 never actually proposed or rejected. **The
caveat:** this is my reading of what the hedge language meant, not a fact
independently verifiable from the text. A reader could just as reasonably
conclude the Decision section's "never... a system service" was meant as a
permanent, general rule, and that the hedge was only "we're not deciding the
*product* question here," not "the *mechanism* question stays open too." If
that's what you meant in August, this is a reversal of ADR 0005, not an
extension of it, and needs to be written and reviewed as one. I can't
resolve that from the document alone — it's your call, not mine.

### ADR 0007 — Option B ("User brings their own Postgres")

**What was rejected.** Alexandryn's Electron-hosted product pointing at a
Postgres instance the user already runs and manages themselves, instead of
Alexandryn bundling and owning one.

**Why it was rejected then** (`0007-postgres-provisioning.md:75-86`, quoted
in full): *"requires a setup screen the design reference doesn't have,
requires the target user (README: 'opens on any device in your home,' not
'requires a home server admin') to already run or stand up Postgres
themselves. Wrong default for this product, though possibly a good *option*
for the subset of users who'd want it — see Option A's deferred Advanced-tab
possibility."* The Decision section itself is more absolute: *"Pointing
Alexandryn at an external, user-managed Postgres instance is explicitly
**not** a v1 goal... deciding that now would be guessing at a requirement
nobody has asked for yet."*

**What changed.** "Nobody has asked for it yet" is no longer true — the
maintainer has now asked for it, explicitly, for a distinct deployment
target. Every reason actually given for rejecting Option B is scoped to
*this product's default, for the household user described in the design
reference* — none of it argues that an externally-managed Postgres is unsafe
or unworkable in general, only that it's the wrong **default** for someone
who wants a double-click desktop app with no setup screen.

**Why it is correct now — with the same honest caveat.** ADR 0007 itself
named a "future 'Advanced' option" as plausible, deliberately undecided for
lack of demand, not rejected on the merits. The container target isn't
literally that Advanced-tab option — it's a separate deployment surface
entirely, with no Electron and no settings UI to gate it — but it's the same
underlying shape (Postgres as something the operator points the backend at,
not something the backend spawns), now with the demand ADR 0007 said it was
missing. **The caveat, stated as plainly as ADR 0005's above:** the Decision
section's "explicitly not a v1 goal" is unambiguous on its face, and a
reader could reasonably hold that this ADR reverses it rather than extends
it into a scope ADR 0007 didn't cover. I'm choosing the extension reading
because the *reasons given* are all scoped to the Electron target's default
UX, not to Postgres provisioning in general — but that's an inference from
the ADR's own stated reasoning, not a fact the document settles for me.

**If either reading is wrong:** the fix is not to quietly rewrite this
section. It's to change the instrument in the amendment plan
(`.claude/audits/0002-topology-gap.md`, A-02-03/A-02-04) from an addendum to
an explicit supersession of the affected ADR, and to say so in this ADR's
own header (`Supersedes:` field) once you've confirmed which reading is
correct.

## Decision

Alexandryn ships **two deployment targets for the same Go binary**, not two
different backends:

1. **Electron-hosted** (the existing, fully-specified target): Electron
   spawns the Go server as a child process; the Go server spawns and owns a
   bundled PostgreSQL instance. ADR 0005 and ADR 0007 continue to describe
   this target exactly as written. Nothing about it changes.
2. **Container-hosted** (new): the Go server (`cmd/server`, the same binary,
   same `go:embed`-baked frontend, same everything ADR 0008 already
   decided) runs as the single process in a container, built from a
   `Dockerfile` at the repository root. PostgreSQL runs as a **separate,
   sibling container**, not spawned or owned by the Go server — a standard
   `docker-compose.yml` at the repository root defines both services, a
   named volume for Postgres's data directory, and startup ordering via
   Compose's own `depends_on: condition: service_healthy` against
   PostgreSQL's built-in healthcheck. The Go server reaches Postgres over
   the Compose network via `DATABASE_URL`, exactly the code path
   `backend-persistence.md` FR-5 already implements for its "developer/CI/
   test" case — that path is retargeted as this target's **normal
   production path**, not left as a dev-only escape hatch.

**Database connection: one config surface, a Compose profile gating the
bundled instance.** The container target does not introduce a second,
split configuration surface (`POSTGRES_HOST`/`POSTGRES_PORT`/`POSTGRES_USER`
as separate application-level keys). The application reads exactly one
value, `DATABASE_URL` — the same key `backend-configuration.md` FR-4
already defines — regardless of what it points at. What changes is where
that value comes from and what it defaults to:

- The repository-root `docker-compose.yml` defines a `postgres` service
  tagged with Compose's `profiles: ["bundled-db"]`, initialized with
  `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB` values that themselves
  default to a fixed, reasonable value (e.g. `alexandryn`/`alexandryn`/
  `alexandryn`) if the operator sets nothing. The `backend` service's own
  `DATABASE_URL` environment entry is computed from those same variables
  by Compose's own interpolation, pointing at the `postgres` service's
  Compose-network hostname — nobody sets a connection string by hand for
  the default case.
- **Default, zero-config path**: `docker compose --profile bundled-db up`.
  No value is set by the operator anywhere; the bundled Postgres container
  starts, `DATABASE_URL` resolves to it automatically. This is the LAN/
  home-server case, and it preserves ADR 0007's original design goal — no
  user-visible connection string, no setup screen — even though the
  underlying mechanism (a sibling container, not a spawned child process)
  is different from the Electron target's.
- **Customized, cloud-migration path**: the operator sets `DATABASE_URL`
  themselves, to anything reachable — a managed provider (RDS, Neon,
  Supabase, Google Cloud SQL), a Postgres on a separate self-hosted
  machine, whatever they land on — and runs plain `docker compose up`
  with no `bundled-db` profile, so the sibling Postgres container never
  starts. One environment variable is the entire migration path; TLS for
  a managed provider needs no separate design, since `sslmode` is just
  part of the connection string `pgx` already parses natively (e.g.
  `postgres://user:pass@host:5432/db?sslmode=require`).

This resolves the "simple but customizable" requirement without adding a
second config mechanism: the default is genuinely zero-input, and
customization is the same single key the application already reads,
never a new one.

**This does not touch constitution §6.** Where the container happens to
run — a home server or a cloud host — is a question about deployment
location. Whether the network can reach the backend without a credential
is a separate question, governed by the loopback-until-phase-12/13 rule
regardless of where the container runs. This decision does not accelerate
or relax that gate; `BIND_ADDRESS`'s loopback-only validation
(`backend-configuration.md` FR-8) still applies, with the container-
network-namespace nuance already named as an open question below,
unresolved by this addition.

No new backend service is introduced. `internal/domain`'s boundary
(constitution §3), the repository pattern (`backend-persistence.md` FR-2),
the API contract (`architecture-contracts.md`), and the error taxonomy are
unchanged — this decision is about how one backend process is *started and
reaches its database*, not about splitting what it does.

**Electron packaging is unaffected and does not go away.** The
container-hosted target does not replace the Electron-hosted one; it is
additive. A user who wants a single-click desktop app on their own machine
still gets exactly what ADR 0005/0007 already describe. A user who wants to
run Alexandryn on an always-on home server reaches it via Compose instead,
with no Electron window at all — that machine has no display to put one on.

## Options considered

### Option A — Two targets, same binary, Postgres as a sibling container under Compose (chosen)

*For* — reuses everything ADR 0008 already built (`go:embed`, one binary,
one artifact to ship) without reversing it; reuses the `DATABASE_URL` code
path `backend-persistence.md` FR-5 already specifies, so the amount of *new*
backend logic is close to zero; matches Docker/Docker Compose already being
named in `README.md`'s stated tech stack and ADR 0004's own "Docker Compose
already planned" packaging note; and, per the analysis above, answers a
question ADR 0005 and ADR 0007 each explicitly flagged as deliberately
undecided rather than reversing a settled one — **conditional on that
reading of their intent being the one you meant.**

*Against* — real: it does reopen two Accepted ADRs (0005, 0007) — as an
addendum under my reading, as a supersession under the alternative reading
above, either way not free — and forces several already-`APPROVED` specs to
amend and move back through review (`backend-configuration.md`,
`backend-persistence.md`, `backend-service-lifecycle.md`,
`backend-test-harness.md`), tracked in full in the companion amendment plan
(`.claude/audits/0002-topology-gap.md`). Two deployment targets is also two
things to keep working and two things to test, permanently, not a one-time
cost.

### Option B — Container-hosted only; retire the Electron target

*For* — one target, not two; removes the entire orphan-prevention,
child-process-spawn, and bundled-Postgres design surface ADR 0005/0007 spent
real effort getting right.

*Against* — rejected outright. Not what was asked for, and it would throw
away a fully-specified, `APPROVED`-through-phase-11 desktop product to solve
a problem ("also support Compose") that doesn't require it. The README's own
pitch — "opens on any device in your home," a double-click desktop app —
depends on the Electron target existing. Retiring it is a product decision
nobody has made, and this ADR does not get to make it by accident while
answering a packaging question.

### Option C — True microservices: split the backend into multiple independently-deployable services

*For* — none found in this project's own record. Rejected before evaluation
depth, because the maintainer confirmed reading (b), not (a), when asked
directly.

*Against* — would supersede, not amend, ADR 0005, ADR 0007, and ADR 0014
(Postgres-backed queue, explicitly not a broker) — three Accepted decisions,
not two, and unlike the two analyzed above, no hedge language anywhere
defers this one. No service boundary within the current domain model
(constitution §3's metadata/source/Alexandryn split is a boundary *within*
one backend, not a proposal for separate deployables) has ever been named as
independently scalable or independently releasable.

### Option D — Frontend and backend as separate containers

*For* — matches how many Compose-deployed web apps are structured (a static
frontend server plus an API container).

*Against* — rejected: contradicts `architecture-system.md` FR-6 (the same
build artifact, served by the same Go server) and ADR 0008's `go:embed`
decision, for no stated benefit. The container target keeps FR-6 true: one
Go server, one embedded frontend, regardless of which target is running it.

## Consequences

**Good** — the container target reuses `backend-persistence.md` FR-5's
existing `DATABASE_URL` code path almost as-is. ADR 0008's monorepo layout,
`go:embed` decision, and "no build-orchestration tool" reasoning all survive
unchanged. Constitution §3's domain boundaries, the API contract, and the
error taxonomy are all topology-independent and need no change at all.

**Bad** — named in full in `.claude/audits/0002-topology-gap.md`, not
minimized here: ADR 0005 and ADR 0007 each need either an addendum
(narrowing a claim currently stated as universal to "true for the
Electron-hosted target") or, if you confirm the stricter reading of their
original intent, a formal supersession — that determination is still open,
not settled by this rewrite, and blocks how those two documents get edited.
Four `APPROVED` specs (`backend-configuration.md`, `backend-persistence.md`,
`backend-service-lifecycle.md`, `backend-test-harness.md`) must be amended
and moved back through the review gate before implementation of the areas
they cover can proceed. `architecture-system.md`'s FR-1 needs a second
post-approval amendment. No test currently exercises the container/Compose
target at all.

**Neutral** — local-dev parity between Supabase's local stack, the Electron
bundled-Postgres path, and the new Compose target is a real open question
this ADR does not resolve — options and trade-offs presented separately, not
decided here.

## Reversal cost

Low right now — nothing has been built against either target. Once
`backend-configuration.md`/`backend-persistence.md`'s amendments are
implemented and a `docker-compose.yml` exists, reversing this (retiring the
container target) is cheap: delete the compose file and the amendment
language; the Electron target is untouched throughout because this decision
was deliberately additive to it. Reversing the *other* direction (retiring
Electron, Option B) would be expensive once phase 05's Electron-specific
code exists — not proposed here, named only so the asymmetry is on the
record.

## Confidence

Medium, and lower than the previous draft of this ADR stated, for a specific
reason: the "addendum, not supersession" framing for ADR 0005 and ADR 0007
rests on an interpretation of what their hedge language meant, which I
cannot verify from the documents alone. High confidence that Option A
(additive container target, same binary, sibling Postgres container, no new
backend service) is the correct *shape* given the maintainer's confirmed
reading (b). Lower confidence — flagged, not resolved — on whether ADR 0005
Option C and ADR 0007 Option B were meant as permanent rejections (making
this a reversal) or deliberately deferred questions (making this an
extension); the "Relationship to ADR 0005 and ADR 0007" section above states
both readings and the textual evidence for each rather than picking one
unilaterally. Also unresolved, per the same honesty standard: how
`BIND_ADDRESS`'s loopback-only rule (`backend-configuration.md` FR-8,
constitution §6) should be read inside a container's own network namespace,
and local-dev parity across the three Postgres-provisioning mechanisms —
both named, neither guessed at.
