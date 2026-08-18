# 0019. Config-file parsing uses `pelletier/go-toml/v2`

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-18 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

`backend-configuration.md` FR-5 fixed the config file's format as TOML,
but named no library — phase 01's own "architecture decisions expected"
list didn't carry this item either, since the config spec (and its TOML
requirement) didn't exist yet when that list was written. Found while
implementing FR-5's file-resolution logic (`tasks/plan.md` T4): no ADR,
spec, or audit in the corpus names a TOML library, and `go.mod` had no
dependencies of any kind before this one.

`internal/config`'s own design resolves each key through one shared,
per-key parse function regardless of which source (default, file,
environment) supplied the raw value (T3's `fieldSpec` table,
`internal/config/config.go`) — so the file source only needs to become a
`map[string]string`-shaped (or coercible) lookup the same per-key parse
functions already handle, not a fully-typed `Config` struct decoded
directly from TOML.

## Decision

The config file is parsed with `github.com/pelletier/go-toml/v2`,
decoded into a `map[string]any` keyed by top-level TOML key, and each
present key's raw value converted to a string before being handed to the
same `fieldSpec.parse` function the environment source already uses.

## Options considered

### Option A — `pelletier/go-toml/v2` (chosen)

*For* — TOML 1.0.0 spec-compliant, actively maintained, and its `go-toml`
v2 rewrite is specifically the faster, lower-allocation successor to v1
(the go-toml/v2 project's own stated reason for the rewrite). Decoding
into a `map[string]any` is a first-class, documented use case, matching
this package's own per-key lookup design directly rather than fighting
it. Small dependency surface: one module, no transitive dependencies of
its own beyond the standard library.

*Against* — Smaller mindshare than `BurntSushi/toml` historically,
though both are still active and TOML 1.0.0-compliant today.

### Option B — `BurntSushi/toml`

*For* — The older and historically most-used Go TOML library, so a
future contributor is more likely to have seen it before. Also TOML
1.0.0-compliant and actively maintained, with a comparably simple
decode-to-map API.

*Against* — No functional advantage over Option A for this package's
actual use case (map decoding); the deciding factor was Option A's
stated performance work and this project's general preference (ADR
0011, 0012) for the more actively-optimized option when the two
candidates are otherwise close. Not a wide gap — recorded because it's
the real alternative, not to overstate the difference.

### Option C — Hand-rolled TOML parsing

*Against* — TOML is not a small format (nested tables, arrays, multiple
string quoting rules, typed scalars); reimplementing a spec-compliant
parser to save one small dependency reproduces exactly the "large
amount of code you don't own" constitution §9 warns against, in
reverse — a hand-rolled parser is more code to own, not less.

## Consequences

**Good** — `internal/config`'s file source becomes a single
`map[string]any` lookup feeding the same per-key `parse` functions the
environment source already uses (T3) — no second, TOML-specific
validation path to keep in sync with the first.

**Bad** — A new dependency to justify under constitution §9. What it
does: parses TOML text into Go values. Why not the standard library:
`encoding/toml` doesn't exist — Go's standard library has no TOML
support, unlike `encoding/json`. What breaks if it's abandoned: low
risk — TOML is a stable, versioned spec (1.0.0), the dependency surface
is one module with no transitive dependencies, and a decode-to-map
usage is small enough to reimplement or replace with the alternative
library (Option B) without touching `internal/config`'s own per-key
resolution logic if it ever became necessary.

**Neutral** — Doesn't change FR-5's own file-location or precedence
rules, only which code parses the file's bytes once resolved.

## Reversal cost

Low. Both this package's own code and the `Config` struct are
insulated from the parser by the `map[string]any` boundary — replacing
the library means changing one function, not the per-key resolution
logic T3 already built.

## Confidence

High. TOML library choice is a narrow, mechanical decision once the
format itself was fixed (FR-5); the two credible options are close
enough that either would have worked, and Option A's stated performance
focus was a reasonable, low-stakes tiebreaker.
