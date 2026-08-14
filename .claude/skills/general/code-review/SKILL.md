---
name: "code-review"
description: "Alexandryn's code-review pass, once real code exists: a thin layer over the built-in code-review skill adding this project's own severity vocabulary and review dimensions instead of a generic checklist. Use for any diff/PR review of Go or TypeScript/React code in this repository."
argument-hint: "[diff range, PR number, or file path]"
---

Not a duplicate of the built-in `code-review` skill — this is a thin
wrapper around it. Invoke the built-in skill for the actual diffing
mechanics and finding-generation; this skill fixes what's specific to
Alexandryn on top of it. Also not a duplicate of the repo's own `review`
skill: that one governs *when* a review must happen and *how the batch
is produced/recorded* (spec transitions, pre-PR); this one governs *what
dimensions get checked* once there's code. Use both together for a PR:
`review` for the batching discipline, this for the review content.

## Severity vocabulary — this project's own, not GitHub's

Blocking / Major / Minor / Nit, exactly as `templates/review.md` and
`reviews/README.md` already define them. Never substitute a generic
scale.

## Dimensions to check, beyond the built-in skill's defaults

- **Domain boundaries (constitution §3)** — does this diff let metadata,
  source, or library concerns bleed into each other? A provider's
  response shape reaching the UI, a source's protocol reaching the
  domain, are Blocking findings here even if the code otherwise works.
- **Hostile input (constitution §4)** — every value crossing a trust
  boundary (a source response, a book file, an HTTP request, an IPC
  message) gets a size limit, a shape check, a timeout. Missing any one
  of the three on a new boundary-crossing point is at least Major.
- **No package-level globals** — see `go-backend-conventions` skill. A
  new global is Blocking, not a style note.
- **Redaction as a type property, not a per-call-site discipline** — a
  new sensitive value that doesn't implement both `slog.LogValuer` and
  `json.Marshaler` is Blocking if it can reach a log line.
- **Copy (constitution §11)** — any new user-facing string: specific,
  calm, no apology, no exclamation marks, says what to do next where
  applicable. A generic "an error occurred" is at least Minor.
- **Accessibility (constitution §7)**, once frontend code exists —
  semantic elements, keyboard reachability, visible focus, labelled
  controls. Not a later pass; missing on a new interactive element is at
  least Major.

## Verdict

Same rule as the `review` skill: any Blocking finding → Needs rework;
Major/Minor only → Approved with changes; Nit-only or clean → Approved.
`Rejected` is independent of the tally — reserved for "the approach
itself is wrong."

## Recording

Follow the `review` skill's recording step — the batch goes to
`.claude/reviews/NNNN-*.md` before anything gets fixed, same discipline
this project has used for every spec review this far.
