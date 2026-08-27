#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
tmp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
test_root="$(mktemp -d "$tmp_root/arp-mcpb-test.XXXXXX")"
cleanup() {
  case "$test_root" in
    "$tmp_root"/arp-mcpb-test.*) find "$test_root" -depth -delete 2>/dev/null || true ;;
    *) echo 'refusing to clean unexpected MCPB test path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

version=1.0.1
commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa

make_fixture_dist() {
  local dist="$1"
  mkdir -p "$dist/stage/darwin/agent-runtime-proof_${version}_darwin_arm64"
  mkdir -p "$dist/stage/linux/agent-runtime-proof_${version}_linux_amd64"
  mkdir -p "$dist/stage/windows/agent-runtime-proof_${version}_windows_amd64"
  printf 'darwin-arm64-binary\n' > "$dist/stage/darwin/agent-runtime-proof_${version}_darwin_arm64/agent-runtime-proof"
  printf 'linux-amd64-binary\n' > "$dist/stage/linux/agent-runtime-proof_${version}_linux_amd64/agent-runtime-proof"
  printf 'windows-amd64-binary\n' > "$dist/stage/windows/agent-runtime-proof_${version}_windows_amd64/agent-runtime-proof.exe"
  chmod 0755 "$dist/stage/darwin/agent-runtime-proof_${version}_darwin_arm64/agent-runtime-proof" "$dist/stage/linux/agent-runtime-proof_${version}_linux_amd64/agent-runtime-proof"
  tar -czf "$dist/agent-runtime-proof_${version}_darwin_arm64.tar.gz" -C "$dist/stage/darwin" "agent-runtime-proof_${version}_darwin_arm64"
  tar -czf "$dist/agent-runtime-proof_${version}_linux_amd64.tar.gz" -C "$dist/stage/linux" "agent-runtime-proof_${version}_linux_amd64"
  (cd "$dist/stage/windows" && zip -q -r "$dist/agent-runtime-proof_${version}_windows_amd64.zip" "agent-runtime-proof_${version}_windows_amd64")
  find "$dist/stage" -depth -delete
}

make_fixture_dist "$test_root/dist-one"
make_fixture_dist "$test_root/dist-two"

python3 "$repo_dir/scripts/build_mcpb.py" --dist "$test_root/dist-one" --version "$version" --commit "$commit"
python3 "$repo_dir/scripts/build_mcpb.py" --dist "$test_root/dist-two" --version "$version" --commit "$commit"
cmp "$test_root/dist-one/agent-runtime-proof_${version}.mcpb" "$test_root/dist-two/agent-runtime-proof_${version}.mcpb"
cmp "$test_root/dist-one/server.json" "$test_root/dist-two/server.json"
python3 "$repo_dir/scripts/render_registry_metadata.py" \
  --template "$repo_dir/packaging/mcp-registry/server.json.in" \
  --mcpb "$test_root/dist-one/agent-runtime-proof_${version}.mcpb" \
  --version "$version" \
  --output "$test_root/rendered-server.json"
cmp "$test_root/dist-one/server.json" "$test_root/rendered-server.json"
python3 "$repo_dir/scripts/verify_registry_metadata.py" --server "$test_root/dist-one/server.json" --mcpb "$test_root/dist-one/agent-runtime-proof_${version}.mcpb" --version "$version"

python3 - "$test_root/dist-one" "$version" <<'PY'
import hashlib
import json
import pathlib
import stat
import sys
import zipfile

dist = pathlib.Path(sys.argv[1])
version = sys.argv[2]
bundle = dist / f"agent-runtime-proof_{version}.mcpb"
with zipfile.ZipFile(bundle) as archive:
    assert archive.namelist() == [
        "LICENSE",
        "assets/icon.svg",
        "manifest.json",
        "server/agent-runtime-proof-darwin-arm64",
        "server/agent-runtime-proof-linux-amd64",
        "server/agent-runtime-proof-windows-amd64.exe",
    ]
    manifest = json.loads(archive.read("manifest.json"))
    assert manifest["manifest_version"] == "0.3"
    assert manifest["version"] == version
    assert manifest["license"] == "Apache-2.0"
    assert manifest["compatibility"]["platforms"] == ["darwin", "linux", "win32"]
    assert [tool["name"] for tool in manifest["tools"]] == [
        "list_local_runtime_candidates",
        "inspect_local_runtimes",
        "verify_local_runtime",
    ]
    for name in archive.namelist()[3:]:
        mode = archive.getinfo(name).external_attr >> 16
        assert mode & stat.S_IXUSR, (name, oct(mode))
    for info in archive.infolist():
        assert info.date_time == (1980, 1, 1, 0, 0, 0)

server = json.loads((dist / "server.json").read_text(encoding="utf-8"))
package = server["packages"][0]
assert package["fileSha256"] == hashlib.sha256(bundle.read_bytes()).hexdigest()
assert not any(token in bundle.read_bytes() for token in (b"/Users/", b"BEGIN PRIVATE KEY", b"__pycache__"))
PY
