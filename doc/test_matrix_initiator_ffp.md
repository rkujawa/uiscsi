# UNH-IOL Initiator FFP Test Matrix

Maps each test from the **UNH-IOL iSCSI Initiator Full Feature Phase Test Suite v0.1** (`doc/initiator_ffp.pdf`) to existing uiscsi E2E and conformance test coverage.

**Legend:**
- **Covered** — test exists that exercises this area
- **Partial** — some aspects covered but not the specific PDU-level validation the IOL test requires
- **Not Covered** — no existing test for this area
- **N/A** — feature intentionally not implemented (e.g., SNACK at ERL>0)

## Summary

| Status | Count | Percentage |
|--------|-------|------------|
| Covered | 11 | 18% |
| Partial | 22 | 35% |
| Not Covered | 29 | 47% |
| **Total** | **62** | |

## Test Matrix

### Group 1: Command Numbering

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #1.1 | Verify CmdSN increments by 1 for each non-immediate command | 3.2.2.1 | Partial | `internal/session/cmdwindow_test.go` | Unit tests verify CmdSN windowing logic; no E2E test explicitly validates wire-level CmdSN sequencing |

### Group 2: Immediate Delivery

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #2.1 | Verify immediate delivery flag + CmdSN for non-TMF commands | 3.2.2.1 | Not Covered | — | No test sends immediate non-TMF commands and validates I-bit + CmdSN on wire |
| #2.2 | Verify immediate delivery for task management commands | 3.2.2.1 | Partial | `test/e2e/tmf_test.go`, `test/conformance/task_test.go` | TMF tests exist but don't validate I-bit or CmdSN behavior at PDU level |

### Group 3: MaxCmdSN / ExpCmdSN (Command Window)

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #3.1 | Initiator respects zero command window (MaxCmdSN = ExpCmdSN-1) | 3.2.2.1 | Partial | `internal/session/cmdwindow_test.go` | Unit tests for window logic; no E2E test closes window to zero then reopens via NOP-In |
| #3.2 | Initiator uses large command window (MaxCmdSN >> ExpCmdSN) | 3.2.2.1 | Partial | `internal/session/cmdwindow_test.go` | Same — unit-level coverage only |
| #3.3 | Initiator respects window size of 1 | 3.2.2.1 | Partial | `internal/session/cmdwindow_test.go` | Same — unit-level coverage only |

### Group 4: Command Retry

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #4.1 | Retried command carries original ITT, CDB, CmdSN on same connection | 3.2.2.1, 6.2.1 | Not Covered | — | Requires ERL≥1 and deliberate non-response to trigger retry; no test for this |

### Group 5: ExpStatSN

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #5.1 | Detect large StatSN / ExpStatSN gap → recovery action | 3.2.2.2 | Not Covered | — | No test injects a massive StatSN jump to verify recovery |

### Group 6: DataSN

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #6.1 | Data-Out DataSN starts at 0 and increments per R2T sequence | 3.2.2.3, 10.7.5 | Partial | `internal/session/dataout_test.go`, `test/e2e/largewrite_test.go` | Unit tests verify DataSN logic; large write E2E exercises multi-R2T but doesn't validate DataSN on wire |

### Group 7: Connection Reassignment

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #7.1 | Initiator performs task reassign on new connection after drop (ERL 2) | 6.2.2 | Not Covered | — | ERL 2 connection reassignment not implemented; `test/e2e/recovery_test.go` covers ERL 0 reconnection only |

### Group 8: Data Transmission

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #8.1 | Unsolicited data respects FirstBurstLength; solicited data follows R2T | 3.2.4.2 | Partial | `test/e2e/negotiation_test.go`, `test/e2e/largewrite_test.go` | Negotiation matrix covers ImmediateData/InitialR2T combos; no explicit FirstBurstLength boundary test |
| #8.2 | No unsolicited data when InitialR2T=Yes, ImmediateData=No | 3.2.4.2 | Covered | `test/e2e/negotiation_test.go` (ImmNo_R2TYes) | Negotiation subtest verifies write succeeds under R2T-only mode |
| #8.3 | Unsolicited Data-Out (not immediate) respects FirstBurstLength | 3.2.4.2 | Partial | `test/e2e/negotiation_test.go` (ImmNo_R2TNo) | Covers the combination but doesn't validate FirstBurstLength boundary at PDU level |

### Group 9: Target Transfer Tag

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #9.1 | Data-Out echoes Target Transfer Tag from R2T | 3.2.4.3, 10.7.4 | Partial | `internal/session/dataout_test.go` | Unit tests verify TTT echo; E2E large write implicitly exercises this but doesn't validate specific tag value |

### Group 10: Data-In

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #10.1 | Accept status in final Data-In PDU (S bit + F bit) | 10.7, 10.7.3 | Partial | `internal/session/datain_test.go` | Unit tests verify S-bit handling; E2E reads work but don't validate PDU-level S/F bit behavior |
| #10.2 | Respond to Data-In A bit with SNACK DataACK (ERL≥1) | 10.7.2 | Partial | `internal/session/snack_test.go`, `test/e2e/erl_test.go` | SNACK unit tests exist; ERL 1 E2E test verifies negotiation but can't trigger A-bit scenario externally |

### Group 11: Data-Out PDU Fields

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #11.1.1 | Data-Out respects target MaxRecvDataSegmentLength | 10.7.7 | Partial | `internal/session/dataout_test.go` | Unit tests verify segment sizing; no E2E test with explicit small MaxRecvDataSegmentLength |
| #11.1.2 | Accept Data-In with DataSegmentLength=0 | 10.7.7 | Not Covered | — | No test sends a zero-length Data-In PDU |
| #11.2.1 | F bit set on last unsolicited Data-Out PDU | 10.7.1 | Partial | `internal/session/dataout_test.go` | Unit tests cover F-bit; not validated at wire level in E2E |
| #11.2.2 | F bit set on last solicited Data-Out PDU | 10.7.1 | Partial | `internal/session/dataout_test.go` | Same as above |
| #11.3 | DataSN starts at 0 per R2T sequence, increments per PDU | 10.7.5 | Partial | `internal/session/dataout_test.go` | See #6.1 — unit-level coverage |
| #11.4 | Buffer Offset increases correctly in Data-Out PDUs | 10.7.6 | Partial | `internal/session/dataout_test.go` | Unit tests; not validated on wire |

### Group 12: R2T Handling

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #12.1 | Single Data-Out response to R2T with correct TTT, offset, length | 10.8 | Partial | `internal/session/dataout_test.go`, `test/e2e/largewrite_test.go` | Unit tests verify; E2E implicitly exercises but doesn't validate PDU fields |
| #12.2 | Multi-PDU response to R2T with F bit, continuous offsets | 10.8 | Partial | `test/e2e/largewrite_test.go` | 1MB write forces multi-PDU R2T responses; no wire-level validation |
| #12.3 | R2T fulfillment order when DataSequenceInOrder=No | 10.8 | Not Covered | — | No test exercises out-of-order R2T buffer offsets |
| #12.4 | Parallel commands: R2T fulfillment order across interleaved commands | 10.8 | Not Covered | — | No test sends parallel writes and validates R2T response ordering |

### Group 13: SNACK

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #13.1 | Data/R2T SNACK construction (skip DataSN, trigger retransmit request) | 10.16 | Partial | `internal/session/snack_test.go` | Unit tests verify SNACK PDU construction; can't trigger gap scenario in E2E |
| #13.2 | DataACK SNACK in response to A-bit | 10.16 | Partial | `internal/session/snack_test.go` | Same — unit-level only |

### Group 14: Logout Request

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #14.1 | Logout after AsyncMessage code 1 (single connection) | 10.9, 10.14 | Partial | `internal/session/logout_test.go` | Unit tests verify logout PDU construction; no E2E test for async message-triggered logout |
| #14.2 | Logout after AsyncMessage code 1 (multi-connection session) | 10.9, 10.14 | Not Covered | — | Multi-connection sessions not implemented in E2E |

### Group 15: NOP-Out

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #15.1 | NOP-Out ping response (TTT echo, ITT=0xffffffff, I-bit, LUN echo) | 10.18, 10.19 | Covered | `internal/session/keepalive_test.go`, `test/conformance/fullfeature_test.go` | Keepalive unit tests + conformance mock target exercises NOP-Out/In exchange |
| #15.2 | NOP-Out ping request (initiator-initiated, valid ITT) | 10.18, 10.19 | Covered | `internal/session/keepalive_test.go` | Library sends NOP-Out pings for keepalive; unit tests validate PDU fields |
| #15.3 | NOP-Out to confirm ExpStatSN (ITT=0xffffffff, I-bit=1) | 10.18, 10.19 | Partial | `internal/session/keepalive_test.go` | Keepalive tests cover NOP-Out but not specifically the ExpStatSN confirmation variant |

### Group 16: SCSI Command

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #16.1.1 | Command PDU fields with ImmediateData=Yes (CmdSN, ExpStatSN, ITT, DSL, CDB) | 10.3 | Partial | `test/e2e/negotiation_test.go` (ImmYes variants) | Write+read integrity verified; not validating individual PDU fields |
| #16.1.2 | Command PDU fields with ImmediateData=No (DSL=0) | 10.3 | Partial | `test/e2e/negotiation_test.go` (ImmNo variants) | Same — functional test, not PDU field validation |
| #16.2.1 | Unsolicited data: ImmediateData=Yes, InitialR2T=Yes — F bit when EDTL = DSL | 10.3.4 | Partial | `test/e2e/negotiation_test.go` (ImmYes_R2TYes) | Functional coverage only |
| #16.2.2 | No unsolicited data: ImmediateData=No, InitialR2T=Yes — DSL=0 | 10.3.4 | Covered | `test/e2e/negotiation_test.go` (ImmNo_R2TYes) | Tests verify write works under this mode |
| #16.2.3 | No immediate data: ImmediateData=No, InitialR2T=No — DSL=0 in command | 10.3.4 | Covered | `test/e2e/negotiation_test.go` (ImmNo_R2TNo) | Tests verify write works under this mode |
| #16.2.4 | Both enabled: ImmediateData=Yes, InitialR2T=No — FirstBurstLength limit | 10.3.4, 12.12 | Partial | `test/e2e/negotiation_test.go` (ImmYes_R2TNo) | Functional but no FirstBurstLength boundary validation |
| #16.3.1 | F bit in SCSI Command when InitialR2T=Yes (no unsolicited Data-Out follows) | 10.3 | Not Covered | — | No explicit F-bit validation test |
| #16.4.1 | Handle CRC error sense data (CHECK CONDITION, sense key 0x0B) | 6.2.1, 10.4.7.2 | Not Covered | — | No test injects CRC-error sense data |
| #16.4.2 | Handle SNACK reject → new command (not retry) | 6.2.1, 10.16 | Not Covered | — | No test for SNACK rejection followed by re-issue |
| #16.4.3 | Handle unexpected unsolicited data error sense | 6.2.1, 10.4.7.2 | Not Covered | — | No test injects this specific sense code |
| #16.4.4 | Handle "not enough unsolicited data" error sense | 6.2.1, 10.4.7.2 | Not Covered | — | No test injects this specific sense code |
| #16.4.5 | Handle BUSY status (0x08) → re-issue later | 6.2.1, SAM-2 | Not Covered | — | No test for BUSY status handling |
| #16.4.6 | Handle RESERVATION CONFLICT (0x18) → re-issue later | 6.2.1, SAM-2 | Not Covered | — | No test for RESERVATION CONFLICT handling |
| #16.5 | Respect MaxCmdSN in SCSI Response (stop issuing if window closed) | 10.4.7.3 | Partial | `internal/session/cmdwindow_test.go` | Unit-level command window tests |
| #16.6 | Expected Data Transfer Length matches actual transfer | 10.3 | Covered | `test/e2e/data_test.go`, `test/e2e/largewrite_test.go` | Write+read integrity confirms EDTL is correct |

### Group 17: Logout

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #17.1 | Clean logout with proper Logout Request/Response exchange | 10.14 | Covered | `test/e2e/e2e_test.go`, `internal/session/logout_test.go` | Every E2E test calls `sess.Close()` which performs logout; unit tests verify PDU |

### Group 18: Text Request

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #18.1 | Text Request text fields (key=value format) | 10.11 | Covered | `test/e2e/e2e_test.go` (Discover) | Discovery uses Text Request; login text codec unit tests in `internal/login/textcodec_test.go` |
| #18.2 | Text Request Initiator Task Tag uniqueness | 10.11 | Partial | `internal/login/textcodec_test.go` | No explicit ITT uniqueness validation across Text Requests |
| #18.3.1 | Text Request Target Transfer Tag (initial = 0xffffffff) | 10.11 | Not Covered | — | No explicit TTT validation in text request tests |
| #18.3.2 | Text Request Target Transfer Tag (continuation) | 10.11 | Not Covered | — | No multi-PDU text negotiation test |
| #18.4 | Text Request other parameters | 10.11 | Not Covered | — | No comprehensive text request parameter validation |
| #18.5 | Text Request negotiation reset | 10.11 | Not Covered | — | No test for mid-negotiation reset |

### Group 19: Task Management

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #19.1 | TMF CmdSN handling | 10.5 | Partial | `test/conformance/task_test.go`, `internal/session/tmf_test.go` | TMF tests exist but don't validate CmdSN at PDU level |
| #19.2 | TMF LUN field | 10.5 | Partial | `test/e2e/tmf_test.go` (LUNReset on LUN 0) | LUN Reset specifies LUN but no validation of PDU LUN encoding |
| #19.3 | TMF RefCmdSN for referenced task | 10.5 | Partial | `test/e2e/tmf_test.go` (AbortTask) | AbortTask sends RefCmdSN but no PDU-level validation |
| #19.4.1 | Abort Task Set: all tasks on LUN aborted | 10.5.1 | Not Covered | — | No Abort Task Set test |
| #19.4.2 | Abort Task Set: verify no new tasks during abort | 10.5.1 | Not Covered | — | No Abort Task Set test |
| #19.4.3 | Abort Task Set: verify response after all tasks cleared | 10.5.1 | Not Covered | — | No Abort Task Set test |
| #19.5 | Task Reassign (ERL 2 connection recovery) | 10.5.3 | Not Covered | — | ERL 2 not implemented |

### Group 20: Asynchronous Message

| IOL Test | Purpose | RFC Ref | uiscsi Coverage | Test File(s) | Notes |
|----------|---------|---------|-----------------|--------------|-------|
| #20.1 | Async Message code 1: target requests logout | 10.9.1 | Not Covered | — | No test for async logout request handling |
| #20.2 | Async Message: drop connection | 10.9.1 | Not Covered | — | No test for async connection drop notification |
| #20.3 | Async Message: drop all connections in session | 10.9.1 | Not Covered | — | No test for async session drop notification |
| #20.4 | Async Message: request negotiation | 10.9.1 | Not Covered | — | No test for async negotiation request |

## Coverage Gap Analysis

### Well-Covered Areas
- **Logout** (#17.1): Every E2E test exercises clean logout
- **NOP-Out/In** (#15.1-15.2): Keepalive unit + conformance tests
- **Data Transfer Integrity** (#16.6): Multiple E2E write+read tests
- **Negotiation Combinations** (#8.2, #16.2.2, #16.2.3): 4-way ImmediateData/InitialR2T matrix
- **Text Request basics** (#18.1): Discovery exercises text request path

### Partial — Unit Tests Exist, E2E Validation Missing
- **Command Numbering** (#1.1): `cmdwindow_test.go` covers logic, no wire validation
- **Data-Out PDU fields** (#6.1, #11.x): `dataout_test.go` covers construction, no E2E capture
- **SNACK** (#13.1-13.2): `snack_test.go` covers PDU building, can't trigger in E2E
- **R2T handling** (#12.1-12.2): Large write exercises multi-R2T implicitly

### Not Covered — Implementation Gaps
- **ERL 2 features** (#7.1, #19.5): Connection reassignment / task reassign not implemented
- **Async Messages** (#20.1-20.4): No async message handling tests
- **Text Request advanced** (#18.3-18.5): TTT continuation, parameter validation, reset
- **Abort Task Set** (#19.4.x): TMF exists but not Abort Task Set specifically
- **Specific SCSI error recovery** (#16.4.1-16.4.6): CRC error, BUSY, RESERVATION CONFLICT handling

### Not Covered — Test Infrastructure Gaps
- **Command Retry** (#4.1): Needs controlled non-response + ERL≥1
- **ExpStatSN gap detection** (#5.1): Needs injected StatSN jump
- **Zero-length Data-In** (#11.1.2): Needs mock target sending DSL=0
- **R2T ordering** (#12.3-12.4): Needs out-of-order R2T injection + parallel writes
- **Immediate delivery validation** (#2.1): Needs wire-level I-bit inspection

## Recommendations

1. **High value, low effort**: Add PDU-level assertions to existing conformance tests using MockTarget (validates #1.1, #11.x, #16.1.x at PDU level)
2. **Medium effort**: Extend MockTarget to inject error conditions (#16.4.x SCSI status codes, #5.1 StatSN gaps, #11.1.2 zero-length Data-In)
3. **High effort**: Implement async message handling (#20.x) and Abort Task Set (#19.4.x)
4. **Deferred**: ERL 2 features (#7.1, #19.5) — out of scope until ERL 2 is implemented
