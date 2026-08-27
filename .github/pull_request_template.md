## Problem and evidence boundary

Describe the false certainty, missing evidence, defect, or documentation gap.

## Change

Explain the smallest change that addresses it.

## Verification

- [ ] I added or updated a test and observed the relevant failure before the fix.
- [ ] `bash scripts/check.sh` passes.
- [ ] I listed every native platform exercised and every platform not exercised.
- [ ] I used task-owned test state and removed processes and temporary files.
- [ ] I reviewed the diff for secrets, private paths, raw argv, and environment values.

## Release impact

State whether this changes contracts, CLI behavior, MCP metadata, packaging, or
only documentation.
