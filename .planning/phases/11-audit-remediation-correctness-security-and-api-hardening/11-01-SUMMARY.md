---
phase: 11-audit-remediation-correctness-security-and-api-hardening
plan: 01
subsystem: transport, auth
tags: [iscsi, security, pdu, chap, error-handling]

requires:
  - phase: 02-connection-and-login
    provides: ReadRawPDU, CHAP, Dial error wrapping
provides:
  - MaxRecvDataSegmentLength enforcement in ReadRawPDU
  - Panic-free CHAP state initialization
  - Correct error classification in Dial (auth vs transport)
affects: [session, login, transport]

tech-stack:
  added: []
  patterns: [maxRecvDSL parameter threading through ReadPump]

key-files:
  created: []
  modified:
    - internal/transport/framer.go
    - internal/transport/conn.go
    - internal/transport/pump.go
    - internal/login/chap.go
    - internal/login/login.go
    - uiscsi.go

key-decisions:
  - "MaxRecvDSL passed as parameter to ReadRawPDU rather than stored in reader state — keeps framer stateless"
  - "maxRecvDSL=0 means unlimited (backward compatible for tests and login phase)"
  - "CHAP error return uses %w for wrapping so callers can inspect underlying crypto/rand errors"

patterns-established:
  - "Transport layer validates wire limits; session layer provides the negotiated values"

requirements-completed: [AUDIT-1, AUDIT-2, AUDIT-3]

duration: 12min
completed: 2026-04-03
---

# Plan 11-01: Critical Audit Fixes Summary

**MaxRecvDSL enforcement prevents memory exhaustion DoS, CHAP returns errors instead of panicking, non-auth login errors correctly classified as TransportError**

## Performance

- **Duration:** 12 min
- **Tasks:** 2
- **Files modified:** 20

## Accomplishments
- ReadRawPDU rejects PDUs with data segments exceeding MaxRecvDataSegmentLength
- newCHAPState returns error instead of panicking on entropy failure
- Dial() else branch wraps non-auth login errors as TransportError, not AuthError

## Task Commits

1. **Task 1: Fix CRITICAL security issues** - `a916e73` (fix)
2. **Task 2: Add tests for CRITICAL fixes** - `4849bba` (test)

## Files Created/Modified
- `internal/transport/framer.go` - Added maxRecvDSL parameter and enforcement check
- `internal/transport/conn.go` - Added MaxRecvDSL() getter
- `internal/transport/pump.go` - Threaded maxRecvDSL through ReadPump
- `internal/login/chap.go` - Changed newCHAPState to return (*chapState, error)
- `internal/login/login.go` - Propagated newCHAPState error
- `uiscsi.go` - Changed else branch from wrapAuthError to TransportError

## Decisions Made
None - followed plan as specified.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
- Existing AuthError tests needed updating for new format (from 11-02 changes cherry-picked earlier)

## Next Phase Readiness
- Transport layer now enforces MaxRecvDSL — session/connection replacement code can rely on it
- CHAP error path fully propagated — ready for concurrency fixes in Wave 2

---
*Phase: 11-audit-remediation-correctness-security-and-api-hardening*
*Completed: 2026-04-03*
