# Phase 07 — Metadata

*Outline — expanded to a full phase document when phase 06 closes.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 06 |
| **Blocks** | 10 |

## Objective

An Open Library adapter that searches, fetches, normalises, and caches work
and edition metadata, with the normalisation boundary (Constitution §3)
preventing Open Library's response shapes from reaching the domain or the UI
directly.

## Scope

**In**

- Open Library client (Works, Editions, Authors, Covers, Search)
- Normalisation into the phase 02 domain model
- Local caching of metadata and cover images
- Discover screen
- Rate limit, timeout, and partial-data handling

**Out**

- File acquisition — Open Library is metadata only. Custom sources — phase 08.

## Exit criteria

- [ ] Search and lookup functional with local caching
- [ ] No raw Open Library JSON reaches the UI, enforced at the adapter boundary
- [ ] Degrades cleanly offline, rate-limited, or given malformed responses
- [ ] Maintainer approval recorded
