---
phase: 04-write-path
verified: 2026-04-01T00:00:00Z
status: passed
score: 5/5 must-haves verified
re_verification:
  previous_status: gaps_found
  previous_score: 4/5
  gaps_closed:
    - "go test ./internal/session/ -count=1 -race -timeout 120s passes without timeout (6.9s actual)"
  gaps_remaining: []
  regressions: []
---

# Phase 4: Write Path Verification Report

**Phase Goal:** A Go application can write data to an iSCSI target through all write path variants with correct R2T handling and burst length enforcement.
**Verified:** 2026-04-01
**Status:** passed
**Re-verification:** Yes — after gap closure (plan 04-04)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Solicited writes work: target sends R2T, initiator responds with correct Data-Out PDUs respecting MaxBurstLength and R2TSN tracking | VERIFIED | `dataout.go` handleR2T caps at MaxBurstLength; taskLoop has `case *pdu.R2T`; TestWriteSolicitedR2T + TestWriteSolicitedR2TLargePayload + TestWriteMultiR2TSequence all PASS |
| 2 | Immediate data works: write data piggybacks on SCSI Command PDU when ImmediateData=Yes, bounded by FirstBurstLength | VERIFIED | `session.go` Submit reads `io.ReadFull(cmd.Data, immBuf)` bounded by `min(s.params.FirstBurstLength, s.params.MaxRecvDataSegmentLength)`; TestSessionSubmitWriteImmediateData PASS; TestWriteMatrix ImmData=true sub-tests PASS |
| 3 | Unsolicited Data-Out works: when InitialR2T=No, initiator sends data before first R2T, bounded by FirstBurstLength | VERIFIED | `dataout.go` sendUnsolicitedDataOut uses TTT=0xFFFFFFFF, subtracts t.bytesSent from FirstBurstLength; Submit dispatches this synchronously when `!s.params.InitialR2T`; TestWriteUnsolicitedDataOut + TestWriteMatrix InitR2T=false sub-tests PASS |
| 4 | All four ImmediateData x InitialR2T combinations produce correct wire behavior, verified by parameterized tests | VERIFIED | TestWriteMatrix has 4 sub-tests covering all combinations; each verifies data integrity via bytes.Equal; all 4 sub-tests PASS |
| 5 | MaxOutstandingR2T is respected and MaxBurstLength is enforced for all solicited data sequences | VERIFIED | handleR2T: `desired := min(r2t.DesiredDataTransferLength, params.MaxBurstLength)` enforces burst cap; TestWriteMaxBurstLengthEnforcement PASS |

**Score:** 5/5 truths verified.

### Re-Verification: Gap Closure Truth

| # | Truth | Previous Status | Current Status | Evidence |
|---|-------|----------------|----------------|----------|
| 6 | Full session test suite passes with `go test ./internal/session/ -count=1 -race -timeout 120s` | PARTIAL (timeout at 120s) | VERIFIED | Suite completes in 6.9s. `respondToLogout` helper in session_test.go (lines 75-106) spawns goroutine in cleanup that reads from target pipe and responds to Logout PDU with LogoutResp. All 3 session constructors updated (newTestSession, newTestSessionWithParams, newTestSessionWithOptions). |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/session/types.go` | Command with Data io.Reader field, no ImmediateData []byte | VERIFIED | Contains `Data io.Reader`; no `ImmediateData []byte` on Command struct |
| `internal/session/datain.go` | task struct extended with isWrite, reader, bytesSent; newTask(3-arg) | VERIFIED | Lines 21-23: `isWrite bool`, `reader io.Reader`, `bytesSent uint32`; `func newTask(itt uint32, isRead bool, isWrite bool)` at line 29 |
| `internal/session/session.go` | Submit: W-bit auto-set, immediate data reading from io.Reader, R2T case in taskLoop | VERIFIED | Line 101: `isWrite := cmd.Data != nil`; line 103: `cmd.Write = true`; line 119: `io.ReadFull(cmd.Data, immBuf)`; line 341: `case *pdu.R2T:` |
| `internal/session/dataout.go` | sendDataOutBurst, handleR2T, sendUnsolicitedDataOut methods | VERIFIED | All three methods present; 112 lines |
| `internal/session/dataout_test.go` | Tests for all write path variants, 2x2 matrix, multi-R2T, boundary | VERIFIED | 1003 lines; 10 test functions covering all required scenarios |
| `internal/session/session_test.go` | respondToLogout helper and updated newTestSession cleanup | VERIFIED | `func respondToLogout(conn net.Conn)` at lines 75-106; cleanup at line 29 calls `go respondToLogout(targetConn)` before `sess.Close()` |
| `internal/session/dataout_test.go` | newTestSessionWithParams uses respondToLogout in cleanup | VERIFIED | Line 22: `go respondToLogout(targetConn)` |
| `internal/session/keepalive_test.go` | newTestSessionWithOptions uses respondToLogout in cleanup | VERIFIED | Line 26: `go respondToLogout(targetConn)` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/session/dataout.go` | `internal/pdu/initiator.go` | pdu.DataOut construction | WIRED | Line 44 constructs `&pdu.DataOut{...}` with all RFC fields |
| `internal/session/session.go` | `internal/session/dataout.go` | taskLoop calls handleR2T | WIRED | Line 344: `tk.handleR2T(p, s.writeCh, s.getExpStatSN, s.params)` |
| `internal/session/dataout.go` | `internal/transport/pool.go` | buffer pool for Data-Out segments | WIRED | Lines 25, 29, 38, 59: transport.GetBuffer / transport.PutBuffer |
| `internal/session/session.go` | `internal/session/types.go` | Command.Data field | WIRED | Line 101: `cmd.Data != nil`; line 119: `io.ReadFull(cmd.Data, ...)` |
| `internal/session/session.go` | `internal/login/params.go` | NegotiatedParams.ImmediateData | WIRED | Line 116: `s.params.ImmediateData`; line 117: `s.params.FirstBurstLength`; line 184: `s.params.InitialR2T` |
| `internal/session/session_test.go` | `internal/session/logout_test.go` | respondToLogout mirrors LogoutResp pattern | WIRED | respondToLogout uses `pdu.OpLogoutResp`, `resp.MarshalBHS()`, `transport.WriteRawPDU` — same pattern as writeLogoutRespPDU in logout_test.go |

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
| Full session suite under race detector, 120s timeout | `go test ./internal/session/ -count=1 -race -timeout 120s` | PASS 6.901s | PASS |
| Full project suite under race detector | `go test ./... -count=1 -race` | PASS — all 6 packages pass | PASS |
| Zero production code changes in gap closure | `git diff 32f7bbb~1 32f7bbb -- session.go types.go datain.go dataout.go` | No output (no changes) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WRITE-01 | 04-02-PLAN.md, 04-03-PLAN.md | R2T handling with R2TSN tracking and MaxOutstandingR2T compliance | SATISFIED | handleR2T in dataout.go; case *pdu.R2T in taskLoop; TestWriteSolicitedR2T, TestWriteMultiR2TSequence PASS |
| WRITE-02 | 04-02-PLAN.md, 04-03-PLAN.md | Solicited Data-Out PDU generation in response to R2T | SATISFIED | sendDataOutBurst echoes r2t.TargetTransferTag; DataSN starts at 0 per burst; Final bit set correctly |
| WRITE-03 | 04-01-PLAN.md | Immediate data support (bounded by FirstBurstLength) | SATISFIED | Submit reads from io.Reader bounded by min(FirstBurstLength, MaxRecvDSL); TestSessionSubmitWriteImmediateData PASS |
| WRITE-04 | 04-02-PLAN.md, 04-03-PLAN.md | Unsolicited Data-Out when InitialR2T=No (bounded by FirstBurstLength) | SATISFIED | sendUnsolicitedDataOut with TTT=0xFFFFFFFF; FirstBurstLength-bytesSent budget; TestWriteUnsolicitedDataOut PASS |
| WRITE-05 | 04-02-PLAN.md, 04-03-PLAN.md | MaxBurstLength enforcement for solicited data sequences | SATISFIED | handleR2T: `desired := min(r2t.DesiredDataTransferLength, params.MaxBurstLength)`; TestWriteMaxBurstLengthEnforcement PASS |

**Requirements coverage: 5/5 — all WRITE-01 through WRITE-05 satisfied.**

No orphaned requirements. REQUIREMENTS.md maps all five WRITE-* IDs exclusively to Phase 4.

### Anti-Patterns Found

None blocking. The minor dead-code path noted in the initial verification (`internal/session/dataout.go` lines 31-35, unreachable second return after partial-read error) is an info-level style issue only and does not affect correctness or test results.

### Human Verification Required

None. All write path behaviors are verifiable programmatically. The implementation is a pure network protocol library with no UI components.

### Gaps Summary

All gaps from the initial verification are closed. The single gap (test suite timeout) was resolved by plan 04-04: a `respondToLogout` helper was added to `session_test.go` (lines 75-106) that spawns a goroutine during test cleanup to read from the target pipe and respond to the Logout PDU with a proper LogoutResp — exactly as a real iSCSI target would. All three test session constructors (`newTestSession`, `newTestSessionWithParams`, `newTestSessionWithOptions`) were updated to use it. Zero production code was changed.

Result: `go test ./internal/session/ -count=1 -race -timeout 120s` completes in 6.9s (was ~220s). `go test ./... -count=1 -race` passes across all 6 packages.

---

## Detailed Artifact Status

### `internal/session/types.go`
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — complete type definitions
- Level 3 (wired): PASS — Command.Data used in session.go Submit
- Level 4 (data flows): PASS — Data field carries caller io.Reader through to PDU encoding

### `internal/session/datain.go`
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — task struct has isWrite, reader, bytesSent fields; newTask(3-arg) signature correct
- Level 3 (wired): PASS — newTask called from session.go Submit; task.reader set by Submit
- Level 4 (data flows): PASS — reader field set to cmd.Data before task goroutine starts

### `internal/session/dataout.go`
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — 112 lines; all three required methods implemented
- Level 3 (wired): PASS — handleR2T called from taskLoop; sendUnsolicitedDataOut called from Submit
- Level 4 (data flows): PASS — sendDataOutBurst reads from t.reader on-demand; GetBuffer/PutBuffer lifecycle correct

### `internal/session/dataout_test.go`
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — 1003 lines; 10 test functions covering all required scenarios
- Level 3 (wired): PASS — tests exercise live session via net.Pipe(); all TestWrite* pass
- Level 4 (data flows): PASS — tests verify byte-level data integrity with bytes.Equal

### `internal/session/session.go`
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — 373 lines; Submit write path complete; taskLoop R2T case present
- Level 3 (wired): PASS — all wiring verified by tests
- Level 4 (data flows): PASS — immediate data flows from io.Reader to PDU DataSegment

### `internal/session/session_test.go` (gap closure artifact)
- Level 1 (exists): PASS
- Level 2 (substantive): PASS — respondToLogout at lines 75-106; reads PDUs in loop, responds to OpLogoutReq with full LogoutResp using MarshalBHS + WriteRawPDU
- Level 3 (wired): PASS — called by newTestSession cleanup (line 29); mirrored in dataout_test.go (line 22) and keepalive_test.go (line 26)

---

_Verified: 2026-04-01_
_Verifier: Claude (gsd-verifier)_
