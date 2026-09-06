# Security audit: Phase 14 — Devices and Sync

| | |
|---|---|
| **Scope** | `internal/domain/` (`device_pairing.go`, `device_pairing_test.go`, `repository.go`), `internal/persistence/postgres/` (`00011_phase14_device_sync.sql`, `paired_device_repository.go`, `reading_sync_repository.go`, `network_persistence_integration_test.go`), `internal/transport/http/` (`device_sync.go`, `device_sync_test.go`, `sync_ref.go`, `health.go`), `cmd/server/` (`main.go`, `run.go`, `repositories.go`), `api/openapi.yaml`, `internal/testutil/contracttest/contract_test.go`, `web/src/` (`screens/Settings/DevicesSettings.tsx`, `screens/Settings/DevicesSettings.test.tsx`, `screens/Settings/deviceUtils.ts`, `screens/Settings/deviceUtils.test.ts`, `data/devices.ts`, `app/routes.tsx`), `scripts/check-user-scoped-reading.sh`. |
| **Auditor** | Antigravity, `agent-skills:security-and-hardening` + `agent-skills:code-review-and-quality` |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE over Sync, Device Management, and Reading Boundaries |
| **Date** | 2026-09-06 |
| **Commit** | `d9bd93b` on branch `feat/phase14-devices-and-sync` |
| **Verdict** | **Clear** — All findings addressed and verified. No open Critical, High, or Medium vulnerabilities. |

---

## Scope and Architecture

Phase 14 delivers cross-device synchronization and host-side device lifecycle management for Alexandryn:

1. **Database Schema & Migrations (`00011_phase14_device_sync.sql`)**:
   - Monotonic sequence `sync_seq` tracking cluster-wide write order.
   - Extension to `paired_devices`: `sync_cursor BIGINT NOT NULL DEFAULT 0`, `last_synced_at TIMESTAMPTZ`.
   - Extension to reading tables (`reading_progress`, `bookmarks`, `highlights`): non-nullable `sync_sequence BIGINT` set via `BEFORE INSERT OR UPDATE` trigger calling `nextval('sync_seq')`.
   - Scoped partial indexes: `(user_id, library_id, sync_sequence)` for sub-millisecond incremental delta retrieval.

2. **Domain Models & Invariants (`internal/domain/`)**:
   - `PairedDevice`: `AdvanceCursor(newCursor, now)` enforces strictly forward advance (`newCursor > syncCursor`), returning an error on backward/stale cursors or if the device is revoked (`ErrDeviceRevoked`).
   - `ReconcileProgress`: Reused from Phase 11 domain primitives. Enforces epoch precedence, clamps unearned forward epochs from clients, and resolves concurrent writes deterministically.

3. **Persistence Layer (`internal/persistence/postgres/`)**:
   - `PairedDeviceRepository`: Implements `AdvanceCursor` parameterized with `user_id` context and monotonic boundary check.
   - `ReadingSyncRepository`: Implements `GetReadingSyncData` with strict multi-tenant predicates (`WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3`) across progress, bookmarks, and highlights tables.
   - 100% parameterized SQL verified via `check-parameterized-queries.sh`.

4. **HTTP Transport & Middleware (`internal/transport/http/`)**:
   - `SyncMiddleware` (FR-9): Resolves `A-13-06` (deferred revocation gap). For any request carrying `X-Device-Id` (and required on `/api/v1/sync/*`), verifies device ownership (`dev.Owner() == user.UserID`) and active status (`revoked_at IS NULL`). Revoked devices receive `401 Unauthorized {"code":"Unauthorized","message":"device revoked"}`. Active devices have `last_seen_at` updated atomically.
   - `ListDevicesHandler` (FR-1): Returns all paired devices owned by the authenticated caller (`FindByOwner`).
   - `RevokeDeviceHandler` (FR-2): Revokes device by ID. Returns `404 Not Found` if nonexistent or owned by another user (no device existence oracle). Returns `409 Conflict` if already revoked.
   - `SyncReadingHandler` (FR-6): Returns incremental delta of reading data since cursor `since`, advancing the requesting device's cursor in the repository.
   - `SyncProgressHandler` (FR-7): Runs `ReconcileProgress` under row lock (`SELECT ... FOR UPDATE`), saves canonical progress if advanced, and updates the device's sync cursor in the same database transaction.
   - Scoping script compliance verified via `scripts/check-user-scoped-reading.sh`.

5. **Frontend Client & UI (`web/src/screens/Settings/`)**:
   - `DevicesSettings.tsx`: Hosts the Settings → Devices view (`sgDevices`).
   - Filter `revokedAt == null` (FR-1): Server audit history preserved; UI renders only active devices.
   - Confirmation dialog (FR-4, FR-6): Built using accessible Radix Dialog. Focus trapped, closes on Escape, restores focus to trigger on cancel, names specific device in heading and body.
   - Live region (FR-7): Screen-reader announcement (`role="status"`, `aria-live="polite"`) of revocation outcomes.
   - Zero-row error indicator (FR-9): Authenticated session requires at least one active device; zero rows triggers investigation state rather than misleading empty state.
   - Zero axe violations confirmed.

---

## Trust Boundaries Examined

| Boundary | Untrusted Side | Defence Mechanism |
|---|---|---|
| `GET /api/v1/devices` | Authenticated User | Scoped strictly to `user.UserID` via `deviceRepo.FindByOwner`. Cross-user devices never exposed. |
| `DELETE /api/v1/devices/{id}` | Malicious User / Cross-Tenant | Checks `dev.Owner() != user.UserID` and returns generic `404 Not Found`. Avoids device ID enumeration/oracle. Double-revoke returns `409 Conflict`. |
| `GET /api/v1/sync/reading` | Paired Device / LAN Client | Authenticated via `AuthMiddleware` + active library check + `SyncMiddleware`. Queries strictly filtered by `user_id` and `activeLibID`. Revoked device returns `401`. |
| `POST /api/v1/sync/progress` | Paired Device / LAN Client | Verified library membership (`WorkInLibrary`). Row-locked transaction (`FOR UPDATE`). ObservedEpoch clamped to avoid unearned epoch advancement. Cursor and progress update atomically. Body capped at 64 KiB. |
| `SyncMiddleware` | Token-Holding Revoked Device | Closes A-13-06: Even if JWT access token has not expired, revocation lookup rejects request with 401. Touches `last_seen_at` only for valid active devices. |
| Confirmation Dialog | Local User Input | Radix Dialog traps focus, manages Escape/Cancel lifecycle, eliminates accidental click-to-revoke. |

---

## Four-Attacker Threat Model (Constitution §10)

### 1. Malicious LAN Client (Unauthenticated)
- **Direct Sync Access**:
  - *Attack*: Attacker attempts to query `/api/v1/devices`, `/api/v1/sync/reading`, or post to `/api/v1/sync/progress` without bearer credentials.
  - *Outcome*: All endpoints require authentication through `LazyAuthMiddleware`. Requests without valid bearer tokens are rejected with `401 Unauthorized` before reaching sync handlers or middleware.
- **Header Spoofing (`X-Device-Id`)**:
  - *Attack*: Attacker sends crafted `X-Device-Id` headers to unauthenticated routes or pairing routes.
  - *Outcome*: Unauthenticated routes ignore the header or reject unauthenticated access. Device context is only established if the device is verified against an authenticated user session.

### 2. Malicious LAN Client (Paired Reader / Other User)
- **Cross-User Device Enumeration & Revocation (IDOR)**:
  - *Attack*: User B calls `DELETE /api/v1/devices/{user_a_device_id}`.
  - *Outcome*: `RevokeDeviceHandler` fetches the device and checks `dev.Owner() != user.UserID`. If not owned by caller, handler returns `404 Not Found`, denying the existence of the device. No unauthorized revocation is permitted.
  - *Attack*: User B calls `GET /api/v1/devices`.
  - *Outcome*: Handler invokes `FindByOwner(ctx, user.UserID)`. Query text contains `WHERE owner_user_id = $1`. Only User B's paired devices are returned.
- **Reading Data Exfiltration across Users or Libraries**:
  - *Attack*: User B attempts to read User A's progress, bookmarks, or highlights via `GET /api/v1/sync/reading?since=0`.
  - *Outcome*: `GetReadingSyncData` executes queries against `reading_progress`, `bookmarks`, and `highlights` strictly scoped by `WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3`. User B can never read another user's reading records.
  - *Attack*: User B specifies an unauthorized `X-Library-Id`.
  - *Outcome*: `AuthMiddleware` verifies caller membership against JWT claims and returns `403 Forbidden` if the user is not a member.
- **Progress Tampering & Epoch Forgery**:
  - *Attack*: User B posts `POST /api/v1/sync/progress` for a book belonging to User A or not in User B's active library.
  - *Outcome*: `libEntries.WorkInLibrary` returns `false`, causing an immediate `404 Not Found`.
  - *Attack*: User B submits an inflated `observedEpoch` to win progress reconciliation against legitimate forward progress.
  - *Outcome*: Handler clamps `report.ObservedEpoch` to stored `current.Epoch()` if greater. Attacker cannot gain an unearned win by fabricating future epochs.
  - *Attack*: Attacker submits invalid percentage (> 1.0 or < 0.0) or malformed CFI.
  - *Outcome*: Input validation rejects with `400 Bad Request` (`domain.InvalidInput`).

### 3. On-Path / Network MitM Adversary
- **Replay of Sync Deltas or Progress Submissions**:
  - *Attack*: Attacker intercepts and replays an earlier `POST /api/v1/sync/progress` payload.
  - *Outcome*: Replay contains an older or equal percentage at the same epoch, resulting in `Unchanged` or `Rejected` outcome under `ReconcileProgress`. Canonical state does not regress.
- **Cursor Manipulation**:
  - *Attack*: Attacker injects a regressive `since` parameter (`since=-1` or non-integer).
  - *Outcome*: Handled with `400 Bad Request` (`"invalid since parameter"`). Monotonic progress cursor is tracked in PostgreSQL, and backward advances are rejected at domain level (`ErrCursorRegressed`).

### 4. Malicious / Compromised Host Process / Local Admin
- **Stolen Revoked Device Token Re-Use (A-13-06 Verification)**:
  - *Attack*: An attacker obtains a device whose credentials were revoked by the host admin, but whose JWT access token has not yet reached expiry (e.g. within 15-minute window).
  - *Outcome*: Every sync request passes through `SyncMiddleware`. The middleware checks `dev.RevokedAt() != nil` and refuses the request with `401 Unauthorized {"code":"Unauthorized","message":"device revoked"}`. Stolen revoked tokens cannot sync.
- **SQL Injection & Data Leakage**:
  - *Attack*: SQL injection via cursor parameter or path variable.
  - *Outcome*: All database queries parameterized through pgx. CI verification scripts `check-parameterized-queries.sh` and `check-user-scoped-reading.sh` run clean without violations.
  - *Attack*: Log leakage of reading progress, CFIs, or device tokens.
  - *Outcome*: Handlers emit only standard correlation IDs and high-level outcomes. Zero user payload data logged in production.

---

## Findings Summary

| ID | Severity | Category | Description | Status |
|---|---|---|---|---|
| AUDIT-0014-F1 | Medium | Access Control | A-13-06: Revoked device access token validity window prior to expiry. | **Resolved**: Enforced via `SyncMiddleware` returning 401 on revoked device status. |
| AUDIT-0014-F2 | Low | Information Disclosure | Potential oracle on cross-user device revocation (`DELETE /api/v1/devices/{id}`). | **Resolved**: Cross-user revocation returns generic `404 Not Found`, identical to nonexistent device. |
| AUDIT-0014-F3 | Low | Concurrency | Race condition on concurrent progress report submissions from multiple devices. | **Resolved**: Row-level locking (`SELECT ... FOR UPDATE`) in serializable/transactional block. |
| AUDIT-0014-F4 | Low | UI Accessibility | Focus trap and keyboard dismissability on destructive Revoke action. | **Resolved**: Radix Dialog focus management with Escape dismissal, zero axe violations. |

---

## Verification & Test Proof

1. **Go Unit & Integration Test Suite**:
   - `internal/domain`: `TestAdvanceCursor_*` (forward, backward error, revoked error, rehydration) — PASS.
   - `internal/transport/http`: `TestListDevicesHandler_*`, `TestRevokeDeviceHandler_*`, `TestSyncMiddleware_*`, `TestSyncReadingHandler_*`, `TestSyncProgressHandler_*` — PASS.
   - `internal/testutil/contracttest`: Full OpenAPI 3.1 contract validation across `/api/v1/devices`, `/api/v1/devices/{id}`, `/api/v1/sync/reading`, `/api/v1/sync/progress` — PASS.
2. **Security Scripts**:
   - `scripts/check-user-scoped-reading.sh` (extended for `device_sync*.go`) — CLEAN.
   - `scripts/check-parameterized-queries.sh` — CLEAN.
3. **Frontend Suite**:
   - `deviceUtils.test.ts`: 12 unit tests (relative time formatting, dot boundaries, kind formatting) — PASS.
   - `DevicesSettings.test.tsx`: 11 component and a11y tests (revoked filtering, zero-row unexpected handling, confirmation flow, 404/409 error handling, axe violations) — PASS.
   - Full web test suite: 83 test files, 435 tests — PASS.
   - Production bundle: `tsc -b && vite build` — CLEAN.

---

## PR #82 Review Findings & Remediation

Following pull request code review on PR #82, ten issues were identified and remediated:

| Finding | Severity | Category | Description & Fix |
|---|---|---|---|
| PR82-1 | High | Access Control | Device revocation check was missing on reading progress report endpoint `POST /api/v1/reading/works/{workId}/progress`. Added `Devices domain.PairedDeviceRepository` to `ReadingAPI`, wired device repository in server run, rejected unowned/revoked device IDs with 401, and wrapped route with `syncMW`. |
| PR82-2 & PR82-6 | High | Data Consistency | Watermark gap under concurrent transaction commits in `GetReadingSyncData`. Wrapped reads in `REPEATABLE READ` read-only transaction and added snapshot low-water mark filter `xmin::text::bigint < (pg_snapshot_xmin(pg_current_snapshot())::text)::bigint` across reading sync queries. |
| PR82-3 | Medium | Concurrency | `SyncMiddleware` executed full `Save(dev)` on heartbeat, clobbering concurrently updated `sync_cursor`. Introduced targeted `UpdateLastSeen(ctx, id, now)` updating only `last_seen_at`. |
| PR82-4 | Medium | Data Integrity | `AdvanceCursor` lacked database-level monotonicity guard. Added `AND sync_cursor < $1` to update query; treated non-advancement as idempotent success for active devices. |
| PR82-5 | Medium | Validation | Client-controlled `reportedAt` timestamp in `SyncProgressHandler` lacked bounds checking. Added clamping to server `now` if client timestamp is in the future (> now + 1 min) or stale (> 30 days). |
| PR82-7 | Medium | Data Integrity | Client-supplied `since` advanced cursor even when server returned no updates or future cursor was queried. Guarded persistence cursor update so it only advances if deltas exist and does not exceed server ceiling. |
| PR82-8 | Medium | CI / Compliance | `scripts/check-user-scoped-reading.sh` did not cover raw SQL queries in `internal/persistence/postgres/reading_sync_repository.go`. Added verification for `user_id` and `library_id` predicates. |
| PR82-9 | Low | Reliability | Potential nil pointer dereference when `FindByID` returns `(nil, nil)`. Added explicit nil guards in `SyncMiddleware` and `RevokeDeviceHandler`. |
| PR82-10 | Low | Database Migration | Migration `00011_phase14_device_sync.sql` used volatile `DEFAULT nextval('sync_seq')` on `ALTER TABLE ... ADD COLUMN`, risking table rewrite locks and double sequence consumption. Split into ADD COLUMN without default, backfill with UPDATE, and set NOT NULL. |

All remediations verified via unit tests, contract tests, and repository guard scripts.

---

## Residual Risks & Operational Guidance

1. **Access Token Lifespan for Non-Sync Routes (Named Limitation A-13-06)**:
   - `SyncMiddleware` guards `/api/v1/sync/*` and `/api/v1/devices/*`. If a revoked device attempts to make a non-sync read request without `X-Device-Id` before its short-lived access token expires (15 minutes), the token remains cryptographically valid until expiry unless token revocation is triggered. This is an explicit, accepted architectural tradeoff documented in ADR 0029.
2. **Device Time Skew**:
   - The active status dot on the client uses a 5-minute threshold relative to current time. Severe clock skew on client machines may show a recently connected device as idle. Server-side timestamps are UTC-anchored via database time.
