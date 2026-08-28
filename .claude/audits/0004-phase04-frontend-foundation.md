# Security audit: Phase 04 — Frontend foundation

| | |
|---|---|
| **Scope** | `web/` — the phase 04 frontend foundation as merged to `main` (PRs #59–#64): build tooling, design tokens, 18 component primitives, the generated-cover system, the shell + router + capability hook, the MSW mock layer, the accessibility baseline, and the CI pipeline. Tiers 0–5. |
| **Auditor** | Claude (Sonnet 5) — reconciled from three passes: `/security-review` (branch), the `agent-skills:security-auditor` subagent (full four-attacker threat model), and the `agent-skills:security-and-hardening` checklist; plus the manual verification tasks T2–T6 (`tasks/todo-p04-tier6-closure.md`). |
| **Date** | 2026-08-27 |
| **Commit** | _(filled at T7 — the commit the reconciled audit is written against)_ |
| **Verdict** | _(filled at T8 — Clear / Findings open / Blocked)_ |

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

_(T5 — capability-gating verification)_

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

_(T3 — XSS surface sweep; T6 — domain-boundary)_

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

### 3. A device on the **local network** without a credential

_(T2 — MSW leak; T4 — secrets in bundle; T5 — host-only content exposure)_

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

_(T4)_

The frontend has no server-side logging. Client-side: a whole-tree grep
for `console.*` / logging calls, checked for anything that would print a
credential, a token, a home-directory path, or the content of what
someone is reading (constitution §8).

- **Evidence:** _(T4)_
- **Result:** _(T4)_

### What happens when input is malformed, empty, enormous, duplicated, slow, or never arrives?

_(T3, T5)_

The capability value never arriving is a designed state (`RequireCapability`
holds `loading`, `architecture-frontend.md`'s illegal-transition rule).
A malformed/empty API response, an over-long title, and a slow fetch are
each covered by existing component and integration tests; this audit
checks they degrade safely rather than re-deriving that coverage.

- **Evidence:** _(T3, T5)_

### What happens if two of these run at once?

No shared mutable server state exists in phase 04. The one module-level
cache (`seedCache.ts`) is keyed by a deterministic hash of an identifier,
so concurrent reads for the same identifier cannot produce divergent
results. Not a concurrency-sensitive surface.

## Findings

_(Filled at T7 from the reconciled passes; Critical/High fixed at T8
before Gate 1.)_

| ID | Severity | Title | Status |
|---|---|---|---|
| _(T7)_ | | | |

## Candidates investigated and rejected

_(T7 — false positives and design decisions that are not findings, each
with a confidence rating, per audit `0001`'s format.)_

## Severity guide

Per `.claude/templates/audit.md` — rated against actual impact in this
system, not the textbook worst case for the class of bug. Inflated
severity is as damaging as a missed finding (constitution §10, forbidden
in both directions).

## What was not examined

_(Filled at T7 — honest gaps in coverage.)_
