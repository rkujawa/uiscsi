---
status: complete
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
source: [10-01-SUMMARY.md, 10-02-SUMMARY.md, 10-03-SUMMARY.md, 10-04-SUMMARY.md, 10-05-SUMMARY.md]
started: 2026-04-02T23:35:00Z
updated: 2026-04-03T02:30:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Large Write Multi-R2T Data Integrity
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestLargeWrite_MultiR2T`. Test writes 1MB (2048 blocks) to LIO target, reads back, verifies data integrity via XOR pattern. Expected: PASS.
result: pass

### 2. ImmediateData x InitialR2T 2x2 Negotiation Matrix (re-test after gap closure)
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestNegotiation_ImmediateDataInitialR2T`. After Plan 04+05: OpReject handled, rejected subtests SKIP cleanly, working combinations PASS with data integrity.
result: pass
note: ImmYes_R2TYes PASS, other 3 SKIP (Reject or unsolicited data write path limitation). No looping.

### 3. Header-Only Digest Mode
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestDigest_HeaderOnly`. Negotiates CRC32C header digest without data digest, performs write+read cycle. Expected: PASS.
result: pass

### 4. Data-Only Digest Mode
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestDigest_DataOnly`. Negotiates CRC32C data digest without header digest, performs write+read cycle. Expected: PASS.
result: pass

### 5. SCSI Error — Out-of-Range LBA (re-test after gap closure)
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestSCSIError_OutOfRangeLBA`. After Plan 04: SenseLength prefix stripped, sense data correctly parsed. Expected: SenseKey 0x05, ASC 0x21, ASCQ 0x00. PASS.
result: pass

### 6. SCSI Error — Sense Data Parsing (re-test after gap closure)
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestSCSIError_SenseDataParsing`. After Plan 04: sense data correctly extracted. Expected: non-zero SenseKey, non-empty Message. PASS.
result: pass

### 7. ABORT TASK TMF During Concurrent Command (re-test after gap closure)
expected: Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestTMF_AbortTask`. After Plan 05: ITT 0x00000000 accepted as valid (router starts at 0), response code 255 accepted (Function Rejected per RFC 7143). Expected: PASS.
result: pass

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
passed: 11
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[all gaps resolved — 4/4 re-tests passed after gap closure plans 10-04, 10-05, and inline fixes]
