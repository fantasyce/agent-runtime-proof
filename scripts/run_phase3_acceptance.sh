#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
if [[ "$(uname -s)" == "Darwin" ]]; then
  task_base="/private/tmp"
else
  task_base="/tmp"
fi
task_prefix="$task_base/agent-runtime-proof-phase3."
run_dir="$(mktemp -d "${task_prefix}XXXXXX")"
installed_dir="$run_dir/installed"
state_root="$run_dir/token-secret-state"
binary="$installed_dir/agent-runtime-proof"
helper="$installed_dir/witness-helper"
secret="token-super-secret-phase3"
active_pids=()

cleanup() {
  for task_pid in "${active_pids[@]:-}"; do
    if [[ -n "$task_pid" ]] && kill -0 "$task_pid" 2>/dev/null; then
      kill -KILL "$task_pid" 2>/dev/null || true
      wait "$task_pid" 2>/dev/null || true
    fi
  done
  case "$run_dir" in
    "$task_prefix"*) find "$run_dir" -depth -delete 2>/dev/null || true ;;
    *) printf 'refusing to clean unexpected Phase 3 path\n' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

sha256_text() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  else
    shasum -a 256 | awk '{print $1}'
  fi
}

file_size() {
  if stat -c '%s' "$1" >/dev/null 2>&1; then
    stat -c '%s' "$1"
  else
    stat -f '%z' "$1"
  fi
}

wait_for_pattern() {
  local path="$1"
  local pattern="$2"
  for _ in $(seq 1 200); do
    if [[ -f "$path" ]] && grep -q "$pattern" "$path"; then
      return 0
    fi
    sleep 0.025
  done
  printf 'timed out waiting for %s in %s\n' "$pattern" "$path" >&2
  return 1
}

assert_process_gone() {
  local pid="$1"
  for _ in $(seq 1 200); do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 0.025
  done
  printf 'process %s survived Phase 3 terminal path\n' "$pid" >&2
  return 1
}

receipt_path_from_stderr() {
  local stderr_path="$1"
  local receipt_id
  receipt_id="$(sed -n 's/.*launch receipt sha256:\([0-9a-f]\{64\}\).*/\1/p' "$stderr_path" | tail -1)"
  [[ -n "$receipt_id" ]]
  printf '%s/launch-receipts/%s.json\n' "$state_root" "$receipt_id"
}

validate_receipt() {
  local receipt_path="$1"
  [[ -f "$receipt_path" ]]
  "$helper" validate-receipt "$receipt_path" "$secret" "$run_dir"
}

send_mcp_requests() {
  sed -n '1p' "$run_dir/mcp.requests"
  sleep 0.1
  sed -n '2p' "$run_dir/mcp.requests"
  sed -n '3p' "$run_dir/mcp.requests"
  sleep 0.2
}

mkdir -p "$installed_dir"
if [[ "${1:-}" == "--prebuilt" ]]; then
  cp "$2/agent-runtime-proof" "$binary"
  cp "$2/witness-helper" "$helper"
else
  go build -trimpath -ldflags '-s -w -X main.version=0.3.0-phase3 -X main.commit=abcdef0' -o "$binary" "$repo_dir/cmd/agent-runtime-proof"
  go build -trimpath -o "$helper" "$repo_dir/testdata/witness-helper"
fi
chmod 700 "$binary" "$helper"

helper_digest="$(sha256_file "$helper")"
helper_size="$(file_size "$helper")"
tree_digest="$(printf '[{"path":"witness-helper","sha256":"%s","size":%s}]' "$helper_digest" "$helper_size" | sha256_text)"
echo_digest="$(printf 'echo' | sha256_text)"
secret_digest="$(printf '%s' "$secret" | sha256_text)"
expectation="$run_dir/expectation.json"
printf '%s\n' "{\"schema_version\":\"agent-runtime-expectation/1.0\",\"subject\":{\"id\":\"phase3.helper\",\"display_name\":\"Phase 3 Helper\",\"version\":\"1\"},\"launch\":{\"kind\":\"native\",\"entrypoint\":\"witness-helper\",\"argument_fingerprints\":[{\"position\":1,\"sha256\":\"$echo_digest\"},{\"position\":2,\"sha256\":\"$secret_digest\"}]},\"artifact\":{\"root\":\"$installed_dir\",\"include\":[\"witness-helper\"],\"exclude\":[],\"sha256\":\"$tree_digest\",\"max_files\":4,\"max_bytes\":134217728,\"max_duration_ms\":10000},\"policy\":{\"allowed_roots\":[\"$installed_dir\"],\"allow_symlinks\":false},\"source\":{\"kind\":\"user-file\",\"locator_hash\":\"0000000000000000000000000000000000000000000000000000000000000000\",\"trust\":\"declared\"}}" > "$expectation"
"$helper" validate-expectation "$expectation"
"$helper" sdk-prepare "$state_root" "$expectation" "$helper" echo "$secret" | grep -q '^SDK_PREPARE=PASS$'
"$helper" sdk-spawn "$state_root" "$expectation" "$helper" echo "$secret" | grep -q '^SDK_SPAWN=PASS$'

printf '\000\377\n{}\000phase3' > "$run_dir/payload.bin"
set +e
AGENT_RUNTIME_PROOF_HOME="$state_root" "$binary" witness --expectation "$expectation" --grace-period 2s -- "$helper" echo "$secret" \
  < "$run_dir/payload.bin" > "$run_dir/proxied.bin" 2> "$run_dir/proxied.stderr"
proxy_code=$?
set -e
if [[ "$proxy_code" -ne 0 ]]; then
  sed 's/token-super-secret-phase3/[redacted]/g' "$run_dir/proxied.stderr" >&2
  printf 'expectation-bound Witness exit=%s\n' "$proxy_code" >&2
  exit 1
fi
cmp "$run_dir/payload.bin" "$run_dir/proxied.bin"
grep -q '^child-stderr$' "$run_dir/proxied.stderr"
primary_receipt="$(receipt_path_from_stderr "$run_dir/proxied.stderr")"
primary_validation="$(validate_receipt "$primary_receipt")"
grep -q '^RECEIPT_ID=sha256:' <<<"$primary_validation"
if grep -Fq "$secret" "$run_dir/proxied.stderr" || grep -Fq "$run_dir" "$run_dir/proxied.stderr"; then
  printf 'Witness stderr leaked argv or task path\n' >&2
  exit 1
fi

set +e
AGENT_RUNTIME_PROOF_HOME="$state_root" "$binary" witness --grace-period 2s -- "$helper" exit 7 \
  </dev/null > "$run_dir/exit.stdout" 2> "$run_dir/exit.stderr"
exit_code=$?
set -e
[[ "$exit_code" -eq 7 ]]
validate_receipt "$(receipt_path_from_stderr "$run_dir/exit.stderr")" >/dev/null

set +e
AGENT_RUNTIME_PROOF_HOME="$state_root" "$binary" witness --grace-period 100ms -- "$helper" hang-eof \
  </dev/null > "$run_dir/eof.stdout" 2> "$run_dir/eof.stderr"
eof_code=$?
set -e
[[ "$eof_code" -ne 0 ]]
eof_validation="$(validate_receipt "$(receipt_path_from_stderr "$run_dir/eof.stderr")")"
eof_pid="$(sed -n 's/^RECEIPT_PID=//p' <<<"$eof_validation")"
assert_process_gone "$eof_pid"

printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"phase3-fixture","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' > "$run_dir/mcp.requests"
send_mcp_requests | AGENT_RUNTIME_PROOF_HOME="$state_root" "$binary" mcp \
  > "$run_dir/mcp.direct" 2> "$run_dir/mcp.direct.stderr"
send_mcp_requests | AGENT_RUNTIME_PROOF_HOME="$state_root" "$binary" witness --grace-period 2s -- "$binary" mcp \
  > "$run_dir/mcp.witness" 2> "$run_dir/mcp.witness.stderr"
cmp "$run_dir/mcp.direct" "$run_dir/mcp.witness"
[[ ! -s "$run_dir/mcp.direct.stderr" ]]
validate_receipt "$(receipt_path_from_stderr "$run_dir/mcp.witness.stderr")" >/dev/null
if grep -q 'agent-runtime-proof:' "$run_dir/mcp.witness"; then
  printf 'ARP diagnostic polluted Witness MCP stdout\n' >&2
  exit 1
fi

signal_fifo="$run_dir/signal.fifo"
mkfifo "$signal_fifo"
AGENT_RUNTIME_PROOF_HOME="$state_root" "$binary" witness --grace-period 2s -- "$helper" term \
  < "$signal_fifo" > "$run_dir/signal.stdout" 2> "$run_dir/signal.stderr" &
signal_witness_pid=$!
active_pids+=("$signal_witness_pid")
exec 9>"$signal_fifo"
wait_for_pattern "$run_dir/signal.stdout" '^ready$'
kill -TERM "$signal_witness_pid"
wait "$signal_witness_pid"
active_pids=()
exec 9>&-
grep -q '^terminated$' "$run_dir/signal.stdout"
signal_validation="$(validate_receipt "$(receipt_path_from_stderr "$run_dir/signal.stderr")")"
signal_target_pid="$(sed -n 's/^RECEIPT_PID=//p' <<<"$signal_validation")"
assert_process_gone "$signal_target_pid"

owner_fifo="$run_dir/owner.fifo"
mkfifo "$owner_fifo"
AGENT_RUNTIME_PROOF_HOME="$state_root" "$binary" witness --grace-period 2s -- "$helper" tree \
  < "$owner_fifo" > "$run_dir/owner.stdout" 2> "$run_dir/owner.stderr" &
owner_witness_pid=$!
active_pids+=("$owner_witness_pid")
exec 8>"$owner_fifo"
wait_for_pattern "$run_dir/owner.stdout" '^target_pid='
target_pid="$(sed -n 's/^target_pid=\([0-9][0-9]*\).*/\1/p' "$run_dir/owner.stdout")"
child_pid="$(sed -n 's/.*child_pid=\([0-9][0-9]*\).*/\1/p' "$run_dir/owner.stdout")"
kill -KILL "$owner_witness_pid"
set +e
wait "$owner_witness_pid" 2>/dev/null
set -e
active_pids=()
exec 8>&-
assert_process_gone "$target_pid"
assert_process_gone "$child_pid"
owner_receipt="$(grep -l "\"pid\":$target_pid" "$state_root"/launch-receipts/*.json | head -1)"
validate_receipt "$owner_receipt" >/dev/null

if grep -R -F "$secret" "$state_root" "$run_dir"/*.stderr; then
  printf 'Phase 3 persisted output leaked secret argv\n' >&2
  exit 1
fi
if command -v strings >/dev/null 2>&1 && strings "$binary" | grep -F "$repo_dir"; then
  printf 'trimmed Witness binary contains source path\n' >&2
  exit 1
fi

receipt_id="$(sed -n 's/^RECEIPT_ID=//p' <<<"$primary_validation")"
platform_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
platform_arch="$(uname -m)"
case "$platform_arch" in
  x86_64) platform_arch="amd64" ;;
  aarch64|arm64) platform_arch="arm64" ;;
esac
trap - EXIT INT TERM
cleanup
[[ ! -e "$run_dir" ]]
printf 'PHASE3_ACCEPTANCE=PASS\nPLATFORM=%s/%s\nPRIMARY_RECEIPT_ID=%s\nPHASE3_RESIDUE=NONE\n' "$platform_os" "$platform_arch" "$receipt_id"
