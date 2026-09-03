#!/usr/bin/env bash
# Reading-authorization discipline check (backend-reading-api.md FR-9,
# review 0050 / audit 0012-C1). Every handler serving data a user owns —
# reading progress, bookmarks, highlights, preferences, export — MUST
# resolve the authenticated user and active library and call the
# user-and-library-scoped repository method (…AndUser / SaveForUser /
# DeleteAndUser / FindByUserAndDevice), never a bare-id or bare-work
# variant. The scoped method existing is not the control; the handler
# calling it is (CLAUDE.md Reflex).
#
# Grep-based and heuristic, same interim spirit as
# check-parameterized-queries.sh. It flags a bare call on one of the
# reading-API deps fields in the reading/reader transport files.
set -euo pipefail

ROOT="${1:-.}"
DIR="$ROOT/internal/transport/http"
FILES=(reading.go reading_export.go reader_content.go)

# Forbidden: `<recv>.<method>(` where recv is a reading-data deps field
# and method is a non-user-scoped variant. The user-scoped forms
# (…AndUser, SaveForUser, DeleteAndUser, FindByUserAndDevice) are the
# only ones a handler may call.
forbidden=(
	'deps\.Progress\.FindByWork\('
	'deps\.Progress\.FindByWorkForUpdate\('
	'deps\.Progress\.Save\('
	'deps\.Bookmarks\.FindByID\('
	'deps\.Bookmarks\.FindByEdition\('
	'deps\.Bookmarks\.Save\('
	'deps\.Bookmarks\.Delete\('
	'deps\.Highlights\.FindByID\('
	'deps\.Highlights\.FindByEdition\('
	'deps\.Highlights\.Save\('
	'deps\.Highlights\.Delete\('
	'deps\.Preferences\.FindByDevice\('
	'deps\.Preferences\.Save\('
	'deps\.Export\.ListProgress\(r\.Context\(\), *"'
)

violations=""

for f in "${FILES[@]}"; do
	path="$DIR/$f"
	[ -f "$path" ] || continue
	for pat in "${forbidden[@]}"; do
		while IFS= read -r line; do
			violations="${violations}${f}: bare (non-user-scoped) reading-repository call — use the …AndUser / SaveForUser / DeleteAndUser variant (backend-reading-api.md FR-9): ${line# }
"
		done < <(grep -nE "$pat" "$path" || true)
	done
done

if [ -n "$violations" ]; then
	echo "check-user-scoped-reading: violations found:" >&2
	printf '%s' "$violations" >&2
	exit 1
fi

echo "check-user-scoped-reading: clean"
