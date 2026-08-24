# Phase 1 Windows Core Acceptance

- Date: 2026-08-25
- Candidate source commit: `43a6990538ef4207395b938b7424daa1c5802eca`
- Branch: `main`
- Decision: **PASS for the Phase 1 Windows core gate**
- Full Phase 1 / v1 decision: **NOT COMPLETE**

## Scope Accepted

This acceptance covers the Windows 11 amd64 read-only CLI core: native process
identity, handle-relative local artifact traversal, file and directory hashing,
junction/reparse-point rejection, final change detection, `verify`, `doctor`,
denied artifact access, and an installed candidate package. It does not accept
MCP, Witness, host Profiles, real Agent hosts, a public release, or a remotely
executed CI matrix.

## Environment

- Microsoft Windows 11 Home, version 10.0.26200, build 26200, amd64
- Go 1.27.0 windows/amd64 from the official portable archive
- Task-owned toolchain, build cache, source extraction, binary, fixtures, and
  helper process under one marked temporary root
- Passwordless public-key SSH restricted to the originating Mac was used only
  as the transport for the acceptance run; ARP opened no network listener

## Implementation Evidence

Windows artifact traversal now opens the local drive root without following a
reparse point, then opens every path component relative to its already-open
parent handle. Each opened handle is checked for a reparse-point attribute.
Files are bound to the volume and 128-bit file identity returned by
`FileIdInfo`; process observation uses the same identity representation.

The walker rechecks handle identity, size, modification/change metadata, and
the final selected-file content digest. The content pass is required because
Windows metadata timestamps may coalesce during rapid same-size rewrites.
Permission denial, cancellation, scan limits, races, and unsupported roots
remain explicit non-passing results. UNC roots are deliberately outside this
local-drive Phase 1 boundary.

## Automated Evidence

On macOS, the complete race-enabled suite and Windows cross-build gates passed:

```bash
go vet ./...
go test -count=1 -race ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath ./cmd/agent-runtime-proof
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/artifact
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/process
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/app
```

On the Windows host, a clean source archive containing no AppleDouble entries
was unpacked beside vendored dependencies. With a fresh task-owned Go cache,
the following passed natively:

```powershell
go vet ./...
go build -trimpath -o agent-runtime-proof.exe ./cmd/agent-runtime-proof
go test -count=1 ./...
```

The Windows tests include root and nested junction rejection, mutation during
read, same-size content replacement detection, scan-limit reason preservation,
Unicode/path handling from the shared suite, 128-bit process/artifact identity,
and doctor capability reporting. Windows race instrumentation was not claimed;
the native run used `CGO_ENABLED=0`, while the full macOS suite ran with the Go
race detector.

The reusable Windows gate is
[`scripts/run_windows_phase1_acceptance.ps1`](../scripts/run_windows_phase1_acceptance.ps1).
It builds a commit-stamped binary, writes a manifest, creates and checks a
Windows amd64 ZIP plus `SHA256SUMS`, extracts it into an independent installation
directory, verifies the installed binary against the manifest, and runs all
live checks from that installed path. The Windows CI definition now runs the
complete native package suite and this acceptance script; remote CI execution
remains separately unproven because this repository has no Git remote.

## Live CLI Evidence

A task-owned native helper was built, started, observed, verified, and removed.
The final installed-candidate positive run returned:

- exit code `0`;
- verdict `MATCHED`;
- proof level `ARTIFACT_OBSERVED`;
- reason `MATCH_CONFIRMED`;
- Proof ID
  `sha256:ad89d203bf200d218f5711613b79c4f18cd38a66b768ffcf8e5ba3a726ecef8d`.

The candidate asset SHA-256 was
`16b99f368e2f015dbab58c340285028f2964a883c4ffa0250129d2dcb31dc43a`;
the installed binary SHA-256 was
`417a1af83eecc2119cbe98f064bf217713b189c5f684e0bfc28ba146a1ddfc25`.

The same live process with an intentionally wrong expected digest returned exit
code `3`, verdict `UNKNOWN`, and reason
`POSSIBLE_STALE_AFTER_REPLACEMENT`; it never returned a false `MATCHED`.
`doctor --format json` returned status `ok` and advertised
`read-only-artifact-digest`, `embedded-contracts`, and `process-observation`.

For the denied-access case, the acceptance script applied an explicit
read-data denial only to its own running helper. The installed CLI returned
exit code `3`, verdict `UNKNOWN`, and reason `ARTIFACT_INACCESSIBLE`. The script
then removed that deny entry and required the same helper to return `MATCHED`
again before cleanup, proving both the safe result and ACL restoration.

## Security and Residue

- No administrator privilege was requested.
- No Windows firewall, SSH, account, service, registry, PATH, Agent
  configuration, or persistent system setting was changed by the implementation
  or acceptance.
- Artifact inspection requested only read-data/read-attributes access and
  shared existing read/write/delete access; it did not write inspected roots.
- The helper was terminated using its captured PID plus creation time. Final
  cleanup and an exact residue/security recheck are recorded in the task handoff.

## Remaining Gates

This evidence closes both recorded Windows Phase 1 core gaps: the safe walker
and the remaining denied-access/installed-candidate gate. Full Phase 1 and v1
are still incomplete because cross-platform core semantics, native Linux amd64,
remote CI, MCP/Witness, real Agent hosts, and public release gates remain
separate. The authoritative list is
[phase1-deferred-gates.md](issues/phase1-deferred-gates.md).
