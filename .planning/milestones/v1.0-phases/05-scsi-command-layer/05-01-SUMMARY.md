---
phase: 05-scsi-command-layer
plan: 01
subsystem: scsi
tags: [scsi, cdb, sense-data, spc-4, sbc-3, binary-encoding]

requires:
  - phase: 03-full-feature-phase
    provides: session.Command and session.Result types for CDB transport
provides:
  - internal/scsi/ package with shared types (opcodes, status, SenseKey, CommandError)
  - Sense data parsing (fixed 0x70/0x71 + descriptor 0x72/0x73 formats)
  - 8 CDB builder functions returning session.Command
  - 6 response parser functions returning typed structs
  - Functional options for CDB flags (WithFUA, WithDBD, etc.)
affects: [05-02-PLAN, 05-03-PLAN, high-level-api]

tech-stack:
  added: []
  patterns: [plain-functions-returning-session.Command, checkResult-helper, functional-options-for-CDB-flags, table-driven-golden-byte-tests]

key-files:
  created:
    - internal/scsi/opcode.go
    - internal/scsi/sense.go
    - internal/scsi/sense_test.go
    - internal/scsi/commands.go
    - internal/scsi/commands_test.go
    - internal/scsi/inquiry.go
    - internal/scsi/inquiry_test.go
    - internal/scsi/capacity.go
    - internal/scsi/capacity_test.go
    - internal/scsi/modesense.go
    - internal/scsi/modesense_test.go
  modified: []

key-decisions:
  - "~70 ASC/ASCQ entries in lookup table covering common SPC-4 Annex D codes"
  - "checkResult helper centralizes status check + sense parse + data read for all parse functions"
  - "WithDBD and WithPageControl options added to modesense.go alongside shared options in opcode.go"

patterns-established:
  - "CDB builder pattern: plain function returns session.Command with packed CDB bytes"
  - "Parse pattern: function takes session.Result, calls checkResult, validates length, returns typed struct"
  - "Defensive copy: Raw []byte fields in response structs are copies, not slices of input"
  - "TDD with golden byte CDB verification in table-driven tests"

requirements-completed: [SCSI-01, SCSI-02, SCSI-04, SCSI-07, SCSI-08, SCSI-09, SCSI-10]

duration: 6min
completed: 2026-04-01
---

# Phase 5 Plan 1: SCSI Foundation Types and Core Commands Summary

**internal/scsi/ package with sense parsing (fixed+descriptor), 14 SenseKey values, CommandError, and 8 CDB builders for TUR/INQUIRY/RC10/RC16/REQUEST SENSE/REPORT LUNS/MODE SENSE 6+10**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-01T12:42:02Z
- **Completed:** 2026-04-01T12:48:05Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- Created internal/scsi/ package with all shared SCSI types: 24 opcode constants, 8 status codes, 14 SenseKey values with String(), functional options, and CommandError with IsSenseKey
- ParseSense handles both fixed (0x70/0x71) and descriptor (0x72/0x73) sense formats with ~70 ASC/ASCQ lookup entries
- 8 CDB builder functions (TestUnitReady, RequestSense, ReportLuns, Inquiry, ReadCapacity10, ReadCapacity16, ModeSense6, ModeSense10) all returning session.Command
- 6 response parsers (ParseReportLuns, ParseInquiry, ParseReadCapacity10, ParseReadCapacity16, ParseModeSense6, ParseModeSense10) with proper error handling via checkResult helper
- 48 tests across 5 test files, all passing under -race and go vet

## Task Commits

Each task was committed atomically (TDD: RED then GREEN):

1. **Task 1: Foundation types, sense parsing, and CommandError**
   - `6bb26df` (test: failing sense tests - RED)
   - `779ea78` (feat: implementation - GREEN)
2. **Task 2: Core command CDB builders and response parsers**
   - `c73274d` (test: failing command tests - RED)
   - `6f47d64` (feat: implementation - GREEN)

## Files Created/Modified
- `internal/scsi/opcode.go` - Opcode constants, SCSI status, SenseKey enum, Option type, CommandError, checkResult helper
- `internal/scsi/sense.go` - SenseData struct, ParseSense (fixed+descriptor), ASC/ASCQ lookup table
- `internal/scsi/sense_test.go` - ParseSense, SenseKey.String(), IsSenseKey, CommandError tests
- `internal/scsi/commands.go` - TestUnitReady, RequestSense, ReportLuns, ParseReportLuns
- `internal/scsi/commands_test.go` - CDB byte verification and parse tests for core commands
- `internal/scsi/inquiry.go` - Inquiry CDB builder, InquiryResponse, ParseInquiry with space trimming
- `internal/scsi/inquiry_test.go` - Inquiry CDB and parse tests including CHECK CONDITION
- `internal/scsi/capacity.go` - ReadCapacity10/16 CDB builders and parsers, RC16 SERVICE ACTION IN pattern
- `internal/scsi/capacity_test.go` - RC10/RC16 CDB byte and response parse tests
- `internal/scsi/modesense.go` - ModeSense6/10 with WithDBD/WithPageControl, parsers for header+BD+pages
- `internal/scsi/modesense_test.go` - ModeSense CDB and parse tests with various option combinations

## Decisions Made
- ~70 ASC/ASCQ entries covering common codes from SPC-4 Annex D (balance between coverage and maintainability)
- checkResult helper centralizes the status/sense/data pipeline for all parse functions
- WithDBD and WithPageControl live alongside the command-specific code in modesense.go since they are MODE SENSE-specific, while general options (WithFUA, WithDPO, etc.) remain in opcode.go

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - all functions are fully implemented with no placeholder data.

## Next Phase Readiness
- Foundation types (opcodes, SenseKey, CommandError, Option) ready for Plan 2 (read/write/verify) and Plan 3 (VPD/provisioning/reservations)
- checkResult pattern established for all future parse functions
- internal/scsi/ package compiles cleanly and tests pass under -race

---
*Phase: 05-scsi-command-layer*
*Completed: 2026-04-01*
