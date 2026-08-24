# Agent Runtime Proof Phase 1 macOS Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the read-only `inspect`, `verify`, and `doctor` CLI core, with real macOS process and artifact evidence, Linux Docker verification, and compile-checked Windows support while Windows live acceptance remains explicitly deferred.

**Architecture:** A small application service coordinates immutable process snapshots, expectation resolution, bounded artifact hashing, verdict evaluation, privacy projection, and Proof generation. Platform adapters implement one narrow process-observer interface; CLI formatting stays outside the domain packages. Phase 1A does not add MCP, Witness, host profiles, persistent state, or network listeners.

**Tech Stack:** Go 1.26 language baseline; standard library; `golang.org/x/sys`; JSON Schema 2020-12; SHA-256; macOS `libproc`; Linux `/proc`; Windows process APIs.

**Spec:** `docs/architecture-development-acceptance.md`

## Global Constraints

- Work only in the independent `agent-runtime-proof` repository and on `codex/phase1-macos-core`.
- All production behavior is test-first: RED, GREEN, refactor, focused commit.
- Runtime inspection is read-only and never asks for administrator or root privileges.
- Public output never contains complete argv, environment values, file contents, usernames, home-directory paths, credentials, or transcripts.
- Every process sample is bound to PID plus creation time and re-read before a verdict is emitted.
- Artifact traversal is root-bounded, rejects links and non-regular files, normalizes relative paths to NFC with `/`, and never returns a partial MATCHED result.
- Phase 1A implements no MCP, Witness, host Profile, daemon, persistence, network listener, automatic repair, or Agent configuration mutation.
- Windows source and cross-compilation are mandatory in Phase 1A; Windows live-machine acceptance remains a recorded hard gate for full Phase 1 and v1.
- Linux acceptance is container-local and does not claim Linux desktop-host coverage.
- Tests use task-owned temporary directories and processes and leave no ARP runtime state.

---

### Task 1: Executable contract model and canonical Proof digest

**Files:**
- Create: `schemas/embed.go`
- Create: `internal/model/model.go`
- Create: `internal/contract/validator.go`
- Create: `internal/canonical/canonical.go`
- Create: `internal/canonical/canonical_test.go`
- Create: `internal/proof/digest.go`
- Create: `internal/proof/digest_test.go`
- Modify: `schemas/agent-runtime-proof-1.0.schema.json`
- Modify: `contracts/privacy-registry.json`
- Modify: `internal/contracttest/privacy_test.go`

**Interfaces:**
- Produces: `contract.ValidateExpectation([]byte) error`, `contract.ValidateProof([]byte) error`, `canonical.Marshal(any) ([]byte, error)`, and `proof.AssignID(*model.Proof) error`.
- Produces domain types `Expectation`, `ProcessIdentity`, `Candidate`, `ArtifactObservation`, and `Proof` with JSON names identical to the public schemas.

- [x] **Step 1: Write failing tests for canonical object ordering, Unicode-key ordering, safe integer rejection, Proof field-tamper detection, and an observation-only Proof with absent expectation and host attribution.**
- [x] **Step 2: Run the focused tests and confirm failures are caused by missing production packages and the current schema rejecting honest observation-only output.**
- [x] **Step 3: Embed the authoritative schemas from the public `schemas` package and implement strict schema validation without filesystem lookup.**
- [x] **Step 4: Implement the restricted RFC 8785 serializer with UTF-16 property ordering, deterministic string escaping, finite safe integers, arrays, objects, and structs; reject floats and unsupported values.**
- [x] **Step 5: Implement `AssignID` by canonicalizing a copy without `proof_id`, hashing exact UTF-8 bytes, and assigning lowercase `sha256:<hex>`.**
- [x] **Step 6: Amend Proof 1.0 before runtime release so `expectation` and `host_attribution` may be null for honest explicit-PID inspection; add privacy classifications and rejection fixtures for contradictory projections.**
- [x] **Step 7: Run contract, canonical, and Proof tests; confirm golden vectors and Phase 0 compatibility still pass.**
- [x] **Step 8: Commit the executable contract model.**

### Task 2: Expectation loading and root-safe resolution

**Files:**
- Create: `internal/expectation/load.go`
- Create: `internal/expectation/load_test.go`
- Create: `internal/expectation/match.go`
- Create: `internal/expectation/match_test.go`

**Interfaces:**
- Consumes: `contract.ValidateExpectation` and `model.Expectation`.
- Produces: `expectation.Load(path string) (expectation.Resolved, error)` and `Resolved.Includes(relativeSlashPath string) bool`.
- `Resolved` contains the validated public expectation, absolute manifest path, resolved artifact root, and resolved allowed roots; it never exposes these local paths in public JSON.

- [x] **Step 1: Write failing tests for valid relative roots, absolute roots, path escape, symlink-root escape, missing file, malformed JSON, unknown fields, duplicate argument positions, and `**` include/exclude matching.**
- [x] **Step 2: Run the tests and confirm the loader API is missing.**
- [x] **Step 3: Implement bounded file reading, schema validation, duplicate-position semantic validation, root resolution relative to the expectation file, symlink evaluation, and component-aware containment checks.**
- [x] **Step 4: Implement slash-normalized `*`, `?`, character class, and recursive `**` matching; exclusion wins over inclusion.**
- [x] **Step 5: Run focused and full tests; verify local absolute paths appear only in the internal resolved value.**
- [x] **Step 6: Commit expectation resolution.**

### Task 3: Bounded artifact digester

**Files:**
- Create: `internal/artifact/digest.go`
- Create: `internal/artifact/digest_test.go`
- Create: `internal/artifact/errors.go`

**Interfaces:**
- Consumes: `expectation.Resolved`, `canonical.Marshal`, and `context.Context`.
- Produces: `artifact.Digest(ctx context.Context, resolved expectation.Resolved, clock artifact.Clock) (model.ArtifactObservation, error)`.
- Domain errors carry one registered Reason Code and never return a partial digest.

- [ ] **Step 1: Write failing tests with hand-derived digests for a file, a directory, empty selection, include/exclude behavior, spaces/Chinese/NFC paths, symlink rejection, root escape, unsupported file type, file-count limit, byte limit, cancellation, and mutation during read.**
- [ ] **Step 2: Run the focused tests and confirm failure because the digester is absent.**
- [ ] **Step 3: Implement regular-file hashing with pre/post identity, size, and modification-time comparison.**
- [ ] **Step 4: Implement directory walking without following links, normalized collision detection, deterministic entry sorting, canonical tree hashing, and strict count/byte/time limits.**
- [ ] **Step 5: Return registered domain errors for every incomplete observation and verify no error path returns an artifact hash.**
- [ ] **Step 6: Run focused tests with the race detector and then the complete gate.**
- [ ] **Step 7: Commit the artifact digester.**

### Task 4: Cross-platform process observer boundary

**Files:**
- Create: `internal/process/process.go`
- Create: `internal/process/errors.go`
- Create: `internal/process/darwin/observer.go`
- Create: `internal/process/darwin/observer_test.go`
- Create: `internal/process/linux/observer.go`
- Create: `internal/process/linux/observer_test.go`
- Create: `internal/process/windows/observer.go`
- Create: `internal/process/observer_darwin.go`
- Create: `internal/process/observer_linux.go`
- Create: `internal/process/observer_windows.go`

**Interfaces:**
- Produces: `process.Observer` with `Snapshot(context.Context, int) (model.Candidate, error)`, `List(context.Context, int) ([]model.Candidate, error)`, and `Revalidate(context.Context, model.ProcessIdentity) error`.
- Candidate exposes safe basename and hashes; absolute executable paths remain in an unexported/internal observation field used by evaluation only.

- [ ] **Step 1: Write platform-neutral contract tests using a deterministic fake observer for identity revalidation, inaccessible fields, stable ordering, cancellation, and current-user filtering.**
- [ ] **Step 2: On macOS, write failing live tests that observe the test process and a controlled helper process without exposing argv or environment.**
- [ ] **Step 3: Implement the Darwin adapter with `libproc` process listing/path/creation metadata, boot-time-derived hash, parent PID, and executable file identity; convert permission failures to typed inaccessible errors.**
- [ ] **Step 4: Implement Linux `/proc` observation and tests runnable inside an Ubuntu container, including deleted executable and permission-limited outcomes.**
- [ ] **Step 5: Implement Windows process listing, image path, creation time, and file identity using wide-character APIs; map access denial to typed inaccessible errors.**
- [ ] **Step 6: Run native macOS tests, Linux container tests, and `GOOS=windows GOARCH=amd64` compile tests.**
- [ ] **Step 7: Commit process observers.**

### Task 5: Verdict evaluation, privacy projection, and Proof generation

**Files:**
- Create: `internal/evaluator/evaluator.go`
- Create: `internal/evaluator/evaluator_test.go`
- Create: `internal/privacy/project.go`
- Create: `internal/privacy/project_test.go`
- Create: `internal/proof/build.go`
- Create: `internal/proof/build_test.go`

**Interfaces:**
- Consumes: immutable Candidate, optional resolved Expectation, optional ArtifactObservation, and registered Reason Code rules.
- Produces: `evaluator.Evaluate(input evaluator.Input) evaluator.Decision`, `privacy.Project(input proof.Input) model.Proof`, and `proof.Build(input proof.Input) (model.Proof, error)`.

- [ ] **Step 1: Write failing table tests for MATCHED, root-outside LEAKED, direct-known-old STALE, mismatch-without-loaded-byte proof UNKNOWN, NOT_RUNNING, permission UNKNOWN, identity-race UNKNOWN, untrusted expectation limitation, scan-limit UNKNOWN, and contradictory reason rejection.**
- [ ] **Step 2: Write failing privacy tests containing fake tokens, cookies, home paths, repository paths, argv, and environment values; assert none appears in Proof JSON or errors.**
- [ ] **Step 3: Implement evidence-to-verdict rules with minimum Proof Levels from the decision registry and no time-order-only STALE conclusion.**
- [ ] **Step 4: Implement safe path and argument hashing with domain separation, `$HOME` projection only for explicitly local table output, and omission lists for unavailable or prohibited fields.**
- [ ] **Step 5: Build schema-valid Proofs, revalidate process identity before finalization, assign Proof IDs, and verify digest recomputation after round trip.**
- [ ] **Step 6: Run focused tests, mutation checks, schema validation, and the complete gate.**
- [ ] **Step 7: Commit evaluator and Proof generation.**

### Task 6: Application service for inspect, verify, and doctor

**Files:**
- Create: `internal/app/service.go`
- Create: `internal/app/service_test.go`
- Create: `internal/app/doctor.go`
- Create: `internal/app/doctor_test.go`

**Interfaces:**
- Produces: `Service.Inspect(context.Context, InspectRequest) (InspectResult, error)`, `Service.Verify(context.Context, VerifyRequest) (VerifyResult, error)`, and `Service.Doctor(context.Context) DoctorResult`.
- Inspect supports explicit PID or a bounded current-user inventory; Verify requires an expectation and explicit PID in Phase 1A; binding selection returns a clear unavailable-input error until host Profiles exist.

- [ ] **Step 1: Write failing service tests for explicit PID, bounded inventory, required expectation, process disappearance, negative domain results as successful Proof output, cancellation, and severity aggregation.**
- [ ] **Step 2: Write failing doctor tests for supported platform, embedded schema health, observer availability, read-only state boundary, and no host Profile claim.**
- [ ] **Step 3: Implement orchestration with dependency injection, immutable snapshots, bounded concurrency, and identity revalidation.**
- [ ] **Step 4: Implement doctor as read-only capability reporting with no filesystem writes or network probes.**
- [ ] **Step 5: Run focused tests with race detection and then the complete gate.**
- [ ] **Step 6: Commit the application service.**

### Task 7: CLI executable and exit-code contract

**Files:**
- Create: `cmd/agent-runtime-proof/main.go`
- Create: `internal/cli/run.go`
- Create: `internal/cli/run_test.go`
- Create: `internal/cli/golden_test.go`
- Create: `testdata/cli/`
- Modify: `README.md`

**Interfaces:**
- Produces the `agent-runtime-proof` executable with `inspect`, `verify`, and `doctor` subcommands.
- `--format json` writes exactly one JSON value to stdout; diagnostics go only to stderr.
- Exit codes are 0 success/MATCHED, 2 negative verdict, 3 UNKNOWN, 64 invalid input, and 70 internal failure.

- [ ] **Step 1: Write failing end-to-end CLI tests for help, malformed arguments, inspect PID JSON/table, verify MATCHED/negative/UNKNOWN, doctor JSON/table, stdout purity, stderr separation, and exit-code severity.**
- [ ] **Step 2: Run tests and confirm the executable behavior is missing.**
- [ ] **Step 3: Implement standard-library flag parsing, explicit mutually exclusive selectors, JSON encoder output, stable human tables, and error-to-exit mapping.**
- [ ] **Step 4: Wire the production observer and application service in `main`; inject version, commit, and toolchain through build metadata without leaking source paths.**
- [ ] **Step 5: Add README usage and honest Phase 1A status without MCP, Witness, Windows-live, or release claims.**
- [ ] **Step 6: Run focused CLI tests, build with `-trimpath`, inspect binary strings for developer paths, and run the complete gate.**
- [ ] **Step 7: Commit the CLI.**

### Task 8: macOS-primary and Linux Docker acceptance

**Files:**
- Create: `scripts/run_phase1_acceptance.sh`
- Create: `scripts/run_linux_acceptance.sh`
- Create: `build/package/Dockerfile.phase1`
- Create: `testdata/acceptance/helper/`
- Create: `docs/phase1-macos-acceptance.md`
- Create: `docs/issues/phase1-deferred-gates.md`
- Modify: `.github/workflows/quality.yml`
- Modify: `scripts/check.sh`
- Modify: `docs/architecture-development-acceptance.md`

**Interfaces:**
- Produces repeatable local acceptance that builds an installed-style binary in a task-owned prefix, launches controlled native/interpreter samples, verifies Proofs and exit codes, runs Linux Docker evidence, and cleans all artifacts/processes.

- [ ] **Step 1: Write the acceptance checklist and fixture assertions before marking any result PASS.**
- [ ] **Step 2: Add macOS journeys for current native MATCHED, outside-root LEAKED, missing process NOT_RUNNING, permission or evidence UNKNOWN, interpreter/tree verification, Unicode paths, scan limits, secret redaction, PID identity revalidation, and zero residue.**
- [ ] **Step 3: Add Ubuntu Docker journeys for live `/proc` observation, deleted executable handling, native CLI, redaction, cancellation, and zero residue.**
- [ ] **Step 4: Add macOS and Ubuntu jobs plus Windows compile/schema tests to CI; make no Windows live claim.**
- [ ] **Step 5: Run Go 1.26 and Go 1.27 gates, race tests, macOS acceptance, Linux Docker acceptance, Windows amd64 cross-build, secret/path/cache scans, binary path scan, and residue checks.**
- [ ] **Step 6: Record exact commits, OS/toolchain/container versions, commands, Proof IDs, test counts, deferrals, and discovered issues.**
- [ ] **Step 7: Self-review the complete Phase 1A diff against the frozen scope, fix every defect with a failing regression test, and rerun every affected acceptance journey.**
- [ ] **Step 8: Commit acceptance evidence and verify a clean candidate branch.**

## Completion Decision

Phase 1A is PASS only when all macOS-primary and Linux Docker items above pass on the final commit, Windows source cross-compiles, no required Phase 1A item is skipped, and cleanup is proven. Full Phase 1 remains pending until Windows 11 amd64 live-process evidence is completed; this deferral is recorded as an external gate and never converted into a PASS or hidden skip.
