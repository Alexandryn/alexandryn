# Phase 04, Tier 6 — Closure — task list

Full plan: [`tasks/plan-p04-tier6-closure.md`](plan-p04-tier6-closure.md).
Tier 6 builds no UI. Every code change is a test, a check-script
hardening, or a doc edit. **Two mandatory stop-and-ask gates** —
constitution §10 / Review-gates clause — neither crossable on the
automated contributor's own judgement.

## Decisions (resolve once)

- [x] D1 — audit file: `.claude/audits/0004-phase04-frontend-foundation.md`, findings `A-0004-NN`
- [x] D2 — carried item 1 (`text-3` AA): **accept permanently** (maintainer, 2026-08-28) — text-3 is secondary/decorative labelling only
- [x] D3 — carried item 2 (test plans): **ratify the per-tier convention** (maintainer, 2026-08-28) — ADR 0016 → Accepted, amend `test-plans/README.md`, add an exit-criteria line
- [x] D4 — Gate 1 audit **APPROVED** (maintainer, 2026-08-28); F27 into PR #65 retitled; carried items 3/4/5 record-only, confirmed

## F26 — Security audit

- [x] T1 — audit doc skeleton from `.claude/templates/audit.md`: Scope/method (incl. the design-conformance sync result + the `/security-review` thin-diff note), the four-boundary table, the four-attacker section headers. Commit (doc).
- [x] T2 — **MSW / mock-boundary production leak**. `npm run build`; (a) plant `dist/mockServiceWorker.js` + an MSW string in a `dist/assets/*.js` → assert `check:dist-msw` exits nonzero on both arms (filename + content), restore; (b) drop a dummy `public/mockServiceWorker.js`, build, assert absent from `dist/` (the `strip-msw-worker-from-build` plugin); (c) grep the built `dist/assets/*.js` for `setupWorker` / `mocks/browser` / `msw` → assert the DEV-gated dynamic import in `main.tsx` is dead-code-eliminated; (d) walk every dev-only path — `e2e/vite.config.ts`, `e2e/a11y-gallery/vite.config.ts`, `e2e/benchmark/vite.config.ts`, `src/mocks/**`, `src/test/setup.ts`, `scripts/gen-fixtures.ts` — confirm none is in the `src/main.tsx` production import graph. Regression test if any gap. Record findings + evidence in the audit doc. Commit (test + doc).
- [x] T3 — **XSS surface sweep**. Whole-`web/` grep: `dangerouslySetInnerHTML`, `innerHTML|outerHTML|insertAdjacentHTML` (writes), `document.write`, `eval(`, `new Function` → expect zero in production source (test-file `container.innerHTML` reads are RTL assertions, not sinks — note them). Read each surface: `GeneratedCover` layers (+ `lib/fnv1a.ts` — confirm `seed.hue` is a bounded number, no string interpolation into `style`), `ErrorState`, `RouteError`, `ScreenPlaceholder`/`ParamPlaceholder`, MSW fixtures. Grep URL sinks: interpolated `href=`/`src=`/`to=`, `window.open`, `location.assign|replace|href =`, non-literal `<Navigate to=`, non-literal RR `redirect(`. Add a regression test: `GeneratedCover` / `ErrorState` rendered with a `<script>` / `<img onerror>` payload in title/author/message/correlationId → payload is escaped text, no element created. Record findings / Informational notes + evidence. Commit (test + doc).
- [x] T4 — **No secrets in the bundle**. `npm run build`; run `check:dist-secrets`; plant an `AKIA…`-shaped key + an `api_key: "…"` assignment in a `dist` file → assert nonzero exit, restore. Manual scan of `dist/` for: home-dir paths (`/home/`, `/Users/`), hardcoded `localhost`/`127.0.0.1` (should be none — `http.ts` uses `window.location.origin`), `.env`-shaped content, shipped `.map` files with absolute paths (confirm `vite.config` default `sourcemap: false`). Confirm `storybook-static` is gitignored + never `go:embed`'d. Record + evidence. Commit (doc; eng only if a gap).
- [x] T5 — **Capability gating is server-told, never client-inferred**. Whole-tree grep for client-side privilege inference: `navigator.userAgent`, `window.electron`, `process.type`, `location.hostname|location.port` in a gating decision, `import.meta.env` used for capability. Read `CapabilityContext`, `CapabilityProvider`, `RequireCapability`, `routes.tsx` `hostOnly()` → confirm (a) value only from the `/api/bootstrap` query, (b) loading/failure never degrades to `granted`, (c) no route infers its own privilege. Confirm `capability.test.tsx` covers withhold-then-grant / no-flash (FR-4). Record: "every capability renders now" = Accepted phase-gated risk (loopback-only until phase 12/13), documented like audit `0001`'s `/healthz`, not a finding. Commit (doc).
- [x] T6 — **Domain-boundary (§3)**. Read `src/data/{bootstrap,library,http}.ts` + MSW fixtures → confirm every shape is a normalised Alexandryn / `architecture-contracts.md` shape, not an Open Library response shape or a source protocol shape; no adapter bypassed (none exist yet). Confirm `GeneratedCover` takes pre-reduced `author: string`, never `Author[]`. Record + evidence. Commit (doc).
- [x] T7 — Run `/security-review` (branch); run the `agent-skills:security-auditor` subagent (full four-attacker threat model over `web/`, briefed with the four boundaries + five sub-scopes); load + walk the `agent-skills:security-and-hardening` checklist. Reconcile all three + T2–T6 into the single audit doc: findings table, per-finding detail, honest severity, "candidates investigated and rejected", "what was not examined". Commit (doc).
- [x] T8 — Fix any Critical/High finding — one commit per fix, a regression test per fix. If none: record "no Critical/High findings" explicitly in the audit verdict.

### ═══ STOP GATE 1 ═══

- [x] Present the audit and every finding. **Maintainer sign-off received 2026-08-28** (PR #65 CI green; verdict Clear; carried-item decisions D2/D3/D4 recorded above).

## F27 — Closure (after audit sign-off)

- [x] T9 — Record a decision for every carried item (1–5) in its named doc: item 1 → `frontend-design-tokens.md` Open questions + `check-token-contrast.ts` comment; item 2 → ADR 0016 status + `test-plans/README.md` (or 6 new test-plan docs); item 3 → both specs' Open questions + roadmap, with measured baselines; item 4 → `frontend-generated-covers.md` Open questions (phase-06 follow-up); item 5 → roadmap close note (phase 04 closes without the host/LAN boundary settled) — it is already in both specs + ANALYSIS.md. One commit per doc or a grouped "carried-item decisions" commit.
- [x] T10 — Walk `.claude/roadmap/04-frontend-foundation/README.md` Exit criteria item-by-item, each citing real evidence (test name / CI step / file path / PR number — per the plan's evidence table). Write the walk into the README as a Closure section; update the README header (`Status`, `Opened`, `Closed`). Commit (doc).
- [x] T11 — Move all six `frontend-*.md` spec statuses `APPROVED → IMPLEMENTED → VERIFIED`. Per spec: re-read its Acceptance criteria, cite evidence each is met, then update the spec's status line **and** `.claude/specs/README.md`'s status column. Gated on T10. Commit (doc).
- [x] T12 — Documentation sweep: `web/README.md`, root pointer files (only if they carry a phase-status line), `tasks/todo-phase04.md` (Tier 6 + P4-G done), `.claude/roadmap/README.md` index, CLAUDE.md "Where the task list lives" (verify not stale). Commit (doc).
- [x] T13 — DONE (2026-08-28): build / lint / 257 Vitest / Storybook / prettier / 7 check scripts / staleness / npm audit 0 / 12 Playwright across 3 projects — all green. Full regression (plan §"Full regression"). Kill stray vite on 5174–5176 first. All green.
- [x] T14 — /code-review high fork done: 3 doc-accuracy findings (benchmark number, test-plan wording, phase-03 row), all fixed in 6f9807f. No code findings (docs-only PR). `/code-review high` as a fork. Fix every real finding, regression test per fix, commit each separately.

### ═══ STOP GATE 2 — Checkpoint P4-G (final) ═══

- [ ] Present: full suite green (build / lint / typecheck / Vitest / Storybook / axe-core / all three Playwright projects / bundle-size / npm audit / the two a11y grep checks / token-styling / token-contrast / staleness), security audit recorded with no open Critical/High, all six specs `VERIFIED`, roadmap exit criteria walked with evidence. Wait for maintainer approval to declare phase 04 complete.

## After approval (not this planning scope — recorded for continuity)

- [ ] T15 — Push; open PR (`gh pr create`, `/make-pr` body conventions — **no** "Co-Authored-By" trailer, **no** "Generated with Claude Code" line); `gh pr checks --watch` until green.
- [ ] T16 — Stop. Report PR green + P4-G status + what maintainer approval is still needed. Do **not** sync `main` or delete the branch — wait to be asked.

## Checkpoint P4-G exit

Full suite green · audit in `.claude/audits/` with no open Critical/High ·
six specs `VERIFIED` · roadmap exit criteria checked with evidence ·
maintainer approval recorded → **phase 04 complete**.
