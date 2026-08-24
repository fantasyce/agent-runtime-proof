# Phase 1 Deferred Gates and Recorded Issues

Updated: 2026-08-24

This register separates Phase 1A's macOS-primary deliverable from work that is
still required for full Phase 1 or v1. None of these items is silently counted
as passed.

| ID | Status | Boundary | Required follow-up |
| --- | --- | --- | --- |
| ARP-P1-001 | External gate | Windows 11 amd64 live process, installed CLI, permission, path, Unicode, junction/reparse-point, and cleanup acceptance has not run. Cross-compilation is not a substitute. | Run the Windows acceptance matrix on the user's Windows host before full Phase 1 or v1. |
| ARP-P1-002 | Deliberately disabled capability | Windows artifact reading returns `UNKNOWN/PLATFORM_EVIDENCE_UNAVAILABLE`; it does not traverse the declared tree because handle-relative junction/reparse-point-safe walking is not implemented yet. | Implement and live-test reparse-point-safe handle traversal before Windows acceptance. |
| ARP-P1-003 | Deliberate honest limitation | Interpreter-script and declared-tree payload bytes can be hashed, but Phase 1A does not collect a privacy-safe launch-argument binding. They return `UNKNOWN/PLATFORM_EVIDENCE_UNAVAILABLE`, never `MATCHED`. | Add hashed argument-position evidence or Witness binding in a later phase. |
| ARP-P1-004 | Full Phase 1 gap | `STALE` evaluation exists only when a caller supplies a directly known prior digest; the Phase 1A CLI has no installed-history/profile input. Multi-process `CONFLICT` aggregation is also deferred. | Add versioned history/Profile inputs without making timestamp-only stale claims. |
| ARP-P1-005 | Local environment boundary | Linux ARM64 container acceptance passed. An attempted amd64 container run on Apple Silicon observed Docker Desktop's `rosetta` process instead of the guest executable, so it was rejected as amd64 runtime evidence. | Run the parameterized acceptance script on a native Linux amd64 runner. |
| ARP-P1-006 | External service boundary | This local repository has no Git remote, so the new GitHub Actions matrix has not executed remotely. | Create/push the independent public repository and require green macOS, Ubuntu, and Windows compile/contract jobs before release. |

Phase 1A may be accepted locally with these items open because its frozen scope
requires macOS live evidence, Linux container evidence, and Windows source plus
cross-compilation. Full Phase 1 and v1 remain blocked by ARP-P1-001 and the
applicable implementation/host gates above.
