---
status: complete
phase: 06-error-recovery-and-task-management
source: [06-01-SUMMARY.md, 06-02-SUMMARY.md, 06-03-SUMMARY.md]
started: 2026-04-01T17:30:00Z
updated: 2026-04-01T17:45:00Z
---

## Current Test

[testing complete]

## Tests

### 1. FaultConn injects read/write faults at byte thresholds
expected: All 6 FaultConn subtests pass with -race: passthrough, fault-after-bytes, concurrent safety
result: pass

### 2. TMF methods send correct function codes and auto-cleanup
expected: All 6 TMF methods (AbortTask, AbortTaskSet, ClearTaskSet, LUNReset, TargetWarmReset, TargetColdReset) send correct RFC 7143 function codes with Immediate=true. AbortTask cancels single task with ErrTaskAborted. LUN-scoped TMFs clean all matching tasks.
result: pass

### 3. ERL 0 reconnects after connection failure and retries commands
expected: Session detects connection drop, re-dials target, re-logs in with same ISID+TSIH for reinstatement. In-flight read commands get retried transparently. Seekable write commands retry via Seek(0). Non-seekable writes fail with ErrRetryNotPossible. Max attempt exhaustion returns error.
result: pass

### 4. ERL 1 SNACK detects DataSN gaps and requests retransmission
expected: When Data-In PDUs arrive out of order (gap in DataSN sequence), SNACK Request is sent with correct BegRun/RunLength using task's own ITT. Out-of-order PDUs are buffered and drained after gap fill. Per-task timeout fires Status SNACK for tail loss detection. ERL 0 sessions treat gaps as fatal (no SNACK).
result: pass

### 5. ERL 2 replaces connection with Logout and task reassignment
expected: Failed connection is dropped, new TCP connection established with same ISID+TSIH. Logout with reasonCode=2 sent on new connection for old CID cleanup. TMF TASK REASSIGN sent for each in-flight task. Logout failure is non-fatal (target may have cleaned up). Multiple concurrent tasks all get reassigned.
result: pass

### 6. Submit blocks during recovery and race safety
expected: Submitting a command while ERL 0 recovery is in progress returns ErrSessionRecovering. All session operations are race-safe under concurrent access (-race clean).
result: pass

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
