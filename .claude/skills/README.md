# Skills

Reusable practice guides — the things a contributor (human or automated) should
know how to do here, written once instead of re-explained per review.

## Three kinds

**`general/`** — engineering practice that would apply to any serious project:
how a code review is conducted here, how an API is designed, how a security
review is structured, how to write a test plan.

**`purpose/`** — Alexandryn-specific knowledge that a generalist wouldn't have:
the domain model, the source provider contract, Open Library's quirks, EPUB
handling, the Electron IPC conventions, the PostgreSQL job-queue conventions.

Do not duplicate a general skill inside a purpose skill. A purpose skill
assumes the general one and adds only what is specific.

**Top level (`skills/<name>/`, no subdirectory)** — repo-workflow automation
that isn't engineering practice or domain knowledge, it's how this specific
repo's Git/GitHub mechanics work: `commit`, `make-pr`, `review`. These
aren't speculative — they exist because the workflow already exists (every
contributor commits; not every contributor knows this repo's specific
conventions), so the "signal to write one" bar below doesn't apply the same
way to this kind.

## When to write one

After the practice has stabilised, not before. A skill written speculatively
describes an imagined workflow and then quietly diverges from the real one.

The signal to write one: the same guidance has been given in review twice.

## Shape

Short, actionable, and opinionated. A skill is a checklist with reasoning, not
an essay. If it can't be applied while working, it belongs in `docs/` instead.

## Tooling already available, not authored here

The table below is skills *this project will write*. Separately, Claude Code
sessions working on this repo already have tools that aren't ours to author —
built-in skills, MCP servers, CLIs. Recording what exists and what it's for,
so a phase doesn't "discover" a tool mid-implementation instead of deciding
on it during planning, per the spec-before-build rule:

| Tool | Kind | Use | First relevant phase |
|---|---|---|---|
| `code-review` skill | Built-in | PR/diff review | Any, once there's code. Overlaps the planned "Code review" purpose skill below — undecided whether that gets authored from scratch or as a thin wrapper adding this project's `templates/review.md` dimensions on top |
| `security-review` skill | Built-in | Audit-gate reviews | Same — overlaps the planned "Security review" row, same open question |
| `frontend-design` skill | Built-in | Aesthetic direction, avoiding templated-default UI | Phase 04 — translating `.design-reference/` into production |
| `dataviz` skill | Built-in | Chart/dashboard design consistency | Phase 15 — Activity, metrics |
| `artifact-design` / `artifact-diagramming` skills | Built-in | Diagrams, design docs | Phase 01 — the three trust-boundary diagrams the phase requires |
| `postgres` MCP server | Project (`.mcp.json`) | Query/inspect the local dev database | Live now. ADR 0004 |
| `playwright` MCP server | Plugin | Drive a real browser | Phase 04 (frontend testing), Phase 17 (accessibility conformance) |
| `gh` CLI | System | GitHub repo/PR/issue operations | Already in use (repo creation) |
| `github` MCP server | Plugin | Same, via MCP | Currently broken (bad auth header) — `gh` CLI covers this, not blocking |

## Status

**`general/` and `purpose/` are no longer empty.** Phase 01/02 settled the
architecture and domain; phase 03's specs and ADRs settled the backend
conventions these five describe:

| Skill | Kind | Covers |
|---|---|---|
| [`purpose/domain-model-and-boundaries`](purpose/domain-model-and-boundaries/SKILL.md) | purpose | The three boundaries (constitution §3), the eleven aggregates, who owns each |
| [`purpose/go-backend-conventions`](purpose/go-backend-conventions/SKILL.md) | purpose | Package layout, dependency direction, no globals, error taxonomy, redaction |
| [`purpose/postgres-access-and-migrations`](purpose/postgres-access-and-migrations/SKILL.md) | purpose | `pgx` native, parameterized queries, transaction boundaries, `goose` migrations |
| [`general/code-review`](general/code-review/SKILL.md) | general | This project's severity vocabulary and dimensions, layered on the built-in `code-review` skill |
| [`general/security-review`](general/security-review/SKILL.md) | general | The four-attacker model and `audits/` recording format, layered on the built-in `security-review` skill |

The overlap question these four carry (thin wrapper vs. from-scratch) is
resolved: thin wrapper, adding only this project's own rules on top of
the built-in mechanics — see each skill's own "not a duplicate" note.

**Top-level workflow skills exist already**, since they don't have that
problem — they describe the repo's Git mechanics, not its architecture:

| Skill | Covers |
|---|---|
| [`commit`](commit/SKILL.md) | Conventional Commits, split-by-meaning, `Refs:` trailers, no `Co-Authored-By` ever |
| [`make-pr`](make-pr/SKILL.md) | PR title convention (react-spectrum), body from `.github/pull_request_template.md`, checks whether a PR is even the right move yet — designed with the maintainer, not authored solo |
| [`review`](review/SKILL.md) | Batched, structured review — draft every finding first, show the complete result before fixing anything. Required before opening a PR, and before moving spec to spec while we aren't making them |

Still-planned `general/`/`purpose/` skills, roughly in the order they'll
become real (the five above are done, removed from this list):

| Skill | Kind | Writable after |
|---|---|---|
| Frontend component conventions | purpose | Phase 04 |
| Accessibility review | general | Phase 04 |
| Electron IPC conventions | purpose | Phase 05 |
| Open Library integration | purpose | Phase 07 |
| Source provider contract | purpose | Phase 08 |
| PostgreSQL job-queue conventions | purpose | Phase 09 |
