# Showcase submission draft

**Name:** Agent Runtime Proof

**Tagline:** Prove the Agent runtime you launched is the artifact you approved.

**Category:** Developer tools / MCP / security

**Description:** Agent Runtime Proof is a local, read-only runtime identity verifier for AI agents and MCP servers. It detects stale, replaced, mismatched, or unverifiable runtimes by comparing live process evidence with an explicit artifact expectation. It exposes three narrow stdio MCP tools, a CLI, and an optional launch Witness without uploading code, secrets, raw process arguments, environment values, or transcripts.

**What is technically distinctive:** ARP separates artifact-on-disk evidence from live-process identity and retains `UNKNOWN` when a platform cannot prove loaded bytes. Its reproducible demo replaces a running fixture's executable and intentionally refuses to overclaim a determinate stale verdict from timing alone.

**Source:** https://github.com/fantasyce/agent-runtime-proof

**Demo:** https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md

**Website:** https://fantasyce.github.io/agent-runtime-proof/

**Release:** RELEASE_URL

Submission requires the account owner to review any platform terms, identity statement, and public-profile attribution before the final submit action.
