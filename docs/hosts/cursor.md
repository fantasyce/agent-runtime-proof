# Cursor

Host ID: `cursor`

Create `.cursor/mcp.json` inside a task-owned workspace:

```json
{"mcpServers":{"agent-runtime-proof":{"command":"/absolute/path/to/agent-runtime-proof","args":["mcp"]}}}
```

From that workspace, run `cursor-agent mcp list-tools agent-runtime-proof` and
then a non-interactive, read-only prompt that calls `verify_local_runtime` with
`binding_id = "cursor.agent-runtime-proof"`. Remove the task workspace and
task-owned Cursor home after the run; never edit `~/.cursor/mcp.json`.

Source: [Cursor MCP configuration](https://docs.cursor.com/context/model-context-protocol) and [Cursor CLI parameters](https://docs.cursor.com/en/cli/reference/parameters).
