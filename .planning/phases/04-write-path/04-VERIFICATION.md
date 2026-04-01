---
phase: 04-write-path
verified: 2026-04-01T00:00:00Z
status: gaps_found
score: 4/5 must-haves verified
gaps:
  - truth: "Full session test suite passes with write path included"
    status: partial
    reason: "go test ./internal/session/ -count=1 -race -timeout 120s times out. Every test session takes ~5s to close due to the graceful logout timeout in Close(). 44 tests x ~5s = ~220s, which exceeds the 120s default timeout. All individual tests pass when run with sufficient timeout. The 5s logout timeout was introduced in Phase 3 (commit 1735202), not Phase 4. Phase 4 added 10 new tests (+50s), making the cumulative total exceed 120s. The issue is not a Phase 4 regression per se, but Phase 4 broke the suite's ability to finish in the default timeout."
    artifacts:
      - path: "internal/session/dataout_test.go"
        issue: "10 new tests x 5s each = 50s added to suite runtime; combined with pre-existing Phase 3 tests the suite now takes ~220s"
    missing:
      - "Either reduce the logout timeout in test sessions (newTestSession should use a shorter Close timeout) or the test suite must be run with -timeout 300s. The test helper newTestSession in session_test.go should call Close() with a short timeout or test sessions should not attempt graceful logout."
---

# Phase 4: Write Path Verification Report

**Phase Goal:** A Go application can write data to an iSCSI target through all write path variants with correct R2T handling and burst length enforcement.
**Verified:** 2026-04-01
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Solicited writes work: target sends R2T, initiator responds with correct Data-Out PDUs respecting MaxBurstLength and R2TSN tracking | VERIFIED | `dataout.go` handleR2T caps at MaxBurstLength; taskLoop has `case *pdu.R2T`; TestWriteSolicitedR2T + TestWriteSolicitedR2TLargePayload + TestWriteMultiR2TSequence all PASS |
| 2 | Immediate data works: write data piggybacks on SCSI Command PDU when ImmediateData=Yes, bounded by FirstBurstLength | VERIFIED | `session.go` Submit reads `io.ReadFull(cmd.Data, immBuf)` bounded by `min(s.params.FirstBurstLength, s.params.MaxRecvDataSegmentLength)`; TestSessionSubmitWriteImmediateData PASS; TestWriteMatrix ImmData=true sub-tests PASS |
| 3 | Unsolicited Data-Out works: when InitialR2T=No, initiator sends data before first R2T, bounded by FirstBurstLength | VERIFIED | `dataout.go` sendUnsolicitedDataOut uses TTT=0xFFFFFFFF, subtracts t.bytesSent from FirstBurstLength; Submit dispatches this synchronously when `!s.params.InitialR2T`; TestWriteUnsolicitedDataOut + TestWriteMatrix InitR2T=false sub-tests PASS |
| 4 | All four ImmediateData x InitialR2T combinations produce correct wire behavior, verified by parameterized tests | VERIFIED | TestWriteMatrix has 4 sub-tests covering all combinations; each verifies data integrity via bytes.Equal; all 4 sub-tests PASS |
| 5 | MaxOutstandingR2T is respected and MaxBurstLength is enforced for all solicited data sequences | VERIFIED | handleR2T: `desired := min(r2t.DesiredDataTransferLength, params.MaxBurstLength)` enforces burst cap; TestWriteMaxBurstLengthEnforcement PASS |

**Score:** 5/5 truths individually verified by test.

### Observable Truth: Full Suite Passes

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 6 | Full session test suite passes with write path included | PARTIAL | `go test ./internal/session/ -count=1 -race -timeout 120s` times out after 120s. 44 tests x ~5s graceful logout timeout = ~220s needed. All tests pass individually and with `-timeout 300s`. Root cause pre-dates Phase 4 (commit 1735202 from Phase 3). |

**Adjusted Score:** 4/5 (suite-level completeness gap)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/session/types.go` | Command with Data io.Reader field, no ImmediateData []byte | VERIFIED | Contains `Data io.Reader`; no `ImmediateData []byte` on Command struct; 115 lines |
| `internal/session/datain.go` | task struct extended with isWrite, reader, bytesSent; newTask(3-arg) | VERIFIED | Lines 21-23: `isWrite bool`, `reader io.Reader`, `bytesSent uint32`; `func newTask(itt uint32, isRead bool, isWrite bool)` at line 29 |
| `internal/session/session.go` | Submit: W-bit auto-set, immediate data reading from io.Reader, R2T case in taskLoop | VERIFIED | Line 101: `isWrite := cmd.Data != nil`; line 103: `cmd.Write = true`; line 119: `io.ReadFull(cmd.Data, immBuf)`; line 341: `case *pdu.R2T:`; 373 lines |
| `internal/session/dataout.go` | sendDataOutBurst, handleR2T, sendUnsolicitedDataOut methods | VERIFIED | All three methods at lines 16, 83, 97; 112 lines (above 80-line minimum) |
| `internal/session/dataout_test.go` | Tests for all write path variants, 2x2 matrix, multi-R2T, boundary | VERIFIED | 1003 lines; TestWriteSolicitedR2T, TestWriteSolicitedR2TLargePayload, TestWriteUnsolicitedDataOut, TestWriteMaxBurstLengthEnforcement, TestWriteMultiPDUBurst, TestWriteReaderError, TestWriteMatrix (4 sub-tests), TestWriteMultiR2TSequence, TestWriteSmallData, TestWriteExactBurstBoundary |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/session/dataout.go` | `internal/pdu/initiator.go` | pdu.DataOut construction | WIRED | Line 44 constructs `&pdu.DataOut{...}` with all RFC fields |
| `internal/session/session.go` | `internal/session/dataout.go` | taskLoop calls handleR2T | WIRED | Line 344: `tk.handleR2T(p, s.writeCh, s.getExpStatSN, s.params)` |
| `internal/session/dataout.go` | `internal/transport/pool.go` | buffer pool for Data-Out segments | WIRED | Lines 25, 29, 38, 59: transport.GetBuffer / transport.PutBuffer |
| `internal/session/session.go` | `internal/session/types.go` | Command.Data field | WIRED | Line 101: `cmd.Data != nil`; line 119: `io.ReadFull(cmd.Data, ...)` |
| `internal/session/session.go` | `internal/login/params.go` | NegotiatedParams.ImmediateData | WIRED | Line 116: `s.params.ImmediateData`; line 117: `s.params.FirstBurstLength`; line 184: `s.params.InitialR2T` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `session.go` Submit | `immediateData []byte` | `io.ReadFull(cmd.Data, immBuf)` consuming caller-supplied io.Reader | Yes — reads from caller's io.Reader up to negotiated limits | FLOWING |
| `dataout.go` sendDataOutBurst | `buf` (Data-Out segments) | `io.ReadFull(t.reader, buf[:chunkSize])` on-demand from task-owned io.Reader | Yes — reads sequentially from the caller's write payload | FLOWING |
| `dataout.go` sendUnsolicitedDataOut | `sent` bytes | Delegates to sendDataOutBurst with TTT=0xFFFFFFFF and FirstBurstLength-bytesSent budget | Yes — same on-demand io.Reader source | FLOWING |
| `dataout.go` handleR2T | `desired` bytes | Delegates to sendDataOutBurst with min(r2t.DesiredDataTransferLength, MaxBurstLength) | Yes — on-demand io.Reader, TTT echoed from R2T | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build compiles cleanly | `go build ./...` | BUILD OK | PASS |
| go vet passes | `go vet ./...` | VET OK | PASS |
| TestWriteSolicitedR2T | `go test ./internal/session/ -run TestWriteSolicitedR2T$ -timeout 30s` | PASS (5.00s) | PASS |
| TestWriteMatrix (all 4 combos) | `go test ./internal/session/ -run TestWriteMatrix -timeout 60s` | PASS (20.03s) | PASS |
| All TestWrite* tests | `go test ./internal/session/ -run TestWrite -timeout 300s` | PASS (55.15s total) | PASS |
| Full session suite | `go test ./internal/session/ -count=1 -race -timeout 120s` | TIMEOUT at 120s — ~220s needed | FAIL |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WRITE-01 | 04-02-PLAN.md, 04-03-PLAN.md | R2T handling with R2TSN tracking and MaxOutstandingR2T compliance | SATISFIED | handleR2T in dataout.go; case *pdu.R2T in taskLoop; TestWriteSolicitedR2T, TestWriteMultiR2TSequence PASS |
| WRITE-02 | 04-02-PLAN.md, 04-03-PLAN.md | Solicited Data-Out PDU generation in response to R2T | SATISFIED | sendDataOutBurst echoes r2t.TargetTransferTag; DataSN starts at 0 per burst; Final bit set correctly |
| WRITE-03 | 04-01-PLAN.md | Immediate data support (bounded by FirstBurstLength) | SATISFIED | Submit reads from io.Reader bounded by min(FirstBurstLength, MaxRecvDSL); TestSessionSubmitWriteImmediateData PASS |
| WRITE-04 | 04-02-PLAN.md, 04-03-PLAN.md | Unsolicited Data-Out when InitialR2T=No (bounded by FirstBurstLength) | SATISFIED | sendUnsolicitedDataOut with TTT=0xFFFFFFFF; FirstBurstLength-bytesSent budget; TestWriteUnsolicitedDataOut PASS |
| WRITE-05 | 04-02-PLAN.md, 04-03-PLAN.md | MaxBurstLength enforcement for solicited data sequences | SATISFIED | handleR2T: `desired := min(r2t.DesiredDataTransferLength, params.MaxBurstLength)`; TestWriteMaxBurstLengthEnforcement PASS |

**Requirements coverage: 5/5 — all WRITE-01 through WRITE-05 satisfied.**

No orphaned requirements detected. REQUIREMENTS.md Traceability table maps all five WRITE-* IDs exclusively to Phase 4. All five are marked complete.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/session/dataout.go` | 31-35 | Double `return sent, fmt.Errorf(...)` for partial read with unexpected error — dead code path (second return is unreachable because first `return` already executes) | Info | Minor code quality issue; does not affect correctness since partial read with unexpected error immediately returns |
| Suite timeout | N/A | `go test ./internal/session/ -timeout 120s` fails — 44 tests at ~5s each requires ~220s. Phase 3 introduced graceful logout with 5s timeout (commit 1735202); Phase 4 added 10 more tests making the full suite exceed 120s. | Warning | Full suite cannot be verified with `go test ./... -count=1 -race` without increasing timeout. The SUMMARY.md for plan 03 claimed "Full project test suite passes under race detector" which was inaccurate for the default timeout. |

### Human Verification Required

None. All write path behaviors are verifiable programmatically. The implementation is a pure network protocol library with no UI components.

### Gaps Summary

Phase 4's implementation is functionally correct and all five WRITE-* requirements are satisfied by the codebase. All 10 write-path tests (13 including sub-tests) pass under the race detector when run with sufficient timeout.

The single gap is a suite-level test execution problem: the full `internal/session` package requires ~220 seconds to run because every test creates a session whose `Close()` attempts a graceful logout with a 5-second context timeout. This pre-existing behavior (from Phase 3, commit 1735202) was not a problem until Phase 4 added 10 more test functions. The PLAN and SUMMARY both claim the full test suite passes, but this is only true with `-timeout 300s` or higher.

The gap does not indicate a behavioral defect in the write path itself — it is a test infrastructure issue. However, it means the stated success criterion "Full project test suite passes under race detector" (per 04-03-PLAN.md) is not satisfiable with the default 120s timeout.

---

## Detailed Artifact Status

### `internal/session/types.go`
- Level 1 (exists): PASS — file present
- Level 2 (substantive): PASS — 115 lines, complete type definitions
- Level 3 (wired): PASS — Command.Data used in session.go Submit
- Level 4 (data flows): PASS — Data field carries caller io.Reader through to PDU encoding

### `internal/session/datain.go`
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — task struct has isWrite, reader, bytesSent fields; newTask(3-arg) signature correct
- Level 3 (wired): PASS — newTask called from session.go Submit; task.reader set by Submit; task.bytesSent updated by sendUnsolicitedDataOut
- Level 4 (data flows): PASS — reader field set to cmd.Data before task goroutine starts

### `internal/session/dataout.go`
- Level 1 (exists): PASS — file created by Phase 4 Plan 02
- Level 2 (substantive): PASS — 112 lines; all three required methods implemented
- Level 3 (wired): PASS — imported by session.go (same package); handleR2T called from taskLoop; sendUnsolicitedDataOut called from Submit
- Level 4 (data flows): PASS — sendDataOutBurst reads from t.reader on-demand; GetBuffer/PutBuffer buffer lifecycle correct

### `internal/session/dataout_test.go`
- Level 1 (exists): PASS — file created by Phase 4 Plan 02, extended by Plan 03
- Level 2 (substantive): PASS — 1003 lines; 10 test functions covering all required scenarios
- Level 3 (wired): PASS — tests exercise live session via net.Pipe(); all TestWrite* pass
- Level 4 (data flows): PASS — tests verify byte-level data integrity with bytes.Equal

### `internal/session/session.go`
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — 373 lines; Submit write path complete; taskLoop R2T case present
- Level 3 (wired): PASS — all wiring verified by tests
- Level 4 (data flows): PASS — immediate data flows from io.Reader to PDU DataSegment; unsolicited dispatch synchronous before task goroutine

---

_Verified: 2026-04-01_
_Verifier: Claude (gsd-verifier)_
