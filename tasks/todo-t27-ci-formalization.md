# T27 — CI workflow formalization (D2/D3/D7 formalized)

## Decisions

- [ ] T27-D1 — fixed container-internal `BIND_ADDRESS=127.0.0.1:8080` for `backend` (ephemeral default won't work for `HEALTHCHECK`)
- [ ] T27-D2 — runtime base image: `alpine`, not distroless (needs `wget` + a shell)
- [ ] T27-D3 — conditional `web/` build step in the `Dockerfile`, matching D2/T16's placeholder precedent
- [ ] T27-D4 — FR-6's CI guard is a new script, `scripts/check-compose-published-port.sh`, matching `check-import-boundaries.sh`'s shape
- [ ] T27-D5 — the three "proven once, not asserted" acceptance items stay one-time proofs, not new permanent CI

## Tasks

- [x] T27-1 — `Dockerfile`: multi-stage, conditional `web/` build, `alpine` runtime, non-root user, `HEALTHCHECK` against `/readyz` — built and proven against real podman (`--format docker` needed; the default OCI format silently drops `HEALTHCHECK`, noted for T27-6)

**Checkpoint T27-A** — done: `docker build .` succeeds; no build toolchain/`/src` in the runtime image; container runs as non-root (uid=100); `/readyz` answers and correctly 503s while waiting for Postgres. Real finding while proving this: the app requires `OPEN_LIBRARY_USER_AGENT` (`backend-configuration.md` FR-3, no default by design — a placeholder would misidentify the client to Open Library's live API) — T27-2 needs to account for this, not silently default it in `docker-compose.yml`

- [x] T27-2 — `docker-compose.yml`: `backend` + `postgres` services, profile, named volume, `depends_on: service_healthy, required: false`, no `ports:` — the `required: false` was itself a real finding (unconditional `depends_on` on a profiled service breaks no-profile `up` outright, confirmed empirically)
- [x] T27-3 — extend root `.env.example` with `POSTGRES_USER`/`PASSWORD`/`DB` and their defaults, plus a note on `OPEN_LIBRARY_USER_AGENT`'s no-default requirement

**Checkpoint T27-B** — done, proven against real podman (Compose 5.4.0): `--profile bundled-db up --wait` reaches Healthy on both services from a clean checkout (temporarily moved this machine's own pre-existing dev `.env` aside to prove it honestly), migrations ran, `/readyz` returned 200, nothing reachable from the host on either port; plain `up` with an external `DATABASE_URL` started only `backend`, DSN untouched. Real hazard found and documented in `docker-compose.yml`: a pre-existing local `.env` (this project's own `go run ./cmd/server`-against-Supabase convention) shares the `DATABASE_URL` key and silently wins over the bundled DSN if present.

- [x] T27-4 — `scripts/check-compose-published-port.sh` + self-test (fixture-based) — 4/4 cases green; Mode A's carve-out not implemented (no "authentication enabled" key exists anywhere yet, noted in the script)

- [x] T27-5 — `ci.yml` stage 6: named no-op contract-test step
- [x] T27-6 — `ci.yml` stage 9: container-target test in the same `backend` job — also wired T27-4's check into stage 3
- [x] T27-7 — `ci.yml` header comment updated (stage 1 still absent, phase 04; stages 2-9 now real)

**Checkpoint T27-C** — locally proven: the exact stage 9 command (`docker compose --profile bundled-db up --build --wait`) succeeds end to end against real podman with no pre-existing `.env` (the real CI condition), both services Healthy, exit 0. Real `gh pr checks` proof pending the PR (T27's own make-pr step).

- [ ] T27-8 — one-time proofs: external-`DATABASE_URL` compose-up starts only `backend`; host can't reach it; T27-4's check fails a reintroduced `ports:` fixture
- [ ] T27-9 — D6: inspect `main`'s real branch-protection settings, confirm `Backend` is required; report, fix only on explicit confirmation

- [ ] T27-10 — docs: check off `deployment-container-packaging.md`'s proven Acceptance criteria; `tasks/todo.md` checks off T27/D6

**Checkpoint T27-D (final)** — full suite green: build/vet, unit-race, integration-race-default-parallelism, golangci-lint, govulncheck, every check script (incl. T27-4's new one), `gh pr checks` green on the real workflow — T27 complete
