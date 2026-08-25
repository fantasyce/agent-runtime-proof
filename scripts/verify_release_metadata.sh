#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_dir"

test "$(tr -d '\r\n' < VERSION)" = "1.0.0"
grep -Fq 'version = "1.0.0-dev"' cmd/agent-runtime-proof/main.go
test "$(jq -r '.version' plugin/agent-runtime-proof/.codex-plugin/plugin.json)" = "1.0.0"
grep -Fq 'Apache License' LICENSE
grep -Fq 'Version 2.0, January 2004' LICENSE
grep -Fq 'Supported Versions' SECURITY.md
grep -Fq 'Private Reporting' SECURITY.md
grep -Fq '## [1.0.0] - 2026-08-26' CHANGELOG.md
grep -Fq 'releases/tag/v1.0.0' README.md
grep -Fq 'docs/install.md' README.md

if rg -n '0\.4\.0-(dev|phase4)' cmd plugin README.md; then
  echo 'Phase development version remains in a public version surface' >&2
  exit 1
fi

echo 'release metadata verified'
