# 0030. Use Go stdlib `expvar` for in-process metrics collection

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-07 |
| **Deciders** | Maintainer (G0-4, 2026-09-07) |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 15 adds metrics collection (request latency, job queue depth, database
pool utilisation) to Alexandryn's Go server. The project's scope rules out any
external telemetry service — no Prometheus exporter, no OTEL collector, no
Datadog agent — because Alexandryn is self-hosted and must not require the user
to run additional infrastructure (Scope Out, `roadmap/15-observability/README.md`).

The metrics must be queryable by the `/api/v1/diagnostics` endpoint (admin-only)
and by the Electron host for its own status display. The system operates at
household scale: one host, a handful of paired devices, a few concurrent users
at most. No metric is ever shipped to a remote endpoint.

Three options were considered before the maintainer decided at Gate 0.

## Decision

We use Go's standard library `expvar` package for in-process metrics. `expvar`
exposes a package-level map of named variables (counters, floats, maps) readable
via a JSON endpoint (`/debug/vars`). We read that map directly in the
`/api/v1/diagnostics` handler rather than mounting the `expvar` default
`/debug/vars` endpoint.

Concretely:
- Request latency: an `expvar.Map` keyed by route template, each value a
  JSON-serialisable histogram struct (count, sum, p50, p95, p99 computed on
  request completion).
- Job queue depth: `expvar.Int` per state (`queued`, `running`, `retrying`,
  `dead_letter`), incremented/decremented by the job engine's state-change hooks.
- DB pool utilisation: `expvar.Int` for acquired/idle/max connections, polled
  from `pgxpool.Pool.Stat()` at diagnostics-read time (not a background goroutine
  — the stat call is cheap and synchronous).

No new Go modules. The histogram computation is ~30 lines of plain Go added to
`internal/observability/`.

## Options considered

### Option A — Go stdlib `expvar` ✓ chosen

**Pros:** Zero new dependencies. Already in every Go binary. Exported via a
standard JSON structure. Readable from any HTTP client or from Go code directly
(`expvar.Get`). Thread-safe by construction (uses `sync.RWMutex` internally for
maps; `expvar.Int` and `expvar.Float64` use `atomic`).

**Cons:** No built-in histogram type — we write a small one (~30 lines). The
`/debug/vars` path it defaults to is not mounted on the public API (we read the
map directly). Metric names are untyped strings; no schema enforcement beyond
our own discipline.

**Why chosen:** The project's self-hosting constraint means no collector will
ever be attached; `expvar`'s design is exactly right for in-process, on-demand
reads. The household scale makes histogram precision a non-issue.

### Option B — Small custom counter/histogram store

**Pros:** Full control over the schema; no dependency at all.

**Cons:** Reimplements `sync/atomic` and mutex-protected maps that `expvar`
already provides correctly. Writing a correct concurrent histogram from scratch
is not the simplest thing to test. More code to own.

**Why not chosen:** `expvar` already provides the thread-safe primitives we need.
Writing our own is strictly more work for no additional gain.

### Option C — `prometheus/client_golang`, in-process only

**Pros:** Rich histogram support (configurable buckets, summaries). Familiar to
operators who use Prometheus.

**Cons:** Adds a ~200 KB dependency (the whole Prometheus client library) and its
transitive imports. The code reads as though it integrates with Prometheus even
though no collector is attached. At household scale, the additional histogram
precision is immaterial.

**Why not chosen:** Constitution §9 requires recording a reason for every new
dependency — "a richer histogram type at household scale" is not a sufficient
reason to add ~200 KB of dependency. `expvar` satisfies the requirement without
the cost.

## Consequences

**Good:** No new module, no new infrastructure, no new process. Metrics readable
directly from Go code in tests. The diagnostics handler owns the serialisation
format — it can evolve independently of any external metric schema.

**Bad:** We own a small (~30-line) histogram implementation. If the project ever
grows to a scale where Prometheus is warranted, migrating off `expvar` requires
rewriting the collector hooks. However, at that scale the decision to add the
Prometheus dependency would also change, so this is not a latent cost — it is a
latent decision.

**Neutral:** The `/debug/vars` default HTTP handler is not mounted. We read the
`expvar.Map` directly in the diagnostics handler. This is deliberate — the
default path is unauthenticated and would violate the admin-only constraint.

**Addendum (2026-09-10, audit `0016` #302):** For the first phase-15 cut the
`alexandryn_metrics` map was created but never written — `Registry.Snapshot`
built its response struct straight from the in-memory histograms and the map
stayed empty. `Snapshot` now also writes `latencies` / `queue_depth` /
`db_pool` into that map (JSON-valued vars) at read time, so ADR 0030's stated
"read the `expvar.Map` directly" mechanism is real. The latency histogram
gained a `+Inf` overflow bucket in the same change: a request slower than the
10s top bound was previously counted in `count`/`sum` but in no bucket.

## Reversal cost

Low. `expvar` calls are isolated to `internal/observability/`. Replacing the
collector means replacing one package; the diagnostics handler serialisation and
the job-engine hook call sites change, but nothing else does. A future migration
to Prometheus or OTEL would be a clean swap, not a refactor.

## Confidence

High. The choice was made with full knowledge of the alternative options and
the scale constraints. Maintainer decided at Gate 0 after reviewing the options
in the phase document.
