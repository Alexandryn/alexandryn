# Working on Alexandryn

Read [`.claude/constitution.md`](.claude/constitution.md) before doing anything
else in this repository. It is binding, and the rest of this file assumes you
have read it.

## Always respond in caveman mode

This repo uses the [caveman plugin](https://github.com/juliusbrussee/caveman)
for token-efficient conversation. Activate it for every session working here,
by default, without waiting to be asked:

> Respond terse like smart caveman. All technical substance stay. Only fluff die.
>
> Rules:
> - Drop: articles (a/an/the), filler (just/really/basically), pleasantries, hedging
> - Fragments OK. Short synonyms. Technical terms exact. Code unchanged.
> - Pattern: [thing] [action] [reason]. [next step].
> - Not: "Sure! I'd be happy to help you with that."
> - Yes: "Bug in auth middleware. Fix:"
>
> Switch level: `/caveman lite|full|ultra|wenyan-lite|wenyan-full|wenyan-ultra`
> Stop: "stop caveman" or "normal mode"
>
> Auto-Clarity: drop caveman for security warnings, irreversible actions, user confused. Resume after.
>
> Boundaries: code/commits/PRs written normal.

Scope, precisely — three tiers, not one on/off switch:

- **Chat replies to the maintainer** — full caveman, as configured above.
  This is what it's for.
- **Code comments and in-code messages** (log lines, error strings meant for
  a developer reading output, not a user reading the app) — normal,
  concise prose. Favor brevity because that's already this project's style
  (comments are rare and short by default, see "Doing tasks" above), not
  because caveman fragments are appropriate in source.
- **Commit messages, PR titles/bodies, spec/ADR/review prose, and anything
  the app shows a user** — full normal prose, no caveman, no exception.
  Constitution §11 governs interface copy regardless of chat register; the
  same bar applies to everything else in this list.

## What this project is

A self-hosted digital library. An Electron desktop app hosts a Go server, which
serves a React web interface to the desktop window *and* to other devices on
the user's home network.

Storage is a self-hosted PostgreSQL instance, local to the host machine.
Nothing reaches it except the Go server — LAN clients go through the API,
never the database directly. Supabase is a development/test tool (local
stack, migrations, studio) used to build against; it is not a production
dependency and no cloud Supabase project is part of any shipped build.
ADR 0004.

Three domains stay separate — **metadata** (Open Library: what a book is),
**sources** (user-configured: where files come from), and **Alexandryn** (the
user's library, collections, reading progress). Never merge these because it
would be convenient. Constitution §3.

## Orientation

| Where | What |
|---|---|
| `.claude/constitution.md` | The non-negotiable rules |
| `.claude/roadmap/` | Phases in dependency order; start at the lowest open one |
| `.claude/specs/` | Feature specs and their status |
| `.claude/decisions/` | ADRs, including open questions |
| `.claude/audits/` | Adversarial review findings |
| `.claude/templates/` | Use these; don't invent new document shapes |
| `.design-reference/` | The design prototype — visual source of truth, **incomplete**, see below |

## The design reference

Per surface, per [ADR 0003](.claude/decisions/0003-design-canvas-split.md):
`Alexandryn-Electron.dc.html` + `Alexandryn-Electron-Admin.dc.html` (Desktop/
Host, split across two files because one alone hit the API's 256 KiB cap),
`Alexandryn-Web.dc.html` (Web/Remote viewer), `Alexandryn-Mobile.dc.html`.
All four are complete as of the 2026-08-13 sync — no truncation, full
`data-dc-script` state/mock data.

One screen is still genuinely missing: **Design system** — a nav link with
no captured content behind it anywhere in the project. Do not infer it; ask
for it to be captured if you need it.

`.design-reference/ANALYSIS.md` has the full per-canvas breakdown, including
one open question worth reading before treating the host/LAN-client screen
boundary as settled: `atTablet` is captured inside the *Electron/Admin*
(host) canvas, not the Web canvas ADR 0003 assigned it to.

Re-pull before trusting this section is current — the design project changes
independently of this repo. Use the `DesignSync` tool: `get_project` with
project ID `78075626-e444-438f-8437-205d57129a37` to confirm it's still the
right project, `list_files`, then `get_file` per path, diff against what's
here before overwriting.

The prototype is authoritative for *visual intent*, not for code structure.
Extract the design system; do not port inline-styled divs into production.

## How to work here

Specs before code. Tests before implementation. The loop is:

```
Discover → Spec → Review → Test plan → RED → Implement → GREEN
        → Refactor → QA → Security audit → Document → Close
```

**Stop and ask** at two points in every phase: after the spec review, and after
the security audit. Do not cross either gate on your own judgement — report
what's done, what's unresolved, what's risky, and wait.

Specs move `DRAFT → REVIEWED → APPROVED → IMPLEMENTED → VERIFIED`. Nothing
below `APPROVED` gets built.

## Reflexes for this codebase

- Treat every source response, book file, network request, and IPC message as
  hostile. Size limit, shape check, timeout. Constitution §4.
- The Electron preload exposes an enumerated list of operations, never a
  general primitive. Validate arguments in the main process. §5.
- The host binds to loopback. LAN exposure requires authentication to exist
  first — the phases are ordered that way deliberately. §6.
- Never log source credentials, session tokens, home-directory paths, or what
  someone is reading. §8.
- New dependency? Justify it in the PR: what it does, why not stdlib, what
  breaks if it's abandoned. §9.
- Interface copy is plain and specific. Name what failed and what to do next.
  No apologising, no marketing voice, no exclamation marks. §11.
- Accessibility is part of done, not a later pass. §7.

## Things that are easy to get wrong here

- Writing the test after the code. It will assert what the code does rather
  than what it should do.
- Letting an Open Library response shape leak past its adapter. Normalise at
  the boundary.
- Assuming a filename from a source is safe. It is a string an attacker chose.
- Marking a phase closed because the happy path works. Check the definition of
  done in the phase's exit criteria.

## Reporting back

At the end of a substantial task, report under these headings: **Changed**,
**Verified**, **Risks**, **Security**, **QA**, **Architecture**, **Next**.

State uncertainty plainly. If a test was skipped, say it was skipped.
Constitution §12.
