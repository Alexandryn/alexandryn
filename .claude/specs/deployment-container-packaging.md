# Spec: Container deployment packaging

| | |
|---|---|
| **Status** | `APPROVED` (maintainer, 2026-08-25 — Luann Moreira; self-reviewed with changes, amended for ADR 0017's TLS/bind condition 2026-08-18, maintainer-directed; amended again same day during T27 implementation — `OPEN_LIBRARY_USER_AGENT` (`backend-configuration.md` FR-3) has no default by design and isn't a `docker-compose.yml`-level concern this spec's FR-4 originally accounted for, see FR-4 and Acceptance criteria below; self-reviewed, needs maintainer re-confirmation) |
| **Phase** | `03-backend-foundation` |
| **Author** | Claude (Sonnet 5), for review by Luann Moreira |
| **Created** | 2026-08-17 |
| **Last updated** | 2026-08-25 |
| **Supersedes** | — |
| **Reviewed in** | [`.claude/reviews/0046-spec-deployment-container-packaging.md`](../reviews/0046-spec-deployment-container-packaging.md) — Approved with changes, findings fixed; self-reviewed, independent read still pending |

## Context

ADR 0015 decided that Alexandryn ships a second deployment target — the
Go server containerized, composed with PostgreSQL via Docker Compose —
alongside the existing Electron-hosted desktop app, and its addenda to
ADR 0005/ADR 0007 established why that's an extension of the existing
process-model decisions rather than a reversal of them. Six specs
(`architecture-system.md`, `architecture-backend.md`,
`backend-configuration.md`, `backend-persistence.md`,
`backend-service-lifecycle.md`, `backend-test-harness.md`) were amended to
describe the container target's *behavior* — what `DATABASE_URL` means to
it, how it starts, what it logs. None of them own the actual `Dockerfile`
or `docker-compose.yml` content, or say how the container target's
readiness is observed without contradicting the loopback-only bind
`backend-configuration.md` FR-8 already requires. This spec is that
content.

## Problem

Nothing has fixed: what the `Dockerfile` actually contains (base image,
build stages, user, healthcheck), what `docker-compose.yml` actually
contains (service definitions, the `bundled-db` profile, volumes,
networking), or the concrete guard that keeps the container target's
correct pre-authentication unreachability from being quietly broken by an
ordinary future change to the compose file.

## Goals

- Fix the `Dockerfile`: multi-stage build matching ADR 0008's `web/`-then-
  `go build` ordering, a minimal runtime image, a non-root user, and a
  `HEALTHCHECK` instruction `backend-test-harness.md` FR-10's CI test
  depends on
- Fix `docker-compose.yml`: the `backend` and `postgres` services, the
  `bundled-db` profile gating the latter, `DATABASE_URL` interpolation
  matching ADR 0015's design exactly, and a named volume for Postgres's
  data directory
- Fix that the default compose file publishes no port for `backend` — the
  container target is unreachable by anything outside its own Compose
  network until phase 12/13 ships authentication, by design, not by
  omission
- Add a CI guard that keeps that unreachability from being broken by an
  unreviewed `ports:` line or `network_mode: host` addition to the
  compose file

## Non-goals

- Publishing a port, designing a reverse proxy, or any LAN/internet-
  reachable configuration for the container target — phase 12/13's, once
  authentication exists; this spec's compose file is intentionally
  unreachable from outside itself
- Image registry, tagging scheme, or release versioning for the built
  image — phase 99's, when a release process exists to version against
- Kubernetes, Helm charts, or any orchestrator beyond Docker Compose —
  never proposed by ADR 0015, out of scope here for the same reason
- Windows container support — the bundled binary already targets
  Linux/macOS/Windows for the Electron target; the container target's own
  base image is Linux-only, consistent with how self-hosted Docker
  deployments are actually run in practice, and nothing in ADR 0015 asked
  for a Windows container image
- A structured logging change for the container's stdout/stderr — the Go
  server's existing structured logging (`backend-errors-and-logging.md`)
  already writes to stdout; Docker/Compose's own log capture needs no new
  application code

## User stories

- As **an operator running the container target on a home server**, I
  want `docker compose --profile bundled-db up` to work with no
  configuration, so the zero-config path ADR 0015 promised is actually
  true.
- As **an operator migrating to a cloud host**, I want to set one
  environment variable and run `docker compose up` (no profile), so
  pointing the backend at a managed Postgres instance is genuinely the
  whole migration.
- As **`backend-test-harness.md` FR-10's CI test**, I want a `Dockerfile`
  with a working `HEALTHCHECK` and a `docker-compose.yml` whose services
  actually start, so there's something real to build and bring up.
- As **a future contributor working on phase 12/13**, I want the compose
  file's current unreachability to be a CI-enforced property, not a
  convention, so I can't accidentally publish a port while working on
  something unrelated.

## Functional requirements

- **FR-1** The `Dockerfile` MUST be a multi-stage build: a build stage
  that runs `web/`'s build (`frontend-tooling.md` FR-6) then
  `go build ./cmd/server` (ADR 0008's ordering, already required by
  `architecture-testing.md` FR-8 and `backend-test-harness.md` FR-8 for
  CI), and a separate, minimal runtime stage that copies only the
  resulting binary — no Go toolchain, no `web/` source, no build
  dependencies in the final image.
- **FR-2** The runtime stage MUST run as a non-root user, created
  explicitly in the `Dockerfile` rather than relying on a base image's
  default — a container compromised through an application-level defect
  (the same hostile-input posture constitution §4 already requires of
  every external input) gains less if the process inside it isn't root.
- **FR-3** The `Dockerfile` MUST declare a `HEALTHCHECK` instruction
  invoking `http://127.0.0.1:<port>/readyz` (`curl` or `wget`, whichever
  is already present in the minimal runtime image or added at negligible
  size cost) — not `/healthz`. Decided 2026-08-25 (maintainer): a
  `backend` that's alive but can't reach Postgres must not report
  healthy to Compose's `depends_on: condition: service_healthy`
  (FR-4), which is exactly `architecture-system.md` FR-7's "process is
  up" vs. "process can serve storage-backed requests" distinction, and
  `/readyz` is the endpoint that already makes it (`internal/transport/http/health.go`).
  This executes inside the container's own namespace, the same
  relationship `docker exec` has to a running container, and is what
  `backend-test-harness.md` FR-10's `docker compose ... --wait` observes.
  It MUST NOT be implemented as, or replaced by, any check that reaches
  the container from outside its own namespace.
- **FR-4** `docker-compose.yml`, at the repository root, MUST define
  exactly two services: `backend` (built from the `Dockerfile`) and
  `postgres` (official `postgres` image, tagged with
  `profiles: ["bundled-db"]`), plus a named volume for Postgres's data
  directory. `backend`'s `DATABASE_URL` environment entry MUST be
  computed by Compose's own variable interpolation from
  `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB`, pointing at the
  `postgres` service's Compose-network hostname — matching ADR 0015's
  design exactly, not a reinterpretation of it. `backend` MUST declare
  `depends_on: { postgres: { condition: service_healthy } }` so Compose
  itself, not application-level retry logic, keeps `backend` from
  starting before `postgres` is ready to accept connections.

  Default values, decided 2026-08-25 (maintainer): `POSTGRES_USER=admin`,
  `POSTGRES_PASSWORD=admin`, `POSTGRES_DB=alexandryn` —
  `${POSTGRES_USER:-admin}` -style Compose interpolation, so the file
  still works with no `.env` present. These MUST NOT be hardcoded only
  in `docker-compose.yml`: a committed `.env.example` at the repository
  root documents the three variables and these defaults; an operator's
  own `.env` (gitignored, Compose's own auto-loaded convention — no
  extra flag needed) is where a self-hoster is expected to set real
  values before this target is ever exposed beyond loopback. The weak
  default is an accepted tradeoff specifically because FR-5 keeps this
  target unreachable by design under the default profile — it is not a
  production credential, it is what a first `docker compose up` needs to
  come up loopback-only with zero configuration.

  Amendment, 2026-08-25 (self-reviewed during T27 implementation, needs
  maintainer re-confirmation): `backend` also requires
  `OPEN_LIBRARY_USER_AGENT` to start at all (`backend-configuration.md`
  FR-3, `categoryRequired`, no default — confirmed by running the FR-1
  image standalone: it fails fast naming the missing key). Unlike
  `POSTGRES_USER`/`PASSWORD`/`DB`, this key MUST NOT get a
  `docker-compose.yml`-level default — `backend-configuration.md`'s own
  stated reasoning is that a placeholder value would misidentify this
  client to Open Library's real, live API, which is exactly what the
  `required`/no-default category exists to prevent, and that reasoning
  applies identically here. `docker-compose.yml` MUST pass it through
  unset (`environment: [OPEN_LIBRARY_USER_AGENT]`, no `=value` —
  Compose's own pass-through-if-set syntax), never default it. An
  operator sets a real value in their own `.env`; `backend-test-harness.md`
  FR-10's own CI job sets one explicitly for that run (an honest,
  CI-identifying string, not a placeholder pretending to be a real
  deployment).
- **FR-5** `docker-compose.yml`'s default profile MUST NOT publish any
  port for the `backend` service (no `ports:` entry) and MUST NOT set
  `network_mode: host` for it. This is the correct default per ADR 0017's
  Mode B (bound private-only): nothing reachable from outside the
  Compose network is correct before both authentication and a transport
  guarantee exist. A separately tracked override file MAY publish a port
  for `backend`, legal only when it also satisfies ADR 0017's Mode A in
  full — `TLS_CERT_FILE`/`TLS_KEY_FILE` mounted into the container and
  authentication enabled in `backend`'s own configuration — never a
  published port on its own. This is deployment-time operator
  configuration for a remote-reachable instance, not this spec's default
  file.
- **FR-6** CI MUST include a check that fails the build if any tracked
  compose file contains a `ports:` entry for the `backend` service or sets
  `network_mode: host` on it, **unless** that same file also declares both
  `TLS_CERT_FILE`/`TLS_KEY_FILE` mounted into `backend` and an
  authentication-enabled setting in `backend`'s environment — ADR 0017's
  Mode A, checked structurally (the keys are present in the file), not
  semantically (this check does not validate the certificate itself; FR-8
  in `backend-configuration.md` does that at the process's own startup).
  A `ports:` entry with neither condition present still fails the build,
  unchanged from before. This is a static check against the compose
  file's own YAML, not a runtime test — the property guarded is "this
  file was never edited to publish a port without also wiring the
  condition that makes doing so legal," not "the running container
  behaves correctly." This is what keeps FR-5's guarantee from depending
  on every future contributor remembering it, now for two legal shapes
  instead of one.

## Non-functional requirements

- **Performance** — no image-size or build-time budget is fixed here;
  FR-1's multi-stage build is what keeps the runtime image close to just
  the binary's own size, without asserting a specific number unmeasured.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; no UI.
- **Reliability** — FR-4's `depends_on: condition: service_healthy` is
  this spec's concrete reliability property: `backend` never attempts a
  connection before `postgres` can accept one, removing a class of
  cold-start race that `backend-persistence.md` FR-5's own bounded retry
  would otherwise have to absorb alone.
- **Observability** — no new requirement; the Go server's existing
  stdout-based structured logging is captured by Docker/Compose's own log
  handling without change.

## Domain model

Not applicable — this spec is deployment packaging, not the Alexandryn
domain.

## API and contracts

- **`backend` service ↔ `postgres` service**: `DATABASE_URL`, computed by
  Compose interpolation (FR-4), over the Compose-managed Docker network —
  no port published to the host on either service by default.
- **CI ↔ this spec's artifacts**: `backend-test-harness.md` FR-10 builds
  the `Dockerfile` (FR-1/FR-2/FR-3) and runs `docker compose --profile
  bundled-db up --wait` against `docker-compose.yml` (FR-4), asserting
  the exit code — the only contract between that spec and this one.

## State transitions

Not applicable — this spec describes static build/compose artifacts, not
runtime state. `architecture-system.md`'s FR-1/FR-2 already describe the
container target's process/container relationship; this spec doesn't
restate it.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `postgres` service fails its own healthcheck | Compose's own health status | `docker compose up --wait` exits non-zero, naming which service | `backend` never starts (FR-4's `depends_on` condition), consistent with `backend-persistence.md` FR-5's connect-directly path never being attempted against an unhealthy target |
| `backend`'s `HEALTHCHECK` never reports healthy | Docker's own health status, polled by `--wait` | CI red at the container-target test stage (`backend-test-harness.md` FR-10) | Compose's `--wait` times out and exits non-zero; no network-level symptom, since nothing outside the container was ever expected to observe this directly |
| A future PR adds a `ports:` line to `docker-compose.yml` | FR-6's CI check | Build fails, naming the offending line | Merge blocked before the change reaches `main` — the exact "unreviewed compose edit" risk this spec's Security considerations names |

## Security considerations

**Why the default posture is "unreachable," and why FR-5's default file
doesn't relax it.** Binding `backend` to `0.0.0.0` inside its own
namespace and relying only on the absence of a `ports:` line to keep it
unreachable was considered and rejected for the default file. Loopback/
private-range binding (ADR 0017's Mode B) is enforced by network topology
at bind time: a process bound this way cannot be reached from outside its
own network no matter what else is misconfigured around it. It fails
closed. A `0.0.0.0` bind protected only by "nobody added a `ports:` line"
fails open the moment any one of several ordinary, plausible mistakes
happens: a stray `ports:` entry added while debugging and left in, a
`docker-compose.override.yml` a contributor creates locally and commits
by accident, `network_mode: host` added to solve an unrelated networking
problem, or a future sibling service on the same Compose network that
didn't exist when this guarantee was designed. Under that alternative,
the actual security boundary moves from something the topology guarantees
to something a compose file's continued correctness guarantees — a weaker
property, silently.

ADR 0017 now gives a second, equally fail-closed shape for the case where
a published port is genuinely wanted — Mode A, a validated certificate
loaded in-process, checked at startup by `backend-configuration.md` FR-8
and structurally by FR-6 above. What stays rejected, in both modes, is a
port published on *convention alone* — "nobody added a `ports:` line" was
never the guarantee; either the topology itself is closed, or a verified
certificate is present. FR-6's conditional check and FR-8's startup
validation are what keep this from depending on every future contributor
remembering it, for whichever of the two legal shapes applies.

**FR-6 exists because "we agreed not to" isn't a control.** The same
reasoning applies one level up: FR-5 states the correct default, FR-6 is
what actually enforces it against a contributor who never read this spec.

**Non-root runtime user (FR-2)** — standard container-hardening practice,
directly serving constitution §4's hostile-input posture: if a defect in
request handling (a future feature, not anything in scope today) is ever
exploitable, a non-root process inside the container has less to give an
attacker than root would.

**Build stage never ships (FR-1)** — the Go toolchain, `web/`'s
`node_modules`, and any build-time secret (none exist today, but this
matters once one does) never reach the runtime image, only the compiled
binary.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Not applicable — this spec has no application code of its own |
| Integration | `backend-test-harness.md` FR-10, in full — the only test layer this spec's artifacts are exercised by |
| Contract | N/A |
| CI (static) | FR-6's compose-file guard — a static check, not a runtime test, since the property is "the file was never edited this way," not "the running system behaves correctly" |

## Acceptance criteria

- [ ] `docker build` succeeds and produces a runtime image containing
      only the compiled binary, proven by inspecting the final image's
      layers for absence of the Go toolchain and `web/` source
- [ ] The runtime image runs as a non-root user, proven by inspecting the
      running container's effective UID
- [ ] `docker compose --profile bundled-db up --wait` succeeds against a
      clean checkout with zero *Postgres* configuration, proven in CI
      (`backend-test-harness.md` FR-10) — `OPEN_LIBRARY_USER_AGENT` is
      the one variable CI must still supply explicitly (amendment above),
      not a gap in this criterion
- [ ] `docker compose up` (no profile), with `DATABASE_URL` set to an
      external Postgres, starts `backend` without starting `postgres`,
      proven by checking which containers are running after `up`
- [ ] `curl`/`nc` against the compose network's published ports from the
      host confirms nothing is reachable for `backend` by default, proven
      once, not asserted
- [ ] FR-6's CI check fails a deliberately reintroduced `ports:` line,
      proven with a test commit on a branch, not just claimed
- [ ] Every FR maps to a line in phase 03's own exit criteria

## Open questions

- ~~Non-loopback bind for the container target~~ — resolved by ADR 0017
  and reflected in FR-5/FR-6 above and `backend-configuration.md` FR-8;
  the rule is decided (two fail-closed modes, TLS+auth gated, checked
  structurally in Compose and at process startup), phase 12/13 still owns
  actually building the authentication and certificate/proxy
  configuration surface the rule depends on.
- ~~`/healthz` versus `/readyz` as the `HEALTHCHECK` target~~ — resolved
  2026-08-25 (maintainer): `/readyz`, now fixed as a requirement in FR-3
  above.
- ~~Default `POSTGRES_USER`/`PASSWORD`/`DB` values~~ — resolved
  2026-08-25 (maintainer): `admin`/`admin`/`alexandryn`, `.env`-overridable,
  now fixed as a requirement in FR-4 above.
- **`docker inspect` as a supplementary CI assertion beyond `--wait`'s
  exit code** — named as open in `backend-test-harness.md`'s own Open
  questions; not re-decided here.

## References

- ADR 0015 — the container-hosted target this spec packages, including
  the `DATABASE_URL`/Compose-profile design FR-4 implements exactly
- ADR 0008 — the `web/`-then-`go build` ordering FR-1 follows, and the
  monorepo layout this spec's artifacts live alongside (its own addendum
  already named `Dockerfile`/`docker-compose.yml` as the new files)
- `architecture-system.md` — FR-1/FR-2 (the container target's process
  shape), FR-3 and Security considerations (the loopback requirement
  FR-5/FR-6 here enforce for this target specifically)
- `backend-configuration.md` FR-4 (target-dependent `DATABASE_URL`
  meaning), FR-8 (the loopback bind this spec does not touch)
- `backend-persistence.md` FR-5 — the connect-directly code path this
  spec's `docker-compose.yml` is the production trigger for
- `backend-test-harness.md` FR-10 — the only test this spec's artifacts
  are built for
- `.claude/audits/0002-topology-gap.md` A-02-11 — the amendment-plan item
  this spec closes
- Constitution §4 (hostile input, FR-2's non-root user), §6 (network
  exposure, FR-5/FR-6's loopback enforcement), §9 (dependencies, FR-1's
  minimal runtime image)
