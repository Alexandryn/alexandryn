# Review: architecture-testing.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-testing.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-14 |
| **Verdict** | Approved with changes (both findings fixed — see Resolution below) |

## Summary

Resolves what five sibling specs deferred here: the Electron E2E tool
(`@playwright/test`, explicitly distinct from the interactive Playwright
MCP), the import-boundary lint's CI mechanics, the contract-test's CI
mechanics, and the real-vs-bundled-Postgres distinction for integration
tests. Ties vulnerability scanning to this project's existing severity
taxonomy instead of inventing a new scale. Two findings: FR-2 asserts
Electron E2E runs "headless" without acknowledging that Electron on a
Linux CI runner typically needs a virtual display (`xvfb`) to run at
all, headless or not — a real operational detail, not a style choice; and
phase 01's own description of this spec explicitly names "Layers" as
something it should fix, which got prose treatment instead of the concrete
table every other spec in this phase used.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Completeness | FR-2 says Electron E2E tests run "headless in GitHub Actions" without qualification. Electron is a GUI application; on Linux CI runners it typically needs a virtual display server (`xvfb-run` or equivalent) to launch at all — "headless" in the browser-testing sense doesn't automatically apply the same way it does to Chromium. Asserting this "just works" risks phase 03 discovering the gap mid-implementation instead of planning for it | Add the `xvfb` (or equivalent) requirement explicitly to FR-2, named as a known CI environment need, not left implicit |
| 2 | Minor | Completeness | Phase 01's own Specifications table describes this spec as covering "Layers, tooling, fixtures, determinism, CI shape" — "Layers" got folded into FR-1's prose list rather than the concrete Unit/Integration/Contract/E2E/Accessibility-style table every sibling spec's Test strategy section used. For the document phase 01 itself calls highest-leverage, the layer stack deserves the same explicit treatment | Add a summary table naming every testing layer across the whole system and which spec/phase owns each, not just prose references scattered through the FRs |

## Dimensions checked

- [x] **Completeness** — both findings are real
- [x] **Ambiguity** — FR-1 through FR-7 individually clear
- [x] **Architecture** — FR-2/FR-3's tool-distinction reasoning is sound and directly resolves prior specs' deferred questions rather than re-deferring them
- [ ] **Domain correctness** — not applicable
- [x] **Security** — FR-4/FR-5 make vulnerability scanning a blocking gate, reusing the existing severity taxonomy instead of a scanner-specific one — genuinely load-bearing, not decorative
- [x] **Testability** — acceptance criteria require *proof* (a deliberately broken lifecycle state, a deliberately vulnerable dependency), not just configuration existing
- [ ] **Accessibility** — not applicable beyond confirming the MCP's real capability, already done in `architecture-frontend.md`
- [x] **UX and copy** — not applicable, no user-facing copy in this spec
- [ ] **Observability** — correctly marked not applicable beyond CI's own pass/fail reporting
- [x] **Maintainability** — FR-2's MCP-vs-`@playwright/test` distinction is exactly the kind of thing that prevents a future contributor from confusing "the tool Claude Code uses" with "the tool CI uses"
- [x] **Evolution** — Non-goals correctly exclude the actual YAML (nothing to build against yet) while fixing the shape it must have

## Contradictions and gaps

Finding 1 is the substantive one — not wrong, but confidently
under-specified in a way that would read as "just works" to an
implementer who hasn't hit the Electron-on-CI display problem before.

## Resolution (2026-08-14)

- **#1** — FR-2 now names the `xvfb`/virtual-display requirement directly,
  with the exact setup step left to phase 03
- **#2** — new "Testing layers" section, one table for the whole system,
  naming tool and owning spec per layer

## What I did not review

Whether GitHub Actions' hosted Linux runners have `xvfb` available by
default or need an explicit setup step (e.g. `xvfb-action`) — I know the
general problem class exists (Electron/GUI apps need a display on headless
Linux CI) but haven't verified the exact current state of GitHub's hosted
runner images. Flagged as a real implementation detail for phase 03, not
asserted with false precision here.
