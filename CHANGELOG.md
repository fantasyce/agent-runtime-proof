# Changelog

All notable changes to Agent Runtime Proof are documented here. The project
uses Semantic Versioning.

## [1.0.1] - 2026-08-26

- Build the macOS arm64 release archive on a native macOS runner with cgo so
  the published binary includes the required Darwin process observer.
- Reject any future Darwin release asset that lacks the native observer and
  run the installed generic/Profile acceptance against the packaged binary.

## [1.0.0] - 2026-08-26

- Add read-only local runtime inspection and expectation-bound Proofs.
- Add the local stdio MCP server with three closed-world tools.
- Add the byte-transparent launch Witness and embeddable Witness SDK.
- Add data-only Host Profiles and real-host acceptance for Codex, Claude Code,
  DeepSeek Harness, AAA, and generic MCP fixtures.
- Add native acceptance for macOS arm64, Windows 11 amd64, and Linux amd64.
- Add reproducible archives, checksums, CycloneDX SBOMs, and GitHub provenance.

[1.0.1]: https://github.com/fantasyce/agent-runtime-proof/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/fantasyce/agent-runtime-proof/releases/tag/v1.0.0
