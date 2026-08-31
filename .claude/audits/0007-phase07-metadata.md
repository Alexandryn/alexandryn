# Security Audit: Phase 07 — Metadata Discovery & Caching

| | |
|---|---|
| **Scope** | Open Library adapter client (`internal/adapters/openlibrary/`), rate limiting (`ratelimit.go`), normalizer (`normalizer.go`), persistent PostgreSQL & filesystem caching repositories (`internal/persistence/postgres/metadata_cache_repository.go`, `cover_cache_repository.go`), Goose migration `00004_phase07_metadata_cache.sql`, HTTP discovery handlers (`internal/transport/http/discover.go`, `lazy.go`), server router & startup wiring (`cmd/server/`), frontend data layer (`web/src/data/discover.ts`), Discover screens & components (`web/src/screens/Discover/`, `web/src/components/DiscoverResultGrid/`), and E2E / contract test suites (`internal/testutil/contracttest/`, `web/e2e/discover.app.spec.ts`, `hostile-boundary.app.spec.ts`). |
| **Auditor** | Antigravity AI Assistant & Senior Security Auditor (`agent-skills:security-auditor`) |
| **Threat Model** | Four-Attacker Threat Model (Constitution §4, §5, §8, §9, §10, §11 / `backend-metadata-adapter.md` / `backend-metadata-caching.md` / `frontend-discover-screen.md`) |
| **Date** | 2026-08-31 |
| **Commit** | Branch `feat/phase07-metadata` (PR #72) |
| **Verdict** | **Clear** — All Phase 07 requirements, constitutional constraints, and security boundary protections verified. Zero open Critical or High findings. Full CI test suites (Go unit/race/integration, Kin-OpenAPI contract tests, Vitest, and Playwright E2E) passing. |

---

## Scope and Method

Phase 07 introduces outbound metadata discovery against Open Library, persistent 30-day PostgreSQL metadata caching, filesystem-backed 500 MiB LRU cover caching with image magic byte verification, and the frontend Discover screen and work detail view.

This audit evaluates the implementation against the project Constitution and governing specifications:
1. **Constitution §4 (Line-1 Validation & Bounded Types):** Explicit length bounds, whitelist enums, rate limits, non-empty IDs, and numeric bounds on all endpoints.
2. **Constitution §5 (Renderer Privilege Boundary):** Web renderer and LAN clients treated as completely untrusted.
3. **Constitution §8 (Redaction & Privacy):** Search queries (`q`) and free-text fields must never be emitted into structured logs or unredacted URLs.
4. **Constitution §9 (Minimalist Dependencies):** Pure Go standard library networking and image validation; zero unvetted runtime dependencies.
5. **Constitution §10 (Four-Attacker Threat Model):** Malicious LAN Client, Hostile Remote Server / SSRF, Untrusted Dependency / Supply Chain, Local Multi-User / Resource Exhaustion.
6. **Constitution §11 (Error Hygiene):** Uniform `{ code, message, correlationId }` error responses; distinct 404 vs 503 copy; no retry buttons on 404 states.
7. **Governing Specs:** `backend-metadata-adapter.md`, `backend-metadata-caching.md`, `frontend-discover-screen.md`.

---

## Four-Attacker Threat Assessment

### Attacker 1: Malicious LAN Client / Untrusted Renderer
*Threats:* Parameter tampering, control character injection, oversized payload DOS, path traversal in work or cover IDs, SQL injection, information leakage via stack traces or internal errors.
*Evaluation:* **High Assurance / Verified Compliant.**
- **Line 1 Validation:** Every handler (`DiscoverSearchHandler`, `DiscoverWorkDetailHandler`, `DiscoverCoverHandler`) validates all inputs on line 1 before invoking any client or repository calls.
- **Bounded Parameters:**
  - `q`: required, trimmed, bounded to `1..200` runes with zero control characters (`unicode.IsControl`).
  - `limit`: bounds enforced to `1..50` (default 20), non-integers rejected with `400 InvalidInput`.
  - `offset`: bounds enforced to `>= 0` (default 0), non-integers rejected with `400 InvalidInput`.
  - `openLibraryId`: strictly validated against regex `^OL[0-9]+[WAM]$` (rejects path traversal, empty strings, and malformed keys).
  - `coverId`: parsed strictly as positive `int64` > 0 (rejects negative numbers, zero, non-integers, and path traversal strings).
- **Parameterized SQL:** All PostgreSQL cache operations in `PostgresMetadataCacheRepository` and `FilesystemCoverCacheRepository` use parameterized queries (`$1, $2, ...`) without string concatenation.
- **Error Hygiene (Constitution §11):** Handlers return domain-mapped `{ code, message, correlationId }` JSON responses. No internal database connection strings, file paths, or stack traces leak to the client.

### Attacker 2: Hostile Remote Server (Open Library / Upstream SSRF / Poisoning)
*Threats:* SSRF attacks, slowloris/unbounded streaming DOS, oversized response payloads, malicious non-image files / polyglot payloads disguised as covers, query leakage in outbound request logging, author fan-out amplification.
*Evaluation:* **Strong Defense.**
- **Hardcoded Upstream Targets (SSRF Prevention):** Outbound HTTP requests exclusively target hardcoded base URLs (`https://openlibrary.org` and `https://covers.openlibrary.org`). Client input parameters are strictly encoded and sanitized.
- **Size Caps & Timeouts:** All HTTP response bodies are wrapped in `io.LimitReader(resp.Body, 5<<20)` (5 MiB cap). Requests execute under a strict 5-second per-request timeout budget.
- **Token-Bucket Rate Limiter:** An in-process token-bucket rate limiter caps outbound requests at 3 req/sec with a maximum concurrency bound of 50 waiters. Excess concurrent requests fail fast with `503 Unavailable`.
- **Author Fan-out Cap:** Author detail queries for work normalization are capped at 20 distinct authors. Single author fetch failures gracefully degrade to name-only records without aborting the work detail request.
- **Image Magic Byte Validation (Polyglot / Remote Exploit Defense):** `FilesystemCoverCacheRepository` decodes cover payloads using `image.DecodeConfig` (standard library `image/jpeg`, `image/png`, `image/gif`, `golang.org/x/image/webp`) before writing to disk or database. Non-image payloads (e.g. HTML, executables, scripts) are rejected with `InvalidInput`.
- **Query Redaction (Constitution §8):** In `internal/adapters/openlibrary/client.go`, `sanitizeURL` and `sanitizeError` sanitize log lines and error strings, replacing `q=<query>` with `q=[REDACTED]` to prevent user search terms from leaking into logs.

### Attacker 3: Untrusted Third-Party / Dependency Compromise
*Threats:* Vulnerable or malicious third-party dependencies introduced for rate limiting, HTTP routing, image decoding, or caching.
*Evaluation:* **Minimalist & Audited.**
- **Zero Unjustified Dependencies (Constitution §9):**
  - Rate limiting is implemented using pure standard library primitives (`sync.Mutex`, `time.Ticker`, channels) in `internal/adapters/openlibrary/ratelimit.go`.
  - Image decoding utilizes the Go standard library and official subrepository `golang.org/x/image/webp`.
  - No third-party HTTP routers or ORM dependencies added.
- **Vulnerability Audit:** Scanned with `govulncheck` and frontend security scripts (`npm run check:dist-secrets`, `npm run check:dist-msw`).

### Attacker 4: Local Multi-User / Storage Resource Exhaustion
*Threats:* Unbounded disk usage filling host storage, corrupt cover cache files, race conditions during simultaneous cover downloads, orphaned temporary files, connection pool exhaustion during cache eviction.
*Evaluation:* **High Assurance Architecture.**
- **LRU Cache Disk Bound:** `FilesystemCoverCacheRepository` tracks cumulative cover bytes in PostgreSQL (`metadata_covers`) and automatically triggers buffered LRU eviction when total disk usage exceeds 500 MiB (`MaxDiskBytes`), evicting least-recently-accessed covers until size drops to 450 MiB (`TargetDiskBytes`).
- **Connection-Safe Eviction:** Eviction queries buffer candidate file records and close the active database rows before executing filesystem removals and PostgreSQL deletion queries, preventing database connection starvation.
- **Atomic Disk Writes:** Cover downloads are written to a temporary file (`cover-*.tmp`) and atomically renamed via `os.Rename` with directory `0700` and file `0600` permissions.
- **Missing File Resilience:** If a cache row exists in the database but the physical file is missing or corrupted on disk, the repository cleanly degrades to a cache miss and returns `ErrCacheMiss` without panicking.
- **Negative Caching (404 Sentinels):** Upstream 404 responses are persisted via `MarkMissing` to short-circuit future requests for nonexistent covers and avoid redundant outbound network traffic.

---

## Verification Results

| Test Suite | Command | Result |
|---|---|---|
| Go Unit & Adapter Tests | `go test -v -race ./internal/adapters/openlibrary/...` | **PASS** (100% pass, 0 race conditions) |
| Go Persistence Integration | `go test -tags=integration -v ./internal/persistence/postgres/...` | **PASS** (PostgreSQL metadata & cover cache integration verified) |
| Go HTTP Transport & Routing | `go test -v ./internal/transport/http/... ./cmd/server/...` | **PASS** (Discover handlers, lazy repos, router wiring verified) |
| OpenAPI Contract Tests | `go test -v ./internal/testutil/contracttest/...` | **PASS** (100% schema alignment with `api/openapi.yaml`) |
| Web Unit & Component Tests | `npm --prefix web test` | **PASS** (65 test files, 318 unit tests passing) |
| Web Lint & TypeScript | `npm --prefix web run lint && npm --prefix web run build` | **PASS** (0 linter errors, 0 type errors, 131.1 KiB gzip bundle) |
| Web Token & A11y Checks | `npm --prefix web run check:token-styling && npm run check:a11y-tabindex && npm run check:a11y-hidden-text` | **PASS** (Clean) |
| Playwright E2E & A11y Audits | `npx playwright test --config web/playwright.config.ts` | **PASS** (24 tests passing across benchmark, app, and gallery suites) |

---

## Conclusion & Stop Gate Readiness

Phase 07 (Metadata Discovery & Caching) is fully implemented, verified, and hardened across Tiers 0 through 6. All functional and non-functional requirements defined in `backend-metadata-adapter.md`, `backend-metadata-caching.md`, and `frontend-discover-screen.md` are satisfied.

The implementation is ready for maintainer sign-off and branch merge.
