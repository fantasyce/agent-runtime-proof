# Reproduce the stale-runtime problem

This demonstration creates all files under a task-owned temporary directory. It
builds and starts one harmless fixture runtime, replaces the executable on disk
with different bytes, and asks ARP whether the still-running process has been
proven to be the replacement.

From a source release or clean checkout:

```bash
bash scripts/demo_stale_runtime.sh
```

Expected result:

```text
Verdict: UNKNOWN
Reason: POSSIBLE_STALE_AFTER_REPLACEMENT
Proof level: ARTIFACT_OBSERVED
```

The result is deliberately not `MATCHED`: the live process started before the
file replacement. It is also not automatically `STALE`: observing replacement
timing and file identity is not the same as directly reading the old loaded
bytes. ARP reports the replacement evidence and keeps the conclusion uncertain.

A determinate `STALE` verdict requires direct evidence that the observed
artifact digest is a known prior digest, such as verified installed-version
history or a bound launch receipt. ARP does not manufacture that evidence for a
more dramatic demo.

To inspect the complete structured Proof:

```bash
set +e
bash scripts/demo_stale_runtime.sh --json
status=$?
set -e
test "$status" -eq 3
```

Exit code `3` is the CLI contract for `UNKNOWN`. The script terminates the
fixture process and removes its temporary directory on success, error, or
signal. It does not edit Agent configuration, request privileges, or open a
network listener.
