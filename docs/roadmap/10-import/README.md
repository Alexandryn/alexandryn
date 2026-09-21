# Phase 10 — Import

| | |
|---|---|
| **Status** | Specs approved, implementation not started |
| **Depends on** | Phase 07, Phase 08, Phase 09 |
| **Blocks** | 11 |
| **Opened** | — |
| **Closed** | — |

## Objective

The pipeline that turns files sitting in a source into library
entries: discovery, extraction, matching, user confirmation,
persistence — with every file treated as adversarial (constitution
§4). This is the phase every earlier phase's own deferred design
decisions converge on: `domain-source.md` FR-2's `SourceOffering` is
finally constructed here (no earlier phase creates one);
`backend-metadata-caching.md`'s and `backend-source-adapter.md`'s own
Non-goals both named "connecting a cache/candidate row to a real
domain row" as this phase's job; `domain-bibliographic.md` FR-4's
matching-decision deferral ("a matching decision made elsewhere —
phase 07/10") is closed here.

## Why here

It needs phase 07 (a metadata source to match candidates against),
phase 08 (a source to discover files from), and phase 09 (a place to
run extraction/matching without blocking a request) — all three,
which is why it waits for all three to close rather than any one. It
sits before phase 11 (reader) because reading requires something
real, imported and owned, to read.

## Scope

**In**

- Discovery: enumerate a source's contents via
  `backend-source-adapter.md`'s existing provider abstraction
- Extraction: EPUB/PDF/CBZ metadata extraction from real file bytes,
  content-sniffed, not trusted by filename or extension
- Zip-bomb, zip-slip (path-traversal via a zip entry name), and
  oversized-file defences — the first phase where file *content*, not
  just a filename or a catalog response, is parsed
- Matching: candidate generation against both the existing library
  (exact-identifier lookup) and Open Library (`backend-metadata-adapter.md`),
  with a confidence score
- A closed, narrow auto-accept rule (only when a file exactly matches
  an `Edition` this library already owns) — everything else requires
  explicit user confirmation, never silently created
- Manual resolution UI: review extracted metadata and suggested
  matches, confirm or reject
- Batch import via the phase 09 job queue — one job per discovered
  file, not one job per whole source, so one bad file doesn't block
  the rest
- The concrete point `SourceOffering` (`domain-source.md` FR-2) and
  `LibraryEntry` (`domain-library.md` FR-1) both actually get created

**Out**

- Reading the imported file — phase 11
- Async/scheduled re-discovery of a source's contents — phase 09's own
  scope is the generic job mechanism, not a recurring discovery
  scheduler; phase 14 (per phase 09's own Non-goals) is where periodic
  re-checking lives, if this phase's own on-demand discovery isn't
  sufficient
- OAuth/Authentication-Document-gated sources, non-OPDS/non-local-folder
  protocols — `backend-source-adapter.md`'s own existing scope,
  unchanged here
- Editing a Work/Edition's metadata after import — that's a library-
  management feature no current phase names; import only ever
  *creates*

## Specifications

| Spec | Covers |
|---|---|
| `backend-file-extractors.md` | Format detection, EPUB/PDF/CBZ metadata extraction, adversarial-file defences |
| `backend-import-pipeline.md` | Discovery, the import job handler, matching/confidence scoring, auto-accept rule, confirm/reject endpoints, `SourceOffering`/`LibraryEntry` persistence |
| `frontend-import-confirmation.md` | Resolution UI: pending candidates, suggested matches, confirm/reject, batch progress |

## Architecture decisions expected

- **Extraction library choices per format** — EPUB is a well-defined
  zip+XML container (`META-INF/container.xml` → an OPF file's
  `<metadata>` block, Dublin Core elements); `backend-file-extractors.md`'s
  own case to make for hand-parsing via stdlib `archive/zip` +
  `encoding/xml` rather than a third-party EPUB library, the same
  stdlib-first reasoning `backend-source-adapter.md` already used for
  OPDS's own Atom feeds. PDF has no such simple text structure — a
  pure-Go PDF library is the expected choice there, justified
  concretely under constitution §9, not assumed. CBZ is a zip of
  images with an optional `ComicInfo.xml` sidecar (a real, existing
  convention) — stdlib-parseable, same reasoning as EPUB.
- **Auto-accept rule scope** — this phase's own to fix narrowly:
  leaning toward "only an exact match to an `Edition` this library
  already owns" (attaching a newly-discovered file to a known book),
  never auto-creating new domain data, however confident a match
  looks — matching the roadmap's own explicit naming of a manual
  resolution UI as in-scope, not a fallback for rare cases.
- **Per-file jobs, not per-source jobs** — `backend-import-pipeline.md`'s
  own case to make concretely, following `backend-job-queue.md`'s own
  worked-example reasoning (a job that fails shouldn't block unrelated
  work) rather than assumed here.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| A crafted EPUB/PDF/CBZ file exploits its container format (zip bomb, zip slip, malformed XML) to exhaust memory/disk or write outside an intended directory | Medium | High | `backend-file-extractors.md`'s own dedicated defences — streaming size caps enforced independent of any header claiming a smaller size, path-traversal-safe extraction, named and tested per format, not assumed from "we use a library so it's handled" |
| Matching against Open Library produces a confident-looking but wrong suggestion, and an over-eager auto-accept rule creates duplicate or wrong library data silently | Medium | High | The narrow auto-accept scope (Architecture decisions expected) — nothing that creates new domain data is ever automatic, only attaching a file to a book already unambiguously owned |
| A large source (thousands of files) makes synchronous discovery itself slow enough to matter | Medium | Low | Flagged in Open questions as a candidate for its own job if real usage shows it's a problem; not solved speculatively here |
| Extraction library (PDF specifically) has its own parsing vulnerabilities, being a much larger and more complex format than EPUB/CBZ's zip+XML/zip+images shape | Medium | Medium | `backend-file-extractors.md`'s own size/timeout bounds apply uniformly regardless of format; the library choice itself is weighed under constitution §9 including this exact risk, not just convenience |

## Test strategy

| Layer | Carries |
|---|---|
| Unit | Format detection (content-sniffed, not extension-trusted) per format; metadata extraction against real, valid fixture files per format; each adversarial defence (zip bomb, zip slip, oversized file, malformed XML/PDF) proven with a deliberately hostile fixture, one test per defence |
| Integration | Full discovery → extraction → matching → auto-accept-or-pending flow against a real PostgreSQL instance and a fake source (`backend-test-harness.md`'s harness, `backend-source-adapter.md`'s own fake-provider pattern reused); job queue integration (`backend-job-queue.md`'s own test harness) proving one bad file's job failing/dead-lettering doesn't block sibling jobs |
| Contract | `architecture-contracts.md` FR-3's contract test, extended to `/api/v1/import*` |
| E2E | Add a source → discover → a high-confidence match auto-imports → a low-confidence match appears in the resolution UI → user confirms → library reflects the new entry |
| Accessibility | `frontend-accessibility.md`'s requirements applied to the resolution UI, including its match-comparison and confirm/reject actions |

## Security considerations

The first phase where file *content*, not just a filename, catalog
response, or credential, is untrusted input:

- **Zip-bomb, zip-slip, oversized-file, malformed-container defences**
  — this phase's own sharpest named risk, closed concretely in
  `backend-file-extractors.md`, not assumed handled by "we used a
  library"
- **No new domain data is ever created without explicit user
  confirmation, except the one narrow, unambiguous auto-accept case**
  — a matching-confidence bug degrades to "asks the user unnecessarily
  often," never to "silently pollutes the library," a deliberate
  asymmetry in how this phase fails
- **`SourceOffering`/`LibraryEntry` creation reuses
  `backend-persistence.md`'s existing parameterized-query and
  transaction discipline** — nothing new to design here, restated for
  completeness since this is the first phase actually exercising that
  write path for real imported data

## Observability

Job status/progress (`backend-job-queue.md`'s existing mechanism) is
this phase's primary observability surface — an import job's current
stage (discovering/extracting/matching/awaiting-confirmation/done) is
queryable the same way any job's status already is, with no new
mechanism needed. Extraction/matching failures log per
`backend-errors-and-logging.md`'s existing discipline; a source's
filename or extracted content is never logged verbatim, only that
extraction failed and why (a closed failure-category vocabulary,
`backend-file-extractors.md`'s own to fix), consistent with
constitution §8's "what someone is reading" concern extended to "what
someone is importing."

## Exit criteria

- [ ] All three specifications `APPROVED` with recorded reviews
- [ ] Import functional from both a local-folder and an OPDS source
- [ ] Metadata correctly extracted across EPUB, PDF, and CBZ
- [ ] Each adversarial-file defence rejected safely, proven by its own
      test, not inferred from the others
- [ ] The narrow auto-accept rule never creates new domain data
      automatically, proven by a test
- [ ] The contract test passes against `/api/v1/import*`
- [ ] Test coverage across unit, integration, and E2E for this slice
- [ ] All specs in this phase are `VERIFIED`
- [ ] Security audit recorded in `.claude/audits/` with no open Critical
      or High findings
- [ ] Documentation updated
- [ ] Maintainer approval recorded
