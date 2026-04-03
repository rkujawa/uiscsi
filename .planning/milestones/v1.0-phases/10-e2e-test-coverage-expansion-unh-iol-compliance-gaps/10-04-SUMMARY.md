---
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
plan: 04
subsystem: session
tags: [iscsi, reject-pdu, sense-data, rfc7143, bug-fix]

requires:
  - phase: 03-session-lifecycle
    provides: "Session dispatch loop and task management"
  - phase: 05-scsi-command-layer
    provides: "ParseSense and SenseData types"
provides:
  - "OpReject PDU handling in both unsolicited and task dispatch paths"
  - "Correct SenseLength prefix stripping per RFC 7143 Section 11.4.7.2"
affects: [10-05, e2e-tests, scsi-error-handling]

tech-stack:
  added: []
  patterns: ["RFC 7143 SenseLength prefix stripping in SCSI Response data segment"]

key-files:
  created: []
  modified:
    - internal/session/session.go
    - internal/session/datain.go
    - internal/session/session_test.go

key-decisions:
  - "OpReject in unsolicited path logs and updates StatSN/window; in task path cancels the task with error"
  - "SenseLength prefix stripped using bounds-checked slice (graceful degradation for short/nil data)"

patterns-established:
  - "Reject PDU handling: unsolicited = log + update counters; task-specific = cancel task + cleanup"

requirements-completed: [E2E-12, E2E-18, E2E-19]

duration: 2min
completed: 2026-04-03
---

# Phase 10 Plan 04: OpReject and Sense Data Bug Fixes Summary

**Fixed OpReject PDU handling (both dispatch paths) and SCSI Response SenseLength prefix stripping per RFC 7143 Section 11.4.7.2**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-03T00:10:34Z
- **Completed:** 2026-04-03T00:12:46Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- OpReject (0x3F) PDU now handled in both unsolicited dispatcher and per-task dispatch loop, preventing connection drops and reconnect loops
- SCSI Response sense data correctly stripped of 2-byte SenseLength prefix, enabling ParseSense to extract correct SenseKey/ASC/ASCQ
- Added unit tests for sense data extraction including edge cases (nil, empty, zero-length)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add OpReject handling to unsolicited and task dispatch paths** - `ee1da6b` (fix)
2. **Task 2: Strip SenseLength prefix from SCSI Response sense data** - `0193346` (fix)

## Files Created/Modified
- `internal/session/session.go` - Added OpReject case in handleUnsolicited and *pdu.Reject case in taskLoop
- `internal/session/datain.go` - Strip 2-byte SenseLength prefix in handleSCSIResponse
- `internal/session/session_test.go` - Added TestSCSIResponseSenseDataExtraction and TestSCSIResponseSenseDataEmpty

## Decisions Made
- OpReject in unsolicited path (ITT=0xFFFFFFFF) logs the rejection and updates sequence counters but does not cancel any task
- OpReject in task path cancels the specific task with a descriptive error including reason code and ITT
- SenseLength extraction uses bounds checking: if data too short or SenseLength exceeds available data, SenseData is nil/empty (graceful degradation)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- OpReject handling enables negotiation matrix tests to complete without reconnect loops
- Correct sense data parsing enables SCSI error E2E tests to validate SenseKey/ASC/ASCQ fields

---
*Phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps*
*Completed: 2026-04-03*
