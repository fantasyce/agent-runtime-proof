# Contributing

Thank you for helping make local Agent runtime evidence more trustworthy.

## Before opening a change

- Search existing issues and Discussions.
- For a behavior change, describe the false certainty or missing evidence it prevents.
- Keep ARP local-only, read-only, and conservative when evidence is incomplete.
- Never include credentials, raw environment values, private configuration, or full command lines in fixtures, issues, commits, or screenshots.
- Report vulnerabilities privately according to `SECURITY.md`.

## Development

Use Go 1.27 or newer and run:

```bash
bash scripts/check.sh
```

New behavior requires a failing test first. Cross-platform process behavior
must include the affected native target; a cross-compile proves only that the
code builds. Release and acceptance scripts must use task-owned temporary
directories and leave no process or file residue.

## Pull requests

Keep each pull request focused. Explain the observed problem, evidence boundary,
test coverage, platforms exercised, and any platform not exercised. A passing
source test does not by itself prove packaged or installed behavior.

By contributing, you agree that your contribution is licensed under the
repository's Apache-2.0 license.
