---
phase: 04-write-path
plan: 02
subsystem: session
tags: [iscsi, data-out, r2t, write-path, io-reader, buffer-pool]

requires:
  - phase: 04-write-path-01
    provides: "Command.Data io.Reader, task.isWrite/reader/bytesSent, immediate data in Submit"
provides:
  - "sendDataOutBurst: core burst function for Data-Out PDU generation"
  - "handleR2T: R2T processing with MaxBurstLength enforcement"
  - "sendUnsolicitedDataOut: unsolicited Data-Out with TTT=0xFFFFFFFF"
  - "taskLoop R2T case for solicited write path"
affects: [04-write-path-03, error-recovery, integration-tests]

tech-stack:
  added: []
  patterns:
    - "Per-burst DataSN reset to 0 per RFC 7143"
    - "io.ReadFull for on-demand reader consumption in Data-Out generation"
    - "transport.GetBuffer/PutBuffer for Data-Out segment allocation"
    - "expStatSN passed as func() uint32 for latest value per PDU"

key-files:
  created:
    - internal/session/dataout.go
    - internal/session/dataout_test.go
  modified:
    - internal/session/session.go

key-decisions:
  - "sendDataOutBurst uses per-burst DataSN=0 reset per RFC 7143 Section 11.7"
  - "handleR2T caps DesiredDataTransferLength at MaxBurstLength before burst"
  - "Unsolicited Data-Out runs synchronously in Submit before taskLoop starts (no concurrent io.Reader access)"
  - "expStatSN closure pattern ensures each Data-Out PDU gets latest value"

patterns-established:
  - "Data-Out burst pattern: loop reading chunks from io.Reader, marshal PDU, send via writeCh"
  - "R2T handling pattern: decode R2T in taskLoop, delegate to handleR2T on task struct"

requirements-completed: [WRITE-01, WRITE-02, WRITE-04, WRITE-05]

duration: 7min
completed: 2026-04-01
---

# Phase 4 Plan 2: Data-Out Engine Summary

**R2T-driven solicited Data-Out with MaxBurstLength enforcement and unsolicited Data-Out when InitialR2T=No, using on-demand io.Reader consumption**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-01T10:14:58Z
- **Completed:** 2026-04-01T10:22:10Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Implemented sendDataOutBurst core function: reads from io.Reader on-demand, segments into Data-Out PDUs bounded by MaxRecvDataSegmentLength, resets DataSN per burst
- Added R2T handling in taskLoop with MaxBurstLength enforcement (caps DesiredDataTransferLength)
- Added unsolicited Data-Out dispatch in Submit when InitialR2T=No, bounded by FirstBurstLength minus immediate data
- Comprehensive test coverage: solicited R2T, unsolicited Data-Out, MaxBurstLength enforcement, multi-PDU bursts, reader error propagation

## Task Commits

Each task was committed atomically:

1. **Task 1: Create dataout.go with Data-Out generation functions** - `0bc34a1` (feat)
2. **Task 2: Create dataout_test.go with write path unit tests** - `2b06185` (test)

**Plan metadata:** (pending docs commit)

## Files Created/Modified
- `internal/session/dataout.go` - sendDataOutBurst, handleR2T, sendUnsolicitedDataOut methods on task struct
- `internal/session/dataout_test.go` - 6 tests covering solicited R2T, unsolicited, MaxBurstLength, multi-PDU, reader errors
- `internal/session/session.go` - taskLoop R2T case, unsolicited Data-Out dispatch in Submit

## Decisions Made
- sendDataOutBurst resets DataSN to 0 per burst per RFC 7143 Section 11.7 (each R2T response and unsolicited sequence starts at DataSN=0)
- handleR2T caps DesiredDataTransferLength at MaxBurstLength before calling sendDataOutBurst, rather than checking mid-burst
- Unsolicited Data-Out runs synchronously in Submit before starting the task goroutine, ensuring no concurrent io.Reader access (Pitfall 6)
- expStatSN passed as func() uint32 closure so each Data-Out PDU in a burst gets the most current ExpStatSN value

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None - all Data-Out functions are fully wired and operational.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Data-Out engine complete, ready for Plan 03 (end-to-end write integration)
- All solicited and unsolicited write paths functional
- Buffer pool integration via transport.GetBuffer/PutBuffer working

---
*Phase: 04-write-path*
*Completed: 2026-04-01*
