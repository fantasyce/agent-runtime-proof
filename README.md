# Agent Runtime Proof

Agent Runtime Proof is an independent Across ecosystem project for verifying
which local runtime an Agent actually launched and whether the observable
runtime matches a declared expectation.

Current development includes the Phase 1 read-only CLI core, the Phase 2 local
`stdio` MCP surface, and the Phase 3 launch Witness. The same contracts and
application layer drive every interface; MCP and Witness do not open a network
listener or maintain a second verdict model.

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
core evidence. The [Phase 2 record](docs/phase2-acceptance.md) captures stdio
MCP and real-host results. The [Phase 3 record](docs/phase3-acceptance.md)
captures the cross-platform Witness, launch receipts, lifecycle, SDK, and
installed-binary evidence.

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

## Launch Witness

Run a local command through the byte-transparent Witness:

```bash
agent-runtime-proof witness --expectation expectation.json -- command arg
```

The Witness binds the declared artifact and argument fingerprints before
launch, creates the child without a shell, records PID plus creation-time
identity, atomically stores a content-addressed launch receipt, and proxies
stdin/stdout without rewriting protocol bytes. Diagnostics and the receipt ID
use stderr only. Without an expectation it emits an explicitly
observation-only receipt and never upgrades that evidence to `MATCHED`.

Unix uses an owned process group and parent-death guardian; Linux also reaps
adopted descendants. Windows creates the child inside a kill-on-close Job
Object at process creation. EOF, cancellation, and termination are bounded and
escalate only against the process tree created by this Witness.

Hosts that already own process creation can embed the same contract through
`sdk/witness`: call `PrepareLaunch`, start the exact returned command and argv,
then call `Spawned` with the child PID.

Phase 4 host Profiles and the full named-host matrix, Phase 5 release assets,
remote attestation, a daemon, network listeners, repair actions, and Agent
configuration writes remain outside the completed Phase 1–3 surface. Passive
inspection of interpreter and declared-tree runtimes remains conservative when
the active entrypoint cannot be observed; an on-disk digest alone is never
reported as a loaded-runtime match.
