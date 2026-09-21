# Spec: System architecture

| | |
|---|---|
| **Status** | `APPROVED` (maintainer confirmed 2026-08-20, both amendments — ADR 0007's bundled-Postgres process and ADR 0015's container target — ratified as written; the async-messaging ownership gap noted in Open questions remains open, not a blocker on this spec itself) |
| **Phase** | `01-architecture` |
| **Author** | Claude (Sonnet 5), reviewed and approved by Luann Moreira; amended 2026-08-14, amended again 2026-08-16 |
| **Created** | 2026-08-13 |
| **Last updated** | 2026-08-16 |
| **Supersedes** | — |
| **Reviewed in** | `.claude/reviews/0004-spec-architecture-system.md` — Approved with changes 2026-08-13; first amendment below not yet re-reviewed at the time of the second; both covered by `0040` |

**Amendment note (2026-08-14):** drafting `architecture-persistence.md`
surfaced that production PostgreSQL needs to be bundled and managed by the
Go server (ADR 0007) — the design reference has no database-configuration
screen anywhere, so a shipped instance can't ask the user for a connection
string. That makes PostgreSQL a *third* long-running process, contradicting
this spec's original FR-1 ("exactly two"). Per constitution §1 and
`specs/README.md`'s own rule — amend the spec, note the change, move it back
through review rather than let the amendment happen silently — FR-1, FR-2,
new FR-12, Security considerations, and Open questions are updated below.
Status moved back to `REVIEWED` until the maintainer confirms the amendment.

**Amendment note (2026-08-16):** ADR 0015 adds a second, additive deployment
target — the backend containerized, composed with PostgreSQL, no Electron
process at all — alongside the target this spec described exhaustively.
FR-1 and FR-2 stated "a running instance MUST consist of..." as if only one
topology were legal; Security considerations stated that an
externally-supplied `DATABASE_URL` "is not how a shipped instance gets its
database," which is no longer true of the container-hosted target. This
amendment scopes those statements to the Electron-hosted target
specifically and adds the container-hosted target as an equally legal
second shape, rather than treating the first amendment's topology as the
only one anyone would ever add. FR-1, FR-2, Security considerations, and
Open questions are updated below. Status stays `REVIEWED`, unconfirmed by
the maintainer for this second amendment.

## Context

CLAUDE.md and the README already state the shape at a high level: an
Electron desktop app hosts a Go server, which serves a React web interface to
the desktop window and to other devices on the user's home network. Storage
is self-hosted PostgreSQL, local to the host machine (ADR 0004). Nothing
below that summary has been written down precisely enough to build against —
this spec is that precision: what processes exist, what talks to what, over
what transport, and what happens at startup, shutdown, and failure.

This is the foundational spec for phase 01. `architecture-backend.md`,
`architecture-frontend.md`, `architecture-desktop-host.md`,
`architecture-persistence.md`, and `architecture-contracts.md` all assume the
process model and trust boundaries this spec defines; they should not
contradict it, and if one needs to, that's a sign this spec was wrong and
needs amending, not that the other spec gets to quietly redefine the system.

## Problem

There is currently no answer, in writing, to: how many processes does a
running instance of Alexandryn consist of, how are they started and stopped,
how does a LAN device reach the same library as the desktop window, and what
does each process do when the one next to it is slow, crashed, or hostile.

## Goals

- Name every long-running process in a running instance, and what starts and
  stops it
- Define the transport and addressing between the Electron renderer, the
  Electron main process, the Go server, and PostgreSQL
- Draw the three trust boundaries (renderer↔main, host↔LAN client,
  system↔external source/metadata provider) onto this concrete process model
- Define the application lifecycle: cold start, ready, degraded, shutdown —
  and what "degraded" means operationally
- Settle the one-binary-vs-two-processes question phase 01 flagged as open

## Non-goals

- The Go package layout and internal dependency rules — `architecture-backend.md`
- The React component/state layering — `architecture-frontend.md`
- The Electron IPC surface's exact enumerated operations — `architecture-desktop-host.md`
- Schema, migrations, connection pooling specifics — `architecture-persistence.md`,
  and the engine choice itself, which is already decided (ADR 0004)
- API endpoint shapes and versioning — `architecture-contracts.md`
- Authentication and pairing — phase 12, phase 13. This spec places the
  boundary where the credential will eventually be checked; it does not
  design the credential
- Multi-host deployment — a single running instance's processes (Electron
  target) or containers (container target, ADR 0015) spread across more
  than one host — remains explicitly out of scope; each target's two
  containers or three-to-four processes run on one host. **Cloud
  deployment is no longer a non-goal**: ADR 0015's container target is
  designed to run unmodified on a cloud host exactly as it runs on a home
  server — this bullet originally conflated the two, before that target
  existed
- Test tooling, fixtures, and what runs in CI — `architecture-testing.md`
- Messaging architecture — the conditions under which work becomes
  asynchronous (phase 01's scope lists this as in-scope for the phase, but no
  spec in phase 01's Specifications table currently owns it, and it isn't
  this one either; see Open questions)

## User stories

Architectural, not feature-shaped — the "user" here is every later phase and
every contributor reasoning about failure:

- As **phase 03 (backend foundation)**, I want a settled process and transport
  model, so that I'm not the one deciding it under implementation pressure.
- As **phase 05 (desktop host)**, I want the Electron/Go boundary defined, so
  that the preload surface (constitution §5) has a concrete thing to
  enumerate operations against.
- As **a contributor debugging a failure**, I want to know which process
  owns which failure mode, so I know where to look first.

## Functional requirements

- **FR-1** A running instance MUST take one of two legal shapes (ADR 0015 —
  amended 2026-08-16; this FR previously described only the first shape as
  if it were exhaustive). **The Electron-hosted target**: three
  long-running processes on the host — the Electron application (main +
  renderer), the Go server, and a bundled PostgreSQL instance the Go server
  spawns and owns (ADR 0007 — amended 2026-08-14; this spec originally said
  "exactly two," written before production Postgres provisioning was
  decided) — **or four on macOS specifically**, where PostgreSQL
  orphan-prevention needs an additional small supervisor process Alexandryn
  writes (`architecture-persistence.md` FR-10), because macOS has no
  parent-side spawn-time death-signal mechanism the way Linux and Windows
  do. Within this target, none of its processes are ever compiled or run as
  a single binary, and Postgres is never spawned by anything other than the
  Go server (or, on macOS, the supervisor acting on the Go server's
  behalf) — this keeps FR-5's "only the Go server talks to Postgres" true
  of process ownership, not just network access, for this target
  specifically. **The container-hosted target** (ADR 0015): two containers
  — the Go server and a separate PostgreSQL instance — with no Electron
  process at all and no spawn relationship between them; each is started
  and stopped by the container orchestrator, not by the other.
- **FR-2** **In the Electron-hosted target**, the Electron main process
  MUST spawn the Go server as a child process on application start, and
  MUST terminate it on application quit — the Go server's lifetime is a
  subset of the Electron app's lifetime, never the reverse. The Go server
  MUST spawn PostgreSQL the same way, one level down: Postgres's lifetime
  is a subset of the Go server's, which is a subset of Electron's.
  Orphan-prevention (ADR 0005, `FR-10` below) applies at each level — the
  Go server must not survive Electron's disappearance, and Postgres must
  not survive the Go server's. **In the container-hosted target** (ADR
  0015), there is no spawn relationship: the Go server and PostgreSQL are
  independent containers, each started and stopped by the container
  orchestrator, and the Go server connects to Postgres over the network
  using a configured `DATABASE_URL` (`backend-configuration.md` FR-4,
  `backend-persistence.md` FR-5) rather than spawning and owning it.
  Neither target's shutdown ordering constrains the other.
- **FR-3** The Go server MUST bind to `127.0.0.1` (loopback) only, on a port
  chosen at startup, until phase 12/13 introduce authenticated LAN binding
  (constitution §6). The port MUST NOT be hardcoded such that two instances
  (e.g. a second user account on the same machine) cannot both run.
- **FR-4** The Electron renderer MUST NOT talk to the Go server, PostgreSQL,
  or the filesystem directly. All such access goes through the preload
  bridge into the main process, or through HTTP calls the renderer makes to
  the Go server's loopback address — never through a Node/Electron API
  exposed into renderer JavaScript (constitution §5).
- **FR-5** The Go server MUST be the only process that connects to
  PostgreSQL. LAN clients and the Electron renderer reach data exclusively
  through the Go server's HTTP API (ADR 0004).
- **FR-6** The web UI served to the Electron window and the web UI served to
  a LAN device MUST be the same build artifact, served by the same Go
  server — not two separately built frontends.
- **FR-7** On startup, the Go server MUST attempt to reach PostgreSQL before
  reporting itself ready, and MUST distinguish "process is up" from "process
  can serve requests that touch storage" in its health/readiness endpoints
  (this is restated from phase 03's objective; this spec is where the
  distinction is first required to exist).
- **FR-8** The Electron main process MUST detect if the Go server child
  process exits unexpectedly, and MUST surface that to the user rather than
  silently presenting a UI that can no longer reach data.
- **FR-9** Shutdown (app quit, OS signal) MUST attempt to let in-flight HTTP
  requests to the Go server complete or be cleanly refused, then terminate
  the Go server, then exit Electron — never the reverse order, and never a
  hard kill as the first resort. The grace period before falling back to a
  hard kill is bounded (placeholder: 10 seconds, unmeasured — see Open
  questions), not indefinite.
- **FR-10** The Go server MUST terminate if the Electron main process that
  spawned it exits or becomes unreachable, by crash or forced kill. It MUST
  NOT continue running as an orphan holding its port bound. On Linux: the Go
  binary sets `prctl(PR_SET_PDEATHSIG)`, prototype-verified (ADR 0005). On
  macOS and Windows: mechanism named but not yet built or verified — see
  Open questions.
- **FR-11** The system MUST prevent, or explicitly and visibly handle, a
  second instance starting while one is already running against the same
  PostgreSQL data. It MUST NOT allow two instances to run silently
  divergent against the same data — either the second start is refused with
  a clear message, or it focuses the existing window. Mechanism undecided;
  see Open questions.
- **FR-12** (Added with ADR 0007) PostgreSQL MUST terminate if the Go
  server that spawned it exits or becomes unreachable, by crash or forced
  kill — the same orphan-prevention requirement FR-10 places on the Go
  server one level up, now applied one level down. It MUST NOT continue
  running as an orphan holding a data directory open. Mechanism per
  platform: not designed here — `architecture-persistence.md` owns it,
  informed by whatever FR-10's phase 05 implementation settles on, since
  the underlying OS mechanisms (Linux `pdeathsig`, macOS `kqueue`, a
  Windows Job Object) are the same family of tool either way.

## Non-functional requirements

- **Performance** — cold start to "ready" (Go server passing its readiness
  check against PostgreSQL) budgeted at under 3 seconds on the reference dev
  machine; this is a placeholder budget for phase 03 to confirm or replace
  with a measured number, not a number derived from any benchmark yet, and is
  flagged as such in Open questions.
- **Security** — see Security considerations below; this spec's main
  contribution is placing the boundaries, not designing what crosses them.
- **Accessibility** — not applicable at this layer; owned by
  `architecture-frontend.md`.
- **Reliability** — the Go server crashing MUST NOT crash Electron, and MUST
  be visible to the user as "can't reach the library" rather than a frozen or
  blank window. Electron crashing takes the Go server down with it (FR-2's
  inverse is not required — a supervisor that keeps the Go server alive after
  Electron dies would imply LAN clients can be served with no desktop app
  running, which is a different product decision this spec does not make).
- **Observability** — **in the Electron-hosted target**, the Go server's
  structured logging (phase 03) and the Electron main process's own logs
  are separate log streams; this spec requires that a correlation ID
  generated for an HTTP request is visible in both if the request path
  crosses into Electron-mediated operations (e.g. a native file dialog
  triggered by an IPC call that also touches the API) — full logging
  contract is phase 03's `backend-errors-and-logging.md`. **In the
  container-hosted target** (amended 2026-08-16, ADR 0015), there is no
  Electron process and no second log stream to correlate against — the Go
  server's structured logging is the entire log surface for that target.
  The container orchestrator's own log aggregation (e.g. `docker compose
  logs`, which interleaves the `backend` and `postgres` containers'
  output by timestamp) is the analogous surface to an operator, but it is
  not a second application-level stream this spec needs a correlation
  requirement for — Postgres's own log lines carry no correlation ID and
  aren't expected to.

## Domain model

Not applicable — this spec describes processes and transport, not the
Alexandryn/metadata/source domain (constitution §3). It does establish *where*
that domain's boundary-normalisation code runs: inside the Go server, never
in the Electron main process and never in the renderer.

## API and contracts

At this spec's level of detail:

- **Renderer ↔ Go server**: HTTP, over loopback (or the LAN address once
  phase 13 allows it), same-origin as served by the Go server itself — the
  renderer does not know a different address for "desktop mode" vs "network
  mode". Exact endpoint shapes: `architecture-contracts.md`.
- **Renderer ↔ Electron main**: the preload-exposed enumerated operation
  list (constitution §5). Exact surface: `architecture-desktop-host.md`.
- **Electron main ↔ Go server (control plane)**: the main process needs to
  know the Go server's actual bound port (if chosen dynamically) and needs to
  signal shutdown. This is a narrow, separate channel from the HTTP API the
  renderer uses — proposed as the Go server writing its bound port to stdout
  on a well-known first line, and the main process sending `SIGTERM` (or
  Windows equivalent) for shutdown. `architecture-desktop-host.md` owns the
  exact mechanism.
- **Go server ↔ PostgreSQL**: `architecture-persistence.md`.
- **Go server ↔ external sources/metadata**: outbound only, from the Go
  server; never from the renderer or Electron main directly. Shape:
  phase 07 (metadata), phase 08 (sources).

## State transitions

Application-level states, as observed from the Electron main process:

```
Starting -> Ready
Starting -> Degraded (Go server up, PostgreSQL unreachable)
Starting -> Failed (Go server did not start: port bind failure, missing binary)
Ready -> Degraded (PostgreSQL connection lost after startup)
Degraded -> Ready (PostgreSQL connection recovered)
Ready -> ShuttingDown
Degraded -> ShuttingDown
ShuttingDown -> Stopped
```

Illegal transitions worth naming explicitly, because they become tests:

- `Failed -> Ready` without passing through `Starting` again (i.e. the app
  must not silently recover from a failed start without a real restart)
- `Stopped -> ` anything other than a fresh `Starting` (no resurrecting a
  stopped instance in place)
- Any transition that reaches `Ready` without the readiness check (FR-7)
  having actually passed

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| PostgreSQL unreachable at startup | Go server's readiness check fails | "Can't reach the library's database. [detail]. Retry, or check it's running." with a retry action, not a crash | Go server reports `Degraded`, keeps retrying on a backoff, Electron shows the state rather than blocking on it forever |
| PostgreSQL connection lost after startup | Query/health-check failure mid-session | In-flight actions that need storage fail with a clear message; read-only cached views (if any exist by then) may still render | Go server transitions `Ready -> Degraded`; does not crash |
| Go server child process exits unexpectedly | Electron main's child-process exit handler | "The library service stopped unexpectedly. Restart Alexandryn." — not a blank or frozen window | Electron detects the exit, surfaces it, does not attempt an infinite silent respawn loop (a bounded retry with backoff is acceptable, an unbounded one masks a real problem) |
| Loopback port already in use | Bind failure on Go server startup | "Alexandryn couldn't start (port in use)." with enough detail to actually debug it, not a raw stack trace (constitution §11) | Go server exits non-zero with a specific error; Electron surfaces it rather than silently retrying the exact same bind forever |
| Electron main process crashes | OS-level; nothing left to detect it from inside the app | The window/app disappears | Go server, as the child, receives no more heartbeat/control signal — exits per FR-10 rather than becoming an orphaned process that keeps a port bound (mechanism undecided; Open questions) |
| Two instances started on the same machine | Second instance's port bind, or a lock file | Either the second instance fails clearly, or focuses the first instance's window — not two silently-diverging instances against the same PostgreSQL data | Handled per FR-11; mechanism not yet decided, Open questions |

## Security considerations

The three trust boundaries phase 01 requires be diagrammed, restated onto
this process model:

- **Renderer ↔ main process** — the renderer is assumed compromised
  (constitution §5). It reaches the main process only through the
  preload-enumerated surface, and reaches the Go server only through HTTP,
  never through a general filesystem/shell/network primitive. FR-4 is the
  concrete requirement.
- **Host ↔ network client** — a LAN client is assumed hostile and
  unauthenticated until phase 12/13. FR-3 (loopback-only binding) is what
  makes this boundary physically absent rather than merely policy, for every
  phase before 13. This is the same requirement constitution §6 states; this
  spec is where it becomes a concrete bind address rather than a principle.
- **System ↔ external source/metadata provider** — outbound-only from the Go
  server (API and contracts, above); the renderer and Electron main never
  originate a request to a source directly, which keeps constitution §4's
  "every source response is hostile" validation in exactly one place (the Go
  server) rather than duplicated or missed in Electron.

Additional, specific to this spec:

- **Go server as a spawned child process** — the Electron main process
  chooses the binary path and arguments it spawns; this must be the
  application's own bundled binary, never a path influenced by
  renderer-supplied or environment-supplied input, or a compromised renderer
  could attempt to have Electron spawn something else.
- **PostgreSQL as a spawned child process (ADR 0007), in the
  Electron-hosted target** — same requirement, one level down: the Go
  server chooses the bundled `postgres` binary path and initializes its own
  data directory, never influenced by renderer- or network-supplied input.
  In that target's production use, the Go server generates its own
  connection string after spawning Postgres itself — it does not receive
  `DATABASE_URL` from an external source the way the dev setup does (ADR
  0004's addendum, Supabase CLI stack). **In the container-hosted target
  (ADR 0015), the opposite is true by design**: `DATABASE_URL` is the
  normal, expected way the Go server learns where Postgres is — either the
  bundled sibling container's Compose-network address (the zero-config
  default) or an externally managed instance the operator points it at (the
  customization/cloud-migration path). An earlier version of this paragraph
  stated flatly that the external-config path "is not how a shipped
  instance gets its database" — that was true only of the Electron-hosted
  target, corrected here rather than left as a project-wide absolute it was
  never meant to be once a second target existed.
- **Control-plane channel (main ↔ Go server)** — narrower than the HTTP API,
  but still a boundary: if the port-announcement mechanism (stdout, above) is
  used, the main process must not trust anything else printed to that stream
  as a control message.
- **Configuration and secrets handed to the spawned process** — the main
  process must pass the Go server its configuration (today: `DATABASE_URL`;
  later: credentials once phase 12 exists) at spawn time. Command-line
  arguments are visible to any local user via `ps`; environment variables are
  visible to same-user processes via `/proc/PID/environ` on Linux and
  equivalent mechanisms elsewhere. Neither is safe for a real secret once one
  exists. This spec does not pick the mechanism (a restricted-permission
  file descriptor or config file, an OS keychain, or accepting env vars for
  now since nothing passed today is more sensitive than a loopback-only dev
  database URL) — see Open questions.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Not directly — this spec has no code of its own |
| Integration | N/A at this spec's level; phase 03 and phase 05 carry the actual tests for the processes this spec describes |
| Contract | N/A — `architecture-contracts.md` |
| E2E | The walkthrough below |
| Accessibility | N/A |

Per phase 01's own test strategy, this spec is verified by walkthrough, not
execution:

- **Walkthrough** — trace "open a book from the library, on a second
  device": LAN device HTTP request to Go server → Go server queries
  PostgreSQL → response served → same code path Electron's own renderer
  uses. If this walkthrough needs a different path for the two clients, this
  spec's FR-6 is violated and needs to be fixed here, not patched around
  later.
- **Hostile walkthrough** — same trace, as a compromised renderer (blocked by
  FR-4) and as an unauthenticated LAN client (blocked by FR-3, until
  phase 13).

## Acceptance criteria

- [ ] Every functional requirement above maps to at least one exit criterion
      in phase 03 or phase 05's own document (cross-reference, not duplicate)
- [x] The one-binary-vs-two-processes question is closed by this spec (FR-1,
      FR-2) and by [ADR 0005](../decisions/0005-process-model.md), which
      records *why* with prototype evidence rather than reasoning alone
- [ ] The state diagram's illegal transitions each have a named owner (phase
      03 or phase 05) for the test that will enforce them
- [ ] Reviewed and at minimum `REVIEWED`, ideally `APPROVED`, before
      `architecture-backend.md` and `architecture-desktop-host.md` are
      considered final (they may be drafted in parallel, but shouldn't lock
      in a contradiction to this spec)

## Open questions

- **Orphaned Go server on Electron crash** — resolved on Linux (ADR 0005,
  prototype-verified: `prctl(PR_SET_PDEATHSIG)`, stdlib only). Still open on
  macOS (kqueue `EVFILT_PROC`/`NOTE_EXIT` monitoring the parent PID, named
  but unbuilt) and Windows (a Job Object with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, same status). Owner: phase 05.
- **Second-instance handling** — resolved in `architecture-desktop-host.md`
  FR-4: `app.requestSingleInstanceLock()`, second start refused and existing
  window focused.
- **Orphaned PostgreSQL on Go-server crash (FR-12, added with ADR 0007)** —
  same status as the Electron/Go-server case above: real requirement, no
  chosen mechanism per platform yet. Owner: `architecture-persistence.md`.
- **Startup budget (3 seconds)** — a placeholder, not a measurement. Phase 03
  should replace it with a real number once there's something to measure, or
  explicitly ratify it as the target.
- **Dynamic vs. fixed loopback port** — FR-3 requires it not be hardcoded in
  a way that blocks a second OS user account, but doesn't pick the mechanism
  (OS-assigned ephemeral port vs. a fixed default with fallback). Owner:
  phase 03, informed by whatever `architecture-desktop-host.md` needs for the
  control-plane channel.
- **Shutdown grace period (10 seconds, FR-9)** — a placeholder, same status
  as the startup budget: not measured, needs phase 03 to confirm or replace
  once there's a real service to time.
- **Config/secrets across the spawn boundary** — argv and env vars are both
  named as insufficient once a real secret exists (Security considerations,
  above). Owner: `architecture-desktop-host.md`, since it owns the spawn
  mechanism, informed by phase 03's `backend-configuration.md` and phase 12's
  credential design once that exists.
- **`BIND_ADDRESS`'s loopback-only rule inside a container's own network
  namespace (ADR 0015, `backend-configuration.md` FR-8)** — "loopback"
  and "unreachable from outside the process's own network boundary" are
  the same statement on bare metal and a different one inside a
  container, where a process bound to `0.0.0.0` may still be reachable by
  nothing outside its own isolated Compose network before Docker
  publishes a port. Not resolved here; owner is the new deployment spec
  ADR 0015's amendment plan names (`.claude/audits/0002-topology-gap.md`
  A-02-11).
- **Local-dev parity between the Electron target's Supabase-backed dev
  loop, its own bundled-Postgres mechanism, and the container target's
  Compose file (ADR 0015)** — three separate Postgres-provisioning
  mechanisms now exist; whether any should be retired or unified is a real
  open question, not decided by this amendment or by ADR 0015 itself.
- **Messaging/async architecture has no owning spec** — phase 01's scope
  lists "the conditions under which work becomes asynchronous" as in-scope
  for the phase, but its Specifications table names no spec for it, and this
  one explicitly isn't it (Non-goals). Needs either a new
  `architecture-messaging.md` added to phase 01's spec table, or an explicit
  decision to fold it into `architecture-backend.md`. Owner: whoever updates
  `01-architecture/README.md` — flagging here since this spec is what
  surfaced the gap.

## References

- CLAUDE.md — "What this project is"
- README.md — "Planned shape" diagram and technology table
- Constitution §3 (domain boundaries), §4 (hostile input), §5 (Electron
  privilege boundary), §6 (network exposure), §11 (copy)
- ADR 0004 — persistence engine is self-hosted PostgreSQL
- ADR 0005 — process model, prototype-backed
- ADR 0007 — production PostgreSQL is bundled and managed by the Go server;
  the reason FR-1, FR-2, FR-10, and Security considerations were amended
  2026-08-14
- ADR 0015 — a second, container-hosted deployment target; the reason FR-1,
  FR-2, and Security considerations were amended again 2026-08-16
- `architecture-desktop-host.md` — resolved second-instance handling (FR-4)
- `docs/roadmap/01-architecture/README.md` — this spec's parent phase
- `docs/roadmap/03-backend-foundation/README.md`,
  `docs/roadmap/05-desktop-host/README.md` — phases this spec constrains
