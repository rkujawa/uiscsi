---
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
plan: 03
subsystem: testing
tags: [iscsi, tmf, abort-task, warm-reset, erl, snack, connection-replacement, e2e]

# Dependency graph
requires:
  - phase: 10-01
    provides: WithOperationalOverrides for ERL negotiation, WithPDUHook for ITT capture
provides:
  - ABORT TASK E2E test with ITT capture via PDU hook during concurrent command
  - TARGET WARM RESET E2E test with session drop handling and re-establishment
  - ERL 1 SNACK recovery negotiation test with best-effort t.Skip fallback
  - ERL 2 connection replacement test with ss -K kill and recovery verification
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "PDU hook ITT capture with sync.Once + channel synchronization"
    - "configfs param write helper for target-side ERL configuration"
    - "best-effort ERL test pattern with t.Skip fallback per D-04"

key-files:
  created:
    - test/e2e/erl_test.go
  modified:
    - test/e2e/tmf_test.go

key-decisions:
  - "Accept both TMF response 0 (Function Complete) and 5 (Task Does Not Exist) for AbortTask since command may complete before abort"
  - "ERL 1/2 tests are best-effort per D-04 -- document negotiation capability as primary outcome"
  - "Use setTargetParam helper for configfs ErrorRecoveryLevel writes with skip on failure"

patterns-established:
  - "PDU hook ITT capture: sync.Once + atomic + channel for concurrent hook synchronization"
  - "ERL configfs setup: setTargetParam writes to /sys/kernel/config/target/iscsi/{iqn}/tpgt_1/param/{key}"

requirements-completed: [E2E-13, E2E-14, E2E-15, E2E-16, E2E-20]

# Metrics
duration: 2min
completed: 2026-04-02
---

# Phase 10 Plan 03: TMF and ERL E2E Tests Summary

**ABORT TASK/TARGET WARM RESET TMF tests with PDU hook ITT capture, plus ERL 1/2 best-effort negotiation tests against real LIO target**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-02T23:25:17Z
- **Completed:** 2026-04-02T23:27:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- TestTMF_AbortTask captures ITT of in-flight SCSI command via PDU hook and sends AbortTask TMF during concurrent read
- TestTMF_TargetWarmReset handles session drop gracefully and re-establishes new session to verify target alive
- TestERL1_SNACKRecovery negotiates ERL 1 with target via configfs + WithOperationalOverrides, verifies session functionality
- TestERL2_ConnectionReplacement negotiates ERL 2, kills TCP via ss -K, verifies session recovery (ERL 2 or ERL 0 fallback)

## Task Commits

Each task was committed atomically:

1. **Task 1: ABORT TASK and TARGET WARM RESET TMF E2E tests** - `06d92f7` (feat)
2. **Task 2: ERL 1/2 E2E tests with best-effort t.Skip fallback** - `3800957` (feat)

## Files Created/Modified
- `test/e2e/tmf_test.go` - Added TestTMF_AbortTask (PDU hook ITT capture + concurrent abort) and TestTMF_TargetWarmReset (session drop + re-establishment)
- `test/e2e/erl_test.go` - New file with TestERL1_SNACKRecovery and TestERL2_ConnectionReplacement, setTargetParam configfs helper

## Decisions Made
- Accept TMF response 0 or 5 for AbortTask (command may complete before abort arrives)
- ERL tests are best-effort per D-04: negotiation + basic functionality is primary outcome
- ERL 2 recovery test accepts both ERL 2 connection replacement and ERL 0 fallback as valid outcomes

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All Phase 10 TMF and ERL E2E tests complete
- Tests skip gracefully when not root or kernel modules not loaded
- Ready for phase verification

---
*Phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps*
*Completed: 2026-04-02*
