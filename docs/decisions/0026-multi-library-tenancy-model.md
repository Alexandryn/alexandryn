# 0026. Multi-library namespacing aggregate and membership model

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-02 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Prior to Phase 12, Alexandryn operated with an implicit single-library domain model (`LibraryEntry`, `Collection`, `Source`, `SourceOffering`, `ReadingProgress`, `Bookmark`, `Highlight`). While bibliographic metadata (`Work`, `Author`, `Edition`) is universal and immutable, user-owned instances, collections, sources, and reading marks belong to specific library namespaces and users.

Phase 12 requires:
1. Multi-library namespacing allowing users to organize books into distinct named libraries (e.g. "Personal", "Family", "Comics").
2. Explicit user membership and per-library roles (`admin` or `reader`).
3. An automated migration creating a default library containing existing library entries, collections, sources, and source offerings.
4. An architecture that binds `LibraryID` to `Source` and `SourceOffering` so future cloud source connectors (Google Drive, OneDrive, Nextcloud) can be attached per-library without schema refactoring.
5. Scoping of personal reading data (`reading_progress`, `bookmarks`, `highlights`, `reading_preferences`) by `user_id` and `library_id` (resolving audit finding A-11-01).

## Decision

1. **`Library` Aggregate Boundary**
   - We introduce `Library` as a top-level domain aggregate (`domain.Library`).
   - Fields: `id` (UUID), `name` (string), `description` (string), `allowReaderUploads` (boolean), `createdAt` (time.Time), `updatedAt` (time.Time).
   - Invariant: A library name must be non-empty, trimmed, and length bounded (1–100 characters).

2. **Namespacing & Relational Foreign Keys**
   - Universal entities (shared metadata): `works`, `authors`, `editions`.
   - Library-namespaced entities (linked by `library_id REFERENCES libraries(id) ON DELETE CASCADE`):
     - `library_entries` (composite unique on `library_id, edition_id`)
     - `collections`
     - `collection_members`
     - `sources`
     - `source_offerings`
     - `import_candidates`
     - `reading_progress` (composite unique on `user_id, library_id, work_id`)
     - `bookmarks` (composite indexed on `user_id, library_id, edition_id`)
     - `highlights` (composite indexed on `user_id, library_id, edition_id`)
     - `reading_preferences` (composite unique on `user_id, device_id`)

3. **Default Library Auto-Creation & Backfill (Migration 00009)**
   - The migration inserts a canonical default library record with well-known UUID `00000000-0000-0000-0000-000000000001` and name `'Default Library'`.
   - Existing rows in `library_entries`, `collections`, `sources`, and `source_offerings` are backfilled with `library_id = '00000000-0000-0000-0000-000000000001'`.
   - Subsequent `NOT NULL` constraints and foreign keys are applied.

4. **Membership and Invitations**
   - `library_memberships`: Links `(library_id, user_id)` with role `admin` or `reader`.
   - `library_invitations`: Secure single-use invitation tokens created by library admins. An invitation is bound to an email, library ID, target role, and expiration timestamp (e.g. 7 days).
   - The master `admin` user is granted `admin` membership on the default library during initial setup.

5. **Request Context and Header Resolution**
   - Active library context is resolved via `X-Library-Id` HTTP header or route parameters.
   - If `X-Library-Id` is omitted, the backend defaults to the first library in the user's membership list.
   - Authorization middleware verifies that the authenticated user possesses valid membership in the requested `library_id`.

## Options Considered

### Option A: Shared database with explicit `library_id` foreign keys and composite constraints (Chosen)
*For*: Clean single PostgreSQL schema, low operational overhead, straightforward transactional consistency, works seamlessly in desktop (embedded Postgres) and server environments.
*Against*: Requires query-level tenancy filtering (`WHERE library_id = $1`).

### Option B: PostgreSQL Schema-per-Library (multi-schema)
*For*: Strong physical separation of tables per library.
*Against*: Heavy DDL operations on library creation; complex migration runners across dynamic schemas; breaks cross-library metadata sharing with universal `works` and `editions`.

### Option C: Standalone database per library
*For*: Total isolation.
*Against*: Totally disproportionate for local desktop and small team instances.

## Consequences

**Good**:
- Clean tenancy isolation at the query and middleware layer.
- Reading data is securely partitioned per user and library, resolving audit finding A-11-01.
- Future cloud connectors and sources are naturally scoped per library aggregate without schema redesign.

**Bad / Trade-offs**:
- Every query touching library assets must include `library_id` in its parameters.
- Deleting a library cascades to its memberships, entries, sources, collections, and reading data.

## Reversal Cost
Medium. Changing tenancy models would require data migration, but the chosen model is standard for multi-tenant relational systems.
