#!/usr/bin/env bash
# Fixture proof for check-coverage.sh (ADR 0033).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-coverage.sh"
fail=0

mk() {
	local total="$1" baseline="$2" d
	d="$(mktemp -d)"
	mkdir -p "$d/scripts"
	echo "$baseline" > "$d/scripts/coverage-baseline.txt"
	cat > "$d/coverage.out" <<PROF
mode: set
example.com/x/x.go:1.1,2.2 1 1
PROF
	# Stub go(1) so the checker's 'go tool cover -func' yields our total.
	cat > "$d/go" <<GO
#!/usr/bin/env bash
echo "total:	(statements)	${total}%"
GO
	chmod +x "$d/go"
	echo "$d"
}

run() { PATH="$1:$PATH" "$CHECKER" "$1" "$1/coverage.out" 2>&1; }

d="$(mk 60.0 55.0)"
if out="$(run "$d")"; then echo "ok - above baseline passes"; else echo "FAIL - above baseline: $out"; fail=1; fi

d="$(mk 54.2 55.0)"
if out="$(run "$d")"; then echo "ok - within the 1pt tolerance passes"; else echo "FAIL - tolerance: $out"; fail=1; fi

d="$(mk 52.0 55.0)"
if out="$(run "$d")"; then echo "FAIL - 3pt drop should fail"; fail=1; else
	grep -q "below the floor" <<<"$out" && echo "ok - a real drop fails and says why" || { echo "FAIL - wrong message: $out"; fail=1; }
fi

[ "$fail" -eq 0 ] && echo "check-coverage_test.sh: all cases passed" || { echo "check-coverage_test.sh: FAILED"; exit 1; }
