#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
task_base="${TMPDIR:-/tmp}"
task_base="${task_base%/}"
[[ -d "$task_base" ]] || { echo 'temporary directory is unavailable' >&2; exit 69; }
test_root="$(mktemp -d "$task_base/agent-runtime-proof-release-test.XXXXXX")"
cleanup() {
  case "$test_root" in
    "$task_base"/agent-runtime-proof-release-test.*) find "$test_root" -depth -delete 2>/dev/null || true ;;
    *) echo 'refusing unexpected cleanup path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT

git clone --quiet --no-hardlinks "$repo_dir" "$test_root/repo"
for relative in VERSION CHANGELOG.md README.md docs/install.md plugin/agent-runtime-proof/.codex-plugin/plugin.json scripts/build_release_assets.sh scripts/package_release_asset.py scripts/verify_release_assets.sh scripts/run_phase4_acceptance.sh scripts/host-matrix/common.sh cmd/agent-runtime-proof/main.go internal/versioninfo; do
  if [[ -e "$repo_dir/$relative" ]]; then
    mkdir -p "$test_root/repo/$(dirname "$relative")"
    if [[ -d "$repo_dir/$relative" ]]; then
      rm -rf "$test_root/repo/$relative"
      cp -R "$repo_dir/$relative" "$test_root/repo/$relative"
    else
      cp "$repo_dir/$relative" "$test_root/repo/$relative"
    fi
  fi
done
git -C "$test_root/repo" add -- VERSION CHANGELOG.md README.md docs/install.md plugin scripts cmd internal/versioninfo
git -C "$test_root/repo" -c user.name='ARP Test' -c user.email='arp-test@example.invalid' commit --quiet --allow-empty -m 'test release candidate'

mkdir -p "$test_root/bin"
cat > "$test_root/bin/cyclonedx-gomod" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
output=''
while (($#)); do
  if [[ "$1" == '-output' ]]; then output="$2"; shift 2; else shift; fi
done
test -n "$output"
printf '%s\n' '{"bomFormat":"CycloneDX","specVersion":"1.6","metadata":{"component":{"name":"github.com/fantasyce/agent-runtime-proof","version":"v1.0.1"}}}' > "$output"
FAKE
chmod +x "$test_root/bin/cyclonedx-gomod"

cd "$test_root/repo"
CYCLONEDX_GOMOD="$test_root/bin/cyclonedx-gomod" \
  bash scripts/build_release_assets.sh "$test_root/dist"
bash scripts/verify_release_assets.sh "$test_root/dist" 1.0.1 "$(git rev-parse HEAD)"
CYCLONEDX_GOMOD="$test_root/bin/cyclonedx-gomod" \
  bash scripts/build_release_assets.sh "$test_root/dist-second"
for asset in "$test_root/dist"/*; do
  name="$(basename "$asset")"
  cmp "$asset" "$test_root/dist-second/$name"
done

expected=(
  SHA256SUMS
  agent-runtime-proof_1.0.1_source.tar.gz
  agent-runtime-proof_1.0.1_darwin_arm64.tar.gz
  agent-runtime-proof_1.0.1_darwin_arm64.cdx.json
  agent-runtime-proof_1.0.1_linux_amd64.tar.gz
  agent-runtime-proof_1.0.1_linux_amd64.cdx.json
  agent-runtime-proof_1.0.1_windows_amd64.zip
  agent-runtime-proof_1.0.1_windows_amd64.cdx.json
)
actual="$(find "$test_root/dist" -maxdepth 1 -type f -exec basename {} \; | LC_ALL=C sort)"
wanted="$(printf '%s\n' "${expected[@]}" | LC_ALL=C sort)"
[[ "$actual" == "$wanted" ]]

echo 'release asset tests passed'
