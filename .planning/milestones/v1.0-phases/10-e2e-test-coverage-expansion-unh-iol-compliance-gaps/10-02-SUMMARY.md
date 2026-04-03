---
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
plan: 02
subsystem: testing
tags: [e2e, digest, crc32c, scsi-error, sense-data, lio]

# Dependency graph
requires:
  - phase: 09-lio-e2e-tests
    provides: E2E test infrastructure with LIO kernel target setup
provides:
  - Header-only and data-only digest mode E2E tests
  - SCSI error condition E2E tests (out-of-range LBA, sense data parsing)
affects: [10-e2e-test-coverage-expansion-unh-iol-compliance-gaps]

# Tech tracking
tech-stack:
  added: []
  patterns: [asymmetric digest test pattern, SCSI error assertion with errors.As]

key-files:
  created: [test/e2e/scsierror_test.go]
  modified: [test/e2e/digest_test.go]

key-decisions:
  - "Reuse existing TestDigests pattern for asymmetric digest tests (same structure, single digest option)"
  - "LBA 200000 chosen for out-of-range tests (well beyond 131072-block / 64MB LUN capacity)"

patterns-established:
  - "SCSI error assertion pattern: errors.As with *uiscsi.SCSIError, check SenseKey/ASC/ASCQ fields"
  - "Asymmetric digest test: WithHeaderDigest or WithDataDigest alone, verify write+read cycle"

requirements-completed: [E2E-17, E2E-18, E2E-19, E2E-20]

# Metrics
duration: 2min
completed: 2026-04-02
---

# Phase 10 Plan 02: Asymmetric Digest and SCSI Error E2E Tests Summary

**Header-only and data-only CRC32C digest modes tested with write+read cycles, SCSI error handling verified with out-of-range LBA producing ILLEGAL_REQUEST sense data**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-02T23:19:13Z
- **Completed:** 2026-04-02T23:21:23Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Header-only CRC32C digest negotiation and data transfer verified against real LIO target
- Data-only CRC32C digest negotiation and data transfer verified against real LIO target
- Out-of-range LBA write returns SCSIError with ILLEGAL_REQUEST sense key (0x05), ASC 0x21/ASCQ 0x00
- Sense data parsing confirmed: SenseKey non-zero, Message non-empty, human-readable error includes "ILLEGAL REQUEST"

## Task Commits

Each task was committed atomically:

1. **Task 1: Header-only and data-only digest E2E tests** - `ce9d7e0` (test)
2. **Task 2: SCSI error condition E2E tests** - `740ad01` (test)

**Plan metadata:** TBD (docs: complete plan)

## Files Created/Modified
- `test/e2e/digest_test.go` - Added TestDigest_HeaderOnly and TestDigest_DataOnly (asymmetric digest modes)
- `test/e2e/scsierror_test.go` - New file with TestSCSIError_OutOfRangeLBA and TestSCSIError_SenseDataParsing

## Decisions Made
- Reused existing TestDigests structure for asymmetric tests -- same setup/cleanup, single digest option instead of both
- Used LBA 200000 for out-of-range tests, well beyond 131072-block LUN capacity boundary

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Digest coverage now includes all three modes: both, header-only, data-only
- SCSI error handling verified end-to-end with real target
- Ready for Plan 03 (multi-LUN and reconnection E2E tests)

---
*Phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps*
*Completed: 2026-04-02*
