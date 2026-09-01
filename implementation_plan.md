# Phase 08 — Sources: Implementation Plan

**Status:** Reviewed 2026-08-31 — maintainer approved recommendations for 0.1, 0.2, 0.3 and open questions 2–6. Cleared to start Tier 0. No code written yet.
**Branch:** `feat/phase08-sources`, created from `main` (phase 07 is merged — `main` tree == old `feat/phase07-metadata` tree, verified by empty `git diff`).
**Governing specs:** `backend-source-adapter.md` (APPROVED), `frontend-source-management.md` (APPROVED), `domain-source.md` (APPROVED, phase 02).
**Roadmap:** `.claude/roadmap/08-sources/README.md`.

---

## 0. Resolved before Tier 0

### 0.1 Design-reference conformance — RESOLVED: proceed on the approved spec

**DesignSync pull, 2026-08-31.** Project `Alexandryn interactive prototype`
(`78075626-e444-438f-8437-205d57129a37`, owner Luann). `list_files` →
`Alexandryn Electron.dc.html` fetched and `diff`ed against the local
`.design-reference/Alexandryn-Electron.dc.html`: **byte-identical for the
Sources sections** (list `atSources`, detail `atSourceDetail`, and the
`addOpen` wizard). The design project has not changed since the local
sync; `ANALYSIS.md`'s 2026-08-13 date is still accurate. No re-sync of
`.design-reference/` needed.

What the capture actually draws (verified against the real remote file,
not memory):

- **`atSources`** — card grid; per card: abbr chip, name, `kind` mono
  label, status pill (dot + one-word `state`), URL, `BOOKS` count,
  `LAST SYNC`, "Sync"/"Open" row actions; a "Source activity" feed
  below; a dashed "Add a source" card.
- **`atSourceDetail`** — back link, header + status pill, "Sync now" /
  "Edit" / "···", four stat tiles (`BOOKS INDEXED`, `LAST SYNC`,
  `STORAGE`, `AUTHENTICATION`), an ACTIVITY feed, a CONFIGURATION panel
  (Sync frequency, Metadata sync, Preferred format, On import),
  "Disconnect source".
- **Add-source modal** — a 4-step wizard: (1) source type, **5 kinds**
  (OPDS, Local Folder, Remote Library, Cloud Storage, Custom Provider);
  (2) Catalog URL + Display name + Username/Password + **"Store
  credentials in the system keychain"** toggle + "Advanced settings";
  (3) Test — "Connection successful / Responded in 148 ms" + a
  Protocol/Auth/Entries/Formats table; (4) Configure — Synchronise
  metadata, Sync frequency, Preferred format, On acquire.
- Mock status vocabulary: **2 states only** (`Connected`/ok,
  `Unavailable`/er).

Every point where the capture exceeds or contradicts the approved
`frontend-source-management.md` is either a **deliberately deferred
phase** or an **approach the backend spec already decided against**:

| Capture | Approved spec | Why spec wins |
|---|---|---|
| 5 source kinds | `local-folder` \| `opds` only | roadmap Scope→Out; `ANALYSIS.md` already tracks this "not a classification problem" |
| "Store credentials in the system keychain" toggle | local key-file AES-256-GCM, always, no keychain | `backend-source-adapter.md` FR-13 + its "Local key file over OS-keychain integration" section decided this explicitly (Electron `safeStorage` is Node-only, would need an IPC round-trip per credential op) |
| 4-step wizard, Test step, Configure step (sync freq / metadata sync / preferred format / on-import) | single form; health check auto-runs on create; no sync/format/import config | wizard step 4 is entirely phase 09 (scheduled re-check) + phase 10 (import) |
| "Sync now" / "Last sync" / "Books indexed" / "Storage" / activity feed | health check (reachable/unreachable + `detail`); no counts, storage, sync, or events | phase 09/10; no source event surface this phase |
| 2 status states | 11 states (FR-3) | 11 is a strict superset — richer, fully compatible |
| no browse/search of contents; no HTTPS warning | FR-6 candidate browse+search; FR-5 inline HTTPS warning | spec adds these; capture is simply silent, not contradictory |

**Decision (maintainer, 2026-08-31): proceed on the approved spec.**
The phase-08 screen is a strict subset shape — card grid + 11-state
health + capability badges + single-form add/edit/remove + candidate
browse/search + HTTPS warning — rendered in the capture's visual
language (card shape, mono `kind`/section labels, status pill, stat
tiles) with the deferred surfaces (sync, counts, storage, activity,
wizard, keychain toggle, extra kinds) left out. `ANALYSIS.md` needs no
edit; this plan section is the recorded design-conformance evidence.

### 0.2 Repository naming — RESOLVED: `SourceRecordRepository`

`internal/persistence/postgres/source_repository.go` already exists (T24) implementing the thin `domain.SourceRepository` (id, label, capabilities, kind) used only by `domain.SourceRemovalService`. Phase 08 needs a richer persistent record (config, encrypted credential, health status/detail/timestamp, detected capabilities, validated search-link URL).

A new type in `internal/persistence/postgres/sources.go` — `SourceRecordRepository` — owns the full phase-08 row. The existing `SourceRepository` and `SourceRemovalService` stay untouched for the atomic delete cascade (`domain-source.md` FR-6); the `DELETE` handler composes `SourceRemovalService` for the cascade and then relies on the same `sources` row being gone. Both types read/write the one `sources` table, which migration `00005` extends. No second table for source config.

### 0.3 Component name — RESOLVED: `<SourceCandidateList>`

The brief's Tier 4 says `<SourceCatalogBrowser>`; the approved spec (FR-6) names `<SourceCandidateList>`. **Spec wins** — the component is `<SourceCandidateList>`, with a thin `<SourceCatalogView>` screen-level wrapper if a container is useful.

### 0.4 Open questions 2–6 — RESOLVED (maintainer: "go with your recommendations")

- **SSRF scope (Q2):** implement exactly `backend-source-adapter.md` FR-4/FR-11 — origin-pin every source-supplied URL (cursor, discovered search link) to the configured `baseUrl` origin, disable redirect-following entirely, treat every `3xx` as failure. **No** DNS-based private-range block on the user's *configured* `baseUrl` (a LAN OPDS server is the primary use case and the spec's accepted threat model permits it). Record as an explicit accepted risk in audit `0008`. No dev override flag is built (nothing to override).
- **Cursor signing key (Q4):** derive an HMAC subkey from the credential key file via stdlib `crypto/hkdf` if the pinned Go version has it; otherwise a second random key persisted in the same key-file format. Confirm at implementation.
- **`Resolve` depth (Q5):** real implementation for both providers (local: traversal-checked `os.Open`; opds: origin-pinned authenticated `GET` of the acquisition href), no HTTP surface (spec Non-goals).
- **`created_at` on `sources` (Q6):** added by migration `00005`; existing rows backfill via `DEFAULT now()`.

---

## 1. Architecture overview

```
internal/adapters/crypto/           credential AES-256-GCM service + key file
internal/adapters/sources/
    candidate.go                    SourceCandidate DTO (adapter-owned, not domain)
    provider.go                     Provider interface: List, Search, Resolve, Probe
    origin.go                       same-origin check (FR-11)
    concurrency.go                  global 50-slot semaphore (FR-14)
    cursor.go                       opaque cursor encode/decode (server-signed)
    local/                          local-folder Provider
    opds/                           OPDS 1.2 (Atom) + 2.0 (JSON) Provider
        client.go                   no-redirect HTTP client, size cap, timeout, Basic Auth
        atom.go / atom_test.go      OPDS 1.2 parse → CandidateFeed
        json.go / json_test.go      OPDS 2.0 parse → CandidateFeed
        capability.go               CanSearch detection from root feed
internal/domain/source_credential.go   Credential value type (dual redaction interfaces)
internal/persistence/postgres/sources.go   SourceRecordRepository (full row)
internal/transport/http/sources.go         8 handlers, line-1 validation
```

Domain boundary held: no OPDS/filesystem type enters `internal/domain`. `SourceCandidate` is adapter-owned (mirrors `NormalisedSearchResult` from phase 07). The only `domain` type the adapter constructs is `domain.FileReference` (`domain-source.md` FR-4, needs no `Edition`). `domain.Credential` is a new value type but only for the redaction contract; the ciphertext lives in the persistence layer.

---

## 2. Tier-by-tier plan

Each tier: RED (failing tests) → GREEN → refactor → two independent subagent reviews (`agent-skills:review` + `agent-skills:security-and-hardening` where security-relevant) → `/commit`. Small commits per `incremental-implementation`.

### Tier 0 — Contract, schema, migration, fixtures

**Files**
- `api/openapi.yaml`: add 8 operations under `/api/v1/sources*` with request/response schemas and **inline `application/json` examples** (gen-fixtures requires them). New component schemas: `Source`, `SourceConfig`, `SourceCredentialInput`, `SourceHealth`, `SourceCapabilities`, `SourceCandidate`, `SourceCandidatePage`, `SourceListResponse`. Reuse existing `Error`.
- `internal/persistence/postgres/migrations/00005_phase08_sources.sql` (Goose up/down). Extends `sources`:
  - `config_base_path TEXT NOT NULL DEFAULT ''`
  - `config_base_url TEXT NOT NULL DEFAULT ''`
  - `credential_ciphertext BYTEA` (nullable — null = no credential)
  - `credential_nonce BYTEA` (nullable)
  - `health_status TEXT NOT NULL DEFAULT 'unknown'` (`unknown|reachable|unreachable`)
  - `health_detail TEXT` (nullable, closed vocab from FR-6)
  - `health_checked_at TIMESTAMPTZ` (nullable)
  - `search_link_url TEXT` (nullable — origin-validated once at health-check, FR-11)
  - `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
  - `can_list/can_search/can_download` already exist; keep, default false.
  - CHECK constraint: `kind IN ('local-folder','opds')` (new rows only; existing test rows use these or none). CHECK: a non-null credential requires `kind='opds'` (FR-1).
  - Down migration drops the added columns.
- `web/scripts/gen-fixtures.ts`: no code change; re-run `npm run mocks:gen-fixtures` to emit fixtures from the new examples.
- `internal/testutil/contracttest/contract_test.go`: extend the route-completeness table so every `/api/v1/sources*` path has a registered handler and vice-versa.

**RED tests**
- Contract route-completeness test fails (paths declared, no handlers yet).
- `schema_integration_test.go`: assert `00005` applies and the new columns/constraints exist; assert `down` then `up` round-trips.
- `check-dist-*` unaffected.

### Tier 1 — Adapters & providers (backend)

#### 1a. Credential crypto (`internal/adapters/crypto/`)

- `crypto.go`: `Service` with `Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error)` and `Decrypt(ciphertext, nonce []byte) ([]byte, error)` — AES-256-GCM, `crypto/aes` + `crypto/cipher` + `crypto/rand`. No new dependency (constitution §9; spec's dependency-justification section).
- `keyfile.go`: `LoadOrCreateKey(path string, hasCredentialedSources func() (int, error), logger *slog.Logger) ([]byte, error)` — FR-13's exact startup logic:
  - key file present → read, verify 32 bytes.
  - absent + `hasCredentialedSources()==0` → generate silently, write `0600`.
  - absent + `>0` → `logger.Warn` with the affected **count only** (never labels), generate replacement `0600`.
  - `0600` set at creation (`os.OpenFile(path, O_CREATE|O_EXCL|O_WRONLY, 0o600)`).
  - Key path: `filepath.Join(userConfigDir, "alexandryn", "source-credentials.key")` — same directory family as `pgdata` (`cmd/server/spawn.go:128` precedent). `userConfigDir` injected for tests.

**RED tests** (`crypto_test.go`, `keyfile_test.go`)
- Encrypt→Decrypt round-trip returns original.
- Tampered ciphertext / wrong key → GCM auth-tag error (not silent garbage).
- Wrong nonce → error.
- Key file created with mode `0600` exactly (stat check).
- Missing key + zero credentialed sources → silent generate, no warn line (capture `slog` output).
- Missing key + N credentialed sources → one `warn` line containing `N`, **not** containing any source label; new key generated.
- Corrupt key file (wrong length) → error, no panic.

#### 1b. `domain.Credential` value type (`internal/domain/source_credential.go`)

- `Credential{ Username, Password string }` — unexported fields, constructor `NewCredential(user, pass string)` validating non-empty + bounded length + no control chars.
- Implements `slog.LogValuer`, `json.Marshaler`, `fmt.Stringer` → all return `"[redacted]"` (mirror `config.RedactedString`; spec cites `backend-errors-and-logging.md` FR-8's dual-interface mechanism, "a future credential" is this case).
- `Reveal() (user, pass string)` — the single deliberate accessor.

**RED tests** (`source_credential_test.go`)
- `slog` of a struct embedding `Credential` never contains the real password (both single-attr and whole-struct paths — FR-8's dual concern).
- `json.Marshal` of same never contains it.
- `%v` / `%s` never contains it.
- `Reveal()` returns the real values.
- Empty username/password/control chars rejected.

#### 1c. Shared adapter pieces (`internal/adapters/sources/`)

- `candidate.go`: `SourceCandidate{ Title string; Author *string; FileReference domain.FileReference; CoverURL *string }`; `CandidateFeed{ Items []SourceCandidate; NextPageURL string }` (adapter-internal); `CandidatePage{ Items []SourceCandidate; NextCursor *string }` (returned to transport). De-dup by upstream entry id within a page (FR-9, mirrors phase 07 review 0034 #17).
- `provider.go`: `Provider` interface —
  ```go
  type Provider interface {
      Probe(ctx context.Context) ProbeResult                      // FR-6 + FR-5
      List(ctx context.Context, cursor string, limit int) (CandidatePage, error)   // FR-7, FR-15
      Search(ctx context.Context, q, cursor string, limit int) (CandidatePage, error) // FR-8
      Resolve(ctx context.Context, ref domain.FileReference) (io.ReadCloser, error)  // FR-15 (stub OK this phase — no HTTP surface, but interface + local impl real)
  }
  type ProbeResult struct { Status string; Detail *string; Capabilities domain.SourceCapabilities; SearchLinkURL *string }
  ```
- `origin.go`: `SameOrigin(configured, candidate string) bool` — scheme+host+port compare after `url.Parse`; rejects non-http(s); used by cursor decode and capability detection (FR-11).
- `concurrency.go`: `Semaphore` = buffered `chan struct{}` size 50, `TryAcquire() bool` (non-blocking, immediate `false` when full → transport maps to `503`), `Release()`. One global instance wired in `cmd/server`. (FR-14 — immediate reject, never queue.)
- `cursor.go`: opaque cursor = base64(`HMAC-SHA256(key, payload) || payload`) where payload is `{sourceID, kind, position}` — position is last-seen filename (local) or the **already-origin-validated** next-page URL (opds). Decode re-checks HMAC + re-checks origin for opds before returning. Signing key: reuse the credential key or a derived subkey (HKDF) — decision: derive with `hkdf` from the same key file via `crypto/hkdf` (Go 1.24 stdlib) to avoid a second key file. **Flag:** if stdlib `crypto/hkdf` unavailable in the pinned Go version, fall back to a second random key in the same file format; will confirm Go version at implementation.

**RED tests** (`origin_test.go`, `concurrency_test.go`, `cursor_test.go`)
- `SameOrigin`: same host/scheme/port true; different host, different scheme, different port, `file://`, `http://169.254.169.254`, credential-in-url all false.
- Semaphore: 50 acquires succeed, 51st `TryAcquire()` returns false; after `Release()` the next succeeds.
- Cursor: tampered payload → decode error; valid round-trips; opds cursor whose embedded URL is off-origin → decode error even with valid HMAC.

#### 1d. Local-folder provider (`internal/adapters/sources/local/`)

- `local.go`: constructed with a **resolved real `basePath`** (`filepath.EvalSymlinks` once at construction). 
- `Probe`: `os.Stat` basePath → `path-not-found` / `path-not-readable` / reachable; caps `CanList=true, CanDownload=true, CanSearch=false` (FR-5).
- `List`: `os.ReadDir`, filter to regular files with a supported ebook extension (`.epub .pdf .mobi .azw3 .cbz .cbr .fb2 .djvu` — closed list, unsupported skipped), sort by name, page by cursor (last-seen filename). For each file: join+clean, `filepath.EvalSymlinks`, prefix-check resolved path is still under resolved basePath; **skip silently** (log `warn`, host-free) if outside or broken link (FR-12). Build `SourceCandidate` with `FileReference{ReferenceID: filename, Format: ext, SizeBytes: &size}` — filename is opaque, never re-concatenated from cursor.
- `Search`: returns `409`-equivalent sentinel error (`CanSearch=false`).
- `Resolve`: real — open the traversal-checked path, return the file handle (phase 10 consumer; TOCTOU residual accepted per FR-12).
- `basePath` **shape** validation (`ValidatePath`) — separate, used by transport at create/update: absolute; no control chars; length ≤ host `PATH_MAX` (Linux 4096) / accept Windows UNC `\\server\share` shape (FR-3 amendment for review 0049 finding 6). Non-existence / unreadable is **not** a shape error — it's a health-check failure.

**RED tests** (`local_test.go`) — table-driven against `t.TempDir()`
- Lists supported files sorted; skips unsupported extensions; skips subdirectories.
- Symlink whose name is inside basePath but target resolves outside → **omitted**, not error; `warn` logged without the target path.
- Broken symlink → omitted.
- Lexical `../` in a returned name is impossible (names come from `ReadDir`) but a crafted filename containing `..` is still treated as opaque and never escapes.
- Pagination: cursor resumes at the right filename, no overlap, no gap; final page `NextCursor==nil`.
- `ValidatePath`: relative path rejected; control-char path rejected; > 4096 chars rejected; UNC shape accepted; `/mnt/does-not-exist` passes shape (health check will fail later).
- `Probe`: missing dir → `path-not-found`; unreadable dir (chmod 000) → `path-not-readable`; good dir → reachable + caps.

#### 1e. OPDS provider (`internal/adapters/sources/opds/`)

- `client.go`: `http.Client` with `CheckRedirect: func(...) error { return http.ErrUseLastResponse }` (FR-11 — redirects disabled entirely), 5s timeout, 5 MiB `io.LimitReader` cap, `Authorization: Basic` header always attached when a credential is configured (FR-4 — no "try without auth first"). Every outbound call goes through the global `Semaphore` (FR-14) — `503` on full. A `3xx` status → `http-3xx-unsupported` (probe) / `Unavailable` (browse/search).
- `atom.go`: OPDS 1.2 parser. `encoding/xml` `Decoder` with `Strict=true`, **no custom `Entity` map**, `d.CharsetReader` restricted to UTF-8/US-ASCII only. Go's `encoding/xml` does not process `<!DOCTYPE>`/`<!ENTITY>` (no DTD support) → Billion Laughs / external-entity XXE structurally impossible; still assert this with fixtures. Additional bounds: max 5 MiB input (already capped upstream), max token depth (reject pathological nesting), max entry count per page (e.g. 500). Extract: feed `<link rel="self">`, `rel="next"`, `rel="search"` (→ OpenSearch description URL); per `<entry>`: `<title>`, `<author><name>`, acquisition link (`rel` prefix `http://opds-spec.org/acquisition`) → `FileReference` (href as opaque ReferenceID, `type` attr → format), `<link rel="http://opds-spec.org/image">` → coverUrl. Normalise into `CandidateFeed`. No raw XML type escapes.
- `json.go`: OPDS 2.0 parser. `encoding/json` into a bounded struct. `metadata`, `links` (`self`/`next`/`search` templated), `publications[].metadata.{title,author}`, `.links[]` acquisition-typed → `FileReference`, `.images[]` → coverUrl. `navigation`/`groups` handled (navigation feed → items are sub-feed links, but for browse we only surface `publications`; a pure-navigation feed yields an empty page + the nav links are not candidates this phase — matches "list a directory's files" scope). Same 500-item cap.
- `capability.go`: `DetectCapabilities(rootFeed)` — `CanList=true`, `CanDownload=true`, `CanSearch = rootFeed advertises rel="search" AND that search URL is same-origin as baseUrl` (FR-11 — off-origin search link → `CanSearch=false`, link discarded, not stored). Returns the validated search URL for persistence.
- `probe.go`: `GET baseUrl`, expect 2xx, body parseable as Atom or 2.0 JSON, run capability detection, classify failures into the FR-6 closed vocabulary. `http-4xx` on a source **with a credential** → `auth-rejected`; credential-decrypt failure also → `auth-rejected` (FR-13, intentional ambiguity).

**RED tests** (`atom_test.go`, `json_test.go`, `client_test.go`, `capability_test.go`) — recorded fixtures under `testdata/`
- OPDS 1.2: real navigation feed, real acquisition feed, feed with duplicated `<entry><id>` → de-duped, feed with `<!DOCTYPE ... <!ENTITY lol ...>>` billion-laughs payload → parsed without expansion / rejected, no memory blowup; feed with external entity (`SYSTEM "file:///etc/passwd"`) → not resolved.
- OPDS 2.0: real feed with `publications`, real navigation feed, templated search link, duplicated publication identifier → de-duped, deeply-nested JSON → bounded/rejected.
- `client`: redirect response (302 to same-origin and to off-origin) → **never followed**, treated as failure; 5 MiB+1 body → capped/`Unavailable`; slow response → context timeout → `timeout`; `Authorization` header present on every request when credential set; absent when not.
- `capability`: root feed with same-origin `rel="search"` → `CanSearch=true` + URL returned; off-origin `rel="search"` → `CanSearch=false`, URL not returned; no search link → `CanSearch=false`.
- `probe`: 200 valid → reachable + caps; 401/403 with credential → `auth-rejected`; 404 no credential → `http-4xx`; 500 → `http-5xx`; 302 → `http-3xx-unsupported`; garbage body → `unparseable-response`; connection refused → `connection-refused`; hang → `timeout`.

### Tier 2 — Persistence (`internal/persistence/postgres/sources.go`)

- `SourceRecordRepository` methods:
  - `Create(ctx, rec SourceRecord) error`
  - `List(ctx) ([]SourceRecord, error)`
  - `Get(ctx, id) (SourceRecord, error)` → `NotFound`
  - `Update(ctx, id, patch SourcePatch) error`
  - `UpdateHealth(ctx, id, status, detail, checkedAt, caps, searchLinkURL) error` — the health-check writeback
  - Delete handled via `SourceRemovalService` (Tier 3 wiring) — this repo exposes no `Delete`.
- `SourceRecord` carries `credential_ciphertext`/`credential_nonce` as `[]byte`; encrypt/decrypt happens in the **transport/service layer** with `crypto.Service`, not here — the repo only stores bytes (matches how `config.RedactedString` is revealed only at the Postgres boundary, inverted).
- Parameterized SQL only (`$1…`), snake_case columns, `TranslateError`.
- All operations honor `executorFrom(ctx, pool)` so they compose with `Transactor` (the delete cascade).

**RED tests** (`sources_integration_test.go`, real Postgres via `backend-test-harness.md` harness — `//go:build integration`)
- Create → Get round-trips every field including caps and null credential.
- Create with credential bytes → Get returns identical bytes → decrypt (with the test key) yields the original username/password (**encryption round-trip through the DB**, acceptance criterion).
- `List` returns all, stable order (by `created_at`).
- `Update` label/config; `UpdateHealth` transitions `unknown→reachable→unreachable` and updates `checked_at`, caps, search link.
- Concurrent `UpdateHealth` on the same row from N goroutines → last-writer-wins, no deadlock, row consistent (FR — "concurrent health probe updates").
- `SourceRemovalService.Remove` against this row + a `source_offerings` row → both gone atomically; a `library_entries` row for the same edition survives (reuses existing `source_removal_atomicity_integration_test.go` pattern).
- `kind` CHECK rejects `'ftp'`; credential-on-local-folder CHECK rejects.

### Tier 3 — HTTP transport (`internal/transport/http/sources.go`)

Handlers, all with **line-1 validation** (constitution §4), correlation ID via `CorrelationIDFromContext`, `WriteError(w, category, msg, id)`:

| Route | Validation (line 1) | Notes |
|---|---|---|
| `POST /api/v1/sources` | body ≤ N KiB; `label` bounded per domain; `kind ∈ {local-folder,opds}`; `config` shape per kind; `credential` only if `opds`; `basePath` shape (FR-3) / `baseUrl` http(s) (FR-4) | runs synchronous health check (FR-1); source persisted even if unhealthy; `201` |
| `GET /api/v1/sources` | none | `hasCredential: bool`, never echo credential (FR-2) |
| `GET /api/v1/sources/{id}` | `id` non-empty, UUID shape | `404` |
| `PATCH /api/v1/sources/{id}` | id shape; partial body; credential update = full replace (FR-2) | re-runs health check |
| `DELETE /api/v1/sources/{id}` | id shape | `SourceRemovalService.Remove` (atomic cascade); `204` |
| `POST /api/v1/sources/{id}/health-check` | id shape | `{status, checkedAt, detail}`; writes back caps + search link |
| `GET /api/v1/sources/{id}/browse` | id shape; `limit ∈ [1,50]` default 20; `cursor` opaque | `Provider.List`; cursor origin-validated before fetch (FR-11); `503` on decrypt-fail / semaphore-full / upstream `3xx` |
| `GET /api/v1/sources/{id}/search` | id shape; `q` ≤ 200, no control chars; `limit ∈ [1,50]`; `cursor` | `409` if stored `CanSearch=false` (FR-8, not empty list); `503` same as browse |

- A `lazy.go`-style `LazySourceRecordRepository` wrapper over `PoolRef` (matches phase 07 `NewLazyMetadataCacheRepository`).
- Credential encryption/decryption at this layer: on `POST`/`PATCH`, `crypto.Service.Encrypt(cred)` → store bytes; on `browse`/`search`/`health-check`, decrypt to build the `Authorization` header, `503`/`auth-rejected` on failure (FR-13).
- Health-check transition logging at `info` with `source id` + `detail` only; SSRF-rejected cursor / traversal-rejected filename at `warn` (Observability section). Never the credential, never the raw upstream body, never the full path.

**RED tests** (`sources_test.go`, `httptest`)
- Each route's line-1 rejections (`400` with `{code,message,correlationId}`): bad kind, non-http baseUrl, credential on local-folder, `limit=0/51/abc`, `q` >200 / control chars, empty id.
- `POST` with unreachable OPDS (fake server 500) → `201` with `health.status=unreachable`, `detail=http-5xx`.
- `GET` list → `hasCredential:true` but no username/password anywhere in the body (assert on raw JSON).
- `PATCH` credential replace → old credential not returned; health re-run.
- `DELETE` → `204`; second `DELETE` → `404`.
- `browse` with a cursor whose embedded URL is off-origin → `400` before any HTTP call (inject a Provider spy asserting zero calls).
- `browse` when semaphore full → `503`.
- `search` on `CanSearch=false` source → `409`, not `200 []`.
- Correlation ID from request context propagates into every error body.
- A captured `slog` buffer over a full `POST`+`browse` cycle contains no username/password and no full basePath.

**Wiring** (`cmd/server/main.go` `newProductionRouter` + `repositories.go`)
- Construct `crypto.Service` from `keyfile.LoadOrCreateKey(...)` at startup; `hasCredentialedSources` closure queries the repo.
- Construct the global `sources.Semaphore` (50).
- Register the 8 routes.
- `newRepositories`: add `sourceRecords SourceRecordRepository`.
- Contract route-completeness test now passes.

### Tier 4 — Frontend

**Data layer** (`web/src/data/sources.ts`)
- Types mirroring the wire: `Source`, `SourceHealth` (11 states), `SourceCapabilities`, `SourceCandidate`, `SourceCandidatePage`.
- `useSources()` (list), `useSource(id)`, `useCreateSource()`, `useUpdateSource()`, `useDeleteSource()`, `useHealthCheck(id)` (mutation → `POST .../health-check`), `useSourceBrowse(id)` / `useSourceSearch(id, q)` via `useInfiniteQuery` on the opaque `nextCursor` (`frontend-library-screens.md` FR-3 precedent). Query keys: `['sources']`, `['sources', id]`, `['sources', id, 'browse']`, `['sources', id, 'search', q]`.

**Components**
- `<SourceStatusBadge status detail>` — maps all 11 states (FR-3) to distinct plain copy + a colored dot. Copy verbatim from FR-3 (constitution §11 — no apology, no `!`).
- `<SourceList>` — card grid (design language from the capture: `--sf` card, mono kind label, pill status badge, capability badges shown only when true). Each card → `/sources/:id`. "Add source" button. Polite `aria-live` region announcing health-status changes and list updates (§7).
- `<SourceFormDialog>` — focus-trapped dialog (`frontend-component-primitives.md` dialog primitive). `label` text; `kind` `<SegmentedControl>` (`local-folder`|`opds`); kind-specific:
  - local-folder: text path input + "Browse…" button rendered **only** when `typeof window!=='undefined' && 'alexandryn' in window` (FR-4), calling `window.alexandryn.source.pickLocalFolder()` (already in the IPC surface — `electron/src/shared/operations.ts`).
  - opds: URL input + "Requires a username and password" toggle → credential sub-form (`username`, `password type=password autocomplete="new-password"`). When `baseUrl` not `https://` and sub-form open → inline non-blocking warning "This source doesn't use HTTPS. Your password will be sent unencrypted." (FR-5). Edit mode: "Password set" static + "Replace" → fresh empty sub-form; never pre-fill.
- `<SourceCandidateList>` — infinite-scroll list of `SourceCandidate` reusing the cover-tile + title/author primitives (`frontend-component-primitives.md`, `frontend-generated-covers.md`), **not** `<DiscoverResultGrid>` / `<WorkGrid>` (FR-6 reasoning). Search input rendered only when `CanSearch` (debounced 300ms).
- Remove action → confirm dialog naming `label` (FR-7).

**Screens** (`web/src/screens/Sources/`)
- Replace the `hostOnly('sources', 'Sources')` / `('sources','Source')` placeholders in `web/src/app/routes.tsx` with real `<Sources>` and `<SourceDetail>` (keep the `RequireCapability` wrapper — `sources` is host-only).

**Mocks** — MSW handlers shaped to the generated fixtures (tier-a) for `/api/v1/sources*`; a mock `window.alexandryn.source.pickLocalFolder` for tests.

**RED tests** (Vitest + Testing Library)
- `<SourceStatusBadge>`: each of the 11 states renders distinct, non-generic copy (snapshot the 11 strings).
- `<SourceFormDialog>`: "Browse…" absent when `window.alexandryn` undefined, present when defined; pick fills the field; cancel leaves it unchanged; picker rejection → inline note, field still editable.
- HTTPS warning shows only for non-`https` baseUrl with sub-form open.
- Edit flow: no DOM node ever contains a previously-entered password (the structural test the spec demands).
- `useInfiniteQuery` pages via `nextCursor`; search hidden when `CanSearch=false`.
- Remove confirm names the label; failed remove keeps the dialog open with inline error.
- Storybook stories for each component (states matrix).

### Tier 5 — E2E, a11y, hostile boundary

- `web/e2e/sources.app.spec.ts` (Playwright, fake backend + fake `window.alexandryn`):
  - Add local-folder source → see "Reachable" → browse its contents.
  - Add OPDS source with credential → see "Reachable" → search it.
  - Replace credential → old value never displayed.
  - Remove with confirmation.
  - Axe-core WCAG AA audit on: source list, add-source form, credential sub-form, each health-status state, browse view.
- `web/e2e/hostile-boundary.app.spec.ts` (extend existing):
  - `browse` with a hand-crafted off-origin cursor → `400`, no navigation.
  - `search` on a `CanSearch:false` source → `409` surfaced as a distinct state, not empty list.
  - Credential never appears in any network request the client can observe on a GET.
  - Health-detail copy for `auth-rejected` shown for both wrong-password and (simulated) key-loss.

### Tier 6 — Audit, review, PR

- `.claude/audits/0008-phase08-sources.md` from `.claude/templates/audit.md` — Four-Attacker pass:
  - **Malicious LAN client:** unvalidated mutations, path traversal in `basePath` / filenames, shell injection (none — no shell), `limit`/`q` bounds, SQL injection (parameterized).
  - **Hostile source:** SSRF via cursor / search link / redirect (FR-11), XXE / billion laughs (FR — `encoding/xml` no-DTD), decompression bomb (no auto-decompression; if `Accept-Encoding: gzip` set, cap decoded size), unbounded feed pages (500-item cap, 5 MiB cap), `169.254.169.254` / private ranges (origin check + no redirect; note: origin check is against the **configured** baseUrl, so a user configuring `baseUrl=http://169.254.169.254` is out of scope per FR-4's accepted threat model — **call this out explicitly in the audit**, and consider whether to add an opt-in `ALEXANDRYN_ALLOW_PRIVATE_SOURCE_HOSTS` dev flag per the brief's "local development override flag"; see open question below).
  - **Supply chain:** zero new runtime deps (crypto stdlib, xml stdlib, json stdlib) — record in PR.
  - **Local multi-user:** key file `0600`, credential ciphertext only in DB, `Reveal()` the sole plaintext path, no plaintext in logs (proven by test).
- Two independent subagent reviews (`agent-skills:code-reviewer`, `agent-skills:security-auditor`).
- `/code-review-and-quality` + `/security-and-hardening`.
- Local CI: `golangci-lint run`, `go test ./... -race`, `go test -tags integration ./...`, `npm run build`, `npm test`, `npm run test:e2e`, contract tests, `check-a11y-*`.
- Mark both specs `IMPLEMENTED` → after audit + maintainer gate, `VERIFIED`.
- `/make-pr` → PR against `main` with the dependency-justification section and the audit link.
- **Second maintainer gate** (constitution Review gates): after the audit, before the phase closes — report and wait.

---

## 3. Open questions for the reviewer — ALL RESOLVED 2026-08-31 (see §0.1–§0.4)

_Kept for the reasoning trail; every item below was accepted as recommended._

1. **0.1 design discrepancy** — proceed on approved spec (recommended) or pause for re-capture?
2. **SSRF scope of the private-IP override.** The brief names a "local development override flag" for SSRF (allow loopback/private OPDS hosts). `backend-source-adapter.md` FR-4/FR-11 only require **origin-pinning** (cursor/search-link must match the configured `baseUrl`'s origin) and **no redirects** — it does **not** require blocking a user from configuring `baseUrl=http://192.168.1.10:8080` (a real self-hosted OPDS case). The spec's accepted threat model explicitly permits it. **Proposal:** implement exactly what the spec says (origin-pin + no redirects), do **not** add a DNS-based private-range block on the configured `baseUrl` (it would break the primary use case — a LAN OPDS server), and record this as an explicit accepted risk in audit 0008. The brief's stricter SSRF language (`127.0.0.0/8`, `10.0.0.0/8`, metadata IP) would then apply only to **source-supplied** URLs (cursors, search links), which the origin-pin already covers since the configured base is itself LAN. Confirm this reading.
3. **Repository name** (0.2) — `SourceRecordRepository` acceptable, or prefer another name / merging with the existing thin repo?
4. **Cursor signing key** — derive from the credential key via stdlib HKDF (preferred, no second file) vs. a second random key in the key file. Will confirm Go version supports `crypto/hkdf`; fallback noted.
5. **`Resolve` depth this phase** — spec FR-15 formalises it for phase 10. Plan: real implementation for local-folder (trivial, traversal-checked open), real for OPDS (authenticated GET of the acquisition href, origin-pinned), but **no HTTP endpoint** exposes it (spec Non-goals). Confirm that's the right amount to build now vs. a stub.
6. **`created_at` on `sources`** — adding it to the existing table; existing test rows will get `now()` via default. Acceptable.

---

## 4. Test-first commit sequence (summary)

1. `test(contract): declare /api/v1/sources* paths + schema, route-completeness RED`
2. `feat(db): migration 00005 phase08 sources columns` (+ schema integration test)
3. `feat(crypto): AES-256-GCM credential service + 0600 key file` (RED→GREEN)
4. `feat(domain): Credential value type with dual redaction` (RED→GREEN)
5. `feat(sources): candidate DTO, origin check, semaphore, opaque cursor` (RED→GREEN)
6. `feat(sources): local-folder provider with symlink-safe traversal` (RED→GREEN)
7. `feat(sources): OPDS 1.2 Atom parser, XXE-safe` (RED→GREEN)
8. `feat(sources): OPDS 2.0 JSON parser` (RED→GREEN)
9. `feat(sources): no-redirect HTTP client, probe, capability detection` (RED→GREEN)
10. `feat(persistence): SourceRecordRepository` (+ integration tests)
11. `feat(http): /api/v1/sources* handlers with line-1 validation` (RED→GREEN)
12. `feat(server): wire crypto, semaphore, source routes`
13. `feat(web): sources data layer + hooks`
14. `feat(web): SourceStatusBadge, SourceList, SourceFormDialog, SourceCandidateList`
15. `feat(web): Sources + SourceDetail screens, route wiring`
16. `test(e2e): sources E2E + a11y + hostile boundary`
17. `docs(audit): 0008 phase08 four-attacker audit`
18. `docs: mark phase 08 specs IMPLEMENTED`
