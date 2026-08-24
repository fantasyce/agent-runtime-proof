---
name: agent-runtime-proof
description: Use when a local Agent runtime process must be inspected or verified against an explicit runtime expectation, especially after an install or upgrade.
---

# Agent Runtime Proof

Use the `agent-runtime-proof` MCP tools for read-only local process evidence.

- Start with `list_local_runtime_candidates` only when the PID is unknown.
- Use `inspect_local_runtimes` for observation without an expectation.
- Use `verify_local_runtime` with an explicit PID and either a trusted local expectation path or an inline expectation whose roots are absolute or home-relative.
- Treat `UNKNOWN`, `STALE`, `LEAKED`, `CONFLICT`, and `NOT_RUNNING` as domain verdicts. Report their reason codes and limitations; do not recast them as transport failures.
- Never claim loaded-memory proof from an artifact digest. Never modify configuration, terminate processes, repair installations, or expose raw paths, argv, environment values, credentials, or transcripts.
