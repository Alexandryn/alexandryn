# Roadmap

Nineteen phases, ordered by dependency rather than by excitement. Each one has
its own directory with an objective, scope, risks, test strategy, security
considerations, and exit criteria.

A phase closes when its exit criteria are met — not when the happy path works.

## How to read this

**Detail decreases with distance — but "distance" means undecided
dependencies, not a fixed phase number.** Phases 00–11 are now specified in
full: each has approved specs, not an outline. Phases 12–17 and 99 are
still outlines, and say so at the top, because their dependencies (
authentication, network exposure, the reader) haven't closed yet. Writing
detailed requirements for phase 14 before phase 12/13 exist produces
confident fiction that later gets treated as a decision — the same
reasoning that applied to phase 06 before phase 03/04 closed, and no
longer applies to it now that they have.

Each outline is expanded into a full phase document when its dependencies
close, at which point this section's phase-count above stops being
accurate for it — check the phase's own status line, not this paragraph,
for what's currently true.

## The phases

| #                              | Phase                | Delivers                                                                | Status                                                                                    |
| ------------------------------ | -------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| [00](00-foundation/)           | Foundation           | Repository, engineering memory, CI skeleton                             | **In progress**                                                                           |
| [01](01-architecture/)         | Architecture         | System, frontend, backend, host, persistence and messaging design       | In progress                                                                               |
| [02](02-domain/)               | Domain               | The model: works, editions, files, sources, progress                    | In progress                                                                               |
| [03](03-backend-foundation/)   | Backend foundation   | Go service skeleton, config, logging, errors, migrations, health        | Complete — all exit criteria met, maintainer approval 2026-08-26                          |
| [04](04-frontend-foundation/)  | Frontend foundation  | React shell, design tokens, component library, routing, data layer      | Implementation complete; in closure — Checkpoint P4-G (final maintainer approval) pending |
| [05](05-desktop-host/)         | Desktop host         | Electron shell, IPC boundary, lifecycle, loopback serving               | Specs approved, implementation not started                                                |
| [06](06-library/)              | Library              | Browse, collections, search, filter, sort — the first real slice        | Specs approved, implementation not started                                                |
| [07](07-metadata/)             | Metadata             | Open Library adapter, normalisation, caching, Discover                  | Specs approved, implementation not started                                                |
| [08](08-sources/)              | Sources              | Source abstraction, capabilities, first provider                        | Specs approved, implementation not started                                                |
| [09](09-async-jobs/)           | Async jobs           | Background work where it is actually justified                          | Specs approved, implementation not started                                                |
| [10](10-import/)               | Import               | Discovery → extraction → matching → confirmation → persistence          | Specs approved, implementation not started                                                |
| [11](11-reader/)               | Reader               | Reading, position, preferences                                          | Specs approved, implementation not started                                                |
| [12](12-authentication/)       | Authentication       | Accounts, sessions, authorisation                                       | Implemented on `feat/phase12-auth`; **not closed** — 2 High authz defects found by review 0050, audit 0012 superseded                |
| [13](13-network-access/)       | Network access       | LAN exposure, binding, pairing, transport security                      | Closed — merged via PR #80, 2026-09-05                                                    |
| [14](14-devices-and-sync/)     | Devices and sync     | Device management, progress across devices                              | Closed — implementation, verification, and audit 0014 complete                            |
| [15](15-observability/)        | Observability        | Metrics, queue visibility, diagnostics, Activity                        | Closed — implementation, verification, and audit 0015 complete                            |
| [16](16-security-hardening/)   | Security hardening   | Threat model consolidation, external-review readiness                   | Closed — whole-app adversarial sweep, audit 0016, 219 issues filed and resolved, maintainer approval 2026-09-11 |
| [17](17-accessibility-and-qa/) | Accessibility and QA | Conformance, the full test matrix, regression suite                     | In progress — Gate 0 scope drafted 2026-09-11, awaiting maintainer approval |
| [99](99-release/)              | Release              | Packaging, versioning, release process, `docs`/`website` repos stood up | Not started                                                                               |

## Dependency graph

```mermaid
graph TD
    P00[00 Foundation] --> P01[01 Architecture]
    P01 --> P02[02 Domain]
    P02 --> P03[03 Backend foundation]
    P01 --> P04[04 Frontend foundation]
    P03 --> P05[05 Desktop host]
    P04 --> P05
    P03 --> P06[06 Library]
    P04 --> P06
    P06 --> P07[07 Metadata]
    P06 --> P08[08 Sources]
    P08 --> P09[09 Async jobs]
    P07 --> P10[10 Import]
    P08 --> P10
    P09 --> P10
    P10 --> P11[11 Reader]
    P06 --> P11
    P05 --> P12[12 Authentication]
    P12 --> P13[13 Network access]
    P13 --> P14[14 Devices and sync]
    P11 --> P14
    P09 --> P15[15 Observability]
    P13 --> P15
    P14 --> P16[16 Security hardening]
    P15 --> P16
    P16 --> P17[17 Accessibility and QA]
    P17 --> P99[99 Release]
```

## Why the order is what it is

**Authentication comes before network exposure.** This is the ordering decision
that matters most. The host binds to loopback from phase 05 and stays there
until phase 12 gives it a credential system. There is no intermediate state
where the library is reachable from another machine without a login — not even
a temporary one behind a "dev only" flag. Constitution §6.

**The domain is settled before the backend is built.** Persistence shaped by
whatever the first API endpoint needed is how a schema ends up describing the
UI instead of the subject matter. Phase 02 produces the model; phase 03
implements against it.

**Metadata, sources, and library are three phases, not one.** They are the
three boundaries the constitution refuses to merge (§3), and building them
separately is what keeps them separate. Phase 06 delivers a working library
with no external integration at all — which proves the domain stands on its
own.

**Async infrastructure arrives when async work does.** RabbitMQ is not part of
the backend foundation. Phase 09 sits immediately before import and source
synchronisation because that is the first point where background processing is
genuinely warranted. If phase 09's own analysis concludes a simpler mechanism
suffices, that is a legitimate outcome and gets recorded as an ADR.

**Reading comes after import.** A reader with nothing to read cannot be tested
against anything real, and EPUB rendering is where hostile content is most
dangerous — it deserves a real corpus of files to defend against.

## Deviations from the original plan

The master brief proposed a phase list. This one differs in six places, each
deliberately:

| Change                                                              | Reason                                                                                                                                         |
| ------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Authentication moved _before_ network exposure                      | The original order would have produced a build serving an unauthenticated library to the LAN. Not acceptable even transiently.                 |
| Observability split: baseline into phase 03, maturity into phase 15 | Structured logging and health checks are part of writing a server, not a later project. Phase 15 covers metrics, queue depth, and diagnostics. |
| Security is a gate on every phase, not one phase                    | Phase 16 is consolidation and external-review readiness. A single security phase invites deferral.                                             |
| Async jobs became its own phase, placed at first need               | Avoids assuming RabbitMQ is warranted before anything needs it.                                                                                |
| "Open Library" renamed to "Metadata"                                | A phase named after a vendor becomes a phase coupled to that vendor.                                                                           |
| Accessibility and QA consolidation added as phase 17                | Per-phase accessibility work still leaves conformance testing and the cross-device matrix as real, separate work.                              |

## Working a phase

```
Discover → Spec → Review → Test plan → RED → Implement → GREEN
        → Refactor → QA → Security audit → Document → Close
```

Two points stop for the maintainer:

1. **Before each spec is drafted** — state its scope and the key decisions
   it will make, and wait. Once approved, drafting, self-review, and
   `APPROVED` status all proceed without a second stop for the same
   content.
2. **After the security audit** — before the phase is marked closed.

Between those, work proceeds without check-ins. An automated contributor must
not cross either gate on its own judgement.

## Status vocabulary

| Status      | Means                                                             |
| ----------- | ----------------------------------------------------------------- |
| Not started | No work has begun.                                                |
| In progress | Specs or implementation underway.                                 |
| Blocked     | Waiting on a dependency or a decision. The phase file says which. |
| Closed      | Exit criteria met, audit clear, maintainer signed off.            |

"Maintainer approval recorded" (an exit criterion on every phase) means the
maintainer fills in the phase file's own **Closed** date, in the header table
at the top of the phase document — not a separate file. `reviews/` is for
specs, ADRs, and boundary-crossing changes; a phase's own closure doesn't get
a second, redundant record. If a phase closes with unresolved disagreement
worth remembering, a line under its **Why here** or a new **Closure notes**
section says so — the date alone isn't enough when the sign-off wasn't
unanimous.
