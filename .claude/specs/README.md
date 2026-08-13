# Specifications

One specification per coherent feature or subsystem. Start from
[`../templates/spec.md`](../templates/spec.md).

## Status lifecycle

```
DRAFT → REVIEWED → APPROVED → IMPLEMENTED → VERIFIED
```

| Status | Means |
|---|---|
| `DRAFT` | Being written. Nobody should build against it. |
| `REVIEWED` | A reviewer has been through it and recorded findings in `../reviews/`. |
| `APPROVED` | Findings resolved, maintainer signed off. **Implementation may begin.** |
| `IMPLEMENTED` | Code exists and tests pass. |
| `VERIFIED` | Audited, accessibility-checked, and confirmed against acceptance criteria. |

Status only moves forward. If implementation shows the spec was wrong, amend
the spec, note the change, and move it back through review deliberately —
don't let the code quietly become the specification.

**Nothing below `APPROVED` gets implemented.** Constitution §1.

## Naming

`<area>-<feature>.md`, kebab-case. Examples: `sources-opds-provider.md`,
`reader-progress-sync.md`, `import-metadata-matching.md`.

Group by area, not by phase — features outlive the phase that introduced them.

## Index

No specifications yet. The first ones arrive with Phase 01.

| Spec | Area | Phase | Status |
|---|---|---|---|
| — | | | |
