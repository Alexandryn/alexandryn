#!/usr/bin/env bash
# Import boundary check: enforces architectural boundaries across packages:
#
#   (a) internal/domain must not import internal/transport or internal/persistence
#   (b) only internal/config may read environment variables or decode TOML configuration
#   (c) no package-level var holds a logger, connection pool, or loaded config
#
# Exits non-zero and names the offending file on the first violation category found.
set -euo pipefail

ROOT="${1:-.}"
DOMAIN_DIR="$ROOT/internal/domain"
CONFIG_DIR="$ROOT/internal/config"

violations=""

add_violation() {
	violations="${violations}${1}
"
}

# A raw string literal (backtick-delimited — Go raw strings can never
# contain a backtick, so this split is exact) commonly embeds another
# file's source as fixture data in a meta-test. Its contents aren't this
# file's own code, so none of the three checks below should see them.
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

# (a) internal/domain must not import internal/transport or internal/persistence.
if [ -d "$DOMAIN_DIR" ]; then
	while IFS= read -r f; do
		if strip_raw_strings "$f" | grep -Eq '"[^"]*/internal/(transport|persistence)(/|")'; then
			add_violation "$f: internal/domain must not import internal/transport or internal/persistence"
		fi
	done < <(find "$DOMAIN_DIR" -name '*.go' -type f 2>/dev/null)
fi

# (b) only internal/config may read the environment or decode TOML.
# The TOML half is scoped to actual import lines, not any string literal
# containing "toml" — a fixture path or a comment shouldn't trip this.
# An _integration_test.go file is exempt from the env-var half: reading
# TEST_DATABASE_URL directly is allowed for test harnesses.
import_lines() {
	strip_raw_strings "$1" | awk '
		/^import \(/ { inblock = 1; next }
		inblock && /^\)/ { inblock = 0; next }
		inblock { print; next }
		/^import[[:space:]]+.*"/ { print }
	'
}

while IFS= read -r f; do
	case "$f" in
	"$CONFIG_DIR"/*) continue ;;
	*_integration_test.go) continue ;;
	esac
	if strip_raw_strings "$f" | grep -Eq '\bos\.(Getenv|LookupEnv)\(' || import_lines "$f" | grep -Eiq 'toml'; then
		add_violation "$f: only internal/config may read an environment variable or decode TOML"
	fi
done < <(find "$ROOT/internal" "$ROOT/cmd" -name '*.go' -type f 2>/dev/null)

# (c) no package-level var holding a logger, pool, or config.
while IFS= read -r f; do
	if strip_raw_strings "$f" | grep -Eq '^var[[:space:]]+[A-Za-z0-9_]+[[:space:]]+\*?(slog\.Logger|pgxpool\.Pool|config\.Config)\b'; then
		add_violation "$f: no package-level var may hold a logger, pool, or config"
	fi
done < <(find "$ROOT/internal" "$ROOT/cmd" -name '*.go' -type f 2>/dev/null)

if [ -n "$violations" ]; then
	echo "check-import-boundaries: violations found:" >&2
	printf '%s' "$violations" >&2
	exit 1
fi

echo "check-import-boundaries: clean"
