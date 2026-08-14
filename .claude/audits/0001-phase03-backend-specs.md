# Security audit: Phase 03 backend foundation specs

| | |
|---|---|
| **Scope** | `.claude/specs/backend-service-lifecycle.md`, `backend-configuration.md`, `backend-errors-and-logging.md`, `backend-http-transport.md`, `backend-persistence.md`, `backend-test-harness.md`, ADRs 0011–0013 |
| **Auditor** | Claude (Sonnet 5), via `/security-review`, two-phase methodology (candidate identification, then independent per-finding false-positive verification) |
| **Date** | 2026-08-14 |
| **Commit** | e083837 (specs as reviewed) |
| **Verdict** | Findings open → Fixed same session (see below) |

## Scope and method

No implementation code exists yet for phase 03 — this audit examined the
six `APPROVED` design specs and three ADRs as the normative source of the
code about to be written from them. Method: an initial candidate-finding
pass reading all specs against the project's threat model (constitution),
followed by three independent verification passes (one per candidate),
each re-reading the relevant specs cold and applying a false-positive
filter calibrated for code-diff review but adapted for design-doc review.
Not code reading, dependency review, or fuzzing — none of those apply yet
to specs with no implementation.

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| Config sources → `internal/config` | Config file, environment variables | Operator-trusted, not attacker-controlled (confirmed correct assumption for `BIND_ADDRESS`; see Finding not-included below) |
| `pgx`/TOML parser error text → log output | Third-party library error strings | Assumed safe to log verbatim; **wrong** for connection-string-bearing errors — this audit's one finding |
| `/healthz`/`/readyz` → LAN (post-phase-13) | Unauthenticated device on local network | Deliberately, explicitly accepted risk (see below) |

## Adversarial questions asked

- What does a malicious **user** of this instance achieve here? — Not
  applicable yet; no authenticated surface exists in phase 03 (phase 12).
- What does a malicious **source** achieve? — Not applicable; sources
  arrive in phase 06+, out of this phase's scope.
- What does a device on the **local network** achieve without
  credentials? — Examined `/healthz`/`/readyz`'s deliberate exemption
  from auth (`backend-http-transport.md` FR-5). Confirmed as a reasoned,
  scoped, industry-standard risk acceptance, not a vulnerability —
  independently verified, rejected as a finding (2/10 confidence).
- What happens when input is malformed, empty, enormous, duplicated,
  slow, or never arrives? — Covered by each spec's own Adversarial cases
  (test plans, not this audit's scope) rather than re-derived here.
- **What is logged that shouldn't be?** — This is where the one real
  finding came from: a `DATABASE_URL`-bearing connection string can reach
  stdout/CI logs via three call sites' raw third-party error text,
  despite the project's own type-level redaction mechanism (FR-7/FR-8)
  correctly protecting the typed `Config` value everywhere it's checked.
- What happens if two of these run at once? — Not applicable to this
  finding; not a concurrency issue.

## Findings

| ID | Severity | Title | Status |
|---|---|---|---|
| A-01-01 | Medium | DSN leakage via unredacted third-party error text at three call sites | Fixed |

### A-01-01 — DSN leakage via unredacted third-party error text

**Severity:** Medium

**Component:** `backend-service-lifecycle.md` FR-3, `backend-persistence.md` FR-6, `backend-configuration.md` FR-6

**Description** — `backend-configuration.md` FR-7 and `backend-errors-and-logging.md` FR-8 require `DATABASE_URL`'s typed value to implement dual-interface redaction (`slog.LogValuer` + `json.Marshaler`) wherever the `Config` struct or the field itself is logged. That protects the type. It does nothing for a different string: the `Error()` text of a `pgx` connect/parse error or a TOML parse error, which can itself embed the raw connection string. `backend-http-transport.md` FR-5 already recognized and closed this exact leak for `/readyz` specifically ("`pgx` connection errors can include the DSN in their error text; the handler MUST use a fixed, generic message… never the raw driver error's `Error()` string"). Three sibling call sites constructing the same class of error had no equivalent instruction: startup connection-failure logging, the migration runner's connection open, and a malformed-TOML parse error on the `DATABASE_URL` line.

**Impact** — In the dev/CI path (`DATABASE_URL` present, never read in production per `backend-configuration.md` FR-4), a transient DB outage or a typo'd connection string at startup or during migration would, per the specs' own "be specific" requirements, log the raw driver/parser error text — potentially including host, user, database, and (in some DSN forms) password — to stdout/CI logs, a channel constitution §8 explicitly forbids for credentials. Low-value target (dev/CI-only, loopback database with no real data at stake per the spec's own framing) but a direct, concrete violation of a named constitutional rule, not a hypothetical one.

**Preconditions** — `DATABASE_URL` set (dev/CI path only); a connection, parse, or migration-connect failure occurs; the implementation follows the spec literally as it was written before this fix (no redaction instruction existed for these three sites).

**Reproduction** — Not applicable to a design-spec audit (no code exists); the finding is a gap in the specification itself, verified by close reading against the pattern the spec authors already applied correctly at a fourth, sibling call site.

**Recommendation** — Extend `backend-http-transport.md` FR-5's fixed-generic-message pattern to all three sites. Done — see Resolution.

**Resolution** — Fixed in commit (pending, this session): amended `backend-service-lifecycle.md` FR-3, `backend-persistence.md` FR-6, and `backend-configuration.md` FR-6 to require the same redaction pattern, each scoped narrowly to the connection/parse-failure case so genuine non-sensitive diagnostic detail (which migration file, which config key) is preserved. Review record: [`0028`](../reviews/0028-spec-amendment-dsn-redaction.md).

---

## Candidates investigated and rejected

Two other candidates were generated in the initial pass and independently
verified as false positives, not included in Findings above:

- **Unauthenticated `/healthz`/`/readyz` after LAN exposure** —
  `backend-http-transport.md` FR-5's own Security considerations section
  already reasons through this exact tradeoff explicitly and scopes it;
  constitution §6 sequences LAN exposure strictly after phase 12's auth.
  A documented decision, not an oversight. Confidence 2/10.
- **`BIND_ADDRESS` loopback validation stated by example, not a canonical
  algorithm** — `BIND_ADDRESS` is operator/config-trusted input, never
  attacker-supplied; no threat actor for this finding under this
  project's own trust model. A spec-clarity note, not a vulnerability.
  Confidence 2/10.

## Severity guide

Per `.claude/templates/audit.md` — rated against actual impact in this
system, not textbook worst case.

## What was not examined

This audit covers design specs only — no implementation exists yet to
audit for memory safety, timing, concurrency bugs, or dependency
vulnerabilities. A full audit against real code is still required before
phase 03 closes (constitution §10, phase 03's own exit criteria: "Security
audit recorded, no open Critical or High findings" — this audit satisfies
that at the spec level; the implementation-level audit is separate,
future work). The four-attacker framework's "malicious source" and
"hostile content" categories don't apply to this phase's scope (no source
integration or file parsing exists until phase 06+) and were not
meaningfully exercisable here.
