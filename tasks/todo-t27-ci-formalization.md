# T27 — CI workflow formalization (D2/D3/D7 formalized)

## Decisions

- [ ] T27-D1 — fixed container-internal `BIND_ADDRESS=127.0.0.1:8080` for `backend` (ephemeral default won't work for `HEALTHCHECK`)
- [ ] T27-D2 — runtime base image: `alpine`, not distroless (needs `wget` + a shell)
- [ ] T27-D3 — conditional `web/` build step in the `Dockerfile`, matching D2/T16's placeholder precedent
- [ ] T27-D4 — FR-6's CI guard is a new script, `scripts/check-compose-published-port.sh`, matching `check-import-boundaries.sh`'s shape
- [ ] T27-D5 — the three "proven once, not asserted" acceptance items stay one-time proofs, not new permanent CI

## Tasks

- [ ] T27-1 — `Dockerfile`: multi-stage, conditional `web/` build, `alpine` runtime, non-root user, `HEALTHCHECK` against `/readyz`

**Checkpoint T27-A** — `docker build .` succeeds; no build toolchain in the runtime image; container runs as non-root; `/readyz` answers inside the container

- [ ] T27-2 — `docker-compose.yml`: `backend` + `postgres` services, profile, named volume, `depends_on: service_healthy`, no `ports:`
- [ ] T27-3 — extend root `.env.example` with `POSTGRES_USER`/`PASSWORD`/`DB` and their defaults

**Checkpoint T27-B** — `docker compose --profile bundled-db up --wait` succeeds from a clean checkout, zero configuration

- [ ] T27-4 — `scripts/check-compose-published-port.sh` + self-test (fixture-based)

- [ ] T27-5 — `ci.yml` stage 6: named no-op contract-test step
- [ ] T27-6 — `ci.yml` stage 9: container-target test in the same `backend` job
- [ ] T27-7 — `ci.yml` header comment updated (stage 1 still absent, phase 04; stages 6/9 now real)

**Checkpoint T27-C** — full `ci.yml` green on a real PR, including both new stages

- [ ] T27-8 — one-time proofs: external-`DATABASE_URL` compose-up starts only `backend`; host can't reach it; T27-4's check fails a reintroduced `ports:` fixture
- [ ] T27-9 — D6: inspect `main`'s real branch-protection settings, confirm `Backend` is required; report, fix only on explicit confirmation

- [ ] T27-10 — docs: check off `deployment-container-packaging.md`'s proven Acceptance criteria; `tasks/todo.md` checks off T27/D6

**Checkpoint T27-D (final)** — full suite green: build/vet, unit-race, integration-race-default-parallelism, golangci-lint, govulncheck, every check script (incl. T27-4's new one), `gh pr checks` green on the real workflow — T27 complete
