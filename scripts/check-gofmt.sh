#!/usr/bin/env bash
# check-gofmt.sh — fail CI if any Go files need reformatting.
#
# Usage: scripts/check-gofmt.sh [dir]        (defaults to repo root)
#
# Only files tracked by the Go toolchain are checked; web/ is excluded
# because it is TypeScript/CSS territory. The script is self-testing:
# run with CHECK_GOFMT_SELF_TEST=1 to verify it catches a bad file and
# passes a good one.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${1:-$ROOT}"

# Collect Go files, skipping the web/ directory and vendor/.
mapfile -t GO_FILES < <(
  find "$TARGET" -name '*.go' \
    -not -path '*/web/*' \
    -not -path '*/.git/*' \
    -not -path '*/node_modules/*' \
    -not -path '*/vendor/*' \
  | sort
)

if [[ "${CHECK_GOFMT_SELF_TEST:-}" == "1" ]]; then
  _self_test() {
    local tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT

    # A well-formatted file must pass.
    cat > "$tmp/good.go" <<'GOEOF'
package main

import "fmt"

func main() {
	fmt.Println("ok")
}
GOEOF
    if gofmt -l "$tmp/good.go" | grep -q .; then
      echo "FAIL: gofmt flagged a correctly formatted file" >&2
      exit 1
    fi

    # A mis-formatted file must be caught.
    cat > "$tmp/bad.go" <<'GOEOF'
package main
import "fmt"
func main(){fmt.Println("bad")}
GOEOF
    if ! gofmt -l "$tmp/bad.go" | grep -q .; then
      echo "FAIL: gofmt missed a mis-formatted file" >&2
      exit 1
    fi

    echo "self-test: PASS"
  }
  _self_test
  exit 0
fi

if [[ ${#GO_FILES[@]} -eq 0 ]]; then
  echo "check-gofmt: no Go files found under $TARGET"
  exit 0
fi

UNFORMATTED="$(gofmt -l "${GO_FILES[@]}" 2>/dev/null || true)"

if [[ -n "$UNFORMATTED" ]]; then
  echo "check-gofmt: the following files need 'gofmt -w':" >&2
  echo "$UNFORMATTED" | sed 's/^/  /' >&2
  echo "" >&2
  echo "Run: gofmt -w ./... (excluding web/)" >&2
  exit 1
fi

echo "check-gofmt: all Go files are formatted correctly"
