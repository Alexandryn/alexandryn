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

# Catalog surface (audit 0016 #88, #133). GET /api/v1/library and
# GET /api/v1/works/{id} serve holdings scoped to the active library.
# The handlers MUST resolve it, and work_repository.go's QueryLibrary /
# FindWorkDetail SQL MUST carry a library_id predicate — the exact seam
# audit 0012's per-phase certification missed.
LIBRARY_HANDLER="$ROOT/internal/transport/http/library.go"
if [ -f "$LIBRARY_HANDLER" ]; then
	if ! grep -q 'ActiveLibraryFromContext' "$LIBRARY_HANDLER"; then
		violations="${violations}library.go: catalog handlers do not resolve ActiveLibraryFromContext — GET /library and GET /works/{id} must be library-scoped (audit 0016 #88)
"
	fi
fi

WORK_REPO="$ROOT/internal/persistence/postgres/work_repository.go"
if [ -f "$WORK_REPO" ]; then
	for fn in QueryLibrary FindWorkDetail; do
		block=$(awk -v f="func (r *WorkRepository) $fn" 'index($0,f){flag=1} flag{print} flag && /^}/{exit}' "$WORK_REPO")
		if [ -n "$block" ] && ! grep -qE 'library_id = \$' <<<"$block"; then
			violations="${violations}work_repository.go: $fn has no 'library_id = \$N' predicate — cross-library holdings disclosure (audit 0016 #88)
"
		fi
	done
fi

# Verify raw SQL queries in reading_sync_repository.go (GetReadingSyncData) enforce user_id and library_id scoping
SYNC_REPO="$ROOT/internal/persistence/postgres/reading_sync_repository.go"
if [ -f "$SYNC_REPO" ]; then
	for table in reading_progress bookmarks highlights; do
		query_block=$(awk -v t="$table" '$0 ~ "FROM " t {flag=1} flag; $0 ~ "ORDER BY" {flag=0}' "$SYNC_REPO" | head -n 5)
		if [ -n "$query_block" ]; then
			if ! echo "$query_block" | grep -q 'user_id ='; then
				violations="${violations}reading_sync_repository.go: query on $table in GetReadingSyncData missing user_id scoping
"
			fi
			if ! echo "$query_block" | grep -q 'library_id ='; then
				violations="${violations}reading_sync_repository.go: query on $table in GetReadingSyncData missing library_id scoping
"
			fi
		fi
	done
fi

if [ -n "$violations" ]; then
	echo "check-user-scoped-reading: violations found:" >&2
	printf '%s' "$violations" >&2
	exit 1
fi

echo "check-user-scoped-reading: clean"
