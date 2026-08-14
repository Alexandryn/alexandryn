---
name: "review"
description: "Structured, batched review pass -- required before opening a PR (once /make-pr is real) and before moving from one spec to the next while we aren't. Drafts every finding first, shows the complete batch before acting on any of it, reuses this project's own severity vocabulary (templates/review.md) instead of inventing one. Adapted from github-pr-review's discipline, not its gh api posting mechanics -- those apply once there's a PR to post to."
argument-hint: "[scope: spec name, PR number, or 'since last checkpoint']"
disable-model-invocation: true
---

Source: [aidankinzett/claude-git-pr-skill](https://github.com/aidankinzett/claude-git-pr-skill)'s
`github-pr-review` skill. That skill's actual subject is the GitHub API
mechanics of posting PR review comments (pending reviews, `suggestion`
blocks, `APPROVE`/`REQUEST_CHANGES`/`COMMENT`). Nothing here posts to
GitHub yet — there's no PR. What's adapted is the *discipline* underneath
the mechanics, which applies regardless of whether the output goes to a
GitHub API call or straight to the maintainer in chat: **draft everything
first, batch it, show the complete result before acting on any of it.**

## When this is required, not optional

- Before opening a PR (`/make-pr` posts against a real PR; this skill is
  what feeds it findings once there's one to post to)
- **Before moving from one spec to the next**, while we aren't making PRs —
  this is the current mandatory trigger. Finishing a spec and starting the
  next one without running this is the exact "I'll post this one now and
  batch the rest later" mistake the source skill calls out, just relocated
  from PR comments to spec transitions.

## Core workflow

1. **Fix scope** — one spec, a diff range, or (with no argument) everything
   changed since the last point the maintainer actually saw a batched
   review. Say the scope out loud before starting; don't let it drift
   while reviewing.
2. **Draft every finding before presenting any of them.** Read the full
   scope, not just the last file touched. This is the source skill's
   "always use pending reviews, even for one comment" rule — a review
   drafted incrementally and shown piecemeal is the failure mode, not a
   time-saver.
3. **Severity, using this project's own vocabulary** — `templates/review.md`
   already has one: Blocking / Major / Minor / Nit. Use it. Don't import
   GitHub's `APPROVE`/`REQUEST_CHANGES`/`COMMENT` — those are event types
   for a PR that doesn't exist yet; this project's own verdicts (Approved /
   Approved with changes / Needs rework / Rejected, `reviews/README.md`)
   already cover the same ground.
4. **Assign one overall verdict for the batch**, not just per-finding
   severities — the source skill's "overall review message" step. Finding
   severity drives three of the four outcomes: any Blocking → Needs
   rework; Major/Minor only, no Blocking → Approved with changes; Nit-only
   or clean → Approved. **`Rejected` is not on that scale** — per
   `reviews/README.md`, it means the *approach itself* is wrong, not that
   findings piled up. Choose it independently of the tally, when the right
   answer is "don't build this the way it's specified," not "fix these
   things."
5. **Write the batch to `.claude/reviews/NNNN-*.md` before fixing anything**
   — this is the actual enforcement mechanism, not the prose instruction
   alone. A batch that exists only in chat scrollback isn't checkable
   months later, which is the entire stated purpose of recording reviews
   at all (`reviews/README.md`). This also means: show the maintainer the
   complete batch in the same message, same reasoning as the source
   skill's `AskUserQuestion` gate before a GitHub post — the difference is
   this project runs with standing authorization to proceed without a
   blocking prompt between spec transitions, so the record substitutes for
   the stop. Same self-review caveat as every other review this session:
   mark it self-reviewed, needing independent confirmation, unless someone
   other than the spec's author is actually running this pass.
6. **Only after the batch is written and shown, fix what's agreed** —
   matching how every spec self-review this session has actually worked in
   practice (draft findings, present, then fix), now an explicit required
   step with a checkable artifact behind it, not just a habit.

## Red flags — stop if thinking any of these

Directly from the source skill, relabeled for this repo's actual situation:

- "This spec is simple, I don't need a full review pass before the next one"
- "I found one issue, let me just fix it and keep reading for more"
- "The maintainer said keep going without approval, so I can skip showing
  the batch too" — no: "go ahead without stopping between specs" and
  "skip showing what a review found" are different permissions; the first
  was given, the second wasn't
- "I'll mention what I found at the end of my summary" — a paragraph
  buried in a long response is not the same as a structured batch
  presented as the review's actual output
- "It's just a docs/spec repo, PR-review discipline doesn't really apply"
  — the discipline (draft first, batch, show before acting) has nothing to
  do with GitHub specifically; it's about not letting review quality
  degrade under momentum

**If any of these fire**: this skill can't invoke itself
(`disable-model-invocation: true`) — say the red flag out loud and ask the
maintainer to run `/review`, don't silently self-apply the discipline
without naming that a shortcut was about to happen.

## Relationship to the built-in `code-review` skill

`skills/README.md` already has an open question about whether the planned
purpose-level "Code review" skill duplicates the built-in `code-review`
skill. This one is a third, different thing: `code-review` reviews a
diff/PR against general code-quality dimensions; this skill governs *when*
a review must happen (spec transitions, pre-PR) and *how the finding batch
is produced and recorded*, reusing this project's own severity vocabulary
rather than a generic one. Not a duplicate — but worth checking again once
the purpose-level "Code review" skill is actually written, in case the
overlap grows then.

## Once PRs are real

When `/make-pr` is actually opening PRs against a remote with reviewers,
this skill's batched findings become the input to the source skill's own
`gh api` pending-review workflow — draft here, then post using its exact
mechanics (pending review created first, `AskUserQuestion` approval before
submitting, correct event type chosen from Blocking findings → likely
`REQUEST_CHANGES`, Nit-only → likely `APPROVE` or `COMMENT`). Not
implemented now because there is nothing to post to; revisit when phase 99
or an earlier phase's first real PR exists.
