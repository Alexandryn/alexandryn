#!/usr/bin/env bash
# Prints one version's section of CHANGELOG.md (without its heading), for the
# body of a GitHub release. Fails, printing nothing, if the version has no
# section or the section is empty, so a release never goes out with blank notes.
#
# Usage: scripts/release-notes.sh <version|tag> [changelog-path]
set -euo pipefail

if [ "$#" -lt 1 ]; then
	echo "usage: release-notes.sh <version> [changelog]" >&2
	exit 2
fi

version="${1#v}"
changelog="${2:-CHANGELOG.md}"

if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
	echo "release-notes: '$1' is not a version like 1.0.0 or v1.0.0" >&2
	exit 2
fi
if [ ! -f "$changelog" ]; then
	echo "release-notes: cannot read $changelog" >&2
	exit 1
fi

# Literal comparison (index), never a regex built from the version.
notes="$(awk -v want="## [$version]" '
	/^## \[/ {
		if (in_section) { exit }
		if (index($0, want) == 1) { in_section = 1; next }
	}
	in_section { print }
' "$changelog")"

# Trim leading and trailing blank lines.
notes="$(printf '%s\n' "$notes" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}')"

if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
	echo "release-notes: $changelog has no non-empty section for $version" >&2
	exit 1
fi
printf '%s\n' "$notes"
