# Phase 1 Closure and Phase 2 stdio MCP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close every applicable Phase 1 gate with real platform evidence, then implement and fully accept the Phase 2 local stdio MCP surface without unresolved defects.

**Architecture:** Keep `internal/app` as the single business layer. Add a typed adapter around the stable official MCP Go SDK, expose it only through `agent-runtime-proof mcp`, and keep stdout protocol-only. The MCP tools return the same safe Proof projections and domain verdict semantics as the CLI. Platform acceptance uses task-owned roots and installed candidates.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk` v1.6.1, JSON Schema contracts, GitHub Actions, native macOS/Windows/Linux acceptance.

**Authoritative spec:** `docs/architecture-development-acceptance.md`, especially sections 13, 18 Phase 1/2, and 19.

**Global constraints:** Read-only process/runtime inspection; no Agent configuration changes; no HTTP transport; no stdout logs; no raw paths, argv, environment, credentials, or transcripts; no persistent test residue.

## Task 1: Close the reachable Phase 1 API and platform gaps

**Files:** `internal/cli/run.go`, `internal/cli/run_test.go`, `scripts/run_linux_acceptance.sh`, `docs/issues/phase1-deferred-gates.md`, new Phase 1 acceptance report.

- [x] Add a failing CLI test proving repeatable, validated `--known-prior-digest` values reach the existing evaluator input.
- [x] Implement the minimal CLI parsing and validation, then prove `STALE` through the installed macOS candidate.
- [x] Run the existing acceptance on a native Linux amd64 host using only a task-owned portable toolchain and clean all remote artifacts.
- [x] Reclassify argument binding and multi-process aggregation according to their authoritative later-phase ownership; do not claim unsupported evidence.
- [x] Run the complete local Phase 1 suite and record exact evidence.

## Task 2: Add the typed MCP server contract

**Files:** `go.mod`, `go.sum`, new `internal/mcpserver/server.go`, new `internal/mcpserver/server_test.go`.

- [x] Write failing in-memory tests for exactly three tools, read-only/closed-world annotations, argument validation, successful domain verdicts, and error mapping.
- [x] Add and lock the stable official MCP Go SDK v1.6.1.
- [x] Implement typed handlers that delegate to `internal/app` and return safe Proof projections without persistence.
- [x] Prove `UNKNOWN`/`STALE` remain successful tool results while malformed inputs and internal failures become tool/protocol errors.

## Task 3: Wire the protocol-only CLI mode

**Files:** `cmd/agent-runtime-proof/main.go`, `internal/cli/run.go`, CLI tests, new stdio integration tests.

- [x] Write failing subprocess tests for `mcp`, stdout purity, EOF exit, malformed input, cancellation, and concurrent calls.
- [x] Wire `agent-runtime-proof mcp` to the official stdio transport with diagnostics only on stderr.
- [x] Add current and previous supported protocol negotiation tests and official SDK client-fixture coverage.
- [x] Verify connection close leaves no child process or temporary-file residue.

## Task 4: Ship the thin plugin and generic host documentation

**Files:** new `plugin/.codex-plugin/plugin.json`, `plugin/mcp.json`, `plugin/skills/agent-runtime-proof/SKILL.md`, new `docs/host-configuration.md`, README updates.

- [x] Create a manifest/config/Skill wrapper that only invokes the PATH binary and contains no core logic or downloader.
- [x] Document generic local stdio configuration, explicit PID/expectation usage, security boundaries, and troubleshooting.
- [x] Validate plugin artifacts structurally and verify the configured command launches the installed candidate.

## Task 5: Complete Phase 2 real-platform and real-host acceptance

**Files:** new or extended acceptance scripts and `docs/phase2-acceptance.md`.

- [x] Build/install a task-owned macOS candidate and run the SDK fixture plus a real Codex-host tool discovery and verify call.
- [x] Run native Windows 11 amd64 build, tests, SDK fixture, installed-candidate MCP calls, EOF/concurrency/error cases, and cleanup.
- [x] Run native Linux amd64 build, tests, SDK fixture, installed-candidate MCP calls, and cleanup.
- [x] Confirm stdout zero pollution, safe projections, cancellation, previous-protocol negotiation, and zero residue on every required platform.

## Task 6: Automation, security, and final verification

**Files:** `.github/workflows/phase1.yml` or successor, `scripts/check.sh`, acceptance reports, issue register.

- [x] Extend the three-platform automated matrix to compile and test the MCP implementation.
- [x] Run formatting, vet, race, full tests, contract checks, secret/path scans, cross-compilation, and acceptance scripts from a clean state.
- [x] Execute the remote matrix in an independent private repository if needed for Actions evidence; do not publish a public release or choose a license implicitly.
- [x] Review the branch diff for scope, privacy, destructive behavior, and unsupported claims; fix and rerun until green.
- [x] Record exact passed evidence, explicit non-Phase-2 future gates, and cleanup results; commit coherent checkpoints.
