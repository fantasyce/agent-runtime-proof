# Phase 3 Witness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a byte-transparent local Witness, content-addressed launch receipts, owned process-tree lifecycle management, and the public `PrepareLaunch -> Spawned` embedding interface on macOS, Linux, and Windows.

**Architecture:** `internal/witness` owns preparation, receipt construction/storage, byte proxying, and a small platform supervisor abstraction. `sdk/witness` exposes the embedding API without exposing internal filesystem paths or command text, while the CLI composes the same controller for `witness -- COMMAND`. Unix starts a dedicated process group and escalates TERM to KILL; Windows creates the child in a kill-on-close Job Object at process creation so descendants cannot escape through the assignment race.

**Tech Stack:** Go 1.26 baseline, standard `os/exec` and syscall APIs, `golang.org/x/sys/windows`, embedded JSON Schema, existing canonical JSON/artifact/expectation/process packages.

**Spec:** `docs/architecture-development-acceptance.md` sections 5, 8, 11, 12, 15-16, 18 Phase 3, and 19.2-19.6.

## Global Constraints

- Work only in the independent `agent-runtime-proof` repository.
- Witness parses direct argv and never invokes a shell.
- Witness stdout contains only child stdout bytes; ARP diagnostics use stderr.
- Stored receipts never contain raw environment values, complete command lines, credentials, transcripts, file contents, or unredacted home paths.
- Process evidence is bound to PID plus creation time; races and denied evidence are errors, never passes.
- Only ARP-owned state under `AGENT_RUNTIME_PROOF_HOME` or the documented default may be written.
- Tests use task-owned roots and prove process/filesystem cleanup.
- Windows cross-build is continuous; Windows 11 native acceptance runs only at the final Phase 3 gate.

---

### Task 1: Freeze the launch-receipt contract and atomic store

**Files:**
- Create: `schemas/agent-runtime-launch-receipt-1.0.schema.json`
- Modify: `schemas/embed.go`
- Create: `sdk/model/receipt.go`
- Create: `internal/receipt/receipt.go`
- Create: `internal/receipt/receipt_test.go`
- Create: `internal/store/receipt.go`
- Create: `internal/store/receipt_test.go`
- Modify: `internal/contract/validator.go`
- Modify: `internal/contract/validator_test.go`
- Modify: `docs/architecture-development-acceptance.md`

**Interfaces:**
- Produces: `sdk/model.LaunchReceipt`, `receipt.Build(receipt.Input) (sdkmodel.LaunchReceipt, error)`, `receipt.Validate([]byte) (sdkmodel.LaunchReceipt, error)`, and `store.WriteReceipt(root string, value sdkmodel.LaunchReceipt) (string, error)`.
- Receipt fields are version, content ID, UTC timestamp, safe tool/platform/process projections, command basename/path hash/positional argument hashes, optional expectation/artifact projections, observation-only marker, reason codes, and privacy projection.

- [x] **Step 1: Write failing schema and semantic tests** using literal valid/invalid JSON: ID recomputation succeeds, a protected-field mutation fails, unknown fields fail, raw argv/environment/path fields fail, and an observation-only receipt requires `WITNESS_EXPECTATION_MISSING`.
- [x] **Step 2: Run `go test ./internal/contract ./internal/receipt`** and verify failure because the schema and receipt API do not exist.
- [x] **Step 3: Add the embedded schema, model, canonical ID builder, and validator**; calculate `receipt_id` from canonical JSON with an empty ID, just as Proof IDs are calculated.
- [x] **Step 4: Run the focused tests** and verify they pass.
- [x] **Step 5: Write failing store tests** proving a filename of `<receipt_id-without-prefix>.json`, same-content idempotence, existing conflicting bytes rejection, no caller-controlled filename, and no temporary-file residue after success or failure.
- [x] **Step 6: Run `go test ./internal/store`** and verify failure because `WriteReceipt` does not exist.
- [x] **Step 7: Implement bounded mkdir, same-directory temporary write, file sync, close, atomic no-replace publication, and directory sync where supported.** Never overwrite an existing different receipt.
- [x] **Step 8: Run `go test ./internal/contract ./internal/receipt ./internal/store`** and verify all pass.
- [x] **Step 9: Update the authoritative design** with the exact receipt schema and observation-only semantics, then commit `feat: define witness launch receipts`.

### Task 2: Implement preparation and the public embedding interface

**Files:**
- Create: `internal/witness/prepare.go`
- Create: `internal/witness/prepare_test.go`
- Create: `sdk/witness/witness.go`
- Create: `sdk/witness/witness_test.go`

**Interfaces:**
- Produces: `sdk/witness.Controller`, `sdk/witness.Request{Command []string, ExpectationPath string, Home string}`, `Controller.PrepareLaunch(context.Context, Request) (*PreparedLaunch, error)`, and `PreparedLaunch.Spawned(context.Context, pid int) (sdkmodel.LaunchReceipt, error)`; `Spawned` obtains and revalidates PID-plus-creation-time evidence through the controller's observer.
- `PreparedLaunch.Command() (string, []string)` returns a defensive copy of the already-resolved direct executable and arguments for a host launcher.

- [ ] **Step 1: Write failing preparation tests** proving empty/NUL command rejection, no shell expansion, PATH resolution to an executable, hashed positional arguments, expectation load/digest before spawn, expected artifact mismatch rejection, native/interpreter entrypoint binding, and no raw secret-bearing argv in JSON or errors.
- [ ] **Step 2: Run `go test ./internal/witness ./sdk/witness`** and verify failure because the APIs are absent.
- [ ] **Step 3: Implement preparation** by composing existing expectation and artifact services, resolving the executable once, checking declared entrypoint/argument fingerprints, and retaining raw argv only in the non-serializable in-memory prepared value.
- [ ] **Step 4: Implement `Spawned`** to require a complete PID/creation-time candidate, build one receipt, and publish it under the resolved ARP home.
- [ ] **Step 5: Run focused tests** and verify pass, including `go test -race ./internal/witness ./sdk/witness`.
- [ ] **Step 6: Commit `feat: add witness embedding interface`**.

### Task 3: Implement owned cross-platform process-tree lifecycle

**Files:**
- Create: `internal/witness/supervisor.go`
- Create: `internal/witness/supervisor_unix.go`
- Create: `internal/witness/supervisor_unix_test.go`
- Create: `internal/witness/supervisor_windows.go`
- Create: `internal/witness/supervisor_windows_test.go`
- Create: `internal/witness/testhelper_test.go`

**Interfaces:**
- Produces: `newSupervisor(command string, args []string, streams Streams) (supervisor, error)` with `Start()`, `PID()`, `Wait()`, `GracefulStop()`, and `ForceStop()`; the implementation owns all process handles until `Wait` completes.

- [ ] **Step 1: Write failing Unix lifecycle tests** proving a dedicated process group, normal and non-zero exit propagation, TERM forwarding, TERM-ignore escalation to KILL after a literal short grace period, and descendant disappearance after every terminal path.
- [ ] **Step 2: Run the focused Unix tests** and verify failure because the supervisor is absent.
- [ ] **Step 3: Implement Unix supervision** with `Setpgid`, negative-PGID signaling, idempotent stop operations, mandatory wait/reap, and no signal outside the owned group.
- [ ] **Step 4: Run the Unix lifecycle tests with `-race -count=5`** and verify pass without leaked helpers.
- [ ] **Step 5: Write Windows tests** for command-line fidelity, inheritable stdio handle scoping, Job creation with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, creation-time job membership, explicit tree termination, and handle cleanup.
- [ ] **Step 6: Implement Windows 11 supervision** with an unnamed Job Object passed through `PROC_THREAD_ATTRIBUTE_JOB_LIST` at `CreateProcessW` time, `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, scoped inheritable stdio handles, no breakaway flag, and process-tree termination through the owned Job.
- [ ] **Step 7: Run `GOOS=windows GOARCH=amd64 go test -c ./internal/witness` and `GOOS=windows GOARCH=arm64 go test -c ./internal/witness`** into a task-owned temporary directory; remove it after verification.
- [ ] **Step 8: Commit `feat: supervise witness process trees`**.

### Task 4: Add the transparent proxy and CLI command

**Files:**
- Create: `internal/witness/run.go`
- Create: `internal/witness/run_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `cmd/agent-runtime-proof/main.go`

**Interfaces:**
- Produces: `witness.Run(context.Context, RunRequest) Result`; `Result` carries the child exit code and a safe receipt ID, never protocol bytes.
- Extends CLI with `agent-runtime-proof witness [--expectation FILE] [--grace-period DURATION] -- COMMAND [ARG...]`.

- [ ] **Step 1: Write failing runner integration tests** comparing binary stdout byte-for-byte with direct execution, passing child stderr unchanged only to parent stderr, writing no ARP diagnostic to stdout, closing child stdin on parent EOF, preserving non-zero exit status, and canceling/escalating without descendants.
- [ ] **Step 2: Run `go test ./internal/witness`** and verify the runner tests fail because `Run` is absent.
- [ ] **Step 3: Implement the proxy** with direct readers/writers, explicit stdin copy completion, receipt publication immediately after process identity observation, signal subscription only for Witness mode, bounded graceful shutdown, and guaranteed wait/cleanup.
- [ ] **Step 4: Run focused runner tests with `-race -count=5`** and verify pass.
- [ ] **Step 5: Write failing CLI tests** for `--` parsing, missing command, invalid expectation/grace period, exact child exit codes, protocol-pure stdout, and sanitized ARP-owned diagnostics.
- [ ] **Step 6: Run `go test ./internal/cli`** and verify expected failure.
- [ ] **Step 7: Wire the CLI and main composition** without extending the read-only application service interface or changing MCP behavior.
- [ ] **Step 8: Run `go test ./internal/cli ./internal/witness ./internal/mcpserver`** and verify pass.
- [ ] **Step 9: Commit `feat: add byte-transparent witness command`**.

### Task 5: Close macOS/Linux acceptance, then final Windows acceptance

**Files:**
- Create: `testdata/witness-helper/main.go`
- Create: `scripts/run_phase3_acceptance.sh`
- Create: `scripts/run_windows_phase3_acceptance.ps1`
- Create: `docs/phase3-acceptance.md`
- Modify: `README.md`

**Interfaces:**
- Produces repeatable, task-owned acceptance entrypoints and a dated evidence matrix that distinguishes source tests, cross-builds, installed binaries, live native processes, and deferred/completed Windows real-machine results.

- [ ] **Step 1: Write the helper and acceptance assertions first** for random binary payload transparency, real MCP direct-vs-Witness output, stderr separation, normal/non-zero exit, EOF, cancellation, parent kill, escalation, receipt ID validation, secret non-disclosure, and zero processes/files after cleanup.
- [ ] **Step 2: Run the macOS script** and verify it initially fails at the first unimplemented or incorrectly wired acceptance assertion; fix only through a regression test and minimal implementation.
- [ ] **Step 3: Run `./scripts/check.sh`, the macOS installed-binary acceptance, Linux amd64 native/container acceptance, and all Windows cross-builds.** Record exact commit, toolchain, platform, time, and uncovered evidence.
- [ ] **Step 4: Keep Windows asleep until every non-Windows gate is green; then send one LAN wake packet and poll only the pinned SSH endpoint.** If wake-on-LAN is unavailable, request one manual wake without changing Windows power settings.
- [ ] **Step 5: On Windows, use the existing pinned key/known-hosts SSH path and a task-owned directory; run native build/tests plus installed Witness tests for Job tree cleanup, protocol transparency, receipts, and no residue.** Do not change firewall, SSH, accounts, services, registry, PATH, or target program state.
- [ ] **Step 6: Run the complete local suite again after any Windows-derived fix, then rerun affected macOS/Linux/Windows gates.** Every fix starts with a failing regression test.
- [ ] **Step 7: Update README and `docs/phase3-acceptance.md` with exact evidence and limitations; never mark Windows passed from cross-compilation.**
- [ ] **Step 8: Run secret/path scans and `git diff --check`; verify clean task roots and no live helper processes.**
- [ ] **Step 9: Commit `docs: accept Phase 3 witness gates`** only when every Phase 3 gate, including native Windows, passes.
