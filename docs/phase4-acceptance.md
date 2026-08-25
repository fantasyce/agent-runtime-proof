# Phase 4 Host Profile and Real Host Matrix Acceptance

- Date: 2026-08-26
- Final code candidate reviewed: `75a121a41d07`
- Branch: `codex/phase4-host-matrix`
- Decision: **PASS**
- Phase 5 release work: **not started**

Phase 4 is complete for the owner-selected environments: DeepSeek Harness on
macOS, Codex on Windows, native Linux amd64 on the Fantasyce VPS, and the AAA
generic MCP integration. Cursor, VS Code/Copilot, and OpenCode were explicitly
deferred because they are not installed; they were not installed merely to
expand the acceptance boundary.

## Delivered implementation

- Seven embedded, versioned, data-only Host Profiles for AAA, Codex, Claude
  Code, Cursor, OpenCode, DeepSeek Harness, and VS Code/GitHub Copilot.
- Bounded JSON, JSONC, TOML, and Cordis YAML parsing that treats host files as
  data, never executes discovered commands, rejects traversal/symlink access,
  and retains only hashes and safe identifiers.
- Deterministic host binding discovery, ambiguity detection, process matching,
  CLI `--host`/`--binding` flows, and equivalent operations through the three
  read-only MCP tools.
- Installed-style generic/Profile runners, real-host runners, platform matrix
  evidence, and bounded performance evidence.

## Passed platform and host gates

### macOS 26.6.2 arm64

- The installed-style generic/Profile runner returned one `MATCHED` and one
  expected `UNKNOWN`, exposed exactly three MCP tools, verified Profile
  attribution and Proof self-verification, and left no marked residue.
- Codex 0.149.1, Claude Code 2.1.235, and DeepSeek Harness 0.1.1-rc.2 each made a
  real `verify_local_runtime` stdio MCP call and returned a schema-valid
  `MATCHED` Proof.
- DeepSeek Harness used its existing authenticated installation read-only; no
  user setting or credential was changed.

### Windows 11 amd64

- The installed generic/Profile runner and complete vendored Go package suite
  passed under the checksum-verified Go 1.27.0 toolchain, including SDK, MCP,
  EOF escalation, and Job Object descendant cleanup.
- The real installed Codex CLI `0.149.0-alpha.4.3` listed the task MCP server,
  called `verify_local_runtime`, and returned `MATCHED` for binding
  `codex.agent-runtime-proof` with Proof ID
  `sha256:1750e2cc0e76def105fd200433c8f46c9c4b59bb62fdf2fe053343f0de0436ac`.
- The Codex task used the existing loopback proxy through task-scoped
  `HTTP_PROXY`, `HTTPS_PROXY`, and `ALL_PROXY`. The values were restored after
  the process. The Windows system proxy remained enabled at `127.0.0.1:7897`.
- All marked Windows transfer and diagnostic roots were removed. No ARP test
  process remained.

### Linux

- The Linux arm64 installed generic/Profile runner passed in the local
  build-support container.
- The native Linux amd64 runner passed on the Ubuntu 24.04 Fantasyce VPS. It
  used only a marked `/tmp` task directory; Xray, Nginx, SSH, and the listener
  baseline were unchanged, and all task residue was removed.

### AAA generic integration

The integration defect was in AAA's common host-to-local-agent bridge, not in
ARP or the three managed plugins. AAA commits `a7a95a8`, `c65bcd1`, and
`081d290` added the private-socket provider, fail-closed read-only filtering,
same-owner MCP lifecycle handling, and a task-scoped stdio proxy for local
Codex.

The rebuilt formal `/Applications/Across Agents Assistant.app` 0.14.3 exposed
exactly the three ARP tools as low-risk, read-only, and approval-free. Its
installed Codex adapter returned `MATCHED`
(`sha256:756b0e993b551d2b0dfc4d1cafb67077274c32e0e220be582d52481d39ef3b44`)
and the expected negative `UNKNOWN`
(`sha256:4a91f259aa5e7d030ef1ae488953864ef7a4f131a2cbbd7aad32ae00bb3511a6`).
Context 0.11.1, Orchestrator 0.10.10, and Autopilot 0.5.3 remained installed,
available, probed, and integrity-valid.

## Windows blocker resolution

The earlier hang was a transport boundary, not an ARP MCP deadlock. HTTPS
requests reached ChatGPT, but the Codex WebSocket repeatedly timed out when the
process relied only on the Windows system proxy. The same MCP-free request
completed when the existing loopback proxy was supplied as task-scoped
environment variables. The Fantasyce path was therefore functioning and did
not require a VPS change.

The rerun also exposed three acceptance-harness issues: Windows paths must use
TOML literal strings, the strict no-symlink expectation must use a
manifest-relative path across the Windows Temp reparse boundary, and the
structured MCP event must be treated as the evidence rather than a model's
textual restatement. These harness defects were fixed and the final installed
Codex run passed.

## Security, evidence, and scope

- Profile input remains non-executable and all binding ambiguity, secret
  projection, traversal, reparse, identity, and cancellation cases remain
  fail-closed or explicit `UNKNOWN` paths.
- Evidence files store bounded host/version/platform identifiers, verdicts,
  Proof IDs, and cleanup state—never prompts, transcripts, account identifiers,
  authentication material, or local source paths.
- No firewall, system proxy, DNS, default route, VPN, registry, service, power,
  Agent credential, or VPS setting was changed to make acceptance pass.
- Cursor, VS Code/Copilot, and OpenCode remain fixture-covered and
  owner-deferred. They are not missing mandatory rows under the frozen matrix.

## Decision boundary

Phase 4 is complete on the feature branch and is ready for an explicit
integration decision. This PASS does not claim that Phase 5 packaging, public
release, tags, GitHub publication, or v1 distribution has occurred.
