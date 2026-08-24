#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"

if [[ "${1:-}" == "--inside" ]]; then
  work_dir="$2"
  binary="$work_dir/agent-runtime-proof"
  helper="$work_dir/token-secret/payload/bin/fixture-runtime"
  cp "$work_dir/new-runtime" "$work_dir/replacement-runtime"
  mv "$work_dir/replacement-runtime" "$helper"
  file_digest="$(sha256sum "$helper" | awk '{print $1}')"
  file_size="$(stat -c '%s' "$helper")"
  tree_digest="$(printf '[{"path":"bin/fixture-runtime","sha256":"%s","size":%s}]' "$file_digest" "$file_size" | sha256sum | awk '{print $1}')"
  printf '%s\n' "{\"schema_version\":\"agent-runtime-expectation/1.0\",\"subject\":{\"id\":\"linux-fixture\",\"display_name\":\"Linux Fixture\",\"version\":\"1.0.0\"},\"launch\":{\"kind\":\"native\",\"entrypoint\":\"bin/fixture-runtime\",\"argument_fingerprints\":[]},\"artifact\":{\"root\":\"$work_dir/token-secret/payload\",\"include\":[\"**\"],\"exclude\":[],\"sha256\":\"$tree_digest\",\"max_files\":10,\"max_bytes\":100000000,\"max_duration_ms\":5000},\"policy\":{\"allowed_roots\":[\"$work_dir/token-secret/payload\"],\"allow_symlinks\":false},\"source\":{\"kind\":\"user-file\",\"locator_hash\":\"0000000000000000000000000000000000000000000000000000000000000000\",\"trust\":\"declared\"}}" > "$work_dir/linux-expectation.json"
  cp "$work_dir/old-runtime" "$helper"
  "$helper" &
  helper_pid=$!
  trap 'kill "$helper_pid" 2>/dev/null || true; wait "$helper_pid" 2>/dev/null || true' EXIT INT TERM
  sleep 1
  cp "$work_dir/new-runtime" "$work_dir/replacement-runtime"
  mv "$work_dir/replacement-runtime" "$helper"
  set +e
  "$binary" verify --expectation "$work_dir/linux-expectation.json" --pid "$helper_pid" --format json > "$work_dir/linux-replaced.json"
  replaced_exit=$?
  set -e
  [[ "$replaced_exit" -eq 3 ]] || { printf 'Linux replaced image returned exit %s, want 3\n' "$replaced_exit" >&2; exit 1; }
  grep -q '"POSSIBLE_STALE_AFTER_REPLACEMENT"' "$work_dir/linux-replaced.json"
  kill "$helper_pid"
  wait "$helper_pid" 2>/dev/null || true
  "$helper" &
  helper_pid=$!
  sleep 1
  "$binary" inspect --pid "$helper_pid" --format json > "$work_dir/linux-inspect.json"
  grep -q '"verdict":"UNKNOWN"' "$work_dir/linux-inspect.json"
  grep -q '"basename":"fixture-runtime"' "$work_dir/linux-inspect.json"
  if grep -Fq "$work_dir/" "$work_dir/linux-inspect.json"; then
    printf 'Linux inspection leaked a local path\n' >&2
    exit 1
  fi
  "$binary" verify --expectation "$work_dir/linux-expectation.json" --pid "$helper_pid" --format json > "$work_dir/linux-matched.json"
  grep -q '"verdict":"MATCHED"' "$work_dir/linux-matched.json"
  if grep -q 'token-secret' "$work_dir/linux-matched.json" || grep -Fq "$work_dir/" "$work_dir/linux-matched.json"; then
    printf 'Linux verification leaked a local path\n' >&2
    exit 1
  fi
  coproc ARP_MCP { "$binary" mcp 2>"$work_dir/mcp-stderr.txt"; }
  mcp_in="${ARP_MCP[1]}"
  mcp_out="${ARP_MCP[0]}"
  mcp_pid="$ARP_MCP_PID"
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"linux-native-fixture","version":"1"}}}' >&"$mcp_in"
  IFS= read -r initialize_response <&"$mcp_out"
  grep -q '"protocolVersion":"2025-06-18"' <<<"$initialize_response"
  printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&"$mcp_in"
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' >&"$mcp_in"
  IFS= read -r tools_response <&"$mcp_out"
  [[ "$(grep -o '"name":"[^"]*"' <<<"$tools_response" | grep -c '_local_runtime\|local_runtime_')" -eq 3 ]]
  printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"inspect_local_runtimes\",\"arguments\":{\"pid\":$helper_pid}}}" >&"$mcp_in"
  IFS= read -r call_response <&"$mcp_out"
  grep -q '"structuredContent":{"proofs":' <<<"$call_response"
  exec {mcp_in}>&-
  wait "$mcp_pid"
  [[ ! -s "$work_dir/mcp-stderr.txt" ]]
  cp "$helper" "$work_dir/deleted-runtime"
  "$work_dir/deleted-runtime" &
  deleted_pid=$!
  unlink "$work_dir/deleted-runtime"
  "$binary" inspect --pid "$deleted_pid" --format json > "$work_dir/linux-deleted.json"
  grep -q '"basename":"deleted-runtime"' "$work_dir/linux-deleted.json"
  kill "$deleted_pid"
  wait "$deleted_pid" 2>/dev/null || true
  kill "$helper_pid"
  wait "$helper_pid" 2>/dev/null || true
  helper_pid=""
  printf 'Phase 1 Linux runtime acceptance PASS\n'
  exit 0
fi

run_dir="$(mktemp -d /tmp/agent-runtime-proof-linux.XXXXXX)"
cleanup() {
  case "$run_dir" in
    /tmp/agent-runtime-proof-linux.*) find "$run_dir" -depth -delete 2>/dev/null || true ;;
    *) printf 'refusing to clean unexpected path\n' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

linux_arch="${ARP_LINUX_ARCH:-$(go env GOARCH)}"
GOOS=linux GOARCH="$linux_arch" CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=0.1.0 -X main.commit=abcdef0' -o "$run_dir/agent-runtime-proof" "$repo_dir/cmd/agent-runtime-proof"
mkdir -p "$run_dir/token-secret/payload/bin"
GOOS=linux GOARCH="$linux_arch" CGO_ENABLED=0 go build -trimpath -ldflags '-X main.marker=old-loaded-image' -o "$run_dir/old-runtime" "$repo_dir/testdata/acceptance/helper"
GOOS=linux GOARCH="$linux_arch" CGO_ENABLED=0 go build -trimpath -ldflags '-X main.marker=new-installed-image' -o "$run_dir/new-runtime" "$repo_dir/testdata/acceptance/helper"
image="${ARP_LINUX_IMAGE:-ubuntu:24.04}"
docker run --rm --network none --platform "linux/$linux_arch" --mount "type=bind,src=$run_dir,dst=/work" --mount "type=bind,src=$repo_dir,dst=/repo,readonly" "$image" bash /repo/scripts/run_linux_acceptance.sh --inside /work

trap - EXIT INT TERM
cleanup
[[ ! -e "$run_dir" ]]
