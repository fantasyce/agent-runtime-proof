#!/usr/bin/env bash
set -euo pipefail

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
cleanup() { rm -rf "$scan_root"; }
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
windows_archive="$dist/agent-runtime-proof_${version}_windows_amd64.zip"
windows_listing="$(unzip -Z1 "$windows_archive")"
grep -Eq '/agent-runtime-proof\.exe$' <<<"$windows_listing"
grep -Eq '/LICENSE$' <<<"$windows_listing"
if grep -Eq '(^|/)(testdata|\.git|\.DS_Store|__pycache__|node_modules|\.venv)(/|$)' <<<"$windows_listing"; then exit 1; fi
mkdir -p "$scan_root/windows_amd64"
unzip -q "$windows_archive" -d "$scan_root/windows_amd64"
if grep -ERa '/Users/[^/]+/|[A-Za-z]:\\Users\\|BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY' "$scan_root/windows_amd64"; then exit 1; fi

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

echo 'release assets verified'
