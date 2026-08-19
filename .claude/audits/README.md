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
