#!/usr/bin/env bash
# Compose published port check:
# Fails if any tracked compose file publishes a port for the `backend` service
# or sets `network_mode: host` on it on the default profile without TLS and
# authentication configured.
set -euo pipefail

ROOT="${1:-.}"

violations=""

add_violation() {
	violations="${violations}${1}
"
}

# Strips a trailing "  # comment" from each line before any pattern
# match below — without this, `ports:  # exposed for debugging` or
# `network_mode: "host" # for LAN testing` silently evades detection,
# since the anchored end-of-line patterns below never match a line that
# still has trailing comment text on it. Matches YAML's own comment rule
# (a `#` preceded by whitespace, or at start of line) closely enough for
# this repo's own compose files — not a full YAML/quote-aware parser,
# same accepted scope every interim check here already has.
strip_comments() {
	sed -E 's/(^|[[:space:]])#.*$//' "$1"
}

# Extracts the body of one top-level service block (2-space indent key,
# 4+-space indent body) from a compose file. Assumes this project's own
# docker-compose.yml formatting (2-space service names under services:,
# 4-space keys under each service) — the same "good enough for this
# repo's own files, not a general YAML parser" scope every interim
# check here already accepts.
extract_service_block() {
	local file="$1" svc="$2"
	strip_comments "$file" | awk -v svc="$svc" '
		/^services:[[:space:]]*$/ { in_services = 1; next }
		in_services && /^[A-Za-z]/ { in_services = 0; in_svc = 0 }
		in_services && $0 ~ "^  " svc ":[[:space:]]*$" { in_svc = 1; next }
		in_svc && $0 ~ /^  [A-Za-z0-9_.-]+:/ { in_svc = 0 }
		in_svc { print }
	'
}

while IFS= read -r f; do
	block="$(extract_service_block "$f" backend)"
	[ -z "$block" ] && continue

	# Block-style `ports:` (list items on following lines) is always a
	# violation; inline-style `ports: [...]` is one unless the array is
	# genuinely empty (`ports: []` publishes nothing).
	if grep -Eq '^[[:space:]]*ports:[[:space:]]*$' <<<"$block"; then
		add_violation "$f: backend service publishes a port (ports:) — not legal on the default profile without TLS and authentication"
	elif grep -Eq '^[[:space:]]*ports:[[:space:]]*\[' <<<"$block" && ! grep -Eq '^[[:space:]]*ports:[[:space:]]*\[[[:space:]]*\][[:space:]]*$' <<<"$block"; then
		add_violation "$f: backend service publishes a port (ports:) — not legal on the default profile without TLS and authentication"
	fi
	if grep -Eq '^[[:space:]]*network_mode:[[:space:]]*["'"'"']?host["'"'"']?[[:space:]]*$' <<<"$block"; then
		add_violation "$f: backend service sets network_mode: host — not legal on the default profile without TLS and authentication"
	fi
done < <(find "$ROOT" -maxdepth 1 \( -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name 'compose*.yaml' \) -type f 2>/dev/null)

if [ -n "$violations" ]; then
	echo "check-compose-published-port: violations found:" >&2
	printf '%s' "$violations" >&2
	exit 1
fi

echo "check-compose-published-port: clean"
