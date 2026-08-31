# Phase 06: Library — Master TODO

## Tier 0 — Tooling & OpenAPI Contract Test Framework
- [x] L01 — Define Phase 06 endpoint schemas in `api/openapi.yaml`
- [x] L02 — Setup OpenAPI contract test tooling in Go test harness / CI

## Tier 1 — Backend Library & Work Endpoints (Go)
- [x] L03 — Implement repository queries for library entries with cursor pagination, sorting, and filtering
- [x] L04 — Implement `/api/v1/library` endpoint handler with query validation and pagination
- [x] L05 — Implement `/api/v1/works/:id` endpoint handler with work detail, owned editions, formats, and collections
- [x] L06 — Write unit, integration, and OpenAPI contract tests for library and work endpoints

## Tier 2 — Backend Collections Endpoints (Go)
- [x] L07 — Implement `/api/v1/collections` list & create handlers
- [x] L08 — Implement `/api/v1/collections/{id}` get, rename, and delete handlers
- [x] L09 — Implement `/api/v1/collections/{id}/works` add and remove work handlers
- [x] L10 — Unit tests, integration tests, and OpenAPI contract tests for collections endpoints


## Tier 3 — Frontend Library Screens & TanStack Query (React)
- [x] L11 — Create TanStack Query hooks for library cursor pagination and work detail
- [x] L12 — Implement Library Home Screen (Grid/List toggle, search, filter chips, sort select)
- [x] L13 — Implement Work Detail Screen with owned editions and collection badges
- [x] L14 — Retire MSW tier-(b) fixtures for `/api/v1/library` and `/api/v1/works/:id`
- [x] L15 — Vitest component and hook tests


## Tier 4 — Frontend Collections Screens & Membership UI (React)
- [ ] L16 — Create TanStack Query hooks for collections CRUD and membership mutation
- [ ] L17 — Implement Collections Index screen
- [ ] L18 — Implement Collection Detail screen
- [ ] L19 — Implement Collection membership toggle dialog on Work Detail
- [ ] L20 — Retire MSW fixtures for collections routes; Vitest component tests

## Tier 5 — E2E Walkthrough & Hostile Boundary Tests
- [ ] L21 — Playwright E2E full library walkthrough (browse, search, filter, paginate, manage collections)
- [ ] L22 — Hostile boundary tests (malformed cursors, invalid IDs, injection payloads)

## Tier 6 — Security Audit & Closure
- [ ] L23 — Four-attacker security audit → `.claude/audits/0006-phase06-library.md`
- [ ] **═══ STOP GATE — present audit, await maintainer sign-off ═══**
- [ ] L24 — Flip 3 specs to `VERIFIED`; update `.claude/specs/README.md`; close Phase 06
