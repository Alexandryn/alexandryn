# Working on Alexandryn

Read [`.claude/constitution.md`](.claude/constitution.md) before doing anything
else in this repository. It is binding, and the rest of this file assumes you
have read it.

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

## The design reference is truncated

`.design-reference/Alexandryn.dc.html` was retrieved through an API with a
256 KiB cap and is cut off mid-element. It has no closing tags, and the
trailing `<script data-dc-script>` block — which held all state logic and mock
data — is missing entirely, along with the Tablet, Mobile, Remote, States and
Design system screens.

`.design-reference/ANALYSIS.md` records what could actually be extracted, and
what couldn't. Do not infer the missing screens. If you need them, ask for a
complete export.

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
