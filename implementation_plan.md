# Phase 10 — Import Pipeline: Implementation Plan

**Status (2026-09-01):** Ready for Maintainer Approval. Specs `backend-file-extractors.md`, `backend-import-pipeline.md`, and `frontend-import-confirmation.md` are all `APPROVED`. ADR `0022` scoped for PDF extraction library (`pdfcpu`).

**Branch:** `feat/phase10-import` from clean `main` (commit `24b70a8`).

**Governing Documents:**
- `CLAUDE.md` and `.claude/constitution.md` (Constitution §1, §2, §3, §4, §7, §8, §9, §10, §11, §12)
- `.claude/specs/backend-file-extractors.md` (`APPROVED`, amended for review `0049`)
- `.claude/specs/backend-import-pipeline.md` (`APPROVED`, amended for review `0049`)
- `.claude/specs/frontend-import-confirmation.md` (`APPROVED`)
- `.claude/roadmap/10-import/README.md`
- `.claude/decisions/0003-design-canvas-split.md` (Design reference check)
- `.claude/decisions/0021-transaction-contract-and-event-outbox.md` (Transactor persistence)

---

## 0. Design Reference & Review Gate Conformance

### Design Reference Conformance (ADR 0003 check)
- **Canvas consulted:** `Alexandryn-Electron.dc.html` (`atImport` at lines 904–1050), last synced 2026-08-13 (`ANALYSIS.md`).
- **Classification:** `atImport` is classified as `Binding`.
- **Divergence / Context:** `ANALYSIS.md` notes: *"atImport: Phase 10 names it — premise contradicted by frontend-import-confirmation.md's Source-gated design, tracked separately"*. `frontend-import-confirmation.md` (approved spec) specifies the source-gated design: "Import from this source" button on `/sources/:id` triggering discovery and navigating to `/import?sourceId=:id` where pending candidates are listed as cards with match comparisons, in-flight progress polling, and distinct failed cards.
- **DesignSync tool:** Not configured/available in this environment; proceeding against local `.design-reference/` per task instructions.

### Review 0049 Amendments Check
All five Phase 10 findings from review `0049` are confirmed incorporated into the approved specs:
1. `backend-import-pipeline.md` FR-4: Three confusable-input shapes (parenthetical qualifier, same-year/series volume collision, standalone colliding with series name) capped at `"medium"` confidence.
2. Multi-file reading unit recorded as an Open Question (not silently assumed).
3. `backend-file-extractors.md` FR-9/FR-10: Undecodable UTF-8 zip entry names -> `ErrMalformed` for OPF in EPUB, drop page in CBZ.
4. `backend-file-extractors.md` FR-2/FR-10: Exclude `__MACOSX/` and dotfiles before classification and page ordering.
5. `domain-bibliographic.md` FR-6 & `backend-file-extractors.md` FR-8: Empty/whitespace-only title treated as `ErrNoTitle`.

### ADR 0022: PDF Library Choice (`pdfcpu`)
- **Library:** `github.com/pdfcpu/pdfcpu` (pure Go, Apache-2.0).
- **Constitution §9 Justification:**
  - *What it does:* Pure Go PDF extraction (trailer `/Info` dictionary and XMP stream extraction).
  - *Why not stdlib:* Go standard library has no PDF package. Unlike zip+XML (EPUB/CBZ), PDF requires object graph parsing, xref tables, and stream filters (FlateDecode).
  - *Why not cgo (Poppler/MuPDF):* Avoids non-Go C runtime dependencies, preserving cross-compilation and self-contained binary distribution.
  - *Mitigations:* Bounded input (250 MiB raw spool limit, 200 MiB decompressed limit, 30s timeout, panic recovery wrapped in `ErrMalformed`).

---

## 1. Tier Breakdown & Commit Sequence

Each tier strictly follows `RED → GREEN → REFACTOR` (TDD, Constitution §2). Integration tests carry `//go:build integration`. Commits are conventional and atomic with no AI attribution trailers.

### Tier 0 — Planning, ADR, Schema Migration & Persistence
1. `docs: add phase 10 import pipeline implementation plan`
2. `docs(decisions): add 0022 pdf metadata extraction library adr` — ADR `0022` in `.claude/decisions/` and updated `.claude/decisions/README.md`.
3. **RED/GREEN**: `00007_phase10_import.sql` migration + schema integration tests in `internal/persistence/postgres/schema_integration_test.go`:
   - Table `import_candidates` with `id TEXT PRIMARY KEY`, `source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE`, `file_reference JSONB NOT NULL`, `status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','pending','auto_imported','confirmed','rejected','failed'))`, `extracted_metadata JSONB`, `match_candidates JSONB`, `job_id TEXT`, `last_error TEXT`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`.
   - Index `import_candidates_source_status_idx ON import_candidates (source_id, status)`.
   - Reversible down migration `DROP TABLE import_candidates;`.
   - Commit: `feat(persistence): add import candidates table migration`.
4. **RED/GREEN**: `import_candidate_repository.go` in `internal/persistence/postgres/` with integration tests (`Create`, `Get`, `List(sourceID, status)`, `UpdateStatus`, `UpdateExtractedAndMatches`, `UpdateLastError`, `ExistsBySourceAndFileRef`).
   - Commit: `feat(persistence): add import candidate postgres repository`.

### Tier 1 — File Format Detection & Extractors (`internal/importer/extract`)
1. **RED/GREEN**: Domain & extractor types, errors (`ErrOversized`, `ErrTooManyEntries`, `ErrMalformed`, `ErrNoTitle`), `ExtractedMetadata` DTO with validation.
   - Commit: `feat(extract): add extractor types, errors, and metadata dto`.
2. **RED/GREEN**: `Materialize(ctx, r)` spooling up to 250 MiB in temp file with streaming limit.
   - Commit: `feat(extract): add stream materializer with 250mib raw cap`.
3. **RED/GREEN**: Content-sniffed `DetectFormat(f)`:
   - `%PDF-` prefix -> `pdf`.
   - Physically first local zip file header `mimetype` with uncompressed `application/epub+zip` -> `epub`.
   - Entry count <= 10,000; filter `__MACOSX/` and dotfiles; majority image content-type sniff -> `cbz`.
   - Unit tests against real files and intentionally misnamed files.
   - Commit: `feat(extract): add content-sniffed format detection`.
4. **RED/GREEN**: EPUB extractor:
   - `META-INF/container.xml` -> rootfile OPF -> Dublin Core metadata -> cover image resolution up to 10 MiB.
   - Undecodable UTF-8 `<rootfile>` target -> `ErrMalformed`.
   - Empty/whitespace-only title -> `ErrNoTitle`.
   - Commit: `feat(extract): add epub metadata and cover extractor`.
5. **RED/GREEN**: CBZ extractor:
   - `ComicInfo.xml` -> Title/Writer; absent -> `ErrNoTitle`.
   - Exclude `__MACOSX/` and dotfiles; filter undecodable UTF-8 entry names; first image cover up to 10 MiB.
   - Commit: `feat(extract): add cbz extractor with comicinfo support`.
6. **RED/GREEN**: PDF extractor:
   - `pdfcpu` integration with `/Info` dictionary and XMP fallback.
   - Commit: `feat(extract): add pdf metadata extractor`.
7. **RED/GREEN**: Adversarial defences, each with dedicated hostile fixture and test:
   - Zip bomb (small file expanding > 200 MiB) -> `ErrOversized` without memory spike.
   - Zip slip (`../../` entry names) -> proven no filesystem writes.
   - Oversized raw stream (> 250 MiB) -> `ErrOversized`.
   - > 10,000 entries zip -> `ErrTooManyEntries`.
   - Malformed XML / truncated PDF / corrupted container -> `ErrMalformed` via panic recovery.
   - Commit: `test(extract): prove adversarial file defences with hostile fixtures`.

### Tier 2 — Discovery & Matching Engine (`internal/importer`)
1. **RED/GREEN**: Matching confidence scoring:
   - Exact ISBN lookup joined through `LibraryEntry`. Exactly 1 hit -> `exact`. >1 hit -> separate `exact` candidates (no auto-accept).
   - Open Library search scoring: `"high"` (close title + author), `"medium"` (close title, author differs/absent), `"low"` (neither).
   - Confusable-input caps: parenthetical qualifier differences, same-year/series volume collisions, standalone vs series collision capped at `"medium"`.
   - Unit tests covering all scoring cases and confusable fixtures.
   - Commit: `feat(importer): add bibliographic matching and confidence scoring`.
2. **RED/GREEN**: Auto-accept rule & domain data persistence:
   - Auto-accept iff exactly one `"exact"` ISBN match to an existing owned `Edition`.
   - Auto-accept creates `SourceOffering` (with verified content-sniffed `Format`) and `LibraryEntry` via `Transactor`.
   - Proves via tests that auto-accept NEVER creates new `Work`/`Edition`/`Author` domain data.
   - Commit: `feat(importer): add narrow auto-accept and transaction persistence`.

### Tier 3 — Import Job Handler (`internal/importer`)
1. **RED/GREEN**: Job Handler for kind `"import"`:
   - One job per discovered file.
   - Step sequence: Load candidate -> `Provider.Resolve` -> `Materialize` -> `DetectFormat` -> `Extract` -> ReportProgress (`resolving`/`extracting`/`matching`) -> Matching -> Auto-accept or set `pending`.
   - Permanent errors (`ErrOversized`, `ErrTooManyEntries`, `ErrMalformed`, `ErrNoTitle`) wrapped in `jobs.Permanent(err)`, sets candidate `status = 'failed'`.
   - Transient `Provider.Resolve` error returns ordinary error for job retry.
   - Commit: `feat(importer): add import job handler for job queue`.
2. **RED/GREEN**: Integration tests with fake provider and real postgres:
   - Multi-file batch where one bad file fails/dead-letters while sibling jobs complete successfully.
   - Commit: `test(importer): prove job handler resilience and batch isolation`.

### Tier 4 — HTTP Surface & Server Wiring (`internal/transport/http`, `cmd/server`)
1. **RED/GREEN**: Import HTTP endpoints:
   - `POST /api/v1/import/discover`: synchronously calls `Provider.List`, deduplicates against existing `SourceOffering` and `import_candidates` (in any status), creates `import_candidates` rows (`status: 'queued'`), enqueues one job per item, returns `{ queued: N }`.
   - `GET /api/v1/import/candidates`: lists candidates with optional `sourceId` and `status` filters in camelCase wire format.
   - `POST /api/v1/import/candidates/:id/confirm`: actions `attach_existing`, `use_open_library_match`, `create_new`. Validates state (409 on non-pending), creates domain data in transaction, sets `status: 'confirmed'`.
   - `POST /api/v1/import/candidates/:id/reject`: sets `status: 'rejected'` (409 on non-pending).
   - Line-1 input validation on all handlers (UUID checks, payload caps, enum validation).
   - Commit: `feat(transport): add import discovery and confirmation http handlers`.
2. **RED/GREEN**: Server wiring:
   - Wire `import` job handler into `cmd/server/main.go` and `cmd/server/run.go` (`jobs.System.Queue().Register`).
   - Thread `*jobs.Queue` and import repositories into `PoolRef` and `newProductionRouter`.
   - Commit: `feat(server): wire import pipeline into service lifecycle and router`.
3. **RED/GREEN**: OpenAPI contract tests:
   - Update `api/openapi.yaml` with `/api/v1/import*` endpoints.
   - Extend `internal/testutil/contracttest` to validate all import endpoints.
   - Commit: `test(contract): extend openapi contract tests to import endpoints`.

### Tier 5 — Frontend Resolution UI (`web/`)
1. **RED/GREEN**: Source Browse View extension (`web/src/pages/SourceDetail.tsx`):
   - "Import from this source" button triggering `POST /api/v1/import/discover` and navigating to `/import?sourceId=:id`.
   - Commit: `feat(web): add import trigger to source browse view`.
2. **RED/GREEN**: TanStack Query hooks & MSW handlers:
   - `useImportCandidates`, `useDiscoverImport`, `useConfirmCandidate`, `useRejectCandidate`.
   - MSW handlers for `/api/v1/import*`.
   - Commit: `feat(web): add import api hooks and msw mock handlers`.
3. **RED/GREEN**: Import Confirmation page (`/import`):
   - In-flight progress indicator ("Processing X of Y...") polling `status=queued`.
   - Pending candidates cards: extracted metadata, cover image (data URL / fallback), suggested matches with visible text confidence labels (`Exact match`, `High confidence`, `Medium confidence`, `Low confidence`).
   - Confirm actions (`attach_existing`, `use_open_library_match`, `create_new`) and "Reject".
   - Distinct "Couldn't be imported" failed section with plain language error and client-side dismissal (`localStorage`).
   - Accessible keyboard controls, focus styles, aria-live updates.
   - Vitest component tests + Axe accessibility scan.
   - Commit: `feat(web): add import confirmation resolution ui`.
4. **RED/GREEN**: Playwright E2E test:
   - Add source -> discover -> auto-accept bypasses UI -> pending candidate reviewed & confirmed -> library reflects new book; failed candidate dismissed.
   - Commit: `test(web): add import resolution e2e flow test`.

### Tier 6 — Verification & Quality Checks
- `go test -race ./...`
- `TEST_DATABASE_URL=... go test -race -tags=integration ./...`
- `golangci-lint run ./...`, `go vet ./...`, `scripts/check-*.sh`
- `npm --prefix web test`, `npm --prefix web run lint`, `npm --prefix web run build`, Playwright test runs.

### Tier 7 — Security Audit `0010`, Spec Documentation, PR & CI
1. Security audit `.claude/audits/0010-phase10-import.md` using `.claude/templates/audit.md` (Four-Attacker + STRIDE).
2. Report audit results to maintainer (Gate 2).
3. Mark specs `backend-file-extractors.md`, `backend-import-pipeline.md`, `frontend-import-confirmation.md` as `IMPLEMENTED`.
4. Update `.claude/audits/README.md`, `.claude/specs/README.md`, tick roadmap criteria in `.claude/roadmap/10-import/README.md`.
5. Open PR against `main` using `make-pr` and monitor CI until green.

---

## 2. Open Questions & Verifications for Maintainer

> [!IMPORTANT]
> 1. **PDF Library Selection:** We propose using `github.com/pdfcpu/pdfcpu` for pure-Go PDF extraction as documented in ADR `0022`.
> 2. **Review 0049 edge cases:** All edge case findings (confusable match scoring caps, UTF-8 entry name decoding handling, `__MACOSX/` filtering, whitespace-only title handling) are planned with dedicated tests.
> 3. **Design Conformance:** As documented in `ANALYSIS.md`, `atImport` in `Alexandryn-Electron.dc.html` serves as visual reference, while the actual interaction is governed by the approved `frontend-import-confirmation.md` source-gated workflow.
