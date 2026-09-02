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

### Where the task list actually lives

This repo has no single `tasks/todo.md` or `tasks/plan.md` as its task list —
generic tooling that expects one should not conclude none exists. The real
task list is distributed: `.claude/roadmap/<NN>-*/README.md`'s Exit criteria
section names what closes each phase, and `.claude/specs/README.md`'s status
column names what's approved, implemented, or still outstanding for each
spec within it. Root-level `SPEC.md` and `tasks/plan.md` are thin pointer
files that exist only so tooling built around those conventional paths can
find this section instead of finding nothing; they are not the task list
themselves, and editing them to add content duplicates what's already
authoritative in `.claude/`.

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

**Stop and ask** at two points in every phase: before drafting each spec —
state its scope and the key decisions it will make, and wait for approval —
and after the security audit. Once a spec's scope is approved, draft it,
self-review it, and mark it `APPROVED` directly; don't stop a second time
to re-approve the same content. Do not cross either gate on your own
judgement — report what's about to happen or what's done, what's
unresolved, what's risky, and wait.

**Stating scope includes a design-conformance check, not just a description
of it.** Before drafting a UI-facing spec, re-read `.design-reference/`'s
canvas(es) for that spec's surface (per ADR 0003's split) fresh — not from
memory of an earlier read — and check whether a captured screen exists for
it and, if so, whether the spec's intended flow matches what's drawn. This
check must leave evidence, not just have been performed: the spec records
which canvas it consulted and the `ANALYSIS.md` sync date as read at
drafting time (a template field exists for this, see
`.claude/templates/spec.md`). If a captured screen contradicts the intended
scope, that's a stop-and-ask before drafting continues, the same as any
other scope conflict — not something to reason around and footnote.

The roadmap decides what is in scope; the design reference decides how an
in-scope surface looks (ADR 0003's addendum). Check `ANALYSIS.md`'s
per-canvas classification: **binding** means match it or escalate;
**exploratory** means no spec is owed; **unclassified** — the default for
anything not explicitly marked — is neither, and must itself be flagged as
a stop-and-ask, not silently treated as either state.

Specs move `DRAFT → REVIEWED → APPROVED → IMPLEMENTED → VERIFIED`. Nothing
below `APPROVED` gets built.

## Boundaries

Every entry here restates something the constitution or an existing
Reflex already binds — nothing below is a new rule invented for this
list.

**Always**
- Spec before implementation; nothing below `APPROVED` gets built (§1).
- Write the failing test before the code that makes it pass (§2).
- Keep metadata / source / Alexandryn boundaries separate; normalise at
  the boundary (§3).
- Validate every external input — source response, book file, network
  request, IPC message — with a size limit, shape check, and timeout (§4).
- Enumerate the Electron preload surface; validate every argument in the
  main process (§5).
- Bind the host to loopback by default; any broader bind is legal only
  when authentication is enforced and one of ADR 0017's two fail-closed
  TLS conditions holds, never on a phase number alone (§6).
- Treat accessibility as part of "done," not a follow-up pass (§7).
- Emit structured logs, request IDs, health checks, and useful failure
  messages from the first line of server code (§8).
- Record a reason for every new dependency: what it does, why not
  stdlib, what breaks if it's abandoned (§9).
- Run the four-attacker adversarial pass before a substantial feature
  closes; record findings in `.claude/audits/`, rated honestly (§10).
- Write interface copy that's plain, specific, and calm (§11).
- State uncertainty plainly — if something's unverified or a test was
  skipped, say so (§12).

**Ask first**
- Amending the constitution itself (its own "Amending this document"
  clause).
- Crossing either review gate — after the spec review, after the security
  audit. The constitution states an automated contributor "must not cross
  a gate on its own judgement" (Review gates).
- Anything that changes behaviour, per `CONTRIBUTING.md`'s own line: a
  typo or CI fix is a PR, anything else starts as an issue and a spec.
- Adding a new dependency — before adding it, not only recorded after
  (§9; `CODEOWNERS` singling out `package.json`/`go.mod`/lockfiles for
  required review is the same rule enforced mechanically).

**Never**
- Implement anything below `APPROVED` spec status (§1).
- Delete or skip a failing test to make a build green (§2).
- Let a metadata provider's response shape reach the UI, or a source's
  protocol reach the domain (§3).
- Treat "the user configured it, so it's fine" as a threat model (§4).
- Disable context isolation or the sandbox, enable Node integration in a
  renderer, or expose a general-purpose file/shell/network primitive from
  the preload (§5).
- Ship a build where the library is reachable from another machine
  without a credential (§6).
- Log source credentials, session tokens, full home-directory paths, or
  the contents of what someone is reading (§8).
- Add a dependency with no recorded reason (§9).
- Inflate or deflate a security finding's severity (§10).
- Use marketing voice, apologise, or use exclamation marks in interface
  copy (§11).
- Record a guess as a decision instead of as a guess, in an ADR, for the
  next person to re-examine (§12).

## Reflexes for this codebase

- Treat every source response, book file, network request, and IPC message as
  hostile. Size limit, shape check, timeout. Constitution §4.
- The Electron preload exposes an enumerated list of operations, never a
  general primitive. Validate arguments in the main process. §5.
- The host binds to loopback by default. Broader exposure — LAN or a
  user-operated remote deployment at their own domain — requires
  authentication plus one of ADR 0017's two fail-closed TLS conditions;
  phase 12 still builds authentication before phase 13 builds the bind/
  certificate surface, but the gate itself is the condition, not the
  phase number. Alexandryn operates no relay or tunnel on any user's
  behalf, under either mode. §6.
- Never log source credentials, session tokens, home-directory paths, or what
  someone is reading. §8.
- A handler serving data a user owns — reading progress, bookmarks,
  highlights, preferences, device list — resolves the authenticated user
  and active library from the request context and calls the
  user-and-library-scoped repository method. Never a bare-ID or bare-work
  variant. The scoped method existing is not the control; the handler
  calling it is. CI enforces this for the reading/reader surface
  (`scripts/check-user-scoped-reading.sh`). §3, §6.
  (Directive from review 0050 / audit 0012-C1 — a security audit certified
  this control from the repository layer without tracing a single request
  to its SQL.)
- Token verification on the authentication path always asserts the token
  *type*, not only the signature. A signature-valid token minted for a
  different purpose (an MFA ticket, a pairing enrolment grant) is a
  rejected token on the access path. Distinct token purposes use distinct
  HKDF signing subkeys. §6. (Directive from review 0050 / audit 0012-C2.)
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
- Certifying an authorization control (a security audit, a review) from the
  layer where the control *could* live — a scoped repository method, a
  migration adding a `user_id` column — instead of the wired call path.
  Trace handler → repository → SQL and read the predicate in the query.
  The happy-path test passing tells you nothing about cross-user access;
  only a test that attempts it does. (Audit 0012 missed a horizontal IDOR
  across the whole reading API this way — see review 0050.)

## Reporting back

At the end of a substantial task, report under these headings: **Changed**,
**Verified**, **Risks**, **Security**, **QA**, **Architecture**, **Next**.

State uncertainty plainly. If a test was skipped, say it was skipped.
Constitution §12.
