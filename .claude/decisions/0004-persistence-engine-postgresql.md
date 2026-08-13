# 0004. Persistence engine is self-hosted PostgreSQL

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-13 |
| **Deciders** | Maintainer |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 01's open-questions list flagged the persistence engine as undecided,
and speculated that the self-hosted, single-machine context argued for an
embedded engine (e.g. SQLite) — but noted the argument needed writing down,
not assuming.

The maintainer has since decided: PostgreSQL, run as a local instance on the
same host as the Go server, with Supabase used as a development/test
convenience (local stack, migrations, a studio UI) rather than as a shipped
dependency. No cloud Supabase project is part of the product. Plain,
self-hostable PostgreSQL is what a deployed instance runs against.

This decision covers all Alexandryn-domain persistence: library, collections,
reading progress, credentials, and auth/sessions (phase 12). It does not
change the domain/source/metadata boundary in constitution §3 — it decides
what the Alexandryn side of that boundary is stored in.

## Decision

We use PostgreSQL as the persistence engine, self-hosted and local to the
host machine the Go server runs on. The Go server is the only process that
talks to it; LAN clients reach data through the API, never through a directly
exposed database port (constitution §6 — network exposure is opt-in and
authenticated, and that applies to the database same as everything else).

Supabase (self-hosted, or its local CLI/Docker stack) is a tool the project
uses to build and test against — migrations, a local Postgres instance with
extras, a studio UI for inspecting data during development. It is not a
runtime dependency of the shipped application, and no build ships pointed at
Supabase's cloud SaaS. A deployed Alexandryn instance runs against plain
PostgreSQL; anyone self-hosting it does not need a Supabase account.

## Options considered

### Option A — Embedded engine (SQLite)

Pros: zero separate process to run or configure, simplest possible
self-hosting story, matches "one desktop app, one household" scale.

Cons: the project already plans LAN-shared, multi-device access (phase 13,
14) with concurrent readers and writers across a household; SQLite's
single-writer model is a worse fit for that than a client-server engine.
Rejected — the multi-device requirement outweighs the setup simplicity.

### Option B — Cloud-hosted Supabase (SaaS)

Pros: managed backups, managed auth primitives, less operational work.

Cons: directly contradicts the project's self-hosted premise (README: "Not a
cloud service. Your library lives on your machine; nothing is uploaded.") and
constitution §8's intent around reading privacy — a third party would hold
what someone reads. Rejected outright; not compatible with what this project
is.

### Option C — Self-hosted PostgreSQL, Supabase as dev/test tooling only (chosen)

Pros: client-server engine suited to concurrent LAN access, plain PostgreSQL
is well-understood and has no vendor lock-in, Supabase's local tooling speeds
up development (migrations, studio, auth scaffolding to borrow patterns from)
without becoming a production dependency. Stays inside the self-hosted,
loopback-first model.

Cons: one more process to run and package alongside the Go server and
Electron shell (relevant to phase 99 packaging — Docker Compose already
planned). Slightly more setup than an embedded engine for a solo user.

## Consequences

**Good** — supports concurrent access from multiple devices in a household
without a single-writer bottleneck; plain PostgreSQL keeps the project
portable across any self-hosting environment, not tied to a vendor; Supabase's
local tooling gives useful development ergonomics for free.

**Bad** — packaging must now bundle or document a PostgreSQL dependency
(Docker Compose, per the README's planned packaging), which an embedded
engine would have avoided; connection lifecycle, pooling, and migration
tooling become real design surface for phase 01/03 rather than "whatever
`database/sql` plus a driver does by default."

**Neutral** — this does not change the metadata/source/Alexandryn boundary
(§3); it decides storage for the Alexandryn side of it only.

## Reversal cost

Expensive once phase 03 has repositories and migrations built against
PostgreSQL-specific behaviour, cheap right now. Forces itself back open if
concurrent-access assumptions turn out wrong, or if packaging a database
process alongside Electron proves impractical during phase 99.

## Confidence

High on "self-hosted, not cloud" — that follows directly from the project's
stated premise and constitution §6/§8. Medium on "PostgreSQL over an embedded
engine" — the multi-device concurrency argument is real but phase 01's
`architecture-persistence.md` still needs to walk the concrete access
patterns (how many concurrent writers, actually) before this is fully load-bearing
rather than a reasonable default.
