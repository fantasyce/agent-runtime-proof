#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_dir"

output_file="$(mktemp "${TMPDIR:-/tmp}/arp-demo-proof.XXXXXX")"
cleanup() {
  case "$output_file" in
    "${TMPDIR:-/tmp}"/arp-demo-proof.*) rm -f "$output_file" ;;
    *) echo 'refusing to clean unexpected demo test path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

set +e
bash scripts/demo_stale_runtime.sh --json >"$output_file"
demo_exit=$?
set -e
[[ "$demo_exit" -eq 3 ]] || {
  echo "demo returned $demo_exit, want ARP UNKNOWN exit 3" >&2
  exit 1
}

python3 - "$output_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
proof = json.loads(path.read_text(encoding="utf-8"))
assert proof["verdict"] == "UNKNOWN", proof["verdict"]
assert "POSSIBLE_STALE_AFTER_REPLACEMENT" in proof["reason_codes"], proof["reason_codes"]
assert proof["proof_level"] == "ARTIFACT_OBSERVED", proof["proof_level"]

serialized = json.dumps(proof, sort_keys=True)
for prohibited in ("arp-runtime-demo.", '"argv"', '"environment"', "/Users/", "/private/tmp/"):
    assert prohibited not in serialized, prohibited
PY

if pgrep -f 'arp-runtime-demo\..*/fixture-runtime' >/dev/null 2>&1; then
  echo 'demo helper process leaked' >&2
  exit 1
fi
