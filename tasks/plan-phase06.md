# Phase 06 (Library) — Implementation Plan

## Context

Alexandryn is a modern personal digital library. Phase 03 built the Go backend foundation and PostgreSQL persistence layer. Phase 04 built the React frontend shell, design tokens, primitives, and MSW mock layer. Phase 05 built the Electron desktop host and process orchestration.

Phase 06 bridges frontend and backend for the core **Library** domain:
1. **Backend Library API** (`.claude/specs/backend-library-api.md`): Exposes `/api/v1/library`, `/api/v1/works/:id`, and `/api/v1/collections` over loopback HTTP with cursor-based pagination, sorting, filtering, and OpenAPI contract tests.
2. **Frontend Library Screens** (`.claude/specs/frontend-library-screens.md`): Implements the real Library Home (grid/list, search/filter/sort, TanStack Query pagination) and Work Detail views, replacing MSW tier-(b) fixtures.
3. **Frontend Collections Screens** (`.claude/specs/frontend-collections-screens.md`): Implements Collections Index, Detail, and membership management.

Governed by `architecture-contracts.md`, `domain-library.md`, `backend-persistence.md`, and Project Constitution.

---

## Tiers Breakdown

### Tier 0 — Tooling & OpenAPI Contract Test Framework
- Establish contract testing tool in CI and local test suite.
- Define `/api/v1/library`, `/api/v1/works/{id}`, and `/api/v1/collections` in `api/openapi.yaml`.

### Tier 1 — Backend Library & Work Endpoints (Go)
- Implement `/api/v1/library` (GET cursor-paginated entries, filter by format/status, sort by title/author/date_added).
- Implement `/api/v1/works/{id}` (GET work detail with owned editions, formats, collection memberships).
- Unit tests, PostgreSQL integration tests, and OpenAPI contract tests.

### Tier 2 — Backend Collections Endpoints (Go)
- Implement `/api/v1/collections` (GET list, POST create).
- Implement `/api/v1/collections/{id}` (GET detail, PATCH rename, DELETE).
- Implement `/api/v1/collections/{id}/works` (POST add work, DELETE remove work).
- Unit tests, integration tests, and contract tests.

### Tier 3 — Frontend Library Screens & Data Fetching (React)
- Create TanStack Query hooks for library cursor pagination and work detail.
- Implement Library Home Screen (Grid/List views, search/filter/sort, pagination).
- Implement Work Detail Screen (owned editions, metadata, collection badges).
- Retire tier-(b) MSW fixtures for library routes; component unit tests.

### Tier 4 — Frontend Collections Screens & Membership UI (React)
- Implement Collections Index screen (cards, counts, create dialog).
- Implement Collection Detail screen (works list, rename, delete).
- Implement Collection membership toggle on Work Detail screen.
- Component and integration tests.

### Tier 5 — Full E2E Lifecycle & Hostile Boundary Tests
- Playwright E2E tests: browse, search, filter, paginate, create collection, add work to collection.
- Hostile payload tests: SQLi / XSS injection attempts in search, malformed pagination cursors, invalid collection IDs.

### Tier 6 — Security Audit, Spec Verification & Closure
- Four-attacker security audit → `.claude/audits/0006-phase06-library.md`.
- **STOP GATE** for maintainer review.
- Flip `backend-library-api.md`, `frontend-library-screens.md`, and `frontend-collections-screens.md` to `VERIFIED`.
