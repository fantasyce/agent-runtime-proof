# Privacy Model

ARP is a local evidence tool, but local does not mean harmless. Process
metadata, command lines, environment values, paths, configuration files, and
artifact contents can expose credentials or identify private work. Privacy is
therefore part of the Proof contract rather than an output-format option.

## Classification source

`contracts/privacy-registry.json` is the executable classification registry.
Every serialized leaf field reachable from the expectation, Proof, and fixture
schemas must match exactly one anchored registry rule. A new schema field that
has zero or multiple matches fails Phase 0 tests.

The five classes are:

- `PUBLIC`: stable contract vocabulary, bounded aggregates, build identity,
  and evidence status safe for the default Proof projection;
- `SAFE_IDENTIFIER`: bounded names or local process identifiers needed to
  interpret evidence but never used as credentials;
- `HASH_ONLY`: local identity represented only as lowercase SHA-256, with the
  registry stating whether the schema includes the `sha256:` prefix;
- `LOCAL_EXPLICIT_ONLY`: local paths or selectors that stay inside an
  expectation or explicit interactive CLI view;
- `PROHIBITED`: data omitted from default CLI JSON, MCP, Proof, receipts, logs,
  fixtures, and release assets.

## Channel rules

### Proof and MCP

Proof JSON and MCP results always use the safe default projection. They never
include absolute home paths, complete argv, environment values, file contents,
transcripts, cookies, credentials, private keys, or extension values without a
separately versioned classification.

### CLI

CLI table and JSON output use the same safe projection. A future
`--show-local-paths` option may expose `LOCAL_EXPLICIT_ONLY` fields only in an
interactive local terminal after explicit user selection. It must not affect
MCP output, stored Proof bytes, logs, or receipts.

### Expectations

An expectation is local input and may contain roots, entrypoints, and include
patterns needed for verification. ARP validates and uses those values locally,
but projects only safe names and hashes into a Proof. Argument matching stores
positions and fingerprints, not literal arguments.

### Fixtures and tests

Fixtures use `$HOME`, `%USERPROFILE%`, and `/fixture` projections with synthetic
identities. They must not contain a developer home path or strings resembling
real credential prefixes. Tests write only inside task-owned temporary roots.

## Hashes are not anonymity

A hash prevents direct recovery of a high-entropy identifier, but a known or
low-entropy candidate can be guessed and compared. Hash projections therefore
remain subject to minimization; the system emits only hashes required to bind
or reproduce evidence.

## Extensions

All `extensions.*` values are `PROHIBITED` by default. A later extension must
publish its own versioned schema and privacy registry before its fields can
enter Proof, MCP, logs, fixtures, or release evidence.
