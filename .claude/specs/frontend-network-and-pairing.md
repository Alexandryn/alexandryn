# Spec: Frontend — network settings and device pairing

| | |
|---|---|
| **Status** | `APPROVED` — scope approved at phase-13 Gate 1 (2026-09-02); independent two-agent review [`0050`](../reviews/0050-phase13-spec-package-and-phase12-authz-review.md), findings 8/9/S-H4 + the `qrcode` §9 record reworked and self-reviewed 2026-09-02. **`fr4` / first-run resolved 2026-09-03: a first-run network step is out of scope for phase 13** (`roadmap/13-network-access/README.md` Scope Out) — this spec builds the standing Settings → Network panel and the pairing flow only. |
| **Phase** | `13-network-access` |
| **Author** | Claude (Sonnet 5) |
| **Created** | 2026-09-02 |
| **Last updated** | 2026-09-02 |
| **Reviewed in** | [`0050`](../reviews/0050-phase13-spec-package-and-phase12-authz-review.md) (two independent agents) — Needs rework at review time, findings addressed |
| **Design reference** | Canvases re-read fresh 2026-09-02. `.design-reference/ANALYSIS.md` synced **2026-08-13**, scope-classification ledger dated **2026-08-17**. **Binding, relied on:** `Alexandryn-Electron-Admin.dc.html` `sgNetwork`'s **"Advanced" disclosure row** — the *only* ledger entry for this surface (**Binding, contents uncaptured** — the row is drawn, its panel body is not); and `Alexandryn-Web.dc.html` `atConnect`/`atAccess` (**Binding**, outline only). **Present in the canvas but NOT relied on as authority:** the first-run `fr4` step — `ANALYSIS.md` classifies `atFirstRun` **Unclassified**, which CLAUDE.md makes a stop-and-ask, not a default (see Open questions); `sgDevices` — **no ledger row at all**; the `sgNetwork` status card, toggle rows, and QR card as *drawn* — they sit on `atSettings`'s Network tab, which the ledger leaves Unclassified apart from the one Advanced row. This spec designs the panel body and the pairing flow from `roadmap/13-network-access/README.md` + ADR 0028, treating the drawn elements as visual reference for *look*, not as a binding contract for *scope*. **Contradictions (in `implementation-plan.md` Gate 1.5, resolved by ADR 0028):** D-1 `atConnect`'s "Library passphrase" → the account password, standard username/password sign-in (§6); D-2 the "Require authentication" toggle → **not built**, a static "Authentication is always on" statement (§7); D-3 no captured Advanced-panel body → designed here from the roadmap; D-4 device list/revoke → phase 14, this phase shows only the pairing flow + "revoke this pairing" (plan C-3); D-5 the "Allow access from this network" toggle → **not built**, a read-only reachability statement (§8). |

## Context

`backend-network-api.md` exposes `/api/v1/network/*`. Phase 12 already
built the auth screens (`SetupScreen`, `LoginScreen`, `MfaPromptModal`),
the `hostOnly()` capability gate (`frontend-shell-and-routing.md` FR-4,
`web/src/app/routes.tsx`), and the token/`X-Library-Id` plumbing
(`web/src/data/http.ts`). The `connect` and `access` routes exist as
viewer-side `ScreenPlaceholder`s. This spec fills in the two real
surfaces phase 13 needs: the host's Network settings panel and the
device's pairing/connect flow.

## Problem

There is no UI to: see whether the server is reachable beyond loopback,
start a pairing, show a QR, join from a new device by entering a code, or
see the current session's permissions. The placeholders at `connect` and
`access` render nothing usable.

## Goals

- A **Settings → Network** panel (host-only) matching `sgNetwork`: server
  status, an "Advanced" disclosure with the bind/TLS/mDNS values
  (read-only + restart-required copy), a QR + address card, and a static
  "authentication is always on" statement in place of the design's
  toggle.
- A **device pairing** experience: on the host, a modal that shows the
  QR + code from `pair/initiate` with a live countdown; on a new device,
  a `connect` screen that accepts a code, calls `pair/verify`, and hands
  the returned enrolment grant to the existing login flow.
- An **`access` / "This session"** view matching `atAccess`: what this
  browser is, where it is reading from, and a sign-out.
- Design tokens only, WCAG AA, full keyboard path, announced errors —
  `done`, not a follow-up (constitution §7, `frontend-accessibility.md`).

## Non-goals

- The standing device inventory (list every paired device, rename,
  manage per-device sessions) — **phase 14**. The only device-management
  affordance here is "revoke this pairing" on the host's pairing modal
  (plan C-3, `backend-network-api.md` FR-6).
- Any control that changes the bind address, TLS certificate, or ACME
  domain from the UI — they are read-only here with a "restart required"
  line (ADR 0028 §8, `backend-network-api.md` FR-5).
- Any "turn authentication off" control — it does not exist (ADR 0028
  §7).
- The `atAccess` `hostModes` selector's `'cloud'` option — declined
  2026-08-18 (`ANALYSIS.md`), rendered as visual reference only in the
  prototype, not built.
- Server-side QR image generation — the client renders from a payload
  string (ADR 0028 Option D, plan C-4).
- mDNS discovery UI ("devices found on your network") — not in scope;
  the panel shows the configured `hostName`, nothing auto-discovers.

## User stories

- As **the host operator**, I open Settings → Network and see at a glance
  that the library is reachable on the LAN, at which address, over which
  transport.
- As **the host operator**, I press "Open on another device", a modal
  shows a QR and a 5-minute countdown, and I can revoke it if I change my
  mind.
- As **someone on a new device**, I scan the QR (or type the code at
  `/connect`), then land on the normal sign-in screen already pointed at
  the right host, log in once, and I am in the library.
- As **someone on a shared device**, I open "This session", see what this
  browser can do, and sign out.
- As **a keyboard-only user**, every step above works with Tab / Enter /
  Escape and the countdown and errors are announced.

## Functional requirements

- **FR-1** `web/src/screens/Settings/NetworkSettings.tsx` renders inside
  the existing `hostOnly('network', 'Network')` route (added to
  `routes.tsx` alongside `settings`/`system`). It fetches
  `GET /api/v1/network/status` via a TanStack Query hook
  (`frontend-shell-and-routing.md` FR-2 — no inline `fetch`). It renders:
  - a **status card**: a state dot + "Server running", the admin-scoped
    `addresses[]` as labelled rows (from `scope`), the `hostName`, and a
    `tlsMode` indication whose copy is **honest about the plaintext case**
    (review `0050` S-H4 — the server cannot know whether a reverse proxy
    sits in front of it):
    - `acme` → "Encrypted with an automatic certificate for <acmeDomain>."
    - `static` → "Encrypted with your certificate file."
    - `none` + `reachability: loopback` → "Only this computer connects.
      Encryption is not needed."
    - `none` + `reachability: private` → "Traffic on your local network
      is **not encrypted** unless a reverse proxy in front of Alexandryn
      terminates TLS. To encrypt it directly, set a certificate in the
      configuration file." (calm, specific — constitution §11, §12.)
  - a **"Authentication is always on"** static row (not a toggle):
    `"Every device must sign in. This cannot be turned off."`
    (constitution §11 — plain, no exclamation);
  - a **reachability** line: loopback → "Only this computer can reach the
    library."; private → "Devices on your local network can reach the
    library after signing in."; public → "This library is reachable from
    the internet at <acmeDomain or host> after signing in.";
  - a **read-only** row where the design draws "Allow access from this
    network" (D-5): it states the current reachability and that the bind
    address is set in the configuration file — no toggle;
  - an **"Advanced"** disclosure (Radix disclosure, keyboard-operable,
    `aria-expanded`) showing bind address, TLS certificate
    **"configured" / "not configured"** (never the path — matches
    `backend-network-api.md` FR-4 scrubbing), ACME domain, and the mDNS
    `hostName` (editable); with the line: `"Bind address and TLS settings
    are read from the configuration file. Changes take effect after
    restarting Alexandryn."`; and, when `tlsMode: acme`, a line stating
    that enabling ACME accepted the certificate authority's terms of
    service, linking them (ADR 0028 §2);
  - the **mDNS name** field and a **"remember devices for N days"**
    (1–90) number input, both saved via `PATCH /api/v1/network/settings`
    (optimistic update, error toast naming what failed);
  - an **"Open on another device"** card: a button opening the pairing
    modal (FR-2), plus the plain network address as selectable/copyable
    text. It does **not** try to render a QR for an "active" session —
    phase 13 has no endpoint to recover an in-progress `PairingSession`
    after a page reload (the client holds `pairingId` only in memory), so
    there is no "if a session is active" branch; the QR lives entirely
    inside the modal for the life of that modal.
- **FR-2** `web/src/screens/Network/DevicePairingModal.tsx` — a Radix
  `Dialog` opened from the Network panel. On open it calls
  `POST /api/v1/network/pair/initiate` (with the `DEVICE_PAIRING_SECRET`
  field only if the server indicated one is required — surfaced via
  `/network/status` or a `409`/`403` retry prompt). It shows: the QR
  (FR-4) of `payload`, the `code` in large `XXXX-XXXX` monospace text, a
  **live countdown** to `expiresAt` (`aria-live="polite"`, announced at
  60s / 30s / 10s / 0 only — not every second, FR-7), a "Revoke" button
  (`DELETE /api/v1/network/pair/{pairingId}`), and a "Done" button. When
  the countdown reaches zero the modal shows "This code expired" and
  offers "Generate a new code". The modal MUST NOT poll `verify` state
  (phase 13 has no "device joined" push; the operator learns the device
  joined when it appears in phase 14's list — for phase 13 the modal is
  fire-and-forget). Escape and the close button both call `DELETE` on the
  session (revoke on dismiss) unless "Done" was pressed.
- **FR-3** The **`/connect` route** (viewer surface, replaces the
  placeholder): a screen titled per the design ("Sign in to Home
  Library") that:
  - reads a `?c=<code>` query param if present (from a scanned QR),
    pre-fills the code field, then **immediately `history.replaceState`s
    to strip `?c=`** so the single-use code does not sit in browser
    history or the back-forward cache (review `0050` L-4);
  - shows one field, "Pairing code" (`XXXX-XXXX`, auto-uppercase,
    hyphen auto-insert), and a "Continue" button;
  - on submit calls `POST /api/v1/network/pair/verify` with
    `{ code, label? }` where `label` is an optional "Name this device"
    field (defaulting empty → server derives);
  - on `200`, stores nothing sensitive, and navigates to the phase-12
    `/login` screen carrying the `enrolmentGrant` and `hostName` in
    router state (not the URL — a grant in a URL lands in history/logs);
  - on `404` shows the server's generic `"pairing code not recognised"`
    with a "Check the code and try again" affordance;
  - on `429` shows "Too many attempts. Wait a minute and try again."
  - The existing note "Or approve this browser from the host computer" is
    reframed: `"Or open Settings → Network on the host computer and press
    Open on another device."`
- **FR-4** QR rendering is **client-side from the `payload` string** (a
  `<scheme>://<host>/connect?c=<code>` URL). The component takes the
  string and renders an SVG QR (crisp at any size, theme-aware via
  `currentColor`, `role="img"`, `aria-label="QR code to open Alexandryn
  on this device"`, with the `payload` URL and the short `code` also
  shown as selectable text beneath, so a screen-reader or keyboard user
  is never blocked on the image). Error-correction level `M`.
  - **Dependency (constitution §9):** the encoder is **`qrcode`** (npm,
    MIT, no runtime transitive dependencies, ~20 KB min). *What it does*:
    QR matrix generation (Reed–Solomon ECC + bit placement). *Why not
    stdlib / hand-rolled*: correct QR encoding is ~600 lines of
    error-prone, spec-bound code for a security-adjacent artifact;
    getting the ECC wrong yields codes that scan intermittently. *What
    breaks if abandoned*: QR display only — the `payload` URL and the
    typed code both still work; swapping to another encoder (`qrcode-generator`,
    a vendored copy) is a localised change behind this one component.
    Its bundle cost is checked against `check:bundle-size` headroom
    **before this spec reaches `APPROVED`** (recorded in the review), and
    only the matrix-generation entry point is imported (not the
    canvas/terminal renderers). If the headroom check fails, the
    fallback is vendoring `qrcode-generator` (~4 KB) — same component
    boundary.
- **FR-5** The **`/access` route** ("This session", viewer surface): shows
  `reachability` / `tlsMode` / the single `address` this browser
  connected on (the reader-scoped `/network/status` shape,
  `backend-network-api.md` FR-4), a **small fixed capability list keyed
  off the current user's role** (`admin` vs `reader`) plus the names of
  the libraries in their JWT `libraries` claim — **not** an invented
  granular permission model (phase 13 has role + library membership and
  nothing finer; `admin` → "Manage this library, sources, and devices",
  `reader` → "Read and track progress", plus "Upload" only if the active
  library allows reader uploads). And a "Sign out of this browser" button
  calling the phase-12 `logout()`. The design's `hostModes` card is
  **not** rendered — its only non-local option is the declined cloud
  relay.
- **FR-6** Every new surface uses design tokens exclusively
  (`check:token-styling` clean — `text-2xs`, `gap-sm`, `bg-bg-surface`
  etc., zero raw px, zero raw hex). The countdown, code, and address use
  the mono token family the design uses (`IBM Plex Mono` → the project's
  mono token). No new token is invented without adding it to
  `frontend-design-tokens.md`'s extraction.
- **FR-7** Accessibility, verified not deferred:
  - the pairing modal traps focus, restores it on close, Escape closes
    (and revokes), the code is in a `<output>`/`aria-live` region;
  - the countdown announcements are `aria-live="polite"` and do not
    spam (announce at 60s, 30s, 10s, 0 — not every second);
  - the Advanced disclosure is a real disclosure widget, keyboard
    toggle, `aria-expanded`;
  - all form fields have visible labels (not placeholder-only),
    errors are associated via `aria-describedby` and announced;
  - `check:a11y-tabindex` and `check:a11y-hidden-text` clean;
  - an `@axe-core/playwright` pass over both new screens and the modal,
    zero violations (`frontend-accessibility.md` FR-4).
- **FR-8** Reduced motion respected: the countdown is a number, not an
  animated ring, when `prefers-reduced-motion` (or a static ring). No
  motion in the QR reveal beyond a token-standard fade.

## Non-functional requirements

- **Performance** — the QR encoder (dep or vendored) must fit the bundle
  budget (`check:bundle-size`); `qrcode` at ~20 KB gzipped-ish is within
  a reasonable delta but the spec review confirms against the current
  budget headroom. `/network/status` is a single small fetch, cached by
  TanStack Query with a short stale time (the panel does not need
  real-time).
- **Security** — no secret is ever placed in a URL (the enrolment grant
  travels in router state / a POST, never a query string, per FR-3). The
  QR `payload` is a URL + a single-use code — acceptable in a URL because
  the code is single-use and short-lived and the QR is the intended
  transport. `localStorage` is not used for anything new here.
- **Accessibility** — see FR-7; it is an acceptance gate, not an NFR to
  defer.
- **Reliability** — the pairing modal handles: the initiate call failing
  (show the error, allow retry), the network dropping mid-countdown (the
  countdown is client-side, keeps running; a stale `payload` just fails
  at `verify` time with the generic 404 on the device side).
- **Observability** — client errors surface as toasts with the server's
  `correlationId` shown in a "Details" affordance
  (`web/src/data/http.ts` `ApiError` already carries it), consistent with
  the rest of the app.

## Domain model

None — this is presentation over `backend-network-api.md`'s shapes. No
client-side domain types beyond the generated fixture types from
`npm run mocks:gen-fixtures`.

## API and contracts

Consumes, via TanStack Query hooks in `web/src/data/network.ts` (new):

- `GET /api/v1/network/status` → the **role-scoped** FR-4 shape from
  `backend-network-api.md` (the reader shape is a subset; the hook's TS
  type is the union and the components render defensively)
- `POST /api/v1/network/pair/initiate` → `{ pairingId, code, payload,
  address, expiresAt }`
- `GET /api/v1/network/pair/{pairingId}/qr` → `{ payload, address, code,
  expiresAt, state }` — used only if the modal remounts while still open;
  not used to "resume" a session after the modal closed
- `POST /api/v1/network/pair/verify` → `{ enrolmentGrant, address,
  hostName }`
- `DELETE /api/v1/network/pair/{pairingId}` → `204`
- `PATCH /api/v1/network/settings` → full settings

The phase-12 `login()` in `web/src/data/auth.ts` gains an optional
`enrolmentGrant` argument passed through to `POST /api/v1/auth/login`
(`backend-network-api.md` FR-9).

MSW handlers (`web/src/mocks/handlers.ts`) gain these routes for the
Vitest and Playwright suites, seeded from the generated fixtures.

## State transitions

Pairing modal (client view):

```
idle ──open──► initiating ──ok──► showing(code, countdown)
                   │ error            │  countdown → 0        │ Revoke / Escape
                   ▼                  ▼                       ▼
                 error ◄──retry──   expired ──new code──► initiating
                                      │
                              Done / Revoke
                                      ▼
                                   closed
```

`/connect` (device view): `entering → verifying → (navigate to /login) |
error(entering)`.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| `/network/status` fails to load | Query error | Panel shows "Couldn't load network status" + retry | No stale data shown as current |
| `pair/initiate` fails | Mutation error | Modal shows the server message + "Try again" | No QR shown |
| Countdown reaches zero | Client timer | "This code expired" + "Generate a new code" | Old `pairingId` abandoned (server sweeps it) |
| Device submits an expired/wrong code | `verify` `404` | Generic "pairing code not recognised" + retry | Rate limiter counts it (server) |
| Device hits the rate limit | `verify` `429` | "Too many attempts. Wait a minute." | — |
| Enrolment grant expires before login | `login` ignores it (server FR-9) | Login still works; device just isn't linked yet | User can re-pair from the host later |
| User edits mDNS name to an invalid label | `PATCH` `400` | Field-level error naming the rule | Nothing persisted |
| Modal dismissed with Escape | FR-2 | Modal closes | `DELETE` revokes the session |
| Reduced motion set | `prefers-reduced-motion` | Static countdown, no animated ring | FR-8 |

## Security considerations

- **No secret in a URL (FR-3)** — the enrolment grant travels in router
  state or the login POST body, never a query string, so it does not
  land in browser history, the server access log, or a `Referer` header.
  The pairing `code` in the QR `payload` URL is acceptable: it is
  single-use, ≤ 5-minute-lived, rate-limited server-side, and the QR is
  its intended delivery channel.
- **The panel never displays a path or a secret** — it shows
  "configured / not configured" for the TLS certificate and the ACME
  domain, mirroring `backend-network-api.md` FR-4's server-side scrubbing.
  A future contributor cannot "just show the cert path for debugging"
  without the server changing its response shape.
- **No "auth off" control (ADR 0028 §7)** — the design's toggle is
  replaced by a statement of fact. There is no code path in this UI that
  could send `authRequired: false` (the server would reject it anyway —
  `backend-network-api.md` FR-5 — but the UI does not offer it).
- **Capability gating** — `NetworkSettings.tsx` and the pairing modal are
  behind `hostOnly()` (`frontend-shell-and-routing.md` FR-4). A viewer
  client that navigates to `/settings/network` directly gets the
  gate's "not available on this device" state, never a flash of
  host-only content (that spec's FR-4 no-flash requirement).
- **React/JSX** — no `dangerouslySetInnerHTML` anywhere in these
  components; the QR is an SVG built from a data matrix, not injected
  markup.
- **STRIDE (UI slice)** — *Spoofing*: the `/connect` screen shows the
  `hostName`/`address` it is talking to so a user can sanity-check.
  *Tampering*: n/a (presentation). *Repudiation*: errors carry the
  server correlation ID. *Information disclosure*: no path/secret shown,
  no grant in URL. *DoS*: n/a client-side. *EoP*: capability gate + the
  server's role checks.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit (Vitest + RTL) | `NetworkSettings`: renders each `reachability`/`tlsMode` variant from mock `/network/status`; shows "configured/not configured", never a path (assert the mock's fake path string is absent from the DOM); the "always on" row is static text, no toggle role in the tree; `PATCH` optimistic update + rollback on error; Advanced disclosure keyboard toggle. `DevicePairingModal`: initiate success renders code + QR; countdown decrements and flips to expired at 0; Revoke and Escape both fire `DELETE`; "Done" does not. `/connect`: `?c=` prefills; code input formats to `XXXX-XXXX`; submit success navigates to `/login` with grant in state (not URL — assert `location.search` is clean); `404`/`429` messages. `/access`: renders permissions from mock claims; sign-out calls `logout()`. QR component: known payload → stable SVG snapshot; `aria-label` present; address shown as text. |
| Integration | Full routing: `/settings/network` behind the capability gate (withheld → gate state, granted → panel, no host-only flash); `/connect` and `/access` reachable as viewer routes. MSW-backed. |
| Contract | Fixtures from `npm run mocks:gen-fixtures` (driven by `backend-network-api.md`'s `example:` blocks) type-check against the `data/network.ts` hook types. |
| E2E (`@playwright/test`) | The pairing happy path end to end against a mock/loopback build: host opens the modal → a second browser context opens `/connect?c=<code>` → verify → redirected to `/login` → logs in → lands in the library. Plus: expired code → generic error; revoke from the host → the code no longer verifies. |
| Accessibility | `@axe-core/playwright` over `NetworkSettings`, `DevicePairingModal` (open), `/connect`, `/access` — zero violations. `check:token-styling`, `check:a11y-tabindex`, `check:a11y-hidden-text` clean over the new files. |

Must fail before implementation: the `NetworkSettings` "never shows a
path" test, the countdown-expiry test, the `/connect` "grant not in URL"
test, the Playwright pairing happy-path.

## Acceptance criteria

- [ ] Settings → Network renders server status, addresses, `tlsMode`, and
      a static "authentication is always on" statement — **no toggle**.
- [ ] The panel never renders a filesystem path, the certificate, or the
      pairing secret — proven with a mock status carrying recognisable
      fake values.
- [ ] Bind address / TLS / ACME values are read-only with a "changes
      take effect after restart" line; only mDNS name and remember-days
      are editable and persist via `PATCH`.
- [ ] The pairing modal shows a QR (rendered client-side from the payload
      string), the `XXXX-XXXX` code, and a live countdown that announces
      at 60/30/10/0s and flips to "expired" at zero.
- [ ] Dismissing the modal with Escape or Revoke calls
      `DELETE /network/pair/{id}`; "Done" does not.
- [ ] `/connect` accepts a code (prefilled from `?c=`), calls `verify`,
      and on success navigates to `/login` with the enrolment grant in
      router state — **not** in the URL (proven by a test asserting a
      clean query string).
- [ ] `/connect` shows the generic `404` message for a bad code and a
      wait message for `429`.
- [ ] `/access` shows this session's reachability + permissions and a
      working sign-out; the `hostModes` cloud option is absent.
- [ ] All new surfaces: design tokens only, `check:token-styling` clean,
      zero raw px/hex.
- [ ] `@axe-core/playwright` zero violations on both screens and the
      modal; full keyboard path; errors announced; reduced-motion
      respected.
- [ ] The Playwright pairing happy-path passes end to end.
- [ ] Every FR maps to a line in phase 13's exit criteria.

## Open questions

- ~~**`fr4` (first-run Network step) is Unclassified — stop-and-ask.**~~
  **Resolved 2026-09-03 (maintainer): out of scope for phase 13.** Phase
  13 builds the standing Settings → Network panel and the pairing flow;
  the first-run flow is a separate surface with its own scope, and a
  network step there — if wanted — is designed and specified with the
  rest of first run, reusing this phase's `NetworkSettings` component. The
  decision does not promote `atFirstRun` from Unclassified; it defers the
  whole first-run surface. Recorded in
  `roadmap/13-network-access/README.md` Scope Out.
- **`sgDevices` has no ledger row.** The device-list/revoke screen the
  design draws is not classified anywhere. Phase 13 ships only "revoke
  this pairing" inside the pairing modal (D-4); the standing device
  inventory is phase 14, which will need `sgDevices` classified before it
  can treat it as binding.
- **"device joined" signal** — phase 13 has no push channel and the modal
  is fire-and-forget. If the maintainer wants immediate confirmation, a
  short client poll of a minimal `GET /network/pair/{id}` state
  (`consumed` → "a device joined") is a small addition. Default: not in
  phase 13.

Resolved during review `0050`: the QR encoder is `qrcode` (npm, FR-4's §9
record), pending the `check:bundle-size` headroom check before `APPROVED`;
the `/access` capability list is a small fixed list keyed off role, not an
invented granular model (FR-5).

## References

- ADR 0028 Option D (client-side QR), §6 (pairing model → the `/connect`
  flow), §7 (no auth-off → the static statement), §8 (restart-only
  settings → read-only Advanced)
- `backend-network-api.md` — every endpoint and shape consumed
- `backend-authentication.md` / phase-12 `LoginScreen`, `logout()` — the
  flow `/connect` hands off to and `/access` calls
- `frontend-shell-and-routing.md` FR-2 (data hooks, no inline fetch),
  FR-4 (`hostOnly()` capability gate, no-flash)
- `frontend-design-tokens.md` — token taxonomy; a new token is added
  there, not inline
- `frontend-accessibility.md` FR-4 (`@axe-core/playwright` gate),
  keyboard map, announced errors
- `frontend-component-primitives.md` — Radix `Dialog`, disclosure, the
  primitives this reuses
- `.design-reference/ANALYSIS.md` — `sgNetwork`/`atConnect`/`atAccess`
  classification; declined cloud-relay
- Constitution §7 (accessibility is done), §9 (dependency reasons), §11
  (copy)
