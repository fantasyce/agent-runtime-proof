# Phase 4 Host Profiles and Real Host Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver versioned, data-only Host Profiles and complete the required real local stdio MCP matrix for AAA, Codex, Claude Code, Cursor, OpenCode, DeepSeek Harness, VS Code/GitHub Copilot, and the generic fixture without weakening ARP's privacy or read-only boundaries.

**Architecture:** `internal/hostprofile` embeds a versioned catalog, reads only bounded declared configuration locations, parses JSON/JSONC, TOML, and YAML as data, and emits immutable bindings plus safe host attribution. CLI, MCP, and the application service consume the same binding resolver; explicit PID/expectation operation remains available when no Profile exists. Real-host acceptance installs one independent ARP candidate, writes only task-owned host configurations, invokes each host's supported stdio path, records Proof IDs and versions, then removes every task-owned process and file.

**Tech Stack:** Go 1.26 language baseline; Go 1.27 acceptance toolchain; `github.com/modelcontextprotocol/go-sdk` v1.6.1; JSON Schema 2020-12; `github.com/tailscale/hujson` for JSONC standardization; `github.com/pelletier/go-toml/v2` for TOML data decoding; `gopkg.in/yaml.v3` node decoding with executable tags rejected; macOS arm64, Windows 11 amd64, and native/container Linux amd64 acceptance.

**Spec:** `docs/architecture-development-acceptance.md` sections 12-15, 18 Phase 4, and 19.1-19.6.

## Global Constraints

- Work only in the independent `agent-runtime-proof` repository and the `codex/phase4-host-matrix` branch based on accepted Phase 3 commit `f5d5f9b`.
- A Host Profile produces candidates, bindings, and attribution only; it never decides a verdict.
- Discovery reads only an embedded Profile's declared paths or a caller-explicit task-owned path; it never performs an unbounded home or disk scan.
- Parsers never execute shell syntax, interpolation, YAML JavaScript tags, plugins, imports, commands, or dynamic expressions.
- Raw config bytes, paths, command lines, environment values, tokens, and transcripts never enter Proof, CLI JSON, MCP results, logs, or acceptance records.
- Unsupported or malformed host configuration yields typed `HOST_CONFIG_INVALID` or `HOST_CONFIG_INACCESSIBLE` evidence without breaking explicit PID/expectation verification.
- `clientInfo` is never trusted as host identity; a Profile or binding is selected only through an explicit input.
- All real-host configurations, installs, homes, caches, logs, and workspaces are task-owned and removed after acceptance. Existing user host configuration and credentials are not copied, read into reports, or modified.
- No network listener, daemon, administrator/root privilege, firewall/DNS/proxy/route/VPN change, or host-specific ARP implementation is allowed.
- Windows native results cannot be replaced by cross-compilation; Linux amd64 results cannot be replaced by Rosetta translation.

---

### Task 1: Freeze the Host Profile contract and embedded catalog

**Files:**
- Create: `schemas/agent-runtime-host-profile-1.0.schema.json`
- Modify: `schemas/embed.go`
- Create: `profiles/hosts/aaa.json`
- Create: `profiles/hosts/codex.json`
- Create: `profiles/hosts/claude-code.json`
- Create: `profiles/hosts/cursor.json`
- Create: `profiles/hosts/opencode.json`
- Create: `profiles/hosts/deepseek-harness.json`
- Create: `profiles/hosts/vscode-copilot.json`
- Create: `profiles/embed.go`
- Create: `internal/hostprofile/model.go`
- Create: `internal/hostprofile/catalog.go`
- Create: `internal/hostprofile/catalog_test.go`
- Modify: `internal/contract/validator.go`
- Modify: `internal/contract/validator_test.go`

**Interfaces:**
- Produces `hostprofile.Profile`, `ConfigSource`, `ProcessMatcher`, `Catalog`, `Catalog.Host(string) (Profile, bool)`, and `contract.ValidateHostProfile([]byte) error`.
- Profile fields are `schema_version`, `host_id`, `display_name`, `fixture_version`, `platforms`, `process_matchers`, and `config_sources`.
- A config source declares `source_id`, `platforms`, `candidate_paths`, `format`, `dialect`, `maximum_bytes`, and safe/secret field classes. Candidate paths may use only `${HOME}`, `${WORKSPACE}`, and `${PROFILE}` placeholders.

- [ ] **Step 1: Add failing schema and catalog tests.** Cover all seven IDs, unique source IDs, supported platform enums, bounded path templates, allowed formats (`json`, `jsonc`, `toml`, `yaml`), allowed dialects (`mcp-servers`, `vscode-servers`, `opencode-v2`, `codex-toml`, `dsh-cordis`, `generic-only`), rejection of unknown fields, absolute literal user paths, `..`, secret-bearing defaults, duplicate profiles, and a catalog lookup that returns a defensive copy.
- [ ] **Step 2: Run `go test ./internal/contract ./internal/hostprofile` and confirm failure because the Profile contract and package are absent.**
- [ ] **Step 3: Implement the schema, embedded catalog, strict validation, and immutable lookup.** Embed only the seven reviewed JSON Profile documents; fail package initialization in tests if any embedded document is invalid.
- [ ] **Step 4: Run focused tests and `GOOS=windows GOARCH=amd64 go test -c ./internal/hostprofile`; confirm all pass.**
- [ ] **Step 5: Commit `feat: define versioned host profiles`.**

### Task 2: Implement bounded data-only config parsing

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/hostprofile/parse.go`
- Create: `internal/hostprofile/parse_json.go`
- Create: `internal/hostprofile/parse_toml.go`
- Create: `internal/hostprofile/parse_yaml.go`
- Create: `internal/hostprofile/parse_test.go`
- Create: `testdata/host-configs/codex/config.toml`
- Create: `testdata/host-configs/claude-code/.mcp.json`
- Create: `testdata/host-configs/cursor/.cursor/mcp.json`
- Create: `testdata/host-configs/opencode/opencode.jsonc`
- Create: `testdata/host-configs/deepseek-harness/.dsh/cordis.patch.yml`
- Create: `testdata/host-configs/vscode-copilot/.mcp.json`
- Create: `testdata/host-configs/invalid/`

**Interfaces:**
- Produces `parseConfig(profile Profile, source ConfigSource, bytes []byte) ([]RawBinding, error)`.
- `RawBinding` retains `ServerName`, `Command`, `Args`, and source metadata only in memory; environment values are discarded before the value leaves the parser.
- Parser limits are 1 MiB input, 128 levels, 4,096 object members, 4,096 array values, 128 servers, 128 args per server, and 8 KiB per scalar.

- [ ] **Step 1: Add failing table tests for every dialect and format.** Fixtures must extract one direct `agent-runtime-proof mcp` entry while ignoring unrelated servers. Include JSONC comments/trailing commas, Codex TOML tables, OpenCode v2 nesting, VS Code `servers`, Claude/Cursor `mcpServers`, and DSH Cordis insert rows.
- [ ] **Step 2: Add failing hostile-input tests.** Reject duplicate keys after JSONC normalization, YAML aliases/cycles, non-scalar keys, `!!js`, anchors, merge keys, TOML dotted-key collisions, shell/interpolation commands, NULs, excessive depth/size/counts, non-string env values, and command arrays that do not map to a direct executable plus argv.
- [ ] **Step 3: Run `go test ./internal/hostprofile -run 'TestParse'` and confirm the expected missing-parser failures.**
- [ ] **Step 4: Add the three locked parser dependencies and implement strict data projection.** JSONC is standardized to JSON before duplicate-key validation; YAML is walked as a node tree and accepts only standard scalar/sequence/mapping tags; no parser exposes environment values.
- [ ] **Step 5: Run focused tests with `-race -count=3`, then `go mod verify`; confirm pass.**
- [ ] **Step 6: Commit `feat: parse host configuration as bounded data`.**

### Task 3: Discover immutable bindings and match live processes

**Files:**
- Create: `internal/hostprofile/discovery.go`
- Create: `internal/hostprofile/discovery_test.go`
- Create: `internal/hostprofile/binding.go`
- Create: `internal/hostprofile/binding_test.go`
- Create: `internal/hostprofile/open_nofollow_unix.go`
- Create: `internal/hostprofile/open_nofollow_windows.go`
- Modify: `internal/model/model.go`
- Modify: `contracts/privacy-registry.json`
- Modify: `internal/contracttest/privacy_test.go`

**Interfaces:**
- Produces `hostprofile.Discover(ctx, Request) (Result, error)`, where `Request` contains `HostID`, `Platform`, `Home`, `Workspace`, `ProfileName`, and optional `ExplicitConfigPath`.
- `Result.Bindings` contains immutable `Binding{ID, HostID, ServerName, CommandBasename, CommandPathHash, ArgumentFingerprints, ConfigSourceHash, Confidence}` plus unexported direct command data used only for matching.
- Binding IDs are deterministic `host_id.server_name` identifiers. If multiple readable sources define different bytes for that identifier, discovery returns a typed ambiguity rather than choosing by timestamp.
- Produces `Binding.Match([]model.Candidate) (model.Candidate, error)` using a resolved direct executable path and safe positional fingerprints; zero candidates means not running, multiple candidates means ambiguous.

- [ ] **Step 1: Add failing discovery tests.** Cover `${HOME}`/`${WORKSPACE}` expansion, Windows profile paths, missing optional files, explicit paths, symlink/junction rejection, no-follow reads, mutation during read, deterministic config hash, stable binding ID, same-content deduplication, conflicting-source ambiguity, unreadable config, and zero residue.
- [ ] **Step 2: Add failing process-match tests.** Cover exact resolved path, basename-only hint that never becomes bound confidence, one candidate, zero candidates, duplicate candidates, interpreter/wrapper mismatch, and PID-creation-time revalidation.
- [ ] **Step 3: Run focused tests and confirm failures because discovery/matching are absent.**
- [ ] **Step 4: Implement pinned bounded reads, canonical config hashing, safe projections, typed errors, and deterministic binding aggregation.** Reuse existing path/privacy hashing domains; never serialize raw `Command` or `Args`.
- [ ] **Step 5: Implement conservative process matching.** A binding is `bound` only with exact resolved executable evidence; basename matches remain `hint` and cannot produce `MATCHED` without expectation/artifact evidence.
- [ ] **Step 6: Run `go test -race ./internal/hostprofile ./internal/contracttest -count=3` and Windows cross-builds; confirm pass.**
- [ ] **Step 7: Commit `feat: discover safe host bindings`.**

### Task 4: Wire bindings through app, CLI, doctor, and MCP

**Files:**
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Modify: `internal/app/doctor.go`
- Modify: `internal/app/doctor_test.go`
- Modify: `internal/proof/build_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `internal/cli/golden_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `internal/mcpserver/stdio_test.go`
- Modify: `cmd/agent-runtime-proof/main.go`

**Interfaces:**
- Extends `app.InspectRequest` with `HostID` and `BindingID`; exactly one of PID, binding, or all is accepted.
- Extends `app.VerifyRequest` so exactly one of PID or binding is accepted while exactly one expectation source remains mandatory.
- Adds `app.DoctorRequest{HostID string}` and `Service.Doctor(context.Context, DoctorRequest) DoctorResult`.
- `Service` receives a `HostProfiles` resolver interface so unit tests never touch real user configuration.
- CLI accepts `inspect --binding ID`, `verify --binding ID`, and `doctor --host HOST_ID` as frozen by the architecture document.
- MCP list results include safe `host_id` and `binding_id`; tool inputs accept explicit host/binding selectors without trusting `clientInfo`.

- [ ] **Step 1: Add failing application tests.** Prove binding-selected inspect/verify, host attribution, missing/ambiguous binding verdicts, profile parse limitations, explicit PID fallback after Profile failure, and no raw config data in errors or Proof JSON.
- [ ] **Step 2: Run application tests and confirm the new selector cases fail.**
- [ ] **Step 3: Implement minimal resolver composition and host-attributed Proof building.** Do not change evaluator verdict rules or let a Profile override expectation/artifact evidence.
- [ ] **Step 4: Add failing CLI tests and goldens for selector exclusivity, JSON output, doctor host checks, negative/unknown exit codes, and sanitized diagnostics.**
- [ ] **Step 5: Implement CLI flags and update help text; run CLI tests until green.**
- [ ] **Step 6: Add failing MCP tests for host candidate listing, binding inspect/verify, domain results vs transport errors, prior-client negotiation, cancellation, EOF, concurrency isolation, and untrusted `clientInfo`.**
- [ ] **Step 7: Implement MCP binding inputs/outputs and runtime composition; run MCP tests with `-race -count=3`.**
- [ ] **Step 8: Run `./scripts/check.sh`, all Windows cross-builds, and `git diff --check`; confirm pass.**
- [ ] **Step 9: Commit `feat: expose host binding verification`.**

### Task 5: Package Profiles, configuration fixtures, and host guidance

**Files:**
- Modify: `docs/host-configuration.md`
- Create: `docs/hosts/aaa.md`
- Create: `docs/hosts/codex.md`
- Create: `docs/hosts/claude-code.md`
- Create: `docs/hosts/cursor.md`
- Create: `docs/hosts/opencode.md`
- Create: `docs/hosts/deepseek-harness.md`
- Create: `docs/hosts/vscode-copilot.md`
- Modify: `plugin/agent-runtime-proof/.mcp.json`
- Modify: `plugin/agent-runtime-proof/.codex-plugin/plugin.json`
- Modify: `plugin/agent-runtime-proof/skills/agent-runtime-proof/SKILL.md`
- Create: `scripts/verify_host_profiles.sh`
- Modify: `.github/workflows/quality.yml`

**Interfaces:**
- Every guide provides one task-owned/project-scoped stdio configuration using an absolute installed ARP path, a discovery command, a read-only verify prompt/action, cleanup, and a clear statement that ARP never writes host configuration.
- `scripts/verify_host_profiles.sh` validates embedded Profiles, fixtures, documentation snippets, plugin manifest/MCP config consistency, no absolute developer paths, and cross-platform parse compatibility.

- [ ] **Step 1: Write failing documentation/config consistency tests in `scripts/verify_host_profiles.sh`.** Verify seven guides/Profiles/fixtures, exactly three MCP tools, direct `mcp` argv, no shell wrapper, schema/catalog consistency, plugin version consistency, and absence of secrets or developer paths.
- [ ] **Step 2: Run the guard and confirm failure for missing guides and stale plugin guidance.**
- [ ] **Step 3: Write host-specific instructions from official public formats, update the thin plugin, and keep every snippet task/project scoped.** Do not include credentials, user paths, or automatic installers in product code.
- [ ] **Step 4: Extend CI to run the Profile guard and Phase 3 gates on macOS/Linux/Windows without claiming authenticated real-host CI.**
- [ ] **Step 5: Run the guard, `./scripts/check.sh`, and secret/path scans; confirm pass.**
- [ ] **Step 6: Commit `docs: add host profile configuration guides`.**

### Task 6: Build repeatable real-host and performance acceptance harnesses

**Files:**
- Create: `scripts/run_phase4_acceptance.sh`
- Create: `scripts/run_windows_phase4_acceptance.ps1`
- Create: `scripts/host-matrix/common.sh`
- Create: `scripts/host-matrix/verify-proof.mjs`
- Create: `scripts/host-matrix/run-dsh.mjs`
- Create: `scripts/measure_phase4_limits.sh`
- Create: `testdata/phase4-helper/main.go`
- Create: `internal/hostprofile/performance_test.go`

**Interfaces:**
- Host runners receive only task-owned `ARP_CANDIDATE`, `ARP_EXPECTATION`, `ARP_HELPER_PID`, `ARP_HOST_HOME`, and `ARP_WORKSPACE` paths/IDs; they emit a bounded result row containing host, host version, platform, ARP commit, tool names, verdict, Proof ID, and residue status.
- Acceptance never prints or persists host authentication state. A proprietary host may use an already authenticated installed session, but the harness neither copies nor inspects its credentials.

- [ ] **Step 1: Write acceptance assertions before running any host.** Require installed-path provenance, three exact tools, one `MATCHED`, one negative verdict, schema-valid/self-verifying Proof, host attribution when a Profile is selected, child exit after EOF, and zero marked residue.
- [ ] **Step 2: Build a versioned `0.4.0-phase4` installed candidate and controlled helper in a task-owned root; generate current and negative expectations without repository paths.**
- [ ] **Step 3: Add the generic MCP fixture for macOS, native Linux amd64, Linux arm64 container, and Windows 11 amd64; confirm explicit PID/expectation remains Profile-independent.**
- [ ] **Step 4: Add reference-runner performance checks.** Measure 20 MCP startups for p95, a synthetic immutable 1,000-candidate inventory for app-layer p95, a bounded 20,000-file/256 MiB declaration case, cancellation within one second, and peak RSS below 150 MiB; record hardware/OS and never generalize results.
- [ ] **Step 5: Run the macOS generic/Profile gates and performance harness; fix every product defect through a failing regression test.**
- [ ] **Step 6: Run native/container Linux gates and Windows cross-builds; keep Windows real-host execution for Task 8.**
- [ ] **Step 7: Commit `test: add Phase 4 host acceptance harnesses`.**

### Task 7: Execute macOS real hosts and AAA generic integration

**Files:**
- Create: `docs/evidence/phase4-macos-host-matrix.json`
- Create: `docs/issues/phase4-blockers.md`

**Interfaces:**
- Evidence JSON stores only versions, platform, ARP commit, safe host/binding IDs, verdicts, Proof IDs, timestamps, and cleanup results. It never stores prompts, responses, transcripts, config paths, account identifiers, or credentials.

- [ ] **Step 1: Verify host availability and official stdio capability immediately before each run.** Use current official docs and installed `--version` output; task-own any new CLI installation and disable its updater where supported.
- [ ] **Step 2: Run Codex on macOS with a command-line/task-scoped MCP override; list the three tools and call binding-based verify.** Preserve the existing Codex account state without reading or copying authentication files.
- [ ] **Step 3: Run Claude Code with task-owned `--mcp-config` and explicit `--allowedTools mcp__agent-runtime-proof`; list tools and call verify.** If authentication is unavailable, record the exact external gate and continue other hosts without substituting Claude Desktop.
- [ ] **Step 4: Run Cursor CLI from a task-owned install/home; use project `.cursor/mcp.json`, `mcp list-tools`, and non-interactive read-only prompt to call verify.** Do not write the real `~/.cursor` config.
- [ ] **Step 5: Run VS Code/GitHub Copilot Agent Host from a task-owned portable install/profile using workspace `.mcp.json`; discover tools and call verify.** Do not modify the user's VS Code/Copilot profile.
- [ ] **Step 6: Run the formal installed AAA app as a generic MCP consumer.** Register the installed ARP candidate through AAA's generic MCP manager using task-owned acceptance state, discover exactly three tools, call one `MATCHED` and one negative verdict, verify visible evidence, then remove the server and all task-owned state. Do not add an ARP-specific AAA API, page, or model.
- [ ] **Step 7: Validate the macOS evidence JSON against the acceptance assertions and confirm no host/server/helper process or task directory remains.**
- [ ] **Step 8: Commit `test: record macOS Phase 4 host matrix` only when every non-external macOS gate passes.**

### Task 8: Execute Linux/Windows real hosts and close the named matrix

**Files:**
- Create: `docs/evidence/phase4-linux-host-matrix.json`
- Create: `docs/evidence/phase4-windows-host-matrix.json`
- Modify: `docs/issues/phase4-blockers.md`

**Interfaces:**
- Windows uses the existing pinned public-key/known-host SSH transport and a marked task root. Linux uses a marked task directory on a native amd64 host or a no-network container where the named host genuinely supports that environment.

- [ ] **Step 1: Run OpenCode on native Linux amd64 or Windows from a task-owned install/config; confirm local MCP status, tool discovery, and a real verify call.**
- [ ] **Step 2: Run DeepSeek Harness with the official `@deepseek-ai/dsh-mcp-client` stdio row in a task-owned Cordis overlay; wait for `mcp__agent-runtime-proof__*` registration, invoke verify through the real host, dispose the plugin, and prove child cleanup.**
- [ ] **Step 3: Run Cursor on Windows in addition to macOS as required by the frozen matrix.** Use a task-owned CLI/config and the same installed ARP asset; record native host and Profile evidence.
- [ ] **Step 4: Ensure at least three genuine Windows Agent hosts have passed.** Use Cursor plus any two of Claude Code, OpenCode, VS Code/Copilot, Codex, or DeepSeek Harness that officially support the installed Windows environment; do not relabel the generic fixture as an Agent host.
- [ ] **Step 5: Run the Windows generic MCP/Profile harness, native full Go suite, installed ARP candidate, and cleanup gate.** Do not change firewall, SSH, accounts, services, registry, PATH, proxy, DNS, power, or Agent configuration.
- [ ] **Step 6: Run Linux amd64 and Windows evidence validation; remove marked toolchains, vendor/source archives, configs, homes, caches, helpers, MCP children, and transfer roots after confirming ownership and no live process.**
- [ ] **Step 7: Commit `test: record Linux and Windows Phase 4 host matrix` only when every required named host has a real call or remains explicitly external-blocked.**

### Task 9: Final regression, review, and Phase 4 acceptance

**Files:**
- Create: `docs/phase4-acceptance.md`
- Modify: `docs/architecture-development-acceptance.md`
- Modify: `README.md`
- Modify: `docs/issues/phase4-blockers.md`
- Modify: `docs/superpowers/plans/2026-08-25-phase4-host-profiles-matrix.md`

**Interfaces:**
- The final acceptance record distinguishes automated tests, installed artifacts, OS-native evidence, real Agent calls, AAA integration, performance results, skipped/external gates, and Phase 5 release work.

- [ ] **Step 1: Run the complete Go 1.26/1.27 local gate, race suite, Profile guard, macOS installed/generic/real-host gates, native Linux amd64 and container gates, Windows cross-builds, and native Windows gate on the final code candidate.**
- [ ] **Step 2: Run source/binary/archive scans for credentials, home/source paths, caches, test content, and private fixture data; validate Proof IDs and every evidence JSON.**
- [ ] **Step 3: Confirm zero marked processes/directories and unchanged proxy, DNS, default route, VPN, firewall, and network-extension baselines after remote testing.**
- [ ] **Step 4: Review every Phase 4 requirement line-by-line.** Any missing named host is a Phase 4 NO-GO, not a skip; preserve it in the blocker register and continue all independent work.
- [ ] **Step 5: Perform an independent code/security review of parser execution risks, path traversal, secret projection, binding ambiguity, process matching, and lifecycle cleanup; fix findings through failing regression tests.**
- [ ] **Step 6: Update README and the authoritative design only if all mandatory Phase 4 gates pass; otherwise keep the status as incomplete and name exact blockers.**
- [ ] **Step 7: Commit `docs: accept Phase 4 host matrix` only when there are no unresolved Phase 4 defects or mandatory external gates.**
- [ ] **Step 8: Rerun `./scripts/check.sh` on the final commit and verify a clean worktree.**
- [ ] **Step 9: Use `superpowers:finishing-a-development-branch` to present integration options; do not merge, push, publish, or begin Phase 5 without the corresponding authorized branch decision and release gates.**
