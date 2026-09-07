# Phase 15 — Observability: task list

Plan with context, dependency graph, and decisions:
[`tasks/plan-phase15-observability.md`](plan-phase15-observability.md).

Execute in order. Each task: RED (write the failing tests from the spec FRs)
→ GREEN → Refactor. Stop at every **Gate**. Do not cross **Gate 2** (security
audit) without maintainer sign-off.

Branch: `feat/phase15-observability`. Never `main`.

## Progress

### Tier 0 — ADRs

- [x] ADR 0030 drafted and accepted (`0030-expvar-metrics.md`) — G0-4
- [x] ADR 0031 drafted and accepted (`0031-activity-log-store.md`) — Gate 1
- [x] ADR 0032 drafted and accepted (`0032-redaction-proof-test.md`) — Gate 1

### Gate 1 — Spec scope approval (one per spec; wait before drafting)

- [x] State `backend-observability.md` scope + ADR 0031 + ADR 0032 → wait (approved 2026-09-07)
- [x] `backend-observability.md` drafted, self-reviewed, marked APPROVED
- [x] State `frontend-activity-screen.md` scope → wait (approved 2026-09-07)
- [x] `frontend-activity-screen.md` drafted, self-reviewed, marked APPROVED
- [ ] State `backend-reading-leaderboard.md` scope → wait
- [ ] `backend-reading-leaderboard.md` drafted, self-reviewed, marked APPROVED

### Test plans (before RED — ADR 0016)

- [ ] `backend-observability` test plan written
- [ ] `frontend-activity-screen` test plan written
- [ ] `backend-reading-leaderboard` test plan written

### Tier 1 — Backend observability

- [ ] T1.1 — `expvar` collector (latency histogram, queue depth gauges, pool stats)
- [ ] T1.2 — `system_events` migration (00012)
- [ ] T1.3 — `SystemEventWriter` (job lifecycle hooks, import pipeline)
- [ ] T1.4 — Retention reaper (ticker, DELETE WHERE purge_at < now(), warn on fail)
- [ ] T1.5 — `GET /api/v1/diagnostics` (admin-only, expvar + runtime + uptime)
- [ ] **T1.6 — Redaction CI test RED** (must fail first — import job + auth path)
- [ ] T1.7 — `GET /api/v1/activity/events` (admin-only, last N system_events by library)
- [ ] T1.8 — Pause-all, Cancel, Retry, Clear endpoints (stubs for Tier 2)

### Tier 2 — Activity screen

- [ ] T2.1 — `ActivityScreen` component, Acquisition tab only (no Reading label)
- [ ] T2.2 — Wire to GET /api/v1/activity/events (polling interval per spec)
- [ ] T2.3 — Pause-all / Cancel / Retry / Clear actions wired
- [ ] T2.4 — Nav badge: orange dot when ACTIVE or FAILED items exist
- [ ] T2.5 — Accessibility pass (keyboard, announcements, badge label)

### Tier 3 — Leaderboard

- [ ] **T3.2 — Privacy test RED** (must fail first — asserts no position/percentage in response)
- [ ] T3.1 — `GET /api/v1/library/finished` (members who finished each work)
- [ ] T3.3 — `GET /api/v1/library/leaderboard` (works ranked by finished-count)
- [ ] T3.4 — IDOR test (user A in lib X cannot see user B in lib Y)

### Tier 4 — Review + Audit + Close

- [ ] Code review round 1
- [ ] Code review round 2 (independent)
- [ ] Security audit (`0015-phase15-observability.md`) with four-attacker pass
- [ ] **Gate 2 — post audit findings; wait for maintainer sign-off before close**
- [ ] CI green: Frontend / Backend / Desktop all passing
- [ ] Specs index updated (all three specs → VERIFIED)
- [ ] Roadmap index updated (phase 15 → Closed)
- [ ] ANALYSIS.md updated if any canvas classification changed
- [ ] PR opened, body under CLAUDE.md "Reporting back" headings
