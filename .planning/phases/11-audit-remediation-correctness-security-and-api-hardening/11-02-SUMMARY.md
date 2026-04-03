---
phase: 11-audit-remediation-correctness-security-and-api-hardening
plan: 02
subsystem: session, errors, pdu
tags: [iscsi, error-handling, pdu-validation, backoff]

requires:
  - phase: 02-connection-and-login
    provides: Session, errors, PDU encoding
provides:
  - Residual overflow detection in submitAndCheck
  - Unparseable sense data reporting
  - CDB length validation in Execute
  - Safe backoff arithmetic
  - AuthError with status codes
  - 24-bit DSL validation in encodeDataSegmentLength
  - AHS type/length validation
affects: [session, errors, pdu]

tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - session.go
    - errors.go
    - internal/session/recovery.go
    - internal/pdu/bhs.go
    - internal/pdu/ahs.go

key-decisions:
  - "encodeDataSegmentLength uses panic for >24-bit values — programmer error, not runtime"
  - "SCSIError intentionally does not implement Unwrap — documented as leaf error"
  - "Unknown AHS types accepted for forward compatibility"

patterns-established: []

requirements-completed: [AUDIT-8, AUDIT-9, AUDIT-10, AUDIT-11, AUDIT-12, AUDIT-13, AUDIT-16, AUDIT-17]

duration: 10min
completed: 2026-04-03
---

# Plan 11-02: Medium/Low Audit Fixes Summary

**Residual overflow detection, CDB validation, safe backoff, improved error messages, PDU encoding validation, and AHS type checking**

## Performance

- **Duration:** 10 min
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- submitAndCheck detects residual overflow and reports unparseable sense data
- Execute rejects CDBs > 16 bytes and empty CDBs
- Reconnect backoff uses max(0, attempt-1) to prevent negative shifts
- AuthError.Error() includes StatusClass and StatusDetail
- encodeDataSegmentLength panics on 24-bit overflow
- UnmarshalAHS validates type and enforces max data length

## Task Commits

1. **Task 1: Fix session.go issues** - `a28251f` (fix)
2. **Task 2: Fix error types, backoff, PDU validation** - `9e2920f` (fix)
3. **Task 3: Add tests for MEDIUM/LOW fixes** - `33b117d` (test)

## Decisions Made
- SCSIError.Unwrap documented as intentionally absent (leaf error type)

## Deviations from Plan
- Existing AuthError tests needed updating for new format (minor test expectation change)

## Issues Encountered
None.

## Next Phase Readiness
- Session layer hardened — ready for concurrency fixes in 11-03
- Error hierarchy complete — Wave 2 can build on it

---
*Phase: 11-audit-remediation-correctness-security-and-api-hardening*
*Completed: 2026-04-03*
