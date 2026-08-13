# Phase 00 — Foundation

| | |
|---|---|
| **Status** | In progress |
| **Depends on** | — |
| **Blocks** | Every other phase |
| **Opened** | 2026-08-12 |
| **Closed** | — |

## Objective

The repository exists, is governed, and knows how to talk about itself. A
contributor arriving with no context can find the rules, the plan, and the
document templates, and can tell what is decided from what is merely assumed.

No application code. This phase builds the container, not the contents.

## Why here

Everything else depends on it, and the cost of retrofitting governance is that
it never happens. Specifically: the licence must be settled before the first
outside contribution, and the specification workflow must exist before the
first specification, or the first one sets a precedent by accident.

## Scope

**In**

- GitHub repository under the `Alexandryn` organisation, private for now
- Repository hygiene: `.gitignore`, `.gitattributes`, `.editorconfig`, CODEOWNERS
- Governance: README, CONTRIBUTING, SECURITY, PR template
- The engineering constitution
- `.claude/` structure with document templates
- This roadmap
- `CLAUDE.md` — orientation for automated contributors
- The design prototype captured as a reference, with its gaps documented
- Branch protection recommendations, documented

**Out**

- Any application code, dependency manifest, or build tooling — phases 01–04
- Real CI pipelines. There is nothing to lint, typecheck, or test yet, and a
  workflow that runs no checks is a green tick that means nothing.
- The architecture itself — phase 01
- Skill authoring, which needs practices that don't exist yet

## Specifications

None. This phase is governance, not behaviour, so there is nothing to specify
in the constitutional sense. Phase 01 produces the first specs.

## Architecture decisions expected

| Decision | Status |
|---|---|
| [0001 — Record architecture decisions](../../decisions/0001-record-architecture-decisions.md) | Accepted |
| [0002 — Project licence](../../decisions/0002-project-licence.md) | **Proposed — blocking the repository going public** |

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| The design reference is truncated; ~six screens and all prototype logic are missing | **Occurred** | High — phases 04, 06, 11 need them | Documented in `.design-reference/ANALYSIS.md`; a complete export must be obtained before phase 04 |
| Licence deferred past the first outside contribution | Medium | High — becomes unfixable without everyone's consent | ADR 0002 raised now; repository stays private until resolved |
| The `.claude/` system is written and then ignored | Medium | High — the whole method collapses quietly | Gates are enforceable and referenced from `CLAUDE.md`; phase closure requires the artefacts to exist |
| Governance documents written aspirationally, then contradicted by the first real pressure | Medium | Medium | The constitution is short and specific enough to be checkable, and amendable by PR rather than by drift |

## Test strategy

Nothing executable to test. Verification is structural:

- Every document referenced from another document exists
- No secrets or personal paths committed
- The repository clones and reads correctly from a fresh checkout

CI arrives in phase 03/04 alongside the first code it can meaningfully check.

## Security considerations

Small surface, but two things matter:

- **Nothing secret enters history.** Git history is effectively permanent; a
  credential committed and later removed is still leaked. `.gitignore` denies
  the usual shapes up front, and the diff was reviewed for tokens and
  home-directory paths.
- **Repository access.** Private at creation. Branch protection is recommended
  below rather than applied, because it needs the owner's decision about
  whether a solo maintainer should be able to push to `main`.

## Observability

Not applicable. The first thing this project must be able to observe is a
running server, which is phase 03.

## Branch protection recommendations

To apply on `main` before the repository becomes public:

- Require a pull request before merging
- Require conversation resolution before merging
- Require status checks to pass once CI exists (phase 03)
- Require linear history — the roadmap's value depends on a readable log
- Block force pushes and deletions
- Include administrators, with an explicit exception for the solo-maintainer
  period if the owner prefers

Not applied automatically: rules that lock a single maintainer out of their own
repository are the kind of thing that should be chosen, not inherited.

## Exit criteria

- [x] Repository created under the `Alexandryn` organisation
- [x] Constitution written
- [x] Roadmap phases defined with dependencies and reasoning
- [x] Document templates in place
- [x] README, CONTRIBUTING, SECURITY, PR template, CODEOWNERS
- [x] Design reference captured, with its incompleteness documented
- [x] Repository hygiene files
- [ ] **Licence decided** (ADR 0002) — blocks public release, not phase closure
- [ ] Complete design export obtained — blocks phase 04, not phase closure
- [ ] Branch protection applied
- [ ] Maintainer approval recorded
