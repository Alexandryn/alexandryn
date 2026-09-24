# Spec: Backend reader content

| | |
|---|---|
| **Status** | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15). **Amended 2026-09-02 (`DRAFT`, pending re-confirmation) for phase-13 review `0050` AUDIT-0012-C1:** FR-1's ownership check MUST be scoped to the caller's validated active library and library membership — an edition's bytes are not streamable cross-library by `editionId` alone. Phase-13 close gate. |
| **Phase** | `11-reader` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | `0038` (two independent agents, cross-spec) — Needs rework at review time (7 Blocking across the batch, 3 confirmed independently by both passes; 9 Major, 6 Minor), all findings fixed; approved by maintainer 2026-08-15 |

## Context

`backend-file-extractors.md` (phase 10) parses an EPUB's metadata at
*import* time and stops there — this spec is the second, structurally
different file-content boundary: serving a specific spine/resource
entry from inside an already-owned `Edition`'s EPUB, on demand, at
*read* time, to a client that will render it. `frontend-reader.md`
(this phase, sibling) is that client. This is the first spec where
this project hands parsed file content *back* to a caller, rather than
only extracting a summary from it server-side — the sandboxing stakes
are correspondingly higher: the content this endpoint serves is
rendered as real HTML/CSS in a browser.

Grounded against real research, not assumed: `foliate-js`
(`frontend-reader.md`'s own chosen rendering engine, per the roadmap's
Architecture decisions expected) defaults to unzipping an EPUB
entirely client-side and serving its contents via `blob:` URLs — its
own maintainer's documentation states plainly that securing that
default approach "is currently impossible... due to the content being
served from the same origin." This spec exists specifically to avoid
that default: content is sanitised **server-side**, before the client
ever receives it, and served from this system's own API origin (not a
client-reconstructed `blob:` URL), which is also what lets this spec's
CSP respond (below) use a plain `'self'` origin policy rather than the
weaker `blob:`-inclusive one `foliate-js`'s own default would require.

## Problem

Nothing exists yet to fetch a specific entry from inside an owned
Edition's EPUB, verify it's safe to serve (reusing, not re-inventing,
phase 10's zip-bomb/zip-slip defences for this new, independent access
path), strip anything that could execute script or phone home to an
external host, and serve it with headers that let a sandboxed client
enforce the rest.

## Goals

- `GET` access to any legitimate spine/resource entry inside an owned
  `Edition`'s EPUB, resolved through the same `Provider.Resolve`
  capability phase 10 already established
- Server-side sanitisation of HTML/XHTML content: no `<script>`, no
  event-handler attributes, no external resource references
- Server-side sanitisation of CSS content: no external `url()`/
  `@import` references
- Binary resources (images, fonts) served as-is, still size-capped
- A response `Content-Security-Policy` a sandboxed client can rely on
  as a second, independent enforcement layer
- Reuse, not re-derive, `backend-file-extractors.md`'s zip-bomb/zip-
  slip/malformed-container defences for this new, read-time access path

## Non-goals

- Rendering, pagination, or CFI generation — `frontend-reader.md`'s
  job entirely; this spec only ever serves bytes
- PDF/CBZ content serving — this phase's own scope cut (roadmap
  Non-goals), matching `frontend-reader.md`'s EPUB-only rendering scope
- Reading position/bookmark/highlight persistence —
  `backend-reading-api.md`'s job
- A general-purpose "download this file" endpoint — every response
  this spec serves is a sanitised, specific resource inside a
  specific, owned Edition's EPUB, never a raw file download

## User stories

- As **`frontend-reader.md`'s rendering engine**, I want to fetch a
  spine item's HTML and its referenced images/fonts/CSS by path, so I
  can render a page without needing the whole EPUB client-side.
- As **the maintainer**, I want a maliciously crafted EPUB opened for
  *reading* to be defended exactly as rigorously as one being
  *imported*, even though the two are different code paths, so a gap
  in one doesn't silently reopen a risk the other already closed.
- As **someone reading a book**, I want nothing inside it to be able
  to phone home to a server I've never heard of, so what I'm reading
  stays private.

## Functional requirements

- **FR-1** `GET /api/v1/library/editions/:editionId/reader/content/*path`
  — `editionId` MUST correspond to an `Edition` with at least one
  `LibraryEntry` (`domain-library.md` FR-1's own ownership fact —
  reusing `backend-import-pipeline.md` FR-4(a)'s own join-through-
  `LibraryEntry` discipline, restated here for a read path instead of
  a match path; not FR-2, which computes a *Work*-level "in library"
  flag from "at least one Edition owned" — a different, coarser
  question than "is this exact Edition owned," the one this FR
  actually needs answered);
  an `Edition` that merely exists without being owned returns `404
  NotFound`, never served.
  - **Amended 2026-09-02 (`DRAFT`) for phase-13 review `0050`
    AUDIT-0012-C1:** "owned" MUST mean owned **in the caller's validated
    active library** — the `LibraryEntry` check MUST carry
    `AND library_id = <ActiveLibraryFromContext>`, and the caller MUST be
    an authenticated member of that library (the auth middleware's
    `X-Library-Id`-vs-`claims.Libraries` check,
    `backend-library-namespaces.md` FR-3). A `reader` in library X MUST
    NOT be able to stream the bytes of an edition owned only in library Y
    by knowing its `editionId`. Same not-found response for
    "doesn't exist" and "not in your library" — no cross-library
    existence oracle. This is part of the phase-13 close gate; the
    shipped handler does an ownership check with no library or membership
    scoping.
  - **Amended 2026-09-23 — how the iframe authenticates.** The reader's
    sandboxed `<iframe src>`, and every image and stylesheet its chapter
    loads by relative URL, is a plain browser fetch that cannot carry an
    `Authorization` header or `X-Library-Id`, so a Bearer-only endpoint
    refused every chapter. The caller MAY instead present the `alx_rc`
    reader-content grant cookie, issued by
    `POST /api/v1/library/editions/:editionId/reader/session`
    (Bearer-authenticated; the edition must be owned in the active
    library). The grant is an HS256 token on its own HKDF subkey
    (`reader-content-grant-v1`) carrying user, library, and edition,
    valid 15 minutes; the cookie is `HttpOnly`, `SameSite=Strict`,
    `Secure` when the request arrived over TLS (directly or via a trusted
    proxy), and its `Path` is this edition's `reader/content/` route. It
    is honoured only on `GET`/`HEAD` of that route, only for the edition
    it names, and only when no `Authorization` header is present; the
    library it names becomes the active library for the ownership check
    above, which still runs on every request. The grant never appears in
    a URL, a response body, or a log line. Accepted residual risks: a
    grant outlives logout by up to 15 minutes, and cookies are not
    port-isolated, so another service on the same host could replay one
    it is sent.
  `*path` is validated against `..`-shaped or
  absolute-path segments before ever being used to look up a zip entry
  (`InvalidInput` otherwise) — the same zip-slip discipline
  `backend-file-extractors.md` FR-6 established, restated here since
  this spec's own lookup key (`*path`, caller-supplied) is a
  structurally different input than that spec's own zip-slip case
  (an entry name the zip itself supplied) — here, the path comes from
  the *client*, so it's validated before it's even used to search the
  zip's entry table, not only after.
- **FR-2** The `Edition`'s content is resolved via the `Edition`'s own
  `SourceOffering`(s) (`domain-source.md` FR-5: many-to-many, an
  `Edition` may have more than one) — this endpoint tries each,
  ordered by most-recently-`observed_at` first, calling
  `backend-source-adapter.md`'s `Provider.Resolve` for each `Source`
  in turn until one succeeds; if every `SourceOffering`'s source is
  currently unreachable, the endpoint returns `503 Unavailable`. A
  successfully resolved raw byte stream is passed through
  `backend-file-extractors.md` FR-1's `Materialize` (the same 250 MiB
  raw-byte cap, spooled to a temp file, reused directly — not
  re-implemented) before any zip parsing happens. **`Materialize`'s
  temp file is removed when its own `context` is done — this FR
  requires that context to be one FR-3's cache owns and controls
  independently of any individual HTTP request's context, cancelled
  only at FR-3's own eviction, never at the end of the request that
  happened to trigger population.** Populating the cache from a
  request-scoped context (the naive default for a Go HTTP handler)
  would delete the temp file the instant that first request completes,
  silently breaking every subsequent request against the same cache
  entry within FR-3's own claimed lifetime — this sentence exists
  specifically to rule that out.
- **FR-3** An **in-process content cache**, keyed by `editionId`,
  holds the materialised temp file and an open `archive/zip.Reader`
  for up to **5** concurrently-open Editions (an LRU eviction policy —
  a reasoned placeholder, flagged in Open questions), each idle-evicted
  after **10 minutes** of no requests — closing the reader and removing
  the temp file on eviction. Idle-timeout tracking MUST use the
  injected `Clock` interface (`backend-test-harness.md` FR-5), never a
  direct `time.Now()`/timer call, so eviction is testable without a
  real 10-minute wait — the same discipline every other phase-03-
  dependent spec in this project already states explicitly. The cache
  is constructed once at process startup and passed to the handler as
  a dependency, not a package-level global (`backend-persistence.md`
  FR-1's/`backend-service-lifecycle.md` FR-2's existing no-globals
  discipline, restated here). **Each cache entry is reference-counted**:
  an in-flight request holding a reference to an entry prevents that
  entry's eviction (and the temp file's/reader's cleanup) until the
  request completes, even if the entry's idle timer would otherwise
  have expired — closing the race between LRU eviction and a
  concurrent in-flight read of the same Edition's content, a real risk
  given file-deletion-while-open semantics differ across the platforms
  this project ships on (POSIX permits it silently; Windows does not).
  This avoids re-fetching and re-materialising the whole EPUB from its
  source on every single page navigation during one reading session,
  the same "don't re-pay a cost repeatedly" reasoning
  `backend-metadata-caching.md` already established for a structurally
  similar problem.
- **FR-4** Once the zip entry matching `*path` is located (a lookup
  against the already-open `zip.Reader`'s entry table — `404 NotFound`
  if no entry matches), it MUST be decompressed through the same
  streaming byte-counting cap `backend-file-extractors.md` FR-5
  established (200 MiB, reused directly — not a new number invented
  here, though its *scope* changes: that spec applied the cap to a
  whole book's extraction, this FR applies the same number per single
  served entry, a stricter reapplication worth stating explicitly, not
  silently assumed identical) and the same entry-count cap (FR-5's
  10,000-entry limit) applies to the zip as a whole, checked once when
  FR-3's cache entry is first created, not per request. Every single
  request against this endpoint (a cache hit or a miss) is additionally
  bounded by a **30-second** per-request timeout via `ctx`
  (`backend-file-extractors.md` FR-3's own figure, reused for
  consistency) — FR-3's cache holding a book open for up to 10 minutes
  does not mean any single request may run that long; a request
  exceeding 30 seconds is aborted and returns `503 Unavailable`, the
  cache entry itself unaffected.
- **FR-5** Content-type dispatch, by the entry's own sniffed content
  (`net/http.DetectContentType`, never the `*path`'s extension alone):
  `text/html`/`application/xhtml+xml` → FR-6's HTML sanitisation;
  `text/css` → FR-7's CSS sanitisation; an image or font MIME type
  **other than `image/svg+xml`** → served as-is (binary formats have
  no script-execution surface to sanitise), still subject to FR-4's
  size cap; `image/svg+xml` as a *standalone* resource, and any
  unrecognised/unexpected content type (e.g. an embedded executable)
  → `400 InvalidInput` (`backend-errors-and-logging.md` FR-1's fixed
  six-category taxonomy has no category mapping to `403` at all —
  an earlier draft of this FR invented one; `InvalidInput` is the
  correct fit: the requested entry, while it exists, cannot be
  serviced as requested). Standalone SVG is refused rather than
  sanitised-and-served (FR-6's note) — named explicitly as a real
  feature loss (an EPUB cover in `.svg` form won't render), flagged in
  Open questions, not silently accepted as a security hole instead.
  - **Amended 2026-09-24 — structural XML.** foliate-js fetches
    `META-INF/container.xml`, the OPF package document, and (EPUB 2)
    the NCX by path before it can find the spine, so XML is a fourth
    route, checked **after** the SVG and HTML cases (the sniffer labels
    every `<?xml`-prefixed file `text/xml`, SVGs and XHTML chapters
    included) and only for a text sniff — an `.xml`/`.opf`/`.ncx`
    extension never makes a binary servable. The whole document is
    parsed: an XHTML root is a content document and goes to FR-6; any
    SVG, MathML, XSLT, or nested XHTML element, an `xml-stylesheet`
    instruction, or XML that does not parse → `400 InvalidInput`;
    anything else is served as-is as `application/xml` with no charset
    parameter (the file's own encoding declaration governs) and a CSP
    `sandbox` directive, so opening one directly yields an inert
    document.
- **FR-6** HTML/XHTML sanitisation via `bluemonday`
  (`microcosm-cc/bluemonday`, justified under constitution §9 below):
  a policy built from `bluemonday.UGCPolicy()`'s baseline (already
  strips `<script>` and event-handler attributes) further restricted
  to **reject any `href`/`src` value that isn't a relative path or a
  `data:` URI** — no absolute `http://`/`https://` reference survives
  sanitisation, closing the external-resource-load privacy risk
  (Security considerations) at the content layer itself, not only via
  the CSP header (FR-9) a client might fail to enforce. `<iframe>`,
  `<object>`, `<embed>`, and every `<link>` `rel` value **except
  `stylesheet`** (`prefetch`, `preconnect`, `dns-prefetch`,
  `modulepreload`, …) are stripped entirely, not merely
  attribute-filtered. **A `<link rel="stylesheet">` whose `href` is a
  relative path or `data:` URI is kept** (amended 2026-09-01) — it
  fetches the *also-sanitised* CSS from this system's own API origin, so
  it is not an external-fetch element in this FR's sense, and an EPUB's
  separate stylesheet files (the common case) would otherwise be lost;
  an `http(s)`/`//host` `href` on such a link is rejected by the same
  relative-or-`data:` rule as an `<img src>`. **`<svg>` (inline, embedded directly in HTML/
  XHTML content) is stripped wholesale, element and all** — SVG's own
  distinct script-execution surface (`onload`, `xlink:href="javascript:..."`,
  `<animate>`/`<set>` event attributes, `<foreignObject>` reintroducing
  arbitrary HTML) is not something `bluemonday`'s HTML-focused policy
  is built to close case by case, so this FR closes it the same way
  FR-5 closes standalone SVG: refused, not attempted. **A `style`
  attribute on any element is stripped entirely** (not passed through
  to FR-7's CSS scanner — an attribute-value fragment is not worth a
  separate parse path when simply disallowing it is just as correct
  for reflowable text content). **A `<style>` block's text content is
  routed through FR-7's own CSS sanitisation** before being allowed to
  remain — it is CSS, not HTML, and gets CSS's own rule, not a
  separate one.
  - **Amended 2026-09-24 — output shape, covers, and review fixes.**
    Chapters are served as `application/xhtml+xml`, so the output MUST
    be a well-formed XHTML document: the root carries the fixed XHTML
    namespace and the `epub` prefix (the policy strips every `xmlns` a
    book supplies), and the sanitised `<style>` sits inside `<head>`.
    `<style>` text is lifted by a tokenizer pre-pass (the same
    `x/net/html` tokenizer `bluemonday` uses) and re-emitted
    **HTML-escaped**, CDATA wrappers dropped, so markup smuggled inside
    a `<style>` can only ever be CSS text. `epub:type` is kept (tokens
    only): foliate-js locates an EPUB 3 book's contents by
    `nav[epub:type~=toc]`. **One SVG shape is kept as an image:** an
    `<svg>` that only wraps a single `<image>` — the EPUB cover-page
    idiom, optionally inside `<g>` beside `<title>`/`<desc>`/
    `<metadata>`/comments, `svg:`-prefixed or not — becomes
    `<img src=… alt=<title> class="alx-svg-cover">`, only for a
    relative or `data:` reference, and still passes the policy; every
    other `<svg>` is dropped as above. A relative `href`/`src` MUST NOT
    begin with, nor follow a leading `/` with, whitespace (ASCII or
    Unicode), a control character, or `\`: the policy tests the
    untrimmed value but emits the trimmed one, and browsers drop or
    normalise those characters, which would turn e.g. `"\u00a0//host"`
    into a protocol-relative reference.
- **FR-7** CSS sanitisation (applied to both standalone `text/css`
  resources, FR-5, and `<style>` block contents, FR-6) strips any
  `url(...)` or `@import` value that has **any URL scheme other than
  `data:`** — defined precisely as "has a `scheme:` prefix at all,
  and that scheme isn't `data:`," not merely "looks like an absolute
  `http(s)://` URL": a bare scheme check this way correctly rejects
  `url(javascript:alert(1))` too, which is neither a relative
  reference nor an `http(s)` URL and would otherwise slip past a
  narrower "block only http(s)" rule — the exact gap an earlier draft
  of this FR left open. A value with no scheme component at all
  (a genuinely relative reference) is allowed through unchanged. This
  is a small, narrowly-scoped hand-written check (a `url(`/`@import`
  token scan with the scheme rule above applied to whatever URL is
  found, not a full CSS parser) — justified under constitution §9 as a
  bounded problem ("does this one construct reference anything other
  than a relative path or `data:` URI") distinct from HTML
  sanitisation's much larger surface, where a real library is
  warranted instead.
- **FR-8** A resource inside the EPUB that itself references another
  resource by a path escaping the zip's own entry namespace (a crafted
  `href="../../../etc/passwd"`-shaped reference *inside* EPUB content,
  as distinct from FR-1's client-supplied `*path`) is not itself a risk
  this spec needs to close beyond what FR-1 already does: every
  resource reference inside sanitised content is still just a
  `*path` value the *client* re-requests through this same endpoint
  on its own next request, subject to FR-1's own validation again —
  there is no server-side "follow this reference automatically" step
  that could be tricked into an unsafe read.
- **FR-9** Every response (HTML, CSS, image, font) includes
  `Content-Security-Policy: default-src 'self'; script-src 'none';
  object-src 'none'` — a second, independent enforcement layer a
  sandboxed client (`frontend-reader.md`) can lean on in addition to
  its own iframe `sandbox` attribute, not a replacement for FR-6/FR-7's
  server-side sanitisation. `'self'` is sufficient (rather than needing
  `blob:` in the policy) specifically because this spec serves content
  from this system's own API origin directly — the architectural
  choice named in Context that avoids `foliate-js`'s own documented
  weak point.

## Non-functional requirements

- **Performance** — FR-3's cache is what keeps repeat page-navigation
  requests fast; a cache miss (first page of a newly-opened book, or
  after eviction) pays `Provider.Resolve` plus `Materialize`'s cost
  once, not per resource within that book.
- **Security** — see dedicated section below.
- **Accessibility** — not applicable; backend-only.
- **Reliability** — FR-2's multi-`SourceOffering` fallback means a book
  available from more than one source keeps working if one source goes
  temporarily unreachable, consistent with `domain-source.md`'s own
  "observed at T, not a guarantee" availability model.
- **Observability** — a sanitisation event that actually stripped
  something (a script tag found and removed, an external URL rejected)
  logs at `info` with the `editionId` and what *kind* of thing was
  stripped (never the stripped content itself, nor the surrounding
  document) — this is a genuinely useful signal (a source or a badly-
  behaved EPUB producer worth knowing about) that constitution §8's
  "never log what someone reads" doesn't prohibit, since it names a
  category of hostile content, not reading activity.

**Dependency justification (constitution §9)**: `microcosm-cc/bluemonday`
— what it does: allowlist-based HTML sanitisation built on Go's own
`net/html` tokenizer (not a third-party parser). Why not stdlib: Go's
stdlib has no sanitiser — hand-rolling HTML sanitisation correctly
(handling nested tags, attribute-value edge cases, encoding tricks
that can smuggle a script past a naive filter) is a well-known place
to introduce exactly the XSS vulnerability this spec exists to close;
`bluemonday` is modelled on the OWASP Java HTML Sanitizer's own
allowlist approach, a well-regarded design for this exact problem.
What breaks if abandoned: sanitisation policy construction is
self-contained to this spec's own code (a policy object built once,
applied per request) — a replacement library would mean swapping one
function call's implementation, not a structural rewrite.

## Domain model

No new domain types. This spec reads `domain-source.md`'s
`SourceOffering` (to find where to fetch bytes from) and
`domain-library.md`'s `LibraryEntry` (to confirm ownership before
serving anything) — both already-established types, used here for the
first time as a *read* path rather than a *write* path. This spec
introduces no persistence of its own beyond FR-3's in-process,
non-persisted cache.

## API and contracts

This endpoint's package, `internal/reader/content`, sits alongside
`internal/import/extract`/`internal/jobs` in `architecture-backend.md`
FR-1's layout — it depends on `internal/source` (for `Provider.Resolve`)
and `internal/import/extract` (for `Materialize`) but is never imported
by `internal/domain`, matching that spec's FR-2 dependency direction.

- `GET /api/v1/library/editions/:editionId/reader/content/*path` →
  `200` (body: sanitised HTML/CSS or raw binary, `Content-Type` set
  per FR-5, `Content-Security-Policy` per FR-9) → `400` (`InvalidInput`,
  malformed `*path` or unservable/unrecognised content type, FR-5),
  `404` (unowned `Edition` or no matching entry), `503` (`Unavailable`, every
  `SourceOffering`'s source unreachable)

## State transitions

Not applicable — every request is independent; FR-3's cache is a
performance optimisation with no caller-visible state machine (a cache
hit and a cache miss produce identical response bodies, differing only
in latency).

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| `editionId` isn't owned (no `LibraryEntry`) | FR-1's ownership check | `404 NotFound` | No content served, no `Provider.Resolve` attempted |
| `*path` contains `..`/absolute-path segments | FR-1's validation | `400 InvalidInput` | Rejected before any zip lookup |
| Every `SourceOffering`'s source is unreachable | `Provider.Resolve` fails for each in turn (FR-2) | `503 Unavailable` | No partial/cached-stale content served |
| Requested entry doesn't exist in the zip | Lookup miss against the open `zip.Reader` | `404 NotFound` | — |
| Entry decompresses past 200 MiB | FR-4's streaming cap (reused from `backend-file-extractors.md` FR-5) | `503 Unavailable` (treated as this book being currently unservable, not the client's fault) | Aborts mid-stream |
| A single request exceeds the 30-second per-request timeout | FR-4's `ctx` deadline | `503 Unavailable` | Request aborted; the FR-3 cache entry itself is unaffected |
| HTML content contains a `<script>`, an inline `<svg>`, a `style` attribute, or an absolute/`javascript:` external reference | FR-6/FR-7's sanitisation | Sanitised content (element/attribute/value silently removed), not an error | Logs at `info` per Observability, request still succeeds with the cleaned content |
| A CSS `url()`/`@import` references a `javascript:` or other non-`data:` scheme | FR-7's scheme check | Sanitised content, value removed | Same as above — a `javascript:` URI is rejected by the same "any scheme other than `data:`" rule as an `http(s)` one, not treated as a separate case |
| Standalone `image/svg+xml` resource requested | FR-5's dispatch | `400 InvalidInput` | Not served — a named, accepted feature loss (Open questions), not an oversight |
| Unrecognised/unservable content type inside the EPUB | FR-5's dispatch | `400 InvalidInput` | Not served |
| LRU eviction would otherwise remove an entry with an in-flight request still holding it | FR-3's reference count | No visible effect to the in-flight request | Eviction deferred until the reference count reaches zero |

## Security considerations

- **No script execution, in two independent layers** — server-side
  stripping (FR-6) and a CSP forbidding `script-src` (FR-9), neither
  alone assumed sufficient; this phase's own named exit criterion
  (a script-injection test fixture) exercises both.
- **No external network requests from rendered content** — FR-6/FR-7's
  external-URL rejection is the concrete mechanism protecting
  constitution's "what someone reads is private" at the point a page
  actually renders, not only at the logging layer.
- **Read-time defences are independent of import-time ones** — this
  spec reuses `backend-file-extractors.md`'s raw-cap, decompressed-
  cap, and entry-count mechanisms directly (Go-level function/constant
  reuse, not re-implementation), so the two access paths can't drift
  out of sync with each other over time.
- **Ownership checked before any byte is fetched** (FR-1) — a
  non-owned `Edition`'s content is never resolved or served, closing
  off this endpoint as a way to read a book the user doesn't actually
  have, even one this system knows about via Discover/metadata alone.
- **The in-process cache (FR-3) holds no cross-request secret** — a
  materialised EPUB's bytes are not sensitive in the credential sense,
  but are still reading-related content; the cache is never persisted
  to disk beyond the temp file `Materialize` itself already creates
  (cleaned up on eviction, same as `backend-file-extractors.md`'s own
  temp-file lifecycle).

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | HTML sanitisation against fixtures containing `<script>`, event-handler attributes, inline `<svg>`, `style` attributes, absolute external URLs, and legitimate EPUB-typical markup (confirming the policy doesn't over-strip normal content); CSS sanitisation against external `url()`/`@import` fixtures **including a `url(javascript:...)` fixture specifically**, both as standalone `.css` resources and inside `<style>` blocks; `*path` validation rejecting `..`/absolute segments |
| Integration | Content serving against a real imported EPUB (`backend-test-harness.md`'s harness, `backend-source-adapter.md`'s fake-provider pattern reused); a zip-bomb/zip-slip fixture proving the reused defences actually apply at this read path, not only at import time; the FR-3 cache's eviction behaviour under the injected `Clock`, including a reference-counted in-flight request surviving an eviction sweep that would otherwise have removed its entry; a second request against an already-cached Edition succeeding *after* the first request's own HTTP connection has fully closed, proving the cache's context is independent of any one request's (FR-2); a test asserting no reading-related content (title, path, sanitised-out content) appears in captured log output |
| Contract | `architecture-contracts.md` FR-3's `kin-openapi` tool, extended to this endpoint |
| E2E | Open a real EPUB → request its first spine item and a referenced image → both render correctly; a script-injection fixture confirmed inert end to end (this phase's own named exit criterion) |
| Accessibility | N/A at this layer — `frontend-reader.md`'s concern |

Tests that must fail before implementation begins: a test asserting a
`<script>` tag, an inline `<svg>`, and a `style` attribute never
survive sanitisation; a test asserting an absolute external URL or a
`javascript:` URI in an `href`/`src`/CSS `url()` is rejected, not
passed through; a test asserting a non-owned `Edition`'s content is
never served regardless of `*path`; a test asserting a `*path`
containing `../` is rejected before any zip lookup; a test asserting a
cached Edition's content is still servable after the request that
first populated the cache has fully completed and its own context has
been cancelled.

## Acceptance criteria

- [ ] A legitimate EPUB spine item, image, and CSS resource all serve
      correctly through this endpoint
- [ ] No `<script>` tag or event-handler attribute survives
      sanitisation, proven by a test
- [ ] No absolute external URL or `javascript:` URI survives
      sanitisation in either HTML or CSS content, proven by a test
- [ ] No inline `<svg>` or `style` attribute survives HTML sanitisation
- [ ] A non-owned `Edition`'s content is never served
- [ ] The zip-bomb/zip-slip/entry-count defences apply at this
      read-time path, proven independently of phase 10's own import-
      time tests
- [ ] A cached Edition's content remains servable after the request
      that first populated the cache has completed
- [ ] No reading-related content appears in logs, proven by a test
- [ ] The CSP header is present on every response
- [ ] The contract test passes against this endpoint

## Open questions

- **FR-3's cache size (5 editions) and idle timeout (10 minutes)** —
  reasoned placeholders, not load-tested; revisit once real reading
  session patterns exist.
- **FR-5's `400` for unrecognised content types, including standalone
  SVG** — a conservative default; SVG cover images specifically are a
  named, real feature loss (Functional requirements, FR-5), not an
  oversight; if a real EPUB legitimately needs SVG support, that's a
  gap to revisit with a dedicated SVG sanitiser, not assumed away now.

## References

- `backend-file-extractors.md` (phase 10) FR-1, FR-5, FR-6 —
  `Materialize`, decompressed-content cap, entry-count cap, all reused
  directly, not re-implemented
- `backend-source-adapter.md` (phase 08) — `Provider.Resolve` reused
- `domain-source.md` (phase 02) FR-5 — multi-`SourceOffering` per
  `Edition`, the basis for FR-2's fallback
- `domain-library.md` (phase 02) FR-1 — ownership fact this spec's
  FR-1 checks; distinguished from FR-2's Work-level "in library" flag
- `backend-import-pipeline.md` (phase 10) FR-4(a) — the join-through-
  `LibraryEntry` discipline this spec's FR-1 restates for a read path
- `backend-metadata-caching.md` (phase 07) — "don't re-pay a cost
  repeatedly" precedent for FR-3's cache
- `backend-errors-and-logging.md` (phase 03) FR-1 — the fixed,
  six-category error taxonomy FR-5's `400 InvalidInput` mapping
  follows
- `frontend-reader.md` (phase 11, sibling) — the client this spec
  serves; its own rendering-engine research (`foliate-js`'s documented
  sandboxing weak point) is this spec's own reason to exist
- Constitution §4 (hostile input, this phase's second file-content
  boundary), §8 (what someone reads is private, extended to the
  rendering boundary), §9 (dependency justification for `bluemonday`)
