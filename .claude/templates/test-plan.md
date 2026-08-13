# Test plan: <feature or phase>

| | |
|---|---|
| **Spec** | `.claude/specs/…` |
| **Status** | `DRAFT` |
| **Created** | YYYY-MM-DD |

## What we are trying to be confident about

The handful of statements this plan exists to establish. Not a list of tests —
a list of beliefs the tests earn.

## Risk assessment

Where is this feature most likely to be wrong? Concentrate effort there rather
than spreading coverage evenly. Name the parts that are hard to get right and
the parts that are merely tedious.

## Layers

### Unit

Pure logic, no I/O. Table-driven where the input space is enumerable.

### Integration

Real boundaries — database, HTTP handlers, message consumers. What is stubbed,
and why stubbing it is still honest.

### Contract

Where the frontend, backend, desktop host and message consumers agree on a
shape. Which side owns the schema, and how a breaking change is caught.

### End to end

The user journeys that must never break. Keep this set small and load-bearing.

### Accessibility

Keyboard traversal, accessible names, focus order, announcements, reduced
motion, contrast. Automated checks plus the manual passes that automation can't
do.

## Adversarial cases

The hostile inputs this feature must survive. Malformed, empty, enormous,
duplicated, slow, never-arriving, and deliberately malicious. Constitution §10.

| Input | Expected behaviour |
|---|---|
| | |

## Fixtures and test data

What data is needed, where it lives, and how it's generated. Fixtures must be
deterministic — no network, no clock, no random unless seeded.

Never use a real user's library, a real credential, or a copyrighted book as a
fixture.

## What is deliberately not tested

And why. An honest gap that's written down is manageable; an unwritten one is a
surprise later.

## Exit criteria

- [ ] Every functional requirement in the spec maps to at least one test
- [ ] Every adversarial case above has a test
- [ ] Tests were observed to fail before the implementation existed
- [ ] The suite is deterministic across repeated runs
