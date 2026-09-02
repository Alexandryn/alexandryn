# Spec: Backend Multi-Library Namespaces & Tenancy

| | |
|---|---|
| **Status** | `APPROVED` (Maintainer Gate 1 sign-off 2026-09-02) |
| **Phase** | `12-auth` |
| **Author** | Claude (Sonnet 4.6), approved by Luann Moreira |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-02 |
| **Supersedes** | — |
| **Reviewed in** | Gate 1 Review Batch |

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
  - If the user does not have active membership in the target library, the request fails with `403 Forbidden` or `404 NotFound`.
- **FR-4: User-Scoped Reading Data Retrofit**:
  - `GET /api/v1/reading/works/:workId/progress` queries `reading_progress` filtered by `user_id = $1 AND library_id = $2 AND work_id = $3`.
  - `POST /api/v1/reading/works/:workId/progress` upserts with row lock on `(user_id, library_id, work_id)`.
  - `Bookmark` and `Highlight` queries filter strictly by `user_id` and `library_id`.
  - `ReadingPreferences` queries filter by `user_id` and `device_id`.
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
