#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  printf 'usage: run-macos-agent-host.sh HOST EVIDENCE_JSON\n' >&2
  exit 2
fi

host="$1"
evidence_file="$2"
case "$host" in codex|claude-code) ;; *) printf 'unsupported macOS host\n' >&2; exit 2 ;; esac

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/../.." && pwd -P)"
source "$script_dir/common.sh"
task_prefix="/private/tmp/agent-runtime-proof-phase4-real-${host}."
run_dir="$(mktemp -d "${task_prefix}XXXXXX")"
installed_dir="$run_dir/installed"
workspace="$run_dir/workspace"
task_home="$run_dir/home"
binary="$installed_dir/agent-runtime-proof"
helper="$installed_dir/phase4-helper"
active_pids=()

cleanup() {
  for task_pid in "${active_pids[@]:-}"; do
    if [[ -n "$task_pid" ]] && kill -0 "$task_pid" 2>/dev/null; then
      kill -TERM "$task_pid" 2>/dev/null || true
      wait "$task_pid" 2>/dev/null || true
    fi
  done
  case "$run_dir" in
    "$task_prefix"*) find "$run_dir" -depth -delete 2>/dev/null || true ;;
    *) printf 'refusing unsafe Phase 4 cleanup\n' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

mkdir -p "$installed_dir" "$workspace/.codex" "$task_home/.codex"
touch "$run_dir/.arp-task-owned"
candidate_commit="$(git -C "$repo_dir" rev-parse --short=12 HEAD)"
go build -trimpath -ldflags "-s -w -X main.version=0.4.0-phase4 -X main.commit=$candidate_commit" -o "$binary" "$repo_dir/cmd/agent-runtime-proof"
go build -trimpath -o "$helper" "$repo_dir/testdata/phase4-helper"
chmod 700 "$binary" "$helper"

expectation="$run_dir/candidate-expectation.json"
write_single_file_expectation "$expectation" agent-runtime-proof "Agent Runtime Proof" 0.4.0-phase4 "$installed_dir" agent-runtime-proof "$binary"

printf '[mcp_servers.agent-runtime-proof]\ncommand = "%s"\nargs = ["mcp"]\n' "$binary" > "$workspace/.codex/config.toml"
printf '{"mcpServers":{"agent-runtime-proof":{"command":"%s","args":["mcp"]}}}\n' "$binary" > "$workspace/.mcp.json"

prompt="Call only the agent-runtime-proof MCP tool verify_local_runtime exactly once with binding_id ${host}.agent-runtime-proof and expectation_path ${expectation}. Do not call shell, file, search, or any other tool. After the tool returns, output exactly: ARP_HOST_PASS verdict=<verdict> proof_id=<proof_id> binding_id=<binding_id>."
events="$run_dir/events.jsonl"
message="$run_dir/message.txt"
list_output="$run_dir/list.json"

case "$host" in
  codex)
    original_codex_home="${CODEX_HOME:-$HOME/.codex}"
    HOME="$task_home" CODEX_HOME="$original_codex_home" codex mcp list --json \
      -c "mcp_servers.agent-runtime-proof.command=\"$binary\"" \
      -c 'mcp_servers.agent-runtime-proof.args=["mcp"]' > "$list_output"
    HOME="$task_home" CODEX_HOME="$original_codex_home" codex exec \
      --ignore-user-config --ignore-rules --ephemeral --skip-git-repo-check \
      --sandbox read-only --json -C "$workspace" -o "$message" \
      -c "mcp_servers.agent-runtime-proof.command=\"$binary\"" \
      -c 'mcp_servers.agent-runtime-proof.args=["mcp"]' "$prompt" > "$events"
    host_version="$(codex --version | awk '{print $NF}')"
    ;;
  claude-code)
    (cd "$workspace" && claude --mcp-config "$workspace/.mcp.json" --strict-mcp-config mcp list > "$list_output")
    (cd "$workspace" && claude -p --no-session-persistence --mcp-config "$workspace/.mcp.json" \
      --strict-mcp-config --allowedTools mcp__agent-runtime-proof__verify_local_runtime \
      --permission-mode dontAsk --output-format stream-json --verbose "$prompt" > "$events")
    grep -E 'ARP_HOST_PASS verdict=[A-Z_]+' "$events" | tail -1 > "$message"
    host_version="$(claude --version | awk '{print $1}')"
    ;;
esac

node "$script_dir/extract-host-result.mjs" "$host" "$host_version" "$candidate_commit" "$list_output" "$events" "$message" > "$evidence_file"
printf 'Phase 4 real host PASS (%s)\n' "$host"
