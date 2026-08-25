#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
source "$script_dir/host-matrix/common.sh"
if [[ "$(uname -s)" == "Darwin" ]]; then task_base="/private/tmp"; else task_base="/tmp"; fi
task_prefix="$task_base/agent-runtime-proof-phase4."
run_dir="$(mktemp -d "${task_prefix}XXXXXX")"
installed_dir="$run_dir/installed"
workspace="$run_dir/workspace"
host_home="$run_dir/host-home"
binary="$installed_dir/agent-runtime-proof"
helper="$installed_dir/phase4-helper"
active_pids=()

cleanup() {
  for task_pid in "${active_pids[@]:-}"; do
    if [[ -n "$task_pid" ]] && kill -0 "$task_pid" 2>/dev/null; then kill -TERM "$task_pid" 2>/dev/null || true; wait "$task_pid" 2>/dev/null || true; fi
  done
  case "$run_dir" in "$task_prefix"*) find "$run_dir" -depth -delete 2>/dev/null || true ;; *) printf 'refusing unsafe Phase 4 cleanup\n' >&2; return 1 ;; esac
}
trap cleanup EXIT INT TERM

mkdir -p "$installed_dir" "$workspace/.cursor" "$host_home"
touch "$run_dir/.arp-task-owned"
if [[ "${1:-}" == "--prebuilt" ]]; then
  cp "$2/agent-runtime-proof" "$binary"
  cp "$2/phase4-helper" "$helper"
else
  candidate_commit="$(git -C "$repo_dir" rev-parse --short=12 HEAD)"
  go build -trimpath -ldflags "-s -w -X main.version=0.4.0-phase4 -X main.commit=$candidate_commit" -o "$binary" "$repo_dir/cmd/agent-runtime-proof"
  go build -trimpath -o "$helper" "$repo_dir/testdata/phase4-helper"
fi
chmod 700 "$binary" "$helper"

helper_expectation="$run_dir/helper-expectation.json"
negative_expectation="$run_dir/negative-expectation.json"
candidate_expectation="$run_dir/candidate-expectation.json"
write_single_file_expectation "$helper_expectation" phase4-helper "Phase 4 Helper" 1.0.0 "$installed_dir" phase4-helper "$helper"
write_single_file_expectation "$negative_expectation" phase4-helper "Phase 4 Helper" 0.0.0 "$installed_dir" phase4-helper "$helper" "$(printf '0%.0s' {1..64})"
write_single_file_expectation "$candidate_expectation" agent-runtime-proof "Agent Runtime Proof" 0.4.0-phase4 "$installed_dir" agent-runtime-proof "$binary"

"$helper" serve > "$run_dir/helper.ready" &
helper_pid=$!
active_pids+=("$helper_pid")
for _ in $(seq 1 100); do [[ -s "$run_dir/helper.ready" ]] && break; sleep 0.02; done
[[ -s "$run_dir/helper.ready" ]]
set +e
"$binary" verify --pid "$helper_pid" --expectation "$helper_expectation" --format json > "$run_dir/generic-matched.json"
generic_exit=$?
set -e
if [[ "$generic_exit" -ne 0 ]]; then
  grep -Eo '"verdict":"[A-Z_]+"|"reason_codes":\[[^]]*\]' "$run_dir/generic-matched.json" >&2 || true
  exit "$generic_exit"
fi
set +e
"$binary" verify --pid "$helper_pid" --expectation "$negative_expectation" --format json > "$run_dir/generic-negative.json"
negative_exit=$?
set -e
[[ "$negative_exit" -eq 3 ]]
assert_safe_proof "$run_dir/generic-matched.json" MATCHED
assert_safe_proof "$run_dir/generic-negative.json" UNKNOWN
"$helper" verify-proof-file "$run_dir/generic-matched.json" > /dev/null
"$helper" verify-proof-file "$run_dir/generic-negative.json" > /dev/null

"$helper" verify-mcp "$binary" "$helper_expectation" "$helper_pid" MATCHED > "$run_dir/generic-mcp.txt"
grep -Eq '^VERDICT=MATCHED PROOF_ID=sha256:[0-9a-f]{64} TOOLS=3$' "$run_dir/generic-mcp.txt"
if command -v node >/dev/null 2>&1; then
  node "$script_dir/host-matrix/verify-proof.mjs" "$binary" "$helper_expectation" "$helper_pid" MATCHED > "$run_dir/generic-node-mcp.json"
  grep -q '"verdict":"MATCHED"' "$run_dir/generic-node-mcp.json"
fi

printf '%s\n' "{\"mcpServers\":{\"agent-runtime-proof\":{\"command\":\"$binary\",\"args\":[\"mcp\"]}}}" > "$workspace/.cursor/mcp.json"
"$helper" hold-mcp "$binary" > "$run_dir/profile-mcp.ready" 2>"$run_dir/profile-mcp.stderr" &
profile_pid=$!
active_pids+=("$profile_pid")
for _ in $(seq 1 100); do [[ -s "$run_dir/profile-mcp.ready" ]] && break; sleep 0.02; done
[[ -s "$run_dir/profile-mcp.ready" ]]
(cd "$workspace" && HOME="$host_home" "$binary" doctor --host cursor --format json > "$run_dir/profile-doctor.json")
(cd "$workspace" && HOME="$host_home" "$binary" inspect --binding cursor.agent-runtime-proof --format json > "$run_dir/profile-inspect.json")
set +e
(cd "$workspace" && HOME="$host_home" "$binary" verify --binding cursor.agent-runtime-proof --expectation "$candidate_expectation" --format json > "$run_dir/profile-matched.json")
profile_exit=$?
set -e
if [[ "$profile_exit" -ne 0 ]]; then
  grep -Eo '"verdict":"[A-Z_]+"|"reason_codes":\[[^]]*\]' "$run_dir/profile-matched.json" >&2 || true
  exit "$profile_exit"
fi
grep -q '"binding_id":"cursor.agent-runtime-proof"' "$run_dir/profile-inspect.json"
assert_safe_proof "$run_dir/profile-matched.json" MATCHED
"$helper" verify-proof-file "$run_dir/profile-matched.json" > /dev/null
grep -q '"host_id":"cursor"' "$run_dir/profile-matched.json"
kill -TERM "$profile_pid"
wait "$profile_pid"
[[ ! -s "$run_dir/profile-mcp.stderr" ]]

"$helper" measure-mcp "$binary" 20 > "$run_dir/performance.txt"
grep -Eq '^ITERATIONS=20 P95_MS=[0-9]+ MAX_RSS_BYTES=[0-9]+$' "$run_dir/performance.txt"

kill -TERM "$helper_pid"
wait "$helper_pid"
active_pids=()
printf 'Phase 4 generic/Profile acceptance PASS (%s/%s)\n' "$(uname -s)" "$(uname -m)"
