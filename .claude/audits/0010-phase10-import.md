# Security audit: Phase 10 — Import pipeline & file extractors

| | |
|---|---|
| **Scope** | `internal/importer/extract/` (`types.go`, `materialize.go`, `detect.go`, `epub.go`, `cbz.go`, `pdf.go`, `adversarial_test.go`), `internal/importer/` (`matching.go`, `service.go`, `job.go`, `discovery.go`), `internal/persistence/postgres/` (`import_candidate_repository.go`, migration `00007_phase10_import.sql`), `internal/transport/http/` (`import.go`, `import_ref.go`), `cmd/server/` (`main.go`, `run.go`, `repositories.go`), and frontend `web/src/screens/Import/`, `web/src/data/import.ts`. |
| **Auditor** | Claude (Sonnet 5), `agent-skills:security-and-hardening` + `agent-skills:security-auditor` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over each trust boundary |
| **Date** | 2026-09-01 |
| **Commit** | Branch `feat/phase10-import` |
| **Verdict** | **Clear** — no open Critical or High findings. Full `go test -race` unit + integration suites, `golangci-lint`, check boundary scripts, and full web suite pass. |

## Scope and method

Phase 10 builds the complete book import pipeline across all tiers:
1. File format extractors (EPUB Dublin Core + cover, CBZ ComicInfo.xml + cover, PDF Info/XMP via pdfcpu) with adversarial defences against hostile archives and streams.
2. Discovery coordinator that enumerates listable sources, deduplicates against existing candidates and offerings, and enqueues background import jobs.
3. Matching and confidence scoring engine evaluating existing library ISBN hits and Open Library search results.
4. Domain importer service with atomic `AutoImport`, `ConfirmAttachExisting`, `ConfirmCreateNew`, `ConfirmOpenLibraryMatch`, and `Reject`.
5. Background job handler for kind `"import"` reporting 10%/40%/70%/90%/100% progress.
6. HTTP transport surface (`/api/v1/import/discover`, `/api/v1/import/candidates`, `/candidates/{id}/confirm`, `/candidates/{id}/reject`).
7. Frontend resolution UI with candidate review cards, in-flight polling, and `localStorage` dismissal for failed candidates.

Method: Complete code inspection of all files in scope; STRIDE threat modeling across each trust boundary; the Four-Attacker adversarial pass; verification of the 5 hostile file protections via dedicated hostile test fixtures; query parameterisation and transaction boundary audits; log redaction audits; memory and resource bound audits; and dependency vet (`pdfcpu v0.15.0` per ADR 0022).

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made & Defence |
|---|---|---|
| Source file bytes → Extractor (`internal/importer/extract`) | Untrusted raw file stream from local folder or remote OPDS source | Spooled through `BoundedReader` with a hard 250 MiB stream limit (`ErrOversized`). Formats sniffed by magic bytes and entry ratios, never file extension. ZIP entries capped at 10,000 (`ErrTooManyEntries`). Decompressed files capped at 200 MiB (`ErrOversized`). Zip Slip directory traversal paths stripped/sanitized. XML parsing bounded with panic recovery. PDF parsing wrapped with panic recovery and bounded info queries without arbitrary rendering. |
| HTTP API requests (`/api/v1/import/*`) | Client/Browser HTTP requests | Request bodies bounded by HTTP middleware (MaxBodyBytes). Line-1 validation of query parameters (`sourceId`, `status`) and path parameters (`candidateId`). Status values validated against 6-value lifecycle enum. JSON unmarshaling into strict Go structs. |
| `import_candidates` table → Domain | Database rows | Staging table separate from core domain aggregates. Domain entities (`Work`, `Edition`, `Author`, `SourceOffering`, `LibraryEntry`) only mutated within transactional boundaries (`domain.Transactor`). |
| Open Library Client → Matcher | External Open Library API | User-agent header supplied. Responses parsed as JSON into sanitized DTOs. Text fields bounded and control characters rejected. Title-only searches never yield `high` confidence when parenthetical qualifiers exist. |
| Background Job Queue → Importer Service | Job payloads | `json.RawMessage` payload parsed into `ImportJobPayload`. Source resolved via crypto codec/service. Failures classified into retriable vs permanent (`jobs.Permanent`) to avoid worker pool starvation. |

## Hostile file extractors defence analysis

| Hostile Attack Vector | Potential Impact | Implemented Defence | Test Evidence |
|---|---|---|---|
| **Zip Bomb (Decompression Bomb)** | Memory/disk exhaustion, denial of service | `io.LimitReader` on all archive entry reads with a hard 200 MiB limit. Exceeding 200 MiB immediately aborts with `ErrOversized`. | `adversarial_test.go`: `TestZipBomb_AbortsWithErrOversized` (passes with synthetic 1 GB decompressed stream). |
| **Zip Slip (Path Traversal)** | Arbitrary file overwrite | Extractors operate purely in-memory via `zip.Reader` and `BoundedReader`. No files are written to host filesystem paths from archive entry names. Entry names normalized with `filepath.Clean`. | `adversarial_test.go`: `TestZipSlip_HandledSafelyInMemory` (passes with malicious entries like `../../../../etc/passwd`). |
| **Infinite / Gigantic Raw Stream** | Disk exhaustion during spooling | `MaterializeTemp` limits incoming streams to 250 MiB max via `io.LimitReader`. Exceeding 250 MiB returns `ErrOversized` and immediately unlinks temp file. | `adversarial_test.go`: `TestStreamExceedingMaxSpool_ReturnsErrOversized` (passes with 300 MiB synthetic stream). |
| **Zip Entry Flooding (100k+ files)** | CPU/Descriptor exhaustion | Format detection checks total ZIP entry count; if entries > 10,000, extraction fails immediately with `ErrTooManyEntries`. | `adversarial_test.go`: `TestTooManyZipEntries_Rejected` (passes with 10,001 entry archive). |
| **Malformed XML / Billion Laughs** | Parser hang or panic | Standard library `encoding/xml` decoder with panic recovery blocks. EPUB/CBZ extractors recover from parser panics and return `ErrMalformed`. | `adversarial_test.go`: `TestMalformedContainerXML_PanicRecovery` (passes with corrupted XML structures). |
| **Malformed / Corrupted PDF** | Memory corruption or parser crash | `pdfcpu` metadata extraction executed inside a deferred `recover()` block. No rendering, no postscript evaluation. | `pdf_test.go`: `TestExtractPDF_HostileCorruptedHeader` (passes with corrupted PDF headers). |

## Adversarial questions asked (Constitution §10)

**What does a malicious user of this instance achieve here?**
- Cannot craft malicious IDs or status strings to execute SQL injection: all queries are strictly parameterised.
- Cannot mutate candidates not in `pending` status: `Confirm` and `Reject` enforce `status = 'pending'` check in SQL and return `409 Conflict` on race or invalid status.
- Cannot bypass capabilities: `can_list` is verified before discovery.

**What does a malicious source achieve — metadata, redirects, filenames, file contents?**
- Malicious source files (zip bombs, zip slips, malformed XML, infinite streams) are neutralized by the multi-layered extractor defences before any domain entity is created.
- Malicious metadata (strings containing control characters or oversized text) is sanitized and bounded by `extract.NewExtractedMetadata`.
- No untrusted file name is ever used in a shell execution or unvalidated filesystem path.

**What does a device on the local network achieve without credentials?**
- Loopback binding ensures API is only reachable according to configured server interfaces (Constitution §6).

**What happens when input is malformed, empty, enormous, duplicated, slow, or never arrives?**
- *Empty/Malformed metadata*: extraction returns `ErrNoTitle` or `ErrMalformed`, permanently failing the candidate and logging without worker retry loop.
- *Enormous file*: rejected at stream materialization (250 MiB) or decompression (200 MiB).
- *Duplicated file*: discovery checks `ExistsBySourceAndFileRefID` and skips already-tracked files; database unique offering constraints prevent duplicate offering insertion.
- *Slow stream*: standard Go timeouts and context deadlines cancel slow network IO.

**What is logged that shouldn't be?**
- Extracted cover bytes and credential ciphertexts are never logged. Structured log entries only carry correlation IDs, candidate IDs, and source IDs.

**What happens if two of these run at once?**
- Two discovery coordinators on the same source: deduplication query prevents creating duplicate `import_candidates` rows.
- Two confirm mutations on the same candidate: `UPDATE import_candidates ... WHERE id = $1 AND status = 'pending'` ensures exactly one succeeds and the other receives `409 Conflict`.
- Transactional domain mutations: `AutoImport` and `Confirm*` execute within atomic `InTx` transactions.

## STRIDE summary

| Threat | Assessment |
|---|---|
| **Spoofing** | Candidate IDs and job IDs generated via cryptographically secure `idgen` (ULID). Open Library matches verified against Open Library API. |
| **Tampering** | All SQL queries are parameterised. Status transitions enforced via CHECK constraints and SQL guards. Metadata sanitized on extraction. |
| **Repudiation** | Discovery runs and background job executions log structured audit trails with correlation IDs. |
| **Information disclosure** | Cover image bytes in HTTP responses are base64-encoded strings of validated image files. Source credentials remain encrypted in database. |
| **Denial of service** | Stream materialization capped at 250 MiB. ZIP decompression capped at 200 MiB. ZIP entry count capped at 10,000. Background jobs use worker pool concurrency limits (default 4). Outbound network semaphore limits parallel downloads. |
| **Elevation of privilege** | Importer service runs strictly within application privileges. No shell execution, no native C bindings. |

## Findings

- **Finding A-10-01 (Informational — Accepted)**: Extracted cover images rendered as base64 data URLs in candidate DTOs.
  - *Detail*: Candidate DTOs embed extracted cover bytes as base64 string to allow immediate rendering on candidate cards without extra HTTP endpoints.
  - *Risk*: Data transfer size per candidate list response can increase if many pending candidates have large covers.
  - *Mitigation*: Extracted cover images from EPUB/CBZ are sniffed and bounded; candidate pagination bounds response count.
- **Finding A-10-02 (Informational — Accepted)**: pdfcpu library dependency added.
  - *Detail*: `github.com/pdfcpu/pdfcpu v0.15.0` used for PDF metadata extraction.
  - *Risk*: Third-party parser vulnerabilities.
  - *Mitigation*: Justified in ADR 0022; PDF parser runs with panic recovery and bounded extraction without rendering pages.

## Conclusion

Phase 10 import pipeline satisfies all security requirements, architectural invariants, and adversarial defences set out in the constitution and phase specifications.
