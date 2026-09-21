# 0024. Served EPUB HTML/XHTML is sanitised server-side with `microcosm-cc/bluemonday`; CSS is scanned by a hand-written check

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-01 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

`backend-reader-content.md` serves individual EPUB spine items and
resources back to the reader client, rendered as real HTML/CSS in a
browser. This is the first place the project hands parsed file content
*back* to a caller rather than only extracting a summary from it
server-side, so the sanitisation stakes are high. EPUB content is
untrusted (constitution §4) — a book file can carry `<script>`, event
handlers, `javascript:` URIs, external tracking pixels, and SVG with its
own script-execution surface.

The spec (FR-6/FR-7) already picks `bluemonday` for HTML and a
hand-written scanner for CSS, with a §9 justification. This ADR records
the choice as a decision because sanitisation is the load-bearing
security control for the whole reader feature, and because it pairs with
ADR 0023: server-side sanitisation is specifically what lets the reader
avoid `foliate-js`'s insecure `blob:`-URL default and serve content from
this system's own origin under a plain `'self'` CSP.

Go's standard library has no HTML sanitiser. Hand-rolling one correctly —
nested tags, attribute-value edge cases, encoding tricks that smuggle a
script past a naive filter — is a well-known way to introduce the exact
XSS this control exists to prevent.

## Decision

**HTML/XHTML** served by `backend-reader-content.md` is sanitised
server-side with **`microcosm-cc/bluemonday`**, exact-version pinned in
`go.mod`. The policy is built from `bluemonday.UGCPolicy()` (which
already strips `<script>` and event-handler attributes), further
restricted to:

- reject any `href`/`src` value that is not a relative path or a
  `data:` URI — no absolute `http(s)://` reference survives;
- strip `<iframe>`, `<object>`, `<embed>`, and external-fetch-triggering
  `<link>` elements entirely;
- strip inline `<svg>` wholesale (element and contents);
- strip `style` attributes entirely;
- route `<style>` block text through the CSS check below before keeping
  it.

**CSS** (standalone `text/css` resources and `<style>` block contents) is
**not** run through a third-party parser. A small hand-written check
scans for `url(...)` and `@import` values and removes any whose URL has a
scheme other than `data:` (defined as "has a `scheme:` prefix at all,
and it isn't `data:`" — which also rejects `url(javascript:...)`). A
value with no scheme (a genuine relative reference) passes unchanged.

Every response carries `Content-Security-Policy: default-src 'self';
script-src 'none'; object-src 'none'` (FR-9) as an independent second
layer.

## Options considered

### Option A — `bluemonday` for HTML, hand-written scanner for CSS (chosen)

*For* — `bluemonday` is an allowlist sanitiser built on Go's own
`net/html` tokenizer (not a third-party parser), modelled on the OWASP
Java HTML Sanitizer's well-regarded design. The policy object is built
once and applied per request; swapping it is one function call's
implementation. CSS's needs here are narrow — "does this one construct
reference anything other than a relative path or `data:`" — so a bounded
token scan is proportionate and avoids pulling in a full CSS parser.

*Against* — one more dependency to track and pin; the split (library for
HTML, hand-written for CSS) is a small inconsistency to explain.

### Option B — hand-roll both

*For* — no new dependency at all.

*Against* — rejected outright for HTML: hand-rolled HTML sanitisation is
a classic source of XSS. The surface (encoding, nesting, attribute
parsing) is exactly what a maintained allowlist library exists to get
right.

### Option C — a full CSS parser for the CSS half

*For* — more thorough than a token scan; would catch exotic constructs.

*Against* — disproportionate. The threat is external resource references
and non-`data:` schemes; a scoped scan covers it. A full parser is a
larger dependency and a larger attack surface for a narrower need.
Recorded as the thing to revisit if real hostile CSS shows the scan is
insufficient.

### Option D — sanitise client-side only

*For* — no backend work.

*Against* — rejected: `foliate-js`'s own maintainer documents that
client-side-only sanitisation of same-origin `blob:` content "is
currently impossible to do securely." The whole point of serving from
the API origin (ADR 0023) is to sanitise before the client ever sees
the bytes.

## Consequences

**Good** — the reader's central security control is an allowlist
(deny-by-default) rather than a blocklist; it runs before content
reaches the browser; it pairs with the CSP so neither layer is trusted
alone. Standalone SVG and inline `<svg>` are refused rather than
partially cleaned, which is the honest choice given SVG's separate
script surface.

**Bad** — a real, named feature loss: an EPUB cover delivered as
standalone `.svg` will not render (`backend-reader-content.md` FR-5, Open
questions). Sanitisation also costs CPU per served HTML resource, though
FR-3's content cache keeps repeat navigation cheap. One more pinned
dependency.

**Neutral** — the HTML/CSS split means two code paths for "clean this
content," documented in the spec so it is not mistaken for an oversight.

## Reversal cost

Low. The policy is a self-contained object built in one place; replacing
`bluemonday` with another Go sanitiser is a localised change, not a
structural one. The CSS scanner is a few dozen lines we own.

## Confidence

High. This is the well-trodden answer to a well-understood problem:
allowlist sanitiser on a real tokenizer for HTML, scoped check for the
narrow CSS case, CSP as defence in depth. The only genuinely uncertain
piece — whether the scoped CSS scan is enough — is flagged for revisit
rather than over-engineered now.
