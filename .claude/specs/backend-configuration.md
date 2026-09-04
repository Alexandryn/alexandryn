# Spec: Backend configuration

| | |
|---|---|
| **Status** | `APPROVED` (amended post-approval six times — `LOG_LEVEL` case-sensitivity ([`0025`](../reviews/0025-spec-amendment-backend-configuration-log-level.md)), DSN redaction in TOML parse errors ([`0028`](../reviews/0028-spec-amendment-dsn-redaction.md)), needs re-confirmation; `OPEN_LIBRARY_USER_AGENT` key added for phase 07 ([`0034`](../reviews/0034-phase07-cross-spec-review.md)), re-confirmed 2026-08-15; `DATABASE_URL`'s target-dependent meaning for the container topology ([`0042`](../reviews/0042-spec-backend-configuration-container-topology.md)), needs maintainer re-confirmation; FR-8's `BIND_ADDRESS` classification rewritten for ADR 0017 (2026-08-18), maintainer-directed, self-reviewed; FR-8's interim note added 2026-08-26 closing out audit A-03-05 (code was already stricter than spec text; spec now says so), self-reviewed, needs maintainer re-confirmation; **phase 13 (2026-09-02, `DRAFT`) — FR-4 gains six keys (`ACME_ENABLED`/`ACME_DOMAIN`/`ACME_EMAIL`/`ACME_CACHE_DIR`, `CORS_ALLOWED_ORIGINS`, `DEVICE_PAIRING_SECRET`) per ADR 0028; FR-8's interim note gains a phase-13 update pointer — pending spec review, not yet maintainer-confirmed**) |
| **Phase** | `03-backend-foundation` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-26 |
| **Supersedes** | — |
| **Reviewed in** | [`0022`](../reviews/0022-phase03-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (2 Blocking findings against this spec specifically), fixed; approved by maintainer 2026-08-14. Amended post-approval, [`0025`](../reviews/0025-spec-amendment-backend-configuration-log-level.md) — `LOG_LEVEL` case-sensitivity gap, self-reviewed, needs maintainer re-confirmation. Amended again, [`0028`](../reviews/0028-spec-amendment-dsn-redaction.md) — DSN redaction gap found by security review, self-reviewed, needs maintainer re-confirmation. Amended again, [`0034`](../reviews/0034-phase07-cross-spec-review.md) — `OPEN_LIBRARY_USER_AGENT` key added to FR-4's table for `backend-metadata-adapter.md` FR-6, cross-spec-reviewed, needs maintainer re-confirmation. Amended again, [`0042`](../reviews/0042-spec-backend-configuration-container-topology.md) — `DATABASE_URL`'s meaning made target-dependent for ADR 0015, self-reviewed, needs maintainer re-confirmation. Amended again, ADR 0017 (2026-08-18) — FR-8 replaces the loopback-only rule with the two-mode classification ADR 0017 decided, maintainer-directed in the same session that produced the ADR, self-reviewed |

## Context

`architecture-backend.md` FR-5 already fixed the precedence order
(compiled-in defaults, then a config file, then environment variables as an
override for `go run` outside Electron) and the fail-loudly rule. What it
didn't fix: the concrete list of configuration keys, their types and
validation rules, the config file's format and location, or how a
compiled-in default and "no default, this is required" are distinguished
from each other in code rather than in prose.

## Problem

Nothing has fixed: what configuration actually exists yet (a database
connection target, a bind address, a shutdown grace period, a log level —
named piecemeal across other phase 01/03 specs but never collected), the
file format, where it lives, or the actual validation each key needs.

## Goals

- Collect every configuration value named so far by other specs into one
  authoritative list, typed and validated
- Fix the config file's format and location
- Fix precedence resolution as a concrete algorithm, not just an ordering
  of sources
- Fix what "fails loudly" means as actual behavior: what's printed, what
  exit code, before any other subsystem initializes
- Draw the line between a value with a safe compiled-in default and a
  value that has none and must be supplied

## Non-goals

- Runtime-mutable configuration (a settings UI that changes behavior
  without a restart) — no such feature exists yet in any phase; if one
  arrives, it's a new spec, not an extension of this one, since "changes
  without a restart" is a materially different reliability story than
  "resolved once at startup"
- Credential/secret configuration for authentication — phase 12; this spec
  covers what exists before then (a database connection target is not a
  user credential)
- The config file's *creation* — `architecture-desktop-host.md` FR-5 already
  says Electron delivers a config file at spawn time in production; this
  spec defines what the Go server does with a file that exists, not how
  Electron produces it

## User stories

- As **`backend-service-lifecycle.md`'s startup sequence**, I want one
  function that returns a fully validated config or a specific error, so
  step 1 of the sequence is a single call, not scattered per-field checks.
- As **a backend developer running `go run ./cmd/server` directly**, I want
  environment variables to work without a config file existing, so local
  iteration doesn't require simulating Electron's spawn-time file delivery.
- As **a contributor adding a new configuration value later**, I want a
  fixed pattern (typed field, compiled default or none, validation
  function) to extend, not a decision to make from scratch each time.

## Functional requirements

- **FR-1** Configuration MUST be represented as one Go struct
  (`internal/config.Config`), constructed by one function
  (`config.Load(sources...)`) that returns either a fully validated
  `Config` or an error — never a partially populated struct with some
  fields still needing validation by the caller. Nothing outside
  `internal/config` MUST read an environment variable or the config file
  directly; every other package receives configuration only through this
  struct, passed at construction (`backend-service-lifecycle.md` FR-2).
- **FR-2** Resolution order, per key, MUST be: (1) the compiled-in default,
  if one exists for that key; (2) the value from the config file, if the
  file exists and the key is present in it; (3) the value from the
  matching environment variable, if set — each source overrides the
  previous one for that specific key, independently; there is no
  all-or-nothing "use the file OR use env vars" switch. This is
  `architecture-backend.md` FR-5's precedence, made concrete as a
  per-key algorithm rather than a per-source one.
- **FR-3** Every configuration key MUST be declared as exactly one of:
  **required** (no compiled-in default; resolution failing to find a value
  from the file or environment for this key is a validation error),
  **optional with a stated default** (a value exists even if no source
  provides one), or **optional with no default, where absence is itself a
  meaningful, valid signal the calling code checks for explicitly** — used
  only where a key's absence is a real, named alternative code path (FR-4's
  `DATABASE_URL` is the one instance of this category in the current
  surface, and its own row states what its absence signals), never as an
  escape hatch for a key that's merely "usually fine to omit." A key MUST
  NOT be **required in one runtime context and unread in another** —
  `config.Load` has no signal telling it which context it's in, so
  "context-dependent requiredness" is not a legal fourth category; a key
  whose relevance depends on context is the third category above, with the
  context-reading logic living in whatever code checks for its absence,
  never in `config.Load` itself.
- **FR-4** The initial configuration surface, typed and sourced per FR-2/
  FR-3:

  | Key | Type | Required? | Default | Source of the requirement |
  |---|---|---|---|---|
  | `DATABASE_URL` | connection string | Optional, no default — absence is a meaningful signal (FR-3's third category), never a validation failure | — | ADR 0004's addendum (Supabase dev stack); `architecture-system.md` Security considerations; ADR 0015 (container target, where its presence is normal) |
  | `BIND_ADDRESS` | host:port, host classified per FR-8 (loopback/private always legal; public requires a valid cert) | Optional | `127.0.0.1:0` (loopback, OS-assigned port) | `architecture-system.md` FR-3, constitution §6, ADR 0017, FR-8 below |
  | `TLS_CERT_FILE` / `TLS_KEY_FILE` | filesystem paths | Required only when `BIND_ADDRESS` resolves to a publicly routable address (FR-8); otherwise unread | — | ADR 0017, FR-8 below |
  | `LOG_LEVEL` | enum: `debug`/`info`/`warn`/`error`, matched case-insensitively | Optional | `info` | `backend-errors-and-logging.md` |
  | `SHUTDOWN_GRACE_PERIOD` | duration | Optional | `10s` | `backend-service-lifecycle.md` FR-5, `architecture-system.md` FR-9's placeholder |
  | `DB_POOL_MAX_CONNS` | integer | Optional | a number `backend-persistence.md` fixes (this spec only reserves the key) | `architecture-persistence.md` FR-3 |
  | `HTTP_MAX_BODY_BYTES` | integer (bytes) | Optional | a number `backend-http-transport.md` fixes (this spec only reserves the key) | `architecture-backend.md` FR-6, constitution §4 |
  | `HTTP_READ_TIMEOUT` | duration | Optional | a duration `backend-http-transport.md` FR-2 fixes | constitution §4 |
  | `HTTP_WRITE_TIMEOUT` | duration | Optional | a duration `backend-http-transport.md` FR-2 fixes | constitution §4 |
  | `HTTP_IDLE_TIMEOUT` | duration | Optional | a duration `backend-http-transport.md` FR-2 fixes | constitution §4 |
  | `OPEN_LIBRARY_USER_AGENT` | string, non-empty | Required, no default — a placeholder default would misidentify this client to Open Library, which FR-3's "required" category exists to prevent | — | `backend-metadata-adapter.md` FR-6, Open Library usage policy |
  | `ACME_ENABLED` | boolean | Optional | `false` | ADR 0028 §1–2, `backend-network-transport.md` FR-1. Selects a certificate *source* (ACME issuance vs. static file) for a public bind; not a security toggle — both Mode A branches fail closed identically. |
  | `ACME_DOMAIN` | DNS name | Required only when `ACME_ENABLED` is `true` (and, if `BIND_ADDRESS`'s host is a name, must equal it); otherwise unread | — | ADR 0028 §2, `backend-network-transport.md` FR-2. Pins `autocert` `HostPolicy` to exactly this name. |
  | `ACME_EMAIL` | string (email) | Optional | — (empty) | ADR 0028 §2. Passed to `autocert` for CA expiry notifications; issuance works without it. |
  | `ACME_CACHE_DIR` | filesystem path | Optional | `acme/` under the per-user data directory (`architecture-persistence.md` FR-1) | ADR 0028 §2. `autocert.DirCache`, created `0700` — holds the ACME account key and issued certificate keys. |
  | `CORS_ALLOWED_ORIGINS` | comma-separated list of `scheme://host[:port]` | Optional | — (empty ⇒ no cross-origin request is ever honoured; the same-origin SPA is unaffected) | ADR 0028 §4, `backend-network-transport.md` FR-6. A malformed entry (path present, no scheme) is a validation error. |
  | `DEVICE_PAIRING_SECRET` | string, redacted type (FR-7) | Optional | — (empty ⇒ `pair/initiate` requires only an admin token) | ADR 0028 §6, `backend-network-api.md` FR-1. An *additional* factor on `pair/initiate`; never a login credential and never accepted in place of one. |

  This table is the authoritative key list at the time this spec is
  written; a later phase adding a key extends this table rather than
  inventing a parallel one elsewhere. `DATABASE_URL`'s presence or
  absence specifically is the signal `backend-persistence.md` FR-5 uses to
  choose between connecting directly (a value is present) and spawning and
  owning a bundled Postgres instance itself (no value present) — what that
  signal *means* now depends on which deployment target is running
  (amended 2026-08-16, ADR 0015; previously described as a dev/CI-vs-
  production split, before a second target existed): in the
  **Electron-hosted target**, its absence is the production case (the Go
  server spawns and owns Postgres) and its presence is the
  developer/CI/test override; in the **container-hosted target**, its
  presence is the *normal* production case (either the bundled sibling
  container's Compose-computed address, or an operator-supplied external
  instance) and there is no spawn-and-own path available to fall back to
  at all. `config.Load` itself does not need to know which target or
  which case it's in — it only needs to never treat the key's absence as
  an error, which is what FR-3's third category guarantees regardless of
  target. Earlier drafts of this table described the key as
  "required in dev," which FR-3 above no longer permits — reconciled here
  by dropping that framing and letting the mode-selection logic live in
  the code that reads the value's presence (`backend-persistence.md`
  FR-5), not in `config.Load`.
  Originally this table reserved a single `HTTP_REQUEST_TIMEOUT` key with
  no wiring to any of `backend-http-transport.md`'s three actual timeout
  fields (`ReadTimeout`/`WriteTimeout`/`IdleTimeout`) — an unusable
  reservation, since nothing consumed it. Split into three keys above,
  each mapped explicitly to the field it configures.
  `LOG_LEVEL`'s matching was originally unspecified for case, a gap found
  while writing this spec's test plan (`.claude/reviews/0024`): an
  environment variable's incidental casing (`LOG_LEVEL=Info` vs `info`)
  is not the kind of typo constitution §11's fail-loudly stance is meant
  to punish, so matching is case-insensitive — the value is lowercased
  before comparison against the four enum values, and validation and
  redaction (FR-6, FR-7) are unaffected since `LOG_LEVEL` carries no
  sensitive data.
- **FR-5** The config file MUST be TOML, located at a path passed to
  `cmd/server` via a single command-line flag (`--config <path>`),
  optional — or, if the flag is absent, resolved from a fixed
  well-known location under the OS's standard per-user config directory —
  the same directory family `architecture-persistence.md` FR-1 already
  uses for the data directory, keeping "where does Alexandryn keep its
  own files" answered once, not per-subsystem. A missing config file is
  not itself an error (FR-2's per-key resolution still lets environment
  variables and defaults fill every key) — a config file with *invalid
  syntax* (unparseable TOML) IS a validation error, distinct from "file
  absent," per FR-6.
- **FR-6** Any of the following MUST cause `config.Load` to return an
  error, and MUST NOT cause a guessed or partial config to be returned:
  a required key resolving to no value from any source; a key's value
  failing its type's validation (e.g. `BIND_ADDRESS` not parseable as a
  host:port, `LOG_LEVEL` not one of the enum values); the config file
  existing but containing invalid TOML syntax. The error message MUST
  name the specific key and what was wrong with it (constitution §11) —
  never a generic "invalid configuration." When the invalid-TOML-syntax
  case's offending line or token is FR-7's `DATABASE_URL` (or any future
  sensitive key), the error message MUST name the key generically (e.g.
  "value for DATABASE_URL is invalid") and MUST NOT include the raw
  parser-reported excerpt, which can otherwise echo the offending line's
  actual content verbatim — the same class of leak `backend-http-transport.md`
  FR-5 and `backend-service-lifecycle.md` FR-3 redact for the DSN
  specifically, applied here to the TOML parser's own error text
  (security review finding, 2026-08-14).
- **FR-7** `DATABASE_URL`, and any future key whose value is a connection
  string or credential, MUST implement **both** `slog.LogValuer` and
  `json.Marshaler`, each returning the same fixed redacted placeholder
  (e.g. `"[redacted]"`) instead of the real value. Both are required, not
  either: `log/slog`'s JSON handler (`backend-errors-and-logging.md`
  FR-6) resolves `slog.LogValuer` when a value is logged as its own
  structured attribute, but falls back to `encoding/json`'s
  reflection-based marshaling — which has no knowledge of
  `slog.LogValuer`, only `json.Marshaler` — when a *containing* struct
  (e.g. the whole `Config`) is logged as one attribute via `slog.Any`.
  A type implementing only `slog.LogValuer` would still leak its raw
  value the moment someone logs `Config` itself rather than the field
  directly; requiring both closes that path. `internal/config.Config`
  MUST NOT be logged as a single attribute anywhere in this codebase
  without going through a method that redacts every sensitive field
  first — this spec does not provide such a method as a safe default, so
  the absence of one is itself the guard: there is no convenient way to
  log the whole struct at all. Ties directly into
  `backend-errors-and-logging.md`'s redaction contract; this spec is
  where the *type* carrying that behavior is defined for configuration
  specifically, since config is the first place a sensitive value exists
  in the process.

- **FR-8** `config.Load` MUST classify `BIND_ADDRESS`'s resolved host into
  exactly one of three categories, and enforce the matching rule, per ADR
  0017:
  - **Loopback or private-range** (`127.0.0.0/8`, `::1`, `localhost`, an
    RFC 1918 IPv4 private range, or an IPv6 unique local address) — always
    legal. No certificate is required of this process; TLS, if any,
    terminates upstream (a reverse proxy), outside this guarantee.
  - **Publicly routable, with `TLS_CERT_FILE`/`TLS_KEY_FILE` both present
    and valid** (loads, parses, key matches certificate, not expired) —
    legal. An invalid or missing certificate while `BIND_ADDRESS` resolves
    to a public address MUST fail startup before any other subsystem
    initializes, exactly like a missing required key — never a degrade to
    an unencrypted listener on a public address.
  - **Publicly routable, with no valid certificate configured** — MUST
    fail startup. This is the only case FR-8 previously covered in full;
    it remains rejected, now as one of three classified outcomes rather
    than the only one.
  This is phase 03's own named exit criterion ("the service refuses to
  bind to a non-loopback address"), phase 13's `TLS_CERT_FILE`/`TLS_KEY_FILE`
  keys, and constitution §6's condition made concrete as a validation rule
  — not a documentation-only expectation, and not weakened by this
  amendment: what changes is which addresses are legal, not whether the
  check is enforced at startup, unconditionally, before any other
  subsystem initializes.

  **Interim note history — the unconditional public-bind refusal is
  lifted for the static-certificate case (2026-09-03, phase 13 Tier 0,
  `DRAFT`).** From 2026-08-26 to 2026-09-03, `validateBindAddress` refused
  *every* publicly routable `BIND_ADDRESS` regardless of certificate
  validity, because no code path served TLS — "a certificate nothing uses
  to encrypt anything looks enforced without being enforced." Phase 13
  Tier 0 changes that:
  - `internal/config/bindaddress.go` classifies the host per ADR 0028 §1
    (IP literal by range; `localhost` private; any other string public,
    no DNS lookup), validates a static `TLS_CERT_FILE`/`TLS_KEY_FILE`
    pair (well-formed, key-match, in-window, and — for a named public
    host — SAN coverage via `x509.Certificate.VerifyHostname`), and
    **accepts** a public bind with a valid pair (or a private bind with
    the opt-in pair). The validated `*tls.Certificate` is stored on
    `Config` (`TLSCertificate()`).
  - `cmd/server/run.go` wraps the listener in `tls.NewListener` whenever
    `Config.TLSCertificate()` is non-nil — so an accepted public bind
    actually serves in-process TLS, never plaintext. The safety property
    the old note protected (no plaintext on a public address) holds
    without the blanket refusal.
  - **Phase 13 Tier 2 (2026-09-04, `DRAFT`):** `validateBindAddress` now
    also accepts `ACME_ENABLED=true` on a public bind (requires
    `ACME_DOMAIN`; if `BIND_ADDRESS`'s host is a name it must equal
    `ACME_DOMAIN`; mutually exclusive with a static `TLS_CERT_FILE`/
    `TLS_KEY_FILE`). It records `Config.Reachability()` (loopback / private
    / public) and `Config.TLSMode()` (none / static / acme). `cmd/server`
    builds the `autocert.Manager` (`HostWhitelist(ACME_DOMAIN)`,
    `DirCache` under `ACME_CACHE_DIR` or `acme/` in the data dir,
    `AcceptTOS`), wires `GetCertificate`, and runs a `:80` HTTP→HTTPS
    redirect listener (serving `manager.HTTPHandler` in ACME mode) for
    every public bind. The `NewTLSConfig` cipher/version/ALPN policy and
    the HSTS middleware also landed. **Still open:** the Pebble
    certificate-lifecycle integration test is written but self-skips —
    Pebble confirms end-to-end issuance locally, the automated assertion
    hits an `x/crypto` acme-client ↔ Pebble finalize-response
    incompatibility on cert download (phase-13 follow-up). This whole
    note is deleted, and the middle bullet stands unqualified, once that
    test is green and the maintainer re-confirms.

## Non-functional requirements

- **Performance** — not applicable; configuration is resolved once at
  startup, not on a hot path.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; no UI.
- **Reliability** — FR-6's "fail loudly, never a partial config" is the
  reliability property `architecture-backend.md` FR-5 already named as
  the point of this whole spec.
- **Observability** — successful config load MUST log which source
  provided each *non-default* value (file vs. environment — not the
  value itself, for any key FR-7 marks as sensitive), so a support
  conversation about "why is it behaving like X" can start from "here's
  what was actually resolved" rather than guessing.

## Domain model

Not applicable — this spec is process configuration, not the Alexandryn
domain.

## API and contracts

- **`cmd/server` ↔ `internal/config`**: one call, `config.Load(configPath)`,
  returning `(*Config, error)` — the only contract this spec defines.
- **`internal/config` ↔ the config file**: TOML, FR-5's location rules.
- **`internal/config` ↔ the environment**: read only for keys FR-4 lists,
  never a blanket "load all environment variables" scan — an unrelated
  environment variable existing on the host must never accidentally
  populate a config key it wasn't meant for.

## State transitions

Not applicable — configuration is resolved once, synchronously, at the
start of `backend-service-lifecycle.md` FR-1 step 1; it has no runtime
states of its own.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Required key missing from every source | FR-6's validation | (via Electron) startup failure naming the missing key | `config.Load` returns an error before any other startup step runs |
| Config file has invalid TOML syntax | FR-6's parse step | Startup failure naming the file and the parse error, not a raw parser stack trace | Same as above |
| A key's value fails type/enum validation | FR-6's validation | Startup failure naming the key and what was expected | Same as above |
| Config file path (via `--config`) points to a file that doesn't exist | FR-5's resolution | Startup failure naming the missing path, if the flag was explicitly passed (an explicitly wrong path is a real error, distinct from "no flag given, file absent is fine") | `config.Load` returns an error — this is the one case where "file absent" is treated as a failure, because the user explicitly pointed at it |
| `BIND_ADDRESS` resolves to a publicly routable host with no valid certificate | FR-8's validation | Startup failure naming the offending address and the missing/invalid certificate condition | `config.Load` returns an error before any other startup step runs — same treatment as any other FR-6 validation failure |

## Security considerations

- **`DATABASE_URL`'s production meaning is target-dependent (FR-4, amended
  2026-08-16, ADR 0015)** — in the Electron-hosted target, it is never
  read in production: production generates its own connection internally
  (restating `architecture-system.md`'s Security considerations), and this
  key's surface there exists for the dev/CI path only. In the
  container-hosted target, the reverse is true by design: `DATABASE_URL`
  is the normal way production learns where Postgres is, whether that's
  the bundled sibling container or an operator-supplied external
  instance. Both statements are true, of different targets — an earlier
  version of this paragraph stated the first as an unqualified rule before
  a second target existed. The type system (FR-7) prevents the value from
  leaking into logs in either target, unconditionally — that protection
  was never target-specific and needs no amendment.
- **No blanket environment scan (API and contracts, above)** — reading
  only named keys, never enumerating `os.Environ()`, is itself a
  narrow-surface security property: a host environment variable that
  happens to share a name pattern with something sensitive can't
  accidentally flow into the process's config.
- **Config file permissions** — not designed in detail here (no secret
  lives in the file yet, since `DATABASE_URL` in dev is a loopback-only
  database with no real data at stake), but flagged: once phase 12
  introduces a real credential that might live in this file, its
  filesystem permissions become a real question this spec doesn't yet
  answer. Named in Open questions, not assumed solved.
- **`--config` path is operator-supplied, not attacker-supplied** — the
  flag is passed by Electron (trusted, same trust level as the binary
  path itself, `architecture-system.md`'s Security considerations) or by
  a developer's own shell; this spec does not need to defend against a
  hostile path here the way `domain-source.md` defends against a
  hostile filename from an external source.
- **`BIND_ADDRESS`'s two-mode classification (FR-8) is this spec's
  concrete implementation of constitution §6 and ADR 0017** — a config
  file or environment variable setting `BIND_ADDRESS` to a publicly
  routable host with no valid certificate configured now fails startup
  the same way a missing required key does, rather than silently
  binding an internet-reachable socket with nothing to authenticate the
  connection or terminate TLS in-process. Loopback and private-range
  binds remain always legal, unconditionally — the check that changed
  is which addresses require a certificate, not whether the check runs.
- **Dual-interface redaction (FR-7) closes a real gap the single-interface
  version left open** — `slog.LogValuer` alone protects a value logged as
  its own attribute; it does not protect the same value nested inside a
  struct logged as one attribute via `slog.Any`, since `log/slog`'s JSON
  handler falls back to `encoding/json`'s reflection-based marshaling in
  that case, which only respects `json.Marshaler`. Requiring both closes
  the gap regardless of which logging call site a future contributor
  writes.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Precedence resolution (FR-2) per key, in isolation, with fake sources; every FR-6 failure case, proven to return an error and never a partial `Config`; FR-8's two-mode classification, proven with loopback/private addresses accepted unconditionally, a publicly routable address accepted with a valid certificate and rejected with an invalid, expired, or missing one; FR-7's redaction, proven two ways — a known `DATABASE_URL` value logged directly, and the *whole* `Config` struct logged as one attribute — both asserted absent from captured output |
| Integration | `backend-service-lifecycle.md`'s own startup test already covers config load as step 1 — no separate integration layer needed here |
| Contract | N/A |
| Accessibility | N/A |

## Acceptance criteria

- [ ] `config.Load` returns a fully validated `Config` or a specific error,
      never a partial struct — proven with a test per FR-6 failure case
- [ ] Every key in FR-4's table has a passing precedence test (default →
      file → environment, each overriding the last)
- [ ] A publicly routable `BIND_ADDRESS` (e.g. `0.0.0.0:8080`) with no
      valid certificate is rejected at startup, proven with a test —
      phase 03's own named exit criterion, updated for ADR 0017's
      two-mode rule; a publicly routable address *with* a valid
      certificate, and loopback/private addresses unconditionally, are
      proven accepted by the same test suite
- [ ] A test proves `DATABASE_URL` never appears in any log line or error
      string produced by a `Config` load, whether successful or failed,
      including when the whole `Config` struct is logged as one attribute
      rather than the field alone
- [ ] Every FR maps to a line in phase 03's own exit criteria

## Open questions

- **Config file permissions once a real secret exists (phase 12)** — not
  designed here; flagged for whoever picks up credential storage
  (`architecture-persistence.md`'s "Open questions" already names a
  parallel gap for encryption at rest — this is the config-file analog).
- **`DB_POOL_MAX_CONNS` and `HTTP_MAX_BODY_BYTES`/`HTTP_READ_TIMEOUT`/
  `HTTP_WRITE_TIMEOUT`/`HTTP_IDLE_TIMEOUT` default values** — reserved as
  keys here (FR-4), actual numbers owned by `backend-persistence.md` and
  `backend-http-transport.md` respectively, consistent with this phase's
  own pattern of fixing shape before numbers.
- **Should `--config` become optional-with-a-flag-default rather than
  read from a fixed well-known path when absent?** FR-5 picks "flag, or a
  fixed fallback location" as the simplest rule that works for both the
  Electron-spawns-with-a-flag case and the bare `go run` case; worth
  revisiting once `architecture-desktop-host.md`'s actual spawn-time
  argument list is implemented, in case it argues for a different split.

## References

- `architecture-backend.md` FR-5 — the precedence order and fail-loudly
  rule this spec makes concrete
- `architecture-system.md` FR-3 (bind address), FR-9 (shutdown grace
  period), Security considerations (`DATABASE_URL` never in production)
- `architecture-persistence.md` FR-1 (per-user data directory convention,
  reused here for the config file location), FR-3 (connection pool bound)
- `architecture-desktop-host.md` FR-5 — Electron delivering the config
  file at spawn time
- `backend-service-lifecycle.md` FR-1 step 1, FR-2 — where config load
  sits in the startup sequence, and the no-globals rule this spec's
  single-struct design supports
- `backend-errors-and-logging.md` — the redaction contract FR-7 ties into
- ADR 0015 — the container-hosted target `DATABASE_URL`'s FR-4 row and
  Security considerations were amended 2026-08-16 to cover
- Constitution §8 (never log secrets), §11 (copy)
