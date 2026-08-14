# Test plans

A test plan accompanies each specification and is written *before* the
implementation. Start from [`../templates/test-plan.md`](../templates/test-plan.md).

The plan's job is not to enumerate every test. It is to decide where confidence
needs to come from, so that effort lands on the parts most likely to be wrong
instead of spreading evenly over the parts that are easy to cover.

## Layers

| Layer | Runs against | Answers |
|---|---|---|
| Unit | Pure functions, no I/O | Is the logic right? |
| Integration | Real database, real handlers, real consumers | Do the parts fit? |
| Contract | Shared schemas between frontend, backend, host, and queue | Do both sides still agree? |
| End to end | The assembled product | Can a person actually do this? |
| Accessibility | Rendered interface | Can a person do it without a mouse or without sight? |

Push tests down to the cheapest layer that can honestly answer the question. An
end-to-end test that exists because a unit test was awkward to write is a slow,
flaky unit test.

## Non-negotiables

**Tests must be observed failing first.** A test that has never failed has not
been shown to test anything. Constitution §2.

**Fixtures are deterministic.** No live network, no ambient clock, no unseeded
randomness. A suite that fails one run in twenty trains people to re-run it
instead of reading it.

**Never use real data as a fixture.** No real credentials, no real user's
library, no copyrighted book. Synthetic EPUBs and recorded source responses
only.

**Adversarial cases are part of the plan, not an extra.** Malformed, empty,
enormous, duplicated, slow, never-arriving, and deliberately hostile input each
get a case. Constitution §10.

## Index

| Plan | Spec | Status |
|---|---|---|
| [`backend-service-lifecycle.md`](backend-service-lifecycle.md) | [`backend-service-lifecycle.md`](../specs/backend-service-lifecycle.md) | `REVIEWED` (independent, findings fixed — [`0023`](../reviews/0023-test-plan-backend-service-lifecycle.md)) |
| [`backend-configuration.md`](backend-configuration.md) | [`backend-configuration.md`](../specs/backend-configuration.md) | `REVIEWED` (independent, findings fixed — [`0024`](../reviews/0024-test-plan-backend-configuration.md)) |
| [`backend-errors-and-logging.md`](backend-errors-and-logging.md) | [`backend-errors-and-logging.md`](../specs/backend-errors-and-logging.md) | `REVIEWED` (independent, findings fixed — [`0026`](../reviews/0026-test-plan-backend-errors-and-logging.md)) |
| [`backend-http-transport.md`](backend-http-transport.md) | [`backend-http-transport.md`](../specs/backend-http-transport.md) | `REVIEWED` (independent, findings fixed — [`0027`](../reviews/0027-test-plan-backend-http-transport.md)) |
