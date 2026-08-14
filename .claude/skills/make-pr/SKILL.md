---
name: "make-pr"
description: "Open a GitHub PR for Alexandryn: title in type(scope): summary format (react-spectrum's PR naming convention, same type vocabulary as /commit), body from .github/pull_request_template.md populated with real content — grouped by theme when the diff is large, anything critical first, Definition of done checked honestly. Never merges. Use once a branch's commits are ready for review."
argument-hint: "[optional context: target branch, draft/ready, issue number]"
disable-model-invocation: true
---

Sources: [react-spectrum's PR naming guide](https://github.com/adobe/react-spectrum/wiki/Pull-Request-Naming-Guide) for the title format; the structural principles (not the emoji, not the tone — constitution §11 rules those out here) of [a PR-review-summary skill](https://www.diabrowser.com/skills/prreviewsummary-ishaan) for the body: group by theme, put anything critical first, close with an honest checklist. `.github/pull_request_template.md` is the actual skeleton — this skill fills it in, it doesn't replace it.

## Is a PR the right move right now?

Check first. Per the maintainer: while work is still text-heavy — specs, ADRs, reviews, roadmap, no ticket to attach a PR to yet — commits go straight to `main` via `/commit`, no PR. PRs become the rule once: (a) tickets exist per roadmap item (created after the roadmap itself is finished), and (b) there's code, lint, and CI to actually gate on. If neither is true yet, stop and say so instead of opening a PR nobody asked for.

If a PR genuinely is warranted right now (the maintainer asked for one, or a branch already exists with commits ready for review), continue.

## Process

1. **Confirm the branch state.** `git status`, `git log main..HEAD --oneline` (or the actual base if not `main`). Every commit on the branch should already follow `/commit`'s conventions — if one doesn't (a stray `Co-Authored-By`, a non-Conventional-Commits message), fix that first via `/commit`'s own rules, don't paper over it in the PR body instead.
2. **Push the branch** if it isn't already on `origin` (`git push -u origin <branch>`), unless it's already there.
3. **Title**: `type(scope): summary`, same type vocabulary as `/commit` — `feat`, `fix`, `refactor`, `spec`, `docs`, `build`, `ci`, `chore`, `test`, `bump`, `revert`. Scope is optional, a contextual noun (`feat(server):`), never an issue number. Lowercase, imperative, no trailing period. If the branch has multiple commit types, the title describes the PR's overall *purpose*, not just its first commit — pick the type that describes what a reviewer is actually approving.
4. **Body**, from `.github/pull_request_template.md`:
   - **What changed** — one paragraph for a small PR. For a large one, group by theme or file area instead of listing commits in order — a reviewer thinks in "what changed about X," not "what happened first." Anything that touches a trust boundary, a domain boundary (constitution §3), or reverses an earlier decision goes first, named as such, not buried.
   - **Related** — `Spec:`, `Phase:`, `Issue:`. If no ticket exists yet (pre-roadmap-completion), write `Issue: none yet — ticket-per-roadmap-item starts after the roadmap is finished`, not a fabricated placeholder number.
   - **How it was verified** — name the actual tests, or the actual review/walkthrough if this is a docs/spec PR with nothing to execute. "Tests pass" alone is not acceptable here any more than it is anywhere else in this project.
   - **Definition of done** — check each box honestly, per this project's own standing rule (`reviews/README.md`: "an unchecked box is useful information, a falsely checked one is worse than no checklist"). Leave a box unchecked and say why in Risks if it doesn't apply or isn't done yet.
   - **Risks** — what could this break, what the first symptom would be. "None I can see" only if actually looked, same bar as the template already states.
5. **Show the composed title and body before opening it** — this is a shared, visible action the moment it's created (GitHub notifications, anyone watching the repo), same category as a push, not a routine step to run silently.
6. **Open it**: `gh pr create --title "..." --body "$(cat <<'EOF' ... EOF)" --base <base, default main>`. Draft (`--draft`) if CI exists and hasn't run yet, or if explicitly asked for; ready otherwise — there's no CI to be draft-until-green against yet.
7. **Report the PR URL.** This skill opens PRs. It does not merge them, approve them, or request reviewers unless explicitly asked — those are separate, explicit actions.

## Open questions, not resolved by this draft

- Draft-vs-ready default once GitHub Actions actually exist — revisit when phase 03's CI lands.
- Reviewer/label assignment conventions — nothing decided yet, ask if it comes up before a convention exists.
