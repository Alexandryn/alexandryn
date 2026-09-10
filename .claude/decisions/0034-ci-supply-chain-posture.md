# 0034. CI workflow supply-chain posture: SHA-pinned actions, least-privilege token, in-CI SAST, cross-job artifact checksum

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-08 |
| **Deciders** | Maintainer, via Phase 16 (audit 0016 #121, #123, #125, #205) |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 16's CI/CD audit (audit 0016) treated `.github/workflows/ci.yml`
itself as an attack surface, not just a tool. It found:

- **#121** — third-party actions referenced by mutable major tag
  (`actions/checkout@v7`). A tag can be repointed; a compromised action
  release then runs with the workflow's token and on the runner that
  builds and (in the desktop job) executes our server binary.
- **#123** — no `permissions:` block, so `GITHUB_TOKEN` gets the
  repository's default scope (historically `write` for many events).
  The workflow only reads the repo and passes artifacts between its own
  jobs.
- **#125** — no static security analysis. `govulncheck` covers known-CVE
  dependencies but nothing scans our own code for insecure patterns.
- **#205** — the `server-binary` artifact is built in the `backend` job
  and executed in the `desktop` job with only an implicit trust chain.

The roadmap (Phase 16 "Architecture decisions expected") calls for this
to be recorded as an ADR with its cost.

## Decision

1. **Every third-party action is pinned to a full commit SHA**, with a
   trailing `# vX` comment. Dependabot's `github-actions` updater bumps
   the SHA and the comment together (already configured). First-party
   `actions/*` included — GitHub's own account is not exempt from the
   "a tag can move" property.
2. **`permissions:` is declared least-privilege.** Top-level
   `contents: read`, and every job restates `contents: read` so a job
   added later does not silently inherit a wider default. No job needs
   more; a future step that does must add the specific scope to that job
   only.
3. **`gosec` runs in the backend job**, scoped to
   `-severity high -confidence high`. The tree is clean at that bar
   today (0 findings), so a new high/high finding is a real regression.
   Broader gosec levels and a JS security linter are deliberately not
   added now — they are noise without a triage budget (#125 notes this).
   `gosec` runs via `go run …@v2.29.0`, ephemeral, no `go.mod` entry —
   same pattern as `govulncheck`, so §9 does not apply.
4. **The `server-binary` artifact carries a SHA-256 checksum.** The
   backend job records `alexandryn-server.sha256`; the desktop job runs
   `sha256sum --check` before `chmod +x`. GitHub artifacts are already
   scoped to one workflow run, so this defends only against a bug or a
   future change that widens artifact sharing — cheap defence in depth
   on the one artifact that gets executed.

## Options considered

### Pinning: SHA vs. tag vs. a pinning action (e.g. `sha-pin`)

Tag is the status quo and the finding. A third-party pinning action adds
another trust root. Plain SHAs with a Dependabot updater is the standard
answer and adds no new dependency. Chosen.

### Permissions: `read-all` vs. `contents: read`

`read-all` is broader than needed (grants `packages: read`,
`id-token: read`, etc. this workflow never uses). `contents: read` is
the minimum for checkout. Chosen; widen per-job if ever required.

### SAST: gosec now (blocking) vs. gosec advisory vs. defer

Advisory SAST that never fails the build is theatre. Deferring leaves
#125 open. Running it blocking but at the high/high bar, where the tree
is already clean, gets the regression-catch value with no upfront
remediation cost. Chosen.

### Artifact trust: checksum vs. cosign/attestation vs. accept run-scoping

Sigstore attestation (`actions/attest-build-provenance`) is the heavier,
more complete answer and needs `id-token: write` — exactly the
permission we just removed. A checksum file is enough for a threat that
is already mostly mitigated by run-scoping. Chosen; revisit if the
binary is ever published outside CI.

## Consequences

- `ci.yml` gains ~15 lines (permissions blocks, gosec step, two checksum
  steps) and every `uses:` line grows a SHA.
- Cost: Dependabot now opens a PR whenever any pinned action releases —
  more PR noise, grouped minor/patch to limit it. A SHA in a diff is
  less human-readable than a tag; the `# vX` comment mitigates.
- `gosec` adds ~20–40s to the backend job (one `go run` compile + scan).
- No ADR is owed for the CODEOWNERS lockfile fix (#128) or the
  Dependabot npm addition (#126) — those are corrections to existing
  config, not decisions.

## Addendum — Branch protection on private repository (audit 0016 #202) (2026-09-10)

GitHub Free plans do not allow branch protection rulesets on private
repositories (API returns HTTP 403 `Upgrade to GitHub Pro or make this
repository public`).

The project trade-off is resolved as follows:
1. When the repository is made public at release under AGPL-3.0 (ADR 0002,
   ADR 0006), GitHub native branch protection rulesets become available
   without fee. At that point, native protection on `main` must be enabled
   (requiring CI checks to pass, 1 approving review, CODEOWNERS approval,
   and disallowing force push).
2. Prior to public release, client-side protection is enforced via
   `.githooks/pre-push` (configured via `scripts/install-git-hooks.sh`),
   which intercepts git pushes and aborts any direct push targeting `main`.

