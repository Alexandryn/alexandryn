#!/usr/bin/env bash
# Fixture proof for check-user-scoped-reading.sh (backend-reading-api.md
# FR-9): a handler calling the user-scoped repository method passes; a
# handler calling a bare method fails, naming the file.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-user-scoped-reading.sh"

fail=0

new_fixture() {
	local d
	d="$(mktemp -d)"
	mkdir -p "$d/internal/transport/http"
	echo "$d"
}

assert_pass() {
	local desc="$1" dir="$2"
	if out="$("$CHECKER" "$dir" 2>&1)"; then
		echo "ok - $desc"
	else
		echo "FAIL - $desc: expected pass, checker rejected a clean tree"
		echo "$out"
		fail=1
	fi
}

assert_fail() {
	local desc="$1" dir="$2" want="$3"
	if out="$("$CHECKER" "$dir" 2>&1)"; then
		echo "FAIL - $desc: expected the checker to reject this tree, it passed"
		fail=1
	else
		if grep -q -- "$want" <<<"$out"; then
			echo "ok - $desc"
		else
			echo "FAIL - $desc: checker failed but didn't name the violation (\"$want\")"
			echo "$out"
			fail=1
		fi
	fi
}

d="$(new_fixture)"
cat >"$d/internal/transport/http/reading.go" <<'EOF'
package http

func h() {
	_, _ = deps.Bookmarks.FindByEditionAndUser(ctx, userID, libID, e)
	_ = deps.Bookmarks.SaveForUser(ctx, userID, libID, b)
	_ = deps.Bookmarks.DeleteAndUser(ctx, userID, id)
	_ = deps.Preferences.SaveForUser(ctx, userID, p)
	_, _ = deps.Editions.FindByID(ctx, editionID) // EditionRepository, not user-owned — fine
}
EOF
assert_pass "handler calls only the user-scoped variants" "$d"

d="$(new_fixture)"
cat >"$d/internal/transport/http/reading.go" <<'EOF'
package http

func h() {
	_ = deps.Bookmarks.Delete(ctx, id)
}
EOF
assert_fail "handler calls the bare Delete" "$d" "reading.go"

d="$(new_fixture)"
cat >"$d/internal/transport/http/reading_export.go" <<'EOF'
package http

func h() {
	_, _ = deps.Export.ListProgress(r.Context(), "some-work")
}
EOF
assert_fail "export handler calls ListProgress with only a workID" "$d" "reading_export.go"

if [ "$fail" -ne 0 ]; then
	echo "check-user-scoped-reading_test.sh: FAILED"
	exit 1
fi
echo "check-user-scoped-reading_test.sh: all cases passed"
