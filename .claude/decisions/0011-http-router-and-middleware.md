# 0011. HTTP routing uses the standard library's `ServeMux`; middleware is hand-rolled, no router framework

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-14 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

`architecture-backend.md` FR-6 fixed the middleware chain's order (panic
recovery, request limits, logging, routing, with an auth slot reserved for
phase 12) but explicitly left the router/middleware library choice to phase
03, per phase 01's own "architecture decisions expected" list, "justified
either way under constitution §9."

Go's standard library `net/http.ServeMux` gained method-based routing and
wildcard path segments (`GET /books/{id}`) in Go 1.22. Before that release,
essentially every real Go HTTP service reached for a third-party router
(`chi`, `gorilla/mux`, `httprouter`) because the stdlib mux couldn't match a
method or extract a path parameter. That reason no longer applies to a
project starting today.

## Decision

Routing uses `net/http.ServeMux` from the standard library. Middleware is a
small number of hand-written `func(http.Handler) http.Handler` wrappers,
composed in the fixed order `architecture-backend.md` FR-6 already
specifies, with no middleware framework or router-bundled middleware system
(`chi`'s `Use()`, for instance) pulled in to do that composition.

Concretely: `internal/transport/http` owns a `Router()` constructor that
registers method+path patterns on a `*http.ServeMux`, then wraps the mux in
the middleware chain by ordinary function composition
(`recoverMiddleware(limitsMiddleware(loggingMiddleware(mux)))`). No
reflection-based route registration, no struct tags, no generated code.

## Options considered

### Option A — Standard library `ServeMux` + hand-rolled middleware (chosen)

*For* — zero new dependencies for something the standard library now does
natively; the middleware chain is four functions with a fixed, spec'd order
(`architecture-backend.md` FR-6) — a framework's `Use()` ordering rules
would be solving an ordering problem this project has already solved by
writing the order down and composing directly. Matches ADR 0008's
established bias against tooling sized for a problem this project doesn't
have ("this serves one household").

*Against* — no built-in route groups, no automatic OpenAPI generation from
route registration (moot — `architecture-contracts.md` FR-1 already fixed
the OpenAPI file as hand-authored, not generated), and pattern-matching
error messages when two routes conflict are less polished than a mature
router's. For the endpoint count this project will have for a long time,
none of this is a real cost.

### Option B — `chi`

*For* — the most common "thin" Go router, real ecosystem middleware
(`chi/middleware`) for things like request-ID generation, mature and
well-maintained.

*Against* — every piece of `chi/middleware` this project would actually use
(recovery, request ID, timeout) is a handful of lines to write directly
against the stdlib, and writing them directly means each one matches this
project's own error taxonomy and logging contract exactly, rather than
adapting a generic middleware's behavior to fit. A dependency needs a
constitution §9 justification beyond "it's popular" — `ServeMux` closes the
actual gap (method/path matching) that used to be the real reason to reach
for `chi`.

### Option C — `gorilla/mux`

*Against* — the project is in maintenance mode (archived, "the Gorilla Mux
maintainers would like to formally archive this project"), a direct
conflict with constitution §9's "what breaks if it's abandoned" question —
it already has.

## Consequences

**Good** — one fewer dependency to audit (§9), to track for CVEs
(`architecture-testing.md` FR-5), and to upgrade across a Go version bump.
The middleware chain's order is enforced by the literal shape of the code
(`f(g(h(mux)))`), not by a framework's registration-order convention that a
future contributor could get subtly wrong.

**Bad** — some hand-written plumbing (a tiny `withRecover`, `withLimits`,
`withLogging` set) that a router framework would have provided pre-built;
real but small, and each one is directly testable in isolation
(`architecture-backend.md`'s own Test strategy already calls for exactly
this: "middleware in isolation"). A more structural cost, not named in
this ADR's first draft (caught by independent review, `.claude/reviews/
0022-phase03-cross-spec-review.md`): manual function composition
(`f(g(h(mux)))`) has no compiler- or lint-level guard against a future
contributor silently reordering the fixed chain
(`architecture-backend.md` FR-6) — unlike this project's own
import-boundary rule, which is enforced by a CI lint check, not just a
unit test someone could forget to run or a reviewer could miss in a large
diff. The mitigation today is a unit test asserting the composed order
(`backend-http-transport.md`'s own Test strategy), which is real but
weaker than a structural guarantee — a test can be weakened or deleted in
the same PR that breaks the order, where a lint rule generally can't be
worked around as quietly.

**Bad, mildly** — this ADR's middleware layer also produced one small,
already-fixed integration gap: an early draft of the correlation-ID
requirement assumed it was always available by the time recovery ran,
which isn't true for a panic in a layer that executes before the logging
middleware (recovery sits outermost, per the fixed chain). Fixed in
`backend-errors-and-logging.md` FR-10 (recovery generates a fallback ID
in that case) — noted here because it's a direct consequence of the fixed
composition order this ADR commits to, not an unrelated bug.

**Neutral** — doesn't change the middleware order or contents
`architecture-backend.md` FR-6 already fixed; this ADR only fixes the
mechanism that composes them.

## Reversal cost

Low. `chi`'s handler signature (`http.Handler`) is stdlib-compatible by
design, specifically so a project can start on the stdlib mux and adopt it
later without a rewrite, if the endpoint count or routing needs ever
outgrow what `ServeMux` handles well.

## Confidence

High. This follows directly from a Go version fact (1.22's `ServeMux`
upgrade) rather than a judgement call under uncertainty — the reason to
reach for a third-party router narrowed materially, and this project's
existing dependency-minimalism (ADR 0008) already sets the tie-breaker for
what's left.
