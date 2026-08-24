# Agent Runtime Proof Project Rules

## Product Boundary

Agent Runtime Proof is an independent open-source Across ecosystem project. It
must build and run without Across Agents Assistant, Across Context, Across
Orchestrator, or Across Autopilot.

Do not place its implementation in any of those four repositories. Do not make
it a fourth first-party managed AAA plugin. AAA may consume it only through the
same generic local interface available to other Agent hosts.

## Version 1 Scope

- Local Agent runtimes only.
- CLI and local `stdio` MCP are the public interfaces.
- A launch Witness may proxy a local child process byte-for-byte.
- macOS and Windows require real-machine acceptance; Linux core, CLI, and MCP
  acceptance may run in local Docker.
- Runtime and host inspection is read-only. The tool must not kill processes,
  rewrite Agent configuration, delete host caches, upgrade plugins, or repair
  installations.
- No HTTP/SSE server, remote Agent support, cloud control plane, telemetry,
  multi-tenancy, or remote attestation in v1.

## Evidence Rules

- Keep verdict and proof strength separate.
- Never claim loaded-memory proof from an on-disk digest.
- Bind process evidence to PID plus creation time to prevent PID reuse.
- Treat permission denial, races, scan limits, and incomplete evidence as an
  explicit indeterminate result, not a pass.
- Do not persist raw environment values, complete command lines, credentials,
  transcripts, file contents, or unredacted home paths.
- Tests must use task-owned roots and leave no process or filesystem residue.

## Authoritative Design

`docs/architecture-development-acceptance.md` defines the architecture,
contracts, development phases, platform matrix, and GO/NO-GO release gates.
Scope or proof-semantics changes require updating that document and its tests.
