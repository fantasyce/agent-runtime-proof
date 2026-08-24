# Agent Runtime Proof

Agent Runtime Proof is an independent Across ecosystem project for verifying
which local runtime an Agent actually launched and whether the observable
runtime matches a declared expectation.

Current status: the Phase 0 contract baseline is complete. Phase 1A implements
the read-only `inspect`, `verify`, and `doctor` CLI core with native macOS,
Windows, and Linux process observation, bounded artifact hashing, and
privacy-safe Proof output. Phase 1A has passed its macOS-primary local
acceptance, and the Windows core candidate has passed native Windows 11 build,
test, safe-junction, installed-candidate, permission-denial, and live CLI
verification. Full Phase 1 and v1 remain incomplete because cross-platform
core semantics, native Linux amd64, remote CI, MCP, Witness, release, and
real-host gates are tracked separately.

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
[Windows core record](docs/phase1-windows-acceptance.md) state exactly what
passed and what remains intentionally deferred.

## Phase 1A CLI

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
```

Check local read-only capabilities:

```bash
agent-runtime-proof doctor
```

Exit codes are `0` for successful inspection/doctor or `MATCHED`, `2` for a
determinate negative verdict, `3` for `UNKNOWN`, `64` for invalid input, and
`70` for an internal failure. JSON mode writes exactly one JSON value to
standard output; sanitized diagnostics use standard error.

Phase 1A deliberately does not include MCP, Witness, host Profiles, a daemon,
network listeners, persistence, repair actions, or Agent configuration writes.
Interpreter-script and declared-tree expectations remain `UNKNOWN` until the
launch argument binding required to identify the active entrypoint can be
observed safely; an artifact digest alone is never reported as a runtime match.
