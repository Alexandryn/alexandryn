# Test plan: Backend device sync

| | |
|---|---|
| **Spec** | `.claude/specs/backend-device-sync.md` |
| **Status** | `DRAFT` |
| **Created** | 2026-09-05 |

## What we are trying to be confident about

1. A device cannot read or write reading data belonging to another user (no horizontal IDOR on any sync endpoint).
2. `ReconcileProgress` produces the correct canonical value when called via the sync transport from multiple concurrent devices; the outcome is invariant to arrival order at the same epoch.
3. A revoked device's sync requests are refused at the SyncMiddleware level; the revocation check is wired, not only present in the repository.
4. Cursor-based incremental fetch is correct: `since=0` returns the full dataset; subsequent calls with the returned cursor return only new changes; retrying with the same cursor is idempotent.
5. The `PairedDevice` extension (sync_cursor, last_synced_at) is persisted correctly and advances inside the same transaction as reconciliation writes.
6. Every new SQL query carries `user_id` and `library_id` predicates; the CI script covers the new handlers.

## Risk assessment

**Hardest to get right:**
- Cross-user IDOR. The pattern failed across the entire reading API in phase 11 (AUDIT-0012-C1) and was only caught by an adversarial review. Every new query must be SQL-traced, not inferred from the repository interface.
- Cursor correctness under concurrent writes. Two devices posting progress simultaneously must both see a monotonically advancing cursor on their next poll, with no missed changes. The sequence-number implementation must be tested under concurrency.
- Revocation enforcement. A-13-06 was deferred twice. The test that proves FR-9 works is the one that actually calls the revoked device's token on a sync endpoint and verifies 401 — not a test that verifies the SyncMiddleware code path alone.

**Merely tedious:**
- Happy-path CRUD for the device list and revocation endpoints.
- Schema migration verification.
- OpenAPI contract test extension.

## Layers

### Unit

Tests for the `PairedDevice` domain extension:

| Test | Assertion |
|---|---|
| `AdvanceCursor`: forward advance | `sync_cursor` updates to new value, `last_synced_at` updates |
| `AdvanceCursor`: backward advance (newCursor ≤ current) | Returns an error; no mutation |
| `AdvanceCursor`: revoked device | Returns error (`ErrDeviceRevoked`); no mutation |
| `Touch()` integration | `lastSeenAt` advances; does not fail on non-revoked device |
| `ReconcileProgress` reuse | Already tested in `domain-reading.md`'s own unit suite — not duplicated here; referenced, not re-written |

### Integration

All integration tests run against a real PostgreSQL instance using the existing test harness (`backend-test-harness.md`).

**Migration 00011:**
- `paired_devices` table has `sync_cursor BIGINT NOT NULL DEFAULT 0` and `last_synced_at TIMESTAMPTZ` after migration.
- `reading_progress`, `bookmarks`, `highlights` tables each have `sync_sequence BIGINT` after migration.

**Device list endpoint (`GET /api/v1/devices`):**
- Returns only devices belonging to the authenticated user — user A's token returns user A's devices only.
- Cross-user IDOR test: user A's token with user B's device ID in the query → returns only user A's (possibly empty) list; user B's device does not appear.
- Active and revoked devices are both returned (revoked with non-null `revokedAt`).
- `lastSyncedAt` is null for a device that has never synced; non-null after a sync.

**Device revocation endpoint (`DELETE /api/v1/devices/{id}`):**
- Revokes the identified device; subsequent `GET /api/v1/devices` shows it with `revokedAt` set.
- Cross-user ownership test: user A tries to revoke user B's device → 404; user B's device is still active.
- Double-revoke → 409 Conflict.
- After revocation: `GET /api/v1/sync/reading` with the revoked device's auth token → 401.

**SyncMiddleware revocation enforcement:**
- Device active: sync request succeeds.
- Device revoked: `GET /api/v1/sync/reading` → 401 with `{"error": "device revoked"}`.
- Device revoked: `POST /api/v1/sync/progress` → 401.
- `Touch()` is called on every sync request for an active device (verify `last_seen_at` advances).

**Sync pull endpoint (`GET /api/v1/sync/reading`):**
- `since=0`: returns full dataset for authenticated user's active library; another user's data does not appear.
- `since=<cursor>`: returns only records with `sync_sequence > cursor`.
- Returned `cursor` equals `max(sync_sequence)` across returned rows.
- Subsequent call with the returned cursor: empty delta (no new writes since last pull).
- Retry with same `since` cursor: same response (idempotent).
- `AdvanceCursor` is called and `last_synced_at` advances.
- Cross-user IDOR: user A's token, library_id for user B's library → 403 (library claim check via AuthMiddleware).

**Sync progress post endpoint (`POST /api/v1/sync/progress`):**
- `Advanced` path: `ObservedEpoch` == stored `Epoch`, higher `Percentage` → canonical updated, cursor advances.
- `Unchanged` path: `ObservedEpoch` == stored `Epoch`, equal `Percentage` → no write, cursor advances.
- `Rejected` path: `ObservedEpoch` < stored `Epoch` → no write, cursor advances.
- `ObservedEpoch` above stored `Epoch`: clamped to stored value; cannot gain unfair win on `Percentage` comparison.
- Work ID not in user's library → 404 (no oracle on other users' works).
- Work ID from another user's library → 404.
- `Percentage` outside [0.0, 1.0] → 422.
- Malformed `PrecisePosition` CFI → 422.
- Cursor advances inside the same transaction as reconciliation write (verify atomicity by injecting a failure after the write and before the cursor update; cursor should not advance).

**Concurrent progress posts:**
- Two goroutines concurrently post different `ProgressReport`s for the same Work at the same epoch. Test asserts: exactly one canonical `ReadingProgress` row exists after both complete; its value equals the max of the two percentages; no panic; no deadlock (timeout check).

**CI script extension:**
- `scripts/check-user-scoped-reading.sh` extended to recognize sync handler source files; runs clean on the committed code.

### Contract

- `api/openapi.yaml` updated with four new paths:
  - `GET /api/v1/devices`
  - `DELETE /api/v1/devices/{id}`
  - `GET /api/v1/sync/reading`
  - `POST /api/v1/sync/progress`
- Existing contract test mechanism (`architecture-contracts.md` FR-3) extended to exercise the new paths.
- Contract test failures are CI-blocking.

### End to end

Playwright (or equivalent) two-session scenario:

1. **Basic sync:** Session A (device A) reads a Work to 40%. Session B (device B) polls sync → sees 40%. Pass.
2. **Forward progress wins:** Session B reads to 60% and posts. Session A polls → sees 60%. Pass.
3. **Offline conflict, higher wins:** Session A goes offline. A reads to 80%. B reads to 90% and posts. A reconnects and posts 80% → response `{"outcome": "Rejected"}`. A polls → sees 90%. Pass.
4. **Revocation:** Revoke device A via the device-management UI (or API call in the test). A's subsequent sync request → 401. Device A no longer appears in B's `GET /api/v1/devices` as active. Pass.

### Accessibility

Not applicable to this spec (backend only). The E2E tests for revocation use the API directly.
The accessibility tests for the device management UI belong to `frontend-device-management.md`'s
test plan (which cannot be written until that spec is unblocked by the missing canvas).

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| `POST /sync/progress` with Work ID from another user's library | 404 — no oracle |
| `POST /sync/progress` with `ObservedEpoch` = `stored Epoch + 100` | Clamped to stored Epoch; percentage comparison proceeds normally |
| `POST /sync/progress` with `Percentage = 1.5` | 422 Unprocessable Entity |
| `POST /sync/progress` with malformed CFI in `precisePosition.value` | 422 |
| `GET /sync/reading?since=-1` | 422 or treated as `since=0` (spec should state which; implementation must be consistent) |
| `GET /sync/reading?since=<far future>` | Empty delta; cursor returned unchanged |
| `DELETE /api/v1/devices/<nonexistent-id>` | 404 |
| `DELETE /api/v1/devices/<other-user's-device-id>` | 404; other device untouched |
| `DELETE /api/v1/devices/<already-revoked-id>` | 409 Conflict |
| Sync request from revoked device's token | 401 `{"error": "device revoked"}` |
| Request body > 64 KiB on `POST /sync/progress` | 413 / 400 (existing body-size limit middleware) |
| Two concurrent `POST /sync/progress` for same Work | Single canonical result; no panic; no LOST UPDATE |
| `GET /api/v1/devices` with a valid JWT for a user with no paired devices | `{"devices": []}` — empty list, not an error |

## Fixtures and test data

- Two test users (`user-sync-a`, `user-sync-b`) with separate libraries, created in the integration harness setup.
- Two `PairedDevice` rows, one per user, created via the existing `InsertProvisional` + `AssignOwnerByPairingSession` path.
- A set of `ReadingProgress`, `Bookmark`, and `Highlight` rows across two Works for user A, used for the sync-pull and cursor-delta tests.
- A synthetic `sync_sequence` sequence — deterministic across test runs (seeded in the test harness, not relying on production sequence state).
- No real user data, no copyrighted books, no real credentials. All fixture data is constructed in the test harness setup and torn down per-test or per-suite (existing test isolation pattern).

## What is deliberately not tested

- **Bookmark/highlight write-path sync** — out of spec scope (Non-goals). The sync pull returns the server's canonical bookmark/highlight state; posting new bookmarks via the sync transport is not built.
- **Push transport (SSE/WebSocket)** — not built in phase 14. Pull-only.
- **Full session invalidation on revocation** — the residual gap named in `backend-device-sync.md` Open questions. The test does verify that sync requests from a revoked device return 401; it does not and cannot verify that the device's access token is invalidated immediately (it isn't — that requires refresh token revocation which is deferred). This is stated explicitly, not silently skipped.
- **`ReadingPreferences` sync** — preferences are device-scoped and not synced across devices (domain-reading.md FR-5). No test for cross-device preferences sync is expected to exist.
- **Polling interval enforcement** — server-side. The spec does not mandate a polling rate; the test does not check one.

## Exit criteria

- [ ] Every functional requirement in `backend-device-sync.md` (FR-1 through FR-9) maps to at least one test
- [ ] Cross-user IDOR test for each of the three sync endpoints (progress post, sync pull, device list) passes
- [ ] Revocation enforcement test passes: revoked device → 401 on sync endpoints
- [ ] Concurrent progress post test passes without panic or deadlock
- [ ] `scripts/check-user-scoped-reading.sh` passes against all new handler source files
- [ ] All adversarial cases above have a test
- [ ] Tests were observed to fail before the implementation existed (RED step, per constitution §2)
- [ ] The suite is deterministic across repeated runs (no clock, no random, no network in unit/integration)
