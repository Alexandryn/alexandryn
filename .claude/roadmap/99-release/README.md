# Phase 99 — Release

| | |
|---|---|
| **Status** | In progress |
| **Depends on** | Phase 18 |
| **Blocks** | — |
| **Opened** | 2026-09-15 |
| **Closed** | — |

## Dependency status at open (verified, not trusted from roadmap)

**Phase 18 (codebase hygiene):** Closed 2026-09-12 (merged via PR #326, CI
green on both runs). Its own roadmap README was left reading "Not started"
after the merge; corrected 2026-09-15 alongside phases 12 and 17 below
(PR #329).

**Phase 17 (accessibility and QA):** Closed 2026-09-11 (merged via PR #313).
0 open Critical/High findings from the original sweep. Two known,
maintainer-accepted gaps remain and are recorded rather than hidden: one
Medium finding (A-17-02, `#315`, deferred) and one incomplete item — the
manual keyboard/320px walkthrough of Reader and Import specifically, which
audit `0017`'s own verdict line records as still open. Neither blocks this
phase; both are candidates for this phase's own release-notes "Known
limitations" entry (Step 2).

**Phase 12 (authentication):** Closed. The two High authorization defects
from review `0050` / audit `0012` (horizontal IDOR across the reading API;
`AuthMiddleware` accepting a token minted for the wrong purpose) were fixed
and re-verified on `feat/phase13-network-access` (audit `0013`, merged via
PR #80, 2026-09-05). `X-Library-Id` validation against the JWT `libraries`
claim was verified in the same audit. The fuller independent re-audit of
the phase-12 RBAC/membership/refresh-rotation surface that the correction
required happened across three passes rather than one — audit `0013`
(route-registration RBAC), audit `0014` (sync/device membership checks),
and audit `0016`'s whole-application sweep (invitation/membership call
path, `#262`) — with 0 open Critical/High remaining as of phase 16's
close.

All three dependencies are satisfied. Nothing blocks packaging work
starting.

## Objective

Packaging, versioning, and the release process itself — including standing
up the two repos that only make sense to create once there's a product to
document and show: `docs` and `website` (ADR 0006). At the end of this
phase, `v1.0.0` exists as a tagged, installable, documented release with
evidence behind every exit criterion, not an assertion that it "should
work."

## Why here

Every phase before this one adds capability or hardens something already
built. This phase adds nothing to the product itself — it takes what
phases 00 through 18 built and makes it reachable by someone who isn't
running it from source with a debugger attached. It depends on phase 18
specifically (not phase 17) because packaging a codebase still carrying
internal process residue — phase numbers, audit citations, AI-session
narration — into a public release would ship that residue to every
self-hosting user and to the `docs`/`website` repos this phase creates.
Cleanup had to land first.

## Scope

**In**

- Electron installers (Linux, macOS, Windows) — including the packaged
  binary's **Electron fuses** (audit 0016 #260, re-scoped here). Set at
  package time via `@electron/fuses`; recommended posture: `RunAsNode`
  off, `EnableNodeCliInspectArguments` off,
  `EnableNodeOptionsEnvironmentVariable` off, `EnableCookieEncryption` on,
  `OnlyLoadAppFromAsar` on, `EnableEmbeddedAsarIntegrityValidation` on
  (macOS/Windows). Phase 16 verified there is no code-level blocker: the
  main process spawns the Go server as a separate executable (not
  `ELECTRON_RUN_AS_NODE`), relies on no `--inspect` / `NODE_OPTIONS`, and
  the renderer already runs with `contextIsolation`/`sandbox` on and
  `nodeIntegration` off. The blocker to clear is simply that no packaging
  pipeline exists yet.
- Docker/Compose self-hosting stack: release-time hardening and
  verification (persistent volumes, image tagging/versioning, registry
  publishing) of the baseline `Dockerfile`/`docker-compose.yml` phase 03's
  `deployment-container-packaging.md` (ADR 0015) already designs — this
  phase doesn't design the container target from scratch
- Semantic versioning, changelog, public release process
- The `alexandryn` repo's OpenAPI spec finalised and published as this
  repo's own API-contract artifact (`architecture-contracts.md`, phase 01) —
  this repo carries the contract, not prose documentation
- Standing up the **`docs`** repo (self-hosting guide, user documentation)
  and the **`website`** repo (public landing page) — new repos, created here,
  not before. What exactly goes in each is designed when this phase is
  reached, per the roadmap's own "detail decreases with distance" rule; ADR
  0006 fixes only that they're separate repos and that they don't exist
  before this phase

**Out**

- Post-v1 roadmap items.
- Any content of `docs` or `website` beyond what this phase needs to ship —
  designed here, not now.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| ~~Container registry has never been chosen~~ — resolved: ADR 0036, GitHub Container Registry, specifically because it needs no separate credential | — | — | Resolved 2026-09-16; `release.yml` wired to push there on tag push |
| Cross-platform installer testing needs macOS/Windows runners or hardware this environment may not have | High | High | Document the gap explicitly per Constitution §12 rather than asserting "should work"; GitHub Actions provides `macos-latest`/`windows-latest` runners for CI-produced artifacts even where local manual testing isn't possible |
| **Realized**: this repo's GitHub Actions is blocked — first by an artifact/cache storage quota (worked around: 560 old artifacts + 14 caches deleted, retention capped in `ci.yml` going forward), then by an org-level billing issue ("recent account payments have failed or your spending limit needs to be increased") that stops jobs before they even start | Confirmed | High | Requires the maintainer's own action on GitHub (org Settings → Billing & plans) — nothing in this repository can resolve it. All CI-equivalent checks are being run locally in the meantime (shell guards, Go/web/desktop test suites, Playwright, the container-target cycle) so this phase's own work keeps moving; macOS/Windows installer builds and the actual GHCR push remain genuinely blocked until Actions runs again |
| Local sandbox cannot install WebKit's system dependencies (`libicu74`, `libxml2`, `libflite1`) without root — confirmed during Step 0's pre-flight battery, `npx playwright install-deps` requires `sudo` and no password is available here | Confirmed | Low | Chromium/Firefox/Mobile Chrome fully verified locally (128/128 passed); WebKit/Mobile Safari verified via CI's own runners (which do have root) rather than this sandbox — CI's historical runs (PR #313, #326) confirm the full matrix passes there |
| Electron fuse configuration silently regresses (e.g., a future dependency bump reintroduces `nodeIntegration` or drops `contextIsolation`) | Low | Critical | Fuse posture is asserted in the packaging pipeline itself (Step 4), not just documented, and re-checked in the Step 9 security audit before tagging |
| Docker image leaks a build-time secret into a layer (registry credential, signing key) | Low | Critical | Audit `Dockerfile` build stages for anything copied before a multi-stage `COPY --from=builder`; Step 9's packaging-attacker pass checks this explicitly |
| `docs`/`website` content drifts from what the shipped build actually does, since both are written during the same phase that's still changing packaging details | Medium | Medium | Write both after packaging steps 4-5 stabilize, not before; review against the actual built artifacts, not the plan |
| Finding volume during the Step 9 audit overwhelms triage the way phases 16/17 saw | Medium | Medium | Same mitigation: one issue per finding, existing severity/label scheme, audit doc's findings table as the index |

## Test strategy

| Exit criterion | How verified | Platform / environment |
|---|---|---|
| Installers built and tested (Linux/macOS/Windows) | CI-produced artifacts per platform via the packaging workflow (Step 4); Linux AppImage/`.deb` can also be smoke-tested in this environment | Linux: this environment + CI. macOS/Windows: CI runners only — no local hardware here, gap stated plainly rather than asserted |
| Electron fuses correct at package time | `@electron/fuses` audit step reads back the packaged binary's fuse bits after packaging, not just the config that requested them | CI packaging job, all platforms |
| Docker stack verified with persistent volumes | `docker compose --profile bundled-db up --build --wait --wait-timeout 120`, then `down` (without `-v`) and `up` again, confirming data survives | This environment (Docker/Podman available) + CI |
| OpenAPI spec published and versioned | `api/openapi.yaml` diffed against the live server's actual routes; CI step added per Step 3.4 to fail on divergence, or the manual-check procedure recorded in an ADR if no tooling exists | CI + this environment |
| `docs` repo self-hosting/admin guides complete | Manual review against the actual Docker/installer steps this phase produces — walked through as if a self-hosting user with no codebase knowledge | This environment |
| `website` landing page live, WCAG AA | `@axe-core/playwright` scan of the built page, same tooling phase 17 used | This environment |
| Release tagged and published | Tag pushed, CI release workflow produces artifacts, GitHub release created linking the changelog | CI |
| Full quality battery (Step 8) green on the exact commit tagged | Every suite re-run on that commit, no suite skipped silently | This environment where possible (confirmed during Step 0: all suites pass except WebKit E2E, an environment gap not a code defect) + CI for the full cross-platform matrix |

## Security considerations

- **Electron fuse posture** is a release blocker, not a hardening nice-to-
  have (Step 4, Constitution's own release-phase safeguards). The Step 9
  packaging-attacker pass must read the fuse bits back from the actual
  packaged binary, not trust the build config that requested them.
- **Docker image surface**: no secrets in image layers or environment
  defaults, non-root runtime user, healthcheck defined and working. The
  Step 9 packaging-attacker pass inspects the built image's layers
  directly (`docker history`, `docker save` + inspect), not just the
  Dockerfile source.
- **Network exposure invariants** (ADR 0017, Constitution §6) apply to
  every released artifact identically — desktop installer, container
  image, source tarball. Loopback by default; broader exposure requires
  authentication plus one of ADR 0017's two fail-closed TLS conditions.
  Nothing about packaging is allowed to change this.
- **Token type assertion** (HKDF signing subkeys per purpose, audit
  0012-C2) must survive the release build unchanged — the Step 9
  authentication-attacker pass re-verifies this against the packaged
  binary, not the source tree.
- **Container registry choice** is not yet an ADR (see Risks). Whatever
  registry is chosen, publishing credentials must never appear in the
  packaging workflow's logs or in the image itself — CI-injected secret,
  never a committed value.
- **Secrets in CI/CD**: the packaging pipeline introduces new secret
  material it didn't need before (installer code-signing keys, registry
  push credentials, possibly ACME account keys for the self-hosting
  guide's TLS walkthrough). Each must be scoped to the minimum job that
  needs it and never echoed into logs.

## Observability

No new instrumentation is expected. This phase verifies that the existing
structured logs, request IDs, and health checks (Constitution §8, built
from phase 03 onward) survive the packaging pipeline unchanged — a
packaged binary that silently drops its own logging is a regression this
phase must catch, not one it introduces. If a packaging step reveals a
genuine observability gap, that's filed as a finding, not built ad hoc
inside this phase.

## Exit criteria

- [ ] Installers built and tested on all three desktop platforms — Linux
      (AppImage + `.deb`) built and fuse-verified locally this session;
      macOS/Windows depend on CI, currently blocked (see Risks)
- [x] Docker self-hosting stack (phase 03's baseline, hardened here)
      verified with persistent volumes — `postgres-data` and (added this
      session, audit `0018` A-18-01) `app-data` (the credential-
      encryption key and ACME cache) both verified to survive a full
      container removal and recreation, not just a stop/start
- [ ] OpenAPI spec published and versioned alongside the release it describes
      — spec itself finalized and versioned (`1.0.0`) this session; actual
      release-asset publication is a Step 10 (tag-gated) action
- [x] `docs` repo created (public, `Alexandryn/docs`), with a Starlight documentation site
      covering self-hosting, administration, security, updating, and the API
      reference, merged to `main` 2026-09-21; not yet deployed (Actions billing)
- [ ] `website` repo created (private, `Alexandryn/website`), landing page built and
      merged to `main` 2026-09-21; **not live**: Pages has not been enabled or run
- [ ] Release tagged and published
- [x] Maintainer approval recorded: audit `0018` approved by Luann Moreira,
      2026-09-21, and the `v1.0.0` tag authorised in the same message
