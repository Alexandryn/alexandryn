# Security Audit: Phase 08 — Sources Management & Ingestion

| | |
|---|---|
| **Scope** | Domain entities & services (`internal/domain/source.go`, `source_removal_service.go`, `source_health.go`, `source_candidate.go`, `cursor.go`, `semaphore.go`), Adapters (`internal/adapters/sources/local/`, `internal/adapters/sources/opds/`, `cursor_codec.go`, `opds_parser.go`), Persistence & Encryption (`internal/persistence/postgres/sources.go`, `internal/crypto/keys.go`, `internal/crypto/service.go`, Goose migration `00005_phase08_sources.sql`), HTTP Transport (`internal/transport/http/sources.go`, `lazy.go`), Server wiring (`cmd/server/`), Frontend data layer & UI (`web/src/data/sources.ts`, `web/src/components/Source*`, `web/src/screens/Sources/`, `web/src/app/routes.tsx`), and E2E / hostile boundary test suites (`web/e2e/sources.app.spec.ts`, `web/e2e/hostile-boundary.app.spec.ts`). |
| **Auditor** | Antigravity AI Assistant & Senior Security Auditor (`agent-skills:security-auditor`) |
| **Threat Model** | Four-Attacker Threat Model (Constitution §4, §5, §8, §9, §10, §11 / `backend-source-adapter.md` / `frontend-source-management.md` / `domain-source.md`) |
| **Date** | 2026-08-31 |
| **Commit** | Branch `feat/phase08-sources` (PR #80) |
| **Verdict** | **Clear** — All Phase 08 requirements, constitutional constraints, and security boundary protections verified. Zero open Critical or High findings. Full CI test suites (Go unit/race/integration, Kin-OpenAPI contract tests, Vitest, and Playwright E2E) passing. |

---

## Scope and Method

Phase 08 introduces the Sources subsystem: connecting, managing, and browsing local filesystem folders and remote OPDS 1.2 catalog feeds, with AES-256-GCM credential encryption at rest, HMAC-SHA256 authenticated pagination cursors, strict SSRF defenses, and frontend management/browsing screens.

This audit evaluates the implementation against the project Constitution and governing specifications:
1. **Constitution §4 (Line-1 Validation & Bounded Types):** Explicit length bounds, whitelist enums, rate limits, non-empty IDs, and numeric bounds on all endpoints.
2. **Constitution §5 (Renderer Privilege Boundary):** Web renderer and LAN clients treated as completely untrusted. Local folder picker gated behind Electron IPC presence check.
3. **Constitution §8 (Redaction & Privacy):** Passwords and credentials encrypted at rest with AES-256-GCM, never logged, never echoed back in API responses (`hasCredential: boolean`). URLs with embedded credentials rejected or redacted in logs.
4. **Constitution §9 (Minimalist Dependencies):** Pure Go standard library networking, HKDF, AES-GCM, and XML parsing; zero unvetted runtime dependencies.
5. **Constitution §10 (Four-Attacker Threat Model):** Malicious LAN Client, Hostile Remote Server / SSRF, Untrusted Dependency / Supply Chain, Local Multi-User / Resource Exhaustion.
6. **Constitution §11 (Error Hygiene):** Uniform `{ code, message, correlationId }` error responses; distinct 404 vs 503 copy; no retry buttons on 404 states.
7. **Governing Specs:** `backend-source-adapter.md`, `frontend-source-management.md`, `domain-source.md`.

---

## Four-Attacker Threat Assessment

### Attacker 1: Malicious LAN Client / Untrusted Renderer
*Threats:* Parameter tampering, control character injection, path traversal via `basePath` or `referenceId`, cursor forgery/tampering to bypass pagination or explore foreign endpoints, credential extraction, SQL injection, information leakage via stack traces or error responses.
*Evaluation:* **High Assurance / Verified Compliant.**
- **Line 1 Validation:** Every handler (`SourceCreateHandler`, `SourceListHandler`, `SourceGetHandler`, `SourceUpdateHandler`, `SourceDeleteHandler`, `SourceHealthCheckHandler`, `SourceBrowseHandler`, `SourceSearchHandler`) validates all inputs on line 1 before invoking any adapter or repository calls.
- **Credential Write-Only Discipline:** The API never returns stored credentials (`username` or `password`). Responses return `hasCredential: boolean`. Edit flows only support writing new credentials, never pre-filling.
- **HMAC-Signed Opaque Cursors:** Paging cursors are serialized as base64url payloads with an HMAC-SHA256 signature derived from a dedicated key (`source-cursor-hmac-v1`). Any forged, tampered, expired, or source-mismatched cursor fails authentication with `400 InvalidInput`.
- **Path Traversal Prevention:** In the local folder adapter, directory traversal (`..`, symlinks pointing outside `basePath`, absolute path references) is strictly rejected. Only files strictly inside `basePath` are discovered or referenced.
- **Parameterized SQL:** All database operations in `SourceRecordRepository` and `SourceRemovalService` use PostgreSQL parameterized queries (`$1, $2, ...`) without string interpolation.

### Attacker 2: Hostile Remote Server / Upstream OPDS Server (SSRF & Poisoning)
*Threats:* SSRF attacks targeting internal infrastructure, cloud metadata endpoints (`169.254.169.254`), loopback (`127.0.0.1`, `::1`), private LAN subnets (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`), DNS rebinding attacks, slowloris/unbounded streaming DOS, oversized response payloads, XML entity expansion (Billion Laughs / XXE).
*Evaluation:* **Strong Defense.**
- **Strict SSRF Probing & URL Filtering:** Remote OPDS URLs are resolved and validated: private IPs, loopback, link-local, carrier-grade NAT, and cloud metadata addresses are unconditionally rejected unless explicitly enabled for testing. Custom `net.Dialer` enforces IP filtering at connect time to prevent DNS rebinding.
- **XML Security (XXE / Entity Bomb Prevention):** `encoding/xml` is configured without external entity resolution. XML token decoding is bounded.
- **Size Caps & Timeouts:** All HTTP response streams are wrapped in `io.LimitReader(resp.Body, 5<<20)` (5 MiB cap). In-flight requests execute under an explicit timeout budget (10 seconds for probes/health-checks, 30 seconds for feeds).
- **Concurrency Limiting:** An active semaphore caps concurrent outbound OPDS adapter requests at 50. Excessive requests fail fast with `503 Unavailable`.

### Attacker 3: Untrusted Third-Party / Dependency Compromise
*Threats:* Vulnerable third-party crypto, HTTP routing, or XML parsing libraries.
*Evaluation:* **Minimalist & Audited.**
- **Zero Third-Party Crypto / Network Dependencies:**
  - Key management and credential encryption use Go standard library `crypto/aes`, `crypto/cipher` (AES-GCM 256), `crypto/rand`, and `golang.org/x/crypto/hkdf`.
  - Cursor HMAC signing uses `crypto/hmac` and `crypto/sha256`.
  - In-process semaphore uses standard library channels and atomic counters.
  - Zero third-party ORMs or frameworks.
- **Vulnerability Audit:** Scanned with `govulncheck` and frontend security scripts (`npm run check:dist-secrets`, `npm run check:dist-msw`).

### Attacker 4: Local Multi-User / Host Resource Exhaustion / Filesystem Traversal
*Threats:* Encryption key theft, database file corruption, orphaned offering records during source deletion, permission escalation via keyfile manipulation.
*Evaluation:* **High Assurance Architecture.**
- **Keyfile Permissions:** Master keys in `LoadOrCreateKey` are written with directory `0700` and file `0600` permissions.
- **Atomic Deletion Cascade:** `SourceRemovalService` executes source deletion and associated offerings cleanup within an atomic PostgreSQL transaction, emitting structured domain removal events.
- **Health Check Decoupling:** Failed reachability checks never delete or mutate source configurations. Health state transitions are recorded with specific `SourceHealthDetail` reasons without leaking internal connection errors.

---

## Verification Results

| Test Suite | Command | Result |
|---|---|---|
| Go Domain & Adapter Tests | `go test -v -race ./internal/domain/... ./internal/adapters/sources/...` | **PASS** (100% pass, 0 race conditions) |
| Go Persistence & Crypto Integration | `go test -tags=integration -v ./internal/persistence/postgres/... ./internal/crypto/...` | **PASS** (PostgreSQL sources repo, AES-GCM key rotation, HMAC derivation verified) |
| Go HTTP Transport & Routing | `go test -v ./internal/transport/http/... ./cmd/server/...` | **PASS** (8 source routes, line-1 validation, lazy repos, router wiring verified) |
| OpenAPI Contract Tests | `go test -v ./internal/testutil/contracttest/...` | **PASS** (100% schema alignment with `api/openapi.yaml`) |
| Web Unit & Component Tests | `npm --prefix web test` | **PASS** (71 test files, 355 unit tests passing) |
| Web Lint & TypeScript | `npm --prefix web run lint && npm --prefix web run build` | **PASS** (0 linter errors, 0 type errors, clean Vite build) |
| Web Token & A11y Checks | `npm --prefix web run check:token-styling && npm run check:a11y-tabindex && npm run check:a11y-hidden-text` | **PASS** (Clean) |
| Playwright E2E & A11y Audits | `npx playwright test --config web/playwright.config.ts` | **PASS** (Sources flow, modal a11y, candidate browsing, hostile boundaries passing) |

---

## Conclusion & Stop Gate Readiness

Phase 08 (Sources Management & Ingestion) is fully implemented, verified, and hardened across Tiers 0 through 6. All functional and non-functional requirements defined in `backend-source-adapter.md`, `domain-source.md`, and `frontend-source-management.md` are satisfied.

The implementation is ready for maintainer sign-off and branch merge.
