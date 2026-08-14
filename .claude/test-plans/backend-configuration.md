# Test plan: Backend configuration

| | |
|---|---|
| **Spec** | `.claude/specs/backend-configuration.md` |
| **Status** | `REVIEWED` (independent, findings fixed — [`0024`](../reviews/0024-test-plan-backend-configuration.md)) |
| **Created** | 2026-08-14 |

## What we are trying to be confident about

- `config.Load` returns either a fully validated `Config` or an error —
  never anything in between. No caller ever receives a struct with one
  field validated and another silently zero-valued.
- Per-key precedence (default, then file, then environment) actually
  resolves independently per key, not as an all-or-nothing switch between
  sources — a file that sets three of nine keys and an environment
  variable that overrides one of those three must produce exactly the
  expected merge, not "file wins entirely" or "env wins entirely."
- `DATABASE_URL` cannot leak into a log line or error string under any
  logging call site this codebase might reasonably write, including the
  one path (`slog.Any` on the whole `Config`) that a single-interface
  redaction design would miss — this is the spec's own most-elaborated
  defense (FR-7) and the one most likely to have a real gap if under-
  tested.
- A non-loopback `BIND_ADDRESS` is rejected before any other subsystem
  initializes — phase 03's own named, security-relevant exit criterion,
  and the same finding review `0022` caught as entirely absent from an
  earlier draft of this spec.
- FR-3's three-category rule holds in code, not just in prose: no key
  is required in one code path and silently unread in another, the exact
  defect (`DATABASE_URL`'s "required in dev" framing) review `0022` also
  caught and the spec was rewritten to close.

## Risk assessment

Highest risk, concentrate here:

- **FR-7's dual-interface redaction.** This is the spec's own most
  elaborate requirement, added specifically because a single-interface
  design (`slog.LogValuer` only) has a documented, non-obvious gap: it
  protects a field logged directly but not the same field reached through
  `encoding/json`'s reflection fallback when a containing struct is logged
  via `slog.Any`. A test suite that only logs `cfg.DatabaseURL` directly
  would pass while the real gap (`slog.Any("config", cfg)`) still leaks —
  this needs its own explicit case, not an inferred one.
- **FR-8's loopback rejection.** Security-relevant, named explicitly by
  phase 03's own exit criteria, and the exact class of defect (a security
  property left as documentation rather than enforced in code) review
  `0022` found missing once already in this same spec area.
- **FR-3's category discipline.** The most likely regression is a future
  key reintroducing "required in dev, unread in production" — exactly the
  pattern the spec was rewritten to forbid. A test for `DATABASE_URL`
  specifically (the one instance of the third category) is necessary but
  not sufficient; the plan also needs a structural check that a key
  can't silently acquire context-dependent behavior undetected.

Lower risk, merely tedious, cover but don't over-invest:

- The remaining eight keys' individual type/enum validation (straightforward,
  table-driven).
- FR-5's config file location fallback — a single resolution-order check.

## Layers

### Unit

Pure resolution and validation logic, no real filesystem beyond a
temp-directory TOML fixture, no real environment beyond `t.Setenv`:

- FR-2 precedence, per key, table-driven across all nine keys in FR-4:
  three cases each — no source sets it (default applies, or error if
  required with none), only the file sets it, only the environment sets
  it, and both the file and the environment set it (environment wins) —
  proving independence per key, not one shared switch.
- FR-3 category discipline: for a required key with every source empty,
  `Load` errors (never a zero value); for an optional-with-default key with
  every source empty, the compiled default is returned; for `DATABASE_URL`
  specifically, absence returns a `Config` with the field's zero value and
  no error — the third category's actual behavior, not inferred from the
  other two.
- FR-3 structural check: a table asserting, for every key in FR-4, exactly
  one of the three categories applies — a key that turns up in both the
  "required" test set and the "absence is a valid signal" test set fails
  this check immediately. This is the concrete mechanism behind the Risk
  assessment's claim above; it's what catches a future key silently
  regaining "required in one context, unread in another," the exact
  defect review `0022` found in an earlier draft, without depending on
  someone remembering to add a matching pair of tests by hand.
- FR-4 type/enum validation, table-driven per key: `BIND_ADDRESS` accepts
  a valid `host:port`; `LOG_LEVEL` accepts the four named values matched
  case-insensitively (`INFO`, `Debug`, etc. all accepted, lowercased
  before comparison — FR-4, amended `0025`) and rejects anything else,
  including a value that isn't one of the four even after lowercasing;
  the three `HTTP_*_TIMEOUT` keys,
  `DB_POOL_MAX_CONNS`, and `HTTP_MAX_BODY_BYTES` accept a parseable
  duration/integer and reject a non-parseable one; `SHUTDOWN_GRACE_PERIOD`
  likewise.
- FR-5 config file resolution: `--config <path>` present and the file
  exists — loaded; `--config` present and the file does not exist — error
  (the one case where file-absence is a failure, per FR-6's own
  distinction); `--config` absent and no file at the well-known fallback
  location — no error, every key resolves from default/environment only;
  `--config` absent and a file exists at the fallback location — loaded.
- FR-6 error content: every failure case (missing required key, invalid
  TOML syntax, a key failing type validation, an explicitly-wrong
  `--config` path) produces an error whose message names the specific key
  or file and the specific problem — asserted by substring match against
  the offending key/path name and the absence of a generic phrase like
  "invalid configuration," not just that an error was returned.
- FR-7 redaction, both interfaces, both call sites: (a) `cfg.DatabaseURL`
  logged directly via `slog.Any("database_url", cfg.DatabaseURL)` —
  captured output must show the redacted placeholder, never the real
  value; (b) the *whole* `cfg` logged as one attribute via
  `slog.Any("config", cfg)` — captured output must still show the
  placeholder, never the real value, at every position `DatabaseURL`
  would otherwise appear in the reflected JSON. Case (b) is the one a
  single-interface design would fail and is the actual reason FR-7
  requires both `slog.LogValuer` and `json.Marshaler`. Its regression
  guard is two fixture types, not a runtime toggle (Go can't switch which
  interfaces a type implements): a test-local `configLogValuerOnlyStub`,
  never shipped in production code, implementing only `slog.LogValuer`,
  run through case (b) first to confirm it *does* leak the real value
  (proving the test can actually detect the gap it exists to catch); then
  the real `Config` type, implementing both interfaces, run through the
  same case (b) and asserted clean.
- FR-7 on a failed load: `Load` called with `DATABASE_URL` set to a
  real-looking value alongside a separate, unrelated validation failure
  (e.g. an invalid `LOG_LEVEL`) — the returned error's `.Error()` string
  is asserted not to contain the `DATABASE_URL` value, covering the
  "whether successful or failed" half of the acceptance criterion the
  two cases above, both on a successful load, don't reach.
- Observability (spec's Observability NFR): a successful `Load` with at
  least one key sourced from the file and at least one from the
  environment logs one line per non-default-sourced key naming which
  source provided it (`"file"` or `"environment"`) — excluding
  `DATABASE_URL`'s actual value from that line even when it's the key
  being reported on, per FR-7.
- FR-8 loopback rejection, table-driven: `127.0.0.1:0`, `127.0.0.1:8080`,
  `[::1]:0`, `localhost:8080` accepted; `0.0.0.0:8080`, `192.168.1.5:8080`,
  a public IP, and an unresolvable hostname rejected, each with an error
  naming the offending address.
- FR-1 structural check, not a runtime test: `architecture-backend.md`
  FR-3 established that an import-boundary-style lint check belongs in
  this project, but its own Open questions leave the concrete lint tool
  unchosen — this is not yet an established mechanism to extend. What
  this plan requires is *a* mechanism, whatever phase 03's own lint-tool
  decision lands on, that fails the build on an `os.Getenv`/
  `os.LookupEnv` call or a TOML-decoding import outside `internal/config`:
  a dedicated `go/analysis` pass, a `golangci-lint` custom rule, or — as
  an interim before that decision — a grep-based CI script. Left open
  here as a pointer to that decision, not claimed as already solved.

### Integration

None of its own — the spec's own Test strategy table says
`backend-service-lifecycle.md`'s startup test already covers config load
as its first step, and this plan agrees: no separate integration layer
adds coverage FR-2/FR-6/FR-8's unit tests don't already provide more
cheaply. The one thing worth confirming at that boundary (that a real
`cmd/server` invocation with a genuinely invalid config exits non-zero
before touching Postgres) is `backend-service-lifecycle.md`'s test plan's
responsibility, not duplicated here.

### Contract

N/A — matches the spec's own Test strategy table; no schema is shared
across a process boundary here.

### End to end

N/A — no user-facing journey; configuration is an internal startup
concern.

### Accessibility

N/A — no UI (spec's own Non-functional requirements agree).

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| Config file with invalid TOML syntax (unclosed string, bad indentation) | `Load` errors, names the file and the parse problem, never a raw parser panic/stack trace (FR-6) |
| Config file present but empty | Not an error by itself — every key resolves from default/environment (FR-2); an empty file is not the same as a missing required key |
| Config file with a key not in FR-4's table | Ignored, not an error — the table is authoritative for what's read; an unrecognized key must not silently do nothing while looking like it did something, so this case also asserts nothing crashes and no unexpected field appears in the resulting `Config` (an inferred behavior, not stated explicitly in the spec — same category of assumption as the `DATABASE_URL` non-connection-string row below; flagged for spec confirmation, not treated as certain) |
| Config file with the same key declared twice (TOML duplicate key) | Whatever the TOML parser's own defined behavior is (error, per the TOML spec) surfaces as an FR-6 error naming the file, not a silently-picked value |
| `BIND_ADDRESS` set to an empty string via environment | Treated as "no value from this source" for FR-2 purposes, not as an explicit invalid value — falls through to the next source or the default, never a validation error for being empty specifically (distinct from being present-and-malformed) |
| `LOG_LEVEL` set to a value differing only in case (`INFO`, `Debug`) | Accepted — matching is case-insensitive (`backend-configuration.md` FR-4, amended [`0025`](../reviews/0025-spec-amendment-backend-configuration-log-level.md)), the value lowercased before comparison against the four enum values |
| `--config` pointing at a directory, not a file | `Load` errors, names the path, does not attempt to parse a directory as TOML |
| `--config` pointing at a file with no read permission | `Load` errors naming the path and that it could not be read, not a raw OS-level permission error surfaced verbatim — this test is skipped automatically when the test process has root/bypass privileges that make `chmod`-based denial unreliable (e.g. an `if os.Geteuid() == 0 { t.Skip(...) }` guard), rather than asserted unconditionally in every CI environment |
| An environment variable set for a key not in FR-4's table (e.g. `HTTP_REQUEST_TIMEOUT`, the retired key name from an earlier draft) | Silently ignored — the blanket-environment-scan prohibition (API and contracts) means only named keys are ever read, so an unrelated or stale-named variable has no effect |
| `DATABASE_URL` set to a non-connection-string value (garbage text) | Explicitly not validated as a real connection string by this spec (type is declared but no parse rule is given in FR-4/FR-6 for this key specifically) — this plan asserts `Load` accepts it as-is and passes it through unexamined; real connection validation is `backend-persistence.md`'s concern at connect time, not this spec's |
| Every optional key's environment variable set simultaneously to valid values, with a config file also setting all of them to different valid values | Environment wins for every key independently (FR-2) — proven as one combined test in addition to the per-key table, since a per-key-only test suite could still miss a bug where one source's presence incorrectly affects a different key |

## Fixtures and test data

- Temp-directory TOML files, generated per test case, never a checked-in
  fixture file that could drift silently from what a test string asserts
  — each test writes exactly the TOML content it needs.
- `t.Setenv` for every environment-variable case — no process-wide
  environment mutation that could leak between parallel tests.
- A fake/spy `slog.Handler` capturing structured output for the FR-7
  redaction tests, so assertions run against parsed captured attributes,
  not string-matching raw log output.
- No real filesystem paths outside `t.TempDir()`; the well-known fallback
  location (FR-5) is tested against an injectable base-directory function,
  not the real OS per-user config directory, so the test suite never
  touches a real user's actual config path.

No real credentials, no real user data — `DATABASE_URL` test values are
syntactically-plausible but fake connection strings, never a real
database credential, even in a temp file that gets deleted.

## What is deliberately not tested

- Real validation of `DATABASE_URL` as a genuine, connectable connection
  string — the spec itself defines no such rule for this key (see the
  adversarial case above); `backend-persistence.md`'s own test plan is
  where connection-time validation is covered.
- `DB_POOL_MAX_CONNS` and the three `HTTP_*_TIMEOUT` keys' actual default
  *values* — the spec's own Open questions defer these numbers to
  `backend-persistence.md` and `backend-http-transport.md` respectively;
  this plan tests that each key parses its declared type correctly, not
  a specific default number not yet fixed here.
- Config file permissions as a security control — the spec's own Open
  questions explicitly defer this to phase 12, when a real secret first
  lives in the file; nothing here to test yet.
- `--config`'s long-term shape (flag vs. fixed fallback vs. some other
  split) — the spec's own Open questions flag this as possibly revisited
  once `architecture-desktop-host.md`'s real spawn-time argument list
  exists; this plan tests the rule as currently specified, not a
  hypothetical future one.
- Concurrent calls to `config.Load` — the spec's own State transitions
  section says configuration is resolved once, synchronously, at startup;
  no concurrent-access property is claimed, so none is tested.
- Resolving `backend-configuration.md`'s own internal inconsistency
  between `config.Load(sources...)` (FR-1) and `config.Load(configPath)`
  (API and contracts) — this plan's tests are written against the intent
  (one call, fully validated `Config` or an error) and don't commit to
  one exact signature; whichever the implementation lands on, the tests'
  call sites adapt without changing what's asserted. Flagged for spec
  clean-up, not resolved here.

## Exit criteria

- [ ] Every functional requirement (FR-1 through FR-8) maps to at least
      one test above
- [ ] Every adversarial case above has a test
- [ ] Tests were observed to fail before the implementation existed
- [ ] The suite is deterministic across repeated runs — no real filesystem
      paths outside `t.TempDir()`, no shared process environment, no
      network
