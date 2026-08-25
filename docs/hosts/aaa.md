# Across Agents Assistant

Host ID: `aaa`

AAA consumes Agent Runtime Proof through its existing generic stdio MCP manager;
there is no AAA-specific transport, API, page, or runtime branch. In a
task-owned AAA acceptance home, register this direct command:

```json
{"mcpServers":{"agent-runtime-proof":{"command":"/absolute/path/to/agent-runtime-proof","args":["mcp"]}}}
```

Use AAA's generic MCP server view to confirm the three tools, then call
`verify_local_runtime` once for a matching expectation and once for a negative
expectation. Remove the registration and the task-owned AAA home afterward.
ARP only reads the supplied state and never writes AAA configuration.

Source: the generic MCP contract in `docs/host-configuration.md`.
