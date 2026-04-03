---
phase: 02-connection-and-login
plan: 01
subsystem: login
tags: [iscsi, negotiation, rfc7143, text-codec, login-error]

requires:
  - phase: 01-pdu-codec-and-transport
    provides: "LoginReq/LoginResp PDU types with Data field for key-value pairs"
provides:
  - "Text key-value codec (EncodeTextKV/DecodeTextKV) for iSCSI text format"
  - "Declarative negotiation engine with all 14 mandatory RFC 7143 Section 13 keys"
  - "NegotiatedParams struct with compile-time safe typed fields"
  - "LoginError type with StatusClass/StatusDetail for login failure handling"
affects: [02-connection-and-login, 03-full-feature-phase]

tech-stack:
  added: []
  patterns: [declarative-key-registry, table-driven-negotiation, typed-params-over-maps]

key-files:
  created:
    - internal/login/textcodec.go
    - internal/login/textcodec_test.go
    - internal/login/negotiation.go
    - internal/login/negotiation_test.go
    - internal/login/params.go
    - internal/login/errors.go
  modified: []

key-decisions:
  - "Declarative key registry pattern: all 14 keys defined as KeyDef structs with type, default, min/max range, and RFC reference"
  - "NegotiatedParams uses typed fields (bool/uint32) not map[string]string for compile-time safety"
  - "Declarative negotiation type uses target's value (initiator reads target's declared capability)"
  - "Post-negotiation clamping: FirstBurstLength automatically capped to MaxBurstLength"

patterns-established:
  - "KeyDef registry: negotiation behavior driven by data, not per-key code paths"
  - "resolveKey dispatcher: single function handles all 6 negotiation types via switch"
  - "applyNegotiatedKeys: generic KeyValue slice to typed struct mapping"

requirements-completed: [LOGIN-02, LOGIN-06, TEST-04]

duration: 3min
completed: 2026-03-31
---

# Phase 02 Plan 01: Login Negotiation Foundation Summary

**Text key-value codec, declarative negotiation engine for all 14 RFC 7143 Section 13 mandatory keys, typed NegotiatedParams, and LoginError with status codes**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-31T23:02:38Z
- **Completed:** 2026-03-31T23:05:48Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Text key-value codec round-trips null-delimited pairs with byte-perfect fidelity and deterministic ordering
- Declarative key registry covers all 14 mandatory parameters with correct negotiation types (ListSelect, BooleanAnd, BooleanOr, NumericalMin, NumericalMax, Declarative)
- NegotiatedParams struct provides compile-time safe typed fields instead of map[string]string
- LoginError with StatusClass/StatusDetail works with errors.As() for structured error handling
- 21 tests with full negotiation matrix coverage per TEST-04

## Task Commits

Each task was committed atomically:

1. **Task 1: Text codec, NegotiatedParams, and LoginError** - `b0e45af` (feat)
2. **Task 2: Declarative negotiation engine with parameterized tests** - `5b4dcdd` (feat)

## Files Created/Modified
- `internal/login/textcodec.go` - EncodeTextKV/DecodeTextKV for iSCSI null-delimited text format
- `internal/login/textcodec_test.go` - 7 test cases: round-trip, byte-exact, empty, trailing null, empty value, comma-separated, order preserved
- `internal/login/params.go` - NegotiatedParams with 14 typed fields + Defaults() with RFC values
- `internal/login/errors.go` - LoginError type, 11 status constants, statusMessage helper
- `internal/login/negotiation.go` - NegotiationType enum, KeyDef, keyRegistry (14 keys), resolveKey, applyNegotiatedKeys
- `internal/login/negotiation_test.go` - 10 test functions: BooleanAnd (4 subtests), BooleanOr (4), NumericalMin (4), NumericalMax (3), ListSelect (4), Declarative, DeclarativeClamped, FullParams, FirstBurstClamping, RegistryCompleteness

## Decisions Made
- Declarative key registry pattern: negotiation behavior defined by data (KeyDef structs) not per-key code branches
- NegotiatedParams uses typed Go fields for compile-time safety per D-05
- Declarative negotiation uses target's declared value (MaxRecvDataSegmentLength is per-direction, initiator reads target's capability)
- Post-negotiation clamping enforces FirstBurstLength <= MaxBurstLength invariant

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Text codec ready for LoginReq/LoginResp Data field encoding/decoding
- Negotiation engine ready for login state machine (Plan 03) to call after operational negotiation stage
- LoginError ready for CHAP auth (Plan 02) and state machine error handling
- keyRegistry available for buildInitiatorKeys in future plans

## Self-Check: PASSED

- All 6 created files verified present
- Both task commits verified: b0e45af, 5b4dcdd
- 21/21 tests passing, go vet clean, no race conditions

---
*Phase: 02-connection-and-login*
*Completed: 2026-03-31*
