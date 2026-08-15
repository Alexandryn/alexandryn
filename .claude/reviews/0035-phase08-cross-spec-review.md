# Review: Phase 08 — two source specs, cross-spec, two independent agents

| | |
|---|---|
| **Subject** | `.claude/specs/backend-source-adapter.md`, `frontend-source-management.md` — plus a post-approval amendment to `.claude/specs/desktop-host-ipc-surface.md` (FR-6, `source.pickLocalFolder`) and `.claude/roadmap/08-sources/README.md`, freshly expanded from a stub as part of this same work |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-15 |
| **Verdict** | Needs rework at review time (2 Blocking, both from one pass but independently derivable and confirmed sound on inspection; 8 Major across both passes, 2 confirmed independently by both; 5 Minor) — see Resolution below for fixed status |

## Summary

This phase is this project's second external, user-configured network
boundary, and its two named sharpest risks — SSRF via a source-supplied
continuation value, and path traversal via a source-supplied filename
— were both claimed closed in the initial draft but, on inspection,
neither mechanism actually covered the exact attack each FR named as
its own motivating example. `backend-source-adapter.md` FR-12's
traversal fix used `filepath.Clean`, which never touches the
filesystem and cannot detect a symlink whose target escapes
`basePath` — the spec's own named example. FR-11's SSRF fix validated
only the browse cursor's origin, leaving the OPDS search-link fetch
completely unvalidated and, independently, leaving every outbound call
open to a same-origin URL being 3xx-redirected off-origin after the
fact, since Go's default HTTP client follows redirects transparently.
Both are fixed: FR-12 now resolves the real, symlink-followed path
before the prefix check; FR-11 now validates both the cursor and the
search-link URL and disables redirect-following entirely across every
outbound call. Two Major findings were independently confirmed by both
passes: `frontend-source-management.md` FR-3 covered only six of
`backend-source-adapter.md` FR-6's eight (now nine, after this review
added a tenth for unsupported redirects) closed `detail` values, and
FR-6's claimed reuse of `<DiscoverResultGrid>` directly contradicted
the reasoning that component's own defining spec used to justify not
generalising `<WorkGrid>` one level up. A process-integrity issue was
also found and fixed: the `desktop-host-ipc-surface.md` amendment's
header briefly asserted this review had already happened and been
maintainer-reconfirmed, before either was true.

## Findings

Consolidated from both passes; duplicate findings merged, each tagged
with which pass(es) raised it.

| # | Severity | Spec(s) | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | `backend-source-adapter.md` FR-12 | The path-traversal fix joined the filename to `basePath` and ran it through `filepath.Clean`, then prefix-checked the *lexical* result. `Clean` never resolves symlinks — a symlink whose name lives inside `basePath` but whose target resolves outside it (the FR's own named example) passes the check every time, so the mechanism did not close the attack it claimed to. | Resolve each candidate's real path via `filepath.EvalSymlinks` before the prefix check, checked against `basePath`'s own resolved real path (computed once, at source create/update, not per file); a broken symlink or one resolving outside `basePath` is omitted from the listing, not surfaced as an error. A residual TOCTOU window between check and later file-open is named explicitly as an accepted risk for this phase, deferred to phase 10's own file-read implementation to close fully. | Pass B |
| 2 | Blocking | `backend-source-adapter.md` FR-8, FR-11 | FR-11's same-origin validation covered only the browse cursor (FR-7); FR-8's search fetches a source-advertised search-link URL with no origin check of any kind, and — independently — no FR anywhere set a redirect policy on the outbound HTTP client, so a same-origin *initial* URL could still be 3xx-redirected to an off-origin target by a malicious source, bypassing whatever origin check existed. This is the phase's own named sharpest risk, not actually closed as specified. | FR-11 rewritten to cover both the browse cursor *and* the search-link URL (validated once, at capture time during FR-6's health check, then persisted and reused — not re-validated per call, since the validation and usage point are now the same read); redirect-following disabled entirely (`CheckRedirect` returning `http.ErrUseLastResponse`) across every outbound call type, with a `3xx` response treated as that call's own failure (`http-3xx-unsupported`, added to FR-6's `detail` vocabulary). | Pass B |
| 3 | Major | `frontend-source-management.md` FR-3 vs. `backend-source-adapter.md` FR-6 | FR-3 claimed to cover FR-6's full closed `detail` vocabulary but only mapped six of eight values, silently omitting `http-4xx` and `http-5xx` — an ordinary non-auth 4xx or a bare 500 from a real OPDS source had no defined UI treatment, contradicting this spec's own Goals ("not a single generic 'error' state") and constitution §11. | FR-3 rewritten to cover all nine `detail` values (eight plus finding #2's new `http-3xx-unsupported`) with distinct copy for each, plus the two non-`detail` states (checking, reachable) — eleven states total, explicit about the count rather than an unstated subset. Test strategy/Acceptance criteria updated to match. | Both, independently |
| 4 | Major | `frontend-source-management.md` FR-6 vs. `frontend-discover-screen.md` FR-1 | FR-6 claimed to reuse `<DiscoverResultGrid>` for rendering `SourceCandidate` items "where the shapes align," directly contradicting the reasoning `frontend-discover-screen.md` FR-1 used to justify *not* generalising `<WorkGrid>` to fit a second DTO one level up — `SourceCandidate` is a third, structurally distinct DTO from `NormalisedSearchResult` (no `openLibraryWorkKey`, has a `fileReference` neither sibling DTO carries), and coupling `<DiscoverResultGrid>` to it repeats the exact distortion that spec's own precedent rejected. | FR-6 rewritten to introduce a new, small `<SourceCandidateList>` component, reusing the same lower-level cover-tile/title-text primitives `<DiscoverResultGrid>` itself was built from — the same relationship `<DiscoverResultGrid>` has to `<WorkGrid>`, applied one level further down the same chain, not a further-overloaded shared grid. | Both, independently |
| 5 | Major | `backend-source-adapter.md` FR-13, roadmap Architecture decisions expected | The roadmap explicitly assigns this spec the job of concretely justifying, not assuming, why credential storage uses a locally-generated key file rather than Electron's `safeStorage`/OS-keychain integration — the spec's Dependency justification only compared stdlib crypto to a third-party Go library, never actually addressing the keychain/Electron-coupling question the roadmap delegated to it. | Added the roadmap's own reasoning, made concrete: `safeStorage` is Node-only (unreachable from the separate Go process without a new IPC round trip per credential operation), would couple credential storage to Electron's own liveness, and Linux's `libsecret`/`kwallet` backing isn't guaranteed present — a local key file keeps credential storage inside the Go backend's existing persistence layer, consistent with how `DATABASE_URL` and the database itself are already owned there. | Pass A |
| 6 | Major | `backend-source-adapter.md` FR-13 | "A missing key file at startup MUST be treated as first run" conflated two different conditions: a genuine first run, and a key file lost while credentialed sources already exist — silently generating a replacement key in the second case permanently orphans existing ciphertext with no startup-level signal, only a slow, one-source-at-a-time discovery indistinguishable from bad passwords. | Startup now checks whether any `Source` row has a non-null credential before treating a missing key as first run; if credentialed sources exist, a `warn` log states the affected count before a replacement key is generated — the per-source `auth-rejected` ambiguity (unchanged) is now paired with a systemic signal for whoever reads server logs. | Pass B |
| 7 | Major | `backend-source-adapter.md` FR-7, FR-8 vs. FR-13 | FR-13 only specified decrypt-failure behavior for the health-check path; FR-7/FR-8 (the actual browse/search calls, which also need a successful decrypt to build the outbound `Authorization` header) had no stated behavior for a decrypt failure mid-call. | FR-13 extended: a browse/search call returns `503 Unavailable` immediately on decrypt failure, the same outcome as a genuinely unreachable source, since the underlying cause is functionally identical from the caller's point of view. | Pass B |
| 8 | Major | `backend-source-adapter.md` — no FR | No FR bounded outbound request rate or concurrency to sources at all, unlike `backend-metadata-adapter.md` FR-7's own shared-budget precedent for its external boundary — a caller could trigger unbounded concurrent health-checks/browses across every configured source with no backstop. | Added FR-14: a global 50-concurrent-request cap across all outbound source calls, rejecting immediately (`503`) rather than queuing — justified as pure resource protection (no external rate policy to respect, unlike Open Library, so immediate rejection is simpler and equally correct). | Pass B |
| 9 | Major | `backend-source-adapter.md` FR-4, Security considerations | Basic Auth is base64, not encryption; FR-4's "always send" policy sends a credential in near-cleartext on every request to a plain `http://` source, and this was never named as an accepted risk or otherwise addressed, unlike this project's usual practice of naming accepted tradeoffs explicitly. | Added an explicit accepted-risk statement to FR-4 and Security considerations (self-hosted/household threat model, constitution §6's own reasoning applied to outbound traffic); `frontend-source-management.md` FR-5 now shows a visible, non-blocking warning when a credential is attached to a non-`https` source. | Pass B |
| 10 | Minor | `backend-source-adapter.md` FR-1 | Cited `domain-bibliographic.md` FR-6 (scoped to `Work`/`Edition` string fields) for `Source.label` validation; `domain-source.md`'s own Domain model section already cites the correct provision, FR-5, for exactly this field. | Citation corrected to FR-5, matching `domain-source.md`'s own precedent. | Pass A |
| 11 | Minor | `backend-source-adapter.md` FR-7 | Grouped `backend-library-api.md` FR-1 and `backend-metadata-adapter.md` FR-1 as both having "already established" the same `[1, 50]`/default-`20` bound; the two specs actually use different numbers (`backend-library-api.md`: default `50`, max `100`). | Reworded to attribute the specific numbers to `backend-metadata-adapter.md` FR-1 only, and the general validate-and-reject discipline (not the numbers) to `backend-library-api.md` FR-1. | Pass A |
| 12 | Minor | `backend-source-adapter.md` FR-9 | No handling for duplicate entries within a single OPDS feed page, despite constitution §10's adversarial checklist naming "duplicated" explicitly and `backend-metadata-adapter.md` having just established a direct, recent precedent (review `0034` finding #17) for the structurally identical case. | Added a de-duplication rule to FR-9, mirroring `backend-metadata-adapter.md`'s. | Pass A |
| 13 | Minor | `backend-source-adapter.md` FR-11, Observability | The SSRF-rejection `warn` log's content wasn't specified — logging a rejected URL/query string verbatim risks a malicious source embedding misleading content into this system's own logs. | Specified: only the rejected origin/host is logged, never the full URL or query string. | Pass B |
| 14 | Minor | `frontend-source-management.md` FR-6, FR-9 (`backend-source-adapter.md`) | FR-6's summary of `SourceCandidate`'s fields ("title, author, format, size") didn't match FR-9's actual shape (`title`, `author`, `fileReference` — itself carrying format/size — and `coverUrl`, silently omitted). | Field list corrected to match FR-9 exactly, including `coverUrl`. | Pass B |
| 15 | Minor | `frontend-source-management.md` FR-4 | The amended IPC operation resolves to `{ path: string } \| null`; FR-4 described filling the text field with "the returned path" without noting the `.path` access. | Reworded to `result.path`. | Pass B |

Two additional Minor findings (FR-3's citation of `architecture-desktop-host.md` for the "same OS user account" assumption, overstating it as an explicit requirement that spec doesn't actually state; and FR-5/FR-8's unstated search-link persistence mechanism) were raised by Pass A and Pass B respectively and are folded into findings #2 and #6's fixes above, plus a standalone citation correction (FR-3 now reworded as this spec's own reasoned inference).

## Dimensions checked

Both agents independently marked these checked in depth: Security (this
phase's own two named sharpest risks — the source of both Blocking
findings), Cross-spec and cross-phase citation accuracy (findings #4,
#5, #10, #11), Completeness against the roadmap's own risk table and
Architecture-decisions-expected section (findings #5, #8), Domain
correctness (`FileReference`/`SourceOffering` usage against
`domain-source.md` confirmed accurate by both passes, no changes
needed), Testability (both passes independently noted the originally-
planned symlink and SSRF tests would not have actually proven what
they claimed, given the un-fixed mechanisms).

- [x] Completeness
- [x] Ambiguity
- [x] Architecture
- [x] Domain correctness
- [x] Security
- [x] Testability
- [ ] Accessibility (lightly checked — citations into `frontend-accessibility.md`/`frontend-component-primitives.md` confirmed accurate, no independent re-verification against those specs' full FR lists)
- [x] UX and copy (finding #3's incomplete state mapping was a direct constitution §11 gap)
- [x] Observability (finding #13; health-transition and rejection logging otherwise consistent with `backend-errors-and-logging.md`)
- [ ] Maintainability (lightly checked only)
- [ ] Evolution (lightly checked only — phase 09/10 deferrals confirmed clean by pass A)

## Contradictions and gaps

This batch's two Blocking findings share a pattern worth naming
explicitly: both FR-11 and FR-12 stated a security mechanism and named
the exact attack it was meant to close in the same sentence, and in
both cases the mechanism as specified didn't actually cover that named
example — not a peripheral gap, but the phase's own two sharpest risks,
from its own risk table, failing against their own stated test case.
The two independently-confirmed Major findings (`<DiscoverResultGrid>`
reuse, the six-of-eight `detail` mapping) are further instances of the
citation/coverage-drift pattern named as recurring in reviews `0031`
through `0034`. A new failure class also surfaced here for the first
time in this project: `desktop-host-ipc-surface.md`'s amendment header
briefly asserted a review outcome and maintainer re-confirmation before
either existed — caught by pass A and corrected before this review
record was finalized, but worth naming as a process risk to watch for
specifically when a spec amendment's header is drafted ahead of the
review it depends on, rather than after.

## What was not reviewed

Neither agent executed or compiled anything — documents-only review.
Neither agent independently re-verified the OPDS 1.2/2.0 specification
claims in `backend-source-adapter.md`'s Context section against the
live `specs.opds.io` documents (accepted at face value, consistent with
how review `0034` scoped this same category of check). Neither agent
independently verified `architecture-desktop-host.md`'s process-model
claims in full detail beyond the specific citation checked for finding
in the folded-in Minor list. PostgreSQL storage for the new
`sources`/`source_credentials`-shaped tables was not independently
schema-reviewed beyond confirming the repository-pattern citation is
accurate.
