# Phase 4 Host Profile and Real Host Matrix Status

- Date: 2026-08-25
- Final code candidate reviewed: `75a121a41d07`
- Branch: `codex/phase4-host-matrix`
- Decision: **NO-GO / incomplete**
- Phase 5 release work: **not started**

This record is an acceptance decision, not a completion claim. On 2026-08-25
the owner narrowed the active environment matrix to hosts that actually exist:
DeepSeek Harness on macOS, Codex on Windows, and a non-disruptive native Linux
amd64 run on the Fantasyce VPS. Cursor, VS Code/Copilot, and OpenCode are
deferred and must not be installed merely to satisfy this phase. Phase 4 is
still incomplete because the Windows host is offline. The AAA integration
blocker has been closed on the final installed application.

## Delivered implementation

- Seven reviewed, embedded, versioned Host Profiles for AAA, Codex, Claude
  Code, Cursor, OpenCode, DeepSeek Harness, and VS Code/GitHub Copilot.
- Bounded JSON, JSONC, TOML, and Cordis YAML parsing that treats host files as
  data, never executes discovered commands, rejects traversal/symlink access,
  limits bytes/depth/values/servers/arguments, and retains only hashes and safe
  identifiers.
- Deterministic host binding discovery, ambiguity detection, process matching,
  CLI `--host`/`--binding` flows, and the same binding operations through the
  existing three read-only MCP tools.
- Host guides, fixtures, installed-style generic/Profile runners, real Codex
  and Claude Code runners, Windows-native acceptance, and bounded performance
  measurement.

## Passed gates

### macOS 26.6.2 arm64

- The final candidate passed the generic/Profile installed runner with one
  `MATCHED`, one `UNKNOWN`, three exact MCP tools, Profile attribution, Proof
  self-verification, EOF cleanup, and no marked residue.
- Codex 0.149.1 and Claude Code 2.1.235 each listed the task MCP server and made
  a real `verify_local_runtime` call on final candidate bytes. Their safe Proof
  IDs are recorded in `docs/evidence/phase4-macos-host-matrix.json`.
- DeepSeek Harness 0.1.1-rc.2 used its official MCP client path and the user's
  existing authenticated state read-only. It made a real
  `verify_local_runtime` call and returned `MATCHED`; settings and credentials
  were unchanged, and the task-owned host/runtime files were removed.
- The reference runner measured 20 MCP startups at 43 ms p95, 19,382,272 bytes
  peak RSS, 331 microseconds candidate-match p95, and 3,964 ms for the bounded
  20,000-file case. These results apply only to the recorded runner.
- `scripts/check.sh` passed formatting, vet, module verification, trimpath
  build, the complete package suite under the race detector, and diff checks.

### Windows 11 amd64

- The native installed generic/Profile runner passed.
- The complete vendored package suite passed under the checksum-verified Go
  1.27.0 portable toolchain, including SDK, MCP transport, EOF escalation, and
  Job Object descendant cleanup.
- Native acceptance found and closed Windows missing-file error normalization,
  UTF-8 BOM input corruption, cleanup retry, cross-platform fixture path, and
  harness process-timeout defects through regression coverage or native rerun.

### Linux arm64 container

- The installed generic/Profile runner passed on the build-support arm64 row.
  A final repeat could not pull a fresh task image because the local Docker
  credential helper blocked. This optional repeat does not affect the separate
  mandatory native Linux amd64 row, which passed on the Fantasyce VPS.

### Linux amd64 native VPS

- The Fantasyce VPS is Ubuntu 24.04 on native x86_64. The installed-style
  generic/Profile runner passed on the candidate bytes.
- The run used only a marked task directory under `/tmp`. Xray, Nginx, SSH, and
  the pre-existing listener set were unchanged before and after the run; the
  marked directory and test processes were removed.

## Mandatory blocker

The complete evidence and closure conditions are in
`docs/issues/phase4-blockers.md`. In summary:

- Windows Codex lists and starts the MCP child but the model session produces
  no host events before timeout. A later minimal direct-call diagnostic could
  not be collected because the Windows host went offline. Historical Codex
  logs end before that attempt and show intermittent connection/time-out
  classes, so they do not yet prove whether the current failure is Codex,
  proxy/VPS routing, or Windows host availability.

Cursor, VS Code/Copilot, and OpenCode are recorded as owner-deferred, not as
active blockers. DeepSeek Harness is covered by the real macOS row. The native
Linux amd64 blocker is closed by the VPS run.

These are not counted as skips. Each missing mandatory row keeps Phase 4 and
v1 release at NO-GO.

## AAA generic integration evidence

The defect was in AAA's common host-to-local-agent bridge, not in ARP and not
in the three managed plugins. AAA commits `a7a95a8`, `c65bcd1`, and `081d290`
add a private-socket host tool provider, fail-closed read-only filtering,
same-owner MCP lifecycle handling, and a task-scoped stdio proxy for local
Codex. Independent review found no remaining actionable issue; 1,444 backend
tests, 33 E2E tests with 20 explicit environment skips, and 188 Swift tests
passed.

The rebuilt formal `/Applications/Across Agents Assistant.app` 0.14.3 used
clean producer sources for Context 0.11.1, Orchestrator 0.10.10, and Autopilot
0.5.3. All three installed plugins reported installed, available, probe true,
and integrity true. AAA connected the installed ARP candidate and exposed the
exact three tools as low-risk, read-only, approval-free tools. The installed
Codex adapter then made real MCP calls through AAA and returned `MATCHED` with
Proof ID
`sha256:756b0e993b551d2b0dfc4d1cafb67077274c32e0e220be582d52481d39ef3b44`
and the negative `UNKNOWN` with Proof ID
`sha256:4a91f259aa5e7d030ef1ae488953864ef7a4f131a2cbbd7aad32ae00bb3511a6`.

## Security and cleanup

- Review found no Profile path that executes configuration content. Binding
  ambiguity, secret projection, path traversal, symlink/reparse access, process
  identity, and cancellation remain fail-closed or explicit `UNKNOWN` paths.
- Evidence JSON contains only bounded versions, platform/host IDs, verdicts,
  Proof IDs, blocker IDs, and cleanup state; no prompts, transcripts, account
  identifiers, authentication material, or developer source paths are stored.
- Earlier Windows acceptance roots were cleaned and reported
  `WINDOWS_PHASE4_CLEANUP=PASS`. The later marked Codex diagnostic root cannot
  be inspected or removed while the whole host is offline; it is the only
  possible remaining task residue and is explicitly bound to the open Windows
  blocker. macOS and Linux task processes and marked roots were removed.
- No firewall, SSH, proxy, DNS, default-route, VPN, registry, service, power, or
  Agent credential setting was changed to make a test pass.

## Decision boundary

Do not merge as a completed Phase 4, do not begin Phase 5 release, and do not
publish v1 from this state. When the Windows machine is reachable, retrieve or
terminate the marked diagnostic process tree, run the bounded MCP-free and
proxy checks, then rerun the ARP Codex call and clean the marked root. That is
the only remaining closure path before replacing this NO-GO record with PASS.
