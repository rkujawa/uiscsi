---
phase: 05-scsi-command-layer
plan: 03
subsystem: scsi
tags: [scsi, cdb, thin-provisioning, reservations, sync-cache, unmap, compare-and-write, sbc-3, spc-4]

# Dependency graph
requires:
  - phase: 05-scsi-command-layer plan 01
    provides: opcode constants, Option type, applyOptions, checkResult, session.Command
provides:
  - SynchronizeCache10/16 CDB builders for cache flush
  - WriteSame10/16 CDB builders with UNMAP/ANCHOR/NDOB flags
  - Unmap CDB builder with parameter data serialization
  - PersistReserveIn/Out CDB builders with response parsers
  - Verify10/16 CDB builders with BYTCHK
  - CompareAndWrite CDB builder with 2x transfer length
  - StartStopUnit CDB builder with power conditions
affects: [06-high-level-api, 07-integration-tests]

# Tech tracking
tech-stack:
  added: []
  patterns: [parameter-data-serialization, service-action-dispatch]

key-files:
  created:
    - internal/scsi/provisioning.go
    - internal/scsi/provisioning_test.go
    - internal/scsi/reservations.go
    - internal/scsi/reservations_test.go
  modified:
    - internal/scsi/commands.go
    - internal/scsi/commands_test.go

key-decisions:
  - "UNMAP parameter data: 8-byte header (data-length, BD-data-length, reserved) + 16-byte descriptors (8B LBA + 4B count + 4B reserved)"
  - "PR OUT 24-byte parameter data: 8B key + 8B saKey + 8B zeros per SPC-4"
  - "CompareAndWrite ExpectedDataTransferLen = 2 * blocks * blockSize per Pitfall 8"
  - "NDOB flag on WriteSame disables data transfer entirely (Write=false, Data=nil)"

patterns-established:
  - "Parameter data builders: functions build byte buffers and attach as bytes.NewReader to cmd.Data"
  - "Service action constants as named package-level consts (PRInReadKeys, PROutRegister, etc.)"

requirements-completed: [SCSI-11, SCSI-12, SCSI-13, SCSI-14, SCSI-15, SCSI-16, SCSI-17, SCSI-19]

# Metrics
duration: 5min
completed: 2026-04-01
---

# Phase 5 Plan 3: Extended SCSI Commands Summary

**Cache, provisioning, reservations, verify, compare-and-write, and start-stop-unit CDB builders with parameter data serialization for UNMAP and PR OUT**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-01T12:50:58Z
- **Completed:** 2026-04-01T12:55:47Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- SynchronizeCache10/16, WriteSame10/16, and Unmap CDB builders with all SBC-3 flags (IMMED, UNMAP, ANCHOR, NDOB)
- UNMAP parameter data serialization with 8-byte header + 16-byte descriptors per Pitfall 6
- PersistReserveIn/Out with service action constants, 24-byte parameter data, and response parsers per Pitfall 7
- Verify10/16 with BYTCHK, CompareAndWrite with 2x transfer length per Pitfall 8, StartStopUnit with power conditions

## Task Commits

Each task was committed atomically:

1. **Task 1: Cache, provisioning, and UNMAP commands** - `303c20f` (feat)
2. **Task 2: Reservations, VERIFY, COMPARE AND WRITE, START STOP UNIT** - `8534a53` (feat)

## Files Created/Modified
- `internal/scsi/provisioning.go` - SynchronizeCache10/16, WriteSame10/16, Unmap CDB builders
- `internal/scsi/provisioning_test.go` - Table-driven tests for all provisioning commands
- `internal/scsi/reservations.go` - PersistReserveIn/Out, ParsePersistReserveInKeys/Reservation
- `internal/scsi/reservations_test.go` - Tests for PR IN/OUT and response parsers
- `internal/scsi/commands.go` - Added Verify10/16, CompareAndWrite, StartStopUnit
- `internal/scsi/commands_test.go` - Tests for verify, CAW, and SSU commands

## Decisions Made
- UNMAP parameter data uses 8-byte header with explicit data-length and BD-data-length fields, followed by 16-byte descriptors
- PR OUT uses minimal 24-byte parameter data (key + saKey + zeros) covering the common case; advanced fields (scope-specific address, APTPL) left zero
- CompareAndWrite ExpectedDataTransferLen computed as 2*blocks*blockSize to account for compare + write data halves
- WriteSame NDOB flag suppresses all data transfer (Write=false, Data=nil, ExpectedDataTransferLen=0)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 19 SCSI command requirements now covered across plans 01-03
- CDB builders ready for high-level API layer (Phase 6)
- Integration test suite (Phase 7) can exercise all commands

## Self-Check: PASSED

- All 6 created/modified files exist on disk
- Commit 303c20f (Task 1) found in git log
- Commit 8534a53 (Task 2) found in git log
- Full test suite passes with -race

---
*Phase: 05-scsi-command-layer*
*Completed: 2026-04-01*
