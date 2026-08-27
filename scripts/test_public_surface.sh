#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_dir"

required_files=(
  README.md
  CONTRIBUTING.md
  CODE_OF_CONDUCT.md
  SUPPORT.md
  docs/quickstart.md
  docs/data-handling.md
  .github/ISSUE_TEMPLATE/bug_report.yml
  .github/ISSUE_TEMPLATE/runtime_case.yml
  .github/ISSUE_TEMPLATE/config.yml
  .github/pull_request_template.md
)

for path in "${required_files[@]}"; do
  [[ -f "$path" ]] || { echo "missing public surface file: $path" >&2; exit 1; }
done

positioning='Prove the agent runtime you launched is the artifact you approved.'
grep -Fq "$positioning" README.md
grep -Fq 'docs/quickstart.md' README.md
grep -Fq 'docs/data-handling.md' README.md
grep -Fq 'CONTRIBUTING.md' README.md
grep -Fq 'CODE_OF_CONDUCT.md' README.md
grep -Fq 'SUPPORT.md' README.md

quickstart_line="$(grep -n -m1 '^## Quickstart$' README.md | cut -d: -f1)"
architecture_line="$(grep -n -m1 '^## Architecture and acceptance$' README.md | cut -d: -f1)"
[[ -n "$quickstart_line" && -n "$architecture_line" && "$quickstart_line" -lt "$architecture_line" ]] || {
  echo 'README must put Quickstart before Architecture and acceptance' >&2
  exit 1
}

if grep -Eiq 'fourth (first-party )?managed (Across )?plugin' README.md; then
  echo 'README must not position ARP as a fourth managed plugin' >&2
  exit 1
fi

if grep -Eiq 'curl[^|]*\|[[:space:]]*(ba)?sh' README.md docs/quickstart.md; then
  echo 'public install instructions must not pipe remote code into a shell' >&2
  exit 1
fi

grep -Fq 'security' .github/ISSUE_TEMPLATE/config.yml
grep -Fq 'Do not include secrets' .github/ISSUE_TEMPLATE/bug_report.yml
grep -Fq 'Do not include secrets' .github/ISSUE_TEMPLATE/runtime_case.yml
