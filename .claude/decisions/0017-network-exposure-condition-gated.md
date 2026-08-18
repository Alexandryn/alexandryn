# 0017. Network exposure is gated on TLS and authentication being verifiably true, not on a phase number or deployment target

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-18 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

<!-- Status: Proposed | Accepted | Rejected | Superseded | Deprecated -->

## Context

Constitution §6, `architecture-system.md` FR-3, `backend-configuration.md`
FR-8, `roadmap/13-network-access/README.md`'s Scope Out, and
`deployment-container-packaging.md` FR-5/FR-6 all gate non-loopback network
exposure the same way: on a phase number ("until phase 12/13") or, for the
container target, on that same phase number restated. None of them state a
runtime condition — they state a point in the roadmap's own history.

A 2026-08-17/18 review of user-operated remote deployment (the user's own
instance, at their own domain, reachable beyond the LAN they run it on —
distinct from and explicitly not an Alexandryn-operated relay or tunnel,
which stays declined) found this framing doesn't fit that case at all: a
remote deployment can have both authentication and TLS fully in place and
still be illegal under every one of the citations above, purely because no
phase number says otherwise yet. The gate was never really "is this safe,"
it was "did phase 12/13 ship" — a proxy for safety that stops being a good
proxy the moment a deployment shape phase 12/13 wasn't written for
[roadmap/13-network-access/README.md]'s Scope Out actually excluded arrives on its own,
correctly configured, terms.

A second problem surfaced turning this into a real condition: "reachable
only over TLS" is not something the Go process can verify in-process when
TLS terminates at a reverse proxy in front of it — the standard shape of a
self-hosted deployment (Caddy, nginx, Traefik terminating TLS, proxying
plaintext HTTP to the app on a private address). A naive "TLS present"
check has no way to see a termination point outside its own process.

## Decision

A `BIND_ADDRESS` is legal for the Go server to bind to if and only if
authentication is enforced (unconditionally, both branches below) **and**
one of two mutually exclusive conditions holds:

- **Mode A — in-process TLS.** `BIND_ADDRESS` resolves to a publicly
  routable address, and the server itself terminates TLS with a certificate
  that is loaded and validated (present, parseable, not expired, key
  matches certificate) at startup. An invalid or missing certificate while
  bound to a public address fails startup — the same unconditional,
  no-partial-state failure `backend-configuration.md` FR-6 already applies
  to every other required-key case.
- **Mode B — upstream TLS.** `BIND_ADDRESS` resolves to a loopback or
  private-range address only (`127.0.0.0/8`, `::1`, RFC 1918 IPv4 private
  ranges, or an IPv6 unique local address) — never a publicly routable one.
  TLS, if present, terminates outside the process entirely; the server's
  own guarantee is narrower and different in kind: it is never itself
  directly reachable from a public address, regardless of what sits in
  front of it.

Both modes fail closed at bind time, not on a config flag naming which mode
is "intended." The check that decides which mode applies is the same kind
of check FR-8 already runs today (inspect `BIND_ADDRESS`'s resolved host)
— widened from "must be loopback" to "must be loopback/private, or public
with a validated cert," never weakened to "trust whatever the operator
says this is."

This replaces every phase-number or deployment-target gate named in
Context with this one condition, checked identically regardless of which
phase shipped the code or which deployment target (Electron-hosted,
container-hosted, container-hosted-remote) is running. What's still true,
unchanged: authentication itself is phase 12's to build, and the
`BIND_ADDRESS`/certificate/reverse-proxy configuration surface is phase
13's — this ADR fixes the *rule* the eventual code enforces, not who
builds which part of it or when.

**Not decided here, explicitly**: Alexandryn operating any relay, tunnel,
or traffic-mediating infrastructure on a user's behalf. That stays
declined, unrelated to this decision — this ADR is entirely about a user's
own instance, at an address and certificate the user controls, mediated by
nothing Alexandryn runs.

## Options considered

### Option A — TLS-present-in-process only, no upstream-TLS mode (rejected)

*For* — simplest possible check: one boolean, "does this process hold a
valid certificate."

*Against* — rejected because it makes the standard self-hosted deployment
shape (reverse proxy terminating TLS) illegal by construction, which isn't
a safety property, it's an accident of how the check was written. The
actual property that matters — this process is never reachable, over
plaintext, from outside a trust boundary the operator controls — is fully
satisfiable by Mode B without the process ever touching a certificate
itself.

### Option B — Trust a config flag ("remote mode: on") instead of inspecting the bind address (rejected)

*For* — less code; one flag instead of parsing and classifying an address.

*Against* — rejected for the same reason `deployment-container-packaging.md`'s
Security considerations already rejected the equivalent shape for the
container target: a flag is something that can be wrong, forgotten, or
left on by accident. Inspecting the actual resolved bind address is a
property of the network topology itself, not a claim about it — it fails
closed the same way the existing loopback-only check does, a flag would
not.

### Option C — Keep the phase-number gate, treat remote deployment as a phase-13 sub-case only (rejected)

*For* — no constitution change, smallest diff.

*Against* — rejected because it doesn't actually describe anything —
"phase 13 shipped" is not a property of a running deployment, it's a
property of this repository's own history. A correctly configured,
TLS-and-auth-verified remote deployment running today under a
hypothetical phase-13 codebase would still have to satisfy a real
condition to be safe; naming the phase instead of the condition just
hides where that condition actually needs to be checked.

## Consequences

**Good** — the rule that decides whether a bind is legal is now a property
of the running system (address + certificate + auth state), checked the
same way regardless of deployment target, and preserves the fail-closed
guarantee the loopback-only check already had rather than trading it for a
flag. User-operated remote deployment becomes expressible without waiting
on a phase number that was never actually the safety mechanism.

**Bad** — more validation logic than a single loopback check: address
classification (public vs. private vs. loopback) and, in Mode A,
certificate loading/validation at startup, both new failure modes that
need their own clear startup error messages (constitution §11). The
container CI guard (`deployment-container-packaging.md` FR-6) becomes a
conditional check instead of a blanket ban, which is a harder static
property to verify than "does this file contain the string `ports:`."

**Neutral** — the phase-12-before-phase-13 build order is unaffected;
authentication still ships first, this ADR only changes what gates the
bind at runtime once both exist.

## Reversal cost

Medium. Unlike ADR 0003's addendum, this changes a validated startup
behavior (`backend-configuration.md` FR-8) and a CI guard
(`deployment-container-packaging.md` FR-6), not just a documentation
convention — reversing it means re-tightening a check that real deployments
may already depend on passing. Low until phase 12/13 actually implement
against it; not low after.

## Confidence

Medium-high. The two-mode split directly answers the reverse-proxy gap
named in Context and preserves fail-closed behavior in both branches,
which is the property that mattered most. Lower confidence on the exact
certificate-validation depth Mode A needs at startup (expiry only, or also
hostname/SAN matching against a configured domain) — left to phase 13's
own implementation, not fixed here.
