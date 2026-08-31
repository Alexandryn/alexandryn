# Phase 06: Library — Master TODO

## Tier 0 — Tooling & OpenAPI Contract Test Framework
- [ ] L01 — Define Phase 06 endpoint schemas in `api/openapi.yaml`
- [ ] L02 — Setup OpenAPI contract test tooling in Go test harness / CI

## Tier 1 — Backend Library & Work Endpoints (Go)
- [ ] L03 — Implement repository queries for library entries with cursor pagination, sorting, and filtering
- [ ] L04 — Implement `/api/v1/library` HTTP handler with cursor encoding/decoding and error handling
- [ ] L05 — Implement `/api/v1/works/{id}` HTTP handler returning owned editions and collection memberships
- [ ] L06 — Unit tests, database integration tests, and OpenAPI contract tests for library & work endpoints

## Tier 2 — Backend Collections Endpoints (Go)
- [ ] L07 — Implement `/api/v1/collections` list & create handlers
- [ ] L08 — Implement `/api/v1/collections/{id}` get, rename, and delete handlers
- [ ] L09 — Implement `/api/v1/collections/{id}/works` add and remove work handlers
- [ ] L10 — Unit tests, integration tests, and OpenAPI contract tests for collections endpoints

## Tier 3 — Frontend Library Screens & TanStack Query (React)
- [ ] L11 — Create TanStack Query hooks for library cursor pagination and work detail
- [ ] L12 — Implement Library Home Screen (Grid/List toggle, search, filter chips, sort select)
- [ ] L13 — Implement Work Detail Screen with owned editions and collection badges
- [ ] L14 — Retire MSW tier-(b) fixtures for `/api/v1/library` and `/api/v1/works/:id`
- [ ] L15 — Vitest component and hook tests

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
