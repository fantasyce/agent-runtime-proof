# VS Code and GitHub Copilot

Host ID: `vscode-copilot`

For portable Agent Host testing, create a task-owned workspace `.mcp.json`:

```json
{"servers":{"agent-runtime-proof":{"type":"stdio","command":"/absolute/path/to/agent-runtime-proof","args":["mcp"]}}}
```

Open that workspace with a task-owned VS Code user-data directory, use **MCP:
List Servers** to confirm the server and its three tools, then ask Agent mode to
call `verify_local_runtime` with `binding_id =
"vscode-copilot.agent-runtime-proof"`. Remove the workspace and portable
profile afterward; never modify the user's VS Code or Copilot profile.

Source: [VS Code MCP servers](https://code.visualstudio.com/docs/agent-customization/mcp-servers) and [MCP configuration reference](https://code.visualstudio.com/docs/agents/reference/mcp-configuration).
