# Spec: Frontend activity screen

| | |
|---|---|
| **Status** | `APPROVED` |
| **Phase** | `15-observability` |
| **Author** | Claude (Sonnet 5), approved by maintainer |
| **Created** | 2026-09-07 |
| **Last updated** | 2026-09-07 |
| **Supersedes** | — |
| **Reviewed in** | Self-review against Gate 1 maintainer-approved scope |
| **Design reference** | `Alexandryn-Electron.dc.html`, synced 2026-08-13 per `.design-reference/ANALYSIS.md` (screen `atActivity`). Acquisition tab is Binding. Reading tab is omitted per Gate 0 maintainer decision G0-1. Leaderboard UI is Unclassified (no canvas). |

## Context

Phase 04 established the React shell, design tokens, and primitive component library. Phase 05 built the Electron Desktop Host and route shell. Phase 09 added background jobs, and `backend-observability.md` (Phase 15) established the Activity event feed (`GET /api/v1/activity/events`) and job action endpoints.

The Desktop Host currently lacks a user interface to display what Alexandryn is acquiring or synchronizing from external sources. The design canvas `Alexandryn-Electron.dc.html` provides the authoritative visual layout for this screen under route state `atActivity`.

## Problem

Users on the desktop host cannot see the progress of active downloads, queued acquisitions, or failed ingestion jobs. When a source catalog synchronization fails or an EPUB download breaks, there is no interface to view the error, retry the operation, or navigate to source settings to correct credentials.

## Goals

- Implement the `ActivityScreen` component rendered on the Electron host at route `/activity`.
- Render the Acquisition queue divided into four sections matching the design canvas: ACTIVE, QUEUED, FAILED, and COMPLETED.
- Poll `GET /api/v1/activity/events` and active jobs on a 10-second interval with automatic re-fetch on window focus.
- Wire contextual job actions: Pause-all, Cancel queued items, Retry failed items, and Clear completed history.
- Render an orange indicator badge on the Activity navigation icon when active or failed items exist.
- Ensure full keyboard accessibility and screen-reader live announcements for state transitions.

## Non-goals

- Rendering the "Reading" tab: Gate 0 decision G0-1 mandates removing the Reading tab label entirely until design authority exists.
- Rendering a leaderboard or finished-book UI: no design canvas exists for the leaderboard (Unclassified gap). The leaderboard is backend-only in this phase.
- Supporting Activity UI on Web viewer or Mobile surfaces: `atActivity` is Host-only per ADR 0003 and `ANALYSIS.md`.

## User stories

- As a **library host user**, I want to see currently downloading books and their percentage completion, so that I know what Alexandryn is fetching.
- As a **library host user**, I want to see failed imports with clear explanations and a direct link to fix the source, so that I can resolve credential or network errors promptly.
- As a **library host user**, I want to pause active downloads or clear completed records, so that I can manage host bandwidth and keep the view clean.
- As a **keyboard or screen-reader user**, I want all activity rows and actions to be keyboard-navigable with status updates announced, so that I can operate the screen without a mouse.

## Functional requirements

### Route and Navigation

- **FR-1** The system MUST render `ActivityScreen` when the application route is `/activity` on the Desktop Host surface. The route MUST be restricted to the desktop host application shell and MUST NOT be accessible on remote web viewer sessions.
- **FR-2** The sidebar navigation item for Activity MUST display a small orange indicator dot when there is at least one item in the ACTIVE or FAILED state. When all items are completed or queued, or the queue is empty, the dot MUST NOT be rendered.
- **FR-3** The sidebar badge dot MUST have an accessible text description (e.g., `aria-label="Activity — action required"`).

### Layout and Sections

- **FR-4** The screen layout MUST conform to `Alexandryn-Electron.dc.html` (`atActivity`), displaying:
  - Header: "Activity" (heading 1).
  - Subtitle: "What Alexandryn is fetching from your sources, and what you have been reading."
  - **No Reading tab:** Per decision G0-1, the secondary "Reading" tab header MUST NOT be rendered. Only the Acquisition content is displayed.
- **FR-5** The Acquisition view MUST render four sections in top-to-bottom order:
  1. **ACTIVE** section: rendered when active acquisitions exist. The section header MUST include a "Pause all" text action button.
  2. **QUEUED** section: rendered when queued jobs exist.
  3. **FAILED** section: rendered when failed or dead-lettered jobs exist.
  4. **COMPLETED** section: rendered when completed jobs exist within the retention window. The section header MUST include a "Clear" text action button.
- **FR-6** If all four sections are empty, the view MUST render an empty state stating: "No recent activity. Books you import or acquire from sources will appear here."

### Item Row Representation

- **FR-7** Each row in the **ACTIVE** list (`acqActive`) MUST render:
  - Book cover thumbnail (or fallback generated cover using `frontend-generated-covers.md`).
  - Work title and author text.
  - Source display name badge in monospace font.
  - Format badge (`EPUB`, `PDF`, etc.).
  - Progress bar showing percentage width (`w`), accompanied by percentage text and byte progress label (e.g., `62% · 2.1 MB of 3.4 MB`).
  - Action button labeled "Pause".
- **FR-8** Each row in the **QUEUED** list (`acqQueued`) MUST render:
  - Small cover thumbnail, work title, source name, and format badge.
  - Status text (e.g., `Waiting for source`, `Queued · position 2`).
  - Action text button labeled "Cancel".
- **FR-9** Each row in the **FAILED** list (`acqFailed`) MUST render:
  - Cover thumbnail, work title, source name.
  - Red error dot indicator followed by sanitized error description (e.g., `Source unavailable · access key expired`).
  - Link button labeled "Fix source" that navigates the user directly to `/sources`.
  - Action button labeled "Retry".
- **FR-10** Each row in the **COMPLETED** list (`acqDone`) MUST render:
  - Small cover thumbnail, work title, source name, format badge.
  - Green success dot indicator followed by relative completion text (e.g., `Added to library · 2 hours ago`).
  - Link button labeled "Read" that navigates to `/book/:id` or opens the reader.

### Actions and Data Synchronization

- **FR-11** Data fetching MUST use TanStack Query querying `GET /api/v1/activity/events` and active jobs. The query MUST poll on a 10-second interval (`refetchInterval: 10000`) and revalidate on window focus (`refetchOnWindowFocus: true`).
- **FR-12** Triggering "Pause all" MUST issue a `POST /api/v1/activity/pause-all` request and invalidate the activity query on success.
- **FR-13** Triggering "Cancel" on a queued item MUST issue `POST /api/v1/activity/jobs/:id/cancel` and optimistically update the item to removed or dead-letter state.
- **FR-14** Triggering "Retry" on a failed item MUST issue `POST /api/v1/activity/jobs/:id/retry`, transition the item to queued status, and trigger a query refresh.
- **FR-15** Triggering "Clear" on completed items MUST issue `POST /api/v1/activity/jobs/clear-completed` and remove cleared items from the local view.

## Non-functional requirements

- **Performance:** Component initial mount and re-renders MUST complete within 16 ms (60 fps) under a list of 50 items. Image covers MUST use lazy loading.
- **Security:** Error messages from the backend MUST be treated as untrusted text and sanitized against HTML injection. The component MUST NOT display raw stack traces, filesystem paths, or credentials.
- **Accessibility:**
  - Full keyboard focus order: Tab navigates through rows and actionable buttons ("Pause all", "Pause", "Cancel", "Fix source", "Retry", "Clear", "Read").
  - All interactive elements MUST have visible focus rings using design system tokens (`--ac`, `--acs`).
  - A visually hidden region with `aria-live="polite"` MUST announce job completions or failures as polling updates arrive.
  - Section headers MUST use standard semantic elements (`<h2>`).
- **Reliability:** If the activity API returns an error or network is lost, the component MUST render an inline error banner with a "Retry connection" button without crashing the application shell.
- **Observability:** Action failures (e.g., failed job retry or pause) MUST be logged to the console at `warn` level with the associated error code and request ID.

## Domain model

Frontend state types:
```typescript
interface ActivityItem {
  id: string;
  workId?: string;
  title: string;
  author?: string;
  sourceName: string;
  format: string;
  status: 'active' | 'queued' | 'failed' | 'completed';
  progressPercent?: number;
  progressText?: string;
  errorMessage?: string;
  completedAt?: string;
  coverUrl?: string;
}

interface ActivityState {
  active: ActivityItem[];
  queued: ActivityItem[];
  failed: ActivityItem[];
  completed: ActivityItem[];
  isLoading: boolean;
  isPaused: boolean;
}
```

## API and contracts

The component consumes the endpoints defined in `backend-observability.md`:
- `GET /api/v1/activity/events?limit=50`: returns recent system events.
- `POST /api/v1/activity/pause-all`: pauses ingestion workers.
- `POST /api/v1/activity/jobs/:id/cancel`: cancels queued work.
- `POST /api/v1/activity/jobs/:id/retry`: restarts failed work.
- `POST /api/v1/activity/jobs/clear-completed`: clears completed records.

## State transitions

Row state lifecycle in UI:
```
[QUEUED] ──(worker claims)──> [ACTIVE] ──(success)──> [COMPLETED] ──(clear)──> (removed)
   │                              │
(cancel)                       (error)
   ▼                              ▼
(removed)                     [FAILED] ──(retry)──> [QUEUED]
```

## Failure modes

| Failure | User sees | Recovery |
|---|---|---|
| Background API returns 500 | Inline error banner: "Unable to load activity" with "Retry" button | Polling pauses; clicking Retry triggers manual refetch |
| Job retry action fails | Toast alert: "Could not retry job. Please check source connection." | Item remains in FAILED section |
| Job cancel action fails | Toast alert: "Could not cancel job." | Item remains in current state |
| Image cover fails to load | Fallback procedural book cover rendered | Automatic degradation via `frontend-generated-covers.md` |

## Security considerations

- **Authorization:** Only administrators have access to `/activity`. Non-admin sessions attempting to load the route are redirected to `/library` by route guards.
- **Cross-Site Scripting (XSS):** Book titles, author names, source names, and error strings are escaped via standard React JSX text nodes.
- **Path and Secret Leakage:** Error strings are checked for file paths; only sanitized server error text is shown.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Data transformation from backend events into `ActivityState` categories. |
| Component | Rendering of ACTIVE, QUEUED, FAILED, and COMPLETED sections with mock fixtures. Empty state rendering. |
| User Interaction | Clicking "Pause all", "Cancel", "Retry", and "Clear" dispatches correct HTTP mutations. |
| Accessibility | Automated axe-core sweep verifying zero a11y violations; keyboard tab order traversal; live region announcement verification. |

## Acceptance criteria

- [ ] `ActivityScreen` renders on Desktop Host at `/activity` matching the canvas layout.
- [ ] Acquisition queue displays ACTIVE, QUEUED, FAILED, and COMPLETED sections correctly.
- [ ] Reading tab label is omitted entirely from the component (G0-1).
- [ ] Polling runs every 10 seconds and refreshes on window focus.
- [ ] "Pause all" calls `/api/v1/activity/pause-all`.
- [ ] "Cancel" calls `/api/v1/activity/jobs/:id/cancel`.
- [ ] "Retry" calls `/api/v1/activity/jobs/:id/retry`.
- [ ] "Clear" calls `/api/v1/activity/jobs/clear-completed`.
- [ ] "Fix source" button navigates to `/sources`.
- [ ] Nav badge displays orange dot when active or failed items exist with accessible label.
- [ ] Keyboard navigation and axe-core accessibility tests pass without violations.

## Open questions

- **Optimistic UI updates vs server refetch:** Cancelling a job immediately removes it locally, but network failure will restore it on next poll. This optimistic behavior is accepted.

## References

- `Alexandryn-Electron.dc.html`: screen `atActivity`
- `docs/specs/backend-observability.md`: backend API contract
- `.design-reference/ANALYSIS.md`: sync date 2026-08-13
- Constitution §7: Accessibility is part of done
