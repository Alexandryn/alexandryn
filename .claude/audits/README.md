# Security audits

Every substantial feature gets an adversarial pass before its phase closes.
Start from [`../templates/audit.md`](../templates/audit.md).

This is not a checklist exercise. The question being answered is narrow and
concrete: *what does an attacker actually get here?*

## The four attackers to consider

Every audit walks all four, because they have different capabilities:

1. **A malicious user of this instance** — has a credential, wants more than
   their share.
2. **A malicious source** — the user configured it, possibly after being
   tricked. It controls its metadata, its filenames, its redirects, and the
   bytes it returns.
3. **A device on the local network** — has no credential and shouldn't need
   one to be dangerous. Includes a browser on the user's own machine visiting a
   hostile page.
4. **Hostile content** — a book file that is fine to possess but not fine to
   parse. Zip bombs, malformed EPUB, embedded scripts, absolute paths in
   archives.

## Naming

`NNNN-<scope>.md`, sequential. Findings inside are `A-NNNN-01`, `A-NNNN-02`.

## Rating honestly

Rate the impact *in this system*, not the textbook worst case for the class of
bug. An XSS in a field only the local user can write is not the same as one a
remote source controls.

Inflated severity is as damaging as a missed finding, because it teaches people
to stop reading the ratings. If the honest answer is "informational", write
informational.

## Closing a finding

A finding closes as `Fixed` (with the commit), `Accepted` (with who accepted
the risk and why), or `Won't fix` (with the reasoning). It does not close by
going quiet.

No phase closes with an open Critical or High finding.

## Index

| Audit | Scope | Date | Critical | High | Verdict |
|---|---|---|---|---|---|
| [0001](0001-phase03-backend-specs.md) | Phase 03 backend foundation specs (6 specs, 3 ADRs), pre-implementation | 2026-08-14 | 0 | 0 | 1 Medium finding, fixed same session |
| [0003](0003-cmd-server-startup-shutdown.md) | `cmd/server` startup/shutdown sequence, Checkpoint F (T17-T20) | 2026-08-19 | 0 | 1 | 5 of 6 findings fixed same session; 1 (spec-text amendment) open, deliberately deferred |
| [0004](0004-phase04-frontend-foundation.md) | Phase 04 frontend foundation (shell, routing, tokens, primitives) | 2026-08-30 | 0 | 0 | Clear |
| [0005](0005-phase05-desktop-host.md) | Phase 05 desktop host (Electron, parentwatch, IPC surface) | 2026-08-31 | 0 | 1 | 1 High finding, fixed same session |
| [0006](0006-phase06-library.md) | Phase 06 library domain (backend API, frontend screens, collections) | 2026-08-31 | 0 | 0 | Clear |
| [0007](0007-phase07-metadata.md) | Phase 07 metadata discovery & caching (Open Library client, cover LRU cache, Discover screen) | 2026-08-31 | 0 | 0 | Clear |
| [0009](0009-phase09-async-jobs.md) | Phase 09 async jobs (PostgreSQL-backed queue, worker pool, retry/backoff, dead-letter, reaper, lifecycle wiring) | 2026-09-01 | 0 | 0 | Clear (2 Low, 2 Informational, all accepted) |
| [0012](0012-phase12-auth.md) | Phase 12 authentication, RBAC & multi-library namespacing | 2026-09-02 | 0 | 2 | 2 High findings resolved and certified in Phase 13 |
| [0013](0013-phase13-network-access.md) | Phase 13 network access & device pairing | 2026-09-05 | 0 | 0 | Clear (1 High, 1 Med, 2 Low fixed; 1 Info accepted) |
| [0014](0014-phase14-devices-and-sync.md) | Phase 14 devices and sync | 2026-09-06 | 0 | 0 | Clear |
| [0015](0015-phase15-observability.md) | Phase 15 observability | 2026-09-07 | 0 | 0 | Clear (1 Med, 2 Low, all resolved) |
| [0016](0016-phase16-security-hardening.md) | Phase 16 whole-application security hardening sweep (all three trust boundaries, deps, CSP, CI/CD, test coverage) + `/code-review ultra` cross-check | 2026-09-07 | 0 | 16 | Findings open — 120 filed as issues #86–#304 (0 Critical, 16 High, 48 Med, 47 Low, 9 Info). Ultra pass found 4 Phase 15 deliverables certified but never wired (#294–#296, #302). Phase closes when Critical/High are resolved |

