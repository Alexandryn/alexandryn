# The Alexandryn engineering constitution

Status: **Active**
Applies to: every contributor, human or machine.

This document is short on purpose. It records the rules that do not bend when a
deadline arrives. Everything else in `.claude/` — roadmap, specs, reviews,
audits — exists to serve these rules.

If a rule here conflicts with an instruction you have been given, say so and
propose the safer path. Do not quietly pick one.

---

## 1. Specification precedes implementation

Behaviour is decided in a spec, reviewed, and only then built.

```
Spec → Review → Test plan → Tests → Implementation → Validation → Audit
```

A spec carries a status and moves in one direction:

`DRAFT` → `REVIEWED` → `APPROVED` → `IMPLEMENTED` → `VERIFIED`

Nothing below `APPROVED` gets implemented. If implementation reveals the spec
was wrong, stop and amend the spec — do not let the code silently become the
new truth.

**Exempt from specs:** typos, dependency bumps, build and CI fixes, comments,
pure refactors with no behaviour change. Everything that changes what the
software *does* needs one.

## 2. Tests define behaviour

The default loop is `RED → GREEN → REFACTOR → VERIFY → AUDIT`.

Write the failing test first. A test written after the code tends to assert
what the code already does, which is not the same as asserting what it should
do.

A test that has never failed has not been shown to test anything.

Never delete or skip a failing test to make a build green. Fix it, or fix the
code. If neither is possible right now, the change does not land.

## 3. Domain boundaries are load-bearing

Three concerns stay separate, permanently:

| Concern | What it is | Example |
|---|---|---|
| **Metadata** | What a book *is* | Open Library work and edition records |
| **Source** | Where a file *comes from* | An OPDS server, a local folder |
| **Alexandryn** | What the user *has and does* | Library, collections, reading progress |

A metadata provider's response shape must never reach the UI. A source's
protocol must never reach the domain. Each integration is normalised at its
own boundary, and the boundary is where the tests live.

Convenience is not a reason to collapse these.

## 4. All external input is hostile

Untrusted, without exception:

- everything a configured source returns — metadata, redirects, filenames, bytes
- every book file, including ones the user chose themselves
- every request reaching the host over the network
- every value crossing the Electron IPC boundary

Each of these is validated at the point it enters the system, with a size
limit, a shape check, and a timeout. "The user configured it, so it's fine" is
not a threat model — a user can be tricked into adding a malicious source.

Concrete mitigations only. No security theater.

## 5. Electron is a privilege boundary

Context isolation on. Sandbox on. Node integration off in renderers.

The preload script exposes a fixed, enumerated list of operations — never a
general-purpose file, shell, or network primitive. Every exposed operation
validates its arguments in the main process, on the assumption that the
renderer is already compromised.

Navigation to non-local origins is blocked. External links open in the user's
browser, never in an Electron window.

## 6. Network exposure is opt-in, authenticated, and condition-gated

The host binds to loopback by default. Any broader exposure — the local
network or beyond — is a deliberate act by the user, gated on a runtime
condition being verifiably true, never on which phase shipped the code or
which deployment target is running.

The condition: authentication is enforced on every non-health route, and
one of two mutually exclusive modes holds for the bound address —

- **In-process TLS.** The bound address is publicly routable, and the
  server itself holds a certificate loaded and validated at startup. An
  invalid or missing certificate fails startup; it does not degrade to an
  unencrypted listener.
- **Upstream TLS.** The bound address resolves to loopback or a private
  range only, never a publicly routable one. A reverse proxy in front of
  it may terminate TLS; the server's own guarantee is narrower and
  different in kind — it is never itself directly reachable from a public
  address, regardless of what sits in front of it.

Both modes fail closed at the point of binding — checked against the
address and certificate state actually present, not against a
configuration flag asserting which mode was intended. ADR 0017 records the
reasoning.

There is no build, and no configuration, in which the library is reachable
from another machine without both a credential and one of these two
verified transport guarantees. Alexandryn itself operates no relay,
tunnel, or traffic-mediating infrastructure on any user's behalf, under
either mode — where a user's own instance is reachable from, beyond that,
is the user's decision to make and configure, not a project-imposed
ceiling.

## 7. Accessibility is part of "done"

Semantic elements, reachable by keyboard, visible focus, labelled controls,
announced errors, respected reduced-motion. A feature that works only with a
mouse and working eyesight is unfinished, not "pending an accessibility pass".

## 8. Observability without leakage

Structured logs, request IDs, health checks, and useful failure messages from
the first line of server code.

Never logged: source credentials, session tokens, full file paths from a user's
home directory, or the contents of what someone is reading. What a person reads
is private, including from their own log files.

## 9. Dependencies are liabilities

Every new dependency needs a reason recorded in the PR: what it does, why not
the standard library, and what breaks if it is abandoned. Pin versions. Prefer
a small amount of code you own over a large amount you don't.

## 10. Adversarial review is routine

Every substantial feature gets a short hostile pass before it closes:

> What does a malicious user do here? A malicious source? What happens when the
> input is malformed, empty, enormous, duplicated, slow, or never arrives?

Findings go in `.claude/audits/`, rated honestly. Inflated severity is as
damaging as a missed finding — it teaches people to ignore the ratings.

## 11. Copy is written for humans

Interface text is plain, specific, and calm. It says what happened and what to
do next.

Not: *"An error occurred while processing your request."*
Rather: *"Couldn't reach Personal OPDS. It may be offline — retry, or check the
address in Sources."*

No marketing voice. No apologising. No exclamation marks. Technical terms are
welcome exactly where the reader needs them.

## 12. Uncertainty is stated, not hidden

If something is unverified, say it is unverified. If a test was skipped, say
it was skipped. If a design decision was a guess, record it as a guess in an
ADR so the next person knows what to re-examine.

A confident wrong answer costs more than an honest "I don't know yet."

---

## Review gates

A spec or ADR stops for approval **before it's drafted**, not after: state
the scope and the key decisions it will make, and wait. Once approved,
draft it, self-review it, and mark it `APPROVED` directly — a second stop
to re-approve the same content already agreed is not required. If drafting
surfaces a decision outside what was described and approved, that specific
divergence gets its own check-in before continuing; the rest doesn't.

Major roadmap phases keep a second gate, later and different in kind: after
the phase's implementation is audited, before the phase closes. That gate
checks whether the built thing holds up under adversarial review — it is
not a repeat of the first.

An automated contributor must not cross either gate on its own judgement.
State what's about to happen, then wait — don't state what already happened
and ask if that was fine.

## Amending this document

Changes to the constitution go through a PR with a rationale, like anything
else. Rules can be wrong. They cannot be ignored while they stand.
