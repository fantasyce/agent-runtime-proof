# Codex

Host ID: `codex`

Put this in a task-owned `CODEX_HOME/config.toml`; do not edit the user's normal
Codex home:

```toml
[mcp_servers.agent-runtime-proof]
command = "/absolute/path/to/agent-runtime-proof"
args = ["mcp"]
```

With that `CODEX_HOME`, run `codex mcp list`, then ask Codex to call
`verify_local_runtime` with `binding_id = "codex.agent-runtime-proof"` and an
explicit expectation. Delete only the task-owned Codex home afterward. ARP
never writes Codex configuration or reads authentication material into Proofs.

Source: `codex mcp --help` and the installed Codex CLI configuration contract.
