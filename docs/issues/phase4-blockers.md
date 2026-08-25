# Phase 4 Blocker Register

Status: Phase 4 has **no open blocker**. Phase 5 release remains separate and
has not started.

There are no rows in the open-blocker table.

## Closed blockers

| ID | Closed by | Closure evidence |
| --- | --- | --- |
| `P4-EXT-WINDOWS-CODEX` | Installed Windows Codex real-host rerun | HTTPS was reachable both directly and through the existing loopback proxy, while the old WebSocket attempt repeatedly timed out. A controlled MCP-free comparison showed that the installed Codex completed only when `HTTP_PROXY`, `HTTPS_PROXY`, and `ALL_PROXY` were supplied to that task process. With those task-scoped variables, the installed Codex made a real `verify_local_runtime` call and returned `MATCHED` (`sha256:1750e2cc0e76def105fd200433c8f46c9c4b59bb62fdf2fe053343f0de0436ac`) for `codex.agent-runtime-proof`. The system proxy remained enabled at `127.0.0.1:7897`; no persistent proxy, firewall, DNS, route, registry, service, power, credential, or VPS setting changed. |
| `P4-HOST-AAA-TOOL-BRIDGE` | AAA commits `a7a95a8`, `c65bcd1`, `081d290` | Formal AAA 0.14.3 exposed exactly three ARP tools as low-risk/read-only/approval-free. Installed Codex called through AAA and returned `MATCHED` (`sha256:756b0e993b551d2b0dfc4d1cafb67077274c32e0e220be582d52481d39ef3b44`) and negative `UNKNOWN` (`sha256:4a91f259aa5e7d030ef1ae488953864ef7a4f131a2cbbd7aad32ae00bb3511a6`). Context 0.11.1, Orchestrator 0.10.10, and Autopilot 0.5.3 all remained installed, available, probed, and integrity-valid. |

## Passed evidence retained

- Codex 0.149.1 and Claude Code 2.1.235 each made a real macOS stdio MCP
  `verify_local_runtime` call against the installed Phase 4 candidate and
  returned a schema-valid `MATCHED` Proof.
- macOS arm64, Linux arm64 container, and native Windows amd64 generic/Profile
  harnesses passed. Native Windows also passed the complete vendored package
  suite under the verified Go 1.27.0 toolchain.
- DeepSeek Harness 0.1.1-rc.2 made a real macOS MCP call and returned a
  schema-valid `MATCHED` Proof without changing its user settings or
  credentials.
- The native Linux amd64 generic/Profile row passed on the Ubuntu 24.04
  Fantasyce VPS with service/listener baselines unchanged and task residue
  removed.
- The Windows rerun also found and corrected acceptance-harness defects: TOML
  basic strings misread Windows backslashes, a raw Temp absolute path crossed
  a reparse boundary under the no-symlink policy, and Windows PowerShell did
  not expose a reliable child exit-code value. The harness now uses literal
  TOML, a manifest-relative artifact root, and the structured MCP event result.
  These were test-harness defects, not ARP verdict-engine defects.
- No host attempt changed firewall, system proxy, DNS, route, registry,
  service, power, Agent credentials, or VPS configuration.

Cursor, VS Code/Copilot, and OpenCode are owner-deferred because they are not
installed in the available environments. They are not active Phase 4 blockers
and were not installed as part of acceptance. A Windows DeepSeek Harness
installation is unnecessary while the real macOS DeepSeek Harness row covers
that host family.
