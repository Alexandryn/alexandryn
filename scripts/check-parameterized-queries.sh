#!/usr/bin/env bash
# Interim SQL-injection-discipline check (backend-persistence.md FR-3):
# internal/persistence/postgres's SQL MUST use pgx's own parameterized-
# query mechanism exclusively, never string-built SQL incorporating a
# value that traces back to hostile input. ADR 0012's "hand-written SQL,
# no query builder" choice makes this a discipline this script checks for
# rather than something a query builder makes structurally hard to
# violate.
#
# Grep-based and heuristic, same interim spirit as D0's import-boundary
# check (tasks/plan.md) — flags two shapes: (a) fmt.Sprintf building a
# string that looks like SQL, (b) string concatenation (+) around a
# string literal that looks like SQL. A query built entirely from string
# literals passed straight to pgx's own query methods, with values passed
# as separate arguments, triggers neither shape.
set -euo pipefail

ROOT="${1:-.}"
DIR="$ROOT/internal/persistence/postgres"

violations=""

add_violation() {
	violations="${violations}${1}
"
}

sql_keyword_pattern='\b(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE|VALUES|SET)\b'

# Same raw-string stripping as check-import-boundaries.sh: a backtick-
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

if [ -d "$DIR" ]; then
	while IFS= read -r f; do
		stripped="$(strip_raw_strings "$f")"

		# (a) fmt.Sprintf building something SQL-shaped.
		if grep -Eiq "fmt\.Sprintf\(" <<<"$stripped" && grep -Eiq "$sql_keyword_pattern" <<<"$stripped"; then
			while IFS= read -r line; do
				if grep -Eiq "fmt\.Sprintf\(" <<<"$line" && grep -Eiq "$sql_keyword_pattern" <<<"$line"; then
					add_violation "$f: fmt.Sprintf building a SQL-shaped string — use pgx's own parameterized-query arguments instead (backend-persistence.md FR-3): ${line# }"
				fi
			done <<<"$stripped"
		fi

		# (b) string concatenation around a SQL-shaped literal.
		while IFS= read -r line; do
			if grep -Eiq "\"[^\"]*${sql_keyword_pattern}[^\"]*\"[[:space:]]*\+" <<<"$line" \
				|| grep -Eiq "\+[[:space:]]*\"[^\"]*${sql_keyword_pattern}[^\"]*\"" <<<"$line"; then
				add_violation "$f: string concatenation around a SQL-shaped literal — use pgx's own parameterized-query arguments instead (backend-persistence.md FR-3): ${line# }"
			fi
		done <<<"$stripped"
	done < <(find "$DIR" -name '*.go' -type f 2>/dev/null)
fi

if [ -n "$violations" ]; then
	echo "check-parameterized-queries: violations found:" >&2
	printf '%s' "$violations" >&2
	exit 1
fi

echo "check-parameterized-queries: clean"
