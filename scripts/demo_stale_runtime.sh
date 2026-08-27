#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
tmp_root="${TMPDIR:-/tmp}"
tmp_root="${tmp_root%/}"
tmp_root="$(cd "$tmp_root" && pwd -P)"
run_dir="$(mktemp -d "$tmp_root/arp-runtime-demo.XXXXXX")"
helper_pid=""

cleanup() {
  if [[ -n "$helper_pid" ]] && kill -0 "$helper_pid" 2>/dev/null; then
    kill "$helper_pid" 2>/dev/null || true
    wait "$helper_pid" 2>/dev/null || true
  fi
  case "$run_dir" in
    "$tmp_root"/arp-runtime-demo.*) find "$run_dir" -depth -delete 2>/dev/null || true ;;
    *) echo 'refusing to clean unexpected demo path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

if [[ $# -gt 1 || ( $# -eq 1 && "$1" != "--json" ) ]]; then
  echo 'usage: demo_stale_runtime.sh [--json]' >&2
  exit 64
fi
json_only=false
[[ $# -eq 1 ]] && json_only=true

install_dir="$run_dir/install/bin"
payload_dir="$run_dir/runtime/payload"
mkdir -p "$install_dir" "$payload_dir/bin"

go build -trimpath -ldflags '-s -w -X main.version=demo -X main.commit=de00000' \
  -o "$install_dir/agent-runtime-proof" "$repo_dir/cmd/agent-runtime-proof"
go build -trimpath -ldflags '-X main.marker=approved-old-runtime' \
  -o "$payload_dir/bin/fixture-runtime" "$repo_dir/testdata/acceptance/helper"

"$payload_dir/bin/fixture-runtime" &
helper_pid=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do
  kill -0 "$helper_pid" 2>/dev/null && break
  sleep 0.1
done
kill -0 "$helper_pid" 2>/dev/null || { echo 'demo helper did not start' >&2; exit 70; }

go build -trimpath -ldflags '-X main.marker=replacement-on-disk' \
  -o "$run_dir/replacement-runtime" "$repo_dir/testdata/acceptance/helper"
mv "$run_dir/replacement-runtime" "$payload_dir/bin/fixture-runtime"

python3 - "$payload_dir" "$run_dir/expectation.json" <<'PY'
import hashlib
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
output = pathlib.Path(sys.argv[2])
binary = root / "bin" / "fixture-runtime"
digest = hashlib.sha256(binary.read_bytes()).hexdigest()
entry = [{"path": "bin/fixture-runtime", "sha256": digest, "size": binary.stat().st_size}]
tree_bytes = json.dumps(entry, separators=(",", ":")).encode("utf-8")
tree_digest = hashlib.sha256(tree_bytes).hexdigest()
expectation = {
    "schema_version": "agent-runtime-expectation/1.0",
    "subject": {"id": "arp.demo.runtime", "display_name": "ARP stale-runtime demo", "version": "replacement"},
    "launch": {"kind": "native", "entrypoint": "bin/fixture-runtime", "argument_fingerprints": []},
    "artifact": {
        "root": str(root),
        "include": ["**"],
        "exclude": [],
        "sha256": tree_digest,
        "max_files": 10,
        "max_bytes": 100000000,
        "max_duration_ms": 5000,
    },
    "policy": {"allowed_roots": [str(root)], "allow_symlinks": False},
    "source": {"kind": "user-file", "locator_hash": "0" * 64, "trust": "declared"},
}
output.write_text(json.dumps(expectation, separators=(",", ":")) + "\n", encoding="utf-8")
PY

proof_file="$run_dir/proof.json"
set +e
"$install_dir/agent-runtime-proof" verify \
  --expectation "$run_dir/expectation.json" \
  --pid "$helper_pid" \
  --format json >"$proof_file"
verify_exit=$?
set -e

[[ "$verify_exit" -eq 3 ]] || {
  echo "demo verification returned $verify_exit, want UNKNOWN exit 3" >&2
  exit 70
}

if $json_only; then
  cat "$proof_file"
  exit 3
fi

python3 - "$proof_file" <<'PY'
import json
import pathlib
import sys

proof = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print("1. Started the approved local runtime.")
print("2. Replaced its executable on disk while the old process kept running.")
print("3. Asked ARP whether the live process is proven to be the replacement.")
print()
print(f"Verdict: {proof['verdict']}")
print("Reason: " + ", ".join(proof["reason_codes"]))
print(f"Proof level: {proof['proof_level']}")
print()
print("ARP detected replacement evidence but did not claim it had observed the old loaded bytes.")
print("That refusal to turn incomplete evidence into MATCHED or STALE is the safety property.")
PY

exit 0
