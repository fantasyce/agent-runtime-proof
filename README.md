# Agent Runtime Proof

Agent Runtime Proof is an independent Across ecosystem project for verifying
which local runtime an Agent actually launched and whether the observable
runtime matches a declared expectation.

Current development includes the Phase 1 read-only CLI core and the Phase 2
local `stdio` MCP surface. The same application layer drives both interfaces;
MCP does not maintain a second verdict model or open a network listener.

The v1 boundary is intentionally narrow:

- local processes and local files only;
- CLI, local `stdio` MCP, and an optional launch Witness;
- macOS, Windows, and Linux;
- read-only observation of Agent and host state;
- no dependency on Across Agents Assistant, Across Context, Across
  Orchestrator, or Across Autopilot.

Across Agents Assistant may be used later as one generic MCP compatibility host,
but Agent Runtime Proof is not a fourth first-party managed Across plugin.

See [the architecture, development, and acceptance design](docs/architecture-development-acceptance.md).

Phase 0 contains the versioned expectation, Proof, and cross-platform fixture
schemas; decision, privacy, and threat registries; canonical digest vectors; and
an executable Go verification harness. Run the complete current gate with:

```bash
bash scripts/check.sh
```

The [Phase 0 acceptance record](docs/phase0-acceptance.md),
[Phase 1A macOS record](docs/phase1-macos-acceptance.md), and
[Windows core record](docs/phase1-windows-acceptance.md) state the platform
core evidence. The [Phase 2 acceptance record](docs/phase2-acceptance.md)
captures the stdio MCP, native platform, CI, and real Codex-host results.

## CLI

Build an installed-style binary without source paths:

```bash
go build -trimpath -o agent-runtime-proof ./cmd/agent-runtime-proof
```

Inspect one process or a bounded current-user inventory:

```bash
agent-runtime-proof inspect --pid 1234 --format json
agent-runtime-proof inspect --all --limit 100
```

Verify a native runtime against an explicit expectation:

```bash
agent-runtime-proof verify --expectation expectation.json --pid 1234 --format json
agent-runtime-proof verify --expectation expectation.json --pid 1234 \
  --known-prior-digest 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

Check local read-only capabilities:

```bash
agent-runtime-proof doctor
```

Exit codes are `0` for successful inspection/doctor or `MATCHED`, `2` for a
determinate negative verdict, `3` for `UNKNOWN`, `64` for invalid input, and
`70` for an internal failure. JSON mode writes exactly one JSON value to
standard output; sanitized diagnostics use standard error.

## Local stdio MCP

An MCP host can start the installed binary with:

```json
{"command":"agent-runtime-proof","args":["mcp"]}
```

The server exposes `list_local_runtime_candidates`, `inspect_local_runtimes`,
and `verify_local_runtime`. All three are read-only and closed-world. See the
[generic host configuration guide](docs/host-configuration.md) and the thin
wrapper in `plugin/agent-runtime-proof`.

The current phases deliberately do not include Witness, host Profiles, a
daemon, network listeners, persistence, repair actions, or Agent configuration writes.
Interpreter-script and declared-tree expectations remain `UNKNOWN` until the
launch argument binding required to identify the active entrypoint can be
observed safely; an artifact digest alone is never reported as a runtime match.
