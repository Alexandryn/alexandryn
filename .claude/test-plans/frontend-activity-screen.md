# Test plan: Frontend activity screen

| | |
|---|---|
| **Spec** | `.claude/specs/frontend-activity-screen.md` |
| **Status** | `DRAFT` |
| **Created** | 2026-09-07 |

## What we are trying to be confident about

1. `ActivityScreen` renders correctly across ACTIVE, QUEUED, FAILED, and COMPLETED states matching the design reference `atActivity`.
2. The "Reading" tab is completely omitted from the rendered DOM per decision G0-1.
3. The sidebar navigation badge reflects live state (orange dot when active or failed items exist) and provides an accessible name.
4. User actions (Pause all, Cancel, Retry, Clear, Fix source) trigger the correct backend mutations and update local state cleanly.
5. All interactive elements are fully keyboard-navigable with visible focus indicators, and state transitions are announced via an `aria-live` polite region.
6. Component handles loading, error, and empty states without unhandled exceptions or shell disruption.

## Risk assessment

**Hardest to get right:**
- Accessibility and live announcements. Dynamic polling updates must not disorient screen-reader users or steal keyboard focus while navigating job lists.
- Optimistic action updates. Cancelling or clearing completed jobs must remain responsive while gracefully recovering if the network call fails.
- Strict design conformance. Visual hierarchy, badges, progress widths, and section dividers must match `Alexandryn-Electron.dc.html` without inventing elements.

**Merely tedious:**
- Mocking TanStack Query polling intervals and query caches in Vitest/Testing Library.
- Empty state rendering across each section independently.

## Layers

### Unit

- Data transformation utilities: mapping raw `system_events` and job API payloads into categorized `ActivityState` slices (`active`, `queued`, `failed`, `completed`).
- Progress string formatting: tests asserting percentage calculation and byte formatting (`2.1 MB of 3.4 MB`).

### Component

Component tests using React Testing Library and Vitest:
- **Full Activity View:** Renders all four sections populated with mock items; asserts presence of progress bars, format badges, and action buttons.
- **Empty State:** Renders single empty message when no items exist across all categories.
- **Partial Sections:** Asserts sections with zero items are omitted or collapsed appropriately.
- **Reading Tab Absence:** Explicit assertion that no element with text "Reading" or tab role exists in the component hierarchy.
- **Nav Badge:** Asserts badge dot is rendered when `active.length > 0` or `failed.length > 0`, and omitted when only queued/completed items exist.

### User Interaction

- Clicking "Pause all" sends `POST /api/v1/activity/pause-all` and updates UI state.
- Clicking "Cancel" on a queued item issues `POST /api/v1/activity/jobs/:id/cancel`.
- Clicking "Retry" on a failed item issues `POST /api/v1/activity/jobs/:id/retry`.
- Clicking "Clear" issues `POST /api/v1/activity/jobs/clear-completed`.
- Clicking "Fix source" triggers route navigation to `/sources`.

### Accessibility

- **Automated axe-core scan:** `axe(container)` reports zero violations for WCAG 2.1 AA.
- **Keyboard navigation:** Tab traversal visits every action button in logical top-to-bottom order without focus traps.
- **Live region:** Verifies updates to jobs trigger text node injection in `<div aria-live="polite">`.
- **Accessible names:** Every icon-only button and badge dot has an `aria-label` or `aria-labelledby` attribute.

## Adversarial cases

| Scenario | Expected behaviour |
|---|---|
| Background activity API returns 500 | Inline error banner rendered with retry button; app does not crash |
| Backend returns malformed job payload (null title or corrupt JSON) | Defensive fallback renders "Unknown work" without throwing error |
| Job title contains XSS script tags | Text escaped as plain string; script does not execute |
| User rapidly clicks "Retry" multiple times | Button disabled while mutation is pending; single request dispatched |
| Rapid stream of completion events | `aria-live` region debounces announcements to avoid flooding screen reader |

## Fixtures and test data

- Mock activity event fixtures (`mockActiveJobs`, `mockQueuedJobs`, `mockFailedJobs`, `mockCompletedJobs`).
- Synthetic job IDs matching `TEXT` UUID shape.

## What is deliberately not tested

- Electron IPC window frame rendering (covered by desktop host specs).
- High-frequency canvas animations (progress bars use CSS transitions).

## Exit criteria

- [ ] All component tests pass in Vitest.
- [ ] Automated axe accessibility test passes with zero violations.
- [ ] Keyboard navigation path verified.
- [ ] Absence of "Reading" tab verified by test.
