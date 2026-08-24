# Phase 1 Deferred Gates and Recorded Issues

Updated: 2026-08-25

This register separates Phase 1A's macOS-primary deliverable from work that is
still required for full Phase 1 or v1. None of these items is silently counted
as passed.

| ID | Status | Boundary | Required follow-up |
| --- | --- | --- | --- |
| ARP-P1-001 | Closed / passed | Windows 11 amd64 native build/test, live process observation, installed candidate CLI `MATCHED` and negative verification, path/Unicode tests, junction rejection, explicit read-data denial returning `UNKNOWN/ARTIFACT_INACCESSIBLE`, ACL restoration, and controlled-helper cleanup passed on 2026-08-25. | Preserve `scripts/run_windows_phase1_acceptance.ps1` in the native Windows matrix. Public release assets remain a later release gate, not part of this local Phase 1 core result. |
| ARP-P1-002 | Closed / passed | Windows now uses local-drive, parent-handle-relative traversal, rejects every opened reparse point, binds `FileIdInfo`, and performs final metadata plus selected-content barriers. Native junction and mutation tests passed. | Preserve these tests in the Windows release matrix; do not silently extend this result to UNC roots. |
| ARP-P1-003 | Deliberate honest limitation | Interpreter-script and declared-tree payload bytes can be hashed, but Phase 1A does not collect a privacy-safe launch-argument binding. They return `UNKNOWN/PLATFORM_EVIDENCE_UNAVAILABLE`, never `MATCHED`. | Add hashed argument-position evidence or Witness binding in a later phase. |
| ARP-P1-004 | Full Phase 1 gap | `STALE` evaluation exists only when a caller supplies a directly known prior digest; the Phase 1A CLI has no installed-history/profile input. Multi-process `CONFLICT` aggregation is also deferred. | Add versioned history/Profile inputs without making timestamp-only stale claims. |
| ARP-P1-005 | Local environment boundary | Linux ARM64 container acceptance passed. An attempted amd64 container run on Apple Silicon observed Docker Desktop's `rosetta` process instead of the guest executable, so it was rejected as amd64 runtime evidence. | Run the parameterized acceptance script on a native Linux amd64 runner. |
| ARP-P1-006 | External service boundary | This local repository has no Git remote, so the new GitHub Actions matrix has not executed remotely. | Create/push the independent public repository and require green macOS, Ubuntu, and Windows compile/contract jobs before release. |

Phase 1A remains accepted locally. The Windows core candidate is separately
recorded in [phase1-windows-acceptance.md](../phase1-windows-acceptance.md).
The Windows Phase 1 core gate is closed. Full Phase 1 and v1 remain blocked by
the applicable cross-platform implementation, CI, platform, protocol, host,
and release gates above.
