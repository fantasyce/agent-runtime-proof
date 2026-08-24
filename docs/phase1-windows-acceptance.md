# Phase 1 Windows Core Acceptance

- Date: 2026-08-25
- Candidate source commit: `2cfc1e19a8bbe79e8261b11902aa0a4768fc04ae`
- Branch: `main`
- Decision: **PASS for the Phase 1 Windows core candidate**
- Full Phase 1 / v1 decision: **NOT COMPLETE**

## Scope Accepted

This acceptance covers the Windows 11 amd64 read-only CLI core: native process
identity, handle-relative local artifact traversal, file and directory hashing,
junction/reparse-point rejection, final change detection, `verify`, and
`doctor`. It does not accept a release asset, elevated/denied-access behavior,
MCP, Witness, host Profiles, real Agent hosts, or the public release matrix.

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

## Live CLI Evidence

A task-owned native helper was built, started, observed, verified, and removed.
The final positive run returned:

- exit code `0`;
- verdict `MATCHED`;
- proof level `ARTIFACT_OBSERVED`;
- reason `MATCH_CONFIRMED`;
- Proof ID
  `sha256:c194f865df5a332f41642e35d8a90fa557e906ce2c144e4f7a0109797886a0b3`.

The same live process with an intentionally wrong expected digest returned exit
code `3`, verdict `UNKNOWN`, and reason
`POSSIBLE_STALE_AFTER_REPLACEMENT`; it never returned a false `MATCHED`.
`doctor --format json` returned status `ok` and advertised
`read-only-artifact-digest`, `embedded-contracts`, and `process-observation`.

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

This evidence closes the Windows safe-walker implementation gap. It only
partially satisfies the wider Windows release gate: denied-access behavior on a
restricted fixture, a formally packaged/installed release asset, public Windows
CI, MCP/Witness, and real Agent-host acceptance remain open. The authoritative
list is [phase1-deferred-gates.md](issues/phase1-deferred-gates.md).
