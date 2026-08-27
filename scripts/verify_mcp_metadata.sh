#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
cd "$repo_dir"

python3 - testdata/mcp/golden-prompts.json <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
document = json.loads(path.read_text(encoding="utf-8"))
assert document["schema_version"] == "arp-mcp-golden-prompts/1.0"
cases = document["cases"]
assert len(cases) >= 10
assert sum(case["group"] == "positive" for case in cases) >= 5
assert sum(case["group"] == "negative" for case in cases) >= 5
ids = [case["id"] for case in cases]
assert len(ids) == len(set(ids))
allowed_actions = {"call", "clarify", "do_not_call"}
allowed_tools = {
    "list_local_runtime_candidates",
    "inspect_local_runtimes",
    "verify_local_runtime",
}
for case in cases:
    assert set(case) == {"id", "group", "prompt", "expected_action", "expected_tool", "rationale"}
    assert case["group"] in {"positive", "negative"}
    assert case["expected_action"] in allowed_actions
    assert isinstance(case["prompt"], str) and case["prompt"].strip()
    assert isinstance(case["rationale"], str) and case["rationale"].strip()
    if case["expected_action"] == "call":
        assert case["expected_tool"] in allowed_tools
    else:
        assert case["expected_tool"] is None
PY

version="$(tr -d '\r\n' < VERSION)"
plugin_version="$(python3 -c 'import json; print(json.load(open("plugin/agent-runtime-proof/.codex-plugin/plugin.json"))["version"])')"
plugin_short_description="$(python3 -c 'import json; print(json.load(open("plugin/agent-runtime-proof/.codex-plugin/plugin.json"))["interface"]["shortDescription"])')"
plugin_default_prompt="$(python3 -c 'import json; print(json.load(open("plugin/agent-runtime-proof/.codex-plugin/plugin.json"))["interface"]["defaultPrompt"])')"
[[ "$version" == "$plugin_version" ]] || {
  echo "plugin version $plugin_version does not match VERSION $version" >&2
  exit 1
}
[[ "$plugin_short_description" == *'approved artifact'* ]] || { echo 'plugin short description lacks the approved-artifact outcome' >&2; exit 1; }
[[ "$plugin_default_prompt" == *'local'* && "$plugin_default_prompt" == *'expectation'* ]] || { echo 'plugin default prompt lacks local expectation guidance' >&2; exit 1; }

grep -Fq 'Do not use Agent Runtime Proof' plugin/agent-runtime-proof/skills/agent-runtime-proof/SKILL.md

for tool in list_local_runtime_candidates inspect_local_runtimes verify_local_runtime; do
  [[ "$(grep -Fco "Name: \"$tool\"" internal/mcpserver/server.go)" -eq 1 ]] || {
    echo "tool metadata missing or duplicated: $tool" >&2
    exit 1
  }
done
