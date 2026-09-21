#!/usr/bin/env bash
# Fixture test for release-notes.sh: it prints exactly one version's
# section, stops at the next version heading, and fails loudly (never
# prints empty notes) when the version is missing, empty, or malformed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOL="$SCRIPT_DIR/release-notes.sh"
fail=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/CHANGELOG.md" <<'MD'
# Changelog

## [Unreleased]

Nothing yet.

## [1.1.0] - 2026-10-01

### Added

- Second thing.

## [1.0.0] - 2026-09-30

Initial public release.

### Added

- First thing.

## [0.9.0] - 2026-09-01

- Older thing.
MD

ok() { echo "ok - $1"; }
bad() { echo "FAIL - $1"; fail=1; }

out="$("$TOOL" 1.0.0 "$tmp/CHANGELOG.md")"
if grep -q "First thing" <<<"$out" && grep -q "Initial public release" <<<"$out"; then ok "prints the requested section"; else bad "prints the requested section"; fi
if grep -q "Second thing\|Older thing" <<<"$out"; then bad "does not leak neighbouring sections"; else ok "does not leak neighbouring sections"; fi
if grep -q '^## \[' <<<"$out"; then bad "omits the heading line itself"; else ok "omits the heading line itself"; fi

out="$("$TOOL" v1.1.0 "$tmp/CHANGELOG.md")"
if grep -q "Second thing" <<<"$out"; then ok "accepts a leading v (a tag name)"; else bad "accepts a leading v"; fi

if "$TOOL" 2.0.0 "$tmp/CHANGELOG.md" >/dev/null 2>&1; then bad "fails for a version that has no section"; else ok "fails for a version that has no section"; fi

printf '# Changelog\n\n## [3.0.0] - Unreleased\n\n## [2.9.0] - 2026-01-01\n\n- x\n' >"$tmp/empty.md"
if "$TOOL" 3.0.0 "$tmp/empty.md" >/dev/null 2>&1; then bad "fails for an empty section"; else ok "fails for an empty section"; fi

if "$TOOL" "1.0.0; rm -rf /" "$tmp/CHANGELOG.md" >/dev/null 2>&1; then bad "rejects a malformed version"; else ok "rejects a malformed version"; fi
if "$TOOL" 1.0.0 "$tmp/missing.md" >/dev/null 2>&1; then bad "fails when the changelog is missing"; else ok "fails when the changelog is missing"; fi
if "$TOOL" >/dev/null 2>&1; then bad "fails without arguments"; else ok "fails without arguments"; fi

# A dotted version must not match by regex wildcard: 1x0x0 is not 1.0.0.
printf '# C\n\n## [1x0x0] - 2026-01-01\n\n- wrong\n' >"$tmp/wild.md"
if "$TOOL" 1.0.0 "$tmp/wild.md" >/dev/null 2>&1; then bad "matches the version literally, not as a pattern"; else ok "matches the version literally, not as a pattern"; fi

exit "$fail"
