---
status: complete
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
source: [10-01-SUMMARY.md, 10-02-SUMMARY.md, 10-03-SUMMARY.md]
started: 2026-04-02T23:35:00Z
updated: 2026-04-02T23:35:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Large Write Multi-R2T Data Integrity
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestLargeWrite_MultiR2T`. Test writes 1MB (2048 blocks) to LIO target, reads back, verifies data integrity via XOR pattern. Expected: PASS.
result: pass

### 2. ImmediateData x InitialR2T 2x2 Negotiation Matrix
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestNegotiation_ImmediateDataInitialR2T`. Test covers all 4 combinations (Yes/Yes, Yes/No, No/Yes, No/No) with configfs target-side configuration. Each subtest writes+reads data and verifies integrity. Expected: PASS (all 4 subtests).
result: issue
reported: "The test loops, which in itself feels wrong. Besides it fucks up around here: login: complete header_digest=false data_digest=false, session: reconnect complete new_tsih=673, session: unhandled unsolicited PDU opcode=0x3F"
severity: major

### 3. Header-Only Digest Mode
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestDigest_HeaderOnly`. Negotiates CRC32C header digest without data digest, performs write+read cycle. Expected: PASS.
result: pass

### 4. Data-Only Digest Mode
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestDigest_DataOnly`. Negotiates CRC32C data digest without header digest, performs write+read cycle. Expected: PASS.
result: pass

### 5. SCSI Error — Out-of-Range LBA
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestSCSIError_OutOfRangeLBA`. Writes to LBA 200000 (beyond 64MB LUN). Expected: SCSIError with SenseKey 0x05 (ILLEGAL_REQUEST), ASC 0x21, ASCQ 0x00. PASS.
result: issue
reported: "SenseKey: got 0x00, want 0x05. ASC/ASCQ: got 0x00/0x00, want 0x21/0x00. Error message is 'scsi: status 0x02' — CHECK CONDITION status is correct but sense data fields are all zeros, not parsed from response PDU."
severity: major

### 6. SCSI Error — Sense Data Parsing
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestSCSIError_SenseDataParsing`. Reads from out-of-range LBA. Expected: SCSIError with non-zero SenseKey and non-empty Message containing human-readable error. PASS.
result: issue
reported: "Same root cause as Test 5. SenseKey is 0x00 (NO SENSE), Message is empty. Sense data not parsed from response PDU data segment."
severity: major

### 7. ABORT TASK TMF During Concurrent Command
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestTMF_AbortTask`. Captures ITT of in-flight SCSI read via PDU hook, sends AbortTask TMF. Accepts response code 0 (Function Complete) or 5 (Task Does Not Exist). Expected: PASS.
result: issue
reported: "Captured ITT is 0x00000000 (invalid/reserved). AbortTask response code is 255 (0xFF, not a valid TMF response). PDU hook ITT capture not extracting correct ITT from SCSI Command PDU, and TMF response parsing returns garbage."
severity: major

### 8. TARGET WARM RESET TMF
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestTMF_TargetWarmReset`. Sends TargetWarmReset, handles session drop, re-establishes new session and verifies target is alive with a read. Expected: PASS.
result: pass

### 9. ERL 1 SNACK Recovery Negotiation
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestERL1_SNACKRecovery`. Configures target for ERL 1 via configfs, negotiates with WithOperationalOverrides. If LIO supports ERL 1: verifies session works. If negotiation fails or target rejects: t.Skip with message. Expected: PASS or SKIP.
result: pass

### 10. ERL 2 Connection Replacement
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestERL2_ConnectionReplacement`. Configures target for ERL 2, kills TCP with ss -K, verifies session recovery. Accepts both ERL 2 replacement and ERL 0 fallback. If negotiation fails: t.Skip. Expected: PASS or SKIP.
result: pass
note: Kernel logs show "Unable to locate key" for SessionType, MaxConnections, InitialR2T, ImmediateData, MaxBurstLength, FirstBurstLength, DefaultTime2Wait, DefaultTime2Retain, MaxOutstandingR2T, DataPDUInOrder, DataSequenceInOrder, ErrorRecoveryLevel during reconnection login. Also "Adding additional connection... would exceed MaxConnections 1, login failed" before successful reinstatement. Pre-existing login negotiation issue — reconnection sends operational keys that LIO rejects during session reinstatement. Not introduced by Phase 10.

### 11. Graceful Skip When Not Root
expected: Run `go test -tags e2e -v -count=1 ./test/e2e/ -run TestLargeWrite_MultiR2T` (WITHOUT sudo, as non-root). Expected: test is SKIPPED with a clear message about requiring root, NOT failed.
result: pass

## Summary

total: 11
passed: 7
issues: 4
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "ImmediateData x InitialR2T 2x2 negotiation matrix test passes all 4 subtests with data integrity"
  status: failed
  reason: "User reported: The test loops, which in itself feels wrong. Besides it fucks up around here: login: complete, session: reconnect complete new_tsih=673, session: unhandled unsolicited PDU opcode=0x3F"
  severity: major
  test: 2
  root_cause: "OpReject (0x3F) PDU from target is not handled in session's unsolicited PDU dispatcher (internal/session/session.go:452). When a negotiation combination causes the target to reject a data PDU, the Reject causes connection drop, auto-reconnect loops, and the test hangs. Two issues: (1) OpReject must be handled as an unsolicited PDU with proper error propagation, (2) the test subtests that cause rejection (likely ImmNo combinations sending unsolicited data) need to handle the Reject error gracefully."
  artifacts:
    - path: "internal/session/session.go"
      issue: "OpReject not handled in handleUnsolicitedPDU switch — falls through to Warn log"
    - path: "test/e2e/negotiation_test.go"
      issue: "Test loops when target rejects data PDU — no Reject handling, auto-reconnect causes infinite loop"
  missing:
    - "Add OpReject case to handleUnsolicitedPDU in session.go — extract reason code and propagate error"
    - "Fix negotiation test to handle or expect Reject for edge-case ImmediateData/InitialR2T combinations"
- truth: "Out-of-range LBA write returns SCSIError with SenseKey 0x05 (ILLEGAL_REQUEST), ASC 0x21, ASCQ 0x00"
  status: failed
  reason: "User reported: SenseKey got 0x00, want 0x05. ASC/ASCQ got 0x00/0x00, want 0x21/0x00. CHECK CONDITION status 0x02 is correct but sense data fields are all zeros — not parsed from response PDU."
  severity: major
  test: 5
  root_cause: "SCSIError is created with SCSI status byte but sense data from the SCSI Response PDU data segment is not being parsed into SenseKey/ASC/ASCQ fields. The error construction path likely only sets the Status field without extracting the fixed-format sense data bytes from the response."
  artifacts:
    - path: "internal/scsi/sense.go"
      issue: "Sense data parsing may not be invoked when constructing SCSIError from SCSI Response PDU"
    - path: "errors.go"
      issue: "SCSIError likely only populated with Status, not SenseKey/ASC/ASCQ from response data segment"
  missing:
    - "Parse sense data from SCSI Response PDU data segment when Status is CHECK CONDITION (0x02)"
    - "Extract SenseKey from byte 2 (& 0x0F), ASC from byte 12, ASCQ from byte 13 of fixed-format sense"
    - "Populate SCSIError.SenseKey, ASC, ASCQ, and generate human-readable Message"
- truth: "ABORT TASK TMF captures ITT of in-flight command and receives valid TMF response (0 or 5)"
  status: failed
  reason: "User reported: Captured ITT is 0x00000000 (invalid/reserved). AbortTask response code is 255 (0xFF, not a valid TMF response). PDU hook ITT capture not working, TMF response parsing returns garbage."
  severity: major
  test: 7
  root_cause: "Two issues: (1) PDU hook ITT extraction reads wrong byte offset — ITT is at BHS bytes 16-19 but hook may receive concatenated data with wrong offset assumption. (2) TMF Response parsing returns 255 (0xFF) instead of valid response code — the response code field extraction from TMF Response PDU BHS is reading the wrong byte."
  artifacts:
    - path: "test/e2e/tmf_test.go"
      issue: "ITT extraction from PDU hook data uses wrong offset — gets 0x00000000 instead of actual ITT"
    - path: "internal/session/tmf.go"
      issue: "TMF Response code extraction may read wrong BHS byte — returns 0xFF instead of valid code"
  missing:
    - "Fix ITT extraction offset in PDU hook callback (ITT is at BHS[16:20] in big-endian)"
    - "Fix TMF Response code extraction from BHS (response code is byte 2 of TMF Response BHS)"
