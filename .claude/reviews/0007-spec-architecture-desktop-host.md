# Review: architecture-desktop-host.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-desktop-host.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-13 |
| **Verdict** | Approved with changes (findings 2–7 fixed; finding 1 remains open by design, not a defect) |

## Summary

Closes the four items `architecture-system.md` explicitly left open
(macOS/Windows orphan prevention, second-instance handling, config/secrets
channel, control-plane mechanism) with concrete, standard-pattern decisions
— `app.requestSingleInstanceLock()`, a permissioned single-use config file,
named platform-specific orphan mechanisms. Across three review passes this
session: findings 2–7 are now fixed, including a genuine internal
contradiction (finding 6 — FR-6 required a UI state the system architecture
said couldn't exist yet, resolved with a bundled-asset requirement) and a
wrong claim caught by actually checking the tool instead of assuming
(finding 3 — the Playwright MCP can't drive Electron, only a browser page).
Finding 1 — where a packaged end user's `DATABASE_URL` actually comes from,
since the design reference has no screen for it — remains genuinely open,
not resolved here on purpose: it's a real product decision (bundle/manage
Postgres invisibly, most likely) that shouldn't be smuggled into a spec
revision without the maintainer's sign-off.

## Findings

Findings 6 and 7 added after a requested overlap-verification pass across
every FR against phase 01's other five specs, phase 05, and phase 03 —
2026-08-13, same session.

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Completeness | FR-5 says the main process "writes a single-use file... before spawning" — but never says *where the value comes from* for a packaged end-user install. Checked the design reference directly (per this review's own "what I did not review" prompt): `atFirstRun`'s captured steps are "Your library / Your sources" → "Create library" → "Host setup" → "Network access", and there's a "Storage location" field plus a Settings "Storage" tab — but every one of those is about *file* storage (where book files live on disk), not a database connection. **No screen anywhere in the design reference configures PostgreSQL.** That means either the design assumes Alexandryn bundles/manages its own Postgres instance with zero user-visible configuration (plausible, and arguably the only sane consumer-product answer — nobody self-hosting a personal library wants to hand-configure a connection string), or this is a real gap in the design too, not just in this spec. Either way, ADR 0004 decided *that* production uses self-hosted PostgreSQL but never decided *how it gets onto an end user's machine* — dev uses the Supabase CLI stack, which isn't a production answer | This spec's FR-5 should either state the assumption explicitly (Alexandryn bundles and manages its own Postgres process, analogous to how it manages the Go server — making FR-5's "config file" internal, never user-facing) or flag this as a new open question feeding back into ADR 0004 / a future ADR on production Postgres provisioning. Not silently assumed |
| 2 | Major | Scope | Test strategy decides Playwright as the E2E tool for this layer, but phase 01's own spec table assigns "tooling, fixtures, determinism, CI shape" to `architecture-testing.md`. Deciding it here risks a contradiction if that spec picks differently, or duplicated authority | **Fixed** — moot once finding 3 was checked: the MCP can't drive Electron at all, so there was nothing to pre-empt. Test strategy now explicitly hands the CI-side Electron E2E tool choice to `architecture-testing.md` |
| 3 | Minor | Verification | The Playwright MCP's actual ability to drive an Electron app (not just a browser) is asserted from general Playwright library knowledge, not verified against what the *MCP server* specifically exposes. `claude mcp list` shows it connected; whether its tool surface includes an Electron launcher wasn't checked | **Fixed** — checked directly (`ToolSearch` against the MCP's tool list): browser-page automation only (`browser_navigate`, `browser_click`, etc.), no Electron launcher. The spec's original claim was wrong, not just unverified. Corrected in Test strategy |
| 4 | Minor | Security | FR-5's config file is described by permissions (`0600`) but not by creation method. A predictable path in a per-run temp directory is a TOCTOU/symlink-attack surface if not created atomically and unpredictably (e.g. `os.CreateTemp` / `mkstemp`-equivalent) | **Fixed** — FR-5 now requires the OS's atomic temp-file API explicitly |
| 5 | Minor | Completeness | Tray icon, dock behavior, and close-vs-quit semantics (does closing the window quit the app or minimize to tray?) are entirely unaddressed, but they directly affect when FR-9's Windows Job Object / FR-8's macOS monitor actually fire — "the app closed" is ambiguous without this | **Fixed** — new FR-11: v1 is close-means-quit, no tray, explicit scope decision with reasoning, not a silent gap |
| 6 | Blocking | Internal contradiction | FR-6 requires showing an `atStates`-based loading state while the Go server isn't ready — but `architecture-system.md` FR-6 says the web UI (where `atStates` lives) is *served by the Go server*. If the Go server isn't up, there's nothing to load the loading-state page from. Not a scope question, an actual contradiction between what this spec requires and what the system architecture says is possible | **Fixed** — FR-6/FR-7 now require a small Electron-bundled asset, loaded from disk, never fetched from the Go server. Resolves the contradiction. One narrower open question remains (which spec owns building that asset) — real, but no longer Blocking |
| 7 | Major | Scope | Phase 05's own roadmap document (`05-desktop-host/README.md`) independently lists "What 'healthy' means for the Go server before the window is shown — polling the health endpoint... timeout/failure UX" as *its own* open architecture decision — which is exactly what this spec's FR-6/FR-7 already answers. Two documents were quietly claiming the same open question | Fixed in this pass: `05-desktop-host/README.md` updated to point at FR-6/FR-7 as the decided pattern, with phase 05's own specs still owning the real implementation and a measured timeout. This spec also gained a "Relationship to phase 05's own specs" section explaining the general layering (pattern here, implementation there) so this doesn't recur silently for the rest of its FRs |

## Dimensions checked

- [x] **Completeness** — findings 1 and 5 are real unstated requirements
- [x] **Ambiguity** — FR-1 through FR-10 individually unambiguous
- [x] **Architecture** — consistent with `architecture-system.md`'s FR-2,
      FR-9, FR-10; doesn't reopen the process-model decision
- [ ] **Domain correctness** — not applicable
- [x] **Security** — FR-2, FR-3, FR-10 correctly restate constitution §5 as
      checkable config; finding 4 is the one gap
- [x] **Testability** — FR-1 through FR-10 are each testable; finding 3
      questions whether the *named tool* for testing them is actually
      available as asserted
- [x] **Accessibility** — Non-functional section requires the same
      keyboard/focus/announcement treatment for loading/error states as any
      other UI, not deferred as "just a splash screen"
- [x] **UX and copy** — cites `atStates` rather than inventing copy, which
      is the right instinct given the design reference was just re-synced
- [ ] **Observability** — correlation ID on error state is named but not
      designed in detail — acceptable, that's phase 03's
      `backend-errors-and-logging.md` territory
- [x] **Maintainability** — Non-goals section names an owner for everything
      excluded except the bootstrapping gap (finding 1), which isn't even
      acknowledged as excluded
- [x] **Evolution** — FR-8/FR-9's "unverified" framing matches ADR 0005's
      honesty about what a prototype actually proved

## Contradictions and gaps

Finding 1 is the substantive one. Finding 2 is a process/authority question
more than a technical contradiction — nothing in `architecture-testing.md`
exists yet to actually contradict, but the risk is real once it's written.

No contradiction found against `architecture-system.md`, ADR 0004, ADR 0005,
or the constitution.

## What I did not review

Checked, after writing the first draft of this review: `atFirstRun`'s full
markup, and grepped both Electron canvas files for
`postgres|database|db.?url|connection string` — zero matches anywhere in
either file. Confirms finding 1 rather than resolving it: nothing in the
captured design, First-run or Settings, configures a database connection.
Alexandryn either bundles/manages Postgres invisibly to the user, or nobody
has designed this screen yet — this review can't tell which, only that it's
real and not addressed by this spec as written.
