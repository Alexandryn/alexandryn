# 0036. Container images publish to GitHub Container Registry (GHCR)

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-15 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 99's release process publishes a tagged Docker image
(`alexandryn:v1.0.0`, `alexandryn:latest`) alongside the desktop
installers. Nothing had picked a registry, and the phase 99 spec (Step
5.3) requires this choice be recorded as an ADR before publishing begins.
The maintainer does not hold credentials for any container registry
today.

## Decision

Publish to GitHub Container Registry, `ghcr.io/alexandryn/alexandryn`.

## Options considered

### Option A — GitHub Container Registry (chosen)

*For* — requires no separate account or credential: GitHub Actions
authenticates to GHCR with its own built-in `GITHUB_TOKEN`, scoped by the
workflow's own `permissions: packages: write`, already the case for the
CI identity this repo uses everywhere else. Lives next to the source and
the release tag it corresponds to; a viewer on the repo can find the
image without being told where to look. Free for a public repository's
public packages, no separate billing relationship to set up.

*Against* — GHCR's own pull-rate and discovery UX is less mature than
Docker Hub's for anonymous, unauthenticated pulls at scale; not a concern
at this project's expected volume.

### Option B — Docker Hub

*For* — the default most self-hosters already have a client configured
for; broadest name recognition.

*Against* — requires the maintainer to create a separate account and
generate a personal access token, store it as a repository secret, and
maintain it going forward — exactly the credential-acquisition step this
ADR exists to avoid. Free-tier pull-rate limiting has also tightened
over time in ways GHCR's public-package path doesn't share.

### Option C — Self-hosted registry

*For* — full control, no third party.

*Against* — is itself another service to run, secure, and keep
available — the opposite of what a release process should be adding
scope to. Rejected without prototyping; not proportionate to this
project's actual need.

## Consequences

**Good** — publishing wires directly into the existing CI identity with
no new secret to create or rotate; the image and the code that produced
it live under the same namespace.

**Bad** — ties the canonical image location to GitHub specifically; a
future move off GitHub would need to either keep GHCR as a legacy
mirror or re-publish elsewhere and update every place the image is
referenced (the self-hosting guide, `docker-compose.yml` if it ever
names a published image instead of building locally).

**Neutral** — GHCR package visibility defaults to private on first push
and must be set to public explicitly (a one-time repository setting, not
a workflow concern) for anonymous `docker pull` to work.

## Reversal cost

Low. The registry name is a single string in the CI workflow and in the
self-hosting guide once written; no data migrates, since the image is
rebuilt from source on every release rather than carried forward.

## Confidence

High for the "no credential to obtain" reasoning — that was the maintainer's
own stated blocker, and GHCR is the only option of the three that doesn't
require one. Lower confidence on long-term registry choice being final:
this is exactly the kind of low-cost-to-reverse decision that doesn't
need more deliberation than it got.
