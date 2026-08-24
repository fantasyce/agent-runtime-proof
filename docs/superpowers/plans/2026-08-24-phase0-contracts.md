# Agent Runtime Proof Phase 0 Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Freeze and automatically verify the Agent Runtime Proof v1 data contracts, decision vocabulary, canonical digest vectors, privacy/threat rules, and cross-platform fixture format.

**Architecture:** Phase 0 produces platform-neutral contract assets at the repository root and a Go test harness that treats those assets as executable specifications. JSON Schema validates structure, registry-consistency tests prevent vocabulary drift, golden vectors freeze canonical byte and digest behavior, and documentation defines semantic rules that JSON Schema cannot safely express.

**Tech Stack:** Go 1.26 language baseline tested locally with Go 1.27.0 on macOS arm64; JSON Schema 2020-12; `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3 as a test-only validator; SHA-256; RFC 8785 canonical JSON profile restricted to integers and finite JSON values.

**Spec:** `docs/architecture-development-acceptance.md`

## Global Constraints

- Work only in the independent `agent-runtime-proof` repository.
- Phase 0 does not implement CLI, MCP, process inspection, artifact walking, Witness, host mutation, networking, or Windows live acceptance.
- Public contract versions are exactly `agent-runtime-expectation/1.0`, `agent-runtime-proof/1.0`, and `agent-runtime-fixture/1.0`.
- Verdicts are exactly `MATCHED`, `STALE`, `LEAKED`, `CONFLICT`, `NOT_RUNNING`, and `UNKNOWN`.
- Proof levels are ordered `PROCESS_OBSERVED`, `CONFIG_BOUND`, `ARTIFACT_OBSERVED`, `LAUNCH_WITNESSED`.
- Unknown security-critical fields are rejected with `additionalProperties: false`; vendor extensions are allowed only inside the explicit `extensions` object with reverse-DNS keys.
- Absolute paths, complete argv, environment values, file contents, transcripts, and credentials are never public contract fields.
- Tests must use repository fixtures only and create no user runtime state.
- Every behavior change follows RED → GREEN → REFACTOR and ends in a focused commit.

---

### Task 1: Go contract-test harness

**Files:**
- Create: `go.mod`
- Create: `internal/contracttest/loader_test.go`
- Create: `internal/contracttest/testdata/valid.json`
- Create: `scripts/check.sh`

**Interfaces:**
- Consumes: repository-relative fixture paths.
- Produces: test helpers `repoRoot(t *testing.T) string` and `loadJSON(t *testing.T, path string) any`; one command, `bash scripts/check.sh`, for Phase 0 verification.

- [ ] **Step 1: Write the failing loader test**

Create `internal/contracttest/loader_test.go` with a test that calls the wished-for `loadJSON` helper on `internal/contracttest/testdata/valid.json` and asserts the decoded object contains `{"ready": true}`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/contracttest -run TestLoadJSON -v`
Expected: compile failure because `loadJSON` is undefined.

- [ ] **Step 3: Implement the minimal test helper**

Add `repoRoot` by walking from `runtime.Caller(0)` to the repository root, and `loadJSON` using `os.ReadFile`, `json.Decoder.UseNumber`, one successful decode, then an EOF check so trailing JSON is rejected. Add `go.mod` with:

```go
module github.com/fantasyce/agent-runtime-proof

go 1.26
```

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/contracttest -run TestLoadJSON -v`
Expected: one passing test.

- [ ] **Step 5: Add the repository check entrypoint**

Create executable `scripts/check.sh` using `set -euo pipefail`, `gofmt -l`, `go vet ./...`, `go test -race ./...`, and `git diff --check`. The script must resolve the repository root from its own location and must not mutate user state.

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/contracttest scripts/check.sh
git commit -m "test: establish contract verification harness"
```

### Task 2: Decision registry and expectation/proof schemas

**Files:**
- Create: `contracts/decision-registry.json`
- Create: `schemas/agent-runtime-expectation-1.0.schema.json`
- Create: `schemas/agent-runtime-proof-1.0.schema.json`
- Create: `testdata/contracts/valid/expectation-native.json`
- Create: `testdata/contracts/valid/proof-matched.json`
- Create: `testdata/contracts/valid/proof-unknown.json`
- Create: `testdata/contracts/invalid/expectation-missing-subject.json`
- Create: `testdata/contracts/invalid/expectation-unknown-field.json`
- Create: `testdata/contracts/invalid/proof-invalid-verdict.json`
- Create: `testdata/contracts/invalid/proof-raw-argv.json`
- Create: `internal/contracttest/schema_test.go`
- Modify: `go.mod`
- Create: `go.sum`

**Interfaces:**
- Consumes: JSON Schema 2020-12 and the fixed vocabulary in Global Constraints.
- Produces: two authoritative public schemas and one registry whose values are checked against schema enums.

- [ ] **Step 1: Write failing schema compilation and fixture tests**

Add table-driven tests that compile both schema paths with `jsonschema/v6`, validate every `testdata/contracts/valid/*.json`, require every `testdata/contracts/invalid/*.json` to fail, and compare schema verdict/proof-level enums with `contracts/decision-registry.json`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/contracttest -run 'TestSchemasCompile|TestContractFixtures|TestDecisionRegistryMatchesSchemas' -v`
Expected: failure because the schemas and registry do not exist.

- [ ] **Step 3: Add the decision registry**

The registry must contain the six verdicts, four ordered proof levels, and reason-code records with `code`, `category`, `description`, and `allowed_verdicts`. Required reason codes are:

```text
MATCH_CONFIRMED
PROCESS_NOT_FOUND
PROCESS_IDENTITY_CHANGED
PROCESS_INACCESSIBLE
EXPECTATION_MISSING
EXPECTATION_INVALID
EXPECTATION_UNTRUSTED
EXPECTATION_AMBIGUOUS
ARTIFACT_MISMATCH
ARTIFACT_VERSION_MISMATCH
ARTIFACT_CHANGED_DURING_READ
ARTIFACT_SCAN_LIMIT_EXCEEDED
ARTIFACT_INACCESSIBLE
ARTIFACT_PATH_ESCAPE
ARTIFACT_SYMLINK_REJECTED
ARTIFACT_NORMALIZATION_COLLISION
ARTIFACT_UNSUPPORTED_TYPE
POSSIBLE_STALE_AFTER_REPLACEMENT
STALE_WITNESS_RECEIPT
RUNTIME_OUTSIDE_ALLOWED_ROOT
MULTIPLE_ACTIVE_DIGESTS
HOST_BINDING_AMBIGUOUS
HOST_CONFIG_INACCESSIBLE
HOST_CONFIG_INVALID
WITNESS_EXPECTATION_MISSING
DYNAMIC_DEPENDENCIES_UNPROVEN
PLATFORM_EVIDENCE_UNAVAILABLE
SCAN_CANCELLED
```

Registry tests must reject duplicate names, empty descriptions, unknown verdict references, and proof-level order gaps.

- [ ] **Step 4: Add strict expectation and proof schemas**

Use `$schema: https://json-schema.org/draft/2020-12/schema`, stable `$id` values rooted at `https://agent-runtime-proof.dev/schemas/`, strict nested objects, SHA-256 patterns of `^[a-f0-9]{64}$` or `^sha256:[a-f0-9]{64}$` as appropriate, RFC 3339 timestamps, non-negative integer limits, and `$HOME`-redacted path projections. The Proof schema must not define raw `argv`, `environment`, `content`, `transcript`, or `secret` properties.

- [ ] **Step 5: Add valid and invalid golden fixtures**

Valid fixtures cover trusted native expectation, hash-backed MATCHED proof, and permission-limited UNKNOWN proof. Invalid fixtures each violate one named rule: missing subject, unknown security field, unsupported verdict, and raw argv disclosure.

- [ ] **Step 6: Verify GREEN and all registry invariants**

Run: `go test ./internal/contracttest -run 'TestSchemasCompile|TestContractFixtures|TestDecisionRegistryMatchesSchemas|TestDecisionRegistryInvariants' -v`
Expected: all tests pass with every invalid fixture observed failing.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum contracts schemas testdata/contracts internal/contracttest/schema_test.go
git commit -m "feat: freeze runtime expectation and proof contracts"
```

### Task 3: Canonical JSON and artifact digest vectors

**Files:**
- Create: `docs/contracts/canonical-json-and-artifact-digest.md`
- Create: `testdata/canonical/proof-vectors.json`
- Create: `testdata/canonical/artifact-tree-vectors.json`
- Create: `internal/contracttest/canonical_vectors_test.go`

**Interfaces:**
- Consumes: RFC 8785 canonical JSON rules and Phase 0 contract objects.
- Produces: golden `{name, canonical, sha256}` vectors for Proof bodies and artifact-entry arrays.

- [ ] **Step 1: Write failing vector tests**

Tests load both vector files, require unique names, parse each `canonical` string as exactly one JSON value using `UseNumber`, require no newline or surrounding whitespace, calculate SHA-256 over the exact UTF-8 bytes, and compare lowercase hexadecimal output with `sha256`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/contracttest -run TestCanonicalVectors -v`
Expected: failure because vector files are missing.

- [ ] **Step 3: Write the canonicalization profile**

The document must state: RFC 8785 property ordering and string escaping; UTF-8; no insignificant whitespace; integers only in public contracts; no NaN/Infinity; timestamps normalized to UTC with `Z`; Unicode strings are preserved rather than silently normalized; paths are normalized separately before entering JSON; `proof_id` hashes the Proof with `proof_id` omitted; directory entries are `{path,size,sha256}` sorted by normalized path; empty directories are not represented; and normalization collisions reject the scan.

- [ ] **Step 4: Add independently computed golden vectors**

Include at least: empty object, reordered ASCII keys, non-ASCII keys, escaped controls, integer limits, one MATCHED Proof body, empty artifact tree, and a two-file artifact tree. Calculate expected SHA-256 with `shasum -a 256` over the exact canonical strings and store only the digest, not a shell command.

- [ ] **Step 5: Verify GREEN**

Run: `go test ./internal/contracttest -run TestCanonicalVectors -v`
Expected: all vectors parse and hash exactly.

- [ ] **Step 6: Commit**

```bash
git add docs/contracts testdata/canonical internal/contracttest/canonical_vectors_test.go
git commit -m "docs: freeze canonical proof and artifact digest vectors"
```

### Task 4: Cross-platform host fixture contract

**Files:**
- Create: `schemas/agent-runtime-fixture-1.0.schema.json`
- Create: `testdata/hosts/darwin/native-matched.json`
- Create: `testdata/hosts/windows/interpreter-inaccessible.json`
- Create: `testdata/hosts/linux/deleted-executable.json`
- Create: `testdata/hosts/invalid/raw-environment.json`
- Create: `docs/contracts/host-fixture-format.md`
- Create: `internal/contracttest/host_fixture_test.go`

**Interfaces:**
- Consumes: expectation/proof schemas and privacy constraints.
- Produces: `agent-runtime-fixture/1.0`, a data-only format for later OS-adapter tests.

- [ ] **Step 1: Write failing fixture schema tests**

Compile the fixture schema, validate every darwin/windows/linux fixture, require every invalid fixture to fail, and assert that each supported platform has at least one valid fixture.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/contracttest -run TestHostFixtures -v`
Expected: failure because the fixture schema and files are missing.

- [ ] **Step 3: Define the fixture schema**

Required fields: `schema_version`, `fixture_id`, `platform`, `snapshot`, `expectation`, and `expected`. `platform` contains `os` and `arch`; `snapshot` contains process identity, safe executable projection, parent relation, observable artifact facts, denied fields, and optional host binding; `expected` contains verdict, proof level, and reason codes. Environment values and complete argv are forbidden.

- [ ] **Step 4: Add representative platform fixtures**

- Darwin: native executable, allowed root, matching artifact digest.
- Windows: interpreter entrypoint with process-image permission denied, yielding UNKNOWN.
- Linux: `/proc/<pid>/exe` reports a deleted executable and a prior digest, yielding STALE.
- Invalid: embeds a raw environment value and must be rejected.

All paths use `$HOME`, `%USERPROFILE%`, or fixture-owned synthetic roots; no developer absolute path appears.

- [ ] **Step 5: Verify GREEN**

Run: `go test ./internal/contracttest -run TestHostFixtures -v`
Expected: three valid fixtures and one rejected privacy fixture.

- [ ] **Step 6: Commit**

```bash
git add schemas/agent-runtime-fixture-1.0.schema.json testdata/hosts docs/contracts/host-fixture-format.md internal/contracttest/host_fixture_test.go
git commit -m "test: define cross-platform runtime fixture contract"
```

### Task 5: Privacy classification and threat model

**Files:**
- Create: `contracts/privacy-registry.json`
- Create: `docs/privacy-model.md`
- Create: `docs/threat-model.md`
- Create: `internal/contracttest/privacy_test.go`

**Interfaces:**
- Consumes: every serialized field in all three schemas.
- Produces: exhaustive field classification and mitigations traceable to automated tests.

- [ ] **Step 1: Write failing privacy coverage tests**

Recursively collect leaf property paths from all three schemas. Require each path to appear exactly once in `contracts/privacy-registry.json`; reject prohibited property names matching case-insensitive `environment|env_value|argv|content|transcript|password|secret|token|cookie|private_key`; require every threat record referenced by a privacy rule to exist in `docs/threat-model.md` using stable IDs such as `T-PID-REUSE`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/contracttest -run TestPrivacy -v`
Expected: failure because registry and documents are missing.

- [ ] **Step 3: Add exhaustive privacy classes**

Use exactly these classes:

- `PUBLIC`: schema versions, verdict, proof level, reason codes, counts, tool/platform versions;
- `SAFE_IDENTIFIER`: subject ID, host ID, safe basename, architecture;
- `HASH_ONLY`: absolute/local path identity, argv segments, parent-chain identity, expectation locator, boot ID;
- `LOCAL_EXPLICIT_ONLY`: resolved path that CLI may display only with explicit local-path opt-in and that MCP never returns;
- `PROHIBITED`: environment values, complete argv, contents, transcripts, secrets, credentials, cookies, private keys.

Every registry row includes `field`, `class`, `projection`, and one or more threat IDs.

- [ ] **Step 4: Write the threat model**

Define assets, trust boundaries, attacker capabilities, non-goals, and mitigations for PID reuse, TOCTOU, symlink/junction escape, normalization collision, unbounded directory denial of service, config execution, command-line secret leakage, privilege escalation, forged expectation, Proof modification, Witness stdout corruption, child leakage, and test-state pollution. Each threat has stable ID, precondition, impact, prevention, detection, residual risk, and required test phase.

- [ ] **Step 5: Verify GREEN**

Run: `go test ./internal/contracttest -run TestPrivacy -v`
Expected: all schema fields classified exactly once and all threat links resolved.

- [ ] **Step 6: Commit**

```bash
git add contracts/privacy-registry.json docs/privacy-model.md docs/threat-model.md internal/contracttest/privacy_test.go
git commit -m "docs: define privacy and threat contracts"
```

### Task 6: macOS Phase 0 acceptance and CI contract

**Files:**
- Create: `.github/workflows/quality.yml`
- Create: `docs/phase0-acceptance.md`
- Modify: `README.md`
- Modify: `docs/architecture-development-acceptance.md`

**Interfaces:**
- Consumes: `bash scripts/check.sh` and all Phase 0 assets.
- Produces: a repeatable macOS-primary acceptance record and future CI definition for Go 1.26.x/1.27.x.

- [ ] **Step 1: Write the acceptance checklist before claiming completion**

Record the exact repository commit, macOS version/architecture, Go version, commands, expected zero-residue boundary, test counts, and explicit deferrals: no CLI/MCP/process/Witness implementation, no Windows live acceptance, and no Linux runtime acceptance in Phase 0.

- [ ] **Step 2: Add CI workflow**

Define least-privilege `contents: read`, no secrets, concurrency cancellation, macOS jobs for Go `1.26.x` and `1.27.x`, dependency download, `bash scripts/check.sh`, and a separate Ubuntu schema-fixture portability job. Do not add Windows live claims; a Windows schema-only CI lane may be added later with Phase 1 planning.

- [ ] **Step 3: Update public status**

README must say Phase 0 contracts are implemented and Phase 1 runtime code has not started. The architecture document must link the acceptance record without changing v1 scope.

- [ ] **Step 4: Run complete verification**

Run, in order:

```bash
bash scripts/check.sh
go test -count=1 -race ./...
go vet ./...
git diff --check
git status --short
```

Expected: all commands exit 0; tests have zero failures; only intended Phase 0 files are modified; no process, receipt, runtime directory, network listener, or user configuration is created.

- [ ] **Step 5: Commit acceptance artifacts**

```bash
git add .github/workflows/quality.yml README.md docs/architecture-development-acceptance.md docs/phase0-acceptance.md
git commit -m "ci: add Phase 0 contract acceptance"
```

- [ ] **Step 6: Verify clean branch**

Run: `bash scripts/check.sh && git status --short --branch`
Expected: checks pass and branch is clean.
