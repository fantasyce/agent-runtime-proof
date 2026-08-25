# Phase 4 Blocker Register

Status: Phase 4 is **NO-GO**. These entries are mandatory acceptance gaps, not
skips. Product work and all independently runnable checks continue, but Phase 4
and v1 release remain incomplete until every blocking row is closed.

| ID | Boundary | Classification | Evidence | Required closure |
| --- | --- | --- | --- | --- |
| `P4-EXT-WINDOWS-CODEX` | Codex on Windows 11 amd64 | Unresolved host/provider/network boundary | The verified native Codex executable listed the task-scoped MCP server and spawned the ARP MCP child, but emitted no host JSON events before the bounded timeout. The next MCP-free direct-call diagnostic began, then the whole Windows machine stopped responding to ping and SSH before its task-owned artifacts could be retrieved. The copied Codex database ends before this attempt; it contains historical connection and model-metadata timeout classes but cannot establish the current root cause. | When the Windows host is reachable, first retrieve or terminate only the marked diagnostic process tree, then run one low-load MCP-free Codex call and one read-only proxy reachability probe. Only if the direct call passes, rerun the ARP host script to a schema-valid Proof. |

## Closed blockers

| ID | Closed by | Closure evidence |
| --- | --- | --- |
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
- The failed host attempts did not require changes to firewall, proxy, DNS,
  route, registry, service, power, or Agent credential configuration.

Cursor, VS Code/Copilot, and OpenCode are owner-deferred because they are not
installed in the available environments. They are not active Phase 4 blockers
and were not installed as part of acceptance. A Windows DeepSeek Harness
installation is unnecessary while the real macOS DeepSeek Harness row covers
that host family.
