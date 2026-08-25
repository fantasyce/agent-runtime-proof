# Phase 4 Host Profile and Real Host Matrix Status

- Date: 2026-08-25
- Final code candidate reviewed: `75a121a41d07`
- Branch: `codex/phase4-host-matrix`
- Decision: **NO-GO / incomplete**
- Phase 5 release work: **not started**

This record is an acceptance decision, not a completion claim. The Phase 4
implementation and every independently runnable gate are preserved, but the
frozen matrix requires every named host plus native Linux amd64 and three real
Windows Agent hosts. Those requirements are not all satisfied.

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
  credential helper blocked; this does not replace the still-missing mandatory
  native Linux amd64 row.

## Mandatory blockers

The complete evidence and closure conditions are in
`docs/issues/phase4-blockers.md`. In summary:

- AAA 0.14.3 discovers all three generic ARP tools but does not expose them to
  its task agent; its risk classifier also marks each read-only tool high risk.
- Cursor, VS Code/Copilot, OpenCode, and DeepSeek Harness lack a usable
  authenticated host in the required environments.
- Windows Codex lists and starts the MCP child but the model session produces
  no host events before timeout; zero of the required three Windows real Agent
  rows therefore pass.
- The available Linux container engine is arm64 and no native Linux amd64
  runner is currently available.

These are not counted as skips. Each missing mandatory row keeps Phase 4 and
v1 release at NO-GO.

## AAA generic integration evidence

The formal `/Applications/Across Agents Assistant.app` 0.14.3 build was used.
Its generic MCP manager connected the installed ARP candidate and displayed
the exact three tools. A UI-created task then failed truthfully because its
agent inventory contained no MCP tools, so neither requested verdict was
fabricated. The failed result was explicitly rejected and retained as a
hash-valid host defect record. The ARP server, helper, candidate directory,
and all task-owned temporary files were removed; AAA's original four MCP
contexts remained connected.

## Security and cleanup

- Review found no Profile path that executes configuration content. Binding
  ambiguity, secret projection, path traversal, symlink/reparse access, process
  identity, and cancellation remain fail-closed or explicit `UNKNOWN` paths.
- Evidence JSON contains only bounded versions, platform/host IDs, verdicts,
  Proof IDs, blocker IDs, and cleanup state; no prompts, transcripts, account
  identifiers, authentication material, or developer source paths are stored.
- Windows and macOS task processes, transfer/toolchain/config homes, helpers,
  temporary candidates, Cursor task install, AAA MCP registration, and marked
  task directories were removed. The Windows cleanup reported
  `WINDOWS_PHASE4_CLEANUP=PASS`.
- No firewall, SSH, proxy, DNS, default-route, VPN, registry, service, power, or
  Agent credential setting was changed to make a test pass.

## Decision boundary

Do not merge as a completed Phase 4, do not begin Phase 5 release, and do not
publish v1 from this state. Resume with the blocker register; once all mandatory
rows pass, rerun every final gate on the resulting commit and replace this
NO-GO record with a PASS record.
