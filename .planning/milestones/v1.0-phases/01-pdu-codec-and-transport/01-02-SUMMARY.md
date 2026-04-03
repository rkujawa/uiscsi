---
phase: 01-pdu-codec-and-transport
plan: 02
subsystem: protocol
tags: [iscsi, pdu, bhs, opcode, ahs, rfc7143, encoding/binary, codec]

# Dependency graph
requires:
  - phase: 01-pdu-codec-and-transport plan 01
    provides: PadLen helper for 4-byte alignment, Go module definition
provides:
  - All 18 iSCSI opcode constants with String/IsInitiator/IsTarget methods
  - BHS encode/decode helpers (24-bit DataSegmentLength, opcode byte with immediate bit)
  - PDU interface (Opcode, MarshalBHS, DataSegment) and Header base struct
  - AHS marshal/unmarshal with 4-byte alignment
  - 8 initiator PDU types with MarshalBHS/UnmarshalBHS (NOPOut, SCSICommand, TaskMgmtReq, LoginReq, TextReq, DataOut, LogoutReq, SNACKReq)
  - 10 target PDU types with MarshalBHS/UnmarshalBHS (NOPIn, SCSIResponse, TaskMgmtResp, LoginResp, TextResp, DataIn, LogoutResp, R2T, AsyncMsg, Reject)
  - DecodeBHS dispatcher returning concrete PDU types by opcode
  - EncodePDU for wire-ready BHS + data segment + padding
affects: [01-03, 02-login-negotiation, 03-full-feature-phase, 04-error-recovery]

# Tech tracking
tech-stack:
  added: [encoding/binary]
  patterns: [typed-pdu-per-opcode, common-header-embed, bhs-marshal-unmarshal, opcode-dispatch-switch]

key-files:
  created:
    - internal/pdu/opcode.go
    - internal/pdu/opcode_test.go
    - internal/pdu/bhs.go
    - internal/pdu/bhs_test.go
    - internal/pdu/header.go
    - internal/pdu/ahs.go
    - internal/pdu/ahs_test.go
    - internal/pdu/initiator.go
    - internal/pdu/target.go
    - internal/pdu/pdu.go
    - internal/pdu/pdu_test.go
  modified: []

key-decisions:
  - "Typed PDU per opcode with embedded Header base struct (D-01/D-03 compliance)"
  - "Manual byte manipulation for sub-byte BHS fields (flags, CSG/NSG bits) with encoding/binary for 16/32-bit fields"
  - "3-byte manual encoding for DataSegmentLength (bytes 5-7) to avoid TotalAHSLength corruption (Pitfall 2)"
  - "Login PDU byte 1 reuses Final bit position for Transit bit (T), matching RFC 7143 Section 11.12"

patterns-established:
  - "Typed PDU per opcode: each opcode gets its own Go struct embedding Header"
  - "marshalHeader/unmarshalHeader for shared first 20 bytes of BHS"
  - "DecodeBHS switch-on-opcode dispatcher returning concrete PDU types"
  - "EncodePDU computes padding via PadLen and returns wire-ready bytes"

requirements-completed: [PDU-01, PDU-04, TEST-03]

# Metrics
duration: 6min
completed: 2026-03-31
---

# Phase 01 Plan 02: PDU Codec Summary

**Complete iSCSI PDU codec with all 18 opcode types (8 initiator, 10 target), BHS marshal/unmarshal, AHS support, and 30+ round-trip tests passing under -race**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-31T20:35:39Z
- **Completed:** 2026-03-31T20:41:38Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- All 18 iSCSI opcode types implemented as typed Go structs with byte-perfect BHS marshaling per RFC 7143 Section 11
- 24-bit DataSegmentLength encoding proven safe via Pitfall 2 regression test (TotalAHSLength at byte 4 preserved)
- AHS marshal/unmarshal with 4-byte alignment for ExtendedCDB and BidiReadDataLen
- 30+ tests including round-trips for all 18 opcodes, CSG/NSG bit packing, residual flags, S-bit/Status, reserved ITT=0xFFFFFFFF, padding verification

## Task Commits

Each task was committed atomically:

1. **Task 1: Opcode constants, BHS helpers, PDU interface, Header, AHS, and all PDU type structs** - `98e9e80` (feat)
2. **Task 2: EncodePDU dispatch and comprehensive round-trip tests** - `61dbc84` (feat)

## Files Created/Modified
- `internal/pdu/opcode.go` - 18 opcode constants with String(), IsInitiator(), IsTarget()
- `internal/pdu/opcode_test.go` - Opcode string, classification, and uniqueness tests
- `internal/pdu/bhs.go` - BHSLength constant, 24-bit DataSegmentLength encode/decode, opcode byte helpers
- `internal/pdu/bhs_test.go` - Round-trip tests, TotalAHSLength corruption regression (0xAA)
- `internal/pdu/header.go` - PDU interface, Header base struct, DecodeBHS opcode dispatcher
- `internal/pdu/ahs.go` - AHS types, MarshalAHS/UnmarshalAHS with 4-byte alignment
- `internal/pdu/ahs_test.go` - AHS round-trip, empty input, truncation error tests
- `internal/pdu/initiator.go` - 8 initiator types: NOPOut, SCSICommand, TaskMgmtReq, LoginReq, TextReq, DataOut, LogoutReq, SNACKReq
- `internal/pdu/target.go` - 10 target types: NOPIn, SCSIResponse, TaskMgmtResp, LoginResp, TextResp, DataIn, LogoutResp, R2T, AsyncMsg, Reject
- `internal/pdu/pdu.go` - EncodePDU: BHS + data segment + zero-padding
- `internal/pdu/pdu_test.go` - 30+ tests: round-trips for all 18 opcodes, edge cases, regressions

## Decisions Made
- Used typed PDU per opcode with embedded Header (D-01/D-03): each type has its own struct with MarshalBHS/UnmarshalBHS. Type safety at compile time.
- Manual 3-byte encoding for DataSegmentLength instead of binary.BigEndian.PutUint32 to avoid corrupting byte 4 (TotalAHSLength). Verified by regression test.
- Login PDU byte 1 handled specially: Transit (T) bit occupies same position as Final bit in other PDUs. Login marshal clears byte 1 before setting T/C/CSG/NSG.
- encoding/binary.BigEndian used for all 16/32-bit BHS fields; manual bit manipulation for sub-byte fields (flags, CSG/NSG, reason codes).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Complete PDU codec ready for transport framing (plan 01-03): read BHS -> DecodeBHS -> read data segment
- EncodePDU produces wire-ready bytes for transport write pump
- All types implement PDU interface for uniform handling in session/transport layers
- Login PDU types ready for Phase 2 (login negotiation)
- SCSI Command/Response/Data-In/Data-Out types ready for Phase 3 (full feature phase)
- R2T, TaskMgmt, SNACK, Reject types ready for Phase 4 (error recovery)

## Self-Check: PASSED

All 11 files verified present. Both commit hashes (98e9e80, 61dbc84) found in git log.

---
*Phase: 01-pdu-codec-and-transport*
*Completed: 2026-03-31*
