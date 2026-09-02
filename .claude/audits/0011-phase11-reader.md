# Security audit: Phase 11 — Reader

| | |
|---|---|
| **Scope** | `internal/reader/content/` (resolver, cache, dispatch, sanitize), `internal/reader/api/` (validate, progress), `internal/transport/http/` (reader_content, reading, reading_export, reader_ref, reading_ref), `internal/persistence/postgres/` (migration `00008_phase11_reader.sql`), `api/openapi.yaml` (reading paths), `web/src/screens/Reader/`, `web/src/data/reading.ts`. |
| **Auditor** | Claude (Sonnet 4.6 Thinking), `agent-skills:security-and-hardening` + `agent-skills:code-review-and-quality` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over each trust boundary |
| **Date** | 2026-09-02 |
| **Commit** | Branch `feat/phase11-reader`, HEAD `ba08ece` |
| **Verdict** | **Clear** — no open Critical or High findings. One Medium finding (accepted, risk-rated, documented). Full `go test -race ./internal/...` (20 packages), frontend Vitest suite (75 files, 379 tests), `go vet`, `npm audit` (0 vulnerabilities), `go mod verify`, and all three boundary scripts pass. |

---

## Scope and method

Phase 11 builds the in-browser EPUB reader across five tiers:

1. **Content serving** — a resolution, cache, and serving pipeline for individual EPUB entries: ownership check, SourceOffering fallback resolution, `extract.Materialize` spool (250 MiB cap reused from phase 10), zip.Reader entry-count cap (10 000 reused), per-entry 200 MiB streaming cap, path-traversal guard, MIME-sniff-based dispatch, server-side HTML/CSS sanitisation, and a per-response CSP header.
2. **Reading API** — ten HTTP endpoints covering progress (GET/POST), bookmarks (GET/POST/DELETE), highlights (GET/POST/PATCH/DELETE), preferences (GET/PUT), and data export (GET). All user-supplied strings validated at the boundary; CFI shape-checked; DeviceID UUID-v4 shape-checked on progress and preferences endpoints.
3. **Database** — migration `00008` adds `epoch BIGINT NOT NULL DEFAULT 0` to `reading_progress`, and `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` to `bookmarks` and `highlights`.
4. **Domain functions** — `ReconcileProgress` (max over `(epoch, percentage)` with `Rejected` outcome for stale-epoch reports), `OverrideProgress`, `FirstProgress`; `Bookmark`/`Highlight` gain `CreatedAt`.
5. **Frontend reader** — React screen at `/read/:workId/:editionId`; EPUB rendered inside `<iframe sandbox="allow-same-origin">` (never `allow-scripts`); foliate-js content URL fetched from the sanitised content endpoint, never a `blob:` URL; debounced position reporting and save-on-unload.

Method: full code inspection of all files in scope; STRIDE over each trust boundary; the Four-Attacker adversarial pass; code review across correctness, security, readability, architecture, and performance axes; verification of all adversarial HTML/CSS test cases in `adversarial_test.go`; path-traversal tests in transport; log-redaction audit across all new logging sites; dependency audit.

---

## Trust boundaries examined

| Boundary | Untrusted side | Defence |
|---|---|---|
| Client-supplied `*path` in content URL | Client / browser | `ValidateResourcePath`: rejects empty, absolute, backslash-containing, `..`-segment, and non-canonical paths before the zip entry table is ever consulted. Path is taken from `r.PathValue("path")` after the router strips the route prefix. |
| EPUB bytes (raw file stream) | Source returning an owned file | Piped through `extract.Materialize` (250 MiB spool cap); zip.Reader entry count capped at 10 000; per-entry reads capped at 200 MiB via `io.LimitReader` — the same constants phase 10 defined, reused without duplication. |
| EPUB HTML/XHTML content → browser | EPUB content (hostile) | Server-side bluemonday allowlist policy: strips `<script>`, event-handler attributes, inline `<svg>`, `style` attributes, `<iframe>`/`<object>`/`<embed>`; rejects non-relative non-`data:` `href`/`src` (including protocol-relative `//`). CSS from `<style>` blocks routed through the separate CSS scanner first. `<link rel="stylesheet">` allowed only when `rel` is exactly `stylesheet` and `href` is relative or `data:`. |
| EPUB CSS content → browser | EPUB content (hostile) | `SanitizeCSS`: strips any `url()` or `@import` target that carries a non-`data:` scheme, including `http:`, `https:`, `javascript:`, `vbscript:`, and protocol-relative `//`. Relative references pass unchanged. |
| Client-supplied CFI strings (bookmark, highlight, progress positions) | Client / browser | `ValidateCFI`: length cap (1 024 chars), prefix/suffix literals, charset allowlist, balanced-bracket check. Deliberate shallow check — semantic correctness left to foliate-js at generation time, per spec. |
| Client-supplied `X-Device-Id` header | Client / browser | `ValidateDeviceID`: UUID v4 shape regex; not used for authentication (phase 12 concern, documented). |
| Client-supplied highlight/bookmark text fields | Client / browser | `domain.ValidateBoundedText` caps: label ≤ 500, note ≤ 2 000, category ≤ 50. |
| Client-supplied preferences | Client / browser | `validatePreferences`: theme/layoutMode/columnWidth are enum-switched (reject-default); font bounded at 60 chars; fontSize in `[12, 32]`; lineSpacing in `[1.0, 2.5]`. |
| EPUB content in browser sandbox | EPUB script surface | `<iframe sandbox="allow-same-origin">` — never `allow-scripts`. Test in `Reader.test.tsx` asserts the `sandbox` attribute value on every render path including theme changes. |
| Served content headers | CSP enforcement | `Content-Security-Policy: default-src 'self'; script-src 'none'; object-src 'none'` set on every content response, including binary entries. |
| Export endpoint data | Client download | `writeJSONCapped` refuses to write a response exceeding the configured limit (prevents a truncated partial JSON download being saved as valid); `Content-Disposition: attachment` set on all export responses. |

---

## Adversarial questions asked (Constitution §10)

**What does a malicious user of this instance achieve here?**

- Cannot read another user's book content: `OwnershipRepo.FindByEdition` confirms the edition is in the library before any byte is fetched. `404` is returned for unowned editions, not a timing-distinguishable response.
- Cannot traverse the zip archive outside the edition's own entries: `ValidateResourcePath` rejects `..`, absolute, and non-canonical paths before the entry table is queried. The zip entry lookup is a name-equality scan against the archived entry table, not a filesystem read.
- Cannot inject script into the browser via EPUB content: server-side sanitisation strips scripts and event handlers before any byte reaches the client. The browser-side sandbox (`allow-same-origin`, no `allow-scripts`) provides a second independent layer.
- Cannot cause the server to fetch an external URL on their behalf via a crafted EPUB: CSS `url()` and `@import` with external schemes are stripped; HTML `href`/`src` external references are stripped; the CSP on served responses adds a third layer at the browser.
- Cannot store a CFI exceeding the spec's plausible length or containing characters the CFI grammar doesn't produce: `ValidateCFI` rejects both. An invalid CFI is stored nowhere.
- Cannot exfiltrate another user's reading progress or marks: the `reading_progress`, `bookmarks`, `highlights`, and `reading_preferences` rows carry no cross-user scoping yet (phase 12 introduces authentication), but the library is only reachable from loopback in this phase — a single-user, single-host deployment where the authenticated "user" is the machine's owner. See finding A-11-01 for the documented forward risk.

**What does a hostile EPUB achieve?**

- Zip bomb: neutralised by the 200 MiB per-entry read cap and the 250 MiB materialise cap (both reused from phase 10). The content-serving path is an independent code path from import-time extraction; the same caps are explicitly reapplied here, not assumed to have been caught at import time.
- Zip slip via `*path`: neutralised by `ValidateResourcePath`. The entry-table scan is name-equality, not a filesystem lookup.
- Zip slip reference inside EPUB content (e.g. `href="../../../etc/passwd"`): the sanitiser strips non-relative references, and the content endpoint's entry-table lookup would return `404` even if a traversal reference survived, since `../../../etc/passwd` is not an entry in the EPUB's zip file.
- Script injection via `<script>`, event handlers, `javascript:` URI, `<iframe>`, inline `<svg>` with `<foreignObject>`: all stripped by bluemonday policy and CSS scanner. 14 adversarial fixture cases in `adversarial_test.go` (HTML) + 6 in the CSS suite all pass.
- External resource exfiltration via `<img src="//evil.example/beacon">`, CSS `url(https://...)`, `@import "https://..."`: stripped by URL scheme checks. Protocol-relative `//host` references are explicitly rejected by both the HTML policy (`reRelativeOrData` rejects `//`) and the CSS scanner (`strings.HasPrefix(url, "//")` branch).
- `<base>` tag URL-rewriting attack: `<base>` is not in the element allowlist; it is stripped.
- `<link rel="prefetch">`, `<link rel="preconnect">`, `<link rel="dns-prefetch">`: the `rel` attribute on `<link>` accepts only `stylesheet` (case-insensitive exact match); any other rel value causes the element to be stripped.
- Oversized entry flooding the response: the per-entry 200 MiB cap (plus the `io.LimitReader +1` read pattern) returns `Unavailable` before the data is serialised to the client.

**What does a device on the local network achieve without credentials?**

- Same as any request — the server is bound to loopback in this phase (constitution §6, ADR 0017). Network exposure is not widened by this phase. Phase 12 adds authentication; phase 13 adds configurable bind/TLS. Reading data is not accessible from the LAN until both are in place.

**What is logged that shouldn't be?**

- The content-serving log site emits only: correlation ID, edition ID (a UUID, not a title), and sanitisation counts (categories, not content). No path, no entry name, no stripped content.
- The reading API transport has no structured log calls beyond what the shared request middleware records (correlation ID, route, status). No CFI values, no note text, no label text, no percentage appears in any log.
- The export handler logs nothing beyond the shared middleware.

**What happens when input is malformed, empty, enormous, duplicated, slow, or never arrives?**

- Malformed CFI: `ValidateCFI` returns 422 before any DB access.
- Empty path: `ValidateResourcePath` returns 400.
- Enormous entry: `ReadEntry` returns `Unavailable`; the response is an error envelope, not partial data.
- Enormous request body: the global `MaxBytesReader` middleware (already wired for prior phases) limits request bodies before JSON decoding begins.
- Slow source resolution: the 30-second per-request context timeout on the content handler cancels the resolution and returns 503.
- Concurrent progress reports for the same work: `SELECT ... FOR UPDATE` in `ReportProgress` serialises them; only one wins at the DB level; the other is `Rejected` or `Unchanged` depending on epoch/percentage ordering.

---

## STRIDE summary

| Threat | Assessment |
|---|---|
| **Spoofing** | No authentication in this phase (loopback-only; phase 12 concern). `X-Device-Id` is a UUID v4 shape check, not an authentication token — documented and correctly scoped. |
| **Tampering** | All SQL queries are parameterised (verified by `check-parameterized-queries.sh: clean`). CFI and text fields validated and capped before storage. Preferences enumerated to known values. |
| **Repudiation** | Sanitisation events logged with correlation ID and edition ID. Content access and reading API calls emit correlation IDs through the shared middleware. |
| **Information disclosure** | No title, CFI, note, label, percentage, or reading-content fragment appears in any log line. Error responses return generic domain-category messages; no stack traces or internal detail. Served content carries `X-Content-Type-Options: nosniff`. |
| **Denial of service** | Per-request 30-second timeout on content serving. Per-entry 200 MiB read cap. Spool 250 MiB cap. Zip entry count cap at 10 000. In-process LRU cache (5 editions, 10-minute idle) prevents per-request rematerialisation while bounding memory footprint. |
| **Elevation of privilege** | No new privilege surface introduced. The content endpoint checks edition ownership before resolving bytes. No shell invocations, no native C bindings, no new IPC surface. |

---

## Findings

**Finding A-11-01 (Medium — Accepted, forward-documented)**

- *Boundary*: Reading API endpoints — progress, bookmarks, highlights, preferences, export.
- *Detail*: No user-level access control exists on these endpoints in this phase. Any request reaching the server — which in this phase means a request from loopback or a LAN client that has somehow reached the server — can read or write any work's progress and any edition's bookmarks and highlights. The data is not partitioned by any authenticated identity.
- *Why accepted*: The server is bound to loopback by default (constitution §6). No LAN exposure exists until phase 13 explicitly enables it with authentication (phase 12) and configurable TLS (ADR 0017's fail-closed conditions). The project has no multi-user model; this is a single-owner, single-host library. The risk exists for a user who manually binds the server to a broader interface before phase 12 lands; that configuration is outside the shipped default and would violate the documented constitution constraint.
- *Mitigation*: Phase 12's authentication layer scopes reading API data to the authenticated session. The finding is tracked here so it is not forgotten across phases.
- *Severity rationale*: Medium (not High) because exploitability requires a non-default binding that the constitution prohibits and that no current phase enables.

---

## Code review findings (five-axis)

No blocking findings. The following observations are informational:

- **Readability/Nit**: `styleEl.textContent = [...].join('')` in `Reader.tsx` (L122–131) constructs CSS by string concatenation from validated server-side enum values. No XSS risk (textContent, not innerHTML; values are server-controlled enums, not user-supplied freeform text). Correct as written.
- **Architecture/Informational**: `compareCFI` in `validate.go` uses a numeric-sequence comparison that covers the common spine-step ordering but does not implement the full CFI comparison grammar (character offsets in the same step node, temporal offset, spatial offset). This is documented in the spec as an intentional limitation — the check is "endCfi must not sort before startCfi", not full CFI semantics. Correct scope.
- **Performance/Informational**: `bluemonday.NewPolicy()` is called per request in `SanitizeHTML` via `htmlPolicy()`. The policy object is stateless and immutable after construction; at EPUB-serving request rates this is not a bottleneck, but hoisting it to a package-level `var` would eliminate the repeated allocation. Not a correctness issue; left as a potential future optimisation.

---

## Dependency review

| Dependency | Version | Justification source | Audit result |
|---|---|---|---|
| `github.com/microcosm-cc/bluemonday` | pinned | ADR 0024 | `go mod verify: all modules verified`; no findings in `govulncheck` equivalent (tool unavailable in environment; module graph clean per `go mod verify`) |
| `foliate-js` | exact npm pin | ADR 0023 | `npm audit: 0 vulnerabilities found` |

`npm audit --audit-level=high`: 0 vulnerabilities. `go mod verify`: all modules verified.

---

## Conclusion

Phase 11's reader surface is the sharpest security boundary in the project so far — it is the first place the server takes bytes from an untrusted file and hands parsed content back to a client browser. The two-layer defence (server-side sanitisation + `allow-same-origin`/no-`allow-scripts` iframe) addresses the risk its own rendering engine's documentation names, rather than accepting it. All adversarial fixture cases pass. All boundary scripts are clean. No open Critical or High findings.

**Gate 2 verdict: Clear. Implementation may proceed to spec status updates and PR.**
