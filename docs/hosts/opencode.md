# OpenCode

Host ID: `opencode`

Point `OPENCODE_CONFIG` at a task-owned `opencode.json` containing:

```json
{"mcp":{"agent-runtime-proof":{"type":"local","command":["/absolute/path/to/agent-runtime-proof","mcp"],"enabled":true}}}
```

Run `opencode mcp list`, then use a bounded read-only run to call
`verify_local_runtime` with `binding_id = "opencode.agent-runtime-proof"`.
Remove the task configuration and home afterward. ARP does not execute OpenCode
plugins or modify any OpenCode configuration.

Source: [OpenCode MCP servers](https://opencode.ai/docs/mcp-servers/) and [OpenCode configuration](https://opencode.ai/docs/config/).
