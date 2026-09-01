# 0022. PDF metadata extraction uses `pdfcpu`

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-01 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

`backend-file-extractors.md` requires format detection and metadata extraction
for EPUB, CBZ, and PDF files. While EPUB and CBZ are zip archives containing
standardized XML (`META-INF/container.xml`, Dublin Core OPF, `ComicInfo.xml`)
that can be parsed with Go's standard library (`archive/zip`, `encoding/xml`),
PDF has no simple stream structure.

Extracting PDF metadata (`/Info` dictionary and XMP packets) requires object
graph navigation, cross-reference (xref) table resolution, and stream filter
decoding (e.g. `FlateDecode`). Hand-rolling a PDF parser to avoid a dependency
would introduce substantial complexity and hostile-file parsing risks that
contradict Constitution §9.

## Decision

PDF metadata extraction uses `github.com/pdfcpu/pdfcpu` (pure Go, Apache-2.0).
Extraction is strictly bounded to metadata reading (never page rendering or full
content extraction) wrapped in `recover()` to safely map parser panics to
`ErrMalformed`.

## Options considered

### Option A — `github.com/pdfcpu/pdfcpu` (chosen)

*For* — Pure Go implementation with zero C/cgo dependencies. It supports
reading PDF trailers, `/Info` dictionaries, and XMP metadata streams. Pure Go
preserves cross-compilation and single-binary host distribution without external
runtime dependencies like Poppler or MuPDF. Actively maintained and Apache-2.0
licensed.

*Against* — PDF parsing is inherently complex; adversarial files could trigger
edge-case panics in the parser. Mitigated by wrapping all library calls in
`recover()` to map panics to `ErrMalformed`, enforcing a 250 MiB raw spool cap,
a 200 MiB decompressed limit, and a 30-second execution deadline.

### Option B — Cgo bindings to Poppler / MuPDF (`poppler-glib`, `mupdf-go`)

*For* — High fidelity and extensive battle-testing in mature desktop reader
ecosystems.

*Against* — Requires Cgo, native shared libraries (`libpoppler` or `libmupdf`),
and platform-specific C toolchains. Breaks the project's dependency isolation and
pure-Go cross-compilation model across Linux, macOS, and Windows.

### Option C — Hand-rolled minimal `/Info` dictionary parser

*Against* — PDF syntax allows linearized streams, object streams, incremental
updates, and compressed xref tables. A naive scanner searching for `/Title` byte
patterns will fail on standard valid PDFs and be vulnerable to trivial adversarial
evasion or resource exhaustion.

## Consequences

**Good** — Clean metadata extraction from valid PDFs without requiring Cgo or
external runtime dependencies.

**Bad** — Adds `github.com/pdfcpu/pdfcpu` to `go.mod` (Constitution §9).
- *What it does:* Pure Go PDF object and metadata extraction.
- *Why not stdlib:* Go standard library has no PDF parsing support.
- *What breaks if abandoned:* The extraction surface used is narrow (metadata
  inspection). If abandoned, replacing `pdfcpu` with an alternative Go library
  affects only `internal/importer/extract/pdf.go` behind the `Extract` interface.

**Neutral** — PDF cover extraction remains out of scope for Phase 10 (as page
rendering is heavier and deferred to reader phases).

## Reversal cost

Low. The extractor is encapsulated in `internal/importer/extract/pdf.go` and
returns the internal `ExtractedMetadata` DTO. No `pdfcpu` types leak outside the
extractor package.

## Confidence

High. Evaluated against the project's hostile input rules and pure-Go distribution
model.
