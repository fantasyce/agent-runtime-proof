# Agent Runtime Proof

**Prove the agent runtime you launched is the artifact you approved.**

Agent Runtime Proof (ARP) is a local, read-only runtime identity verifier for
AI agents and MCP servers. It detects stale, replaced, mismatched, or
unverifiable runtimes without uploading code, secrets, process arguments, or
process data.

The failure ARP is built for is simple: you replace an Agent or MCP executable,
but yesterday's process keeps doing the work. The file on disk is new; the
loaded runtime is not. ARP binds live process identity to an explicit artifact
expectation and reports the uncertainty instead of treating the file on disk as
proof of what is running.

See the [reproducible stale-runtime demonstration](docs/demo.md), or start with
the [five-minute quickstart](docs/quickstart.md).

## Quickstart

The current release is
[v1.0.1](https://github.com/fantasyce/agent-runtime-proof/releases/tag/v1.0.1)
for macOS arm64, Linux amd64, and Windows amd64. Download the archive and
`SHA256SUMS` from that release, verify the bytes, then install the single binary
in a user-owned directory. Full commands for each platform are in
[docs/quickstart.md](docs/quickstart.md); lifecycle and rollback details are in
[docs/install.md](docs/install.md).

After installation:

```bash
agent-runtime-proof --version
agent-runtime-proof doctor --format json
agent-runtime-proof inspect --all --limit 20
```

Verify a process against an explicit expectation:

```bash
agent-runtime-proof verify \
  --expectation /absolute/path/to/expectation.json \
  --pid 1234 \
  --format json
```

ARP returns `MATCHED` only when the evidence supports that conclusion.
`UNKNOWN`, `STALE`, `LEAKED`, `CONFLICT`, and `NOT_RUNNING` are domain verdicts,
not transport errors.

## Local stdio MCP

ARP can run as a local MCP server:

```json
{
  "mcpServers": {
    "agent-runtime-proof": {
      "command": "/absolute/path/to/agent-runtime-proof",
      "args": ["mcp"]
    }
  }
}
```

It exposes exactly three read-only tools:

- `list_local_runtime_candidates`
- `inspect_local_runtimes`
- `verify_local_runtime`

Host profiles are available for Codex, Claude Code, Cursor, OpenCode, DeepSeek
Harness, VS Code/GitHub Copilot, and generic hosts. See
[the host configuration guide](docs/host-configuration.md). ARP does not edit
the host's configuration.

## What ARP proves

These controls answer different questions and work best together:

| Evidence | Question it answers |
| --- | --- |
| SBOM | What dependencies were declared in this build? |
| Signature or build attestation | Who produced these artifact bytes, and through which build? |
| Checksum | Did the downloaded bytes change? |
| Agent Runtime Proof | Is the live local process bound to the artifact expectation I approved? |

ARP does not replace signing, provenance, SBOMs, sandboxing, or host policy. It
closes the gap between an approved file and the process that is actually doing
the work.

## Privacy and safety boundary

ARP is intentionally narrow:

- local processes and local files only;
- CLI, local `stdio` MCP, and an optional launch Witness;
- read-only observation of Agent and host state;
- no daemon, network listener, repair action, or configuration write;
- no dependency on Across Agents Assistant, Across Context, Across
  Orchestrator, or Across Autopilot.

MCP responses omit raw argv, environment values, command lines, file contents,
credentials, and transcripts. Read the exact guarantees and limitations in
[docs/data-handling.md](docs/data-handling.md),
[docs/privacy-model.md](docs/privacy-model.md), and
[docs/threat-model.md](docs/threat-model.md).

## CLI and Witness

Inspect one process or a bounded current-user inventory:

```bash
agent-runtime-proof inspect --pid 1234 --format json
agent-runtime-proof inspect --all --limit 100
```

Run a local command through the byte-transparent launch Witness:

```bash
agent-runtime-proof witness --expectation expectation.json -- command arg
```

The Witness records process identity and a content-addressed launch receipt,
then proxies stdin/stdout without rewriting protocol bytes. Hosts that own
process creation can embed the same contract through `sdk/witness`.

Exit codes are `0` for inspection, doctor, or `MATCHED`; `2` for a determinate
negative verdict; `3` for `UNKNOWN`; `64` for invalid input; and `70` for an
internal failure. JSON mode writes one JSON value to stdout; sanitized
diagnostics use stderr.

## Supported platforms and limits

Release archives are built for macOS 14+ arm64, Linux amd64, and Windows 11
amd64. Passive inspection of interpreter and declared-tree runtimes remains
conservative when the active entrypoint cannot be observed. An on-disk digest
alone is never reported as a loaded-runtime match.

Remote attestation, a daemon, network listeners, repair actions, and Agent
configuration writes remain outside v1.

## Architecture and acceptance

The same contracts and application layer drive the CLI, MCP server, Witness,
and data-only Host Profiles. Technical design is documented in
[architecture-development-acceptance.md](docs/architecture-development-acceptance.md).

Acceptance records:

- [Phase 0 contracts](docs/phase0-acceptance.md)
- [Phase 1 macOS core](docs/phase1-macos-acceptance.md)
- [Phase 1 Windows core](docs/phase1-windows-acceptance.md)
- [Phase 2 MCP](docs/phase2-acceptance.md)
- [Phase 3 Witness](docs/phase3-acceptance.md)
- [Phase 4 host matrix](docs/phase4-acceptance.md)
- [Phase 5 open-source release](docs/phase5-acceptance.md)

Maintainers can run the complete source gate with:

```bash
bash scripts/check.sh
```

Release assets include CycloneDX SBOMs, SHA-256 checksums, and GitHub artifact
attestations. Publication state is determined by the public GitHub Release, not
by a local acceptance record.

## Community

Contributions and independent host/runtime cases are welcome. Read
[CONTRIBUTING.md](CONTRIBUTING.md), [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md),
and [SUPPORT.md](SUPPORT.md). Report vulnerabilities privately as described in
[SECURITY.md](SECURITY.md); do not put secrets or sensitive process data in a
public issue.

Apache-2.0 licensed. See [LICENSE](LICENSE).
