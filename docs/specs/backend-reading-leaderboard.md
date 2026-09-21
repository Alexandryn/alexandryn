# Spec: Backend reading leaderboard

| | |
|---|---|
| **Status** | `APPROVED` |
| **Phase** | `15-observability` |
| **Author** | Claude (Sonnet 5), approved by maintainer |
| **Created** | 2026-09-07 |
| **Last updated** | 2026-09-07 |
| **Supersedes** | — |
| **Reviewed in** | Self-review against Gate 1 maintainer-approved scope |
| **Design reference** | `N/A` (no canvas for leaderboard; approved by maintainer under Gate 0 decision G0-3 as backend-first). |

## Context

Alexandryn is a shared digital library hosted in households and small communities. While reading itself is an individual experience, sharing reading milestones (such as whether a book has been finished) provides a shared library context and helps readers discover popular works.

However, Constitution §8 and [`domain-reading.md`](domain-reading.md) establish a strict privacy boundary: precise reading coordinates (EPUB CFI locations, page numbers, chapter titles, time remaining, and fractional progress percentages) are sensitive personal data. They must remain strictly confidential to the individual reading user. Only the binary milestone "finished" (`percentage >= 100`) is permitted to be shared across library members.

`backend-reading-api.md` (amended 2026-09-02) explicitly deferred the library-visible "finished" aggregate and leaderboard view to Phase 15. Gate 0 decision G0-3 formally included this capability in Phase 15 with its own specification and an automated privacy verification test.

## Problem

Library members cannot see which books have been finished by other members of the library or identify the most-read works in their shared catalog without exposing private, in-progress reading coordinates or breaching tenant boundaries.

## Goals

- Provide a library-scoped `GET /api/v1/library/finished` endpoint returning works finished by members (`percentage >= 100`).
- Provide a library-scoped `GET /api/v1/library/leaderboard` endpoint returning works ranked by finished member count.
- Enforce the privacy invariant structurally: no fractional percentage, reading position, chapter, or time-remaining value is ever queried, projected, or returned in these endpoints.
- Enforce multi-library tenancy isolation: library members can only observe finished data for the library specified in their active library context (`X-Library-Id`).
- Implement an automated RED privacy test proving that in-progress reading records (`percentage < 100`) and private reading coordinates are completely absent from response payloads.

## Non-goals

- Exposing in-progress reading percentage (e.g., "User X is at 45% of Book Y"): strictly prohibited by Constitution §8 and Non-goals.
- Social feeds, activity streams, or comments: out of scope for Phase 15.
- Leaderboard user interface in this phase: no design canvas exists (Unclassified gap). The capability is headless backend API only.
- Cross-library global aggregates: all queries are strictly isolated to the active `library_id`.

## User stories

- As a **library member**, I want to see which books have been finished by other members in my library, so that I can find books that people enjoyed and discuss them.
- As a **library member**, I want to see a ranked list of the most-read books in my library, so that I can browse popular works.
- As a **reader**, I want to know that my in-progress reading position, chapter, and reading percentage are never shared with anyone, so that my reading privacy is completely preserved.

## Functional requirements

### Privacy Invariant and Query Layer

- **FR-1** The system MUST strictly enforce the privacy invariant: **only the binary status of a completed read (`percentage >= 100`) is shareable within a library**. In-progress reading progress, fractional percentage values, precise EPUB CFI locations, position strings, current chapter titles, and estimated reading time remaining MUST NEVER be returned by any library-wide or multi-user endpoint.
- **FR-2** The database query for finished works MUST execute against `reading_progress` filtered by:
  ```sql
  WHERE library_id = $1 AND percentage >= 100
  ```
  where `$1` is the caller's validated active library ID. The SQL projection MUST NOT select `percentage`, `position`, or `current_chapter`.
- **FR-3** Any reading progress record with `percentage < 100` (including `percentage = 99.9`) MUST be excluded from all leaderboard and finished aggregates.

### Endpoints

- **FR-4** The server MUST provide `GET /api/v1/library/finished`.
  - The endpoint MUST require valid authentication (`AuthMiddleware`).
  - The endpoint MUST resolve the active library from `X-Library-Id` or the session default and verify membership.
  - The endpoint MUST permit any authenticated member of the library, including users with role `reader` and role `admin`.
  - The response MUST return a JSON list of works that have at least one completion in the active library. For each work, it MUST provide `work_id` and a list of completions containing `user_id`, `display_name` (user's username or profile name), and `finished_at` (timestamp).
- **FR-5** The server MUST provide `GET /api/v1/library/leaderboard`.
  - The endpoint MUST require valid authentication and active library membership.
  - The endpoint MUST accept an optional query parameter `limit` (integer, default 20, maximum 50).
  - The response MUST return a JSON list of works ordered in descending order of distinct users who have finished the work (`finished_count DESC`), returning `work_id` and `finished_count`.
- **FR-6** Both endpoints MUST return HTTP 401 Unauthorized for unauthenticated requests, and HTTP 403 Forbidden if the user is not an active member of the requested library.
- **FR-7** Both endpoints MUST enforce multi-library tenancy: a member of Library A MUST NOT be able to see finished entries or leaderboard counts from Library B.

### Automated Privacy Verification (RED Gate)

- **FR-8** The test suite MUST include a dedicated integration privacy test (`TestLeaderboard_PrivacyAndTenantIsolation`) that executes before endpoint implementation and fails against unredacted or improperly filtered queries.
- **FR-9** The privacy test MUST verify:
  1. A user with `percentage = 99` on a work is completely excluded from `GET /api/v1/library/finished` and does not increment `finished_count` in `GET /api/v1/library/leaderboard`.
  2. A user with `percentage = 100` is included in the finished list and increments the count.
  3. The raw JSON response payloads of both endpoints contain zero occurrences of the keys `percentage`, `position`, `cfi`, `chapter`, `location`, `progress`, or numeric percentage values.
  4. Cross-library isolation: records in Library B do not appear when querying Library A.

## Non-functional requirements

- **Performance:** Leaderboard aggregation queries MUST complete within 50 ms under 100,000 `reading_progress` rows. Index `reading_progress (library_id, percentage, work_id)` MUST be utilized.
- **Security:** Strict multi-library tenant isolation. No sensitive in-progress coordinates or personal habits can be inferred through differential query timing or error states.
- **Accessibility:** Not applicable (headless API).
- **Reliability:** If a library has no finished books, the endpoints MUST return empty JSON arrays (`[]`) with HTTP 200 OK, not HTTP 404.
- **Observability:** Queries executing longer than 100 ms MUST log at `warn` level with request ID and library ID, without logging user IDs or work IDs.

## Domain model

This feature consumes existing domain models from `domain-reading.md` and `domain-library.md`:
- `reading_progress` table (`internal/persistence/postgres`):
  - `work_id` (TEXT)
  - `user_id` (TEXT)
  - `library_id` (TEXT)
  - `percentage` (NUMERIC)
  - `updated_at` (TIMESTAMPTZ)
- DTO models:
  ```go
  type FinishedWorkItem struct {
      WorkID     string            `json:"work_id"`
      FinishedBy []FinishedUserRef `json:"finished_by"`
  }

  type FinishedUserRef struct {
      UserID      string    `json:"user_id"`
      DisplayName string    `json:"display_name"`
      FinishedAt  time.Time `json:"finished_at"`
  }

  type LeaderboardItem struct {
      WorkID        string `json:"work_id"`
      FinishedCount int    `json:"finished_count"`
  }
  ```

## API and contracts

### `GET /api/v1/library/finished`

- **Headers:** `Authorization: Bearer <token>`, `X-Library-Id: <library_id>`
- **Response 200 OK:**
  ```json
  {
    "works": [
      {
        "work_id": "work-uuid-1",
        "finished_by": [
          {
            "user_id": "user-uuid-1",
            "display_name": "alice",
            "finished_at": "2026-09-01T12:00:00Z"
          },
          {
            "user_id": "user-uuid-2",
            "display_name": "bob",
            "finished_at": "2026-09-05T15:30:00Z"
          }
        ]
      }
    ]
  }
  ```

### `GET /api/v1/library/leaderboard`

- **Headers:** `Authorization: Bearer <token>`, `X-Library-Id: <library_id>`
- **Query Parameters:** `limit` (integer, optional, default 20, max 50)
- **Response 200 OK:**
  ```json
  {
    "leaderboard": [
      {
        "work_id": "work-uuid-1",
        "finished_count": 2
      },
      {
        "work_id": "work-uuid-2",
        "finished_count": 1
      }
    ]
  }
  ```

## State transitions

`reading_progress` rows transition according to `domain-reading.md`:
```
[In-Progress: percentage < 100] ──(reading progress update)──> [Finished: percentage >= 100]
             │                                                              │
             ▼                                                              ▼
   Excluded from Finished &                                       Included in Finished &
      Leaderboard Views                                              Leaderboard Views
```
If a reader resets progress or an epoch reconciliation updates progress backwards (legal in domain model for restarts), the row drops out of the finished aggregate.

## Failure modes

| Failure | Detected how | Caller sees | System does |
|---|---|---|---|
| Non-member queries library | Membership check against context | HTTP 403 Forbidden | Request rejected, logged at `warn` |
| Unauthenticated request | `AuthMiddleware` | HTTP 401 Unauthorized | Request rejected |
| Library has zero finished works | SQL returns zero rows | HTTP 200 `{"works": []}` / `{"leaderboard": []}` | Normal empty response served |
| Negative or non-numeric limit | Input validator | HTTP 400 Bad Request | Invalid parameter error returned |
| Database timeout | SQL context deadline | HTTP 500 Internal Server Error | Error logged with request ID |

## Security considerations

- **STRIDE Threat Modeling:**
  - *Spoofing:* Authenticated Bearer JWT required on all requests.
  - *Tampering:* Endpoints are read-only projections.
  - *Information Disclosure (Constitution §8):* Reading privacy is the core threat. SQL query explicitly filters `percentage >= 100` and projects zero position or percentage columns. Responses cannot leak whether a user is at 0%, 50%, or 99%.
  - *Denial of Service:* Aggregations are bounded by `LIMIT` clauses and backed by compound indexes on `(library_id, percentage, work_id)`.
  - *Elevation of Privilege:* Reader role is permitted access to their own library's finished works, but cannot access libraries they do not belong to.
- **Enforcement Point:** Handlers verify active library membership (`ActiveLibraryFromContext`). Repository queries enforce `WHERE rp.library_id = $1 AND rp.percentage >= 100`.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Sorting and JSON serialization of `FinishedWorkItem` and `LeaderboardItem` structs. |
| Integration | Repository query execution against real PostgreSQL instance, confirming `percentage >= 100` filtering and index usage. |
| Privacy (RED Gate) | `TestLeaderboard_PrivacyAndTenantIsolation`: asserts that a user at `percentage = 99` is absent; asserts response JSON contains no `percentage`, `position`, `cfi`, or `chapter` keys; asserts Library A member cannot observe Library B entries. |
| Contract | OpenAPI contract test verifying JSON schema conformity. |

## Acceptance criteria

- [ ] `GET /api/v1/library/finished` returns works finished by members in the active library.
- [ ] `GET /api/v1/library/leaderboard` returns works ordered by completion count.
- [ ] Users with `percentage < 100` are strictly excluded from both endpoints.
- [ ] Response payloads contain zero reading coordinates, CFI strings, chapter names, or percentage numbers.
- [ ] Multi-library tenancy is strictly enforced; cross-library data access returns HTTP 403 or empty data.
- [ ] RED privacy test fails before implementation and passes green after repository implementation.
- [ ] Negative authorization tests confirm non-members receive HTTP 403.

## Open questions

- **Ties in leaderboard ranking:** When works have identical finished counts, ordering falls back to `work_id ASC` for determinism.

## References

- Constitution §8: Reading privacy without leakage
- `domain-reading.md`: ReadingProgress model and reconciliation
- `backend-reading-api.md`: Reading API endpoints and deferred leaderboard note
- `docs/roadmap/15-observability/README.md`: Gate 0 decision G0-3
