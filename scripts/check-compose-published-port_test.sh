#!/usr/bin/env bash
# Fixture proof for check-compose-published-port.sh: a clean compose file
# passes, a backend ports: entry or network_mode: host fails naming the
# file, and a published port on a different service (postgres) does not
# false-positive — the check is scoped to backend specifically
# (deployment-container-packaging.md FR-5/FR-6).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-compose-published-port.sh"

fail=0

new_fixture() {
	mktemp -d
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

d="$(new_fixture)"
cat >"$d/docker-compose.yml" <<'EOF'
services:
  backend:
    build: .
    environment:
      - DATABASE_URL
    depends_on:
      postgres:
        condition: service_healthy
        required: false
  postgres:
    image: postgres:16-alpine
    profiles: ["bundled-db"]
volumes:
  data:
EOF
assert_pass "clean compose file, no ports/network_mode on backend" "$d"

d="$(new_fixture)"
cat >"$d/docker-compose.yml" <<'EOF'
services:
  backend:
    build: .
    ports:
      - "8080:8080"
  postgres:
    image: postgres:16-alpine
volumes:
  data:
EOF
assert_fail "backend ports: entry" "$d" "docker-compose.yml"

d="$(new_fixture)"
cat >"$d/docker-compose.yml" <<'EOF'
services:
  backend:
    build: .
    network_mode: host
  postgres:
    image: postgres:16-alpine
EOF
assert_fail "backend network_mode: host" "$d" "docker-compose.yml"

d="$(new_fixture)"
cat >"$d/docker-compose.yml" <<'EOF'
services:
  backend:
    build: .
  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
volumes:
  data:
EOF
assert_pass "ports: on a different service (postgres) is not a backend violation" "$d"

if [ "$fail" -ne 0 ]; then
	echo "check-compose-published-port_test.sh: FAILED"
	exit 1
fi
echo "check-compose-published-port_test.sh: all cases passed"
