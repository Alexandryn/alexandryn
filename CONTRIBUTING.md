# Contributing to Alexandryn

Thanks for looking. Before anything else, read
[the constitution](.claude/constitution.md). It's short, and it explains why
this project asks for things other projects don't.

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

| Purpose | Command |
|---|---|
| Build backend | `go build ./cmd/server` (and `./cmd/pg-supervisor` on macOS) |
| Build frontend | `npm run -w web build` (produces `web/dist`, must run before backend build for embedded assets) |
| Build desktop host | `npm run -w @alexandryn/desktop build` |
| Build container image | `docker build .` |
| Run backend unit tests | `go test ./...` |
| Run backend integration tests | `go test -tags=integration ./...` (requires `TEST_DATABASE_URL`) |
| Run with race detector | `go test -race ./...` |
| Run contract tests | `go test -race -v ./internal/testutil/contracttest/...` |
| Bring up container target | `docker compose --profile bundled-db up --wait` |
| Run frontend tests | `npm run -w web test` |
| Run frontend lint | `npm run -w web lint` |
| Run desktop tests | `npm run -w @alexandryn/desktop test` |
| Run end-to-end tests | `npx playwright test` |
| Dependency vulnerability scan | `govulncheck ./...` |
| Security scanner | `gosec -quiet -severity high -confidence high ./...` |
| Run import boundary check | `bash scripts/check-import-boundaries.sh .` |
| Run parameterized query check | `bash scripts/check-parameterized-queries.sh .` |
| Run user-scoped reading check | `bash scripts/check-user-scoped-reading.sh .` |

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
maintainer's approval — before each spec is drafted (its scope and key
decisions, not its finished text), and after the audit.

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
