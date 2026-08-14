# Phase 99 — Release

*Outline — expanded to a full phase document when phase 17 closes.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 17 |
| **Blocks** | — |

## Objective

Packaging, versioning, and the release process itself — including standing
up the two repos that only make sense to create once there's a product to
document and show: `docs` and `website` (ADR 0006).

## Scope

**In**

- Electron installers (Linux, macOS, Windows)
- Docker/Compose self-hosting stack
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

## Exit criteria

- [ ] Installers built and tested on all three desktop platforms
- [ ] Docker self-hosting stack verified with persistent volumes
- [ ] OpenAPI spec published and versioned alongside the release it describes
- [ ] `docs` repo created, with self-hosting and admin documentation complete
- [ ] `website` repo created, with the landing page live
- [ ] Release tagged and published
- [ ] Maintainer approval recorded
