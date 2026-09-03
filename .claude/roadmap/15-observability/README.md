# Phase 15 — Observability

*Outline — expanded to a full phase document when phases 09, 13 close.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 09, Phase 13 |
| **Blocks** | 16 |

## Objective

Observability maturity beyond phase 03's baseline: metrics, queue depth,
system diagnostics, and the Activity screen — with redaction proven, not
assumed (Constitution §8).

## Scope

**In**

- Metrics collection (latency, queue depth, DB pool), health/diagnostics
  endpoint
- Activity log store and UI
- Automated proof that credentials, tokens, and reading history never reach
  logs or metrics

**Candidate (not yet claimed — needs its own spec + scope gate)**

- **Library-visible reading activity.** After a user *finishes* a book,
  other members of the same library see it marked as read by them, and a
  per-library **most-read leaderboard** — **without exposing anyone's
  reading position or progress percentage**. Raised by the maintainer
  during phase 13 (review 0050). It is a distinct read model layered on
  the per-user-private reading data: the phase-13 hardening keeps
  `user_id` + `library_id` on every progress row, so a
  `WHERE work_id = $1 AND library_id = $2 AND percentage >= 100`
  aggregate over the shared library is a straightforward addition. The
  privacy line is load-bearing — "completed" is shareable within a
  library; position, current chapter, and time-remaining are not
  (`domain-reading.md`, constitution §8). Deferred here, not designed.

**Out**

- Any external telemetry service — self-hosted stays local.
- Exposing any reading *position* or *progress percentage* to anyone but
  the reading user — the leaderboard candidate above shares only the
  binary "finished" fact, per library.

## Exit criteria

- [ ] Activity screen shows real system events, job status, import history
- [ ] Automated test proves zero sensitive data in logs, metrics, or diagnostics
- [ ] Maintainer approval recorded
