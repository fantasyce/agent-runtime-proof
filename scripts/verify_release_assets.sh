#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"

if (($# != 3)); then
  echo 'usage: verify_release_assets.sh DIST VERSION COMMIT' >&2
  exit 64
fi
dist="$1"
version="$2"
commit="$3"
[[ -d "$dist" && "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ && "$commit" =~ ^[0-9a-f]{40}$ ]] || exit 64

expected=(
  SHA256SUMS
  "agent-runtime-proof_${version}_source.tar.gz"
  "agent-runtime-proof_${version}_darwin_arm64.tar.gz"
  "agent-runtime-proof_${version}_darwin_arm64.cdx.json"
  "agent-runtime-proof_${version}_linux_amd64.tar.gz"
  "agent-runtime-proof_${version}_linux_amd64.cdx.json"
  "agent-runtime-proof_${version}_windows_amd64.zip"
  "agent-runtime-proof_${version}_windows_amd64.cdx.json"
  "agent-runtime-proof_${version}.mcpb"
  server.json
)
actual="$(find "$dist" -maxdepth 1 -type f -exec basename {} \; | LC_ALL=C sort)"
wanted="$(printf '%s\n' "${expected[@]}" | LC_ALL=C sort)"
[[ "$actual" == "$wanted" ]] || { echo 'release file set mismatch' >&2; exit 1; }

(
  cd "$dist"
  if command -v shasum >/dev/null; then shasum -a 256 -c SHA256SUMS; else sha256sum -c SHA256SUMS; fi
)

for target in darwin_arm64 linux_amd64 windows_amd64; do
  jq -e --arg version "$version" '
    .bomFormat == "CycloneDX" and .specVersion == "1.6" and
    .metadata.component.name == "github.com/fantasyce/agent-runtime-proof" and
    .metadata.component.version == ("v" + $version)
  ' "$dist/agent-runtime-proof_${version}_${target}.cdx.json" >/dev/null
done

task_base="${TMPDIR:-/tmp}"
task_base="${task_base%/}"
[[ -d "$task_base" ]] || { echo 'temporary directory is unavailable' >&2; exit 69; }
scan_root="$(mktemp -d "$task_base/agent-runtime-proof-release-scan.XXXXXX")"
mcp_pid=""
cleanup() {
  if [[ -n "$mcp_pid" ]]; then
    kill "$mcp_pid" 2>/dev/null || true
    wait "$mcp_pid" 2>/dev/null || true
  fi
  case "$scan_root" in
    "$task_base"/agent-runtime-proof-release-scan.*) find "$scan_root" -depth -delete 2>/dev/null || true ;;
    *) echo 'refusing unexpected release scan cleanup path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT

for target in darwin_arm64 linux_amd64; do
  archive="$dist/agent-runtime-proof_${version}_${target}.tar.gz"
  listing="$(tar -tzf "$archive")"
  grep -Eq "/agent-runtime-proof$" <<<"$listing"
  grep -Eq '/LICENSE$' <<<"$listing"
  if grep -Eq '(^|/)(testdata|\.git|\.DS_Store|__pycache__|node_modules|\.venv)(/|$)' <<<"$listing"; then exit 1; fi
  target_root="$scan_root/$target"
  mkdir -p "$target_root"
  tar -xzf "$archive" -C "$target_root"
  if grep -ERa '/Users/[^/]+/|[A-Za-z]:\\Users\\|BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY' "$target_root"; then exit 1; fi
done
darwin_binary="$scan_root/darwin_arm64/agent-runtime-proof_${version}_darwin_arm64/agent-runtime-proof"
go version -m "$darwin_binary" | grep -Eq '^[[:space:]]*build[[:space:]]+CGO_ENABLED=1$' || {
  echo 'Darwin release binary must use the native cgo process observer' >&2
  exit 1
}
windows_archive="$dist/agent-runtime-proof_${version}_windows_amd64.zip"
windows_listing="$(unzip -Z1 "$windows_archive")"
grep -Eq '/agent-runtime-proof\.exe$' <<<"$windows_listing"
grep -Eq '/LICENSE$' <<<"$windows_listing"
if grep -Eq '(^|/)(testdata|\.git|\.DS_Store|__pycache__|node_modules|\.venv)(/|$)' <<<"$windows_listing"; then exit 1; fi
mkdir -p "$scan_root/windows_amd64"
unzip -q "$windows_archive" -d "$scan_root/windows_amd64"
if grep -ERa '/Users/[^/]+/|[A-Za-z]:\\Users\\|BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY' "$scan_root/windows_amd64"; then exit 1; fi

mcpb="$dist/agent-runtime-proof_${version}.mcpb"
mcpb_listing="$(unzip -Z1 "$mcpb")"
expected_mcpb_listing=$'LICENSE\nassets/icon.svg\nmanifest.json\nserver/agent-runtime-proof-darwin-arm64\nserver/agent-runtime-proof-linux-amd64\nserver/agent-runtime-proof-windows-amd64.exe'
[[ "$mcpb_listing" == "$expected_mcpb_listing" ]] || { echo 'MCPB file set mismatch' >&2; exit 1; }
mkdir -p "$scan_root/mcpb"
unzip -q "$mcpb" -d "$scan_root/mcpb"
if grep -ERa '/Users/[^/]+/|[A-Za-z]:\\Users\\|BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY|__pycache__|node_modules|\.git/' "$scan_root/mcpb"; then exit 1; fi
python3 "$script_dir/verify_registry_metadata.py" --server "$dist/server.json" --mcpb "$mcpb" --version "$version"
python3 - "$scan_root/mcpb/manifest.json" "$version" "$commit" <<'PY'
import json
import pathlib
import sys
manifest = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert manifest["manifest_version"] == "0.3"
assert manifest["version"] == sys.argv[2]
assert manifest["_meta"]["io.agent-runtime-proof/build"]["commit"] == sys.argv[3]
assert manifest["server"]["type"] == "binary"
assert manifest["server"]["mcp_config"]["args"] == ["mcp"]
assert [tool["name"] for tool in manifest["tools"]] == [
    "list_local_runtime_candidates", "inspect_local_runtimes", "verify_local_runtime"
]
PY

source_archive="$dist/agent-runtime-proof_${version}_source.tar.gz"
prefix="agent-runtime-proof-${version}/"
source_listing="$(tar -tzf "$source_archive")"
awk -v prefix="$prefix" 'index($0,prefix)!=1 {exit 1}' <<<"$source_listing"
if grep -Eq '(^|/)(\.git|\.DS_Store|__pycache__|node_modules|\.venv)(/|$)' <<<"$source_listing"; then exit 1; fi

case "$(uname -s)/$(uname -m)" in
  Darwin/arm64) native_target=darwin_arm64 ;;
  Linux/x86_64|Linux/amd64) native_target=linux_amd64 ;;
  *) echo "native version smoke unavailable on $(uname -s)/$(uname -m)" >&2; exit 69 ;;
esac
native_archive="$dist/agent-runtime-proof_${version}_${native_target}.tar.gz"
native_root="$scan_root/native"
mkdir -p "$native_root"
tar -xzf "$native_archive" -C "$native_root"
native_binary="$native_root/agent-runtime-proof_${version}_${native_target}/agent-runtime-proof"
[[ "$("$native_binary" --version)" == "agent-runtime-proof $version ($commit)" ]]

if [[ "$native_target" == darwin_arm64 ]]; then
  mcpb_native="$scan_root/mcpb/server/agent-runtime-proof-darwin-arm64"
else
  mcpb_native="$scan_root/mcpb/server/agent-runtime-proof-linux-amd64"
fi
chmod 0755 "$mcpb_native"
[[ "$("$mcpb_native" --version)" == "agent-runtime-proof $version ($commit)" ]]
"$mcpb_native" doctor --format json | jq -e '.status == "ok"' >/dev/null
mkfifo "$scan_root/mcpb.stdin" "$scan_root/mcpb.stdout"
"$mcpb_native" mcp < "$scan_root/mcpb.stdin" > "$scan_root/mcpb.stdout" 2> "$scan_root/mcpb-stderr.txt" &
mcp_pid=$!
exec 8>"$scan_root/mcpb.stdin"
exec 9<"$scan_root/mcpb.stdout"
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"mcpb-release-smoke","version":"1"}}}' >&8
IFS= read -r initialize_response <&9
jq -e '.id == 1 and .result.protocolVersion == "2025-06-18"' <<<"$initialize_response" >/dev/null
printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&8
printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' >&8
IFS= read -r tools_response <&9
jq -e '.id == 2 and (.result.tools | map(.name) | sort == ["inspect_local_runtimes","list_local_runtime_candidates","verify_local_runtime"])' <<<"$tools_response" >/dev/null
exec 8>&-
exec 9<&-
wait "$mcp_pid"
mcp_pid=""
[[ ! -s "$scan_root/mcpb-stderr.txt" ]]

if [[ "$native_target" == darwin_arm64 ]]; then
  prebuilt_root="$scan_root/darwin-native-acceptance"
  mkdir -p "$prebuilt_root"
  cp "$native_binary" "$prebuilt_root/agent-runtime-proof"
  go build -C "$repo_dir" -trimpath -o "$prebuilt_root/phase4-helper" ./testdata/phase4-helper
  bash "$script_dir/run_phase4_acceptance.sh" --prebuilt "$prebuilt_root"
fi

echo 'release assets verified'
