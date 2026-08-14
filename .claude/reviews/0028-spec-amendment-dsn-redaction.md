# Review: Spec amendment — DSN redaction in startup/migration/config-parse error logging

| | |
|---|---|
| **Subject** | `.claude/specs/backend-service-lifecycle.md` FR-3, `.claude/specs/backend-persistence.md` FR-6, `.claude/specs/backend-configuration.md` FR-6 |
| **Reviewer** | Self-reviewed |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (the amendment itself) |

## Summary

A `/security-review` pass across the six phase 03 backend specs (run via
the project's own adversarial-review discipline, constitution §10) found
one confirmed finding: `backend-http-transport.md` FR-5 already redacts
`/readyz`'s failure body against a real, named failure mode — `pgx`
connection errors can embed the DSN (including, in some connection-string
forms, a password) in their own `Error()` text — but three sibling call
sites that construct the same class of error had no equivalent
instruction: startup connection-failure logging
(`backend-service-lifecycle.md` FR-3), the migration runner's connection
open (`backend-persistence.md` FR-6), and a malformed TOML config file
whose parser error could echo the offending `DATABASE_URL` line verbatim
(`backend-configuration.md` FR-6). Full finding and independent
false-positive verification: see the security review conducted this
session (not separately filed — recorded here as the amendment record for
the three specs it touches).

## Decision

Extend `backend-http-transport.md` FR-5's existing pattern — a fixed,
generic message, never the raw driver/parser error's `Error()` string, for
any error that could carry `DATABASE_URL` — to the three sibling call
sites named above. Each amendment is scoped narrowly to the
connection/parse-failure case specifically, not to every error the spec
already required be logged with detail (e.g. `backend-persistence.md`
FR-6 still requires naming the failing migration file and `goose`'s own
error text for a genuine SQL/schema failure, which carries no
connection-string risk and remains useful for diagnosis).

## Findings

None beyond the one this amendment closes. All three amendments follow
the same mechanism `backend-http-transport.md` FR-5 already established
and this project's own reviews (`0022`) already validated for that call
site — this is propagation of an existing, already-reviewed pattern, not
a new design decision.

## Dimensions checked

- [x] Completeness — checked all six phase 03 specs for other call sites
      that construct a `pgx`/TOML error and log it; no fourth site found
      (repository query errors are covered by `backend-errors-and-logging.md`
      FR-3's translation-before-crossing-boundary requirement, which
      already prevents a raw driver error from reaching a log line
      un-translated)
- [x] Consistency — confirmed the redaction language in all three
      amendments matches `backend-http-transport.md` FR-5's existing
      wording rather than inventing a new phrasing per site
- [ ] Independent review — not run; this is a mechanical extension of an
      already-independently-reviewed pattern (`backend-http-transport.md`
      FR-5 itself went through review `0022`'s two-agent pass) to three
      more call sites with no new design tradeoff. Self-review only, per
      this project's own proportionality judgment
      (`[[feedback-make-improve-review-fix-cycle]]`).

## Resolution

Fixed in this pass:

- `backend-service-lifecycle.md` FR-3 — added the redaction requirement
  for the FR-1 step 5 (Postgres-connect) failure case specifically.
- `backend-persistence.md` FR-6 — added the redaction requirement for the
  migration runner's connection-open failure, distinguished from a
  genuine SQL/schema failure (which keeps its existing detailed logging).
- `backend-configuration.md` FR-6 — added the redaction requirement for a
  TOML parse error whose offending line/token is `DATABASE_URL` or a
  future sensitive key.

Maintainer re-confirmation of all three amendments is still outstanding,
consistent with every other post-approval amendment this project has
recorded (`0025`, and `architecture-system.md`'s ADR 0007 amendment).
