# Security audit: Phase 04 — Frontend foundation

| | |
|---|---|
| **Scope** | `web/` — the phase 04 frontend foundation as merged to `main` (PRs #59–#64): build tooling, design tokens, 18 component primitives, the generated-cover system, the shell + router + capability hook, the MSW mock layer, the accessibility baseline, and the CI pipeline. Tiers 0–5. |
| **Auditor** | Claude (Sonnet 5) — reconciled from three passes: `/security-review` (branch), the `agent-skills:security-auditor` subagent (full four-attacker threat model), and the `agent-skills:security-and-hardening` checklist; plus the manual verification tasks T2–T6 (`tasks/todo-p04-tier6-closure.md`). |
| **Date** | 2026-08-27 |
| **Commit** | Branch `feat/phase04-tier6-closure`, audit reconciled at the T7/T8 commits (see git log); `web/` as of the 20-commit stack over `main` (Tiers 5 + 6). |
| **Verdict** | **Clear** — no open Critical or High finding. Eight items: one fixed this tier (A-0004-07), one accepted phase-gated design decision (A-0004-03), the rest carried-forward hooks for phases 05/06/12 or an accepted judgment call. |

## Scope and method

Phase 04 is a fully client-side-rendered React frontend, developed
entirely against a mock backend. It ships **no server, no
authentication, no Electron IPC surface, and no file parsing** — those
arrive in phases 05, 06, 10, and 12. This audit is therefore scoped to
phase 04's *real* trust boundaries and to the security properties that
phase 06 and later inherit from this shell, not to a threat model the
code cannot yet exercise.

Method, three reconciled passes plus manual verification:

1. **`/security-review` on the branch** (`feat/phase04-tier6-closure`).
   Recorded honestly: Tiers 0–5 are already merged to `main`, so the
   branch diff is only Tier 6's own doc and test additions. This pass
   confirms the Tier 6 changes introduce nothing; it is not where the
   four-attacker depth comes from.
2. **`agent-skills:security-auditor` subagent** — a full four-attacker
   threat model over the whole `web/` tree, briefed with the four trust
   boundaries and five sub-scopes below. This is the pass with coverage
   depth.
3. **`agent-skills:security-and-hardening` checklist** — walked against
   `web/`.
4. **Manual verification (T2–T6)** — each producing concrete evidence: a
   nonzero exit code from a check script against a planted artifact, a
   whole-tree grep result, an inspection of the real built bundle.

Not covered by this audit's method: runtime memory/timing/concurrency
analysis (no long-lived server process exists), dependency CVE triage
beyond `npm audit --audit-level=high` (which CI already gates), and the
"malicious source" / "hostile book file" attacker categories, which have
no reachable surface in phase 04 (see below).

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| `web/dist` → any browser it is served to, including a LAN client (`architecture-system.md` FR-6: one build artifact for the Electron window and LAN devices alike) | The bundle is world-readable the moment phase 05/06 serve it over loopback or the LAN | No secret, no credential, no host-directory path, and no mock/stub layer is present in the shipped artifact |
| Future source-, file-, or metadata-provider-derived text → React render tree (titles, authors, filenames, API `code`/`message`/`correlationId`, route params) — phases 06/07/08/10 | Every one of those strings is chosen by an attacker (a hostile source, a poisoned metadata record, a crafted deep link) | React's default escaping holds everywhere; no `dangerouslySetInnerHTML`, no `innerHTML` write, no unescaped `href`/`src`/redirect sink exists anywhere in the tree for phase 06 to fall into |
| Go server capability value → `useCapability()` hook → route rendering (`architecture-frontend.md` FR-3) | The renderer is assumed already compromised; it does not get to decide its own privilege level | The gate is server-*told*, never client-*inferred* (no decision keys off port, origin, `isElectron`, user-agent, or `import.meta.env`); a missing or failed value fails **closed** (`loading` forever, never optimistic `granted`) |
| Metadata-provider response shapes / source protocol shapes → `src/data/` → UI (constitution §3) | Open Library's JSON shape; an OPDS/local-folder source's protocol | Nothing un-normalised reaches a component; each integration is normalised at its own boundary; no adapter is bypassed |

## Adversarial questions asked

Constitution §10. Each of the four attackers is walked; the two with no
reachable surface in phase 04 are scoped out with reasoning, following
audit `0001`'s precedent for a phase whose threat surface is not yet
built.

### 1. A malicious **user** of this instance

Phase 04 has no authentication surface, no per-user data, and no active
privilege tiers — the mock `/api/bootstrap` grants every capability
unconditionally. The real question is whether the *capability-gating
architecture* the shell is built around can be subverted: does any code
infer privilege from a client-side signal that a user with devtools can
change?

- **Evidence (T5):**
  - Whole-tree grep for client-side privilege inference —
    `navigator.*`, `userAgent`, `window.electron`, `process.type`,
    `.hostname`, `.protocol`, `isElectron`, `isHost` — returns **zero**.
    The only `import.meta.env` use is `main.tsx`'s MSW dev-mode gate,
    which is not a capability decision.
  - The capability value has exactly one source:
    `CapabilityProvider` → `useQuery(['bootstrap'], fetchBootstrap)` →
    `getJson('/api/bootstrap')`. Nothing else can populate it.
  - Fail-closed: `data === undefined` (initial load **or** a rejected
    request) → `{ status: 'loading' }`. `granted` is reached only when
    `data` is present, and `can()` is a strict
    `data.capabilities[capability] === true`. No error branch degrades
    to an optimistic `granted`.
  - `RequireCapability` renders the loading state — never `children` —
    while `status === 'loading'`, per `architecture-frontend.md`'s
    illegal-transition rule.
  - `routes.tsx`'s `hostOnly()` wrapper is the single place host-only
    routing is declared; no route infers its own privilege.
  - Tests: `capability.test.tsx` covers loading→granted, the
    no-host-only-flash acceptance criterion (5-microtask loop), and — added
    here — **stays loading when the bootstrap fetch fails, never
    optimistic granted**. Observed failing (3 tests) against a planted
    optimistic fallback in `CapabilityProvider`, green after revert.
- **Result:** The gate is server-told and fails closed. That every
  capability currently renders (the mock grants all) is **by design** —
  `architecture-frontend.md` FR-3: every connection is loopback-only and
  trusted at the same level as Electron's renderer until phase 12/13.
  Recorded as an **Accepted, phase-gated risk** (A-0004-03), exactly as
  audit `0001` recorded the deliberately unauthenticated
  `/healthz`/`/readyz`. The load-bearing property — a single server-told
  enforcement seam for phase 12/13 to attach real auth to — holds.

### 2. A malicious **source** or metadata provider

No source or metadata integration is wired in phase 04 (phases 07/08).
The audit's job is to confirm phase 04 does not make XSS-safe rendering
*harder to keep* once that text arrives. Concrete surfaces (F26's own
wording): the generated-cover layers, `ErrorState` / `RouteError` (which
render an API error's `code` / `message` / `correlationId`),
`ScreenPlaceholder` / `ParamPlaceholder` (which echo route params), and
the MSW fixtures.

- **Evidence (T3):**
  - Whole-`web/` grep for HTML-injection sinks — `dangerouslySetInnerHTML`,
    `.innerHTML =` / `.outerHTML =`, `insertAdjacentHTML`,
    `document.write`, `eval(`, `new Function(` — returns **zero** in
    production source, tests, `e2e/`, and `scripts/`. The only `innerHTML`
    occurrences are read-only RTL assertions in
    `GeneratedCover.determinism.test.tsx` (comparing rendered output for
    determinism), not sinks.
  - Every provider-derived string is rendered as a JSX text child, which
    React escapes: `TitleLayer` (`{title}`), `AuthorLayer` (`{author}`),
    `ErrorState` (`{title}`, `{description}`, `{code}`, `{correlationId}`),
    `ScreenPlaceholder` / `ParamPlaceholder` (`{note}`, built from
    `params[param]`), `Library` (`{item.title}`, `{item.author}`).
  - `RouteError` — for a non-`ApiError` render error it renders a **fixed
    generic string**, never `error.message` or a stack trace, so a
    render error in a screen cannot leak internal detail to a LAN viewer.
    An `ApiError` shows only its `message` / `code` / `correlationId`,
    all as text children.
  - `SpineLayer` / `TextureLayer` build `hsl(...)` by template literal,
    but the interpolated `seed.hue` is `fnv1a(identifier) % 360` — a
    0–359 integer — and `seed.pattern` is one of three string literals.
    A caller cannot inject a CSS string. Guard test included.
  - URL / redirect sinks: no interpolated `href` / `src`, no
    `window.open`, no `location.assign|replace|href =`, no non-literal
    `redirect()`. Every `<Navigate to>` / `<Link to>` / `navigate()`
    target is a static literal **except** `Library.tsx`'s
    `to={\`/book/${item.id}\`}` — see the Informational note below.
  - The MSW fixtures (`src/mocks/fixtures/**`) are static object literals;
    no user input reaches them.
  - Client-side logging: the only `console.*` call in production source
    is `main.tsx`'s dev-only MSW-failure `console.error(msg, error)` — no
    credential, token, home-directory path, or reading content
    (constitution §8). Unreachable in a production build anyway.
  - **Regression guard added:** `web/src/test/xssEscaping.test.tsx` —
    renders `ErrorState`, `TitleLayer`, `AuthorLayer`, and `GeneratedCover`
    with a `<script>` + `<img onerror>` payload in every text prop and
    asserts no `<script>` / `onerror` element is created and the payload
    survives only as visible text; plus a case proving the spine hue
    stays a bounded number. Observed failing against a planted
    `dangerouslySetInnerHTML` in `ErrorState`, passing after revert.
  - **One Informational note (A-0004-02):** `Library.tsx` interpolates an
    unvalidated `item.id` into a React Router `<Link to={\`/book/${id}\`}>`.
    React Router resolves `to` as a path, not a URL (no `javascript:`
    execution), but an `id` containing `/` or `..` segments could
    redirect the link to a different route. Not exploitable in phase 04
    (the fixture ids are `ol-1`…`ol-3`); phase 06 should validate the id
    shape or navigate by param when the id comes from the real backend.
- **Result:** No XSS or injection finding at Critical/High/Medium.
  Two Informational notes (A-0004-01 carried from T2, A-0004-02 here).
- **Evidence (domain boundary, constitution §3, T6):**
  - `src/data/http.ts` — `ApiErrorBody` / `ApiError` is exactly
    `architecture-contracts.md` FR-5's `Error` schema
    (`code`/`message`/`correlationId`), verified against
    `api/openapi.yaml`. A normalised system-contract shape.
  - `src/data/library.ts` — `LibraryItem` is `{ id, title, author }`,
    `author` a single reduced string, **not** Open Library's
    `authors: [{key,name}]` array or a work/edition record. A minimal
    normalised display shape (comment: "phase 06 replaces this with the
    real library resource").
  - `src/data/bootstrap.ts` — `Bootstrap` / `Capability` is an
    Alexandryn capability shape (`architecture-frontend.md` FR-3's named
    set), not a provider or source-protocol shape.
  - No source protocol (OPDS, local-folder) exists anywhere in phase 04
    (sources are phase 08). No metadata adapter exists (phase 07). The
    `src/data/` layer is the seam those phases will normalise at; phase
    04 populates it only with already-normalised shapes, so no adapter
    is bypassed.
  - `GeneratedCover` takes `author?: string` — a pre-reduced display
    string, never a raw `Author[]` — decoupled from the domain array
    shape per `frontend-generated-covers.md` FR-1.
  - The MSW fixtures model normalised `LibraryItem[]`, the capability
    shape, and the contract `Error` shape — none models an Open Library
    JSON response or an OPDS feed.
  - **Result:** No domain-boundary violation. No metadata-provider
    response shape and no source protocol reaches the UI.

### 3. A device on the **local network** without a credential

Phase 04 ships no listener, but `web/dist` is the exact artifact a LAN
client is served once phase 05/06 wire the Go server's file serving. Two
angles: does the bundle leak anything a LAN client should not see, and
does the shell expose host-only *content* to a viewer-capability client?

- **Evidence (MSW / mock-boundary leak, T2)** — against a real
  `npm run build` (`dist/assets/index-*.js`, 103.9 KiB gzipped):
  - A whole-bundle grep for `mockServiceWorker`, `setupWorker`,
    `msw/browser`, `mocks/browser`, `from "msw"`, `onUnhandledRequest`,
    and the fixture identifiers (`bootstrapFixture`, `libraryItemsFixture`,
    `notFoundError`, `generatedFixtures`) returns **nothing**. The
    `import.meta.env.DEV`-gated `await import('./mocks/browser')` in
    `src/main.tsx` is the only path to the mock layer from production
    source, and it is dead-code-eliminated — no MSW package code, worker
    registration, handler, or fixture reaches `dist`.
  - `web/vite.config.ts`'s `strip-msw-worker-from-build` plugin: with a
    `public/mockServiceWorker.js` present (it is — committed, `797b03a`,
    the real 361-line MSW 2.x worker), `npm run build` produces a `dist/`
    with **no** `mockServiceWorker.js` — the plugin removes it on
    `writeBundle` after Vite's `public/` copy. Verified by build +
    `test -f dist/mockServiceWorker.js` → absent.
  - `check:dist-msw` fires on both arms: a planted `dist/mockServiceWorker.js`
    (any content) → exit 1, `matched filename`; a planted `mockServiceWorker`
    string inside `dist/assets/index-*.js` → exit 1, `matched
    mockServiceWorker`. Clean build → exit 0. `scripts/checks/mswExclusion.test.ts`
    already covers both arms plus false-positive resistance (`msword`,
    `worker` in isolation).
  - The strip plugin is additionally covered end-to-end in CI: the
    committed `public/mockServiceWorker.js` would land in `dist` if the
    plugin regressed, and CI's `check:dist-msw` step runs against the
    real `npm run build` output.
  - Dev-only paths walked (`e2e/vite.config.ts`,
    `e2e/a11y-gallery/vite.config.ts`, `e2e/benchmark/vite.config.ts`,
    `src/mocks/**`, `src/test/setup.ts`, `scripts/gen-fixtures.ts`): none
    is reachable from the `index.html → src/main.tsx` production import
    graph. `src/test/setup.ts` (imports `mocks/node`) is Vitest-only; the
    three `e2e/*` Vite configs are invoked only by Playwright's
    `webServer` and none produces `web/dist`.
  - **One Informational note (A-0004-01):** `src/main.tsx`'s
    `enableMocking()` body is eliminated, but the surrounding
    `.catch(...).finally(render)` promise chain is retained, so the
    English log string `"MSW dev worker failed to start; continuing
    without mocks"` survives into the bundle. It carries no `msw` /
    `mockServiceWorker` / `setupWorker` reference and has zero impact —
    recorded only for tidiness.
- **Evidence (secrets in bundle, T4)** — against a real `npm run build`:
  - `check:dist-secrets` on the clean build → exit 0. A planted
    `AKIA…`-shaped key in `dist/assets/index-*.js` → exit 1,
    `matched AKIA[0-9A-Z]{16}`; restored → exit 0.
    `scripts/checks/secretsGrep.test.ts` covers the AWS-key pattern, the
    generic `apiKey:`/`secret:`/`token:` assignment pattern, and a secret
    in `index.html` at the dist root (not just `assets/`).
  - Manual scan of `dist/` for `127.0.0.1` / `localhost` / `0.0.0.0` /
    dev ports: **one** hit — `http://localhost` inside a bundled
    dependency (React Router's internal history-base fallback, `function
    F(){let r=\`http://localhost\`; e&&(r=e.location.origin…)}`),
    immediately overwritten by `window.location.origin` at runtime. Not
    our code, not a secret, no impact.
  - No `/home/…` or `/Users/…` absolute path in `dist/`.
  - No `.map` / source-map files emitted (`build.sourcemap` is Vite's
    default `false`); no `process.env`, `DATABASE_URL`, or `.env`-shaped
    content.
  - The only production `console.*` call is `main.tsx`'s dev-only,
    build-unreachable MSW-failure log (see T3).
  - `storybook-static/` is git-ignored and untracked, and no Go code
    `go:embed`s `web/dist` yet (phase 05/06) — the Storybook build is
    never a shipped artifact.
- **Evidence (host-only content exposure, T5)** — see attacker 1: the
  shell currently renders every capability because the mock grants all,
  which `architecture-frontend.md` FR-3 makes correct for the
  loopback-only phase-04/pre-12 state. Accepted, phase-gated
  (A-0004-03). The gate is server-told and fails closed, so phase 12/13
  attaches real enforcement without restructuring.
- **Result (attacker 3):** No leak. The bundle carries no secret and no
  mock layer; the "host-only content renders for everyone" behaviour is
  an accepted, documented, phase-gated design decision, not a
  vulnerability.

### 4. Hostile **content** — a book file that is fine to possess but not to parse

No file parsing exists in phase 04 (format detection and the
EPUB/PDF/CBZ extractors are phase 10, `backend-file-extractors.md`). The
generated-cover system consumes only already-validated domain fields
(title, author, identifier) passed as props — never bytes, never a
parsed structure. This attacker category has no reachable surface in
phase 04 and is scoped out, as audit `0001` scoped out the same category
for phase 03's pre-implementation spec audit.

### What is logged that shouldn't be?

The frontend has no server-side logging. Client-side: a whole-tree grep
for `console.*` / logging calls, checked for anything that would print a
credential, a token, a home-directory path, or the content of what
someone is reading (constitution §8).

- **Evidence (T3/T4):** the only `console.*` call in production source is
  `main.tsx`'s dev-only `console.error('MSW dev worker failed to start;
  continuing without mocks', error)` — no credential, token,
  home-directory path, or reading content, and unreachable in a
  production build.
- **Result:** Nothing logged that shouldn't be.

### What happens when input is malformed, empty, enormous, duplicated, slow, or never arrives?

The capability value never arriving is a designed state (`RequireCapability`
holds `loading`, `architecture-frontend.md`'s illegal-transition rule;
T5's fail-closed test). A malformed/empty API response degrades to
`ErrorState` with the response's `correlationId` (`QueryResult`, covered
by `QueryResult.test.tsx` / `routes.test.tsx`); an over-long title is
clamped by `TitleLayer`'s `line-clamp-3 break-words`
(`TitleLayer.test.tsx`); an empty successful response renders
`EmptyState`, not a blank pane (`Library.test.tsx`). This audit confirms
these degrade safely rather than re-deriving the coverage.

- **Evidence:** T3 (rendering), T5 (capability fail-closed); existing
  component/integration suites for the rest.

### What happens if two of these run at once?

No shared mutable server state exists in phase 04. The one module-level
cache (`seedCache.ts`) is keyed by a deterministic hash of an identifier,
so concurrent reads for the same identifier cannot produce divergent
results. Not a concurrency-sensitive surface.

## `/security-review` pass (branch)

Run against the actual branch diff — **20 commits vs `main`** (Tiers 5
*and* 6; `feat/phase04-tier5-accessibility` is not yet merged — see "What
was not examined"). Not the thin pass originally predicted. Result: **no
HIGH or MEDIUM security findings.** The diff adds accessibility grep
checks (Node build-time scripts, fixed path args, no untrusted input),
`src/a11y.css` (a `@media (prefers-contrast)` token override, no
content-exposing selectors), the a11y-gallery Playwright harness
(test-only, not in the app graph), DataTable roving-tabindex/keyboard
nav (no new sink), and the Tier 6 audit doc + two regression tests. No
`dangerouslySetInnerHTML` / `innerHTML` / `eval`; no secret; the CI
workflow adds only `npm run` check steps (no `pull_request_target`, no
untrusted-input eval). Two sub-threshold items carried below (A-0004-01,
A-0004-02).

## `agent-skills:security-auditor` subagent — independent four-attacker pass

An independent pass over `web/`, briefed with the four boundaries and
five sub-scopes, given no access to the T2–T6 findings. It performed its
own real `npm run build` + `dist/` inspection, `npm audit` (0
advisories), and whole-tree greps.

**It reached the same conclusions and found no new issue at Medium or
above.** Six items, all **Low** or **Informational**, most forward-
looking at what phase 05/06 inherits. Its findings are merged into the
table below (IND-01 → A-0004-05, IND-02 → A-0004-04, IND-03 → A-0004-06,
IND-04 → A-0004-07, IND-05 → A-0004-08, IND-06 concurs with A-0004-02).
Its "candidates investigated and rejected" list (12 candidates, all
high-confidence rejections — prototype pollution, `favicon.svg` script
execution, open redirect, storage-API token leakage, ReDoS in the check
scripts, concurrency, domain-boundary leak) is folded into the section
below.

## `agent-skills:security-and-hardening` checklist walk

| Checklist area | Result for phase 04 |
|---|---|
| Threat model / trust boundaries | Done — four boundaries, four attackers (above) |
| Validate external input at the boundary | `getJson<T>` casts the API response (`… as T`) with no runtime shape check — **A-0004-05** below. Mock-only in phase 04; phase 06 must normalise here (§3/§4). |
| Parameterised DB queries | N/A — no database in the frontend |
| Output encoding / XSS | React auto-escaping, no bypass (T3) — clean |
| HTTPS / transport | N/A phase 04 — transport is phase 13; loopback in dev |
| Password hashing, session cookies, auth rate-limiting | N/A — no authentication surface (phase 12) |
| Security headers (CSP, HSTS, X-Frame-Options, X-Content-Type-Options) | None set — no `index.html` CSP meta, and no serving layer yet (phase 05/06) — **A-0004-04** below |
| Secrets in code / git history | None (T4) |
| Logging sensitive data | Only a dev-only, build-unreachable MSW log string (T3/T4) — clean |
| Client-side validation as a security boundary | The capability gate is explicitly *not* the boundary — server-told, phase 12 enforces (A-0004-03) |
| `eval` / `innerHTML` with user data | None (T3) |
| Auth tokens in client storage | No `localStorage` / `sessionStorage` anywhere; `seedCache` is a deliberately in-memory `Map` |
| Stack traces exposed to users | `RouteError` renders a fixed generic string for a non-`ApiError` (T3) |
| SSRF (server-side URL fetch) | N/A — every `fetch` path in `getJson` is a hardcoded literal resolved against `window.location.origin`; client-side, not server-side |
| Dependency audit / one lockfile / frozen install | `web/package-lock.json`, `npm ci` in CI, `npm audit --audit-level=high` as a CI gate — verified at T13 |
| Dependency install scripts blocked | Not blocked (npm default). Recorded as a hardening observation, not a finding — no untrusted install surface in this repo's flow |
| PII / data privacy | N/A — phase 04 collects no user data, has no accounts |
| AI / LLM output handling | N/A — no LLM features |

## Findings

Merged from all three passes + T2–T6. **No Critical or High finding.**
Every item is Informational, Low, or an accepted phase-gated design
decision. A-0004-07 was fixed in this tier (T8); the rest are
carried-forward hooks for phases 05/06/12 or accepted judgment calls.

| ID | Severity | Title | Status |
|---|---|---|---|
| A-0004-01 | Informational | Dev-only MSW error-log string retained in the production bundle | Open (accepted judgment call) |
| A-0004-02 | Informational | `Library` interpolates an unvalidated `item.id` into a `<Link>` path | Carried → phase 06 |
| A-0004-03 | — | Every capability renders (mock grants all) — loopback-only until phase 12/13 | Accepted (phase-gated) |
| A-0004-04 | Low | No Content-Security-Policy / security headers on the shipped shell | Carried → phase 05/06 |
| A-0004-05 | Low | `getJson` has no timeout, size cap, or response-shape check (constitution §4) | Carried → phase 06 |
| A-0004-06 | Informational | Cross-origin / DNS-rebinding reachability of `/api/*` once served | Carried → phase 05/06 |
| A-0004-07 | Informational | `axe-core` declared as a runtime `dependency` | **Fixed (T8)** |
| A-0004-08 | Informational | No install-script policy / provenance check in CI (supply chain) | Open (repo-wide, recommended to maintainer) |

### A-0004-01 — Dev-only MSW error-log string retained in the production bundle

**Severity:** Informational

**Component:** `web/src/main.tsx`

**Description** — `enableMocking()`'s body (the `await import('./mocks/browser')`)
is dead-code-eliminated behind the statically-false `import.meta.env.DEV`,
but the surrounding `enableMocking().catch(...).finally(render)` promise
chain is retained, so the string literal `"MSW dev worker failed to
start; continuing without mocks"` appears in `dist/assets/index-*.js`.

**Impact** — None. No `msw` / `mockServiceWorker` / `setupWorker`
reference, no behaviour, no data. A reader disassembling the bundle sees
one English phrase referencing a dev concept.

**Preconditions** — None.

**Recommendation** — Optional tidiness: hoist the whole `enableMocking`
call behind `if (import.meta.env.DEV)` in `main.tsx` so the production
bundle carries no MSW-adjacent strings at all. Not required for the
spec's exclusion guarantee, which is met.

**Resolution** — Accepted judgment call: not fixed. The string carries no `msw` reference and zero impact; hoisting the promise chain behind `import.meta.env.DEV` is optional tidiness a later `main.tsx` change can fold in.

### A-0004-02 — `Library` interpolates an unvalidated `item.id` into a `<Link>` path

**Severity:** Informational

**Component:** `web/src/screens/Library/Library.tsx`

**Description** — `<Link to={\`/book/${item.id}\`}>` builds a route path
from `item.id`, which in phase 06 will come from the backend / a metadata
provider. React Router resolves `to` as a path, not a URL (no
`javascript:` execution), but an `id` containing `/` or `..` segments
could resolve the link to a different route than intended (e.g.
`item.id = "x/../settings"` → `/settings`).

**Impact** — None in phase 04: the fixture ids are `ol-1`…`ol-3`. In
phase 06 the worst case is a crafted metadata id navigating the user to
an unexpected in-app route — still gated by the (phase 12) server-side
capability check for any host-only destination.

**Preconditions** — Phase 06 wiring a real backend; an attacker
controlling a work/edition id.

**Recommendation** — Phase 06: validate the id shape at the `src/data/`
boundary (it is an opaque token, `domain-bibliographic.md`), or navigate
by route param object rather than string interpolation.

**Resolution** — Carried to phase 06 (`frontend-library-screens.md`).

### A-0004-03 — Every capability renders (mock grants all)

**Severity:** Accepted (phase-gated) — not an open finding

**Component:** `web/src/app/capability/*`, `web/src/data/bootstrap.ts`

**Description** — The mock `/api/bootstrap` grants every capability, so
every host-only route renders for any client, including a future LAN
viewer.

**Impact** — None in phase 04: constitution §6 keeps the host bound to
loopback until phase 12/13, and `architecture-frontend.md` FR-3 states
every connection is therefore trusted at the same level as Electron's
renderer until then. There is no LAN client yet.

**Preconditions** — Phases 12/13 (authentication, network access) — at
which point the real bootstrap endpoint supplies a per-client capability
set and this stops being "grant all".

**Recommendation** — None. The load-bearing property — the gate is
server-told and fails closed (verified T5), giving phase 12/13 a single
enforcement seam — is correct. Recorded as an accepted, documented,
phase-gated design decision, exactly as audit `0001` recorded the
deliberately unauthenticated `/healthz`/`/readyz`.

**Resolution** — Accepted; whoever implements phase 12 owns the real gate.

### A-0004-04 — No Content-Security-Policy / security headers on the shipped shell

**Severity:** Low (missing defence-in-depth)

**Component:** `web/index.html`; the serving layer (phase 05/06)

**Description** — The built `index.html` carries no CSP `<meta>`, and
phase 04 has no server to set a `Content-Security-Policy`,
`X-Content-Type-Options: nosniff`, `X-Frame-Options` / `frame-ancestors`,
or (in the in-process-TLS bind mode) `Strict-Transport-Security` header.
No spec or ADR line yet records that phase 05/06's serving layer MUST
emit them. Once phase 06 renders source- and metadata-derived text
(phase 07/10), a CSP restricting `script-src` / `object-src` / `base-uri`
is a meaningful second line behind React's escaping; without
`frame-ancestors` the LAN UI can be framed for clickjacking once
reachable (phase 13).

**Impact** — None today (no untrusted text is rendered, no server
exists). Entirely forward-looking: it shrinks the blast radius of any
future escaping mistake and closes a clickjacking vector on the
LAN-served UI.

**Preconditions** — A future DOM-XSS bug in a phase-06+ screen; LAN
exposure (phase 13) for the framing vector.

**Recommendation** — Add an explicit checklist item to
`desktop-host-window-and-serving.md` (phase 05, Electron `BrowserWindow`)
and `backend-http-transport.md` (phase 06, the static-asset response
headers): the served response carries `Content-Security-Policy`
(`default-src 'self'`; Tailwind ships static CSS and there is no inline
script, so no `unsafe-inline` is needed), `X-Content-Type-Options:
nosniff`, `X-Frame-Options: DENY` / `frame-ancestors 'none'`, and HSTS in
the in-process-TLS mode. Not phase 04's to implement (no serving layer).

**Resolution** — Carried to phase 05/06.

### A-0004-05 — `getJson` has no timeout, size cap, or response-shape check

**Severity:** Low (constitution §4 control missing at a boundary that is
mock-only today)

**Component:** `web/src/data/http.ts`

**Description** — `getJson<T>` is the single seam every data hook flows
through (`fetchBootstrap`, `useLibraryItems`, and all of phase 06+). It
calls `fetch(...)` with no `AbortController` / timeout, then returns
`(await res.json()) as T` — a compile-time assertion with no runtime
shape check and no bound on body size. Constitution §4 and CLAUDE.md's
reflexes require "a size limit, a shape check, and a timeout" on "every
request reaching the host over the network."

**Impact** — None in phase 04: every response is an MSW fixture whose
shape the frontend controls. Realistic worst case once phase 06 wires the
real backend: a stalled server leaves a LAN client on a permanent
spinner (TanStack Query has no default timeout, the promise never
settles); a malformed `/api/bootstrap` body that parses but omits
`capabilities` makes `can()` throw `undefined['sources']`, sending
host-only routes into a generic `RouteError` instead of the intended
fail-closed `loading` state. Not remotely exploitable, no data exposure.

**Preconditions** — Phase 06 backend wired; a slow or buggy server (or,
only if constitution §6 were ever violated, a pre-TLS LAN MITM).

**Recommendation** — Before phase 06 builds on it, add to `getJson`: a
default request timeout (`AbortSignal.timeout(...)`), a body-size ceiling
(`Content-Length` / streamed), and a per-response-type runtime validator
(a small hand-rolled type guard per type is enough — no new dependency),
so every hook inherits the three §4 controls from one place. This is
already the specs' "phase 06 normalises at the boundary" design;
recorded as an audit-tracked item so it is verified then.

**Resolution** — Carried to phase 06 (`frontend-library-screens.md`,
`architecture-contracts.md`, §3/§4).

### A-0004-06 — Cross-origin / DNS-rebinding reachability of `/api/*` once served

**Severity:** Informational (forward guidance — no server exists)

**Component:** the serving layer (phase 05/06); `web/src/data/http.ts`

**Description** — `getJson` issues `GET` with only `Accept:
application/json` — a CORS-"simple" request, no preflight. Once phase
05/06 serve the API on a loopback / LAN port, a hostile web page in the
user's browser, or a short-TTL DNS name rebinding to `127.0.0.1`, can
*send* requests to `/api/*`. CORS blocks *reading* a GET response but not
issuing the request, and phase 06+ adds state-changing endpoints.

**Impact** — None in phase 04 (no server, no state-changing endpoint).

**Preconditions** — Phase 06 serving the API; phase 06+ state-changing
routes.

**Recommendation** — Phase 05/06: validate the `Host` header against an
allowlist on every non-health route, and require state-changing requests
to carry a non-simple content type or an anti-CSRF token. Recorded here
so it is not rediscovered.

**Resolution** — Carried to phase 05/06 (`backend-http-transport.md`).

### A-0004-07 — `axe-core` declared as a runtime `dependency`

**Severity:** Informational

**Component:** `web/package.json`

**Description** — `axe-core` was listed under `dependencies`, but it is
imported only by `*.test.tsx` and `src/test/axe.ts` (test-only). Verified
absent from `dist/assets/*.js` (Vite tree-shakes it). A classification
error that inflated the dependency surface seen by `npm install
--omit=dev` and supply-chain reasoning; no runtime or bundle impact.
(`msw` staying in `dependencies` is deliberate — the code comments
explain it, and `check:dist-msw` + the strip plugin enforce its
exclusion.)

**Impact** — None functional. Supply-chain-hygiene only.

**Preconditions** — None.

**Recommendation** — Move `axe-core` to `devDependencies`.

**Resolution** — **Fixed (T8)** — moved to `devDependencies`; lockfile
updated; build, the axe test suites, and `npm audit` re-verified.

### A-0004-08 — No install-script policy / provenance check in CI

**Severity:** Informational (repo-wide standing tooling policy, not a
phase-04 defect)

**Component:** repository root — no `.npmrc`; `.github/workflows/ci.yml`

**Description** — CI runs `npm ci` (which executes dependency lifecycle
scripts by default) and gates only on `npm audit --audit-level=high`.
There is no `ignore-scripts` policy with a reviewed allowlist and no
`npm audit signatures` (provenance) step. `npm audit` matches known
advisories only — it does not catch a newly-malicious or typosquatted
package.

**Impact** — None demonstrated. Defence-in-depth against a future
supply-chain compromise.

**Preconditions** — A malicious or compromised package entering the
dependency graph.

**Recommendation** — Repo-wide, low urgency, and a maintainer decision
(adding `ignore-scripts=true` can break local installs until an
allowlist is tuned): add `.npmrc` with `ignore-scripts` + a minimal
allowlist, and an `npm audit signatures` CI step. Not implemented in this
tier — flagged for the maintainer.

**Resolution** — Open; recommended to the maintainer as standing tooling
work, tracked here.

## Candidates investigated and rejected

- **XSS via `TitleLayer` / `AuthorLayer` / `ErrorState` rendering
  provider text** — every value is a React-escaped JSX text child; no
  `dangerouslySetInnerHTML`. Guard test added (`xssEscaping.test.tsx`).
  Confidence it is a real issue: 1/10.
- **CSS injection via the generated-cover `hsl(...)` template literals**
  — the interpolated `seed.hue` is `fnv1a(id) % 360`, a bounded integer;
  `seed.pattern` is one of three literals. No caller-controlled string
  reaches `style`. Guard test added. Confidence: 1/10.
- **Capability gate spoofable client-side** — no code infers privilege
  from port / origin / `isElectron` / user-agent / env (grep, T5). The
  gate reads one server-told value and fails closed. Confidence: 1/10.
- **MSW / fixtures shipped to production** — dead-code-eliminated;
  `check:dist-msw` (both arms) + the strip plugin verified against a real
  build (T2). Confidence: 1/10.
- **Secret / token / DSN in the bundle** — `check:dist-secrets` verified;
  manual scan clean; the one `localhost` literal is a bundled-dependency
  runtime fallback (T4). Confidence: 1/10.
- **`RouteError` leaking a stack trace to a LAN viewer** — renders a
  fixed generic string for any non-`ApiError`; an `ApiError` shows only
  its own `message`/`code`/`correlationId`. Confidence: 1/10.

From the `security-auditor` subagent's independent pass (all
high-confidence rejections):

- **Prototype pollution** via `deriveSeed`'s `%`-indexing, the
  `seedCache` Map, or fixture object spreads — array index access, a
  `Map` with string keys, no user-controlled object merge. 1/10.
- **`favicon.svg` script execution** — the file has only `<path>` /
  `<ellipse>` / `<filter>` elements, no `<script>` / handlers /
  `<foreignObject>`, and `<link rel="icon">` does not execute SVG script
  regardless. 1/10.
- **Open redirect / `window.open` / `location.*` assignment /
  `target="_blank"` without `rel`** — none present; every navigation
  target is a static literal except A-0004-02. 1/10.
- **Auth token / sensitive data in `localStorage` / `sessionStorage` /
  IndexedDB** — none of these APIs is used anywhere. 1/10.
- **ReDoS in `secretsGrep.ts` / the check-script regexes** — no
  catastrophic-backtracking construct; CI-only, over trusted build
  artifacts. 1/10.
- **Concurrency / shared mutable state** — no server process; the one
  module-level cache is hash-keyed and deterministic; `makeQueryClient`
  is a per-entry-point factory. 1/10.
- **Domain-boundary leak** (Open Library / OPDS shape reaching a
  component) — `data/` holds only normalised shapes; no adapter exists
  yet; `gen-fixtures.ts` reads the contract, which defines only health +
  the shared `Error`. 1/10.

## Severity guide

Per `.claude/templates/audit.md` — rated against actual impact in this
system, not the textbook worst case for the class of bug. Inflated
severity is as damaging as a missed finding (constitution §10, forbidden
in both directions).

## What was not examined

- **Runtime behaviour of a real server.** Phase 04 has no Go server
  process; memory, timing, concurrency, and the actual response-header
  set are phase 05/06 and were not exercisable here.
- **Dependency CVEs beyond `npm audit --audit-level=high`.** That gate
  runs in CI and is re-verified at T13; a deeper SCA (provenance,
  transitive-graph review) is out of scope for this audit.
- **The "malicious source" and "hostile book file" attackers** — no
  source integration (phase 08) or file parsing (phase 10) exists to
  exercise. Scoped out with reasoning above, per audit `0001`.
- **Branch topology note (not a security gap):** this audit's
  `/security-review` pass and the whole Tier 6 branch stack on
  `feat/phase04-tier5-accessibility`, which is **not yet merged to
  `main`** (20 commits ahead: 14 Tier 5 + 6 Tier 6). The phase-04 close
  and the Tier 6 PR both depend on Tier 5 (PR #64) merging first, or on
  Tier 6 being a stacked PR. Flagged to the maintainer at Gate 1 —
  recorded here because it changes what "the phase 04 surface" means for
  a later re-reader of this audit.
- **Real-browser runtime behaviour** — no Playwright / browser run was
  performed as part of this audit; findings are from static reading and
  build inspection. (The phase's own `axe-core` / Playwright suites run
  in CI and are re-verified at T13.)
- **Enormous-input rendering cost** — a multi-MB title string would sit
  in the DOM (clamped only visually by `line-clamp-3`); the size limit
  for that belongs at the Go / metadata boundary (phase 07,
  constitution §4), so it is inherited, not a phase-04 finding —
  captured under A-0004-05's recommendation.
