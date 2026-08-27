# The file is new. The Agent process may not be.

You approve a new Agent or MCP server build, replace the executable, and check its checksum. The bytes on disk are correct. But the process doing the work may still be yesterday's process.

That gap is what Agent Runtime Proof (ARP) is built to inspect.

ARP is a local, read-only runtime identity verifier for AI agents and MCP servers. You give it an explicit artifact expectation and a process identity; it gathers platform evidence and reports whether the live runtime is matched, stale, leaked, conflicting, not running, or still unknown. It never converts incomplete evidence into success.

## Reproduce the problem

From a clean source checkout:

```bash
bash scripts/demo_stale_runtime.sh
```

The demonstration starts a harmless fixture runtime, replaces the executable on disk, and asks whether the still-running process has been proven to be the replacement. The expected result is `UNKNOWN / POSSIBLE_STALE_AFTER_REPLACEMENT`, because replacement timing is evidence of risk—not direct proof of the old loaded bytes.

This conservative result is the point. Runtime security tools should make uncertainty visible, not manufacture a dramatic verdict.

## What ARP adds

- A standalone CLI for explicit or bounded local process inspection.
- Three read-only local MCP tools for discovery, inspection, and verification.
- An optional byte-transparent launch Witness and embeddable Go SDK.
- Native release assets for macOS arm64, Linux amd64, and Windows amd64.
- Checksums, CycloneDX SBOMs, GitHub artifact attestations, and an MCPB bundle.

ARP does not replace signatures, provenance, SBOMs, sandboxing, or host policy. Those controls establish how an artifact was produced and transported. ARP asks a different question: is the live local process bound to the artifact expectation you approved?

## The boundary

ARP has no network listener, telemetry, repair action, process termination, or configuration writer. MCP responses exclude raw arguments, environments, command lines, file contents, credentials, and transcripts. Permission denial and incomplete observation stay explicit as `UNKNOWN`.

Start with the [five-minute quickstart](https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/quickstart.md), run the [reproducible demo](https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md), or inspect the [source and acceptance evidence](https://github.com/fantasyce/agent-runtime-proof).

We are looking for maintainers of local Agent hosts and MCP clients who can contribute a sanitized real-world runtime case. The useful feedback is concrete: host, platform, launch shape, expected evidence boundary, and any observation ARP could not make safely.
