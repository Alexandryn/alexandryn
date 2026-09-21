# 0023. The in-browser reader renders EPUB with `foliate-js`, pinned and vendored through npm

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-01 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Phase 11 builds the in-browser reader. The project has carried an open
question since phase 01 — "how the reader renders EPUB, and in what
sandbox." `frontend-reader.md` FR-1 already names `foliate-js` and
carries a constitution §9 dependency justification; `backend-reader-content.md`
carries the server-side sanitisation half. This ADR promotes that
spec-level reasoning to a first-class decision record, because the
rendering engine is a vendored third-party dependency whose own
maintainer documents two material weaknesses (instability, and an
insecure default rendering path), and a §9 justification buried in a
feature spec is easy to lose track of when the risk later materialises.

What we know: EPUB rendering needs correct pagination (reflowable
CSS-multi-column and scrolled) and correct EPUB CFI generation/parsing.
Both are large, well-specified, well-trodden problems. Phase 10's file
work stayed stdlib-only because zip + Dublin Core XML is genuinely
small; this is not that.

What we do not know: whether `foliate-js`'s stability warning bites in
practice for our usage, and whether its API changes under us before
phase 14 builds sync on top of the CFI data it produces.

## Decision

The reader renders EPUB content with **`foliate-js`** — its reflowable
paginator, its scrolled paginator, and its own `epubcfi.js` for CFI
generation and resolution.

It enters the codebase as **vendored source** under
`web/src/vendor/foliate/` (`epub.js`, `epubcfi.js`, the MIT `LICENSE`,
and hand-written `.d.ts` files) — *amended 2026-09-01*: the only
`foliate-js` npm package (`foliate-js@1.0.1`) is an **unofficial
third-party republish** (published by `shmandadi`, not the upstream
author `johnfactotum`), so pinning it would take a supply-chain
dependency on a republisher rather than on the project. The vendored
files are copied verbatim from that tarball after confirming they match
the official `johnfactotum/foliate-js` layout and the MIT licence; their
SHA-256 sums are recorded in the phase 11 PR. A refresh is a deliberate
re-vendor with a diff, not an automatic `npm update`. eslint and
`tsc`'s `src` scan treat `web/src/vendor/` as not-ours (eslint `ignores`;
the `.js` files carry no types so only the `.d.ts` shims are checked).

`foliate-js`'s default rendering path — unzip the EPUB in the client and
serve entries as `blob:` URLs — is **not used**. Its own documentation
states that path "is currently impossible to do securely... due to the
content being served from the same origin." Instead, every resource the
renderer fetches points at `backend-reader-content.md`'s content
endpoint on this system's own API origin, which serves already-sanitised
content (ADR 0024) under a strict `Content-Security-Policy`. The
renderer runs inside an `<iframe sandbox="allow-same-origin">` with **no**
`allow-scripts` (`frontend-reader.md` FR-1).

## Options considered

### Option A — `foliate-js` (chosen)

*For* — real EPUB CFI support via a dedicated, standalone module
(`epubcfi.js`); an EPUB structure parser (`epub.js`) that works through
a caller-supplied loader, so it reads the OPF/nav via the sanitised
content endpoint and never unzips client-side; no hard dependencies;
modular enough that a future replacement could swap the rendering layer
while keeping the CFI-shaped position data, since CFI is a W3C/IDPF
standard rather than the library's invention. The full `<foliate-view>`
CSS-multi-column paginator is available for a later pass; the first cut
renders one spine document per `<iframe>` load.

*Against* — its own README warns it is not stable and "may break... at
any time"; its documented default rendering approach is not securely
sandboxable and must be deliberately avoided (done here via server-side
sanitisation + same-origin serving). Both are named honestly in the
phase 11 roadmap Risks table.

### Option B — `futurepress/epub.js`

*For* — larger install base, longer history, iframe-based with scripts
disabled by default.

*Against* — its own maintainers' forks cite architectural problems with
its layout-rebuild model; CFI support is present but less cleanly
separable than `foliate-js`'s dedicated module. Not obviously more
stable in practice — its issue tracker carries long-standing
pagination-correctness bugs.

### Option C — hand-rolled pagination + CFI

*For* — zero third-party rendering dependency; total control.

*Against* — correct CFI generation against arbitrary EPUB DOM structure
and correct CSS-multi-column pagination are each a substantial project
with a long tail of correctness bugs a maintained library has already
worked through. Rejected: this is exactly the "large amount of code you
don't want to own" §9 warns against, and unlike phase 10's parser, there
is no small correct core to hand-roll.

## Consequences

**Good** — the reader gets standards-based position addressing (CFI)
without the project inventing one; both reading modes come from one
library; the phase-14 sync work inherits portable position data.

**Bad** — a real, named upstream-instability risk: a `foliate-js` change
could cause a regression, and there is no fallback engine staged. The
mitigation is "pin exactly, watch the changelog, upgrade deliberately,"
not a second implementation. We also carry a permanent discipline
requirement: the `blob:`-URL path must never be re-enabled for
convenience, and the iframe sandbox must never gain `allow-scripts`.

**Neutral** — the server now owns EPUB unzipping and per-resource
serving (ADR 0024, `backend-reader-content.md`) rather than the client,
which is more backend work but is the thing that closes the sandboxing
gap.

## Reversal cost

Medium. The CFI position data is a standard and would survive an engine
swap; the rendering integration code (`web/src/screens/Reader/`) would
be rewritten against a new library's API. No stored data migration.
What would force the question open: `foliate-js` becoming unmaintained,
or a breaking change we cannot pin our way around.

## Confidence

Medium. The capability fit is high — `foliate-js` does exactly what the
reader needs and its CFI module is the cleanest available. The stability
concern is real and acknowledged; this is a judgement that a maintained
library's known instability is still a better bet than owning the CFI
and pagination problem outright.
