# Launch FAQ

## Why is checking the file checksum not enough?

A checksum proves the bytes currently at a path. It does not prove that an earlier process stopped or that those bytes are what the live process loaded.

## Does ARP certify a runtime as secure?

No. ARP answers a narrower identity question. It complements signatures, attestations, SBOMs, sandboxing, permissions, and host policy.

## Why can the demo return `UNKNOWN` instead of `STALE`?

Starting before replacement is strong evidence of possible staleness, but not always direct evidence of the old loaded bytes. ARP uses `STALE` only when the evidence contract supports that determinate conclusion.

## Does ARP upload process information?

No. ARP is local and has no network listener or telemetry path. MCP responses omit raw arguments, environments, command lines, file contents, credentials, and transcripts.

## Can ARP repair or restart an Agent?

No. It is read-only. The host or operator decides what to do with the evidence.

## Which platforms are supported?

Published native assets target macOS 14+ arm64, Linux amd64, and Windows 11 amd64. Observation depth varies with platform permissions and runtime shape.

## Is ARP tied to Across Agents Assistant?

No. ARP is an independent Apache-2.0 project with a CLI, stdio MCP server, optional Witness, and Go SDK.

## How should I report a real runtime case?

Use the runtime-case issue form and sanitize private paths and identifiers. Never include credentials, environment values, raw command lines, transcripts, or proprietary code.
