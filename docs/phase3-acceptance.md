# Phase 3 Witness Acceptance

- Date: 2026-08-25
- Accepted implementation commit: `99110378fc361c8775ebac6c5432a599c8df1453`
- Branch: `codex/phase3-witness`
- Decision: **PASS**
- Unresolved Phase 3 defects: **none**

## Accepted Surface

Phase 3 adds the optional local Witness without changing ARP's read-only host
boundary. The CLI accepts direct argv through
`agent-runtime-proof witness [--expectation FILE] -- COMMAND [ARG...]`; it does
not invoke a shell. The public `sdk/witness` package exposes the same
`PrepareLaunch -> Spawned` contract to hosts that already own child creation.

Expectation-bound launches validate the entrypoint, positional argument
fingerprints, artifact root, limits, and declared digest before spawning.
Observation-only launches remain explicitly marked with
`WITNESS_EXPECTATION_MISSING` and do not claim `MATCHED`.

Each accepted spawn produces an `agent-runtime-launch-receipt/1.0` document.
The receipt binds PID plus creation time, boot identity, safe command hashes,
optional expectation/artifact projections, platform and tool identity. Its
content ID is recomputed from canonical JSON. Raw argv, environment values,
complete executable paths, file contents, transcripts, credentials, and
unredacted home paths are absent. Storage uses a bounded same-directory
temporary file, sync, atomic no-replace publication, conflict detection, and
temporary-file cleanup.

## Lifecycle and Protocol Evidence

The reusable macOS/Linux gate is
[`scripts/run_phase3_acceptance.sh`](../scripts/run_phase3_acceptance.sh); the
native Windows gate is
[`scripts/run_windows_phase3_acceptance.ps1`](../scripts/run_windows_phase3_acceptance.ps1).
They build independent installed-style binaries and task-owned helpers.

The accepted journeys cover:

- arbitrary binary stdin/stdout bytes preserved exactly;
- real MCP initialize and tool-list responses identical byte-for-byte when
  direct and when proxied through Witness;
- child stderr preserved while ARP diagnostics never enter stdout;
- normal and non-zero child exit status preservation;
- stdin EOF grace timeout and force escalation;
- termination-signal forwarding on Unix;
- Witness parent death and complete descendant cleanup;
- SDK preparation and spawn receipt creation;
- receipt schema/content-ID validation and secret/path non-disclosure;
- no helper process, receipt, temporary file, or marked task-directory residue.

Unix launches a dedicated guardian and target process group. The guardian owns
an anonymous parent-liveness pipe and receives target argv through a separate
anonymous configuration pipe, so command text is not placed on its command
line. On Linux it is a child subreaper and reaps adopted descendants after
group termination. Windows creates the child in an unnamed Job Object at
process creation with kill-on-close semantics; it does not rely on a later,
racy Job assignment.

## Platform Evidence

### macOS 26.6.2 arm64

Go 1.27.0 ran the complete repository gate and installed Witness acceptance.
The final installed journey returned `PHASE3_ACCEPTANCE=PASS`,
`PLATFORM=darwin/arm64`, a schema-valid primary launch receipt, and
`PHASE3_RESIDUE=NONE`.

### Linux amd64 and arm64

An Ubuntu 24.04.4 LTS amd64 host ran the installed amd64 candidate natively.
Three consecutive final runs returned `PHASE3_ACCEPTANCE=PASS`,
`PLATFORM=linux/amd64`, and `PHASE3_RESIDUE=NONE`. This is native-host evidence,
not a Linux desktop support claim.

An Ubuntu 24.04 arm64 container with no network and a read-only repository also
passed the same installed gate. It confirmed the Linux subreaper fix and
reported `PLATFORM=linux/arm64`; arm64 remains build/smoke support rather than
the formal v1 Linux architecture.

An amd64 binary under Apple Silicon Docker was deliberately not accepted as
native process-image evidence: Rosetta correctly appears as
`/proc/<pid>/exe`. ARP returned an executable mismatch instead of converting an
emulated process into a false target-image proof.

### Windows 11 amd64

`DESKTOP-6OTKL7F` ran Microsoft Windows 11 Home 10.0.26200.0 and a
checksum-verified, task-owned Go 1.27.0 portable toolchain. The official Go
archive SHA-256 was
`f0c0a0d33ba94f4d2c5dbc887334ce678b21813504ddb3aafcb06e60a5a667c4`.
Dependencies were transferred as a project-owned vendor archive because the
host could not reach the Go module proxy; no proxy, DNS, firewall, or route was
changed.

The native `go test -count=1 ./...` suite passed, including Windows Job Object
tests. The installed gate then reported:

- `WINDOWS_PHASE3_ACCEPTANCE=PASS`;
- primary receipt
  `sha256:bbeb84f82a6d7abb96cd9fabd99bc5bc5b47e3ed2dd729a6ef67b9de519bc519`;
- `WINDOWS_JOB_TREE=PASS`;
- `WINDOWS_MCP_TRANSPARENCY=PASS`;
- `WINDOWS_PHASE3_RESIDUE=NONE`.

Windows acceptance used the existing pinned public-key SSH path only as a
transport. It required no administrator privilege and changed no firewall,
SSH, account, service, registry, PATH, power, Agent, or persistent system
setting. Before deleting the marked transfer root, the cleanup gate confirmed
that no process executable remained under it.

## Defects Found and Closed

Acceptance found and closed four Phase 3 issues:

1. macOS temporary paths exposed an OS symlink boundary, so the fixture moved
   to the resolved `/private/tmp` root rather than weakening artifact checks.
2. Immediate MCP stdin EOF could race server responses; the real protocol
   fixture now waits for initialize before continuing, matching client
   behavior.
3. Linux process-group termination could leave adopted zombie descendants when
   PID 1 did not reap them; the guardian now uses child-subreaper semantics and
   explicitly reaps the owned tree.
4. New Witness/store tests embedded Unix-only paths, executable naming, and
   mode-bit assertions; fixtures and permission assertions are now platform
   specific and the complete native Windows suite passes.

## Boundary

Phase 3 is complete. This is not a v1 release decision. Phase 4 host Profiles
and the complete named-host matrix, Phase 5 public release assets, SBOM,
provenance, tag/origin-main requirements, AAA generic integration finalization,
and the final section 19 GO/NO-GO remain separate gates.
