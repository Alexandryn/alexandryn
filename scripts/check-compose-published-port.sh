#!/usr/bin/env bash
# Interim compose-file lint (deployment-container-packaging.md FR-6):
# fails if any tracked compose file publishes a port for the `backend`
# service (a `ports:` entry) or sets `network_mode: host` on it — FR-5's
# default-profile guarantee (nothing reachable from outside the Compose
# network) depends on this holding against a future edit, not just
# present intent.
#
# Mode A's exception (FR-6: legal when the same file also declares
# TLS_CERT_FILE/TLS_KEY_FILE mounted into backend AND an
# authentication-enabled setting in backend's own environment) is not
# implemented here: no config key for "authentication enabled" exists
# anywhere in this codebase yet — that's phase 12's to name. This check
# is unconditional until then, matching backend-test-harness.md's own
# "no override file exists today" reality; a follow-up gets the real
# Mode A carve-out once that key exists.
#
# Grep/awk-based, same interim spirit as check-import-boundaries.sh —
# revisited once a real YAML tool replaces it.
set -euo pipefail

ROOT="${1:-.}"

violations=""

add_violation() {
	violations="${violations}${1}
"
}

# Extracts the body of one top-level service block (2-space indent key,
# 4+-space indent body) from a compose file. Assumes this project's own
# docker-compose.yml formatting (2-space service names under services:,
# 4-space keys under each service) — the same "good enough for this
# repo's own files, not a general YAML parser" scope every interim
# check here already accepts.
extract_service_block() {
	local file="$1" svc="$2"
	awk -v svc="$svc" '
		/^services:[[:space:]]*$/ { in_services = 1; next }
		in_services && /^[A-Za-z]/ { in_services = 0; in_svc = 0 }
		in_services && $0 ~ "^  " svc ":[[:space:]]*$" { in_svc = 1; next }
		in_svc && $0 ~ /^  [A-Za-z0-9_.-]+:/ { in_svc = 0 }
		in_svc { print }
	' "$file"
}

while IFS= read -r f; do
	block="$(extract_service_block "$f" backend)"
	[ -z "$block" ] && continue

	if grep -Eq '^[[:space:]]*ports:[[:space:]]*$' <<<"$block" || grep -Eq '^[[:space:]]*ports:[[:space:]]*\[' <<<"$block"; then
		add_violation "$f: backend service publishes a port (ports:) — not legal on the default profile without both TLS_CERT_FILE/TLS_KEY_FILE and an authentication-enabled setting (deployment-container-packaging.md FR-5/FR-6)"
	fi
	if grep -Eq '^[[:space:]]*network_mode:[[:space:]]*"?host"?[[:space:]]*$' <<<"$block"; then
		add_violation "$f: backend service sets network_mode: host — same FR-5/FR-6 violation as a published port"
	fi
done < <(find "$ROOT" -maxdepth 1 \( -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name 'compose*.yaml' \) -type f 2>/dev/null)

if [ -n "$violations" ]; then
	echo "check-compose-published-port: violations found:" >&2
	printf '%s' "$violations" >&2
	exit 1
fi

echo "check-compose-published-port: clean"
