# T27 — CI workflow formalization (D2/D3/D7 formalized)

## Overview

`tasks/todo.md`'s own T27 line: "CI workflow (D2/D3/D7 formalized)."
`backend-test-harness.md` FR-8 fixes nine ordered CI stages;
`.github/workflows/ci.yml`'s own header comment names three still
absent: stage 1 (`web/` build, blocked on phase 04 — no `web/` directory
exists), stage 6 (contract test, blocked on D3), stage 9
(container-target test, blocked on D7). D7 is resolved this session —
the maintainer approved `deployment-container-packaging.md` (`REVIEWED`
→ `APPROVED`), and its two open questions (HEALTHCHECK target, default
Postgres credentials) are decided. T27 builds what that approval
unblocks: stage 6 as a named no-op (D3), stage 9 for real (the
`Dockerfile`/`docker-compose.yml`/FR-6 CI guard the spec requires), and
D6's branch-protection live-gating verification. Stage 1 stays out of
scope — genuinely blocked on phase 04, not this task's to fake.

## Decisions (resolve before/alongside the tasks that need them)

- **T27-D1 — fixed container-internal `BIND_ADDRESS`, not the
  ephemeral default.** Surfaced during research, not in the
  spec text: `internal/config`'s own `BIND_ADDRESS` default is
  `127.0.0.1:0` (`internal/config/config.go:183`) — an ephemeral port.
  A `Dockerfile HEALTHCHECK` needs a *fixed* port to curl/wget against.
  `validateBindAddress` (`internal/config/bindaddress.go`) also only
  accepts loopback/private hosts without TLS — `0.0.0.0` is neither
  (`net.IP.IsLoopback`/`IsPrivate` both false for it), so the container
  can't bind wide even internally, no code change needed to keep that
  true. **Chosen:** `docker-compose.yml` sets `BIND_ADDRESS=127.0.0.1:8080`
  explicitly for `backend`. Still loopback (legal today, no TLS
  needed), still satisfies FR-5 (nothing outside the container's own
  namespace was ever going to reach a Compose-internal port), gives
  `HEALTHCHECK`/FR-3 a stable target. Port `8080` is an arbitrary,
  container-internal-only choice — no existing convention claims it,
  and nothing outside this container's own namespace ever sees it.
- **T27-D2 — runtime base image: `alpine`, not distroless.**
  FR-3 allows `curl` or `wget`, "whichever is already present... or
  added at negligible size cost." `alpine`'s busybox already ships
  `wget` — zero extra install, small image, and (unlike
  `distroless/static`) still has a shell for the non-root user setup
  and a working `HEALTHCHECK` executable. Rejected: `distroless`,
  smaller but ships no HTTP client and no shell, which would make FR-3
  and FR-2's own setup steps harder for no real size win at this
  project's scale.
- **T27-D3 — conditional `web/` build in the `Dockerfile`, matching
  D2's own placeholder precedent.** FR-1 requires the builder stage run
  `web/`'s build then `go build ./cmd/server`, in that order, once
  `web/` exists (phase 04). It doesn't exist yet. `RUN if [ -d web ];
  then cd web && npm ci && npm run build; fi` satisfies FR-1's ordering
  the moment phase 04 lands, without breaking `docker build` today —
  `go build ./cmd/server` already works standalone against the
  committed placeholder `embed.FS` (T16, D2). Same "build against
  what's committed now, swap later" pattern, not a new one.
- **T27-D4 — FR-6's CI guard is a new script,
  `scripts/check-compose-published-port.sh`, matching
  `check-import-boundaries.sh`'s shape exactly** (self-test companion,
  same grep/awk-based interim style, same reasoning for staying interim
  until `golangci-lint`/a real YAML tool replaces it). Checked
  structurally (key presence for Mode A's `TLS_CERT_FILE`/`TLS_KEY_FILE`
  + an auth-enabled env setting), not semantically — FR-6's own text is
  explicit that certificate validity is `backend-configuration.md`
  FR-8's job, not this check's.
- **T27-D5 — the three Acceptance-criteria items the spec itself marks
  "proven once, not asserted" stay one-time, not new permanent CI.**
  (`docker compose up` with an external `DATABASE_URL` starts only
  `backend`; `curl`/`nc` from the host confirms nothing reachable;
  FR-6's check fails against a deliberately reintroduced `ports:`
  fixture.) Run for real in this session (podman's Docker-CLI/Compose
  emulation, confirmed working), recorded in the PR, not built as
  ongoing test infrastructure — that's what the spec's own wording
  asks for, not a gap.

## Task list

**Tier 0 — `Dockerfile` (FR-1/FR-2/FR-3, T27-D1/D2/D3)**

- [ ] T27-1 — `Dockerfile`: multi-stage build, conditional `web/` build
  step, `go build ./cmd/server`, `alpine` runtime stage, non-root user
  (`addgroup -S app && adduser -S app -G app`, `USER app`),
  `HEALTHCHECK CMD wget -qO- http://127.0.0.1:8080/readyz || exit 1`.
  RED first: a `docker build .` that fails/doesn't exist, then build it
  green; inspect the final image for absence of the Go toolchain/`web/`
  source and confirm the running container's effective UID is non-root
  (spec's own Acceptance criteria, first two items).

  > **Checkpoint T27-A** — `docker build .` succeeds; runtime image has
  > no build toolchain; container runs as non-root; `/readyz` answers
  > inside the container (`docker exec` a curl/wget against it).

**Tier 1 — `docker-compose.yml` + `.env.example` (FR-4/FR-5, T27-D1)**

- [ ] T27-2 — `docker-compose.yml`: `backend` (`build: .`,
  `BIND_ADDRESS=127.0.0.1:8080`, `DATABASE_URL` computed from
  `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB` via Compose
  interpolation, `depends_on: { postgres: { condition: service_healthy
  } }`, no `ports:`), `postgres` (official image, `profiles:
  ["bundled-db"]`, named volume, its own healthcheck).
- [ ] T27-3 — extend the existing root `.env.example` with a new
  section documenting `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB`
  and their `admin`/`admin`/`alexandryn` defaults — one file, not a
  second one.

  > **Checkpoint T27-B** — `docker compose --profile bundled-db up
  > --wait` succeeds from a clean checkout, zero configuration
  > (spec's own Acceptance criteria, third item).

**Tier 2 — FR-6's CI guard (T27-D4)**

- [ ] T27-4 — `scripts/check-compose-published-port.sh` +
  `check-compose-published-port_test.sh`: fails on a `ports:` entry
  under `backend` or `network_mode: host`, unless
  `TLS_CERT_FILE`/`TLS_KEY_FILE` + an auth-enabled setting are also
  present in the same file (structural check only). RED first: fixture
  compose files (one deliberately bad, one good, one Mode-A-legal) the
  self-test asserts against, written before the script exists.

**Tier 3 — `ci.yml` stages 6 and 9**

- [ ] T27-5 — stage 6: named no-op step ("Contract test (not yet
  implemented)"), explains why, exits 0.
- [ ] T27-6 — stage 9, same `backend` job (FR-10's own "no separate
  job" text): `docker build .`, T27-4's check, `docker compose
  --profile bundled-db up --wait`, asserting exit code.
- [ ] T27-7 — update the workflow's own header comment: stage 1 stays
  noted absent (phase 04); stages 6 and 9 move from "absent" to "real."

  > **Checkpoint T27-C** — full `ci.yml` green on a real PR, including
  > the two new stages.

**Tier 4 — one-time proofs + D6 (T27-D5)**

- [ ] T27-8 — run and record: `docker compose up` (no profile, external
  `DATABASE_URL`) starts only `backend`; `curl`/`nc` from the host
  confirms nothing reachable; T27-4's check fails against a
  deliberately reintroduced `ports:` fixture (proving the real check,
  not just its unit-level self-test).
- [ ] T27-9 — D6: inspect `main`'s real GitHub branch-protection
  settings (`gh api repos/.../branches/main/protection`), confirm the
  required check list actually includes `Backend`. Report the finding;
  fix via `gh api` only if a real gap exists and the user confirms —
  changing branch protection is a shared-repo setting, not a routine
  code change.

**Tier 5 — docs closure**

- [ ] T27-10 — check off `deployment-container-packaging.md`'s
  Acceptance criteria items proven in Tiers 0/1/4; `tasks/todo.md`
  checks off T27 and D6, closes Checkpoint H's remaining pieces.

  > **Checkpoint T27-D (final)** — full suite green: `go build/vet
  > ./...`, `go test -race ./...`, `go test -race -tags=integration
  > ./...` (default parallelism, per T26), `golangci-lint run ./...`,
  > `govulncheck ./...`, every existing check script plus T27-4's new
  > one, `gh pr checks` green on the real workflow — done. T27 complete,
  > Checkpoint H's own final walk (roadmap exit criteria, D5 cleanup PR)
  > still separate, not this task's to close alone.

## Critical files

- `.claude/specs/deployment-container-packaging.md` — FR-1 through
  FR-6, already `APPROVED`, this plan's primary source
- `internal/config/bindaddress.go`, `internal/config/config.go:183` —
  the `BIND_ADDRESS` ephemeral-default/loopback-only validation T27-D1
  routes around, unchanged by this task
- `cmd/server/main.go:139-140` — `/healthz`/`/readyz` route
  registration, confirms FR-3's target exists today
- `.env.example` — extended, not replaced
- `.github/workflows/ci.yml` — stages 6/9 inserted, header comment
  updated
- `scripts/check-import-boundaries.sh` + `_test.sh` — the self-test
  pattern T27-4 mirrors
- `internal/transport/http/webdist/placeholder/` (T16, D2) — the
  "build against what's committed, swap when phase 04 lands" precedent
  T27-D3 follows

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| `docker`/`docker compose` in this environment is podman's emulation layer, not real Docker | Low-medium — behavior could differ subtly from GitHub Actions' real Docker | Confirmed working (`docker --version`/`docker compose version` both respond) before relying on it; first real-Docker proof is CI itself once pushed, named explicitly if anything differs |
| `alpine`'s `wget` behavior (`-qO-`) differs from a fuller `curl` in edge cases | Low — `HEALTHCHECK` only needs a 2xx/non-2xx distinction | `wget`'s own exit code already does this; no response-body parsing needed |
| FR-6's structural check could have false positives/negatives on YAML shapes it doesn't anticipate | Low — same accepted interim-tool risk `check-import-boundaries.sh` already carries | Same grep/awk-based, self-tested pattern already in production use elsewhere in this repo |
| D6's branch-protection change (if a real gap is found) is a shared-repo setting | Medium if acted on without confirmation | Report-then-ask, not report-then-act — explicit in T27-9 |

## Open questions

- Whether `main`'s actual branch protection already requires the
  `Backend` check — genuinely unknown until T27-9 inspects it for real,
  not assumed either way.
- Whether podman's Compose emulation matches GitHub Actions' real
  Docker closely enough that Tier 0-2's local proofs fully predict CI
  — stated as a real gap in Risks, not glossed over.
