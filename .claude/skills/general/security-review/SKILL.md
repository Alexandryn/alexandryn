---
name: "security-review"
description: "Alexandryn's security-review pass: a thin layer over the built-in security-review skill adding this project's own four-attacker model and audit recording format instead of a generic vulnerability checklist. Use for audit-gate reviews (before a phase closes) or any change crossing a trust boundary."
argument-hint: "[scope: diff range, spec name, or 'phase NN']"
---

Not a duplicate of the built-in `security-review` skill — invoke it for
the actual diffing/vulnerability-identification mechanics (it runs a
multi-phase subagent pipeline: identify candidates, then independently
verify each one to filter false positives). This skill fixes what's
specific to Alexandryn on top of that mechanism.

## Scope: code diffs by default, specs when there's no diff to review

The built-in skill diffs against `origin/HEAD` or working-tree changes.
If there's nothing pending (everything already merged, or the target is
a set of design specs with no implementation yet), diff against a fixed
base commit instead, or run the same methodology directly against the
spec files' content — the built-in skill's hard exclusion "do not report
findings in documentation files" does not apply to a document that *is*
the normative design for security-critical code about to be written from
it. State explicitly which mode you're running in before starting.

## The four attackers (`audits/README.md`) — walk all four, every time

1. **A malicious user of this instance** — has a credential, wants more
   than their share.
2. **A malicious source** — user-configured, possibly after being
   tricked; controls its metadata, filenames, redirects, and bytes.
3. **A device on the local network** — no credential, shouldn't need one
   to be dangerous. Includes a browser on the user's own machine visiting
   a hostile page.
4. **Hostile content** — a book file that's fine to possess but not fine
   to parse.

Not every phase has all four in scope yet (phase 03 has none — no
sources, no auth, no file parsing exist there). Say which ones are N/A
and why, don't silently skip them.

## Constitution's hard rules — check these explicitly, not just generically

- §4: every trust-boundary crossing has a size limit, a shape check, a
  timeout.
- §5: Electron preload exposes an enumerated operation list, never a
  general primitive; every operation validates its arguments in the main
  process.
- §6: no build reaches the LAN without a credential; loopback is the
  default, LAN binding is physically absent until phase 12/13, not just
  discouraged.
- §8: never logged — source credentials, session tokens, full
  home-directory paths, what someone is reading.

## Recording — `audits/`, this project's own format

Every finding goes in `.claude/audits/NNNN-<scope>.md`, following
`templates/audit.md` — not the built-in skill's raw markdown output
format. Rate honestly per `templates/audit.md`'s severity guide (rated
against actual impact in *this* system, not the textbook worst case for
the vulnerability class). A finding closes as `Fixed` (with the commit),
`Accepted` (who accepted the risk and why), or `Won't fix` (the
reasoning) — never by going quiet. No phase closes with an open Critical
or High finding.

## Confidence threshold — judgment, not a hard cutoff

The built-in skill's own instructions say to drop findings under 8/10
confidence to control noise in automated PR scanning. For a security
audit specifically, surface anything that's mechanistically real and
concretely fixable even at 6-7/10, if the underlying issue is a
constitution-level concern (credential leakage, a trust-boundary gap) —
note the confidence plainly rather than silently filtering. False
positives get killed by the built-in skill's own independent
verification step regardless; don't double-filter on top of that.
