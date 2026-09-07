# Phase 15 — Observability

| | |
|---|---|
| **Status** | In progress |
| **Depends on** | Phase 09, Phase 13 |
| **Blocks** | 16 |
| **Opened** | 2026-09-07 |
| **Closed** | — |

## Gate 0 decisions (2026-09-07, maintainer)

| # | Decision |
|---|---|
| G0-1 | Reading tab: remove the tab label entirely until there is drawn content — this phase does not build it. |
| G0-2 | `/api/v1/diagnostics` endpoint: backend only, no UI. Not a design-conformance question; proceed. |
| G0-3 | Library-visible leaderboard: include in phase 15 with its own spec and a named privacy test. |
| G0-4 | Metrics mechanism: Go stdlib `expvar`. ADR 0030 records this as Accepted. |

## Dependency status at open (verified, not trusted from roadmap)

**Phase 09 (Async jobs):** The roadmap status line reads "Specs approved,
implementation not started" — stale. The spec index shows `backend-job-queue.md`
as `IMPLEMENTED` (branch `feat/phase09-async-jobs`, 2026-09-01; independent
review `0036`, findings fixed; security audit `0009` clear). Phase 09 commits
(`993e223`, `c48262d`, `b28ab38`, `b3e0a80` in `origin/main`) confirm the job
queue, engine, and status API are in `main`. The branch
`origin/feat/phase09-async-jobs` is not merged (as a Git merge commit) into
`main` — code arrived via commits that were also merged through subsequent
phases. Spec status is `IMPLEMENTED`; `VERIFIED` is pending a formal maintainer
close. **This phase depends on the job queue infrastructure being present and
tested in `main`, which it is. The open formal-close is not a blocker for phase
15, but the stale roadmap status line is corrected here for the record.**

**Phase 13 (Network access):** Closed — PR #80, recorded in `main` by
`454af67` ("docs(roadmap): close phase 13, record its merge to main") on
2026-09-05. All exit criteria met; security audit `0013` recorded with no open
Critical/High findings. Four specs `APPROVED`; ADR 0028 `Accepted`. Verified
from git. Status confirmed. (An earlier draft of this line cited `369190f` as a
"merge commit" — that commit is an unrelated staticcheck fix; the repo uses
rebase/squash merges, not merge commits.)

**Phase 14 (Devices and sync):** Closed — PR #82 merged to `main`
(2026-09-07), including the post-review sync-hardening fixes (`a3b9753`,
`01ecbb9`) and audit `0014`. Phase 14 is not a dependency of phase 15; noted
for context.

## Objective

Observability maturity beyond phase 03's baseline: structured metrics collection
(request latency, job queue depth, database pool utilisation), a
health-and-diagnostics endpoint queryable by the desktop host and operators, the
Activity screen backed by real system events and job state, and automated proof
that no credential, session token, reading content, or precise reading position
ever reaches a log line or a metric label. Redaction is proven by a CI test, not
assumed.

## Why here

Phase 03 established the baseline: structured logs, request IDs, a `/health`
endpoint, and useful failure messages from the first line of server code
(constitution §8). Phases 09–14 added the things worth observing: background
jobs with state machines (phase 09), LAN exposure with rate limiting and
security headers (phase 13), device sync events (phase 14). Phase 15 builds the
surface that ties these together into something an operator and the desktop host
can see and act on.

Running earlier would have produced a dashboard with nothing in it — no job
states to show, no sync events, no network-exposure conditions to surface. Phase
09 must exist for queue-depth metrics to mean anything; phase 13 must exist for
the Activity screen to show acquisition jobs from LAN-paired sources.

Running later risks phase 16 (security hardening) auditing a surface that
includes sensitive operational data with no proven redaction discipline. The §8
test must pass before the hardening phase begins.

## Scope

**In**

- Metrics collection: request latency (histogram by route), job queue depth by
  state (`queued`, `running`, `retrying`, `dead_letter`), database pool
  utilisation — collected in-process using Go stdlib `expvar` (G0-4). No
  external telemetry service.
- A `/api/v1/diagnostics` endpoint, authenticated, admin-only, backend only with
  no UI in this phase (G0-2): returns the current `expvar` snapshot plus server
  uptime, Go runtime stats, and build version.
- Activity log store: a `system_events` table recording job lifecycle events
  (enqueued, started, completed, failed, dead-lettered), import history, and
  source acquisition events. Admin-only reads. Retention policy: configurable,
  defaulting to 30 days.
- Activity screen UI (Electron/Host only, `atActivity`): **Acquisition tab only**
  (ACTIVE / QUEUED / FAILED / COMPLETED sections, with Pause-all and
  Clear-completed actions). The Reading tab label is not rendered in this phase
  (G0-1). The orange nav badge reflects live state.
- Automated redaction proof: a CI test that captures structured log output over
  a real import-job and auth path and asserts that no credential, session token,
  home-directory path, book title, reading position, or percentage value appears
  in any log line or metric label.
- **Library-visible reading activity / most-read leaderboard** (G0-3): after a
  library member finishes a book (`percentage >= 100`), other members of the
  same library see it marked as read and a per-library most-read count. Only the
  binary "finished" fact is shared — never reading position, current chapter,
  percentage, or time-remaining. Requires its own spec and a named privacy test
  proving position never reaches the aggregate.

**Out**

- Any external telemetry service.
- Exposing reading position, current chapter, time-remaining, or percentage to
  anyone but the reading user — the leaderboard shares only "finished", nothing
  else.
- The `/api/v1/health` endpoint already built in phase 03 — this phase extends
  alongside it.
- Mobile or Web-viewer Activity screen — `atActivity` is Electron-only per
  ANALYSIS.md.
- Reading tab content and label — removed entirely until a canvas exists (G0-1).


## Design conformance

**Canvas consulted:** `Alexandryn-Electron.dc.html` (the Electron/Host canvas
per ADR 0003). Re-read fresh on 2026-09-07. ANALYSIS.md sync date: 2026-08-13,
byte-identical on the last re-pull per ANALYSIS.md's own statement. The Admin
canvas (`Alexandryn-Electron-Admin.dc.html`) was also consulted for `atSystem`.

**`atActivity` classification:** Binding for phase 15, per ANALYSIS.md's
classification table.

**What the canvas shows for `atActivity`:**

- Heading: "Activity". Subtitle: "What Alexandryn is fetching from your sources,
  and what you have been reading."
- Two-tab layout: "Acquisition" (active, underlined) | "Reading" (secondary
  colour, inactive — no drawn content).
- Acquisition tab — four sections:
  - **ACTIVE** + "Pause all" text action: each row shows cover thumbnail, title,
    author, source name, format badge, progress bar, "NN% · NNmb of NNmb"
    label, and an action button (e.g. "Pause").
  - **QUEUED**: each row shows smaller thumbnail, title, source name, format
    badge, status text (e.g. "Waiting for source"), "Cancel" text action.
  - **FAILED**: each row shows thumbnail, title, source name, error dot + error
    text, "Fix source" link (navigates to Sources), and a "Retry"-style button.
  - **COMPLETED** + "Clear" text action: each row shows smaller thumbnail, title,
    source name, format badge, green dot + status text, "Read" action link.
- The Activity nav item has an orange dot badge, visible when the route is
  active.

**`atSystem` in Admin canvas:** This screen is the **Design system** reference
(typography, colour, controls) — it is not a server diagnostics or metrics
screen. There is no drawn UI for operational metrics/diagnostics in any captured
canvas file.

**Reading tab — resolved by G0-1.** The "Reading" tab label is drawn but its
content is not; the undrawn content is Unclassified. The maintainer decided
(G0-1) to remove the tab label entirely this phase. The frontend spec builds the
Acquisition tab only, as Binding, with no "Reading" tab rendered.

**`/api/v1/diagnostics` — resolved by G0-2.** No canvas surface exists for a
metrics/diagnostics screen. The maintainer decided (G0-2) that the endpoint is
backend-only operational infrastructure with no UI this phase — not a
design-conformance question. Proceed.

## Specifications

| Spec | Status |
|---|---|
| `backend-observability.md` | Not yet drafted |
| `frontend-activity-screen.md` | Not yet drafted |
| `backend-reading-leaderboard.md` | Not yet drafted |

## Architecture decisions expected

**ADR 0030 — In-process metrics mechanism (Accepted, G0-4).** Go stdlib `expvar`
chosen by the maintainer. The ADR records the options considered and the
reasoning; no further deliberation needed on the mechanism itself.

**ADR 0031 — Activity log store model, retention, and LAN-client read
access (Proposed).** Three questions decided together: (a) schema — typed event
table or JSONB payload; FK relationship to the `jobs` table; (b) retention —
who triggers expiry and at what schedule; configurable key name and default
(30 days); (c) LAN-client read access — canvas places `atActivity` on the host
only, but the spec must state the access decision explicitly.

**ADR 0032 — Redaction-proof test: what it scans for and how it fails CI
(Proposed).** Must be decided before the RED step: what patterns constitute a
violation (credential-shaped strings, JWT format, home-directory path prefix,
book title in a log line during import, percentage value), how the test captures
log output (the `log/slog` spy handler at `internal/testutil/slogspy.go`, the
same primitive phase 09's `internal/jobs/redaction_integration_test.go` already
uses), and what `FAIL` looks like (any match → test fails and reports the
offending line). The test design precedes all implementation.

## Leaderboard (G0-3 — in scope, own spec)

The maintainer decided (G0-3) to include the library-visible reading activity /
most-read leaderboard in phase 15, as its own spec (`backend-reading-leaderboard.md`)
with a named privacy test.

- **What is shared:** after a library member finishes a book
  (`percentage >= 100`), other members of the same library see it marked as read
  and a per-library most-read count.
- **What is never shared:** reading position, current chapter, percentage, or
  time-remaining — for anyone but the reading user. The aggregate read model is
  layered over the per-user-private reading data (`user_id` + `library_id` on
  every progress row, phase 13 hardening); the query is
  `WHERE library_id = $1 AND percentage >= 100` and selects `work_id` + finisher
  identity only.
- **Design:** no canvas exists for a leaderboard surface. This is an Unclassified
  UI gap the frontend spec must flag at its own Gate 1; the backend read model
  does not depend on it.
- **Privacy test (RED):** a test that attempts to read a position/percentage
  value through the leaderboard endpoint and asserts it returns nothing, plus a
  test that a member at `percentage = 99` is absent from the aggregate. Written
  before the endpoint exists.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Redaction test written after implementation — asserts what the code does rather than what it should do | High | High | ADR 0032 decided before any implementation; test written first (RED step); constitution §2 |
| Metrics collection adds measurable overhead on the hot request path | Low | Medium | In-process counters only (no network I/O on hot path); latency budget set in spec NFRs |
| Activity log store grows without bound if reaper is not wired | Medium | Low | Retention reaper wiring is an exit criterion; test confirms rows are purged past the retention window |
| A "Reading" tab is rendered despite G0-1 | Low | Medium | Frontend spec + component test assert the Acquisition tab is the only tab; no "Reading" label in the DOM |
| Leaderboard aggregate leaks a position or percentage value | Low | High | G0-3 privacy test (RED): read-position attempt returns nothing; `percentage = 99` member absent; audit traces handler → SQL predicate |
| `/api/v1/diagnostics` leaks operational data to unprivileged users | Low | High | Admin-only auth guard; audit traces handler → middleware → role check |
| Metric labels carry user-identifying data (book title, source URL) | Medium | High | Redaction-proof test covers metric labels, not only log lines |

## Test strategy

The hardest thing to test is the redaction invariant across the full call path.
A job that fetches book content from a source must not emit the source
credential, the book title, or any position value at any log level. This requires
capturing log output from a real integration path, not a unit test of an
individual log call.

| Layer | What it covers |
|---|---|
| Unit | Metrics counter/histogram correctness; activity log event serialisation; retention-reaper cutoff calculation; Activity screen component states (active/queued/failed/completed, empty each) |
| Integration | Job lifecycle → `system_events` row written; `/api/v1/diagnostics` returns correct pool and queue stats; admin-only auth guard on diagnostics; reader-role token → 403; IDOR test |
| Redaction (RED gate) | Log output for import job, sync event, and auth path scanned for credential/token/title/position patterns — must fail before redaction code exists |
| Contract | Diagnostics endpoint response shape; Activity screen API response shape |
| E2E | Activity screen shows an in-flight job; failed acquisition shows with "Fix source" link; "Pause all" disables active acquisitions |
| Accessibility | Activity screen: keyboard-navigable list, Pause/Cancel/Retry/Clear actions keyboard-reachable, status updates announced, nav badge has accessible name |

## Security considerations

Trust boundaries this phase creates or extends:

1. **`GET /api/v1/diagnostics`**: Authenticated, admin-only. The audit must trace
   handler → middleware → role assertion → response shape and confirm no path
   bypasses the role check. Reader-role token must produce 403. Response must not
   include usernames, email addresses, or any value identifying a specific user.

2. **`system_events` table**: Written by the job engine and import pipeline. The
   event payload must not include source credentials, session tokens, book
   content, or reading positions. Write path is internal; the ADR 0031 decision
   on LAN read access must precede the handler.

3. **Activity screen API**: Admin/host-only consistent with the canvas placement.
   If it queries `system_events`, the read must be scoped by `library_id`
   predicate at the SQL layer. Audit traces this from handler to SQL.

4. **Metric labels**: No label may carry a book title, user identifier, or source
   URL — only route templates, outcome categories, and numeric values.

Constitution §8 governs all of the above. The redaction-proof CI test is a
security control: failing it at Gate 2 must block the phase.

## Observability

This phase is itself an observability phase. The section covers what it must not
break:

- Existing `/health` endpoint and request-ID logging (phase 03) continue
  unchanged.
- New metrics collection does not suppress or replace existing structured log
  output.
- Activity log reaper logs a `warn`-level message if it fails to run, rather than
  silently allowing unbounded table growth.

## Exit criteria

- [x] ADR 0030 (metrics mechanism) accepted — G0-4
- [ ] ADR 0031 (activity log store model, retention, LAN access) accepted
- [ ] ADR 0032 (redaction-proof test design) accepted
- [x] Reading tab design question resolved — G0-1 (label removed, Acquisition tab only)
- [x] Diagnostics endpoint design question resolved — G0-2 (backend-only, no UI)
- [ ] `backend-observability.md` is `VERIFIED`
- [ ] `frontend-activity-screen.md` is `VERIFIED`
- [ ] `backend-reading-leaderboard.md` is `VERIFIED`
- [ ] Test plans exist for every spec in this phase, written before this phase's
      RED step (ADR 0016) — or an explicit deferral is recorded
- [ ] Redaction-proof CI test exists, was observed to fail before implementation,
      and passes green
- [ ] `/api/v1/diagnostics` returns correct metrics, is admin-only, and the auth
      guard is traced from handler to role check in the audit
- [ ] Activity screen shows ACTIVE / QUEUED / FAILED / COMPLETED acquisition jobs
      backed by real system events
- [ ] Activity screen Acquisition tab: Pause-all, Cancel, Retry, Clear actions
      functional and keyboard-accessible
- [ ] Activity-log retention reaper wired and tested
- [ ] Security audit recorded in `.claude/audits/` with no open Critical or High
      findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
