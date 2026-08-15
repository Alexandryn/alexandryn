# Spec: Backend source adapter

| | |
|---|---|
| **Status** | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| **Phase** | `08-sources` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0035`](../reviews/0035-phase08-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (2 Blocking, 8 Major, 5 Minor), all findings fixed; approved by maintainer 2026-08-15 |

## Context

`domain-source.md` (phase 02) fixed the model: a `Source` declares
capabilities from a fixed set (`CanList`, `CanSearch`, `CanDownload`),
availability is expressed only as `SourceOffering` — "this `Source`
claims to offer this `Edition`, observed at T" — and a `FileReference`
is an opaque, `Source`-scoped identifier, never interpretable as a path
by the domain. `backend-persistence.md` (phase 03) fixed the repository
pattern this spec's storage reuses. `backend-metadata-adapter.md` (phase
07) established this project's first external-boundary discipline
(rate limiting, size/shape/timeout checks, normalise-at-the-boundary,
DTO-not-aggregate output) against a fixed, credential-free endpoint
(Open Library). This spec is this project's *second* external boundary,
and a harder one: the endpoint itself is user-configured and often
authenticated, exactly the case constitution §4 names directly ("a user
can be tricked into adding a malicious source").

OPDS, researched against its own real specifications rather than
invented shapes (`specs.opds.io/opds-1.2.html`, `specs.opds.io/opds-2.0.html`,
`drafts.opds.io/authentication-for-opds-1.0.html`):

- **OPDS 1.2** is Atom+Dublin Core XML. A feed document is an Atom
  `<feed>` with `<id>`, `<title>`, `<updated>`, `<author>`, a
  `<link rel="self">`, and zero or more `<entry>` elements. A
  **Navigation feed** (`type=".../profile=opds-catalog;kind=navigation"`)
  links to other feeds; an **Acquisition feed**
  (`.../kind=acquisition"`) contains entries with at least one
  Acquisition Link (`rel` beginning `http://opds-spec.org/acquisition`,
  e.g. plain acquisition, `open-access`, `borrow`, `buy`, `sample`,
  `subscribe`). Pagination follows RFC 5005 (`rel="next"`/`"previous"`
  links). Search, when offered, is a separate OpenSearch description
  document linked with `rel="search"`.
- **OPDS 2.0** is JSON, built on the Readium Web Publication Manifest.
  A feed's top level has `metadata` (`title`, optionally
  `numberOfItems`/`itemsPerPage`/`currentPage`), `links` (must include
  `self`; may include `search`), and at least one of `navigation`,
  `publications`, or `groups`. A `Publication` has its own `metadata`
  (`title`, `author`, `identifier`, `language`, `modified`,
  `description`), `links` (including acquisition-typed relations like
  `download`/`buy`/`borrow`), and `images`. Pagination is
  `links`-based (`rel="next"`/`"previous"`/`"first"`/`"last"`), search
  is a templated `rel="search"` link.
- **Authentication**: OPDS's own "Authentication for OPDS" draft covers
  both Basic Auth and OAuth 2.0/2.1 flows, but its own text notes Basic
  Auth "continues to be used by a large number of organizations and
  remains useful for simple use cases, such as self-hosted catalogs" —
  this project's exact use case. This spec scopes to Basic Auth only
  (Non-goals); the full Authentication Document flow is future work.

## Problem

Nothing exists yet to store a source's configuration or credential,
check whether it's reachable, list or search what it offers, or turn a
raw OPDS/local-folder response into something this project's domain or
UI can trust.

## Goals

- `Source` CRUD with encrypted-at-rest credential storage
- A health check: is this source reachable, run at create/update time
  and on demand
- A local-folder provider: list a configured directory's files
- An OPDS provider: fetch and normalise OPDS 1.2 and OPDS 2.0
  navigation/acquisition feeds, search when advertised
- Capability detection per source, at health-check time
- Every response and filename a source produces treated as hostile
  input (constitution §4), with the two sharpest concrete risks —
  SSRF via an off-origin continuation link, path traversal via a
  filename — closed explicitly, not assumed

## Non-goals

- Any async/scheduled re-checking of health or availability — phase 09;
  this spec's health check is synchronous, triggered by a create/update
  or an explicit on-demand call, never a background job
- The import pipeline (matching a browsed item to a domain `Work`/
  `Edition`, creating a `LibraryEntry`) — phase 10; this spec's
  browse/search output is a new, non-domain DTO, never a
  `domain-source.md` `SourceOffering` (see Domain model — no `Edition`
  exists yet for a browsed item to attach to)
- A public HTTP endpoint for downloading raw file bytes — this spec
  exposes a Go-level `Provider.Resolve(ctx, FileReference)` capability
  internally (for phase 10 to call), but no `/api/v1/sources/*`
  endpoint hands back file bytes; that would be a feature with nothing
  downstream to use it yet
- OAuth 2.0/2.1, the "Authentication for OPDS" Authentication Document
  flow — Basic Auth only, justified in Context above
- Any source protocol other than local-folder and OPDS 1.2/2.0

## User stories

- As **someone setting up their library**, I want to point this system
  at a folder of ebook files or an OPDS catalog and see immediately
  whether it's reachable, so I know my configuration is correct before
  relying on it.
- As **the maintainer**, I want a source's credential to be
  unreadable on disk and absent from every log line, so a leaked
  database file or log doesn't leak someone's library-server password.
- As **the system**, I want a malicious or compromised source to never
  be able to make this system fetch an arbitrary URL or read an
  arbitrary file outside what the user configured, no matter what it
  returns.

## Functional requirements

- **FR-1** `POST /api/v1/sources` creates a `Source`: `label` (string,
  validated per `domain-bibliographic.md` FR-5's validation pattern —
  `domain-source.md`'s own Domain model section already cites this
  exact provision for `Source.label`, reused here rather than
  reinvented), `kind` (`local-folder` | `opds`, closed
  vocabulary, `400 InvalidInput` for anything else), `config` (kind-specific:
  `{ basePath: string }` for `local-folder`, `{ baseUrl: string }` for
  `opds`), and an optional `credential: { username: string, password:
  string }` (Basic Auth only, `opds` sources only — a `credential` on a
  `local-folder` source is `400 InvalidInput`, since there's nothing to
  authenticate to). A synchronous health check (FR-6) runs as part of
  creation; the `Source` is still created even if the health check
  fails (an unreachable source right now may become reachable later —
  the user should be able to save the configuration and see *why* it's
  currently failing, not be blocked from saving it at all).
- **FR-2** `GET /api/v1/sources` lists every configured `Source`
  (`id`, `label`, `kind`, redacted `config` — `basePath`/`baseUrl` are
  shown, since they're not secret, but a present `credential` is
  represented only as `hasCredential: boolean`, never echoing the
  username or password back), each with its current health status
  (FR-6) and detected capabilities (FR-5). `GET /api/v1/sources/:id`
  returns the same shape for one source. `PATCH /api/v1/sources/:id`
  updates `label`/`config`/`credential` (a credential update replaces
  the stored value entirely; there is no partial-field credential
  update) and re-runs the health check. `DELETE /api/v1/sources/:id`
  removes the `Source` — per `domain-source.md` FR-6, this MUST cascade
  to remove every `SourceOffering` referencing it (a no-op in practice
  until phase 10 ever creates one, but implemented per the domain's own
  rule now, not deferred).
- **FR-3** Local-folder `config.basePath` MUST be validated, at both
  create/update time and every subsequent browse request, as: an
  absolute path, currently existing, a directory, and readable by the
  process — `InvalidInput` at create/update time for a shape violation
  (not absolute, control characters), and a health-check failure (FR-6,
  not a hard create-time rejection) for "doesn't exist" or "not
  readable," since the folder may exist later (an unmounted external
  drive, for instance). The Go server process is assumed to run under
  the same OS user account as the desktop session that spawned it —
  this spec's own reasoned inference from Electron spawning the Go
  server as a child process (`architecture-desktop-host.md`'s general
  process-spawning model), not a requirement that spec states
  explicitly itself. This spec does not itself solve cross-user
  filesystem permission scenarios.
- **FR-4** OPDS `config.baseUrl` MUST be validated as a well-formed
  `http`/`https` URL at create/update time (`InvalidInput` otherwise).
  A configured credential (FR-1) is sent as an HTTP `Authorization:
  Basic <base64>` header on every request to that source, including the
  health check and every browse/search call — never conditionally
  based on the response (no "try without auth first" probing, which
  would double every request and needlessly expose whether a resource
  is reachable anonymously). **Accepted risk, named explicitly rather
  than silently assumed**: Basic Auth is base64 encoding, not
  encryption; a credential attached to an `http://` (not `https://`)
  `baseUrl` is sent in near-cleartext on every request, and FR-4's
  own "always send" policy means this happens more often than a
  challenge-response scheme would. This project's accepted threat model
  for outbound source traffic is the same one constitution §6 already
  applies to inbound LAN traffic before phase 12/13 — a self-hosted,
  household-scale deployment, not a hardened multi-tenant service — so
  this spec does not block an `http://` source with a credential
  outright. `frontend-source-management.md` surfaces this concretely
  as a visible, non-blocking warning at the point a credential is
  attached to a non-`https` URL, rather than leaving it silent.
- **FR-5** Capability detection runs as part of the health check (FR-6),
  not per browse/search request: `CanList` is always `true` for both
  `kind`s once reachable (both protocols support listing by
  construction). `CanDownload` is always `true` once reachable (both
  protocols yield a resolvable `FileReference`). `CanSearch` is `true`
  for `opds` only when the root feed advertises a search mechanism (a
  `rel="search"` link — an OpenSearch description document for 1.2, a
  templated search link for 2.0) and always `false` for `local-folder`
  (full-text local file search is out of scope for this phase). The
  detected set is stored on the `Source` row and returned by FR-2; it is
  re-detected on every health check (create, update, or on-demand), not
  cached indefinitely, since a source's own capabilities can change
  (a catalog operator adding search support).
- **FR-6** `POST /api/v1/sources/:id/health-check` (also run
  automatically by FR-1/FR-2's create/update) performs a lightweight
  reachability probe: `local-folder` — the FR-3 path checks;
  `opds` — an HTTP `GET` of `config.baseUrl` with a 5-second timeout
  (`backend-http-transport.md`'s figure, reused for consistency, not
  literal precedent, the same phrasing `backend-metadata-adapter.md`
  FR-9 already established for this project's other external-timeout
  citation) and a 5 MiB response-size cap, expecting a `2xx` status and
  a body parseable as either OPDS 1.2 Atom or OPDS 2.0 JSON. Result is
  `{ status: "reachable" | "unreachable", checkedAt: timestamp, detail:
  string | null }` — `detail` is a short, closed-vocabulary category
  (`"timeout"`, `"connection-refused"`, `"http-4xx"`, `"http-5xx"`,
  `"http-3xx-unsupported"`, `"unparseable-response"`, `"path-not-found"`,
  `"path-not-readable"`, `"auth-rejected"`), never a raw error message
  or response body, which could otherwise leak the credential in an
  echoed `Authorization` header or a verbose server error page. An
  `http-4xx` on an authenticated source specifically (likely a bad
  credential) is reported as its own detail value, `"auth-rejected"`,
  distinguishing "wrong password" from "any other 4xx" without needing
  the raw status text; the same `"auth-rejected"` value is also used
  for a credential-decryption failure (FR-13), an intentional,
  documented ambiguity. `"http-3xx-unsupported"` is FR-11's own
  addition: any redirect response, since this adapter never follows
  one.
- **FR-7** `GET /api/v1/sources/:id/browse?cursor=<opaque>&limit=<n>`
  (`limit` integer `[1, 50]`, default `20`, `InvalidInput` outside
  range — the exact numbers match `backend-metadata-adapter.md` FR-1's
  own precedent; `backend-library-api.md` FR-1 established the general
  "validate and reject out-of-range" discipline this FR also follows,
  but with different numbers of its own (default `50`, max `100`), not
  a matching bound) MUST
  return `{ items: SourceCandidate[], nextCursor: string | null }`.
  For `local-folder`: `items` are the directory's files (not recursing
  into subdirectories in this phase — flagged in Open questions),
  sorted by filename, `cursor` an opaque token encoding the last-seen
  filename (never a raw path — decoded only to resume a sort position,
  never concatenated into a filesystem path). For `opds`: `items` are
  the feed's entries/publications normalised into `SourceCandidate`
  (FR-9), `cursor` an opaque token wrapping the feed's own next-page
  link — validated per FR-11's SSRF requirement before ever being
  fetched, never trusted as-is from the caller. `CanList` MUST be
  `true` for the browse call to succeed; it always is, per FR-5, so
  this is stated for completeness, not because a real `false` case
  currently exists.
- **FR-8** `GET /api/v1/sources/:id/search?q=<string>&cursor=<opaque>&limit=<n>`
  (`q` bounded at 200 characters, matching `backend-library-api.md`
  FR-2's and `backend-metadata-adapter.md` FR-1's own precedent for
  this project's search fields) returns the same `SourceCandidate[]`
  shape as FR-7, via the source's advertised OpenSearch (1.2) or
  templated search link (2.0). MUST return `409 Conflict` (this
  project's existing category for "the request is well-formed but the
  current state doesn't allow it," `backend-errors-and-logging.md`
  FR-1) if the source's detected `CanSearch` is `false` — never a
  silent empty result, which would be indistinguishable from "searched,
  found nothing."
- **FR-9** `SourceCandidate` — the shared normalised shape for both
  FR-7 and FR-8 — MUST contain only: `title` (string, validated per
  FR-1's discipline), `author` (nullable string, same validation),
  `fileReference` (`domain-source.md` FR-4's constrained type: opaque
  `Source`-scoped identifier, declared format, size in bytes if known),
  and `coverUrl` (nullable — for `opds` sources whose entry/publication
  advertises a cover-image link; always absent for `local-folder`,
  which has no cover concept). No raw Atom XML element or OPDS 2.0 JSON
  field name reaches this shape or anything beyond this adapter — the
  same normalise-at-the-boundary discipline `backend-metadata-adapter.md`
  already established, applied here to two upstream formats instead of
  one. Duplicate entries within a single upstream page (the same OPDS
  entry `id`, or Atom `<id>`, appearing twice in one feed page) MUST be
  de-duplicated, later occurrences dropped — the same rule
  `backend-metadata-adapter.md` established for duplicate Open Library
  search results (review `0034` finding #17), applied here to the
  structurally identical case for both OPDS versions.
- **FR-10** `SourceCandidate` MUST NOT be, or be convertible into, a
  `domain-source.md` `SourceOffering` — no `Edition` exists to attach
  it to yet (that match is phase 10's job). This adapter has no code
  path that constructs a `SourceOffering`; `FileReference` (FR-9) is
  the only `domain-source.md` type this spec's browse/search output
  actually uses, matching FR-4's own design (a `FileReference` needs no
  `Edition` to exist).
- **FR-11** Every URL this adapter fetches that originated from a
  source's own response content — a browse cursor's wrapped
  continuation link (FR-7), and a search-link URL discovered during
  capability detection (FR-5, consumed by FR-8) — MUST be validated,
  before ever being fetched, to resolve to the same origin (scheme +
  host + port) as the `Source`'s configured `config.baseUrl`. A value
  resolving to a different origin is rejected with `InvalidInput` (a
  cursor) or treated as "no search capability" (a search link found at
  capability-detection time — FR-5's `CanSearch` stays `false` rather
  than recording an unusable, untrusted link), never fetched either
  way. The search-link URL is captured and origin-validated **once**,
  during FR-6's health check (the same pass that runs FR-5's capability
  detection), and persisted on the `Source` row for FR-8 to reuse
  as-is — not re-fetched from the root feed on every search call, so
  the validation point and the usage point are the same URL, not two
  independently-trusted reads of it.

  Separately, and covering **every** outbound call this adapter makes —
  health checks, browse, search, and continuation-link fetches alike —
  the HTTP client MUST have redirect-following **disabled entirely**
  (Go's `http.Client.CheckRedirect` returning `http.ErrUseLastResponse`),
  and a `3xx` response MUST be treated as that call's own failure
  (a health check reports `detail: "http-3xx-unsupported"`, added to
  FR-6's closed vocabulary; a browse/search call returns `Unavailable`),
  never followed automatically. This closes the gap an origin check on
  the *initial* URL alone would leave open: a same-origin URL that
  itself 3xx-redirects to an off-origin target would otherwise bypass
  the check above the moment Go's default client followed it
  transparently. Disabling redirects entirely, rather than validating
  each hop, is this spec's own deliberate choice — simpler than a
  redirect-chain validator, and a self-hosted OPDS catalog has no real
  need to redirect its own clients.

  This is the concrete closure of the SSRF risk named in the phase 08
  roadmap's own risk table — this project's first genuine SSRF closure
  against a backend-side outbound fetch. Review `0034`'s
  `backend-metadata-adapter.md` cover-URL finding is a related but
  distinct precedent, worth naming accurately rather than overstating:
  that bug was an architecture-boundary violation (a client-facing
  field handed the browser a direct third-party URL, bypassing this
  system's own proxy/cache/rate-limit path entirely) — no untrusted
  value was ever fetched by the *backend* itself in that case. This
  FR's risk is the reverse and, in a self-hosted-adapter sense, more
  serious: a value from a hostile source steering this system's own
  outbound request. The two are related in spirit ("don't trust an
  externally-supplied continuation value") but are not the same
  mechanism, and this FR should not be read as merely re-applying an
  already-closed fix.
- **FR-12** A `local-folder` filename returned by the OS directory
  listing MUST have its **real, symlink-resolved path** computed
  (`filepath.EvalSymlinks` on the joined-and-cleaned candidate path),
  then MUST be verified — a prefix check of that *resolved* path
  against `basePath`'s own resolved real path (computed once, when the
  source is created/updated, not per file) — to still be inside
  `basePath` before any file operation is performed against it.
  `filepath.Clean` alone is explicitly **not** sufficient and MUST NOT
  be treated as the traversal check: `Clean` only normalises the
  lexical string (collapsing `.`/`..`/separators) and never touches the
  filesystem, so a symlink whose *name* lives inside `basePath` but
  whose *target* resolves outside it — the concrete attack this FR
  exists to close — passes a `Clean`-only check every time. This closes
  the path-traversal risk `domain-source.md`'s own Security
  considerations already names generically, enforced concretely here
  for the first time. A filename whose resolved path is outside
  `basePath`, or whose symlink target doesn't resolve at all (broken
  link), is skipped from FR-7's listing entirely, not surfaced as an
  error — the browse response simply doesn't include it, the same
  "degrade the one item, not the request" posture
  `backend-metadata-adapter.md` FR-4/FR-5 already established. A
  residual TOCTOU window exists between this check and the later file
  operation (the symlink target could change in between) — accepted for
  this phase given a `local-folder` source is, by definition, local
  filesystem state the same user controls, named explicitly here rather
  than silently assumed away; closing it fully (e.g. `O_NOFOLLOW`-based
  open semantics) is deferred to whichever future phase actually reads
  file bytes through this path (phase 10's import, which owns FR-4's
  "resolving a `FileReference` into actual bytes" job).
- **FR-13** A stored `credential` MUST be encrypted at rest with
  AES-256-GCM, using a symmetric key generated on first startup (via
  `crypto/rand`) and stored as a file, `source-credentials.key`, under
  `backend-configuration.md` FR-5's existing app-data directory family,
  with `0600` permissions (owner read/write only) set at creation time.
  The credential type MUST implement both `slog.LogValuer` and
  `json.Marshaler` (`backend-errors-and-logging.md` FR-8's own
  mechanism, which explicitly anticipates "a future credential" as a
  covered case — this is that case, not a new redaction design), each
  returning a fixed placeholder, never the real username or password.
  A missing key file at startup is **not automatically treated as
  first run**: startup MUST first check whether any `Source` row has a
  non-null encrypted `credential`. If none do, this genuinely is first
  run (or every credentialed source was already removed) and a new key
  is generated silently. If at least one credentialed source exists,
  a missing key file means the key was lost — not a first run — and
  startup MUST log a `warn` line stating how many sources are affected
  (never which ones by label, to avoid over-identifying a specific
  source in a log line for no operational benefit) before generating a
  replacement key; every one of those sources' existing ciphertext is
  now permanently undecryptable with the new key. A key file that
  exists and fails to decrypt a stored credential (corrupted file, or
  this replacement-key scenario) MUST surface as that one source's
  health check failing with `detail: "auth-rejected"` (indistinguishable
  from a genuinely wrong password from the caller's point of view,
  which remains an accepted, intentional ambiguity **at the per-source
  level** — this system does not reveal "your key file is gone" as a
  distinct per-source signal, since the remediation the user needs,
  re-entering the credential, is the same either way; the startup
  `warn` line above is the *systemic* signal, aimed at whoever reads
  server logs, not at the per-source UI).
  FR-7/FR-8's browse/search calls, which also need a successful
  decrypt to build the outbound `Authorization` header, MUST return
  `503 Unavailable` immediately if decryption fails — the same outcome
  a caller sees when the source is simply unreachable, since the
  underlying cause (this call cannot proceed) is functionally
  identical; no separate error category is introduced for this case,
  and the source's own next health check is what surfaces the
  distinguishing `auth-rejected` detail.
- **FR-14** Outbound requests to sources (health checks, browse, search,
  continuation-link fetches combined) MUST be bounded by a **global cap
  of 50 concurrent in-flight requests**, tracked by a simple counting
  semaphore — a request arriving when the cap is already reached MUST
  return `503 Unavailable` immediately, never queued. Unlike
  `backend-metadata-adapter.md` FR-7's shared rate limiter (a single
  external service, Open Library, with one stated rate policy this
  system must not exceed), sources are independent hosts with no shared
  external policy to respect — so this FR's purpose is purely this
  system's own resource protection (bounded goroutines/connections),
  not courtesy to an upstream rate limit, which is why a wait-then-retry
  design (FR-7's 5-second budget) isn't adopted here: there's no
  external ceiling to wait out, only this system's own concurrency
  budget, so rejecting immediately is simpler and equally correct. This
  is a single global counter across every configured source, not
  per-source — a user with many configured sources triggering many
  simultaneous health checks (FR-1/FR-2's automatic checks, for
  instance, if several sources are edited in quick succession) is
  exactly the case this cap protects against.

## Non-functional requirements

- **Performance** — a health check (FR-6) MUST complete within its own
  5-second timeout; a browse/search page (FR-7/FR-8) is bounded by the
  same timeout per upstream call, with no retry (a slow source is a
  `503`/`Unavailable`, `Unavailable`'s existing meaning reused from
  `backend-errors-and-logging.md` FR-1, not a hang).
- **Security** — see dedicated section below.
- **Accessibility** — not applicable to a backend adapter;
  `frontend-source-management.md`'s concern.
- **Reliability** — a single unreachable source MUST NOT affect any
  other source's availability or any other endpoint; each `Source`'s
  health/capability state is independent, stored per-row, never a
  shared or global flag.
- **Observability** — a health-check transition (reachable →
  unreachable or the reverse) MUST log at `info` with the source `id`
  and the `detail` category (FR-6) — never the credential, never the
  raw upstream response. An SSRF-rejected cursor (FR-11) or a
  traversal-rejected filename (FR-12) MUST log at `warn`, since both
  represent a source behaving outside its expected shape, worth a
  maintainer's attention distinct from an ordinary reachability
  failure.

**Dependency justification (constitution §9)**: no new dependency for
encryption (Go stdlib `crypto/aes`/`crypto/cipher`/`crypto/rand` — the
same "well-understood, hand-rolling it is the risk, stdlib isn't a
third-party dependency in the usual sense" reasoning `backend-metadata-adapter.md`
already used for `golang.org/x/time/rate`, applied here to something
even more core to Go's own standard library). An OPDS 1.2 parser needs
an XML library — Go's stdlib `encoding/xml` is sufficient for Atom's
element set (no XML namespace complexity beyond what `encoding/xml`
already handles); no third-party XML dependency is introduced. OPDS
2.0 parsing uses stdlib `encoding/json`, matching `backend-metadata-adapter.md`'s
own approach to Open Library's JSON responses.

**Local key file over OS-keychain integration**: the roadmap's own
Architecture decisions expected section assigns this spec the job of
concretely justifying this choice, not assuming it. Electron's
`safeStorage` API (backed by macOS Keychain, Windows DPAPI, or Linux
`libsecret`/`kwallet`) is Node-only — reachable from the main process,
not from the Go backend, which is a separate OS process. Using it would
require a new enumerated IPC round trip (`desktop-host-ipc-surface.md`'s
mechanism) for every credential encrypt/decrypt operation, coupling
this system's credential storage to Electron's own process being alive
and reachable — true today (Electron always spawns the Go server,
`desktop-host-process-model.md`), but an unnecessary coupling for a
capability the Go backend can provide itself with three stdlib
packages. A locally-generated key file keeps credential storage
entirely within the Go backend's own persistence layer, consistent
with how every other piece of this system's state (the database
itself, `DATABASE_URL`) is already owned there, and avoids a
platform-specific dependency (Linux's `libsecret` in particular is not
guaranteed present on every desktop environment this project might
eventually target) for a self-hosted, single-user system where the
key file's own filesystem permissions (FR-13) are a proportionate
control.

## Domain model

No new `domain-source.md` types. This spec's `SourceCandidate` (FR-9)
is a new, adapter-owned DTO — not a domain type, not convertible to a
`SourceOffering` (FR-10) — following the same DTO-not-aggregate pattern
`backend-metadata-adapter.md`'s Domain model section already
established for `NormalisedWork`/etc.: a browsed item is a candidate
that phase 10's import flow may, or may not, ever turn into a real
`domain-source.md` `SourceOffering` and a `domain-library.md`
`LibraryEntry`. `FileReference` (`domain-source.md` FR-4) is the one
domain type this spec constructs directly, since FR-4 itself requires
no `Edition` to exist first. A `Source`'s persisted row (`kind`,
`config`, encrypted `credential`, health/capability state) is this
spec's own storage concern, not `domain-source.md`'s FR-1 identity
model extended with new fields — `domain-source.md` FR-1 fixes what a
`Source` *is* to the domain (an internal ID, a label, declared
capabilities); this spec's persistence layer additionally stores what
the domain doesn't need to know (a filesystem path, a URL, an encrypted
credential, a health-check timestamp), the same domain/persistence
separation `architecture-backend.md` FR-2 already requires generally.

## API and contracts

All endpoints follow `architecture-contracts.md`'s existing conventions
(base path `/api/v1`, error envelope `{ code, message, correlationId }`,
versioning).

- `POST /api/v1/sources` → `201 { id, label, kind, config, hasCredential, health, capabilities }` → `400` (`InvalidInput`)
- `GET /api/v1/sources` → `200 { sources: [...] }`
- `GET /api/v1/sources/:id` → `200 { ...same shape... }` → `404` (`NotFound`)
- `PATCH /api/v1/sources/:id` → `200 { ...same shape... }` → `400`, `404`
- `DELETE /api/v1/sources/:id` → `204` → `404`
- `POST /api/v1/sources/:id/health-check` → `200 { status, checkedAt, detail }` → `404`
- `GET /api/v1/sources/:id/browse` → `200 { items: SourceCandidate[], nextCursor }` → `400`, `404`, `503` (`Unavailable`)
- `GET /api/v1/sources/:id/search` → `200 { items: SourceCandidate[], nextCursor }` → `400`, `404`, `409` (`Conflict`, FR-8), `503`

## State transitions

A `Source` moves `created (health unknown until the create-time check
runs) → reachable | unreachable`, and `reachable ⇄ unreachable` freely
on any subsequent health check — no state is sticky or requires manual
reset. `capabilities` moves independently, re-derived on every health
check (FR-5), not tied to the reachable/unreachable transition itself
(a source can go from reachable-without-search to reachable-with-search
if the operator adds it, without ever passing through unreachable).

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| OPDS source unreachable (timeout, connection refused) | FR-6's 5-second timeout / connection error | Health status shows "unreachable," `detail: "timeout"` or `"connection-refused"` | Logs at `info` (Observability), no retry |
| OPDS source returns a `4xx` on an authenticated request | HTTP status check | Health status "unreachable," `detail: "auth-rejected"` | Distinguishes bad-credential from other `4xx` explicitly |
| OPDS source returns malformed/unparseable XML or JSON | Parse failure | Health status "unreachable," `detail: "unparseable-response"` | Logs at `info`, raw body never logged |
| Local-folder path doesn't exist or isn't readable | `os.Stat`/permission check | Health status "unreachable," `detail: "path-not-found"` or `"path-not-readable"` | No filesystem operation attempted beyond the check itself |
| Browse cursor, or a discovered search-link, resolves off-origin (SSRF attempt, FR-11) | Origin comparison before fetch | `400 InvalidInput` (cursor) or `CanSearch: false` (search link) | Rejected before any upstream fetch; logs at `warn`, host only, never the full URL/query string |
| Source responds with a `3xx` redirect (FR-11) | Redirect-following disabled, `CheckRedirect` short-circuits | Health status "unreachable," `detail: "http-3xx-unsupported"`; a browse/search call returns `Unavailable` | No redirect followed |
| Filename resolves outside configured `basePath` after symlink resolution (traversal attempt, FR-12) | `filepath.EvalSymlinks` + prefix check against the resolved real `basePath` | Item silently omitted from the browse result | Logs at `warn`, no filesystem operation against the offending path |
| `search` called on a source with `CanSearch: false` | FR-5's stored capability check | `409 Conflict`, distinct from an empty result | No upstream call made |
| Credential decryption fails (corrupted/regenerated key) | AES-GCM authentication tag mismatch | Health status "unreachable," `detail: "auth-rejected"`; a browse/search call instead returns `Unavailable` immediately (FR-13) | Logs at `info`; the ambiguity with a genuinely wrong password is intentional |
| Credential key file missing with existing credentialed sources (FR-13) | Startup check against `Source` rows | Affected sources show `auth-rejected` on next health check | Startup logs `warn` with the affected count; new key generated |
| Duplicate entry within one upstream page (FR-9) | Repeated `id` within a single response | Item appears once in the result, not twice | Later duplicate dropped |
| Global outbound concurrency cap reached (FR-14) | Semaphore full | `503 Unavailable` for the newly-arriving request | Rejected immediately, never queued |

## Security considerations

- **A source is untrusted by construction** (constitution §4's own
  example) — every response and filename from it is validated,
  size-capped, and timeout-bounded before this system trusts anything
  it says.
- **SSRF via continuation links and search links (FR-11)** — the
  concrete mitigation for this phase's sharpest named risk, closed
  against both places an upstream-supplied URL enters this adapter (a
  browse cursor, a discovered search link), and against a same-origin
  URL being redirected off-origin after the fact (redirect-following
  disabled entirely, every `3xx` treated as failure) — an origin check
  on the initial URL alone would have left that second path open.
- **Path traversal via filenames, including symlinks (FR-12)** —
  `domain-source.md`'s generic named risk, closed concretely against
  the specific attack named as the motivating example: `filepath.Clean`
  alone does not resolve symlinks and cannot detect a symlink whose
  target escapes `basePath`; this spec resolves the real path
  (`filepath.EvalSymlinks`) before the prefix check, with a named,
  accepted TOCTOU residual (Functional requirements, FR-12).
- **Credential encryption at rest (FR-13)** — AES-256-GCM with a
  locally-generated key, `0600`-permissioned key file, dual-interface
  redaction from logs/JSON per `backend-errors-and-logging.md` FR-8.
  The key file itself is a single point of failure for every stored
  credential — an accepted tradeoff for a self-hosted, single-machine,
  single-user system with no key-management infrastructure to lean on;
  named explicitly in Open questions, not silently assumed acceptable.
- **No credential in health-check detail (FR-6)** — `detail` is a
  closed-vocabulary category, never a raw response body or error
  string, which could otherwise leak an echoed `Authorization` header
  or a verbose server error page containing configuration details.
- **No new trust boundary on the loopback side** — still loopback-only,
  still no authentication between the LAN client and this host; the
  authentication this spec adds is outbound only (this system to a
  source), matching the phase 08 roadmap's own framing.
- **Resource exhaustion (FR-14)** — a global 50-concurrent-request cap
  across every configured source, rejecting immediately rather than
  queuing, bounds how many goroutines/connections a user (or a
  compromised renderer) can force this adapter to hold open at once.
- **Basic Auth over plain HTTP (FR-4)** — named explicitly as an
  accepted risk, not silently assumed away: a credential attached to an
  `http://` source is sent in near-cleartext on every request, within
  this project's existing self-hosted/household-scale threat model
  (constitution §6's own reasoning, applied here to outbound rather
  than inbound traffic); `frontend-source-management.md` FR-5 surfaces
  this to the user at the point they attach a credential to such a
  source.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | OPDS 1.2 Atom parsing and OPDS 2.0 JSON parsing against real, recorded fixtures of each format, including a fixture with a duplicated entry `id` (FR-9); credential encryption/decryption round-trip, including a corrupted-ciphertext case and the key-loss-with-existing-credentials startup path (FR-13); path-traversal rejection (FR-12) with a symlink-outside-`basePath` fixture, resolved via `EvalSymlinks`, not a lexical-only check; SSRF rejection (FR-11) with both an off-origin continuation-link fixture and an off-origin search-link fixture; a redirect-response fixture asserting no redirect is followed (FR-11); capability detection per source `kind` and per feed shape; the 50-concurrent-request cap (FR-14) rejecting a 51st in-flight request |
| Integration | Source CRUD against a real PostgreSQL instance (`backend-test-harness.md`'s harness); health checks against a fake local temp directory and a fake OPDS HTTP server (never a real, live OPDS catalog in CI); a full create → health-check → browse round trip per `kind` |
| Contract | `architecture-contracts.md` FR-3's `kin-openapi` tool (`backend-library-api.md` FR-8's tool, reused, not re-decided), extended to `/api/v1/sources*` |
| E2E | Add a local-folder source → see it reachable → browse its contents; add an OPDS source with Basic Auth → see it reachable → search it — both against fakes; a credential decryption-failure scenario (corrupt the key file, confirm the source reports `auth-rejected` rather than crashing, and that a concurrent browse/search call for that source returns `Unavailable`) |
| Accessibility | Not applicable at this layer — `frontend-source-management.md`'s concern |

Tests that must fail before implementation begins: a test asserting a
browse cursor or a discovered search link pointing at a different host
than the source's `baseUrl` is rejected before any HTTP call is made; a
test asserting a redirect response is never followed; a test asserting
a filename whose *resolved* path lies outside `basePath` (a symlink,
specifically — not just a lexical `../`) is silently omitted, not
surfaced as an error; a test asserting a captured log/JSON dump of a
`Source` with a credential never contains the raw username or
password; a test asserting a 51st concurrent outbound request is
rejected with `503`, not queued.

## Acceptance criteria

- [ ] `Source` CRUD works against a real PostgreSQL instance
- [ ] A local-folder source's health check reflects real path
      existence/readability
- [ ] An OPDS source's health check reflects real reachability, and
      distinguishes an auth failure from other failure categories
- [ ] Browsing a local-folder source lists its files; browsing an OPDS
      source lists its feed entries, both paginated
- [ ] Searching an OPDS source with `CanSearch: true` returns results;
      searching one with `CanSearch: false` returns `409`, not an empty
      list
- [ ] No credential appears in any log line or JSON dump, proven by a
      test covering both the single-field and whole-struct logging
      paths (`backend-errors-and-logging.md` FR-8's own dual-path
      concern)
- [ ] An off-origin browse cursor or search link is rejected before any
      upstream fetch, and no redirect is ever followed
- [ ] A filename whose resolved (symlink-followed) path lies outside
      the configured `basePath` never reaches a filesystem operation
- [ ] A missing credential key file with existing credentialed sources
      logs a distinct startup warning, not silent regeneration
- [ ] A 51st concurrent outbound source request is rejected with `503`
- [ ] The contract test passes against `/api/v1/sources*`

## Open questions

- **No subdirectory recursion for local-folder browsing (FR-7)** — a
  reasoned scope cut for this phase, not a considered limit; revisit if
  real usage shows a flat single-directory model is too restrictive.
  Recursion would also reopen the traversal-safety analysis (FR-12) for
  a deeper tree, worth treating as its own design pass rather than
  folding in casually.
- **Single-key-file credential encryption (FR-13)** — no key rotation,
  no per-credential key, a single point of failure by design; open to
  revision once this project has a real backup/export story (not yet
  scoped in any phase) that would need to account for this key
  specifically.
- **Capability re-detection cadence** — FR-5 re-detects on every health
  check, which is user- or create/update-triggered only in this phase;
  once phase 09 adds scheduled re-checking, whether capability
  detection should run on that same cadence or a slower one is that
  phase's own question, not resolved here.

## References

- `domain-source.md` (phase 02) — `Source`/`SourceOffering`/
  `FileReference` model this spec implements against
- `backend-persistence.md` (phase 03) — repository pattern, transactions
- `backend-configuration.md` (phase 03) FR-5, FR-8 — app-data directory
  family; loopback-only binding, unrelated but same spec
- `backend-errors-and-logging.md` (phase 03) FR-1, FR-8 — error
  taxonomy; dual-interface redaction, reused directly for credentials
- `backend-http-transport.md` (phase 03) — timeout figure reused for
  consistency, not literal precedent
- `backend-metadata-adapter.md` (phase 07) — external-boundary
  discipline precedent (size/shape/timeout, DTO-not-aggregate,
  per-field degradation, shared rate limiting, duplicate-entry
  de-duplication); review `0034`'s cover-URL finding there is a related
  but distinct precedent for FR-11 ("don't trust an externally-supplied
  continuation value"), not the same SSRF mechanism — this spec's FR-11
  is this project's first actual backend-side SSRF closure
- `backend-library-api.md` (phase 06) FR-1, FR-2, FR-8 — cursor-
  pagination and search-bound precedent; `kin-openapi` contract tool
- `desktop-host-ipc-surface.md` (phase 05) — amended (FR-6) to add
  `source.pickLocalFolder`, this spec's `config.basePath` input source
  when running inside Electron
- Open Library API documentation is not relevant here; OPDS
  documentation: `specs.opds.io/opds-1.2.html`, `specs.opds.io/opds-2.0.html`,
  `drafts.opds.io/authentication-for-opds-1.0.html`
- Constitution §3 (source/domain separation), §4 (hostile input,
  applied to this phase's SSRF/traversal risks specifically), §8
  (credential redaction), §9 (dependency justification)
