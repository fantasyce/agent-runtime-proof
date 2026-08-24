# Agent Runtime Proof

Agent Runtime Proof is an independent Across ecosystem project for verifying
which local runtime an Agent actually launched and whether the observable
runtime matches a declared expectation.

Current status: Phase 0 contract baseline implemented and locally accepted on
macOS arm64. Phase 1 runtime, CLI, MCP, process inspection, and Witness code has
not started.

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

The [Phase 0 acceptance record](docs/phase0-acceptance.md) states exactly what
passed and what remains intentionally deferred.
