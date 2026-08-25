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

- [x] T27-8 — one-time proofs, all done against real podman: `docker compose up` (no profile) with an external `DATABASE_URL` starts only `backend`, DSN untouched; `curl`/`nc` from the host confirms nothing reachable on either service's port; T27-4's check fails when a `ports:` line is reintroduced into a real copy of the actual `docker-compose.yml` (not just the self-test's synthetic fixture)
- [x] T27-9 — D6: inspected `main`'s real branch protection — **genuinely unavailable**, not just unconfigured: `gh api repos/.../branches/main/protection` and the newer `.../rules/branches/main` both return 403 "Upgrade to GitHub Pro or make this repository public to enable this feature." This is a private repo on GitHub's free tier, which doesn't include branch protection or rulesets at all. Real consequence: **`main` is not actually gated by CI today** — nothing technical stops a direct push bypassing every check this workflow runs. Not fixable via `gh api`; the only paths are a paid plan or making the repo public, both outside this task's authority — reported to the user, not silently worked around or checked off as passing

- [x] T27-10 — docs: checked off `deployment-container-packaging.md`'s 7 Acceptance criteria (all proven), the phase 03 exit-criteria line for this spec's approval, `tasks/todo.md` (T27 + D6, with D6's real finding), a new Carry-over for the branch-protection plan-tier gap

**Checkpoint T27-D (final)** — full suite green locally: build/vet (incl. `-tags=integration`), unit-race, integration-race-default-parallelism (T26), golangci-lint (0 issues), govulncheck (clean), every check script including T27-4's new one, the exact stage-9 command proven end to end. Real `gh pr checks` on the actual workflow is this task's last remaining proof, via the PR this build closes with — T27 complete pending that.
