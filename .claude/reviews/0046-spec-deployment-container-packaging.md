# Review: `deployment-container-packaging.md`

| | |
|---|---|
| **Subject** | `.claude/specs/deployment-container-packaging.md` |
| **Reviewer** | Claude (Sonnet 5), self-reviewed — independent read still pending |
| **Date** | 2026-08-17 |
| **Verdict** | Approved with changes (finding below fixed before this review) |

## Summary

New spec fixing the `Dockerfile` and `docker-compose.yml` content ADR
0015's container target needs and the six already-amended specs assume
exist. States plainly that the container target is unreachable by design
until phase 12/13, with the kernel-enforced-vs-config-mutable reasoning
for why that isn't relaxed, and adds a CI guard (FR-6) so the property
holds against ordinary future edits, not just present intent.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Blocking | FR-5/FR-6 relationship | An early draft stated FR-5 (no `ports:`, no `network_mode: host`) without FR-6 (a CI check enforcing it), leaving the property as a convention rather than an enforced one — exactly the failure mode this spec's own Security considerations argues against for the bind-relaxation alternative it rejects. Would have been self-contradictory to reject "protected only by absence of a `ports:` line" as too weak for the bind question while accepting the identical weakness for the compose file's own state. | Fixed — FR-6 added as a required, not optional, static CI check; Security considerations' closing line makes the parallel explicit ("FR-6 exists because 'we agreed not to' isn't a control") |

## Dimensions checked

- [x] **Completeness** — covers Dockerfile, compose file, CI test
      dependency, and the security reasoning the stop-condition exchange
      surfaced, not just the mechanical artifacts
- [x] **Ambiguity** — FR-1 through FR-6 are specific enough that two
      engineers building from this spec would produce the same
      `docker-compose.yml` shape (two services, one profile, one volume,
      no published port)
- [x] **Architecture** — confirmed no overlap with `architecture-testing.md`
      FR-4 (Docker image vulnerability scanning, phase 99) — that's a
      release-time scan of a tagged image, this spec is the build/compose
      definition itself; different concerns, correctly left un-merged
- [ ] **Domain correctness** — not applicable
- [x] **Security** — this spec's central content *is* a security decision
      (why loopback stays literal); checked that its reasoning doesn't
      contradict `backend-configuration.md` FR-8 or `architecture-system.md`
      anywhere, and that FR-6 actually closes the gap FR-5 alone would
      leave open
- [x] **Testability** — every FR maps to an acceptance criterion that
      names how it's proven, not just what's true
- [ ] **Accessibility** — not applicable
- [ ] **UX and copy** — not applicable
- [x] **Observability** — confirmed no new logging requirement was needed
      (stdout capture is automatic under Compose) rather than assuming so
- [x] **Maintainability** — Non-goals section is explicit about registry/
      versioning/Kubernetes/Windows-container being out of scope, so a
      future contributor doesn't read silence as "undecided"
- [x] **Evolution** — Open questions explicitly name what phase 12/13
      inherits and where (this spec, and `decisions/README.md`'s table),
      so the non-loopback-bind question isn't lost between now and then

## Contradictions and gaps

Found and fixed (Finding #1). Checked this spec against all six specs it
references for contradiction — found none; each reference is either a
direct implementation of something already decided (ADR 0015's
`DATABASE_URL` design, FR-4 here) or an explicit non-goal deferring to
where the real decision belongs (phase 12/13, phase 99).

## What I did not review

The actual base image choice (Alpine, distroless, or otherwise) — FR-1
requires "minimal," not a specific image, left as an implementation
choice with no correctness consequence spelled out here. Whether
`curl` or `wget` is available in whatever minimal image gets chosen for
FR-3's `HEALTHCHECK` command — flagged in FR-3's own text as a detail to
resolve during implementation, not decided in this pass.
