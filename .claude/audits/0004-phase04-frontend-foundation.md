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

- **Evidence:** _(T5)_
- **Result:** _(T5)_

### 2. A malicious **source** or metadata provider

_(T3 — XSS surface sweep; T6 — domain-boundary)_

No source or metadata integration is wired in phase 04 (phases 07/08).
The audit's job is to confirm phase 04 does not make XSS-safe rendering
*harder to keep* once that text arrives. Concrete surfaces (F26's own
wording): the generated-cover layers, `ErrorState` / `RouteError` (which
render an API error's `code` / `message` / `correlationId`),
`ScreenPlaceholder` / `ParamPlaceholder` (which echo route params), and
the MSW fixtures.

- **Evidence:** _(T3, T6)_
- **Result:** _(T3, T6)_

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
- **Evidence (secrets in bundle):** _(T4)_
- **Evidence (host-only content exposure):** _(T5)_

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
