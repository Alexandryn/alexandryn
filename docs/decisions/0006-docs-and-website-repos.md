# 0006. Documentation and the landing page live in separate repos, created at release

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-13 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Nothing had fixed where user-facing documentation or a public landing page
would live. Phase 99's outline named "admin self-hosting guide" and "admin
documentation" as in-repo deliverables, with no repo ever named for a landing
page at all — both were implicitly going to accrete into the `alexandryn`
repo by default, the way undecided things do.

Separately, phase 01's own open-questions list still has "API style, and
where its schema is the single source of truth" unresolved. This ADR doesn't
fully close that — `architecture-contracts.md` still owns the real design —
but it does fix one piece of it early: the `alexandryn` repo carries an
OpenAPI specification as its contract artifact, not prose API documentation.

## Decision

Three repos, not one:

- **`alexandryn`** (this repo) — code, and the OpenAPI specification as the
  API contract's source of truth. No user-facing documentation lives here
  beyond what a developer needs to build and run the thing (README,
  CONTRIBUTING). `.claude/` stays — it was never "documentation" in the
  sense this ADR is about; see Consequences.
- **`docs`** — self-hosting guide, user documentation. Does not exist yet.
  Created in phase 99 (release), not before — there is nothing true to
  document until then, and a docs repo started early describes a product
  that doesn't exist, the same failure mode `docs/roadmap/README.md`
  already names for over-detailed distant phases ("confident fiction").
- **`website`** — the public landing page. Same timing: created in phase 99,
  as part of the release itself, not after it and not before it.

"Future," precisely: after the product is actually released to a public
repo — which is what phase 99 *is*. Not an indefinite someday.

## Options considered

### Option A — Three repos, `docs`/`website` created at phase 99 (chosen)

*For* — keeps the `alexandryn` repo's scope narrow and stable (code +
contract) for its entire pre-release life; avoids writing documentation and
marketing copy against a product that's still changing shape every phase;
matches the project's existing bias against premature artifacts (skills
stayed empty until Phase 01/02 settled things, for the same reason).

*Against* — self-hosting instructions don't exist as a separate,
citable thing until release; anyone trying to self-host from source before
then has only this repo's own README and specs, which are written for
contributors, not operators.

### Option B — One repo, `docs/` folder inside `alexandryn`

*For* — simplest, no repo-management overhead, docs and code reviewed
together.

*Against* — was the de facto default this ADR is replacing. Bloats the
product repo with content that has a different audience, release cadence,
and review bar than code; a docs site generator's own tooling/dependencies
would sit next to the Go/React/Electron toolchain for no structural reason.

### Option C — Documentation and website repos created now, alongside phase 00

*For* — repo-scaffolding is cheap to do once, while already doing it for
`alexandryn`.

*Against* — exactly the "confident fiction" failure mode: a landing page
for a product with no working build, and a self-hosting guide for
instructions that don't exist yet. Rejected on the same grounds phase 01's
own risk table already uses elsewhere in this project.

## Consequences

**Good** — `alexandryn`'s scope stays code-and-contract for its whole
pre-release life, which is checkable (does a PR add prose documentation
where it shouldn't?) rather than a matter of taste. Phase 99 gets two new,
explicit, named deliverables instead of an implicit assumption.

**Bad** — no separately-citable self-hosting guide exists before release;
early testers or contributors before phase 99 rely on this repo's own
(contributor-facing) documentation. This is accepted as correct for a
pre-alpha project explicitly saying so in its own README.

**Neutral** — this doesn't touch `.claude/`, which stays in `alexandryn`
regardless (Non-goals) — it was never "documentation" in the sense this ADR
is about; it's the engineering process itself, versioned with the code it
governs (ADR 0001).

## Reversal cost

Low before phase 99 (nothing has been created yet to unwind). Medium after —
moving content between repos loses in-place history unless done deliberately
(e.g. `git subtree`/filtered history import), and external links (search
results, the landing page's own inbound links) would need redirects.

## Confidence

High. This defers two decisions this project isn't ready to make well yet
(what the docs site looks like, what the landing page says) to the point
where there's a real product to describe, while fixing the one thing that
needed fixing now — that they're not silently going to accrete into this
repo by default.
