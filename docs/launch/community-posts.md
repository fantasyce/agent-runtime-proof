# Community launch copy

Replace `RELEASE_URL` only after the public v1.1.0 release is independently verified.

## Show HN

**Title:** Show HN: Agent Runtime Proof – verify the live Agent, not just the file on disk

I built Agent Runtime Proof (ARP) for a small but consequential gap: replacing an Agent or MCP executable does not replace the process already running.

ARP is a local, read-only CLI and stdio MCP server that compares live process evidence with an explicit approved-artifact expectation. It reports MATCHED, STALE, LEAKED, CONFLICT, NOT_RUNNING, or UNKNOWN—and keeps uncertainty when the OS cannot prove loaded bytes.

The repository includes a reproducible demo that starts a fixture process, replaces its on-disk executable, and correctly returns `UNKNOWN / POSSIBLE_STALE_AFTER_REPLACEMENT` instead of overclaiming STALE.

No daemon, network listener, telemetry, configuration edits, raw argv, environment values, or transcripts. Native assets: macOS arm64, Linux amd64, Windows amd64; Apache-2.0.

Demo: https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md
Release: RELEASE_URL
Source: https://github.com/fantasyce/agent-runtime-proof

I would value concrete host/runtime cases—especially interpreter processes and launchers where loaded-code identity is difficult to observe safely.

## OpenAI Developer Forum

Agent Runtime Proof (ARP) is a local, read-only MCP server for checking whether the Agent process doing the work matches an approved artifact expectation. It addresses the stale-process gap that remains after an executable is replaced on disk.

ARP exposes three narrowly described read-only tools: bounded candidate discovery, explicit runtime inspection, and expectation-based verification. It does not edit host configuration, open a network listener, upload process data, or return raw command lines and environment values.

The reproducible demo deliberately preserves uncertainty: replacement timing produces `POSSIBLE_STALE_AFTER_REPLACEMENT`, not a false claim that the old loaded bytes were directly observed.

Quickstart: https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/quickstart.md
Demo: https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md
Release: RELEASE_URL

Feedback from developers integrating local stdio MCP servers is welcome, particularly around safe runtime discovery boundaries.

## MCP community

Agent Runtime Proof v1.1.0 packages a local stdio MCP server that verifies live Agent/MCP runtime identity against an explicit artifact expectation.

Why: new bytes on disk do not prove the old process stopped. ARP exposes exactly three read-only tools, returns conservative evidence, and does not modify client configuration or expose raw argv/env/transcripts.

The release includes a cross-platform MCPB, Registry metadata, checksums, SBOMs, and attestations. Reproduce the stale-runtime condition here: https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md

Release: RELEASE_URL
Source: https://github.com/fantasyce/agent-runtime-proof

## Reddit

**Title:** Open-source tool to verify the live AI Agent/MCP process—not only its executable on disk

I released Agent Runtime Proof (ARP), an Apache-2.0 local runtime identity verifier for Agent and MCP processes.

The failure case: an executable is upgraded, its checksum is correct, but the already-running process is still old. ARP compares live process evidence with an explicit approved-artifact expectation and reports uncertainty rather than assuming disk bytes equal loaded code.

It is read-only and local: no daemon, telemetry, network listener, repair action, raw argv/env, or transcripts. There is a CLI, three-tool stdio MCP server, optional launch Witness, and builds for macOS arm64, Linux amd64, and Windows amd64.

The demo is intentionally conservative: https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md

Release: RELEASE_URL

I am looking for sanitized host/runtime cases, not stars: which local Agent launch shapes are hardest to verify on your platform?

## X

New bytes on disk do not prove yesterday's Agent process stopped.

Agent Runtime Proof v1.1.0 locally verifies a live Agent/MCP runtime against the artifact you approved. Read-only CLI + stdio MCP, conservative UNKNOWN results, no telemetry or config edits.

Demo: https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md
RELEASE_URL

## LinkedIn

Replacing an AI Agent or MCP executable does not replace the process already doing the work.

Agent Runtime Proof (ARP) closes that narrow evidence gap. It is a local, read-only runtime identity verifier that compares a live process with an explicit approved-artifact expectation—and preserves `UNKNOWN` when the operating system cannot prove more.

The v1.1.0 release includes a CLI, three-tool stdio MCP server, optional launch Witness, cross-platform MCPB, checksums, SBOMs, and build attestations. It does not upload process data, edit Agent configuration, or expose raw arguments and environment values.

Reproducible demo: https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md
Release: RELEASE_URL

We welcome concrete, sanitized runtime cases from Agent host and MCP client maintainers.

## 中文

磁盘上的 Agent 或 MCP 可执行文件已经升级，不代表昨天启动的进程已经退出。

Agent Runtime Proof（ARP）是一款本地、只读的运行时身份验证工具：它把正在运行的进程与明确批准的构件预期进行比对；证据不足时保留 `UNKNOWN`，不会把磁盘上的新文件误当成已加载的新代码。

ARP 提供 CLI、本地 stdio MCP 服务和可选启动 Witness；不上传进程数据，不读取或返回原始参数、环境变量及对话内容，也不会修改 Agent 配置。

可复现实验：https://github.com/fantasyce/agent-runtime-proof/blob/main/docs/demo.md
正式版本：RELEASE_URL
源码：https://github.com/fantasyce/agent-runtime-proof
