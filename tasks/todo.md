# Phase 03 (Backend foundation) — task list

Full plan with context/approach/rationale: [`tasks/plan.md`](plan.md).
Execute in order; each task is RED (existing test-plan case, see
`.claude/test-plans/`) → GREEN → Refactor. Stop at every lettered checkpoint.

## Decisions (resolve once, don't re-derive mid-task)

- [ ] D0 — import-boundary lint mechanism: interim grep/file-walk script
- [ ] D1 — phase 02 gating: re-check at Checkpoint G, don't assume
- [ ] D2 — `web/dist` placeholder embed for FR-8 (T16)
- [ ] D3 — CI "contract test" stage as a named no-op (T27)
- [ ] D4 — macOS `pg-supervisor` E2E: portable subset only in-phase (T11); full E2E needs a real macOS runner
- [ ] D5 — `backend-configuration.md` stale FR-8 phrasing cleanup (3 spots, alongside T6)
- [ ] D6 — branch-protection live-gating verification (infra, not code; Checkpoint H)
- [ ] D7 — get `deployment-container-packaging.md` to `APPROVED` before T27 relies on it

## Tasks

**Tier 0**
- [ ] T0 — module bootstrap + D0's lint script
- [ ] T1 — `internal/testutil`: Clock/FS/IDGenerator fakes + spy handler
- [ ] T2 — build-tag / `TEST_DATABASE_URL` convention

**Checkpoint A** — T1+T2 green, `go vet ./...` clean, before anything else

- [ ] T3 — `internal/config` core (FR-1/2/3/4/6)
- [ ] T4 — `internal/config` FR-5 (file resolution)
- [ ] T5 — `internal/config` FR-7 (DATABASE_URL redaction)
- [ ] T6 — `internal/config` FR-8 (BIND_ADDRESS, current ADR 0017 rule) + D5

**Checkpoint B** — `internal/config` fully green before `internal/logging`

- [ ] T7 — `internal/logging` + `internal/domain/error.go` bootstrap
- [ ] T8 — `internal/persistence/postgres` FR-1 (pool)
- [ ] T9 — `internal/persistence/postgres` FR-3 (parameterized-query static check)

**Tier 1**
- [ ] T10 — FR-6 migration runner
- [ ] T11 — FR-5 branch selection + FR-8 spawn-args (portable subset, D4)

**Checkpoint C** — restate D1, skip repositories, continue to transport/lifecycle

- [ ] T12 — FR-6/FR-7 Integration (no domain rows needed)

**Tier 2**
- [ ] T13 — transport FR-1/FR-3/FR-4 (Unit)
- [ ] T14 — transport FR-2/FR-6/FR-7 (limits, Content-Type, 404, wiring)
- [ ] T15 — transport FR-5 (healthz/readyz)

**Checkpoint E** — transport Unit layer green; smoke-test all six error categories

- [ ] T16 — transport FR-8 (embedded web/dist fallback, D2)

**Tier 3 (highest risk)**
- [ ] T17 — `cmd/server` FR-1 steps 1-4 + FR-2 + FR-7
- [ ] T18 — `cmd/server` FR-1 steps 5-6 + FR-3 (repo stub per D1, explicit TODO)
- [ ] T19 — `cmd/server` FR-4/5/6 (graceful shutdown)

**Checkpoint F** — Unit-layer fakes reviewed before real-listener/`-race` layers

- [ ] T20 — `cmd/server` Integration + Concurrency (`-race`)

**Tier 4**
- [ ] T21 — errors-and-logging FR-3/5/7/10/11 against the real chain
- [ ] T22 — harness FR-3 Variant A, schema-at-head, parallelism static check

**Checkpoint G — domain-readiness gate.** Re-check phase 02 status directly.
If not ready: stop and ask (pause T24-T26, or explicit provisional scaffold
— don't pick silently).

- [ ] T24 — `backend-persistence.md` FR-2, all 11 repositories
- [ ] T25 — remaining persistence Integration/E2E (macOS E2E excluded, D4)
- [ ] T26 — harness FR-3 Variant B + real repository wiring in `run.go`
- [ ] T27 — CI workflow (D2/D3/D7 formalized)

**Checkpoint H (final)** — D6 verification, D5 cleanup PR, full-suite run,
roadmap exit-criteria walked item by item (including fixing the exit-criteria
line that still describes the pre-ADR-0017 loopback-only rule).

## Carry-overs — noted, not acted on this pass

- [ ] **FR-8 Mode B**: confirm the authentication condition is asserted
  *independently* of the address classification, not implied by it —
  i.e. a loopback/private bind must not be treated as "therefore
  authenticated" anywhere in the startup path; the two checks are separate
  and both must actually run. Check when T6/T18 are implemented, not now.
- [ ] **Phase 13's CSRF item is blocked on phase 12's session-mechanism
  ADR** (the ADR `roadmap/12-authentication/README.md` now names as
  in-scope, per the 2026-08-18 session). Nothing to do until phase 12
  actually opens and that ADR exists.
