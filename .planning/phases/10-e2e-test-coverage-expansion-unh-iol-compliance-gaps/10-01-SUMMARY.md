---
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
plan: 01
subsystem: testing
tags: [e2e, lio, iscsi, r2t, negotiation, configfs]

requires:
  - phase: 09-lio-e2e-tests
    provides: LIO test infrastructure (lio.Setup, RequireRoot, RequireModules)
  - phase: 04-write-path
    provides: R2T/Data-Out engine, ImmediateData x InitialR2T matrix
provides:
  - WithOperationalOverrides public Option for login parameter override
  - Large write multi-R2T E2E test (1MB, 2048 blocks)
  - ImmediateData x InitialR2T 2x2 negotiation matrix E2E test
affects: [10-02, 10-03, public-api]

tech-stack:
  added: []
  patterns:
    - "WithOperationalOverrides patches buildInitiatorKeys output via map overlay"
    - "configfs param/ writes to set target-side negotiation preferences"

key-files:
  created:
    - test/e2e/largewrite_test.go
    - test/e2e/negotiation_test.go
  modified:
    - internal/login/login.go
    - options.go

key-decisions:
  - "operationalOverrides patches in-place without changing key order or adding new keys"
  - "1MB write at default MaxBurstLength=262144 triggers ~4 R2T sequences implicitly"
  - "Target-side configfs param/ writes ensure both sides agree on negotiation outcome"

patterns-established:
  - "WithOperationalOverrides pattern: map[string]string overlay on buildInitiatorKeys defaults"

requirements-completed: [E2E-11, E2E-12, E2E-20]

duration: 2min
completed: 2026-04-02
---

# Phase 10 Plan 01: WithOperationalOverrides and Multi-R2T / Negotiation Matrix E2E Tests Summary

**WithOperationalOverrides login option enabling 1MB multi-R2T data transfer and 2x2 ImmediateData x InitialR2T negotiation matrix E2E tests against real LIO target**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-02T23:19:12Z
- **Completed:** 2026-04-02T23:21:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- WithOperationalOverrides option in public API patches login negotiation keys via map overlay
- TestLargeWrite_MultiR2T verifies 1MB (2048 block) write+read integrity exercising multi-R2T sequences
- TestNegotiation_ImmediateDataInitialR2T covers all 4 ImmediateData x InitialR2T combinations with configfs target-side configuration and data integrity verification
- All tests skip gracefully when not root or kernel modules not loaded

## Task Commits

Each task was committed atomically:

1. **Task 1: Add WithOperationalOverrides login option** - `c5f606e` (feat)
2. **Task 2: Large write multi-R2T and negotiation matrix E2E tests** - `7cc5ba3` (test)

## Files Created/Modified
- `internal/login/login.go` - Added operationalOverrides field, WithOperationalOverrides LoginOption, override loop in buildInitiatorKeys
- `options.go` - Added public WithOperationalOverrides Option wrapping login.WithOperationalOverrides
- `test/e2e/largewrite_test.go` - 1MB multi-R2T write+read data integrity test
- `test/e2e/negotiation_test.go` - 2x2 ImmediateData x InitialR2T matrix with configfs target configuration

## Decisions Made
- operationalOverrides patches in-place (iterate default keys, replace matching values) without changing key order or adding new keys
- 1MB write at default MaxBurstLength=262144 triggers ~4 R2T sequences implicitly without needing explicit R2T count verification
- Target-side configfs param/ writes ensure both initiator and target agree on negotiation outcome, avoiding Reject responses

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed type mismatch in test code**
- **Found during:** Task 2 (E2E test creation)
- **Issue:** numBlocks declared as int but WriteBlocks/ReadBlocks require uint32; XOR expression had mismatched int/byte types
- **Fix:** Declared numBlocks as uint32, fixed byte XOR expression with proper casting
- **Files modified:** test/e2e/largewrite_test.go, test/e2e/negotiation_test.go
- **Verification:** go vet -tags e2e ./test/e2e/ passes
- **Committed in:** 7cc5ba3 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Type fix necessary for compilation. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- WithOperationalOverrides available for plan 10-02 and 10-03 tests that need parameter override
- E2E test patterns for configfs target configuration established

## Self-Check: PASSED

All 4 files exist. Both commit hashes (c5f606e, 7cc5ba3) verified. All must_haves key_links present.

---
*Phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps*
*Completed: 2026-04-02*
