#!/usr/bin/env bash
# Fixture test for check-license.sh.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-license.sh"
fail=0

agpl_header() {
	printf '                    GNU AFFERO GENERAL PUBLIC LICENSE\n                       Version 3, 19 November 2007\n\n  ...\n'
}

pkg() { printf '{\n  "name": "%s",\n  "license": "%s"\n}\n' "$1" "$2"; }

# 1. A well-formed tree passes.
d="$(mktemp -d)"
mkdir -p "$d/web" "$d/electron"
agpl_header > "$d/LICENSE"
pkg root AGPL-3.0-or-later > "$d/package.json"
pkg web AGPL-3.0-or-later > "$d/web/package.json"
pkg desktop AGPL-3.0-or-later > "$d/electron/package.json"
if out="$("$CHECKER" "$d" 2>&1)"; then echo "ok - well-formed tree passes"; else echo "FAIL - well-formed: $out"; fail=1; fi

# 2. Missing LICENSE fails.
d="$(mktemp -d)"
pkg root AGPL-3.0-or-later > "$d/package.json"
if "$CHECKER" "$d" >/dev/null 2>&1; then echo "FAIL - missing LICENSE should fail"; fail=1; else echo "ok - missing LICENSE fails"; fi

# 3. Wrong SPDX in a package.json fails.
d="$(mktemp -d)"
mkdir -p "$d/web"
agpl_header > "$d/LICENSE"
pkg root AGPL-3.0-or-later > "$d/package.json"
pkg web MIT > "$d/web/package.json"
if "$CHECKER" "$d" >/dev/null 2>&1; then echo "FAIL - MIT in web/package.json should fail"; fail=1; else echo "ok - inconsistent SPDX fails"; fi

# 4. LICENSE that is not AGPL fails.
d="$(mktemp -d)"
printf 'MIT License\n' > "$d/LICENSE"
pkg root AGPL-3.0-or-later > "$d/package.json"
if "$CHECKER" "$d" >/dev/null 2>&1; then echo "FAIL - MIT LICENSE text should fail"; fail=1; else echo "ok - non-AGPL LICENSE text fails"; fi

[ "$fail" -eq 0 ] && echo "check-license_test.sh: all cases passed" || { echo "check-license_test.sh: FAILED"; exit 1; }
