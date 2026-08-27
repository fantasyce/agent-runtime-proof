# Agent Runtime Proof Promotion and Launch Design

Date: 2026-08-27
Status: Proposed for user review

## Objective

Turn the already released Agent Runtime Proof (ARP) v1.0.1 into a product that
an unfamiliar Agent developer can discover, understand, install, and verify in
minutes, then launch it through credible ecosystem channels without weakening
its local-only privacy boundary.

The launch succeeds when ARP has a conversion-ready public surface, an
official MCP Registry listing, a verified v1.1.0 release, and coordinated
community announcements backed by a reproducible stale-runtime demonstration.

## Product Positioning

The primary message is independent of the Across product family:

> Prove the agent runtime you launched is the artifact you approved.

The Chinese equivalent is:

> 证明正在运行的 Agent / MCP，正是你批准的那份构建。

ARP is described as a local, read-only runtime identity verifier for AI agents
and MCP servers. It detects stale, replaced, mismatched, or unverifiable
runtimes without uploading code, secrets, process arguments, or process data.

Across Agents Assistant is one validated integration, not a dependency. The
launch must not describe ARP as a fourth AAA managed plugin.

## Audience and Launch Story

The initial audience is deliberately narrow:

1. Maintainers of local MCP servers and Agent hosts.
2. Advanced users of Codex, Claude Code, Cursor, OpenCode, DeepSeek Harness,
   and VS Code/GitHub Copilot.
3. Platform and application-security engineers concerned with software supply
   chain evidence and runtime drift.

The primary demonstration reproduces one concrete failure: an old process
continues running after its executable is replaced. Checksums and release
attestations describe the downloaded file; ARP shows that the live process is
still bound to the old runtime. The demo ends with a structured ARP verdict and
does not require privileged access or modify host configuration.

## Scope

### 1. Conversion-ready repository

The README will be reorganized around this order:

1. One-sentence value proposition.
2. Reproducible stale-runtime demonstration and expected verdict.
3. Fast, verified install paths for macOS arm64, Linux amd64, and Windows
   amd64.
4. First useful CLI verification and MCP configuration.
5. Privacy and read-only guarantees.
6. Supported hosts, architecture, lifecycle, and acceptance evidence.

The repository will add:

- a deterministic, task-owned stale-runtime demo script and checked expected
  output;
- a concise quickstart document;
- `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, issue templates, pull-request
  template, and community support guidance;
- a privacy/data-handling document that accurately states the local-only
  boundary;
- a compact comparison explaining that SBOMs, signatures/attestations, and
  runtime identity answer different questions;
- GitHub topics, homepage metadata, Discussions, and initial labelled issues;
- a small static GitHub Pages landing page that links directly to installation,
  the demo, releases, security policy, and source.

Install guidance will not recommend an opaque `curl | sh` pipeline. Download,
checksum, GitHub attestation, extraction, and `doctor` remain visible and
reviewable. MCPB becomes the one-click path for clients that support it.

### 2. MCP packaging and discovery

ARP will add an MCPB v0.3 bundle containing the existing local binary MCP
server, its three declared tools, icon assets, repository/support links,
keywords, Apache-2.0 license metadata, and platform-specific binary selection.
The bundle remains a `stdio` server and opens no network listener.

Packaging will be produced from the same release commit as the existing native
archives. Tests will validate the manifest, unpack each bundle, verify the
contained binary digest and executable behavior on its native target, and run
the existing MCP smoke contract against installed bundle bytes.

The repository will include an official MCP Registry `server.json` under the
GitHub-owned namespace. Release automation will publish only after quality,
cross-platform packaging, checksum, SBOM, provenance, and registry validation
gates pass. Registry publication will use GitHub OIDC or the least-privileged
supported authentication flow; no long-lived registry token will be committed.

The MCP tool metadata will declare accurate read-only, closed-world, and
non-destructive annotations. A golden prompt set will cover direct prompts,
indirect runtime-drift prompts, and negative prompts where ARP should not be
selected.

### 3. Release

The promotion-ready release is v1.1.0. This is a minor release because it adds
a new supported distribution format and discovery surface while preserving the
v1 verdict and privacy contracts.

The release process will:

1. Run the complete repository gate and open-source scan.
2. Test the demo and clean install from release-shaped artifacts.
3. Build native archives, SBOMs, checksums, attestations, and MCPB bundles.
4. Validate installed CLI and MCP behavior on supported native targets.
5. Merge through `main`; the tag must point to `origin/main`.
6. Publish the GitHub Release and official MCP Registry record.
7. Re-download a sample of public assets and independently verify their
   checksums and GitHub attestations.
8. Remove release worktrees, temporary homes, unpack directories, recordings,
   and logs that are not intentional published artifacts.

No community announcement is posted until the public v1.1.0 assets and Registry
record have passed post-publication verification.

### 4. Promotion assets and channels

The launch kit will contain:

- a 45-60 second terminal demonstration;
- one technical long-form launch article;
- tailored English posts for Show HN, OpenAI Developer Forum, MCP community,
  Reddit, X, and LinkedIn;
- concise Chinese launch posts for the user's preferred Chinese channels;
- maintainer outreach text focused on testing, not stars;
- an FAQ covering privacy, permissions, false certainty, SBOM/signature
  complementarity, and supported hosts;
- a design-partner invitation and public GitHub Discussion.

Announcements will link to the same reproducible demo and release. They will
not claim adoption, endorsements, security certification, or host support
beyond evidence actually produced by the release matrix.

OpenAI Developer Showcase submission is in scope. OpenAI's universal Plugins
Directory is not an immediate release gate because its public MCP submission
flow requires a production HTTPS MCP URL, while ARP must inspect local state and
intentionally provides only local `stdio`. A skills-only or other compliant
directory path may be evaluated separately, but the launch will not add a
remote process-data service merely to satisfy a directory requirement.

### 5. External-action boundary

The user has authorized public promotion. The agent may therefore update public
repository metadata, enable Discussions, publish the release and MCP Registry
record, create public GitHub issues/discussions, and publish promotional posts
through already authenticated accounts when the final content and target are
unambiguous.

The agent must stop before any action that requires the user to:

- verify a personal or business identity;
- accept platform, marketplace, legal, privacy, or payment terms;
- provide credentials, MFA, recovery codes, or account ownership proof;
- choose between materially different public identities or brands;
- grant a new third-party application permission with broad account access.

For those actions, the agent prepares the complete submission and leaves only
the user-controlled confirmation. Failed or unavailable logins are reported as
unsubmitted, never as published.

## Testing and Evidence

The implementation uses test-first changes for scripts, manifests, release
composition, metadata, and demo output. The acceptance ladder is:

`source -> packaged assets -> installed binaries -> MCP process -> public release -> registry listing -> public announcement`

Evidence required before completion:

- repository tests and open-source checks pass;
- stale-runtime demo is reproducible from a clean temporary directory;
- every release asset is generated from the tagged `main` commit;
- checksums, SBOMs, and attestations verify after public download;
- native MCP tool discovery exposes exactly the expected three read-only tools;
- official Registry search returns the released name and version;
- GitHub repository metadata, Pages site, Discussion, and release links resolve;
- every attempted external post has a recorded public URL or an explicit
  blocked reason;
- all local and task-owned remote temporary artifacts are removed;
- the local repository ends clean on `main` and synchronized with
  `origin/main`.

## Adoption Measurement

ARP will not add silent telemetry. For the first 30 days, adoption will be
measured through public, privacy-preserving signals:

- GitHub unique views and clones;
- release and MCPB asset downloads;
- stars, forks, Discussions, issues, and external pull requests;
- MCP Registry availability and downstream listings;
- opt-in design-partner confirmations and linked third-party integrations.

The first milestone is ten confirmed external installations, five real runtime
verification cases, three external feedback items, and two third-party
integrations. These are goals, not claims made at launch.

## Non-goals

- No remote attestation service, daemon, network listener, or cloud account.
- No upload of local process, file, path, command-line, or credential data.
- No ARP-specific branch in AAA or any other host.
- No paid advertising or purchased engagement in the first launch.
- No broad channel spam or identical copy pasted across communities.
- No changes to ARP verdict semantics solely for marketing.

## Rollback and Corrections

Repository and packaging changes are delivered through normal Git history and
can be corrected in a patch release. A broken Registry entry is deprecated or
replaced using the Registry lifecycle API rather than silently left active.
Incorrect public posts are corrected transparently at the original URL when
the platform permits; they are not deleted merely to hide a factual mistake.
If post-publication verification fails, announcements stop and the release is
marked clearly until repaired.
