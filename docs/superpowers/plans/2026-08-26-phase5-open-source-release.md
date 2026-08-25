# Phase 5 Open Source Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish Agent Runtime Proof v1.0.0 as a reproducible public open-source release with three supported platform archives, checksums, CycloneDX SBOMs, GitHub build provenance, lifecycle documentation, and installed-host evidence.

**Architecture:** Keep the Go runtime unchanged and add a repository-owned release layer around it. A single strict asset builder cross-compiles the three supported targets from one commit, packages the plugin and public contracts, invokes the pinned official CycloneDX Go tool, and produces deterministic checksums; a tag-only GitHub workflow verifies the tree, attests the assets, and publishes them from `origin/main`.

**Tech Stack:** Go 1.26 language baseline, Go 1.27 release toolchain, Bash, PowerShell, GitHub Actions, CycloneDX GoMod v1.12.0, GitHub artifact attestations.

**Spec:** `docs/architecture-development-acceptance.md` sections 18 Phase 5, 19, and 20.

## Global Constraints

- Release version is `1.0.0`; Git tag is `v1.0.0`.
- Supported release targets are `darwin/arm64`, `windows/amd64`, and `linux/amd64` with `CGO_ENABLED=0`.
- Tags and GitHub Releases must target the exact public `origin/main` commit.
- Release archives must not contain development paths, credentials, caches, test fixtures, or source implementation.
- Public runtime interfaces remain CLI, local stdio MCP, and Witness; Phase 5 adds no network listener, daemon, repair, telemetry, or remote-Agent behavior.
- AAA consumes ARP only through generic MCP and must not bundle ARP as a fourth managed plugin.
- Apache-2.0 applies to ARP; no AAA AGPL implementation is copied into this repository.

---

### Task 1: Freeze v1 version and public governance

**Files:**
- Create: `VERSION`
- Create: `LICENSE`
- Create: `SECURITY.md`
- Create: `CHANGELOG.md`
- Modify: `cmd/agent-runtime-proof/main.go`
- Modify: `plugin/agent-runtime-proof/.codex-plugin/plugin.json`
- Modify: `README.md`
- Create: `scripts/verify_release_metadata.sh`

**Interfaces:**
- Consumes: version string `1.0.0` and repository identity `fantasyce/agent-runtime-proof`.
- Produces: one canonical `VERSION` value used by builds, plugin metadata, docs, and release gates.

- [ ] **Step 1: Add a failing metadata gate** that requires exact version agreement, Apache-2.0 text, security reporting instructions, current changelog entry, installation links, and no Phase 4 development suffix.
- [ ] **Step 2: Run `bash scripts/verify_release_metadata.sh` and confirm it fails because the v1 surfaces do not exist.**
- [ ] **Step 3: Add the minimal v1 metadata and governance files, and make the Go/plugin development defaults `1.0.0-dev` while release ldflags inject exact `1.0.0`.**
- [ ] **Step 4: Run the metadata gate, Profile gate, and focused Go tests; confirm they pass.**
- [ ] **Step 5: Commit with `docs: freeze Agent Runtime Proof v1 metadata`.**

### Task 2: Build reproducible release assets and SBOMs

**Files:**
- Create: `scripts/build_release_assets.sh`
- Create: `scripts/verify_release_assets.sh`
- Create: `scripts/test_release_assets.sh`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `VERSION`, a full Git commit, Go 1.27, and `cyclonedx-gomod` v1.12.0.
- Produces: `agent-runtime-proof_<version>_<os>_<arch>.tar.gz` or `.zip`, one `.cdx.json` per binary, `agent-runtime-proof_<version>_source.tar.gz`, and `SHA256SUMS` under a caller-owned output directory.

- [ ] **Step 1: Add failing release-asset tests** for target names, embedded version/commit, archive allowlists, executable mode, checksum completeness, SBOM identity, deterministic source archive, secret/path/cache rejection, and refusal of dirty or mismatched inputs.
- [ ] **Step 2: Run `bash scripts/test_release_assets.sh` and confirm the missing builder fails.**
- [ ] **Step 3: Implement the minimal strict builder and verifier** with task-owned temporary roots, `-trimpath`, explicit target tuples, archive allowlists, and pinned SBOM tooling.
- [ ] **Step 4: Run the asset tests twice and compare checksums; run binaries or native metadata inspection appropriate to each target.**
- [ ] **Step 5: Commit with `build: add reproducible v1 release assets`.**

### Task 3: Add tag-only GitHub publication and provenance

**Files:**
- Create: `.github/workflows/release.yml`
- Create: `scripts/verify_release_workflow.sh`
- Modify: `.github/workflows/quality.yml`

**Interfaces:**
- Consumes: an annotated `v1.0.0` tag whose commit equals `origin/main`, release asset scripts, and GitHub OIDC.
- Produces: a non-draft GitHub Release, uploaded assets, and GitHub artifact attestations covering every checksum-listed file.

- [ ] **Step 1: Add a failing workflow guard** requiring tag-only trigger, exact tag/version validation, immutable action SHAs, least-privilege permissions, full local gates, checksums, SBOMs, artifact attestation, and GitHub release upload.
- [ ] **Step 2: Run the workflow guard and confirm it fails because `release.yml` is absent.**
- [ ] **Step 3: Add the minimal workflow** pinned to checkout `3d3c42e5aac5ba805825da76410c181273ba90b1`, setup-go `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e`, upload-artifact `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`, and attest-build-provenance `4d101475d8b20a2381f78447822ac1eab6504dd8`.
- [ ] **Step 4: Run the workflow guard, metadata guard, Profile guard, and full local check.**
- [ ] **Step 5: Commit with `ci: publish attested Agent Runtime Proof releases`.**

### Task 4: Document and test install lifecycle

**Files:**
- Create: `docs/install.md`
- Create: `scripts/test_install_lifecycle.sh`
- Modify: `README.md`

**Interfaces:**
- Consumes: a verified release archive and `SHA256SUMS`.
- Produces: documented macOS/Linux/Windows install, upgrade, downgrade, uninstall, checksum, SBOM, and attestation verification paths.

- [ ] **Step 1: Add a failing isolated lifecycle test** covering clean install, same-version repair, upgrade from a synthetic prior version, downgrade back to it, uninstall, CLI/MCP smoke, and zero residue.
- [ ] **Step 2: Run the lifecycle test and confirm it fails because install documentation/contracts are missing.**
- [ ] **Step 3: Add the minimal lifecycle guide and safe task-owned test implementation.**
- [ ] **Step 4: Build final candidate assets and run lifecycle, source/archive, secret/path, and residue gates.**
- [ ] **Step 5: Commit with `docs: add v1 install and verification lifecycle`.**

### Task 5: Record Phase 5 acceptance and integrate through PR

**Files:**
- Create: `docs/phase5-acceptance.md`
- Modify: `docs/architecture-development-acceptance.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: Phase 1-4 evidence, final asset verification, lifecycle evidence, and AAA generic integration evidence.
- Produces: the v1 release GO/NO-GO decision with explicit public-release boundaries.

- [ ] **Step 1: Run every final local gate and record exact results, target names, hashes, tool versions, limitations, and cleanup.**
- [ ] **Step 2: Write the Phase 5 acceptance record and update status to release-candidate ready without claiming publication.**
- [ ] **Step 3: Push `codex/phase5-release`, create a non-draft PR to `main`, and wait for all required checks.**
- [ ] **Step 4: Merge the green PR with branch deletion, fetch/prune, and verify `origin/main` contains Phase 1-5 with no unmatched local patch.**

### Task 6: Publish and verify v1.0.0

**Files:**
- No source changes after the release commit.

**Interfaces:**
- Consumes: public `origin/main`, the exact merged commit, and green default-branch quality checks.
- Produces: public repository, immutable `v1.0.0` tag/Release, assets, SBOMs, checksums, attestations, and only the long-lived remote `main` branch.

- [ ] **Step 1: Make the repository public only after secret/history audit and branch protection checks pass.**
- [ ] **Step 2: Create `v1.0.0` from the exact `origin/main` SHA and wait for the tag release workflow.**
- [ ] **Step 3: Verify every release asset checksum, SBOM, attestation, tag target, release visibility, open PR/issue status, CodeQL state, and remote branch list.**
- [ ] **Step 4: Install the released macOS arm64 asset outside the repository and rerun CLI, MCP, Codex/DeepSeek host smoke, and AAA generic integration.**
- [ ] **Step 5: Remove task-owned release artifacts and preserve only the installed release binary and documented acceptance evidence.**

### Task 7: Release AAA v0.14.4 with the generic MCP bridge

**Files:**
- Use AAA's `OPEN_SOURCE_RELEASE_HANDBOOK.md` and release/version surfaces.

**Interfaces:**
- Consumes: public ARP v1.0.0, AAA commits `a7a95a8`, `c65bcd1`, `081d290`, and unchanged released Context 0.11.1, Orchestrator 0.10.10, Autopilot 0.5.3.
- Produces: AAA v0.14.4 on public `main`, a formal installed app, healthy managed plugins, and released-ARP generic MCP evidence.

- [ ] **Step 1: Review and disposition every open dependency PR before release.**
- [ ] **Step 2: Merge the generic MCP bridge through a feature PR and delete its remote branch.**
- [ ] **Step 3: Create `codex/release-v0.14.4`, update all AAA version/release surfaces and changelog, and run focused tests.**
- [ ] **Step 4: Run the full AAA automated release matrix, mandatory packaged-App user journeys, three-plugin integrity/probe checks, and released-ARP `MATCHED` plus negative verdict integration.**
- [ ] **Step 5: Run GitHub Live E2E, create and merge the release PR, publish `v0.14.4` from exact `origin/main`, and delete short-lived remote branches.**
- [ ] **Step 6: Rebuild `/Applications/Across Agents Assistant.app` from released main and verify version, signature, backend socket, three plugins, Worker/local fallback, and ARP generic tools.**

### Task 8: Final ecosystem audit

**Files:**
- No required source changes.

**Interfaces:**
- Consumes: ARP v1.0.0 and AAA v0.14.4 public release state.
- Produces: final evidence that tags equal `origin/main`, remote branches are clean, releases are public, installed bytes match, and no task residue remains.

- [ ] **Step 1: Fetch/prune/tags for all affected repositories and compare release tags to `origin/main`.**
- [ ] **Step 2: Verify GitHub PRs, issues, releases, Actions, CodeQL, visibility, and remote heads.**
- [ ] **Step 3: Verify local installed versions and live health, then remove merged local feature/release worktrees and branches only when Git proves they are fully merged.**
- [ ] **Step 4: Report released versus deferred scope without treating fixture-only hosts or Apple notarization as failures.**
