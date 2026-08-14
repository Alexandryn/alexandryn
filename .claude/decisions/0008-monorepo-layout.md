# 0008. Monorepo layout: Go module at root, two npm workspace packages, no build-orchestration tool

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-14 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 01's open-questions list has carried "monorepo layout and tooling"
since phase 00. Several specs already assumed an answer provisionally —
`architecture-contracts.md` FR-2 marked `api/openapi.yaml`'s repo-root
location "provisional, pending the still-open monorepo-layout decision";
`architecture-backend.md` FR-1 assumed "wherever the monorepo decision
ends up placing it" for the Go module. Three different toolchains need a
home: Go (server), TypeScript/React (web UI), and Electron's own main/
preload code (also TypeScript, but a different runtime target than the
browser-facing UI).

## Decision

- **Go module at the repository root** (`go.mod` at `/`), not nested under
  a subdirectory — idiomatic for a Go-primary backend, and
  `architecture-backend.md`'s `internal/`, `cmd/server`, `cmd/pg-supervisor`
  layout already assumes a module root without specifying where; this
  fixes it at `/`.
- **`web/`** — the React/TypeScript/Tailwind app
  (`architecture-frontend.md`), an npm package, builds to `web/dist/`.
- **`electron/`** — Electron main process and preload script
  (`architecture-desktop-host.md`), a separate npm package. It does not
  bundle a copy of the web UI: per `architecture-system.md` FR-6 and
  `architecture-frontend.md` FR-7, the Electron window loads the UI from
  the Go server's own loopback URL at runtime, the same as a LAN browser
  would — there is nothing for `electron/` to bundle.
- **`web/dist/` is embedded into the Go binary via `go:embed`**
  (`cmd/server`), not read from a filesystem path at runtime. One
  build-order dependency this creates: `web/` must build before `go
  build` runs. One shipped binary serves its own frontend — no separate
  asset-copying step at install or first-run time.
- **npm workspaces** (`package.json` at repo root, `"workspaces":
  ["web", "electron"]`) for the two JS packages — not Turborepo, Nx, or a
  similar build-orchestration tool. Two packages sharing a lockfile is
  what npm workspaces already does; a tool that exists to parallelize and
  cache builds across dozens of packages is solving a problem this project
  (phase 01's own risk table: "this serves one household") doesn't have.
- **`api/openapi.yaml` stays at the repository root** — confirms
  `architecture-contracts.md` FR-2's provisional guess was correct; no
  amendment needed there beyond removing the "provisional" caveat.

## Options considered

### Option A — Go at root, two npm workspace packages, no orchestration tool (chosen)

*For* — matches the actual shape of the problem (one backend toolchain,
two closely-related JS packages) without adopting tooling sized for a
much bigger monorepo. `go:embed` for the frontend build is a well-known,
idiomatic pattern — one shipped binary, no runtime asset-path guessing.

*Against* — the `web/` → `go:embed` build-order dependency means `go
build` alone doesn't produce a working server; CI and local dev both need
to know to build `web/` first. A real constraint, not free.

### Option B — Turborepo or Nx managing all three packages

*For* — real caching and task-graph benefits at a certain project size,
industry-standard tooling.

*Against* — sized for a problem this project doesn't have yet (phase 01's
own repeated "single household" non-goal). A build-orchestration tool is
itself a dependency requiring the constitution §9 justification this
project applies to everything else — "we might grow into needing it" is
exactly the reasoning phase 01's risk table already rejects for other
decisions.

### Option C — Separate repos per package (Go server, web UI, Electron shell)

*For* — clean toolchain isolation, independent versioning.

*Against* — contradicts this project's own existing practice (ADR 0001:
specs are versioned with the code they describe, reviewable in the same
PR) — a change spanning the API contract, the Go handler, and the React
component consuming it would need three PRs across three repos for one
coherent change. Rejected on the same grounds ADR 0006 used to keep
`.claude/` in this repo rather than splitting it out.

## Consequences

**Good** — one clone gets a contributor everything; `go:embed` means
`cmd/server` is genuinely one artifact to ship, matching the self-hosted,
single-binary-where-possible ethos this project has followed since ADR
0005; no tooling to justify beyond what Go and npm already provide.

**Bad** — the `web/`-before-`go build` ordering is a real CI/local-dev
detail every build script and the `architecture-testing.md` CI pipeline
needs to get right, or "just run `go build`" silently produces a server
with no frontend to serve. `electron/` and `web/` are two npm packages
that need their shared dependencies (TypeScript config, lint config) kept
consistent by hand, without a tool doing it for them.

**Neutral** — doesn't change any decided FR in the six specs written
against a provisional or unstated layout; this ADR is what makes "wherever
the monorepo decision ends up placing it" concrete, not a new design.

## Reversal cost

Low right now — nothing has been built. Medium once phase 03/04 write real
code against these paths — moving `internal/` or `web/` later is a
mechanical but real refactor, not a redesign.

## Confidence

High on Go-at-root and npm-workspaces-not-a-build-tool — both follow
directly from this project's own repeated stated non-goals. Medium on
`go:embed` specifically over a filesystem-path approach — reasonable and
idiomatic, but not weighed against alternatives as rigorously as ADR
0005's prototyped process model was.
