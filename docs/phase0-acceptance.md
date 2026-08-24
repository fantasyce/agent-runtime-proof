# Phase 0 Contract Acceptance

- Date: 2026-08-24
- Status: PASS
- Scope: contract and threat-model baseline only
- Accepted source: the Git commit containing this record
- Primary environment: macOS 26.6.2 (`25G83`), arm64
- Toolchain: Go 1.27.0, module language baseline Go 1.26

## Direct decision

All mandatory Phase 0 criteria in the architecture design passed on the primary
macOS environment. Phase 0 is complete. This result authorizes Phase 1 planning;
it does not claim that runtime inspection, CLI, MCP, Witness, or live
cross-platform behavior already exists.

## Delivered contracts

- three JSON Schema 2020-12 contracts:
  `agent-runtime-expectation/1.0`, `agent-runtime-proof/1.0`, and
  `agent-runtime-fixture/1.0`;
- six verdicts, four ordered proof levels, and 28 reason codes;
- 51 exhaustive privacy-classification rules covering every schema leaf field;
- 13 executable threat records with prevention, detection, residual risk, and
  required implementation phase;
- eight canonical JSON and artifact-tree SHA-256 vectors;
- three valid expectation/Proof fixtures, five structurally rejected fixtures,
  and two structurally valid but semantically rejected Verdict/Reason or
  Verdict/Limitation fixtures;
- one valid Darwin, Windows, and Linux host fixture plus one rejected raw-state
  fixture;
- privacy, canonical digest, host fixture, and threat-model documentation;
- a repository check script and least-privilege GitHub Actions definition.

## Verification evidence

The final candidate was checked with:

```bash
bash scripts/check.sh
go test -count=1 -race ./...
go vet ./...
git diff --check
```

Results:

- 11 top-level Go tests passed;
- 15 test/subtest pass events, zero failures;
- both public schemas and the cross-platform fixture schema compiled;
- every valid fixture was accepted and every rejection fixture was rejected;
- every Reason Code and Limitation was checked against its allowed Verdict and
  minimum Proof Level;
- all canonical vectors reproduced their independently stored SHA-256 values;
- every reachable schema field matched exactly one privacy rule;
- all privacy threat references resolved to a complete threat record;
- Go module contents verified against `go.sum`;
- formatting, `go vet`, race-enabled tests, diff whitespace, secret-pattern,
  absolute-developer-path, and placeholder scans passed;
- no `~/.across/data/agent-runtime-proof` or
  `~/.across/run/agent-runtime-proof` state was created.

## Deferrals that are not Phase 0 failures

- macOS process observation, artifact walking, Proof generation, CLI, and local
  stdio MCP belong to Phase 1 and later implementation phases.
- Witness lifecycle and byte-transparent proxying belong to Phase 3.
- Real Agent host compatibility belongs to Phase 4.
- Windows live-machine acceptance remains a v1 release hard gate but is
  intentionally deferred as directed; Phase 0 contains only its validated
  platform-neutral fixture.
- Linux runtime acceptance is deferred; Phase 0 contains only its validated
  platform-neutral fixture.
- The repository has no GitHub remote yet, so the workflow definition was
  reviewed locally but no external Actions run is claimed.

No skipped item belongs to the mandatory Phase 0 scope.
