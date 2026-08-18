#!/usr/bin/env bash
# Fixture proof for check-parameterized-queries.sh (backend-persistence.md
# FR-3): a parameterized query passes; fmt.Sprintf and string
# concatenation around a SQL-shaped string both fail, naming the file.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-parameterized-queries.sh"

fail=0

new_fixture() {
	local d
	d="$(mktemp -d)"
	mkdir -p "$d/internal/persistence/postgres"
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
cat >"$d/internal/persistence/postgres/work_repository.go" <<'EOF'
package postgres

import "context"

func (r *WorkRepository) byID(ctx context.Context, id string) error {
	_, err := r.pool.Query(ctx, "SELECT id, title FROM works WHERE id = $1", id)
	return err
}
EOF
assert_pass "a parameterized query, values passed as separate arguments" "$d"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/work_repository.go" <<'EOF'
package postgres

import (
	"context"
	"fmt"
)

func (r *WorkRepository) byTitle(ctx context.Context, title string) error {
	q := fmt.Sprintf("SELECT id FROM works WHERE title = '%s'", title)
	_, err := r.pool.Query(ctx, q)
	return err
}
EOF
assert_fail "fmt.Sprintf building a SQL-shaped string" "$d" "work_repository.go"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/work_repository.go" <<'EOF'
package postgres

import "context"

func (r *WorkRepository) byTitle(ctx context.Context, title string) error {
	q := "SELECT id FROM works WHERE title = '" + title + "'"
	_, err := r.pool.Query(ctx, q)
	return err
}
EOF
assert_fail "string concatenation around a SQL-shaped literal" "$d" "work_repository.go"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/fixture.go" <<'EOF'
package postgres

// fixtureSource embeds another file's source as fixture data — a
// SELECT/fmt.Sprintf-shaped string that is not this file's own code, so
// the checker must not flag it.
var fixtureSource = `
q := fmt.Sprintf("SELECT id FROM works WHERE title = '%s'", title)
`
EOF
assert_pass "raw-string fixture data is not this file's own code" "$d"

if [ "$fail" -ne 0 ]; then
	echo "check-parameterized-queries_test.sh: FAILED"
	exit 1
fi
echo "check-parameterized-queries_test.sh: all cases passed"
