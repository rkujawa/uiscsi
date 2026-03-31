---
phase: 01-pdu-codec-and-transport
plan: 01
subsystem: protocol
tags: [iscsi, crc32c, serial-arithmetic, rfc1982, rfc7143, padding]

# Dependency graph
requires: []
provides:
  - RFC 1982 serial number arithmetic (LessThan, GreaterThan, InWindow, Incr)
  - CRC32C digest computation (HeaderDigest, DataDigest with padding)
  - PDU padding helper (PadLen with double-modulo formula)
  - Go module definition (github.com/rkujawa/uiscsi)
affects: [01-02, 01-03, 02-login-negotiation, 03-full-feature-phase]

# Tech tracking
tech-stack:
  added: [go-1.25, hash/crc32, encoding/binary]
  patterns: [table-driven-tests, tdd-red-green, internal-packages]

key-files:
  created:
    - go.mod
    - internal/serial/serial.go
    - internal/serial/serial_test.go
    - internal/digest/crc32c.go
    - internal/digest/crc32c_test.go
    - internal/pdu/padding.go
    - internal/pdu/padding_test.go
  modified: []

key-decisions:
  - "int32 cast trick for RFC 1982 serial comparison (int32(s1-s2) < 0)"
  - "Package-level crc32cTable via crc32.MakeTable(crc32.Castagnoli) for one-time init"
  - "Double-modulo padding formula (4 - (n % 4)) % 4 to avoid returning 4 for aligned inputs"

patterns-established:
  - "Table-driven tests with t.Run subtests for all pure functions"
  - "Package doc comments referencing RFC sections"
  - "Internal packages under internal/ for implementation hiding"

requirements-completed: [PDU-02, PDU-03, PDU-04]

# Metrics
duration: 3min
completed: 2026-03-31
---

# Phase 01 Plan 01: Foundation Utilities Summary

**RFC 1982 serial arithmetic with wrap-around, CRC32C digest with padding-inclusive DataDigest, and 4-byte PadLen helper -- all stdlib-only, all passing under -race**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-31T20:30:06Z
- **Completed:** 2026-03-31T20:32:53Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Go module initialized with github.com/rkujawa/uiscsi, Go 1.25
- Serial number arithmetic (LessThan, GreaterThan, InWindow, Incr) correctly handles 2^32 wrap-around
- CRC32C digest matches all 4 RFC test vectors; DataDigest includes zero-padding in CRC computation
- PadLen uses correct double-modulo formula returning 0-3 bytes

## Task Commits

Each task was committed atomically:

1. **Task 1: Go module init, serial arithmetic, and padding helpers** - `6e0782e` (test+feat)
2. **Task 2: CRC32C digest computation with RFC test vectors** - `591eb9b` (feat)

## Files Created/Modified
- `go.mod` - Go module definition (github.com/rkujawa/uiscsi, go 1.25)
- `internal/serial/serial.go` - RFC 1982 serial number arithmetic (LessThan, GreaterThan, InWindow, Incr)
- `internal/serial/serial_test.go` - Table-driven tests with wrap-around edge cases (99 lines)
- `internal/digest/crc32c.go` - CRC32C HeaderDigest and DataDigest with padding
- `internal/digest/crc32c_test.go` - RFC test vectors and padding-inclusion verification
- `internal/pdu/padding.go` - PadLen with double-modulo formula
- `internal/pdu/padding_test.go` - Boundary value tests for padding computation

## Decisions Made
- Used int32 cast trick for serial comparison per RFC 1982 Section 3 -- concise and correct
- Package-level crc32cTable computed once at init -- avoids repeated MakeTable calls
- Double-modulo padding formula (4 - (n % 4)) % 4 -- prevents returning 4 when n is aligned

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Serial arithmetic ready for CmdSN/StatSN/DataSN/R2TSN comparisons in PDU codec (plan 01-02)
- CRC32C digest ready for header/data digest verification in transport layer (plan 01-03)
- PadLen ready for PDU framing and data segment padding (plan 01-02)
- All foundations tested under -race, safe for concurrent use

## Self-Check: PASSED

All 7 files verified present. Both commit hashes (6e0782e, 591eb9b) found in git log.

---
*Phase: 01-pdu-codec-and-transport*
*Completed: 2026-03-31*
