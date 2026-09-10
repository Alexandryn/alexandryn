#!/usr/bin/env bash
# Configure git to use project-tracked hooks directory (.githooks).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
git config core.hooksPath "$ROOT/.githooks"
chmod +x "$ROOT/.githooks/"* 2>/dev/null || true
echo "Git hooks configured from .githooks (pre-push protects 'main')"
