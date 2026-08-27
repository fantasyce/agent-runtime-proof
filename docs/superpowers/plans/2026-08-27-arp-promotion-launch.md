# Agent Runtime Proof Promotion Launch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish ARP v1.1.0 with a conversion-ready repository, reproducible stale-runtime demo, verified cross-platform MCPB distribution, official MCP Registry entry, and evidence-backed public launch.

**Architecture:** Preserve the existing Go CLI, verdict model, local `stdio` MCP server, and no-network privacy boundary. Add release-shaping scripts and metadata around those stable interfaces, validate every artifact from clean temporary roots, and gate all public announcements on post-publication verification of the GitHub Release and Registry record.

**Tech Stack:** Go 1.27, Bash, PowerShell, Python 3 standard library, GitHub Actions, CycloneDX, GitHub artifact attestations, MCPB manifest v0.3, official MCP Registry publisher, static HTML/CSS GitHub Pages.

**Spec:** `docs/superpowers/specs/2026-08-27-arp-promotion-launch-design.md`

## Global Constraints

- The release version is `1.1.0` and its tag must point to `origin/main`.
- ARP remains local-only, read-only, `stdio`-only, and opens no network listener.
- ARP remains independent of AAA; AAA is one generic MCP host, not a dependency.
- Supported release targets remain macOS arm64, Linux amd64, and Windows amd64.
- Do not add silent telemetry, upload local state, or recommend `curl | sh` installation.
- Every release-shaped test uses task-owned temporary directories and removes them.
- Public announcements occur only after GitHub Release and Registry verification.

---

### Task 1: Repository conversion contract and community surface

**Files:**
- Create: `scripts/test_public_surface.sh`
- Create: `docs/quickstart.md`
- Create: `docs/data-handling.md`
- Create: `CONTRIBUTING.md`
- Create: `CODE_OF_CONDUCT.md`
- Create: `SUPPORT.md`
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/runtime_case.yml`
- Create: `.github/ISSUE_TEMPLATE/config.yml`
- Create: `.github/pull_request_template.md`
- Modify: `README.md`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: existing CLI commands, release links, host guides, privacy and threat documents.
- Produces: `scripts/test_public_surface.sh`, the repository-wide public-content gate used by later release tasks.

- [ ] **Step 1: Write the failing public-surface test**

Create `scripts/test_public_surface.sh` with `set -euo pipefail`. Assert that the README contains the exact positioning sentence, a `Quickstart` heading before architecture/acceptance material, links to `docs/quickstart.md`, `docs/data-handling.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, and `SUPPORT.md`, and that all community-template files exist. Assert that README does not call ARP a fourth managed plugin and does not contain `curl.*\|.*sh`.

- [ ] **Step 2: Run the test and verify RED**

Run: `bash scripts/test_public_surface.sh`

Expected: non-zero because the new quickstart and community files do not exist.

- [ ] **Step 3: Implement the public surface**

Rewrite README in the order required by the spec. Keep detailed architecture and acceptance links below the install, first-proof, MCP, and privacy sections. Add focused community documents and GitHub templates with public issue links, private security-report routing, reproduction fields, OS/architecture fields, and a checkbox forbidding secrets or raw process data.

- [ ] **Step 4: Add the gate to the repository check**

Append `bash scripts/test_public_surface.sh` to `scripts/check.sh` after `git diff --check` so source and release CI enforce the same public surface.

- [ ] **Step 5: Run GREEN verification**

Run: `bash scripts/test_public_surface.sh && bash scripts/check.sh`

Expected: both exit 0.

- [ ] **Step 6: Commit**

Run:

```bash
git add -- README.md CONTRIBUTING.md CODE_OF_CONDUCT.md SUPPORT.md docs/quickstart.md docs/data-handling.md .github/ISSUE_TEMPLATE/bug_report.yml .github/ISSUE_TEMPLATE/runtime_case.yml .github/ISSUE_TEMPLATE/config.yml .github/pull_request_template.md scripts/test_public_surface.sh scripts/check.sh
git commit -m "docs: make ARP ready for public adoption"
```

### Task 2: Reproducible stale-runtime demonstration

**Files:**
- Create: `scripts/test_stale_runtime_demo.sh`
- Create: `scripts/demo_stale_runtime.sh`
- Create: `testdata/demo-helper/main.go`
- Create: `docs/demo.md`
- Modify: `README.md`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: `agent-runtime-proof verify`, native expectations, `--known-prior-digest`, and task-owned helper processes.
- Produces: `scripts/demo_stale_runtime.sh [--json]`, whose default output is a human-readable before/replace/verdict transcript and whose JSON mode emits the final Proof.

- [ ] **Step 1: Write the failing demo test**

Create `scripts/test_stale_runtime_demo.sh`. It creates a temporary directory with a cleanup trap, runs `scripts/demo_stale_runtime.sh --json`, parses the returned Proof with Python, and asserts that the verdict is a determinate non-match demonstrating runtime/file drift, that reason codes include the existing stale/replacement reason emitted by the evaluator, and that no absolute temporary path, argv, or environment value appears in the JSON.

- [ ] **Step 2: Run the test and verify RED**

Run: `bash scripts/test_stale_runtime_demo.sh`

Expected: non-zero because `scripts/demo_stale_runtime.sh` does not exist.

- [ ] **Step 3: Implement the minimal deterministic demo**

Implement a small Go helper that blocks until terminated and embeds a build marker. The demo builds two distinct helper binaries, starts the first, atomically replaces its on-disk path with the second, constructs a bounded expectation for the replacement, and invokes ARP with the prior loaded digest. It must never use a source checkout binary as the inspected subject, elevate privileges, or leave the helper running.

- [ ] **Step 4: Document the exact expected story**

Add `docs/demo.md` with copy-paste commands, expected verdict/reason meanings, limitations per supported OS, and cleanup behavior. Link it from the README immediately below the positioning statement.

- [ ] **Step 5: Run GREEN verification**

Run: `bash scripts/test_stale_runtime_demo.sh && bash scripts/demo_stale_runtime.sh >/tmp/arp-demo-output && test -s /tmp/arp-demo-output && rm /tmp/arp-demo-output`

Expected: exit 0, a determinate stale/replacement verdict, and no residual helper process or task directory.

- [ ] **Step 6: Add to source gates and commit**

Add the demo test to `scripts/check.sh`, then run `bash scripts/check.sh` and commit only the listed files with message `feat: add reproducible stale runtime demo`.

### Task 3: MCP discovery metadata and prompt-selection tests

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Create: `testdata/mcp/golden-prompts.json`
- Create: `scripts/verify_mcp_metadata.sh`
- Modify: `plugin/agent-runtime-proof/.codex-plugin/plugin.json`
- Modify: `plugin/agent-runtime-proof/skills/agent-runtime-proof/SKILL.md`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: the existing three MCP tools and official Go SDK tool annotations.
- Produces: three tools whose annotations are `readOnlyHint=true`, `destructiveHint=false`, and `openWorldHint=false`, plus a golden prompt set with `call`, `clarify`, and `do_not_call` expectations.

- [ ] **Step 1: Add failing Go assertions for annotations**

Extend `internal/mcpserver/server_test.go` to inspect all three registered tools and assert the exact annotation values and descriptions that begin with an explicit use case and state their non-use boundary.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/mcpserver -run 'Test.*Tool.*Annotation' -count=1`

Expected: failure because annotations or boundary descriptions are absent.

- [ ] **Step 3: Implement minimal annotations and descriptions**

Use the SDK annotation fields without changing input schemas, output schemas, tool names, verdict handling, or server transport.

- [ ] **Step 4: Add a failing metadata fixture gate**

Create `testdata/mcp/golden-prompts.json` with at least five direct/indirect positive prompts and five negative prompts. Create `scripts/verify_mcp_metadata.sh` to validate the fixture shape, unique IDs, allowed expected actions, plugin version equality, and the three tool names found in source.

- [ ] **Step 5: Run RED, complete metadata, and run GREEN**

Run the script before completing the fixture to observe the expected failure, then complete the fixture and run:

`go test ./internal/mcpserver -count=1 && bash scripts/verify_mcp_metadata.sh`

Expected: exit 0.

- [ ] **Step 6: Add to checks and commit**

Add `bash scripts/verify_mcp_metadata.sh` to `scripts/check.sh`, run the full check, and commit with message `feat: publish precise MCP discovery metadata`.

### Task 4: MCPB packaging and Registry metadata

**Files:**
- Create: `packaging/mcpb/manifest.json.in`
- Create: `packaging/mcp-registry/server.json.in`
- Create: `scripts/build_mcpb.py`
- Create: `scripts/test_mcpb_packaging.sh`
- Create: `scripts/verify_registry_metadata.py`
- Modify: `scripts/build_release_assets.sh`
- Modify: `scripts/verify_release_assets.sh`
- Modify: `scripts/test_release_assets.sh`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: release binaries created by `scripts/build_release_assets.sh`, version from `VERSION`, and commit from Git.
- Produces: `agent-runtime-proof_<version>.mcpb` and generated `dist/server.json` with `registryType=mcpb`, the public release URL, SHA-256, version, `stdio` transport, repository metadata, and `io.github.fantasyce/agent-runtime-proof` name.

- [ ] **Step 1: Write the failing packaging test**

Create `scripts/test_mcpb_packaging.sh`. Build release assets into a temporary directory, require the `.mcpb` and `server.json`, inspect the ZIP with Python, validate manifest version `0.3`, tool names, Apache-2.0 license, supported platforms, no absolute paths, no source/build caches, and exact contained binary digests. Validate that Registry `fileSha256` equals the MCPB digest.

- [ ] **Step 2: Run the test and verify RED**

Run: `bash scripts/test_mcpb_packaging.sh`

Expected: failure because the MCPB asset is absent.

- [ ] **Step 3: Implement deterministic MCPB building**

Implement `scripts/build_mcpb.py` with Python standard library only. It receives `--dist`, `--version`, and `--commit`, copies the three prebuilt native binaries into `server/`, writes a sorted compact manifest from the template, preserves Unix executable modes in ZIP metadata, sets every ZIP timestamp to `1980-01-01T00:00:00`, rejects unexpected targets, and writes the archive atomically.

- [ ] **Step 4: Implement Registry generation and validation**

Generate `dist/server.json` after MCPB creation. `scripts/verify_registry_metadata.py` validates the official schema URL, namespace, title, version, release URL, SHA-256, `stdio` transport, and public repository URL. The published `server.json` copied at repository root must use the final v1.1.0 URL/digest only after release asset generation; CI publishes the generated dist copy.

- [ ] **Step 5: Extend release verification**

Add the MCPB and `server.json` to the expected asset list, secret/path/cache scans, deterministic rebuild comparison, and native installed-MCP smoke path. Do not weaken existing archive, SBOM, or source verification.

- [ ] **Step 6: Run GREEN verification**

Run: `bash scripts/test_mcpb_packaging.sh && bash scripts/test_release_assets.sh`

Expected: exit 0.

- [ ] **Step 7: Add to checks and commit**

Add the focused MCPB metadata check to `scripts/check.sh`, run the full check, and commit with message `feat: package ARP for the MCP Registry`.

### Task 5: Landing page and launch kit

**Files:**
- Create: `site/index.html`
- Create: `site/styles.css`
- Create: `site/arp-runtime-proof.svg`
- Create: `site/404.html`
- Create: `scripts/test_site.sh`
- Create: `.github/workflows/pages.yml`
- Create: `docs/launch/launch-article.md`
- Create: `docs/launch/community-posts.md`
- Create: `docs/launch/maintainer-outreach.md`
- Create: `docs/launch/faq.md`
- Create: `docs/launch/showcase-submission.md`
- Create: `docs/launch/launch-manifest.json`
- Modify: `scripts/check.sh`

**Interfaces:**
- Consumes: positioning, quickstart, demo, release, security, and repository URLs.
- Produces: a dependency-free GitHub Pages site and a launch manifest mapping each external channel to final copy, target URL, publication state, and resulting public URL.

- [ ] **Step 1: Invoke the frontend design skill and define the page direction**

Use a restrained security-tool visual system: light/dark system colors, monospace evidence details, no gradients, no fake terminal chrome, no tracking scripts, and no third-party font or JavaScript dependencies.

- [ ] **Step 2: Write the failing site test**

Create `scripts/test_site.sh` to require the positioning sentence, demo/release/security/source links, semantic landmarks, one H1, accessible SVG title/description, responsive viewport, no external scripts/fonts/trackers, no placeholder links, and successful local HTTP retrieval using a task-owned port.

- [ ] **Step 3: Run the test and verify RED**

Run: `bash scripts/test_site.sh`

Expected: failure because `site/index.html` does not exist.

- [ ] **Step 4: Implement the site and Pages workflow**

Build the static files and a pinned-actions Pages workflow that deploys only from `main`. The site must remain useful with CSS disabled and must not claim certification or adoption.

- [ ] **Step 5: Write channel-specific launch assets**

Create English launch copy for Show HN, OpenAI Developer Forum, MCP community, Reddit, X, and LinkedIn; concise Chinese copy; maintainer outreach; FAQ; and Showcase submission. Each version leads with the stale-process problem, links to the reproducible demo, and uses wording appropriate to the channel rather than duplicate spam.

- [ ] **Step 6: Run GREEN verification and commit**

Run: `bash scripts/test_site.sh && bash scripts/test_public_surface.sh && git diff --check`, add the site test to `scripts/check.sh`, run the full check, and commit with message `docs: add ARP launch site and campaign kit`.

### Task 6: v1.1.0 metadata and release automation

**Files:**
- Modify: `VERSION`
- Modify: `cmd/agent-runtime-proof/main.go`
- Modify: `plugin/agent-runtime-proof/.codex-plugin/plugin.json`
- Modify: `CHANGELOG.md`
- Modify: `README.md`
- Modify: `docs/install.md`
- Modify: `.github/workflows/release.yml`
- Create: `.github/workflows/publish-mcp.yml`
- Modify: `scripts/verify_release_metadata.sh`
- Modify: `scripts/verify_release_workflow.sh`

**Interfaces:**
- Consumes: all gates and assets from Tasks 1-5.
- Produces: version-consistent v1.1.0 source and a tag workflow that publishes GitHub assets before Registry metadata, with least-privileged GitHub OIDC authentication.

- [ ] **Step 1: Add failing release-workflow assertions**

Extend `scripts/verify_release_metadata.sh` and `scripts/verify_release_workflow.sh` to require version `1.1.0`, MCPB/server assets, a Registry publish job dependent on successful release publication, `id-token: write`, pinned tool acquisition with checksum verification, and no repository secret token for Registry authentication.

- [ ] **Step 2: Run focused gates and verify RED**

Run: `bash scripts/verify_release_metadata.sh && bash scripts/verify_release_workflow.sh`

Expected: non-zero because source remains v1.0.1 and no Registry workflow exists.

- [ ] **Step 3: Update version and release notes**

Set `VERSION`, development version, plugin version, install examples, release links, and changelog to 1.1.0. Describe distribution and onboarding changes without claiming external adoption.

- [ ] **Step 4: Implement the Registry workflow**

Use a separate workflow triggered after the Release workflow succeeds for tag `v1.1.0`, check out the exact tag, download the public MCPB asset, regenerate and validate `server.json`, authenticate with GitHub OIDC, publish with the official publisher, and query the public Registry API for exact name/version. Pin downloaded publisher version and verify its published checksum before execution.

- [ ] **Step 5: Run GREEN verification**

Run: `bash scripts/verify_release_metadata.sh && bash scripts/verify_release_workflow.sh && bash scripts/check.sh && bash scripts/test_release_assets.sh`

Expected: exit 0.

- [ ] **Step 6: Commit**

Commit only the listed version/release files with message `release: prepare ARP v1.1.0`.

### Task 7: Review, merge, tag, publish, and post-publication verification

**Files:**
- Modify during evidence update only: `docs/launch/launch-manifest.json`

**Interfaces:**
- Consumes: clean feature branch, complete checks, GitHub Actions, Release assets, Registry API.
- Produces: merged `main`, tag `v1.1.0`, verified GitHub Release, verified Registry listing, and a clean local checkout.

- [ ] **Step 1: Run the complete pre-PR gate**

Run all of:

```bash
bash scripts/check.sh
bash scripts/verify_release_metadata.sh
bash scripts/verify_host_profiles.sh
bash scripts/verify_release_workflow.sh
bash scripts/test_install_lifecycle.sh
bash scripts/test_release_assets.sh
```

Expected: every command exits 0 with no unexplained skips.

- [ ] **Step 2: Inspect and publish the branch**

Review `git status`, `git diff origin/main...HEAD`, commit history, and secret/path scans. Push the branch and open a non-draft PR because the user explicitly authorized formal release. Wait for required GitHub checks and inspect failures rather than bypassing them.

- [ ] **Step 3: Merge and restore clean main**

Merge only after checks pass, delete the remote feature branch, switch local checkout to `main`, fast-forward from origin, and verify `HEAD == origin/main` with no tracked or ignored residue.

- [ ] **Step 4: Tag and wait for publication**

Create and push signed or annotated `v1.1.0` at the exact `origin/main` commit. Wait for Release and Registry workflows to finish and inspect their logs.

- [ ] **Step 5: Verify public artifacts independently**

Download Release assets into a fresh temporary directory, verify `SHA256SUMS`, GitHub artifact attestations, SBOM shape, MCPB digest/content, native Darwin binary version/doctor/MCP discovery, and public Registry API exact version. Remove the directory afterward.

- [ ] **Step 6: Verify Pages and repository metadata**

Set repository description, homepage, and topics; enable Discussions; verify the Pages URL, release URL, security policy, and source links over HTTPS.

### Task 8: Public launch and design-partner outreach

**Files:**
- Modify: `docs/launch/launch-manifest.json`

**Interfaces:**
- Consumes: verified public v1.1.0 URLs and prepared channel copy.
- Produces: public launch URLs or explicit platform-specific blocked reasons, plus a GitHub Discussion and initial public issues.

- [ ] **Step 1: Create GitHub community launch surfaces**

Create a public launch Discussion, two `good first issue` items, one host-integration request, and a design-partner invitation. Link each to the verified release and demo.

- [ ] **Step 2: Publish authenticated community posts**

Publish channel-tailored posts only where an authenticated session exists and no new legal/identity/permission acceptance is required. Record the resulting public URL immediately in `docs/launch/launch-manifest.json`.

- [ ] **Step 3: Submit eligible showcases/directories**

Submit the OpenAI Developer Showcase and other compatible public listings when forms require only factual project data. Stop before identity verification, legal acceptance, MFA, or broad new app authorization and record the precise blocker.

- [ ] **Step 4: Send bounded maintainer outreach**

Contact a small first cohort of relevant maintainers through public issue/discussion/email channels that explicitly permit project proposals. Personalize each message to the host/server and request a reproducible test or integration review, not stars.

- [ ] **Step 5: Commit the publication ledger**

Validate that every channel entry has one of `published`, `submitted`, `blocked`, or `not_applicable`, with timestamp, target, and public URL or reason. Commit with message `docs: record ARP v1.1.0 launch` and merge through the same checked PR path if this occurs after the release merge.

- [ ] **Step 6: Final cleanliness and evidence review**

Verify local `main` is synchronized and clean, remote short-lived branches are deleted, only `main` remains where repository policy requires it, temporary files/processes are absent, and public release/Registry/Pages/community URLs still resolve. Report exact published actions and exact blocked actions separately.
