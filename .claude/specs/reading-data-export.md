# Spec: Reading data export

| | |
|---|---|
| **Status** | `APPROVED` (drafted and self-reviewed 2026-09-01 against maintainer-approved scope; scope agreed in the phase 11 Gate 1 exchange) |
| **Phase** | `11-reader` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-09-01 |
| **Last updated** | 2026-09-01 |
| **Supersedes** | — |
| **Reviewed in** | Self-review 2026-09-01 (phase 11 Gate 1). No independent review cycle — a small, read-only sibling spec added mid-phase; folded into the phase 11 security audit (`0011`) instead. |
| **Design reference** | `Alexandryn-Web.dc.html` `atReader` block, `.design-reference/ANALYSIS.md` synced 2026-08-13 (scope pass 2026-08-17). The canvas carries a Marks-panel "Export" control (unwired placeholder); this spec is its read side. No separate captured export screen exists — the trigger is a control on the already-captured reader chrome. |

## Context

Phase 11 is the first phase to persist reading data for real —
`ReadingProgress` (per `Work`), `Bookmark`/`Highlight` (per `Edition`),
`ReadingPreferences` (per `DeviceID`). The `atReader` design canvas puts
an "Export" control in the Marks panel, and its own empty-state copy
tells the user *"Marks are stored with the library, not in this
browser."* Users reasonably expect to be able to get their own reading
history and annotations back out — as a portable file, for backup or for
feeding another tool.

No roadmap phase currently claims data portability or backup;
`architecture-persistence.md` explicitly leaves backup/restore ownership
open. This spec takes the narrow, read-only slice that belongs with the
data model landing now, and leaves the rest for a later, dedicated
phase.

## Problem

There is no way to get a copy of the user's reading progress, bookmarks,
and highlights out of the system.

## Goals

- A single endpoint that returns the user's reading data — progress,
  bookmarks, highlights — as one versioned JSON document
- A stable, documented JSON schema, versioned so a future import path and
  third-party integrations have a contract to build against
- A web-client action that saves that document as a file

## Non-goals

- **Import** — reading an export document back in. That is hostile-input
  territory (constitution §4), needs dedup and conflict resolution that
  overlaps phase 14's `ReconcileProgress` work, and is deferred to a
  later phase.
- **Exporting `ReadingPreferences`** — device-local rendering settings
  are not reading history; they carry no value in a portable export and
  `domain-reading.md` FR-5 explicitly forbids cross-device preference
  inheritance. Out of scope.
- **Exporting book files** — this endpoint never serves EPUB bytes.
  `backend-reader-content.md` is the only path that serves content, and
  only sanitised, per-resource, for rendering.
- **A desktop-native "Save as…" dialog** — writing a file to an
  arbitrary path from the Electron window crosses the preload privilege
  boundary (constitution §5) and needs an enumerated IPC operation.
  Deferred; the web/LAN client's in-browser download is what ships here.
- **Scheduled or automatic exports** — this is a user-triggered,
  on-demand pull only.
- **Per-collection or per-shelf filtering** — the export is the user's
  whole reading dataset or a single `Work`'s slice (FR-2), nothing finer.

## User stories

- As **someone who has read and annotated many books**, I want to
  download my highlights and notes as a file, so I have my own copy
  independent of this server.
- As **someone who might move to a different setup later**, I want my
  reading progress in a documented, versioned format, so a future import
  or migration tool has something stable to read.
- As **someone building a personal tool**, I want the export to be plain
  JSON with a schema version, so I can parse it without reverse-
  engineering.

## Functional requirements

- **FR-1** `GET /api/v1/reading/export` returns a single JSON document
  containing **every** `ReadingProgress`, `Bookmark`, and `Highlight` in
  the library, with a top-level envelope: `schemaVersion` (integer,
  currently `1`), `exportedAt` (RFC 3339 UTC timestamp, current server
  time), and the data arrays (FR-3). `Content-Type: application/json`.
  The response is served with `Content-Disposition: attachment;
  filename="alexandryn-reading-export.json"` so a browser treats it as a
  download.
- **FR-2** `GET /api/v1/reading/export?workId=:workId` returns the same
  envelope scoped to one `Work` — that `Work`'s `ReadingProgress` (or
  `null`), and the `Bookmark`s/`Highlight`s of every `Edition` belonging
  to that `Work` (`domain-bibliographic.md`'s `Edition.WorkID`). An
  unknown `workId` returns `404 NotFound`. A malformed `workId` returns
  `400 InvalidInput`.
- **FR-3** The document body shape (`schemaVersion: 1`):

  ```json
  {
    "schemaVersion": 1,
    "exportedAt": "2026-09-01T12:00:00Z",
    "works": [
      {
        "workId": "wrk_...",
        "progress": {
          "percentage": 0.42,
          "epoch": 3,
          "precisePosition": { "editionId": "edn_...", "cfi": "epubcfi(/6/4!/4/2)" },
          "observedAt": "2026-08-30T09:15:00Z"
        }
      }
    ],
    "editions": [
      {
        "editionId": "edn_...",
        "bookmarks": [
          { "id": "bmk_...", "cfi": "epubcfi(/6/4!/10)", "label": "the turn", "createdAt": "2026-08-28T20:00:00Z" }
        ],
        "highlights": [
          { "id": "hlt_...", "startCfi": "epubcfi(/6/4!/10/1:0)", "endCfi": "epubcfi(/6/4!/10/1:88)", "note": "", "category": "blue", "createdAt": "2026-08-28T20:01:00Z" }
        ]
      }
    ]
  }
  ```

  `progress` is `null` for a `Work` with no `ReadingProgress` row (only
  possible in the `?workId=` form; the unfiltered form omits such `Work`s
  entirely). `precisePosition` is `null` when the `ReadingProgress` has
  none. `works` and `editions` are each sorted by their ID for a
  deterministic, diff-friendly document.
- **FR-4** `Bookmark` and `Highlight` records MUST carry a `createdAt`
  timestamp. The existing `bookmarks` and `highlights` tables
  (`00002_phase02_schema.sql`) have no such column — migration
  `00008_phase11_reader.sql` adds `created_at TIMESTAMPTZ NOT NULL
  DEFAULT now()` to both. `domain.Bookmark` and `domain.Highlight` gain a
  `CreatedAt` field, set at construction (`backend-reading-api.md`
  FR-6/FR-7's `POST` handlers pass the current server time).
- **FR-5** The endpoint is **read-only** — it performs no writes, opens
  no source connections, and never touches `backend-reader-content.md`'s
  content path. It is a straight read of three repositories, composed
  into the envelope.
- **FR-6** The response body is bounded. If the assembled document would
  exceed a fixed cap (**25 MiB** — a reasoned placeholder, flagged in
  Open questions; a library with hundreds of heavily-annotated books
  stays well under it), the endpoint streams what it can and returns
  `500` rather than buffering an unbounded document in memory — the same
  "bound every response" discipline the rest of the backend applies.
  Under normal library sizes this never triggers.

## Non-functional requirements

- **Performance** — three indexed table scans (`reading_progress` by
  nothing / all rows, `bookmarks` and `highlights` all rows, or filtered
  by `Work`/`Edition` for FR-2). No per-row source calls, no content
  parsing. Expected to complete well within the standard request
  timeout for any realistic library.
- **Security** — see dedicated section.
- **Accessibility** — the web-client trigger is a normal button with an
  accessible name ("Export marks and progress"); the download itself is
  browser-native. No new keyboard-map entry beyond the button being in
  the Marks panel's focus order (`frontend-reader.md` FR-7 / NFR
  accessibility).
- **Reliability** — a read-only endpoint; nothing to recover on restart.
  A failed export leaves no state behind.
- **Observability** — a completed export logs at `info` with the request
  ID, whether it was scoped (`workId` present or not), and the record
  **counts** (works / bookmarks / highlights) — never a title, CFI,
  label, note, or percentage (constitution §8, `domain-reading.md`
  Security considerations). The counts are an operational signal, not
  reading content.

No new dependency (constitution §9) — standard-library JSON encoding
over the existing repositories.

## Domain model

No new domain types. `Bookmark` and `Highlight` gain a `CreatedAt
time.Time` field (FR-4). This spec reads `domain-reading.md`'s
`ReadingProgress`/`Bookmark`/`Highlight` and
`domain-bibliographic.md`'s `Edition`→`Work` relation; it introduces no
persistence and no events. The export document is a transport DTO
assembled in `internal/reader/api`, normalised at that boundary
(constitution §3) — no repository row shape reaches the response.

## API and contracts

`internal/reader/api` (alongside `backend-reading-api.md`'s handlers).

- `GET /api/v1/reading/export` → `200` (JSON document, FR-3;
  `Content-Disposition: attachment`) → `500` (document over the FR-6 cap)
- `GET /api/v1/reading/export?workId=:workId` → `200` (scoped document)
  → `400` (`InvalidInput`, malformed `workId`), `404` (`NotFound`,
  unknown `workId`), `500` (over cap)

Added to `api/openapi.yaml` and covered by
`architecture-contracts.md` FR-3's contract test, same as the rest of
`/api/v1/reading*`. The `schemaVersion: 1` document shape is part of the
contract — a change to it is a `schemaVersion` bump, not an in-place
edit.

## State transitions

Not applicable — every request is independent and read-only.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Malformed `workId` | FR-2 validation | `400 InvalidInput` | No read |
| Unknown `workId` | `Work` lookup miss | `404 NotFound` | No read |
| Assembled document exceeds the FR-6 cap | Byte count during encoding | `500` (a rare, large-library edge; retry with `?workId=` scoping suggested in the error message) | Aborts; no partial file is treated as complete by the client (the `500` status prevents the browser saving it as a valid download) |
| A repository read fails mid-assembly | Error from the repo | `500` (`backend-errors-and-logging.md`'s generic shape) | No partial document returned |
| Export requested for an empty library | Normal | `200` with empty `works`/`editions` arrays and a valid envelope | Not an error |

## Security considerations

- **Trust boundary** — a LAN/loopback client requests its own library's
  reading data. Same loopback-trust model as every other phase-11
  endpoint (no authentication until phase 12); no `DeviceID` header
  required (the export is library-wide reading data, not device-scoped —
  and `ReadingPreferences`, the only device-scoped type, is deliberately
  excluded).
- **Information disclosure** — the export *is* a disclosure of reading
  content by design: it is the user pulling their own data. The control
  that matters is that it never reaches a **log**: FR-6's observability
  rule logs counts only. The `Content-Disposition: attachment` header
  and the JSON content type reduce the chance of the document being
  rendered inline somewhere it could leak into a referrer or a browser
  history preview.
- **Resource exhaustion** — FR-6's response cap and the read-only,
  no-source-calls design bound the work. No user input widens the query
  beyond an optional single `workId`.
- **Injection / traversal** — `workId` is validated shape-wise and used
  only as a parameterised query argument
  (`scripts/check-parameterized-queries.sh`); it never forms a path or a
  file name (the download filename is fixed, FR-1).
- **No write path** — the endpoint cannot mutate anything, so it adds no
  tampering or elevation surface.
- **CSRF** — a `GET` with no side effects; nothing to protect. When
  phase 12 adds sessions, this endpoint is read-only and needs no CSRF
  token, but does become auth-gated like everything else.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Envelope assembly from fixture repository data — sorting determinism, `null` progress, `null` precisePosition, empty library; the FR-6 byte-cap boundary |
| Integration | `GET /api/v1/reading/export` against a real PostgreSQL instance seeded with progress + bookmarks + highlights across several works/editions — full document and `?workId=` scoped document; `created_at` present and populated (FR-4); a test asserting **no** title / CFI / label / note / percentage appears in captured log output, only counts |
| Contract | `architecture-contracts.md` FR-3's `kin-openapi` tool, extended to `/api/v1/reading/export`; the `schemaVersion: 1` body shape asserted |
| E2E | Create a bookmark and a highlight through the reading API, then `GET .../export` → both appear in the document with correct CFIs and `createdAt`; the web-client "Export" button produces a downloaded file with the expected JSON |
| Accessibility | The Export button has an accessible name and is in the Marks panel's focus order (`axe`, keyboard) |

Tests that must fail before implementation begins: a test asserting the
export document contains a created bookmark and highlight with their
CFIs and `createdAt`; a test asserting `?workId=` scoping returns only
that Work's data; a test asserting no reading content (only counts)
appears in logs; a test asserting the `schemaVersion` field is present
and equals `1`.

## Acceptance criteria

- [ ] `GET /api/v1/reading/export` returns a valid `schemaVersion: 1`
      document with all progress, bookmarks, and highlights
- [ ] `?workId=` scoping returns exactly that Work's slice; unknown
      `workId` is `404`, malformed is `400`
- [ ] `Bookmark` and `Highlight` carry a populated `createdAt` (migration
      `00008` + domain field)
- [ ] The document is deterministically ordered (diff-friendly)
- [ ] No reading content appears in logs, proven by a test — only counts
- [ ] The response carries `Content-Disposition: attachment` and
      `application/json`
- [ ] The contract test passes against `/api/v1/reading/export`
- [ ] The web client's "Export" control downloads the document as a file
- [ ] The endpoint performs no writes and opens no source connections,
      proven by a test

## Open questions

- **FR-6's 25 MiB response cap** — a reasoned placeholder, not
  load-tested against a real large annotated library; revisit if a real
  library approaches it, alongside whether the endpoint should paginate
  or stream NDJSON instead of one document.
- **Import** — deliberately deferred to a later, dedicated phase (near
  phase 14, since it shares conflict-resolution concerns). The
  `schemaVersion` envelope exists now specifically so that phase has a
  stable contract to read.
- **Desktop-native save dialog** — the Electron window currently gets the
  same in-browser download as the web client. A native "Save as…" flow
  through an enumerated preload operation is deferred (constitution §5).
- **`ReadingPreferences` in a future full backup** — excluded here on
  purpose, but a whole-instance backup (a different, later phase) might
  legitimately include them per-device; not this spec's call.

## References

- `domain-reading.md` (phase 02) — `ReadingProgress`/`Bookmark`/
  `Highlight` types exported here; `CreatedAt` field added by FR-4
- `backend-reading-api.md` (phase 11, sibling) — shares
  `internal/reader/api`; its `POST` handlers set the new `CreatedAt`
- `backend-reader-content.md` (phase 11, sibling) — explicitly **not**
  used; this endpoint never serves book content
- `domain-bibliographic.md` (phase 02) — `Edition`→`Work` relation FR-2's
  scoping walks
- `architecture-contracts.md` (phase 01) FR-3 — contract test extended
- `architecture-persistence.md` (phase 01) — notes backup/restore
  ownership as open; this spec is a narrow read-only slice, not that
- `.design-reference/Alexandryn-Web.dc.html` `atReader` — the Marks-panel
  "Export" control this spec is the read side of
- Constitution §3 (normalise at the boundary), §4 (validate `workId`),
  §8 (reading content never logged), §9 (no new dependency)
