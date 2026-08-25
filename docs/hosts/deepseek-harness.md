# DeepSeek Harness

Host ID: `deepseek-harness`

Use a task-owned DSH home/profile and a task-owned `cordis.patch.yml`:

```yaml
- insert:
    - id: mcp-agent-runtime-proof
      name: '@deepseek-ai/dsh-mcp-client'
      config:
        serverName: agent-runtime-proof
        transport: stdio
        command: /absolute/path/to/agent-runtime-proof
        args: [mcp]
```

Pin `@deepseek-ai/dsh-mcp-client` to the DSH host version, inspect the composed
tree with `dsh --profile web --dump-config`, then start DSH with only that
task-owned patch and call `verify_local_runtime`. Stop the host and remove the
task home afterward. ARP parses the patch as inert data and rejects executable
YAML tags or interpolation.

Source: [DeepSeek Harness architecture](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/architecture.md) and [official MCP client examples](https://github.com/deepseek-ai/deepseek-harness/blob/master/examples/mcp-memory/README.md).
