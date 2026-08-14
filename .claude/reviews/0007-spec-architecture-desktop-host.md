# Review: architecture-desktop-host.md

| | |
|---|---|
| **Subject** | `.claude/specs/architecture-desktop-host.md` |
| **Reviewer** | Claude (self-review — same author; needs an independent read before this counts as real review) |
| **Date** | 2026-08-13 |
| **Verdict** | Needs rework |

## Summary

Closes the four items `architecture-system.md` explicitly left open
(macOS/Windows orphan prevention, second-instance handling, config/secrets
channel, control-plane mechanism) with concrete, standard-pattern decisions
— `app.requestSingleInstanceLock()`, a permissioned single-use config file,
named platform-specific orphan mechanisms. Two real gaps found on
self-review: the config file's origin (FR-5) is unaddressed for a real end
user, not a developer with `.env`, and the Playwright E2E decision may
belong to `architecture-testing.md` instead of here.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Completeness | FR-5 says the main process "writes a single-use file... before spawning" — but never says *where the value comes from* for a packaged end-user install. Checked the design reference directly (per this review's own "what I did not review" prompt): `atFirstRun`'s captured steps are "Your library / Your sources" → "Create library" → "Host setup" → "Network access", and there's a "Storage location" field plus a Settings "Storage" tab — but every one of those is about *file* storage (where book files live on disk), not a database connection. **No screen anywhere in the design reference configures PostgreSQL.** That means either the design assumes Alexandryn bundles/manages its own Postgres instance with zero user-visible configuration (plausible, and arguably the only sane consumer-product answer — nobody self-hosting a personal library wants to hand-configure a connection string), or this is a real gap in the design too, not just in this spec. Either way, ADR 0004 decided *that* production uses self-hosted PostgreSQL but never decided *how it gets onto an end user's machine* — dev uses the Supabase CLI stack, which isn't a production answer | This spec's FR-5 should either state the assumption explicitly (Alexandryn bundles and manages its own Postgres process, analogous to how it manages the Go server — making FR-5's "config file" internal, never user-facing) or flag this as a new open question feeding back into ADR 0004 / a future ADR on production Postgres provisioning. Not silently assumed |
| 2 | Major | Scope | Test strategy decides Playwright as the E2E tool for this layer, but phase 01's own spec table assigns "tooling, fixtures, determinism, CI shape" to `architecture-testing.md`. Deciding it here risks a contradiction if that spec picks differently, or duplicated authority | Either move the general tool decision to `architecture-testing.md` and only keep *this layer's application* of it here, or explicitly note here that this pre-empts that spec and why (this layer's testability being unusually tool-specific — Electron needs an Electron-aware driver, not a generic choice) |
| 3 | Minor | Verification | The Playwright MCP's actual ability to drive an Electron app (not just a browser) is asserted from general Playwright library knowledge, not verified against what the *MCP server* specifically exposes. `claude mcp list` shows it connected; whether its tool surface includes an Electron launcher wasn't checked | Verify before phase 05 relies on it — a quick check of the MCP's tool list, or a small prototype the same way ADR 0005 prototyped the process model |
| 4 | Minor | Security | FR-5's config file is described by permissions (`0600`) but not by creation method. A predictable path in a per-run temp directory is a TOCTOU/symlink-attack surface if not created atomically and unpredictably (e.g. `os.CreateTemp` / `mkstemp`-equivalent) | Add "created with a non-predictable name via the OS's atomic temp-file API" to FR-5's wording |
| 5 | Minor | Completeness | Tray icon, dock behavior, and close-vs-quit semantics (does closing the window quit the app or minimize to tray?) are entirely unaddressed, but they directly affect when FR-9's Windows Job Object / FR-8's macOS monitor actually fire — "the app closed" is ambiguous without this | Add an explicit decision or non-goal with owner (could reasonably be v1: close = quit, no tray, simplest option — but say so) |

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
