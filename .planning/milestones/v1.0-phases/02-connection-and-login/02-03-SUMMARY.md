---
phase: 02-connection-and-login
plan: 03
subsystem: auth
tags: [iscsi, login, chap, mutual-chap, state-machine, functional-options, digest-negotiation]

# Dependency graph
requires:
  - phase: 02-connection-and-login (plan 01)
    provides: "text codec, negotiation engine, NegotiatedParams, LoginError"
  - phase: 02-connection-and-login (plan 02)
    provides: "CHAP authentication state machine (chapState, processChallenge, verifyMutualResponse)"
  - phase: 01-pdu-codec-and-transport
    provides: "PDU types (LoginReq, LoginResp), transport (Conn, ReadRawPDU, WriteRawPDU)"
provides:
  - "Login() function with functional options API for iSCSI login"
  - "Login state machine: SecurityNeg -> OperationalNeg -> FullFeaturePhase"
  - "Mock iSCSI target test harness for login testing"
  - "buildInitiatorKeys() for operational parameter proposal"
affects: [session-management, discovery, full-feature-phase]

# Tech tracking
tech-stack:
  added: []
  patterns: [functional-options-for-login, mock-target-test-harness, synchronous-pdu-exchange-during-login]

key-files:
  created:
    - internal/login/login.go
    - internal/login/login_test.go
  modified: []

key-decisions:
  - "Synchronous PDU exchange via raw net.Conn during login (not read/write pumps per Pitfall 5)"
  - "buildInitiatorKeys placed in login.go since it is login-specific configuration logic"
  - "Mock target uses loopback TCP (not net.Pipe) for realistic transport.Dial integration"
  - "CmdSN not incremented during login per RFC 7143 (Pitfall 10)"
  - "Digests activated only after login completes (Pitfall 6)"

patterns-established:
  - "Functional options pattern: LoginOption func(*loginConfig) with WithTarget, WithCHAP, etc."
  - "Mock target pattern: runMockTarget goroutine with mockTargetConfig for login test scenarios"
  - "Login state machine pattern: loginState struct drives CSG 0->1->3 transitions"

requirements-completed: [LOGIN-01, LOGIN-03, INTEG-01, INTEG-02, INTEG-03]

# Metrics
duration: 4min
completed: 2026-03-31
---

# Phase 02 Plan 03: Login State Machine Summary

**Login state machine with functional options API, CHAP/mutual-CHAP auth, digest negotiation, and mock target test harness with 10 integration tests**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-31T23:09:16Z
- **Completed:** 2026-03-31T23:13:30Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Login() function with 8 functional options (WithTarget, WithCHAP, WithMutualCHAP, WithHeaderDigest, WithDataDigest, WithInitiatorName, WithSessionType, WithISID)
- Login state machine driving SecurityNegotiation -> OperationalNegotiation -> FullFeaturePhase transitions
- Mock iSCSI target test harness supporting AuthMethod=None, CHAP, and mutual CHAP with configurable operational params
- 10 integration tests covering all auth paths, digest negotiation, error handling, and context cancellation -- all passing under -race

## Task Commits

Each task was committed atomically:

1. **Task 1: Login function, functional options, and state machine** - `9e9f64b` (feat)
2. **Task 2: Mock target test harness and login integration tests** - `16e6c2a` (test)

## Files Created/Modified
- `internal/login/login.go` - Login function, LoginOption type, loginState state machine, buildInitiatorKeys, sendLogin PDU helper
- `internal/login/login_test.go` - Mock target infrastructure (mockTargetConfig, runMockTarget), 10 integration tests

## Decisions Made
- Synchronous PDU exchange via raw net.Conn during login (not read/write pumps) per Pitfall 5
- buildInitiatorKeys placed in login.go since it constructs login-specific configuration, not reusable negotiation logic
- Mock target uses loopback TCP with net.Listen/transport.Dial for realistic integration testing
- CmdSN not incremented during login (Pitfall 10), StatSN tracked via expStatSN = resp.StatSN + 1 (Pitfall 9)
- Digests activated on transport.Conn only after successful login completes (Pitfall 6)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created buildInitiatorKeys in login.go**
- **Found during:** Task 1 (login state machine implementation)
- **Issue:** Plan referenced buildInitiatorKeys as existing in negotiation.go, but it was not implemented there
- **Fix:** Implemented buildInitiatorKeys in login.go as it is login-specific configuration logic
- **Files modified:** internal/login/login.go
- **Verification:** go build and go vet pass
- **Committed in:** 9e9f64b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Auto-fix necessary for task completion. Function placement in login.go is appropriate as it constructs login-specific key proposals.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Login state machine complete, ready for session-level integration
- SendTargets discovery can use Login with WithSessionType("Discovery")
- Full feature phase development can proceed with authenticated sessions

---
*Phase: 02-connection-and-login*
*Completed: 2026-03-31*
