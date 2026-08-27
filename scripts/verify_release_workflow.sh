#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
workflow="$repo_dir/.github/workflows/release.yml"
registry_workflow="$repo_dir/.github/workflows/publish-mcp.yml"

[[ -f "$workflow" ]] || { echo 'release workflow is missing' >&2; exit 1; }
[[ -f "$registry_workflow" ]] || { echo 'MCP Registry workflow is missing' >&2; exit 1; }

require() {
  local pattern="$1"
  local message="$2"
  grep -Eq -- "$pattern" "$workflow" || { echo "$message" >&2; exit 1; }
}

require '^  push:$' 'release workflow must use a push trigger'
require '^      - ['"'"']v\*['"'"']$' 'release workflow must be tag-only'
require '^  contents: read$' 'release workflow must default to read-only contents'
require '^      contents: write$' 'release publication job needs release upload permission'
require '^      id-token: write$' 'release build job needs job-scoped OIDC permission'
require '^      attestations: write$' 'release build job needs attestation permission'
require '^    runs-on: macos-14$' 'release workflow must build the native Darwin asset on macOS'
require 'actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1' 'checkout must be immutable'
require 'actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e' 'setup-go must be immutable'
require 'actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a' 'upload-artifact must be immutable'
require 'actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8' 'provenance action must be immutable'
require 'git rev-parse origin/main' 'release commit must be checked against origin/main'
require 'cyclonedx-gomod/cmd/cyclonedx-gomod@v1\.12\.0' 'CycloneDX tool must be pinned'
require 'bash scripts/check\.sh' 'full local gate is required'
require 'bash scripts/verify_release_metadata\.sh' 'release metadata gate is required'
require 'bash scripts/verify_host_profiles\.sh' 'Profile gate is required'
require 'bash scripts/test_install_lifecycle\.sh' 'release workflow must exercise install lifecycle gates'
require 'bash scripts/test_release_assets\.sh' 'release workflow must compare deterministic release rebuilds'
require 'bash scripts/build_release_assets\.sh' 'release assets must be built by the repository script'
require 'bash scripts/verify_release_assets\.sh' 'release assets must be verified'
require 'scripts/smoke_mcpb\.py' 'release workflow must run the final MCPB on native hosts'
require 'ubuntu-24\.04' 'release workflow must smoke the final Linux MCPB binary'
require 'windows-2025' 'release workflow must smoke the final Windows MCPB binary'
require 'actions/download-artifact@70fc10c6e5e1ce46ad2ea6f2b72d43f7d47b13c3' 'download-artifact must be immutable'
require 'needs: \[build, smoke\]' 'public Release must wait for build and native MCPB smoke jobs'
require 'gh release create' 'GitHub Release publication is required'

registry_require() {
  local pattern="$1"
  local message="$2"
  grep -Eq -- "$pattern" "$registry_workflow" || { echo "$message" >&2; exit 1; }
}

registry_require '^  workflow_run:$' 'Registry publication must wait for the Release workflow'
registry_require '^      - Release$' 'Registry publication must depend on the Release workflow'
registry_require "github\.event\.workflow_run\.conclusion == 'success'" 'Registry publication must require a successful Release workflow'
registry_require '^      id-token: write$' 'Registry publication needs job-scoped GitHub OIDC permission'
registry_require '^      contents: read$' 'Registry publication must use read-only repository permission'
registry_require 'ref: v1\.1\.0' 'Registry publication must check out the exact release tag'
registry_require 'mcp-publisher_linux_amd64\.tar\.gz' 'Registry publisher download is missing'
registry_require 'releases/download/v1\.8\.1/' 'Registry publisher must be version-pinned'
registry_require 'a06c9096dcb9727c13555b6be26c7effa707b01f06a4c561ba7a3635443cf2cc' 'Registry publisher checksum must be pinned'
registry_require 'login github-oidc' 'Registry publication must use GitHub OIDC'
registry_require 'publish .*server\.json' 'Registry publication command is missing'
registry_require 'registry\.modelcontextprotocol\.io/v0\.1/servers/' 'Registry publication must verify the public API'
if grep -Eq 'secrets\.(MCP|REGISTRY|GITHUB).*TOKEN|MCP_GITHUB_TOKEN' "$registry_workflow"; then
  echo 'Registry workflow must not use a repository token secret' >&2
  exit 1
fi

echo 'release workflow verified'
