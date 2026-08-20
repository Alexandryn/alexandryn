#!/usr/bin/env bash
# Fixture proof for check-integration-test-parallelism.sh
# (backend-test-harness.md FR-3): an _integration_test.go file without
# t.Parallel() passes; one that calls it fails, naming the file. A
# non-integration test file calling t.Parallel() is out of this check's
# scope entirely (nothing to flag). Raw-string fixture data embedding the
# literal text isn't this file's own code.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-integration-test-parallelism.sh"

fail=0

new_fixture() {
	local d
	d="$(mktemp -d)"
	mkdir -p "$d/internal/testutil"
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
cat >"$d/internal/testutil/isolation_integration_test.go" <<'EOF'
//go:build integration

package testutil_test

import "testing"

func TestIsolationA(t *testing.T) {
	t.Log("no parallelism here")
}
EOF
assert_pass "an _integration_test.go file with no t.Parallel()" "$d"

d="$(new_fixture)"
cat >"$d/internal/testutil/isolation_integration_test.go" <<'EOF'
//go:build integration

package testutil_test

import "testing"

func TestIsolationA(t *testing.T) {
	t.Parallel()
}
EOF
assert_fail "an _integration_test.go file that calls t.Parallel()" "$d" "isolation_integration_test.go"

d="$(new_fixture)"
cat >"$d/internal/testutil/isolation_integration_test.go" <<'EOF'
//go:build integration

package testutil_test

import "testing"

func TestIsolationA(t *testing.T) {
	tc := t
	tc.Parallel()
}
EOF
assert_fail "a table-driven subtest receiver calling .Parallel()" "$d" "isolation_integration_test.go"

d="$(new_fixture)"
cat >"$d/internal/testutil/clock_test.go" <<'EOF'
package testutil_test

import "testing"

func TestClock(t *testing.T) {
	t.Parallel()
}
EOF
assert_pass "t.Parallel() in a non-integration test file is out of this check's scope" "$d"

d="$(new_fixture)"
cat >"$d/internal/testutil/isolation_integration_test.go" <<'EOF'
//go:build integration

package testutil_test

// fixtureSource embeds another file's source as fixture data — a
// parallelism-call-shaped string that is not this file's own code, so
// the checker must not flag it.
var fixtureSource = `
func TestIsolationA(t *testing.T) {
	t.Parallel()
}
`
EOF
assert_pass "raw-string fixture data is not this file's own code" "$d"

if [ "$fail" -ne 0 ]; then
	echo "check-integration-test-parallelism_test.sh: FAILED"
	exit 1
fi
echo "check-integration-test-parallelism_test.sh: all cases passed"
