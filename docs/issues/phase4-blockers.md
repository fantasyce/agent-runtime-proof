# Phase 4 Blocker Register

Status: Phase 4 is **NO-GO**. These entries are mandatory acceptance gaps, not
skips. Product work and all independently runnable checks continue, but Phase 4
and v1 release remain incomplete until every blocking row is closed.

| ID | Boundary | Classification | Evidence | Required closure |
| --- | --- | --- | --- | --- |
| `P4-HOST-AAA-TOOL-BRIDGE` | AAA 0.14.3 on macOS | Existing host integration defect; not introduced by ARP | AAA's generic MCP manager connected ARP and displayed exactly three tools. The task agent then reported that no MCP tools were present in its callable inventory, so neither the required `MATCHED` nor negative call ran. The same UI also classified all three read-only ARP tools as high risk because `runtime` matched a broad write-operation substring. | Fix AAA's generic MCP-to-task tool bridge and read-only annotation/risk handling, rebuild the formal app, then rerun both visible verdicts using the unchanged generic ARP server. |
| `P4-EXT-CURSOR-AUTH` | Cursor on macOS | External authentication gate | A task-owned Cursor CLI install was available, but the host reported no authenticated session and no task-scoped API credential was provided. | Authenticate Cursor through the normal user-controlled flow, then rerun the task-owned MCP call. |
| `P4-EXT-VSCODE-HOST` | VS Code/GitHub Copilot on macOS or Windows | External host/account gate | No supported VS Code/Copilot Agent Host with an authenticated session was available in either acceptance environment. | Install and authenticate a supported host without copying credentials into the harness, then run tool discovery and verify. |
| `P4-EXT-WINDOWS-CODEX` | Codex on Windows 11 amd64 | External provider/session gate | The verified native Codex executable listed the task-scoped MCP server and spawned the ARP MCP child, but emitted no host JSON events before the bounded timeout. Network inspection showed only the host's existing local proxy path; ARP itself did not hang. | Restore a working native Codex model session and rerun the bounded host script to a schema-valid Proof. |
| `P4-EXT-WINDOWS-WSL` | Cursor on Windows | External platform gate | The installed Windows host has no WSL environment; the supported Cursor CLI path requires WSL rather than native Windows. | Enable a supported WSL environment through an owner-approved system change, or provide another supported Windows Cursor environment. |
| `P4-EXT-OPENCODE-HOST` | OpenCode on Windows or Linux | External host/account gate | No authenticated, officially usable OpenCode host was available on the native Windows machine or an amd64 Linux runner. | Provide a supported authenticated host and complete a real stdio MCP call. |
| `P4-EXT-DSH-HOST` | DeepSeek Harness on Windows or Linux | External host/runtime/account gate | The Windows machine has no usable Node/DSH host state, and no task-scoped provider credential was available. macOS DSH state was not reused because the frozen matrix requires Windows or Linux and credentials must not be copied. | Provide a supported Windows/Linux DSH environment with user-controlled authentication and run the official MCP client path. |
| `P4-EXT-LINUX-AMD64` | Linux amd64 | External runner gate | The local Docker engine is Linux arm64. Emulation on Apple Silicon was deliberately rejected as native amd64 evidence, and the Windows host has no WSL Linux runner. | Provide a real Linux amd64 runner or amd64 CI evidence and rerun generic/Profile, CLI, MCP, Witness, lifecycle, and residue gates. |

## Passed evidence retained

- Codex 0.149.1 and Claude Code 2.1.235 each made a real macOS stdio MCP
  `verify_local_runtime` call against the installed Phase 4 candidate and
  returned a schema-valid `MATCHED` Proof.
- macOS arm64, Linux arm64 container, and native Windows amd64 generic/Profile
  harnesses passed. Native Windows also passed the complete vendored package
  suite under the verified Go 1.27.0 toolchain.
- The failed host attempts did not require changes to firewall, proxy, DNS,
  route, registry, service, power, or Agent credential configuration.

The successful rows do not compensate for a missing mandatory named host or
platform row.
