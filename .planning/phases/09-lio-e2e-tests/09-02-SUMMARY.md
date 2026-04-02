---
phase: 09-lio-e2e-tests
plan: 02
subsystem: testing
tags: [e2e, lio, data-integrity, chap, digest, crc32c, multi-lun, tmf, error-recovery]

# Dependency graph
requires:
  - phase: 09-lio-e2e-tests-plan-01
    provides: test/lio/ helper package, test/e2e/ TestMain + TestBasicConnectivity
provides:
  - 6 E2E test files covering data integrity, CHAP, digests, multi-LUN, TMF, error recovery
  - Complete E2E test suite exercising all 7 critical protocol scenarios against real kernel target
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [ss -K for TCP connection kill in recovery tests, block-index pattern encoding for data integrity verification]

key-files:
  created: [test/e2e/data_test.go, test/e2e/chap_test.go, test/e2e/digest_test.go, test/e2e/multilun_test.go, test/e2e/tmf_test.go, test/e2e/recovery_test.go]
  modified: []

key-decisions:
  - "ss -K for TCP connection kill (requires root, which e2e tests already have) instead of TCP proxy or NP removal"
  - "AbortTask not tested (synchronous tests cannot create in-flight tasks to abort); LUNReset validates TMF path"
  - "Retry loop with backoff for post-reconnect Inquiry in recovery test"

patterns-established:
  - "Data integrity pattern: block-index-encoded test data for deterministic write/read verification"
  - "CHAP test pattern: separate tests for one-way, mutual, and bad-password scenarios"
  - "Recovery test pattern: ss -K TCP kill + retry loop for ERL 0 reconnect verification"

requirements-completed: [E2E-04, E2E-05, E2E-06, E2E-07, E2E-08, E2E-09]

# Metrics
duration: 3min
completed: 2026-04-02
---

# Phase 9 Plan 2: E2E Test Scenarios Summary

**6 E2E test files covering data integrity, CHAP auth, CRC32C digests, multi-LUN enumeration, TMF LUNReset, and ERL 0 connection recovery against real LIO kernel target**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-02T15:16:09Z
- **Completed:** 2026-04-02T15:18:48Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- TestDataIntegrity: write-then-read byte-for-byte verification at LBA 0 and non-zero LBA 100
- TestCHAP + TestCHAPMutual + TestCHAPBadPassword: one-way CHAP, mutual CHAP, and rejection of bad credentials
- TestDigests: CRC32C header+data digest negotiation with write+read cycle exercising digests in both directions
- TestMultiLUN: 3 LUNs (32/64/128MB) enumerated via ReportLuns, capacities verified via ReadCapacity
- TestTMF_LUNReset: LUN Reset TMF response code 0 (Function Complete), session survives
- TestErrorRecovery_ConnectionDrop: TCP kill via ss -K, ERL 0 reconnect with retry loop

## Task Commits

Each task was committed atomically:

1. **Task 1: Data integrity, CHAP, and digest E2E tests** - `c8044ab` (feat)
2. **Task 2: Multi-LUN, TMF, and error recovery E2E tests** - `6676f92` (feat)

## Files Created/Modified
- `test/e2e/data_test.go` - Data integrity write/read byte comparison at LBA 0 and LBA 100
- `test/e2e/chap_test.go` - One-way CHAP, mutual CHAP, and bad password rejection
- `test/e2e/digest_test.go` - CRC32C header+data digest negotiation with data transfer
- `test/e2e/multilun_test.go` - 3-LUN enumeration via ReportLuns + ReadCapacity + Inquiry
- `test/e2e/tmf_test.go` - LUN Reset TMF execution and post-reset session validation
- `test/e2e/recovery_test.go` - TCP connection drop via ss -K and ERL 0 reconnect

## Decisions Made
- Used `ss -K` for TCP connection kill in recovery test -- cleanest approach requiring only root (already required for e2e tests), no need for TCP proxy complexity or NP removal/recreation
- AbortTask not tested in E2E -- synchronous test cannot create in-flight tasks to abort; LUNReset validates the TMF infrastructure end-to-end since both use the same PDU type and response handling
- Recovery test uses retry loop (10 attempts with increasing backoff) to handle reconnect timing uncertainty

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unreachable fmt import in recovery_test.go**
- **Found during:** Task 2
- **Issue:** `fmt` import was unused -- the `_ = fmt.Sprintf` suppression was unreachable after `t.Fatalf`
- **Fix:** Removed the import and the unreachable line
- **Files modified:** test/e2e/recovery_test.go
- **Verification:** `go vet -tags e2e ./test/e2e/` passes clean

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Trivial fix for clean compilation. No scope change.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Complete E2E test suite with 8 test functions across 7 test files
- All tests gated behind //go:build e2e tag -- existing test suite unaffected
- Run with: `sudo go test -tags e2e -v -count=1 ./test/e2e/`

## Self-Check: PASSED

All 6 created files verified present. Both task commits (c8044ab, 6676f92) verified in git log.

---
*Phase: 09-lio-e2e-tests*
*Completed: 2026-04-02*
