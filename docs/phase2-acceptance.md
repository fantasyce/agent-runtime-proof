# Phase 2 Local stdio MCP Acceptance

- Date: 2026-08-25
- Accepted implementation commit: `345f8205f1dda2ec52d3e7e4bf9b3fe349d8df27`
- Branch: `codex/phase1-phase2-completion`
- Decision: **PASS**
- Unresolved Phase 2 defects: **none**

## Accepted Surface

The installed binary now supports `agent-runtime-proof mcp` through the stable
official MCP Go SDK v1.6.1. It exposes exactly three typed, read-only,
closed-world tools:

- `list_local_runtime_candidates`;
- `inspect_local_runtimes`;
- `verify_local_runtime`.

All handlers delegate to `internal/app`. `UNKNOWN`, `STALE`, and other domain
verdicts remain successful structured tool results. Invalid inputs and internal
failures are sanitized. Ordinary MCP calls do not persist state, and the server
opens no network listener.

The verify tool accepts either a local expectation path or an inline
expectation. Inline roots must be absolute or home-relative, preventing an MCP
host working directory from silently retargeting artifact access. Directly
known prior digests are lowercase 64-hex values and can produce `STALE` without
making a timestamp-only claim.

## Protocol and Automated Evidence

The in-memory and command-transport tests use the official Go SDK client. They
cover exact tool discovery and annotations, typed schemas and structured
output, safe error mapping, domain-result semantics, concurrency, request
cancellation, stdin EOF shutdown, current SDK negotiation, previous stable
protocol `2025-06-18`, malformed two-megabyte input, stdout protocol purity,
and subprocess cleanup.

The complete local gate passed on macOS arm64:

```bash
bash scripts/check.sh
bash scripts/run_phase1_acceptance.sh
```

This includes formatting, vet, module verification, trimmed builds, the full
race-enabled package suite, contract fixtures, installed CLI `MATCHED`, direct
`STALE`, negative verdicts, privacy scans, and MCP command-transport tests.

The thin plugin under `plugin/agent-runtime-proof` passed both the plugin
manifest validator and Skill validator. It invokes only the PATH command
`agent-runtime-proof mcp`; it contains no core implementation or downloader.

## Real Platform Evidence

### Windows 11 amd64

On `DESKTOP-6OTKL7F`, Go 1.27.0 windows/amd64 from the official task-owned
portable archive ran `go vet ./...` and `go test -count=1 ./...`. The installed
candidate acceptance then passed Phase 1 CLI and Phase 2 MCP checks:

- previous-protocol initialize and exactly three tools;
- installed-binary `inspect_local_runtimes` against a real helper PID;
- structured Proof output, EOF exit, and empty MCP stderr;
- CLI `MATCHED`, directly known `STALE`, permission-denial `UNKNOWN`, and ACL
  restoration;
- Proof ID `sha256:2d767f53dacb38522eec5cc7c8b082fa574f2e4f973174ea8dc57638acf807c0`;
- candidate asset SHA-256
  `e3e16f0fa03ca3ced4d01ffc6c3b883c348ba1f97535a374d08ea831a8230a67`;
- no remaining helper/MCP process or marked task directory.

No firewall, SSH, account, service, registry, PATH, Agent configuration, or
persistent system setting was changed.

### Linux amd64

A native Linux amd64 host used an official checksum-verified, task-owned Go
1.26.0 toolchain. The full package suite passed. An installed candidate then
passed real-process replacement, `MATCHED`, deleted executable, previous MCP
protocol, exact tool discovery, structured inspect, EOF, empty stderr, and
privacy checks. Source, module cache, toolchain, results, and processes were
removed; `LINUX_PHASE2_RESIDUE=NONE` was confirmed.

### macOS arm64 and real Agent host

The official SDK command fixture started the installed-style binary, listed and
called tools, and closed it through EOF. Separately, Codex CLI 0.149.1 was run
ephemerally with only a command-line MCP override. It first called
`agent_runtime_proof.list_local_runtime_candidates` with `limit=2` and received
two structured candidate summaries. A second isolated run called
`verify_local_runtime` against a task-owned real helper and expectation. It
returned `MATCHED`, proof level `ARTIFACT_OBSERVED`, reason `MATCH_CONFIRMED`,
and Proof ID
`sha256:2714532bd1e65d3e75f20a6a3ece61d341f10ad0ee120b12a81542e4589b66d7`.
No Codex configuration was written, and the helper, server, and task directory
were removed after the call.

## Remote Matrix

Private GitHub Actions run `32764337638` passed all four jobs from the accepted
implementation commit:

- macOS 14 / Go 1.26.x;
- macOS 14 / Go 1.27.x plus installed acceptance;
- Ubuntu 24.04 amd64 plus container real-process CLI/MCP acceptance;
- Windows 2025 amd64 plus native package and installed CLI/MCP acceptance.

The repository remains private. Public visibility, license approval, tags,
release assets, SBOM, provenance, Witness, Host Profiles, and the full named
Agent matrix are later-phase or release gates and are not represented as Phase
2 defects.

## Final Boundary

Phase 1 and Phase 2 are accepted. This does not claim v1 release readiness:
Phase 3 Witness, Phase 4 Host Profiles/integration, Phase 5 full named-host and
release gates remain intentionally unimplemented.
