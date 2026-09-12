#!/usr/bin/env bash
# SQL-injection check: internal/persistence/postgres SQL queries must use
# pgx parameterized arguments exclusively, never string formatting or concatenation
# incorporating potentially hostile input.
#
# Flags: (a) fmt.Sprintf building a SQL string, (b) string concatenation (+)
# around SQL literals.
set -euo pipefail

ROOT="${1:-.}"
DIR="$ROOT/internal/persistence/postgres"

violations=""

add_violation() {
	violations="${violations}${1}
"
}

sql_keyword_pattern='\b(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE|VALUES|SET)\b'

# Raw-string stripping: a backtick-delimited literal can't contain a backtick,
# so this keeps fixture source embedded as test data from tripping this check.
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

		# (a) fmt.Sprintf building something SQL-shaped (handles single-line and multi-line calls).
		if grep -Eiq "fmt\.Sprintf\(" <<<"$stripped" && grep -Eiq "$sql_keyword_pattern" <<<"$stripped"; then
			matched=0
			while IFS= read -r line; do
				if grep -Eiq "fmt\.Sprintf\(" <<<"$line" && grep -Eiq "$sql_keyword_pattern" <<<"$line"; then
					add_violation "$f: fmt.Sprintf building a SQL-shaped string — use pgx parameterized query arguments instead: ${line# }"
					matched=1
				fi
			done <<<"$stripped"
			if [ "$matched" -eq 0 ]; then
				if python3 -c "
import sys, re
c = sys.stdin.read()
if re.search(r'fmt\.Sprintf\s*\([^)]*\b(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE|VALUES|SET)\b', c, re.IGNORECASE | re.DOTALL):
    sys.exit(0)
sys.exit(1)
" <<<"$stripped" 2>/dev/null; then
					add_violation "$f: fmt.Sprintf building a SQL-shaped string across multiple lines — use pgx parameterized query arguments instead"
				fi
			fi
		fi

		# (b) string concatenation around a SQL-shaped literal.
		while IFS= read -r line; do
			if grep -Eiq "\"[^\"]*${sql_keyword_pattern}[^\"]*\"[[:space:]]*\+" <<<"$line" \
				|| grep -Eiq "\+[[:space:]]*\"[^\"]*${sql_keyword_pattern}[^\"]*\"" <<<"$line"; then
				add_violation "$f: string concatenation around a SQL-shaped literal — use pgx parameterized query arguments instead: ${line# }"
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
