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
# Every non-test handler file on the reading/reader transport surface —
# a glob, not a fixed list, so a new handler file is covered
# automatically. Excludes the *_ref.go / *_test.go plumbing.
mapfile -t FILES < <(cd "$DIR" 2>/dev/null && ls reading*.go reader_content*.go device_sync*.go 2>/dev/null | grep -Ev '_test\.go$|_ref\.go$' || true)

# Forbidden: `<recv>.<method>(` where recv is a reading-data deps field
# and method is a non-user-scoped variant. The user-scoped forms
# (…AndUser, SaveForUser, DeleteAndUser, FindByUserAndDevice) are the
# only ones a handler may call.
forbidden=(
	'(deps\.Progress|progressRepo)\.FindByWork\('
	'(deps\.Progress|progressRepo)\.FindByWorkForUpdate\('
	'(deps\.Progress|progressRepo)\.Save\('
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
	# Export: reject a call that passes only (ctx, workID) — the
	# user-scoped signature takes (ctx, userID, libraryID, workID). The
	# compile-time signature change is the primary guard; this is
	# defence in depth against a future re-loosening.
	'deps\.Export\.ListProgress\([^,]*, *"'
	'deps\.Export\.ListMarks\([^,]*, *"'
	'deps\.Export\.ListProgress\([^,]*, *workID *\)'
	'deps\.Export\.ListMarks\([^,]*, *workID *\)'
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
