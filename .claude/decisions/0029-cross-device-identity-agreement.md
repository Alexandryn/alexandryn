# 0029. Cross-device identity agreement: how two devices establish they are reporting progress against the same Work/Edition

| | |
|---|---|
| **Status** | Proposed |
| **Date** | 2026-09-05 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

<!-- Status: Proposed | Accepted | Rejected | Superseded | Deprecated -->

## Context

`domain-reading.md` FR-6 (`ReconcileProgress`) is correct and fully implemented (phase 11).
Its commutativity and associativity proofs hold within a well-defined set: "both sides already
agree which Work/Edition a ProgressReport is about." That precondition is stated but never
built — it is the open question recorded in `domain-reading.md`'s Open Questions section, review
`0049` finding 8:

> `ReconcileProgress` assumes the system and every reporting device already agree on which
> Work/Edition a ProgressReport is about. Nothing built so far addresses how that agreement
> is reached, or what happens when it's asymmetric — one device's sync agreeing, another
> silently not, for what should be the same file.

The problem in concrete terms: a user has the same EPUB on two devices. One device imported it
from Source A; the other from Source B. Both have a `LibraryEntry` that resolved to the same
`Work` (via Open Library identity — ADR 0010). But a `ProgressReport` carries a `Work` ID, not
a file hash. If the identity resolution was correct on both devices, both `ProgressReport`s name
the same `Work` and `ReconcileProgress` works correctly. If the resolution was incorrect or
asymmetric — one device mapped the file to Work W1, the other to Work W2, for what the user
considers the same book — then:

- Reconciliation runs on two separate `ReadingProgress` rows, neither of which is "wrong"
  from the domain's perspective.
- The user sees stale progress on one device and no error anywhere.
- The failure mode is silent.

A secondary problem: `ProgressReport` carries an optional `PrecisePosition` tagged with an
`Edition` ID (CFI format, phase 11). Two devices may have imported different `Edition`s of the
same `Work` (e.g. different EPUB vendors, same text). The `PrecisePosition` from one Edition's
CFI addressing is meaningless on another. `domain-reading.md` FR-2 already handles this — a
`PrecisePosition` is Edition-tagged and is only applied when the user is reading that same
Edition; otherwise, `Percentage` serves as the fallback. So the Edition-level mismatch is
handled gracefully already. The Work-level mismatch is the actual unresolved gap.

There is also a mechanical question this ADR must also resolve: where does per-device sync state
(a cursor tracking which server-side changes a device has seen, allowing incremental fetch rather
than full-dataset polling) live? Options: extending `PairedDevice` with cursor fields, or a
separate `device_sync_state` model. This is recorded here because both questions concern what
a device "knows" about Works, and the answer to the identity question constrains the cursor model.

## Decision

### Part 1: Cross-device Work identity agreement

**We use the server's canonical Work ID as the sole sync key, and we rely on the import
pipeline's existing identity resolution (ADR 0010: Open Library work ID, or internal ID when no
external match exists) to be the ground truth both devices converge on.**

Concretely:

- A `ProgressReport` from any device carries the `Work` ID assigned by the *server* at import
  time. The server is the single source of truth for Work identity; a device does not invent
  or negotiate Work IDs.
- When a device imports a file, the server resolves its Work identity via `backend-import-pipeline.md`'s
  existing matching logic and returns the canonical `Work` ID. The device stores that ID and
  uses it on all subsequent progress reports. There is no client-side Work identity — only
  the server's resolved ID.
- If a user imports "the same book" on two devices via different Sources, both import jobs run
  on the same server. The import pipeline's deduplication logic (ADR 0010: ISBN/Open Library
  match → merge; no external match → separate internal IDs) determines whether they resolve to
  the same `Work`. If they do, `ReconcileProgress` runs correctly. If they don't — the pipeline
  judged them to be different Works — that is an import-identity problem, not a sync problem.
  The sync layer trusts the import layer's judgment.
- **What this does not fix:** two genuinely ambiguous imports (same ISBN, different publishers)
  that the pipeline resolves as the same Work when they are arguably different, or vice versa.
  That is the import-identity problem (ADR 0010's own known limitation: "internal-ID-primary;
  external references optional, never required"). Sync does not fix it; sync inherits it. The
  Open Questions section below names this explicitly so it is not rediscovered.

**Rationale for this option over the alternatives (see Options considered):** it does not
require a new content-hashing scheme (which would be a substantial new trust boundary — hashing
arbitrary user files server-side at sync time, not import time), it does not require devices
to negotiate identity among themselves (which would be architecturally novel and hard to audit),
and it correctly identifies the import layer as the right place to fix ambiguous identity —
which is already where the problem has always lived (ADR 0010).

### Part 2: Per-device sync state model

**We extend `PairedDevice` with sync cursor fields rather than introducing a separate model.**

A per-device sync cursor records the server's logical clock position (or a monotonic sequence
number) that a device has acknowledged. On each sync poll, the device sends its current cursor
value; the server returns all changes since that cursor; the device advances its cursor to the
new position on success.

We extend `PairedDevice` with two new fields:
- `sync_cursor` — a server-assigned opaque token (a monotonic sequence number or timestamp-based
  cursor) representing the most recent change this device has acknowledged.
- `last_synced_at` — the wall-clock timestamp of the device's last successful sync pull.
  Already implied by `PairedDevice.Touch()` (which phase 13 deferred for phase 14 with an
  explicit comment); `last_synced_at` formalizes that deferred use.

`sync_cursor` is initialized to zero/null on device creation. It advances only on a successful
sync response that the device acknowledges (not on the server's own writes, which are not a
"device has seen this" assertion).

**Rationale for extending `PairedDevice` over a separate model:** the sync state IS about this
device's relationship to the server's data — it is not a separate lifecycle from the device
itself. Keeping it on `PairedDevice` avoids a join on every sync poll, keeps revocation
semantically complete (revoking a `PairedDevice` row atomically removes both the device's
trust relationship and its sync cursor), and reflects the actual domain: a cursor with no device
is meaningless. The "two concerns on one aggregate" argument for separation is real but not
decisive here — `PairedDevice` is not carrying sync business logic, only two additional columns
of state. If this coupling creates pressure later (e.g. a sync cursor needed for a non-paired
entity), a separation can be made then with a migration; the reversal cost is low.

### Sync protocol shape (consequences of Part 1 + Part 2)

The sync transport that the `backend-device-sync.md` spec will build on top of this decision:

- **Pull-based polling, not push.** A LAN device polls `GET /api/v1/sync/reading?since=<cursor>`
  periodically. The server returns all progress/bookmark/highlight changes with a sequence number
  greater than `cursor`, along with the new cursor value to store. The polling interval is a
  client-side concern (the spec will set a recommended default); the server does not push.
  Server-sent events or WebSocket push are explicitly not adopted in this phase — they add a new
  stateful connection class to audit before phase 16's security hardening has run.
- **Progress write path:** a device posts `POST /api/v1/sync/progress` with its `ProgressReport`
  (carrying `Work` ID — assigned by the server at import, not invented by the client —
  `ObservedEpoch`, `Percentage`, optional `PrecisePosition`, `DeviceID`). The server runs
  `ReconcileProgress`. The response carries the resulting canonical `ReadingProgress` and the
  device's new sync cursor.
- **Cursor advance:** the server advances the device's `sync_cursor` on the `PairedDevice` row
  inside the same transaction as the reconciliation write. If the reconciliation produces
  `Rejected`, the cursor still advances — the device's observation is acknowledged even when its
  report did not win.

## Options considered

### Option A — Server-canonical Work ID as sync key (chosen, Part 1)

The server resolves Work identity at import time and the device uses that ID forever.

**Pros:** no new trust boundary; no client-side identity logic; revocation and identity are
both server-side decisions; correct for the majority case where both devices import from the
same or compatible sources through the same server.

**Cons:** does not fix genuinely ambiguous imports (same ISBN, different Works); the fix for
those lives in the import pipeline, not here.

### Option B — Content hash as sync key

Compute a stable fingerprint of the EPUB bytes (e.g. SHA-256 of a canonical subset) at import
time, and use that hash as the cross-device identity key in lieu of `Work` ID.

**Pros:** two devices with the same bytes always agree, regardless of import-pipeline metadata
resolution.

**Cons:** requires hashing arbitrary user files; adds a new file-content-processing trust
boundary that does not exist today; does not handle the case where the "same book" is two
different EPUB files with different bytes (corrected OCR, updated edition). Rejected: the cost
(a new hostile-input processing path, content-keyed storage, migration) is not justified by the
marginal improvement over Option A for the specific failure mode this ADR is resolving.

### Option C — Device-side identity negotiation

Devices exchange and negotiate Work identity before posting progress reports, similar to a
distributed merge protocol.

**Pros:** in theory, catches mismatches that the server's import pipeline produces.

**Cons:** architecturally novel; requires authenticated device-to-device communication (not built
and not in this phase's scope); adds a new security surface before phase 16 hardening; introduces
byzantine-fault-tolerant reasoning into what is currently a simple server-mediated system.
Rejected: out of scope and disproportionate to the problem.

### Option D — Separate `device_sync_state` model (alternative to Part 2)

Keep `PairedDevice` as the pairing bootstrap record only; add a new `device_sync_state` table
keyed on `device_id`.

**Pros:** cleaner concern separation; sync state is not the pairing layer's business.

**Cons:** a join on every sync poll; revocation must cascade to sync state via a foreign key
(doable but slightly more schema surface); the new table is trivially small and provides no
query or indexing advantage over two additional columns on `paired_devices`.
Rejected in favour of extending `PairedDevice` for this phase; the separation can be made if
needed later without a semantic change to the domain.

## Consequences

**Good** — the sync key question has a single, server-authoritative answer that requires no new
trust boundary and no protocol between devices. Per-device cursor state is co-located with the
device record, eliminating a join on the hot sync-poll path and making revocation atomic.

**Bad** — genuinely ambiguous import-identity cases (two imports the pipeline resolves as
different Works, that the user considers the same book) are not fixed by this decision — they
are correctly identified as an import-pipeline problem and left for ADR 0010 / the import
pipeline to address eventually. This ADR documents the inherited limitation, not a silent
omission.

**Neutral** — the sync transport shape (pull-based polling, `since=<cursor>` parameter)
follows from Part 2's cursor model without requiring a new protocol design.

## Open questions

- **Ambiguous import identity (inherited from ADR 0010).** Two imports that resolve to different
  Works for what a user considers the same book are a sync-invisible split — each device sees
  its own `ReadingProgress` record and does not know about the other. This phase does not fix
  it. A future improvement to the import pipeline (improved deduplication, a user-facing merge
  UI) is the appropriate fix. Flagged here so it is not rediscovered as a "sync bug."

- **Cursor implementation: sequence number vs. timestamp.** This ADR recommends a monotonic
  sequence number (e.g. a PostgreSQL sequence incremented on every reading-data write) over a
  wall-clock timestamp, because a sequence number is exact and not subject to clock skew.
  The `backend-device-sync.md` spec must choose the concrete implementation and record it.

## Reversal cost

**Part 1 (sync key = server Work ID):** Low. This is not a new constraint — the server has
always been the Work ID authority. Reversing to content-hash keying would require a migration
and a new file-processing pipeline, but no existing sync data would be corrupted; the cursor
would simply reset.

**Part 2 (cursor on PairedDevice):** Low-Medium. The two new columns can be extracted to a
separate table via a migration with no data loss. No application logic would need to change
beyond the query; the domain concept is unchanged.

## Confidence

Medium-High for Part 1: the server-as-identity-authority model follows directly from the
existing architecture (ADR 0010, import pipeline) and requires no new mechanisms. The inherited
limitation (ambiguous import identity) is real but pre-existing and correctly out of scope.

Medium for Part 2: the cursor-on-PairedDevice decision is a pragmatic call, not a derived
consequence. The "two concerns" argument for separation is real; this ADR bets that the coupling
is benign at this scale. Constitution §12: if this bet is wrong, it is cheap to reverse.
