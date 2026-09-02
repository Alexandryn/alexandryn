# Spec: Role-Based Access Control (RBAC) & Authorization

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

Alexandryn requires explicit authorization boundaries distinguishing system administrators (`admin`) from regular library consumers (`reader`). In addition, administrators must be able to control whether readers can ingest/upload books to a specific library.

## Roles & Permissions Matrix

| Resource / Capability | `admin` | `reader` (Uploads Allowed) | `reader` (Uploads Disabled) |
|---|:---:|:---:|:---:|
| System Setup (`/auth/setup`) | Initial boot only | ❌ | ❌ |
| Library Creation / Deletion / Edit | ✅ | ❌ | ❌ |
| Library Ingestion Policy Toggle (`allow_reader_uploads`) | ✅ | ❌ | ❌ |
| Source & Adapter Management (`/sources*`) | ✅ | ❌ | ❌ |
| User Invitations & Role Management (`/libraries/:id/members*`) | ✅ | ❌ | ❌ |
| Work / Book Browsing & Searching (`/library*`, `/discover*`) | ✅ | ✅ | ✅ |
| EPUB Reading & Content Serving (`/library/editions/:id/reader/*`) | ✅ | ✅ | ✅ |
| Reading Progress Reconcile & State (`/reading/works/:id/progress`) | ✅ (own user) | ✅ (own user) | ✅ (own user) |
| Bookmarks & Highlights CRUD (`/reading/editions/:id/*`) | ✅ (own user) | ✅ (own user) | ✅ (own user) |
| Reading Preferences (`/reading/preferences`) | ✅ (own user) | ✅ (own user) | ✅ (own user) |
| File Upload & Ingestion (`/import/*`, `/library/upload`) | ✅ | ✅ | ❌ (`403 Forbidden`) |

## Functional Requirements

- **FR-1: Role Definitions**:
  - `admin`: Full administrative control across the instance or library.
  - `reader`: Read-only or read-and-ingest user within authorized library namespaces.
- **FR-2: Admin Ingestion Toggle**:
  - The `Library` aggregate has `allow_reader_uploads` (boolean, default `false`).
  - Only users with role `admin` on the library (or global `admin`) can toggle this setting via `PATCH /api/v1/libraries/:id`.
  - When `allow_reader_uploads` is `true`, users with role `reader` in that library may upload files, discover candidates, and confirm imports.
  - When `allow_reader_uploads` is `false`, any ingestion/import request by a `reader` is rejected with `403 Forbidden` (`{"code": "Forbidden", "message": "reader uploads are disabled for this library"}`).
- **FR-3: Authorization Middleware**:
  - `RequireRole(requiredRole ...Role)`: Ensures the authenticated user has at least one of the specified global roles.
  - `RequireLibraryMembership(minRole Role)`: Resolves the active `library_id` from the request (via path parameter `:libraryId`, `X-Library-Id` header, or JWT libraries claim) and verifies that the authenticated user is an active member with role ≥ `minRole`.
  - `RequireIngestPermission()`: Verifies that the user is an `admin` or that the active library has `allow_reader_uploads == true`.
- **FR-4: Error Responses**:
  - Unauthenticated access returns `401 Unauthorized`.
  - Authenticated but unauthorized access returns `403 Forbidden`.
  - Operations on non-existent or inaccessible library resources return `404 NotFound` to prevent tenancy enumeration.

## Non-functional Requirements

- **Fail Closed**: If no user context or membership is found, authorization immediately rejects with `401` or `403`.
- **Audit Logging**: Privileged operations (role changes, member invitations, ingestion policy toggles) are logged with correlation ID, actor user ID, target user ID, and timestamp.

## Acceptance Criteria

- [ ] Readers cannot access or mutate sources, invitations, or library configuration.
- [ ] Readers can read books and manage their own reading data.
- [ ] Readers are blocked from importing/uploading books when `allow_reader_uploads = false`.
- [ ] Admin can toggle `allow_reader_uploads` and manage members.
