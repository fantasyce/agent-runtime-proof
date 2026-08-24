# Threat Model

## Scope and assets

ARP v1 protects the integrity and privacy of local runtime observations,
expectations, artifact digests, launch receipts, and Proof JSON. It also
protects the user's Agent configuration, processes, filesystem, credentials,
and formal Across state from side effects caused by inspection or tests.

ARP v1 does not claim a trusted kernel, hostile-admin resistance, publisher
identity, hardware attestation, remote attestation, or proof of every loaded
memory page.

## Trust boundaries

1. User or Agent input crosses into schema and semantic validation.
2. Host configuration crosses from potentially executable formats into a
   data-only parser.
3. OS process metadata crosses into an immutable observation snapshot.
4. Filesystem objects cross into a bounded artifact digester.
5. Internal evidence crosses the privacy projection before serialization.
6. Canonical Proof bytes cross into local storage or an Agent result.
7. Witness-owned protocol bytes cross stdin/stdout without interpretation.

## Attacker and failure capabilities

The design assumes an unprivileged local process can exit, replace files,
change configuration, create misleading paths, supply malformed declarations,
consume resources, and place sensitive strings in observable metadata. It also
assumes normal races, permission denial, crashes, and partial filesystem
failures. ARP responds with a bounded negative or indeterminate result; it does
not escalate privilege or mutate the target to obtain stronger evidence.

## Threat register

`contracts/threat-registry.json` is the executable register. It covers:

- PID reuse and process substitution;
- artifact TOCTOU;
- path, symlink, junction, and reparse-point escape;
- Unicode, separator, and Windows case normalization collisions;
- unbounded directory and parser resource use;
- execution hidden in host configuration;
- secret, command-line, environment, content, transcript, and path disclosure;
- pressure to request administrator or root rights;
- forged or ambiguous expectations;
- Proof mutation after observation;
- MCP or Witness stdout corruption;
- child-process leakage;
- acceptance-test pollution of formal user state.

Each register item defines its precondition, impact, prevention, detection,
residual risk, and required implementation phase. Privacy registry rules refer
to those stable IDs, so a classified field cannot cite an undefined threat.

## Fail-closed semantics

Permission denial, identity change, artifact change, scan limits, path escape,
normalization collision, unsupported file types, malformed configuration, and
cancelled work never produce `MATCHED`. They produce the contract verdict and
reason code appropriate to the evidence, usually `UNKNOWN`.

An untrusted expectation can still be byte-equal, but equality does not elevate
its source trust. An unsigned Proof digest detects mutation but does not identify
its producer. These residual limits must survive every output projection.

## Required evidence by phase

- Phase 0 freezes schemas, vocabularies, privacy coverage, threat records,
  canonical vectors, and synthetic cross-platform fixtures.
- Phase 1 must exercise process identity, permissions, TOCTOU, paths, links,
  normalization, limits, and local Proof recomputation on macOS first.
- Phase 3 must exercise Witness byte transparency and child lifecycle.
- Phase 4 must exercise data-only host configuration parsing and real host
  attribution.
- Final acceptance must scan source and release assets, use task-owned roots,
  and prove zero process and filesystem residue.
