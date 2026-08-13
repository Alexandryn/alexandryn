# Phase 10 — Import

*Outline — expanded to a full phase document when phases 07, 08, 09 close.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 07, Phase 08, Phase 09 |
| **Blocks** | 11 |

## Objective

The pipeline that turns files sitting in a source into library entries:
discovery, extraction, Open Library matching, user confirmation,
persistence — with every file treated as adversarial.

## Scope

**In**

- Discovery → extraction → matching → confirmation → persistence pipeline
- EPUB/PDF/CBZ metadata extractors
- Zip-bomb, path-traversal, and oversized-file defences
- Match confidence scoring and manual resolution UI
- Batch import via the phase 09 job queue

**Out**

- Reading the imported file — phase 11.

## Exit criteria

- [ ] Import functional from local and remote sources
- [ ] Metadata correctly extracted across supported formats
- [ ] Hostile files rejected safely, with a test proving it for each defence
- [ ] Maintainer approval recorded
