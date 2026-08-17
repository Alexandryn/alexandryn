# Contributing to Alexandryn

Thanks for looking. Before anything else, read
[the constitution](.claude/constitution.md). It's short, and it explains why
this project asks for things other projects don't.

> **Right now:** Alexandryn is pre-alpha and the architecture is still being
> specified. Code contributions are premature — there's nothing to build on
> yet. Feedback on the roadmap and the specs is genuinely useful, and that's
> where the leverage is until the foundation lands.

## Local development

Storage is PostgreSQL, run locally via the Supabase CLI ([ADR
0004](.claude/decisions/0004-persistence-engine-postgresql.md)):

```
npm install -g supabase   # once
supabase start            # spins up local Postgres + dev tooling
cp .env.example .env      # DATABASE_URL matches the URL supabase start prints
```

`supabase stop` when done. Nothing here is a cloud dependency — no Supabase
account needed, no data leaves the machine.

## Commands

**No build exists yet.** There is no `go.mod`, no `package.json`, and no
CI workflow file in this repository — every command below is what the
approved specs require to exist, not something you can run today. Each is
traced to the functional requirement that mandates it, so this list stays
checkable against the spec set rather than becoming its own, separately
maintainable claim.

| Purpose | Command | Required by |
|---|---|---|
| Build backend | `go build ./cmd/server` (and `./cmd/pg-supervisor` on macOS) | `backend-test-harness.md` FR-8, `architecture-testing.md` FR-8 |
| Build frontend | `npm run build` (inside `web/`, produces `web/dist`) | `frontend-tooling.md` FR-1/FR-6, must run **before** the backend build (ADR 0008's `go:embed` ordering) |
| Build container image | `docker build .` | `deployment-container-packaging.md` FR-1 |
| Run unit tests | `go test ./...` (no tag; MUST NOT require PostgreSQL, Docker, or any external service) | `backend-test-harness.md` FR-1 |
| Run integration tests | `go test -tags=integration ./...`, with `TEST_DATABASE_URL` set | `backend-test-harness.md` FR-2 |
| Run the bundled-spawn suite | `go test -tags=spawn ./...` (`spawn` is the spec's own named example — *"`//go:build spawn`, or an equivalent distinct tag"* — not fixed as the literal, final tag name) | `backend-test-harness.md` FR-7 |
| Run with the race detector | `go test -race ./...` (unit and integration) | `backend-test-harness.md` FR-9 |
| Bring up the container target | `docker compose --profile bundled-db up --wait` | `deployment-container-packaging.md` FR-4, `backend-test-harness.md` FR-10 |
| Run frontend unit/component tests | Vitest (exact `npm` script alias not fixed by any FR — the tool is `frontend-tooling.md` FR-7's, the invocation is an implementation choice) | `frontend-tooling.md` FR-7 |
| Run frontend lint | ESLint, with `typescript-eslint` and `eslint-plugin-jsx-a11y` (same caveat — tool fixed, script alias not) | `frontend-tooling.md` FR-3 |
| Dependency vulnerability scan (Go) | `govulncheck ./...` | `architecture-testing.md` FR-5, `backend-test-harness.md` FR-8 stage 8 |
| Run backend dev server | `go run ./cmd/server [--config <path>]` | `backend-configuration.md` FR-5 |
| Run frontend dev server | `npm run dev` (inside `web/`, Vite) | `frontend-tooling.md` FR-1; referenced by `desktop-host-process-model.md` |

**Where a tool is genuinely unchosen, this list says so instead of
guessing:**

- **Backend general lint** — `golangci-lint` is referenced by name across
  several specs (`backend-test-harness.md` Non-goals, `backend-configuration.md`
  review `0024`) as the presumed tool, but no ADR or spec FR formally
  selects it, and its rule configuration is explicitly deferred
  (`architecture-testing.md` Non-goals: *"Specific lint rule
  configuration... phase 03/04's to tune"*). No command given.
- **Backend import-boundary lint** — the mechanism that enforces
  `architecture-backend.md` FR-2/FR-3 (domain must not import
  persistence/transport) is unchosen among three named candidates: *"a
  `go/analysis` pass, a `golangci-lint` custom rule, or an interim
  grep-based CI script"* (`architecture-backend.md` Open questions,
  `backend-configuration.md` review `0024`). No command given.

## The short version

1. Behaviour changes start with a specification, not a branch.
2. Tests come before implementation, and they must fail first.
3. External input is hostile until validated.
4. Keyboard and screen-reader support is part of finishing, not a follow-up.
5. Say what you're unsure about.

## How work is organised

Everything lives in `.claude/`, and it's meant to be read:

| Directory | What's in it |
|---|---|
| `roadmap/` | Phases in dependency order, each with exit criteria |
| `specs/` | Feature specifications, each with a status |
| `decisions/` | Architecture decision records, including open questions |
| `test-plans/` | What gets tested, and at which layer |
| `reviews/` | Spec and code review records |
| `audits/` | Adversarial reviews and their findings |
| `templates/` | Start here when creating any of the above |

A phase moves through: discover, specify, review, plan tests, implement
test-first, QA, security audit, document, close. Two points stop for a
maintainer's approval — after the spec review, and after the audit.

## Making a change

**Small and obvious** — typo, broken link, dependency bump, CI fix. Open a PR.
No spec needed.

**Anything that changes behaviour** — open an issue first and describe the
problem, not your solution. If it's substantial, it becomes a spec in
`.claude/specs/` using [the template](.claude/templates/spec.md), and the spec
gets reviewed before code is written. This feels slow once and saves rework
repeatedly.

### Branches and commits

Branch from `main`: `feat/…`, `fix/…`, `spec/…`, `docs/…`, `chore/…`.

Commits follow [Conventional Commits](https://www.conventionalcommits.org):

```
feat(sources): reject OPDS redirects to private address ranges

An OPDS feed could point at 169.254.169.254 and turn the host into an
SSRF proxy for the LAN. Resolve and check the target before connecting,
and re-check on every redirect hop.

Refs: .claude/specs/sources-ssrf.md
```

The body explains *why*. The diff already shows what.

Keep commits atomic. One commit that adds a feature and reformats forty files
is two commits wearing a coat.

### Before you open a PR

- The new behaviour has a test that fails without your change
- Error, empty, and loading states are handled
- Anything interactive works from the keyboard and has an accessible name
- Untrusted input is size-limited, shape-checked, and timed out
- No secrets, tokens, or `/home/yourname/` paths in the diff
- Docs updated if setup or behaviour changed

The PR template asks you to name the tests that cover the change. Please
actually name them.

## Review

Reviews are direct about the code and generous about the person. Expect
questions about failure modes, hostile input, and what happens on the second
device. If a reviewer asks "what does a malicious source do here", that's the
process working, not suspicion of you.

Disagreement is fine — argue it. Decisions that stick get written down as an
ADR so nobody has to relitigate them from memory.

## Reporting a bug

Include what you expected, what happened, and how to reproduce it. If it's a
security issue, don't file it publicly — see [SECURITY.md](SECURITY.md).

## Code of conduct

Be decent. Assume good faith, especially across a language barrier. Harassment
of any kind means you're out.

Concerns go to the maintainers privately through
[a security advisory](https://github.com/Alexandryn/alexandryn/security/advisories/new),
which is the only private channel the repository currently has.
