# Security audit: cmd/server's startup and shutdown sequence (Checkpoint F)

| | |
|---|---|
| **Scope** | `cmd/server`'s full lifecycle — `backend-service-lifecycle.md` FR-1 through FR-7 — as built across T17-T20 (`tasks/plan.md` Tier 3): `cmd/server/main.go`, `cmd/server/run.go`, and the config/serving boundary in `internal/config/bindaddress.go` |
| **Auditor** | Claude (Sonnet 5) — two independent passes (general code review + security-focused), synthesized, for review by Luann Moreira |
| **Date** | 2026-08-19 |
| **Commit** | `27d68b0`–`5a29f61` (T17-T19, PRs #22-#25), `5726142` (BIND_ADDRESS fix, PR #26), `edd973c`-`bc856f4` (T20, PR #27) — all merged to `main` |
| **Verdict** | All 6 findings `Fixed` — 5 same session, `A-03-05`'s deferred spec-text half closed 2026-08-26 (Checkpoint H) |

This is Checkpoint F, the plan's own named highest-risk stop point
("worth an extra review pass specifically here... before T20's real-
listener, `-race`-gated Integration/Concurrency layers"). Constitution §10
requires this recorded here, rated honestly — this review happened inline
across several PR threads and chat turns and was never written up as its
own document until now; this file is that write-up, after the fact but
complete.

## Scope and method

Two independent subagent passes against `cmd/server/main.go`,
`cmd/server/run.go`, and `cmd/server/run_test.go` as they stood after T19
(PR #25, before its own fix commit): one general code-quality review, one
security-focused review, launched in parallel with no shared context so
neither could anchor on the other's findings. Both were asked to verify
claims empirically (temporary scratch tests, reading actual stdlib source)
rather than reason from documentation alone. Findings were synthesized,
the four clearly in-scope ones fixed via TDD (RED confirmed against the
pre-fix code, then GREEN) in a follow-up commit on the same PR, and the
fifth (TLS/`BIND_ADDRESS`) taken to the maintainer as a scope decision
before touching code, since it crossed into `internal/config` (T6) and an
already-`APPROVED` spec's text.

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| Shutdown signal timing | The OS / Electron control-plane channel (`SIGTERM`, `ctx` cancellation) | That a signal arriving at any point in the startup sequence is handled the same way (FR-4) |
| `DATABASE_URL` | Operator-supplied config (constitution §4: configured ≠ trusted) | That a slow, hostile, or misconfigured host can't turn a bounded retry into an unbounded one |
| `BIND_ADDRESS` / TLS certificate | Operator-supplied config, and the network itself once bound to anything beyond loopback | That FR-8's validation actually corresponds to what the process does at serve time |
| In-flight request vs. grace-period expiry | A slow or hostile concurrent client | That `Shutdown`'s grace period is a real ceiling, not merely a suggestion `Shutdown` alone doesn't enforce |

## Adversarial questions asked

- **What does a malicious/misconfigured user of this instance achieve
  by controlling `DATABASE_URL`?** A host that accepts the TCP handshake
  and never completes Postgres's own protocol exchange could block a
  single connect attempt indefinitely — `A-03-03`.
- **What happens when a shutdown signal arrives at an unexpected time?**
  Mid-retry, mid-migration, or mid-pool-construction, it used to be
  silently absorbed and reported as an ordinary database failure instead
  of a clean shutdown — `A-03-01`.
- **What happens when input (here, a slow client) is slow or never
  arrives?** A request outliving the grace period was left for process
  exit to kill, not cleanly cancelled by the code — `A-03-02`.
- **What is logged that shouldn't be?** The migrate step's failure log
  had no DSN-redaction guard, unlike its two siblings (currently safe
  only because a lower layer pre-sanitizes) — `A-03-04`.
- **What does a device on the local network achieve without
  credentials?** If an operator sets a non-loopback `BIND_ADDRESS` with a
  valid certificate, `config.Load` succeeds while nothing in the process
  ever calls `ServeTLS` — the certificate is validated but never used —
  `A-03-05`.
- **What happens if two of these run at once?** A second shutdown signal
  arriving immediately after the first must not panic or double-close
  the pool — verified by construction (`context.CancelFunc` is
  idempotent; `signal.NotifyContext` reverts to default OS disposition
  on a second real signal) and by test (`TestConcurrency_ShutdownUnderLoad`
  sends `cancel()` twice). No finding — see "What was not examined" for
  the one thing this doesn't cover. Separately: two integration-tagged
  packages sharing one `TEST_DATABASE_URL` running concurrently (Go's
  default cross-package test parallelism) collide over the same shared
  schema — `A-03-06`, a test-infrastructure reliability gap, not a
  production security finding, included here because it was found in
  the same pass and constitution §10 doesn't distinguish "keep looking
  vs. stop at the vuln boundary" for what gets written down.

## Findings

| ID | Severity | Title | Status |
|---|---|---|---|
| A-03-01 | Medium | Mid-startup shutdown signal absorbed into ordinary FR-3 failure | Fixed — `5a29f61` |
| A-03-02 | Medium | Grace-period expiry doesn't force-close active connections | Fixed — `5a29f61` |
| A-03-03 | Medium | No per-attempt timeout on Postgres connect | Fixed — `5a29f61` |
| A-03-04 | Low | Migrate-step failure log missing defense-in-depth DSN guard | Fixed — `5a29f61` |
| A-03-05 | High | `BIND_ADDRESS`/TLS: certificate validated but never enforced at serve time | Fixed — code in `5726142`; spec text amended 2026-08-26 (`backend-configuration.md` FR-8's interim note), see Resolution |
| A-03-06 | Informational | Cross-package integration-test DB isolation gap | Fixed — T26 (`backend-test-harness.md` FR-3 Variant B): each integration-tagged package now creates and migrates its own physical database (`testutil.EnsurePackageDatabase`); `-p 1` removed from `ci.yml` |

### A-03-01 — Mid-startup shutdown signal absorbed into ordinary FR-3 failure

**Severity:** Medium

**Component:** `cmd/server/run.go` — `waitForPostgres`, and the three
`if err != nil` blocks following `waitForPostgres`/`runMigrations`/
`newPool` in `run`

**Description** — once the listener bound (FR-1 step 4) and `run` moved
on to steps 5/6, none of the remaining blocking calls (the Postgres
retry loop, its backoff sleep, migrations, pool construction) ever
checked `ctx.Err()`. A shutdown signal arriving in that window was
treated as just another failed attempt: the retry loop kept retrying
(or slept through its full backoff), and the process eventually exited
through the ordinary FR-3 failure path (`return 1`, no `Shutdown` call)
— exactly the transition `backend-service-lifecycle.md`'s State
transitions section calls illegal ("the process exiting without
attempting `Shutdown(ctx)` first, when the signal was a clean `SIGTERM`
... violates FR-4").

**Impact** — in production, `postgresReadyMaxAttempts = 30` /
`postgresReadyBackoff = 1s` meant a `SIGTERM` arriving during that wait
could add up to ~29 extra seconds of unresponsive-to-shutdown blocking,
then exit mislabeled as a database failure with `os.Exit(1)`, abandoning
the already-accepting listener instead of draining it.

**Preconditions** — none; triggerable any time a shutdown signal arrives
after the listener binds but before step 6 completes, which is a real
window on every cold start, not a rare race.

**Reproduction** — empirically confirmed (round 1 reviewer): a scratch
test with `obtainPostgres` cancelling `ctx` on its first call, then
returning `ctx.Err()` on every subsequent call, showed `run()` burn
through the full retry budget and exit via `return 1` with `shutdown`
never appearing in the call-order log.

**Recommendation** — check `ctx.Err()` in the retry loop (before each
attempt and after a failed one, returning immediately rather than
retrying or sleeping) and after each of the three blocking steps' own
error checks; route to the same graceful-shutdown sequence the
post-startup path already used.

**Resolution** — fixed in `5a29f61`. `waitForPostgres` now returns
`ctx.Err()` immediately on cancellation instead of continuing to retry;
`run` checks `ctx.Err() != nil` after each of the three steps' failures
and calls `gracefulShutdown` (a new shared helper) instead of the
ordinary failure path. Regression tests
(`TestRun_ShutdownSignalDuringPostgresRetry_CallsShutdownNotOrdinaryFailure`
and its migration/pool-construction siblings) confirmed RED against the
pre-fix code before the fix, GREEN after.

### A-03-02 — Grace-period expiry doesn't force-close active connections

**Severity:** Medium

**Component:** `cmd/server/run.go` — the `case <-ctx.Done()` branch
(pre-fix), now `gracefulShutdown`

**Description** — `net/http.Server.Shutdown`'s own documentation states
it shuts down "without interrupting any active connections" — it only
closes *idle* ones and waits for the rest; on its context expiring it
just returns `ctx.Err()`. The pre-fix code treated
`context.DeadlineExceeded` from `Shutdown` as an expected, no-op outcome
and proceeded straight to `pool.Close()` and `return 0`; `main.go` then
called `os.Exit(code)` synchronously, killing every still-running
handler goroutine and its connection instantly. There was no fallback
`srv.Close()` (the stdlib's real hard-close, which does force-close
active connections and does propagate cancellation to their request
contexts) anywhere in the path.

**Impact** — under exactly the scenario `backend-service-lifecycle.md`
names as "the hardest thing to test here" (shutdown under load), a
request still executing when the grace period expired was torn down by
process exit, not given the clean cancellation FR-4 requires ("Requests
still in flight when the timeout expires MUST be cleanly cancelled...
rather than the process being hard-killed out from under them"). The
existing Unit-layer tests couldn't have caught this — `fakeServer.Shutdown`
was a hand-rolled stub that never exercised real `net/http.Server`
behavior.

**Preconditions** — any in-flight request whose handler runs longer than
the configured `ShutdownGracePeriod` at the moment a shutdown signal
arrives.

**Reproduction** — confirmed by reading the Go 1.26 stdlib source
directly (`net/http/server.go`'s `Shutdown` and `Close` implementations)
rather than inferred from documentation alone; a handler blocking 12s
against a 10s grace period was traced through the pre-fix code path to
confirm no `Close()` call existed anywhere reachable from
`context.DeadlineExceeded`.

**Recommendation** — call `srv.Close()` when `Shutdown` returns
`context.DeadlineExceeded`, before proceeding to `pool.Close()`.

**Resolution** — fixed in `5a29f61`. `gracefulShutdown` now calls
`srv.Close()` specifically on `context.DeadlineExceeded`, logging (but
not treating as fatal) any error from `Close()` itself.
`shutdownableServer` gained a `Close() error` method to make this
injectable; `TestRun_Shutdown_GracePeriodExpiryStillClosesPoolAndExitsZero`
asserts `closeCalls == 1` on that path, and
`TestConcurrency_ShutdownUnderLoad` (T20, PR #27) proves the real
end-to-end behavior: a request whose delay deliberately exceeds the
grace period gets a client-visible error (connection reset), never a
200 or a hang.

### A-03-03 — No per-attempt timeout on Postgres connect

**Severity:** Medium

**Component:** `cmd/server/main.go` — `connectPostgres` (pre-fix)

**Description** — `waitForPostgres` bounds the *count* of attempts
(`postgresMaxAttempts`) but each individual attempt was unbounded:
`connectPostgres` passed the top-level signal `ctx` straight to
`pgx.Connect` with no per-attempt timeout. Verified against
`pgx@v5.10.0`: `connect_timeout` is only set if the DSN or
`PGCONNECT_TIMEOUT` explicitly provides one; otherwise the dialer is a
bare `&net.Dialer{}` with no deadline. TCP-level SYN timeouts bound the
dial phase loosely and by OS default, but once TCP connects, Postgres's
own startup-message handshake had no timeout at all beyond the
already-unbounded `ctx`.

**Impact** — a misconfigured or hostile `DATABASE_URL` pointing at a
host that accepts the TCP handshake but never completes the Postgres
protocol exchange could block a single retry attempt far longer than the
documented ~30-second total budget implies — constitution §4 explicitly
forbids treating "the user configured it, so it's fine" as a threat
model for exactly this class of operator-supplied input.

**Preconditions** — `DATABASE_URL` pointing at a TCP endpoint that
accepts connections but never speaks the Postgres protocol (a
misconfiguration, a firewall dropping packets post-handshake, or a
deliberately hostile endpoint).

**Reproduction** — `TestConnectPostgresWithTimeout_BoundsABlackHoleHost`
(added with the fix) opens a real `net.Listener` that accepts and then
sends nothing, ever; run against the pre-fix `connectPostgresWithTimeout`
(with the `context.WithTimeout` wrapper stripped), the test hung past
its own 3-second failsafe, confirming the unbounded block empirically.

**Recommendation** — wrap each attempt in its own
`context.WithTimeout`, sized independently of the retry backoff.

**Resolution** — fixed in `5a29f61`. `connectPostgresWithTimeout(cfg,
timeout)` wraps each attempt in `context.WithTimeout(ctx,
postgresConnectAttemptTimeout)` (5 seconds, same provisional-guess
caveat as the retry budget itself — real numbers belong to
`backend-persistence.md`). RED-confirmed against the black-hole-host
test before the fix, GREEN after.

### A-03-04 — Migrate-step failure log missing defense-in-depth DSN guard

**Severity:** Low

**Component:** `cmd/server/run.go` — the `runMigrations` error branch
(pre-fix)

**Description** — the postgres-connect and pool-construction steps both
independently check `if cfg.DatabaseURL != ""` before logging, replacing
the driver error's text with a fixed generic message per FR-3's DSN-
redaction amendment. The migrate step had no equivalent guard; it logged
`err.Error()` unconditionally. This was not currently exploitable —
`internal/persistence/postgres.RunMigrations` already returns
pre-sanitized, fixed-text errors for every connection-class failure
(`connectionFailure`, translated before the error ever reaches
`run.go`) — but it was the one place in `run.go` a future change to that
package's error wrapping (e.g. "improving" diagnostics by switching a
generic `errors.New` back to `fmt.Errorf("...: %w", err)`) could
silently reintroduce the exact leak class `.claude/audits/0001` already
found once.

**Impact** — none live today; a defense-in-depth gap, not an active
leak.

**Preconditions** — a future regression in
`internal/persistence/postgres`'s own error handling; not exploitable
against the code as it stands.

**Reproduction** — code inspection; no PoC needed for a defense-in-depth
gap with no live path.

**Recommendation** — add the same guard the sibling steps have, for
consistency and to match the spec's instruction that redaction apply
"at every log call it introduces, not just the ones those specs directly
own."

**Resolution** — fixed in `5a29f61`. `TestRun_MigrationFailureWithDatabaseURLNeverLeaksTheDSN`
added as the regression guard.

### A-03-05 — `BIND_ADDRESS`/TLS: certificate validated but never enforced at serve time

**Severity:** High

**Component:** `internal/config/bindaddress.go` (`validateBindAddress`,
`validatePublicBindCertificate`), `cmd/server/run.go` (`srv.Serve`, never
`srv.ServeTLS`), `internal/transport/http/server.go` (`NewServer`, no
`TLSConfig`)

**Description** — `config.Load` → `validateBindAddress` requires
`TLS_CERT_FILE`/`TLS_KEY_FILE` to load into a valid, currently-dated
certificate whenever `BindAddress` resolves outside loopback/private
ranges (ADR 0017 Mode A, constitution §6). Nothing downstream ever used
`cfg.TLSCertFile`/`cfg.TLSKeyFile`: `deps.listen` was plain `net.Listen`,
`NewServer` set only `ReadTimeout`/`WriteTimeout`/`IdleTimeout` and never
a `TLSConfig`, and `run.go` called `srv.Serve(listener)`, never
`ServeTLS`. Grepped `cmd/server` and `internal/transport/http` for
`ServeTLS`/`TLSConfig`/`tls.` — zero matches at audit time.

**Impact** — an operator setting a publicly routable `BIND_ADDRESS` with
a valid cert/key pair (satisfying config validation, reasonably believing
Mode A was now active) got a server that bound that address and served
every route in cleartext — exactly the failure mode constitution §6
forbids ("Both modes fail closed at the point of binding — checked
against the address and certificate state actually present, not against
a configuration flag"). Nothing was checked at the point of binding at
all; config-time cert validation created a false sense that TLS was
enforced, which is worse than no validation, since it looks enforced but
isn't.

**Preconditions** — an operator setting `BIND_ADDRESS` to a publicly
routable address with `TLS_CERT_FILE`/`TLS_KEY_FILE` pointing at an
otherwise-valid certificate. Not exploitable against the default config
(`BIND_ADDRESS` defaults to `127.0.0.1:0`).

**Reproduction** — `BIND_ADDRESS=0.0.0.0:8443` with a valid cert/key
pair: `config.Load` succeeded; `run()` called `net.Listen("tcp",
"0.0.0.0:8443")` and `srv.Serve(listener)` with a plain `*http.Server`;
any client on the network could `curl http://<host>:8443/healthz` and
get a valid plaintext response — no TLS negotiation ever occurred.

**Recommendation, and the scope decision actually made** — two options
were brought to the maintainer directly rather than decided
unilaterally, since either touches an already-`APPROVED` spec's text or
adds real TLS-serving scope this spec (`backend-service-lifecycle.md`)
doesn't own: (a) wire real `ServeTLS` now, or (b) fail closed on any
public bind, even with a valid cert, until `ServeTLS` actually exists.
The maintainer chose (b), scoped narrowly — the already-approved
private-range case (Mode B: no certificate required, TLS assumed to
terminate upstream via a reverse proxy) was explicitly left unchanged,
since broadening the fix to reject private-range binds too would have
meant reversing FR-8 text the maintainer already approved under ADR
0017, a spec-amendment decision distinct from this one.

**Resolution** — fixed in `5726142` (code). `validateBindAddress` now
refuses a publicly routable `BIND_ADDRESS` outright, regardless of
certificate validity, until `ServeTLS` is wired — `validatePublicBindCertificate`
is untouched and still runs first (an invalid/expired/missing cert still
gets its own specific error), it's just no longer sufficient on its own
to make a public bind legal. `TestLoad_BindAddress_PubliclyRoutableWithValidCertAccepted`
was flipped to `TestLoad_BindAddress_PubliclyRoutableRejectedEvenWithValidCert`,
RED-confirmed against the pre-fix code, GREEN after.

**Closed, 2026-08-26 (Checkpoint H)** — `backend-configuration.md` FR-8
now carries an interim note stating plainly that the code is
deliberately stricter than the bullet list above it: today, a publicly
routable `BIND_ADDRESS` is refused unconditionally, valid certificate or
not, until phase 13 wires `ServeTLS`. Spec and code no longer disagree —
the spec says what the code actually does, and says why, and names what
removes the note (`ServeTLS` landing). Self-reviewed, same as this
spec's five prior post-approval amendments, needs maintainer
re-confirmation.

### A-03-06 — Cross-package integration-test DB isolation gap

**Severity:** Informational (test-infrastructure reliability, not a
production security finding — included here per this file's own stated
reasoning in "Adversarial questions asked")

**Component:** `cmd/server/run_integration_test.go`,
`internal/persistence/postgres/migrate_integration_test.go` — both
`resetSchema` against the one shared `TEST_DATABASE_URL`

**Description** — `go test -tags=integration ./...` across the whole
module can flake: both packages reset the shared schema
(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`), and Go
parallelizes different packages' tests by default, so one package's
reset can wipe out another's in-progress migration mid-run.

**Impact** — a spurious, hard-to-diagnose CI/local failure with no
relationship to the actual code under test — the exact "mystery flake"
category that erodes trust in a test suite over time.

**Preconditions** — running `go test -tags=integration ./...` (no `-p`
override) against a single shared `TEST_DATABASE_URL` while more than
one integration-tagged package exists that resets the same schema. True
today now that `cmd/server` is the second such package.

**Reproduction** — reproduced directly: `go test -tags=integration -race
./...` (default `-p`) failed intermittently in
`TestPartialMigration_RefusedOnNextAttempt`
(`internal/persistence/postgres`) with "the migration before the broken
one never applied"; the identical invocation with `-p 1` passed
reliably, and each package run alone (`-count=10`) was independently
stable.

**Recommendation** — real per-package DB isolation (a distinct schema or
database per package, or an advisory lock around schema resets) is
`backend-test-harness.md` FR-3 Variant B's job — **T26** in
`tasks/plan.md` (Tier 4), not reached yet at the time of this audit (this
recommendation originally mislabeled the fix as Variant A's job; Variant
A, T22, was schema-at-head plus the intra-package truncate-teardown/
`t.Parallel()` static check — a different, already-closed concern from
this cross-package one). In the meantime, any whole-module integration
run needs `-p 1`.

**Resolution** — fixed in T26: `internal/testutil/packagedb.go` adds
`EnsurePackageDatabase`, called from each of the three integration-tagged
packages' `TestMain`s (`internal/testutil`, `internal/persistence/postgres`,
`cmd/server`), giving each its own physical database derived from
`TEST_DATABASE_URL` (`<original dbname>_<pkgName>`) before any test in
that package runs. `ci.yml`'s `-p 1` workaround is removed; verified
locally with `go test -race -tags=integration -count=5 ./...` at default
parallelism against a real Postgres, all five re-executions green.

## What was not examined

The four-attacker adversarial pass this constitution names for a
substantial feature's close (`.claude/audits/README.md`) was not run in
full here — this review is scoped to Checkpoint F's own stated concern
(the startup/shutdown sequence's correctness and the mid-review security
pass), not a complete pre-close audit of all of `cmd/server` or the
transport/persistence layers it calls into. In particular: no fuzzing or
malformed-input testing against the HTTP layer itself (owned by
`backend-http-transport.md`'s own T13-T16 work, already built and
smoke-tested separately per that spec's Checkpoint E); the repository-
construction gap (`TODO(D1)` in `run.go`) is inert today (phase 03
registers zero `/api/v1` routes) and is tracked by Checkpoint G, not
re-examined here; macOS-specific spawn mechanics (`cmd/pg-supervisor`)
don't exist yet and aren't in scope until D4/T25. Windows shutdown-signal
delivery (`CTRL_CLOSE_EVENT` or equivalent) remains the spec's own open
question, untested, as stated in `backend-service-lifecycle.md`'s Open
questions and this plan's "What is deliberately not tested."
