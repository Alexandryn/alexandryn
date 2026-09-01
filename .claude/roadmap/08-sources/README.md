# Phase 08 — Sources

| | |
|---|---|
| **Status** | `IMPLEMENTED` (Tiers 0–6 complete, security audit clear, ready for maintainer verification) |
| **Depends on** | Phase 06 |
| **Blocks** | 09, 10 |
| **Opened** | — |
| **Closed** | — |

## Objective

`domain-source.md`'s capability model (phase 02) implemented against two
real providers — a local folder and an OPDS catalog (1.2 and 2.0) — with
source configuration, credential storage, and health checks, and every
byte a source returns treated as hostile input (constitution §4). At the
end of this phase, a user can add a source, see whether it's reachable,
and browse or search what it advertises. Nothing this phase produces is
matched against the user's library or imported — that composition is
phase 10's job, deliberately kept out so a defect here can be isolated to
"the source adapter" rather than tangled with "does import work."

## Why here

It needs `domain-source.md`'s model (phase 02) and a real backend to
attach to (phase 03), so it waits for both — same reasoning phase 06 and
07 already gave. It sits after phase 07 (metadata) for a structural
reason, not a data dependency: phase 07 was this project's first
external, third-party network boundary (Open Library), and this phase is
its second, structurally different one — a source is user-configured and
often authenticated, unlike Open Library's fixed, credential-free
endpoint. Landing phase 07 first means the "every external response is
hostile" discipline (size/shape/timeout, normalise-at-the-boundary) is
already proven once before this phase adds a second, harder version of
the same problem: one where the endpoint itself, not just its response
shape, is untrusted (a user can be tricked into adding a malicious
source — constitution §4's own named example). It sits before phase 09
(async jobs, for background re-checking availability) and phase 10
(import, for turning a browsed listing into a real library entry)
deliberately, so both of those phases have a working, tested adapter
layer to build on rather than co-designing it under them.

## Scope

**In**

- `Source` CRUD (`POST`/`GET`/`PATCH`/`DELETE /api/v1/sources`) —
  label, `Kind` (`local-folder` | `opds`), provider-specific config
  (a filesystem path; a base URL), and an optional credential
- Encrypted-at-rest credential storage for a source that requires
  authentication (HTTP Basic Auth only — scoped deliberately, see
  Architecture decisions expected)
- Health checks: a connectivity/reachability probe run at source
  create/update time and on demand, with the result and its observation
  timestamp visible to the user — a source-level "is this endpoint
  reachable" fact, distinct from `domain-source.md`'s per-`Edition`
  `SourceOffering` availability model
- Local folder provider: list a configured directory's files
- OPDS provider: fetch and normalise OPDS 1.2 (Atom) and OPDS 2.0 (JSON)
  navigation/acquisition feeds; search, when the source's root feed
  advertises it
- Capability detection (`CanList`/`CanSearch`/`CanDownload`,
  `domain-source.md` FR-1's fixed set) at health-check time, not
  per-request
- Source management screen: add/edit/remove a source, see its health
  status, browse/search what it offers
- Hostile-input handling for every catalog response and every filename a
  source returns (constitution §4), applied for the first time against
  an endpoint that is itself user-configured, not fixed

**Out**

- Async/scheduled re-checking of source health or availability — phase
  09; this phase's health check is synchronous and user- or
  create/update-triggered only
- The import pipeline (discovery → extraction → matching →
  confirmation) — phase 10; this phase's browse/search results are
  unmatched candidates, never a `domain-library.md` `LibraryEntry` or a
  `domain-source.md` `SourceOffering` (see Architecture decisions
  expected — no `Edition` exists yet for a browsed-but-unmatched item to
  attach to)
- A public HTTP endpoint for downloading raw file bytes — this phase's
  adapter can resolve a `FileReference` into a byte stream internally
  (the Go-level capability phase 10 will call), but no HTTP surface
  exposes that yet; a download endpoint with nothing downstream to hand
  the bytes to would be a half-finished feature, not this phase's to
  build
- OAuth 2.0 / the "Authentication for OPDS" document flow — Basic Auth
  only, this phase's own scoped choice
- Non-OPDS, non-local-folder source protocols (Calibre-Web,
  Standard Ebooks-specific integrations, etc.) — future work, not named
  in any current phase

## Specifications

| Spec | Covers |
|---|---|
| `backend-source-adapter.md` | Source CRUD, credential encryption, health checks, local-folder and OPDS providers, `/api/v1/sources*` endpoints |
| `frontend-source-management.md` | Source management screen: add/edit/remove, health status, browse/search |

## Architecture decisions expected

- **Credential encryption at rest**: `architecture-persistence.md`'s own
  Open questions and `backend-configuration.md`'s own Open questions
  both flag this exact gap ("encryption at rest: not decided, not
  assumed" / "flagged for whoever picks up credential storage") — this
  phase is where it gets decided, not deferred again. Leaning toward a
  locally-generated symmetric key file under the app-data directory
  (constitution §9: no new dependency, stdlib `crypto/aes`+`crypto/cipher`),
  not an OS-keychain integration, to avoid coupling the Go backend's
  credential storage to Electron's `safeStorage` API (Node-only, would
  require a new enumerated IPC round trip per credential operation) —
  `backend-source-adapter.md`'s own to justify concretely, not assumed
  here.
- **Auth scope: Basic Auth only, not OAuth 2.0/"Authentication for
  OPDS"** — grounded in real OPDS usage research: Basic Auth "continues
  to be used by a large number of organizations and remains useful for
  simple use cases, such as self-hosted catalogs" (the OPDS community's
  own drafts), which is exactly this project's use case; a full OAuth
  2.1 authorization-code flow is disproportionate to a single-household
  self-hosted library and is explicitly out of scope above.
- **Browse/search results are not `SourceOffering` rows** — the single
  most important architectural call this phase makes, and worth stating
  here since it's non-obvious: `domain-source.md` FR-2 defines
  `SourceOffering` as "this `Source` claims to offer this `Edition`" —
  but nothing has matched a browsed item to a domain `Edition` yet (that
  match is phase 10's whole job). This phase's browse/search results are
  therefore a new, non-domain DTO (a "candidate" — title/author guess,
  `FileReference`, format), never a domain aggregate, mirroring the
  DTO-not-aggregate design phase 07's adapter already established for
  Open Library search results.
- **Pagination**: OPDS itself exposes a real next-page link (RFC 5005
  for 1.2, `links`/`metadata` for 2.0) — unlike phase 07's Open Library
  Search API, which had no cursor to forward. `backend-source-adapter.md`
  to decide the concrete cursor encoding, but leaning toward
  `backend-library-api.md`'s cursor-pagination precedent (an opaque,
  server-validated token) over phase 07's offset/limit exception, since
  the reason for that exception (no upstream cursor to wrap) doesn't
  hold here.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| A malicious or compromised OPDS source returns a crafted next-page or search link pointing off-origin, or 3xx-redirects a same-origin request elsewhere (SSRF) | Medium | High | Related in spirit to this project's own phase 07 review (`0034`, a proxy-bypass bug, not itself SSRF) but this project's first actual backend-side SSRF vector; `backend-source-adapter.md` validates every server-supplied URL (continuation cursor, search link) against the source's configured origin before fetching, and disables redirect-following entirely, checked explicitly in Security considerations, not assumed |
| A local-folder source's configured path, or a filename it returns, is used to escape the configured directory (path traversal) | Medium | High | `domain-source.md` FR-4 already keeps `FileReference` from being interpretable as a path anywhere in the domain; this phase's adapter is where that boundary is actually enforced with a real traversal-safe resolution (`filepath.Clean` + prefix check against the configured root, never raw concatenation) |
| Credential encryption key handling done carelessly (key logged, key stored unencrypted alongside the data it protects with no access restriction) | Low | High | Constitution §8's "never logged" list gets a concrete new member (source credentials); this phase's key file gets explicit filesystem-permission and no-log requirements, tested, not just asserted |
| OPDS 1.2 (Atom/XML) and OPDS 2.0 (JSON) turn out to need meaningfully different capability-detection or pagination logic that this phase's shared abstractions don't actually accommodate | Medium | Medium | `backend-source-adapter.md`'s own normalisation layer is scoped explicitly per-version from the start (two parsers, one shared output DTO), not retrofitted after building against only one version |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Capability detection per source `Kind`; credential encryption/decryption round-trip; path-traversal rejection for local-folder filenames; OPDS 1.2 Atom parsing and OPDS 2.0 JSON parsing against real, recorded fixtures from each format |
| Integration | Source CRUD against a real PostgreSQL instance (`backend-test-harness.md`'s harness); health checks against a fake local directory and a fake OPDS HTTP server (never a real, live OPDS catalog in CI) |
| Contract | `architecture-contracts.md` FR-3's contract test, extended to `/api/v1/sources*` |
| E2E | Add a local-folder source → see it healthy → browse its contents; add an OPDS source with Basic Auth → see it healthy → search it — both against fakes, not live services |
| Accessibility | `frontend-accessibility.md`'s requirements applied to the source management screen, including its credential-entry form |

## Security considerations

The project's second external, third-party network boundary, and its
first user-configured one:

- **A source is untrusted by construction, not by reputation** —
  constitution §4's own explicit example ("a user can be tricked into
  adding a malicious source") is the threat model for this entire phase,
  applied to every catalog response and filename
- **Credentials never appear in logs** — constitution §8, extended from
  phase 03's `DATABASE_URL` redaction precedent (dual `slog.LogValuer` +
  `json.Marshaler` typed redaction, `backend-configuration.md` FR-7) to
  a new credential type this phase introduces
- **SSRF via source-supplied continuation links** — named above as this
  phase's sharpest concrete risk, given phase 07's review just found and
  fixed the closely analogous cover-URL bug
- **Path traversal via source-supplied filenames** — `domain-source.md`'s
  own named risk (FR-4's Security considerations), enforced concretely
  here for the first time
- **No new trust boundary on the loopback side** — still loopback-only,
  still no authentication between the LAN client and this host, same
  accepted model every phase through 11 shares; the authentication this
  phase adds is *outbound*, this system authenticating itself to a
  source, not inbound

## Observability

Health-check results (reachable/unreachable, with the failure category
when unreachable — timeout, auth failure, malformed response) are the
main new observable fact this phase introduces; per-request logging
(`backend-errors-and-logging.md`) extends to `/api/v1/sources*`
unchanged. Source credentials and full local-folder paths are excluded
from routine logging per constitution §8 — the same discipline phase 07
already applied to search queries, applied here to a stricter class of
data.

## Exit criteria

- [ ] Both specifications `APPROVED` with recorded reviews
- [ ] Source abstraction isolates the domain from provider protocol
      details — no OPDS/local-folder-specific type reaches
      `internal/domain`
- [ ] Local folder and OPDS (1.2 and 2.0) providers functional: add,
      health-check, browse, and (where supported) search
- [ ] Source credentials never appear in logs, per constitution §8,
      proven by a test, not just asserted
- [ ] A path-traversal attempt via a local-folder filename is rejected,
      proven by a test
- [ ] An off-origin OPDS continuation link is rejected, proven by a test
- [ ] Test coverage across unit, integration, and E2E for this slice
- [ ] All specs in this phase are `VERIFIED`
- [ ] Security audit recorded in `.claude/audits/` with no open Critical
      or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
