# Phase 04, Tier 6 — Closure — implementation plan

Full phase plan: [`tasks/plan-phase04.md`](plan-phase04.md) (F26–F27,
Checkpoint P4-G). Per-tier sub-doc convention: D1, `tasks/plan-phase04.md`
(same as Tiers 2–5). Task list: [`tasks/todo-p04-tier6-closure.md`](todo-p04-tier6-closure.md).

Tier 6 builds **no UI and no product behaviour**. It is the security
audit (F26), the closure paperwork (F27), and two mandatory
constitution stop-and-ask gates. Every code change in this tier is a
test, a check-script hardening, or a documentation edit — nothing that
changes what the app does (constitution §1's refactor/CI/doc exemption
applies; no new spec is owed for Tier 6 itself).

## Context

Tiers 0–5 are merged to `main` (PRs #59–#64). The frontend foundation
exists: 18 primitives, the generated-cover system, the shell + router +
capability hook, the MSW mock layer, the accessibility baseline, and the
CI pipeline that gates all of it. Six specs sit at `APPROVED`; the
roadmap phase is open.

This tier closes the phase. The constitution requires a hard stop after
the audit and before closing (§10, Review-gates clause), so the flow is
two gates, neither crossable on the automated contributor's own
judgement:

```
F26 audit + Critical/High fixes ──▶ STOP: present audit, wait for maintainer sign-off
                                        │
F27 spec statuses / exit-criteria walk / docs
                                        │
                                   full regression + /code-review high fork
                                        │
                                   STOP: Checkpoint P4-G, wait for final maintainer approval
                                        │
                                   push, PR, gh pr checks --watch, report, stop
```

## Design-conformance check (done 2026-08-27, this session)

Re-synced all four `.dc.html` canvases via the `DesignSync` MCP against
project `78075626-e444-438f-8437-205d57129a37` (`get_project`,
`list_files`, `get_file` per path) and diffed each against
`.design-reference/`:

| Canvas | Result |
|---|---|
| `Alexandryn-Electron.dc.html` | SHA-256 identical to local |
| `Alexandryn-Electron-Admin.dc.html` | SHA-256 identical to local |
| `Alexandryn-Web.dc.html` | SHA-256 identical to local |
| `Alexandryn-Mobile.dc.html` | SHA-256 identical to local |

No drift since the 2026-08-13 sync. `.design-reference/ANALYSIS.md`'s
scope-classification table (binding / exploratory / **unclassified**) and
its `atTablet` note are current as written. `get_project` reports
`type: PROJECT_TYPE_PROJECT` (not `PROJECT_TYPE_DESIGN_SYSTEM`) and
`canEdit: true` — unchanged, read-only use here, no push.

Tier 6 builds no UI, so no binding/exploratory/unclassified screen is
implemented against. F27's roadmap-exit-criteria walk (T10) does cite the
design reference for items 2–5; those citations use this sync.

**`atTablet` flagged again** (carried item 5): captured inside the
Electron/Admin (host) canvas, not the Web canvas ADR 0003 assigned it to.
Not phase 04's to resolve — F27 records it as an explicit open question
carried into phase 05/13, and phase 04 does **not** close as if the
host/LAN-client screen boundary were settled.

## F26 — Security audit (constitution §10 four-attacker pass)

### Scope and trust boundaries

Phase 04 is a fully-mocked, client-side-rendered frontend. It ships **no
server, no authentication, no Electron IPC, and no file parsing** — those
are phases 05/06/10/12. The audit is scoped to phase 04's *real* trust
boundaries and to the properties phase 06+ inherits from this shell:

| Boundary | Untrusted side | Assumption being audited |
|---|---|---|
| `web/dist` → any browser it is served to (incl. a LAN client, `architecture-system.md` FR-6) | The bundle is world-readable once phase 05/06 serve it | No secrets, no mock layer, no host-dir paths in the shipped artifact |
| Future source / file / metadata text → React render tree | Titles, authors, filenames, API `code`/`message`/`correlationId`, route params (phase 06/07/08/10) | React's default escaping holds; no `dangerouslySetInnerHTML`, no HTML/URL sink anywhere in the tree |
| Server capability value → `useCapability()` → route rendering | The renderer is not trusted to decide its own privilege (`architecture-frontend.md` FR-3) | The gate is server-*told*, never client-*inferred*; failure fails closed (`loading`, never `granted`) |
| Metadata-provider / source shapes → `src/data/` → UI (constitution §3) | Open Library response shapes, source protocol shapes | Nothing un-normalised reaches a component; no adapter is bypassed |

### The four attackers (each walked, N/A ones scoped out with reasoning, per audit `0001`'s precedent)

1. **Malicious user of this instance** — no auth surface, no per-user
   data, no active privilege tiers (the mock grants every capability).
   The real angle is the *capability-gating architecture*: does any code
   infer privilege from a client-side signal (port, origin,
   `isElectron`, user-agent, `import.meta.env`)? If so, the shell hands
   phase 12/13 a spoofable gate. Spot-check already done — only
   `window.location.origin` (URL base in `data/http.ts`, not a privilege
   check) and `import.meta.env.DEV` (the MSW gate in `main.tsx`, not a
   privilege check) appear. T5 confirms exhaustively.

2. **Malicious source / metadata provider** — sources arrive phase 08,
   metadata phase 07; nothing is wired. Phase 04's job is to not make
   XSS-safe rendering *harder to keep* later. Concrete surfaces (from
   F26's own wording): the generated-cover layers, `ErrorState` /
   `RouteError` (render API `code` / `message` / `correlationId`),
   `ScreenPlaceholder` / `ParamPlaceholder` (echo route params), the MSW
   fixtures. T3 sweeps all of them plus a whole-tree grep for
   `dangerouslySetInnerHTML` / `innerHTML` writes / `document.write` /
   `eval` / URL sinks.

3. **Device on the local network (no credential)** — phase 04 ships no
   listener, but `web/dist` is the artifact a LAN client is served. Two
   angles: (a) does the bundle leak anything a LAN client shouldn't see
   (T4: `check:dist-secrets` re-verified against a real build); (b) does
   the shell expose host-only *content* to a viewer (T5: today every
   capability renders — this is *by design*, loopback-only until phase
   12/13, `architecture-frontend.md` FR-3 — recorded as an **Accepted,
   phase-gated risk**, exactly as audit `0001` recorded the
   unauthenticated `/healthz`, not as a finding). The browser-visiting-a-
   hostile-page / DNS-rebinding angle has no state-changing endpoint to
   hit in phase 04's mock — noted Informational, carried to phase 12.

4. **Hostile content (a book file)** — no file parsing in phase 04
   (extractors are phase 10). Fully N/A; the generated-cover system takes
   only already-validated domain fields as props, never bytes. Scoped
   out with reasoning, per audit `0001`'s handling of the same category.

### Method

Run all three, then reconcile into one honest write-up (F26's explicit
instruction):

1. **`/security-review`** on the branch. Recorded honestly: the branch
   diff vs `main` is Tier 6's doc/test additions only (Tiers 0–5 are
   already merged), so this pass is naturally thin. Its value is
   confirming the Tier 6 changes themselves introduce nothing; the
   four-attacker depth comes from (2) and the manual tasks.
2. **`agent-skills:security-auditor` subagent** — full four-attacker
   threat model over the whole `web/` tree, briefed with the four
   boundaries and five sub-scopes above. This is where coverage depth
   lives.
3. **`agent-skills:security-and-hardening` skill** — load its checklist
   and walk it against `web/`.

Plus the manual verification tasks T2–T6 below, each producing concrete
evidence (a nonzero exit code, a grep result, a built-bundle inspection).

Reconcile all of it into **`.claude/audits/0004-phase04-frontend-foundation.md`**
(next sequential number; template `.claude/templates/audit.md`; findings
`A-0004-01`…). Severity rated honestly — the constitution forbids
inflating *or* deflating (§10). Fix Critical/High before Gate 1 with a
regression test per fix; document accepted Medium/Low as judgment calls.

### Expected outcome (not a pre-judgement — recorded so the calibration is explicit)

Based on the spot-checks already done: **0 Critical, 0 High** is the
likely result. Plausible lower-severity findings — the `ParamPlaceholder`
raw-param-echo *habit* (Informational: safe as a text node, a sink the
moment phase 06 puts a param in an attribute); the interim breadth of
`secretsGrep.ts`'s pattern set (Low / defence-in-depth); the "every
capability renders" phase-gated design decision (Accepted, not a
finding). If T2's built-bundle inspection shows the MSW dynamic import is
*not* fully dead-code-eliminated, that is a real Medium. The plan does
not assume the outcome; T7 rates whatever T2–T6 actually find.

## F27 — Closure

### Carried-forward items — each needs a decision recorded in Tier 6

These are put to the maintainer in this plan (and again at Gate 1 if not
answered here). None is silently deferred.

| # | Item | Options | Plan's recommendation |
|---|---|---|---|
| 1 | `text-3` base-case WCAG AA contrast exception (`web/scripts/check-token-contrast.ts`) — Tier 5 mitigated it under `prefers-contrast`; the default-palette decision is still open | (a) accept permanently (text-3 is decorative / non-essential text only) · (b) darken the token · (c) escalate for a design-reference change | **(a)**, *if* the maintainer confirms text-3 is only ever secondary/decorative labelling. text-3 is `#9C978F` straight from every canvas's `:root` (`--tx3`); darkening it unilaterally violates `frontend-design-tokens.md` FR-4 (no invented values), and (c) is heavier than the risk warrants given Tier 5's `prefers-contrast` mitigation already covers the high-contrast user. Record the acceptance in `frontend-design-tokens.md` (Open questions → resolved) and update the `KNOWN_EXCEPTIONS` comment in `check-token-contrast.ts` to cite the decision. |
| 2 | Phase-04 formal test plans (ADR 0016, still "Proposed"; Tier 5 G3 deferred the reconcile to here) | (a) author `.claude/test-plans/frontend-*.md` for all six specs now · (b) move ADR 0016 → Accepted and amend `test-plans/README.md` to ratify the per-tier plan-doc convention Tiers 0–5 actually used | **(b)**. ADR 0016's own Option A reasoning already argues against writing test plans far ahead of implementation; Tiers 0–5 carried their test strategy in the per-tier `tasks/plan-p04-*.md` docs, which *is* "before that spec's RED step". Moving 0016 to Accepted, plus adding the exit-criteria checklist line 0016's "Against" section names (a per-phase "test plans exist" line), closes the enforcement gap without 6 retrospective documents. If the maintainer wants (a), it is ~6 × 200–300-line docs and belongs in its own task block. |
| 3 | FR-4 numeric budgets (250 KiB gzipped bundle; `frontend-generated-covers.md`'s 100ms / 500-cover / 60fps) — both specs' Open questions defer confirm-or-replace to phase 06 | Record the deferral explicitly with the **current measured baseline**, not leave it implicit | Measure now (`check:bundle-size` output; the Playwright `benchmark` project's actual numbers) and write "phase-04 baseline: bundle ≈ N KiB gzipped, benchmark passing at M ms initial paint" into both specs' Open questions and the roadmap close note. No decision to make — just make the deferral concrete. |
| 4 | Benchmark-harness Tailwind-scope gap — `e2e/benchmark` imports `../../src/theme.css` but has no `@source '../../src'`, so Tailwind doesn't generate the utility classes `TitleLayer`/`AuthorLayer` use; the measured cover-render numbers are taken without those styles applied. Tier 5 fixed the same gap in `e2e/a11y-gallery` with `@source` but left `e2e/benchmark` deliberately, because fixing it shifts the measured numbers | (a) fix `@source` in `e2e/benchmark` now and re-baseline · (b) file as a phase-06 follow-up tied to item 3 | **(b)**. Item 3 already defers the benchmark numbers' confirm-or-replace to phase 06; re-baselining twice (once here, once in phase 06 against a real grid) is wasted motion. File it as one phase-06 task with item 3. Record the gap explicitly in `frontend-generated-covers.md` Open questions so phase 06 has the checklist. |
| 5 | `atTablet` file placement (`.design-reference/ANALYSIS.md`, `architecture-frontend.md` / `frontend-shell-and-routing.md` Open questions) — needs the Claude Design project owner's confirmation | Not phase 04's to resolve | F27 records it as an explicit open question carried into phase 05/13. It is *already* in both specs' Open questions and ANALYSIS.md's classification table (row: "Binding, location TBD"); T10/T12 add a line to the roadmap close note stating phase 04 closes **without** the host/LAN screen boundary settled, so phase 05 re-reads this rather than assuming it. |

### Roadmap exit-criteria walk (T10)

`.claude/roadmap/04-frontend-foundation/README.md` has a 10-item Exit
criteria checklist. Each item gets walked with **real evidence** — a test
name, a CI step, a file path, a PR number — the same discipline phase
03's Checkpoint H used, never "looks done":

| Exit criterion | Evidence to cite |
|---|---|
| All six specs `APPROVED` with recorded reviews | reviews `0031` (cross-spec, two agents), `0032` (post-approval amendments); `.claude/specs/README.md` |
| Component library + design tokens implemented and documented | `web/src/components/*` (18 primitives), `web/src/theme.css` + `tokens.css` (generated, `tokens:generate` + `git diff --exit-code` in CI), `check:token-styling` green; PRs #60, #61; `tasks/todo-p04-tier2-component-primitives.md` |
| Generated cover tested at every degradation step | `web/src/components/GeneratedCover/GeneratedCover.test.tsx` (3 ladder steps), `.determinism.test.tsx`, `.a11y.test.tsx`; PR #62 |
| Every primitive keyboard-operable with a visible focus state | `web/src/test/keyboardMap.test.tsx`, `web/src/lib/focusRing.ts`, the Playwright `gallery` project; PRs #61, #64 |
| Automated accessibility tests passing on all shell layouts | `web/e2e/a11y.app.spec.ts`, `web/e2e/a11y-gallery.gallery.spec.ts`, `web/src/test/axe.test.tsx`; CI "Playwright — E2E, accessibility and cover benchmark" step |
| Component preview environment functional | Storybook, `build-storybook` CI step, `*.stories.tsx` for every primitive |
| All specs in this phase `VERIFIED` | T11 (this tier) |
| Security audit recorded, no open Critical/High | `.claude/audits/0004-phase04-frontend-foundation.md` (this tier, F26) |
| Documentation updated | T12 (this tier) |
| Maintainer approval recorded | Checkpoint P4-G (Gate 2) |

The walk is written into the roadmap README as a "Closure" section (or
evidence-annotated checkboxes), and the README header (`Status`,
`Opened`, `Closed`) is updated.

### Spec statuses → `VERIFIED` (T11)

All six phase-04 specs are `APPROVED`. The lifecycle is
`APPROVED → IMPLEMENTED → VERIFIED`; `VERIFIED` means "audited,
accessibility-checked, and confirmed against acceptance criteria"
(`.claude/specs/README.md`). Per spec, before flipping the status:
re-read its Acceptance criteria and cite the evidence that each one is
met (the T10 walk supplies most of it). Then update the spec's own status
line **and** the `.claude/specs/README.md` status column. Only after
T10's exit-criteria walk is complete — status does not move on the
strength of "the code exists".

### Documentation sweep (T12)

`web/README.md`, root `README.md` / `SPEC.md` / `tasks/plan.md` pointer
files (only if they carry a phase-status line), `tasks/todo-phase04.md`
(mark Tier 6 + P4-G done), `.claude/roadmap/README.md` phase index if it
tracks status, ADR 0016 + `test-plans/README.md` (if carried-item 2
resolves that way), `frontend-design-tokens.md` +
`check-token-contrast.ts` (carried-item 1), `frontend-generated-covers.md`
+ `frontend-tooling.md` Open questions (carried items 3, 4). CLAUDE.md's
"Where the task list lives" — verify nothing goes stale.

## Full regression before review (T13)

Run from `web/` (Bash cwd resets to repo root each turn — `cd web` or
absolute paths every command). Kill stray vite on 5174–5176 first.

`npm run build` · `npm run lint` · `npm test` · `npm run build-storybook`
· `npm run format:check` · `npm run tokens:generate && git diff --exit-code`
· `npm run tokens:check-contrast` · `npm run mocks:gen-fixtures && git diff --exit-code`
· `npm run check:token-styling` · `npm run check:a11y-tabindex` ·
`npm run check:a11y-hidden-text` · `npm run check:bundle-size` ·
`npm run check:dist-secrets` · `npm run check:dist-msw` ·
`npm audit --audit-level=high` · `npx playwright test` (all three
projects: benchmark, app, gallery).

## Code review (T14)

`/code-review high` as a fork before opening the PR. Fix every real
finding, add a regression test per fix, commit each fix separately.

## Critical files

- `.claude/templates/audit.md` — the audit's shape
- `.claude/audits/0001-phase03-backend-specs.md`, `0003-cmd-server-startup-shutdown.md` — format + severity calibration
- `.claude/audits/README.md` — the four attackers, honest-rating rule, "no phase closes with an open Critical or High"
- `.claude/roadmap/04-frontend-foundation/README.md` — the exit-criteria checklist T10 walks
- `.claude/specs/README.md` + the six `frontend-*.md` specs — T11's status moves and per-spec acceptance criteria
- `.claude/decisions/0016-test-plan-cadence.md` — carried item 2
- `web/scripts/checks/mswExclusion.ts`, `web/scripts/checks/secretsGrep.ts`, `web/vite.config.ts`, `web/src/main.tsx` — F26's MSW / secrets verification targets
- `web/src/components/{GeneratedCover,ErrorState}/`, `web/src/app/RouteError.tsx`, `web/src/screens/ScreenPlaceholder.tsx` — F26's XSS surfaces
- `web/src/app/capability/*`, `web/src/data/*`, `web/src/mocks/fixtures/*` — F26's capability-gating and domain-boundary surfaces

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| `/security-review` on the branch is thin (no code diff vs `main`) | Low — could read as "audit was shallow" | Recorded explicitly in the audit's Scope/method; depth comes from the `security-auditor` subagent + T2–T6 manual verification, not the branch diff |
| A Critical/High finding surfaces late (e.g. T2's built-bundle inspection) | Medium — blocks Gate 1 until fixed | Each F26 task produces its own evidence and is committed separately; a fix is a task with its own regression test, not a scramble; Gate 1 does not open until Critical/High are closed |
| Spec statuses flipped to `VERIFIED` before every acceptance criterion is actually evidenced | Medium — the lifecycle table's whole point | T11 is gated on T10's completed walk; each criterion cites evidence before the flip, no "code exists → done" |
| Carried items silently deferred instead of decided | Medium — constitution §12 | The table above puts all five to the maintainer in this plan; unanswered ones are raised again at Gate 1; each gets a recorded decision in a named doc |
| Phase closes as if `atTablet` / the host-LAN boundary were settled | Low for phase 04, real for phase 05/13 | T12 writes an explicit "closes without this settled" line into the roadmap close note; both specs' Open questions already carry it |

## Open questions

- Carried items 1 and 2 need a maintainer decision (see the table). 3, 4,
  5 are recorded-deferral / not-ours-to-resolve and do not block.
- Whether the maintainer wants the `/security-review` pass re-run against
  a synthetic diff of the whole phase-04 surface rather than the (thin)
  branch diff — noted, not assumed.
