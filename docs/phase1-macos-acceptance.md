# Phase 1A macOS-Primary Acceptance

- Date: 2026-08-24
- Candidate source commit: `56d29ee3de306050cc8669ff4cdb5102836fb58d`
- Branch: `codex/phase1-macos-core`
- Decision: **PASS for the frozen Phase 1A scope**
- Full Phase 1 / v1 decision: **NOT COMPLETE**

## Scope Accepted

Phase 1A is the read-only CLI core: embedded contracts, expectation loading,
bounded artifact hashing, native process observation, privacy-safe Proofs,
`inspect`, explicit-PID `verify`, and `doctor`. It excludes MCP, Witness, host
Profiles, persistence, network listeners, repair actions, Agent configuration
mutation, and Windows live acceptance.

## Environment

- macOS 26.6.2 (25G83), Darwin 25.6.0, Apple arm64
- Go 1.26.7 and Go 1.27.0
- Docker Desktop engine 29.7.2, Linux arm64
- Ubuntu 24.04 arm64 local image digest:
  `sha256:446f072af4cce90a0c5cbbf49c22325ecd98b4754211007875660fcf12049db2`

## Automated Evidence

The complete gate passed with both supported Go toolchains:

```bash
PATH=/opt/homebrew/opt/go@1.26/bin:$PATH bash scripts/check.sh
bash scripts/check.sh
```

The current suite contains 92 passing tests in 11 tested packages with zero
test skips. The gate includes formatting, vet, module verification, a trimmed
build, race-enabled tests, and whitespace validation.

The installed-style macOS journey passed from a task-owned prefix:

```bash
bash scripts/run_phase1_acceptance.sh
```

It exercised a real controlled native process and verified:

- `MATCHED` with an `ARTIFACT_OBSERVED` Proof;
- executable-to-declared-entrypoint binding;
- an old loaded executable atomically replaced by new expected bytes returning
  `UNKNOWN/POSSIBLE_STALE_AFTER_REPLACEMENT`, never a false `MATCHED`;
- `LEAKED` outside the allowed runtime root;
- `NOT_RUNNING` for a missing process;
- scan-limit `UNKNOWN` without a partial match;
- declared-tree `UNKNOWN` while argument binding is unavailable;
- Unicode and fake-secret paths without path/secret projection;
- explicit-PID inspection, doctor, exit codes, trimmed binary path scan;
- PID identity revalidation and task-owned process/directory cleanup.

The final recorded macOS `MATCHED` Proof ID for this candidate was
`sha256:4282630f28aedc09565c705502733ab969a1e8fb427bbb4794c38b532bb31527`.

The Linux container journey passed on the ARM64 container-local runtime:

```bash
ARP_LINUX_IMAGE=ubuntu-local:24.04-arm64 \
ARP_LINUX_ARCH=arm64 bash scripts/run_linux_acceptance.sh
```

It exercised real `/proc` observation, native `MATCHED`, old-loaded/new-disk
replacement rejection, privacy projection, a running executable deleted from disk, CLI output, and cleanup. Linux process
cancellation and current-user inventory are additionally covered by live
container package tests.

Windows amd64 source and CLI/process-adapter cross-compilation passed:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath ./cmd/agent-runtime-proof
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/process
```

The resulting files were identified as PE32+ x86-64 binaries, and the CLI
binary contained no developer home or repository source path. This is compile evidence only.

## Security and Residue

- Public JSON omits argv, environment values, local absolute paths, and fake
  secret-bearing path segments.
- No administrator/root privilege, network access, Agent configuration, Across
  state, proxy, DNS, route, firewall, VPN, or network extension was changed.
- Acceptance used only task-owned temporary directories and controlled helper
  processes. Exact residue scans found no Phase 1A test directories afterward.

## Review Evidence

A read-only review of the complete implementation found and blocked three
false-`MATCHED` classes: loaded-image replacement, unobserved declared
arguments, and path/scan TOCTOU. The final candidate binds the loaded native
image identity to the hashed entrypoint on macOS/Linux, returns `UNKNOWN` for
unobserved argument bindings, pins Unix traversal to no-follow directory
handles with a final identity/change-time barrier, and disables Windows
artifact reads until an equivalent reparse-safe walker exists. A second review
of commit `56d29ee3de306050cc8669ff4cdb5102836fb58d` reported no remaining Critical
or Important findings and a merge verdict of **Yes**.

## Decision Boundary

Phase 1A is accepted because every mandatory item in its frozen macOS-primary
scope passed on the tested source. It does not authorize a public v1 release or
claim that full Phase 1 is complete. Windows live acceptance and the other
recorded gaps remain explicit in
[phase1-deferred-gates.md](issues/phase1-deferred-gates.md).
