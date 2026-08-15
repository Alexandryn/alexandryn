# Spec: Backend file extractors

| | |
|---|---|
| **Status** | `APPROVED` (independent review, findings fixed, maintainer signed off 2026-08-15) |
| **Phase** | `10-import` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-15 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0037`](../reviews/0037-phase10-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time (7 Blocking across the batch, each found by one pass — the two passes' Blocking findings were complementary, not overlapping; 11 Major, 6 Minor, 2 Nit), all findings fixed; approved by maintainer 2026-08-15 |

## Context

`backend-source-adapter.md` (phase 08) resolves a `FileReference` into
a byte stream (`Provider.Resolve`, that spec's own Non-goals) but never
parses what's inside those bytes — parsing is explicitly this phase's
job. This spec is the first place in the project where file *content*
(not a filename, not a catalog response, not a credential) is
untrusted input, per constitution §4's own checklist and this phase's
roadmap risk table. `domain-bibliographic.md`'s `Work`/`Edition` field
shapes (title, subtitle, publisher, ISBN, language) are this spec's
extraction target — the same field-level validators
`backend-metadata-adapter.md` already reused for Open Library data
apply here identically, since a string is a string regardless of which
external source produced it.

Container formats, confirmed against their own specifications:

- **EPUB** (2 and 3) is a zip archive. A required, uncompressed-and-
  first `mimetype` entry contains the literal bytes
  `application/epub+zip`. `META-INF/container.xml` (XML) points to a
  root OPF file via a `<rootfile full-path="...">` element. The OPF's
  own `<metadata>` block uses Dublin Core elements: `<dc:title>`,
  `<dc:creator>` (repeatable, one per author), `<dc:identifier>`
  (repeatable — an ISBN is one distinguished by an `opf:scheme="ISBN"`
  attribute in EPUB2 or an `identifier-type` refinement in EPUB3),
  `<dc:language>`, `<dc:publisher>`, `<dc:description>`. A cover image
  is referenced either by an EPUB2 `<meta name="cover"
  content="<manifest-item-id>">` or an EPUB3 manifest `<item
  properties="cover-image">`.
- **PDF** has no simple text structure — an object model with an xref
  table and a trailer pointing to a `/Info` dictionary (`/Title`,
  `/Author`, `/Subject`, `/Keywords`) and, in newer PDFs, XMP metadata
  (an embedded XML packet, Dublin Core-based, in a
  `/Metadata` stream). No sane hand-parsing is possible; this spec uses
  a library (Architecture/dependency justification below).
- **CBZ** is a zip archive of image files (no formal specification —
  a de facto convention), read in filename-sorted order as pages. An
  optional `ComicInfo.xml` sidecar entry (the ComicRack-originated,
  still-widely-used convention) carries `<Title>`, `<Series>`,
  `<Writer>`, `<Number>` and similar fields when present.

## Problem

Nothing exists yet to determine what format a resolved file actually
is (never trusting its filename or extension), extract title/author/
ISBN/language/cover-reference from it, or defend against a file
deliberately shaped to exhaust memory/disk or write outside an
intended directory during that extraction.

## Goals

- Content-sniffed format detection — EPUB, PDF, CBZ — never trusting a
  filename or extension alone
- Metadata extraction per format into one shared `ExtractedMetadata`
  DTO
- Zip-bomb, zip-slip, oversized-file, and malformed-container defences,
  each with its own concrete mechanism and test, not inferred from
  "we used a library so it's handled"

## Non-goals

- Reading/rendering the file for a user — phase 11; this spec extracts
  metadata only, never renders page content
- Matching extracted metadata against the library or Open Library —
  `backend-import-pipeline.md`'s job
- Any format beyond EPUB, PDF, CBZ — `domain-source.md` FR-4's
  `FileReference` carries a declared format; an unrecognised or
  unsupported one is a named failure mode (below), not silently
  attempted
- Extracting a *complete* page count for CBZ (the image-entry count is
  a reasonable proxy, used as one, but this spec makes no claim of
  matching a dedicated comic-metadata tool's exact page-counting rules
  for edge cases like cover-only sidecar images)

## User stories

- As **`backend-import-pipeline.md`'s import job**, I want a resolved
  file's bytes turned into structured metadata, so I have something
  real to match against the library and Open Library.
- As **the maintainer**, I want a maliciously crafted EPUB/PDF/CBZ file
  to fail cleanly — a specific, logged failure category — rather than
  exhaust memory, fill disk, or write outside an intended directory.

## Functional requirements

- **FR-1** `Materialize(ctx, r io.Reader) (*os.File, error)` — the
  mandatory first step for every call into this package, run before
  either `DetectFormat` or `Extract` below: the resolved file's raw
  bytes are spooled to a temp file (`os.CreateTemp`, removed when the
  caller's `context` is done) through an `io.LimitReader` capped at
  **250 MiB** — a hard ceiling on the *raw, as-received* byte count,
  independent of and prior to FR-3/FR-5's 200 MiB *decompressed-content*
  cap. This exists because Go's `archive/zip.NewReader` requires an
  `io.ReaderAt` with a known size to parse the central directory at
  all — there is no way to enumerate or count zip entries without
  first having the whole raw file available, so a raw-size cap MUST
  be enforced before that parse ever happens, not assumed covered by
  FR-5's later, decompressed-content-only checks. A raw stream
  exceeding 250 MiB aborts with `ErrOversized` (the same category
  FR-5 uses — the caller doesn't need to distinguish "raw too big"
  from "decompressed too big," both mean "reject this file"), and
  nothing downstream (`DetectFormat`, `Extract`) ever sees more than
  250 MiB of raw data, however that data is shaped internally (a
  bloated central directory, deeply nested structures, or a genuine
  zip bomb) — this is what actually closes the "expensive to merely
  enumerate" risk a hostile zip's own metadata could otherwise pose,
  not FR-5's entry-count check alone, which itself only runs after
  this cap has already bounded what there is to enumerate.
- **FR-2** `DetectFormat(f *os.File) (Format, error)` — takes FR-1's
  already-capped temp file, never a raw, unbounded stream. `Format` is
  `epub` | `pdf` | `cbz` | `unknown`, determined entirely from content:
  a `%PDF-` byte prefix is `pdf`; a valid zip whose **physically first
  local file header** (not merely its first central-directory entry,
  which an adversarial zip can order differently) is named `mimetype`,
  stored (not deflated), with contents exactly `application/epub+zip`,
  is `epub`; a valid zip that is not `epub` and whose entries are
  majority image files (`.jpg`/`.jpeg`/`.png`/`.gif`/`.webp` by
  content-type sniff, `net/http.DetectContentType` on each entry's
  leading bytes only — never decompressing a full entry just to
  classify it — not by the entry's own name) is `cbz`; anything else —
  an unparseable zip, a zip with no clear image-majority, neither zip
  nor `%PDF-` — is `unknown`. Enumerating entries for this
  classification is itself bounded by FR-5's 10,000-entry cap, checked
  first, before any per-entry sniffing — `DetectFormat` and `Extract`
  share this cap, it is not `Extract`-only. The `FileReference`'s own
  declared format (`domain-source.md` FR-4) is used only as a hint for
  which detector to try first (an optimisation, trying the claimed
  format before falling back to the others), never as the actual
  answer — a source lying about a file's format (a `.epub`-named file
  that's actually something else entirely) is caught here, not
  trusted.
- **FR-3** `Extract(ctx, format Format, f *os.File) (ExtractedMetadata,
  error)` — also takes FR-1's capped temp file, dispatches to the
  matching format's extractor. Every extractor MUST additionally
  respect a shared **200 MiB** total-*decompressed*-read cap
  (constitution §4's size limit, applied here — a reasoned placeholder
  for a book/comic file, flagged in Open questions) and a **30-second**
  timeout via `ctx`, regardless of what any container-internal size
  field claims — this cap is enforced by counting actual decompressed
  bytes read as extraction proceeds (FR-5), layered on top of FR-1's
  raw-byte cap, not a replacement for it.
- **FR-4** `ExtractedMetadata` — the shared output DTO, matching
  `domain-bibliographic.md`'s field-level validation discipline
  (`backend-metadata-adapter.md`'s own reused pattern, Domain model
  below): `title` (string, required — extraction fails per-file if
  absent, FR-8), `authors` ([]string, may be empty), `isbn` (nullable
  string, ISBN-10/13 shape-validated, `domain-bibliographic.md` FR-2's
  own rule reused), `language` (nullable, BCP-47), `publisher`
  (nullable string), `description` (nullable string), `coverBytes`
  (nullable `[]byte`, size-capped independently at **10 MiB** — a
  cover image inside a container has no reason to be larger than that,
  a tighter bound than the whole-file cap), `format` (the detected
  `Format`, FR-2). A field failing `domain-bibliographic.md`'s own
  construction-time validation (oversized, control characters) is
  dropped from the result, not a whole-extraction failure — the same
  per-field degradation `backend-metadata-adapter.md` FR-4 already
  established for a structurally identical problem (external data,
  some fields malformed, most fields fine).
- **FR-5** **Zip-bomb defence** (EPUB, CBZ — both zip-based),
  decompressed-content half: every entry is decompressed through a
  streaming reader wrapped in a byte-counting limiter set to FR-3's
  200 MiB cap — `io.Copy` into an `io.LimitReader`-wrapped destination,
  aborting with a specific `ErrOversized` the instant the cap is
  exceeded, never after fully decompressing into memory first. This
  holds regardless of the zip central directory's own claimed
  uncompressed size for any entry — a crafted entry claiming a small
  size while actually decompressing to gigabytes is caught by this
  streaming count, not by trusting the header. A zip with more than
  **10,000** entries is rejected outright before reading any entry's
  *content* (`ErrTooManyEntries`, FR-2's own shared check) — a second,
  independent zip-bomb variant (many small entries rather than one
  huge one). Together with FR-1's raw-byte cap (the third, prior
  layer — bounding the file *before* any of this defence even has
  something to decompress), this spec has three independent bounds:
  raw size, entry count, decompressed size per entry — no single one
  alone is treated as sufficient.
- **FR-6** **Zip-slip defence** (EPUB, CBZ): no entry is ever written
  to a real filesystem path constructed from its own name — every
  extractor reads entries directly into memory (metadata extraction
  needs no filesystem staging at all, unlike a hypothetical "unzip
  this EPUB to disk" operation this spec doesn't perform). This
  closes the traversal risk structurally, by never constructing a
  path from an untrusted entry name in the first place, rather than by
  validating one after construction (`backend-source-adapter.md`
  FR-12's resolve-then-check pattern, which this spec doesn't need
  because it has no equivalent write step to defend).
- **FR-7** **Malformed-container defence**: an unparseable zip central
  directory, a `META-INF/container.xml` that doesn't parse as XML or
  has no `<rootfile>`, an OPF that doesn't parse as XML, or a PDF whose
  library-level open fails, all map to a single `ErrMalformed` — never
  a panic, never an unhandled parser error propagating past this
  spec's boundary. Every parse call — `archive/zip.NewReader`/
  `OpenReader` and iterating its entries, **not only** the XML and PDF
  library calls — MUST be wrapped so a panic inside a parsing library
  (a genuine risk with hostile input against any parser, including
  Go's own `archive/zip` against a sufficiently adversarial archive)
  is recovered and converted to `ErrMalformed`, the same recover-and-
  convert discipline `backend-job-queue.md` FR-6 already established
  for handler panics.
- **FR-8** A file that parses successfully as its container format but
  has no extractable `title` (a required `ExtractedMetadata` field,
  FR-4) is a distinct failure, `ErrNoTitle` — not folded into
  `ErrMalformed`, since a structurally valid EPUB/PDF/CBZ with no title
  metadata is a real, different case a caller (`backend-import-pipeline.md`)
  needs to handle differently. `backend-import-pipeline.md` FR-2
  treats `ErrNoTitle` as a permanent failure (`job.Permanent`), the
  same as `ErrMalformed`/`ErrOversized`/`ErrTooManyEntries` — this
  spec deliberately doesn't invent a filename-derived fallback itself,
  since `domain-source.md` FR-4's `FileReference` is an opaque,
  `Source`-scoped identifier with no guaranteed human-readable
  filename to fall back to.
- **FR-9** EPUB extraction reads `META-INF/container.xml`, resolves the
  OPF path from its `<rootfile>` element (validated as a path *inside*
  the zip's own entry namespace — never resolved against a real
  filesystem, so no traversal risk applies to this specific resolution
  the way FR-6 covers actual file writes), parses the OPF's
  `<metadata>` block per Context's Dublin Core field mapping, and
  resolves the cover reference (EPUB2 `<meta name="cover">` or EPUB3
  `properties="cover-image"`) to the matching manifest entry's bytes,
  capped per FR-4's 10 MiB cover bound.
- **FR-10** CBZ extraction looks for a `ComicInfo.xml` entry first; if
  present and parseable, `<Title>`/`<Writer>` map to `title`/`authors`
  (a comma-split on `<Writer>`, since that field is conventionally a
  comma-separated list, not a repeated element the way EPUB's
  `dc:creator` is). If `ComicInfo.xml` is absent or unparseable
  (`ErrMalformed` is NOT raised for this specific case — a missing
  sidecar is a normal, common state, not a malformed container),
  `title` falls back to `nil`, triggering FR-8's `ErrNoTitle`. The
  first image entry in filename-sorted order, capped per FR-4's cover
  bound, is used as `coverBytes`.
- **FR-11** PDF extraction opens the file via the chosen library
  (Architecture decisions below) with FR-3's byte-cap/timeout applied
  to the library's own read, and maps `/Info` dictionary fields
  (`/Title`, `/Author`) preferentially over XMP metadata when both are
  present and disagree (the `/Info` dictionary is simpler and more
  universally populated; XMP is used only to fill fields `/Info`
  lacks, e.g. a `/Subject`-absent PDF with XMP `dc:description`). No
  cover extraction for PDF this phase — a PDF's "cover" would mean
  rendering its first page as an image, a materially heavier operation
  than the zip-based formats' simple embedded-image lookup, and out of
  scope here (Non-goals, implicitly — flagged explicitly in Open
  questions instead, since it's a real gap worth naming, not silently
  omitted).

## Non-functional requirements

- **Performance** — FR-1's 250 MiB raw cap and FR-3's 200 MiB/30s
  decompressed-content bounds apply per file,
  independent of how many files a batch import processes concurrently
  (concurrency itself is `backend-import-pipeline.md`'s job-queue-level
  concern, not this spec's).
- **Security** — see dedicated section below.
- **Accessibility** — not applicable; backend-only.
- **Reliability** — a single file's extraction failure (any error
  category above) MUST NOT affect any other file's extraction; each
  call is independent, stateless, and reentrant.
- **Observability** — every extraction failure logs at `info` with the
  detected/attempted `Format` and the error category (`ErrOversized`/
  `ErrTooManyEntries`/`ErrMalformed`/`ErrNoTitle`), never the file's
  own content or the source filename (constitution §8, this phase's
  "what someone is importing" extension already named in the roadmap).

**Dependency justification (constitution §9)**: EPUB and CBZ extraction
use only Go stdlib (`archive/zip`, `encoding/xml`, `net/http`'s content
sniffing) — both are well-defined zip-based containers this spec parses
directly, the same reasoning `backend-source-adapter.md` already used
for OPDS's Atom feeds. **PDF extraction uses `github.com/pdfcpu/pdfcpu`**
— what it does: a pure-Go PDF processing library including metadata
extraction, with no `cgo` dependency (unlike PDF libraries that shell
out to or link against `poppler`/`mupdf`, which would introduce a
non-Go runtime dependency this project has avoided everywhere else).
Why not stdlib: PDF's object model, xref table, and stream/filter
structure (`FlateDecode` and friends) have no reasonable hand-rolled
subset — unlike EPUB/CBZ's simple zip-plus-XML shape, a "just parse the
`/Info` dictionary" hand implementation would still need a real PDF
tokenizer/object-graph reader to locate that dictionary safely against
a hostile file, which is most of what a PDF library actually is. What
breaks if abandoned: `pdfcpu` is actively maintained at the time of
writing and Apache-2.0 licensed; if abandoned, this spec's PDF
extraction surface is narrow (metadata only, not the library's full
manipulation feature set), so a replacement library or a scoped-down
hand implementation targeting only `/Info`-dictionary extraction
remains a bounded, achievable fallback — not a rewrite of this whole
spec.

## Domain model

No new domain types. `ExtractedMetadata` (FR-4) is a wire/internal DTO
following the same DTO-not-aggregate pattern
`backend-metadata-adapter.md`'s Domain model section already
established — this spec never constructs a `domain-bibliographic.md`
`Work`/`Edition` directly; it reuses that spec's field-level validators
(length bounds, control-character rejection, ISBN/BCP-47 shape rules)
against DTO fields, exactly as `backend-metadata-adapter.md` already
does for Open Library data. `backend-import-pipeline.md` is what
eventually constructs a real domain `Work`/`Edition` from this DTO,
once matching and (where required) user confirmation decide to.

## API and contracts

No HTTP surface — this is a pure Go package
(`internal/import/extract`, alongside `internal/jobs` in
`architecture-backend.md` FR-1's layout), called directly by
`backend-import-pipeline.md`'s job handler. Public functions:
`DetectFormat`, `Extract`, both above.

## State transitions

Not applicable — every call is a stateless, single-file operation with
no persisted state of its own; `backend-import-pipeline.md` owns
whatever state tracks an individual file's import progress.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Raw resolved file exceeds 250 MiB, before any parsing | `io.LimitReader` during spooling (FR-1) | `ErrOversized` | Aborts mid-spool; `archive/zip.NewReader` never called against an unbounded stream |
| Zip decompresses past 200 MiB (zip bomb) | Streaming byte-count limiter (FR-5) | `ErrOversized` | Aborts mid-stream, no full decompression ever attempted |
| Zip has more than 10,000 entries | Entry count check before reading any entry's content (FR-2) | `ErrTooManyEntries` | Rejected before any entry is read; shared by `DetectFormat` and `Extract` |
| Zip/XML/PDF fails to parse | Parser error or recovered panic (FR-7) | `ErrMalformed` | No partial state leaked; every parser call, including `archive/zip`'s own, wrapped in `recover()` |
| Valid container, no extractable title | Field absent after successful parse (FR-8) | `ErrNoTitle` | `backend-import-pipeline.md` treats this as a permanent failure |
| Unrecognised format | Content-sniff finds no match (FR-2) | `unknown` `Format`, no extraction attempted | Caller surfaces this as its own distinct case, not a malformed-file error |
| Extraction exceeds the 30-second timeout | `ctx` deadline | `context.DeadlineExceeded`, mapped by the caller | Extraction goroutine abandoned; no retry inside this spec (a job-level concern, `backend-job-queue.md`'s own retry mechanism) |

## Security considerations

- **Zip bomb (FR-1, FR-2, FR-5)** — this phase's own sharpest named
  risk, closed in three independent layers: a raw-byte cap enforced
  *before* `archive/zip` ever parses a central directory (FR-1,
  closing the "expensive just to enumerate" gap a bloated central
  directory could otherwise exploit), an entry-count cap checked
  before any entry's content is read (FR-2), and streaming decompressed-
  byte counting independent of any claimed size header (FR-5). No
  single layer is treated as sufficient alone.
- **Zip slip (FR-6)** — closed structurally by never writing an entry
  to a constructed filesystem path at all, not by validating a path
  after construction — no traversal is possible because no traversal-
  capable operation exists in this spec.
- **Malformed input causing a crash (FR-7)** — every parser call,
  including `archive/zip`'s own, wrapped in `recover()`; a crafted
  file that would otherwise panic a parsing library (a real, common
  vulnerability class against hostile files, and one Go's own stdlib
  `archive/zip` is not immune to against a sufficiently adversarial
  archive) degrades to a clean `ErrMalformed`, never takes down a
  worker goroutine.
- **Resource exhaustion beyond zip-specific attacks** — the 200 MiB/
  30s decompressed-content bounds (FR-3) apply uniformly to every
  format, including PDF via `pdfcpu`, layered on top of FR-1's raw
  cap, which also applies uniformly (PDF's own `%PDF-`-prefixed bytes
  are spooled and capped the same way before the library ever opens
  them).
- **No filesystem or network access from this package** — every
  extractor operates purely on an in-memory/streamed `io.Reader`
  argument the caller supplies (already resolved via
  `backend-source-adapter.md`'s own `Provider.Resolve`); this spec
  never itself reaches a source, a source's credential, or a real
  filesystem path.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Format detection against real valid fixtures per format and against deliberately mislabeled ones (a `.epub`-named PDF, etc.); metadata extraction against real, valid EPUB/PDF/CBZ fixtures including one with and one without `ComicInfo.xml`; each defence proven with a purpose-built hostile fixture — a zip bomb (small compressed size, huge decompressed size), a >10,000-entry zip, a zip with a `../`-shaped entry name (proving FR-6 never constructs a path from it regardless), a truncated/corrupted zip and PDF, a valid container with no title field, and a raw file just over 250 MiB (proving FR-1's cap rejects before `archive/zip.NewReader` is ever called) |
| Integration | N/A — this spec has no external dependency (database, network) to integrate against; covered entirely at the unit layer |
| Contract | N/A — no HTTP surface |
| E2E | Covered by `backend-import-pipeline.md`'s own E2E, not duplicated here |
| Accessibility | N/A |

Tests that must fail before implementation begins: a test asserting a
zip bomb (real compressed-small/decompressed-huge fixture) returns
`ErrOversized` without the process's memory usage spiking, proven by
asserting the read stops well before full decompression; a test
asserting a zip entry named `../../../etc/passwd` never results in any
filesystem write, by asserting no `os.Create`/`os.OpenFile` call
happens anywhere in the extraction path (a structural test, not a
behavioural one — this spec's own FR-6 design means there's nothing to
behaviourally observe going wrong, so the test proves the *absence* of
the write capability, matching the "closed structurally" claim); a
test asserting a hostile file that would panic a naive parser
(malformed nested XML, a truncated PDF object stream) returns
`ErrMalformed`, not a crash.

## Acceptance criteria

- [ ] EPUB, PDF, and CBZ files each extract correct metadata from real,
      valid fixtures
- [ ] Format detection never trusts a filename/extension over content
- [ ] A zip bomb is rejected without full decompression
- [ ] A >10,000-entry zip is rejected before reading content
- [ ] No extraction path ever constructs a filesystem write from an
      untrusted entry name
- [ ] A malformed file of each format returns `ErrMalformed`, not a
      crash
- [ ] A file with no extractable title returns `ErrNoTitle`, distinct
      from `ErrMalformed`

## Open questions

- **200 MiB whole-file cap, 10 MiB cover cap, 30-second timeout,
  10,000-entry cap** — reasoned placeholders sized for "a very large
  book/comic, not a video file," not load-tested against real import
  volume; revisit once phase 10 has real usage data.
- **No PDF cover extraction** — named as a real gap (Functional
  requirements, FR-11), not silently omitted; a future revision could
  add first-page rendering, a materially heavier operation this spec
  deliberately doesn't take on now.
- **CBZ page-count accuracy** — the image-entry count is a working
  proxy, not a guarantee of matching a dedicated comic-reader's own
  page-counting conventions for edge cases (a sidecar-only cover image
  not meant to count as a page, for instance).

## References

- `domain-bibliographic.md` (phase 02) — field-level validators reused
  for `ExtractedMetadata`
- `domain-source.md` (phase 02) FR-4 — the `FileReference`'s declared
  format, used only as a detection hint (FR-2), never trusted
- `backend-source-adapter.md` (phase 08) — `Provider.Resolve`, the
  byte stream this spec's callers pass in; FR-12's path-traversal
  precedent, distinguished from this spec's structural (no-write)
  approach (FR-6)
- `backend-metadata-adapter.md` (phase 07) — DTO-not-aggregate pattern,
  per-field degradation pattern, both reused directly (FR-4)
- `backend-job-queue.md` (phase 09) FR-6 — panic-recovery precedent,
  reused for parser panics (this spec's own FR-7)
- `architecture-backend.md` (phase 01) FR-1 — package layout
  `internal/import/extract` sits within
- Constitution §4 (hostile input, this phase's first file-content
  instance), §8 (redaction, extended to "what someone is importing"),
  §9 (dependency justification for `pdfcpu`)
