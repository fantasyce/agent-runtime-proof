#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_dir"

unformatted_files="$(gofmt -l .)"
if [[ -n "$unformatted_files" ]]; then
  printf 'gofmt required:\n%s\n' "$unformatted_files" >&2
  exit 1
fi

go vet ./...
go mod verify
go build -trimpath ./...
go test -count=1 -race ./...
git diff --check
bash scripts/test_public_surface.sh
bash scripts/test_stale_runtime_demo.sh
bash scripts/verify_mcp_metadata.sh
bash scripts/test_mcpb_packaging.sh
