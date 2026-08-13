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

**Out**

- Any external telemetry service — self-hosted stays local.

## Exit criteria

- [ ] Activity screen shows real system events, job status, import history
- [ ] Automated test proves zero sensitive data in logs, metrics, or diagnostics
- [ ] Maintainer approval recorded
