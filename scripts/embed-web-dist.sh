#!/usr/bin/env bash
# embed-web-dist.sh — copy the built web/dist into the directory
# internal/transport/http/static.go's //go:embed directive targets, so
# `go build ./cmd/server` embeds the real frontend instead of the
# committed placeholder.
#
# Go's //go:embed patterns cannot contain ".." path elements, so the
# server package (internal/transport/http, deep under the module root)
# cannot embed web/dist (at the module root) directly — this script is
# the only way real content reaches that embed. It must run, with
# web/dist already built, before every `go build` that produces a
# binary meant to actually serve users: Dockerfile, ci.yml's backend
# job, and release.yml's per-platform legs.
#
# Usage: scripts/embed-web-dist.sh [repo-root]        (defaults to .)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="${1:-$ROOT}"

WEB_DIST="$REPO_ROOT/web/dist"
EMBED_TARGET="$REPO_ROOT/internal/transport/http/webdist/placeholder"

if [[ ! -f "$WEB_DIST/index.html" ]]; then
  echo "embed-web-dist: $WEB_DIST/index.html not found — run 'npm run -w web build' first" >&2
  exit 1
fi

rm -rf "$EMBED_TARGET"
mkdir -p "$EMBED_TARGET"
cp -r "$WEB_DIST"/. "$EMBED_TARGET"/

echo "embed-web-dist: copied $WEB_DIST -> $EMBED_TARGET"
