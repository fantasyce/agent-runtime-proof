#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
run_dir="$(mktemp -d /private/tmp/agent-runtime-proof-phase1.XXXXXX)"
helper_pid=""

cleanup() {
  if [[ -n "$helper_pid" ]] && kill -0 "$helper_pid" 2>/dev/null; then
    kill "$helper_pid" 2>/dev/null || true
    wait "$helper_pid" 2>/dev/null || true
  fi
  case "$run_dir" in
    /private/tmp/agent-runtime-proof-phase1.*) find "$run_dir" -depth -delete 2>/dev/null || true ;;
    *) printf 'refusing to clean unexpected path\n' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

install_dir="$run_dir/install/bin"
payload_dir="$run_dir/token-secret-资料/payload"
mkdir -p "$install_dir" "$payload_dir/bin"

go build -trimpath -ldflags '-s -w -X main.version=0.1.0 -X main.commit=abcdef0' \
  -o "$install_dir/agent-runtime-proof" "$repo_dir/cmd/agent-runtime-proof"
go build -trimpath -ldflags '-X main.marker=old-loaded-image' -o "$payload_dir/bin/fixture-runtime" "$repo_dir/testdata/acceptance/helper"

"$payload_dir/bin/fixture-runtime" &
helper_pid=$!
sleep 1
go build -trimpath -ldflags '-X main.marker=new-installed-image' -o "$run_dir/replacement-runtime" "$repo_dir/testdata/acceptance/helper"
mv "$run_dir/replacement-runtime" "$payload_dir/bin/fixture-runtime"

file_digest="$(shasum -a 256 "$payload_dir/bin/fixture-runtime" | awk '{print $1}')"
file_size="$(stat -f '%z' "$payload_dir/bin/fixture-runtime")"
tree_digest="$(printf '[{"path":"bin/fixture-runtime","sha256":"%s","size":%s}]' "$file_digest" "$file_size" | shasum -a 256 | awk '{print $1}')"

write_expectation() {
  local output="$1"
  local kind="$2"
  local max_bytes="$3"
  printf '%s\n' "{\"schema_version\":\"agent-runtime-expectation/1.0\",\"subject\":{\"id\":\"fixture-runtime\",\"display_name\":\"Fixture Runtime\",\"version\":\"1.0.0\"},\"launch\":{\"kind\":\"$kind\",\"entrypoint\":\"bin/fixture-runtime\",\"argument_fingerprints\":[]},\"artifact\":{\"root\":\"$payload_dir\",\"include\":[\"**\"],\"exclude\":[],\"sha256\":\"$tree_digest\",\"max_files\":10,\"max_bytes\":$max_bytes,\"max_duration_ms\":5000},\"policy\":{\"allowed_roots\":[\"$payload_dir\"],\"allow_symlinks\":false},\"source\":{\"kind\":\"user-file\",\"locator_hash\":\"0000000000000000000000000000000000000000000000000000000000000000\",\"trust\":\"declared\"}}" > "$output"
}

expectation_path="$run_dir/expectation.json"
limit_expectation_path="$run_dir/expectation-limit.json"
tree_expectation_path="$run_dir/expectation-tree.json"
stale_expectation_path="$run_dir/expectation-stale.json"
write_expectation "$expectation_path" native 100000000
write_expectation "$limit_expectation_path" native 1
write_expectation "$tree_expectation_path" declared-tree 100000000
sed "s/$tree_digest/$(printf 'b%.0s' {1..64})/" "$expectation_path" > "$stale_expectation_path"

set +e
"$install_dir/agent-runtime-proof" verify --expectation "$expectation_path" --pid "$helper_pid" --format json > "$run_dir/replaced-loaded-image.json"
replaced_exit=$?
set -e
[[ "$replaced_exit" -eq 3 ]] || { printf 'replaced loaded image returned exit %s, want 3\n' "$replaced_exit" >&2; exit 1; }
jq -e '.verdict == "UNKNOWN" and (.reason_codes | index("POSSIBLE_STALE_AFTER_REPLACEMENT"))' "$run_dir/replaced-loaded-image.json" >/dev/null
kill "$helper_pid"
wait "$helper_pid" 2>/dev/null || true
helper_pid=""

"$payload_dir/bin/fixture-runtime" &
helper_pid=$!
sleep 1

matched_json="$run_dir/matched.json"
"$install_dir/agent-runtime-proof" verify --expectation "$expectation_path" --pid "$helper_pid" --format json > "$matched_json"
jq -e '.verdict == "MATCHED" and .proof_level == "ARTIFACT_OBSERVED" and (.proof_id | startswith("sha256:"))' "$matched_json" >/dev/null

set +e
"$install_dir/agent-runtime-proof" verify --expectation "$stale_expectation_path" --pid "$helper_pid" --known-prior-digest "$tree_digest" --format json > "$run_dir/stale.json"
stale_exit=$?
set -e
[[ "$stale_exit" -eq 2 ]] || { printf 'STALE returned exit %s, want 2\n' "$stale_exit" >&2; exit 1; }
jq -e '.verdict == "STALE" and (.reason_codes | index("ARTIFACT_MISMATCH"))' "$run_dir/stale.json" >/dev/null

set +e
"$install_dir/agent-runtime-proof" verify --expectation "$expectation_path" --pid $$ --format json > "$run_dir/leaked.json"
leaked_exit=$?
"$install_dir/agent-runtime-proof" verify --expectation "$expectation_path" --pid 99999999 --format json > "$run_dir/not-running.json"
not_running_exit=$?
"$install_dir/agent-runtime-proof" verify --expectation "$limit_expectation_path" --pid "$helper_pid" --format json > "$run_dir/limit.json"
limit_exit=$?
"$install_dir/agent-runtime-proof" verify --expectation "$tree_expectation_path" --pid "$helper_pid" --format json > "$run_dir/tree.json"
tree_exit=$?
set -e

[[ "$leaked_exit" -eq 2 ]] || { printf 'LEAKED returned exit %s, want 2\n' "$leaked_exit" >&2; exit 1; }
jq -e '.verdict == "LEAKED" and (.reason_codes | index("RUNTIME_OUTSIDE_ALLOWED_ROOT"))' "$run_dir/leaked.json" >/dev/null
[[ "$not_running_exit" -eq 2 ]] || { printf 'NOT_RUNNING returned exit %s, want 2\n' "$not_running_exit" >&2; exit 1; }
jq -e '.verdict == "NOT_RUNNING" and .observation.process == null' "$run_dir/not-running.json" >/dev/null
[[ "$limit_exit" -eq 3 ]] || { printf 'scan limit returned exit %s, want 3\n' "$limit_exit" >&2; exit 1; }
jq -e '.verdict == "UNKNOWN" and (.reason_codes | index("ARTIFACT_SCAN_LIMIT_EXCEEDED"))' "$run_dir/limit.json" >/dev/null
[[ "$tree_exit" -eq 3 ]] || { printf 'declared tree returned exit %s, want 3\n' "$tree_exit" >&2; exit 1; }
jq -e '.verdict == "UNKNOWN" and (.reason_codes | index("PLATFORM_EVIDENCE_UNAVAILABLE"))' "$run_dir/tree.json" >/dev/null

"$install_dir/agent-runtime-proof" inspect --pid "$helper_pid" --format json > "$run_dir/inspect.json"
"$install_dir/agent-runtime-proof" doctor --format json > "$run_dir/doctor.json"
jq -e '.proofs | length == 1' "$run_dir/inspect.json" >/dev/null
jq -e '.status == "ok"' "$run_dir/doctor.json" >/dev/null

if grep -En 'token-secret|/Users/|process\.argv[^\"]*:' \
  "$matched_json" "$run_dir/leaked.json" "$run_dir/not-running.json" "$run_dir/limit.json" \
  "$run_dir/tree.json" "$run_dir/stale.json" "$run_dir/replaced-loaded-image.json" "$run_dir/inspect.json" "$run_dir/doctor.json"; then
  printf 'privacy scan found prohibited output\n' >&2
  exit 1
fi
if strings "$install_dir/agent-runtime-proof" | grep -F "$repo_dir"; then
  printf 'trimmed binary contains source path\n' >&2
  exit 1
fi

proof_id="$(jq -r '.proof_id' "$matched_json")"
kill "$helper_pid"
wait "$helper_pid" 2>/dev/null || true
helper_pid=""
trap - EXIT INT TERM
cleanup
[[ ! -e "$run_dir" ]]
printf 'Phase 1A macOS acceptance PASS proof_id=%s\n' "$proof_id"
