# 0002. Project licence

| | |
|---|---|
| **Status** | **Proposed — awaiting the owner's decision** |
| **Date** | 2026-08-12 |
| **Deciders** | Project owner |
| **Supersedes** | — |
| **Superseded by** | — |

> This record is deliberately unresolved. Licensing is the owner's call, not an
> engineering default, and picking one silently would be the wrong kind of
> initiative. Until it is settled, the repository is "all rights reserved" —
> which is fine while it is private, and blocking before it goes public.

## Context

Alexandryn is intended to be open source and self-hosted. Both halves of that
matter to the choice.

Self-hosted means the software runs as a *server*. That is precisely the case
where ordinary copyleft has a gap: someone can modify the code, run it as a
service for other people, and never distribute a binary, so never trigger any
obligation to share their changes. Whether that gap should be closed is the
central question here.

The relevant precedent in this exact product category is split. Jellyfin is
GPL-2.0. Calibre, Kavita and Audiobookshelf are GPL-3.0. Komga is MIT. Immich —
the most recent large self-hosted media project — chose AGPL-3.0.

A second constraint: the licence must be chosen before the repository becomes
public. Adding one later is possible but messy, because contributions made
under no licence have unclear terms.

Contribution flows are also affected. Strong copyleft discourages corporate
contribution and some downstream packaging; permissive licensing invites
proprietary forks. Neither is wrong — they optimise for different things.

## Decision

**Recommended: AGPL-3.0-or-later.**

Not yet accepted. The owner should confirm or override before the repository is
made public.

## Options considered

### Option A — AGPL-3.0-or-later (recommended)

Copyleft that extends to network use: anyone who runs a modified Alexandryn as
a service for others must offer their source.

*For* — matches what the software actually is. The whole product is a server
that other people connect to, so the network clause is the operative one, not
an edge case. It keeps hosted forks honest and keeps improvements flowing back.

*Against* — many companies forbid AGPL software outright, so it costs some
contributors and some adoption. It is the most likely licence to be
misunderstood by a well-meaning user who just wants to run it at home (it
imposes nothing on them, but the reputation says otherwise). Some
redistributors avoid it.

### Option B — GPL-3.0-or-later

Copyleft on distribution. Modified desktop builds must be shared; modified
hosted instances need not be.

*For* — the strongest precedent in this category, and better understood than
AGPL. Fully sufficient for the Electron application, which genuinely is
distributed.

*Against* — leaves the server gap open, which is the gap that matters most
here.

### Option C — Apache-2.0

Permissive, with an explicit patent grant.

*For* — maximum adoption and contribution, no friction anywhere, and the patent
clause is real protection that MIT lacks.

*Against* — a company can take Alexandryn, host it commercially, improve it,
and share nothing. If that outcome would feel like a theft rather than a
success, this is the wrong licence.

### Option D — MIT

Permissive and short enough to read.

*For* — the least friction of any option; Komga demonstrates it works in this
category.

*Against* — everything in Option C's against, plus no patent grant.

### Option E — MPL-2.0

File-level copyleft: modified Alexandryn files stay open, but the project can
be combined with proprietary code.

*For* — a genuine middle path, and unproblematic for corporate contributors.

*Against* — the file-level boundary is awkward for an application (as opposed
to a library), and it does not close the hosting gap either.

## Consequences

Whichever is chosen:

- It goes in `LICENSE` at the repository root, with an SPDX identifier in
  `package.json` and `go.mod` where applicable.
- Every dependency must be licence-compatible with it, checked in CI from the
  first dependency onward. Copyleft choices make this check stricter, not
  looser.
- Changing it later requires the agreement of every contributor, or a CLA
  gathered from the start. **This is the reason to decide now rather than
  after the first outside contribution.**
- Whether to require a CLA or DCO sign-off is a separate decision that should
  be made at the same time.

## Reversal cost

**High, and it rises with every contributor.** While the project is
single-author it is a file edit. After the first external pull request it needs
that person's permission. This is among the most expensive decisions in the
repository to defer.

## Confidence

Medium on the recommendation, high on the analysis. AGPL best fits what
Alexandryn *is*, but the trade against contributor reach is a values judgement
about what the project is for, and that judgement isn't an engineer's to make.
