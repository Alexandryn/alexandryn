# 0032. Redaction-proof test: violation patterns, capture mechanism, and fail mode

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-07 |
| **Deciders** | Maintainer (Gate 1, 2026-09-07) |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Constitution §8 requires: "Automated proof that credentials, tokens, and reading
history never reach logs or metrics." Phase 15 must deliver this proof as a
CI test, not as a code inspection. The test must be written before the
implementation exists (RED step, ADR 0016 cadence) — which means it must fail
against the unimplemented state and pass only after the redaction code is in
place.

Three design questions must be answered before the test can be written:

1. **What patterns constitute a violation?** The test must know what to scan for.
2. **How does the test capture log output?** The server writes structured JSON
   logs; the test must intercept them without depending on a file path.
3. **What does FAIL look like?** One matching line? Any matching pattern
   anywhere? The failure message must be actionable.

This ADR also records which paths the test runs — because a redaction test that
only covers paths the author expected to redact is weaker than it appears.

## Decision

### Violation patterns

A log line or metric label is a violation if it contains any of the following:

| Pattern | Why |
|---|---|
| A string matching the format of a stored OPDS credential (username: any non-empty string; password: any non-empty string preceded by `"password":` or after a `:` in an `Authorization:` header context) | Source credentials stored in `sources.credentials` must not appear in logs |
| A JWT-shaped string: three base64url segments separated by `.` with total length > 60 | Session tokens (access and refresh JWTs) must not appear |
| A string starting with `/Users/`, `/home/`, or `/root/` | Home-directory paths reveal the host filesystem layout |
| A reading position value: any field whose key is `position`, `cfi`, `location`, `chapter`, `percentage`, or `pct` combined with a numeric value | Reading positions are private to the reading user (constitution §8) |
| A verbatim book title occurring in a log line emitted during an import or acquisition job | Book titles are user data — they appear in the `books` table, not in log output |

**Deliberately not checked:**
- User-visible error messages (these are checked by the spec's error-shape FRs,
  not by the redaction test — the redaction test covers machine log output only).
- Route paths that appear in access logs (these are expected and do not leak
  sensitive data — route templates like `/api/v1/reading/:id` are fine).

The violation patterns are encoded as Go `regexp.Regexp` values in a shared
test helper (`internal/testutil/redaction.go`, alongside `slogspy.go`) so they
can be reused by future tests in other packages.

### Capture mechanism — `log/slog` spy handler in an integration test

Alexandryn logs through the standard library `log/slog`. The project already has
the capture primitive: `internal/testutil/slogspy.go` — `testutil.NewSpyHandler()`
returns a `*SpyHandler` (an `slog.Handler`) that records every `slog.Record` it
receives instead of writing it anywhere. It exposes `Records() []slog.Record`
(iterate for regex checks against `r.Message` and each attr value) and
`Contains(substr string) bool` (the "this exact secret never appears" check).

The redaction integration test wires `slog.New(testutil.NewSpyHandler())` as the
logger passed to the server, the job engine, the import pipeline, and the
`SystemEventWriter`. After a test path runs, it iterates the spy's records and
applies the violation patterns to each record's message and each attribute value,
and asserts `Contains` is false for every injected secret literal.

This is an established pattern, not a new one: phase 09 already ships
`internal/jobs/redaction_integration_test.go`, which uses `testutil.NewSpyHandler()`
to prove job payloads and errors never reach the log. Phase 15 extends the same
approach across the credential, auth, import, and `system_events` paths.

**Why not a buffer/pipe:** A `bytes.Buffer` behind a `slog.NewJSONHandler` would
work, but then the test parses JSON lines and is fragile against field-order and
escaping changes. The spy handler hands back structured `slog.Record` values with
no parsing.

**Why not a log file:** Log files depend on a path, require cleanup, and create a
test-environment dependency. The spy is in-memory and goes away with the test.

### Test paths covered

The redaction test is an integration test that exercises these paths with a
test-injected logger:

1. **Source credential path:** configure a source with a username + password,
   trigger a catalog sync job, assert no credential appears in any log entry
   emitted during the job.
2. **Authentication path:** perform a login (username + password → JWT), assert
   the password and the issued JWT do not appear in any log entry.
3. **Import job path:** enqueue an import job for a known book title, run the
   job to completion, assert the book title and any reading-position-shaped
   value do not appear.
4. **`system_events` write path:** verify that the payload written to
   `system_events` during the import job (checked via the test database) contains
   no credential, token, or position field.

Paths 1–3 run against the full server (test HTTP server via `httptest.NewServer`)
with a test PostgreSQL database (the same test-infra used by existing integration
tests). Path 4 queries the test database directly.

### Fail mode

A single matching line in any log entry or any `system_events` payload field
constitutes a test failure. The test calls `t.Errorf` (not `t.Fatalf`) for each
violation found, so a single test run reports all violations rather than stopping
at the first. The error message includes:

- Which violation pattern matched.
- The full log entry (message + all fields) in which it was found, or the
  `system_events` row ID and field name.
- The path/operation that generated the entry.

## Alternatives considered

### Alternative A — Scan log files after an integration run

**Pros:** Exercises the real log path without test-injected infrastructure.
**Cons:** Requires a writable log file path, a file-format parser, test cleanup.
Slower (file I/O). Coupling to the log format (line vs. JSON structured).
**Why not chosen:** The `slog` spy handler captures the same data with less
infrastructure and no parsing fragility.

### Alternative B — Unit tests per logging call site

**Pros:** Precise; tests exactly the functions that emit log lines.
**Cons:** Does not prove the whole path — a new log call site added later that
isn't unit-tested would escape coverage. The redaction guarantee must cover
emergent log output from combined code paths, not only individual functions.
**Why not chosen:** Integration coverage of a complete path (credential flows
through source sync → job engine → log output) is stronger than a collection of
unit tests.

### Alternative C — Static analysis (grep for literal credential variables)

**Pros:** No runtime overhead; catches mistakes at build time.
**Cons:** Cannot detect log calls that format a struct containing a credential
field; cannot catch a credential value that arrives via a variable rather than
a literal. Would require custom analysis passes (vet plugin). Much weaker than
runtime path coverage.
**Why not chosen:** Static analysis can complement the integration test as a
secondary check, but it cannot replace it.

## Consequences

**Good:** The test proves the full path — source credential → job engine → log
output — not just individual functions. A new log call that emits a credential
will be caught on the next CI run without any change to the test.

**Bad:** The test requires a running test database and a built server. It is an
integration test, not a unit test, so it is slower (~5–10 seconds) and less
isolated. It belongs in the `integration` test tag, not the default `go test`.

**Neutral:** The violation patterns are Go regexps in a shared helper. Adding
a new pattern (e.g. if a new credential shape is introduced by a future source
type) requires a one-line addition to the helper and a re-run.

## Reversal cost

Low. The test is a self-contained integration test file. Removing or replacing
it does not affect production code. However, removing it without a replacement
would leave the §8 guarantee unchecked — the audit at Gate 2 must confirm the
test exists and was observed to fail before the implementation was in place.

## Confidence

High on the capture mechanism — the `slog` spy handler already exists
(`internal/testutil/slogspy.go`) and is already used this way in phase 09.
Medium on the violation patterns — the list above is derived from the spec's
private-data categories (constitution §8, `domain-reading.md`), but future
source kinds may introduce new credential shapes that are not yet listed. The
spec must name the patterns explicitly and note that the list is expected to grow
with new source types.
