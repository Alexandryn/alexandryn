# Test plan: Backend reading leaderboard

| | |
|---|---|
| **Spec** | `.claude/specs/backend-reading-leaderboard.md` |
| **Status** | `DRAFT` |
| **Created** | 2026-09-07 |

## What we are trying to be confident about

1. The privacy invariant is inviolable: **only the binary "finished" status (`percentage >= 100`) is ever exposed**.
2. No response payload ever contains reading coordinates, EPUB CFI locations, page numbers, chapter titles, time-remaining estimates, or fractional percentages.
3. Readers at `percentage < 100` (including `99.9%`) are completely absent from `GET /api/v1/library/finished` and do not contribute to `GET /api/v1/library/leaderboard`.
4. Multi-library tenancy is strictly enforced: a user querying Library A cannot see completions or leaderboard statistics from Library B.
5. Leaderboard aggregation query scales efficiently under large datasets using indexed lookups.

## Risk assessment

**Hardest to get right:**
- Preventing subtle privacy leaks. If an endpoint includes an object where `percentage` or `position` is serialized with an empty or default value, or if an error message reflects in-progress state, the privacy guarantee is weakened. The privacy test must scan the entire raw JSON byte response.
- Query precision at boundary conditions. Testing `percentage = 99.9` versus `100.0` to ensure numeric comparisons do not round up prematurely.
- Multi-tenant isolation. Enforcing that `X-Library-Id` is strictly checked against user membership and used in query predicates.

**Merely tedious:**
- Table-driven DTO serialization tests.
- Handling empty library states (no finished books).

## Layers

### Unit

- Table-driven serialization tests for `FinishedWorkItem` and `LeaderboardItem`: asserts correct JSON keys and omission of any coordinate fields.
- Query parameter validation: asserts `limit` parser rejects negative numbers, zero, or values exceeding 50.

### Integration

Integration tests running against real PostgreSQL (`backend-test-harness.md`):
- **Boundary condition verification:**
  - Seeds `reading_progress` rows with percentages: 0, 25.5, 50.0, 99.0, 99.9, 100.0, 105.0.
  - Queries `GET /api/v1/library/finished`.
  - Asserts only users with percentages 100.0 and 105.0 are returned in `finished_by`.
  - Queries `GET /api/v1/library/leaderboard`.
  - Asserts `finished_count` reflects only completed reads.
- **Tenant isolation:**
  - Seeds finished books in Library A and Library B.
  - User in Library A queries `/finished` and `/leaderboard` with `X-Library-Id: Library-A`.
  - Asserts zero data from Library B appears in response.
  - User in Library A attempts to query with `X-Library-Id: Library-B` -> receives HTTP 403 Forbidden.
- **Empty library response:**
  - Newly initialized library returns `{"works": []}` and `{"leaderboard": []}` with HTTP 200 OK.

### Contract

- OpenAPI contract test validating schema compliance for both endpoints.

### Privacy Verification (RED Gate)

- **`TestLeaderboard_PrivacyAndTenantIsolation` (T3.2):**
  - Executes prior to repository and endpoint implementation.
  - Asserts that raw response JSON contains:
    - Zero occurrences of string `"position"`
    - Zero occurrences of string `"cfi"`
    - Zero occurrences of string `"chapter"`
    - Zero occurrences of string `"percentage"`
    - Zero occurrences of string `"progress"`
  - Asserts that a reader with `percentage = 99` is absent from all results.
  - Fails if any coordinate or fractional value is leaked.

## Adversarial cases

| Scenario | Expected behaviour |
|---|---|
| User with `percentage = 99.99` queries finished list | Excluded from results |
| User with role `reader` accesses finished list in own library | HTTP 200 OK; access permitted |
| User attempts SQL injection in `limit` parameter (`limit=10; DROP TABLE`) | HTTP 400 Bad Request; parameterized query |
| User requests `limit=1000` | Clamped to maximum 50 |
| Member of Library A sends `X-Library-Id` for Library B | HTTP 403 Forbidden; audit event logged at `warn` |

## Fixtures and test data

- Synthetic libraries A and B.
- Deterministic test users: Alice (finished Book 1), Bob (at 99% of Book 1), Charlie (finished Book 1 and Book 2), Dave (Library B member, finished Book 1).

## What is deliberately not tested

- Rendering of leaderboard cards in HTML (no UI surface in Phase 15).
- Re-reading milestones (multiple completions by same user on same work). Handled by existing reading progress unique constraint `(user_id, work_id)`.

## Exit criteria

- [ ] All integration tests pass against PostgreSQL.
- [ ] RED privacy test fails before implementation and passes green after endpoint implementation.
- [ ] Multi-tenant isolation verified by cross-library tests.
- [ ] Response byte scanner confirms zero coordinate leaks.
