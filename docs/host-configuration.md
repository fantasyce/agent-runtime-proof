# Local stdio MCP Host Configuration

Agent Runtime Proof exposes one local, read-only MCP server. The host starts the
installed binary directly; there is no network listener, daemon, or remote
transport.

## Generic configuration

```json
{
  "mcpServers": {
    "agent-runtime-proof": {
      "command": "agent-runtime-proof",
      "args": ["mcp"]
    }
  }
}
```

Install the binary in a trusted PATH location or replace the command with the
absolute path to a trusted installed candidate. Do not point a production host
at a repository build directory.

The server publishes exactly three tools:

- `list_local_runtime_candidates` returns bounded, safe summaries without
  artifact hashing.
- `inspect_local_runtimes` observes an explicit PID or bounded current-user
  inventory and returns observation Proofs.
- `verify_local_runtime` checks an explicit PID against either a trusted local
  expectation file or an inline expectation. Inline filesystem roots must be
  absolute or home-relative so host working-directory changes cannot retarget
  them.

`UNKNOWN`, `STALE`, `LEAKED`, `CONFLICT`, and `NOT_RUNNING` are successful MCP
tool results with domain verdicts. Inspect `reason_codes` and `limitations`;
do not treat these verdicts as transport failures.

## Security boundary

The server reads local current-user process and declared artifact state. It
does not edit host configuration, terminate processes, install or update
software, open a network port, or persist ordinary MCP calls. MCP output never
offers the CLI's local-path display exception and omits raw argv, environment
values, command lines, file contents, credentials, and transcripts.

Host and binding selectors are reserved for later versioned Host Profiles. In
Phase 2, use an explicit PID. A non-empty unsupported selector is rejected
instead of being guessed.

## Lifecycle and troubleshooting

The host owns the subprocess lifecycle. Closing stdin or the MCP connection
causes the server to exit. Server stdout is protocol-only; sanitized startup or
protocol failures, if any, use stderr.

If the tools do not appear:

1. Run `agent-runtime-proof doctor` outside MCP to confirm the binary and local
   observer are available.
2. Confirm the host launches `agent-runtime-proof` with the single argument
   `mcp`.
3. Confirm the host did not replace PATH or the working user.
4. Treat permission denial as an evidence limitation, not a reason to elevate
   privileges or weaken host security.

The repository's thin Codex-compatible wrapper is under
`plugin/agent-runtime-proof`; it contains only a manifest, this stdio launch
configuration, and Agent guidance.
