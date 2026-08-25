#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_dir"

hosts=(aaa codex claude-code cursor opencode deepseek-harness vscode-copilot)
fixtures=(aaa codex claude-code cursor opencode deepseek-harness vscode-copilot)

for host in "${hosts[@]}"; do
  test -f "profiles/hosts/$host.json"
  test -f "docs/hosts/$host.md"
  rg -q "host_id.*$host|Host ID.*$host" "docs/hosts/$host.md"
done

for fixture in "${fixtures[@]}"; do
  test -d "testdata/host-configs/$fixture"
done

test "$(rg -c 'mcp\.AddTool\(' internal/mcpserver/server.go)" = "3"
rg -q '"command": "agent-runtime-proof"' plugin/agent-runtime-proof/.mcp.json
rg -q '"args": \["mcp"\]' plugin/agent-runtime-proof/.mcp.json
if rg -n '"command"[[:space:]]*:[[:space:]]*"(sh|bash|zsh|fish|cmd|powershell|pwsh)(\.exe)?"' plugin testdata/host-configs profiles/hosts; then
  echo "shell wrapper found in a host configuration" >&2
  exit 1
fi

plugin_version="$(sed -n 's/.*"version": "\([^"]*\)".*/\1/p' plugin/agent-runtime-proof/.codex-plugin/plugin.json | head -1)"
binary_version="$(sed -n 's/[[:space:]]*version = "\([^"]*\)"/\1/p' cmd/agent-runtime-proof/main.go)"
test -n "$plugin_version"
test "$plugin_version" = "$binary_version"

go test ./internal/contract ./internal/hostprofile ./internal/mcpserver -run 'TestEmbeddedCatalog|TestValidateHostProfile|TestServerPublishesExactlyThree' -count=1

if rg -n '/Users/fanhcy|[A-Za-z]:\\Users\\fanhcy|AKIA[0-9A-Z]{16}|sk-[A-Za-z0-9]{20,}' docs/hosts docs/host-configuration.md plugin testdata/host-configs profiles/hosts; then
  echo "developer path or credential-shaped text found" >&2
  exit 1
fi

echo "host profiles, fixtures, guides, and plugin metadata verified"
