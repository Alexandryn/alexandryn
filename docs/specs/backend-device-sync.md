# Spec: Backend device sync

| | |
|---|---|
| **Status** | `APPROVED` |
| **Phase** | `14-devices-and-sync` |
| **Author** | Claude (Sonnet 4.6 Thinking), for review by Luann Moreira |
| **Created** | 2026-09-05 |
| **Last updated** | 2026-09-05 |
| **Supersedes** | — |
| **Reviewed in** | Self-reviewed against maintainer-approved scope (phase 14 planning prompt); ADR 0029 Proposed |
| **Design reference** | N/A — no UI surface in this spec. Backend endpoints only. Canvas consulted for scope exclusion check: `Alexandryn-Electron-Admin.dc.html`, `Alexandryn-Web.dc.html`, synced 2026-08-13. No device-sync API screen appears in any canvas; the absence is expected for a backend-only spec. |

## Context

Phase 11 built `ReconcileProgress` (a pure domain function, fully tested, `domain-reading.md`
FR-6) and wired it to a single-device write path in `backend-reading-api.md`. Phase 13 built
the `PairedDevice` aggregate and its repository. Nothing connects the two: a LAN reader device
has no way to pull the current canonical reading state from the server, no way to post its own
progress report through a sync transport, and no endpoint through which the user can manage
(list or revoke) their devices.

ADR 0029 resolves the unresolved question from `domain-reading.md`'s Open Questions (review
`0049` finding 8): the server's canonical Work ID is the sync key (not a file hash or a
client-negotiated ID); per-device sync state (cursor + last-synced-at) extends `PairedDevice`
rather than living in a separate model.

## Problem

A LAN reader cannot sync its reading progress, bookmarks, or highlights with the server's
canonical state. The user cannot see or manage their paired devices through any API endpoint.
`FindByOwner` exists in the repository but is called from no handler.

## Goals

- A remote device can fetch the canonical reading state for all Works in its library (progress,
  bookmarks, highlights) with incremental fetch after the first full pull.
- A remote device can post a progress report; the server reconciles it via `ReconcileProgress`
  and returns the new canonical state and the device's updated sync cursor.
- The authenticated user can list their paired devices and revoke any of them through API
  endpoints.
- All reading-data endpoints in this spec enforce per-user + per-library scoping at the handler
  call and at the SQL predicate, SQL-traced.

## Non-goals

- Push transport (server-sent events, WebSocket). Deferred past phase 16 security hardening.
- Bookmark/highlight write-path sync (a device posting new bookmarks/highlights via the sync
  transport). Phase 11's existing bookmark/highlight endpoints remain the write path; sync
  delivers the server's canonical state, not a separate merge channel for annotations. If this
  is wanted later, it needs its own spec and ADR (the merge semantics for bookmarks are more
  complex than progress reconciliation).
- Sync for `ReadingPreferences`. Preferences are device-scoped (domain-reading.md FR-5), not
  synced across devices. Each device keeps its own.
- Library-visible finished/leaderboard view — explicitly deferred to phase 15.
- Device-to-device sync without server mediation.
- Any Alexandryn-operated relay or tunnel infrastructure.
- Mobile native app — out of scope for this entire roadmap.

## User stories

- As **a user reading on a phone**, I want my phone's progress to sync to the server so that
  when I pick up my tablet, it starts where the phone left off.
- As **a user who read offline on a tablet**, I want my offline progress to be accepted by the
  server when the tablet reconnects, unless I read even further on my phone in the meantime.
- As **a user**, I want to see which devices are paired to my account and remove one if I no
  longer use it.
- As **a user who revokes a device**, I want subsequent sync requests from that device to be
  refused.

## Functional requirements

### Device list and revocation

- **FR-1** `GET /api/v1/devices` MUST return the authenticated user's paired devices (active and
  revoked), owner-scoped: the handler MUST call `PairedDeviceRepository.FindByOwner` with the
  user ID resolved from the JWT, never a bare-list-all variant. The response MUST include for
  each device: `id`, `label`, `deviceClass`, `enrolledVia`, `createdAt`, `lastSeenAt`,
  `lastSyncedAt` (null if never synced), `revokedAt` (null if active).
- **FR-2** `DELETE /api/v1/devices/{id}` MUST revoke the identified device. The handler MUST
  verify that the device belongs to the authenticated user before revoking — call
  `FindByID` and assert `Owner() == requestUser`, return 404 on mismatch (not 403, no oracle on
  whether the device exists for another user). A device already revoked MUST return 409 Conflict.
- **FR-3** After revocation via FR-2, the device's `sync_cursor` is preserved on the
  `PairedDevice` row (for forensic/audit purposes) but subsequent sync requests carrying that
  device's token MUST be refused with 401 (the token's associated device is revoked, resolved
  in the sync middleware described in FR-9).

### PairedDevice schema extension (ADR 0029 Part 2)

- **FR-4** The `paired_devices` table MUST gain two new columns via a migration:
  - `sync_cursor BIGINT NOT NULL DEFAULT 0` — the server-side monotonic sequence number the
    device last acknowledged. Zero means the device has never synced.
  - `last_synced_at TIMESTAMPTZ` — nullable; the wall-clock time of the device's last
    successful sync pull. Updated in the same transaction as cursor advance.
  These columns MUST be added via a new migration (`00011_phase14_device_sync.sql`).
- **FR-5** A `PairedDeviceRepository.AdvanceCursor(ctx, deviceID, newCursor, now)` method MUST
  be added, executing `UPDATE paired_devices SET sync_cursor = $1, last_synced_at = $2 WHERE id = $3 AND revoked_at IS NULL`
  (refusing to advance a revoked device's cursor). This update MUST run inside the same
  transaction as the reconciliation write (ADR 0021's transaction contract).

### Sync transport

- **FR-6** `GET /api/v1/sync/reading?since=<cursor>` MUST return the canonical reading state for
  all Works in the authenticated user's active library, filtered to changes with
  `sync_sequence > since`. `since=0` or omitting `since` returns the full dataset (initial sync).
  The response MUST include:
  - `cursor`: the new cursor value the device MUST store and send on its next poll.
  - `progress`: array of `ReadingProgress` records (Work ID, Percentage, Epoch, PrecisePosition
    if present, DeviceID of the last writer, observedAt).
  - `bookmarks`: array of bookmark records for all works in the library.
  - `highlights`: array of highlight records for all works in the library.
  The handler MUST enforce `WHERE user_id = $1 AND library_id = $2` on every query (SQL-traced).
  It MUST call `PairedDeviceRepository.AdvanceCursor` with the returned `cursor` value inside
  the same transaction that reads the data (ensuring cursor and data are consistent).
- **FR-7** `POST /api/v1/sync/progress` MUST accept a `ProgressReport` (Work ID, Percentage,
  ObservedEpoch, optional PrecisePosition, DeviceID) and call `ReconcileProgress` on the
  current canonical `ReadingProgress` for that Work (or create a first-progress record if none
  exists — the same "first progress" path wired in phase 11's single-device flow). The request
  handler MUST:
  1. Resolve the authenticated user and active library from the JWT.
  2. Load `ReadingProgress` by `(user_id, library_id, work_id)` — never by work_id alone.
  3. Validate that the `Work` ID belongs to the user's library (user owns a `LibraryEntry` for
     that Work in the active library).
  4. Call `ReconcileProgress(canonical, incoming)` — a pure function, no I/O.
  5. Write the result if `Advanced`; no write if `Unchanged` or `Rejected`.
  6. Advance the device's `sync_cursor` inside the same transaction as step 5.
  7. Return the resulting canonical `ReadingProgress`, the reconciliation outcome tag
     (`Advanced`/`Unchanged`/`Rejected`), and the device's new `cursor`.
- **FR-8** The sync sequence number used as the cursor MUST be a PostgreSQL sequence
  (`sync_seq`), incremented by a trigger or by the application on every reading-data write
  (progress, bookmark, or highlight creation/deletion). Each reading-data row gains a
  `sync_sequence BIGINT` column (non-nullable, set on insert). The `GET /api/v1/sync/reading`
  query filters on `sync_sequence > $since` and returns `max(sync_sequence)` as the new cursor.
  Clock skew between devices does not affect correctness — the cursor is a sequence number, not
  a timestamp.

### Sync middleware and revocation enforcement (A-13-06)

- **FR-9** A `SyncMiddleware` MUST be wired on all `/api/v1/sync/*` and `/api/v1/devices/*`
  routes. In addition to the existing `AuthMiddleware` (token type, library claim), it MUST:
  1. Extract the `DeviceID` from the authenticated context (the device's `PairedDevice.ID`
     resolved from the JWT or a `X-Device-Id` header validated against the user's devices).
  2. Verify the device is not revoked: call `FindByID`, assert `RevokedAt() == nil`. Return 401
     `{"error": "device revoked"}` if revoked.
  3. Call `PairedDevice.Touch(now)` and persist the update (updating `last_seen_at` —
     the phase-13 deferred `Touch()` caller, per `device_pairing.go` line 479 comment).
  This resolves A-13-06: revocation now refuses sync requests from the revoked device's token.
  Existing access tokens remain valid until expiry (the fundamental limit named in A-13-06), but
  the sync path now enforces revocation at the per-request level for all sync and device-mgmt
  routes. This is not a complete session-invalidation (which would require revoking refresh
  tokens too — see Open questions) but closes the named sync-path gap.

## Non-functional requirements

- **Performance** — `GET /api/v1/sync/reading?since=<cursor>` MUST complete in ≤ 500ms p95
  for a library of 1000 works with 10 000 total sync-sequence rows. Enforce with an index on
  `sync_sequence` on each reading-data table.
- **Security** — see Security considerations below.
- **Accessibility** — not applicable; backend-only spec.
- **Reliability** — cursor advance is inside the same transaction as data reads (FR-6) and
  reconciliation writes (FR-7); partial failure leaves the device at its previous cursor,
  causing it to refetch on the next poll rather than miss changes.
- **Observability** — per-sync events logged at `info` (device ID, cursor before/after,
  outcome tag, row counts). No Work titles, highlight text, or positions in logs. Revocation
  events logged at `info` (device ID, revoked-at, revoking user).

## Domain model

**Extended** (per ADR 0029 Part 2):
- `PairedDevice` gains `sync_cursor` (int64, zero-value = never synced) and `last_synced_at`
  (*time.Time, nil = never synced). These are new accessor methods; existing accessors unchanged.
- New domain method `PairedDevice.AdvanceCursor(newCursor int64, at time.Time) error` — validates
  the device is not revoked, asserts `newCursor > current` (a cursor that goes backward is an
  internal error, not a normal path), updates both fields.

**New** (schema-only, no new domain type needed):
- `sync_sequence` column on `reading_progress`, `bookmarks`, `highlights` tables — a PostgreSQL
  sequence value written on every insert or update, used as the sync cursor baseline.
- `sync_seq` PostgreSQL sequence — a single, library-wide monotonic counter. Advances on every
  reading-data write. The `GET /api/v1/sync/reading` response carries `max(sync_sequence)` across
  the returned rows as the new cursor; a library with no data returns cursor `0`.

**Unchanged**: `ReconcileProgress`, `OverrideProgress`, `ReadingProgress`, `Bookmark`,
`Highlight`, `ReadingPreferences` — no domain logic changes.

## API and contracts

All endpoints require authentication (`AuthMiddleware`) plus `SyncMiddleware` (FR-9).

### `GET /api/v1/devices`

Response `200`:
```json
{
  "devices": [
    {
      "id": "string",
      "label": "string",
      "deviceClass": "phone|tablet|desktop|tv|unknown",
      "enrolledVia": "pairing_code|password_login",
      "createdAt": "2026-09-05T00:00:00Z",
      "lastSeenAt": "2026-09-05T00:00:00Z",
      "lastSyncedAt": "2026-09-05T00:00:00Z",
      "revokedAt": null
    }
  ]
}
```

### `DELETE /api/v1/devices/{id}`

Response `204` on success. `404` if not found or belongs to another user. `409` if already revoked.

### `GET /api/v1/sync/reading?since={cursor}`

`cursor` is an integer ≥ 0. `since=0` or absent returns full dataset.

Response `200`:
```json
{
  "cursor": 1234,
  "progress": [ /* ReadingProgress records */ ],
  "bookmarks": [ /* Bookmark records */ ],
  "highlights": [ /* Highlight records */ ]
}
```

### `POST /api/v1/sync/progress`

Request body:
```json
{
  "workId": "string",
  "percentage": 0.42,
  "observedEpoch": 3,
  "precisePosition": { "editionId": "string", "value": "epubcfi(/6/4[chap01]!/4/2/1:0)" },
  "deviceId": "string",
  "reportedAt": "2026-09-05T00:00:00Z"
}
```

Response `200`:
```json
{
  "outcome": "Advanced|Unchanged|Rejected",
  "progress": { /* canonical ReadingProgress */ },
  "cursor": 1235
}
```

Error `404` if Work not found in user's library. Error `422` if shape invalid.

All endpoints are added to `api/openapi.yaml`. Contract test via `architecture-contracts.md`
FR-3's mechanism, extended to the new paths.

## State transitions

```
Device cursor = 0 (created, never synced)
  → first GET /api/v1/sync/reading (since=0 or absent)
    → full dataset returned, cursor = max(sync_sequence) or 0 if empty
  → subsequent GET /api/v1/sync/reading?since=<cursor>
    → incremental delta; cursor advances

POST /api/v1/sync/progress (incoming ProgressReport)
  → ReconcileProgress(canonical, report)
    → Advanced: canonical updated, sync_sequence incremented, cursor returned
    → Unchanged: no write, cursor still advanced (observation acknowledged)
    → Rejected: no write, cursor still advanced (stale report acknowledged)

Device revoked via DELETE /api/v1/devices/{id}
  → subsequent sync requests from that device → 401 (SyncMiddleware FR-9)
```

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Work ID in ProgressReport not in user's library | FR-7 step 3: LibraryEntry lookup returns not-found | `404 Not Found` | Refuses reconciliation; no write. No oracle on whether the Work exists for another user. |
| `ObservedEpoch` below stored `Epoch` (stale device) | `ReconcileProgress` → `Rejected` | `{"outcome": "Rejected"}` with current canonical returned | No mutation. Device must advance its sync cursor and re-fetch before competing again. |
| Sync cursor goes backward (server internal error) | `AdvanceCursor` assertion | 500 with request ID | Logs at `error` with request ID and cursor delta; no cursor write. |
| Device revoked mid-session | `SyncMiddleware` FR-9 revocation check | `401 {"error": "device revoked"}` | Refuses sync and device-mgmt requests. Existing access token remains valid until expiry (named limitation — see A-13-06). |
| Two concurrent `POST /api/v1/sync/progress` for the same Work from different devices | `ReconcileProgress` called twice under a row lock on `reading_progress` (same transaction pattern as phase 11's single-device write, per ADR 0021) | Each gets a deterministic result; no data loss | The row lock serialises the two writes; `max` over `(epoch, percentage)` is applied to each in arrival order. Both devices see the same canonical value on their next pull. |
| Network timeout during sync pull | Client retries with the same `since` cursor | Stale UI until retry succeeds | Server is idempotent: re-running `GET /api/v1/sync/reading?since=<same>` returns the same data; cursor only advances on successful acknowledgement. |
| `sync_sequence` column missing (migration not applied) | Database error on insert | 500 | Application startup MUST assert all migrations are applied (existing startup check). |

## Security considerations

**Trust boundary: authenticated LAN client vs. server reading data.**

- **FR-1/FR-6 per-user + per-library scoping (AUDIT-0012-C1 discipline).**
  Every query in this spec MUST carry `WHERE user_id = $1 AND library_id = $2` in the SQL text.
  The CI script `scripts/check-user-scoped-reading.sh` MUST be extended to cover all new
  sync-path handlers and verified clean in the same commit that ships them. A device must not
  be able to read or write another user's sync state.
  Enforcement point for FR-6: `ReadingProgressRepository.FindByUserAndLibrary` (not a bare
  `FindByWorkID`); the handler passes `(userID, libraryID, workID)` — all three from the
  authenticated context. The SQL carries all three as parameters.
  Enforcement point for FR-7: `LibraryEntryRepository.ExistsForUserAndLibrary(userID, libraryID, workID)`
  must return true before any reconciliation runs. Cross-user IDOR test: user A posts a Work
  that belongs to user B's library; server returns 404.

- **FR-2 device ownership check.** `DELETE /api/v1/devices/{id}` calls `FindByID` and asserts
  `Owner() == requestUser`. The ownership check must be in the handler, not only in the
  repository method. The test that proves it: user A tries to revoke user B's device; server
  returns 404, and user B's device is still active in the database.

- **FR-9 revocation enforcement (A-13-06 resolution).** `SyncMiddleware` verifies
  `RevokedAt() == nil` on every sync request. The test that proves it: device revoked via
  FR-2, then `GET /api/v1/sync/reading` with that device's token returns 401.

- **Work ID validation (FR-7 step 3).** The Work ID in a `ProgressReport` is supplied by the
  client. It must be validated as an existing Work in the authenticated user's library — not
  only as a syntactically valid UUID. An attacker supplying a Work ID from another user's
  library must receive 404, not reconciliation output.

- **Epoch and ObservedEpoch.** The `Epoch` field is server-assigned and never accepted from the
  client. `ObservedEpoch` is accepted from the client but treated as an untrusted value —
  `ReconcileProgress` clamps an `ObservedEpoch` above the stored `Epoch` down to the stored
  value before comparison (per `domain-reading.md` FR-6). A client cannot advance its own
  `ObservedEpoch` to win a reconciliation it would otherwise lose.

- **Input validation.** `Percentage` must be in [0.0, 1.0] (domain validation, already exists).
  `PrecisePosition.Value` must pass the CFI shape check from `backend-reading-api.md` FR-4.
  `DeviceID` must be the device's own ID (the one in its JWT/session context), not an arbitrary
  UUID. Request body size limit: 64 KiB (consistent with existing reading API limits).

- **No reading data in logs.** Work titles, highlight text, bookmark labels, precise positions,
  and reading percentages MUST NOT appear in any log line from this spec's handlers, even
  indirectly. Constitution §8. Only IDs and outcome tags.

- **`sync_cursor` is not secret** — it is a monotonic sequence number, visible in API responses.
  An attacker knowing the cursor value learns nothing about the library's content.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | `PairedDevice.AdvanceCursor` — refusal on revoked device, refusal on backward cursor, correct field updates; `ReconcileProgress` reuse (already tested, not re-tested here) |
| Integration | `GET /api/v1/devices` returns user-scoped list; `DELETE /api/v1/devices/{id}` owner-check + revocation + 409 on double revoke; `GET /api/v1/sync/reading` full dataset on `since=0`, incremental delta on subsequent cursor, `WHERE user_id AND library_id` verified in SQL text; `POST /api/v1/sync/progress` Advanced/Unchanged/Rejected paths, cursor advances on all three, Work ownership check |
| Contract | New sync endpoints added to `api/openapi.yaml`; `architecture-contracts.md` FR-3 contract test extended |
| E2E | Two browser sessions (device A and device B) for same user; device A reads to 40%; device B polls, sees 40%; device B reads to 60%, posts; device A polls, sees 60%; offline scenario: A reads to 80% offline, B reads to 90% online and posts, A reconnects and posts 80% → Rejected (90% wins); A polls and sees 90% |
| Adversarial | Work ID in ProgressReport belongs to a different user → 404; `ObservedEpoch` higher than stored `Epoch` → clamped (no unearned win); revoked device sync request → 401; `since` cursor far in the future (beyond current max) → empty delta, new cursor = same value; `Percentage` outside [0.0, 1.0] → 422; `PrecisePosition` with malformed CFI → 422 |

## Acceptance criteria

- [ ] `GET /api/v1/devices` returns authenticated user's devices only — cross-user IDOR test passes (user B cannot see user A's devices)
- [ ] `DELETE /api/v1/devices/{id}` refuses to revoke another user's device (returns 404, device still active)
- [ ] After revocation, `GET /api/v1/sync/reading` from revoked device returns 401
- [ ] `GET /api/v1/sync/reading?since=0` returns full dataset; subsequent call with returned cursor returns only new changes
- [ ] `POST /api/v1/sync/progress` with `ObservedEpoch` equal to stored `Epoch` and higher `Percentage` → `Advanced`, canonical updated
- [ ] `POST /api/v1/sync/progress` with `ObservedEpoch` below stored `Epoch` → `Rejected`, canonical unchanged
- [ ] `POST /api/v1/sync/progress` with Work ID not in user's library → `404`
- [ ] Two concurrent `POST /api/v1/sync/progress` from different devices for the same Work produce a single canonical value equal to the max — no lost writes, no panic
- [ ] `sync_sequence` column present on `reading_progress`, `bookmarks`, `highlights` tables after migration `00011`
- [ ] `scripts/check-user-scoped-reading.sh` extended to sync endpoints and passes
- [ ] All new SQL queries in this spec are parameterized (100% — existing CI check passes)
- [ ] OpenAPI spec updated with new endpoints; contract test extended and passes

## Open questions

- **Refresh token revocation on device revoke** (A-13-06, inherited from phase 13). This spec
  closes the sync-path gap (FR-9: sync requests from a revoked device's token are refused).
  But the device's existing access token remains valid until its natural expiry — a revoked
  device can still make non-sync API calls until that token expires. A full session-invalidation
  would require a `device_id → refresh_token_jti` mapping and a token revocation list. This is
  a real residual gap. Decision: accept the same architectural deferral ADR 0028 §6 named, now
  with the sync-path gap closed. A future spec can add the mapping. Recorded here so it is not
  silently inherited a third time without acknowledgement.

- **Bookmark/highlight write-path sync.** A device that creates a bookmark while offline has no
  way to post it through the sync transport (Non-goals above). The current Non-goal is correct
  for phase 14 — the merge semantics are more complex than progress reconciliation and warrant
  their own spec. The gap is: a device that creates bookmarks while offline loses them if it is
  revoked before reconnecting. Named here, not fixed here.

- **Polling interval.** `GET /api/v1/sync/reading?since=<cursor>` is pull-based. The polling
  interval is a client concern; this spec does not mandate one. The `backend-device-sync.md`
  implementation may document a recommended default (e.g. 30 seconds) in an API comment, but
  does not enforce it server-side (no rate limiting specific to sync beyond the general rate
  limiter from phase 13).

## References

- ADR 0029 — cross-device identity agreement (this spec's load-bearing design)
- `domain-reading.md` FR-6/FR-7 — `ReconcileProgress` / `OverrideProgress` (used unchanged)
- `backend-reading-api.md` — the single-device write path this spec extends to multi-device
- `domain-device-pairing.md` — `PairedDevice` aggregate extended by FR-4/FR-5
- `backend-network-api.md` — existing device repository methods used by FR-1/FR-2
- ADR 0021 — transaction contract (cursor advance inside reconciliation transaction)
- ADR 0028 §6 — A-13-06 refresh token revocation deferral (carried forward in Open questions)
- Constitution §6 (no relay/tunnel), §8 (never log what someone reads)
