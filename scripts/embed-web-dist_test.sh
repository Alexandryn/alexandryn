#!/usr/bin/env bash
# Fixture test for embed-web-dist.sh: a real web/dist gets copied over
# the placeholder untouched; a missing web/dist fails loudly instead of
# silently leaving the committed placeholder in place.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EMBEDDER="$SCRIPT_DIR/embed-web-dist.sh"

fail=0

new_fixture_repo() {
	local d
	d="$(mktemp -d)"
	mkdir -p "$d/internal/transport/http/webdist/placeholder"
	cat >"$d/internal/transport/http/webdist/placeholder/index.html" <<'EOF'
<html><body>Alexandryn — placeholder build.</body></html>
EOF
	echo "$d"
}

d="$(new_fixture_repo)"
mkdir -p "$d/web/dist/assets"
echo '<html><body><div id="root"></div></body></html>' >"$d/web/dist/index.html"
echo "console.log('app')" >"$d/web/dist/assets/app.js"

if out="$("$EMBEDDER" "$d" 2>&1)"; then
	if grep -q 'id="root"' "$d/internal/transport/http/webdist/placeholder/index.html" \
		&& [[ -f "$d/internal/transport/http/webdist/placeholder/assets/app.js" ]] \
		&& ! grep -q 'placeholder build' "$d/internal/transport/http/webdist/placeholder/index.html"; then
		echo "ok - real web/dist replaces the placeholder"
	else
		echo "FAIL - real web/dist replaces the placeholder: target still looks like the old placeholder"
		fail=1
	fi
else
	echo "FAIL - real web/dist replaces the placeholder: script rejected a valid web/dist"
	echo "$out"
	fail=1
fi

d="$(new_fixture_repo)"
if out="$("$EMBEDDER" "$d" 2>&1)"; then
	echo "FAIL - missing web/dist: expected the script to fail, it succeeded"
	fail=1
else
	if grep -q "web/dist/index.html not found" <<<"$out"; then
		echo "ok - missing web/dist fails loudly, naming the missing path"
	else
		echo "FAIL - missing web/dist: failed but didn't name the problem"
		echo "$out"
		fail=1
	fi
fi
if grep -q 'placeholder build' "$d/internal/transport/http/webdist/placeholder/index.html" 2>/dev/null; then
	: # placeholder correctly left untouched on failure
else
	echo "FAIL - missing web/dist: the committed placeholder was modified despite the script failing"
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	echo "embed-web-dist_test.sh: FAILED"
	exit 1
fi
echo "embed-web-dist_test.sh: all cases passed"
