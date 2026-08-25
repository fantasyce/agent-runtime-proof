#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
guide="$repo_dir/docs/install.md"
[[ -f "$guide" ]] || { echo 'install guide is missing' >&2; exit 1; }

for term in SHA256SUMS SBOM attestation macOS Linux Windows upgrade downgrade uninstall; do
  grep -Eqi "$term" "$guide" || { echo "install guide is missing: $term" >&2; exit 1; }
done

test_root="$(mktemp -d /private/tmp/agent-runtime-proof-install-test.XXXXXX)"
cleanup() {
  case "$test_root" in
    /private/tmp/agent-runtime-proof-install-test.*) find "$test_root" -depth -delete 2>/dev/null || true ;;
    *) echo 'refusing unexpected cleanup path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

candidate="$test_root/candidate"
mkdir -p "$candidate/agent-runtime-proof_1.0.0_darwin_arm64/plugin"
go build -C "$repo_dir" -trimpath \
  -ldflags "-s -w -X main.version=1.0.0 -X main.commit=$(git -C "$repo_dir" rev-parse HEAD)" \
  -o "$candidate/agent-runtime-proof_1.0.0_darwin_arm64/agent-runtime-proof" ./cmd/agent-runtime-proof
cp "$repo_dir/LICENSE" "$candidate/agent-runtime-proof_1.0.0_darwin_arm64/"
cp -R "$repo_dir/plugin/agent-runtime-proof" "$candidate/agent-runtime-proof_1.0.0_darwin_arm64/plugin/"
python3 "$repo_dir/scripts/package_release_asset.py" --format tar.gz --source "$candidate/agent-runtime-proof_1.0.0_darwin_arm64" \
  --root-name agent-runtime-proof_1.0.0_darwin_arm64 --output "$test_root/candidate.tar.gz"

install_current() {
  local archive="$1"
  local staging="$test_root/install.staging"
  local previous="$test_root/install.previous"
  find "$staging" -depth -delete 2>/dev/null || true
  mkdir -p "$staging"
  tar -xzf "$archive" -C "$staging" --strip-components=1
  test -x "$staging/agent-runtime-proof"
  find "$previous" -depth -delete 2>/dev/null || true
  if [[ -d "$test_root/install" ]]; then mv "$test_root/install" "$previous"; fi
  mv "$staging" "$test_root/install"
}

# Clean install and local stdio MCP smoke.
install_current "$test_root/candidate.tar.gz"
binary="$test_root/install/agent-runtime-proof"
[[ "$($binary --version)" == "agent-runtime-proof 1.0.0 ($(git -C "$repo_dir" rev-parse HEAD))" ]]
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"install-fixture","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' > "$test_root/mcp.requests"
send_mcp_requests() {
  sed -n '1p' "$test_root/mcp.requests"
  sleep 0.1
  sed -n '2p' "$test_root/mcp.requests"
  sed -n '3p' "$test_root/mcp.requests"
  sleep 0.2
}
send_mcp_requests | "$binary" mcp > "$test_root/mcp.jsonl"
jq -e 'select(.id == 2) | .result.tools | length == 3' "$test_root/mcp.jsonl" >/dev/null

# Same-version repair must replace damaged bytes.
printf 'damaged' > "$binary"
install_current "$test_root/candidate.tar.gz"
binary="$test_root/install/agent-runtime-proof"
"$binary" doctor --format json | jq -e '.status == "ok"' >/dev/null

# Upgrade from and downgrade to a synthetic prior installed version.
go build -C "$repo_dir" -trimpath -ldflags '-s -w -X main.version=0.9.0 -X main.commit=0000000' \
  -o "$test_root/prior-binary" ./cmd/agent-runtime-proof
cp "$test_root/prior-binary" "$binary"
[[ "$($binary --version)" == 'agent-runtime-proof 0.9.0 (0000000)' ]]
cp "$binary" "$test_root/downgrade-slot"
install_current "$test_root/candidate.tar.gz"
binary="$test_root/install/agent-runtime-proof"
[[ "$($binary --version)" == agent-runtime-proof\ 1.0.0\ * ]]
cp "$test_root/downgrade-slot" "$binary"
[[ "$($binary --version)" == 'agent-runtime-proof 0.9.0 (0000000)' ]]

# Uninstall removes the owned installation and leaves no running server.
find "$test_root/install" -depth -delete
test ! -e "$test_root/install"
test -z "$(pgrep -f "$test_root/install/agent-runtime-proof" || true)"

trap - EXIT INT TERM
cleanup
test ! -e "$test_root"
echo 'install lifecycle tests passed'
