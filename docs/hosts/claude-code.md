# Claude Code

Host ID: `claude-code`

Create a task-owned `.mcp.json`:

```json
{"mcpServers":{"agent-runtime-proof":{"command":"/absolute/path/to/agent-runtime-proof","args":["mcp"]}}}
```

Run Claude Code with `--mcp-config /absolute/task/path/.mcp.json`, `-p`, and
`--allowedTools mcp__agent-runtime-proof__verify_local_runtime`. Ask it for one
read-only binding verification. Remove the task file afterward. Do not replace
or merge the user's Claude configuration, and do not copy authentication data.

Source: [Anthropic Claude Code CLI reference](https://docs.anthropic.com/en/docs/claude-code/cli-usage).
