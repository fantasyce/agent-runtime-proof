# Cross-Platform Host Fixture Contract

`agent-runtime-fixture/1.0` is a data-only test boundary for future macOS,
Windows, and Linux process adapters. A fixture is not a captured user process,
does not execute a command, and does not assert that an OS adapter is already
implemented.

## Purpose

Each fixture binds four things:

1. a synthetic platform identity;
2. a bounded immutable process/artifact snapshot;
3. a complete `agent-runtime-expectation/1.0` object;
4. the verdict, proof level, and reason codes a future evaluator must produce.

The same fixtures can be consumed by unit tests on every development platform.
Real OS acceptance later adds live evidence; it does not replace these cases.

## Safety rules

- Paths use `$HOME`, `%USERPROFILE%`, or `/fixture` synthetic projections.
- `created_at_unix_nano` is a decimal string to preserve precision.
- Complete argv and environment maps are forbidden.
- Hash fields contain lowercase SHA-256 projections, never raw identifiers.
- `denied_fields` records missing evidence without inventing a substitute.
- Fixture parsers must not expand variables, follow paths, evaluate tags, or
  run the declared entrypoint.
- Unknown fields are rejected except inside the explicit reverse-DNS
  `extensions` object.

## Platform examples

- Darwin `native-matched.json` describes a visible native executable whose
  declared artifact digest matches.
- Windows `interpreter-inaccessible.json` describes a visible process whose
  image and artifact bytes are denied, so the only safe verdict is UNKNOWN.
- Linux `deleted-executable.json` describes a deleted executable whose old
  bytes remain directly observable through the process handle and differ from
  the current expectation, so the fixture expects STALE.

These are contract fixtures, not claims that Phase 0 has implemented or live
tested the corresponding platform adapter.
