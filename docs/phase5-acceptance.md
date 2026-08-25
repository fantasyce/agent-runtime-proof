# Phase 5 Open Source Release Candidate Acceptance

- Date: 2026-08-26
- Code candidate: `2f8b985ae585dbdd99c8983e9c8b4629d12f86bc`
- Branch: `codex/phase5-release`
- Version: `1.0.0`
- Local release-candidate decision: **PASS**
- Public tag, GitHub Release, provenance, and released-host installation:
  **not yet claimed by this record**

This record approves the repository-owned v1 release candidate for integration
through a pull request. Publication remains a separate evidence layer: the
repository must be public, the tag must resolve to the exact merged
`origin/main`, the tag workflow must succeed, and the published assets and
attestations must be downloaded and reverified before v1 is called released.

## Delivered release surface

- Canonical `VERSION`, Apache-2.0 license, security policy, changelog, and
  public installation entrypoints.
- Deterministic source, macOS arm64, Linux amd64, and Windows amd64 archives.
- A target-specific CycloneDX 1.6 SBOM for each binary archive and one complete
  `SHA256SUMS` manifest.
- A tag-only GitHub workflow with immutable action revisions, Go 1.27,
  CycloneDX GoMod 1.12.0, OIDC artifact provenance, retained workflow assets,
  and GitHub Release publication.
- Isolated clean-install, same-version repair, upgrade, downgrade, uninstall,
  CLI, local `stdio` MCP, and residue tests.
- A stable `--version` interface containing the exact release version and Git
  commit injected by the builder.

## Fresh final local matrix

The complete sequence below passed on the clean candidate worktree after the
last code and acceptance-harness changes:

```text
bash scripts/check.sh
bash scripts/verify_release_metadata.sh
bash scripts/verify_release_workflow.sh
bash scripts/verify_host_profiles.sh
bash scripts/test_release_assets.sh
bash scripts/test_install_lifecycle.sh
bash scripts/run_phase1_acceptance.sh
bash scripts/run_phase3_acceptance.sh
bash scripts/run_phase4_acceptance.sh
bash scripts/run_linux_acceptance.sh
```

Observed evidence:

- Full formatting, module verification, build, vet, and race suite: PASS.
- macOS Phase 1 Proof:
  `sha256:62cc7bb334a6ca1c3ba66dbd0974f13d14048cd9b886ea18aa676336f7b8bbae`.
- macOS Phase 3 primary receipt:
  `sha256:db7b2d26eb97d2e93f64bc724ba5c35b34fafc85f332ba74664a493e15d25e60`;
  residue: `NONE`.
- macOS generic/Profile Phase 4: PASS.
- Linux amd64 CLI, live process, replacement detection, local MCP, and cleanup
  in `ubuntu:24.04`: PASS.
- Release-asset test built the complete file set twice from one commit and
  compared every byte: PASS.
- Install lifecycle and final worktree cleanliness: PASS.

Reference environment:

- macOS 26.6.2 arm64, build `25G83`;
- Go `1.27.0` darwin/arm64;
- CycloneDX GoMod `1.12.0`, module sum
  `h1:OuFUYNhnjpju7RNArOVPPchFPWNobGfhrHODDPKcgZs=`;
- Docker client/server `29.7.2`;
- Ubuntu image
  `ubuntu@sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517`.

## Official-tool candidate assets

The official CycloneDX tool built the candidate twice in a clean, locally
tagged clone. Every corresponding output was byte-identical, the exact file
allowlist passed, all checksums verified, the native binary reported version
and commit correctly, and archive scans found no secret, developer path,
cache, VCS directory, or test-data entry.

```text
d71221d325ab02576163c51fdb6f207ae407dea8e3968ffba50a8e83c4c5c949  agent-runtime-proof_1.0.0_darwin_arm64.cdx.json
9c5e6c1e533b19048f0aa96cfede21bdcebd9004cbcde213dcab370ca19d949a  agent-runtime-proof_1.0.0_darwin_arm64.tar.gz
a2edf5064da4eeca2227d61d213494fcd42ea830a5390c455144c13a309ff1dc  agent-runtime-proof_1.0.0_linux_amd64.cdx.json
e1d34458cd5e67168457e33056b8bcdc1d75262ac9ca780ebc7689849d449ed3  agent-runtime-proof_1.0.0_linux_amd64.tar.gz
ac34cf0bb5b5b08322b3ffdcc032cd52b876830c6c6314403a7a04008e7b54f3  agent-runtime-proof_1.0.0_source.tar.gz
538bf471540e68409f9b676e92a248d73883deb903bd41edefffcabb2d175a52  agent-runtime-proof_1.0.0_windows_amd64.cdx.json
0f541434c56275559a5e4740559bd923dbe5a6071dd44aeb5b2f6be4c76e2494  agent-runtime-proof_1.0.0_windows_amd64.zip
```

These hashes apply to the named code candidate. The publication workflow
rebuilds from the final merged tag commit, so the GitHub Release's own
`SHA256SUMS` is authoritative for downloads.

## Defects found and closed during Phase 5

- Archive scanning used early-exit pipelines under `pipefail`, causing an
  intermittent verifier exit despite correct bytes. Verification now extracts
  to a task-owned root and scans deterministically.
- The pre-release Profile gate required the formal plugin version and the
  source default `-dev` version to be identical. It now accepts the explicit
  development suffix while release builds still inject exact `1.0.0`.
- The existing macOS process observer treated a transient `EINVAL` or an
  exhausted executable-region walk as an internal failure, aborting a whole
  current-user inventory. Such unobservable processes are now conservatively
  classified as inaccessible, with repeated race coverage.
- The existing Linux launcher contained named `coproc` syntax that macOS Bash
  3.2 could not parse before starting Docker. It now uses portable FIFOs and
  cleans up both the MCP process and helper on every terminal path.
- The first PR run showed that release/Profile gates depended on the local
  `rg` utility, which is not part of the GitHub macOS runner contract. Public
  gates now use platform-provided `grep` and remain dependency-free.
- The second PR run showed that the new release scripts assumed macOS
  `/private/tmp`; Linux has no such directory. All release build, verification,
  asset, and lifecycle scripts now honor the system temporary-directory
  contract and retain marked-path cleanup guards.
- A later macOS 1.26 race run showed that process argument visibility can
  change while PID, creation identity, and loaded file identity remain stable.
  Arguments remain Host binding evidence, but are no longer mistaken for
  process identity during revalidation; 100 repeated race checks and the full
  macOS/Linux acceptance sequence passed after the correction.
- The Linux asset gate then showed that the verifier always tried to execute
  the Darwin archive. Native version smoke now selects Darwin arm64 on macOS
  and Linux amd64 on the Linux release runner; non-native archives continue
  through checksum, SBOM, allowlist, and content verification.
- The Linux Phase 3 gate exposed a real Witness startup signal race: a fast
  child could report readiness while receipt binding was still running, before
  the owner registered its termination handler. Signal registration now occurs
  before the child can start. The race suite passed 20 repetitions and the
  native Linux Phase 3 lifecycle passed 10 consecutive repetitions.
- A Windows run completed CLI, Proof, permission, and MCP gates but briefly
  retained an executable file handle during final deletion. Cleanup now waits
  for the exact helper identity, disposes its process handle, and retries only
  the marked task root for a bounded five seconds; residue remains a hard
  failure after that bound.
- A subsequent Linux Phase 3 run exposed a harness ordering race: the helper
  published its target PID before the Witness had atomically committed the
  matching receipt, so a slow runner could kill the owner too early. The
  owner-death case now waits for durable receipt evidence before killing the
  Witness and then proves full tree recovery.

The macOS observer and Linux launcher defects predated Phase 5 and were exposed
by the expanded final regression. The release verifier, version-gate, and
local-tool dependency defects were introduced in the new Phase 5 release
layer. All are fixed; there is no unresolved candidate defect.

## Prior platform and real-host evidence retained

Phase 3 and Phase 4 already contain native Windows 11 amd64, native Linux
amd64, macOS DeepSeek Harness, Windows Codex, and AAA generic MCP evidence.
Phase 5 did not change MCP tools, Proof contracts, Witness semantics, or the
three existing Across managed plugins. The only runtime-facing addition is the
bounded `--version` path; the macOS observer hardening preserves conservative
`UNKNOWN` behavior for unobservable processes.

The final published macOS asset must still be installed outside the checkout
and rerun through a real host and AAA. The published Windows archive must be
downloaded and natively smoke-tested before the public release audit closes.

## GO / NO-GO boundary

**GO for PR integration and tag-workflow preparation.** Do not claim public v1
publication from this document alone. Public-release GO requires all of the
following after merge: public visibility, green protected-main checks, exact
`v1.0.0 == origin/main`, successful provenance workflow, downloaded asset
verification, released-host smoke tests, AAA final generic integration, and
task-residue cleanup.
