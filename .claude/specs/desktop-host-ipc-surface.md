# Spec: Desktop host IPC surface

| | |
|---|---|
| **Status** | `APPROVED` (amendment pending — `source.pickLocalFolder` operation proposed for phase 08, under cross-spec review as part of that phase's batch; not yet maintainer-reconfirmed) |
| **Phase** | `05-desktop-host` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-15 |
| **Supersedes** | — |
| **Reviewed in** | [`0032`](../reviews/0032-spec-amendments-phase05-cross-phase-findings.md) (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14. `source.pickLocalFolder` (FR-6) proposed post-approval as part of phase 08's batch, following this spec's own Non-goals-anticipated extension point ("any domain-feature operation... arrives with its own feature phase") — pending phase 08's cross-spec review and maintainer re-confirmation, not yet granted |

## Context

`architecture-desktop-host.md` FR-1/FR-2 already fixed the pattern: one
namespaced object (`window.alexandryn`) via `contextBridge`, every method
backed by a main-process handler that validates its own arguments,
`contextIsolation`/`sandbox` on, `nodeIntegration` off, no exception. That
spec explicitly left the concrete method list to each feature phase
(Non-goals) and left open, for phase 05 specifically, whether the
surface is generated from a schema or hand-maintained, and how a new
operation gets reviewed before it's addable at all
(`roadmap/05-desktop-host/README.md`'s own "Architecture decisions
expected"). Phase 05 itself needs no domain-feature operations — those
arrive with phase 06 onward — but the *mechanism* has to exist and be
provably enforced now, not deferred until the first real operation shows
up under feature-delivery pressure.

## Problem

Nothing has fixed: how the enumerated list is authored and kept in sync
between preload and main (hand-maintained vs. generated), what a new
operation's review checklist actually requires, the validation library
used to check argument shape, or a concrete worked example proving the
whole pattern end to end — `architecture-desktop-host.md`'s own pattern
is a shape, not yet a thing anyone can point at and say "like that one."

## Goals

- Fix hand-maintained vs. schema-generated, justified under constitution
  §9
- Fix the validation library and where validation actually runs (main
  process only, never trusted from preload/renderer)
- Fix the addition process: what a PR adding a new IPC operation must
  include before it's mergeable
- Ship one concrete, minimal operation as a worked example, so the
  pattern is proven against something real rather than purely
  hypothetical

## Non-goals

- Any domain-feature operation (library, sources, reader, etc.) — each
  arrives with its own feature phase, per `architecture-desktop-host.md`'s
  own Non-goals
- The main-process spawn/lifecycle logic the example operation might
  read from — `desktop-host-process-model.md`
- Window/rendering concerns — `desktop-host-window-and-serving.md`

## User stories

- As **a contributor adding a new IPC operation in phase 06+**, I want a
  fixed template to follow (type definition, main-process handler,
  validation schema, test) so adding one is mechanical, not a design
  decision each time.
- As **a security reviewer**, I want a single file listing every
  IPC operation that exists, so auditing the preload surface means
  reading one file, not grepping the codebase for `ipcMain.handle` calls.
- As **a renderer developer**, I want every operation's argument and
  return types checked by TypeScript at the call site, so a shape
  mismatch is a compile error, not a runtime surprise.

## Functional requirements

- **FR-1** The enumerated surface is **hand-maintained**, not generated
  from a schema — justified under constitution §9: phase 05's own
  surface is a handful of operations (this spec ships one), schema
  generation (e.g. from an OpenAPI-shaped IPC definition) would be
  tooling investment disproportionate to the current surface size, and
  a hand-maintained list is easier to review line-by-line in a PR, which
  is exactly the property this spec's FR-3 review process depends on.
  Revisit if the operation count grows enough that hand-maintenance
  itself becomes the bottleneck (Open questions).
- **FR-2** Every operation is declared in exactly one file,
  `electron/src/preload/operations.ts` — a single source of truth
  listing each operation's name, argument type, return type, and the
  **Zod** schema (constitution §9: small, well-maintained, TypeScript-
  native — the standard choice for exactly this "validate an untrusted
  shape at a boundary" problem, replacing what would otherwise be
  hand-rolled type guards duplicated per operation; if Zod were
  abandoned, exit cost is low — schemas are declarative TypeScript
  objects with no runtime coupling elsewhere in the codebase beyond the
  `.parse()`/`.safeParse()` call sites this same file's generated
  handlers make, so a replacement validator would mean rewriting schema
  declarations in this one file and regenerating from it, not touching
  every operation's business logic) that validates the argument shape
  at runtime in the main process. **Mechanism, made concrete rather than
  left as "generated":** this is **runtime iteration**, not a build-time
  codegen step — `operations.ts` exports a plain array of operation
  descriptors (`{ name, schema, handler }`), and both the preload
  script's `contextBridge.exposeInMainWorld` call and the main process's
  `ipcMain.handle` registrations are produced by iterating that same
  array directly at startup (`operations.forEach(op => ipcMain.handle(...))`,
  correspondingly on the preload side) — no separate build tool, no
  generated output file to keep in sync, no code-generation step in the
  build pipeline at all. This is a smaller, simpler mechanism than the
  "lightweight code-generation" an earlier draft of this spec described,
  which left genuine ambiguity between two materially different
  implementations (a real build-time codegen step vs. this runtime
  approach) — a cross-phase review caught the ambiguity; runtime
  iteration is simpler and sufficient at this surface's current size,
  and is what this spec now commits to.
- **FR-3** A new operation is only mergeable with, in the same PR: (1) an
  entry in `operations.ts` (FR-2) with a Zod schema covering every field,
  including explicit bounds on any numeric/string field (constitution
  §4 — no unbounded string accepted without a documented reason); (2) a
  main-process handler that does nothing before validation succeeds —
  the schema check is the handler's first line, a rejected shape never
  reaches business logic; (3) a unit test exercising both a valid and a
  deliberately invalid argument shape; (4) an explicit reviewer
  acknowledgment in the PR description of what the new operation can do
  and why the renderer needs it — a manual checklist item, not
  automatable, but named here so it's a defined expectation rather than
  an assumption about what "review" already covers.
- **FR-4** No operation's handler accepts a free-form string used as a
  dynamic dispatch key, file path, or command — constitution §4's
  hostile-input discipline applied to this boundary specifically: an
  operation like `readFile(path: string)` that lets the renderer name an
  arbitrary path is exactly the "general primitive" constitution §5
  prohibits, even if scoped through IPC rather than a raw Node API. Any
  future operation needing filesystem access takes a *purpose-typed*
  argument (e.g. `exportLibraryBackup()`, no path argument at all — the
  main process decides the path) rather than a path-accepting one.
- **FR-5** The one operation this phase ships,
  `system.getAppVersion(): Promise<string>` — returns the packaged
  app's version string (from `package.json`, read once at main-process
  startup, never from a renderer-supplied value). Chosen specifically
  because it has no security-relevant surface (no argument to validate
  beyond "takes nothing," return value is non-sensitive, no filesystem/
  network access) — it exists purely to prove FR-2/FR-3's mechanism
  works end to end with something real, not to be a meaningfully useful
  feature.
- **FR-6** `source.pickLocalFolder(): Promise<{ path: string } | null>` —
  the first domain-feature operation added since this spec shipped,
  following the extension point this spec's own Non-goals section
  already reserved ("any domain-feature operation... arrives with its
  own feature phase"). Added for `frontend-source-management.md`
  (phase 08): opens the OS's native folder-selection dialog
  (`dialog.showOpenDirectory` in the main process) and returns the
  chosen absolute path, or `null` if the user cancels. Takes **no
  argument** — this is the concrete reason FR-6 doesn't violate FR-4's
  "no operation accepts a free-form string used as... a file path" rule:
  the renderer never supplies a path here, it only ever *receives* one
  the OS dialog UI produced from the user's own selection; the main
  process performs no filesystem access with this path itself (it
  neither reads nor writes anything at the returned path) — the
  renderer forwards the chosen path to the Go backend's own
  `POST /api/v1/sources` over the normal loopback HTTP API
  (`backend-source-adapter.md`), where it is validated as a real,
  readable directory before anything trusts it, the same hostile-input
  discipline every path this project handles gets. This operation is
  only ever meaningful when the renderer is running inside Electron —
  `frontend-source-management.md`'s own concern is detecting that and
  falling back to a plain text field otherwise, not this spec's.

## Non-functional requirements

- **Performance** — not applicable; IPC round-trips at this scale
  (single-digit operations, all synchronous logic) carry no measurable
  budget.
- **Security** — see Security considerations below; this entire spec is
  substantially a security spec, restating constitution §5's privilege
  boundary as enforceable structure.
- **Accessibility** — not applicable; no UI.
- **Reliability** — FR-3's mandatory-test requirement is what keeps a
  future operation's validation from silently regressing as the
  codebase grows.
- **Observability** — every rejected (invalid-shape) IPC call is logged
  by the main-process handler before rejecting, naming the operation and
  what was wrong with the shape (constitution §11's bar applied to an
  internal log line, not just user-facing copy) — never a silent reject
  a renderer bug could hide behind.

## Domain model

Not applicable — IPC mechanism, not the Alexandryn domain.

## API and contracts

- **Renderer ↔ preload**: `window.alexandryn.<namespace>.<method>(args):
  Promise<Result>` — TypeScript types generated from `operations.ts`
  (FR-2), so a call-site shape mismatch is a compile error.
- **Preload ↔ main**: `ipcRenderer.invoke`/`ipcMain.handle`, one channel
  per declared operation (never one shared channel with an operation-name
  argument — that would be exactly the "wildcard passthrough" constitution
  §5 and this spec's FR-1/FR-2 both exist to prevent).
- **Main ↔ Zod schema (FR-2)**: every handler's first statement is
  `schema.parse(args)` (or `.safeParse`, rejecting cleanly rather than
  throwing into an unhandled path) — never business logic reached before
  this line runs.

## State transitions

Not applicable — each IPC call is a stateless request/response; no
operation in this phase's scope has multi-step state.

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Renderer calls an operation with a malformed/oversized argument | FR-2's Zod schema, checked first in the handler | The `Promise` rejects with a generic error (never a raw validation-library stack trace, constitution §11) | Logged server-side (Observability), request never reaches business logic |
| Renderer attempts to invoke a channel that doesn't correspond to any declared operation | Electron's own `ipcMain` — no handler registered for an undeclared channel | The `invoke` call rejects (Electron's default behavior for no registered handler) | No custom handling needed; this is the mechanism's own structural guarantee — nothing to add a handler for could exist outside `operations.ts` |
| A future contributor adds an operation without following FR-3's checklist | Code review; a CI check can verify test-file existence (item 3) and Zod-schema presence (item 1) mechanically, but not items 2/4 | N/A — caught before merge, not a runtime failure | PR blocked at review, not a defect that ships |
| `source.pickLocalFolder` (FR-6) called from a LAN browser tab with no `window.alexandryn` global | Renderer's own capability check (`frontend-source-management.md`'s concern) never calls the operation in the first place | The text-field fallback is shown instead — no error, since the operation was never invoked | N/A — this spec has no defined behavior for the impossible case of a non-Electron caller reaching `window.alexandryn`, since that global doesn't exist there |
| User cancels the native folder dialog | `dialog.showOpenDirectory`'s own cancel result | The renderer's text field stays as it was, no error shown | Returns `null`, not an error — cancellation is a normal outcome, not a failure |

## Security considerations

- **Enumerated surface, never a wildcard (FR-1/FR-2)** — constitution
  §5's central requirement, made structurally true: a wildcard channel
  is not just discouraged, it's a different, larger amount of code to
  add (a new `ipcMain.handle` registration outside the generated set)
  that a reviewer would immediately notice as inconsistent with every
  other operation's shape.
- **No general-purpose primitive (FR-4)** — the concrete rule closing
  the exact gap phase 05's own risk table names ("preload surface grows
  into a general-purpose bridge... defeats the entire sandboxing model").
- **Validation runs only in main, never trusted from preload/renderer
  (FR-2/FR-3)** — restates constitution §5's "renderer assumed
  compromised" as the reason validation can't live in preload code,
  which executes in a context a sufficiently compromised renderer could
  still influence indirectly (though `contextIsolation` limits this,
  defense in depth means the authoritative check is where the renderer
  categorically cannot reach it: the separate main process).
- **`system.getAppVersion` (FR-5) deliberately chosen to carry zero
  security-relevant surface** — the first real example is a case where
  getting the mechanism wrong has no real consequence, letting the
  pattern be proven before a higher-stakes operation (a future file
  dialog, an export operation) uses the same mechanism.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Every declared operation's Zod schema (FR-2), valid and invalid cases; `system.getAppVersion` (FR-5) end to end against a mocked `package.json` |
| Integration | Full preload → main round trip via Electron's own test harness (real `contextBridge`, real `ipcMain`), proving `contextIsolation`/`sandbox` don't block the legitimate path while still blocking everything FR-4 forbids |
| Security | A test asserting no `ipcMain.handle` registration exists outside the set generated from `operations.ts` — a structural check, the same category other phase 03/05 specs use for their own no-globals/no-wildcard rules; a hostile-argument test per declared operation (oversized string, wrong type, extra fields) |

## Acceptance criteria

- [ ] Every IPC operation is declared in `operations.ts`, with no
      `ipcMain.handle` call anywhere else in the codebase, proven by a
      structural test
- [ ] `system.getAppVersion` works end to end (FR-5), proven by an
      integration test through the real preload/main boundary
- [ ] Every declared operation's Zod schema rejects at least one
      deliberately malformed argument shape, proven per operation
- [ ] `contextIsolation`, `sandbox` on and `nodeIntegration` off are
      verified by an automated test — `architecture-desktop-host.md`
      FR-2's own requirement, this spec's concrete proof of it
- [ ] `source.pickLocalFolder` (FR-6) returns the OS-selected path on a
      real selection, and `null` on cancel, proven by an integration
      test through the real preload/main boundary
- [ ] Every FR maps to an exit criterion in phase 05's own document

## Open questions

- **When hand-maintenance (FR-1) stops scaling** — no operation count
  threshold is fixed here; revisit once phase 06+ have added enough
  operations that reviewing `operations.ts` by hand becomes the actual
  bottleneck, not before.
- **CI enforcement of FR-3's full checklist** — items 1 and 3 (schema
  presence, test existence) are mechanically checkable; items 2 and 4
  (validation-first handler ordering, reviewer acknowledgment) are not,
  without deeper static analysis this spec doesn't design. Left as a
  human-review responsibility for now.

## References

- `architecture-desktop-host.md` FR-1, FR-2 — the pattern this spec
  makes concrete
- `roadmap/05-desktop-host/README.md` — the schema-vs-hand-maintained
  and review-process open decisions this spec resolves
- `desktop-host-process-model.md` — the spawn/lifecycle logic a future
  IPC operation might need to read from, not this spec's own concern
- Constitution §4 (hostile input), §5 (Electron privilege boundary), §9
  (dependencies — Zod justification), §11 (copy — error messages)
