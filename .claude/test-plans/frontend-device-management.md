# Test plan: Frontend device management

| | |
|---|---|
| **Spec** | `.claude/specs/frontend-device-management.md` |
| **Status** | `DRAFT` |
| **Created** | 2026-09-06 |

## What we are trying to be confident about

1. The device list renders exactly the devices that `GET /api/v1/devices` returns
   with `revokedAt == null` — no more, no fewer, with no client-side filtering beyond
   that one predicate.
2. A revocation always requires a confirmation step that names the specific device; the
   destructive request is never sent on a single unconfirmed action.
3. The full revocation flow — confirm → `DELETE` → row disappears → screen-reader
   announcement — is reachable and operable with keyboard alone, with no mouse path.
4. Error paths (network failure, 404/409 race) are handled with an inline error and a
   re-fetch, not a silent failure or a stale list.
5. The self-service "Sign out" path (Web/Mobile) calls the existing logout endpoint, never
   `DELETE /api/v1/devices/{id}`.
6. None of the canvas elements this spec explicitly excludes (IP address, session-expiry
   text, OFFLINE COPIES, passphrase/pending-approval) appear in the rendered output.

## Risk assessment

**Hardest to get right:**

- **revokedAt filter on the API response (FR-1).** `backend-device-sync.md` FR-1 returns
  both active and revoked devices (for server-side audit purposes). A naive "render all
  rows" implementation would silently display already-revoked devices. The component test
  that catches this is the one that sends a mixed response containing a revoked device and
  asserts it does not render.
- **Confirmation dialog focus management (FR-6).** Focus trapping and Escape-to-close are
  fiddly to test correctly; JSDOM-based tests often miss real focus-trap failures that only
  show up under AT or in a real browser. The accessibility layer tests carry more weight
  here than the component tests.
- **Live-region announcement timing (FR-7).** A live region that exists in the DOM but is
  added after the announcement text (or whose `aria-live` attribute appears after the text)
  does not announce. The test must assert the announcement after the action settles.
- **Re-fetch-on-error, not retry (FR-5).** A subtle behavioral constraint: 404/409 triggers
  `GET /api/v1/devices` (re-fetch), not a second `DELETE`. Tests must verify the fetch
  count, not just the visible error.

**Merely tedious:**

- Relative time-string formatting for last-seen and last-synced.
- Verifying that FR-3 fields (IP, session-expiry) are absent from the rendered output.
- Sign-out calls the correct endpoint.

## Layers

### Unit

Pure logic, no DOM, no I/O:

| Test | Input → Assertion |
|---|---|
| `formatRelativeTime`: < 5 min ago | "Active now" |
| `formatRelativeTime`: 37 min ago | "Active 37 minutes ago" |
| `formatRelativeTime`: 3 h ago | "Active 3 hours ago" |
| `formatRelativeTime`: 25 h ago (yesterday) | A date string, not "Active N hours ago" |
| `formatRelativeTime`: null / undefined (`lastSyncedAt`) | "Never synced" |
| `isActiveDot`: `lastSeenAt` within 5 min | true |
| `isActiveDot`: `lastSeenAt` exactly 5 min ago | false (boundary — test both sides) |
| `buildKindLine`: `deviceClass="phone"`, `enrolledVia="pairing_code"` | "Phone · paired by code" |
| `buildKindLine`: `deviceClass="desktop"`, `enrolledVia="password_login"` | "Desktop · direct login" |

All unit tests use a pinned `now` value — no `Date.now()` calls without injection.

### Component

Run with Vitest + Testing Library against the Settings screen's Devices tab, using
`msw` (or the project's existing handler-mock pattern, whichever phase 13 used in
`NetworkSettings.test.tsx`) to intercept `GET /api/v1/devices` and
`DELETE /api/v1/devices/{id}`.

**Device list rendering (FR-1 / FR-2 / FR-3):**

| Test | Setup → Assertion |
|---|---|
| Renders active devices | Response with 3 active devices → 3 rows |
| Filters revoked devices | Response with 2 active + 1 `revokedAt`-non-null device → 2 rows (revoked row absent) |
| Shows label, kind, last-seen, last-synced | Active device fixture → each field visible in the row |
| Does not show IP | Fixture with no `ip` field → no IP text in any row |
| Does not show session-expiry text | "Owner"/"Remembered device"/"Expires in N days" text absent from rendered output |
| Re-fetches on tab activation | Tab activated twice → `GET /api/v1/devices` called twice |
| Zero-row response is not a normal empty state | Empty-array response → inline error/loading indicator, not a "No devices" empty-state message (FR-9) |

**Revocation flow (FR-4 / FR-5 / FR-6 / FR-7):**

| Test | Interaction → Assertion |
|---|---|
| Revoke opens confirmation naming device | Click "Revoke" on "Pixel 8" row → dialog visible with "Pixel 8" in heading/body |
| Cancel sends no request | Open confirmation, click Cancel → `DELETE` not called, dialog closes |
| Confirm sends DELETE with correct ID | Open confirmation, click Confirm → `DELETE /api/v1/devices/device-id-123` called once |
| Success: row disappears | `DELETE` → 204 → "Pixel 8" row gone from rendered list |
| Success: success announcement | `DELETE` → 204 → live region text includes device label or generic success copy |
| 404: inline error + re-fetch | `DELETE` → 404 → error message visible, `GET /api/v1/devices` called again (not a second DELETE) |
| 409: inline error + re-fetch | `DELETE` → 409 → same as above |
| Network error: inline error + re-fetch | `DELETE` → network failure → inline error, re-fetch triggered |
| Re-fetch after error updates the list | 404 on DELETE, then GET returns updated list → list reflects the re-fetch (not stale pre-delete state) |

**Self-service session view (FR-10 / FR-11):**

| Test | Assertion |
|---|---|
| Sign-out calls logout endpoint | Click "Sign out of this browser/device" → `POST /api/v1/auth/logout` called |
| Sign-out does not call DELETE | Click "Sign out" → `DELETE /api/v1/devices/{id}` never called |
| OFFLINE COPIES card absent | Rendered output contains no offline-copies section |
| Passphrase/pending-approval absent | No passphrase input, no pending-approval state rendered |

### Accessibility

Follow the existing pattern in
`web/src/screens/Network/DevicePairingModal.test.tsx` and
`web/src/screens/Network/networkA11y.test.tsx` — axe + keyboard-path tests with
Testing Library's `userEvent`.

| Test | Assertion |
|---|---|
| Full tab order reaches Revoke | `Tab` key from list start reaches every row's "Revoke" control |
| Revoke activatable via Enter | Focus on Revoke control → `Enter` → confirmation dialog opens |
| Revoke activatable via Space | Focus on Revoke control → `Space` → confirmation dialog opens |
| Confirmation dialog traps focus | Dialog open → `Tab` cycles within dialog; focus does not leave |
| Escape dismisses dialog | Dialog open → `Escape` → dialog closed, no DELETE sent |
| Cancel returns focus to Revoke | Click Cancel → focus is on the Revoke control that opened the dialog |
| Confirm is reachable by keyboard | Dialog open → `Tab` to Confirm → `Enter` sends DELETE |
| Live-region announces success | Revoke confirmed → `DELETE` → 204 → `aria-live` region text updated (asserted via accessible role query) |
| Live-region announces failure | DELETE → 404 → `aria-live` region text updated with error copy |
| axe: no violations on initial render | `axe(container)` on the loaded device list → zero violations |
| axe: no violations on dialog open | `axe(container)` with confirmation dialog visible → zero violations |

These tests use the same axe configuration and `aria-live` assertion helpers already
established in `networkA11y.test.tsx`. Do not introduce a second configuration.

### End to end

One Playwright scenario covering the full server-round-trip path:

1. Seed two `PairedDevice` rows for the test user via the existing test harness setup.
2. Open Settings → Devices tab.
3. Assert both device rows are visible, with correct labels.
4. Click "Revoke" on device 2. Confirm.
5. Assert device 2's row disappears from the UI.
6. Assert via `GET /api/v1/devices` (direct API call in the test) that device 2 has a non-null `revokedAt`.
7. Reload the Devices tab. Assert device 2 does not reappear (re-fetch respects FR-1 filter).

This test also serves as the E2E gate that the frontend and backend are wired end-to-end.

### Contract

No new `api/openapi.yaml` changes (this spec adds no endpoints). The existing contract
tests for `GET /api/v1/devices` and `DELETE /api/v1/devices/{id}` (added by
`backend-device-sync.md`'s own contract test obligation) are sufficient. The frontend
test's `msw` handlers MUST mirror the response shape from the approved OpenAPI spec —
any drift between the mock and the spec will be caught when the backend tests run against
the real OpenAPI definition.

## Adversarial cases

| Input | Expected behaviour |
|---|---|
| API response contains a device with `revokedAt` set | That device does not render as a row (FR-1 filter) |
| API response is an empty array | Inline error/loading indicator; no "No devices" empty-state message (FR-9) |
| `GET /api/v1/devices` returns 500 | Inline error with retry control; no crash |
| `DELETE` returns 404 (race: already revoked by another session) | Inline error, list re-fetched; no retry of the DELETE |
| `DELETE` returns 409 | Same as 404 case |
| `DELETE` response is a network timeout | Inline error, list re-fetched; no assumed success |
| Rapidly double-clicking "Confirm" on the revocation dialog | Only one `DELETE` request sent (idempotent-click guard or button disabled-during-inflight) |
| `lastSyncedAt` is null | "Never synced" rendered, no crash |
| `lastSeenAt` is a far-future timestamp (clock skew) | Relative-time function renders gracefully (does not show negative "minutes ago") |
| Device label is 100 chars (max, per backend spec) | Rendered without truncation that hides the name from the confirmation dialog |
| Device label contains HTML entities (`<script>`, `&amp;`) | Rendered as text, not interpreted as HTML |

## Fixtures and test data

- Two active `PairedDevice`-shaped JSON objects: `fixture-device-active-1.json` (label
  "Pixel 8", class "phone", enrolled "pairing_code"), `fixture-device-active-2.json`
  (label "iPad Air", class "tablet", enrolled "pairing_code"). `revokedAt: null` on both.
- One revoked fixture: same shape with `revokedAt` set to a past ISO timestamp — used
  exclusively in the "does not render revoked" test (FR-1 filter).
- A pinned `now` timestamp for all unit and component tests — passed as a prop or injected
  via context, not read from `Date.now()` directly in any testable function.
- No real user data, no copyrighted content, no real credentials. All fixtures constructed
  inline or in test-setup files, torn down per-test or per-suite.

## What is deliberately not tested

- **Backend authorization.** Cross-user IDOR, ownership checks, and token validation are
  all server-side concerns already covered by `backend-device-sync.md`'s own test plan.
  The frontend tests use mocked API responses and do not exercise the server's authorization
  logic.
- **IP address and session-expiry rendering.** FR-3 says these must NOT be shown; tests
  assert their absence. There is no positive rendering test for them, since they are not
  implemented.
- **The "this is the device I'm using" highlight.** Named explicitly as unimplemented in
  FR-1/FR-2 and the Open questions section — there is no device-ID signal in the JWT.
  No test covers self-identification because no implementation exists.
- **The `atConnect` passphrase/pending-approval flow.** Out of scope per Non-goals. Not tested.
- **Offline/SW behaviour.** No service-worker or offline-capable caching is built — the list
  is always fetched live. No offline test.
- **Pagination / virtualization.** Device counts are single digits to low tens; the
  component is not virtualized and does not need a large-list test.

## Exit criteria

- [ ] Every functional requirement in `frontend-device-management.md` (FR-1 through FR-11)
      maps to at least one test
- [ ] The FR-1 `revokedAt == null` filter has a dedicated component test verifying the
      revoked device does not render
- [ ] Every adversarial case above has a test
- [ ] Accessibility tests pass: axe clean on both initial render and dialog-open state;
      keyboard path reaches and activates Revoke; live region announces both success and failure
- [ ] E2E test passes end-to-end against a real server (seeded fixture, revoke, API
      verification, reload check)
- [ ] Tests were observed to fail before the implementation existed (RED step, per constitution §2)
- [ ] The suite is deterministic across repeated runs (pinned `now`, no network in unit/component)
