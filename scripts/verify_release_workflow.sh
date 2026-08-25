#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
workflow="$repo_dir/.github/workflows/release.yml"

[[ -f "$workflow" ]] || { echo 'release workflow is missing' >&2; exit 1; }

require() {
  local pattern="$1"
  local message="$2"
  rg -q -- "$pattern" "$workflow" || { echo "$message" >&2; exit 1; }
}

require '^  push:$' 'release workflow must use a push trigger'
require '^      - ['"'"']v\*['"'"']$' 'release workflow must be tag-only'
require '^  contents: write$' 'release workflow needs release upload permission'
require '^  id-token: write$' 'release workflow needs OIDC permission'
require '^  attestations: write$' 'release workflow needs attestation permission'
require 'actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1' 'checkout must be immutable'
require 'actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e' 'setup-go must be immutable'
require 'actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a' 'upload-artifact must be immutable'
require 'actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8' 'provenance action must be immutable'
require 'git rev-parse origin/main' 'release commit must be checked against origin/main'
require 'cyclonedx-gomod/cmd/cyclonedx-gomod@v1\.12\.0' 'CycloneDX tool must be pinned'
require 'bash scripts/check\.sh' 'full local gate is required'
require 'bash scripts/verify_release_metadata\.sh' 'release metadata gate is required'
require 'bash scripts/verify_host_profiles\.sh' 'Profile gate is required'
require 'bash scripts/build_release_assets\.sh' 'release assets must be built by the repository script'
require 'bash scripts/verify_release_assets\.sh' 'release assets must be verified'
require 'gh release create' 'GitHub Release publication is required'

echo 'release workflow verified'
