# Security Policy

## Supported Versions

Security fixes are provided for the latest v1 release. Pre-release development
commits and older minor lines are not supported release channels.

## Private Reporting

Please use GitHub private vulnerability reporting for
`fantasyce/agent-runtime-proof`. Do not open a public issue containing
credentials, private paths, process arguments, environment values,
transcripts, or unredacted Proof material.

Include the affected version and platform, the smallest safe reproduction, the
expected security boundary, and whether the behavior requires elevated access.
The project coordinates fixes through a new patch release and never rewrites a
published tag.

## Product Boundary

Agent Runtime Proof is read-only and local. It does not expose a network
listener, repair installations, terminate observed processes, or upload
telemetry. Permission denial and incomplete observation remain explicit
`UNKNOWN` results, never successful verification.
