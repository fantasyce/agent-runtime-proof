#!/usr/bin/env bash
set -euo pipefail

if (($# != 1)); then
  echo 'usage: build_release_assets.sh OUTPUT_DIRECTORY' >&2
  exit 64
fi
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
output_dir="$1"
[[ "$output_dir" = /* ]] || output_dir="$PWD/$output_dir"

version="$(tr -d '\r\n' < "$repo_dir/VERSION")"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'invalid VERSION' >&2; exit 64; }
commit="$(git -C "$repo_dir" rev-parse HEAD)"
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || { echo 'invalid release commit' >&2; exit 64; }
[[ -z "$(git -C "$repo_dir" status --porcelain --untracked-files=all)" ]] || { echo 'release source tree is dirty' >&2; exit 64; }

if [[ -e "$output_dir" ]] && [[ -n "$(find "$output_dir" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
  echo 'output directory must be absent or empty' >&2
  exit 64
fi
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd -P)"

cyclonedx="${CYCLONEDX_GOMOD:-cyclonedx-gomod}"
command -v "$cyclonedx" >/dev/null || { echo 'cyclonedx-gomod is required' >&2; exit 69; }
command -v python3 >/dev/null || { echo 'python3 is required for deterministic packaging' >&2; exit 69; }

task_base="${TMPDIR:-/tmp}"
task_base="${task_base%/}"
[[ -d "$task_base" ]] || { echo 'temporary directory is unavailable' >&2; exit 69; }
build_root="$(mktemp -d "$task_base/agent-runtime-proof-release-build.XXXXXX")"
cleanup() { rm -rf "$build_root"; }
trap cleanup EXIT

source_name="agent-runtime-proof_${version}_source.tar.gz"
git -C "$repo_dir" archive --format=tar --prefix="agent-runtime-proof-${version}/" HEAD | gzip -n -9 > "$output_dir/$source_name"

targets=(darwin/arm64 linux/amd64 windows/amd64)
for target in "${targets[@]}"; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  base="agent-runtime-proof_${version}_${target_os}_${target_arch}"
  stage="$build_root/$base"
  mkdir -p "$stage/plugin"
  binary_name='agent-runtime-proof'
  archive_format='tar.gz'
  archive_name="$base.tar.gz"
  if [[ "$target_os" == windows ]]; then
    binary_name='agent-runtime-proof.exe'
    archive_format='zip'
    archive_name="$base.zip"
  fi
  GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED=0 \
    go build -C "$repo_dir" -trimpath -buildvcs=true \
      -ldflags "-s -w -X main.version=$version -X main.commit=$commit" \
      -o "$stage/$binary_name" ./cmd/agent-runtime-proof
  cp "$repo_dir/LICENSE" "$repo_dir/README.md" "$repo_dir/SECURITY.md" "$repo_dir/CHANGELOG.md" "$stage/"
  cp -R "$repo_dir/plugin/agent-runtime-proof" "$stage/plugin/"
  GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED=0 \
    "$cyclonedx" app -json -output-version 1.6 -noserial -notimestamp \
      -output "$output_dir/$base.cdx.json" -main cmd/agent-runtime-proof "$repo_dir"
  python3 "$script_dir/package_release_asset.py" \
    --format "$archive_format" --source "$stage" --root-name "$base" \
    --output "$output_dir/$archive_name"
done

checksum_tool=(shasum -a 256)
if ! command -v shasum >/dev/null; then checksum_tool=(sha256sum); fi
(
  cd "$output_dir"
  find . -maxdepth 1 -type f ! -name SHA256SUMS -print | sed 's#^./##' | LC_ALL=C sort |
    while IFS= read -r release_file; do "${checksum_tool[@]}" "$release_file"; done > SHA256SUMS
)

echo "release assets built in $output_dir"
