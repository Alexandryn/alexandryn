# Spec: Frontend collections screens

| | |
|---|---|
| **Status** | `VERIFIED` (2026-08-31, phase 06 Tier 6 / L24 — Tiers 0–5 / PR #71, audit [`0006`](../audits/0006-phase06-library.md), verified by maintainer) |
| **Phase** | `06-library` |
| **Author** | Claude (Sonnet 5), approved by Luann Moreira |
| **Created** | 2026-08-14 |
| **Last updated** | 2026-08-31 |

| **Supersedes** | — |
| **Reviewed in** | [`0033`](../reviews/0033-phase06-cross-spec-review.md) (two independent agents, cross-spec) — Needs rework at review time, all findings fixed; approved by maintainer 2026-08-14 |

## Context

`backend-library-api.md` FR-6/FR-7 fixed collection CRUD and membership
endpoints. `frontend-shell-and-routing.md` FR-1 already reserved
`/collections`; `/collection/:id` is a plausible, consistent extension
of that route list's own singular-detail pattern (`/book/:id` for a
single work) rather than a route that spec names explicitly — this spec
is where it's actually fixed, not merely inherited. Both were MSW-mocked
(tier-(b), `frontend-shell-and-routing.md` FR-6, since no real endpoint
existed at phase 04). This spec builds the real screens and removes
those hand-written mocks outright, the same pattern
`frontend-library-screens.md` uses for its own routes.

## Problem

Nothing has fixed: the collections index screen's layout, the collection
detail screen's layout, how a `Work` gets added to a collection from the
UI (from the collections index, from work detail, or both), the
create/rename/delete UI, or the confirmation pattern for delete
(`domain-library.md` FR-10's non-cascading delete is safe data-wise, but
still a destructive, one-way UI action worth confirming).

## Goals

- Fix the collections index screen: list, create, rename, delete
- Fix the collection detail screen: member works, remove-from-collection
- Fix the add-to-collection interaction, reachable from work detail
  (`frontend-library-screens.md` FR-5)
- Retire the MSW mock for `/api/v1/collections*` routes

## Non-goals

- Library browse/search/work detail's own content —
  `frontend-library-screens.md`
- The backend endpoints — `backend-library-api.md`
- Collection ordering/nesting — `domain-library.md`'s own Open questions
  already left this undesigned; this spec doesn't invent it either
- Sharing collections across users/accounts — phase 12, matching
  `domain-library.md` FR-5's "one shared copy" model already in place

## User stories

- As **a user**, I want to see all my collections at a glance and create
  a new one without leaving the screen.
- As **a user viewing a work**, I want to add it to a collection (new or
  existing) without navigating away from the work.
- As **a user deleting a collection**, I want a clear confirmation that
  names what will happen (the collection is deleted, the books are not),
  since `domain-library.md` FR-10's non-cascading behavior is exactly
  the kind of detail a user would want confirmed, not assumed understood.

## Functional requirements

- **FR-1** Collections index (`/collections`) renders a grid of
  `<StatCard>`-based collection tiles (`frontend-component-primitives.md`'s
  existing primitive — a collection's name, member count, and a small
  preview of its first few members' covers, reusing `frontend-generated-
  covers.md`'s system for any member without a real cover), each
  linking to `/collection/:id`. A "New collection" affordance opens a
  `<Modal>` (`frontend-component-primitives.md` FR-1's Radix-wrapped
  `Dialog`) with a single name input, submitting to
  `backend-library-api.md` FR-6's create endpoint.
- **FR-2** Collection detail (`/collection/:id`) renders the collection's
  name (with an inline rename affordance — click-to-edit, submitting to
  `backend-library-api.md` FR-6's `PATCH`), and its member works via
  **`<WorkGrid>`** (`frontend-library-screens.md` FR-1's new shared
  component, taking this screen's own `Collection` detail response's
  member-work list rather than a library query — the same rendering
  implementation, a different data source, per that component's own
  data-source-agnostic contract). Each member work has a
  remove-from-collection affordance (`backend-library-api.md` FR-7's
  `DELETE`).
- **FR-3** Deleting a collection (from either the index, FR-1, or detail,
  FR-2) requires confirmation via a `<Modal>` naming the specific
  consequence: "Delete '<name>'? The books in it will stay in your
  library." (constitution §11's bar — specific, says what actually
  happens, not a generic "are you sure?") — never a bare browser
  `confirm()` dialog, which can't carry this specific, reassuring detail
  and doesn't match the app's own visual language.
- **FR-4** Adding a work to a collection, reachable from work detail
  (`frontend-library-screens.md` FR-5): an "Add to collection" affordance
  opens a `<Modal>` listing existing collections (checkbox-style,
  reflecting current membership, toggling calls `backend-library-api.md`
  FR-7's add/remove endpoints) plus an inline "create new collection and
  add" option that combines FR-1's create flow with FR-7's add in one
  submission from the user's perspective (two API calls made in
  sequence, not a new combined backend endpoint — `backend-library-api.md`'s
  own scope doesn't need to grow for this, it's a frontend-only
  composition of two existing calls). The sequence's two failure classes
  are handled differently, not collapsed into one: **(a)** the create
  call itself fails (network error, validation error) — the whole
  combined action failed, surfaced as one error, no add attempted, no
  ambiguity, since nothing was created; **(b)** the create call
  *succeeds* but its response is lost before this client observes it
  (a timeout, a dropped connection after the server committed) — the
  client genuinely cannot tell success from failure client-side. This
  spec does not attempt automatic recovery for case (b): the combined
  action is **never automatically retried** once the create request has
  been sent (retrying blind risks a second collection with the same
  name, since `backend-library-api.md` FR-6's create endpoint has no
  idempotency key). Instead, on an ambiguous outcome, the modal shows an
  explicit "unable to confirm — check your collections list" message
  and closes without attempting the add; the user's next action (opening
  the collections index, FR-1) shows the true state directly, since that
  screen always reflects the real backend. This is a deliberate
  make-the-user-check design, not a solved automatic-retry mechanism —
  named explicitly rather than silently left undesigned.
- **FR-5** The MSW mock for `/api/v1/collections`,
  `/api/v1/collections/:id`, and the membership sub-routes is **removed
  outright**, not regenerated — same correction as
  `frontend-library-screens.md` FR-6: these were always tier-(b)
  hand-written fixtures (`frontend-shell-and-routing.md` FR-6, no real
  endpoint existed at phase 04), never tier-(a) fixtures being
  "retired." The `TODO(phase-06)` marker's absence on these fixture
  files is this FR's own acceptance criterion. Every other
  still-mocked route is untouched.

## Non-functional requirements

- **Performance** — not separately budgeted; inherits
  `frontend-tooling.md`'s overall bundle budget and
  `frontend-library-screens.md` FR-4's virtualization pattern if a
  collection's member count ever grows large enough to warrant it (the
  same `<WorkGrid>` component, so the same threshold applies without
  this spec needing its own number).
- **Security** — see Security considerations below.
- **Accessibility** — the delete-confirmation modal (FR-3) and add-to-
  collection modal (FR-4) both follow `frontend-component-primitives.md`
  FR-3's `Modal` accessibility contract (focus trap, focus return,
  `Escape` closes) directly — no new accessibility design needed since
  both are instances of the already-specified `Modal` primitive.
- **Reliability** — FR-4's two-sequential-calls composition (create,
  then add) is not atomic at the network level. Case (a), create fails
  outright: nothing partial, one clean error. Case (b), create succeeds
  but the add fails with a *known* response (not a timeout — the server
  answered, the add itself was rejected): the UI surfaces this as a
  partial-failure state (the new collection now appears in the list the
  user can manually add to), never silently swallowing the failure as if
  the whole operation succeeded. Case (c), create's own response is
  lost (FR-4's own ambiguous-outcome case): no automatic retry, an
  explicit "check your collections list" message instead — deliberately
  not treated the same as case (b), since case (b) knows the collection
  was created and case (c) doesn't know anything for certain.
- **Observability** — inherits `frontend-shell-and-routing.md` FR-7's
  correlation-ID error display for both API calls.

## Domain model

Not applicable — renders `domain-library.md`'s `Collection` model via
`backend-library-api.md`'s shapes.

## API and contracts

- **Collections index ↔ `GET /api/v1/collections`**: `useQuery`, key
  `['collections']`, invalidated on create/delete (FR-1/FR-3).
- **Collection detail ↔ `GET /api/v1/collections/:id`**: `useQuery`, key
  `['collection', id]`, invalidated on rename/membership change
  (FR-2/FR-4).
- **Mutations** (`useMutation`, TanStack Query's own mutation primitive):
  create, rename, delete, add-member, remove-member — each invalidating
  the relevant query key(s) above on success, never manually patching
  the cache by hand (avoids the cache and the server silently
  disagreeing after a mutation).

## State transitions

- Modal-driven interactions (FR-1's create, FR-3's delete-confirm, FR-4's
  add-to-collection) are standard open/closed state, using
  `frontend-component-primitives.md` FR-1's `Modal` primitive's own
  state management — no bespoke modal state machine invented here.
- FR-4's partial-failure state (Reliability, above, case b): `created →
  add-pending → add-failed` is a real, distinct state the UI must
  render (a visible note that the collection was created but the work
  wasn't added, with a retry-the-add affordance), not silently collapsed
  into either "success" or "failure."
- FR-4's ambiguous-outcome state (Reliability, case c): `create-sent →
  response-lost → user-directed-to-check` — deliberately terminal, no
  automatic transition back to a retry state, per FR-4's own no-blind-
  retry rule.

Illegal transitions, restated for this layer:

- A delete proceeding without FR-3's confirmation modal
- A collection appearing in the index (FR-1) before its create mutation
  has actually resolved successfully (optimistic-update risk this spec
  deliberately avoids — the modal stays open with a loading state until
  the real response arrives, per `architecture-frontend.md`'s own
  no-optimistic-rendering-before-confirmation spirit)

## Failure modes

| Failure | Detected how | User sees | System does |
|---|---|---|---|
| Create-collection mutation fails (FR-1) | `useMutation`'s error state | Inline error in the still-open modal, name input retained (not cleared, so the user doesn't retype) | Modal stays open; no navigation, no partial state |
| Delete-collection mutation fails (FR-3) | `useMutation`'s error state | Error shown in the confirmation modal; collection remains in the index | No optimistic removal — the collection is only removed from view once the server confirms |
| FR-4's create-then-add sequence: create succeeds (known), add fails | Second mutation's own error state, checked explicitly since the two calls aren't wrapped in a single transaction | The new collection appears (real, created); an inline note that adding the current work failed, with a retry affordance | The two calls' outcomes are tracked and surfaced independently, never conflated into one success/failure signal |
| FR-4's create call: response lost (timeout/dropped connection, outcome unknown) | The mutation's own timeout/network-error state, distinguished from a normal rejection | "Unable to confirm — check your collections list," modal closes | No automatic retry attempted; no add attempted; the user's next visit to the index (FR-1) shows ground truth |
| Rename to an invalid name (`domain-bibliographic.md`-shared validation bar) | `backend-library-api.md` FR-6's `400` response | Inline error on the rename input, same specificity as any other validation error | Rename not applied; previous name remains displayed |

## Security considerations

- **No new trust boundary** — same as `frontend-library-screens.md`;
  collections are visible identically to every capability level until
  phase 12/13.
- **Collection names rendered, never trusted as safe HTML** — React's
  default escaping covers this; no primitive in this spec's composition
  uses `dangerouslySetInnerHTML`, consistent with
  `frontend-component-primitives.md`'s own Security considerations.

## Test strategy

| Layer | What it covers |
|---|---|
| Unit | Modal open/close state transitions (FR-1/FR-3/FR-4) |
| Component | Collection tile rendering with 0/1/many members (FR-1), the partial-failure state's specific UI (Reliability) |
| Integration | Full create/rename/delete/add/remove flow against the real backend (`backend-library-api.md`), including the FR-4 two-call composition's known-failure path (case b) — deliberately triggered by targeting the add call at a `workId` that doesn't exist, a real, naturally-occurring `404` from the real backend rather than a network-layer interception, so the test exercises actual application error-handling, not a simulated transport failure — and the ambiguous-outcome path (case c), triggered by aborting the create request client-side after the server would have already committed it (a controlled simulation of a lost response, the one case that can't be produced by a real backend response alone) |
| Accessibility | `axe` checks on both screens and all three modals; focus-trap/return verified for each modal instance, not assumed shared correctness from one check |
| E2E | Create a collection → add a work to it from work detail → view it in collection detail → remove the work → delete the collection — the phase 06 collections slice of the reference walkthrough |

## Acceptance criteria

- [ ] Collections index and detail render real data, MSW no longer
      intercepting these routes, `TODO(phase-06)` marker gone from both
      fixture files, proven by a grep-based check (FR-5)
- [ ] Create, rename, delete all proven end to end against the real
      backend, including delete's required confirmation step (FR-3)
- [ ] Add-to-collection from work detail proven, including the
      create-new-and-add combined flow (FR-4)
- [ ] The create-succeeds-add-fails partial-failure state (case b) is
      proven with a real `404` from a bad `workId`, not a simulated
      network failure
- [ ] The create-response-lost ambiguous-outcome state (case c) is
      proven with a controlled aborted-request simulation, confirming no
      automatic retry occurs
- [ ] Collection detail renders member works via `<WorkGrid>`, including
      a member work reachable only via `filter=wanted` on the library
      side (a "want to read" work can still be a collection member with
      zero owned editions)
- [ ] Every modal (create, delete-confirm, add-to-collection) passes an
      `axe` check with zero violations and correct focus trap/return
- [ ] Every FR maps to an exit criterion in phase 06's own document

## Open questions

- **Collection tile preview's member-cover count** — how many covers to
  show in a collections-index tile preview isn't fixed here, a visual
  design detail informed by `frontend-design-tokens.md`'s spacing once
  a real layout is built against it.
- **Whether the two-call FR-4 composition should eventually become a
  single backend endpoint** (`backend-library-api.md`'s own scope,
  Open questions there) — not decided here; revisit if the partial-
  failure UX proves confusing enough in practice to justify the backend
  API surface growing.
- **FR-4's ambiguous-outcome (case c) "check your collections list"
  design is deliberately the simplest correct option, not the most
  robust one** — a client-generated idempotency key on the create call
  (letting a safe automatic retry distinguish "already created" from
  "never created") would remove the manual-check step entirely, but
  requires `backend-library-api.md` FR-6 to support one, which it
  doesn't yet. Worth revisiting if this failure mode proves common
  enough in practice to justify that backend addition — not assumed
  necessary here, since a lost response is a narrow race window, not
  the common case.

## References

- `backend-library-api.md` FR-6, FR-7 — the endpoints this spec consumes
- `frontend-library-screens.md` FR-1 — the shared `<WorkGrid>` component
  this spec's FR-2 depends on
- `frontend-component-primitives.md` FR-1, FR-3 — `Modal`/`StatCard`
  primitives and the accessibility contract FR-3/FR-4 inherit
- `frontend-shell-and-routing.md` FR-6, FR-7 — the MSW migration path
  FR-5 executes, correlation-ID error display
- `domain-library.md` FR-6, FR-9, FR-10 — non-cascading delete/removal,
  membership timestamps this spec's UI reflects
- Constitution §7 (accessibility), §11 (copy — FR-3's specific
  confirmation copy)
