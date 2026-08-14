---
name: "commit"
description: "Create a git commit following Alexandryn's conventions: Conventional Commits format (feat/fix/refactor/spec/docs/build/chore/test, with optional scope), split by meaning, a Refs: trailer for anything touching .claude/, no Co-Authored-By trailer ever. Use for every commit in this repository, including the commit step inside other skills."
argument-hint: "[optional context for the message]"
disable-model-invocation: true
---

This repo's commit conventions override the global default commit behavior — see [`CONTRIBUTING.md`](../../../CONTRIBUTING.md#branches-and-commits), which this skill implements.

1. Review before touching anything: `git status` and `git diff` (staged and unstaged).
2. **Split by meaning, not by "everything I happened to touch."** Before staging anything, identify each distinct functional change in the working tree — a spec draft, a review of that spec, fixes to that review's findings, and a roadmap status update are four different meanings even when they land in the same working session. If you can't describe the pending changes in one sentence without "and" joining two unrelated ideas, it's more than one commit. When in doubt, split further — a run of several small, obviously-related commits is correct here, not a smell.
   - This applies with extra force to `.claude/` work specifically: drafting a spec, reviewing it, fixing findings from that review, and a maintainer approving it are each their own commit even though they often happen back-to-back. Don't bundle "wrote the spec and reviewed it" into one commit just because no code changed in between.
3. Stage only the files (or, if a single file legitimately contains two meanings, only the relevant hunks via a non-interactive means — e.g. reconstructing the file's staged version by hand) for the ONE meaning being committed right now. Never `git add -A`/`git add .` reflexively — check what a broad add would sweep in first.
   - **`git add` and `git diff --cached --name-status` are one atomic step, never separated.** Every `git add` is immediately followed by `git diff --cached --name-status` in the same breath, read before running `git commit`.
   - **Before typing the file list, state out loud (in your own output) which files you expect this commit to contain.** Compare that stated list against the actual `git diff --cached --name-status` output, file by file. Any mismatch — an extra file, a missing file, a rename you didn't expect — **stop. Do not run `git commit`.** Unstage the surprise (`git restore --staged <path>`) or stage the missing piece, then re-diff and re-compare.
   - Corollary: once a file's edits for the current meaning are finished and verified, stage it right then — don't leave it unstaged while moving on to other files.
4. Write the message in **Conventional Commits format** (https://www.conventionalcommits.org/en/v1.0.0/): `<type>(<optional scope>): <description>`.
   - Types, matching this repo's own branch prefixes (`CONTRIBUTING.md`): `feat` (new capability), `fix` (bug fix), `refactor` (behavior-preserving restructuring), `spec` (adding, reviewing, or revising anything under `.claude/specs/`, `.claude/decisions/`, `.claude/reviews/`, `.claude/audits/`, or `.claude/roadmap/`), `docs` (README, CONTRIBUTING, CLAUDE.md, or other project documentation — not `.claude/`'s engineering-memory documents, which are `spec`), `build` (tooling, dependencies, CI), `chore` (repo housekeeping that isn't build-related), `test` (test-only changes).
   - Scope is optional, useful once the monorepo has named packages (`feat(server):`, `feat(web):`). Omit for repo-wide changes — most `.claude/` work is repo-wide.
   - Description: lowercase, imperative mood, no trailing period.
   - Add a body (blank line, then 1-2 sentences) when the *why* isn't obvious from the description alone. The body explains why; the diff already shows what.
   - When the commit is about a spec, ADR, review, or audit, add a trailer line `Refs: .claude/<path>` pointing at the specific document, matching `CONTRIBUTING.md`'s own example. Multiple `Refs:` lines are fine if more than one document is central to the change.
5. **Do not add a `Co-Authored-By` trailer, or any other AI-attribution line, under any circumstance.** This overrides the general default — commits in this repo are authored solely by the user, full stop, no exception for any skill or workflow.
6. Never use `--no-verify`, `--amend` (unless explicitly asked), or force-push — routine commits are always additive. (A deliberate history rewrite, if the user explicitly asks for one, is a separate, one-off operation outside this normal flow — not something this skill does on its own initiative.)
7. After committing, run `git status` to confirm a clean result and report the commit hash plus a one-line summary.
8. If the pending changes split into multiple commits, repeat steps 2-7 per meaning, in a sensible dependency order (each intermediate commit should still leave the repo in a working, buildable state), until the working tree is fully committed.

If `$ARGUMENTS` gives context for what this commit is for, use it to inform the message rather than pasting it in verbatim.
