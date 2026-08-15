# Review: Phase 07 — three metadata specs, cross-spec, two independent agents

| | |
|---|---|
| **Subject** | `.claude/specs/backend-metadata-adapter.md`, `backend-metadata-caching.md`, `frontend-discover-screen.md` — plus `.claude/roadmap/07-metadata/README.md`, freshly expanded from a stub outline as part of this same work |
| **Reviewer** | Two independent `general-purpose` agents, run in parallel with no shared context or coordination |
| **Date** | 2026-08-15 |
| **Verdict** | Needs rework at review time (1 Blocking, independently confirmed by both passes; 8 Major across both passes, 2 independently confirmed twice; 10 Minor/Nit) — see Resolution below for fixed status |

## Summary

Both agents read the three specs together as a batch against
`domain-bibliographic.md` (phase 02), `backend-errors-and-logging.md`/
`backend-persistence.md`/`backend-configuration.md`/`backend-http-transport.md`
(phase 03), `backend-library-api.md` (phase 06), and the freshly-expanded
phase 07 roadmap document. Both independently converged on the same
Blocking finding: `backend-metadata-adapter.md` FR-8 constructed
`coverUrl` as a literal `covers.openlibrary.org` URL, directly
contradicting `backend-metadata-caching.md`'s entire cover-caching design
and `frontend-discover-screen.md`'s own explicit security claim that
covers "never" go directly to Open Library — a self-contradiction that
would have shipped undetected by the batch's own test plans, since every
fixture mirrored the buggy shape. This is the phase's first external,
third-party network boundary, and the contradiction sat exactly at the
one place that boundary's rate-limiting and caching discipline was
supposed to be enforced. Two Major findings were also independently
confirmed by both passes: `frontend-discover-screen.md` FR-7 attributed
a "lazy-loading and layout-stability discipline" to
`frontend-generated-covers.md`, which — per its own FR-1 ("No layer
fetches external data") — has no such discipline to attribute, a
self-contradiction inside the same sentence; and both backend specs'
Non-functional requirements pointed at an "Observability" section that
didn't exist anywhere in either document, silently dropping the
roadmap's own named logging requirements (distinguishing "Open Library
is down" from "Open Library changed shape," never logging search
queries).

## Findings

Consolidated from both passes; duplicate findings merged, each tagged
with which pass(es) raised it.

| # | Severity | Spec(s) | Finding | Required change | Raised by |
|---|---|---|---|---|---|
| 1 | Blocking | `backend-metadata-adapter.md` FR-8 vs. `backend-metadata-caching.md` FR-5/FR-6 vs. `frontend-discover-screen.md` Security considerations | Adapter FR-8 built `coverUrl` as the literal external Open Library covers URL; the caching spec's whole FR-5/FR-6/FR-7 design (rate-limited fetch, disk cache, 500 MiB LRU bound) never actually engaged, since nothing would ever call its own `GET /api/v1/discover/covers/:coverId` endpoint; the frontend spec's Security considerations flatly asserted covers are "never a direct `<img src>` to `covers.openlibrary.org`," which was false as written. | FR-8 rewritten to construct `coverUrl` as this system's own path (`/api/v1/discover/covers/<cover_i>`); the external OL covers URL is now built server-side, only inside `backend-metadata-caching.md` FR-6's handler, on a genuine cache miss. Frontend spec's claim is now true. | Both, independently |
| 2 | Major | `backend-metadata-adapter.md` FR-2/FR-3, `backend-metadata-caching.md` FR-1, `frontend-discover-screen.md` Domain model | `NormalisedAuthor`'s field shape was never defined anywhere, despite three documents presupposing it exists (FR-2's inline, differently-shaped author pair; FR-3's bare reference to `NormalisedAuthor[]`; the caching spec's schema claiming to "mirror" it; the frontend spec claiming to render it). | Added an explicit `NormalisedAuthor = { openLibraryAuthorKey: string \| null, name: string }` definition to FR-2, stated as the one shape reused by both FR-2's search results and FR-3's work-detail authors. | A: undefined shape. Both independently flagged the shape was load-bearing but unstated. |
| 3 | Major | `frontend-discover-screen.md` FR-7 | Claimed `frontend-generated-covers.md` "already established" a lazy-loading/layout-stability discipline for its covers, then in the same sentence stated this is "the first screen loading network images rather than client-rendered ones" — a self-contradiction, since `frontend-generated-covers.md` FR-1 explicitly fetches nothing external and has no `<img>` load-state concept to have established. | FR-7 rewritten to introduce the discipline fresh, as this spec's own new requirement, reusing only `frontend-generated-covers.md`'s fallback *rendering path* on a load error — not a nonexistent "loading discipline." | Both, independently |
| 4 | Major | `backend-metadata-adapter.md`, `backend-metadata-caching.md`, Non-functional requirements | Both specs' Non-functional requirements said "Observability — see dedicated section below," but neither document contained an `## Observability` heading — unlike every prior approved backend spec in this project, which write Observability content inline. The roadmap's own named requirements (distinguish "OL is down" from "OL changed shape" in logs; never log search queries) were consequently unspecified anywhere. | Added real inline Observability content to both specs: adapter spec now specifies distinct `warn` log messages for rate-limit rejection vs. upstream outage, `info`-level per-field-drop logging excluding the dropped value, and explicit exclusion of the search query string from logs; caching spec now specifies `warn` on cache-write failure and `info` on eviction-pass summaries. | Pass A |
| 5 | Major | `backend-metadata-adapter.md` FR-6 | Required a configuration-driven `User-Agent` but never named the actual config key, nor classified it under `backend-configuration.md` FR-3's three required/optional categories — leaving no fixed key name or defined behavior for an unset value. | Named the key `OPEN_LIBRARY_USER_AGENT`, classified as required/no-default (a placeholder default would itself violate Open Library's policy). Amended `backend-configuration.md` FR-4's authoritative key table to add the row, and its own header to record the amendment (same post-approval-amendment discipline established in review `0032`). | Pass B |
| 6 | Major | `backend-metadata-adapter.md` FR-3, Open questions | Author-fetch fan-out for a work-detail request was entirely unbounded, left only as an Open question rather than mitigated — combined with FR-7's shared 3 req/s budget across every Open Library call type, an unbounded-author work could monopolize the budget and starve concurrent lookups, exactly the DoS-composition risk the roadmap's own risk table names as unresolved. | Capped at the first 20 distinct authors per work-detail request; a single author's own fetch failure now explicitly degrades to that one author being omitted (`domain-bibliographic.md` FR-7's "zero authors is legal" extended to "fewer than expected is legal"), not a whole-request failure. | Pass B |
| 7 | Major | `backend-metadata-adapter.md` FR-4 | The "drop the field, don't fail the request" promise didn't reconcile with Go's typed JSON decoding for numeric fields (`cover_i`, `first_publish_year`, `edition_count`) — a type-mismatched value from Open Library would fail a strict struct unmarshal for the *entire* response, not degrade just that field, unlike a string field's post-decode validation. | FR-4 now specifies decoding into a permissive intermediate form (`map[string]any`/`json.RawMessage` per field) so a numeric field's type mismatch is caught and dropped per-field, the same as any other invalid field. | Pass B |
| 8 | Major | `backend-metadata-caching.md` FR-1 | The schema FR said the cache tables "mirror" the adapter's normalised shapes but never specified the relational structure (join tables vs. array columns) for a work's editions or authors — a gap the roadmap's own "Architecture decisions expected" section explicitly assigns to this spec. | Added the relational shape: `metadata_editions.work_key` as a foreign key, and a `metadata_work_authors(work_key, author_key)` join table for the many-to-many author relationship. FR-4's write-through updated to upsert the join rows too. | Pass B |
| 9 | Major | `backend-metadata-caching.md` FR-6 | The cache-miss → fetch → write → serve flow for covers had no concurrency control: two simultaneous requests for the same uncached `coverId` could both fetch and both write to the same file path with no stated atomicity, risking a torn file served to a concurrent reader. | Required atomic writes (temp file + `os.Rename`) and explicitly accepted the redundant-fetch race (two concurrent misses both hitting Open Library) as a narrow, low-cost tradeoff rather than adding per-key fetch locking — the same honesty pattern this spec already used for its eviction race. Also added a magic-byte/format check before caching or serving any fetched bytes, and an explicit size cap/timeout for the cover fetch itself. | Pass B |
| 10 | Major | `backend-metadata-adapter.md` FR-6 | Cited `backend-errors-and-logging.md`'s "redaction discipline" as the source of "don't log outbound headers wholesale" — that spec's actual FR-8 mechanism is type-level redaction for specific secret *values*, not a general outbound-header rule; a fabricated-citation instance of the pattern named as recurring in reviews `0031`–`0033`. | Reworded as this spec's own reasoned choice, not an inherited rule. | Both, independently |
| 11 | Minor | `backend-metadata-adapter.md` FR-2 | Self-citation error: `coverUrl (nullable string, FR-7 constructs it)` — FR-7 is the rate limiter; construction is FR-8. | Fixed the FR number. | Pass B |
| 12 | Minor | `backend-metadata-adapter.md` Security considerations | SSRF closure argument named fixed hostnames but didn't require proper query-parameter encoding for `q`, leaving a theoretical query-injection gap into the outbound Search API URL even with the host fixed. | Added an explicit requirement that `q` (and every inbound value placed into an outbound URL) is encoded via `net/url`'s own API, never string-concatenated. | Pass B |
| 13 | Minor | `frontend-discover-screen.md` FR-1 | Didn't state whether Discover has a grid/list view toggle (`<WorkGrid>` has one) or explain why `<DiscoverResultGrid>` skips `<WorkGrid>`'s 100-item virtualization threshold — both left as unstated inferences. | FR-1 now states explicitly: no view toggle (Discover renders grid-only, a deliberate UX choice); virtualization is skipped because a Discover page is capped at 50 items, below the threshold. | Pass B |
| 14 | Minor | `backend-metadata-adapter.md` FR-7 | "MUST NOT queue unboundedly" had no concrete bound on concurrent waiters, only a 5-second per-waiter timeout. | Added an explicit 50-waiter cap; a request arriving past the cap returns `Unavailable` immediately rather than joining the queue. | Pass B |
| 15 | Minor | `backend-metadata-adapter.md` FR-9 | Cited `backend-http-transport.md`'s "existing timeout discipline" as governing this spec's outbound-call timeout — that spec's FR-2 timeouts govern this system's own *inbound* listener, not outbound third-party calls; the figure is reused for consistency, not literal precedent. | Reworded to state the number is reused for consistency across the codebase's timeout values, not inherited discipline. | Pass A |
| 16 | Minor | `backend-metadata-caching.md` FR-5 | Cited `architecture-persistence.md` as defining a general "app-data root" — that spec's FR-1 is specific to the PostgreSQL data directory; `backend-configuration.md` FR-5 is the actual precedent for reusing that directory family for other local files. | Citation corrected to `backend-configuration.md` FR-5. | Pass A |
| 17 | Minor | `backend-metadata-adapter.md`, `backend-metadata-caching.md` | Constitution §10's adversarial checklist names "duplicated" alongside malformed/enormous/slow; neither spec addressed Open Library returning duplicate entries within one response. | Added a Failure modes row to the adapter spec: duplicate work keys within one `docs[]` response are de-duplicated, later entries dropped. | Pass A |
| 18 | Minor | `backend-metadata-caching.md` FR-6 | No validation that fetched cover bytes are actually a well-formed image before caching/serving — a shape-check gap at a hostile external boundary. | Added a magic-byte/format check (part of finding #9's fix). | Pass A |
| 19 | Minor | `backend-metadata-caching.md` FR-2 | Left ambiguous whether cache-checking applies independently per sub-entity (work, editions, each author) or only gates the top-level work fetch. | FR-2 now states explicitly that each of the three entity types is checked/served independently. | Pass A |
| 20 | Minor | `backend-metadata-adapter.md` Domain model vs. API and contracts | Ambiguous whether `Normalised*` types are literal `domain.Work`/`Edition`/`Author` instances or separate DTOs, affecting the adapter package's dependency direction. | Domain model section rewritten to state explicitly these are wire-level DTOs reusing `domain-bibliographic.md`'s field-level *validators*, never full domain aggregates — a domain `Work` is only ever constructed later, by phase 10's import flow. | Pass A |
| 21 | Nit | `frontend-discover-screen.md` FR-1 | `WorkSummary`'s ownership/collection-membership fields were asserted without noting this is an inference from `backend-library-api.md` FR-9 rather than an independently verified field list. | Reworded to cite FR-9 explicitly as the source of the inference. | Pass A |
| 22 | Nit | `backend-metadata-adapter.md` Domain model | `internal/metadata/openlibrary` package name flagged as illustrative, with no corresponding entry in `architecture-backend.md`'s package layout. | No spec change — both passes agreed this is correctly hedged already; flagged for `architecture-backend.md` reconciliation at implementation time, not a spec defect. | Pass A |

## Dimensions checked

Both agents independently marked these checked in depth: Completeness
against `domain-bibliographic.md`'s identity/optionality model (FR-1/
FR-2/FR-3/FR-7, confirmed the DTO-not-aggregate design in finding #20
respects it), Security (SSRF, path traversal, injection, the coverUrl
architecture violation itself — finding #1 — and the concurrency/format
gaps in finding #9), Cross-spec and cross-phase citation accuracy (the
source of findings #2, #3, #10, #15, #16), Resource exhaustion/DoS
composition (rate limiter, author fan-out, cover eviction — findings
#6, #9, #14), Testability (every FR's "must fail before implementation"
claim independently spot-checked as genuinely derivable), Constitution
§9 dependency justification (`golang.org/x/time/rate`'s justification
confirmed complete, including abandonment risk).

- [x] Completeness
- [x] Ambiguity
- [x] Architecture
- [x] Domain correctness
- [x] Security
- [x] Testability
- [x] Accessibility (frontend spec's a11y coverage checked against `frontend-accessibility.md`'s existing conventions, no new design needed)
- [ ] Observability (checked as a completeness gap — finding #4 — not re-checked after the fix within this same review pass)
- [x] UX and copy (both named degraded-state strings in `frontend-discover-screen.md` FR-5 confirmed constitution §11-compliant)
- [ ] Maintainability (lightly checked only)
- [ ] Evolution (lightly checked only — phase 10's later import/matching step confirmed cleanly deferred throughout)

## Contradictions and gaps

This batch's Blocking finding is the third instance of a cross-document
self-contradiction pattern this project's reviews keep finding (after
`0032`'s cross-phase `backend-http-transport.md`/`frontend-design-tokens.md`
gaps and `0033`'s single-spec self-contradiction in `backend-library-api.md`):
here, three separate documents each made an independently-reasonable
claim about the same wire field (`coverUrl`) that only cohered once
compared side by side — exactly why this project's cross-spec review
step exists rather than reviewing each spec alone. The two
independently-confirmed Major findings (FR-7's self-contradicting
citation, the missing Observability sections) are both instances of the
now-familiar citation-drift and section-stub habits named in reviews
`0031`–`0033`; worth continuing to treat as a standing risk specifically
when a spec references a sibling document's "dedicated section" or
"established discipline" without the reviewer confirming that section
or discipline actually exists as claimed.

## What was not reviewed

Neither agent executed or compiled anything — documents-only review.
Neither agent independently re-verified Open Library's real API
response shapes against the live service (both accepted the Context
sections' citations of `openlibrary.org/dev/docs/...` at face value,
per their own task scope). Neither agent independently assessed
whether 3 req/s, 30-day cache staleness, or the 500 MiB cover bound are
"correct" numbers — all three are explicitly flagged as reasoned
placeholders in their own Open questions sections, and both reviewers
accepted that framing rather than relitigating unmeasured numbers.
Neither agent read `.design-reference/` directly to independently
confirm `frontend-discover-screen.md`'s claim that no Discover screen
capture exists — both trusted the spec's own Open questions note on
this.
