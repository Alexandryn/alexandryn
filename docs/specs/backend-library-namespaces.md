# Spec: Backend Multi-Library Namespaces & Tenancy

| | |
|---|---|
| **Status** | `APPROVED` (Maintainer Gate 1 sign-off 2026-09-02). **Amended 2026-09-02 (`DRAFT` amendments, pending re-confirmation) for phase-13 review `0050`:** FR-3 now requires the auth middleware to validate the resolved library against `claims.Libraries` (P12-4); FR-4 spells out that per-user scoping applies to by-id reads/deletes and to `export`, and that the enforcement point is the handler's call not the repository's capability (AUDIT-0012-C1). The **implementation** on `feat/phase12-auth` does neither — that is the phase-13 hardening prelude and close gate. |
| **Phase** | `12-auth` |
| **Author** | Claude (Sonnet 4.6), approved by Luann Moreira |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-02 |
| **Supersedes** | — |
| **Reviewed in** | Gate 1 Review Batch; phase-13 amendments in `0050` |

## Context

Alexandryn supports multiple libraries (e.g. "Personal", "Work", "Sci-Fi", "Family") running on a single instance. Books, collections, sources, and user reading progress belong to specific library namespaces.

## Domain Model & Aggregates

```
┌────────────────────────────────────────────────────────┐
│                        Library                         │
│  ID: UUID, Name: string, AllowReaderUploads: bool      │
└──────────┬─────────────────┬─────────────────┬─────────┘
           │                 │                 │
           ▼                 ▼                 ▼
   ┌───────────────┐ ┌───────────────┐ ┌───────────────┐
   │ LibraryEntry  │ │  Collection   │ │    Source     │
   │ (Edition, ...)│ │ (Works, ...)  │ │(Offerings, ...)
   └───────────────┘ └───────────────┘ └───────────────┘
```

Personal reading data (`ReadingProgress`, `Bookmark`, `Highlight`, `ReadingPreferences`) is doubly scoped by `(user_id, library_id)`.

## Functional Requirements

- **FR-1: Library Management API**:
  - `GET /api/v1/libraries`: Returns a list of libraries accessible by the authenticated user with their membership role.
  - `POST /api/v1/libraries`: Creates a new named library (requires global `admin` role). Assigns the creator as `admin` member.
  - `GET /api/v1/libraries/:id`: Returns library details and membership count.
  - `PATCH /api/v1/libraries/:id`: Updates library `name`, `description`, or `allow_reader_uploads` (requires library `admin` role).
  - `DELETE /api/v1/libraries/:id`: Deletes the library and cascades to its entries, collections, sources, and memberships (requires library `admin` role). Default library cannot be deleted.
- **FR-2: Library Memberships & Invitations**:
  - `GET /api/v1/libraries/:id/members`: Lists members of the library (requires library `admin`).
  - `POST /api/v1/libraries/:id/invitations`: Creates an invitation for an email with a target role (`admin` or `reader`) and expiration (7 days). Returns invitation token / URL.
  - `POST /api/v1/invitations/:token/accept`: Accepts an invitation using a valid token, adding the authenticated user to the library.
  - `DELETE /api/v1/libraries/:id/members/:userId`: Removes a user's membership from the library.
- **FR-3: Active Library Context Resolution**:
  - Transport resolves the target library in order:
    1. Route parameter (e.g., `/api/v1/libraries/:libraryId/...`).
    2. Header: `X-Library-Id: <uuid>`.
    3. User default library: First library in user's memberships.
  - **The resolved library MUST be validated against the authenticated
    token's `libraries` claim in the auth middleware, before any handler
    runs.** A route parameter or `X-Library-Id` header naming a library
    not in `claims.Libraries` → `403 Forbidden`. (Amended 2026-09-02 for
    review `0050`
    P12-4: the shipped `AuthMiddleware` copies `X-Library-Id` into the
    request context **without this check** — `auth_middleware.go:104`.
    Low impact at loopback, **High** once phase 13 opens the bind. The
    phase-13 hardening prelude adds the check + a handler test;
    `backend-network-transport.md` FR-13 carries the middleware change.)
    The check is applied to **every** authenticated `/api/v1` route, not
    only the reading surface — `X-Library-Id` is the active-library
    selector and a header naming a non-member library is always wrong.
    The **frontend** consequence (PR #78 review, finding 2): a client
    that persists an active library and later loses membership will get a
    `403` on every request until it switches libraries; the SPA MUST
    detect this `Forbidden` code and clear/reset its stored active
    library rather than looping. Owned by `frontend-auth-and-tenancy.md`.
- **FR-4: User-Scoped Reading Data Retrofit**:
  - `GET /api/v1/reading/works/:workId/progress` queries `reading_progress` filtered by `user_id = $1 AND library_id = $2 AND work_id = $3`.
  - `POST /api/v1/reading/works/:workId/progress` upserts with row lock on `(user_id, library_id, work_id)`.
  - `Bookmark` and `Highlight` queries filter strictly by `user_id` and `library_id` — **including `FindByID` and `Delete` by a single row id**, which MUST additionally carry `AND user_id = $2` so a row cannot be read or deleted by id alone.
  - `ReadingPreferences` queries filter by `user_id` and `device_id`.
  - `GET /api/v1/reading/export` filters every underlying query by `user_id` (and `library_id`) — it returns only the caller's data, never the instance's.
  - **The enforcement point is the handler's call, not the repository's
    capability.** Every reading/reader handler MUST resolve the
    authenticated user + active library and call the `…AndUser`
    repository method; `scripts/check-user-scoped-reading.sh` fails CI on
    a bare-ID call. (Amended 2026-09-02 for review `0050` AUDIT-0012-C1:
    the shipped handlers call the bare methods — `reading.go` never calls
    `UserFromContext` — so `GET /api/v1/reading/export` currently returns
    every user's progress, bookmarks, and highlight notes, and any
    account can read/edit/delete any other account's bookmark by id.
    This is the phase-13 **close gate**: the phase does not close until
    per-endpoint IDOR tests and an `export` isolation test pass. The
    detailed handler-level FRs live in `backend-reading-api.md` /
    `backend-reader-content.md`.)
- **FR-5: Future Cloud Connector Isolation**:
  - All `sources` and `source_offerings` are constrained to `library_id`. Future remote cloud drives (e.g. Nextcloud, Google Drive) connect per-library, isolating file access across namespaces.

## API Contracts

- `GET /api/v1/libraries` -> `200 { libraries: LibrarySummary[] }`
- `POST /api/v1/libraries` -> `201 { library: LibraryDetail }` | `400` | `403`
- `GET /api/v1/libraries/:id` -> `200 { library: LibraryDetail }` | `404`
- `PATCH /api/v1/libraries/:id` -> `200 { library: LibraryDetail }` | `400` | `403`
- `DELETE /api/v1/libraries/:id` -> `204` | `400` | `403`
- `GET /api/v1/libraries/:id/members` -> `200 { members: LibraryMember[] }`
- `POST /api/v1/libraries/:id/invitations` -> `201 { invitation: InvitationDetail }`
- `POST /api/v1/invitations/:token/accept` -> `200 { libraryId: string, role: string }` | `400` | `404`

## Acceptance Criteria

- [ ] Multi-library creation and membership management works as specified.
- [ ] Users only see and access books/sources belonging to their joined libraries.
- [ ] Reading progress, bookmarks, and highlights are strictly isolated per user and library.
