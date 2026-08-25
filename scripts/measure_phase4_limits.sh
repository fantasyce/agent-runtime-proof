#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
task_prefix="${TMPDIR:-/tmp}/agent-runtime-proof-phase4-performance."
run_dir="$(mktemp -d "${task_prefix}XXXXXX")"
cleanup() { case "$run_dir" in "$task_prefix"*) find "$run_dir" -depth -delete 2>/dev/null || true ;; *) return 1 ;; esac; }
trap cleanup EXIT INT TERM

candidate="$run_dir/agent-runtime-proof"
helper="$run_dir/phase4-helper"
commit="$(git -C "$repo_dir" rev-parse --short=12 HEAD)"
go build -trimpath -ldflags "-s -w -X main.version=0.4.0-phase4 -X main.commit=$commit" -o "$candidate" "$repo_dir/cmd/agent-runtime-proof"
go build -trimpath -o "$helper" "$repo_dir/testdata/phase4-helper"
"$helper" measure-mcp "$candidate" 20
ARP_RUN_PERFORMANCE=1 go test ./internal/hostprofile -run '^TestPhase4ReferencePerformance$' -count=1 -v
