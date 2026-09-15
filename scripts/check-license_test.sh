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

# 5. The root package-lock.json entry (keyed by "") and every workspace
# member entry (keyed by its own directory, e.g. "web", "electron") carry
# this project's own AGPL license and must not trip the copyleft gate.
# npm 11+ writes a `license` field on these local-package entries (older
# npm didn't), which previously made the checker flag the project's own
# license against itself. Neither has a `resolved` field — nothing was
# fetched for them — which is what distinguishes them from a real
# dependency below.
d="$(mktemp -d)"
mkdir -p "$d/web" "$d/electron"
agpl_header > "$d/LICENSE"
pkg root AGPL-3.0-or-later > "$d/package.json"
pkg web AGPL-3.0-or-later > "$d/web/package.json"
pkg desktop AGPL-3.0-or-later > "$d/electron/package.json"
cat > "$d/package-lock.json" <<'EOF'
{
  "name": "alexandryn",
  "packages": {
    "": {
      "name": "alexandryn",
      "license": "AGPL-3.0-or-later"
    },
    "web": {
      "name": "web",
      "license": "AGPL-3.0-or-later",
      "dependencies": {}
    },
    "electron": {
      "name": "@alexandryn/desktop",
      "license": "AGPL-3.0-or-later",
      "devDependencies": {}
    },
    "node_modules/some-dep": {
      "resolved": "https://registry.npmjs.org/some-dep/-/some-dep-1.0.0.tgz",
      "license": "MIT"
    }
  }
}
EOF
if out="$("$CHECKER" "$d" 2>&1)"; then echo "ok - own AGPL license on root and workspace-member entries does not trip the copyleft gate"; else echo "FAIL - local-package entries: $out"; fail=1; fi

# 6. A real copyleft *dependency* (has `resolved`, unlike the local
# entries above) still fails.
d="$(mktemp -d)"
mkdir -p "$d/web" "$d/electron"
agpl_header > "$d/LICENSE"
pkg root AGPL-3.0-or-later > "$d/package.json"
pkg web AGPL-3.0-or-later > "$d/web/package.json"
pkg desktop AGPL-3.0-or-later > "$d/electron/package.json"
cat > "$d/package-lock.json" <<'EOF'
{
  "name": "alexandryn",
  "packages": {
    "": {
      "name": "alexandryn",
      "license": "AGPL-3.0-or-later"
    },
    "node_modules/copyleft-dep": {
      "resolved": "https://registry.npmjs.org/copyleft-dep/-/copyleft-dep-1.0.0.tgz",
      "license": "GPL-3.0"
    }
  }
}
EOF
if "$CHECKER" "$d" >/dev/null 2>&1; then echo "FAIL - real GPL production dependency should fail"; fail=1; else echo "ok - real copyleft dependency still fails"; fi

[ "$fail" -eq 0 ] && echo "check-license_test.sh: all cases passed" || { echo "check-license_test.sh: FAILED"; exit 1; }
