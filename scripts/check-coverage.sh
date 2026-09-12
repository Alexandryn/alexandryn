#!/usr/bin/env bash
# Coverage non-regression floor: fails when unit suite statement coverage
# falls more than 1.0 percentage point below scripts/coverage-baseline.txt.
set -euo pipefail

ROOT="${1:-.}"
PROFILE="${2:-$ROOT/coverage.out}"
BASELINE_FILE="$ROOT/scripts/coverage-baseline.txt"
TOLERANCE="1.0"

if [ ! -f "$PROFILE" ]; then
	echo "check-coverage: no profile at $PROFILE — run 'go test -coverprofile=coverage.out ./...' first" >&2
	exit 1
fi
if [ ! -f "$BASELINE_FILE" ]; then
	echo "check-coverage: no baseline at $BASELINE_FILE" >&2
	exit 1
fi

baseline="$(tr -d '[:space:]' < "$BASELINE_FILE")"
actual="$(go tool cover -func="$PROFILE" | awk '/^total:/ {gsub(/%/,"",$NF); print $NF}')"

if [ -z "$actual" ]; then
	echo "check-coverage: could not read a total from $PROFILE" >&2
	exit 1
fi

floor="$(awk -v b="$baseline" -v t="$TOLERANCE" 'BEGIN { printf "%.1f", b - t }')"
ok="$(awk -v a="$actual" -v f="$floor" 'BEGIN { print (a + 0 >= f + 0) ? "yes" : "no" }')"

if [ "$ok" != "yes" ]; then
	echo "check-coverage: coverage $actual% is below the floor $floor% (baseline $baseline%, tolerance $TOLERANCE)" >&2
	echo "check-coverage: add tests, or lower scripts/coverage-baseline.txt with a recorded reason" >&2
	exit 1
fi

echo "check-coverage: $actual% (baseline $baseline%, floor $floor%) — ok"
