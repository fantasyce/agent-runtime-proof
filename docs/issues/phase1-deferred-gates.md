# Phase 1 Deferred Gates and Recorded Issues

Updated: 2026-08-25

This register separates Phase 1A's macOS-primary deliverable from work that is
still required for full Phase 1 or v1. None of these items is silently counted
as passed.

| ID | Status | Boundary | Required follow-up |
| --- | --- | --- | --- |
| ARP-P1-001 | Closed / passed | Windows 11 amd64 native build/test, live process observation, installed candidate CLI `MATCHED` and negative verification, path/Unicode tests, junction rejection, explicit read-data denial returning `UNKNOWN/ARTIFACT_INACCESSIBLE`, ACL restoration, and controlled-helper cleanup passed on 2026-08-25. | Preserve `scripts/run_windows_phase1_acceptance.ps1` in the native Windows matrix. Public release assets remain a later release gate, not part of this local Phase 1 core result. |
| ARP-P1-002 | Closed / passed | Windows now uses local-drive, parent-handle-relative traversal, rejects every opened reparse point, binds `FileIdInfo`, and performs final metadata plus selected-content barriers. Native junction and mutation tests passed. | Preserve these tests in the Windows release matrix; do not silently extend this result to UNC roots. |
| ARP-P1-003 | Closed as a Phase 1 gate / explicit later-phase limitation | Interpreter-script and declared-tree payload bytes can be hashed, but passive Phase 1 process observation does not prove privacy-safe launch-argument binding. They correctly return `UNKNOWN/PLATFORM_EVIDENCE_UNAVAILABLE`, never `MATCHED`. Phase 1's authoritative deliverables require ProcessIdentity, not Witness launch binding. | Preserve the honest result; add direct argument/Witness evidence in Phase 3 rather than weakening Phase 1 proof semantics. |
| ARP-P1-004 | Closed for Phase 1 | The CLI now accepts repeatable, lowercase 64-hex `--known-prior-digest` values and passes them to the existing evaluator. Installed macOS and Windows candidates produced `STALE` only with a directly known current artifact digest. | Versioned installed history/Profile discovery and multi-binding `CONFLICT` aggregation remain later Host Profile/store work, not an unimplemented Phase 1 CLI gate. |
| ARP-P1-005 | Closed / passed | The full package suite and real-process acceptance passed on native Linux amd64 with an official checksum-verified task-owned Go 1.26.0 toolchain. The Ubuntu amd64 GitHub runner and container acceptance also passed. | Preserve both native-host and container evidence; do not reinterpret the earlier Rosetta process as guest evidence. |
| ARP-P1-006 | Closed / passed for development CI | Independent private repository `fantasyce/agent-runtime-proof` was created without a release or license decision. Final GitHub Actions run `32764337638` passed macOS Go 1.26/1.27, Ubuntu amd64, and Windows 2025 jobs. | Public visibility, license approval, protected-branch policy, signed release assets, and provenance remain Phase 5 release gates. |

Phase 1 is complete. The macOS and Windows records remain the detailed core
evidence, while native Linux amd64 and the remote matrix close the former
environment gaps. Phase 2 evidence is recorded separately in
[phase2-acceptance.md](../phase2-acceptance.md). Later Witness, Host Profile,
full named-host, release, and public-governance gates remain outside Phase 1.
