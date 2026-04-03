---
phase: 06-error-recovery-and-task-management
plan: 02
subsystem: session
tags: [iscsi, erl0, reconnect, recovery, session-reinstatement, exponential-backoff]

# Dependency graph
requires:
  - phase: 06-01
    provides: "TMF framework, FaultConn, recovery session options (maxReconnectAttempts, reconnectBackoff)"
provides:
  - "ERL 0 automatic session reinstatement after connection failure"
  - "WithTSIH login option for session reinstatement"
  - "WithReconnectInfo session option for reconnect context"
  - "Transparent in-flight command retry after reconnect"
  - "Seekable write retry via io.Seeker check"
  - "ErrRetryNotPossible and ErrSessionRecovering sentinels"
affects: [06-03, session-api]

# Tech tracking
tech-stack:
  added: []
  patterns: ["ERL 0 reconnect FSM: detect->stop->snapshot->redial->relogin->replace->retry", "triggerReconnect single-flight pattern", "Wait for old goroutines before replacing channels"]

key-files:
  created: ["internal/session/recovery.go", "internal/session/recovery_test.go"]
  modified: ["internal/login/login.go", "internal/login/params.go", "internal/session/session.go", "internal/session/async.go", "internal/session/types.go", "internal/session/datain.go"]

key-decisions:
  - "Wait for old dispatchLoop done channel before replacing session channels to avoid data races"
  - "Direct connection close for test fault injection instead of FaultConn (cleaner for reconnect tests)"
  - "Task stores original Command for retry, reuses original resultCh for transparent caller experience"
  - "Close() captures s.conn under lock to avoid race with concurrent reconnect"

patterns-established:
  - "Reconnect FSM: cancel context + close conn -> wait for done -> snapshot tasks -> backoff dial -> reinstate login -> replace internals -> retry tasks"
  - "Session field access under s.mu in Close() for race safety with background reconnect"

requirements-completed: [ERL-01, TEST-05]

# Metrics
duration: 10min
completed: 2026-04-01
---

# Phase 06 Plan 02: ERL 0 Reconnect Summary

**Automatic session reinstatement with same ISID+TSIH after connection failure, transparent in-flight command retry with exponential backoff**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-01T16:17:43Z
- **Completed:** 2026-04-01T16:27:30Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- ERL 0 reconnect FSM: detect connection loss, stop old pumps, snapshot in-flight tasks, exponential backoff re-dial, re-login with same ISID+TSIH (RFC 7143 Section 6.3.5), replace session internals, retry tasks
- WithTSIH login option enables session reinstatement; ISID field added to NegotiatedParams
- Write commands with io.Seeker retry via Seek(0, SeekStart); non-seekable writes fail with descriptive ErrRetryNotPossible
- 5 integration tests covering read retry, write seekable retry, non-seekable failure, max attempts exhaustion, and submit-during-recovery blocking

## Task Commits

Each task was committed atomically:

1. **Task 1: WithTSIH login option and reconnect FSM** - `a23f545` (feat)
2. **Task 2: ERL 0 reconnect integration tests** - `aa07b69` (test)

## Files Created/Modified
- `internal/session/recovery.go` - ERL 0 reconnect FSM: triggerReconnect, reconnect, retryTasks
- `internal/session/recovery_test.go` - Integration tests with recoverableTarget mock
- `internal/login/login.go` - Added WithTSIH LoginOption, wired tsih to loginState
- `internal/login/params.go` - Added ISID [6]byte to NegotiatedParams
- `internal/session/session.go` - Recovery gate in Submit, reconnect fields, race-safe Close()
- `internal/session/async.go` - AsyncEvent 2 triggers reconnect when configured
- `internal/session/types.go` - WithReconnectInfo option, targetAddr/loginOpts in sessionConfig
- `internal/session/datain.go` - cmd Command field in task struct for retry

## Decisions Made
- Wait for old dispatchLoop's done channel to close before replacing session channels -- prevents data race on s.done and s.unsolCh
- Capture s.conn under s.mu in Close() to avoid race with concurrent reconnect goroutine
- Read s.err under s.mu in Close() before attempting graceful logout
- Use direct tc.NetConn().Close() for connection drop simulation in tests instead of FaultConn (cleaner for these specific scenarios)
- Store original Command in task struct for retry -- reuse same resultCh so caller sees transparent recovery

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed data race on s.done and s.unsolCh during reconnect**
- **Found during:** Task 2 (integration tests with -race)
- **Issue:** reconnect() replaced s.done and s.unsolCh while old dispatchLoop goroutine was still reading them
- **Fix:** Capture old done channel before cancel(), wait for it to close before replacing session fields
- **Files modified:** internal/session/recovery.go
- **Verification:** All tests pass with -race flag
- **Committed in:** aa07b69 (Task 2 commit)

**2. [Rule 1 - Bug] Fixed data race on s.conn and s.err in Close()**
- **Found during:** Task 2 (integration tests with -race)
- **Issue:** Close() read s.conn and s.err without lock while reconnect() could be writing them
- **Fix:** Capture s.conn under s.mu before closing; read s.err under s.mu before logout check
- **Files modified:** internal/session/session.go
- **Verification:** All tests pass with -race flag
- **Committed in:** aa07b69 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both fixes essential for correctness under concurrent access. No scope creep.

## Issues Encountered
None beyond the race conditions caught by -race flag and fixed inline.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- ERL 0 foundation complete, session automatically reconnects and retries in-flight commands
- Ready for Plan 03 (ERL 1/2 or further error recovery refinement)
- recoverableTarget mock provides reusable test infrastructure for future recovery tests

---
*Phase: 06-error-recovery-and-task-management*
*Completed: 2026-04-01*
