#!/usr/bin/env bash
# Licence presence and consistency check. The repository must carry the
# verbatim AGPL-3.0-or-later text at LICENSE, and every package.json must
# declare the same SPDX identifier.
set -euo pipefail

ROOT="${1:-.}"
SPDX="AGPL-3.0-or-later"

fail() {
	echo "check-license: $1" >&2
	exit 1
}

[ -f "$ROOT/LICENSE" ] || fail "no LICENSE file at the repository root"

head -1 "$ROOT/LICENSE" | grep -qi "GNU AFFERO GENERAL PUBLIC LICENSE" \
	|| fail "LICENSE is not the GNU Affero General Public License text"
grep -q "Version 3, 19 November 2007" "$ROOT/LICENSE" \
	|| fail "LICENSE is not AGPL version 3"

for pkg in "$ROOT/package.json" "$ROOT/web/package.json" "$ROOT/electron/package.json"; do
	[ -f "$pkg" ] || continue
	grep -q "\"license\"[[:space:]]*:[[:space:]]*\"$SPDX\"" "$pkg" \
		|| fail "$pkg does not declare \"license\": \"$SPDX\""
done

# Dependency license gate for production npm packages.
# Fails if any production package uses copyleft licenses (GPL, AGPL, LGPL).
if [ -f "$ROOT/package-lock.json" ] && command -v python3 >/dev/null 2>&1; then
	python3 -c "
import json, sys
with open('$ROOT/package-lock.json') as f:
    d = json.load(f)
for name, pkg in d.get('packages', {}).items():
    if pkg.get('dev', False):
        continue
    lic = str(pkg.get('license', ''))
    for bad in ['GPL', 'AGPL', 'LGPL']:
        if bad in lic:
            print(f'check-license: forbidden copyleft license {lic} in production npm package {name}', file=sys.stderr)
            sys.exit(1)
" || fail "copyleft dependency found in package-lock.json"
fi

echo "check-license: LICENSE present (AGPL-3.0), package.json SPDX consistent, production deps clean — ok"
