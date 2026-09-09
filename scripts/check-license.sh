#!/usr/bin/env bash
# Licence presence and consistency (audit 0016 #130, ADR 0002). The repo
# must carry the verbatim AGPL-3.0-or-later text at LICENSE, and every
# published package.json must declare the same SPDX identifier so tooling
# and downstreams agree with the file.
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

echo "check-license: LICENSE present (AGPL-3.0), package.json SPDX consistent — ok"
