#!/usr/bin/env bash
# Parallelism-hazard check (backend-test-harness.md FR-3, test plan's own
# "CI infrastructure" layer): truncate-based teardown has no per-test
# isolation mechanism (no per-test schema, no per-test transaction) that
# would make concurrent truncation of overlapping tables by two tests in
# the same package safe. No _integration_test.go file may call
# t.Parallel() — a gap the spec itself doesn't name; this script is the
# enforceable rule the test plan adds to close it.
#
# Grep-based and heuristic, same interim spirit as D0's import-boundary
# check and check-parameterized-queries.sh.
set -euo pipefail

ROOT="${1:-.}"

violations=""

add_violation() {
	violations="${violations}${1}
"
}

# Same raw-string stripping as the other interim checks: a backtick-
# delimited literal can't contain a backtick, so this split is exact, and
# it keeps fixture source embedded as test data from tripping this check.
strip_raw_strings() {
	awk '
		{
			line = $0
			out = ""
			while ((pos = index(line, "`")) > 0) {
				if (!in_raw) out = out substr(line, 1, pos - 1)
				in_raw = !in_raw
				line = substr(line, pos + 1)
			}
			if (!in_raw) out = out line
			print out
		}
	' "$1"
}

while IFS= read -r f; do
	stripped="$(strip_raw_strings "$f")"
	while IFS= read -r line; do
		if grep -Eq '\.Parallel\(\)' <<<"$line"; then
			add_violation "$f: t.Parallel() must never appear in an _integration_test.go file — truncate-based teardown has no per-test isolation (backend-test-harness.md FR-3): ${line# }"
		fi
	done <<<"$stripped"
done < <(find "$ROOT" -name '*_integration_test.go' -type f 2>/dev/null)

if [ -n "$violations" ]; then
	echo "check-integration-test-parallelism: violations found:" >&2
	printf '%s' "$violations" >&2
	exit 1
fi

echo "check-integration-test-parallelism: clean"
