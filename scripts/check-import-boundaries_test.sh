#!/usr/bin/env bash
# Fixture proof for check-import-boundaries.sh: a clean tree passes, an
# introduced violation of each of the three rules fails with a message
# naming the offending file. D0 (tasks/plan.md) — interim grep-based check,
# revisited once golangci-lint/go-analysis can carry it (ADR 0018).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-import-boundaries.sh"

fail=0

new_fixture() {
	local d
	d="$(mktemp -d)"
	mkdir -p "$d/internal/domain" "$d/internal/transport/http" "$d/internal/persistence/postgres" "$d/internal/config"
	echo "$d"
}

assert_pass() {
	local desc="$1" dir="$2"
	if "$CHECKER" "$dir" >/tmp/checker-out.$$ 2>&1; then
		echo "ok - $desc"
	else
		echo "FAIL - $desc: expected pass, checker rejected a clean tree"
		cat /tmp/checker-out.$$
		fail=1
	fi
	rm -f /tmp/checker-out.$$
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

# --- Rule (a): internal/domain must not import transport or persistence ---

d="$(new_fixture)"
cat >"$d/internal/domain/work.go" <<'EOF'
package domain

type Work struct {
	Title string
}
EOF
assert_pass "rule (a): domain with no boundary-crossing import" "$d"

d="$(new_fixture)"
cat >"$d/internal/domain/work.go" <<'EOF'
package domain

import "github.com/Alexandryn/alexandryn/internal/persistence/postgres"

type Work struct {
	Title string
	_     postgres.Pool
}
EOF
assert_fail "rule (a): domain importing persistence" "$d" "internal/domain/work.go"

d="$(new_fixture)"
cat >"$d/internal/domain/work.go" <<'EOF'
package domain

import "github.com/Alexandryn/alexandryn/internal/transport/http"

type Work struct {
	Title string
	_     http.Handler
}
EOF
assert_fail "rule (a): domain importing transport" "$d" "internal/domain/work.go"

# --- Rule (b): only internal/config may read the environment or decode TOML ---

d="$(new_fixture)"
cat >"$d/internal/config/config.go" <<'EOF'
package config

import "os"

func load() string {
	return os.Getenv("DATABASE_URL")
}
EOF
assert_pass "rule (b): os.Getenv confined to internal/config" "$d"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/migrate_integration_test.go" <<'EOF'
package postgres_test

import "os"

func testDSN() string {
	return os.Getenv("TEST_DATABASE_URL")
}
EOF
assert_pass "rule (b): TEST_DATABASE_URL read directly in an _integration_test.go file (backend-test-harness.md FR-2)" "$d"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/pool_test.go" <<'EOF'
package postgres_test

import "os"

func leaky() string {
	return os.Getenv("SOME_VAR")
}
EOF
assert_fail "rule (b): os.Getenv in an ordinary _test.go file (not _integration_test.go) is still a violation" "$d" "pool_test.go"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/pool.go" <<'EOF'
package postgres

import "os"

func dsn() string {
	return os.Getenv("DATABASE_URL")
}
EOF
assert_fail "rule (b): os.Getenv outside internal/config" "$d" "internal/persistence/postgres/pool.go"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/pool.go" <<'EOF'
package postgres

import "github.com/pelletier/go-toml/v2"

func decode(data []byte, v any) error {
	return toml.Unmarshal(data, v)
}
EOF
assert_fail "rule (b): TOML decode import outside internal/config" "$d" "internal/persistence/postgres/pool.go"

d="$(new_fixture)"
cat >"$d/internal/persistence/postgres/fixture.go" <<'EOF'
package postgres

// A path like "config.toml" here is a string literal in test fixture
// data, not an import — the checker must not flag it.
var fixturePath = "config.toml"
EOF
assert_pass "rule (b): \"toml\" inside a non-import string literal is not a violation" "$d"

# --- Rule (c): no package-level var holding a logger/pool/config ---

d="$(new_fixture)"
cat >"$d/internal/transport/http/router.go" <<'EOF'
package http

import "log/slog"

func newRouter(logger *slog.Logger) {
	_ = logger
}
EOF
assert_pass "rule (c): logger passed as a parameter, no package global" "$d"

d="$(new_fixture)"
cat >"$d/internal/transport/http/router.go" <<'EOF'
package http

import "log/slog"

var logger *slog.Logger
EOF
assert_fail "rule (c): package-level logger global" "$d" "internal/transport/http/router.go"

# --- Raw-string fixture data must not trip any rule ---

d="$(new_fixture)"
cat >"$d/internal/transport/http/proof.go" <<'EOF'
package http

// fixtureSource embeds another file's source as fixture data for a
// meta-test — an env-var read call, a domain-boundary-crossing import,
// and a package-level logger, none of it this file's own code, so the
// checker must not flag it.
var fixtureSource = `
package fixture

import "github.com/Alexandryn/alexandryn/internal/persistence/postgres"

var logger *slog.Logger

func read() string {
	return os.Getenv("SOME_VAR")
}
`
EOF
assert_pass "raw-string fixture data (Getenv, boundary import, global var) is not this file's own code" "$d"

if [ "$fail" -ne 0 ]; then
	echo "check-import-boundaries_test.sh: FAILED"
	exit 1
fi
echo "check-import-boundaries_test.sh: all cases passed"
